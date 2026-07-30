---
description: Setup or update the k-playbook config file (K-PLAYBOOK.MD) in the current project. Creates the complete fixed project-local k-playbook/ structure and keeps host-global installation separate.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-setup

Install or update the k-playbook configuration in the current project.

**Scope of this command:**

- Run a short host-local install preflight and report whether `/k-install` should be run on this server.
- Detect whether `K-PLAYBOOK.MD` exists at the project root.
- Create the complete fixed project-local structure under `./k-playbook/`.
- Write `K-PLAYBOOK.MD` as the machine-readable config file that later commands read.
- Migrate older managed blocks that stored individual paths such as `tasks: ./tasks` or active/inactive building blocks.

**Out of scope:** changing project code, running reviews, executing tasks, or changing global OpenCode registration. Global OpenCode registration is owned by `/k-install`; `/k-setup` only performs a preflight and reports the status.

Important framing:

- `K-PLAYBOOK.MD` is **not user documentation**. It is a machine-readable config file. Managed by this command.
- The project-local layout is fixed and complete. `/k-setup` must not ask which directories to create.
- All standard project-local k-playbook artifacts live below `./k-playbook/`.
- `K-PLAYBOOK.MD` stores setup metadata, not per-block path values and not active/inactive switches.
- `/k-setup` is the only update/migration path for `K-PLAYBOOK.MD`; do not introduce a separate update command for managed-block format changes.
- `/k-setup` also owns project-wide workflow policy blocks that are not specific to one scanner. In particular, it owns `k-setup-remediation`, which defines how `/k-remediation` is allowed to turn findings into work.
- `repo:` in `K-PLAYBOOK.MD` is fixed to `~/dev/k-playbook`. It is written for visibility and for commands to read, but `/k-setup` must not ask the user for an alternative repo path. If the real repo is elsewhere, `/k-install` or the Devcontainer setup must create a symlink so `~/dev/k-playbook` works.

## Fixed Layout

Always create or ensure:

| Artifact | Type | Fixed path | Purpose |
|---|---|---|---|
| playbook base | directory | `k-playbook/` | project-local k-playbook root |
| tasks | directory | `k-playbook/tasks/` | task files handled by `/k-task-create`, `/k-run` |
| todo | file | `k-playbook/TODO.md` | project todo list handled by `/k-todo` |
| checks | directory | `k-playbook/checks/` | project-specific verification scripts / rules |
| reviews | directory | `k-playbook/reviews/` | review definitions and review artefacts |
| guidelines | directory | `k-playbook/guidelines/` | project styleguides / conventions |
| enforcement | directory | `k-playbook/enforcement/` | project-local enforcement rules |
| docs | directory | `k-playbook/docs/` | curated internal documentation |

Do not ask whether to create these. Empty directories are valid.

## Step 0 — Host install preflight

Before project setup, check whether k-playbook is installed for the current host/server using the fixed path contract:

1. Set the expected playbook repo path to `~/dev/k-playbook`; expand `~` against the current user.
2. If `~/dev/k-playbook` is missing but `/workspaces/k-playbook/commands/k-setup.md` exists, treat this as a Devcontainer path-contract gap and create or instruct creation of `~/dev/k-playbook -> /workspaces/k-playbook`. In non-interactive Devcontainer setup, the setup script may create it automatically.
3. If `~/dev/k-playbook` is missing and current working directory itself is the k-playbook repo, tell the user to move/clone it to `~/dev/k-playbook` or run `/k-install` to create the symlink.
4. Do not read `K-PLAYBOOK.MD` to choose an alternative basis-repo path. Existing non-standard `repo:` values should be migrated back to `~/dev/k-playbook` when the managed block is updated.
5. Check OpenCode command symlinks and `skills.paths` read-only. If missing/outdated, do **not** run install logic here; mention at the end that `/k-install` should be run.

Do not block project setup just because global installation is incomplete.

## Step 1 — Detect current state

Read the current working directory. Check whether `./K-PLAYBOOK.MD` exists (exact filename, uppercase preserved; but also accept `K-PLAYBOOK.md` if present — treat as the same file).

If the command is accidentally run from `<project>/k-playbook/` and the parent contains `K-PLAYBOOK.MD`, switch the project root to the parent and announce that correction.

