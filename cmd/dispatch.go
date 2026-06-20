package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
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

// ErrReposFailed marks a run where the command executed but failed in at
// least one repo. main maps it to exit code 1 (vs 2 for usage/config
// errors) and must not re-print it: dispatchSummary already reported the
// failures on stderr.
var ErrReposFailed = errors.New("repos failed")

//nolint:gochecknoglobals // CLI flag definitions are package-level by nature
var dispatchFlags = append([]cli.Flag{
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
}, statusFilterFlags...)

// loadAndResolve loads the config and resolves the CLI scope (-r flag plus
// positional args, all of which must be known repos or groups). Args after
// a "--" separator are scope too for these commands — they take no
// subprocess args. The loaded config is returned alongside
// errNoReposMatched so callers can inspect it (e.g. for the ls onboarding
// hint).
func loadAndResolve(
	cfgPath *string,
	cmd *cli.Command,
	dashTail []string,
) (config.Config, []string, error) {
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("dispatch: %w", err)
	}

	names, err := resolveScope(cmd, dashTail, &cfg)
	if err != nil {
		return config.Config{}, nil, err
	}

	if len(names) == 0 {
		return cfg, nil, errNoReposMatched
	}

	return cfg, names, nil
}

func resolveScope(cmd *cli.Command, dashTail []string, cfg *config.Config) ([]string, error) {
	names := slices.Clone(cmd.StringSlice(cmdReposFlag))

	for _, arg := range slices.Concat(cmd.Args().Slice(), dashTail) {
		name, ok := scopeName(arg, cfg)
		if !ok {
			return nil, fmt.Errorf("%w: %s", errUnknownScope, arg)
		}

		names = append(names, name)
	}

	names, err := cfg.ResolveScope(names)
	if err != nil {
		return nil, fmt.Errorf("resolving scope: %w", err)
	}

	return names, nil
}

// scopeName reports whether arg names a known repo or group, returning the
// canonical name (group "@" prefix stripped).
func scopeName(arg string, cfg *config.Config) (string, bool) {
	if _, ok := cfg.Repos[arg]; ok {
		return arg, true
	}

	if _, ok := cfg.Groups[arg]; ok {
		return arg, true
	}

	if g := stripGroupPrefix(arg); g != arg {
		if _, ok := cfg.Groups[g]; ok {
			return g, true
		}
	}

	return "", false
}

// loadAndSplit loads the config and splits positional args into scope and
// subprocess args for the commands that take trailing command args (git,
// jj, shell). See splitScopeArgs for the split rules.
func loadAndSplit(
	cfgPath *string,
	cmd *cli.Command,
	dashTail []string,
) (config.Config, []string, []string, error) {
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("dispatch: %w", err)
	}

	scope, cmdArgs, err := splitScopeArgs(cmd.Args().Slice(), dashTail, &cfg)
	if err != nil {
		return config.Config{}, nil, nil, err
	}

	names := slices.Concat(cmd.StringSlice(cmdReposFlag), scope)

	resolved, err := cfg.ResolveScope(names)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("resolving scope: %w", err)
	}

	if len(resolved) == 0 {
		return cfg, nil, nil, errNoReposMatched
	}

	return cfg, resolved, cmdArgs, nil
}

// splitScopeArgs splits positional args into repo/group scope and
// subprocess args. dashTail is the verbatim argv after "--" (nil when no
// separator was given; see SplitDashTail). With a separator, every
// positional must be a known repo or group and the tail passes through
// untouched — repo names are never filtered out of the command. Without
// one, the leading args that match repos/groups form the scope and the
// first non-matching arg starts the subprocess args.
func splitScopeArgs(args, dashTail []string, cfg *config.Config) ([]string, []string, error) {
	var scope []string

	for i, arg := range args {
		if name, ok := scopeName(arg, cfg); ok {
			scope = append(scope, name)

			continue
		}

		if dashTail != nil || strings.HasPrefix(arg, "@") {
			return nil, nil, fmt.Errorf("%w: %s", errUnknownScope, arg)
		}

		return scope, args[i:], nil
	}

	return scope, dashTail, nil
}

