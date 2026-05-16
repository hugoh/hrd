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

	"github.com/BurntSushi/toml"
)

var (
	errActiveContextNotFound = errors.New("active context group not found")
	errUnknownRepo           = errors.New("unknown repo")
)

const (
	defaultConcurrency = 8
	defaultDirPerm     = 0o750
)

// Repo represents a single tracked repository.
type Repo struct {
	// Path is the absolute path to the repository root.
	Path string `toml:"path"`

	// Backends holds all detected VCS backends at this path.
	// The first element is the active backend.
	Backends []string `toml:"backends"`
}

// ActiveBackend returns the first configured backend for this repo.
// If no backends are configured, returns empty string.
func (r Repo) ActiveBackend() string {
	if len(r.Backends) > 0 {
		return r.Backends[0]
	}

	return ""
}

// Group is a named collection of repo names.
type Group struct {
	Repos []string `toml:"repos"`
}

// Context holds the active scope used when no repos/group is specified.
type Context struct {
	// Current is a group name, or empty to mean all repos.
	Current string `toml:"current,omitempty"`
}

// Settings holds global tunables.
type Settings struct {
	// Concurrency caps the number of parallel subprocess invocations.
	Concurrency int `toml:"concurrency"`
}

// Config is the top-level config structure that maps directly to the TOML file.
type Config struct {
	Repos    map[string]Repo  `toml:"repos"`
	Groups   map[string]Group `toml:"groups"`
	Context  Context          `toml:"context"`
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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %q: %w", path, err)
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

	return cfg, nil
}

// Save writes cfg to path, creating parent directories as needed.
func Save(path string, cfg Config) (err error) {
	if err := os.MkdirAll(
		filepath.Dir(path),
		defaultDirPerm,
	); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	file, err := os.Create(path) //nolint:gosec // trusted config path
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}()

	enc := toml.NewEncoder(file)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
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

	return c.validatedRepos(names)
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

	for gname, g := range c.Groups {
		filtered := slices.DeleteFunc(g.Repos, func(r string) bool { return r == name })
		c.Groups[gname] = Group{Repos: filtered}
	}
}

// AddGroup creates or replaces a group.
func (c *Config) AddGroup(name string, repos []string) error {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(repos))

	if existing, ok := c.Groups[name]; ok {
		for _, repo := range existing.Repos {
			seen[repo] = true
			unique = append(unique, repo)
		}
	}

	for _, repo := range repos {
		if seen[repo] {
			continue
		}

		if _, ok := c.Repos[repo]; !ok {
			return fmt.Errorf("%w %q", errUnknownRepo, repo)
		}

		seen[repo] = true
		unique = append(unique, repo)
	}

	c.Groups[name] = Group{Repos: unique}

	return nil
}

func (c *Config) allRepos() ([]string, error) {
	if c.Context.Current != "" {
		g, ok := c.Groups[c.Context.Current]
		if !ok {
			return nil, fmt.Errorf("%w %q", errActiveContextNotFound, c.Context.Current)
		}

		return g.Repos, nil
	}

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

func (c *Config) validatedRepos(names []string) ([]string, error) {
	for _, name := range names {
		lookupName := stripGroupPrefix(name)
		if _, ok := c.Repos[lookupName]; !ok {
			return nil, fmt.Errorf("%w %q", errUnknownRepo, name)
		}
	}

	return names, nil
}
