package jj

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jjSampleCompletion() string {
	return `
_jj() {
    local i cur prev opts cmd
    COMPREPLY=()
    case "${cmd},${i}" in
        ",$1")
            cmd="jj"
            ;;
        jj,abandon)
            cmd="jj__subcmd__abandon"
            ;;
        jj,bookmark)
            cmd="jj__subcmd__bookmark"
            ;;
        jj,commit)
            cmd="jj__subcmd__commit"
            ;;
        jj,describe)
            cmd="jj__subcmd__describe"
            ;;
        jj,diff)
            cmd="jj__subcmd__diff"
            ;;
        jj,git)
            cmd="jj__subcmd__git"
            ;;
        jj,log)
            cmd="jj__subcmd__log"
            ;;
        jj,new)
            cmd="jj__subcmd__new"
            ;;
        jj,rebase)
            cmd="jj__subcmd__rebase"
            ;;
        jj,status)
            cmd="jj__subcmd__status"
            ;;
        jj,b)
            cmd="jj__subcmd__bookmark__b"
            ;;
    esac
}
`
}

// setupJJDir creates a temp directory with a minimal .jj structure.
func setupJJDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o750)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, ".jj", "repo"), []byte("."), 0o644)
	require.NoError(t, err)

	return dir
}

// initJJRepo initializes a real jj repository and skips the test if jj is not available.
func initJJRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("jj", "git", "init")
	cmd.Dir = dir
	_ = cmd.Run()

	if _, err := os.Stat(filepath.Join(dir, ".jj")); err != nil {
		t.Skipf("jj git init did not create a .jj directory, skipping")
	}

	return dir
}

func TestBackend_Priority(t *testing.T) {
	b := &Backend{}
	assert.Positive(t, b.Priority())
}

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
	// "ahead by 3" means remote is 3 ahead → local is 3 behind.
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (ahead by 3 commits)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 0, result[0].Ahead)
	assert.Equal(t, 3, result[0].Behind)
	assert.Equal(t, "behind", result[0].State.String())
}

func TestParseBookmarks_WithRemoteBehind(t *testing.T) {
	// "behind by 2" means remote is 2 behind → local is 2 ahead.
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (behind by 2 commits)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, 2, result[0].Ahead)
	assert.Equal(t, 0, result[0].Behind)
	assert.Equal(t, "ahead", result[0].State.String())
}

func TestParseBookmarks_WithRemoteDiverged(t *testing.T) {
	// "ahead by 2, behind by 1" means remote ahead by 2 (local behind 2)
	// and remote behind by 1 (local ahead 1).
	input := "main: rlkvwrto 9f3a1b2c commit message\n  @origin (ahead by 2 commits, behind by 1 commit)\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].Ahead)
	assert.Equal(t, 2, result[0].Behind)
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

func TestParseBookmarks_SkipGitRemote(t *testing.T) {
	// Colocated jj+git repos have a synthetic @git remote that always
	// appears before @origin in the output. We must skip @git and
	// use @origin as the true remote.
	input := "main: lolyspwl 5d874311 lint fix\n" +
		"  @git: lolyspwl 5d874311 lint fix\n" +
		"  @origin (ahead by 1 commits, behind by 1 commits): lolyspwl/1 6f42490f (hidden) lint fix\n"
	result := parseBookmarks(input)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 1, result[0].Ahead)
	assert.Equal(t, 1, result[0].Behind)
	assert.Equal(t, "diverged", result[0].State.String())
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

func TestBackend_SubcommandArgs_JJ(t *testing.T) {
	b := &Backend{}

	tests := []struct {
		op   string
		want []string
	}{
		{"status", []string{"status"}},
		{"fetch", []string{"git", "fetch"}},
		{"push", []string{"git", "push"}},
		{"pull", []string{"pull"}},
		{"log", []string{"log"}},
		{"diff", []string{"diff"}},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			assert.Equal(t, tt.want, b.SubcommandArgs(tt.op))
		})
	}
}

