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
- **Repo groups and context** — organize repos into named groups and set an active context so commands default to a focused scope.
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
# Track some repos (VCS is auto-detected)
hrd repo add ~/dev/myproject ~/dev/dotfiles ~/dev/infra

# Organize into groups
hrd group add work myproject infra
hrd context set work

# Live status across all repos in context
hrd ls

# Run a command across all repos
hrd git -- fetch --all

# Or just the ones you care about right now
hrd jj --repos dotfiles -- log

# Arbitrary shell commands
hrd shell -- 'echo $(basename $PWD): $(git rev-parse --short HEAD)'
```

**Tip**: Group names are displayed with an `@` prefix (e.g., `@work`, `@oss`) to distinguish them from repo names. The `@` is optional on input — `hrd context set work` and `hrd context set @work` both work.

## Status dashboard

```text
NAME         VCS  REF               MSG
myproject    git  main ↑2*          feat: add new feature (2 hours ago)
dotfiles     jj   main ✓∅           config: update zshrc (1 day ago)
infra        git  feat/rework ↑1↓3  refactor: networking layer (3 days ago)
old-service  jj   legacy ✗✓!‼       fix: critical bug (1 week ago)
```

Status symbols at a glance:

| Symbol | Meaning                |
| ------ | ---------------------- |
| `✓`    | synced with remote     |
| `↑2`   | 2 commits ahead        |
| `↓1`   | 1 commit behind        |
| `↑2↓1` | diverged               |
| `∅`    | local only, no remote  |
| `!`    | bookmark conflict (jj) |
| `✗`    | remote was deleted     |
| `*`    | dirty working copy     |
| `‼`    | unresolved conflict    |
| `?`    | unknown remote state   |

## Command reference

```text
hrd [--config <path>] <command>

# Repo management
hrd repo add <path>... [--vcs git|jj] [--name <n>]
hrd repo rm  <name>...
hrd repo ls  [--group <g>]
hrd repo rename <old> <new>
hrd repo refresh <name>... | --all

# Group management
hrd group add <name> <repo>...  # @-prefix optional on input; displayed as @<name>
hrd group rm  <name>
hrd group ls [<name>]

# Context (default scope)
hrd context set   <group>       # @-prefix optional on input; displayed as @<group>
hrd context clear
hrd context show

# Status
hrd ls [--repos <name,...>] [--names] [--dirs] [--message]
hrd ll [--repos <name,...>]   # detailed view (alias for ls --message)

# Dispatch
hrd git   [--repos <r>] [--interactive] -- <git args>
hrd jj    [--repos <r>] [--interactive] -- <jj args>
hrd shell [--repos <r>] [--interactive] -- <shell command>

# VCS subcommands (use each repo's active backend)
hrd status [--repos <name,...>]
hrd diff   [--repos <name,...>]
hrd log    [--repos <name,...>]

# Shell completion
eval "$(hrd completion bash)"   # add to .bashrc
eval "$(hrd completion zsh)"    # add to .zshrc
hrd completion fish | source    # add to config.fish
```

## Configuration

Config lives at `~/.config/hrd/config.toml` (respects `$XDG_CONFIG_HOME`).

```toml
[repos.dotfiles]
path = "/home/hugo/.local/share/chezmoi"
vcs = "jj"

[repos.myproject]
path = "/home/hugo/dev/myproject"
vcs = "git"

[groups.oss]
repos = ["myproject", "infra"]

[context]
current = "oss"

[settings]
concurrency = 8
```

**Note**: Group names in the CLI are prefixed with `@` (e.g., `hrd context set @oss`, `hrd group ls @work`) to distinguish them from repo names. The `@` is optional on input — `work` and `@work` are treated identically. The config file stores group names without the `@` prefix. on dispatch commands to run with a real terminal (pagers, interactive diffs). Interactive commands run sequentially on one repo at a time rather than in parallel.

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
