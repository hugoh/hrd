package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

const (
	cmdNameShell = "shell"

	cmdReposFlag = "repos"
)

var (
	errNoReposMatched     = errors.New("no repos matched")
	errNoShellCommand     = errors.New("no shell command provided")
	errNoArgsFmt          = errors.New("no args provided")
	errNoReposWithBackend = errors.New("no repos with backend")
	errNonZeroExit        = errors.New("non-zero exit")
	errUnknownScope       = errors.New("unknown repo or group")
)

//nolint:gochecknoglobals // CLI flag definitions are package-level by nature
var dispatchFlags = []cli.Flag{
	&cli.StringSliceFlag{
		Name:    cmdReposFlag,
		Aliases: []string{"r"},
		Usage:   "comma-separated repo names or a single group name",
	},
	&cli.BoolFlag{
		Name:    "interactive",
		Aliases: []string{"i"},
		Usage:   "run with a real terminal (sequential, one repo at a time)",
	},
}

// loadAndResolve loads the config, resolves the CLI scope, and returns
// both. It returns errNoReposMatched when no repos match.
func loadAndResolve(cfgPath *string, cmd *cli.Command) (config.Config, []string, error) {
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("dispatch: %w", err)
	}

	names, err := resolveScope(cmd, &cfg)
	if err != nil {
		return config.Config{}, nil, err
	}

	if len(names) == 0 {
		return config.Config{}, nil, errNoReposMatched
	}

	return cfg, names, nil
}

func resolveScope(cmd *cli.Command, cfg *config.Config) ([]string, error) {
	var names []string

	names = append(names, cmd.StringSlice("repos")...)
	for _, arg := range cmd.Args().Slice() {
		if _, ok := cfg.Repos[arg]; ok {
			names = append(names, arg)
		} else if _, ok := cfg.Groups[arg]; ok {
			names = append(names, arg)
		} else if _, ok := cfg.Groups[stripGroupPrefix(arg)]; ok {
			names = append(names, stripGroupPrefix(arg))
		} else if strings.HasPrefix(arg, "@") {
			return nil, fmt.Errorf("%w: %s", errUnknownScope, arg)
		}
	}

	names, err := cfg.ResolveScope(names)
	if err != nil {
		return nil, fmt.Errorf("resolving scope: %w", err)
	}

	return names, nil
}

// cmdArgsFilter returns the subset of args that are not repo/group names.
// It skips "--" separator.
func cmdArgsFilter(
	args []string,
	repos map[string]config.Repo,
	groups map[string]config.Group,
) []string {
	var filtered []string

	for _, arg := range args {
		if arg == "--" {
			continue
		}

		if _, ok := repos[arg]; !ok {
			if _, ok := groups[arg]; !ok {
				if _, ok := groups[stripGroupPrefix(arg)]; !ok {
					filtered = append(filtered, arg)
				}
			}
		}
	}

	return filtered
}

func cmdArgs(cmd *cli.Command, cfg *config.Config) []string {
	return cmdArgsFilter(cmd.Args().Slice(), cfg.Repos, cfg.Groups)
}

func vcsCmd(cfgPath *string, name string) *cli.Command {
	return &cli.Command{
		Name:            name,
		Usage:           fmt.Sprintf("run a %s command across repos", name),
		ArgsUsage:       "[repo|group...] -- <args>",
		SkipFlagParsing: false,
		Flags:           dispatchFlags,
		ShellComplete:   repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runDispatch(ctx, cmd, cfgPath, name)
		},
	}
}

func statusCmd(cfgPath *string) *cli.Command {
	cmd := vcsSubcmdCmd(
		cfgPath,
		"status",
		"show detailed status for repos (git status or jj status)",
	)
	cmd.Aliases = []string{"st"}

	return cmd
}

func diffCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(cfgPath, "diff", "show diff for repos (git diff or jj diff)")
}

func logCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(cfgPath, "log", "show log for repos (git log or jj log)")
}

func fetchCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"fetch",
		"fetch from remotes (git fetch or jj git fetch)",
	)
}

func pushCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"push",
		"push to remotes (git push or jj git push)",
	)
}

func pullCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"pull",
		"pull from remotes (git pull or jj git pull)",
	)
}

