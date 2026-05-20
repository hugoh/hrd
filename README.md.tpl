# hrd

Herd your repos. Run commands across them in parallel. Watch results stream in live.

`hrd` is a multi-repo manager for developers who work across many repositories and use both **git** and **jj** (Jujutsu). It keeps your repos organized into groups, runs VCS commands across all of them at once, and shows a live unified status dashboard — with full awareness of branches, bookmarks, remote tracking, ahead/behind counts, and conflicts.

[![CI](https://github.com/hugoh/hrd/actions/workflows/ci.yml/badge.svg)](https://github.com/hugoh/hrd/actions/workflows/ci.yml)
[![codecov](https://codecov.io/github/hugoh/hrd/graph/badge.svg?token=91HAIC8SER)](https://codecov.io/github/hugoh/hrd)
[![Go Report Card](https://goreportcard.com/badge/github.com/hugoh/hrd)](https://goreportcard.com/report/github.com/hugoh/hrd)

## Features

- **git and jj as first-class citizens** — both backends are fully supported with native status parsing. Colocated repos (jj on top of git) are handled correctly.
- **Parallel execution** — commands run concurrently across all matched repos, with results streaming in as each one completes.
- **Live status dashboard** — `hrd ls` shows a color-coded table of every repo's ref, remote sync state, dirty flag, and per-bookmark/branch badges, updating in real time.
- **Interactive TUI** — `hrd` (or `hrd tui`) opens a full-screen terminal UI for browsing repos, filtering by group or name, and dispatching commands across multiple repos with live streaming output.
- **Repo groups** — organize repos into named groups for easier filtering.
- **Three dispatch commands** — `git`, `jj`, and `shell`.
- **Shell completion** — bash, zsh, and fish, with dynamic completion of repo and group names from your live config.
- **Extensible backend system** — new VCS backends implement a single interface and self-register.

## Install

### Homebrew (macOS/Linux)

```sh
brew install hugoh/tap/hrd
```

### Linux (deb/rpm)

Download the `.deb` or `.rpm` from the [releases page](https://github.com/hugoh/hrd/releases) and install with your package manager:

```sh
# Debian/Ubuntu
sudo apt install ./hrd_*.deb

# RHEL/Fedora
sudo dnf install ./hrd_*.rpm
```

### mise

```sh
mise use -g github:hugoh/hrd
```

### Go install

```sh
go install github.com/hugoh/hrd@latest
```

### From source

```sh
git clone https://github.com/hugoh/hrd
cd hrd
go build -o hrd .
```

## Quick start

```sh
# Track some repos
hrd repo add ~/dev/myproject ~/dev/infra
hrd repo add -n dotfiles ~/.local/share/chezmoi

# Start the TUI
hrd

# Organize into groups
hrd group add work myproject infra

# Live status across all repos in context
hrd ls

# Run a command across all repos
hrd fetch

# Or just the ones you care about right now
hrd jj dotfiles log

# Arbitrary shell commands
hrd shell -- 'echo $(basename $PWD): $(git rev-parse --short HEAD)'
```

**Tip**: Group names are displayed with an `@` prefix (e.g., `@work`, `@oss`) to distinguish them from repo names. The `@` is optional on input — `hrd ls @work` and `hrd ls work` both work.

## Status dashboard

```text
{{ .LLOutput -}}
```

Status symbols at a glance:

{{ .SymbolsTable }}

## Interactive TUI

Run `hrd` (or `hrd tui`) to open the full-screen terminal UI:

- Browse all tracked repos in a sortable table.
- Filter by group with `@` — type `@work` to show only work repos, or select individual repos with `Space`.
- Run VCS commands (`status`, `diff`, `log`, `fetch`, `pull`, `push`) from a single key press — results stream in live as each repo completes.
- The command palette (`:`) gives access to every subcommand without leaving the TUI.
- Shortcuts: `s` (status), `l` (log), `d` (diff), `f` (fetch), `p` (pull), `P` (push), `@` (group picker), `q` or `Esc` (quit).

The TUI mirrors the CLI: the same backends, the same parallel execution, the same status parsing — just in an interactive, always-on view.

## Command reference

```text
{{ .HelpOutput -}}
```

## Configuration

Config lives at `~/.config/hrd/config.toml` (respects `$XDG_CONFIG_HOME`).

```toml
[repos.dotfiles]
path = "/home/alice/.local/share/chezmoi"

[repos.myproject]
path = "/home/alice/dev/myproject"

[repos.infra]
path = "/home/alice/dev/infra"

[groups.work]
repos = ["myproject", "infra"]

[settings]
concurrency = 8
```

**Note**: Group names in the CLI are prefixed with `@` (e.g., `hrd ls @oss`, `hrd group ls @work`) to distinguish them from repo names. The `@` is optional on input — `work` and `@work` are treated identically. The config file stores group names without the `@` prefix.

---

## Contributing

Contributions are very welcome! Please open an issue or submit a pull request. See [development instructions](AGENTS.md).

### Adding a backend

Implement the `Backend` interface in a new package, add a `Register()` function that calls `backend.Register()`, and call it from `main.go`'s `Run()` function. The interface is four methods: `Name`, `Detect`, `Status`, and `Run`.

---

## Related tools

[gita](https://github.com/nosarthur/gita) is the direct inspiration for `hrd`.

[Jujutsu (jj) VCS](https://github.com/jj-vcs/jj) motivated creating `hrd` with first-class non-git support.

## Disclaimer

LLMs were used to put together the initial version.
