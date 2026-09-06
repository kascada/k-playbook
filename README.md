# k-playbook

`k-playbook` is a toolbox of slash commands, skills, review recipes, rules and checks. It is cloned into a subdirectory of the target project, `<project>/k-playbook/`. The project's own artifacts live next to it. A project is self-contained that way.

## Why k-playbook

- **It learns the project and keeps what it learned.** `/k-docs-code`, `/k-docs-tools`,
  `/k-docs-extract` and `/k-doc-inventory` write project knowledge into
  `k-playbook-local/docs/`; `/k-docs-index` anchors it in `AGENTS.md` and
  `opencode.json`, so the next session starts from it instead of reading the code
  again. Review decisions persist as well: whatever is recorded in
  `known-decisions.md` as a false positive or an accepted risk never shows up as a
  finding in a later run.
- **An indexed body of knowledge instead of code search.** `docs/README.md` is the one
  index across all origins — `code/`, `libs/`, `extracted/`, `versions/`, `manual/` —
  with an alphabetical keyword index and a question→file mapping. The claim: any
  question reaches the right file in at most two lookups.
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
  under `scripts/`; `install-security-tools.sh` installs them, and every job writes
  SARIF into the run directory. What was checked and what was not is stated with a
  reason on the entry.
- **Its own MCP server.** `k-playbook mcp` gives the assistant the resolved paths,
  instructions and catalogs, plus the tools for review runs. It is registered for
  Claude Code, Cursor and OpenCode.

## Installation

k-playbook is cloned into the project it accompanies. There is no central installation
and no fixed host path; every project carries its own.

```bash
cd /path/to/project
git clone git@github.com:kascada/k-playbook.git
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

The last call opens the web interface in the browser. On first run it finds no
`K-PLAYBOOK.yaml` yet and proposes where to create it: the directory above the clone. It
also proposes where the project repository lives — either the main directory itself or a
subdirectory next to it, for instance when the code is checked out alongside the
playbook. Nothing is written before you confirm:

```text
project/
├── K-PLAYBOOK.yaml     the anchor; its location determines the main directory
└── k-playbook/         the installation
```

The same interface then creates the project-owned structure and sets up the linking for
the assistants.

## Directory structure

```text
project/
├── K-PLAYBOOK.yaml       the anchor
├── AGENTS.md             instructions, one source for all assistants
├── CLAUDE.md             include file holding the line @AGENTS.md; the direction is fixed
├── .claude/
│   ├── commands/ ──┐     one symlink per command
│   └── skills/     │     one symlink per skill; OpenCode reads along here
├── .opencode/      │
│   └── commands/ ──┤
├── .cursor/        │
│   └── commands/ ──┤
├── k-playbook/   ←─┤     the installation, fully replaceable
│   ├── commands/ skills/ rules/ reviews/ checks/
│   ├── bin/ scripts/
│   ├── k-playbook.md     shipped instruction layer
│   └── installer/ docs/
└── k-playbook-local/ ←─┘ project-owned, committed
    ├── rules/            overlay onto k-playbook/rules/
    ├── reviews/          overlay onto k-playbook/reviews/
    ├── checks/           overlay onto k-playbook/checks/
    ├── commands/         overlay onto k-playbook/commands/
    ├── skills/           overlay onto k-playbook/skills/
    ├── results/          everything reviews produce; not versioned
    ├── docs/             project knowledge for AI sessions, separated by origin
    │   └── manual/       hand-written docs; no command writes in here
    ├── guidelines/
    ├── tasks/done/
    ├── priv/             notes and work in progress
    ├── material/         raw material as a source for docs, never indexed
    ├── k-playbook.md     project-owned instruction layer
    ├── TODO.md
    └── version-sources.yaml   version sources of the version inventory, hand-maintained
