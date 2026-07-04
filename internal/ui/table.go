package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/muesli/reflow/truncate"
)

func RenderTable(header []string, rows [][]string, widths []int) string {
	var buf strings.Builder

	writeHeader(&buf, header, widths)
	buf.WriteByte('\n')

	for _, row := range rows {
		writeRow(&buf, row, widths)
		buf.WriteByte('\n')
	}

	return buf.String()
}

func RenderHeader(cells []string, widths []int) string {
	var buf strings.Builder
	writeHeader(&buf, cells, widths)

	return buf.String()
}

func RenderRow(cells []string, widths []int) string {
	var buf strings.Builder
	writeRow(&buf, cells, widths)

	return buf.String()
}

func EffectiveWidths(header []string, rows [][]string, maxWidths []int) []int {
	eff := make([]int, len(maxWidths))

	for i := range maxWidths {
		w := lipgloss.Width(header[i])

		for _, row := range rows {
			w = max(w, lipgloss.Width(row[i]))
		}

		eff[i] = min(w, maxWidths[i])
		eff[i] = max(eff[i], 1)
	}

	return eff
}

//nolint:gochecknoglobals // package-level style, consistent with ui.go's Style helpers
var headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipglossColor("cyan"))

func writeHeader(buf *strings.Builder, cells []string, widths []int) {
	writeCells(buf, cells, widths, func(cell string) string {
		return headerStyle.Render(cell)
	})
}

func writeRow(buf *strings.Builder, cells []string, widths []int) {
	writeCells(buf, cells, widths, nil)
}

// writeCells pads/truncates each cell to its column width and writes it to
// buf, space-separated. style, if non-nil, wraps the padded cell (e.g. in
// ANSI codes) before it's written.
func writeCells(buf *strings.Builder, cells []string, widths []int, style func(string) string) {
	for i, cell := range cells {
		if lipgloss.Width(cell) > widths[i] {
			cell = truncate.String(cell, uint(widths[i]))
		}

		cell += strings.Repeat(" ", widths[i]-lipgloss.Width(cell))

		if style != nil {
			cell = style(cell)
		}

		if i > 0 {
			buf.WriteByte(' ')
		}

		buf.WriteString(cell)
	}
}
