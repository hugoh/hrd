package tui

import "github.com/hugoh/hrd/internal/runner"

// ---------------------------------------------------------------------------
// Bubble Tea message types for async operations.
// ---------------------------------------------------------------------------

// The gen field on the status/exec messages is the run generation they were
// produced under. A message whose gen no longer matches the model's is a
// leftover from a superseded run and is dropped.

// statusUpdateMsg carries a single repo status result as it streams in.
type statusUpdateMsg struct {
	gen    int
	result runner.StatusResult
}

// statusDoneMsg signals that all status results have been streamed.
type statusDoneMsg struct{ gen int }

// execResultMsg carries a single dispatch result as it streams in.
type execResultMsg struct {
	gen    int
	result execResult
	err    error
}

// execDoneMsg signals that all dispatch results have been collected.
type execDoneMsg struct{ gen int }

// vcsCompletionsMsg delivers a backend's subcommand list, loaded
// asynchronously for command-bar tab completion.
type vcsCompletionsMsg struct {
	name string
	cmds []string
}

// execResult pairs a repo name with its outcome.
type execResult struct {
	name   string
	result runner.Result
}
