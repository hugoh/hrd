package tui

import (
	"context"
	"os/exec"
	"sort"
	"strings"
)

// loadGitCompletions shells out to git help -a and returns a sorted, deduplicated
// list of prefixed git subcommands (e.g. "git status", "git log") for tab
// completion in the command bar. Returns nil on error or if git is unavailable.
func loadGitCompletions() []string {
	out, err := exec.CommandContext(context.Background(), "git", "help", "-a").Output()
	if err != nil {
		return nil
	}

	return parseGitCommands(string(out))
}

func parseGitCommands(help string) []string {
	seen := make(map[string]bool)

	var cmds []string

	for line := range strings.SplitSeq(help, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Indented lines are command entries; section headers and the preamble
		// are not indented.
		if !strings.HasPrefix(line, "   ") && !strings.HasPrefix(line, "\t") {
			continue
		}

		name, _, _ := strings.Cut(trimmed, " ")
		if name == "" {
			continue
		}

		if !seen[name] {
			seen[name] = true
			cmds = append(cmds, "git "+name)
		}
	}

	sort.Strings(cmds)

	return cmds
}

// loadJjCompletions shells out to jj util completion bash and returns a sorted
// list of prefixed jj subcommands (e.g. "jj status", "jj log") for tab
// completion in the command bar. Returns nil on error or if jj is unavailable.
func loadJjCompletions() []string {
	out, err := exec.CommandContext(context.Background(), "jj", "util", "completion", "bash").
		Output()
	if err != nil {
		return nil
	}

	return parseJjCommands(string(out))
}

func parseJjCommands(completion string) []string {
	seen := make(map[string]bool)

	var cmds []string

	for line := range strings.SplitSeq(completion, "\n") {
		// bash-completion entries look like:  jj,<subcommand>)
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "jj,") {
			continue
		}

		rest, _, _ := strings.Cut(line, ")")

		_, sub, _ := strings.Cut(rest, ",")
		if sub == "" {
			continue
		}

		if !seen[sub] {
			seen[sub] = true
			cmds = append(cmds, "jj "+sub)
		}
	}

	sort.Strings(cmds)

	return cmds
}
