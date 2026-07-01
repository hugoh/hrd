package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zenizh/go-capturer"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	git.Register()
	jj.Register()
	goleak.VerifyTestMain(m)
}

// Helper to create a config file and return its path + cleanup function.
func setupTestConfig(t *testing.T, cfg config.Config) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	err := config.Save(cfgPath, cfg)
	require.NoError(t, err)

	return cfgPath
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	return capturer.CaptureStdout(fn)
}

// makeFakeGitRepoAt turns dir into what backend.DetectAll recognizes as a git repo.
func makeFakeGitRepoAt(t *testing.T, dir string) {
	t.Helper()
	// Initialize a real git repo so backend.DetectAll can find it.
	err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750)
	require.NoError(t, err)
	// Create minimal git repo structure
	err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]\n"), 0o644)
	require.NoError(t, err)
}

// Helper to create a temporary directory that looks like a git repo.
func setupFakeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	makeFakeGitRepoAt(t, dir)

	return dir
}

// Helper to create a temporary directory that looks like a jj repo.
func setupFakeJJRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o750)
	require.NoError(t, err)

	return dir
}

func TestFilterMatching(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	jjDir := setupFakeJJRepo(t)
	bothDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bothDir, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(bothDir, ".jj"), 0o750))

	repos := map[string]config.Repo{
		"gitrepo":  {Path: gitDir},
		"jjrepo":   {Path: jjDir},
		"bothrepo": {Path: bothDir},
	}

	tests := []struct {
		name      string
		names     []string
		backend   string
		wantNames []string
	}{
		{
			"git backend",
			[]string{"gitrepo", "jjrepo", "bothrepo"},
			"git",
			[]string{"gitrepo", "bothrepo"},
		},
		{
			"jj backend",
			[]string{"gitrepo", "jjrepo", "bothrepo"},
			"jj",
			[]string{"jjrepo", "bothrepo"},
		},
		{"nonexistent repo", []string{"gitrepo", "unknown"}, "git", []string{"gitrepo"}},
		{"empty names", nil, "git", nil},
		{"no match", []string{"jjrepo"}, "git", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterMatching(tt.names, repos, tt.backend)
			assert.Equal(t, tt.wantNames, got)
		})
	}
}

// ─── Command-level tests via app.Run() ──────────────────────────────────────────

// checkNameFlagHint asserts the app error nudges the user toward --name/-n,
// the error TestRepoAdd expects on any repo-name collision.
func checkNameFlagHint(t *testing.T, _ string, appErr error) {
	t.Helper()
	assert.Contains(t, appErr.Error(), "--name/-n")
}

