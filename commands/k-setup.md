---
description: Check or update the core k-playbook project config (K-PLAYBOOK.yaml) in the current project. Reports missing installer-managed structure, docs/memory and optional follow-up setup.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-setup

Check or update the core k-playbook configuration in the current project.

**Scope of this command:**

- Run a short host-local install preflight and report whether `/k-install` should be run on this server.
- Detect whether `K-PLAYBOOK.yaml` exists at the project root.
- Validate and, after confirmation, normalize the core YAML fields owned by this command.
- Check the fixed project-local `k-playbook/` structure read-only and point to the Installer if it is incomplete.
- Check docs and memory read-only and point to `/k-code2docs` if needed.

**Out of scope:** creating the fixed project-local `k-playbook/` structure, creating `k-playbook/TODO.md`, creating `k-playbook/reviews/known-decisions.md`, choosing or changing Remediation policy, changing project code, running reviews, executing tasks, or changing global OpenCode registration.

The Installer GUI owns initial project onboarding:

- adding a project to the local Installer list,
- creating `K-PLAYBOOK.yaml` when missing,
- creating or completing the fixed `k-playbook/` structure,
- writing the default `remediation:` block,
- changing `remediation.mode` later from the project list.

Important framing:

- `K-PLAYBOOK.yaml` is **not user documentation**. It is a machine-readable config file.
- The project-local layout is fixed. `/k-setup` must not ask which directories to create and must not create them directly.
- All standard project-local k-playbook artifacts live below `./k-playbook/`.
- `K-PLAYBOOK.yaml` stores setup metadata, policies and tool decisions, not per-block path values and not active/inactive switches.
- `k_playbook.repo` in `K-PLAYBOOK.yaml` is fixed to `~/dev/k-playbook`. Do not ask the user for an alternative repo path. If the real repo is elsewhere, `/k-install` or the DevContainer setup must create a symlink so `~/dev/k-playbook` works.
- `remediation:` is consumed by `/k-remediation`, but its default and later mode changes are handled by the Installer GUI. `/k-setup` preserves an existing block and does not ask for a mode.

## Fixed Layout Check

Check these fixed paths read-only:

| Artifact | Type | Fixed path | Owner |
|---|---|---|---|
| playbook base | directory | `k-playbook/` | Installer |
| tasks | directory | `k-playbook/tasks/` | Installer |
| done tasks | directory | `k-playbook/tasks/done/` | Installer |
| todo | file | `k-playbook/TODO.md` | Installer, `/k-todo` may append |
| checks | directory | `k-playbook/checks/` | Installer |
| reviews | directory | `k-playbook/reviews/` | Installer |
| known decisions | file | `k-playbook/reviews/known-decisions.md` | Installer |
| guidelines | directory | `k-playbook/guidelines/` | Installer |
| enforcement | directory | `k-playbook/enforcement/` | Installer |
| docs | directory | `k-playbook/docs/` | Installer, `/k-code2docs` fills content |

If paths are missing, do not create them. Tell the user:

> Projektstruktur unvollstaendig. Oeffne den k-playbook Installer und nutze im Projektblock `Vervollstaendigen`.

## Step 0 - Host install preflight

Before project setup, check whether k-playbook is installed for the current host/server using the fixed path contract:

1. Set the expected playbook repo path to `~/dev/k-playbook`; expand `~` against the current user.
2. If `~/dev/k-playbook` is missing but `/workspaces/k-playbook/commands/k-setup.md` exists, treat this as a DevContainer path-contract gap and create or instruct creation of `~/dev/k-playbook -> /workspaces/k-playbook`. In non-interactive DevContainer setup, the setup script may create it automatically.
3. If `~/dev/k-playbook` is missing and current working directory itself is the k-playbook repo, tell the user to move/clone it to `~/dev/k-playbook` or run `/k-install` to create the symlink.
4. Do not read `K-PLAYBOOK.yaml` to choose an alternative basis-repo path. Existing non-standard `k_playbook.repo` values should be written back to `~/dev/k-playbook` when the config is updated.
5. Check OpenCode command symlinks and `skills.paths` read-only. If missing/outdated, do **not** run install logic here; mention at the end that `/k-install` should be run.

Do not block project setup just because global installation is incomplete.

## Step 1 - Detect current state

Read the current working directory. Check whether `./K-PLAYBOOK.yaml` exists.

If the command is accidentally run from `<project>/k-playbook/` and the parent contains `K-PLAYBOOK.yaml`, switch the project root to the parent and announce that correction.

Supported existing schema:

- YAML schema version `1` with `layout: fixed-project-k-playbook`.

If `K-PLAYBOOK.yaml` is missing, stop and tell the user to add the project with the k-playbook Installer. The Installer creates the file and the fixed structure together.

If `K-PLAYBOOK.yaml` contains unknown top-level fields, preserve them unless they conflict with fields owned by `/k-setup`.

## Step 2 - Status table

Determine read-only status for:

- Core YAML fields.
- Fixed structure paths.
- Docs and memory checks.
- Host install preflight.

Example:

```text
Punkt                        Status
K-PLAYBOOK.yaml              ok
schema_version               ok
layout                       ok
k_playbook.repo              ok
setup.updated_at             alt
Projektstruktur              fehlt: k-playbook/tasks/done
Docs                         fehlt
Memory                       fehlt
Host-Installation            ok
```

Ask for confirmation only if core YAML fields need updating:

> Ich aktualisiere nur die Kernfelder in `K-PLAYBOOK.yaml` und lasse Struktur, Remediation, Tools und unbekannte Felder unveraendert. Passt das?

## Step 3 - Draft K-PLAYBOOK.yaml core update

Prepare an update that owns only these fields:

- `schema_version: 1`.
- `layout: fixed-project-k-playbook`.
- `k_playbook.repo: ~/dev/k-playbook`. Do not ask. Do not preserve older absolute host paths.
- `setup.updated_at`: today's date (`YYYY-MM-DD`).

Preserve without changing:

- unknown non-owned top-level YAML fields,
- `remediation`,
- `tools`, including `tools.codeql`,
- all other project decisions.

Show the draft or concise diff to the user with:

> Passt das so, oder soll ich abbrechen?

Wait for confirmation.

## Step 4 - Execute

After confirmation:

1. Write the updated `K-PLAYBOOK.yaml` at the project root using the confirmed core-field changes.
2. Do not create directories or initialization files.
3. Print a short summary:
   - Updated core YAML fields.
   - Preserved `remediation` and `tools`.
   - Project structure status: `ok` or `Installer: Vervollstaendigen`.
   - Host install status: `ok` or `run /k-install`.

If no core YAML update is required, do not rewrite the file. Print status only.

## Step 5 - Docs- und Memory-Check

After setup, check docs and memory read-only:

| Punkt | Erwartet | Status |
|---|---|---|
| `k-playbook/docs/README.md` vorhanden | Datei existiert | ok / fehlt |
| `k-playbook/docs/README.md` befuellt | >= 20 Zeilen und enthaelt einen Stichwort-Index-Header | ok / leer |
| Memory registriert | `AGENTS.md` im Projekt-Root vorhanden **und** `opencode.json` (oder `.jsonc`) enthaelt `instructions` mit `AGENTS.md` und `references.docs` auf `./k-playbook/docs` | ok / fehlt |

If one or more points are not `ok`, give this combined hint:

> Die Docs sind noch nicht vollstaendig aufgesetzt. Vorschlag: **`/k-code2docs`** aufrufen - der Command scannt den Code semantisch, erzeugt eine thematische Doc-Struktur mit Index und registriert `AGENTS.md` + `opencode.json` in einem Rutsch. Danach OpenCode neu starten.

`/k-setup` does not run `/k-code2docs` automatically.

## Step 6 - Optionaler CodeQL-Hinweis

At the end, always briefly mention optional CodeQL setup, but do not generate CodeQL config and do not run a follow-up command.

Hinweistext:

> Optional: Wenn dieses Projekt CodeQL fuer Security-, Qualitaets- oder Enforcement-Checks nutzen soll, fuehre als naechsten Schritt `/k-setup-codeql` aus. Der Command fragt GitHub-CodeQL vs. lokale CodeQL-Datenbank separat ab und traegt die Entscheidung in `K-PLAYBOOK.yaml` ein.

## Step 7 - Abschluss-Hinweis zur Host-Installation

Am Ende immer kurz den Host-Install-Status nennen:

- Wenn Step 0 ok war:
  > Host-Installation: ok. Neue oder aktualisierte Commands sind verlinkt. OpenCode ggf. neu starten.
- Wenn Step 0 nicht ok war oder nicht ausgefuehrt wurde:
  > Hinweis: Wenn neue `/k-*`-Commands nicht im Autocomplete auftauchen, auf diesem Server einmal `/k-install` ausfuehren und OpenCode neu starten.

## K-PLAYBOOK.yaml core fields

Core fields owned by this command:

```yaml
schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: 2026-07-30
```

Rules for the config:

- `schema_version` must be `1`.
- `layout:` must be `fixed-project-k-playbook`.
- `k_playbook.repo` is the fixed logical repo path `~/dev/k-playbook`; portability is achieved with a symlink when the physical repo lives elsewhere.
- Standard paths are not stored. Commands derive them from `<project>/k-playbook/`.
- `remediation` is not owned by `/k-setup`; use the Installer project list to change `remediation.mode`.
- `tools.codeql` is not owned by `/k-setup`; use `/k-setup-codeql`.

## Notes

The following are explicitly **not** done by this command:

- Creating the fixed `k-playbook/` structure or initialization files. Use the Installer project block `Vervollstaendigen`.
- Asking for or changing Remediation policy. Use the Installer project list.
- Executing tasks, reviews, docs generation, or todo management.
- Creating templates, guideline stubs, or example checks inside the directories.
- Erzeugen von Docs oder MEMORY-Registrierung - dafuer ist `/k-code2docs` zustaendig. `/k-setup` prueft nur and verweist.
- Erzeugen oder Aendern von CodeQL-Konfiguration - dafuer ist `/k-setup-codeql` zustaendig. `/k-setup` weist nur darauf hin.
