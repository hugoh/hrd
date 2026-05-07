package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// execInteractive runs bin with args in dir, inheriting the current terminal.
// This is used for commands like `jj log` or `git difftool` that require
// a real TTY and user interaction.
func execInteractive(ctx context.Context, dir, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin comes from trusted config
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", bin, err)
	}

	return nil
}
