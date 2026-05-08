# hrd

Herd your repos. Run commands across them in parallel. Watch results stream in live.

`hrd` is a multi-repo manager for developers who work across many repositories and use both **git** and **jj** (Jujutsu). It keeps your repos organized into groups, runs VCS commands across all of them at once, and shows a live unified status dashboard — with full awareness of branches, bookmarks, remote tracking, ahead/behind counts, and conflicts.

[![codecov](https://codecov.io/github/hugoh/hrd/graph/badge.svg?token=91HAIC8SER)](https://codecov.io/github/hugoh/hrd)

## Features

- **git and jj as first-class citizens** — both backends are fully supported with native status parsing. Colocated repos (jj on top of git) are handled correctly.
- **Parallel execution** — commands run concurrently across all matched repos, with results streaming in as each one completes.
- **Live status dashboard** — `hrd ls` shows a color-coded table of every repo's ref, remote sync state, dirty flag, and per-bookmark/branch badges, updating in real time.
- **Repo groups and context** — organize repos into named groups and set an active context so commands default to a focused scope.
- **Three dispatch commands** — `git`, `jj`, and `shell`. Clean and composable.
- **Extensible backend system** — new VCS backends implement a single interface and self-register. Zero changes to core code.
- **Shell completion** — bash, zsh, and fish, with dynamic completion of repo and group names from your live config.

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

## Status dashboard

```text
REPO                 VCS   REF            FLG  BOOKMARKS / BRANCHES
────────────────────────────────────────────────────────────────────
myproject            git   main            *   [main ↑2]
dotfiles             jj    rlkvwrto            [main ✓] [feat]
infra                git   feat/rework         [feat/rework ↑1↓3]
old-service          jj    qpvuntop        ‼   [legacy ✗] [fix !]
```

Bookmark badges are color-coded at a glance:

| Badge         | Meaning                |
| ------------- | ---------------------- |
| `[main ✓]`    | synced with remote     |
| `[main ↑2]`   | 2 commits ahead        |
| `[main ↓1]`   | 1 commit behind        |
| `[main ↑2↓1]` | diverged               |
| `[feat]`      | local only, no remote  |
| `[main !]`    | bookmark conflict (jj) |
| `[old ✗]`     | remote was deleted     |

Flags: `*` dirty working copy · `‼` unresolved conflict

## Command reference

```text
hrd [--config <path>] <command>

# Repo management
hrd repo add <path>... [--vcs git|jj] [--name <n>]
hrd repo rm  <name>...
hrd repo ls  [--group <g>]
hrd repo rename <old> <new>

# Group management
hrd group add <name> <repo>...
hrd group rm  <name>
hrd group ls

# Context (default scope)
hrd context set   <group>
hrd context clear
hrd context show

# Status
hrd ls [--repos <name,...>]
hrd ls -l [--repos <name,...>]

# Dispatch
hrd git   [--repos <r>] [--strict] -- <git args>
hrd jj    [--repos <r>] [--strict] -- <jj args>
hrd shell [--repos <r>] -- <shell command>

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
interactive_commands = ["log", "diff", "difftool", "mergetool", "show"]
```

`interactive_commands` lists VCS subcommands that require a real terminal (pagers, interactive diffs). These always run sequentially on a single repo rather than in parallel.

## Adding a backend

Implement the `Backend` interface in a new package, call `backend.Register()` in `init()`, and add a blank import to `main.go`. The interface is four methods: `Name`, `Detect`, `Status`, and `Run`.

---

## Related tools

**[gita](https://github.com/nosarthur/gita)** is the direct inspiration for `hrd`. It pioneered the repo-groups-context mental model for multi-repo management and demonstrated how useful a clean status dashboard is in a polyrepo workflow. `hrd` builds on those ideas with native jj support, an extensible backend system, richer bookmark/branch tracking, and a live-updating terminal UI.

**[gitbatch](https://github.com/isacikgoz/gitbatch)** — interactive TUI for batch git operations.

**[ghq](https://github.com/x-motemen/ghq)** — manages repository locations and clones. Complementary to `hrd`: use `ghq` to clone and organize, `hrd` to operate across them.

**[jj](https://github.com/jj-vcs/jj)** — the Jujutsu VCS that motivated first-class non-git support in `hrd`.
