package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/backend/backendtest"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusFilterMatches(t *testing.T) {
	dirtyStatus := backend.RepoStatus{Dirty: true}
	aheadStatus := backend.RepoStatus{
		Bookmarks: []backend.BookmarkStatus{{Name: "main", Ahead: 2}},
	}
	behindStatus := backend.RepoStatus{
		Bookmarks: []backend.BookmarkStatus{{Name: "main", Behind: 3}},
	}
	localAheadStatus := backend.RepoStatus{LocalAhead: 1}
	cleanStatus := backend.RepoStatus{
		Bookmarks: []backend.BookmarkStatus{{Name: "main"}},
	}

	for _, tt := range []struct {
		name   string
		filter statusFilter
		status backend.RepoStatus
		want   bool
	}{
		{name: "dirty matches dirty", filter: statusFilter{dirty: true}, status: dirtyStatus, want: true},
		{name: "dirty skips clean", filter: statusFilter{dirty: true}, status: cleanStatus, want: false},
		{name: "ahead matches bookmark ahead", filter: statusFilter{ahead: true}, status: aheadStatus, want: true},
		{name: "ahead matches local-only work", filter: statusFilter{ahead: true}, status: localAheadStatus, want: true},
		{name: "ahead skips behind", filter: statusFilter{ahead: true}, status: behindStatus, want: false},
		{name: "behind matches bookmark behind", filter: statusFilter{behind: true}, status: behindStatus, want: true},
		{name: "behind skips ahead", filter: statusFilter{behind: true}, status: aheadStatus, want: false},
		{name: "union matches either", filter: statusFilter{dirty: true, behind: true}, status: behindStatus, want: true},
		{name: "no flags matches nothing", filter: statusFilter{}, status: dirtyStatus, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.matches(tt.status))
		})
	}
}

// setupRealGitRepo creates a real git repo with one commit. When dirty is
// true an uncommitted file is left in the working tree.
func setupRealGitRepo(t *testing.T, dirty bool) string {
	t.Helper()
	backendtest.RequireIsolatedGit(t)

	dir := t.TempDir()

	for _, args := range [][]string{
		{"init", "-q"},
		{
			"-c", "user.email=test@test.com", "-c", "user.name=Test",
			"commit", "-q", "--allow-empty", "-m", "initial",
		},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}

	if dirty {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "wip.txt"), []byte("wip"), 0o600))
	}

	return dir
}

func TestLsDirtyFilter(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"ls", "--dirty", "--names"})
	assert.Equal(t, "dirtyrepo", strings.TrimSpace(out))
}

func TestLsFilterNoMatch(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"cleanrepo": {Path: setupRealGitRepo(t, false)},
	}})

	out := runAppCapture(t, cfgPath, []string{"ls", "--dirty", "--names"})
	assert.Empty(t, strings.TrimSpace(out))
}

// TestLsDirtyFilterRendersStatus exercises replayGather: with a status
// filter active and --names/--dirs not set, ls must render status using the
// statuses already gathered for filtering, without re-querying the repos.
func TestLsDirtyFilterRendersStatus(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"ls", "--dirty"})
	assert.Contains(t, out, "dirtyrepo")
	assert.NotContains(t, out, "cleanrepo")
}

func TestShellDirtyFilter(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"shell", "--dirty", "--", "echo ran"})
	assert.Contains(t, out, "dirtyrepo")
	assert.NotContains(t, out, "cleanrepo")
}
