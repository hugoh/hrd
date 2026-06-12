package cmdspec_test

import (
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/internal/cmdspec"
	"github.com/stretchr/testify/assert"
)

//nolint:gochecknoinits // backends must be registered for prefix parsing
func init() {
	git.Register()
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name       string
		input      string
		wantPrefix string
		wantCmd    string
	}{
		{name: "bang shell", input: "!make clean", wantPrefix: "sh", wantCmd: "make clean"},
		{name: "sh prefix", input: "sh make clean", wantPrefix: "sh", wantCmd: "make clean"},
		{name: "shell prefix", input: "shell make clean", wantPrefix: "sh", wantCmd: "make clean"},
		{name: "backend prefix", input: "git push --force", wantPrefix: "git", wantCmd: "push --force"},
		{name: "bare subcommand", input: "pull --rebase", wantPrefix: "", wantCmd: "pull --rebase"},
		{name: "plain word", input: "status", wantPrefix: "", wantCmd: "status"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			prefix, cmd := cmdspec.Parse(tt.input)
			assert.Equal(t, tt.wantPrefix, prefix)
			assert.Equal(t, tt.wantCmd, cmd)
		})
	}
}

func TestExpandAlias(t *testing.T) {
	aliases := map[string]string{
		"sync": "pull --rebase",
		"mkc":  "!make clean",
	}

	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "bare alias", input: "sync", want: "pull --rebase"},
		{name: "alias with trailing args", input: "sync --autostash", want: "pull --rebase --autostash"},
		{name: "shell alias", input: "mkc", want: "!make clean"},
		{name: "not an alias", input: "status", want: "status"},
		{name: "alias only matches first token", input: "log sync", want: "log sync"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cmdspec.ExpandAlias(aliases, tt.input))
		})
	}
}
