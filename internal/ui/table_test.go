package ui_test

import (
	"strings"
	"testing"

	"github.com/hugoh/hrd/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestRenderTable(t *testing.T) {
	header := []string{"NAME", "VCS", "REF"}
	rows := [][]string{
		{"myproject", "git", "main ✓"},
		{"dotfiles", "jj", "main"},
	}
	widths := []int{15, 3, 20}

	out := ui.RenderTable(header, rows, widths)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[1], "myproject")
	assert.Contains(t, lines[2], "dotfiles")
	assert.Contains(t, lines[1], "git")
	assert.Contains(t, lines[2], "jj")
}

func TestRenderTable_EmptyRows(t *testing.T) {
	header := []string{"NAME"}
	widths := []int{10}

	out := ui.RenderTable(header, nil, widths)
	assert.Contains(t, out, "NAME")
	assert.NotContains(t, out, "\n\n")
}

func TestRenderTable_TruncatesWideContent(t *testing.T) {
	header := []string{"N"}
	rows := [][]string{{"hello world"}}
	widths := []int{5}

	out := ui.RenderTable(header, rows, widths)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 2)
	assert.Equal(t, "hello", lines[1])
}

func TestRenderRow(t *testing.T) {
	cells := []string{"myproject", "git", "main ✓"}
	widths := []int{15, 3, 20}

	out := ui.RenderRow(cells, widths)
	assert.Contains(t, out, "myproject")
	assert.Contains(t, out, "git")
}

func TestRenderRow_Truncates(t *testing.T) {
	cells := []string{"hello world", "a"}
	widths := []int{5, 1}

	out := ui.RenderRow(cells, widths)
	assert.Equal(t, "hello a", out)
}

func TestRenderHeader(t *testing.T) {
	cells := []string{"NAME", "VCS"}
	widths := []int{10, 5}

	out := ui.RenderHeader(cells, widths)
	assert.Equal(t, "\x1b[1;96mNAME      \x1b[0m \x1b[1;96mVCS  \x1b[0m", out)
}

func TestRenderHeader_Truncates(t *testing.T) {
	cells := []string{"LONG HEADER"}
	widths := []int{5}

	out := ui.RenderHeader(cells, widths)
	assert.Equal(t, "\x1b[1;96mLONG \x1b[0m", out)
}

func TestRenderTable_TruncatesHeader(t *testing.T) {
	header := []string{"VERY LONG HEADER NAME"}
	rows := [][]string{{"short"}}
	widths := []int{5}

	out := ui.RenderTable(header, rows, widths)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "VERY")
	assert.NotContains(t, lines[0], "HEADER")
}

func TestEffectiveWidths(t *testing.T) {
	header := []string{"A", "B"}
	rows := [][]string{{"hello", "x"}, {"world", "y"}}
	maxWidths := []int{10, 10}

	eff := ui.EffectiveWidths(header, rows, maxWidths)
	assert.Equal(t, []int{5, 1}, eff)
}

func TestEffectiveWidths_CappedAtMax(t *testing.T) {
	header := []string{"A"}
	rows := [][]string{{strings.Repeat("x", 50)}}
	maxWidths := []int{10}

	eff := ui.EffectiveWidths(header, rows, maxWidths)
	assert.Equal(t, []int{10}, eff)
}

func TestEffectiveWidths_MinimumOne(t *testing.T) {
	header := []string{""}
	rows := [][]string{{""}}
	maxWidths := []int{10}

	eff := ui.EffectiveWidths(header, rows, maxWidths)
	assert.Equal(t, []int{1}, eff)
}
