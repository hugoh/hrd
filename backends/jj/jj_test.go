package jj

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/backend/backendtest"
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

// isolateJJConfig points JJ_CONFIG at a fresh per-test config file with a
// fixed user identity, so tests don't depend on (or corrupt) the
// developer's/CI runner's ambient jj config.
func isolateJJConfig(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj not found in PATH")
	}

	cfgPath := filepath.Join(t.TempDir(), "jj-config.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"[user]\nname = \"Test\"\nemail = \"test@test.com\"\n",
	), 0o644))
	t.Setenv("JJ_CONFIG", cfgPath)
}

// initJJRepo initializes a real jj repository and skips the test if jj is not available.
func initJJRepo(t *testing.T) string {
	t.Helper()
	isolateJJConfig(t)

	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "jj", "git", "init")
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
	st, err := parseWorkingCopy("")
	require.Error(t, err)
	assert.Empty(t, st.Ref)
	assert.False(t, st.Dirty)
	assert.False(t, st.Conflict)
}

func TestParseWorkingCopy_Malformed(t *testing.T) {
	st, err := parseWorkingCopy("not json")
	require.Error(t, err)
	assert.Empty(t, st.Ref)
	assert.False(t, st.Dirty)
	assert.False(t, st.Conflict)
}

func TestParseWorkingCopy_ChangeIDOnly(t *testing.T) {
	input := `{"changeId":"rlkvwrto","dirty":false,"conflict":false,"description":"","ago":""}`
	st, err := parseWorkingCopy(input)
	require.NoError(t, err)
	assert.Equal(t, "rlkvwrto", st.Ref)
}

func TestParseWorkingCopy_WithDirty(t *testing.T) {
	input := `{"changeId":"rlkvwrto","dirty":true,"conflict":false,"description":"msg","ago":"time"}`
	st, err := parseWorkingCopy(input)
	require.NoError(t, err)
	assert.Equal(t, "rlkvwrto", st.Ref)
	assert.True(t, st.Dirty)
}

func TestParseWorkingCopy_WithConflict(t *testing.T) {
	input := `{"changeId":"rlkvwrto","dirty":false,"conflict":true,"description":"msg","ago":"time"}`
	st, err := parseWorkingCopy(input)
	require.NoError(t, err)
	assert.True(t, st.Conflict)
}

func TestParseWorkingCopy_FullFields(t *testing.T) {
	input := `{"changeId":"rlkvwrto","dirty":true,"conflict":true,` +
		`"description":"my commit message","ago":"3 days ago"}`
	st, err := parseWorkingCopy(input)
	require.NoError(t, err)
	assert.Equal(t, "rlkvwrto", st.Ref)
	assert.True(t, st.Dirty)
	assert.True(t, st.Conflict)
	assert.Equal(t, "my commit message", st.CommitMsg)
	assert.Equal(t, "(3 days ago)", st.CommitTime)
}

func TestParseWorkingCopy_CommitTimeFormatting(t *testing.T) {
	input := `{"changeId":"rlkvwrto","dirty":false,"conflict":false,"description":"","ago":"3 days ago"}`
	st, err := parseWorkingCopy(input)
	require.NoError(t, err)
	assert.Equal(t, "(3 days ago)", st.CommitTime)
}

func TestParseWorkingCopy_NoFields(t *testing.T) {
	st, err := parseWorkingCopy("{}")
	require.NoError(t, err)
	assert.Empty(t, st.Ref)
	assert.False(t, st.Dirty)
	assert.False(t, st.Conflict)
}

func bookmarkRefJSON(
	name, remote string,
	tracked, present, conflict bool,
	ahead, behind int,
) string {
	b, err := json.Marshal(bookmarkRef{
		Name: name, Remote: remote, Tracked: tracked, Present: present,
		Conflict: conflict, Ahead: ahead, Behind: behind,
	})
	if err != nil {
		panic(err)
	}

	return string(b)
}

func TestParseBookmarkRefs_Empty(t *testing.T) {
	result := parseBookmarkRefs("", nil)
	assert.Nil(t, result)
}

func TestParseBookmarkRefs_NoRemote(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
	assert.Empty(t, result[0].Remote)
	assert.Equal(t, "local", result[0].State.String())
}

func TestParseBookmarkRefs_WithRemoteSynced(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 0, result[0].Ahead)
	assert.Equal(t, 0, result[0].Behind)
	assert.Equal(t, "synced", result[0].State.String())
}

func TestParseBookmarkRefs_WithRemoteAhead(t *testing.T) {
	// Local 3 behind remote.
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 3) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 0, result[0].Ahead)
	assert.Equal(t, 3, result[0].Behind)
	assert.Equal(t, "behind", result[0].State.String())
}