func vcsSubcmdCmd(cfgPath *string, subcmd string, usage string) *cli.Command {
	return &cli.Command{
		Name:      subcmd,
		Usage:     usage,
		ArgsUsage: "[repo|group...]",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    cmdReposFlag,
				Aliases: []string{"r"},
				Usage:   "repo names or a group name",
			},
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"i"},
				Usage:   "run with a real terminal (sequential, one repo at a time)",
			},
		},
		ShellComplete: repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, names, err := loadAndResolve(cfgPath, cmd)
			if err != nil {
				return err
			}

			if cmd.Bool("interactive") {
				return runSubcmdInteractive(ctx, cfg.Repos, names, subcmd)
			}

			return dispatch(names, subcmd, func(resultCh chan<- runner.Result) {
				ch := runner.VCSSubcmd(
					ctx,
					cfg.Repos,
					names,
					subcmd,
					int64(cfg.Settings.Concurrency),
				)
				for res := range ch {
					resultCh <- res
				}
			})
		},
	}
}

func shellCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:          cmdNameShell,
		Usage:         "run an arbitrary shell command across repos",
		ArgsUsage:     "[repo|group...] -- <shell command>",
		Flags:         dispatchFlags,
		ShellComplete: repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return shellCmdAction(ctx, cmd, cfgPath)
		},
	}
}

func shellCmdAction(ctx context.Context, cmd *cli.Command, cfgPath *string) error {
	cfg, names, err := loadAndResolve(cfgPath, cmd)
	if err != nil {
		return err
	}

	shellArgs := cmdArgs(cmd, &cfg)
	if len(shellArgs) == 0 {
		return errNoShellCommand
	}

	shellCmdStr := strings.Join(shellArgs, " ")

	if cmd.Bool("interactive") {
		runShellInteractive(ctx, cfg.Repos, names, shellCmdStr)

		return nil
	}

	return dispatch(
		names,
		"shell: "+shellCmdStr,
		func(resultCh chan<- runner.Result) {
			dispatchCh := runner.Shell(
				ctx,
				cfg.Repos,
				names,
				shellCmdStr,
				int64(cfg.Settings.Concurrency),
			)
			for res := range dispatchCh {
				resultCh <- res
			}
		},
	)
}

func runShellInteractive(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	cmdStr string,
) {
	runInteractiveEach(
		ctx,
		repos,
		names,
		func(ctx context.Context, _ string, repo config.Repo) error {
			return runInteractive(ctx, repo.Path, "sh", []string{"-c", cmdStr})
		},
	)
}

func lsCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Aliases:   []string{"ll"},
		Usage:     "show status of repos",
		ArgsUsage: "[repo|group...]",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "repos",
				Aliases: []string{"r"},
				Usage:   "repo names or a group name",
			},
			&cli.BoolFlag{
				Name:    "message",
				Aliases: []string{"m"},
				Usage:   "show commit message and time",
			},
			&cli.BoolFlag{
				Name:    "names",
				Aliases: []string{"n"},
				Usage:   "show repo names only, one per line",
			},
			&cli.BoolFlag{
				Name:    "dirs",
				Aliases: []string{"d"},
				Usage:   "show repo dirs only, one per line",
			},
		},
		ShellComplete: repoGroupCompleter(cfgPath),
		Action:        lsAction(cfgPath),
	}
}

func lsNamesOnly(names []string) {
	for _, n := range names {
		ui.Outf(n)
	}
}

func lsDirsOnly(repos map[string]config.Repo, names []string) {
	for _, n := range names {
		if repo, ok := repos[n]; ok {
			ui.Outf(repo.Path)
		}
	}
}

func lsGatherCallback(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
) func(chan<- runner.StatusResult) {
	return func(resultCh chan<- runner.StatusResult) {
		statusCh := runner.GatherStatus(
			ctx,
			repos,
			names,
			concurrency,
		)
		for res := range statusCh {
			resultCh <- res
		}
	}
}

