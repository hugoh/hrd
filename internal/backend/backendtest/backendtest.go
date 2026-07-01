// Package backendtest holds shared assertions for backend.Backend
// implementations, so each concrete backend's test suite doesn't
// re-implement the same interface-level checks.
package backendtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const markerDirPerm = 0o750

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
