# k-playbook

`k-playbook` is a toolbox of slash commands, skills and review recipes. It is installed as one binary per machine; a project carries only its anchor, `.k-playbook/K-PLAYBOOK.yaml`, and what it owns itself. Everything shipped comes with the binary.

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
  names it, and a session reads it before it reads code.
- **Multi-agent loops instead of a single opinion.** `/k-task-refine` hardens tasks in
  rounds of Critic, Editor and Moderator — the advisors are read-only, only the
  Moderator writes, and every round lands in the review log. In an audit several
  recipes assess the same evidence, each from its own perspective, and `scan-triage`
  then pulls it together.
- **That saves tokens.** Scanners produce the evidence, not the model: the AI reads the
  merged `review-input.json` instead of the repository. Project knowledge comes from
  the docs instead of a fresh analysis, and the run state is stored with the run — an
  interrupted session resumes rather than starting over.
- **Scan tools instead of guesswork.** gitleaks, trufflehog, trivy, semgrep, ruff,
  gosec, govulncheck, osv-scanner, grype, pip-audit and syft are listed in a matrix
  under `share/scripts/`; `install-security-tools.sh` installs them, and every job writes
  SARIF into the run directory. What was checked and what was not is stated with a
  reason on the entry.
- **Its own MCP server.** `k-playbook mcp` gives the assistant the resolved paths
  and catalogs, plus the tools for review runs. It is registered for
  Claude Code and OpenCode.

## Installation

The installation is the binary alone: one `k-playbook` per machine, once on the host and
once in every devcontainer. Nothing of the tool is copied into the project.

```bash
curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash
cd /path/to/project
k-playbook
```

Without `curl`, the same bootstrap runs with `wget`:

```bash
wget -qO- https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash
```

**Go is not required.** `bin/install` downloads the release binary matching the platform
and installs it to `~/.local/bin/k-playbook`. On the host and inside a devcontainer it
runs in its own environment and therefore installs the matching binary in each.

**The installation needs network access, but not `git`.** The binaries are not in the
repository but attached as assets to the releases. `bin/install` determines the newest
published release (the redirect of `releases/latest` on GitHub, over HTTPS, anonymously,
with `curl` or `wget`), downloads the asset of its tag and verifies it against the
`SHA256SUMS` committed at the tag.

**Kephalaion comes along.** If no `kephalaion` is on the `PATH`, `bin/install` then
installs it through the `install.sh` of the latest
Kephalaion release into `~/.local/bin/kephalaion`. Only the binary: setting up the node,
the service and the token stays with Kephalaion (`kephalaion node init`, see its README);
inside a devcontainer the node is usually the host's. An existing `kephalaion` is left
alone by `bin/install`; `k-playbook upgrade` updates it (see below). If this step fails, `bin/install` still succeeds,
because k-playbook itself is installed, and names the command to catch up.

The last call opens the web interface in the browser. On first run it finds no
`K-PLAYBOOK.yaml` yet and proposes where to create it: the root of the Git repository the
call was made in, otherwise the working directory. It also proposes where the project
repository lives — either the main directory itself or a subdirectory of it, for instance
when the code is checked out one level below. Nothing is written before you confirm:

```text
project/
└── .k-playbook/
    └── K-PLAYBOOK.yaml the anchor; the directory holding .k-playbook/ is the main directory
```

The same interface then registers the MCP server and sets up the linking for the
assistants. If the environment has VS Code, it
also installs the extension *k-playbook Workspace Tools* into the VS Code server there, in the background and without
a window reload. With Kephalaion
set up, the extension binds the project's own collection into the workspace whenever a window starts, opening or creating the project's
`.code-workspace` file if only the folder is open. Whoever never starts the interface
installs it with `k-playbook vscode install`. See [`defaults/docs/handbook/vscode.md`](./defaults/docs/handbook/vscode.md).

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
└── .k-playbook/    │
    ├── K-PLAYBOOK.yaml   the anchor; its block kephalaion: names hub and collection
    ├── share     ←─┤     -> ~/.local/share/k-playbook: shipped commands and skills,
    │               │        unpacked by the binary once per machine; not versioned
    ├── commands/ ←─┤     project-owned commands, committed (only when used)
    └── skills/   ←─┘     project-owned skills, committed (only when used)
