# Agent Instructions

## Project Overview

## Build & Development Tasks

This project uses `mise` for task management. Use the following Mise tasks for all build, test, and lint operations:

### Building

```bash
mise build    # Build application binary to dist/tmhi-cli
mise dev      # Run in development mode (go run .)
```

### Testing

```bash
mise test # Run tests using gotestsum
mise coverage # Generate test coverage report (cover.out)
mise covercheck # Check coverage meets threshold (80%)
```

### Linting & Formatting

```bash
mise lint   # Run all lint checks (go mod tidy -diff, golangci-lint, prettier, goreleaser)
mise format # Format code (golangci-lint fmt, prettier --write)
mise fix    # Auto-fix lint issues (golangci-lint run --fix)
```

### Module Maintenance

```bash
mise tidy  # Tidy Go module (go mod tidy -v)
mise depup # Upgrade dependencies
mise clean # Clean build artifacts
```

### CI

```bash
mise ci # Run full CI checks (lint, test, covercheck)
```

## Agent Workflow

When making changes to this codebase:

1. **Before editing**: Run `mise lint` to understand current state
2. **After editing**:
   - Run `mise format` to format code
   - Run `mise test` to verify tests pass
   - Run `mise lint` to check for remaining issues
3. **If lint or formatting issues remain**: Run `mise fix` and `mise format` to autofix, then re-run lint
4. **Before completion**: Run `mise ci` to ensure all checks pass

## Notes

- Test coverage threshold is configured in `.testcoverage.yml`.
- Prettier is used for non-Go files (Markdown, YAML, etc.)
- Always use `mise` tasks rather than calling tools directly
