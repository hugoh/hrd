package cmd

import (
	"context"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zenizh/go-capturer"
)

func tagListConfig() config.Config {
	return config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Tags: []string{"work"}},
			"repo2": {Path: "/tmp/repo2", Tags: []string{"work"}},
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
			cfg:         tagListConfig(),
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
			cfg:        tagListConfig(),
			args:       []string{"group", "ls", "work"},
			wantOutput: "repo1\nrepo2\n",
		},
		{
			name:       "TestGroupListWithNameAtPrefix",
			cfg:        tagListConfig(),
			args:       []string{"group", "ls", "@work"},
			wantOutput: "repo1\nrepo2\n",
		},
		{
			name: "TestGroupListUnknownName",
			cfg: config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Tags: []string{"work"}},
				},
			},
			args:    []string{"group", "ls", "nonexistent"},
			wantErr: errUnknownGroup,
		},
		{
			name: "TestGroupListUnknownAtName",
			cfg: config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Tags: []string{"work"}},
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
				err = app.Run(context.Background(), fullArgs)
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

func TestRepoTags(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1"},
			"repo2": {Path: "/tmp/repo2"},
		},
	})

	err := runApp(t, cfgPath, []string{"repo", "tag", "repo1", "work"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"work"}, cfg.Repos["repo1"].Tags)
	assert.Contains(t, cfg.Groups, "work")
	assert.Equal(t, []string{"repo1"}, cfg.Groups["work"].Repos)

	// Tag same repo again — should warn, not duplicate
	err = runApp(t, cfgPath, []string{"repo", "tag", "repo1", "work"})
	require.NoError(t, err)

	cfg, err = config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"work"}, cfg.Repos["repo1"].Tags)
}

func TestRepoTagsUnknownRepo(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	err := runApp(t, cfgPath, []string{"repo", "tag", "nonexistent", "work"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}

func TestRepoUntag(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Tags: []string{"work", "oss"}},
			"repo2": {Path: "/tmp/repo2", Tags: []string{"work"}},
		},
	})

	err := runApp(t, cfgPath, []string{"repo", "untag", "repo1", "work"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"oss"}, cfg.Repos["repo1"].Tags)
	assert.NotContains(t, cfg.Groups["work"].Repos, "repo1")

	// Untag non-existent tag — should warn, not error
	err = runApp(t, cfgPath, []string{"repo", "untag", "repo1", "nonexistent"})
	require.NoError(t, err)
}

func TestRepoUntagUnknownRepo(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	err := runApp(t, cfgPath, []string{"repo", "untag", "nonexistent", "work"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}
