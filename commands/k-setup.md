---
description: Setup or update the k-playbook config file (K-PLAYBOOK.MD) in the current project. Creates the requested playbook directories/files (tasks, todo, checks, reviews, guidelines, enforcement, docs) and writes a pointer file at project root that later commands read their paths from.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-setup

Install or update the k-playbook configuration in the current project.

**Scope of this command:**
- Run a short host-local install preflight and report whether `/k-install` should be run on this server.
- Detect whether `K-PLAYBOOK.MD` exists at the project root.
- If not, ask the user where the playbook directories/files should live.
- If present, update the managed block in place and apply small schema migrations such as adding missing metadata.
- Offer each known building block for activation.
- Create the chosen directories.
- Write `K-PLAYBOOK.MD` as the single pointer file that later commands consult.

**Out of scope:** changing project code, running reviews, executing tasks, or changing global OpenCode registration. Global OpenCode registration is owned by `/k-install`; `/k-setup` only performs a preflight and reports the status.

Important framing:
- `K-PLAYBOOK.MD` is **not user documentation**. It is a machine-readable pointer/config file. Managed by this command.
- Existing commands and skills are **not** created or modified here. They stay where they are.
- `K-PLAYBOOK.MD` stores the chosen project-local base path as `base:` plus the individual block paths, so commands can use the base path directly when useful and do not need to infer it repeatedly.
- `K-PLAYBOOK.MD` **always** contains the paths, even when they match the defaults, so later tools have one source of truth.
- `/k-setup` is the only update/migration path for `K-PLAYBOOK.MD`; do not introduce a separate update command for managed-block format changes.
- `/k-setup` also owns project-wide workflow policy blocks that are not specific to one scanner. In particular, it owns `k-setup-remediation`, which defines how `/k-remediation` is allowed to turn findings into work.
- `repo:` in `K-PLAYBOOK.MD` is fixed to `~/dev/k-playbook`. It is written for visibility and for commands to read, but `/k-setup` must not ask the user for an alternative repo path. If the real repo is elsewhere, `/k-install` or the Devcontainer setup must create a symlink so `~/dev/k-playbook` works.

## Known building blocks

The command knows the following building blocks (in this order):

| Key           | Type      | Default path   | Purpose (short)                                     |
|---------------|-----------|----------------|-----------------------------------------------------|
| `tasks`       | directory | `tasks/`       | Task files handled by `/k-task-create`, `/k-run`    |
| `todo`        | file      | `TODO.md`      | Project todo list handled by `/k-todo`              |
| `checks`      | directory | `checks/`      | Project-specific verification scripts / rules       |
| `reviews`     | directory | `reviews/`     | Code-review definitions (orchestrated externally)   |
| `guidelines`  | directory | `guidelines/`  | Project styleguides / conventions                   |
| `enforcement` | directory | `enforcement/` | Global-plus-local enforcement rules                 |
| `docs`        | directory | `docs/`        | Curated internal documentation                      |

## Step 0 — Host install preflight

Before project setup, check whether k-playbook is installed for the current host/server using the fixed path contract:

1. Set the expected playbook repo path to `~/dev/k-playbook`; expand `~` against the current user.
2. If `~/dev/k-playbook` is missing but `/workspaces/k-playbook/commands/k-setup.md` exists, treat this as a Devcontainer path-contract gap and create or instruct creation of `~/dev/k-playbook -> /workspaces/k-playbook`. In non-interactive Devcontainer setup, the setup script may create it automatically.
3. If `~/dev/k-playbook` is missing and current working directory itself is the k-playbook repo, tell the user to move/clone it to `~/dev/k-playbook` or run `/k-install` to create the symlink.
4. Do not read `K-PLAYBOOK.MD` to choose an alternative basis-repo path. Existing non-standard `repo:` values should be migrated back to `~/dev/k-playbook` when the managed block is updated.
5. Check OpenCode command symlinks:
   - Source files: `<repo>/commands/k-*.md`
   - Target dir: `~/.config/opencode/command/`
   - For each source file, expected target: `~/.config/opencode/command/<filename>`
