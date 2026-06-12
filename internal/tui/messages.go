package tui

import "github.com/hugoh/hrd/internal/runner"

// ---------------------------------------------------------------------------
// Bubble Tea message types for async operations.
// ---------------------------------------------------------------------------

// statusUpdateMsg carries a single repo status result as it streams in.
type statusUpdateMsg struct {
	result runner.StatusResult
}

// statusDoneMsg signals that all status results have been streamed.
type statusDoneMsg struct{}

// execResultMsg carries a single dispatch result as it streams in.
type execResultMsg struct {
	result execResult
	err    error
}

// execDoneMsg signals that all dispatch results have been collected.
type execDoneMsg struct{}

// errMsg wraps an unrecoverable error.
type errMsg struct{}

// execResult pairs a repo name with its outcome.
type execResult struct {
	name   string
	result runner.Result
}
