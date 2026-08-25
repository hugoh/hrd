package cmd

import (
	"bytes"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCompletesReposAndGroups(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo-a": {Path: "/tmp/repo-a", Groups: []string{"work"}},
			"repo-b": {Path: "/tmp/repo-b"},
		},
	})

	app := NewApp()

	var out bytes.Buffer
	app.SetOut(&out)
	app.SetErr(&out)
	app.SetArgs([]string{"--config", cfgPath, "__complete", ""})

	require.NoError(t, app.Execute())

	got := out.String()
	assert.Contains(t, got, "repo-a")
	assert.Contains(t, got, "repo-b")
	assert.Contains(t, got, "@work")
}
