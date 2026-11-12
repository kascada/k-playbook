# k-playbook

`k-playbook` is a toolbox of slash commands, skills, review recipes and checks. It is cloned into a subdirectory of the target project, `<project>/k-playbook/`. The project's own artifacts live next to it. A project is self-contained that way.

## Why k-playbook

- **It learns the project and keeps what it learned.** `/k-docs-code` writes the code
  documentation into the project's Kephalaion collection (`keph:/docs/generated/code/`)
  through `k-playbook docs checkout|publish code`, which reports conflicts instead of
  overwriting and keeps the hand-maintained `hints` of each document; `/k-docs-tools`
  writes pitfall-oriented library profiles into `keph:/docs/generated/libs/` the same
  way, its candidates taken from the version inventory,
  `/k-doc-inventory` writes the
  version inventory into the project's Kephalaion collection
  (`keph:/docs/generated/versions/inventory.md`), so the next session starts from that
  documentation instead of reading the code again. Review decisions persist as well: whatever is recorded in
  `known-decisions.md` as a false positive or an accepted risk never shows up as a
  finding in a later run.
- **A body of knowledge instead of code search.** The entry of the project
  documentation is `keph:/docs/` in the project's Kephalaion collection; the briefing
  names it, and a session reads it before it reads code. `docs/README.md` on disk is
  the older index from before the move; `/k-upgrade` moves the rest into the collection,
  deletes old code documentation on disk and removes the doc references from
  `AGENTS.md`.
- **Multi-agent loops instead of a single opinion.** `/k-task-refine` hardens tasks in
  rounds of Critic, Editor and Moderator — the advisors are read-only, only the
  Moderator writes, and every round lands in the review log. In an audit several
  recipes assess the same evidence, each from its own perspective, and `scan-triage`
  then pulls it together.
- **That saves tokens.** Scanners produce the evidence, not the model: the AI reads the
  merged `review-input.json` instead of the repository. Project knowledge comes from
  the docs instead of a fresh analysis, and the run state lives on disk — an
  interrupted session resumes rather than starting over.
- **Scan tools instead of guesswork.** gitleaks, trufflehog, trivy, semgrep, ruff,
  gosec, govulncheck, osv-scanner, grype, pip-audit and syft are listed in a matrix
  under `share/scripts/`; `install-security-tools.sh` installs them, and every job writes
  SARIF into the run directory. What was checked and what was not is stated with a
  reason on the entry.
- **Its own MCP server.** `k-playbook mcp` gives the assistant the resolved paths,
  instructions and catalogs, plus the tools for review runs. It is registered for
  Claude Code and OpenCode.

## Installation

k-playbook is cloned into the project it accompanies. There is no central installation
and no fixed host path; every project carries its own.

```bash
cd /path/to/project
git clone https://github.com/kascada/k-playbook.git
make -C k-playbook install
k-playbook
```

Without `make`, the same bootstrap runs directly:

```bash
k-playbook/bin/install
```

The target directory must be named `k-playbook`, because commands and skills address it
that way. Without a target argument the name follows from the repository name; do not
pass a different one. Only when cloning from a fork or mirror under a diverging name,
append `k-playbook` as the second argument.

**Go is not required.** `bin/install` downloads the release binary matching the platform
and installs it to `~/.local/bin/k-playbook`. On the host and inside a devcontainer it
runs in its own environment and therefore installs the matching binary in each.

**The installation needs network access.** The binaries are not in the repository but
attached as assets to the release named by `VERSION` in the root directory.
`bin/install` downloads the matching asset and verifies it against the shipped
`SHA256SUMS`.

**Kephalaion comes along.** If no `kephalaion` is on the `PATH`, `bin/install` then runs
`bin/install-kephalaion`, which installs it through the `install.sh` of the latest
Kephalaion release into `~/.local/bin/kephalaion`. Only the binary: setting up the node,
the service and the token stays with Kephalaion (`kephalaion node init`, see its README);
inside a devcontainer the node is usually the host's. An existing `kephalaion` is left
alone by `bin/install`; `k-playbook upgrade` updates it (see below). If this step fails, `bin/install` still succeeds,
because k-playbook itself is installed, and names the command to catch up.

