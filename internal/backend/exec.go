package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ErrNoArgs is returned by Backend.Run when called with no arguments.
var ErrNoArgs = errors.New("no arguments provided")

// defaultNonInteractiveTimeout bounds how long a non-interactive subprocess
// may run. It exists as defense-in-depth against a credential/host-key
// prompt that isn't caught by the GIT_TERMINAL_PROMPT/GIT_SSH_COMMAND
// guards below (e.g. a credential helper that ignores them), so a single
// stuck repo can't stall an entire bulk operation forever.
const defaultNonInteractiveTimeout = 2 * time.Minute

// commandOptions holds RunCommand tuning knobs, set via Option funcs.
type commandOptions struct {
	timeout time.Duration
}

// Option configures RunCommand behavior beyond its required parameters.
type Option func(*commandOptions)

// WithTimeout overrides the default non-interactive subprocess timeout.
// Ignored when interactive is true.
func WithTimeout(d time.Duration) Option {
	return func(o *commandOptions) {
		o.timeout = d
	}
}

// gitSSHCommandEnv returns the GIT_SSH_COMMAND value to use for
// non-interactive runs: ssh's own -oBatchMode=yes, which makes ssh fail
// immediately on a passphrase prompt or an unknown/changed host key
// instead of blocking on /dev/tty. Any ssh command/flags the user already
// configured are preserved by appending rather than overwriting, falling
// back to plain "ssh" if none is set.
func gitSSHCommandEnv() string {
	existing := os.Getenv("GIT_SSH_COMMAND")
	if existing == "" {
		existing = "ssh"
	}

	return existing + " -oBatchMode=yes"
}

// commandEnv builds the environment for cmd.Env: os.Environ() plus PWD
// tracking cmd.Dir (see below), plus, for non-interactive runs,
// GIT_TERMINAL_PROMPT=0 and a batch-mode GIT_SSH_COMMAND so git/ssh fail
// fast on a credential or host-key prompt instead of blocking on /dev/tty.
func commandEnv(path string, interactive bool) []string {
	env := os.Environ()
	if path != "" {
		// mise shims resolve the tool version from $PWD, not the process cwd, so it must track cmd.Dir.
		env = append(env, "PWD="+path)
	}

	if !interactive {
		env = append(env, "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND="+gitSSHCommandEnv())
	}

	return env
}

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
// When interactive, it passes through stdin/stdout/stderr to the terminal
// and the caller is responsible for letting the subprocess prompt (e.g.
// for credentials) directly on the terminal.
// Otherwise it captures combined stdout+stderr into the returned RunResult,
// and disables git/ssh credential and host-key prompting
// (GIT_TERMINAL_PROMPT=0, GIT_SSH_COMMAND with -oBatchMode=yes) so a repo
// needing auth fails fast instead of blocking on /dev/tty — which callers
// running many repos concurrently (or under a TUI that owns the terminal)
// could never satisfy anyway. A bounded timeout (defaultNonInteractiveTimeout,
// override via WithTimeout) backstops any prompt path these env vars don't
// cover.
// ExitError is unwrapped: the exit code is set on the result and the error is
// cleared (non-zero exit is not considered an infrastructure failure).
func RunCommand(
	ctx context.Context,
	binary string,
	path string,
	args []string,
	interactive bool,
	opts ...Option,
) (RunResult, error) {
	options := commandOptions{timeout: defaultNonInteractiveTimeout}
	for _, opt := range opts {
		opt(&options)
	}

	if !interactive {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}

	//nolint:gosec // controlled command execution, args from user input
	cmd := exec.CommandContext(ctx, binary, args...)

	cmd.Dir = path
	cmd.Env = commandEnv(path, interactive)

	if !interactive {
		// A subprocess that backgrounds a grandchild (e.g. a shell running
		// `cmd &`) can exit itself while that grandchild keeps the
		// inherited stdout/stderr pipe open. Wait() then blocks draining
		// that pipe for the grandchild's entire lifetime, regardless of
		// ctx cancellation — WaitDelay bounds that drain the same way the
		// context above bounds the process itself.
		cmd.WaitDelay = options.timeout
	}

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

	if ctxErr := ctx.Err(); ctxErr != nil {
		// The subprocess was killed by our own deadline (or an ancestor
		// context), not a plain non-zero exit: surface it as an
		// infrastructure failure rather than unwrapping an exit code.
		return RunResult{Output: buf.String()}, fmt.Errorf("%s: %w", binary, ctxErr)
	}

	if ec, ok := ExtractExitCode(err); ok {
		exitCode = ec
		err = nil
	}

	if interactive {
		return RunResult{ExitCode: exitCode}, err
	}

	return RunResult{Output: buf.String(), ExitCode: exitCode}, err
}

// RunTool validates args and runs binary via RunCommand, wrapping any
// infrastructure error with the binary name and failing subcommand. It
// implements the common case of Backend.Run: reject empty args, run, wrap
// errors. Backends with extra dispatch logic (e.g. multi-step ops) should
// perform that logic first and fall through to RunTool for the common path.
func RunTool(
	ctx context.Context,
	binary string,
	path string,
	args []string,
	interactive bool,
) (RunResult, error) {
	if len(args) == 0 {
		return RunResult{}, ErrNoArgs
	}

	res, err := RunCommand(ctx, binary, path, args, interactive)
	if err != nil {
		return RunResult{}, fmt.Errorf("%s %s: %w", binary, args[0], err)
	}

	return res, nil
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
