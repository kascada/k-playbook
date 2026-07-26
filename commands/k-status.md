---
description: Fast read-only health check for K-PLAYBOOK.MD, registered paths, tasks, TODOs, reviews, enforcement, CodeQL, Git, and docs, with compact next-action recommendations.
argument-hint: [full|codeql|reviews|json|strict]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-status

Show a fast, read-only health overview for the current project.

This command is a status preflight, not a repair command:

- Read `K-PLAYBOOK.MD` as the single source of truth for playbook paths and CodeQL decisions.
- Prefer small existence and metadata checks over heavy scans.
- Do not create files, change config, install tools, create CodeQL databases, run CodeQL analysis, upload SARIF, or print Git diffs.
- Keep output compact and grouped by section so more checks can be added later.

## Modes

Interpret `$ARGUMENTS` as one optional mode:

- Empty: compact default report with all sections.
- `full`: run the default report and additionally print `K-PLAYBOOK.MD` in full when it exists; if it is long, summarize first and then include the full content under a separate heading.
- `codeql`: only run target resolution, `K-PLAYBOOK.MD` loading, and the `codeql` section plus recommendations relevant to CodeQL.
- `reviews`: only run target resolution, `K-PLAYBOOK.MD` loading, and the `reviews` section plus recommendations relevant to reviews.
- `json`: best-effort machine-readable JSON output. If producing exact JSON without a script would be too fragile for the current shell environment, print the normal checks as a compact JSON-shaped object and clearly mark this mode as an extension point.
- `strict`: run the same checks as default, but label warnings as failed health gates in the summary. Do not change exit behavior and do not modify the filesystem.

If `$ARGUMENTS` is anything else, print the supported modes and stop without running deeper checks.

## Step 1 — Target And Source

Determine `TARGET_DIR` with `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`:

- If the command receives an explicit target directory in a future extension, resolve it with `realpath` and validate it exists.
- For the current argument set, modes are not target paths; use `TARGET_DIR = realpath(CWD)`.
- Apply the project-local base guard from the shared module: if `TARGET_DIR` has no `K-PLAYBOOK.MD`, but its parent has one and the parent's `base:` resolves to the current directory, correct `TARGET_DIR` to the parent project root and show this correction in the preflight.
- Read `<TARGET_DIR>/K-PLAYBOOK.MD` if present. If missing, record `K_PLAYBOOK_FOUND=false` and continue with the checks that do not require it.
- If `K-PLAYBOOK.MD` exists but `base:` is missing, report it as `FAIL` and recommend `/k-setup`; do not infer a base path.

Determine `PLAYBOOK_REPO` best-effort from `K-PLAYBOOK.MD` `## Playbook-Quelle` → `repo:`, else from the command file location, else `~/dev/k-playbook`. Do not ask in this command; if unclear, report it as `WARN`.

## Step 2 — Parse K-PLAYBOOK.MD

If `<TARGET_DIR>/K-PLAYBOOK.MD` exists, parse only simple metadata from it:

- Managed setup block markers:
  - `<!-- k-setup:managed:begin -->`
  - `<!-- k-setup:managed:end -->`
- Managed CodeQL block markers:
  - `<!-- k-setup-codeql:managed:begin -->`
  - `<!-- k-setup-codeql:managed:end -->`
- Managed Dependabot block markers:
  - `<!-- k-setup-dependabot:managed:begin -->`
  - `<!-- k-setup-dependabot:managed:end -->`
- `## Pfade` entries shaped like `- key: value`.
- `## Playbook-Quelle` entries `repo:` and `setup-run:`.
- `## CodeQL` entries listed in the `codeql` section.
- `## Dependabot` entries listed in the `dependabot` section.

Marker status:

- `OK`: every present managed block has exactly one begin marker and one matching end marker in the correct order.
- `WARN`: a managed block is absent but the file otherwise exists.
- `FAIL`: begin/end counts differ, a marker order is inverted, or markers are duplicated.

If `K-PLAYBOOK.MD` is missing, `playbook` is `FAIL` in default, `strict`, `full`, `codeql`, and `reviews` modes.

If `base:` is missing, `playbook` is also `FAIL`; `/k-setup` is the only migration path that may ask for and write the base value.

## Section: playbook

Report:

- `Projekt`: absolute `TARGET_DIR`.
- `K-PLAYBOOK`: `OK`, `WARN`, or `FAIL` with marker status.
- `repo:` from `## Playbook-Quelle`, if present.
- `setup-run:` from `## Playbook-Quelle`, if present.
- In `full` mode, print or fully summarize `K-PLAYBOOK.MD` after the compact report.

## Section: paths

From `## Pfade`, read at least these keys:

- `base`
- `tasks`
- `todo`
- `checks`
- `reviews`
- `guidelines`
- `enforcement`
- `docs`

Resolve paths as follows:

- Treat missing keys, empty values, and `-` as unset.
- Resolve relative values against `TARGET_DIR`.
- Keep absolute values absolute and mark them as `absolute` in detail output.
- If a resolved path is outside `TARGET_DIR`, mark it as `outside-target` but do not fail solely for that reason.
- If a path itself is a symlink, mark it as `symlink`. Resolve only enough to identify where it points; do not follow it into heavy scans.
- Do not recursively traverse configured directories except where a later section explicitly requires a shallow direct-child count.

