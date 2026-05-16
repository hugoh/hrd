# Development Instructions

## Build & Development Tasks

This project uses `mise` for task management. Use the following Mise tasks for all build, test, and lint operations:

### Building

```bash
mise build    # Build application binary into dist/
mise dev      # Run in development mode (go run .)
```

### Testing

```bash
mise test # Run tests using gotestsum
mise coverage # Generate test coverage report (cover.out)
mise covercheck # Check coverage meets threshold (80%)
```

### Linting & Formatting

#### Go Codebase

```bash
mise lint   # Run all lint checks (go mod tidy -diff, golangci-lint, goreleaser)
mise format # Format code (w/ golangci-lint)
mise fix    # Auto-fix lint issues (w/ golangci-lint)
```

#### Other Files

```bash
hk check --all # Runs all linters
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
hk check # Run hooks
```

## Agent Workflow

When making changes to this codebase:

1. **Before editing**: Run `mise lint` to understand current state
2. **After editing**:
   - Run `mise format` to format code
   - Run `mise test` to verify tests pass
   - Run `mise lint` to check for remaining issues
3. **If lint or formatting issues remain**: Run `mise fix` and `mise format` to auto-fix, then re-run lint
4. **Before completion**: Run `mise ci` to ensure all checks pass

Always use `mise` tasks rather than calling tools directly

## Test Coverage

Test coverage threshold is configured in `.testcoverage.yml`.