func TestBackend_Detect(t *testing.T) {
	t.Run("with jj dir", func(t *testing.T) {
		dir := t.TempDir()
		err := os.MkdirAll(filepath.Join(dir, ".jj"), 0o750)
		require.NoError(t, err)

		b := &Backend{}
		ok, err := b.Detect(dir)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("without jj dir", func(t *testing.T) {
		dir := t.TempDir()

		b := &Backend{}
		ok, err := b.Detect(dir)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("error on invalid path", func(t *testing.T) {
		b := &Backend{}
		ok, err := b.Detect("\x00invalid")
		assert.False(t, ok)
		assert.Error(t, err)
	})
}

func TestBackend_Detect_NonColocated(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("jj", "git", "init", "--no-colocate")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	if _, err := os.Stat(filepath.Join(dir, ".jj")); err != nil {
		t.Skipf("jj git init --no-colocate did not create a .jj directory: %s", string(out))
	}

	t.Run("jj detects non-colocated repo", func(t *testing.T) {
		b := &Backend{}
		ok, err := b.Detect(dir)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("git does not detect non-colocated repo", func(t *testing.T) {
		b := &git.Backend{}
		ok, err := b.Detect(dir)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	_, err := os.Stat(filepath.Join(dir, ".git"))
	assert.True(t, os.IsNotExist(err), "non-colocated jj repo should not have a .git directory")
}

func TestBackend_Run_Interactive(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{}
	_, err := b.Run(t.Context(), dir, []string{"log", "-r:", "-n1"}, true)
	assert.NoError(t, err)
}

func TestBackend_Run_InteractiveNonZero(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"nonexistent-command"}, true)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestRegister_JJ(t *testing.T) {
	assert.NotPanics(t, func() {
		Register()
	})
}

func TestBackend_Status(t *testing.T) {
	dir := initJJRepo(t)

	b := &Backend{}
	st, err := b.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.NotEmpty(t, st.Ref)
}

func TestBackend_Status_AncestorWithDescription(t *testing.T) {
	dir := initJJRepo(t)

	setup := func(args ...string) {
		c := exec.Command("jj", args...)
		c.Dir = dir
		_ = c.Run()
	}
	setup("describe", "-m", "feat: initial")
	setup("bookmark", "set", "main")
	setup("new")

	b := &Backend{}
	st, err := b.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, "feat: initial", st.CommitMsg)
	assert.Len(t, st.Bookmarks, 1)
	assert.Equal(t, "main", st.Bookmarks[0].Name)
}

func TestBackend_Status_AncestorWalkError(t *testing.T) {
	dir := initJJRepo(t)

	// Make the working copy have no description so the ancestor walk runs,
	// then cause runJJ to fail on ancestor revs.
	b := &Backend{}
	b.runJJFn = func(ctx context.Context, _ string, args []string) (string, error) {
		// Let the working copy query (@) succeed, fail on ancestor queries (@- etc).
		if slices.Contains(args, "@") {
			return defaultRunJJ(ctx, dir, args)
		}

		return "", assert.AnError
	}
	st, err := b.Status(t.Context(), dir)
	// The ancestor walk should not hard-error — it breaks on first failure
	// and returns whatever the working copy gave us.
	require.NoError(t, err)
	assert.NotEmpty(t, st.Ref)
}

func TestEnrichWithRemoteBookmark_Found(t *testing.T) {
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
		switch {
		case slices.Contains(args, "bookmark") && slices.Contains(args, "list"):
			return "main: sxoqvoon 2c688398 (empty) Merge pull request #15\n" +
				"  @git: sxoqvoon 2c688398 (empty) Merge pull request #15\n" +
				"main@origin: opxqzwyo e67b1a90 (empty) Merge pull request #17\n", nil
		case slices.Contains(args, "main..main@origin") && slices.Contains(args, "--count"):
			return "1", nil
		case slices.Contains(args, "main@origin..main") && slices.Contains(args, "--count"):
			return "0", nil
		default:
			return "", nil
		}
	}

	bm := &backend.BookmarkStatus{Name: "main", State: backend.RefStateNoRemote}
	b.enrichWithRemoteBookmark(t.Context(), "/tmp", "main", bm)

	assert.Equal(t, "origin", bm.Remote)
	assert.Equal(t, 0, bm.Ahead)
	assert.Equal(t, 1, bm.Behind)
	assert.Equal(t, "behind", bm.State.String())
}

func TestEnrichWithRemoteBookmark_NotFound(t *testing.T) {
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
		if slices.Contains(args, "bookmark") && slices.Contains(args, "list") {
			return "main: sxoqvoon 2c688398\n  @git: sxoqvoon 2c688398\n", nil
		}

		return "", nil
	}

	bm := &backend.BookmarkStatus{Name: "main", State: backend.RefStateNoRemote}
	b.enrichWithRemoteBookmark(t.Context(), "/tmp", "main", bm)

	assert.Empty(t, bm.Remote)
	assert.Equal(t, backend.RefStateNoRemote, bm.State)
}

func TestEnrichWithRemoteBookmark_SkipGit(t *testing.T) {
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
		if slices.Contains(args, "bookmark") && slices.Contains(args, "list") {
			return "main: sxoqvoon 2c688398\n" +
				"  @git: sxoqvoon 2c688398\n" +
				"main@git: sxoqvoon 2c688398\n", nil
		}

		return "", nil
	}

	bm := &backend.BookmarkStatus{Name: "main", State: backend.RefStateNoRemote}
	b.enrichWithRemoteBookmark(t.Context(), "/tmp", "main", bm)

	assert.Empty(t, bm.Remote)
	assert.Equal(t, backend.RefStateNoRemote, bm.State)
}

func TestEnrichWithRemoteBookmark_FetchError(t *testing.T) {
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, _ []string) (string, error) {
		return "", assert.AnError
	}

	bm := &backend.BookmarkStatus{Name: "main", State: backend.RefStateNoRemote}
	b.enrichWithRemoteBookmark(t.Context(), "/tmp", "main", bm)

	assert.Empty(t, bm.Remote)
}

func TestCountRevs(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   int
	}{
		{"no commits", "0", nil, 0},
		{"one commit", "1", nil, 1},
		{"two commits", "2", nil, 2},
		{"empty string error path", "", nil, 0},
		{"non-numeric output", "abc", nil, 0},
		{"runJJ error", "", assert.AnError, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Backend{}
			b.runJJFn = func(_ context.Context, _ string, _ []string) (string, error) {
				return tt.output, tt.err
			}

			assert.Equal(t, tt.want, b.countRevs(t.Context(), "/tmp", "main..main@origin"))
		})
	}
}