```

What k-playbook ships is not in the project. Commands, skills and scripts are unpacked by
the binary to `~/.local/share/k-playbook/`, once per machine; the shipped texts —
instructions, review recipes, guidelines, the texts of the commands and the handbook — lie
in the public collection `k-playbook` on the project's Kephalaion hub and are read as
`k-playbook:/…` references. In the former model a project carried a clone of this
repository under `k-playbook/` instead; a clone that is still there is no longer read, and
`/k-upgrade` deletes it.

Everything a project owns lies in its Kephalaion collection or under `.k-playbook/`.
Tasks, todos, the project documentation and the source configuration of the version
inventory (`keph:/k-playbook/inventory.md`) live in the collection, and so do project-owned
review recipes, instructions and includes (`keph:/k-playbook/`) and project-owned
guidelines (`keph:/docs/guidelines/`); guidelines behind `k-playbook:` come shipped only.
The collection's hub and name stand in the block `kephalaion:` of
`.k-playbook/K-PLAYBOOK.yaml`, committed with the project; the account of each machine, its
token and the node's address come from Kephalaion itself (`kephalaion node client`).
Project-owned commands and skills live in `.k-playbook/commands/` and
`.k-playbook/skills/`.

`/k-upgrade` has one conversion that needs a judgement (3.8): it deletes a leftover clone
`k-playbook/` and then removes the `.gitignore` lines that kept `k-playbook/` out of Git,
each only once its directory is gone.

The reviews are paused: the location of review results is not decided yet. Until then
`/k-review`, `/k-audit` and `/k-remediation` only say so, `k-playbook scan` and
`k-playbook merge` end with an error, and the MCP tools `k_playbook_review_status`,
`_create`, `_scan`, `_merge` and `_write_ai_entry` answer `reviews_paused` and write
nothing.

A project-owned entry of the same name fully replaces the shipped one; an empty one
disables it. This holds for review recipes, instructions and includes in the project
collection under `keph:/k-playbook/`, and in the same way for `commands/` and `skills/`
in `.k-playbook/`.

That is why `.claude/commands/` and the other two targets are **real directories with
individual symlinks**, not directory symlinks: only that way do both sources arrive. The
interface compares the resolved catalog with what is registered, names the deviations and
offers to fix them.

What applies in the end is not computed by any command itself. A session gets it with
the briefing, the one call at its start:

```bash
k-playbook briefing
```

What only single commands need — the finding for `gh`, the base tools, the remediation
policy — is not part of it and comes with `k-playbook briefing --part environment`.

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

**The program updates itself from the release.** There are exactly two
version sources: the program's own version, stamped into the binary, and the newest
published release on GitHub. `k-playbook version` prints the first, `k-playbook version
--all` adds the second, and the health check (the briefing, `k-playbook health`) warns when
the program is older than the newest release. The health check reads the newest release
from a cache under `~/.cache/k-playbook/` and asks GitHub at most every few hours;
`k-playbook upgrade` and `version --all` always ask right away. If it is, the interface offers `Programm aktualisieren`: one
click downloads the release binary of the newest tag directly from GitHub, verifies it
against the `SHA256SUMS` of that tag, replaces `~/.local/bin/k-playbook`, and the service
then restarts itself from the new program — the open page follows to the new address.
Nothing is downloaded on startup or during a check, only on that click. If the running
program is equally old or newer, as a build with `make dev-install VERSION=…` above the
newest tag, nothing is installed. The shipped texts need no update in the project at all:
they come from the hub.

By hand it is one call, due once in every environment — on the host as well as in the
devcontainer:

```bash
k-playbook upgrade
```

It is also the way to go if the installation from the interface fails. If
`k-playbook upgrade` itself cannot install, or if `k-playbook` in the `PATH` resolves to a
different file than `~/.local/bin/k-playbook`, the way is the first installation again
(`curl … | bash`, see [Installation](#installation)); in the second case the interface shows
a note with both paths instead of the button, because a program installed there would not be
the one the next call starts. If only commands, instructions or recipes are new, the service
keeps running.

In the terminal, `k-playbook upgrade` does both in one go — first Kephalaion (installed when
missing from the `PATH`, otherwise updated through `kephalaion upgrade --auto` where
Kephalaion considers that safe, with its node restarted; a hub, a global installation or an
older `kephalaion` without `--auto` are only reported), then the release binary when the
running program is older than the newest release tag, then
`AGENTS.md`, `CLAUDE.md`, linking, the missing fields of the `maintenance:` block in
`K-PLAYBOOK.yaml` (with the backup switched off; existing values stay) and MCP entries (a
stale entry is replaced, a missing one added when the file is not tracked by Git), and last
the `README.md` of the
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
the program was updated. For the new commands to arrive, the assistant has to be
restarted once; it reads its list at startup.

## Maintenance

`k-playbook maintain` is the project's maintenance run: named steps, without AI and without
asking, so a cron job can call it later. Without `--only` it runs the daily programme from
`maintenance.daily` in `K-PLAYBOOK.yaml`; `--only <step>[,<step>…]` picks steps, `--dry-run`
shows what would happen without writing, `--commit` asks for a Git commit after a complete
export. Today there is one step, `docs-backup`: when `maintenance.backup.enabled` is `true` it
mirrors `keph:/docs/` through `kephalaion node dir pull … --delete` into a folder of the
project root reserved for it (`keph-backup/docs` by default, marked with `.k-playbook-backup`)
and, when asked, commits only that folder into the Git repository it lies in — never a push;
without a requested commit it needs no Git at all. The backup is off until switched on; in a
project with several people `git.commit` stays `false` and only the person in charge
runs `--commit`. Fields, target checks, exit codes and the upgrade behaviour:
[`k-playbook-format.md`](./defaults/docs/handbook/k-playbook-format.md#maintenance).

```bash
k-playbook maintain --dry-run
k-playbook maintain --only docs-backup --commit
```

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

A release runs in two steps, so that `VERSION` on `main` never points at a tag without
downloads. Installed programs and `bin/install` look at the newest published release, and
CI publishes a release only once all its assets are uploaded: until then they still see the
previous one.

```bash
make release VERSION=v0.2.0          # builds, commits VERSION and SHA256SUMS, pushes the tag
# CI rebuilds, verifies against SHA256SUMS and uploads the assets
make release-publish VERSION=v0.2.0  # brings the same commit onto main
```

The pushes and `gh` calls of `make release` and `make release-publish` run through
`k-playbook github run k-playbook` (see [`defaults/docs/handbook/github-access.md`](./defaults/docs/handbook/github-access.md)); the
service name can be overridden with `GITHUB_SERVICE=…`.

## Core principles

- The installation is the binary alone, one per machine. A project carries its anchor and what it owns, no copy of the tool.
- Shipped and project-owned are strictly separated: what comes with the binary — `~/.local/share/k-playbook/` and the texts from the hub — is replaced by an update, what the project owns is never touched by one.
- Shipped reviews are not edited. A project deviates from a review by overlay: a project-owned recipe of the same name under `keph:/k-playbook/reviews/` replaces completely, an empty one disables.
- Paths are not part of the configuration. They follow from the main directory, the one holding `.k-playbook/K-PLAYBOOK.yaml`.
- Tasks, todos, the project documentation and project-owned review recipes are documents in the project collection; project-owned commands and skills lie under `.k-playbook/`. The location of review results is not decided yet; reviews are paused until then.
- Projects may use their own venvs. Security tools are installed separately from those, host-/user-local or in dedicated k-playbook tool venvs; they are one of the two deliberate exceptions to project locality.
- The base tools k-playbook calls itself — `git`, `curl` or `wget`, `tar`, `python3`, `rg` — are the second exception, and a different one: they are not scanners but the ground the commands stand on, so a missing one warns and never stops a run. `install-base-tools.sh` prefers the distribution package manager and falls back to a user-local release install; k-playbook never escalates to root by itself, it prints the `sudo` command.
- Nothing is written except after confirmation, step by step.

## Documentation

- [`docs/README.md`](./docs/README.md) - the complete documentation index.
- [`defaults/docs/handbook/manual.md`](./defaults/docs/handbook/manual.md) - purpose, core model and standard workflows.
- [`defaults/docs/handbook/k-playbook-format.md`](./defaults/docs/handbook/k-playbook-format.md) - the contract: `K-PLAYBOOK.yaml`, structure, overlay.
- [`defaults/docs/handbook/installation.md`](./defaults/docs/handbook/installation.md) - installing the binary, setup steps, updating, security tools.
- [`defaults/docs/handbook/commands.md`](./defaults/docs/handbook/commands.md) - index of the slash commands.
- [`defaults/docs/handbook/vscode.md`](./defaults/docs/handbook/vscode.md) - the VS Code extension: commands, settings, installation, build.
- [`defaults/docs/handbook/migration.md`](./defaults/docs/handbook/migration.md) - state of the migration, decisions and open points.
