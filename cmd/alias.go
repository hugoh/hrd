package cmd

import (
	"context"
	"fmt"
	"slices"

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
	aliases map[string]string,
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

func aliasCmd(cfgPath *string, name, expansion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " [repo|group...] [-- <extra args>]",
		Short: "alias for: " + expansion,
		Long: fmt.Sprintf(`Alias defined in config, expanding to: %s

The expansion is routed the same way as in the TUI: a leading "!" or "sh "/
"shell " runs it via the shell; a leading backend name (e.g. "git ") runs it
through that backend directly; otherwise it's treated as a VCS subcommand
resolved per repo's active backend (like "hrd status"). Extra args after
"--" are appended to the expansion.`, expansion),
		GroupID: "aliases",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAlias(cmd, args, cfgPath, expansion)
		},
	}
	addDispatchFlags(cmd.Flags())
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}

// runAlias resolves the scope like any dispatch command, then routes the
// alias expansion per the unified command grammar (see internal/cmdspec).
// Extra positional args (typically after "--") are appended to the
// expanded command.
func runAlias(
	cmd *cobra.Command,
	args []string,
	cfgPath *string,
	expansion string,
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

	prefix, rest := cmdspec.Parse(expansion)
	if prefix == cmdspec.PrefixShell {
		sh := rest
		if len(extraArgs) > 0 {
			sh += " " + shellJoin(extraArgs)
		}

		return runShell(ctx, &scope.cfg, names, sh, interactive)
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
			return dispatchInteractive(ctx, scope.cfg.Repos, names, prefix, args)
		}

		return dispatchNonInteractive(ctx, &scope.cfg, names, prefix, args)
	}

	return runAliasSubcmd(
		ctx,
		&scope.cfg,
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
