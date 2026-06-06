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

// Group is a named collection of repo names.
type Group struct {
	Repos []string `toml:"repos"`
}

// Settings holds global tunables.
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

// DefaultPath returns the platform config path for the hrd config file.
func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "hrd", "config.toml")
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".config", "hrd", "config.toml")
}

// Load reads the config file at path. If the file does not exist a default
// config is returned without error — the file is created on first write.
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

	if cfg.Settings.Concurrency == 0 {
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

// ResolveScope returns the list of repo names to operate on given an explicit
// set of names/group passed on the CLI.
func (c *Config) ResolveScope(names []string) ([]string, error) {
	if len(names) == 0 {
		return c.allRepos()
	}

	if len(names) == 1 {
		if repos, ok := c.groupRepos(names[0]); ok {
			return repos, nil
		}
	}

	return c.ValidatedRepos(names)
}

// AddRepo adds or updates a repo entry. The name is derived from the
// directory base name unless it would conflict, in which case the caller
// should provide an explicit name.
func (c *Config) AddRepo(name string, repo Repo) {
	c.Repos[name] = repo
}

// RemoveRepo removes a repo and scrubs it from all groups.
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

// AddRepoToGroup adds a repo to a group and updates the groups cache.
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

// RemoveRepoFromGroup removes a repo from a group and updates the groups cache.
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

// ValidatedRepos checks that all names exist in the config and returns them.
func (c *Config) ValidatedRepos(names []string) ([]string, error) {
	for _, name := range names {
		lookupName := stripGroupPrefix(name)
		if _, ok := c.Repos[lookupName]; !ok {
			return nil, fmt.Errorf("%w %q", errUnknownRepo, name)
		}
	}

	return names, nil
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

func (c *Config) allRepos() ([]string, error) {
	out := make([]string, 0, len(c.Repos))
	for name := range c.Repos {
		out = append(out, name)
	}

	slices.Sort(out)

	return out, nil
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
