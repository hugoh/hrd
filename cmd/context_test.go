package cmd

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextSet(t *testing.T) { //nolint:funlen
	tests := []struct {
		name        string
		cfg         config.Config
		args        []string
		wantErr     error
		wantErrText string
		wantCurrent string
		wantShow    string
	}{
		{
			name:        "TestContextSetAndShow",
			cfg:         baseContextConfig(""),
			args:        []string{"context", "set", "work"},
			wantCurrent: "work",
			wantShow:    "work",
		},
		{
			name:        "TestContextSetWithAtPrefix",
			cfg:         baseContextConfig(""),
			args:        []string{"context", "set", "@work"},
			wantCurrent: "work",
			wantShow:    "@work",
		},
		{
			name:    "TestContextSetTooFewArgs",
			cfg:     config.Config{Groups: map[string]config.Group{"work": {Repos: []string{}}}},
			args:    []string{"context", "set"},
			wantErr: errContextSetUsage,
		},
		{
			name:    "TestContextSetUnknownGroup",
			cfg:     config.Config{},
			args:    []string{"context", "set", "unknown"},
			wantErr: errUnknownGroup,
		},
		{
			name:        "TestContextSetUnknownAtGroup",
			cfg:         config.Config{},
			args:        []string{"context", "set", "@unknown"},
			wantErr:     errUnknownGroup,
			wantErrText: "@unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)

			err := runApp(t, cfgPath, tt.args)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				if tt.wantErrText != "" {
					assert.Contains(t, err.Error(), tt.wantErrText)
				}

				return
			}

			require.NoError(t, err)

			cfg, err := config.Load(cfgPath)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCurrent, cfg.Context.Current)

			if tt.wantShow != "" {
				stdout := runAppCapture(t, cfgPath, []string{"context", "show"})
				assert.Contains(t, stdout, tt.wantShow)
			}
		})
	}
}

func TestContextShow(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.Config
		wantContent string
	}{
		{name: "TestContextShowEmpty", cfg: config.Config{}, wantContent: "all repos"},
		{name: "TestContextShowWithAtPrefix", cfg: baseContextConfig("work"), wantContent: "@work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)
			stdout := runAppCapture(t, cfgPath, []string{"context", "show"})
			assert.Contains(t, stdout, tt.wantContent)
		})
	}
}

func TestContextClear(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
	}{
		{name: "TestContextClear", cfg: baseContextConfig("work")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := setupTestConfig(t, tt.cfg)
			err := runApp(t, cfgPath, []string{"context", "clear"})
			require.NoError(t, err)

			cfg, err := config.Load(cfgPath)
			require.NoError(t, err)
			assert.Empty(t, cfg.Context.Current)
		})
	}
}

func baseContextConfig(current string) config.Config {
	return config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1"}},
		},
		Context: config.Context{Current: current},
	}
}
