package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasSpec_Resolve(t *testing.T) {
	for _, tt := range []struct {
		name        string
		spec        AliasSpec
		backend     string
		wantCommand string
		wantOK      bool
	}{
		{
			name:        "plain command applies to any backend",
			spec:        AliasSpec{Command: "pull --rebase"},
			backend:     "jj",
			wantCommand: "pull --rebase",
			wantOK:      true,
		},
		{
			name:        "backend variant present",
			spec:        AliasSpec{Backends: map[string]string{"git": "git-cmd", "jj": "jj-cmd"}},
			backend:     "jj",
			wantCommand: "jj-cmd",
			wantOK:      true,
		},
		{
			name:    "backend variant missing",
			spec:    AliasSpec{Backends: map[string]string{"git": "git-cmd"}},
			backend: "jj",
			wantOK:  false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ok := tt.spec.Resolve(tt.backend)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantCommand, cmd)
		})
	}
}

func TestConfig_EffectiveAliases(t *testing.T) {
	t.Run("built-in up is present with no user config", func(t *testing.T) {
		cfg := defaultConfig()

		effective := cfg.EffectiveAliases()

		up, ok := effective["up"]
		require.True(t, ok)

		gitCmd, ok := up.Resolve("git")
		require.True(t, ok)
		assert.Contains(t, gitCmd, "git fetch")

		jjCmd, ok := up.Resolve("jj")
		require.True(t, ok)
		assert.Contains(t, jjCmd, "jj git fetch")
	})

	t.Run("user-defined alias overrides the built-in", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Aliases = map[string]AliasSpec{
			"up": {Command: "!echo custom"},
		}

		effective := cfg.EffectiveAliases()

		cmd, ok := effective["up"].Resolve("git")
		require.True(t, ok)
		assert.Equal(t, "!echo custom", cmd)
	})

	t.Run("user aliases besides the built-ins pass through untouched", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Aliases = map[string]AliasSpec{
			"sync": {Command: "pull --rebase"},
		}

		effective := cfg.EffectiveAliases()

		cmd, ok := effective["sync"].Resolve("git")
		require.True(t, ok)
		assert.Equal(t, "pull --rebase", cmd)
	})
}

func TestLoad_Aliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `[aliases]
sync = "pull --rebase"
up = { git = "git up", jj = "jj up" }
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)

	require.Contains(t, cfg.Aliases, "sync")
	cmd, ok := cfg.Aliases["sync"].Resolve("git")
	require.True(t, ok)
	assert.Equal(t, "pull --rebase", cmd)

	require.Contains(t, cfg.Aliases, "up")
	gitCmd, ok := cfg.Aliases["up"].Resolve("git")
	require.True(t, ok)
	assert.Equal(t, "git up", gitCmd)

	jjCmd, ok := cfg.Aliases["up"].Resolve("jj")
	require.True(t, ok)
	assert.Equal(t, "jj up", jjCmd)
}

func TestLoad_Aliases_InvalidBackendValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `[aliases]
bad = { git = 5 }
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func TestSaveLoad_AliasesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := defaultConfig()
	cfg.Aliases = map[string]AliasSpec{
		"sync": {Command: "pull --rebase"},
		"up":   {Backends: map[string]string{"git": "git up", "jj": "jj up"}},
	}

	require.NoError(t, Save(path, cfg))

	loaded, err := Load(path)
	require.NoError(t, err)

	require.Contains(t, loaded.Aliases, "sync")
	cmd, ok := loaded.Aliases["sync"].Resolve("git")
	require.True(t, ok)
	assert.Equal(t, "pull --rebase", cmd)

	require.Contains(t, loaded.Aliases, "up")
	gitCmd, ok := loaded.Aliases["up"].Resolve("git")
	require.True(t, ok)
	assert.Equal(t, "git up", gitCmd)
}

func TestSave_DoesNotPersistBuiltinAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// A config with no user-defined aliases at all.
	cfg := defaultConfig()

	require.NoError(t, Save(path, cfg))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(
		t,
		string(data),
		"up",
		"built-in default aliases must never be written to the user's config file",
	)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Empty(
		t,
		loaded.Aliases,
		"Load must not surface built-in defaults in Aliases; use EffectiveAliases",
	)
}
