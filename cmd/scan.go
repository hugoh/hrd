package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
)

const (
	defaultScanDepth = 5
	cmdNameScan      = "scan"
	cmdNameScanAdd   = "add"
	cmdNameScanList  = "list"
)

//nolint:gochecknoglobals // package-level variable required for test injection
var confirmFn func(string) bool = ui.Confirm

func repoScanCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameScan,
		Short: "discover repositories under one or more directories",
	}
	cmd.AddCommand(
		repoScanAddCmd(cfgPath),
		repoScanListCmd(cfgPath),
	)

	return cmd
}

func repoScanAddCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameScanAdd + " <dir>...",
		Short: "discover and add repositories under one or more directories",
		Long: `Walks each given directory (up to --depth levels deep) looking for git and
jj repositories, and adds every match to the config under its directory name.
Use --pattern to filter by name and -g/--group to assign found repos to a
group. -i/--confirm prompts before adding each repo instead of adding all
matches unconditionally.`,
		Example: `  hrd repo scan add ~/code
  hrd repo scan add ~/code --depth 2 --pattern 'api-*' -g backend
  hrd repo scan add ~/code -i`,
		Args: cobra.ArbitraryArgs,
		RunE: repoScanAddAction(cfgPath),
	}
	cmd.Flags().
		StringP("pattern", "p", "", "glob pattern matched against repo directory name to filter results")
	cmd.Flags().StringP(cmdNameGroup, "g", "", "add found repos to this group")
	cmd.Flags().Int("depth", defaultScanDepth, "maximum directory depth to descend")
	cmd.Flags().BoolP("confirm", "i", false, "prompt before adding each repo")

	return cmd
}

func repoScanListCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameScanList + " <dir>...",
		Short: "list repositories discovered under one or more directories",
		Long: `Walks each given directory (up to --depth levels deep) looking for git and
jj repositories and prints what it finds, without modifying the config. Use
--tracked/--untracked to filter by whether a match is already in the config,
and -g/--group to preview what a subsequent "repo scan add -g" would assign.`,
		Example: `  hrd repo scan list ~/code
  hrd repo scan list ~/code --untracked
  hrd repo scan list ~/code --depth 2 --pattern 'api-*'`,
		Args: cobra.ArbitraryArgs,
		RunE: repoScanListAction(cfgPath),
	}
	cmd.Flags().
		StringP("pattern", "p", "", "glob pattern matched against repo directory name to filter results")
	cmd.Flags().Int("depth", defaultScanDepth, "maximum directory depth to descend")
	cmd.Flags().StringP(cmdNameGroup, "g", "", "assign tracked repos in results to this group")
	cmd.Flags().Bool("tracked", false, "show only repos already in config")
	cmd.Flags().Bool("untracked", false, "show only repos not yet in config")

	return cmd
}

func loadScanConfig(cfgPath *string, args []string, op string) (config.Config, error) {
	if len(args) == 0 {
		return config.Config{}, errAtLeastOnePath
	}

	cfg, err := loadConfig(cfgPath, "repo scan "+op)
	if err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func collectRepoPaths(roots []string, depth int) ([]string, error) {
	var all []string

	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolving %q: %w", root, err)
		}

		paths, err := scanForRepos(abs, depth)
		if err != nil {
			return nil, fmt.Errorf("scanning %q: %w", root, err)
		}

		all = append(all, paths...)
	}

	return all, nil
}

func repoScanAddAction(cfgPath *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := loadScanConfig(cfgPath, args, "add")
		if err != nil {
			return err
		}

		group := stripGroupPrefix(flagString(cmd, cmdNameGroup))
		tracked := trackedPaths(&cfg)
		pattern := flagString(cmd, "pattern")
		confirm := flagBool(cmd, "confirm")

		repoPaths, err := collectRepoPaths(args, flagInt(cmd, "depth"))
		if err != nil {
			return err
		}

		filtered, err := filterByPattern(repoPaths, pattern)
		if err != nil {
			return err
		}

		added := addScanned(&cfg, tracked, filtered, group, confirm)

		if added == 0 {
			ui.Warnf("no new repos found")

			return nil
		}

		return config.Save(*cfgPath, cfg)
	}
}

func repoScanListAction(cfgPath *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := loadScanConfig(cfgPath, args, "list")
		if err != nil {
			return err
		}

		tracked := trackedPaths(&cfg)
		pattern := flagString(cmd, "pattern")
		group := stripGroupPrefix(flagString(cmd, cmdNameGroup))
		onlyTracked := flagBool(cmd, "tracked")
		onlyUntracked := flagBool(cmd, "untracked")

		allPaths, err := collectRepoPaths(args, flagInt(cmd, "depth"))
		if err != nil {
			return err
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

		return groupTracked(&cfg, *cfgPath, filtered, tracked, group)
	}
}

// groupTracked assigns all tracked paths in filtered to group and saves config.
// A no-op when group is empty or no tracked paths are found.
func groupTracked(
	cfg *config.Config,
	cfgPath string,
	filtered []string,
	tracked map[string]string,
	group string,
) error {
	if group == "" {
		return nil
	}

	grouped := 0

	for _, path := range filtered {
		name, isTracked := tracked[path]
		if !isTracked {
			continue
		}

		cfg.AddRepoToGroup(name, group)

		grouped++
	}

	if grouped == 0 {
		return nil
	}

	ui.Outf("assigned %d repo(s) to group %s", grouped, displayGroup(group))

	return config.Save(cfgPath, *cfg) //nolint:wrapcheck // caller provides context
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
