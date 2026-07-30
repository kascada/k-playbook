---
description: Fast read-only health check for K-PLAYBOOK.MD, OpenCode symlinks, fixed project-local k-playbook layout, tasks, TODOs, reviews, enforcement, CodeQL, Git, and docs, with compact next-action recommendations.
argument-hint: [full|codeql|reviews|json|strict]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-status

Show a fast, read-only health overview for the current project.

This command is a status preflight, not a repair command:

- Read `K-PLAYBOOK.MD` as the project setup metadata and CodeQL decision source. Project-local paths are derived from the complete fixed `k-playbook/` layout.
- Check host-local OpenCode command symlinks read-only against the resolved k-playbook repo.
- Check the canonical k-playbook repo path contract, including the Devcontainer symlink case.
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
- Apply the fixed-layout guard from the shared module: if `TARGET_DIR` has no `K-PLAYBOOK.MD`, but its parent has one and `TARGET_DIR` is named `k-playbook`, correct `TARGET_DIR` to the parent project root and show this correction in the preflight.
- Read `<TARGET_DIR>/K-PLAYBOOK.MD` if present. If missing, record `K_PLAYBOOK_FOUND=false` and continue with the checks that do not require it.

Set `PLAYBOOK_REPO` to the fixed logical path `~/dev/k-playbook`. Expand `~` against the current process home before checking the filesystem. Read `K-PLAYBOOK.MD` `## Playbook-Quelle` → `repo:` only as validation metadata; expected value is `~/dev/k-playbook`. Do not ask for an alternate repo path in this command.

Canonical path rule:

- Require `repo: ~/dev/k-playbook` in project `K-PLAYBOOK.MD`; `/k-setup` owns writing or migrating that value.
- Absolute host paths such as `/home/kleist/dev/k-playbook` are valid only on that host and should be reported as `WARN` in portable projects, especially when `/workspaces/k-playbook` exists.
- In a Devcontainer, `repo: ~/dev/k-playbook` should resolve to `/home/vscode/dev/k-playbook`, usually a symlink to `/workspaces/k-playbook`.

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
- `## Setup` entries `layout:`, `repo:`, and `setup-run:`.
- Legacy `## Bausteine` or `## Pfade` entries only as migration signals.
- `## CodeQL` entries listed in the `codeql` section.
- `## Dependabot` entries listed in the `dependabot` section.

Marker status:

- `OK`: every present managed block has exactly one begin marker and one matching end marker in the correct order.
- `WARN`: a managed block is absent but the file otherwise exists.
- `FAIL`: begin/end counts differ, a marker order is inverted, or markers are duplicated.

If `K-PLAYBOOK.MD` is missing, `playbook` is `FAIL` in default, `strict`, `full`, `codeql`, and `reviews` modes.

If legacy `## Pfade` or `## Bausteine` is present, report `playbook` as `WARN` and recommend `/k-setup` to migrate the managed block. Do not fail solely because old `base:` is missing.

## Section: playbook

Report:

- `Projekt`: absolute `TARGET_DIR`.
- `K-PLAYBOOK`: `OK`, `WARN`, or `FAIL` with marker status.
- `repo:` from `## Playbook-Quelle`, if present.
- `setup-run:` from `## Playbook-Quelle`, if present.
- In `full` mode, print or fully summarize `K-PLAYBOOK.MD` after the compact report.

## Section: opencode

Run in default, `full`, and `strict` modes. In `codeql` and `reviews` modes, skip this section unless the resolved `PLAYBOOK_REPO` itself is unclear and relevant to the mode.

Check the host-local OpenCode registration read-only:

- `OPENCODE_COMMAND_DIR = ~/.config/opencode/command`.
- `OPENCODE_CONFIG_FILE`: prefer `~/.config/opencode/opencode.jsonc`, else `~/.config/opencode/opencode.json`, else mark config as missing.
- Count files `<PLAYBOOK_REPO>/commands/k-*.md`.
- For each repo command, check whether `<OPENCODE_COMMAND_DIR>/k-*.md` exists.
- If the matching OpenCode entry is a symlink, resolve it and check whether it points to the repo command file.
- If the matching OpenCode entry exists but is not a symlink, report it as `WARN` because `/k-install` cannot safely assume ownership.
- Count broken or stale `k-*.md` symlinks in `OPENCODE_COMMAND_DIR` that point into the resolved `PLAYBOOK_REPO` but whose target no longer exists.
- Check whether `PLAYBOOK_REPO` appears in `skills.paths` when the OpenCode config exists. If JSON/JSONC parsing is too fragile, use a conservative text search for the repo path and report `WARN` when unclear.

