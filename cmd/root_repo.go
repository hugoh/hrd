package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
)

const (
	cmdNameRoot      = "root"
	defaultRootDepth = 1
)

var (
	errUnknownRoot        = errors.New("unknown root")
	errRootExists         = errors.New("root already exists")
	errAtLeastOneRootName = errors.New("at least one root name required")
)

// repoRootCmd returns the `repo root` subcommand with its children. Roots
// are directories walked live on every invocation for repos to track,
// instead of being materialized into individual repo entries — see
// `repo scan add` for the one-shot alternative.
func repoRootCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameRoot,
		Short: "manage live-resolved directory roots",
	}
	cmd.AddCommand(
		repoRootAddCmd(cfgPath),
		repoRootRemoveCmd(cfgPath),
		repoRootListCmd(cfgPath),
	)

	return cmd
}

func repoRootAddCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameAdd + " <dir>...",
		Short: "track one or more directories to walk for repos on every invocation",
		Long: `Registers each dir as a live-resolved root: every hrd invocation walks it
(up to --depth levels deep, default 1) looking for git and jj repositories
and merges what it finds into the working repo set. Nothing is written to
config.toml for the discovered repos themselves — only the root is
persisted, so new repos added under dir appear automatically.`,
		Example: `  hrd repo root add ~/my-repos
  hrd repo root add ~/my-repos --depth 2 -g personal
  hrd repo root add ~/work-repos ~/oss-repos`,
		Args: cobra.ArbitraryArgs,
		RunE: repoRootAddAction(cfgPath),
	}
	cmd.Flags().StringP("name", "n", "", "explicit name (only valid when adding a single root)")
	cmd.Flags().
		StringP(cmdNameGroup, "g", "", "assign repos discovered under these roots to a group")
	cmd.Flags().
		Int("depth", defaultRootDepth, "maximum directory depth to walk on every invocation")

	return cmd
}

func repoRootAddAction(cfgPath *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errAtLeastOnePath
		}

		flags, cfg, err := prepareAdd(cfgPath, cmd, args, "repo root add")
		if err != nil {
			return err
		}

		depth := flagInt(cmd, "depth")

		for _, dir := range args {
			if err := addRoot(&cfg, dir, flags.name, flags.group, depth); err != nil {
				return err
			}
		}

		return config.Save(*cfgPath, cfg)
	}
}

// addRoot validates and registers a single directory root in cfg. An empty
// explicitName derives the name from the directory base name.
func addRoot(cfg *config.Config, dir, explicitName, group string, depth int) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", dir, err)
	}

	name := explicitName
	if name == "" {
		name = filepath.Base(abs)
	}

	if _, exists := cfg.Roots[name]; exists {
		return fmt.Errorf(
			"%w %q (path: %s). use --name/-n to specify a unique name",
			errRootExists,
			name,
			cfg.Roots[name].Path,
		)
	}

	var groups []string
	if group != "" {
		groups = []string{group}
	}

	cfg.AddRoot(name, config.Root{
		Path:   abs,
		Depth:  depth,
		Groups: groups,
	})

	ui.Infof("tracking root %s as %q", abs, name)

	return nil
}

func repoRootRemoveCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>...",
		Short: "stop tracking one or more directory roots",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errAtLeastOneRootName
			}

			cfg, err := loadConfig(cfgPath, "repo root rm")
			if err != nil {
				return err
			}

			for _, name := range args {
				if _, ok := cfg.Roots[name]; !ok {
					return fmt.Errorf("%w %q", errUnknownRoot, name)
				}

				cfg.RemoveRoot(name)
				ui.Infof("removed root %q", name)
			}

			return config.Save(*cfgPath, cfg)
		},
	}
}

func repoRootListCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "list tracked directory roots",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cfgPath, "repo root ls")
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Roots))
			for name := range cfg.Roots {
				names = append(names, name)
			}

			slices.Sort(names)

			if len(names) == 0 {
				return nil
			}

			const nameWidth, depthWidth, gap = 20, 6, 2

			pathWidth := ui.GetTermWidth() - nameWidth - depthWidth - gap

			rows := make([][]string, 0, len(names))
			for _, name := range names {
				root := cfg.Roots[name]
				rows = append(rows, []string{name, root.Path, strconv.Itoa(root.Depth)})
			}

			header := []string{NameLabel, PathLabel, "DEPTH"}
			widths := []int{nameWidth, pathWidth, depthWidth}

			_, _ = fmt.Fprint(os.Stdout, ui.RenderTable(
				header, rows, ui.EffectiveWidths(header, rows, widths),
			))

			return nil
		},
	}
}
