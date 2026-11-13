# Commands

Compact index of slash commands. Detailed flows are on dedicated topic pages; this page does not duplicate them.

Shipped review recipes and checks are under `k-playbook/`; the shipped commands and skills are unpacked by the binary into `~/.local/share/k-playbook/` and linked through `.k-playbook/share` (source in the repo: `share/`). `k-playbook-local/` contains project-owned content: review recipes, guidelines and instructions, plus results. Checks have no project-owned side since task 134: `k-check` runs only the shipped ones. Both locations derive from the project root, the directory holding `.k-playbook/K-PLAYBOOK.yaml`; there are no configured paths anymore.

A project-owned command with the name of a shipped command **replaces** it; an empty one disables it. This page lists the shipped commands; the assistants section in the interface shows what actually applies in a particular project.

## Detail pages

| Topic | Detail page |
|---|---|
| Code review flow | [`code-review.md`](./code-review.md) |
| Task flow | [`task-flow.md`](./task-flow.md) |
| Installation | [`installation.md`](./installation.md) |
| Project configuration | [`k-playbook-format.md`](./k-playbook-format.md) |

## Overview

New commands become visible only after linking is in place and the assistant has been restarted. `k-playbook briefing`, the call at the beginning of every session, brings linking up to date automatically; the restart remains necessary because Claude Code and OpenCode read their command list at startup.

