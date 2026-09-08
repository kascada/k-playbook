# Commands

Compact index of slash commands. Detailed flows are on dedicated topic pages; this page does not duplicate them.

Shipped commands, skills, rules, review recipes, and checks are under `k-playbook/`. `k-playbook-local/` contains project-owned content: the same five kinds, plus tasks and results. Both locations derive from the location of `K-PLAYBOOK.yaml`; there are no configured paths anymore.

A project-owned command with the name of a shipped command **replaces** it; an empty one disables it. This page lists the shipped commands; the assistants section in the interface shows what actually applies in a particular project.

## Detail pages

| Topic | Detail page |
|---|---|
| PR review | [`pr-review.md`](./pr-review.md) |
| Code review flow | [`code-review.md`](./code-review.md) |
| Task flow | [`task-flow.md`](./task-flow.md) |
| Review, results, and remediation artifacts | [`reviews-and-results.md`](./reviews-and-results.md) |
| Installation | [`installation.md`](./installation.md) |
| Project configuration | [`k-playbook-format.md`](./k-playbook-format.md) |

## Overview

New commands become visible only after linking is in place and the assistant has been restarted. `k-playbook context`, the call at the beginning of every session, brings linking up to date automatically; the restart remains necessary because Claude Code, OpenCode, and Cursor read their command list at startup.

| Command | Purpose | Detail |
|---|---|---|
| **Project** | | |
| `/k-gui` | Start the interface | guides through configuration, project-owned structure, and assistant linking; the Workflows section with its Tasks, Reviews, and Todos pages, plus the Docs and Inventory sections, the last of which shows the version inventory and initiates its collection |
| **Docs** | | |
| `/k-docs` | Check documentation inventory and offer possible actions | read-only status; can dispatch to code, tool, extract, inventory, or index actions |
| `/k-docs-code` | Generate semantic project documentation from the code | writes one file per topic to `k-playbook-local/docs/code/`; the `ks-overlay-repo-analyse` skill also writes there |
| `/k-docs-tools` | Add library/tool documentation | generates one pitfall file per selected tool under `k-playbook-local/docs/libs/` |
| `/k-docs-extract` | Condense raw material from `k-playbook-local/material/` into documentation | writes one file per topic to `k-playbook-local/docs/extracted/`, with source and confidence |
| `/k-doc-inventory` | Collect the version inventory | writes `k-playbook-local/docs/versions/inventory.md` through the `k-playbook inventory` subcommand; the same run is behind "Update" in the interface's "Inventory" section; contract in [`version-inventory.md`](./version-inventory.md), synchronization required for version jumps in `rules/docs-sync.md` |
| `/k-docs-index` | Build the single docs index and register docs for AI sessions | writes `k-playbook-local/docs/README.md`, plus `AGENTS.md` and `opencode.json` (or `opencode.jsonc` if only that exists) |
| **Session** | | |
| `/k-danke` | Close a working session | presents the findings written to `k-playbook-local/material/befunde/` during the work, promotes confirmed ones through `/k-docs-extract`, stores operating pitfalls and checks the docs follow-up via `/k-enforcement`; what gets recorded while working is defined by `k-playbook/rules/befunde.md` and applied by the skill `ks-befunde` |
| **Code review** | | |
| `/k-pr-review` | Load and assess GitHub PRs and optionally approve, merge, or validate them locally | [`pr-review.md`](./pr-review.md) |
| `/k-review` | Run review recipes | [`code-review.md`](./code-review.md) |
| `/k-audit` | Create or continue a complete audit sweep through MCP and triage after the merge | [`review-runs.md`](./review-runs.md) |
| `/k-remediation` | Bundle findings and turn them into tasks or fixes | [`code-review.md`](./code-review.md) |
| **Task flow** | | |
| `/k-task-create` | Create a task file from the conversation context | [`task-flow.md`](./task-flow.md) |
| `/k-task-refine` | Harden task files before execution through a Critic/Editor dialogue | [`task-flow.md`](./task-flow.md) |
| `/k-task-run` | Execute task files sequentially | [`task-flow.md`](./task-flow.md) |
| `/k-todo` | Display or add to `k-playbook-local/TODO.md` | |
| **Helpers** | | |
| `/k-enforcement` | Explicit check against the effective rule set | read-only report; fixes only after approval |
| `/k-test-check` | Run tests and diagnose root causes of failures | deliberately starts tests, not only status checks |
| `/k-verlauf` | Search old AI histories | read-only |
| `/k-vscode-project-color` | Set VS Code window color and title per project | writes `.vscode/settings.json` |

