// Package backendtest holds shared assertions for backend.Backend
// implementations, so each concrete backend's test suite doesn't
// re-implement the same interface-level checks.
package backendtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const markerDirPerm = 0o750

// integrationRequired reports whether a missing external binary (or a failed
// repo init) must fail the test instead of skipping it. CI sets
// REQUIRE_INTEGRATION=1 to guarantee the integration tests actually run.
func integrationRequired() bool {
	return os.Getenv("REQUIRE_INTEGRATION") == "1"
}

// RequireExternalBinary skips the test when name is not on PATH, unless
// REQUIRE_INTEGRATION=1, in which case the absence is a hard failure.
func RequireExternalBinary(t *testing.T, name string) {
	t.Helper()

	if _, err := exec.LookPath(name); err != nil {
		msg := name + " not found in PATH"
		if integrationRequired() {
			t.Fatalf("%s but REQUIRE_INTEGRATION=1", msg)
		}

		t.Skip(msg)
	}
}

// RequireToolRepo asserts that dir is a working repo of the expected kind by
// checking for marker (e.g. ".jj", ".git"). A missing marker skips the test,
// unless REQUIRE_INTEGRATION=1, where it is a hard failure. detail is
// surfaced in the message (e.g. combined output of the failed init command).
func RequireToolRepo(t *testing.T, tool, dir, marker, detail string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
		msg := fmt.Sprintf("%s init did not create %s: %s", tool, marker, detail)
		if integrationRequired() {
			t.Fatal(msg)
		}

		t.Skip(msg)
	}
}

// RequireIsolatedGit skips the test if git isn't on PATH, then points HOME
// and GIT_CONFIG_GLOBAL at throwaway locations so a real `git` invocation in
// the test never reads (or writes) the developer's actual git config.
func RequireIsolatedGit(t *testing.T) {
	t.Helper()

	RequireExternalBinary(t, "git")

	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
}

// AssertDetect exercises the common Backend.Detect contract: true when the
// marker directory (e.g. ".git", ".jj") is present, false when it's absent,
// and an error on an invalid path. newBackend must return a fresh Backend
// for each call.
func AssertDetect(t *testing.T, newBackend func() backend.Backend, marker string) {
	t.Helper()

	t.Run("with "+marker+" dir", func(t *testing.T) {
		dir := t.TempDir()
		err := os.MkdirAll(filepath.Join(dir, marker), markerDirPerm)
		require.NoError(t, err)

		ok, err := newBackend().Detect(dir)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("without "+marker+" dir", func(t *testing.T) {
		dir := t.TempDir()

		ok, err := newBackend().Detect(dir)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("error on invalid path", func(t *testing.T) {
		ok, err := newBackend().Detect("\x00invalid")
		assert.False(t, ok)
		assert.Error(t, err)
	})
}

// AssertRunNoArgs checks that Backend.Run rejects an empty args slice with
// backend.ErrNoArgs.
func AssertRunNoArgs(t *testing.T, b backend.Backend) {
	t.Helper()

	dir := t.TempDir()

	_, err := b.Run(t.Context(), dir, nil, false)
	assert.ErrorIs(t, err, backend.ErrNoArgs)
}

// AssertSubcommandsErrorsOnCanceledContext checks that Backend.Subcommands
// returns an error when given an already-canceled context.
func AssertSubcommandsErrorsOnCanceledContext(t *testing.T, b backend.Backend) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := b.Subcommands(ctx)
	assert.Error(t, err)
}