func TestBackend_Status_NotAJJRepo(t *testing.T) {
	dir := t.TempDir()
	b := &Backend{}
	// When jj is not available or dir is not a jj repo, it may return empty output
	st, _ := b.Status(t.Context(), dir)
	// Should get empty or error status
	_ = st
}

func TestBackend_Run(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"log", "-r:", "-n1"}, false)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output)
}

func TestBackend_Run_NonZeroExit(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"nonexistent"}, false)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestBackend_Run_NoExecutable(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{}

	t.Setenv("PATH", "")

	_, err := b.Run(t.Context(), dir, []string{"log"}, false)
	assert.Error(t, err)
}

func TestRunJJ_Failure(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{runJJFn: defaultRunJJ}
	_, err := b.runJJ(
		t.Context(),
		dir,
		[]string{"log", "-r", "@", "--template", "invalid_template"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jj log")
}

func TestBackend_Status_JjLogFailure(t *testing.T) {
	dir := setupJJDir(t)

	b := &Backend{}

	t.Setenv("PATH", "")

	_, err := b.Status(t.Context(), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jj log")
}

//nolint:cyclop,funlen // table-driven test with 3 cases
func TestBackend_Status_LocalAhead(t *testing.T) {
	tests := []struct {
		name      string
		wcOutput  string
		wantAhead int
		wantDirty bool
		wantMsg   string
	}{
		{
			name:      "described",
			wcOutput:  "rlkvwrto\x1f\x1f\x1ffeat: initial\x1f2 hours ago\n",
			wantAhead: 2,
			wantMsg:   "feat: initial",
		},
		{
			name:      "undescribed",
			wcOutput:  "rlkvwrto\x1f\x1f\x1f\x1f2 hours ago\n",
			wantAhead: 2,
		},
		{
			name:      "undescribed dirty",
			wcOutput:  "rlkvwrto\x1fdirty\x1f\x1f\x1f2 hours ago\n",
			wantAhead: 2,
			wantDirty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Backend{}
			b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
				if slices.Contains(args, "@") && slices.Contains(args, "--template") {
					return tt.wcOutput, nil
				}

				if slices.Contains(args, "bookmarks.first().name()") {
					return "main\n", nil
				}

				if slices.Contains(args, "bookmark") && slices.Contains(args, "list") {
					return "main: rlkvwrto ...\n  (no tracking remote)\n", nil
				}

				if (slices.Contains(args, "main..@") || slices.Contains(args, "..@")) &&
					slices.Contains(args, "--count") {
					return "3", nil
				}

				return "", nil
			}

			st, err := b.Status(t.Context(), "/tmp")
			require.NoError(t, err)

			assert.Equal(t, tt.wantAhead, st.LocalAhead)
			assert.Equal(t, "main", st.Bookmarks[0].Name)
			assert.Equal(t, tt.wantDirty, st.Dirty)

			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, st.CommitMsg)
			}
		})
	}
}

func TestMultiStepOps_Table(t *testing.T) {
	steps, ok := multiStepOps["pull"]
	require.True(t, ok, "pull must be a multi-step op")
	require.Len(t, steps, 2, "pull must have 2 steps")

	assert.Equal(t, []string{"git", "fetch"}, steps[0])
	assert.Equal(t, []string{"rebase", "-d", "trunk()"}, steps[1])
}

