package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cmdNameShell = "shell"

// progressPercentMax converts a done/total ratio to the 0-100 scale expected
// by ui.ProgressOSC.
const progressPercentMax = 100

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

// addDispatchFlags registers the flags shared by every command that
// dispatches a VCS operation across a repo scope.
func addDispatchFlags(fs *pflag.FlagSet) {
	fs.BoolP("interactive", "i", false, "run with a real terminal (sequential, one repo at a time)")
	addStatusFilterFlags(fs)
}

// resolvedScope bundles what loadAndResolve/loadAndSplit produce for a
// command's repo scope: the loaded config, the expanded repo names, and
// whether the raw scope explicitly named the "@@attention" reserved
// pseudo-group (attentionRequested) — which forces applyStatusFilter's
// status filter on even without --dirty/--ahead/--behind. Keeping these
// together as one value stops each new cross-cutting scope concern from
// growing every function signature along the loadAndResolve/loadAndSplit →
// applyStatusFilter chain.
//
// attentionRequested is intentionally specific to @@attention, not a
// generic "this scope needs live status" flag: the filter it forces
// (dirty/ahead/behind=true) is exactly @@attention's definition
// (backend.RepoStatus.NeedsAttention()). A future second reserved group
// requiring live status but with different filter semantics would need its
// own mechanism here, not a rename of this one.
type resolvedScope struct {
	cfg                config.Config
	names              []string
	attentionRequested bool
}

// loadAndResolve loads the config and resolves the CLI scope (positional
// args, all of which must be known repos or groups). These commands take no
// subprocess args, so args after a "--" separator are scope too — parsing
// already folds both sides of "--" into one args slice (see splitScopeArgs
// for the commands that do distinguish the two sides). The loaded config is
// returned alongside errNoReposMatched so callers can inspect it (e.g. for
// the ls onboarding hint).
func loadAndResolve(
	cfgPath *string,
	args []string,
) (resolvedScope, error) {
	cfg, err := loadResolvedConfig(cfgPath, "dispatch")
	if err != nil {
		return resolvedScope{}, err
	}

	names, attentionRequested, err := resolveScope(args, &cfg)
	if err != nil {
		return resolvedScope{}, err
	}

	scope := resolvedScope{cfg: cfg, names: names, attentionRequested: attentionRequested}
	if len(names) == 0 {
		return scope, errNoReposMatched
	}

	return scope, nil
}

// hasAttentionScope reports whether names contains the "@@attention"
// reserved pseudo-group token, prior to it being expanded by ResolveScope.
func hasAttentionScope(names []string) bool {
	return slices.Contains(names, config.ReservedAttention)
}

func resolveScope(args []string, cfg *config.Config) ([]string, bool, error) {
	names := make([]string, 0, len(args))

	for _, arg := range args {
		name, ok := scopeName(arg, cfg)
		if !ok {
			return nil, false, fmt.Errorf("%w: %s", errUnknownScope, arg)
		}

		names = append(names, name)
	}

	attentionRequested := hasAttentionScope(names)

	names, err := cfg.ResolveScope(names)
	if err != nil {
		return nil, false, fmt.Errorf("resolving scope: %w", err)
	}

	return names, attentionRequested, nil
}

// scopeName reports whether arg names a known repo, group, or reserved
// pseudo-group, returning the canonical name (group "@" prefix stripped;
// a reserved "@@" token is returned unchanged).
func scopeName(arg string, cfg *config.Config) (string, bool) {
	if config.IsReservedGroupName(arg) {
		return arg, config.IsKnownReservedGroup(arg)
	}

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
	cmd *cobra.Command,
	args []string,
) (resolvedScope, []string, error) {
	cfg, err := loadResolvedConfig(cfgPath, "dispatch")
	if err != nil {
		return resolvedScope{}, nil, err
	}

	rawScope, cmdArgs, err := splitScopeArgs(cmd, args, &cfg)
	if err != nil {
		return resolvedScope{}, nil, err
	}

	attentionRequested := hasAttentionScope(rawScope)

	resolved, err := cfg.ResolveScope(rawScope)
	if err != nil {
		return resolvedScope{}, nil, fmt.Errorf("resolving scope: %w", err)
	}

	scope := resolvedScope{cfg: cfg, names: resolved, attentionRequested: attentionRequested}
	if len(resolved) == 0 {
		return scope, nil, errNoReposMatched
	}

	return scope, cmdArgs, nil
}

