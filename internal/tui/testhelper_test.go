package tui

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestModel(tb testing.TB, ctx context.Context, opts Options) (*model, error) {
	tb.Helper()

	if opts.StatePath == "" {
		opts.StatePath = filepath.Join(tb.TempDir(), "state.json")
	}

	return newModel(ctx, opts)
}
