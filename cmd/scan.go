package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

const (
	defaultScanDepth = 5
	cmdNameScan      = "scan"
	cmdNameScanAdd   = "add"
	cmdNameScanList  = "list"
)

//nolint:gochecknoglobals // package-level variable required for test injection
var confirmFn func(string) bool = ui.Confirm

func repoScanCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameScan,
		Usage: "discover repositories under one or more directories",
		Commands: []*cli.Command{
			repoScanAddCmd(cfgPath),
			repoScanListCmd(cfgPath),
		},
	}
}

func repoScanAddCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameScanAdd,
		Usage:     "discover and add repositories under one or more directories",
		ArgsUsage: "<dir>...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "pattern",
				Aliases: []string{"p"},
				Usage:   "glob pattern matched against repo directory name to filter results",
			},
			&cli.StringFlag{
				Name:    cmdNameGroup,
				Aliases: []string{"g"},
				Usage:   "add found repos to this group",
			},
			&cli.IntFlag{
				Name:  "depth",
				Value: defaultScanDepth,
				Usage: "maximum directory depth to descend",
			},
			&cli.BoolFlag{
				Name:    "confirm",
				Aliases: []string{"i"},
				Usage:   "prompt before adding each repo",
			},
		},
		Action: repoScanAddAction(cfgPath),
	}
}

func repoScanListCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameScanList,
		Usage:     "list repositories discovered under one or more directories",
		ArgsUsage: "<dir>...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "pattern",
				Aliases: []string{"p"},
				Usage:   "glob pattern matched against repo directory name to filter results",
			},
			&cli.IntFlag{
				Name:  "depth",
				Value: defaultScanDepth,
				Usage: "maximum directory depth to descend",
			},
			&cli.BoolFlag{
				Name:  "tracked",
				Usage: "show only repos already in config",
			},
			&cli.BoolFlag{
				Name:  "untracked",
				Usage: "show only repos not yet in config",
			},
		},
		Action: repoScanListAction(cfgPath),
	}
}

func repoScanAddAction(cfgPath *string) func(context.Context, *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		if cmd.NArg() == 0 {
			return errAtLeastOnePath
		}

		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("repo scan add: %w", err)
		}

		group := stripGroupPrefix(cmd.String(cmdNameGroup))
		tracked := trackedPaths(&cfg)
		pattern := cmd.String("pattern")
		confirm := cmd.Bool("confirm")
		added := 0

		for _, root := range cmd.Args().Slice() {
			abs, err := filepath.Abs(root)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", root, err)
			}

			repoPaths, err := scanForRepos(abs, cmd.Int("depth"))
			if err != nil {
				return fmt.Errorf("scanning %q: %w", root, err)
			}

			filtered, err := filterByPattern(repoPaths, pattern)
			if err != nil {
				return err
			}

			added += addScanned(&cfg, tracked, filtered, group, confirm)
		}

		if added == 0 {
			ui.Warnf("no new repos found")

			return nil
		}

		return config.Save(*cfgPath, cfg)
	}
}

func repoScanListAction(cfgPath *string) func(context.Context, *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		if cmd.NArg() == 0 {
			return errAtLeastOnePath
		}

		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("repo scan list: %w", err)
		}

		tracked := trackedPaths(&cfg)
		pattern := cmd.String("pattern")
		onlyTracked := cmd.Bool("tracked")
		onlyUntracked := cmd.Bool("untracked")

		var allPaths []string

		for _, root := range cmd.Args().Slice() {
			abs, err := filepath.Abs(root)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", root, err)
			}

			repoPaths, err := scanForRepos(abs, cmd.Int("depth"))
			if err != nil {
				return fmt.Errorf("scanning %q: %w", root, err)
			}

			allPaths = append(allPaths, repoPaths...)
		}

		filtered, err := filterByPattern(allPaths, pattern)
		if err != nil {
			return err
		}

		const nameWidth, statusWidth, gap = 30, 10, 2

		pathWidth := ui.GetTermWidth() - nameWidth - statusWidth - gap

		header := []string{"NAME", "PATH", "STATUS"}
		maxWidths := []int{nameWidth, pathWidth, statusWidth}

		rows := buildScanListRows(filtered, tracked, &cfg, onlyTracked, onlyUntracked)

		if len(rows) == 0 {
			ui.Warnf("no repos found")

			return nil
		}

		_, _ = fmt.Fprint(os.Stdout, ui.RenderTable(
			header, rows, ui.EffectiveWidths(header, rows, maxWidths),
		))

		return nil
	}
}