Expected type:

- `todo` is a file.
- All other standard keys are directories.

Per entry status:

- `OK`: value is set and the expected file/directory exists.
- `WARN`: value is unset or `-`.
- `FAIL`: value is set but the expected file/directory is missing.

Summarize as counts: `Pfade: OK <n> / WARN <n> / FAIL <n>`.

## Section: tasks

Run only if `tasks:` is set and the directory exists.

Checks:

- Count `.md` files directly inside `tasks:` whose filename starts with a number, for example `002-k-status.md`.
- Ignore `done/` for open tasks.
- Optionally count numbered `.md` files directly inside `tasks:/done/` as completed tasks.
- Determine the next open task by sorting numeric prefixes ascending.

Status:

- `OK`: `tasks:` exists and no open numbered task files are found.
- `WARN`: open numbered task files exist.
- `FAIL`: `tasks:` is set but missing; this is normally already counted in `paths`.
- `WARN`: `tasks:` is unset.

Do not read every task file; filenames are enough for this section.

## Section: todo

Run only if `todo:` is set and the file exists.

Checks:

- Count open Markdown checkboxes with a simple text search for lines containing `- [ ]`.
- If no checkboxes are present, report the file as present and `0 checkboxen`, not as an error.

Status:

- `OK`: file exists and has no open checkbox items.
- `WARN`: file exists and has open checkbox items.
- `FAIL`: `todo:` is set but missing; this is normally already counted in `paths`.
- `WARN`: `todo:` is unset.

## Section: reviews

Run in default, `full`, `strict`, and `reviews` modes.

If `reviews:` is set and exists:

- Count `review-*.md` files directly inside it.
- Check whether `log.md` exists.
- Check whether `known-decisions.md` exists.
- Best-effort due reviews:
  - For each `review-*.md`, read only frontmatter if convenient and extract `interval-weeks`.
  - Read `log.md` only shallowly enough to find a last run date for the review.
  - If `last-run + interval-weeks <= today`, count it as due.
  - If this is too ambiguous from the available data, skip due-date calculation and report only counts plus missing log/decision files.

Status:

- `OK`: review directory exists, review files are present or intentionally empty, and support files exist.
- `WARN`: no review files, missing `log.md`, missing `known-decisions.md`, or due reviews exist.
- `FAIL`: `reviews:` is set but missing; this is normally already counted in `paths`.
- `WARN`: `reviews:` is unset.

In `reviews` mode, include the short list of review filenames and due candidates when available.

## Section: enforcement

Run only if `enforcement:` is set and the directory exists.

Checks:

- Count `.md` rule files directly inside `enforcement:`.
- Show a short list of rule filenames, capped at a small readable number; summarize the remainder if needed.

Status:

- `OK`: path exists and at least one `.md` rule file exists.
- `WARN`: path is unset, missing, or empty. A missing/empty enforcement path is not a `FAIL` because enforcement can be global-only.

## Section: codeql

Run in default, `full`, `strict`, and `codeql` modes.

Parse the CodeQL managed block from `K-PLAYBOOK.MD` when present:

- `enabled`
- `target`
- `github`
- `workflow`
- `local-database`
- `database`
- `languages`
- `queries`
- `setup-run`

Rules:

- Valid values for `github` and `local-database` are `true`, `false`, and `planned`.
- Treat unset, empty, or `-` paths as missing.
- Treat missing `target:` as legacy project-root target `.` and report it as `WARN` only when the project root is not a Git worktree but a nested Git/app root is likely present.
- If `target:` is set, check that the referenced path exists. If it is a Git worktree, use it for the CodeQL/Git-oriented status detail; do not run analysis.
- If `enabled: false`, report CodeQL as disabled and do not do deeper checks beyond marker/config plausibility.
- If `github: true` or `github: planned`, check that `workflow:` is set and that the referenced file exists.
- If `local-database: true` or `local-database: planned`, check that `database:` is set and that the referenced path exists.
- Try `codeql version` only when any of `enabled`, `github`, or `local-database` is `true` or `planned`.
- If `codeql version` fails or the command is unavailable, report `CLI fehlt` as `WARN`, not as a hard failure by itself.

Status:

- `OK`: disabled cleanly, or enabled/planned paths and CLI preflight are plausible.
- `WARN`: enabled/planned but optional pieces are missing, CLI is unavailable, languages are unset, or setup is planned.
- `FAIL`: the CodeQL managed block has contradictory markers, `target:` is set but missing, or an active/planned configured path is set but missing.

Do not run `codeql database create`, `codeql database analyze`, or any upload command.

## Section: dependabot

Run in default, `full`, `strict`, and `reviews` modes.

Parse the optional Dependabot managed block from `K-PLAYBOOK.MD` when present:

- `enabled`
- `target`
- `repo`
- `config`
- `alerts`
- `pull-requests`
- `setup-run`

Rules:

