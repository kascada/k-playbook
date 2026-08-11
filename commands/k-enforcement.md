---
description: Load the effective enforcement rules (shipped plus project-local, via overlay), check the current work or target path against them, and report whether required follow-ups such as docs sync are done. Defaults to the current directory, or uses [target-dir] if given.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-enforcement

## Erster Schritt

Fuehre zuerst `commands/_shared/context.md` aus: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe.


Check k-playbook enforcement rules for the current project.

This command is the explicit after-the-fact or mid-work check. The matching Skill `ks-enforcement` applies the same rules continuously during implementation.

`/k-enforcement` does not guess project paths. The project must have `K-PLAYBOOK.yaml`; if it is missing, run `k-playbook-installer init`. Project-local rules are read from `paths.enforcement`; docs checks use `paths.docs`.

## Step 1 — Resolve target and paths

Read and apply `<DIST_DIR>/commands/_shared/path-resolution.md`.

- If `$ARGUMENTS` is set, pass it as the explicit directory argument to discovery.
- If `$ARGUMENTS` is empty, discovery starts at the current working directory.

For this command, resolve configured blocks from `K-PLAYBOOK.yaml`:

- `enforcement` -> `RESOLVED_ENFORCEMENT_DIR = <PLAYBOOK_DIR>/<paths.enforcement>`.
- `docs` -> `DOCS_DIR = <PLAYBOOK_DIR>/<paths.docs>`.

The rules are checked against `PROJECT_REPO_ROOT_DIR`, not against `PLAYBOOK_DIR`.

Command-specific policy:

- If `paths.enforcement` or `paths.docs` is missing: ask for the path relative to `PLAYBOOK_DIR`, recommend the conventional value from the shared module, add it to `K-PLAYBOOK.yaml`, then continue.
- If `RESOLVED_ENFORCEMENT_DIR` is missing: warn and continue with the shipped rules only.
- If `DOCS_DIR` is missing: warn for docs-sync checks, but do not invent a default docs path.

## Step 2 — Load rule files

Read and apply `<DIST_DIR>/commands/_shared/overlay-resolution.md` for kind `rules`.

That module combines the shipped catalog `<DIST_DIR>/rules/` with
`<LOCAL_DIR>/rules/`. Do not implement a
separate merge here, and do not de-duplicate by path — the overlay key is the
filename without extension, which already resolves collisions.

A project-local rule **replaces** the shipped rule with the same key; the shipped
file is then not read at all. This is the supported way to deviate, because
`<DIST_DIR>` is read-only and replaced on every update.

Load the effective set sorted by key. If it is empty, report this and stop; do not
fall back to the shipped catalog.

Print a compact startup summary:

```text
Enforcement-Check
─────────────────────────────
Ziel:       <PROJECT_REPO_ROOT_DIR>
Regeln:     <N> aktiv  (<A> dist, <B> local, <C> override)
Abgeschaltet: <D> (leere lokale Datei) | —
Projekt:    <ENFORCEMENT_DISPLAY_PATH> | fehlt
Docs:       <DOCS_DISPLAY_PATH> | fehlt
```

## Step 3 — Determine current change scope

If `PROJECT_REPO_ROOT_DIR` is a git repo or inside a git worktree:

1. Inspect `git status --short`.
2. Inspect the current diff for tracked files (`git diff`) and staged files (`git diff --cached`) if present.
3. Use the changed files as the primary check scope.

If there is no git repo or no diff:

- Ask what should be checked, unless `$ARGUMENTS` points to a specific subdirectory with obvious files.
- For a pure rule-read request, summarize the loaded rules and stop.

Do not modify files in this command unless the user explicitly asks to fix a found issue.

## Step 4 — Check rules

For each loaded rule:

1. Decide whether it is relevant to the current scope.
2. If relevant, check whether the current state satisfies it.
3. If unclear, ask one short question instead of guessing.

Rules apply to new or changed work, not retroactively to the entire legacy codebase, unless a rule explicitly says otherwise.

## Step 5 — Docs-sync check

Always perform this check when code files changed.

Use the loaded rules, especially `docs-sync.md`, and inspect:

- Changed source/config/schema/API files.
- Docs under `DOCS_DIR`, if it exists.
- `README.md`, `AGENTS.md`, and obvious architecture/setup/API docs.

Decide one of:

- **Docs updated:** relevant docs were changed together with code.
- **No docs needed:** explain why in one sentence.
- **Docs missing:** name the likely docs that need an update.
- **Unclear:** ask the user whether to update, create, or intentionally skip docs.

Do not silently pass a code change without one of these outcomes.

## Step 6 — Report

Output:

```text
Enforcement
─────────────────────────────
Regeln geladen:  <N> aktiv (<A> dist, <B> local, <C> override, <D> abgeschaltet)
Geprüfter Scope: <files/dirs or "kein Diff">
Relevant:        <rule filenames>

Ergebnis:
- <rule> [dist|local|override]: ok | offen | unklar | verletzt

Docs-Sync:       angepasst | nicht nötig (<Grund>) | fehlt (<Pfad/Thema>) | unklar
```

If anything is `offen`, `unklar`, or `verletzt`, finish with the concrete next action and ask before editing.