// splitScopeArgs splits positional args into repo/group scope and
// subprocess args. cmd.ArgsLenAtDash (see `go doc pflag.FlagSet.ArgsLenAtDash`)
// tells an explicit "--" separator (every positional before it must be a
// known repo or group, and the tail passes through untouched — repo names
// are never filtered out of the command) from no separator at all (the
// leading args that match repos/groups form the scope, and the first
// non-matching arg starts the subprocess args). args is the already-parsed
// positional list, which folds in both sides of "--".
func splitScopeArgs(
	cmd *cobra.Command,
	args []string,
	cfg *config.Config,
) ([]string, []string, error) {
	dashIdx := cmd.ArgsLenAtDash()

	head := args

	var dashTail []string

	if dashIdx >= 0 {
		head, dashTail = args[:dashIdx], args[dashIdx:]
	}

	var scope []string

	for i, arg := range head {
		if name, ok := scopeName(arg, cfg); ok {
			scope = append(scope, name)

			continue
		}

		if dashIdx >= 0 || strings.HasPrefix(arg, "@") {
			return nil, nil, fmt.Errorf("%w: %s", errUnknownScope, arg)
		}

		return scope, head[i:], nil
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

func vcsCmd(cfgPath *string, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " [repo|group...] -- <args>",
		Short: fmt.Sprintf("run a %s command across repos", name),
		Long: fmt.Sprintf(
			`Runs "%s <args>" in each scoped repo. Everything before "--" selects the
scope (repo/group names, or -r/--repos); everything after "--" is passed to
%s verbatim. With no scope, all tracked repos are used. Only repos using the
%s backend are affected; others are skipped. Use -i/--interactive to run
sequentially with a real terminal instead of in parallel.`,
			name, name, name,
		),
		Example: fmt.Sprintf(
			"  hrd %s -- status\n  hrd %s myrepo otherrepo -- log -n5\n  hrd %s @backend -- fetch --all",
			name,
			name,
			name,
		),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDispatch(cmd.Context(), cmd, args, cfgPath, name)
		},
	}
	addDispatchFlags(cmd.Flags())
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}

func statusCmd(cfgPath *string) *cobra.Command {
	cmd := vcsSubcmdCmd(
		cfgPath,
		"status",
		"show detailed status for repos (git status or jj status)",
	)
	cmd.Aliases = []string{"st"}

	return cmd
}

func diffCmd(cfgPath *string) *cobra.Command {
	return vcsSubcmdCmd(cfgPath, "diff", "show diff for repos (git diff or jj diff)")
}

func logCmd(cfgPath *string) *cobra.Command {
	return vcsSubcmdCmd(cfgPath, "log", "show log for repos (git log or jj log)")
}

func fetchCmd(cfgPath *string) *cobra.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"fetch",
		"fetch from remotes (git fetch or jj git fetch)",
	)
}

func pushCmd(cfgPath *string) *cobra.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"push",
		"push to remotes (git push or jj git push)",
	)
}

func pullCmd(cfgPath *string) *cobra.Command {
	return vcsSubcmdCmd(
		cfgPath,
		"pull",
		"pull from remotes (git pull or jj git pull)",
	)
}

func vcsSubcmdCmd(cfgPath *string, subcmd, usage string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   subcmd + " [repo|group...]",
		Short: usage,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			scope, err := loadAndResolve(cfgPath, args)
			if err != nil {
				return err
			}

			names, _ := applyStatusFilter(ctx, cmd, scope)
			if len(names) == 0 {
				return nil
			}

			if flagBool(cmd, "interactive") {
				return runSubcmdInteractive(ctx, scope.cfg.Repos, names, subcmd, nil)
			}

			return dispatch(names, subcmd, func(resultCh chan<- runner.Result) {
				ch := runner.VCSSubcmd(
					ctx,
					scope.cfg.Repos,
					names,
					subcmd,
					nil,
					scope.cfg.Settings.Concurrency,
				)
				for res := range ch {
					resultCh <- res
				}
			})
		},
	}
	addDispatchFlags(cmd.Flags())
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}

func shellCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameShell + " [repo|group...] -- <shell command>",
		Short: "run an arbitrary shell command across repos",
		Long: `Runs a command via "sh -c" in each scoped repo's directory. Everything
before "--" selects the scope (repo/group names, or -r/--repos); everything
after "--" is the command. Pass it as one quoted string to keep shell syntax
like pipes and redirection working, or as separate words to have them
rejoined with automatic quoting. Use -i/--interactive to run sequentially
with a real terminal instead of in parallel.`,
		Example: `  hrd shell -- "go test ./..."
  hrd shell myrepo otherrepo -- "git status --short"
  hrd shell @backend -i -- "npm install"`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return shellCmdAction(cmd, args, cfgPath)
		},
	}
	addDispatchFlags(cmd.Flags())
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}

func shellCmdAction(
	cmd *cobra.Command,
	args []string,
	cfgPath *string,
) error {
	ctx := cmd.Context()

	scope, shellArgs, err := loadAndSplit(cfgPath, cmd, args)
	if err != nil {
		return err
	}

	if len(shellArgs) == 0 {
		return errNoShellCommand
	}

	names, _ := applyStatusFilter(ctx, cmd, scope)
	if len(names) == 0 {
		return nil
	}

	return runShell(ctx, &scope.cfg, names, shellJoin(shellArgs), flagBool(cmd, "interactive"))
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

func addLsFlags(fs *pflag.FlagSet) {
	fs.BoolP("message", "m", false, "show commit message and time")
	fs.BoolP("names", "n", false, "show repo names only, one per line")
	fs.BoolP("dirs", "d", false, "show repo dirs only, one per line")
	addStatusFilterFlags(fs)
}

func lsCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [repo|group...]",
		Short: "show status of repos",
		Args:  cobra.ArbitraryArgs,
		RunE:  lsAction(cfgPath, false),
	}
	addLsFlags(cmd.Flags())
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}

// llCmd is lsCmd with the message column always on.
func llCmd(cfgPath *string) *cobra.Command {
	cmd := lsCmd(cfgPath)
	cmd.Use = "ll [repo|group...]"
	cmd.Short = "show status of repos with commit message and time"
	cmd.RunE = lsAction(cfgPath, true)

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
	forceMessage bool,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		scope, err := loadAndResolve(cfgPath, args)
		if errors.Is(err, errNoReposMatched) {
			if len(scope.cfg.Repos) == 0 {
				ui.Warnf("no repos tracked — run \"%s repo add <path>\" to get started", cmdNameHRD)
			} else {
				ui.Warnf("no repos matched")
			}

			return nil
		}

		if err != nil {
			return err
		}

		names, statuses := applyStatusFilter(ctx, cmd, scope)
		if len(names) == 0 {
			return nil
		}

		switch {
		case flagBool(cmd, "names"):
			lsNamesOnly(names)

			return nil
		case flagBool(cmd, "dirs"):
			lsDirsOnly(scope.cfg.Repos, names)

			return nil
		}

		vcsByName := make(map[string]string, len(names))
		for _, n := range names {
			vcsByName[n] = scope.cfg.Repos[n].ActiveBackend()
		}

		message := forceMessage || flagBool(cmd, "message")

		gather := lsGatherCallback(ctx, scope.cfg.Repos, names, scope.cfg.Settings.Concurrency)
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
	cmd *cobra.Command,
	args []string,
	cfgPath *string,
	backendName string,
) error {
	scope, cmdArgs, err := loadAndSplit(cfgPath, cmd, args)
	if err != nil {
		return err
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("%w; use: %s %s [repos] -- <args>", errNoArgsFmt, cmdNameHRD, backendName)
	}

	names, _ := applyStatusFilter(ctx, cmd, scope)
	if len(names) == 0 {
		return nil
	}

	if flagBool(cmd, "interactive") {
		return dispatchInteractive(ctx, scope.cfg.Repos, names, backendName, cmdArgs)
	}

	return dispatchNonInteractive(ctx, &scope.cfg, names, backendName, cmdArgs)
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
		ui.Errf("%s", ui.FormatSummary(total, failed))

		return fmt.Errorf("%w: %d/%d", ErrReposFailed, len(failed), total)
	}

	ui.Infof("%s", ui.FormatSummary(total, failed))

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

