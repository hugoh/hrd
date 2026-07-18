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
- **Repo groups** — group repos for easier filtering.
- **Live-resolved directory roots** — track a directory once and new repos added under it show up automatically, with nothing written to config for the discovered repos.
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

# Or discover everything under a directory at once
hrd repo scan add ~/dev
hrd repo scan add -g work ~/work     # ...and group what it finds

# Or keep a directory always in sync — new repos show up with no re-scan
hrd repo root add ~/my-repos -g personal

# Start the TUI
hrd

# Add repos to groups
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

**Tip**: Group names are displayed with an `@` prefix (e.g., `@work`, `@oss`) to distinguish them from repo names. The `@` is optional on input — `hrd ls @work` and `hrd ls work` both work. `@@none` is a reserved pseudo-group matching repos with no group at all (e.g. `hrd ls @@none`) — the double `@` keeps it from ever colliding with a real group name, which you also can't create (group names can't start with `@`).

## Status dashboard

```text
NAME        VCS STATUS                                                   
myproject   git main ↑2*  feat: add new feature (2 hours ago)            
dotfiles    jj  main ✓∅⇡5  config: update zshrc (1 day ago)              
infra       git feat/rework ↑1↓3  refactor: networking layer (3 days ago)
old-service jj  legacy ✗✓!‼  fix: critical bug (1 week ago)
```

Status symbols at a glance:

| Symbol | Meaning |
| ------ | ------- |
| `✓` | Synced with remote |
| `↑N` | N commits ahead of remote |
| `↓N` | N commits behind remote |
| `⇡N` | Working copy ahead of bookmark (local) |
| `↑N↓N` | Diverged (ahead and behind) |
| `∅` | Local only, no remote |
| `‼` | Unresolved conflict |
| `!` | Bookmark conflict (jj) |
| `✗` | Remote was deleted |
| `*` | Dirty working copy |
| `?` | Unknown remote state |

## Interactive TUI

Run `hrd` (or `hrd tui`) to open the full-screen terminal UI:

- Browse all tracked repos in a sortable table.
- Filter by group with `@` — type `@work` to show only work repos, or select individual repos with `Space`. `[ungrouped]`/`[attention]` (alongside `[all]`) filter to repos in no group, or repos needing attention (dirty/out of sync), respectively. `*` toggles the attention filter directly without opening the picker.
- Type-to-filter with `/` — fuzzy-match repos by name as you type; `Enter` keeps the filter, `Esc` clears it.
- Run VCS commands (`status`, `diff`, `log`, `fetch`, `pull`, `push`) from a single key press — results stream in live as each repo completes.
- The command palette (`:`) gives access to every subcommand without leaving the TUI.
- Shortcuts: `S` (status), `l` (log), `d` (diff), `f` (fetch), `p` (pull), `P` (push), `@` (group picker), `/` (name filter), `q` or `Esc` (quit).

The TUI mirrors the CLI: the same backends, the same parallel execution, the same status parsing — just in an interactive, always-on view.

## Command reference

```text
hrd manages a set of git and jj repositories tracked in a config file,
letting you run status checks and VCS operations across many of them at once.

Run with no subcommand to open the TUI, optionally scoped to specific repos or
groups. -c/--config applies to every subcommand and defaults to
$XDG_CONFIG_HOME/hrd/config.toml, falling back to ~/.config/hrd/config.toml.

Usage:
  hrd [flags]
  hrd [command]

Examples:
  hrd                    # open the TUI across all tracked repos
  hrd myrepo             # open the TUI scoped to one repo
  hrd repo add ~/code/*  # start tracking repos
  hrd ls                 # show status of all tracked repos

Repository management:
  group       manage repo groups
  repo        manage tracked repositories

Inspect:
  diff        show diff for repos (git diff or jj diff)
  ll          show status of repos with commit message and time
  log         show log for repos (git log or jj log)
  ls          show status of repos
  status      show detailed status for repos (git status or jj status)

Remote sync:
  fetch       fetch from remotes (git fetch or jj git fetch)
  pull        pull from remotes (git pull or jj git pull)
  push        push to remotes (git push or jj git push)

Run commands:
  git         run a git command across repos
  jj          run a jj command across repos
  shell       run an arbitrary shell command across repos
  tui         interactive terminal UI for browsing and running commands across repos

Additional Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command

Flags:
  -c, --config string   path to config file (default "~/.config/hrd/config.toml")
  -h, --help            help for hrd
  -v, --version         version for hrd

Use "hrd [command] --help" for more information about a command.
```

### Scoping and `--`

