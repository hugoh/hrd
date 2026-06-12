// Package config handles loading, saving, and querying the hrd configuration
// file (~/.config/hrd/config.toml by default).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/pelletier/go-toml/v2"
)

var errUnknownRepo = errors.New("unknown repo")

const (
	defaultConcurrency = 8
	defaultDirPerm     = 0o750
	defaultFilePerm    = 0o600
)

// Repo represents a single tracked repository.
type Repo struct {
	// Path is the absolute path to the repository root.
	Path   string   `toml:"path"`
	Groups []string `toml:"groups"`
}

// ActiveBackend detects and returns the active VCS backend for this repo.
// Returns empty string if no VCS is detected.
func (r Repo) ActiveBackend() string {
	b, err := backend.Detect(r.Path)
	if err != nil {
		return ""
	}

	return b.Name()
}

type Group struct {
	Repos []string `toml:"repos"`
}

type Settings struct {
	// Concurrency caps the number of parallel subprocess invocations.
	Concurrency int `toml:"concurrency"`
}

// Config is the top-level config structure that maps directly to the TOML file.
type Config struct {
	Repos    map[string]Repo  `toml:"repos"`
	Groups   map[string]Group `toml:"-"` // derived from Repos[].Groups, not persisted
	Settings Settings         `toml:"settings"`
}

// defaultConfig returns a Config with sensible defaults.
func defaultConfig() Config {
	return Config{
		Repos:  make(map[string]Repo),
		Groups: make(map[string]Group),
		Settings: Settings{
			Concurrency: defaultConcurrency,
		},
	}
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "hrd", "config.toml")
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".config", "hrd", "config.toml")
}

// Load returns a default config when the file does not exist.
// The file is created on first write.
func Load(path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path) //nolint:gosec // trusted config path
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}

		return cfg, fmt.Errorf("loading config %q: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("decoding config %q: %w", path, err)
	}
	// Ensure maps are non-nil even if the TOML sections were absent.
	if cfg.Repos == nil {
		cfg.Repos = make(map[string]Repo)
	}

	if cfg.Groups == nil {
		cfg.Groups = make(map[string]Group)
	}

	if cfg.Settings.Concurrency < 1 {
		cfg.Settings.Concurrency = defaultConcurrency
	}

	cfg.rebuildGroupsCache()

	return cfg, nil
}

// Save writes cfg to path, creating parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(
		filepath.Dir(path),
		defaultDirPerm,
	); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, defaultFilePerm); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func stripGroupPrefix(name string) string {
	return strings.TrimPrefix(name, "@")
}

// ResolveScope resolves explicit CLI names/groups into repo names.
// Each name may be a repo or a group (with or without the "@" prefix);
// groups are expanded inline. Duplicates are removed, preserving the
// order of first appearance. Unknown names are an error.
func (c *Config) ResolveScope(names []string) ([]string, error) {
	if len(names) == 0 {
		return c.allRepos(), nil
	}

	out := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	add := func(repo string) {
		if _, ok := seen[repo]; !ok {
			seen[repo] = struct{}{}

			out = append(out, repo)
		}
	}

	for _, name := range names {
		if repos, ok := c.groupRepos(name); ok {
			for _, r := range repos {
				add(r)
			}

			continue
		}

		repo := stripGroupPrefix(name)
		if _, ok := c.Repos[repo]; !ok {
			return nil, fmt.Errorf("%w %q", errUnknownRepo, name)
		}

		add(repo)
	}

	return out, nil
}

// AddRepo derives the name from the directory base name unless it would
// conflict, in which case the caller should provide an explicit name.
func (c *Config) AddRepo(name string, repo Repo) {
	c.Repos[name] = repo
}

// RemoveRepo scrubs a repo from the config and all groups.
func (c *Config) RemoveRepo(name string) {
	delete(c.Repos, name)

	for gName, g := range c.Groups {
		filtered := slices.DeleteFunc(g.Repos, func(r string) bool { return r == name })
		if len(filtered) == 0 {
			delete(c.Groups, gName)
		} else {
			c.Groups[gName] = Group{Repos: filtered}
		}
	}
}

// AddRepoToGroup updates the groups cache after adding the repo.
func (c *Config) AddRepoToGroup(name, group string) {
	repo := c.Repos[name]

	if slices.Contains(repo.Groups, group) {
		return
	}

	repo.Groups = append(repo.Groups, group)
	c.Repos[name] = repo

	g, ok := c.Groups[group]
	if !ok {
		g = Group{}
	}

	g.Repos = append(g.Repos, name)
	slices.Sort(g.Repos)
	c.Groups[group] = g
}

// RemoveRepoFromGroup updates the groups cache after removing the repo.
func (c *Config) RemoveRepoFromGroup(name, group string) {
	repo := c.Repos[name]
	before := len(repo.Groups)
	repo.Groups = slices.DeleteFunc(repo.Groups, func(g string) bool { return g == group })

	if len(repo.Groups) == before {
		return
	}

	c.Repos[name] = repo

	g, ok := c.Groups[group]
	if !ok {
		return
	}

	g.Repos = slices.DeleteFunc(g.Repos, func(r string) bool { return r == name })

	if len(g.Repos) == 0 {
		delete(c.Groups, group)
	} else {
		c.Groups[group] = g
	}
}

func (c *Config) rebuildGroupsCache() {
	c.Groups = make(map[string]Group, len(c.Repos))

	for repoName, repo := range c.Repos {
		for _, tag := range repo.Groups {
			g := c.Groups[tag]
			g.Repos = append(g.Repos, repoName)
			c.Groups[tag] = g
		}
	}

	for name, group := range c.Groups {
		slices.Sort(group.Repos)
		c.Groups[name] = group
	}
}

func (c *Config) allRepos() []string {
	out := make([]string, 0, len(c.Repos))
	for name := range c.Repos {
		out = append(out, name)
	}

	slices.Sort(out)

	return out
}

func (c *Config) groupRepos(name string) ([]string, bool) {
	if g, ok := c.Groups[name]; ok {
		return g.Repos, true
	}

	stripped := stripGroupPrefix(name)
	if stripped != name {
		if g, ok := c.Groups[stripped]; ok {
			return g.Repos, true
		}
	}

	return nil, false
}
