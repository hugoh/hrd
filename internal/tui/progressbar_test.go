package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/require"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name  string
		model *model
		want  *tea.ProgressBar
	}{
		{name: "idle", model: &model{}, want: nil},
		{
			name: "loading statuses",
			model: &model{
				statusTotal: 4,
				pending:     map[string]bool{"a": true, "b": true, "c": true},
			},
			want: tea.NewProgressBar(tea.ProgressBarDefault, 25),
		},
		{
			name: "loading statuses with an error",
			model: &model{
				statusTotal:  4,
				statusAnyErr: true,
				pending:      map[string]bool{"a": true, "b": true, "c": true},
			},
			want: tea.NewProgressBar(tea.ProgressBarError, 25),
		},
		{
			name: "executing beats a concurrent status refresh",
			model: &model{
				executing: true,
				execTotal: 4,
				execResults: []execResult{
					{name: "a"},
					{name: "b", result: runner.Result{ExitCode: 1}},
				},
				statusTotal: 4,
				pending:     map[string]bool{"a": true},
			},
			want: tea.NewProgressBar(tea.ProgressBarError, 50),
		},
		{
			name:  "finished exec clears the bar",
			model: &model{execTotal: 2, execResults: []execResult{{name: "a"}, {name: "b"}}},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.model.progressBar())
		})
	}
}

func TestViewCarriesProgressBar(t *testing.T) {
	m := testModel(func(m *model) {
		m.statusTotal = 2
		m.pending = map[string]bool{"a": true}
	})

	require.Equal(t, tea.NewProgressBar(tea.ProgressBarDefault, 50), m.View().ProgressBar)
}
