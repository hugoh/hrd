package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/hugoh/hrd/internal/cmdspec"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
)

// aliasCommands builds one top-level command per config alias, in name
// order. Aliases shadowing a built-in command (or one of its aliases) are
// skipped with a warning.
func aliasCommands(
	cfgPath *string,
	aliases map[string]config.AliasSpec,
	taken map[string]bool,
) []*cobra.Command {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}

	slices.Sort(names)

	cmds := make([]*cobra.Command, 0, len(names))

	for _, name := range names {
		if taken[name] {
			ui.Warnf("alias %q shadows a built-in command; ignored", name)

			continue
		}

		cmds = append(cmds, aliasCmd(cfgPath, name, aliases[name]))
	}

	return cmds
}

func aliasCmd(cfgPath *string, name string, spec config.AliasSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:     name + " [repo|group...] [-- <extra args>]",
		Short:   "alias for: " + aliasSummary(spec),
		Long:    aliasHelp(spec),
		GroupID: "aliases",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAlias(cmd, args, cfgPath, spec)
		},
	}
	addDispatchFlags(cmd.Flags())
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}

// aliasSummary renders a one-line description of an alias's expansion(s)
// for cobra's Short help text: the plain command, or "git: ...; jj: ..."
// for a per-backend alias.
func aliasSummary(spec config.AliasSpec) string {
	if spec.Command != "" {
		return spec.Command
	}

	backendNames := make([]string, 0, len(spec.Backends))
	for name := range spec.Backends {
		backendNames = append(backendNames, name)
	}

	slices.Sort(backendNames)

	parts := make([]string, 0, len(backendNames))
	for _, name := range backendNames {
		parts = append(parts, name+": "+spec.Backends[name])
	}

	return strings.Join(parts, "; ")
}

func aliasHelp(spec config.AliasSpec) string {
	return fmt.Sprintf(`Alias defined in config, expanding to: %s

The expansion is routed the same way as in the TUI: a leading "!" or "sh "/
"shell " runs it via the shell; a leading backend name (e.g. "git ") runs it
through that backend directly; otherwise it's treated as a VCS subcommand
resolved per repo's active backend (like "hrd status"). Extra args after
"--" are appended to the expansion. When the alias defines per-backend
variants, each repo in the selection runs the variant matching its own
active backend; repos whose backend has no variant are skipped with a
warning.`, aliasSummary(spec))
}

// runAlias resolves the scope like any dispatch command, then routes the
// alias expansion(s) per the unified command grammar (see internal/cmdspec).
// Extra positional args (typically after "--") are appended to the
// expanded command.
func runAlias(
	cmd *cobra.Command,
	args []string,
	cfgPath *string,
	spec config.AliasSpec,
) error {
	ctx := cmd.Context()

	scope, extraArgs, err := loadAndSplit(cfgPath, cmd, args)
	if err != nil {
		return err
	}

	names, _ := applyStatusFilter(ctx, cmd, scope)
	if len(names) == 0 {
		return nil
	}

	interactive := flagBool(cmd, "interactive")

	if spec.Command != "" {
		return runAliasExpansion(ctx, &scope.cfg, names, extraArgs, spec.Command, interactive)
	}

	return runAliasPerBackend(ctx, &scope.cfg, names, extraArgs, spec.Backends, interactive)
}

// runAliasPerBackend groups names by their repo's active backend and runs
// each group against that backend's variant of the alias, one dispatch per
// backend. A repo's active backend is the single one backend.Detect would
// pick (jj wins over git for colocated repos) — not "any backend whose
// marker is present", which filterMatching checks and would run both
// variants against a colocated jj repo. Repos whose backend has no defined
// variant are skipped with a warning rather than failing the whole run.
func runAliasPerBackend(
	ctx context.Context,
	cfg *config.Config,
	names []string,
	extraArgs []string,
	variants map[string]string,
	interactive bool,
) error {
	groups := make(map[string][]string, len(variants))
	for _, n := range names {
		if repo, ok := cfg.Repos[n]; ok {
			active := repo.ActiveBackend()
			groups[active] = append(groups[active], n)
		}
	}

	backendNames := make([]string, 0, len(variants))
	for name := range variants {
		backendNames = append(backendNames, name)
	}

	slices.Sort(backendNames)

	matched := make(map[string]bool, len(names))

	var errs []error

	for _, backendName := range backendNames {
		group := groups[backendName]
		if len(group) == 0 {
			continue
		}

		for _, n := range group {
			matched[n] = true
		}

		if err := runAliasExpansion(
			ctx,
			cfg,
			group,
			extraArgs,
			variants[backendName],
			interactive,
		); err != nil {
			errs = append(errs, err)
		}
	}

	for _, n := range names {
		if !matched[n] {
			ui.Warnf("alias: no command defined for %q's backend; skipped", n)
		}
	}

	if len(matched) == 0 {
		return fmt.Errorf(
			"alias: %w (defined for: %s)",
			errNoReposWithBackend,
			strings.Join(backendNames, ", "),
		)
	}

	return errors.Join(errs...)
}

// runAliasExpansion parses a single, already backend-resolved alias
// expansion and dispatches it across names.
func runAliasExpansion(
	ctx context.Context,
	cfg *config.Config,
	names []string,
	extraArgs []string,
	expansion string,
	interactive bool,
) error {
	prefix, rest := cmdspec.Parse(expansion)
	if prefix == cmdspec.PrefixShell {
		sh := rest
		if len(extraArgs) > 0 {
			sh += " " + shellJoin(extraArgs)
		}

		return runShell(ctx, cfg, names, sh, interactive)
	}

	tokens, err := shlex.Split(rest)
	if err != nil {
		return fmt.Errorf("alias %q: invalid syntax: %w", expansion, err)
	}

	if len(tokens) == 0 {
		return fmt.Errorf("alias %q: %w", expansion, errNoArgsFmt)
	}

	if prefix != "" {
		args := slices.Concat(tokens, extraArgs)
		if interactive {
			return dispatchInteractive(ctx, cfg.Repos, names, prefix, args)
		}

		return dispatchNonInteractive(ctx, cfg, names, prefix, args)
	}

	return runAliasSubcmd(
		ctx,
		cfg,
		names,
		tokens[0],
		slices.Concat(tokens[1:], extraArgs),
		interactive,
	)
}

// runAliasSubcmd runs a bare alias expansion ("pull --rebase") through
// per-repo backend routing, like the built-in VCS subcommands.
func runAliasSubcmd(
	ctx context.Context,
	cfg *config.Config,
	names []string,
	op string,
	args []string,
	interactive bool,
) error {
	if interactive {
		return runSubcmdInteractive(ctx, cfg.Repos, names, op, args)
	}

	label := op
	if len(args) > 0 {
		label += " " + shellJoin(args)
	}

	return dispatch(names, label, func(resultCh chan<- runner.Result) {
		ch := runner.VCSSubcmd(ctx, cfg.Repos, names, op, args, cfg.Settings.Concurrency)
		for res := range ch {
			resultCh <- res
		}
	})
}