func lsAction(cfgPath *string) func(context.Context, *cli.Command) error {
	return func(ctx context.Context, cmd *cli.Command) error {
		cfg, names, err := loadAndResolve(cfgPath, cmd)
		if errors.Is(err, errNoReposMatched) {
			return nil
		}

		if err != nil {
			return err
		}

		switch {
		case cmd.Bool("names"):
			lsNamesOnly(names)

			return nil
		case cmd.Bool("dirs"):
			lsDirsOnly(cfg.Repos, names)

			return nil
		}

		vcsByName := make(map[string]string, len(names))
		for _, n := range names {
			vcsByName[n] = cfg.Repos[n].ActiveBackend()
		}

		// ll alias implies -m (message)
		message := cmd.Bool("message")
		if cmd.Name == "ll" || cmd.Root().Args().First() == "ll" {
			message = true
		}

		return gatherStatus(
			names,
			vcsByName,
			message,
			lsGatherCallback(ctx, cfg.Repos, names, int64(cfg.Settings.Concurrency)),
		)
	}
}

func filterMatching(names []string, repos map[string]config.Repo, backendName string) []string {
	bck, err := backend.ByName(backendName)
	if err != nil {
		return nil
	}

	var out []string

	for _, n := range names {
		if r, ok := repos[n]; ok {
			ok, _ := bck.Detect(r.Path)
			if ok {
				out = append(out, n)
			}
		}
	}

	return out
}

func runDispatch(ctx context.Context, cmd *cli.Command, cfgPath *string, backendName string) error {
	cfg, names, err := loadAndResolve(cfgPath, cmd)
	if err != nil {
		return err
	}

	cmdArgs := cmdArgs(cmd, &cfg)
	if len(cmdArgs) == 0 {
		return fmt.Errorf("%w; use: %s %s [repos] -- <args>", errNoArgsFmt, cmdNameHRD, backendName)
	}

	if cmd.Bool("interactive") {
		return dispatchInteractive(ctx, cfg.Repos, names, backendName, cmdArgs)
	}

	return dispatchNonInteractive(ctx, &cfg, names, backendName, cmdArgs)
}

func runInteractive(ctx context.Context, dir, bin string, args []string) error {
	return execInteractive(ctx, dir, bin, args)
}

// runInteractiveEach executes fn sequentially for each repo, collecting
// failures and printing a summary. This is the shared loop body for all
// interactive dispatch variants.
func runInteractiveEach(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	fn func(context.Context, string, config.Repo) error,
) {
	var failed []string

	for _, name := range names {
		repo := repos[name]

		if err := fn(ctx, name, repo); err != nil {
			ui.Errf("%s: %v", name, err)
			failed = append(failed, name)
		}
	}

	dispatchSummary(len(names), failed)
}

func dispatchSummary(total int, failed []string) {
	if len(failed) > 0 {
		ui.Fail("%s", ui.FormatSummary(total, failed))
	} else {
		ui.Success("%s", ui.FormatSummary(total, failed))
	}
}

// runSubcmdInteractive runs subcmd sequentially across repos with a real TTY.
// Each repo uses its own active backend (git or jj), resolving arg prefixes
// like "git fetch" for jj automatically.
func runSubcmdInteractive(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	subcmd string,
) error {
	runInteractiveEach(
		ctx,
		repos,
		names,
		func(ctx context.Context, _ string, repo config.Repo) error {
			bck, err := backend.ByName(repo.ActiveBackend())
			if err != nil {
				return fmt.Errorf("checking backend: %w", err)
			}

			res, err := bck.Run(ctx, repo.Path, bck.SubcommandArgs(subcmd), true)
			if err != nil {
				return fmt.Errorf("%s %s: %w", bck.Name(), subcmd, err)
			}

			if res.ExitCode != 0 {
				return fmt.Errorf("%s %s: %w", bck.Name(), subcmd, errNonZeroExit)
			}

			return nil
		},
	)

	return nil
}

func dispatchInteractive(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	backendName string,
	cmdArgs []string,
) error {
	names = filterMatching(names, repos, backendName)
	if len(names) == 0 {
		return fmt.Errorf("%w %s", errNoReposWithBackend, backendName)
	}

	runInteractiveEach(
		ctx,
		repos,
		names,
		func(ctx context.Context, _ string, repo config.Repo) error {
			return runInteractive(ctx, repo.Path, backendName, cmdArgs)
		},
	)

	return nil
}

