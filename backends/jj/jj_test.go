package jj

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWorkingCopy_Empty(t *testing.T) {
	st := parseWorkingCopy("")
	assert.Empty(t, st.Ref)
	assert.False(t, st.Dirty)
	assert.False(t, st.Conflict)
}

func TestParseWorkingCopy_ChangeIDOnly(t *testing.T) {
	input := "rlkvwrto\n"
	st := parseWorkingCopy(input)
	assert.Equal(t, "rlkvwrto", st.Ref)
}

func TestParseWorkingCopy_WithDirty(t *testing.T) {
	input := "rlkvwrto\x1fdirty\x1f\x1fmsg\x1ftime\n"
	st := parseWorkingCopy(input)
	assert.Equal(t, "rlkvwrto", st.Ref)
	assert.True(t, st.Dirty)
}

func TestParseWorkingCopy_WithConflict(t *testing.T) {
	input := "rlkvwrto\x1f\x1fconflict\x1fmsg\x1ftime\n"
	st := parseWorkingCopy(input)
	assert.True(t, st.Conflict)
}

func TestParseWorkingCopy_FullFields(t *testing.T) {
	input := "rlkvwrto\x1fdirty\x1fconflict\x1fmy commit message\x1f3 days ago\n"
	st := parseWorkingCopy(input)
	assert.Equal(t, "rlkvwrto", st.Ref)
	assert.True(t, st.Dirty)
	assert.True(t, st.Conflict)
	assert.Equal(t, "my commit message", st.CommitMsg)
	assert.Equal(t, "(3 days ago)", st.CommitTime)
}

func TestParseWorkingCopy_PartialFields(t *testing.T) {
	input := "rlkvwrto\x1fdirty\n"
	st := parseWorkingCopy(input)
	assert.Equal(t, "rlkvwrto", st.Ref)
	assert.True(t, st.Dirty)
}

func TestParseWorkingCopy_CommitTimeFormatting(t *testing.T) {
	input := "rlkvwrto\x1f\x1f\x1f\x1f3 days ago\n"
	st := parseWorkingCopy(input)
	assert.Equal(t, "(3 days ago)", st.CommitTime)
}

func TestParseWorkingCopy_NoFields(t *testing.T) {
	input := "\x1f\x1f\x1f\x1f\n"
	st := parseWorkingCopy(input)
	assert.Empty(t, st.Ref)
	assert.False(t, st.Dirty)
	assert.False(t, st.Conflict)
}

func TestParseBookmarks_Empty(t *testing.T) {
	result := parseBookmarks("")
	assert.Nil(t, result)
}

func TestParseBookmarks_NoRemote(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  (no tracking remote)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
	assert.Empty(t, result[0].Remote)
	assert.Equal(t, "local", result[0].State.String())
}

func TestParseBookmarks_WithRemoteSynced(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (tracking)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 0, result[0].Ahead)
	assert.Equal(t, 0, result[0].Behind)
	assert.Equal(t, "synced", result[0].State.String())
}

func TestParseBookmarks_WithRemoteAhead(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (ahead by 3 commits)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 3, result[0].Ahead)
	assert.Equal(t, 0, result[0].Behind)
	assert.Equal(t, "ahead", result[0].State.String())
}

func TestParseBookmarks_WithRemoteBehind(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (behind by 2 commits)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, 0, result[0].Ahead)
	assert.Equal(t, 2, result[0].Behind)
	assert.Equal(t, "behind", result[0].State.String())
}

func TestParseBookmarks_WithRemoteDiverged(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (ahead by 2 commits, behind by 1 commit)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, 2, result[0].Ahead)
	assert.Equal(t, 1, result[0].Behind)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarks_Gone(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (gone)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, "gone", result[0].State.String())
}

func TestParseBookmarks_Conflicted(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c (conflicted)\n  @origin (tracking)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.True(t, result[0].Conflict)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarks_ConflictedNoRemote(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c (conflicted)\n  (no tracking remote)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.True(t, result[0].Conflict)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarks_MultipleBookmarks(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c\n  @origin (tracking)\nfeature: qpvuntop 1a2b3c4d\n  @origin (ahead by 1 commits)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 2)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "feature", result[1].Name)
}

func TestParseBookmarks_MultipleBookmarksDifferentStates(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c\n  @origin (tracking)\n" +
		"feature: qpvuntop 1a2b3c4d\n  @origin (gone)\n" +
		"old: zzzzzzzz 00000000\n  (no tracking remote)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 3)
	assert.Equal(t, "synced", result[0].State.String())
	assert.Equal(t, "gone", result[1].State.String())
	assert.Equal(t, "local", result[2].State.String())
}

func TestParseBookmarks_SkipAtInName(t *testing.T) {
	input := "main@something: rlkvwrto 9f3a1b2c\n  @origin (tracking)\n"
	result := parseBookmarks(input)
	assert.Empty(t, result)
}

func TestParseBookmarks_FirstRemoteOnly(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c\n  @origin (tracking)\n  @upstream (tracking)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
}

func TestParseBookmarks_NoTrackingRemote(t *testing.T) {
	input := "main: rlkvwrto 9f3a1b2c commit message\n  (no tracking remote)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].Remote)
	assert.Equal(t, "local", result[0].State.String())
}

func TestParseBookmarks_EmptyBookmarkName(t *testing.T) {
	input := ": rlkvwrto 9f3a1b2c\n  @origin (tracking)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].Name)
}

