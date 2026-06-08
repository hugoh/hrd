package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ExtractExitCode returns the exit code from an exec.ExitError.
// Returns ok=false if err is nil or not an ExitError.
func ExtractExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}

	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode(), true
	}

	return 0, false
}

// RunCommand executes a binary with args in the given directory.
// When interactive, it passes through stdin/stdout/stderr to the terminal.
// Otherwise it captures combined stdout+stderr into the returned RunResult.
// ExitError is unwrapped: the exit code is set on the result and the error is
// cleared (non-zero exit is not considered an infrastructure failure).
func RunCommand(
	ctx context.Context,
	binary string,
	path string,
	args []string,
	interactive bool,
) (RunResult, error) {
	//nolint:gosec // controlled command execution, args from user input
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = path

	var buf bytes.Buffer

	if interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdin = nil
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}

	exitCode := 0

	err := cmd.Run()
	if ec, ok := ExtractExitCode(err); ok {
		exitCode = ec
		err = nil
	}

	if interactive {
		return RunResult{ExitCode: exitCode}, err
	}

	return RunResult{Output: buf.String(), ExitCode: exitCode}, err
}

// DetectDir returns true if the directory at path contains a directory with
// the given marker name (e.g. ".git" or ".jj").
func DetectDir(path, marker string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, marker))
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("stating %s: %w", marker, err)
	}

	return true, nil
}