```

`k-playbook-local/` belongs in the project's repository. Three directories inside it are
a matter of choice — `results/`, `priv/` and `material/`; one of them, `results/`, is
already created as private during setup, and all three remain switchable. Review results
are a snapshot from one machine and may contain found secrets in clear text; for `priv/`
and `material/` the project decides, and k-playbook writes no `.gitignore` there on its
own. What currently applies is shown and switched by the **Local settings** block of the
interface — measured with `git check-ignore`, not guessed.

An entry of the same name in `k-playbook-local/` fully replaces the shipped one; an empty
one disables it. This holds for all five kinds — `rules/`, `reviews/`, `checks/`,
`commands/` and `skills/`.

That is why `.claude/commands/` and the other three targets are **real directories with
individual symlinks**, not directory symlinks: only that way do both sources arrive. The
interface compares the resolved catalog with what is registered, names the deviations and
offers to fix them.

What applies in the end is not computed by any command itself:

```bash
k-playbook context
```

Linking is set up for Claude Code, OpenCode and Cursor. Skills exist only once, under
`.claude/skills`, because OpenCode searches that directory as well and Cursor has no
skill concept. `CLAUDE.md` is a small include file holding the line `@AGENTS.md`: Claude
Code reads `CLAUDE.md` exclusively, while OpenCode and Cursor prefer `AGENTS.md` — so
there is exactly one instruction file, and Claude Code loads it through the import.

The direction is the same everywhere. If a project brings only a real `CLAUDE.md`, setup
**renames** it to `AGENTS.md` and creates `CLAUDE.md` anew as an include; the content is
preserved and not duplicated. A `CLAUDE.md` that already carries the line `@AGENTS.md` is
set up — including one with its own house rules alongside. A symlink from an older
version is replaced without loss on the first `k-playbook context`. What cannot be
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

**If `VERSION` changed along the way, the new state comes with a different binary.** The
background service then exits and names the bootstrap; it does not install itself. The
call is the same as for the initial installation and is due once in every environment —
on the host as well as in the devcontainer:

```bash
make -C k-playbook install
```

Without `make`: `k-playbook/bin/install`. A target project has no `install` target of its
own; the call always goes through the clone. If only commands, rules or recipes are new,
`VERSION` does not change and the service keeps running.

The linking for the assistants catches up by itself afterwards — on the next
`k-playbook context`, the call at the start of every session, or on the next look at the
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

## Core principles

- Every project carries its own installation in a subdirectory. No fixed host path, no global symlink.
- Installation and project ownership are strictly separated: `k-playbook/` is fully replaced on every update, everything beside it is never touched.
- Shipped rules, reviews and checks are not edited. A project deviates by overlay: a local file of the same name replaces completely, an empty one disables.
- Paths are not part of the configuration. They follow from the location of `K-PLAYBOOK.yaml`.
- Tasks, reviews and results stay project-owned artifacts under `k-playbook-local/`.
- Projects may use their own venvs. Security tools are installed separately from those, host-/user-local or in dedicated k-playbook tool venvs; they are one of the two deliberate exceptions to project locality.
- The base tools k-playbook calls itself — `git`, `curl` or `wget`, `tar`, `python3`, `rg` — are the second exception, and a different one: they are not scanners but the ground the commands stand on, so a missing one warns and never stops a run. `install-base-tools.sh` prefers the distribution package manager and falls back to a user-local release install; k-playbook never escalates to root by itself, it prints the `sudo` command.
- Nothing is written except after confirmation, step by step.

## Documentation

The documents below are written in German.

- [`docs/README.md`](./docs/README.md) - the complete documentation index.
- [`docs/handbuch.md`](./docs/handbuch.md) - purpose, core model and standard workflows.
- [`docs/k-playbook-format.md`](./docs/k-playbook-format.md) - the contract: `K-PLAYBOOK.yaml`, structure, overlay.
- [`docs/installation.md`](./docs/installation.md) - clone, setup steps, security tools.
- [`docs/commands.md`](./docs/commands.md) - index of the slash commands.
- [`docs/umbau.md`](./docs/umbau.md) - state of the migration, decisions and open points.
