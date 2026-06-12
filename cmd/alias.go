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
	"github.com/urfave/cli/v3"
)

// aliasCommands builds one top-level command per config alias, in name
// order. Aliases shadowing a built-in command (or one of its aliases) are
// skipped with a warning.
func aliasCommands(
	cfgPath *string,
	aliases map[string]string,
	taken map[string]bool,
	dashTail []string,
) []*cli.Command {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}

	slices.Sort(names)

	cmds := make([]*cli.Command, 0, len(names))

	for _, name := range names {
		if taken[name] {
			ui.Warnf("alias %q shadows a built-in command; ignored", name)

			continue
		}

		cmds = append(cmds, aliasCmd(cfgPath, name, aliases[name], dashTail))
	}

	return cmds
}

func aliasCmd(cfgPath *string, name, expansion string, dashTail []string) *cli.Command {
	return &cli.Command{
		Name:          name,
		Usage:         "alias for: " + expansion,
		ArgsUsage:     "[repo|group...] [-- <extra args>]",
		Category:      "aliases",
		Flags:         dispatchFlags,
		ShellComplete: repoGroupCompleter(cfgPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runAlias(ctx, cmd, cfgPath, expansion, dashTail)
		},
	}
}

// runAlias resolves the scope like any dispatch command, then routes the
// alias expansion per the unified command grammar (see internal/cmdspec).
// Extra positional args (typically after "--") are appended to the
// expanded command.
func runAlias(
	ctx context.Context,
	cmd *cli.Command,
	cfgPath *string,
	expansion string,
	dashTail []string,
) error {
	cfg, names, extraArgs, err := loadAndSplit(cfgPath, cmd, dashTail)
	if err != nil {
		return err
	}

	names, _ = applyStatusFilter(ctx, cmd, &cfg, names)
	if len(names) == 0 {
		return nil
	}

	interactive := cmd.Bool("interactive")

	prefix, rest := cmdspec.Parse(expansion)
	if prefix == cmdspec.PrefixShell {
		sh := rest
		if len(extraArgs) > 0 {
			sh += " " + shellJoin(extraArgs)
		}

		return runShell(ctx, &cfg, names, sh, interactive)
	}

	tokens, err := shlex.Split(rest)
	if err != nil || len(tokens) == 0 {
		return fmt.Errorf("alias %q: %w", expansion, errNoArgsFmt)
	}

	if prefix != "" {
		args := slices.Concat(tokens, extraArgs)
		if interactive {
			return dispatchInteractive(ctx, cfg.Repos, names, prefix, args)
		}

		return dispatchNonInteractive(ctx, &cfg, names, prefix, args)
	}

	return runAliasSubcmd(
		ctx,
		&cfg,
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
