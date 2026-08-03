package backend

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractExitCode_NilError(t *testing.T) {
	code, ok := ExtractExitCode(nil)
	assert.False(t, ok)
	assert.Equal(t, 0, code)
}

func TestExtractExitCode_GenericError(t *testing.T) {
	code, ok := ExtractExitCode(assert.AnError)
	assert.False(t, ok)
	assert.Equal(t, 0, code)
}

func TestExtractExitCode_ExitError(t *testing.T) {
	err := exec.CommandContext(t.Context(), "sh", "-c", "exit 42").Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)

	code, ok := ExtractExitCode(err)
	assert.True(t, ok)
	assert.Equal(t, 42, code)
}

func TestRunCommand_Success(t *testing.T) {
	res, err := RunCommand(t.Context(), "echo", "", []string{"hello"}, false)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Contains(t, res.Output, "hello")
}

func TestRunCommand_ExitError(t *testing.T) {
	res, err := RunCommand(t.Context(), "sh", "", []string{"-c", "exit 7"}, false)
	require.NoError(t, err) // ExitError is unwrapped, non-zero exit is not an error
	assert.Equal(t, 7, res.ExitCode)
}

func TestRunCommand_BinaryNotFound(t *testing.T) {
	_, err := RunCommand(t.Context(), "nonexistent-binary-12345", "", nil, false)
	require.Error(t, err)
}

func TestRunCommand_Interactive(t *testing.T) {
	res, err := RunCommand(t.Context(), "echo", "", []string{"ignored"}, true)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	// In interactive mode, output is not captured.
	assert.Empty(t, res.Output)
}

func TestRunCommand_PWDMatchesDir(t *testing.T) {
	dir := t.TempDir()

	res, err := RunCommand(t.Context(), "sh", dir, []string{"-c", "echo $PWD"}, false)
	require.NoError(t, err)
	assert.Equal(t, dir, strings.TrimSpace(res.Output))
}

func TestRunCommand_EmptyPathKeepsRealPWD(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	res, err := RunCommand(t.Context(), "sh", "", []string{"-c", "echo $PWD"}, false)
	require.NoError(t, err)
	assert.Equal(t, wd, strings.TrimSpace(res.Output))
}

func TestRunCommand_NonInteractiveDisablesGitPrompting(t *testing.T) {
	res, err := RunCommand(
		t.Context(),
		"sh",
		"",
		[]string{"-c", "echo $GIT_TERMINAL_PROMPT"},
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, "0", strings.TrimSpace(res.Output))
}

func TestRunCommand_NonInteractiveSetsSSHBatchMode(t *testing.T) {
	res, err := RunCommand(t.Context(), "sh", "", []string{"-c", "echo $GIT_SSH_COMMAND"}, false)
	require.NoError(t, err)
	assert.Contains(t, res.Output, "-oBatchMode=yes")
}

func TestRunCommand_NonInteractivePreservesExistingSSHCommand(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "ssh -F /custom/config")

	res, err := RunCommand(t.Context(), "sh", "", []string{"-c", "echo $GIT_SSH_COMMAND"}, false)
	require.NoError(t, err)
	assert.Contains(t, res.Output, "/custom/config")
	assert.Contains(t, res.Output, "-oBatchMode=yes")
}

func TestRunCommand_InteractiveDoesNotSetPromptEnv(t *testing.T) {
	t.Setenv("GIT_TERMINAL_PROMPT", "")

	// Interactive mode captures no output, so assert indirectly: run a
	// script that fails if GIT_TERMINAL_PROMPT got forced to "0".
	res, err := RunCommand(
		t.Context(),
		"sh",
		"",
		[]string{"-c", `test "$GIT_TERMINAL_PROMPT" != "0"`},
		true,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
}

func TestRunCommand_NonInteractiveTimesOut(t *testing.T) {
	_, err := RunCommand(
		t.Context(),
		"sh",
		"",
		[]string{"-c", "sleep 60"},
		false,
		WithTimeout(50*time.Millisecond),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRunTool_NoArgs(t *testing.T) {
	_, err := RunTool(t.Context(), "echo", "", nil, false)
	assert.ErrorIs(t, err, ErrNoArgs)
}

func TestRunTool_Success(t *testing.T) {
	res, err := RunTool(t.Context(), "echo", "", []string{"hello"}, false)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Contains(t, res.Output, "hello")
}

func TestRunTool_BinaryNotFound(t *testing.T) {
	_, err := RunTool(t.Context(), "nonexistent-binary-12345", "", []string{"arg"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-binary-12345 arg")
}

func TestDetectDir_MarkerExists(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".testmarker"), 0o750)
	require.NoError(t, err)

	ok, err := DetectDir(dir, ".testmarker")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDetectDir_MarkerMissing(t *testing.T) {
	dir := t.TempDir()

	ok, err := DetectDir(dir, ".nonexistent")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDetectDir_PathDoesNotExist(t *testing.T) {
	ok, err := DetectDir("/nonexistent/path/12345", ".git")
	require.NoError(t, err)
	assert.False(t, ok)
}