var shSafeToken = regexp.MustCompile(`^[\w@%+=:,./~^-]+$`)

// shQuote single-quotes a token for POSIX sh unless it is already safe.
func shQuote(s string) string {
	if shSafeToken.MatchString(s) {
		return s
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellJoin reassembles tokenized args into a single `sh -c` command
// string, quoting tokens so arg boundaries survive. A single arg (the
// `hrd shell -- 'cmd ...'` form) passes through verbatim so shell syntax
// like pipes and expansions keeps working.
func shellJoin(args []string) string {
	if len(args) == 1 {
		return args[0]
	}

	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shQuote(a)
	}

	return strings.Join(quoted, " ")
}

func vcsCmd(cfgPath *string, name string, dashTail []string) *cli.Command {
	return &cli.Command{
		Name:            name,
		Usage:           fmt.Sprintf("run a %s command across repos", name),
		ArgsUsage:       "[repo|group...] -- <args>",
		SkipFlagParsing: false,
		Flags:           dispatchFlags,
		ShellComplete:   repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runDispatch(ctx, cmd, cfgPath, name, dashTail)
		},
	}
}

func statusCmd(cfgPath *string, dashTail []string) *cli.Command {
	cmd := vcsSubcmdCmd(
		cfgPath,
		dashTail,
		"status",
		"show detailed status for repos (git status or jj status)",
	)
	cmd.Aliases = []string{"st"}

	return cmd
}

func diffCmd(cfgPath *string, dashTail []string) *cli.Command {
	return vcsSubcmdCmd(cfgPath, dashTail, "diff", "show diff for repos (git diff or jj diff)")
}

func logCmd(cfgPath *string, dashTail []string) *cli.Command {
	return vcsSubcmdCmd(cfgPath, dashTail, "log", "show log for repos (git log or jj log)")
}

func fetchCmd(cfgPath *string, dashTail []string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		dashTail,
		"fetch",
		"fetch from remotes (git fetch or jj git fetch)",
	)
}

func pushCmd(cfgPath *string, dashTail []string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		dashTail,
		"push",
		"push to remotes (git push or jj git push)",
	)
}

func pullCmd(cfgPath *string, dashTail []string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		dashTail,
		"pull",
		"pull from remotes (git pull or jj git pull)",
	)
}

func vcsSubcmdCmd(cfgPath *string, dashTail []string, subcmd string, usage string) *cli.Command {
	return &cli.Command{
		Name:      subcmd,
		Usage:     usage,
		ArgsUsage: "[repo|group...]",
		Flags: append([]cli.Flag{
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
		}, statusFilterFlags...),
		ShellComplete: repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, names, err := loadAndResolve(cfgPath, cmd, dashTail)
			if err != nil {
				return err
			}

			names, _ = applyStatusFilter(ctx, cmd, &cfg, names)
			if len(names) == 0 {
				return nil
			}

			if cmd.Bool("interactive") {
				return runSubcmdInteractive(ctx, cfg.Repos, names, subcmd, nil)
			}

			return dispatch(names, subcmd, func(resultCh chan<- runner.Result) {
				ch := runner.VCSSubcmd(
					ctx,
					cfg.Repos,
					names,
					subcmd,
					nil,
					cfg.Settings.Concurrency,
				)
				for res := range ch {
					resultCh <- res
				}
			})
		},
	}
}

func shellCmd(cfgPath *string, dashTail []string) *cli.Command {
	return &cli.Command{
		Name:          cmdNameShell,
		Usage:         "run an arbitrary shell command across repos",
		ArgsUsage:     "[repo|group...] -- <shell command>",
		Flags:         dispatchFlags,
		ShellComplete: repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return shellCmdAction(ctx, cmd, cfgPath, dashTail)
		},
	}
}