func TestParseBookmarkRefs_WithRemoteBehind(t *testing.T) {
	// Local 2 ahead of remote.
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 2, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, 2, result[0].Ahead)
	assert.Equal(t, 0, result[0].Behind)
	assert.Equal(t, "ahead", result[0].State.String())
}

func TestParseBookmarkRefs_WithRemoteDiverged(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 1, 2) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].Ahead)
	assert.Equal(t, 2, result[0].Behind)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarkRefs_UntrackedRemoteResolvesAheadBehind(t *testing.T) {
	// jj only computes tracking_ahead_count/tracking_behind_count for
	// tracked remotes, so an untracked-but-present remote always carries
	// ahead=0/behind=0 in the JSON itself. resolveUntracked must be
	// consulted instead of trusting those zeroed fields.
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", false, true, false, 0, 0) + "\n"

	var gotName, gotRemote string

	result := parseBookmarkRefs(input, func(name, remote string) (int, int) {
		gotName, gotRemote = name, remote

		return 2, 5
	})

	require.Len(t, result, 1)
	assert.Equal(t, "main", gotName)
	assert.Equal(t, "origin", gotRemote)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 2, result[0].Ahead)
	assert.Equal(t, 5, result[0].Behind)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarkRefs_UntrackedRemoteNilResolver(t *testing.T) {
	// A nil resolver (e.g. from a bare parseBookmarkRefs(raw, nil) call)
	// must not panic, and simply leaves ahead/behind at zero.
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", false, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, 0, result[0].Ahead)
	assert.Equal(t, 0, result[0].Behind)
	assert.Equal(t, "synced", result[0].State.String())
}

func TestParseBookmarkRefs_Gone(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, false, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, "gone", result[0].State.String())
}