6. Check OpenCode skill path:
   - `~/.config/opencode/opencode.jsonc` or `~/.config/opencode/opencode.json`
   - `skills.paths` should include `~/dev/k-playbook`.

If everything is ok: continue silently or with one short line.

If command symlinks or skill path are missing/outdated: do **not** run install logic from here. Continue with project setup and mention at the end that `/k-install` should be run on this server so new commands appear in OpenCode.

Do not block project setup just because global installation is incomplete. `/k-install` is the only command that changes host-global OpenCode registration.

## Step 1 — Detect current state

Read the current working directory. Check whether `./K-PLAYBOOK.MD` exists (exact filename, uppercase preserved; but also accept `K-PLAYBOOK.md` if present — treat as the same file).

- **Not present** → fresh setup, continue with Step 2.
- **Present** → update mode, continue with Step 4.

## Step 2 — Fresh setup: choose layout

Ask the user:

> Wo sollen die k-playbook-Verzeichnisse und Dateien in diesem Projekt liegen?
> 
> (a) Direkt im Projekt-Root (Default)
> (b) In einem Unterverzeichnis — Default-Name: `k-playbook/`
> (c) Anderes Unterverzeichnis — Name angeben

If (a): `base` = `.`
If (b): `base` = `k-playbook`
If (c): read the name (must be a valid single-segment directory name), then `base` = that name.

Normalize the value written to `K-PLAYBOOK.MD` as `base:`:
- `.` -> `.`
- any relative directory -> `./<directory>`
- absolute paths are allowed only if the user explicitly requested one.

## Step 3 — Fresh setup: pick building blocks

For each known building block, ask whether to activate it. Present them as a compact list first, then confirm the selection. Default suggestion: activate `tasks` and `todo`. All others: ask — do not enable silently.

For each activated block:
- Show the computed path (`<base>/<default-path>`).
- Offer to rename it. Empty input → keep default.

Collect the resulting `{ key, path, active }` set. Inactive blocks are still listed in `K-PLAYBOOK.MD` but marked inactive (path = `-`).

Continue with Step 5.

## Step 4 — Update mode: gap analysis

Parse the existing `K-PLAYBOOK.MD`. Extract the `## Pfade` block (see Step 6 for format). Read `base:` if present. Build the current `{ key, path, active }` set.

If `base:` is missing in an existing file, do **not** infer it silently. Ask the user which base should be written. Offer these options:

1. Projektverzeichnis (`.`)
2. Projektverzeichnis/k-playbook (`./k-playbook`) — recommend this when active paths already start with `./k-playbook/`
3. Aktuelles Verzeichnis — only show this when the current working directory differs from the project root and is inside the project
4. Anderes Verzeichnis — user enters a relative or explicitly absolute path

Normalize the selected value as in Step 2 and show that `base:` will be added before writing.

If `base:` is set to a non-root path such as `./k-playbook`, detect legacy root-level default paths and offer to migrate them into the base path:
- For each active block whose path exactly matches its default root path (`./tasks`, `./TODO.md`, `./checks`, `./reviews`, `./guidelines`, `./enforcement`, `./docs`), compute the migrated path as `<base>/<default-path>`.
- Show these as `Pfad passt nicht zu base:` in the status table, with the proposed replacement.
- Do not rewrite custom paths that are not exact default paths unless the user explicitly chooses "Pfade umbenennen".
- If the migrated target already exists, prefer updating only `K-PLAYBOOK.MD` to point to it.
- If only the old root-level path exists, ask before moving files/directories; default recommendation is to update paths only when the target exists, otherwise create missing target directories/files without deleting the old path.

For each known building block, determine:
- Is it listed in `K-PLAYBOOK.MD`?
- Is it marked active?
- If active directory block: does the referenced directory actually exist on disk?
- If active file block: does the referenced file exist, and does its parent directory exist?

Present a status table to the user, e.g.:

```
Baustein      Pfad             Status
tasks         ./tasks          ok
todo          ./TODO.md        ok
checks        —                inaktiv
reviews       ./review         referenziert, aber Verzeichnis fehlt
guidelines    ./guidelines     ok
enforcement   —                nicht in K-PLAYBOOK.MD
docs          ./docs           ok
tasks         ./tasks          Pfad passt nicht zu base: Vorschlag ./k-playbook/tasks
```

Then ask the user what to do. Offer at least:
- Managed-Block aktualisieren / fehlende Metadaten ergänzen (default, includes adding `base:` and applying confirmed base-path migrations).
- Root-Level-Defaultpfade an `base:` anpassen (for example `./tasks` -> `./k-playbook/tasks`).
- Fehlende Verzeichnisse anlegen und fehlende initialisierte Dateien erzeugen (für referenzierte, aber nicht existierende Pfade).
- Bisher nicht aufgeführte Bausteine ergänzen (aktivieren oder inaktiv listen).
- Pfade umbenennen.
- Nichts ändern und beenden.

The user may pick multiple actions or skip. Apply the user's choices to the collected set. Continue with Step 5.

Do **not** silently overwrite or remove anything the user did not confirm.

## Step 5 — Draft K-PLAYBOOK.MD

Compose the file content (see Step 6 for the exact format) with the resulting set of paths and metadata:
- `base`: the project-local playbook base path chosen in Step 2 or preserved/updated in Step 4.
- `repo`: always `~/dev/k-playbook`. Do not ask. Do not preserve older absolute host paths in the managed block.
- `setup-run`: today's date (`YYYY-MM-DD`).
- Preserve unmanaged content from an existing file (anything outside the managed sections — see Step 6).
- Preserve or add the optional `k-setup-remediation` managed block. If missing in update mode, ask which Remediation Mode to use:
  - `task-branch-pr` - every correction is planned as a Task/Bundle with branch and PR. Best for production projects.
  - `task-first` - corrections become Tasks/Bundles first; direct fixes only after explicit approval.
  - `direct-allowed` - small safe fixes may be applied directly; larger work becomes Tasks.

Recommended default for production or shared repos: `task-branch-pr`.

Show the draft to the user with:

> Passt das so, oder soll ich etwas anpassen?

Wait for confirmation.

## Step 6 — Execute

After confirmation:

1. For each active directory block whose directory does not yet exist: `mkdir -p` the path.
2. For each active file block: `mkdir -p` its parent directory if needed.
3. **Baustein-spezifische Initialisierung** — für jeden aktiven Baustein, falls definiert (siehe Step 6b).
4. Write `K-PLAYBOOK.MD` at the project root using the confirmed content.
5. Print a short summary:
   - Created directories (list).
   - Created initialization files (list — see Step 6b).
   - Written / updated file: `K-PLAYBOOK.MD`.
   - Skipped or inactive blocks (list).
   - Host install status: `ok` or `run /k-install`.

## Step 6b — Baustein-spezifische Initialisierung

Manche Bausteine brauchen mehr als nur ein leeres Verzeichnis. Für die folgenden Bausteine legt `/k-setup` beim Aktivieren zusätzliche Dateien an. **Existierende Dateien werden nie überschrieben** — nur angelegt, wenn sie fehlen.

### `reviews`

Wenn `reviews` aktiv ist und `<reviews>/known-decisions.md` noch nicht existiert: Datei mit folgendem Skelett anlegen.

```markdown
# Known Decisions

Einträge in dieser Datei dokumentieren bewusste Design-Entscheidungen und bekannte Trade-offs.
Bei Reviews (`/k-review`, `/k-remediation`) werden passende Befunde automatisch als „Akzeptiert (A)"
eingestuft — kein manuelles Durchgehen nötig.

Format je Eintrag: ID (KD-NNN), Kurztitel, Bereich (Datei/Modul/Konzept), Begründung, Datum.

---

<!-- Einträge folgen hier -->
```

