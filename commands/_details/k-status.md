---
description: Fast read-only health overview for the current project, backed by the k-playbook context output.
argument-hint: [full|codeql|reviews|json|strict]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-status

Show a fast, read-only health overview for the current project.

This command is a status preflight, not a repair command:

- The context output from the first step is the authoritative base. Everything else is a
  cheap existence or metadata check on top of it.
- Prefer small existence and metadata checks over scans.
- Do not create files, change config, install tools, run tests, run builds, start
  scanners, create CodeQL databases, run CodeQL analysis, upload SARIF, call GitHub APIs,
  or print Git diffs.
- Do not repair anything. Every problem this command finds ends in a recommendation,
  normally `/k-gui`.

## Modes

Interpret `$ARGUMENTS` as one optional mode:

- Empty: compact human-readable report.
- `json`: print the context JSON unchanged.
- `full`: compact report plus the effective catalogs listed entry by entry with origin.
- `strict`: compact report plus a health-gate summary where warnings count as failed gates.
- `codeql`: compact report plus lightweight CodeQL config metadata; do not run CodeQL.
- `reviews`: compact report focused on review status; do not run reviews.

If `$ARGUMENTS` is anything else, print the supported modes and stop without running
deeper checks.

## Step 1 - Base

Take the context output. If the call failed, the directory is not a k-playbook project —
report that and stop. Modes are not target paths; there is nothing else to resolve.

From it, take `project`, `playbook`, `local`, `instructions`, `catalogs`, `remediation`
and `guidelines`.

## Step 2 - Cheap checks

Check existence only — do not read contents beyond what a line count needs:

| Row | Check |
|---|---|
| Projekt | `project.dir`, and whether `project.repoRoot` differs from it |
| Installation | `playbook.dir` exists, contains `bin/k-playbook`, and is unmodified — see below |
| Projekteigenes | `local.dir` exists |
| Instruktionen | how many files `instructions` names, and whether each exists |
| Struktur | which of `tasks/`, `tasks/done/`, `docs/`, `results/`, `guidelines/`, `rules/`, `reviews/`, `checks/`, `priv/` are missing under `local.dir` |
| Docs | `<local.dir>/docs/` — exists, and how many `*.md` besides `README.md` |
| Tasks | `<local.dir>/tasks/*.md` — count, and the lowest-numbered file as the next one |
| TODO | `<local.dir>/TODO.md` — exists, and how many unchecked `- [ ]` lines |
| Reviews | count from `catalogs.reviews`, plus whether `<local.dir>/results/known-decisions.md` and `log.md` exist |
| Regeln | count from `catalogs.rules`, split by origin |
| Checks | count from `catalogs.checks`, split by origin |
| Remediation | `remediation.mode`, and whether `remediation.configured` is true |
| Git | `project.vcs`; if `git`, `git status --short` in `project.repoRoot` |
| Assistenten | whether `.claude/commands`, `.claude/skills`, `.opencode/commands`, `.cursor/commands` exist under `project.dir` and point into `playbook.dir` or `local.dir` |

### The installation must be unmodified

`playbook.dir` is a clone and is replaced on every update, so nothing may be written
there. That rule does not enforce itself, and the failure is silent: if a locally changed
file does not also change upstream, `git pull` runs through cleanly and leaves it in
place. The change then survives every update without ever being reported.

Run in `playbook.dir`:

```bash
git -C <playbook.dir> status --porcelain
git -C <playbook.dir> rev-list --count @{u}..HEAD
```

- Tracked files changed or deleted → `FAIL`. Name them, and give
  `git -C k-playbook checkout -- .` as the fix.
- Untracked files → `WARN`. What the project produces belongs in `local.dir`.
- Local commits (`@{u}..HEAD` greater than zero) → `FAIL`. They block `--ff-only` and
  have to be resolved by hand; do not suggest a command for it.

If the wrapper itself is the broken file, the documented `k-playbook/bin/k-playbook`
call fails. Say so plainly rather than reporting the project as not a k-playbook
project — the host-wide `k-playbook` from `PATH` resolves the same context and is the
way out.

## Compact Output

```text
/k-status
------------------------
Projekt:       /path/to/project (repo: ., git)
Installation:  FAIL, bin/k-playbook lokal veraendert
Projekteigen:  OK, k-playbook-local/
Instruktionen: OK, 2 Dateien
Struktur:      WARN, 2 Verzeichnisse fehlen (priv, guidelines)
Docs:          WARN, noch keine Markdown-Dateien
Tasks:         WARN, 3 offen, naechste: 002-example.md
TODO:          OK, 0 offen
Reviews:       WARN, 8 Rezepte, known-decisions fehlt
Regeln:        OK, 4 aktiv (4 dist)
Checks:        OK, 6 aktiv (6 dist)
Remediation:   OK, task-first
Git:           WARN, dirty (4 geaendert, 1 untracked)
Assistenten:   WARN, .cursor/commands fehlt

Naechste Aktionen:
1. git -C k-playbook checkout -- .
2. /k-gui
3. /k-code2docs
```

Use these labels consistently:

- `OK`: the row was checked and nothing is missing.
- `WARN`: something is missing or empty, but the project remains usable.
- `FAIL`: the context call failed, `playbook.dir` / `local.dir` do not exist, or the
  installation carries local changes.

Derive the recommended actions from the WARN and FAIL rows, most blocking first. Missing
directories and missing assistant links are always `/k-gui`.

## JSON Mode

Print the context JSON unchanged. Do not prepend warnings or prose unless the call
failed.

## Full Mode

Print the compact report, then list the three catalogs entry by entry with origin and
`disabled` marker, in the shape used by every other command:

```text
Regeln (rules): 4 aktiv
  [dist]     codeql
  [override] docs-sync            (ueberlagert k-playbook/rules/docs-sync.md)
  [disabled] tool-install-scope   (leere lokale Datei)
```

Do not print file contents, diffs, logs, task contents, review contents, or docs
contents.

## Strict Mode

- Print the compact report.
- Add `Health-Gates: OK` only when every shown row is OK.
- Add `Health-Gates: FAIL (<warn> warn gates, <fail> fail gates)` when warnings or
  failures exist.
- Do not change exit behavior and do not modify files.

## CodeQL Mode

- Print the compact report.
- Read the optional `tools.codeql` block from `<project.dir>/K-PLAYBOOK.yaml` when
  present. This is the one place where this command reads the config file directly: the
  block is not part of the context output yet. Read nothing else from the file.
- Check configured paths only with existence metadata.
- Do not run `codeql version`, `codeql database create`, `codeql database analyze`,
  uploads, or GitHub API calls.

## Reviews Mode

- Print the compact report.
- List `catalogs.reviews` with origin, and report whether `<local.dir>/results/log.md`
  and `known-decisions.md` exist.
- Do not run review recipes and do not read full review result contents.