func TestRepoAdd(t *testing.T) { //nolint:funlen
	tests := []struct {
		name    string
		setup   func(t *testing.T) (string, []string)
		wantErr error
		check   func(t *testing.T, cfgPath string, appErr error)
	}{
		{
			name: "TestRepoAddAndList",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()
				gitDir := setupFakeGitRepo(t)

				return setupTestConfig(
						t,
						config.Config{},
					), []string{
						"repo",
						"add",
						"--name",
						"mygit",
						gitDir,
					}
			},
			check: func(t *testing.T, cfgPath string, _ error) {
				t.Helper()

				app := NewApp()

				var err error

				captureStdout(t, func() {
					app.ErrWriter = &bytes.Buffer{}
					err = app.Run(
						t.Context(),
						[]string{"hrd", "--config", cfgPath, "repo", "ls"},
					)
				})
				require.NoError(t, err)

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)

				_, ok := cfg.Repos["mygit"]
				assert.True(t, ok)
			},
		},
		{
			name: "TestRepoAddMultiple",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()
				gitDir := setupFakeGitRepo(t)
				jjDir := setupFakeJJRepo(t)

				return setupTestConfig(t, config.Config{}), []string{"repo", "add", gitDir, jjDir}
			},
			check: func(t *testing.T, cfgPath string, _ error) {
				t.Helper()

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)
				assert.Len(t, cfg.Repos, 2)
			},
		},
		{
			name: "TestRepoAddExplicitNameCollision",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()
				gitDir := setupFakeGitRepo(t)
				cfgPath := cfgRepoNameTaken(t)

				return cfgPath, []string{"repo", "add", "--name", "repo", gitDir}
			},
			wantErr: errRepoExists,
			check:   checkNameFlagHint,
		},
		{
			name: "TestRepoAddImplicitNameCollision",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()
				dir := t.TempDir()
				repoDir := filepath.Join(dir, "repo")
				makeFakeGitRepoAt(t, repoDir)
				cfgPath := cfgRepoNameTaken(t)

				return cfgPath, []string{"repo", "add", repoDir}
			},
			wantErr: errRepoExists,
			check:   checkNameFlagHint,
		},
		{
			name: "TestRepoAddNoPath",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()

				return setupTestConfig(t, config.Config{}), []string{"repo", "add"}
			},
			wantErr: errAtLeastOnePath,
		},
		{
			name: "TestRepoAddNameWithMultiple",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()
				gitDir := setupFakeGitRepo(t)
				jjDir := setupFakeJJRepo(t)

				return setupTestConfig(
						t,
						config.Config{},
					), []string{
						"repo",
						"add",
						"--name",
						"mygit",
						gitDir,
						jjDir,
					}
			},
			wantErr: errNameSingleRepo,
		},
		{
			name: "TestRepoAddNoVCS",
			setup: func(t *testing.T) (string, []string) {
				t.Helper()
				emptyDir := t.TempDir()

				return setupTestConfig(t, config.Config{}), []string{"repo", "add", emptyDir}
			},
			wantErr: errRepoNoVCS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath, args := tt.setup(t)

			err := runApp(t, cfgPath, args)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.check != nil {
				tt.check(t, cfgPath, err)
			}
		})
	}
}

func TestRepoRemove(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		args    []string
		wantErr error
		check   func(t *testing.T, cfgPath string)
	}{
		{
			name: "TestRepoRemove",
			cfg: config.Config{Repos: map[string]config.Repo{
				"myrepo": {Path: "/tmp/myrepo"},
			}},
			args: []string{"repo", "rm", "myrepo"},
			check: func(t *testing.T, cfgPath string) {
				t.Helper()

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)
				assert.Empty(t, cfg.Repos)
			},
		},
		{
			name: "TestRepoRemoveNoName",
			cfg: config.Config{Repos: map[string]config.Repo{
				"myrepo": {Path: "/tmp/myrepo"},
			}},
			args:    []string{"repo", "rm"},
			wantErr: errAtLeastOneName,
		},
		{
			name: "TestRepoRemoveUnknown",
			cfg: config.Config{Repos: map[string]config.Repo{
				"myrepo": {Path: "/tmp/myrepo"},
			}},
			args:    []string{"repo", "rm", "unknown"},
			wantErr: errUnknownRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)

			err := runApp(t, cfgPath, tt.args)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, cfgPath)
			}
		})
	}
}

func TestRepoRename(t *testing.T) { //nolint:funlen
	tests := []struct {
		name      string
		cfg       config.Config
		argGroups [][]string
		wantErr   error
		check     func(t *testing.T, cfgPath string)
	}{
		{
			name: "TestRepoRename",
			cfg: config.Config{Repos: map[string]config.Repo{
				"oldname": {Path: "/tmp/oldname"},
			}},
			argGroups: [][]string{{"repo", "rename", "oldname", "newname"}},
			check: func(t *testing.T, cfgPath string) {
				t.Helper()

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)

				_, ok := cfg.Repos["newname"]
				assert.True(t, ok)
				_, ok = cfg.Repos["oldname"]
				assert.False(t, ok)
			},
		},
		{
			name:      "TestRepoRenameUsageError",
			cfg:       config.Config{},
			argGroups: [][]string{{"repo", "rename"}, {"repo", "rename", "onlyone"}},
			wantErr:   errRepoRenameUsage,
		},
		{
			name:      "TestRepoRenameUnknown",
			cfg:       config.Config{},
			argGroups: [][]string{{"repo", "rename", "unknown", "new"}},
			wantErr:   errUnknownRepo,
		},
		{
			name: "TestRepoRenameExists",
			cfg: config.Config{Repos: map[string]config.Repo{
				"repo1": {Path: "/tmp/repo1"},
				"repo2": {Path: "/tmp/other"},
			}},
			argGroups: [][]string{{"repo", "rename", "repo1", "repo2"}},
			wantErr:   errRepoExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)
			for _, args := range tt.argGroups {
				err := runApp(t, cfgPath, args)
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
				} else {
					require.NoError(t, err)
				}
			}

			if tt.check != nil {
				tt.check(t, cfgPath)
			}
		})
	}
}