Do not create directories, edit config, fix links, or delete stale links.

Status:

- `OK`: command dir exists, every repo command has a symlink pointing to the expected file, no stale owned links were found, and `skills.paths` appears to contain `PLAYBOOK_REPO`.
- `WARN`: command dir missing, some command links are missing/wrong/non-symlink, stale owned links exist, OpenCode config is missing, or `skills.paths` cannot be confirmed.
- `FAIL`: only if the resolved `PLAYBOOK_REPO` has no `commands/k-*.md` files or cannot be read.

Suggested detail format:

```text
OpenCode:      WARN, commands 18/20 verlinkt, 1 falsch, 1 verwaist, skills.paths ok
```

In `full` mode, include a short list of missing, wrong, non-symlink, and stale links, capped at a readable number. Recommend `/k-install` when this section is `WARN` or `FAIL` due to link/config registration issues.

## Section: devcontainer

Run in default, `full`, and `strict` modes when either `/workspaces/k-playbook` exists, `/home/vscode` exists, the current `HOME` is `/home/vscode`, or `TARGET_DIR` is under `/workspaces/`. Otherwise skip this section silently.

Check the Devcontainer path contract read-only:

- `/workspaces/k-playbook/commands/` exists and contains `k-*.md`.
- `~/dev/k-playbook` exists after tilde expansion in the container.
- If `~/dev/k-playbook` is a symlink, `readlink` or equivalent resolution points to `/workspaces/k-playbook`.
- If `K-PLAYBOOK.MD` has `repo: ~/dev/k-playbook`, the resolved path exists and points to the same physical directory as `/workspaces/k-playbook` when possible.
- If `K-PLAYBOOK.MD` has an absolute host path while the Devcontainer mount exists, report a portability `WARN` and recommend changing it to `~/dev/k-playbook` plus the symlink.
- `~/.config/opencode/command/k-install.md` exists and resolves to a command file under the resolved playbook repo or under `/workspaces/k-playbook`.
- `~/.config/opencode/opencode.jsonc` or `.json` contains `skills.paths` with `~/dev/k-playbook` or the resolved equivalent.

Do not create the symlink and do not edit OpenCode config from `/k-status`.

Status:

- `OK`: mount exists, `~/dev/k-playbook` exists and resolves to the mount, command symlink exists, and `skills.paths` is plausible.
- `WARN`: mount exists but symlink/config/command registration is missing or `repo:` is host-absolute instead of portable.
- `FAIL`: Devcontainer appears active but neither `/workspaces/k-playbook` nor the resolved `PLAYBOOK_REPO` contains `commands/k-*.md`.

Suggested detail format:

```text
Devcontainer: OK, ~/dev/k-playbook -> /workspaces/k-playbook, command links ok
```

Recommended fix when the symlink is missing:

```bash
mkdir -p /home/vscode/dev
ln -sfn /workspaces/k-playbook /home/vscode/dev/k-playbook
```

## Section: layout

Check these fixed paths:

- `tasks`
- `todo`
- `checks`
- `reviews`
- `guidelines`
- `enforcement`
- `docs`

If legacy `## Pfade` or `## Bausteine` exists, report a migration warning. Do not use those blocks to decide which paths are expected; all fixed paths are expected.

Resolve fixed paths as follows:

- `PLAYBOOK_BASE_DIR = <TARGET_DIR>/k-playbook`.
- `tasks` -> `k-playbook/tasks`.
- `todo` -> `k-playbook/TODO.md`.
- `checks` -> `k-playbook/checks`.
- `reviews` -> `k-playbook/reviews`.
- `guidelines` -> `k-playbook/guidelines`.
- `enforcement` -> `k-playbook/enforcement`.
- `docs` -> `k-playbook/docs`.
- Do not recursively traverse derived directories except where a later section explicitly requires a shallow direct-child count.

Expected type:

- `todo` is a file.
- All other standard keys are directories.

Per entry status:

- `OK`: expected file/directory exists.
- `FAIL`: expected file/directory is missing.

Summarize as counts: `Layout: OK <n> / WARN <n> / FAIL <n>`.

## Section: tasks

Run only if `k-playbook/tasks` exists.

Checks:

- Count `.md` files directly inside `k-playbook/tasks` whose filename starts with a number, for example `002-k-status.md`.
- Ignore `done/` for open tasks.
- Optionally count numbered `.md` files directly inside `k-playbook/tasks/done/` as completed tasks.
- Determine the next open task by sorting numeric prefixes ascending.