There is no `/k-install-security-tools` command anymore. The interface provides the status and installation command; `k-playbook/scripts/install-security-tools.sh` handles everything else itself. See [`installation.md`](./installation.md#security-tools).

## Review flow

The code-review family is deliberately staged:

| Occasion | Command | Result |
|---|---|---|
| complete security sweep across suitable tools and audit recipes | `/k-audit` | `k-playbook-local/results/YYYY-MM-DD/review-input.json`, `review-input.md`, `review-triage.md` |
| focused individual review recipe, interactive or as a report | `/k-review <name>` | interactive change proposals or `<family>/YYYY-MM-DD/review-input.json` and `review-triage.md` |
| harden a task/instruction file before execution | `/k-task-refine [path]` | review log directly in the reviewed task/instruction file |

1. `/k-pr-review` assesses a specific pull request and remains read-only by default.
2. `/k-review <name>` runs a recipe and, depending on the recipe, produces interactive change proposals or `review-triage.md` as a report handoff.
3. `/k-audit` orchestrates the run model: creates or continues a run, starts scanners, runs evidence recipes before the merge, starts the merge, leads perspectives afterward, and writes `review-triage.md`.
4. `/k-remediation <result>` plans how to address findings. It takes exactly one result file; merging happens exclusively in the audit run.

When `/k-remediation` creates tasks, they belong in the normal task flow: first `/k-task-refine`, then `/k-task-run`.

## Task flow

```text
/k-task-create
/k-task-refine
/k-task-run
```

Tasks arise directly from the conversation or from `/k-remediation`. In both cases, they are reviewed before execution.

## Where commands find their targets

No command reads or guesses a path. Everything derives from the location of `K-PLAYBOOK.yaml`:

| Command | writes to |
|---|---|
| `/k-task-create`, `/k-task-run` | `k-playbook-local/tasks/`, completed tasks to `tasks/done/` |
| `/k-todo` | `k-playbook-local/TODO.md` |
| `/k-review`, `/k-audit` | `k-playbook-local/results/` |
| `/k-docs-code`, skill `ks-overlay-repo-analyse` | `k-playbook-local/docs/code/` |
| `/k-docs-tools` | `k-playbook-local/docs/libs/` |
| `/k-docs-extract` | `k-playbook-local/docs/extracted/` |
| `/k-doc-inventory` | `k-playbook-local/docs/versions/`, plus `k-playbook-local/version-sources.yaml` only after explicit confirmation and exclusively by adding to it |
| `/k-docs-index` | `k-playbook-local/docs/README.md`, plus `AGENTS.md` and `opencode.json` (or `opencode.jsonc`) in the project root |
| `/k-danke`, skill `ks-befunde` | `k-playbook-local/material/befunde/` — the only place any command writes below `material/`; after confirmation also `k-playbook-local/guidelines/betrieb.md` and `k-playbook-local/rules/` |

It additionally reads from `k-playbook/`: rules, recipes, checks, and scripts. It never writes there.

## The resolved working state

No command determines what applies itself. The tool does that:

```bash
k-playbook context
```

The JSON output names the resolved directories, instruction files in read order, remediation policy, guidelines, and the three catalogs, with shipped and project-owned content already merged:

```json
{
  "instructions": [
    "/project/k-playbook/k-playbook.md",
    "/project/k-playbook-local/k-playbook.md"
  ],
  "catalogs": {
    "rules": [
      { "name": "review-authoring.md",   "key": "review-authoring",  "origin": "dist" },
      { "name": "docs-sync.md",          "key": "docs-sync",         "origin": "override" },
      { "name": "my-api-rules.md",       "key": "my-api-rules",      "origin": "local" },
      { "name": "tool-install-scope.md", "key": "tool-install-scope","origin": "override",
        "disabled": true }
    ]
  }
}
```

`origin` is `dist`, `local`, or `override`. `disabled` appears where the project-owned file is empty; that is how to disable a shipped entry.

The call occurs at the beginning of every command, but only once per session. Its output does not change during work and is the same for every command, so subsequent commands reuse it, and the files from `instructions` are read only once. It is retrieved again when `K-PLAYBOOK.yaml` has been written, the inventory of rules, reviews, checks, or guidelines has changed, or work has moved to another project.

`/k-review`, `/k-enforcement`, and `k-check` work on this set and display it before working. The detailed rules are in [`k-playbook-format.md`](./k-playbook-format.md#merge-shipped-and-project-owned-content).

## k-check

`k-playbook/bin/k-check` is not a slash command, but a CLI runner for the effective check set:

```bash
k-playbook/bin/k-check --mode changed
k-playbook/bin/k-check --mode baseline
```

The stable check interface is `.sh`. A check may use Python or something else internally, but must write exactly one status line, `K_CHECK_STATUS=ok|skip|fail`, and optionally `K_CHECK_REASON=<text>`. Details are in [`../checks/README.md`](../checks/README.md).