| Command | Purpose | Detail |
|---|---|---|
| **Project** | | |
| `/k-gui` | Start the interface | guides through configuration, project-owned structure, and assistant linking; the Workflows section with its Tasks, Reviews, and Todos pages, plus the Docs and Inventory sections, the last of which shows the version inventory and initiates its collection |
| `/k-upgrade` | Update the program and pull the project after it | its hull runs `k-playbook upgrade` already when the command is invoked, through the shell embedding `` !`K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade 2>&1` `` (Claude Code and OpenCode, measured 2026-10-25, with the variable 2026-11-11; the variable suppresses the terminal hint to call `/k-upgrade`), so the briefing that follows sees the new state; the text relays the message and runs the call itself only if the embedding did not run: `git pull` in the clone, the binary through `k-playbook/bin/install` only when the running program is older than the clone, then `AGENTS.md` (prompt replaced, superseded blocks removed), `CLAUDE.md`, the linking and the MCP entries, and at the end the `README.md` under `keph:/docs/` from their templates (task 137; only with a ready Kephalaion, at most 10 s, reported as a hint and never an abort, skipped by `--check`); after a binary change the new program performs the project step, and the message ends with the hint to restart the session (`k-playbook upgrade --check` reports what a run would do). The first command whose text lies in `defaults/commands/` — the file under `share/commands/` is a hull with frontmatter and the reference `k-playbook:/commands/k-upgrade.md`, read through `k_playbook_doc_read` or `k-playbook read` (task 092, [verweisschema.md](./verweisschema.md#resolving-k-playbook)). Right after the program's step, and before anything else, the session runs the project's own conversion script `scripts/k-upgrade-umbau.sh` from the project root if it exists — without asking, since it belongs to the project and clears away what the conversions would otherwise check one by one; its output is relayed verbatim, a non-zero exit stops before the conversions until the user decides, and it also runs when the binary changed. After that come the conversions that need judgement and are therefore done by the session, each only when its occasion is present (a conversion without occasion is not reported; when none has one, step 3 is the single line `Umstellungen: alles erledigt.`, since the command runs often and a "nothing to do" per conversion says nothing) and, except for `cache/`, `inbox/`, `queue/` in 3.0 and what 3.4 deletes (the empty directories, plus `guidelines/` and `results/` even with content), each after the user confirms: 3.0 (task 123), which clears the old knowledge store on disk — `cache/`, `inbox/` and `queue/` under `k-playbook-local/` are deleted without checking their content and without asking, one line `gelöscht: <n> Dateien` each, and every file in `knowledge/` except `README.md` and the managed `.gitignore` goes through the procedure 3.2 uses for loose files: `[verschieben] → keph:/docs/manual/<name>.md` (frontmatter rebuilt to `title`, `description`, `sources`, references to `inbox/…` removed) or `[löschen]`, two files with the same proposal `[unklar]`, and `knowledge/` is deleted once only its `README.md`/`.gitignore` remain; like 3.4 it runs only when step 2 reported the program as current; 3.1, which empties `k-playbook-local/material/befunde/` — referenced findings become workbench topics and the references point there, the rest becomes workbench topics, findings in `queue/findings/` (`entstanden: migration`) or is dropped —, 3.2 (task 101), which moves the documentation under `k-playbook-local/docs/` per origin: `versions/` is deleted because its producer writes to `keph:/docs/generated/versions/`, `manual/` is moved to `keph:/docs/manual/` (`create`, compare when the target exists, delete on disk only after a confirmed write), loose files and `extracted/` are proposed per file, the old index `docs/README.md` and the references to it in `AGENTS.md`/`opencode.json[c]` are removed; `code/` is proposed for deletion file by file because `/k-docs-code` now writes to `keph:/docs/generated/code/` (task 109, no move — regenerate with `/k-docs-code`), every reference with the prefix `k-playbook-local/docs/` in `AGENTS.md`/`opencode.json[c]` — FAQ tables, paragraphs built around it, single references, `references.docs` — is proposed for removal, never rewritten; `libs/` is proposed for deletion file by file in the same way because `/k-docs-tools` now writes to `keph:/docs/generated/libs/` (task 120, regenerate with `/k-docs-tools` after the inventory); 3.3 (task 105), which moves task files from `k-playbook-local/tasks/` under the same name to `keph:/user/<user>/tasks/` (a missing `title` is built from the first heading, in double quotes since task 136); and 3.4 (task 107), the cleanup: of the nine directories setup no longer creates — `docs/`, `tasks/`, `tasks/done/`, `data/`, `material/`, `commands/` and `skills/` (task 129), `rules/` (task 135) and `checks/` (task 134) — it deletes, without asking, each that holds nothing but `README.md` files and the managed `.gitignore` — directly under `data/` the renamed leftovers of the todo migration, `todos.json.migriert` and `todos.json.nachzuegler-<date>[-<n>]`, count the same, each on its own (task 112), while a `data/todos.json` not yet taken over keeps `data/` and its line names `k-playbook todo migrate` —, rechecks each right before deleting it, does not read or judge what a `README.md` says, names what lies in the others (`bleibt: …`) as a report, not a question (task 117) — for `commands/` and `skills/` each entry with its target under `.k-playbook/`, for `rules/` the warning that these rules are no longer read, for `checks/` the note that these checks no longer run, since `k-check` runs only the shipped ones —, deletes `guidelines/` (todo #40) and `results/` (task 146) even with content, without asking and without a move, naming every deleted file instead of at most five — project-owned guidelines live under `keph:/docs/guidelines/`, review results can be reproduced from the code, and since the content of `results/` was usually not versioned it cannot be recovered from Git afterwards — and runs only when step 2 reported the program as current, since an older program recreated the directories on "Create". After a binary change the command stops before them; a second run after the restart does them. Conversion 3.6 (task 144) moves the source configuration of the version inventory from `k-playbook-local/version-sources.yaml` into `keph:/k-playbook/inventory.md` (through the shared include `_shared/inventory-config.md`): `exclude`, relative `sources` and `helm_values` are added, an empty `roots: []` is dropped silently, a non-empty `roots`, absolute paths and `helm_values` from a file with `schema_version: 1` are named, not moved; it runs only when `k-playbook briefing --json` in the shell already reports `context.versionSources.ref`, and deletes the old file only after a separate confirmation |
| **Docs** | | |
| `/k-docs` | Report the state of the project documentation in `keph:/docs/` and offer the producers | read-only without a choice: one Kephalaion `list` of `docs` (`recursive`, `frontmatter`) counts inventory, code docs, library profiles, hand-written docs and sources, and reports findings without repairing them — missing `title`/`description`, an old header (fields outside the one frontmatter in `_shared/docs.md`), a `generated.by` that does not match its directory, a producer directory without `README.md`, generated documents without `questions`, dead `keph:` references in `sources`, library profiles read against another version than the inventory now holds, files left under `k-playbook-local/docs/` (→ `/k-upgrade` 3.2) and working directories under `.k-playbook/work/docs/` that never published; the inventory state comes only from its header (`generated.stats`, legacy `inventory.*`). It also counts the open documentation orders in `keph:/queue/docs/` (`k_playbook_doc_list` and `k_playbook_doc_grep`, kind `docs`): one line `Aufträge` split by `code` and `tools` with the oldest `entstanden`, and the number per producer next to `code` and `tools` (task 124). It also checks the `README.md` of every directory with a template against it — `k_playbook_docs_readme` with `check: true`, read-only — and reports the missing (`missing`) and differing (`differs`) ones in a line `README`; directories without a template are not reported, Kephalaion places its reminder there (task 137). Dispatches `inventory`, `code` and `tools` to the texts of `/k-doc-inventory`, `/k-docs-code` and `/k-docs-tools`; `alle` runs them in that order, each with its own preflight, since the profiles build on the inventory, and finally writes every missing or differing README from its template (`k_playbook_docs_readme`). Hull in `share/commands/`, text read as `k-playbook:/commands/k-docs.md` |
| `/k-docs-code` | Generate semantic project documentation from the code | one document per topic in `keph:/docs/generated/code/` of the project collection, written only through `k-playbook docs checkout\|publish code` (MCP `k_playbook_docs_checkout`, `k_playbook_docs_publish`): checkout fetches the directory into `.k-playbook/work/docs/code/`, the session writes there, publish creates, writes and deletes against the revisions of the checkout and reports conflicts instead of overwriting. Scope is `project.repoRoot`, an argument adds one path inside the project; `k-playbook/`, `k-playbook-local/` and `.k-playbook/` are always excluded; the project's permanent exclusions come from `context.docs.exclude` (block `docs.exclude` in `K-PLAYBOOK.yaml`), are shown on their own preflight line `Aus K-PLAYBOOK.yaml` and apply to `hints` sources too, while a path named as the argument is scanned anyway; exclusions the user adds in the preflight are offered for saving after the run and, after confirmation, written by `k_playbook_docs_exclude_add` (`k-playbook docs exclude add`) — the command never reads or edits the file itself (task 116). These scope rules and the one frontmatter of every document under `keph:/docs/` — `title`, `description`, `generated` (`by`, `at`, optional `stats`), `hints`, `questions`, `sources` (`resource`, `item`, `version`), `verified`, nothing beyond — stand once in `defaults/commands/_shared/docs.md`; the scope applies only to `/k-docs-code` and `/k-docs-tools` (task 120). Frontmatter `generated.by: k-docs-code`, `hints` (hand-maintained, changed only after confirmation, `allow_hints`) and `questions`; documents of other writers — the `ks-overlay-repo-analyse` skill writes into the same directory — stay untouched. The preflight lists the open documentation orders with `erzeuger: code` from `keph:/queue/docs/`; after a successful publish (`open` empty, `cleaned` true) exactly those are moved to `queue/docs/done/` with `k_playbook_doc_done`, orders added later stay open (task 124, `k-playbook:/instructions/on-demand/doku-auftraege.md`). Without a ready Kephalaion binding there is no run and no fallback to disk. Hull in `share/commands/`, text read as `k-playbook:/commands/k-docs-code.md` (task 109) |
| `/k-docs-tools` | Write curated, pitfall-oriented profiles of the project's libraries and tools | one document per selected item of the version inventory in `keph:/docs/generated/libs/` of the project collection, written only through `k-playbook docs checkout\|publish libs` (MCP `k_playbook_docs_checkout`, `k_playbook_docs_publish`), like `/k-docs-code`. The candidates come only from `keph:/docs/generated/versions/inventory.md` — every item, its rows merged across sources and environments; without an inventory the command offers `k-playbook inventory` and otherwise does not run, it never scans manifests itself. Scoring A/B/C with the known-pitfall list, one selection message (existing profiles are marked `vorhanden` and not preselected), web research for class A only, at most three fetches per item. File name: the item in lower case, every character outside `a–z 0–9 . _ -` replaced by `-`; `sources` starts with one entry per inventory version (`resource: keph:/docs/generated/versions/inventory.md`, `item`, `version`). Scope and frontmatter come from `_shared/docs.md`; the scope limits only counting usage in the code. Frontmatter `generated.by: k-docs-tools` (legacy `k-tools-scan`). Documentation orders with `erzeuger: tools` are listed in the preflight and closed after a successful publish, as with `/k-docs-code` (task 124). Without a ready Kephalaion binding there is no run and no fallback to disk. Hull in `share/commands/`, text read as `k-playbook:/commands/k-docs-tools.md` (task 120) |
| `/k-doc-inventory` | Collect the version inventory | writes `keph:/docs/generated/versions/inventory.md` in the project collection through the `k-playbook inventory` subcommand, then the directory's `README.md` from its template `k-playbook:/docs/generated/versions/README.md` when it is missing or differs (one line `README:`; if only that fails, e.g. `template_missing`, the inventory is still written and the call ends with exit 1, task 137) — without a ready Kephalaion binding there is no inventory (`kephalaion_unavailable`) and no fallback to disk; hull in `share/commands/`, text read as `k-playbook:/commands/k-doc-inventory.md`; the same run is behind "Update" in the interface's "Inventory" section; contract in [`version-inventory.md`](./version-inventory.md), after a version jump in the current pass the inventory is updated right away through this run, the profiles are queued (`erzeuger: tools`) |
| **Session** | | |
| `/k-danke` | Close a working session | presents the session's findings in the collection's `queue/findings/`, adds what is missing, sets each finding's `ziel` (`wissen`, `falle`, `regel`, `verwerfen`) after the user confirms it, and checks the docs follow-up via `/k-enforcement` as a safety net -- a documentation order of this session in `keph:/queue/docs/` counts as done; it writes nothing else, the takeover from the queue does the rest. What gets recorded while working is defined by `k-playbook:/instructions/on-demand/befunde.md` and applied by the skill `ks-befunde` |
| **Code review** | | |
| `/k-review` | Run review recipes — **temporarily unavailable until about 2026-11-13** while the results storage moves; the command only says so | [`code-review.md`](./code-review.md) |
| `/k-audit` | Create or continue a complete audit sweep through MCP and triage after the merge — **temporarily unavailable until about 2026-11-13** while the results storage moves; the command only says so | [`review-runs.md`](./review-runs.md) |
| `/k-remediation` | Bundle findings and turn them into tasks or fixes — **temporarily unavailable until about 2026-11-13** while the results storage moves; the command only says so | [`code-review.md`](./code-review.md) |
| **Task flow** | | |
| `/k-task-create` | Create a task file from the conversation context | [`task-flow.md`](./task-flow.md) |
| `/k-task-refine` | Harden task files before execution through a Critic/Editor dialogue | [`task-flow.md`](./task-flow.md) |
| `/k-task-run` | Execute task files sequentially; execution and code review run in subagents, a critical finding stops the run | [`task-flow.md`](./task-flow.md) |
| `/k-todo` | List, add, edit, complete, and delete the project todos through `k-playbook todo` | |
| **Helpers** | | |
| `/k-enforcement` | Explicit report on the docs follow-up and `k-check` | determines the changed files of the working tree against `HEAD` including untracked ones and passes the same file set to `<playbook.dir>/bin/k-check --mode changed --files-from …`, which the command runs and reports. Checks the docs follow-up per the always-instruction `k-playbook:/instructions/always/220-regeln-anwenden.md`: hand-written docs adjusted, and for generated docs whether an order of this session lies in `keph:/queue/docs/`; after approval it queues one (`k_playbook_doc_create`, kind `docs`, then Kephalaion `write`). Fixes only after approval. The former check against a rule set is gone with the rule kind (task 135). Hull in `share/commands/`, text read as `k-playbook:/commands/k-enforcement.md` (task 124) |
| `/k-test-check` | Run tests and diagnose root causes of failures | deliberately starts tests, not only status checks |
| `/k-verlauf` | Search old AI histories | read-only |
| `/k-vscode-project-color` | Set VS Code window color and title per project | writes `.vscode/settings.json` |

There is no `/k-install-security-tools` command anymore. The interface provides the status and installation command; `~/.local/share/k-playbook/scripts/install-security-tools.sh` handles everything else itself. See [`installation.md`](./installation.md#security-tools).

## Review flow

The code-review family is deliberately staged:

| Occasion | Command | Result |
|---|---|---|
| complete security sweep across suitable tools and audit recipes | `/k-audit` | `k-playbook-local/results/YYYY-MM-DD/review-input.json`, `review-input.md`, `review-triage.md` |
| focused individual review recipe, interactive or as a report | `/k-review <name>` | interactive change proposals or `<family>/YYYY-MM-DD/review-input.json` and `review-triage.md` |
| harden a task/instruction file before execution | `/k-task-refine [path]` | review log directly in the reviewed task/instruction file |

1. `/k-review <name>` runs a recipe and, depending on the recipe, produces interactive change proposals or `review-triage.md` as a report handoff.
2. `/k-audit` orchestrates the run model: creates or continues a run, starts scanners, runs evidence recipes before the merge, starts the merge, leads perspectives afterward, and writes `review-triage.md`.
3. `/k-remediation <result>` plans how to address findings. It takes exactly one result file; merging happens exclusively in the audit run.

When `/k-remediation` creates tasks, they belong in the normal task flow: first `/k-task-refine`, then `/k-task-run`.

## Task flow

```text
/k-task-create
/k-task-refine
/k-task-run
```

Tasks arise directly from the conversation or from `/k-remediation`. In both cases, they are reviewed before execution.

## Where commands find their targets

No command reads or guesses a path. Everything derives from the project root, the directory holding `.k-playbook/K-PLAYBOOK.yaml`:

| Command | writes to |
|---|---|
| `/k-task-create`, `/k-task-run` | `keph:/user/<user>/tasks/` in the project collection, completed tasks to `done/` below it (`k_playbook_doc_done`); nothing on disk. `/k-task-run` also keeps one ref `refs/k-task-run/<task>` per task in the execution root until the task moves to `done/` |
| `/k-todo` | `todos/` in the project collection, written only through `k-playbook todo` or the MCP tools |
| `/k-review`, `/k-audit` | `k-playbook-local/results/` |
| `/k-docs-code`, skill `ks-overlay-repo-analyse` | `keph:/docs/generated/code/` in the project collection, only through `k-playbook docs publish code`; in between the machine-local working directory `.k-playbook/work/docs/code/` with the checkout record next to it (`/work` is git-ignored), removed after a complete publish together with `.k-playbook/work/docs/` and `.k-playbook/work/` when they are left empty; closed documentation orders move to `keph:/queue/docs/done/` |
| `/k-docs-tools` | `keph:/docs/generated/libs/` in the project collection, only through `k-playbook docs publish libs`; in between the machine-local working directory `.k-playbook/work/docs/libs/`, removed after a complete publish like the one of `/k-docs-code`; closed documentation orders move to `keph:/queue/docs/done/`; after confirmation also the inventory through `k-playbook inventory` when it is missing |
| `/k-doc-inventory` | `keph:/docs/generated/versions/inventory.md` and the directory's `README.md` (from its template) in the project collection, through `k-playbook inventory`; plus the source configuration `keph:/k-playbook/inventory.md` only after explicit confirmation and exclusively by adding to it |
| `/k-docs alle` | besides what the producers write, the `README.md` under `keph:/docs/` that are missing or differ from their templates, through `k-playbook docs readme` |
| `/k-danke`, skill `ks-befunde` | `keph:/queue/findings/` in the project collection, one document per finding; no command writes below `material/` any more |

It additionally reads from `k-playbook/`: recipes, checks, and scripts. It never writes there.

## The resolved working state

No command determines what applies itself. The tool does that:

```bash
k-playbook context
```

The JSON output names the resolved directories, instruction files in read order, remediation policy, guidelines, and the catalogs `reviews` and `checks`, with shipped and project-owned reviews already merged; `checks` lists only the shipped checks (task 134):

```json
{
  "instructions": [
    "/project/k-playbook/k-playbook.md",
    "/project/k-playbook-local/k-playbook.md"
  ],
  "catalogs": {
    "reviews": [
      { "name": "review-code.md",  "key": "code",  "origin": "dist" },
      { "name": "review-tech.md",  "key": "tech",  "origin": "override" },
      { "name": "review-api.md",   "key": "api",   "origin": "local" },
      { "name": "review-sast.md",  "key": "sast",  "origin": "override",
        "disabled": true }
    ]
  }
}
```

`origin` is `dist`, `local`, or `override`. `disabled` appears where the project-owned file is empty; that is how to disable a shipped entry. For `checks`, `origin` is always `dist` and nothing is disabled: checks have no project-owned side.

The call occurs at the beginning of every command, but only once per session. Its output does not change during work and is the same for every command, so subsequent commands reuse it, and the files from `instructions` are read only once. It is retrieved again when `K-PLAYBOOK.yaml` has been written, the inventory of reviews, checks, or guidelines has changed, or work has moved to another project.

`/k-review`, `/k-enforcement`, and `k-check` work on this set and display it before working. The detailed rules are in [`k-playbook-format.md`](./k-playbook-format.md#merge-shipped-and-project-owned-content).

## k-check

`k-playbook/bin/k-check` is not a slash command, but a CLI runner for the effective check set:

```bash
k-playbook/bin/k-check --mode changed
k-playbook/bin/k-check --mode baseline
```

The stable check interface is `.sh`. A check may use Python or something else internally, but must write exactly one status line, `K_CHECK_STATUS=ok|skip|fail`, and optionally `K_CHECK_REASON=<text>`. Details are in [`../checks/README.md`](../checks/README.md).

Among the shipped checks, `check_command_tool_guards.sh` enforces the guard requirement for external tools on the commands themselves: a tool from `scripts/base-tools.tsv` called at the start of a command in a shell code block needs a local `if command -v <tool>` guard or a `<tool> … || …` fallback. Deliberately deferred call sites are listed one by one, with file, line, tool, and reason, in `command-tool-guard-exceptions.tsv` next to the effective check; a stale entry fails the check. It reads the hulls under `share/commands/` and, since task 108, the command texts under `defaults/commands/`, each root with its own key. `check_verweise.sh` reports relative links and bare paths where a reference is meant, in the shipped and project-own commands, instructions, rules, guidelines and skills, following `k-playbook:/instructions/on-demand/verweise-nachziehen.md`; exceptions are in `verweis-exceptions.tsv` (see `checks/README.md`). `/k-enforcement` runs `k-check --mode changed` over the same file set as its rule check.
