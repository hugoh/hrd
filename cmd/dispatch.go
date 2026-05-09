package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/urfave/cli/v3"
)

const (
	percentDivisor = 100 // divisor for percentage calculations
	minStatusWidth = 20  // minimum width for status column
	colName        = 1   // column number for repo name
	colVCS         = 2   // column number for VCS type
	colStatus      = 3   // column number for status
	colMsg         = 4   // column number for message
	minNameWidth   = 15  // minimum width for name column
	cmdNameGit     = "git"
	cmdNameShell   = "shell"
	fmtSuccess     = "%d/%d repos completed successfully"
	fmtFailSummary = "%d/%d repos completed successfully; failed: %s"
)

var (
	errNoReposMatched     = errors.New("no repos matched")
	errNoShellCommand     = errors.New("no shell command provided")
	errNoArgsFmt          = errors.New("no args provided")
	errNoReposWithBackend = errors.New("no repos with backend")
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

const cmdReposFlag = "repos"

// loadAndResolve loads the config, resolves the CLI scope, and returns
// both. It returns errNoReposMatched when no repos match.
func loadAndResolve(cfgPath *string, cmd *cli.Command) (config.Config, []string, error) {
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("loading config: %w", err)
	}

	names, err := resolveScope(cmd, &cfg)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("resolving scope: %w", err)
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
		}
	}

	names, err := cfg.ResolveScope(names)
	if err != nil {
		return nil, fmt.Errorf("resolving scope: %w", err)
	}

	return names, nil
}

// vcsArgsFilter returns the subset of args that are not repo/group names.
// It skips "--" separator.
func vcsArgsFilter(
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
				filtered = append(filtered, arg)
			}
		}
	}

	return filtered
}

func vcsArgs(cmd *cli.Command, cfg *config.Config) []string {
	return vcsArgsFilter(cmd.Args().Slice(), cfg.Repos, cfg.Groups)
}

func gitCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:            cmdNameGit,
		Usage:           "run a git command across repos",
		ArgsUsage:       "[repo|group...] -- <git args>",
		SkipFlagParsing: false,
		Flags:           dispatchFlags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runDispatch(ctx, cmd, cfgPath, cmdNameGit)
		},
	}
}

func jjCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:            "jj",
		Usage:           "run a jj command across repos",
		ArgsUsage:       "[repo|group...] -- <jj args>",
		SkipFlagParsing: false,
		Flags:           dispatchFlags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runDispatch(ctx, cmd, cfgPath, "jj")
		},
	}
}

func statusCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"status",
		"show detailed status for repos (git status or jj status)",
	)
}

func diffCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(cfgPath, "diff", "show diff for repos (git diff or jj diff)")
}

func logCmd(cfgPath *string) *cli.Command {
	return vcsSubcmdCmd(cfgPath, "log", "show log for repos (git log or jj log)")
}

func vcsSubcmdCmd(cfgPath *string, subcmd string, usage string) *cli.Command {
	return &cli.Command{
		Name:      subcmd,
		Usage:     usage,
		ArgsUsage: "[repo|group...]",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "repos",
				Aliases: []string{"r"},
				Usage:   "repo names or a group name",
			},
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"i"},
				Usage:   "run with a real terminal (sequential, one repo at a time)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, names, err := loadAndResolve(cfgPath, cmd)
			if err != nil {
				return err
			}

			if cmd.Bool("interactive") {
				return runSubcmdInteractive(ctx, cfg.Repos, names, subcmd)
			}

			return dispatch(names, subcmd, func(resultCh chan<- runner.Result) {
				ch := runner.VCS(ctx, cfg.Repos, names, subcmd, int64(cfg.Settings.Concurrency))
				for res := range ch {
					resultCh <- res
				}
			})
		},
	}
}

func shellCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameShell,
		Usage:     "run an arbitrary shell command across repos",
		ArgsUsage: "[repo|group...] -- <shell command>",
		Flags:     dispatchFlags,
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

	shellArgs := vcsArgs(cmd, &cfg)
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
	var failed []string

	for _, name := range names {
		repo := repos[name]

		if err := runInteractive(ctx, repo.Path, "sh", []string{"-c", cmdStr}); err != nil {
			ui.Errf("%s: %v", name, err)
			failed = append(failed, name)
		}
	}

	dispatchSummary(len(names), failed)
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
		Action: lsAction(cfgPath),
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
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		names, err := resolveScope(cmd, &cfg)
		if err != nil {
			return fmt.Errorf("resolving scope: %w", err)
		}

		if len(names) == 0 {
			ui.Outf("no repos tracked")

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
	var out []string

	for _, n := range names {
		if r, ok := repos[n]; ok && slices.Contains(r.Backends, backendName) {
			out = append(out, n)
		}
	}

	return out
}

