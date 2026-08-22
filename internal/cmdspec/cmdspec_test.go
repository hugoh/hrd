package cmdspec_test

import (
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/internal/cmdspec"
	"github.com/hugoh/hrd/internal/config"
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

func TestExpandAliasForBackend(t *testing.T) {
	aliases := map[string]config.AliasSpec{
		"sync": {Command: "pull --rebase"},
		"mkc":  {Command: "!make clean"},
		"up": {Backends: map[string]string{
			"git": "!git up",
			"jj":  "!jj up",
		}},
	}

	for _, tt := range []struct {
		name       string
		input      string
		backend    string
		want       string
		wantExpand bool
	}{
		{name: "bare alias", input: "sync", backend: "git", want: "pull --rebase", wantExpand: true},
		{
			name: "alias with trailing args", input: "sync --autostash", backend: "git",
			want: "pull --rebase --autostash", wantExpand: true,
		},
		{name: "shell alias", input: "mkc", backend: "git", want: "!make clean", wantExpand: true},
		{name: "not an alias", input: "status", backend: "git", want: "status", wantExpand: true},
		{name: "alias only matches first token", input: "log sync", backend: "git", want: "log sync", wantExpand: true},
		{name: "per-backend alias resolves git variant", input: "up", backend: "git", want: "!git up", wantExpand: true},
		{name: "per-backend alias resolves jj variant", input: "up", backend: "jj", want: "!jj up", wantExpand: true},
		{name: "per-backend alias with no variant for backend", input: "up", backend: "hg", wantExpand: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cmdspec.ExpandAliasForBackend(aliases, tt.input, tt.backend)
			assert.Equal(t, tt.wantExpand, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