// liveProgress renders a bottom-anchored, redrawn-in-place status line (bar
// + done/total + success/fail counts + ETA) while a dispatch or
// status-gather run is in flight. Every method is nil-safe — newLiveProgress
// returns nil when stdout isn't a terminal, so callers invoke clear/update/
// draw unconditionally without a separate TTY check at each call site, and
// a non-TTY run (piped output, CI) never draws anything, byte-for-byte
// matching pre-live-bar output.
type liveProgress struct {
	total     int
	done      int
	succeeded int
	failed    int
	start     time.Time
}

func newLiveProgress(total int) *liveProgress {
	if !ui.IsTTY() {
		return nil
	}

	return &liveProgress{total: total, start: time.Now()}
}

func (lp *liveProgress) update(failed bool) {
	if lp == nil {
		return
	}

	lp.done++

	if failed {
		lp.failed++
	} else {
		lp.succeeded++
	}
}

func (lp *liveProgress) clear() {
	if lp == nil {
		return
	}

	ui.ClearLiveLine()
}

func (lp *liveProgress) draw() {
	if lp == nil {
		return
	}

	ui.DrawLiveLine(lp.line())
}

func (lp *liveProgress) line() string {
	counts := ui.ApplyColor("green", fmt.Sprintf("✓%d", lp.succeeded))
	if lp.failed > 0 {
		counts += " " + ui.ApplyColor("red", fmt.Sprintf("✗%d", lp.failed))
	}

	suffix := ui.MutedStyle().Render(fmt.Sprintf(" [%d/%d]", lp.done, lp.total)) + " " + counts

	if eta, ok := ui.EstimateETA(lp.start, lp.done, lp.total); ok {
		suffix += ui.MutedStyle().Render("  ETA " + ui.FormatETA(eta))
	}

	barWidth := ui.ProgressBarWidth(ui.TextWidth(suffix) + 1)
	bar := ui.RenderProgressBar(float64(lp.done)/float64(lp.total), barWidth)

	return " " + bar + suffix
}

func dispatch(
	names []string,
	cmdLabel string,
	dispatch func(resultCh chan<- runner.Result),
) error {
	ui.Out(cmdLabel)

	defer ui.ProgressOSCDone()

	lp := newLiveProgress(len(names))

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

	var (
		done      int
		anyFailed bool
	)

	for res := range resultCh {
		lp.clear()
		printDispatchResult(res)

		results[resultIdx[res.RepoName]] = res

		done++
		failed := res.Err != nil || res.ExitCode != 0
		anyFailed = anyFailed || failed
		ui.ProgressOSC(done*progressPercentMax/len(names), anyFailed)

		lp.update(failed)
		lp.draw()
	}

	lp.clear()

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
	ui.Out("")
}

// printStatusHeader prints the ls/ll table header and returns the effective
// column widths used to render each subsequent row.
func printStatusHeader(names []string, vcsByName map[string]string, details bool) []int {
	widths, header := statusTableConfig(details)

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

	return eff
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

	defer ui.ProgressOSCDone()

	nameIdx := make(map[string]int, len(names))
	for i, n := range names {
		nameIdx[n] = i
	}

	eff := printStatusHeader(names, vcsByName, details)

	lp := newLiveProgress(len(names))

	resultCh := make(chan runner.StatusResult, len(names))
	go func() {
		gather(resultCh)
		close(resultCh)
	}()

	results := make([]*runner.StatusResult, len(names))
	next := 0

	var (
		done      int
		anyFailed bool
	)

	for res := range resultCh {
		idx := nameIdx[res.RepoName]
		results[idx] = &res

		done++
		failed := res.Err != nil
		anyFailed = anyFailed || failed
		ui.ProgressOSC(done*progressPercentMax/len(names), anyFailed)

		lp.update(failed)
		lp.clear()

		for next < len(names) && results[next] != nil {
			cells := statusRow(names[next], vcsByName[names[next]], results[next], details)
			ui.Out(ui.RenderRow(cells, eff))

			next++
		}

		lp.draw()
	}

	lp.clear()

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
