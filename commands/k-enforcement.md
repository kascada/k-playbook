---
description: Load global and project-local enforcement rules, check the current work or target path against them, and report whether required follow-ups such as docs sync are done. Defaults to the current directory, or uses [target-dir] if given.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-enforcement

Check k-playbook enforcement rules for the current project.

This command is the explicit after-the-fact or mid-work check. The matching Skill `ks-enforcement` applies the same rules continuously during implementation.

`/k-enforcement` does not guess project paths. The project must have `K-PLAYBOOK.MD` and `base:` configured by `/k-setup`. The `enforcement:` and `docs:` blocks may be inactive; in that case the command uses only global enforcement rules and skips registered-docs path checks rather than inventing default paths.

## Step 1 — Resolve target and paths

Determine `TARGET_DIR`:

- If `$ARGUMENTS` is set: treat it as the target directory, resolve with `realpath`, and abort if it does not exist.
- If `$ARGUMENTS` is empty: `TARGET_DIR = realpath(CWD)`.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `enforcement:` → `PROJECT_ENFORCEMENT_DIR`. Treat this as the resolved absolute path; `-` / missing / empty means unset.
- `docs:` → `DOCS_DIR`. Treat this as the resolved absolute path; `-` / missing / empty means unset.

Also require `base:` from `K-PLAYBOOK.MD`; use it only as validation metadata, not to infer `enforcement:` or `docs:`.

Also set:

- `GLOBAL_ENFORCEMENT_DIR` = `<PLAYBOOK_REPO>/k-playbook/enforcement/`
- If `DOCS_DIR` is unset: leave it unset. Do not default to `<TARGET_DIR>/docs`.

Command-specific policy:

- If `K-PLAYBOOK.MD` is missing: abort and tell the user to run `/k-setup` first.
- If `base:` is missing: abort and tell the user to run `/k-setup` first. Do not infer it from existing paths.
- If `PROJECT_ENFORCEMENT_DIR` is unset or inactive: continue with global rules only.
- If `PROJECT_ENFORCEMENT_DIR` is set but missing: abort and tell the user to run `/k-setup` to create/migrate the configured directory.
- If `DOCS_DIR` is set but missing: warn for docs-sync checks, but do not invent a default docs path.
- If `GLOBAL_ENFORCEMENT_DIR` is missing: abort, because the global rule source cannot be found.

## Step 2 — Load rule files

Collect Markdown files:

- Global: `GLOBAL_ENFORCEMENT_DIR/*.md`
- Project-local: `PROJECT_ENFORCEMENT_DIR/*.md`, if set and exists

Sort each group by filename. Project-local rules do not replace global rules unless a project rule explicitly says so.

De-duplicate rule files by canonical path (`realpath`):

- Load global files first.
- Then load project-local files only if their canonical path was not already loaded.
- If `GLOBAL_ENFORCEMENT_DIR` and `PROJECT_ENFORCEMENT_DIR` resolve to the same directory, report project-local as `identisch mit global` and count each rule once.
- If two different filenames point to the same file via symlink, count it once and mention the skipped duplicate in the startup summary.

If no rule files are found: report this and stop.

Print a compact startup summary:

```text
Enforcement-Check
─────────────────────────────
Ziel:       <TARGET_DIR>
Global:     <N> Regeln aus <GLOBAL_ENFORCEMENT_DIR>
Projekt:    <M> Regeln aus <PROJECT_ENFORCEMENT_DIR> | identisch mit global | —
Docs:       <DOCS_DISPLAY_PATH> | — (inaktiv/nicht registriert)
```

## Step 3 — Determine current change scope

If `TARGET_DIR` is a git repo or inside a git worktree:

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

Use the loaded rules, especially `001-code-docs-sync.md`, and inspect:

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
Regeln geladen:  <N global>, <M projektlokal>
Geprüfter Scope: <files/dirs or "kein Diff">
Relevant:        <rule filenames>

Ergebnis:
- <rule>: ok | offen | unklar | verletzt

Docs-Sync:       angepasst | nicht nötig (<Grund>) | fehlt (<Pfad/Thema>) | unklar
```

If anything is `offen`, `unklar`, or `verletzt`, finish with the concrete next action and ask before editing.
