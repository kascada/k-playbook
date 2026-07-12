---
description: Setup or update the k-playbook config file (K-PLAYBOOK.MD) in the current project. Creates the requested playbook directories (tasks, checks, reviews, guidelines, enforcement, docs) and writes a pointer file at project root that later commands read their paths from. Phase 1 only — global install and command adjustments are separate steps.
allowed-tools: [Read, Write, Edit, Bash, Glob]
---

# k-setup

Install or update the k-playbook configuration in the current project.

**Scope of this command (Phase 1):**
- Detect whether `K-PLAYBOOK.MD` exists at the project root.
- If not, ask the user where the playbook directories should live.
- Offer each known building block for activation.
- Create the chosen directories.
- Write `K-PLAYBOOK.MD` as the single pointer file that later commands consult.

**Out of scope (later phases):** adapting the other `/k-*` commands to read paths from `K-PLAYBOOK.MD`; global install checks (`opencode.jsonc`, command symlinks); user-facing project documentation.

Important framing:
- `K-PLAYBOOK.MD` is **not user documentation**. It is a machine-readable pointer/config file. Managed by this command.
- Existing commands and skills are **not** created or modified here. They stay where they are.
- `K-PLAYBOOK.MD` **always** contains the paths, even when they match the defaults, so later tools have one source of truth.

## Known building blocks

The command knows the following building blocks (in this order):

| Key           | Default path   | Purpose (short)                                     |
|---------------|----------------|-----------------------------------------------------|
| `tasks`       | `tasks/`       | Task files handled by `/k-task-create`, `/k-run`    |
| `checks`      | `checks/`      | Project-specific verification scripts / rules       |
| `reviews`     | `reviews/`     | Code-review definitions (orchestrated externally)   |
| `guidelines`  | `guidelines/`  | Project styleguides / conventions                   |
| `enforcement` | `enforcement/` | Global-plus-local enforcement rules                 |
| `docs`        | `docs/`        | Curated internal documentation                      |

## Step 1 — Detect current state

Read the current working directory. Check whether `./K-PLAYBOOK.MD` exists (exact filename, uppercase preserved; but also accept `K-PLAYBOOK.md` if present — treat as the same file).

- **Not present** → fresh setup, continue with Step 2.
- **Present** → update mode, continue with Step 4.

## Step 2 — Fresh setup: choose layout

Ask the user:

> Wo sollen die k-playbook-Verzeichnisse in diesem Projekt liegen?
> 
> (a) Direkt im Projekt-Root (Default)
> (b) In einem Unterverzeichnis — Default-Name: `k-playbook/`
> (c) Anderes Unterverzeichnis — Name angeben

If (a): `base` = `.`
If (b): `base` = `k-playbook`
If (c): read the name (must be a valid single-segment directory name), then `base` = that name.

## Step 3 — Fresh setup: pick building blocks

For each known building block, ask whether to activate it. Present them as a compact list first, then confirm the selection. Default suggestion: activate `tasks`. All others: ask — do not enable silently.

For each activated block:
- Show the computed path (`<base>/<default-path>`).
- Offer to rename it. Empty input → keep default.

Collect the resulting `{ key, path, active }` set. Inactive blocks are still listed in `K-PLAYBOOK.MD` but marked inactive (path = `-`).

Continue with Step 5.

## Step 4 — Update mode: gap analysis

Parse the existing `K-PLAYBOOK.MD`. Extract the `## Pfade` block (see Step 6 for format). Build the current `{ key, path, active }` set.

For each known building block, determine:
- Is it listed in `K-PLAYBOOK.MD`?
- Is it marked active?
- If active: does the referenced directory actually exist on disk?

Present a status table to the user, e.g.:

```
Baustein      Pfad             Status
tasks         ./tasks          ok
checks        —                inaktiv
reviews       ./review         referenziert, aber Verzeichnis fehlt
guidelines    ./guidelines     ok
enforcement   —                nicht in K-PLAYBOOK.MD
docs          ./docs           ok
```

Then ask the user what to do. Offer at least:
- Fehlende Verzeichnisse anlegen (für referenzierte, aber nicht existierende Pfade).
- Bisher nicht aufgeführte Bausteine ergänzen (aktivieren oder inaktiv listen).
- Pfade umbenennen.
- Nichts ändern und beenden.

The user may pick multiple actions or skip. Apply the user's choices to the collected set. Continue with Step 5.