func TestParseBookmarks_BlankLines(t *testing.T) {
	input := "\nmain: rlkvwrto 9f3a1b2c\n\n  @origin (tracking)\n\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
}

func TestExtractCount(t *testing.T) {
	tests := []struct {
		s        string
		keyword  string
		expected int
	}{
		{"ahead by 2 commits", "ahead by", 2},
		{"behind by 1 commit", "behind by", 1},
		{"no match here", "ahead by", 0},
		{"", "ahead by", 0},
		{"ahead by 10 commits", "ahead by", 10},
		{"behind by 100 commits", "behind by", 100},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractCount(tt.s, tt.keyword))
		})
	}
}

func TestExtractCommitMsg(t *testing.T) {
	assert.Equal(t, "msg", extractCommitMsg("msg"))
	assert.Empty(t, extractCommitMsg(""))
}

func TestExtractCommitTime(t *testing.T) {
	assert.Equal(t, "time", extractCommitTime("msg\x1ftime"))
	assert.Empty(t, extractCommitTime("msg"))
	assert.Empty(t, extractCommitTime(""))
}

func TestBackend_Name_JJ(t *testing.T) {
	b := &Backend{}
	assert.Equal(t, "jj", b.Name())
}

func TestBackend_Detect_WithJJDir(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)

	b := &Backend{}
	ok, err := b.Detect(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBackend_Detect_WithoutJJDir(t *testing.T) {
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
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, ".jj", "repo"), []byte("."), 0o644)
	require.NoError(t, err)

	b := &Backend{}
	_, err = b.Run(context.Background(), dir, []string{"log", "-r:", "-n1"}, true)
	assert.NoError(t, err)
}

func TestBackend_Run_InteractiveNonZero(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, ".jj", "repo"), []byte("."), 0o644)
	require.NoError(t, err)

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"nonexistent-command"}, true)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestRegister_JJ(t *testing.T) {
	assert.NotPanics(t, func() {
		Register()
	})
}

func TestBackend_Status(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("jj", "init", "--git")
	cmd.Dir = dir
	_ = cmd.Run()

	if _, err := os.Stat(filepath.Join(dir, ".jj")); err != nil {
		t.Skip("jj init --git did not create a .jj directory, skipping")
	}

	b := &Backend{}
	st, err := b.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.NotEmpty(t, st.Ref)
}

func TestBackend_Status_AncestorWithDescription(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("jj", "init", "--git")
	cmd.Dir = dir
	_ = cmd.Run()

	if _, err := os.Stat(filepath.Join(dir, ".jj")); err != nil {
		t.Skip("jj init --git did not create a .jj directory, skipping")
	}

	setup := func(args ...string) {
		c := exec.Command("jj", args...)
		c.Dir = dir
		_ = c.Run()
	}
	setup("describe", "-m", "feat: initial")
	setup("bookmark", "set", "main")
	setup("new")

	b := &Backend{}
	st, err := b.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "feat: initial", st.CommitMsg)
	assert.Len(t, st.Bookmarks, 1)
	assert.Equal(t, "main", st.Bookmarks[0].Name)
}

func TestBackend_Status_AncestorWalkError(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("jj", "init", "--git")
	cmd.Dir = dir
	_ = cmd.Run()

	if _, err := os.Stat(filepath.Join(dir, ".jj")); err != nil {
		t.Skip("jj init --git did not create a .jj directory, skipping")
	}

	// Make the working copy have no description so the ancestor walk runs,
	// then cause runJJ to fail on ancestor revs.
	origRunJJ := runJJ

	runJJ = func(ctx context.Context, _ string, args []string) (string, error) {
		// Let the working copy query (@) succeed, fail on ancestor queries (@- etc).
		if slices.Contains(args, "@") {
			return origRunJJ(ctx, dir, args)
		}

		return "", assert.AnError
	}
	defer func() { runJJ = origRunJJ }()

	b := &Backend{}
	st, err := b.Status(context.Background(), dir)
	// The ancestor walk should not hard-error — it breaks on first failure
	// and returns whatever the working copy gave us.
	require.NoError(t, err)
	assert.NotEmpty(t, st.Ref)
}

func TestBackend_Status_NotAJJRepo(t *testing.T) {
	dir := t.TempDir()
	b := &Backend{}
	// When jj is not available or dir is not a jj repo, it may return empty output
	st, _ := b.Status(context.Background(), dir)
	// Should get empty or error status
	_ = st
}

func TestBackend_Run(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, ".jj", "repo"), []byte("."), 0o644)
	require.NoError(t, err)

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"log", "-r:", "-n1"}, false)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output)
}

func TestBackend_Run_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, ".jj", "repo"), []byte("."), 0o644)
	require.NoError(t, err)

	b := &Backend{}
	res, err := b.Run(context.Background(), dir, []string{"nonexistent"}, false)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestBackend_Run_NoExecutable(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)

	b := &Backend{}

	t.Setenv("PATH", "")

	_, err = b.Run(context.Background(), dir, []string{"log"}, false)
	assert.Error(t, err)
}

func TestRunJJ_Failure(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)

	_, err = runJJ(
		context.Background(),
		dir,
		[]string{"log", "-r", "@", "--template", "invalid_template"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jj log")
}

func TestBackend_Status_JjLogFailure(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o755)
	require.NoError(t, err)

	b := &Backend{}

	t.Setenv("PATH", "")

	_, err = b.Status(context.Background(), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jj log")
}