Supported existing schemas:

- Current schema: `## Setup` with `layout: fixed-project-k-playbook`.
- Legacy schema: `## Bausteine` active/inactive entries.
- Legacy schema: `## Pfade` entries shaped like `- tasks: ./tasks` or `- docs: ./docs`.

For legacy schemas:

- Ignore legacy `base:` and per-block paths for future path derivation. The fixed base is always `./k-playbook`.
- Show a migration table from legacy values to fixed paths, e.g. `tasks ./tasks -> k-playbook/tasks`.
- If a legacy source path exists and the fixed target path does not exist, ask before moving data. Default recommendation: create the fixed target and leave old files untouched unless the user explicitly asks to move.
- Preserve unmanaged content outside managed blocks.

## Step 2 — Status table

Determine for every fixed path whether it already exists.

Present a status table, e.g.:

```text
Pfad                         Status
k-playbook/                  ok
k-playbook/tasks             ok
k-playbook/TODO.md           fehlt
k-playbook/checks            fehlt
k-playbook/reviews           fehlt
k-playbook/guidelines        fehlt
k-playbook/enforcement       fehlt
k-playbook/docs              fehlt
```

If legacy paths exist, include them as migration notes.

Ask one confirmation:

> Ich lege die fehlende feste k-playbook-Struktur an und aktualisiere `K-PLAYBOOK.MD`. Passt das?

Do **not** ask which directories to create.

Do **not** silently overwrite, remove, or move anything the user did not confirm.

## Step 3 — Draft K-PLAYBOOK.MD

Compose the file content with setup metadata:

- `layout: fixed-project-k-playbook`.
- `repo: ~/dev/k-playbook`. Do not ask. Do not preserve older absolute host paths in the managed block.
- `setup-run`: today's date (`YYYY-MM-DD`).
- Preserve unmanaged content from an existing file (anything outside managed sections).
- Preserve or add the optional `k-setup-remediation` managed block. If missing in update mode, ask which Remediation Mode to use:
  - `task-branch-pr` - every correction is planned as a Task/Bundle with branch and PR. Best for production projects.
  - `task-first` - corrections become Tasks/Bundles first; direct fixes only after explicit approval.
  - `direct-allowed` - small safe fixes may be applied directly; larger work becomes Tasks.

Recommended remediation default for production or shared repos: `task-branch-pr`.

Show the draft to the user with:

> Passt das so, oder soll ich etwas anpassen?

Wait for confirmation.

## Step 4 — Execute

After confirmation:

1. Create all fixed directories from the layout table with `mkdir -p`.
2. Apply explicitly confirmed legacy moves, if any.
3. Run standard initialization from Step 4b.
4. Write `K-PLAYBOOK.MD` at the project root using the confirmed content.
5. Print a short summary:
   - Created directories.
   - Created initialization files.
   - Written / updated file: `K-PLAYBOOK.MD`.
   - Host install status: `ok` or `run /k-install`.

## Step 4b — Standard initialization

### `k-playbook/reviews/known-decisions.md`

If missing: create with this skeleton.

```markdown
# Known Decisions

Einträge in dieser Datei dokumentieren bewusste Design-Entscheidungen und bekannte Trade-offs.
Bei Reviews (`/k-review`, `/k-remediation`) werden passende Befunde automatisch als „Akzeptiert (A)"
eingestuft — kein manuelles Durchgehen nötig.

Format je Eintrag: ID (KD-NNN), Kurztitel, Bereich (Datei/Modul/Konzept), Begründung, Datum.

---

<!-- Einträge folgen hier -->
```

The review log `k-playbook/reviews/log.md` is not created here; `/k-review` creates it lazily.

### `k-playbook/TODO.md`

If missing: create with this skeleton.

```markdown
# TODO

```

Existing files are never overwritten.

## Step 5 — Docs- und Memory-Check

After setup, check docs and memory read-only:

