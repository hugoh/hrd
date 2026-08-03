package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/backend/backendtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepoWithCommit creates a temp git repo with one commit and returns its path.
func initGitRepoWithCommit(t *testing.T) string {
	t.Helper()
	backendtest.RequireIsolatedGit(t)

	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})
	runGitCmd(t, dir, []string{"config", "user.email", "test@test.com"})
	runGitCmd(t, dir, []string{"config", "user.name", "Test"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGitCmd(t, dir, []string{"add", "test.txt"})
	runGitCmd(t, dir, []string{"commit", "-m", "initial"})

	return dir
}

func TestBackend_Priority(t *testing.T) {
	b := &Backend{}
	assert.Positive(t, b.Priority())
}

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

func TestBackend_SubcommandArgs(t *testing.T) {
	b := &Backend{}

	tests := []struct {
		op   string
		want []string
	}{
		{"status", []string{"status"}},
		{"fetch", []string{"fetch"}},
		{"push", []string{"push"}},
		{"pull", []string{"pull"}},
		{"log", []string{"log"}},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			assert.Equal(t, tt.want, b.SubcommandArgs(tt.op))
		})
	}
}

func TestBackend_Detect(t *testing.T) {
	backendtest.AssertDetect(t, func() backend.Backend { return &Backend{} }, ".git")
}

func TestBackend_Run_Interactive(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"rev-parse", "HEAD"}, true)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
}

func TestBackend_Subcommands(t *testing.T) {
	b := &Backend{}

	cmds, err := b.Subcommands(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, cmds)

	expect := []string{"add", "status", "log", "diff", "commit", "fetch", "pull", "push"}

	for _, want := range expect {
		assert.Contains(t, cmds, want)
	}
}

func TestBackend_Subcommands_Error(t *testing.T) {
	backendtest.AssertSubcommandsErrorsOnCanceledContext(t, &Backend{})
}

func TestParseGitCmdList(t *testing.T) {
	sample := `See 'git help <command>' to read about a specific subcommand

Main Porcelain Commands
   add                     Add file contents to the index
   branch                  List, create, or delete branches
   checkout                Switch branches or restore working tree files
   commit                  Record changes to the repository
   diff                    Show changes between commits, commit and working tree, etc
   fetch                   Download objects and refs from another repository
   log                     Show commit logs
   merge                   Join two or more development histories together
   pull                    Fetch from and integrate with another repository or a local branch
   push                    Update remote refs along with associated objects
   status                  Show the working tree status

Ancillary Commands / Manipulators
   config                  Get and set repository or global options
   help                    Display help information about Git
   mergetool               Run merge conflict resolution tools to resolve merge conflicts
`

	got := parseGitCmdList(sample)
	want := []string{
		"add", "branch", "checkout", "commit",
		"config", "diff", "fetch", "help",
		"log", "merge", "mergetool", "pull",
		"push", "status",
	}

	require.Len(t, got, len(want))
	assert.Equal(t, want, got)
}

func TestParseGitCmdListEmpty(t *testing.T) {
	assert.Empty(t, parseGitCmdList(""))
}

func TestParseGitCmdListNoSections(t *testing.T) {
	assert.Empty(t, parseGitCmdList("Just some text\nNo indented commands\n"))
}

func TestParseGitCmdListDeduplicates(t *testing.T) {
	got := parseGitCmdList("   status\n   log")
	require.Len(t, got, 2)
	assert.Equal(t, []string{"log", "status"}, got)
}

func TestParseGitCmdListTabIndent(t *testing.T) {
	got := parseGitCmdList("\tstatus		Show status\n\tlog		Show log")
	require.Len(t, got, 2)
}

func TestParseGitCmdListSorted(t *testing.T) {
	got := parseGitCmdList("   status\n   log\n   add\n   fetch")
	assert.True(t, slices.IsSorted(got))
}

func TestBackend_Run_InteractiveNonZero(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"log"}, true)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestRegister(t *testing.T) {
	backend.ClearRegisteredBackends()

	assert.NotPanics(t, func() {
		Register()
	})

	assert.Panics(t, func() {
		Register()
	})
}

func TestBackend_Status(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	b := &Backend{}
	st, err := b.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.NotEmpty(t, st.Ref)
}

func TestBackend_Status_Dirty(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("modified"), 0o644))

	b := &Backend{}
	st, err := b.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.True(t, st.Dirty)
}

func TestBackend_Status_NoRepo(t *testing.T) {
	dir := t.TempDir()
	b := &Backend{}
	_, err := b.Status(t.Context(), dir)
	assert.Error(t, err)
}

func TestBackend_Run(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"rev-parse", "HEAD"}, false)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output)
}

func TestBackend_Run_NoArgs(t *testing.T) {
	backendtest.AssertRunNoArgs(t, &Backend{})
}

func TestBackend_Run_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"log", "--nonexistent-flag"}, false)
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestBackend_Run_OutputCapture(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	b := &Backend{}
	res, err := b.Run(t.Context(), dir, []string{"log", "--oneline", "-1"}, false)
	require.NoError(t, err)
	assert.Contains(t, res.Output, "initial")
}

func TestKnownRemotes_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	remotes := knownRemotes(t.Context(), dir)
	assert.Nil(t, remotes, "knownRemotes in non-git dir should return nil")
}

func TestBackend_Run_NonExecutablePath(t *testing.T) {
	dir := t.TempDir()
	runGitCmd(t, dir, []string{"init"})

	t.Setenv("PATH", "")

	b := &Backend{}
	_, err := b.Run(t.Context(), dir, []string{"status"}, false)
	assert.Error(t, err)
}

func runGitCmd(t *testing.T, dir string, args []string) {
	t.Helper()

	_, err := runGit(t.Context(), dir, args)
	require.NoError(t, err)
}

// stubGitCountingCalls wraps the real git binary with a shim that appends
// one line per invocation to logPath before delegating, then prepends the
// shim's directory to PATH so exec.CommandContext("git", ...) resolves to it.
func stubGitCountingCalls(t *testing.T, logPath string) {
	t.Helper()

	realGit, err := exec.LookPath("git")
	require.NoError(t, err)

	binDir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\necho \"$*\" >> %q\nexec %q \"$@\"\n", logPath, realGit)
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBackend_Status_MergesCommitMsgAndTimeQuery(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	logPath := filepath.Join(t.TempDir(), "calls.log")
	stubGitCountingCalls(t, logPath)

	b := &Backend{}
	st, err := b.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, "initial", st.CommitMsg)
	assert.NotEmpty(t, st.CommitTime)

	calls, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := strings.Count(string(calls), "\n")
	assert.Equal(t, 3, lines,
		"Status should issue exactly 3 git calls (status, remote, log) — commit message and "+
			"commit time must come from a single combined log call, not two")
}

func TestBackend_Status_ParallelizesRemoteLookupWithStatus(t *testing.T) {
	const delay = 60 * time.Millisecond

	b := &Backend{}
	b.runGitFn = func(_ context.Context, _ string, args []string) (string, error) {
		time.Sleep(delay)

		switch {
		case len(args) > 0 && args[0] == subCmdStatus:
			return "# branch.head main\n", nil
		case len(args) > 0 && args[0] == "remote":
			return "origin\n", nil
		default:
			return "", nil
		}
	}

	start := time.Now()
	_, err := b.Status(t.Context(), "/tmp")
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 3*delay,
		"git status and the remote lookup should run concurrently, not sequentially")
}
