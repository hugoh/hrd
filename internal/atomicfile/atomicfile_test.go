package atomicfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/atomicfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileCreatesFileWithContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")

	require.NoError(t, atomicfile.WriteFile(path, []byte("hello"), 0o600))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestWriteFileReplacesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")

	require.NoError(t, atomicfile.WriteFile(path, []byte("first"), 0o600))
	require.NoError(t, atomicfile.WriteFile(path, []byte("second"), 0o600))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second", string(got))
}

func TestWriteFileErrorsOnMissingParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-subdir", "out.txt")

	err := atomicfile.WriteFile(path, []byte("data"), 0o600)
	assert.Error(t, err, "WriteFile does not create parent directories")
}