func shellCmdAction(
	ctx context.Context,
	cmd *cli.Command,
	cfgPath *string,
	dashTail []string,
) error {
	cfg, names, shellArgs, err := loadAndSplit(cfgPath, cmd, dashTail)
	if err != nil {
		return err
	}

	if len(shellArgs) == 0 {
		return errNoShellCommand
	}

	names, _ = applyStatusFilter(ctx, cmd, &cfg, names)
	if len(names) == 0 {
		return nil
	}

	return runShell(ctx, &cfg, names, shellJoin(shellArgs), cmd.Bool("interactive"))
}

// runShell dispatches a shell command string across names, sequentially
// with a TTY when interactive, in parallel otherwise.
func runShell(
	ctx context.Context,
	cfg *config.Config,
	names []string,
	shellCmdStr string,
	interactive bool,
) error {
	if interactive {
		return runShellInteractive(ctx, cfg.Repos, names, shellCmdStr)
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
				cfg.Settings.Concurrency,
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
) error {
	bin, flag := runner.ShellCommand()

	return runInteractiveEach(
		ctx,
		repos,
		names,
		func(ctx context.Context, _ string, repo config.Repo) error {
			return runInteractive(ctx, repo.Path, bin, []string{flag, cmdStr})
		},
	)
}

func lsCmd(cfgPath *string, dashTail []string) *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Usage:     "show status of repos",
		ArgsUsage: "[repo|group...]",
		Flags: append([]cli.Flag{
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
		}, statusFilterFlags...),
		ShellComplete: repoGroupCompleter(cfgPath),
		Action:        lsAction(cfgPath, dashTail, false),
	}
}

// llCmd is lsCmd with the message column always on.
func llCmd(cfgPath *string, dashTail []string) *cli.Command {
	cmd := lsCmd(cfgPath, dashTail)
	cmd.Name = "ll"
	cmd.Usage = "show status of repos with commit message and time"
	cmd.Action = lsAction(cfgPath, dashTail, true)

	return cmd
}

func lsNamesOnly(names []string) {
	for _, n := range names {
		ui.Out(n)
	}
}

func lsDirsOnly(repos map[string]config.Repo, names []string) {
	for _, n := range names {
		if repo, ok := repos[n]; ok {
			ui.Out(repo.Path)
		}
	}
}

