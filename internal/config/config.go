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

	"github.com/hugoh/hrd/internal/atomicfile"
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

// Root represents a directory that is walked live on every invocation for
// repos to track, rather than being materialized into individual Repos
// entries.
type Root struct {
	// Path is the absolute path to the directory to walk.
	Path   string   `toml:"path"`
	Depth  int      `toml:"depth"`
	Groups []string `toml:"groups"`
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
	Repos  map[string]Repo  `toml:"repos"`
	Roots  map[string]Root  `toml:"roots"`
	Groups map[string]Group `toml:"-"` // derived from Repos[].Groups, not persisted

	// Aliases maps user-defined command names to their expansion in the
	// unified command grammar (see internal/cmdspec): "pull --rebase"
	// routes per-repo, "git ..."/"jj ..." to that backend, "!..." or
	// "sh ..." to the shell.
	Aliases map[string]string `toml:"aliases,omitempty"`

	Settings Settings `toml:"settings"`
}

// defaultConfig returns a Config with sensible defaults.
func defaultConfig() Config {
	return Config{
		Repos:  make(map[string]Repo),
		Roots:  make(map[string]Root),
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

	if cfg.Roots == nil {
		cfg.Roots = make(map[string]Root)
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

	if err := atomicfile.WriteFile(path, data, defaultFilePerm); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func stripGroupPrefix(name string) string {
	return strings.TrimPrefix(name, "@")
}

// ReservedNone is the pseudo-group matching repos with no group. Reserved
// group tokens start with "@@" (two ats), distinguishing them from real
// group names (which always use a single "@" and can never start with two,
// see ValidGroupName) so this namespace can never collide with a
// user-created group.
const ReservedNone = "@@none"

// ReservedAttention is the pseudo-group matching repos that need attention:
// a dirty working copy, or a branch out of sync with its remote (ahead,
// behind, diverged, or gone). Unlike ReservedNone, this can't be resolved
// from static config alone — GroupRepos expands it to every repo, and the
// actual attention filtering happens later, once live status has been
// gathered (see cmd.applyStatusFilter and backend.RepoStatus.NeedsAttention).
const ReservedAttention = "@@attention"

// ReservedGroupInfo pairs a reserved "@@" pseudo-group token with a
// one-line explanation, for CLI/TUI surfaces that list available reserved
// groups (see cmd's "hrd group" and the TUI's "@" popup).
type ReservedGroupInfo struct {
	Name string
	Desc string

	// Live is true when computing this group's repo membership requires
	// gathering live git/jj status (e.g. ReservedAttention), as opposed to
	// a free, static lookup from config alone (e.g. ReservedNone).
	Live bool
}

// ReservedGroups lists all reserved pseudo-groups in a stable, deliberate
// order (map iteration order is not used) so "hrd group ls" and the TUI
// popup show them consistently.
//
//nolint:gochecknoglobals // read-only registry, not mutated at runtime
var ReservedGroups = []ReservedGroupInfo{
	{Name: ReservedNone, Desc: "repos with no group", Live: false},
	{
		Name: ReservedAttention,
		Desc: "repos needing attention (dirty, or ahead/behind/diverged/gone vs. remote)",
		Live: true,
	},
}

const reservedGroupPrefix = "@@"

// IsReservedGroupName reports whether name is in the reserved "@@" pseudo-
// group namespace.
func IsReservedGroupName(name string) bool {
	return strings.HasPrefix(name, reservedGroupPrefix)
}

var errReservedGroupName = errors.New("group names cannot start with \"@\" (reserved)")

// ValidGroupName rejects any group name starting with "@" — real group
// names are stored bare (the CLI's leading "@" is display/input sugar,
// stripped before storage) — so the "@@" pseudo-group namespace can never
// collide with a stored group.
func ValidGroupName(name string) error {
	if strings.HasPrefix(name, "@") {
		return fmt.Errorf("%w: %q", errReservedGroupName, name)
	}

	return nil
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
		if repos, ok := c.GroupRepos(name); ok {
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

// AddRoot registers a directory to be walked live on every invocation.
func (c *Config) AddRoot(name string, root Root) {
	c.Roots[name] = root
}

// RemoveRoot stops tracking name as a live-resolved directory root.
func (c *Config) RemoveRoot(name string) {
	delete(c.Roots, name)
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

// UngroupedRepos returns the names of repos with no group, sorted.
func (c *Config) UngroupedRepos() []string {
	out := make([]string, 0, len(c.Repos))

	for name, repo := range c.Repos {
		if len(repo.Groups) == 0 {
			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

// GroupRepos resolves name to a repo list: a real group (with or without
// its "@" prefix), or a reserved pseudo-group (ReservedNone, ReservedAttention).
func (c *Config) GroupRepos(name string) ([]string, bool) {
	if name == ReservedNone {
		return c.UngroupedRepos(), true
	}

	if name == ReservedAttention {
		return c.allRepos(), true
	}

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
