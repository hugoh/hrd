// Package cmdspec parses the unified command grammar shared by the TUI
// command bar and config aliases: a backend prefix ("git ...", "jj ...")
// routes to that backend, a shell prefix ("!...", "sh ...", "shell ...")
// to the system shell, and anything else to per-repo VCS subcommand
// routing (each repo's own backend decides how to run it).
package cmdspec

import (
	"strings"

	"github.com/hugoh/hrd/internal/backend"
)

// PrefixShell is the prefix returned for shell commands.
const PrefixShell = "sh"

// Parse splits input into a routing prefix and the command remainder.
// The prefix is PrefixShell, a registered backend name, or "" for
// per-repo VCS subcommand routing.
func Parse(input string) (string, string) {
	if rest, ok := strings.CutPrefix(input, "!"); ok {
		return PrefixShell, strings.TrimSpace(rest)
	}

	for _, prefix := range []string{"sh ", "shell "} {
		if rest, ok := strings.CutPrefix(input, prefix); ok {
			return PrefixShell, strings.TrimSpace(rest)
		}
	}

	for _, name := range backend.Names() {
		if rest, ok := strings.CutPrefix(input, name+" "); ok {
			return name, strings.TrimSpace(rest)
		}
	}

	return "", input
}

// ExpandAlias replaces a leading alias token in input with its expansion,
// keeping the rest of the input as trailing text. Returns input unchanged
// when the first token is not a known alias.
func ExpandAlias(aliases map[string]string, input string) string {
	first, rest, _ := strings.Cut(strings.TrimSpace(input), " ")

	expansion, ok := aliases[first]
	if !ok {
		return input
	}

	if rest == "" {
		return expansion
	}

	return expansion + " " + rest
}
