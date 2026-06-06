package tui

// Completion logic has moved to the VCS backends. Each backend's
// Subcommands(ctx) method returns available subcommands, and the TUI
// iterates backend.Names() to load and cache them dynamically.
