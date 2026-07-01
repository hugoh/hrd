package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func TestConfirm_NonTTYReturnsFalse(t *testing.T) {
	// os.Stdin under `go test` is never a TTY, so Confirm must short-circuit
	// to false without launching the bubbletea program.
	assert.False(t, Confirm("proceed?"))
}

func TestConfirmModel_Init(t *testing.T) {
	m := &confirmModel{prompt: "proceed?"}
	assert.Nil(t, m.Init())
}

func TestConfirmModel_View(t *testing.T) {
	m := &confirmModel{prompt: "proceed?"}
	assert.Equal(t, "proceed? [y/N]: ", m.View().Content)
}

func TestConfirmModel_Update(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		wantConfirmed bool
	}{
		{"lowercase y confirms", "y", true},
		{"uppercase Y confirms", "Y", true},
		{"other key declines", "n", false},
		{"enter declines", "enter", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &confirmModel{prompt: "proceed?"}

			updated, cmd := m.Update(tea.KeyPressMsg{Code: rune(tt.key[0]), Text: tt.key})

			resultModel, ok := updated.(*confirmModel)
			assert.True(t, ok)
			assert.Equal(t, tt.wantConfirmed, resultModel.confirmed)
			assert.NotNil(t, cmd)
		})
	}
}

func TestConfirmModel_Update_NonKeyMsg(t *testing.T) {
	m := &confirmModel{prompt: "proceed?"}

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	resultModel, ok := updated.(*confirmModel)
	assert.True(t, ok)
	assert.False(t, resultModel.confirmed)
	assert.Nil(t, cmd)
}
