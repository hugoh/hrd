package cmd

import (
	"context"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupAdd(t *testing.T) { //nolint:funlen
	tests := []struct {
		name    string
		cfg     config.Config
		args    []string
		wantErr error
		anyErr  bool
		check   func(t *testing.T, cfgPath string)
	}{
		{
			name: "TestGroupAddAndList",
			cfg: config.Config{Repos: map[string]config.Repo{
				"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
				"repo2": {Path: "/tmp/repo2", Backends: []string{"git"}},
			}},
			args: []string{"group", "add", "work", "repo1", "repo2"},
			check: func(t *testing.T, cfgPath string) {
				t.Helper()
				stdout := runAppCapture(t, cfgPath, []string{"group", "ls"})
				assert.Contains(t, stdout, "work")
			},
		},
		{
			name: "TestGroupAddWithAtPrefix",
			cfg: config.Config{Repos: map[string]config.Repo{
				"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
				"repo2": {Path: "/tmp/repo2", Backends: []string{"git"}},
			}},
			args: []string{"group", "add", "@work", "repo1", "repo2"},
			check: func(t *testing.T, cfgPath string) {
				t.Helper()

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)

				_, ok := cfg.Groups["work"]
				assert.True(t, ok, "group should be stored without @ prefix")
				_, ok = cfg.Groups["@work"]
				assert.False(t, ok, "group should NOT be stored with @ prefix")

				stdout := runAppCapture(t, cfgPath, []string{"group", "ls"})
				assert.Contains(t, stdout, "@work", "group table should show @ prefix")
			},
		},
		{
			name: "TestGroupAddWithoutAtPrefix",
			cfg: config.Config{Repos: map[string]config.Repo{
				"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
			}},
			args: []string{"group", "add", "work", "repo1"},
			check: func(t *testing.T, cfgPath string) {
				t.Helper()

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)

				_, ok := cfg.Groups["work"]
				assert.True(t, ok)

				stdout := runAppCapture(t, cfgPath, []string{"group", "ls"})
				assert.Contains(
					t,
					stdout,
					"@work",
					"group table should show @ prefix even for groups added without @",
				)
			},
		},
		{
			name: "TestGroupAddTooFewArgs",
			cfg: config.Config{Repos: map[string]config.Repo{
				"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
			}},
			args:    []string{"group", "add", "work"},
			wantErr: errGroupAddUsage,
		},
		{
			name:   "TestGroupAddUnknownRepo",
			cfg:    config.Config{Repos: map[string]config.Repo{}},
			args:   []string{"group", "add", "work", "unknown"},
			anyErr: true,
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

			if tt.anyErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, cfgPath)
			}
		})
	}
}

func TestGroupRemove(t *testing.T) { //nolint:funlen
	tests := []struct {
		name    string
		cfg     config.Config
		args    []string
		wantErr error
		check   func(t *testing.T, cfgPath string)
	}{
		{
			name:  "TestGroupRemove",
			cfg:   baseGroupConfig(),
			args:  []string{"group", "rm", "work"},
			check: assertGroupRemoved,
		},
		{
			name:  "TestGroupRemoveWithAtPrefix",
			cfg:   baseGroupConfig(),
			args:  []string{"group", "rm", "@work"},
			check: assertGroupRemoved,
		},
		{
			name:  "TestGroupRemoveWithoutAtPrefix",
			cfg:   baseGroupConfig(),
			args:  []string{"group", "rm", "work"},
			check: assertGroupRemoved,
		},
		{
			name:    "TestGroupRemoveTooFewArgs",
			cfg:     config.Config{Repos: map[string]config.Repo{}},
			args:    []string{"group", "rm"},
			wantErr: errGroupRmUsage,
		},
		{
			name: "TestGroupRemoveClearsContext",
			cfg: config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
				},
				Groups:  map[string]config.Group{"work": {Repos: []string{"repo1"}}},
				Context: config.Context{Current: "work"},
			},
			args: []string{"group", "rm", "work"},
			check: func(t *testing.T, cfgPath string) {
				t.Helper()

				cfg, err := config.Load(cfgPath)
				require.NoError(t, err)
				assert.Empty(t, cfg.Context.Current)
			},
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
			cfg:         baseGroupConfig(),
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
				Groups: map[string]config.Group{"work": {Repos: []string{"repo1"}}},
			},
			args:    []string{"group", "ls", "nonexistent"},
			wantErr: errUnknownGroup,
		},
		{
			name: "TestGroupListUnknownAtName",
			cfg: config.Config{
				Groups: map[string]config.Group{"work": {Repos: []string{"repo1"}}},
			},
			args:        []string{"group", "ls", "@nonexistent"},
			wantErr:     errUnknownGroup,
			wantErrText: "@nonexistent",
		},
		{
			name: "TestGroupFlowThroughAddThenList",
			cfg: config.Config{
				Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
				},
			},
			args:        []string{"group", "ls"},
			wantContent: "@work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)
			if tt.name == "TestGroupFlowThroughAddThenList" {
				require.NoError(t, runApp(t, cfgPath, []string{"group", "add", "work", "repo1"}))
			}

			app := newTestApp()

			var err error

			stdout := captureStdout(t, func() {
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

func baseGroupConfig() config.Config {
	return config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
			"repo2": {Path: "/tmp/repo2", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1", "repo2"}},
		},
	}
}

func groupListConfig() config.Config {
	return config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
			"repo2": {Path: "/tmp/repo2", Backends: []string{"git"}},
			"repo3": {Path: "/tmp/repo3", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1", "repo2"}},
		},
	}
}

func assertGroupRemoved(t *testing.T, cfgPath string) {
	t.Helper()

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	_, ok := cfg.Groups["work"]
	assert.False(t, ok, "group should be removed")
}