The last call opens the web interface in the browser. On first run it finds no
`K-PLAYBOOK.yaml` yet and proposes where to create it: the directory above the clone. It
also proposes where the project repository lives — either the main directory itself or a
subdirectory next to it, for instance when the code is checked out alongside the
playbook. Nothing is written before you confirm:

```text
project/
├── .k-playbook/
│   └── K-PLAYBOOK.yaml the anchor; the directory holding .k-playbook/ is the main directory
└── k-playbook/         the installation
```

The same interface then creates the project-owned structure and sets up the linking for
the assistants. If the environment has VS Code, it also installs the extension
*k-playbook Workspace Tools* into the VS Code server there, in the background and without
a window reload; it brings the command *k-playbook: OpenCode im neuen Tab*. With Kephalaion
set up, the extension also binds the public collection `k-playbook` and the project's own
collection into the workspace whenever a window starts, opening or creating the project's
`.code-workspace` file if only the folder is open. Whoever never starts the interface
installs it with `k-playbook vscode install`. See [`docs/vscode.md`](./docs/vscode.md).

## Directory structure

```text
project/
├── AGENTS.md             instructions, one source for all assistants
├── CLAUDE.md             include file holding the line @AGENTS.md; the direction is fixed
├── .claude/
│   ├── commands/ ──┐     one symlink per command
│   └── skills/     │     one symlink per skill; OpenCode reads along here
├── .opencode/      │
│   └── commands/ ──┤
├── .k-playbook/    │
│   ├── K-PLAYBOOK.yaml   the anchor; its block kephalaion: names hub and collection
│   ├── share     ←─┤     -> ~/.local/share/k-playbook: shipped commands and skills,
│   │               │        unpacked by the binary once per machine; not versioned
│   ├── commands/ ←─┤     project-owned commands, committed (only when used)
│   └── skills/   ←─┘     project-owned skills, committed (only when used)
├── k-playbook/           the installation, fully replaceable
│   ├── reviews/ checks/
│   ├── bin/
│   ├── k-playbook.md     shipped instruction layer
│   └── installer/ docs/
└── k-playbook-local/     project-owned, committed
    ├── reviews/          overlay onto k-playbook/reviews/
    ├── results/          everything reviews produce; not versioned
    ├── guidelines/
    ├── priv/             notes and work in progress
    └── k-playbook.md     project-owned instruction layer
```

Tasks, todos, the project documentation and the source configuration of the version
inventory (`keph:/k-playbook/inventory.md`) live in the project's Kephalaion collection.
Its hub and name stand in the block `kephalaion:` of `.k-playbook/K-PLAYBOOK.yaml`, committed
with the project; the account of each machine, its token and the node's address come from
Kephalaion itself (`kephalaion node client`). Setup no longer creates `docs/`, `tasks/`, `data/` or `material/`; `/k-upgrade` removes
them from an existing project once they hold nothing but their README. The former
rule directory is gone as well (task 135); `/k-upgrade` removes it once empty or warns
that what is left there is no longer read. The same goes for the former directory of
project-owned checks (task 134): `k-check` runs only the shipped checks, and
`/k-upgrade` removes the directory once empty or notes that what is left there no longer
runs. Project-owned
commands and skills live in `.k-playbook/commands/` and `.k-playbook/skills/` since task
129; `k-playbook-local/commands/` and `k-playbook-local/skills/` are no longer read, and
`/k-upgrade` removes them once empty or names each entry left there with its target. The former
knowledge directories on disk are gone as well (task 123); `/k-upgrade` (3.0) clears what
is left of them and offers each knowledge file for a move to `keph:/docs/manual/` or for
deletion.