func TestBackend_Run_Pull(t *testing.T) {
	dir := initJJRepo(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"pull"}, false)
	require.NoError(t, err)

	// In a fresh repo there's no remote, so fetch fails with exit 1.
	// That's fine — the multi-step path was taken and returned the failure.
	assert.NotZero(t, res.ExitCode, "fetch should fail in repo with no remote")
	assert.Contains(t, res.Output, "No git remotes", "fetch error should mention no remotes")
}

func TestBackend_Run_Pull_FirstStepFails(t *testing.T) {
	dir := initJJRepo(t)
	b := &Backend{}

	orig := multiStepOps["pull"]
	multiStepOps["pull"] = [][]string{{"nonexistent-command"}, {"log"}}

	t.Cleanup(func() { multiStepOps["pull"] = orig })

	res, err := b.Run(t.Context(), dir, []string{"pull"}, false)
	require.NoError(t, err)
	assert.NotZero(t, res.ExitCode, "should fail on first step and not attempt step 2")
	assert.Contains(t, res.Output, "error", "output should contain error from nonexistent command")
}

func TestRunSteps_AllSucceed(t *testing.T) {
	dir := initJJRepo(t)

	res, err := runSteps(t.Context(), dir, "test",
		[][]string{{"log", "-r", "@", "-n1", "--no-graph", "--color=never"}}, false)

	require.NoError(t, err)
	assert.Zero(t, res.ExitCode)
}

func TestRunSteps_AccumulatesOutput(t *testing.T) {
	dir := initJJRepo(t)

	logStep := []string{"log", "-r", "@", "-n1", "--no-graph", "--color=never",
		"--template", `"step-output\n"`}

	res, err := runSteps(t.Context(), dir, "test",
		[][]string{logStep, logStep}, false)

	require.NoError(t, err)
	assert.Zero(t, res.ExitCode)
	assert.Equal(t, "step-output\nstep-output\n", res.Output,
		"output from every step should be returned, not dropped")
}

func TestRunSteps_InfraError(t *testing.T) {
	dir := initJJRepo(t)

	t.Setenv("PATH", "")

	_, err := runSteps(t.Context(), dir, "test",
		[][]string{{"log"}}, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jj test")
}

func TestBackend_Subcommands(t *testing.T) {
	b := &Backend{}

	cmds, err := b.Subcommands(t.Context())
	if err != nil {
		t.Skipf("jj not available: %v", err)
	}

	require.NotEmpty(t, cmds)

	expect := []string{"status", "log", "diff", "new", "describe", "commit", "git"}

	for _, want := range expect {
		assert.Contains(t, cmds, want)
	}
}

func TestBackend_Subcommands_Error(t *testing.T) {
	b := &Backend{}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := b.Subcommands(ctx)
	assert.Error(t, err)
}

func TestParseJjCmdList(t *testing.T) {
	got := parseJjCmdList(jjSampleCompletion())
	want := []string{
		"abandon", "b", "bookmark", "commit",
		"describe", "diff", "git", "log",
		"new", "rebase", "status",
	}

	require.Len(t, got, len(want))
	assert.Equal(t, want, got)
}

func TestParseJjCmdListAliases(t *testing.T) {
	sample := `
        jj,status)
                cmd="jj__subcmd__status"
        jj,st)
                cmd="jj__subcmd__status__st"
        jj,diff)
                cmd="jj__subcmd__diff"
        jj,d)
                cmd="jj__subcmd__diff__d"
`

	got := parseJjCmdList(sample)
	assert.Len(t, got, 4)
}

func TestParseJjCmdListEmpty(t *testing.T) {
	assert.Empty(t, parseJjCmdList(""))
}

func TestParseJjCmdListNoEntries(t *testing.T) {
	assert.Empty(t, parseJjCmdList("some random bash code\nno jj entries here\n"))
}

func TestParseJjCmdListSkipsEmptySub(t *testing.T) {
	got := parseJjCmdList("        jj,)\n        jj,status)\n")
	assert.Equal(t, []string{"status"}, got)
}

func TestParseJjCmdListDeduplicates(t *testing.T) {
	input := `        jj,status)
                cmd="x"
        jj,status)
                cmd="y"
        jj,log)
                cmd="z"
`

	got := parseJjCmdList(input)
	assert.Len(t, got, 2)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	assert.Panics(t, func() { Register() })
}

func TestParseJjCmdListSorted(t *testing.T) {
	got := parseJjCmdList(`
        jj,status)
        jj,log)
        jj,abandon)
`)
	assert.True(t, slices.IsSorted(got))
}
