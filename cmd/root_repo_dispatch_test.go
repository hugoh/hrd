package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLsDiscoversRootRepos verifies that a repo dropped under a configured
// Root shows up in `hrd ls` output without any config.toml mutation for the
// discovered repo itself.
func TestLsDiscoversRootRepos(t *testing.T) {
	backend.ResetDetectCache()

	root := t.TempDir()
	repoDir := filepath.Join(root, "foo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0o750))

	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{"myroot": {Path: root}},
	})

	before, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	out := runAppCapture(t, cfgPath, []string{"ls", "--names"})
	assert.Equal(t, "foo", strings.TrimSpace(out))

	after, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}