| Punkt | Erwartet | Status |
|---|---|---|
| `k-playbook/docs/README.md` vorhanden | Datei existiert | ok / fehlt |
| `k-playbook/docs/README.md` befüllt | >= 20 Zeilen und enthält einen Stichwort-Index-Header | ok / leer |
| Memory registriert | `AGENTS.md` im Projekt-Root vorhanden **und** `opencode.json` (oder `.jsonc`) enthält `instructions` mit `AGENTS.md` und `references.docs` auf `./k-playbook/docs` | ok / fehlt |

If one or more points are not `ok`, give this combined hint:

> Die Docs sind noch nicht (vollständig) aufgesetzt. Vorschlag: **`/k-code2docs`** aufrufen — der Command scannt den Code semantisch, erzeugt eine thematische Doc-Struktur mit Index und registriert `AGENTS.md` + `opencode.json` in einem Rutsch. (Danach OpenCode neu starten.)

`/k-setup` does not run `/k-code2docs` automatically.

## Step 6 — Optionaler CodeQL-Hinweis

At the end, always briefly mention optional CodeQL setup, but do not generate CodeQL config and do not run a follow-up command.

Hinweistext:

> "Optional: Wenn dieses Projekt CodeQL für Security-, Qualitäts- oder Enforcement-Checks nutzen soll, führe als nächsten Schritt `/k-setup-codeql` aus. Der Command fragt GitHub-CodeQL vs. lokale CodeQL-Datenbank separat ab und trägt die Entscheidung in `K-PLAYBOOK.MD` ein."

## Step 7 — Abschluss-Hinweis zur Host-Installation

Am Ende immer kurz den Host-Install-Status nennen:

- Wenn Step 0 ok war:
  > "Host-Installation: ok. Neue oder aktualisierte Commands sind verlinkt. OpenCode ggf. neu starten."
- Wenn Step 0 nicht ok war oder nicht ausgeführt wurde:
  > "Hinweis: Wenn neue `/k-*`-Commands nicht im Autocomplete auftauchen, auf diesem Server einmal `/k-install` ausführen und OpenCode neu starten."

## K-PLAYBOOK.MD format

Exact format written by this command. Everything between the `k-setup:managed:begin` and `k-setup:managed:end` markers is managed by `/k-setup` and may be rewritten. Content outside the markers is preserved on updates.

```markdown
<!--
K-PLAYBOOK config file — verwaltet von /k-setup.
Diese Datei ist keine User-Doku, sondern eine Config-Datei:
Commands (/k-run, /k-task-create, ...) leiten ihre Pfade aus dem festen ./k-playbook/-Layout ab.
-->

# K-PLAYBOOK

<!-- k-setup:managed:begin -->

## Setup

- layout:     fixed-project-k-playbook
- repo:       ~/dev/k-playbook
- setup-run: 2026-07-30

<!-- k-setup:managed:end -->

<!-- k-setup-remediation:managed:begin -->

## Remediation

- mode:           task-branch-pr
- target:         .
- grouping:       true
- quick-wins:     true
- branch-prefix:  remediation/
- pr-required:    true
- direct-fixes:   false
- setup-run:      2026-07-30

<!-- k-setup-remediation:managed:end -->
```

Rules for the managed block:

- `## Setup` lists `layout`, `repo`, and `setup-run`.
- `layout:` must be `fixed-project-k-playbook`.
- `repo:` is the fixed logical repo path `~/dev/k-playbook`; portability is achieved with a symlink when the physical repo lives elsewhere.
- Parsers must still tolerate legacy `## Pfade` and `## Bausteine` blocks during migration, but `/k-setup` rewrites to this format.
- `## Remediation` defines the project workflow for remediation work. `mode:` is required when the block exists. `target:` is the default code/Git root for remediation tasks; use `.` or a project-relative path such as `./app`.

## Notes

The following are explicitly **not** done by this command:

- Executing tasks, reviews, docs generation, or todo management. `/k-setup` only creates the fixed project-local structure and initializes required skeleton files.
- Creating templates, guideline stubs, or example checks inside the new directories. Directories start empty.
- Erzeugen von Docs oder MEMORY-Registrierung — dafür ist `/k-code2docs` zuständig. `/k-setup` prüft nur (Step 5) und verweist.
- Erzeugen oder Ändern von CodeQL-Konfiguration — dafür ist `/k-setup-codeql` zuständig. `/k-setup` weist nur darauf hin.
