package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/require"
)

type addCase struct {
	name    string
	setup   func(t *testing.T) (config.Config, [][]string)
	wantErr error
	check   func(t *testing.T, cfg config.Config)
}

func runAddCases(t *testing.T, tests []addCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial, argGroups := tt.setup(t)
			cfgPath := setupTestConfig(t, initial)

			var err error

			for _, args := range argGroups {
				err = runHRD(t, cfgPath, args)
				if err != nil {
					break
				}
			}

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			cfg, err := config.Load(cfgPath)
			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestRepoAddIdempotency(t *testing.T) { //nolint:funlen
	runAddCases(t, []addCase{
		{
			name: "same path and name is a no-op",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := setupFakeGitRepo(t)

				return config.Config{
					Repos: map[string]config.Repo{filepath.Base(dir): {Path: dir}},
				}, [][]string{{"repo", "add", dir}}
			},
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Len(t, cfg.Repos, 1)
			},
		},
		{
			name: "same path under a different name is rejected",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := setupFakeGitRepo(t)

				return config.Config{
					Repos: map[string]config.Repo{"first": {Path: dir}},
				}, [][]string{{"repo", "add", "-n", "second", dir}}
			},
			wantErr: errRepoPathTracked,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Len(t, cfg.Repos, 1)
			},
		},
		{
			name: "symlink to a tracked path is rejected",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := setupFakeGitRepo(t)
				link := filepath.Join(t.TempDir(), "alias")
				require.NoError(t, os.Symlink(dir, link))

				return config.Config{
					Repos: map[string]config.Repo{"first": {Path: dir}},
				}, [][]string{{"repo", "add", link}}
			},
			wantErr: errRepoPathTracked,
		},
		{
			name: "batch skips already tracked repos",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				tracked := setupFakeGitRepo(t)
				fresh := setupFakeJJRepo(t)

				return config.Config{
					Repos: map[string]config.Repo{filepath.Base(tracked): {Path: tracked}},
				}, [][]string{{"repo", "add", tracked, fresh}}
			},
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Len(t, cfg.Repos, 2)
			},
		},
		{
			name: "a failing arg does not discard the others",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()

				return config.Config{}, [][]string{
					{"repo", "add", setupFakeGitRepo(t), t.TempDir(), setupFakeJJRepo(t)},
				}
			},
			wantErr: errRepoNoVCS,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Len(t, cfg.Repos, 2)
			},
		},
	})
}

func TestRepoRootAddIdempotency(t *testing.T) { //nolint:funlen
	runAddCases(t, []addCase{
		{
			name: "identical re-add is a no-op",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := t.TempDir()

				return config.Config{}, [][]string{
					{"repo", "root", "add", dir},
					{"repo", "root", "add", dir},
				}
			},
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Len(t, cfg.Roots, 1)
			},
		},
		{
			name: "re-add merges groups",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := t.TempDir()

				return config.Config{}, [][]string{
					{"repo", "root", "add", dir, "-n", "r", "-g", "a"},
					{"repo", "root", "add", dir, "-n", "r", "-g", "a", "-g", "b"},
				}
			},
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.ElementsMatch(t, []string{"a", "b"}, cfg.Roots["r"].Groups)
			},
		},
		{
			name: "different depth is rejected",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := t.TempDir()

				return config.Config{}, [][]string{
					{"repo", "root", "add", dir, "--depth", "2"},
					{"repo", "root", "add", dir, "--depth", "3"},
				}
			},
			wantErr: errRootDepthConflict,
		},
		{
			name: "same path under a different name is rejected",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()
				dir := t.TempDir()

				return config.Config{
					Roots: map[string]config.Root{"first": {Path: dir}},
				}, [][]string{{"repo", "root", "add", dir, "-n", "second"}}
			},
			wantErr: errRootPathTracked,
		},
		{
			name: "missing directory is rejected",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()

				return config.Config{}, [][]string{
					{"repo", "root", "add", filepath.Join(t.TempDir(), "nope")},
				}
			},
			wantErr: errRootNotDir,
		},
		{
			name: "a failing arg does not discard the others",
			setup: func(t *testing.T) (config.Config, [][]string) {
				t.Helper()

				return config.Config{}, [][]string{
					{
						"repo",
						"root",
						"add",
						t.TempDir(),
						filepath.Join(t.TempDir(), "nope"),
						t.TempDir(),
					},
				}
			},
			wantErr: errRootNotDir,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Len(t, cfg.Roots, 2)
			},
		},
	})
}