- Treat missing block as `WARN` only when a Dependabot config exists under a nested Git/app root.
- If `enabled: false`, report disabled cleanly and do not query GitHub.
- If `target:` is set, check that it exists. If missing, report `FAIL`.
- If `config:` is set, check that the file exists. If missing while enabled, report `FAIL`.
- If `repo:` is set, use it as the GitHub Dependabot Alerts source. If missing, derive from the GitHub remote of `target:` when possible; otherwise report `WARN`.
- Check `gh --version` and `gh auth status` as lightweight preflight only. Do not call the Dependabot alerts API in `/k-status`.
- `pull-requests: false` is acceptable and should not be reported as a warning when `alerts: true` is set.

Status:

- `OK`: enabled or disabled consistently, target/config exist, repo is known, and `gh` auth is plausible when alerts are enabled.
- `WARN`: block missing, `gh` unavailable/unauthenticated, repo not derivable, alerts planned/unclear, or PRs intentionally disabled with no alerts flag.
- `FAIL`: managed markers contradictory, enabled target/config path missing.

## Section: git

Checks:

- Determine whether `TARGET_DIR` is a Git worktree with `git -C <TARGET_DIR> rev-parse --is-inside-work-tree`.
- If yes, report current branch or detached HEAD.
- Count changed tracked files and untracked files using status metadata only.
- Do not print diffs, patch stats, or file contents.

Status:

- `OK`: Git worktree and clean.
- `WARN`: Git worktree and dirty, or not a Git worktree.

Suggested commands may mention normal project hygiene, but do not recommend a k-playbook command solely for dirty Git state.

## Section: docs

Checks:

- Resolve `docs:` from `K-PLAYBOOK.MD`.
- Check whether the docs directory exists.
- Check whether `<docs>/README.md` exists.
- Check whether `<docs>/libs/README.md` exists.

Status:

- `OK`: docs path and both indexes exist.
- `WARN`: docs path is unset, missing, or one of the index files is missing.
- `FAIL`: only if `docs:` is set to a path that is expected by config but cannot be resolved at all; simple missing docs are normally recommendations, not blockers.

Recommendations for docs:

- Missing `<docs>/README.md` → suggest `/k-code2docs`.
- Missing `<docs>/libs/README.md` while `<docs>/README.md` exists → suggest `/k-tools-scan`.

## Section: recommendations

Derive at most three next actions from the findings.

Priority order:

1. Missing `K-PLAYBOOK.MD` or missing `base:` → `/k-setup`.
2. Missing required or configured playbook paths from `paths` → `/k-setup`.
3. CodeQL active/planned with missing workflow → `/k-setup-codeql`.
4. CodeQL local database active/planned with missing database or CLI → `/k-install-codeql`.
5. Dependabot enabled with missing target/config/repo/auth → `/k-review dependabot-alerts` only after setup is corrected.
6. Open numbered tasks → `/k-run`.
7. Review files exist and are due, or review support files are incomplete → `/k-review`.
8. Missing docs index → `/k-code2docs`.
9. Missing libs index → `/k-tools-scan`.

Do not list more than three commands. Do not recommend commands that would clearly be irrelevant to the current mode; for example, `codeql` mode should not recommend `/k-run` just because tasks exist.

## Output Format

Default and `strict` output should be compact and scanable:

```text
/k-status
────────────────────────
Projekt:       /path/to/project
K-PLAYBOOK:    OK (setup-run 2026-07-20)
Pfade:         OK 7 / WARN 1 / FAIL 0
Tasks:         WARN, 3 offen, nächste: 002-k-status.md
TODO:          OK, 0 offen
Reviews:       WARN, 2 vorhanden, known-decisions fehlt
Enforcement:   OK, 1 Regel
CodeQL:        WARN, target=./omni-gw, enabled=true, github=true workflow fehlt
Dependabot:    OK, target=./omni-gw, repo=koelnmesse-IT/omni-gw, PRs deaktiviert
Git:           WARN, dirty (4 geändert, 1 untracked)
Docs:          WARN, docs/README.md fehlt

Nächste Aktionen:
1. /k-setup-codeql
2. /k-run
3. /k-code2docs
```

Use `OK`, `WARN`, and `FAIL` consistently:

- `OK`: present and plausible.
- `WARN`: optional, planned, unset, dirty, or incomplete but not necessarily broken.
- `FAIL`: `K-PLAYBOOK.MD` missing, configured required paths missing, contradictory managed markers, or active/planned configured CodeQL paths missing.

In `strict` mode, keep the same section lines but add a short health-gate summary, for example:

```text
Health-Gates: FAIL (2 warn gates, 1 fail gate)
```

In `json` mode, produce best-effort JSON with these top-level keys when feasible:

```json
{
  "project": "/path/to/project",
  "mode": "json",
  "playbook": {},
  "paths": {},
  "tasks": {},
  "todo": {},
  "reviews": {},
  "enforcement": {},
  "codeql": {},
  "dependabot": {},
  "git": {},
  "docs": {},
  "recommendations": []
}
```

If exact JSON escaping is too error-prone without a helper script, print a `WARN` line before the object stating that JSON mode is best-effort and should be hardened by a future script.