func runDispatch(ctx context.Context, cmd *cli.Command, cfgPath *string, backendName string) error {
	cfg, names, err := loadAndResolve(cfgPath, cmd)
	if err != nil {
		return err
	}

	vcsArgs := vcsArgs(cmd, &cfg)
	if len(vcsArgs) == 0 {
		return fmt.Errorf("%w; use: %s %s [repos] -- <args>", errNoArgsFmt, cmdNameHRD, backendName)
	}

	if cmd.Bool("interactive") {
		return dispatchInteractive(ctx, cfg.Repos, names, backendName, vcsArgs)
	}

	return dispatchNonInteractive(ctx, &cfg, names, backendName, vcsArgs)
}

func runInteractive(ctx context.Context, dir, bin string, args []string) error {
	return execInteractive(ctx, dir, bin, args)
}

func dispatchSummary(total int, failed []string) {
	success := total - len(failed)

	if len(failed) > 0 {
		ui.Fail(fmtFailSummary, success, total, strings.Join(failed, ", "))
	} else {
		ui.Success(fmtSuccess, success, total)
	}
}

// runSubcmdInteractive runs subcmd sequentially across repos with a real TTY.
// Each repo uses its own active backend (git or jj).
func runSubcmdInteractive(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	subcmd string,
) error {
	var failed []string

	for _, name := range names {
		repo := repos[name]
		bin := repo.ActiveBackend()

		if bin == "" {
			ui.Errf("%s: no active backend", name)
			failed = append(failed, name)

			continue
		}

		if err := runInteractive(ctx, repo.Path, bin, []string{subcmd}); err != nil {
			ui.Errf("%s: %v", name, err)
			failed = append(failed, name)
		}
	}

	dispatchSummary(len(names), failed)

	return nil
}

func dispatchInteractive(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	backendName string,
	vcsArgs []string,
) error {
	names = filterMatching(names, repos, backendName)
	if len(names) == 0 {
		return fmt.Errorf("%w %s", errNoReposWithBackend, backendName)
	}

	var failed []string

	for _, name := range names {
		repo := repos[name]

		if err := runInteractive(ctx, repo.Path, backendName, vcsArgs); err != nil {
			ui.Errf("%s: %v", name, err)
			failed = append(failed, name)
		}
	}

	dispatchSummary(len(names), failed)

	return nil
}

func dispatchNonInteractive(
	ctx context.Context,
	cfg *config.Config,
	names []string,
	backendName string,
	vcsArgs []string,
) error {
	label := backendName + " " + strings.Join(vcsArgs, " ")

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
			vcsArgs,
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
	results := make([]runner.StatusResult, 0, len(names))

	resultCh := make(chan runner.StatusResult, len(names))
	go func() {
		gather(resultCh)
		close(resultCh)
	}()

	for res := range resultCh {
		results = append(results, res)
	}

	rank := make(map[string]int, len(names))
	for i, name := range names {
		rank[name] = i
	}

	sort.Slice(results, func(indexI, indexJ int) bool {
		return rank[results[indexI].RepoName] < rank[results[indexJ].RepoName]
	})

	tbl := ui.NewTable()

	colConfigs, header := statusTableConfig(details)
	tbl.AppendHeader(header)
	tbl.SetColumnConfigs(colConfigs)

	appendStatusRows(tbl, results, vcsByName, details)

	tbl.Render()

	return nil
}

