// Package config handles loading, saving, and querying the hrd configuration
// file (~/.config/hrd/config.toml by default).
package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	renameio "github.com/google/renameio/v2/maybe"
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

	// Aliases holds only the user's own alias definitions, as parsed from
	// config.toml; built-in defaults (see defaultAliases) are layered on
	// top by EffectiveAliases and never appear here, so Save never writes
	// them back into the user's file. Decoded/encoded by hand in Load/Save
	// (see decodeAliasSpecs/encodeAliasSpecs) because each value can be
	// either a plain command string or a per-backend table, which plain
	// struct-tag (un)marshaling can't express.
	Aliases map[string]AliasSpec `toml:"-"`

	Settings Settings `toml:"settings"`
}

// AliasSpec is one alias's command expansion: either a single command used
// for every repo regardless of backend, or per-backend variants keyed by
// backend name (e.g. "git", "jj"). In config.toml this is written as either
// a plain string ("pull --rebase") or an inline table
// ({ git = "...", jj = "..." }).
type AliasSpec struct {
	Command  string
	Backends map[string]string
}

// Resolve returns the command expansion to use for backendName, and
// whether one was found: Command if set (a plain alias applies to every
// backend), otherwise the per-backend variant.
func (a AliasSpec) Resolve(backendName string) (string, bool) {
	if a.Command != "" {
		return a.Command, true
	}

	cmd, ok := a.Backends[backendName]

	return cmd, ok
}

const (
	backendNameGit = "git"
	backendNameJJ  = "jj"
)

//nolint:gochecknoglobals // read-only registry of built-in aliases, not mutated at runtime
var defaultAliases = map[string]AliasSpec{
	"up": {
		Backends: map[string]string{
			backendNameGit: `!git fetch --prune && git rebase @{u}`,
			backendNameJJ:  `!jj util exec -- sh -c "jj git fetch && jj rebase --skip-emptied -d 'trunk()'"`,
		},
	},
}

// EffectiveAliases merges the built-in default aliases with the user's own,
// which take precedence on a name collision. The result is what callers
// (CLI command registration, the TUI command bar) should use to resolve an
// alias name — never Aliases directly, which holds only what's on disk.
func (c *Config) EffectiveAliases() map[string]AliasSpec {
	out := make(map[string]AliasSpec, len(defaultAliases)+len(c.Aliases))

	maps.Copy(out, defaultAliases)
	maps.Copy(out, c.Aliases)

	return out
}

var (
	errAliasInvalidType   = errors.New("alias must be a string or a table of backend -> command")
	errAliasBackendNotStr = errors.New("alias backend command must be a string")
)

// decodeAliasSpecs converts the raw TOML values decoded for the "aliases"
// table (each either a string or a table of strings) into AliasSpecs.
func decodeAliasSpecs(raw map[string]any) (map[string]AliasSpec, error) {
	if len(raw) == 0 {
		return nil, nil //nolint:nilnil // absent "aliases" table is a valid, non-error empty result
	}

	out := make(map[string]AliasSpec, len(raw))

	for name, v := range raw {
		spec, err := aliasSpecFromRaw(v)
		if err != nil {
			return nil, fmt.Errorf("alias %q: %w", name, err)
		}

		out[name] = spec
	}

	return out, nil
}

func aliasSpecFromRaw(raw any) (AliasSpec, error) {
	switch v := raw.(type) {
	case string:
		return AliasSpec{Command: v}, nil
	case map[string]any:
		backends := make(map[string]string, len(v))

		for backendName, val := range v {
			s, ok := val.(string)
			if !ok {
				return AliasSpec{}, fmt.Errorf("backend %q: %w", backendName, errAliasBackendNotStr)
			}

			backends[backendName] = s
		}

		return AliasSpec{Backends: backends}, nil
	default:
		return AliasSpec{}, fmt.Errorf("%w: got %T", errAliasInvalidType, raw)
	}
}

// encodeAliasSpecs converts Aliases back into the raw string-or-table shape
// Save writes to TOML.
func encodeAliasSpecs(specs map[string]AliasSpec) map[string]any {
	if len(specs) == 0 {
		return nil
	}

	out := make(map[string]any, len(specs))

	for name, spec := range specs {
		if spec.Backends != nil {
			out[name] = spec.Backends
		} else {
			out[name] = spec.Command
		}
	}

	return out
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

	var aliasesRaw struct {
		Aliases map[string]any `toml:"aliases"`
	}
	if err := toml.Unmarshal(data, &aliasesRaw); err != nil {
		return cfg, fmt.Errorf("decoding config %q: %w", path, err)
	}

	cfg.Aliases, err = decodeAliasSpecs(aliasesRaw.Aliases)
	if err != nil {
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

	out := struct {
		Repos    map[string]Repo `toml:"repos"`
		Roots    map[string]Root `toml:"roots"`
		Aliases  map[string]any  `toml:"aliases,omitempty"`
		Settings Settings        `toml:"settings"`
	}{
		Repos:    cfg.Repos,
		Roots:    cfg.Roots,
		Aliases:  encodeAliasSpecs(cfg.Aliases),
		Settings: cfg.Settings,
	}

	data, err := toml.Marshal(out)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := renameio.WriteFile(path, data, defaultFilePerm); err != nil {
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

// IsKnownReservedGroup reports whether name is one of the tokens listed in
// ReservedGroups — as opposed to merely having the "@@" prefix (which
// IsReservedGroupName checks). Scope-validation code should use this so a
// new entry added to ReservedGroups is automatically accepted, instead of
// requiring a separate hardcoded name check to be kept in sync.
func IsKnownReservedGroup(name string) bool {
	return slices.ContainsFunc(ReservedGroups, func(rg ReservedGroupInfo) bool {
		return rg.Name == name
	})
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