// buildScanListRows classifies discovered paths as tracked or untracked and
// returns table rows filtered by the caller's onlyTracked/onlyUntracked flags.
func buildScanListRows(
	paths []string,
	tracked map[string]string,
	cfg *config.Config,
	onlyTracked, onlyUntracked bool,
) [][]string {
	showAll := !onlyTracked && !onlyUntracked

	var rows [][]string

	for _, path := range paths {
		trackedName, isTracked := tracked[path]

		name := resolveDisplayName(cfg, path, trackedName, isTracked)

		status := "tracked"
		if !isTracked {
			status = "untracked"
		}

		if showAll || (onlyTracked && isTracked) || (onlyUntracked && !isTracked) {
			rows = append(rows, []string{name, path, status})
		}
	}

	return rows
}

// resolveDisplayName returns the config name for a tracked path, or the
// would-be auto-generated name for an untracked one.
func resolveDisplayName(cfg *config.Config, path, trackedName string, isTracked bool) string {
	if isTracked {
		return trackedName
	}

	if name, ok := scanRepoName(cfg, path); ok {
		return name
	}

	return filepath.Base(path) + " (!)"
}

// addScanned registers each discovered path in cfg, returning how many were
// added. Already-tracked paths are skipped silently; unresolvable name
// conflicts are skipped with a warning. When confirm is true the user is
// prompted before each addition.
func addScanned(
	cfg *config.Config,
	tracked map[string]string,
	repoPaths []string,
	group string,
	confirm bool,
) int {
	added := 0

	for _, path := range repoPaths {
		if _, ok := tracked[path]; ok {
			continue
		}

		name, ok := scanRepoName(cfg, path)
		if !ok {
			ui.Warnf("skipping %s: names %q and %q already taken", path, filepath.Base(path), name)

			continue
		}

		if confirm && !promptYN(fmt.Sprintf("Add %s as %q?", path, name)) {
			continue
		}

		cfg.AddRepo(name, config.Repo{Path: path})
		tracked[path] = name

		if group != "" {
			cfg.AddRepoToGroup(name, group)
			ui.Success("added %s as %q in group %s", path, name, group)
		} else {
			ui.Success("added %s as %q", path, name)
		}

		added++
	}

	return added
}

// promptYN asks the user a yes/no question via confirmFn.
func promptYN(prompt string) bool {
	return confirmFn(prompt)
}

// filterByPattern returns paths whose base name matches the given glob
// pattern. Returns all paths unchanged when pattern is empty.
func filterByPattern(paths []string, pattern string) ([]string, error) {
	if pattern == "" {
		return paths, nil
	}

	var out []string

	for _, p := range paths {
		matched, err := filepath.Match(pattern, filepath.Base(p))
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}

		if matched {
			out = append(out, p)
		}
	}

	return out, nil
}

// scanForRepos walks root up to maxDepth levels deep and returns the
// directories managed by a known VCS. Detected repos are not descended
// into (nested checkouts like vendored deps stay untracked), and hidden
// directories are skipped. Unreadable directories are warned about and
// skipped rather than aborting the scan.
func scanForRepos(root string, maxDepth int) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			ui.Warnf("scan: %v", err)

			return nil
		}

		if !d.IsDir() {
			return nil
		}

		if path != root && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}

		if scanDepth(root, path) > maxDepth {
			return fs.SkipDir
		}

		if _, derr := backend.Detect(path); derr == nil {
			found = append(found, path)

			return fs.SkipDir
		}

		return nil
	})
	if err != nil { // coverage-ignore — WalkDir errors are handled in the callback
		return nil, fmt.Errorf("walking %q: %w", root, err)
	}

	return found, nil
}

// scanDepth returns how many levels below root the path sits (root = 0).
func scanDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}

	return strings.Count(rel, string(filepath.Separator)) + 1
}

// scanRepoName picks a config name for a discovered repo: the directory
// base name, falling back to "<parent>-<base>" on conflict. Returns false
// when both are taken; the second return is the attempted fallback name.
func scanRepoName(cfg *config.Config, path string) (string, bool) {
	base := filepath.Base(path)
	if _, exists := cfg.Repos[base]; !exists {
		return base, true
	}

	alt := filepath.Base(filepath.Dir(path)) + "-" + base
	if _, exists := cfg.Repos[alt]; !exists {
		return alt, true
	}

	return alt, false
}

// trackedPaths indexes the config by repo path for already-tracked checks.
func trackedPaths(cfg *config.Config) map[string]string {
	paths := make(map[string]string, len(cfg.Repos))
	for name, repo := range cfg.Repos {
		paths[repo.Path] = name
	}

	return paths
}