`k-playbook-local/` belongs in the project's repository. Two directories inside it are
a matter of choice — `results/` and `priv/`; `results/` is already created as private
during setup, and both remain switchable. Review results are a snapshot from one machine
and may contain found secrets in clear text. For `priv/` the project decides, and
k-playbook writes no `.gitignore` there on its own. What currently applies is shown and
switched by the **Local settings** block of the interface — measured with
`git check-ignore`, not guessed. `material/` has no switch any more; a managed
`.gitignore` already in place keeps working.

An entry of the same name in `k-playbook-local/` fully replaces the shipped one; an empty
one disables it. This holds for `reviews/`, and in the same way for `commands/` and
`skills/` in `.k-playbook/`. Checks have no project-owned layer: only the shipped ones
run.

That is why `.claude/commands/` and the other two targets are **real directories with
individual symlinks**, not directory symlinks: only that way do both sources arrive. The
interface compares the resolved catalog with what is registered, names the deviations and
offers to fix them.

What applies in the end is not computed by any command itself. A session gets it with
the briefing, the one call at its start:

```bash
k-playbook briefing
```

`k-playbook context` still prints the resolved working state as JSON; the briefing carries
what is left of it in its section `context` until that output has been taken apart.

Linking is set up for Claude Code and OpenCode. Skills exist only once, under
`.claude/skills`, because OpenCode searches that directory as well. `CLAUDE.md` is a small
include file holding the line `@AGENTS.md`: Claude Code reads `CLAUDE.md` exclusively,
while OpenCode prefers `AGENTS.md` — so there is exactly one instruction file, and Claude
Code loads it through the import.

The direction is the same everywhere. If a project brings only a real `CLAUDE.md`, setup
**renames** it to `AGENTS.md` and creates `CLAUDE.md` anew as an include; the content is
preserved and not duplicated. A `CLAUDE.md` that already carries the line `@AGENTS.md` is
set up — including one with its own house rules alongside. A symlink from an older
version is replaced without loss on the first `k-playbook briefing`. What cannot be
resolved automatically — two real files without an import line, a deliberately set link
to a different target, a git-ignored `AGENTS.md` — is reported as a **conflict** and left
untouched. As long as it stands, Claude Code sees nothing of the playbook.

## Updating

After startup the interface checks whether the installation lags behind the remote and
pulls on a single click. For the pull, `k-playbook/` is made writable and set back to
read-only afterwards. By hand it works the same way:

```bash
cd /path/to/project
make -C k-playbook installer-update
```

`k-playbook/` contains nothing project-owned and is fully replaceable because of that.

**If `VERSION` changed along the way, the new state comes with a different binary.** If the
running program is older than that version, the interface installs it on the spot: one
click runs the clone's bootstrap, which verifies the asset against `SHA256SUMS` and
replaces `~/.local/bin/k-playbook`, and the service then restarts itself from the new
program — the open page follows to the new address. Nothing is downloaded on startup or
during a check, only on that click. If the running program is equally old or newer, as in a
development repository after `make dev-install`, nothing is installed.

By hand it stays the same call as for the initial installation, due once in every
environment — on the host as well as in the devcontainer:

```bash
make -C k-playbook install
```

Without `make`: `k-playbook/bin/install`. A target project has no `install` target of its
own; the call always goes through the clone. It is the way to go if the installation fails
or if `k-playbook` in the `PATH` resolves to a different file than
`~/.local/bin/k-playbook`; in that case the interface shows a note with both paths instead
of the button, because a program installed there would not be the one the next call starts.
If only commands, instructions or recipes are new, `VERSION` does not change and the service keeps
running.

In the terminal, `k-playbook upgrade` does both in one go — first Kephalaion (installed when
missing from the `PATH`, otherwise updated through `kephalaion upgrade --auto` where
Kephalaion considers that safe, with its node restarted; a hub, a global installation or an
older `kephalaion` without `--auto` are only reported), then the pull, the binary when
the running program is older, then
moving hub and collection out of an old `k-playbook-local/K-PLAYBOOK.local.yaml` into the
block `kephalaion:` of `.k-playbook/K-PLAYBOOK.yaml`,
`AGENTS.md`, `CLAUDE.md`, linking and MCP entries, and last the `README.md` of the
directories under `keph:/docs/` from their templates (only with a ready Kephalaion, reported
as a hint) —
and `/k-upgrade` in an assistant session relays its message; `k-playbook upgrade --check`
only reports what a run would do. Kephalaion comes first because k-playbook depends on it and
the later steps need its node. After a Kephalaion upgrade or node restart, as after a program
change, the session has to be restarted. Once, when coming from a binary that predates this
order, the first run does not update Kephalaion yet; the second `/k-upgrade` does.