func statusTableConfig(details bool) ([]table.ColumnConfig, table.Row) {
	const vcsWidth = 3

	var namePercent, statusPercent int

	if !details {
		namePercent = 25
		statusPercent = 75
	} else {
		namePercent = 20
		statusPercent = 37
	}

	termWidth := ui.GetTermWidth()
	nameWidth := max(termWidth*namePercent/percentDivisor, minNameWidth)
	namePlusStatusWidth := termWidth * (namePercent + statusPercent) / percentDivisor
	statusWidth := ui.ComputeRemainderWidth(
		namePlusStatusWidth,
		minStatusWidth,
		nameWidth,
		vcsWidth,
	)

	header := table.Row{"NAME", "VCS", "REF"}

	colConfigs := []table.ColumnConfig{
		{Number: colName, WidthMax: nameWidth, WidthMaxEnforcer: ui.Truncate},
		{Number: colVCS, WidthMax: vcsWidth, WidthMaxEnforcer: ui.Truncate},
		{Number: colStatus, WidthMax: statusWidth},
	}

	if details {
		header = append(header, "MSG")

		msgWidth := ui.ComputeRemainderWidth(
			termWidth,
			minStatusWidth,
			nameWidth,
			vcsWidth,
			statusWidth,
		)
		colConfigs = append(colConfigs, table.ColumnConfig{
			Number: colMsg, WidthMax: msgWidth, WidthMaxEnforcer: ui.Wrap,
		})
	}

	return colConfigs, header
}

func appendStatusRows(
	tbl table.Writer,
	results []runner.StatusResult,
	vcsByName map[string]string,
	details bool,
) {
	for _, res := range results {
		if res.Err != nil {
			tbl.AppendRow(table.Row{
				res.RepoName,
				vcsByName[res.RepoName],
				ui.ColorSprint(text.Colors{text.FgRed}, fmt.Sprintf("%v", res.Err)),
			})

			continue
		}

		status := res.Status

		var statusStr string

		if len(status.Bookmarks) > 0 {
			statusStr = status.Bookmarks[0].Name
		} else {
			statusStr = status.Ref
		}

		var symbols []string

		for _, bookmark := range status.Bookmarks {
			symbols = append(symbols, bookmarkSymbols(bookmark)...)
		}

		if status.Dirty {
			symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgYellow}, "*"))
		}

		if status.Conflict {
			symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgRed}, "‼"))
		}

		symStr := strings.Join(symbols, "")
		situColor := statusColor(status.OverallState)
		combined := situColor.Sprintf("%s %s", statusStr, symStr)
		row := []any{
			res.RepoName,
			vcsByName[res.RepoName],
			combined,
		}

		if details {
			row = append(row, formatDetail(res.Status.CommitMsg, res.Status.CommitTime))
		}

		tbl.AppendRow(row)
	}
}

func bookmarkSymbols(bookmark backend.BookmarkStatus) []string {
	var symbols []string

	switch bookmark.State {
	case backend.RefStateSynced:
		symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgGreen}, "✓"))
	case backend.RefStateAhead:
		symbols = append(
			symbols,
			ui.ColorSprint(text.Colors{text.FgBlue}, fmt.Sprintf("↑%d", bookmark.Ahead)),
		)
	case backend.RefStateBehind:
		symbols = append(
			symbols,
			ui.ColorSprint(text.Colors{text.FgYellow}, fmt.Sprintf("↓%d", bookmark.Behind)),
		)
	case backend.RefStateDiverged:
		symbols = append(
			symbols,
			ui.ColorSprint(
				text.Colors{text.FgRed},
				fmt.Sprintf("↑%d↓%d", bookmark.Ahead, bookmark.Behind),
			),
		)
	case backend.RefStateGone:
		symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgMagenta}, "✗"))
	case backend.RefStateNoRemote:
		symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgMagenta}, "∅"))
	case backend.RefStateUnknown:
		symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgMagenta}, "?"))
	}

	if bookmark.Conflict {
		symbols = append(symbols, ui.ColorSprint(text.Colors{text.FgRed}, "!"))
	}

	return symbols
}

func statusColor(state backend.RefState) text.Colors {
	switch state {
	case backend.RefStateSynced:
		return text.Colors{text.FgGreen}
	case backend.RefStateAhead:
		return text.Colors{text.FgBlue}
	case backend.RefStateBehind:
		return text.Colors{text.FgYellow}
	case backend.RefStateDiverged:
		return text.Colors{text.FgRed}
	case backend.RefStateGone, backend.RefStateNoRemote, backend.RefStateUnknown:
		return text.Colors{text.FgMagenta}
	default:
		return text.Colors{text.FgHiBlack}
	}
}

func formatDetail(msg, time string) string {
	if msg == "" {
		return time
	}

	if time == "" {
		return msg
	}

	return msg + " " + time
}