func TestParseBookmarkRefs_ConflictedAndGoneKeepsDiverged(t *testing.T) {
	// A bookmark that is both locally conflicted and whose tracked remote
	// has been deleted must report "diverged" (the more specific signal),
	// not have it clobbered by the "gone" remote state.
	input := bookmarkRefJSON("main", "", false, true, true, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, false, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.True(t, result[0].Conflict)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarkRefs_Conflicted(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, true, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.True(t, result[0].Conflict)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarkRefs_ConflictedNoRemote(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, true, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.True(t, result[0].Conflict)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarkRefs_MultipleBookmarks(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("feature", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("feature", "origin", true, true, false, 0, 1) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 2)
	assert.Equal(t, "main", result[0].Name)
	assert.Equal(t, "feature", result[1].Name)
}

func TestParseBookmarkRefs_MultipleBookmarksDifferentStates(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("feature", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("feature", "origin", true, false, false, 0, 0) + "\n" +
		bookmarkRefJSON("old", "", false, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 3)
	assert.Equal(t, "synced", result[0].State.String())
	assert.Equal(t, "gone", result[1].State.String())
	assert.Equal(t, "local", result[2].State.String())
}

func TestParseBookmarkRefs_FirstRemoteOnly(t *testing.T) {
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "upstream", true, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
}

func TestParseBookmarkRefs_EmptyBookmarkName(t *testing.T) {
	input := bookmarkRefJSON("", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("", "origin", true, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].Name)
}

func TestParseBookmarkRefs_SkipGitRemote(t *testing.T) {
	// Colocated jj+git repos have a synthetic @git remote that always
	// appears before @origin in the output. We must skip @git and
	// use @origin as the true remote.
	input := bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "git", true, true, false, 0, 0) + "\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 1, 1) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "origin", result[0].Remote)
	assert.Equal(t, 1, result[0].Ahead)
	assert.Equal(t, 1, result[0].Behind)
	assert.Equal(t, "diverged", result[0].State.String())
}

func TestParseBookmarkRefs_BlankLines(t *testing.T) {
	input := "\n" + bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n\n" +
		bookmarkRefJSON("main", "origin", true, true, false, 0, 0) + "\n\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
}

func TestParseBookmarkRefs_MalformedLine(t *testing.T) {
	input := "not json\n" + bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n"
	result := parseBookmarkRefs(input, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "main", result[0].Name)
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
	backendtest.AssertDetect(t, func() backend.Backend { return &Backend{} }, ".jj")
}

func TestBackend_Detect_NonColocated(t *testing.T) {
	isolateJJConfig(t)
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "jj", "git", "init", "--no-colocate")
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
		c := exec.CommandContext(t.Context(), "jj", args...)
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
	assert.NotEmpty(
		t,
		st.CommitTime,
		"ancestor CommitTime must be filled in the same '(...)' form as the working copy",
	)
	assert.Len(t, st.Bookmarks, 1)
	assert.Equal(t, "main", st.Bookmarks[0].Name)
}

func TestBackend_Status_AncestorCommitTimeFormat(t *testing.T) {
	// fillCommitMsgFromAncestors reuses parseWorkingCopy to decode ancestor
	// output, so it must apply the same "(...)" CommitTime formatting.
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
		switch {
		case slices.Contains(args, "@") && slices.Contains(args, "--template"):
			return `{"changeId":"rlkvwrto","dirty":false,"conflict":false,"description":"","ago":""}`, nil
		case slices.Contains(args, "@-") && slices.Contains(args, "--template"):
			return `{"changeId":"qzkswnpm","dirty":false,"conflict":false,` +
				`"description":"feat: ancestor","ago":"5 minutes ago"}`, nil
		case slices.Contains(args, "bookmarks.first().name()"):
			return "", nil
		default:
			return "", nil
		}
	}

	st, err := b.Status(t.Context(), "/tmp")
	require.NoError(t, err)
	assert.Equal(t, "feat: ancestor", st.CommitMsg)
	assert.Equal(t, "(5 minutes ago)", st.CommitTime)
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

//nolint:cyclop // runJJFn dispatch switch covers several distinct jj subcommands
func TestBackend_Status_ColocatedRemoteFromSingleQuery(t *testing.T) {
	// In colocated jj/git repos, a single `bookmark list --all-remotes <name>`
	// call already returns the @git and @origin entries together — no second
	// query is needed to discover the real remote. @origin is untracked here
	// (the common colocated-repo case), so jj itself reports ahead=0/behind=0
	// for it and Status must fall back to countRevs to get the real counts.
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
		switch {
		case slices.Contains(args, "@") && slices.Contains(args, "--template"):
			return `{"changeId":"sxoqvoon","dirty":false,"conflict":false,"description":"msg","ago":"now"}`, nil
		case slices.Contains(args, "bookmarks.first().name()"):
			return "main\n", nil
		case slices.Contains(args, "bookmark") && slices.Contains(args, "list"):
			return bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n" +
				bookmarkRefJSON("main", "git", true, true, false, 0, 0) + "\n" +
				bookmarkRefJSON("main", "origin", false, true, false, 0, 0) + "\n", nil
		case slices.Contains(args, "main@origin..main") && slices.Contains(args, "--count"):
			return "0", nil
		case slices.Contains(args, "main..main@origin") && slices.Contains(args, "--count"):
			return "1", nil
		default:
			return "0", nil
		}
	}

	st, err := b.Status(t.Context(), "/tmp")
	require.NoError(t, err)
	require.Len(t, st.Bookmarks, 1)
	assert.Equal(t, "origin", st.Bookmarks[0].Remote)
	assert.Equal(t, 0, st.Bookmarks[0].Ahead)
	assert.Equal(t, 1, st.Bookmarks[0].Behind)
	assert.Equal(t, "behind", st.Bookmarks[0].State.String())
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

func TestBackend_Run_NoArgs(t *testing.T) {
	backendtest.AssertRunNoArgs(t, &Backend{})
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

func TestBackend_Status_MalformedWorkingCopyOutput(t *testing.T) {
	// jj merges stdout and stderr (defaultRunJJ), so a stray warning line
	// can corrupt the JSON even though the process exits 0. Status must
	// surface this as an error, not silently return a zeroed status.
	b := &Backend{}
	b.runJJFn = func(_ context.Context, _ string, args []string) (string, error) {
		if slices.Contains(args, "@") && slices.Contains(args, "--template") {
			return "Warning: something jj printed to stderr\n{\"changeId\":\"abc\"}", nil
		}

		return "", nil
	}

	st, err := b.Status(t.Context(), "/tmp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jj log")
	assert.Empty(t, st.Ref)
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
			name: "described",
			wcOutput: `{"changeId":"rlkvwrto","dirty":false,"conflict":false,` +
				`"description":"feat: initial","ago":"2 hours ago"}`,
			wantAhead: 2,
			wantMsg:   "feat: initial",
		},
		{
			name: "undescribed",
			wcOutput: `{"changeId":"rlkvwrto","dirty":false,"conflict":false,` +
				`"description":"","ago":"2 hours ago"}`,
			wantAhead: 2,
		},
		{
			name: "undescribed dirty",
			wcOutput: `{"changeId":"rlkvwrto","dirty":true,"conflict":false,` +
				`"description":"","ago":"2 hours ago"}`,
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
					return bookmarkRefJSON("main", "", false, true, false, 0, 0) + "\n", nil
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

	logStep := []string{
		"log", "-r", "@", "-n1", "--no-graph", "--color=never",
		"--template", `"step-output\n"`,
	}

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
	backendtest.AssertSubcommandsErrorsOnCanceledContext(t, &Backend{})
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