The linking for the assistants catches up by itself afterwards — on the next
`k-playbook briefing`, the call at the start of every session, or on the next look at the
assistant block of the interface. It follows the project's catalog, not the path by which
the installation was updated. For the new commands to arrive, the assistant has to be
restarted once; it reads its list at startup.

## Building it yourself

For normal operation the release assets that `bin/install` downloads are enough. Anyone
working on the tool, or preferring to build it themselves, needs Go:

```bash
make dist        # builds all platforms into dist/, the path before a release
make dist-host   # builds only this platform, considerably faster
make dev-install # builds this platform and replaces ~/.local/bin/k-playbook
make gui         # dev-install and start
```

This keeps the development loop network-free: `make gui` builds, installs and starts the
freshly built state without downloading anything. `dist/` is not versioned.

The build uses the same flags CI uses for the release assets — `-trimpath`,
`CGO_ENABLED=0`, `-buildvcs=false` and the toolchain pinned in `installer/go.mod` — so
that both paths produce bit-identical binaries. The versioned `SHA256SUMS` rests on
exactly that.

A release runs in two steps, so that `VERSION` never points at a tag without downloads:

```bash
make release VERSION=v0.2.0          # builds, commits VERSION and SHA256SUMS, pushes the tag
# CI rebuilds, verifies against SHA256SUMS and uploads the assets
make release-publish VERSION=v0.2.0  # brings the same commit onto main
```

The pushes and `gh` calls of `make release` and `make release-publish` run through
`k-playbook github run k-playbook` (see [`docs/github-access.md`](./docs/github-access.md)); the
service name can be overridden with `GITHUB_SERVICE=…`.

## Core principles

- Every project carries its own installation in a subdirectory. No fixed host path, no global symlink.
- Installation and project ownership are strictly separated: `k-playbook/` is fully replaced on every update, everything beside it is never touched.
- Shipped reviews and checks are not edited. A project deviates from a review by overlay: a local file of the same name replaces completely, an empty one disables. Checks have no overlay; only the shipped ones run.
- Paths are not part of the configuration. They follow from the main directory, the one holding `.k-playbook/K-PLAYBOOK.yaml`.
- Reviews and results stay project-owned artifacts under `k-playbook-local/`; tasks and todos are documents in the project collection.
- Projects may use their own venvs. Security tools are installed separately from those, host-/user-local or in dedicated k-playbook tool venvs; they are one of the two deliberate exceptions to project locality.
- The base tools k-playbook calls itself — `git`, `curl` or `wget`, `tar`, `python3`, `rg` — are the second exception, and a different one: they are not scanners but the ground the commands stand on, so a missing one warns and never stops a run. `install-base-tools.sh` prefers the distribution package manager and falls back to a user-local release install; k-playbook never escalates to root by itself, it prints the `sudo` command.
- Nothing is written except after confirmation, step by step.

## Documentation

- [`docs/README.md`](./docs/README.md) - the complete documentation index.
- [`docs/manual.md`](./docs/manual.md) - purpose, core model and standard workflows.
- [`docs/k-playbook-format.md`](./docs/k-playbook-format.md) - the contract: `K-PLAYBOOK.yaml`, structure, overlay.
- [`docs/installation.md`](./docs/installation.md) - clone, setup steps, security tools.
- [`docs/commands.md`](./docs/commands.md) - index of the slash commands.
- [`docs/vscode.md`](./docs/vscode.md) - the VS Code extension: commands, settings, installation, build.
- [`docs/migration.md`](./docs/migration.md) - state of the migration, decisions and open points.