The `git`, `jj`, and `shell` commands take an optional repo/group scope
followed by the command to run. Everything after `--` is passed to the
subprocess verbatim — repo names in the command are never reinterpreted as
scope:

```sh
hrd git myrepo @work -- log --oneline -5
hrd shell -- grep -r TODO .
```

Without `--`, the leading args that match repo or group names form the
scope and the first non-matching arg starts the command (`hrd git myrepo
log` works, but flags like `--oneline` need the `--` form).

### Repo discovery

`hrd repo scan add <dir>...` walks each directory (default depth 5, tune
with `--depth`) and tracks every git/jj repo it finds as its own entry in
the config. Detected repos are not descended into, so vendored or nested
checkouts stay untracked; hidden directories are skipped. `-g/--group`
assigns everything found to a group. Name collisions fall back to
`<parent>-<dir>`; already-tracked paths are left alone, so re-running
`scan add` is safe. `hrd repo scan ls <dir>...` previews the same walk
without saving — purely read-only — with `--tracked`/`--untracked` to
filter by whether a match is already in the config.

```sh
hrd repo scan add ~/dev --depth 2 --pattern 'api-*' -g backend
hrd repo scan ls ~/dev --untracked
```

For a directory you want to stay in sync automatically, use `hrd repo root
add <dir>` instead: it's a standing declaration, walked live on every `hrd`
invocation, so repos added under it later show up with no re-scan and
nothing is written to the config for the discovered repos themselves —
only the root is persisted. `--depth` (default 1) caps how deep it walks
and `-g/--group` tags every repo found under it.

```sh
hrd repo root add ~/my-repos --depth 2 -g personal
hrd repo root ls
hrd repo root rm my-repos
```

### Status filters

`ls`, `ll`, the VCS subcommands (`status`, `diff`, `log`, `fetch`, `pull`,
`push`), `shell`, `git`, and `jj` accept status filters that narrow the
scope to repos in a given state (multiple flags are a union):

```sh
hrd push --ahead          # push only repos with unpushed commits
hrd pull --behind @work   # pull only the work repos that are behind
hrd ls --dirty --names    # script-friendly list of dirty repos
hrd shell --dirty -- git stash list
```

| Flag       | Matches repos that…                              |
| ---------- | ------------------------------------------------ |
| `--dirty`  | have uncommitted changes in the working copy     |
| `--ahead`  | are ahead of their remote, or have local-only work |
| `--behind` | are behind their remote                          |

### Aliases

Define your own commands in the config and they become first-class
subcommands — with repo/group scoping, `--interactive`, status filters, and
shell completion, and listed under `hrd --help`:

```toml
[aliases]
sync = "pull --rebase"
mkclean = "!make clean"
```

```sh
hrd sync @work          # git pull --rebase / jj git pull, per backend
hrd mkclean --dirty     # make clean, only in dirty repos
hrd sync -- --autostash # extra args append to the expansion
```

The first word decides the routing: `git`/`jj` pins a backend, `!` (or
`sh`) runs a shell command, anything else routes through each repo's own
backend like the built-in subcommands. The TUI command bar completes and
expands the same aliases. Aliases that would shadow a built-in command are
ignored with a warning.

### Exit codes

| Code | Meaning                                          |
| ---- | ------------------------------------------------ |
| 0    | All repos succeeded                              |
| 1    | The command ran but failed in at least one repo  |
| 2    | Usage or config error (unknown repo, bad flags)  |

## Configuration

Config lives at `~/.config/hrd/config.toml` (respects `$XDG_CONFIG_HOME`).

```toml
[repos.dotfiles]
path = "/home/alice/.local/share/chezmoi"

[repos.myproject]
path = "/home/alice/dev/myproject"
groups = ["work"]

[repos.infra]
path = "/home/alice/dev/infra"
groups = ["work"]

[roots.personal]
path = "/home/alice/my-repos"
depth = 1
groups = ["personal"]

[settings]
concurrency = 8

[aliases]
sync = "pull --rebase"          # per-repo routing (git pull / jj git pull)
gpf = "git push --force-with-lease"  # always that backend
mkclean = "!make clean"         # "!" or "sh " prefix = shell command
```

**Note**: Groups are derived from the `groups` field on each repo, plus the `groups` field on any `[roots.*]` entry (inherited by every repo discovered under it). Group names are displayed with an `@` prefix (e.g., `@work`) to distinguish them from repo names. The `@` is optional on input — `work` and `@work` are treated identically. Group names can never start with `@` (so `hrd group add @work myrepo` stores `work`, not `@work`) — this reserves the `@@` namespace (e.g. `@@none`, see the Quick start tip) for built-in pseudo-groups, which can never collide with anything you create.

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
