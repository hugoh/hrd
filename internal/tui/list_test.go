package tui

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGroupItems_UngroupedDescription(t *testing.T) {
	items := buildGroupItems(
		[]string{labelAllRepos, labelUngrouped, "work"},
		map[string]config.Group{"work": {Repos: []string{"a"}}},
		3,
		2,
		0,
	)

	require.Len(t, items, 3)
	gi, ok := items[1].(groupItem)
	require.True(t, ok)
	assert.Equal(t, labelUngrouped, gi.name)
	assert.Equal(t, "2 repos", gi.desc)
}

func TestBuildGroupItems_AttentionDescription(t *testing.T) {
	items := buildGroupItems(
		[]string{labelAllRepos, labelUngrouped, labelAttention, "work"},
		map[string]config.Group{"work": {Repos: []string{"a"}}},
		3,
		2,
		1,
	)

	require.Len(t, items, 4)
	gi, ok := items[2].(groupItem)
	require.True(t, ok)
	assert.Equal(t, labelAttention, gi.name)
	assert.Equal(t, "1 repo", gi.desc)
}