Do **not** silently overwrite or remove anything the user did not confirm.

## Step 5 — Draft K-PLAYBOOK.MD

Compose the file content (see Step 6 for the exact format) with the resulting set of paths and metadata:
- `repo`: absolute path to the k-playbook repository (best-effort; ask the user if unclear).
- `setup-run`: today's date (`YYYY-MM-DD`).
- Preserve unmanaged content from an existing file (anything outside the managed sections — see Step 6).

Show the draft to the user with:

> Passt das so, oder soll ich etwas anpassen?

Wait for confirmation.

## Step 6 — Execute

After confirmation:

1. For each active block whose directory does not yet exist: `mkdir -p` the path.
2. Write `K-PLAYBOOK.MD` at the project root using the confirmed content.
3. Print a short summary:
   - Created directories (list).
   - Written / updated file: `K-PLAYBOOK.MD`.
   - Skipped or inactive blocks (list).

## Step 7 — Docs- und Memory-Check (nur wenn `docs` aktiv)

Falls in Step 3/4 der Baustein `docs` aktiviert wurde: kurzer Zustands-Check am Ende, **nur Prüfen und Hinweisen** — kein automatisches Erzeugen.

Prüfe drei Punkte und zeige sie kompakt:

| Punkt                              | Erwartet                                                | Status |
|------------------------------------|---------------------------------------------------------|--------|
| `<docs>/README.md` vorhanden       | Datei existiert                                         | ok / fehlt |
| `<docs>/README.md` befüllt         | ≥ 20 Zeilen und enthält einen Stichwort-Index-Header    | ok / leer |
| Memory registriert                 | `AGENTS.md` im Projekt-Root vorhanden **und** `opencode.json` (oder `.jsonc`) enthält `instructions` mit `AGENTS.md` und `references.docs` | ok / fehlt |

Wenn einer oder mehrere Punkte nicht `ok` sind, dem User als **einen** kombinierten Hinweis geben:

> Die Docs sind noch nicht (vollständig) aufgesetzt. Vorschlag: **`/k-code2docs`** aufrufen — der Command scannt den Code semantisch, erzeugt eine thematische Doc-Struktur mit Index und registriert `AGENTS.md` + `opencode.json` in einem Rutsch. (Danach OpenCode neu starten.)

Wenn alle drei Punkte `ok` sind: das dem User bestätigen (keine Aktion nötig).

`/k-setup` **führt `/k-code2docs` nicht automatisch aus**. Der User startet das gezielt, wenn er will.

## K-PLAYBOOK.MD format

Exact format written by this command (Phase 1). Everything between the `k-setup:managed:begin` and `k-setup:managed:end` markers is managed by `/k-setup` and may be rewritten. Content outside the markers is preserved on updates.

```markdown
<!--
K-PLAYBOOK config file — verwaltet von /k-setup.
Diese Datei ist keine User-Doku, sondern eine Pointer-/Config-Datei:
sie listet, wo die Playbook-Bausteine in diesem Projekt liegen.
Commands (/k-run, /k-task-create, ...) lesen ihre Pfade hier heraus.
-->

# K-PLAYBOOK

<!-- k-setup:managed:begin -->

## Pfade

- tasks:       ./tasks
- checks:      -
- reviews:     -
- guidelines:  ./guidelines
- enforcement: -
- docs:        ./docs

## Playbook-Quelle

- repo: ~/dev/k-playbook
- setup-run: 2026-07-12

<!-- k-setup:managed:end -->
```

Rules for the managed block:
- `## Pfade` lists every known building block in the fixed order given in the "Known building blocks" table above.
- Active blocks: value is the relative path (e.g. `./tasks` or `./k-playbook/tasks`).
- Inactive blocks: value is `-`.
- Two spaces after the colon, then aligned values (visual only; a parser must accept single space too).
- `## Playbook-Quelle` lists the repo path (as given) and the ISO date of the last setup run.

## Notes on later phases

The following are explicitly **not** done by this command in Phase 1:

- Registering skills or symlinking commands into `~/.config/opencode/`.
- Modifying `/k-run`, `/k-task-create`, or any other command so they read paths from `K-PLAYBOOK.MD`. These are separate follow-up tasks per command.
- Creating templates, guideline stubs, or example checks inside the new directories. Directories start empty.
- Erzeugen von Docs oder MEMORY-Registrierung — dafür ist `/k-code2docs` zuständig. `/k-setup` prüft nur (Step 7) und verweist.