func lsGatherCallback(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int,
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

func lsAction(
	cfgPath *string,
	dashTail []string,
	forceMessage bool,
) func(context.Context, *cli.Command) error {
	return func(ctx context.Context, cmd *cli.Command) error {
		cfg, names, err := loadAndResolve(cfgPath, cmd, dashTail)
		if errors.Is(err, errNoReposMatched) {
			if len(cfg.Repos) == 0 {
				ui.Warnf("no repos tracked — run \"%s repo add <path>\" to get started", cmdNameHRD)
			} else {
				ui.Warnf("no repos matched")
			}

			return nil
		}

		if err != nil {
			return err
		}

		names, statuses := applyStatusFilter(ctx, cmd, &cfg, names)
		if len(names) == 0 {
			return nil
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

		message := forceMessage || cmd.Bool("message")

		gather := lsGatherCallback(ctx, cfg.Repos, names, cfg.Settings.Concurrency)
		if statuses != nil {
			gather = replayGather(names, statuses)
		}

		return gatherStatus(names, vcsByName, message, gather)
	}
}

// replayGather feeds pre-gathered statuses (from the status filter) into
// the rendering pipeline instead of hitting the repos a second time.
func replayGather(
	names []string,
	statuses map[string]runner.StatusResult,
) func(chan<- runner.StatusResult) {
	return func(resultCh chan<- runner.StatusResult) {
		for _, n := range names {
			resultCh <- statuses[n]
		}
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

func runDispatch(
	ctx context.Context,
	cmd *cli.Command,
	cfgPath *string,
	backendName string,
	dashTail []string,
) error {
	cfg, names, cmdArgs, err := loadAndSplit(cfgPath, cmd, dashTail)
	if err != nil {
		return err
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("%w; use: %s %s [repos] -- <args>", errNoArgsFmt, cmdNameHRD, backendName)
	}

	names, _ = applyStatusFilter(ctx, cmd, &cfg, names)
	if len(names) == 0 {
		return nil
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
) error {
	var failed []string

	for _, name := range names {
		repo := repos[name]

		if err := fn(ctx, name, repo); err != nil {
			ui.Errf("%s: %v", name, err)
			failed = append(failed, name)
		}
	}

	return dispatchSummary(len(names), failed)
}

// dispatchSummary prints the success/failure summary and returns
// ErrReposFailed when any repo failed, so callers can surface a non-zero
// exit code.
func dispatchSummary(total int, failed []string) error {
	if len(failed) > 0 {
		ui.Fail("%s", ui.FormatSummary(total, failed))

		return fmt.Errorf("%w: %d/%d", ErrReposFailed, len(failed), total)
	}

	ui.Success("%s", ui.FormatSummary(total, failed))

	return nil
}

// runSubcmdInteractive runs subcmd sequentially across repos with a real TTY.
// Each repo uses its own active backend (git or jj), resolving arg prefixes
// like "git fetch" for jj automatically. extraArgs (may be nil) are appended
// after the resolved subcommand args.
func runSubcmdInteractive(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	subcmd string,
	extraArgs []string,
) error {
	return runInteractiveEach(
		ctx,
		repos,
		names,
		func(ctx context.Context, _ string, repo config.Repo) error {
			bck, err := backend.ByName(repo.ActiveBackend())
			if err != nil {
				return fmt.Errorf("checking backend: %w", err)
			}

			args := append(bck.SubcommandArgs(subcmd), extraArgs...)

			res, err := bck.Run(ctx, repo.Path, args, true)
			if err != nil {
				return fmt.Errorf("%s %s: %w", bck.Name(), subcmd, err)
			}

			if res.ExitCode != 0 {
				return fmt.Errorf("%s %s: %w", bck.Name(), subcmd, errNonZeroExit)
			}

			return nil
		},
	)
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

	return runInteractiveEach(
		ctx,
		repos,
		names,
		func(ctx context.Context, _ string, repo config.Repo) error {
			return runInteractive(ctx, repo.Path, backendName, cmdArgs)
		},
	)
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

	dispatchCh, err := runner.Dispatch(
		ctx,
		cfg.Repos,
		names,
		backendName,
		cmdArgs,
		cfg.Settings.Concurrency,
	)
	if err != nil {
		return fmt.Errorf("dispatch %s: %w", backendName, err)
	}

	return dispatch(names, label, func(resultCh chan<- runner.Result) {
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
	ui.Out(cmdLabel)

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

	return dispatchSummary(len(names), errs)
}

func printDispatchResult(res runner.Result) {
	ui.Out(ui.RenderDispatchResult(res))

	if res.Err != nil {
		ui.Errf("%s", res.Err)
	} else if res.Output != "" {
		for line := range strings.SplitSeq(strings.TrimRight(res.Output, "\n"), "\n") {
			ui.Out("  " + line)
		}
	}

	ui.Out("")
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

	ui.Out(ui.RenderHeader(header, eff))

	results := make([]*runner.StatusResult, len(names))
	next := 0

	for res := range resultCh {
		idx := nameIdx[res.RepoName]
		results[idx] = &res

		for next < len(names) && results[next] != nil {
			cells := statusRow(names[next], vcsByName[names[next]], results[next], details)
			ui.Out(ui.RenderRow(cells, eff))

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
		return []string{name, vcs, ui.ApplyColor("red", fmt.Sprintf("%v", res.Err))}
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

	weights := layoutWeights{name: 25, status: 75} //nolint:mnd // layout weight percentages
	if details {
		weights = layoutWeights{name: 20, status: 80} //nolint:mnd // layout weight percentages
	}

	termWidth := ui.GetTermWidth()
	pct := func(p int) int { return termWidth * p / 100 } //nolint:mnd // percentage conversion

	nameWidth := max(pct(weights.name), minNameWidth)
	statusWidth := ui.ComputeRemainderWidth(termWidth, minStatusWidth, nameWidth, vcsWidth)

	return []int{nameWidth, vcsWidth, statusWidth}, []string{NameLabel, VCSLabel, StatusLabel}
}