Status:

- `OK`: `k-playbook/tasks` exists and no open numbered task files are found.
- `WARN`: open numbered task files exist.
- `FAIL`: `k-playbook/tasks` is missing; this is normally already counted in `layout`.

Do not read every task file; filenames are enough for this section.

## Section: todo

Run only if `k-playbook/TODO.md` exists.

Checks:

- Count open Markdown checkboxes with a simple text search for lines containing `- [ ]`.
- If no checkboxes are present, report the file as present and `0 checkboxen`, not as an error.

Status:

- `OK`: file exists and has no open checkbox items.
- `WARN`: file exists and has open checkbox items.
- `FAIL`: `k-playbook/TODO.md` is missing; this is normally already counted in `layout`.

## Section: reviews

Run in default, `full`, `strict`, and `reviews` modes.

If `k-playbook/reviews` exists:

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
- `FAIL`: `k-playbook/reviews` is missing; this is normally already counted in `layout`.

In `reviews` mode, include the short list of review filenames and due candidates when available.

## Section: enforcement

Run only if `k-playbook/enforcement` exists.

Checks:

- Count `.md` rule files directly inside `k-playbook/enforcement`.
- Show a short list of rule filenames, capped at a small readable number; summarize the remainder if needed.

Status:

- `OK`: `k-playbook/enforcement` exists and at least one `.md` rule file exists.
- `WARN`: enforcement directory is missing or empty. A missing/empty enforcement path is not a `FAIL` because enforcement can be global-only.

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

- Use `k-playbook/docs`.
- Check whether the docs directory exists.
- Check whether `k-playbook/docs/README.md` exists.
- Check whether `k-playbook/docs/libs/README.md` exists.

Status:

- `OK`: docs path and both indexes exist.
- `WARN`: docs is missing or one of the index files is missing.
- `FAIL`: only if the fixed docs path cannot be checked at all; simple missing docs are normally recommendations, not blockers.

Recommendations for docs:

- Missing `k-playbook/docs/README.md` → suggest `/k-code2docs`.
- Missing `k-playbook/docs/libs/README.md` while `k-playbook/docs/README.md` exists → suggest `/k-tools-scan`.

## Section: recommendations

Derive at most three next actions from the findings.

Priority order:

1. Missing `K-PLAYBOOK.MD` or legacy `## Pfade` schema → `/k-setup`.
2. Missing required fixed-layout paths from `layout` → `/k-setup`.
3. Devcontainer mount/symlink missing while running in a Devcontainer → rebuild/fix the Devcontainer setup script; do not run `/k-setup` for this.
4. OpenCode command symlinks or `skills.paths` incomplete → `/k-install`.
5. CodeQL active/planned with missing workflow → `/k-setup-codeql`.
6. CodeQL local database active/planned with missing database or CLI → `/k-install-codeql`.
7. Dependabot enabled with missing target/config/repo/auth → `/k-review dependabot-alerts` only after setup is corrected.
8. Open numbered tasks → `/k-run`.
9. Review files exist and are due, or review support files are incomplete → `/k-review`.
10. Missing docs index → `/k-code2docs`.
11. Missing libs index → `/k-tools-scan`.

Do not list more than three commands. Do not recommend commands that would clearly be irrelevant to the current mode; for example, `codeql` mode should not recommend `/k-run` just because tasks exist.

## Output Format

Default and `strict` output should be compact and scanable:

```text
/k-status
────────────────────────
Projekt:       /path/to/project
K-PLAYBOOK:    OK (setup-run 2026-07-20)
OpenCode:      WARN, commands 18/20 verlinkt, 1 verwaist, skills.paths ok
Devcontainer: OK, ~/dev/k-playbook -> /workspaces/k-playbook
Layout:        OK 7 / WARN 1 / FAIL 0
Tasks:         WARN, 3 offen, nächste: 002-k-status.md
TODO:          OK, 0 offen
Reviews:       WARN, 2 vorhanden, known-decisions fehlt
Enforcement:   OK, 1 Regel
CodeQL:        WARN, target=./app, enabled=true, github=true workflow fehlt
Dependabot:    OK, target=./app, repo=example-org/example-app, PRs deaktiviert
Git:           WARN, dirty (4 geändert, 1 untracked)
Docs:          WARN, k-playbook/docs/README.md fehlt

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
  "opencode": {},
    "layout": {},
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
