package discover

import (
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/discover/discovertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	git.Register()
	m.Run()
}

func TestRepos_FindsRepos(t *testing.T) {
	backend.ResetDetectCache()

	root := discovertest.Tree(t)

	found, warnings, err := Repos(root, 5)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.ElementsMatch(t, []string{
		filepath.Join(root, "work", "app"),
		filepath.Join(root, "oss", "app"),
	}, found)
}

func TestRepos_RespectsDepth(t *testing.T) {
	backend.ResetDetectCache()

	root := discovertest.Tree(t)

	found, _, err := Repos(root, 1)
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestRepos_SkipsNested(t *testing.T) {
	backend.ResetDetectCache()

	root := discovertest.Tree(t)

	found, _, err := Repos(root, 5)
	require.NoError(t, err)
	assert.NotContains(t, found, filepath.Join(root, "work", "app", "vendor", "dep"))
}

func TestDepth(t *testing.T) {
	root := "/a"
	assert.Equal(t, 0, Depth(root, "/a"))
	assert.Equal(t, 1, Depth(root, "/a/b"))
	assert.Equal(t, 2, Depth(root, "/a/b/c"))
}