func dispatchNonInteractive(
	ctx context.Context,
	cfg *config.Config,
	names []string,
	backendName string,
	cmdArgs []string,
) error {
	label := backendName + " " + strings.Join(cmdArgs, " ")

	names = filterMatching(names, cfg.Repos, backendName)
	if len(names) == 0 {
		return fmt.Errorf("%w %s", errNoReposWithBackend, backendName)
	}

	return dispatch(names, label, func(resultCh chan<- runner.Result) {
		dispatchCh, err := runner.Dispatch(
			ctx,
			cfg.Repos,
			names,
			backendName,
			cmdArgs,
			int64(cfg.Settings.Concurrency),
		)
		if err != nil {
			return
		}

		for res := range dispatchCh {
			resultCh <- res
		}
	})
}

func dispatch(
	names []string,
	cmdLabel string,
	dispatch func(resultCh chan<- runner.Result),
) error {
	ui.Outf(cmdLabel)

	resultCh := make(chan runner.Result, len(names))
	go func() {
		defer close(resultCh)

		dispatch(resultCh)
	}()

	results := make([]runner.Result, len(names))

	resultIdx := make(map[string]int, len(names))
	for i, n := range names {
		resultIdx[n] = i
	}

	for res := range resultCh {
		printDispatchResult(res)

		results[resultIdx[res.RepoName]] = res
	}

	var errs []string

	for _, r := range results {
		if r.Err != nil || r.ExitCode != 0 {
			errs = append(errs, r.RepoName)
		}
	}

	dispatchSummary(len(names), errs)

	return nil
}

func printDispatchResult(res runner.Result) {
	ui.Outf(ui.RenderDispatchResult(res))

	if res.Err != nil {
		ui.Errf("%s", res.Err)
	} else if res.Output != "" {
		for line := range strings.SplitSeq(strings.TrimRight(res.Output, "\n"), "\n") {
			ui.Outf("  " + line)
		}
	}

	ui.Outf("")
}

func gatherStatus(
	names []string,
	vcsByName map[string]string,
	details bool,
	gather func(resultCh chan<- runner.StatusResult),
) error {
	if len(names) == 0 {
		return nil
	}

	widths, header := statusTableConfig(details)

	resultCh := make(chan runner.StatusResult, len(names))
	go func() {
		gather(resultCh)
		close(resultCh)
	}()

	nameIdx := make(map[string]int, len(names))
	for i, n := range names {
		nameIdx[n] = i
	}

	pending := make([][]string, len(names))
	for i, name := range names {
		pending[i] = statusRow(name, vcsByName[name], nil, details)
	}

	const colStatus = 2

	eff := ui.EffectiveWidths(header, pending, widths)
	if len(eff) > colStatus {
		eff[colStatus] = widths[colStatus]
	}

	ui.Outf(ui.RenderHeader(header, eff))

	results := make([]*runner.StatusResult, len(names))
	next := 0

	for res := range resultCh {
		idx := nameIdx[res.RepoName]
		results[idx] = &res

		for next < len(names) && results[next] != nil {
			cells := statusRow(names[next], vcsByName[names[next]], results[next], details)
			ui.Outf(ui.RenderRow(cells, eff))

			next++
		}
	}

	return nil
}

const placeholder = "..."

func statusRow(name, vcs string, res *runner.StatusResult, details bool) []string {
	if res == nil {
		return []string{name, vcs, placeholder}
	}

	if res.Err != nil {
		return []string{name, vcs, ui.ColorSprint("red", fmt.Sprintf("%v", res.Err))}
	}

	line := ui.FormatDispatchStatusLine(res.Status, details)

	return []string{name, vcs, line}
}

func statusTableConfig(details bool) ([]int, []string) {
	const (
		minNameWidth   = 15
		minStatusWidth = 20
		vcsWidth       = 3
	)

	type layoutWeights struct{ name, status int }

	weights := layoutWeights{name: 25, status: 75} //nolint:mnd
	if details {
		weights = layoutWeights{name: 20, status: 80} //nolint:mnd
	}

	termWidth := ui.GetTermWidth()
	pct := func(p int) int { return termWidth * p / 100 } //nolint:mnd

	nameWidth := max(pct(weights.name), minNameWidth)
	statusWidth := ui.ComputeRemainderWidth(termWidth, minStatusWidth, nameWidth, vcsWidth)

	return []int{nameWidth, vcsWidth, statusWidth}, []string{NameLabel, VCSLabel, StatusLabel}
}
