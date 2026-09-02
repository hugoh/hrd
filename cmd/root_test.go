package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCompletesReposAndGroups(t *testing.T) {
	cfgPath := twoRepoGroupConfig(t)

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