Grund: Sowohl `/k-review` als auch `/k-remediation` erwarten diese Datei an genau dieser Stelle und legen sie selbst **nicht** an — sie warnen nur, wenn sie fehlt. Die Anlage ist damit ausschließlich Aufgabe von `/k-setup`.

Das Review-Log (`<reviews>/log.md`) wird **nicht** hier angelegt — es entsteht lazy, wenn `/k-review` das erste Mal läuft.

### `todo`

Wenn `todo` aktiv ist und die referenzierte Datei noch nicht existiert: Datei mit folgendem Skelett anlegen.

```markdown
# TODO

```

Existierende TODO-Dateien werden nie überschrieben.

### Weitere Bausteine

Aktuell hat kein anderer Baustein eine automatische Initialisierung. Wenn später einer dazukommt, wird er hier ergänzt.

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

## Step 8 — Optionaler CodeQL-Hinweis

Am Ende immer kurz auf das optionale CodeQL-Setup hinweisen, aber **keine** CodeQL-Konfiguration automatisch erzeugen und **keinen** Folge-Command automatisch ausführen.

Hinweistext:

> "Optional: Wenn dieses Projekt CodeQL für Security-, Qualitäts- oder Enforcement-Checks nutzen soll, führe als nächsten Schritt `/k-setup-codeql` aus. Der Command fragt GitHub-CodeQL vs. lokale CodeQL-Datenbank separat ab und trägt die Entscheidung in `K-PLAYBOOK.MD` ein."

Grund: Slash-Commands werden nicht als verlässliche Subroutines verkettet. `/k-setup` bleibt für Basis-Pfade und Initialisierung zuständig; `/k-setup-codeql` übernimmt später die CodeQL-spezifische Konfiguration für Security, Qualität und Enforcement.

## Step 9 — Abschluss-Hinweis zur Host-Installation

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
Diese Datei ist keine User-Doku, sondern eine Pointer-/Config-Datei:
sie listet, wo die Playbook-Bausteine in diesem Projekt liegen.
Commands (/k-run, /k-task-create, ...) lesen ihre Pfade hier heraus.
-->

# K-PLAYBOOK

<!-- k-setup:managed:begin -->

## Pfade

- base:        .
- tasks:       ./tasks
- todo:        ./TODO.md
- checks:      -
- reviews:     -
- guidelines:  ./guidelines
- enforcement: -
- docs:        ./docs

## Playbook-Quelle

- repo: ~/dev/k-playbook
- setup-run: 2026-07-12

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
- setup-run:      2026-07-26

<!-- k-setup-remediation:managed:end -->
```

Rules for the managed block:
- `## Pfade` first lists `base:`, then every known building block in the fixed order given in the "Known building blocks" table above.
- `base:` is the chosen project-local playbook base path. It is active metadata, not a building block; it is never `-`.
- Active blocks: value is the relative path (e.g. `./tasks`, `./TODO.md`, or `./k-playbook/TODO.md`).
- Inactive blocks: value is `-`.
- Two spaces after the colon, then aligned values (visual only; a parser must accept single space too).
- `## Playbook-Quelle` lists the fixed logical repo path `~/dev/k-playbook` and the ISO date of the last setup run. The path is not user-selectable; portability is achieved with a symlink when the physical repo lives elsewhere.
- `## Remediation` defines the project workflow for remediation work. `mode:` is required when the block exists. `target:` is the default code/Git root for remediation tasks; use `.` or a project-relative path such as `./app`.

## Notes

The following are explicitly **not** done by this command:

- Executing tasks, reviews, docs generation, or todo management. `/k-setup` only registers paths and initializes required skeleton files.
- Creating templates, guideline stubs, or example checks inside the new directories. Directories start empty.
- Erzeugen von Docs oder MEMORY-Registrierung — dafür ist `/k-code2docs` zuständig. `/k-setup` prüft nur (Step 7) und verweist.
- Erzeugen oder Ändern von CodeQL-Konfiguration — dafür ist `/k-setup-codeql` zuständig. `/k-setup` weist nur darauf hin.
