---
description: Check the current work or target path against the effective rule catalog and report whether required follow-ups such as docs sync are done. Defaults to the current directory, or uses [target-dir] if given.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-enforcement

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Check k-playbook enforcement rules for the current project.

This command is the explicit after-the-fact or mid-work check. The matching Skill `ks-enforcement` applies the same rules continuously during implementation.

## Step 1 — Resolve target and paths

From the context output:

- `PROJECT_REPO_ROOT_DIR = project.repoRoot` — the rules are checked against the code,
  not against the playbook directory.
- `DOCS_DIR = <local.dir>/docs`.

- If `$ARGUMENTS` is set, it names the directory to check. It must lie inside
  `PROJECT_REPO_ROOT_DIR`; otherwise stop and say so.
- If `$ARGUMENTS` is empty, check the current working directory.

If `DOCS_DIR` is missing, warn for the docs-sync check but do not invent a default
docs path.

## Step 2 — Load rule files

Take `catalogs.rules` from the context output. It already merges the shipped catalog
with the project-local one and records `origin` per entry — do not list directories
yourself and do not re-derive which entry wins.

Read the files in the order given and skip entries marked `disabled`. If the set is
empty, report this and stop; an empty set is a deliberate project decision, not a
reason to fall back to the shipped catalog.

Print a compact startup summary:

```text
Enforcement-Check
─────────────────────────────
Ziel:         <geprüftes Verzeichnis>
Regeln:       <N> aktiv  (<A> dist, <B> local, <C> override)
Abgeschaltet: <D> (leere lokale Datei) | —
Docs:         k-playbook-local/docs | fehlt
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