func TestResolveScope(t *testing.T) {
	cfg := cfgTwoReposOneGroup()

	// Note: resolveScope takes a *cli.Command, which is hard to mock directly.
	// We'll test it indirectly through the command actions.
	// For now, let's test the config.ResolveScope directly.
	names, err := cfg.ResolveScope([]string{"repo1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo1"}, names)

	names, err = cfg.ResolveScope([]string{"work"})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo1"}, names)
}

func TestCompletion(t *testing.T) {
	tests := []struct {
		name  string
		shell string
	}{
		{name: "TestCompletionBash", shell: "bash"},
		{name: "TestCompletionZsh", shell: "zsh"},
		{name: "TestCompletionFish", shell: "fish"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			stdout := captureStdout(t, func() {
				app.ErrWriter = &bytes.Buffer{}
				err := app.Run(t.Context(), []string{"hrd", "completion", tt.shell})
				assert.NoError(t, err)
			})
			assert.Contains(t, stdout, "hrd")
		})
	}
}

// Test error cases for Git and JJ commands without args.
func TestGitCommandNoArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1"},
		},
	})

	app := newTestApp()

	// git command without -- and args should fail
	err := app.Run(t.Context(), []string{"hrd", "--config", cfgPath, "git"})
	assert.Error(t, err)
}

func TestRepoList(t *testing.T) {
	tests := []struct {
		name         string
		group        string
		wantErr      bool
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "TestRepoLsWithGroupFilter",
			group:        "work",
			wantContains: []string{"repo1"},
			wantExcludes: []string{"repo2"},
		},
		{name: "TestRepoLsUnknownGroup", group: "unknown", wantErr: true},
		{
			name:         "TestRepoListWithAtGroup",
			group:        "@work",
			wantContains: []string{"repo1"},
			wantExcludes: []string{"repo2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Groups: []string{"work"}},
					"repo2": {Path: "/tmp/repo2"},
				},
			})
			app := NewApp()

			var err error

			stdout := captureStdout(t, func() {
				app.ErrWriter = &bytes.Buffer{}
				err = app.Run(
					t.Context(),
					[]string{"hrd", "--config", cfgPath, "repo", "ls", "--group", tt.group},
				)
			})
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, stdout, want)
			}

			for _, exclude := range tt.wantExcludes {
				assert.NotContains(t, stdout, exclude)
			}
		})
	}
}

func TestHelpersStripGroupPrefix(t *testing.T) {
	assert.Equal(t, "work", stripGroupPrefix("@work"))
	assert.Equal(t, "work", stripGroupPrefix("work"))
	assert.Empty(t, stripGroupPrefix("@"))
	assert.Empty(t, stripGroupPrefix(""))
}

func TestHelpersDisplayGroup(t *testing.T) {
	assert.Equal(t, "@work", displayGroup("work"))
	assert.Equal(t, "@work", displayGroup("@work"))
	assert.Equal(t, "@", displayGroup("@"))
	assert.Equal(t, "@", displayGroup(""))
}

func TestRootAction(t *testing.T) {
	orig := runTUI

	runTUI = func(_ context.Context, opts tui.Options) error {
		assert.Equal(t, config.DefaultPath(), opts.ConfigPath)

		return nil
	}

	defer func() { runTUI = orig }()

	app := NewApp()
	err := app.Action(t.Context(), app)
	require.NoError(t, err)
}
