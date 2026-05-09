package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatus_Empty(t *testing.T) {
	st := parseStatus("", nil)
	assert.Empty(t, st.Ref)
	assert.False(t, st.Dirty)
	assert.Empty(t, st.Bookmarks)
}

func TestParseStatus_BranchOnly(t *testing.T) {
	input := "# branch.head main\n"
	st := parseStatus(input, nil)
	assert.Equal(t, "main", st.Ref)
	assert.Equal(t, "main", st.Bookmarks[0].Name)
	assert.False(t, st.Dirty)
}

func TestParseStatus_DetachedHEAD(t *testing.T) {
	input := "# branch.head (detached)\n"
	st := parseStatus(input, nil)
	assert.Equal(t, "(detached)", st.Ref)
	assert.Equal(t, "(detached)", st.Bookmarks[0].Name)
}

func TestParseStatus_WithUpstream(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n"
	st := parseStatus(input, nil)
	assert.Equal(t, "origin", st.Bookmarks[0].Remote)
}

func TestParseStatus_UpstreamNoSlash(t *testing.T) {
	input := "# branch.head main\n# branch.upstream myremote\n"
	st := parseStatus(input, nil)
	assert.Equal(t, "myremote", st.Bookmarks[0].Remote)
}

func TestParseStatus_WithAheadBehind(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n# branch.ab +3 -1\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Equal(t, 3, bm.Ahead)
	assert.Equal(t, 1, bm.Behind)
	assert.Equal(t, "origin", bm.Remote)
}

func TestParseStatus_Dirty(t *testing.T) {
	input := "# branch.head main\nM  README.md\n"
	st := parseStatus(input, nil)
	assert.True(t, st.Dirty)
}

func TestParseStatus_MultipleDirtyLines(t *testing.T) {
	input := "# branch.head main\nM  file1.go\nA  file2.go\n?? file3.go\n"
	st := parseStatus(input, nil)
	assert.True(t, st.Dirty)
}

func TestParseStatus_Combined(t *testing.T) {
	input := "# branch.head feature/foo\n# branch.upstream origin/feature/foo\n# branch.ab +2 -0\nM  somefile.go\n"
	st := parseStatus(input, nil)
	assert.Equal(t, "feature/foo", st.Ref)
	assert.True(t, st.Dirty)
	bm := st.Bookmarks[0]
	assert.Equal(t, "origin", bm.Remote)
	assert.Equal(t, 2, bm.Ahead)
	assert.Equal(t, 0, bm.Behind)
	assert.Equal(t, "ahead", bm.State.String())
}

func TestParseStatus_Synced(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n# branch.ab +0 -0\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Equal(t, 0, bm.Ahead)
	assert.Equal(t, 0, bm.Behind)
	assert.Equal(t, "synced", bm.State.String())
}

func TestParseStatus_BehindOnly(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n# branch.ab +0 -5\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Equal(t, 5, bm.Behind)
	assert.Equal(t, "behind", bm.State.String())
}

func TestParseStatus_Diverged(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -3\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Equal(t, "diverged", bm.State.String())
}

func TestParseStatus_AheadOnly(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n# branch.ab +4 -0\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Equal(t, 4, bm.Ahead)
	assert.Equal(t, "ahead", bm.State.String())
}

func TestParseStatus_NoUpstream(t *testing.T) {
	input := "# branch.head main\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Empty(t, bm.Remote)
	assert.Equal(t, "local", bm.State.String())
}

func TestParseStatus_CleanWorkingTree(t *testing.T) {
	input := "# branch.head main\n# branch.upstream origin/main\n# branch.ab +0 -0\n"
	st := parseStatus(input, nil)
	assert.False(t, st.Dirty)
}

func TestParseStatus_WithConflict(t *testing.T) {
	input := "# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +0 -0\n" +
		"u UU N... 100644 100644 100644 abc def file.txt\n"

	st := parseStatus(input, nil)
	assert.True(t, st.Dirty)
	assert.True(t, st.Conflict, "porcelain v2 'u' lines should mark conflict")
}

func TestParseStatus_ConflictAndChanges(t *testing.T) {
	input := "# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +0 -0\n" +
		"u UU N... 100644 100644 100644 abc def conflicted.txt\n" +
		"1 M. N... 100644 100644 100644 abc modified.txt\n"
	st := parseStatus(input, nil)
	assert.True(t, st.Dirty)
	assert.True(t, st.Conflict)
}

func TestParseStatus_SlashInRemoteName(t *testing.T) {
	input := "# branch.head main\n# branch.upstream upstream/team/main\n# branch.ab +0 -0\n"
	remotes := []string{"origin", "upstream/team"}
	st := parseStatus(input, remotes)
	bm := st.Bookmarks[0]
	assert.Equal(t, "upstream/team", bm.Remote, "should match longest known remote prefix")
}

