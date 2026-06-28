package cmd

import (
	"context"
	"fmt"
	"io/fs"
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
)

func repoScanCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameScan,
		Usage:     "discover and add repositories under one or more directories",
		ArgsUsage: "<dir>...",
		Flags: []cli.Flag{
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
				Name:  "dry-run",
				Usage: "print what would be added without saving",
			},
		},
		Action: repoScanAction(cfgPath),
	}
}

func repoScanAction(cfgPath *string) func(context.Context, *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		if cmd.NArg() == 0 {
			return errAtLeastOnePath
		}

		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("repo scan: %w", err)
		}

		dryRun := cmd.Bool("dry-run")
		group := stripGroupPrefix(cmd.String(cmdNameGroup))
		tracked := trackedPaths(&cfg)
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

			added += addScanned(&cfg, tracked, repoPaths, group, dryRun)
		}

		if added == 0 {
			ui.Warnf("no new repos found")

			return nil
		}

		if dryRun {
			ui.Outf("would add %d repo(s); re-run without --dry-run to save", added)

			return nil
		}

		return config.Save(*cfgPath, cfg)
	}
}

// addScanned registers each discovered path in cfg (unless dryRun),
// returning how many were added. Already-tracked paths are skipped
// silently; unresolvable name conflicts are skipped with a warning.
func addScanned(
	cfg *config.Config,
	tracked map[string]string,
	repoPaths []string,
	group string,
	dryRun bool,
) int {
	added := 0

	for _, path := range repoPaths {
		if name, ok := tracked[path]; ok {
			ui.Outf("%s already tracked as %q", path, name)

			continue
		}

		name, ok := scanRepoName(cfg, path)
		if !ok {
			ui.Warnf("skipping %s: names %q and %q already taken", path, filepath.Base(path), name)

			continue
		}

		added++

		if dryRun {
			ui.Outf("would add %s as %q", path, name)

			continue
		}

		cfg.AddRepo(name, config.Repo{Path: path})
		tracked[path] = name

		if group != "" {
			cfg.AddRepoToGroup(name, group)
		}

		if group != "" {
				ui.Success("added %s as %q in group %s", path, name, group)
			} else {
				ui.Success("added %s as %q", path, name)
			}
	}

	return added
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
