package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
