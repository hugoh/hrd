package ui_test

import (
	"io"
	"os"
	"testing"

	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDispatchResult_Success(t *testing.T) {
	res := runner.Result{RepoName: "tmhi-cli", ExitCode: 0}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "tmhi-cli")
	assert.Contains(t, out, "✓")
}

func TestRenderDispatchResult_Error(t *testing.T) {
	res := runner.Result{RepoName: "bad", Err: assert.AnError}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "bad")
	assert.Contains(t, out, "✗")
}

func TestRenderDispatchResult_NonZeroExit(t *testing.T) {
	res := runner.Result{RepoName: "fail", ExitCode: 1}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "fail")
	assert.Contains(t, out, "✗")
	assert.Contains(t, out, "exit 1")
}

func TestColorSprint(t *testing.T) {
	out := ui.ColorSprint(text.Colors{text.FgGreen}, "hello")
	assert.Contains(t, out, "hello")
}

func TestTableStyle(t *testing.T) {
	s := ui.TableStyle()
	assert.NotNil(t, s)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hel", ui.Truncate("hello world", 3))
	assert.Equal(t, "hello world", ui.Truncate("hello world", 99))
}

func TestWrap(t *testing.T) {
	wrapped := ui.Wrap("hello world", 5)
	assert.Contains(t, wrapped, "hello")
}

func TestComputeRemainderWidth(t *testing.T) {
	// termWidth=100, minWidth=10, separators=1, used=[20, 30]
	// total used = 50, separator width = 2 * 1 = 2
	// remainder = 100 - 50 - 2*2 = 46
	w := ui.ComputeRemainderWidth(100, 10, 20, 30)
	assert.Equal(t, 46, w)

	// When remainder < min, should return min
	w2 := ui.ComputeRemainderWidth(10, 20, 20, 30)
	assert.Equal(t, 20, w2)
}

func TestNewTable(t *testing.T) {
	tbl := ui.NewTable()
	assert.NotNil(t, tbl)
}

func TestGetTermWidth(t *testing.T) {
	w := ui.GetTermWidth()
	assert.Positive(t, w)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w

	fn()

	_ = w.Close()

	os.Stdout = old

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(out)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = w

	fn()

	_ = w.Close()

	os.Stderr = old

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(out)
}

func TestOutf(t *testing.T) {
	out := captureStdout(t, func() {
		ui.Outf("test %d", 42)
	})
	assert.Contains(t, out, "test 42")
}

func TestErrf(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Errf("error %s", "msg")
	})
	assert.Contains(t, out, "error msg")
}

func TestSuccess(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Success("done %s", "ok")
	})
	assert.Contains(t, out, "done ok")
}

func TestWarn(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Warn("warning %d", 1)
	})
	assert.Contains(t, out, "warning 1")
}

func TestInfo(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Info("info %s", "test")
	})
	assert.Contains(t, out, "info test")
}

func TestFail(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Fail("fail %s", "err")
	})
	assert.Contains(t, out, "fail err")
}