func TestParseStatus_SlashInRemoteName_FirstMatchFallback(t *testing.T) {
	// When both "upstream" and "upstream/team" are remotes,
	// the longest prefix ("upstream/team") must win.
	input := "# branch.head main\n# branch.upstream upstream/team/main\n# branch.ab +0 -0\n"
	remotes := []string{"upstream", "upstream/team"}
	st := parseStatus(input, remotes)
	bm := st.Bookmarks[0]
	assert.Equal(
		t,
		"upstream/team",
		bm.Remote,
		"should match longest remote even if shorter one appears first",
	)
}

func TestParseStatus_SlashInRemoteName_NoRemotesProvided(t *testing.T) {
	// When no known remotes are provided, fall back to first-segment split.
	input := "# branch.head main\n# branch.upstream upstream/team/main\n# branch.ab +0 -0\n"
	st := parseStatus(input, nil)
	bm := st.Bookmarks[0]
	assert.Equal(t, "upstream", bm.Remote, "without remotes, falls back to first-segment split")
}

func TestBackend_Name(t *testing.T) {
	b := &Backend{}
	assert.Equal(t, "git", b.Name())
}

func TestBackend_Detect_WithGitDir(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750)
	require.NoError(t, err)

	b := &Backend{}
	ok, err := b.Detect(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBackend_Detect_WithoutGitDir(t *testing.T) {
	dir := t.TempDir()

	b := &Backend{}
	ok, err := b.Detect(dir)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestBackend_Detect_ErrorOnPath(t *testing.T) {
	b := &Backend{}
	ok, err := b.Detect("\x00invalid")
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestBackend_Run_Interactive(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})
	runGitCmd(t, dir, []string{"config", "user.email", "test@test.com"})
	runGitCmd(t, dir, []string{"config", "user.name", "Test"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGitCmd(t, dir, []string{"add", "test.txt"})
	runGitCmd(t, dir, []string{"commit", "-m", "initial"})

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"rev-parse", "HEAD"}, true)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
}

func TestBackend_Run_InteractiveNonZero(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"log"}, true)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestRegister(t *testing.T) {
	assert.NotPanics(t, func() {
		Register()
	})
}

func TestBackend_Status(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})
	runGitCmd(t, dir, []string{"config", "user.email", "test@test.com"})
	runGitCmd(t, dir, []string{"config", "user.name", "Test"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGitCmd(t, dir, []string{"add", "test.txt"})
	runGitCmd(t, dir, []string{"commit", "-m", "initial"})

	b := &Backend{}
	st, err := b.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.NotEmpty(t, st.Ref)
}

func TestBackend_Status_Dirty(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})
	runGitCmd(t, dir, []string{"config", "user.email", "test@test.com"})
	runGitCmd(t, dir, []string{"config", "user.name", "Test"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGitCmd(t, dir, []string{"add", "test.txt"})
	runGitCmd(t, dir, []string{"commit", "-m", "initial"})

	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("modified"), 0o644))

	b := &Backend{}
	st, err := b.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, st.Dirty)
}

func TestBackend_Status_NoRepo(t *testing.T) {
	dir := t.TempDir()
	b := &Backend{}
	_, err := b.Status(context.Background(), dir)
	assert.Error(t, err)
}

func TestBackend_Run(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})
	runGitCmd(t, dir, []string{"config", "user.email", "test@test.com"})
	runGitCmd(t, dir, []string{"config", "user.name", "Test"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGitCmd(t, dir, []string{"add", "test.txt"})
	runGitCmd(t, dir, []string{"commit", "-m", "initial"})

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"rev-parse", "HEAD"}, false)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output)
}

func TestBackend_Run_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"log", "--nonexistent-flag"}, false)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestBackend_Run_OutputCapture(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})
	runGitCmd(t, dir, []string{"config", "user.email", "test@test.com"})
	runGitCmd(t, dir, []string{"config", "user.name", "Test"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGitCmd(t, dir, []string{"add", "test.txt"})
	runGitCmd(t, dir, []string{"commit", "-m", "initial"})

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"log", "--oneline", "-1"}, false)
	require.NoError(t, err)
	assert.Contains(t, res.Output, "initial")
}

func TestKnownRemotes_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	remotes := knownRemotes(context.Background(), dir)
	assert.Nil(t, remotes, "knownRemotes in non-git dir should return nil")
}

func TestBackend_Run_NonExecutablePath(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})

	t.Setenv("PATH", "")

	b := &Backend{}
	_, err := b.Run(context.Background(), dir, []string{"status"}, false)
	assert.Error(t, err)
}

func runGitCmd(t *testing.T, dir string, args []string) {
	t.Helper()

	_, err := runGit(context.Background(), dir, args)
	require.NoError(t, err)
}
