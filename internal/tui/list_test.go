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
	)

	require.Len(t, items, 3)
	gi, ok := items[1].(groupItem)
	require.True(t, ok)
	assert.Equal(t, labelUngrouped, gi.name)
	assert.Equal(t, "2 repos", gi.desc)
}
