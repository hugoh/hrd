package cmd

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zenizh/go-capturer"
)

func groupListConfig() config.Config {
	return config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Groups: []string{"work"}},
			"repo2": {Path: "/tmp/repo2", Groups: []string{"work"}},
			"repo3": {Path: "/tmp/repo3"},
		},
	}
}

func TestGroupList(t *testing.T) { //nolint:funlen
	tests := []struct {
		name        string
		cfg         config.Config
		args        []string
		wantErr     error
		wantOutput  string
		wantContent string
		wantErrText string
	}{
		{
			name:        "TestGroupListWithPanel",
			cfg:         groupListConfig(),
			args:        []string{"group", "ls"},
			wantContent: "work",
		},
		{
			name:        "TestGroupListNoGroups",
			cfg:         config.Config{Repos: map[string]config.Repo{}},
			args:        []string{"group", "ls"},
			wantContent: "no groups defined",
		},
		{
			name:       "TestGroupListWithName",
			cfg:        groupListConfig(),
			args:       []string{"group", "ls", "work"},
			wantOutput: "repo1\nrepo2\n",
		},
		{
			name:       "TestGroupListWithNameAtPrefix",
			cfg:        groupListConfig(),
			args:       []string{"group", "ls", "@work"},
			wantOutput: "repo1\nrepo2\n",
		},
		{
			name: "TestGroupListUnknownName",
			cfg: config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Groups: []string{"work"}},
				},
			},
			args:    []string{"group", "ls", "nonexistent"},
			wantErr: errUnknownGroup,
		},
		{
			name:       "TestGroupListReservedNone",
			cfg:        groupListConfig(),
			args:       []string{"group", "ls", "@@none"},
			wantOutput: "repo3\n",
		},
		{
			name: "TestGroupListUnknownAtName",
			cfg: config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Groups: []string{"work"}},
				},
			},
			args:        []string{"group", "ls", "@nonexistent"},
			wantErr:     errUnknownGroup,
			wantErrText: "@nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)

			app := newTestApp()

			var err error

			stdout := capturer.CaptureStdout(func() {
				fullArgs := append([]string{"hrd", "--config", cfgPath}, tt.args...)
				err = RunApp(t.Context(), app, fullArgs)
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				if tt.wantErrText != "" {
					assert.Contains(t, err.Error(), tt.wantErrText)
				}

				return
			}

			require.NoError(t, err)

			if tt.wantOutput != "" {
				assert.Equal(t, tt.wantOutput, stdout)
			}

			if tt.wantContent != "" {
				assert.Contains(t, stdout, tt.wantContent)
			}
		})
	}
}

func TestRepoGroup(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1"},
			"repo2": {Path: "/tmp/repo2"},
		},
	})

	err := runHRD(t, cfgPath, []string{"repo", "group", "repo1", "work"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"work"}, cfg.Repos["repo1"].Groups)
	assert.Contains(t, cfg.Groups, "work")
	assert.Equal(t, []string{"repo1"}, cfg.Groups["work"].Repos)

	// Add same group again — should no-op
	err = runHRD(t, cfgPath, []string{"repo", "group", "repo1", "work"})
	require.NoError(t, err)

	cfg, err = config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"work"}, cfg.Repos["repo1"].Groups)
}

func TestRepoGroupUnknownRepo(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	err := runHRD(t, cfgPath, []string{"repo", "group", "nonexistent", "work"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}

func TestRepoUngroup(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Groups: []string{"work", "oss"}},
			"repo2": {Path: "/tmp/repo2", Groups: []string{"work"}},
		},
	})

	err := runHRD(t, cfgPath, []string{"repo", "ungroup", "repo1", "work"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"oss"}, cfg.Repos["repo1"].Groups)
	assert.NotContains(t, cfg.Groups["work"].Repos, "repo1")

	// Ungroup non-existent group — should no-op
	err = runHRD(t, cfgPath, []string{"repo", "ungroup", "repo1", "nonexistent"})
	require.NoError(t, err)
}

func TestRepoUngroupUnknownRepo(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	err := runHRD(t, cfgPath, []string{"repo", "ungroup", "nonexistent", "work"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}
