package cmd

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBatchRemovalAppliesKnownNamesDespiteUnknown(t *testing.T) { //nolint:funlen
	tests := []struct {
		name    string
		cfg     config.Config
		args    []string
		wantErr error
		check   func(t *testing.T, cfg config.Config)
	}{
		{
			name: "repo rm",
			cfg: config.Config{Repos: map[string]config.Repo{
				"a": {Path: "/tmp/a"},
				"b": {Path: "/tmp/b"},
			}},
			args:    []string{"repo", "rm", "a", "ghost", "b"},
			wantErr: errUnknownRepo,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Empty(t, cfg.Repos)
			},
		},
		{
			name: "repo root rm",
			cfg: config.Config{Roots: map[string]config.Root{
				"a": {Path: "/tmp/a"},
				"b": {Path: "/tmp/b"},
			}},
			args:    []string{"repo", "root", "rm", "a", "ghost", "b"},
			wantErr: errUnknownRoot,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Empty(t, cfg.Roots)
			},
		},
		{
			name: "group rm",
			cfg: config.Config{Repos: map[string]config.Repo{
				"a": {Path: "/tmp/a", Groups: []string{"g"}},
				"b": {Path: "/tmp/b", Groups: []string{"g"}},
			}},
			args:    []string{"group", "rm", "g", "a", "ghost", "b"},
			wantErr: errUnknownRepo,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Empty(t, cfg.Repos["a"].Groups)
				require.Empty(t, cfg.Repos["b"].Groups)
			},
		},
		{
			name: "group add",
			cfg: config.Config{Repos: map[string]config.Repo{
				"a": {Path: "/tmp/a"},
				"b": {Path: "/tmp/b"},
			}},
			args:    []string{"group", "add", "g", "a", "ghost", "b"},
			wantErr: errUnknownRepo,
			check: func(t *testing.T, cfg config.Config) {
				t.Helper()
				require.Equal(t, []string{"g"}, cfg.Repos["a"].Groups)
				require.Equal(t, []string{"g"}, cfg.Repos["b"].Groups)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)

			require.ErrorIs(t, runHRD(t, cfgPath, tt.args), tt.wantErr)

			cfg, err := config.Load(cfgPath)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}
