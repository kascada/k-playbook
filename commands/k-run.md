---
description: "Execute one or more task files. If no path is given, uses the tasks: path from K-PLAYBOOK.MD. Pass a single .md file or a directory to override. Multiple tasks are executed in order by their numeric prefix. On success, appends an execution summary and moves the file to done/. On partial execution or error, appends a status note and leaves the file in place."
argument-hint: "[file-or-directory]"
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, Task]
---

# k-run

Execute task files. If `$ARGUMENTS` is empty, use the `tasks:` path from `K-PLAYBOOK.MD`.

`/k-run` does not guess project paths. A project without `K-PLAYBOOK.MD`, without `base:`, or without an active `tasks:` path must be migrated with `/k-setup` first.

## Step 1 - Resolve target path and collect tasks

If `$ARGUMENTS` is provided: treat it as the explicit execution target.

- If it is a single `.md` file: use that file as a one-item list.
- If it is a directory: use that directory.
- If it does not exist: abort with a clear error.

If `$ARGUMENTS` is empty: read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `tasks:` -> `TASKS_DIR`

Also require `base:` from `K-PLAYBOOK.MD`; use `PLAYBOOK_BASE_DIR` only as validation metadata, not to infer `tasks:`.

Command-specific policy:

- If `K-PLAYBOOK.MD` is missing: abort and tell the user to run `/k-setup` first, or pass an explicit file/directory argument for a one-off run.
- If `base:` is missing: abort and tell the user to run `/k-setup` first. Do not infer it from existing paths.
- If `tasks:` is unset or inactive (`-`): abort and tell the user to activate the `tasks` block with `/k-setup`, or pass an explicit file/directory argument for a one-off run.
- If `tasks:` is set but missing on disk: abort and tell the user to run `/k-setup` to create/migrate the configured directories. Do not create it from `/k-run`; there are no tasks to execute.
- If `tasks:` is set and exists: use it as the execution target.

Remember the chosen absolute target as `RUN_TARGET` and the display path as `RUN_TARGET_DISPLAY`.

Collect tasks:

- If `RUN_TARGET` is a file: use that file as a one-item list.
- If `RUN_TARGET` is a directory:
  - Find all `.md` files directly in that directory (not subdirectories).
  - Exclude `done/`, `old/`, and any archived/completed task subdirectories.
  - Sort by the leading number in the filename (e.g. `013-foo.md` before `014-bar.md`; also accept `_`).
  - Skip files without a leading number.
  - If no runnable `.md` files are found: report "Keine offenen Task-Dateien gefunden" and stop.

Announce the list of tasks to be executed before starting. Check the **last** task file for an `## Intent` section and include it in the announcement if present:

```
Tasks:
  Pfad: tasks/
  1. 014-setup-tts.md
  2. 015-integrate-tts.md  <- letzte

Intent (aus letzter Datei):
  <intent text>

Tools (zusätzlich): ...
```

If any task file contains a `## Tools` section: collect all listed tools across all tasks and show them to the user upfront before executing anything. This allows the user to grant additional permissions before the run starts.

## Step 1.5 - k-run-Config aus CLAUDE.md

Collect all CLAUDE.md files that apply to this project (`.claude/CLAUDE.md` in the working directory, plus any `CLAUDE.md` in parent directories up to the repo root). Then run:

```bash
rg -l "## k-run" <collected CLAUDE.md paths>
```

**If the command exits with a non-zero code (no match found):**

Print this hint to the user - exactly as shown, no modifications:

```
Hinweis: In CLAUDE.md kann ein `## k-run`-Abschnitt eingetragen werden,
um Vor- und Nach-Ausführung automatisch Befehle zu rufen (z. B. Backup)
und den Git-Diff automatisch zu erstellen. Beispiel:

  ## k-run

  before: make sichern
  after: make sichern
```

Set `DIFF_ENABLED=false`, `BEFORE_CMD=`, `AFTER_CMD=`. Continue.

**If the command exits with 0 (match found) - read that file and extract the `## k-run` section:**

- Extract `before:` value -> `BEFORE_CMD` (empty if not present)
- Extract `after:` value -> `AFTER_CMD` (empty if not present)
- Check for a git repo: `git -C <working-dir> rev-parse --git-dir 2>/dev/null`
  - If found: record `BASELINE_HASH=$(git rev-parse HEAD)`, set `DIFF_ENABLED=true`
  - If not found: set `DIFF_ENABLED=false`

**If `BEFORE_CMD` is set:** run it now before executing any tasks. If it fails, stop and report the error to the user - do not proceed.

## Step 2 - Execute each task

For each task file, **in strict sequential order** (never parallel - two agents must not modify code simultaneously):

### 2a - Read and understand

Read the task file completely.

### 2b - Clarify before delegating

**Before** spawning the sub-agent: read the task file carefully and identify anything that is unclear, ambiguous, or requires a decision that cannot be inferred from the task description or codebase.

If any such questions exist: **stop and ask the user**. Wait for answers before continuing. Do not skip this step, do not guess, and do not make quick-and-dirty decisions to avoid asking - ambiguities must be resolved in the main context where the user can answer them.

Only proceed once all open questions are resolved.

### 2c - Delegate to sub-agent

Spawn a `general-purpose` sub-agent to carry out the task. Pass it:

- The full content of the task file
- The working directory path
- Instruction to read all `CLAUDE.md` files in the project tree before starting
- All clarifications from Step 2b as additional context
- If `DIFF_ENABLED=true`: the baseline commit hash, with instruction to run `git diff <hash>` after completion and return the diff in its result

The sub-agent must not ask the user questions - by this point all ambiguities are resolved. If the sub-agent encounters an unexpected blocker or decision it cannot cleanly resolve, it must **not** make a quick-and-dirty decision - instead it must stop and report the issue clearly in its result summary so the main agent can escalate to the user.

Wait for the sub-agent to finish before proceeding to the next task.

### 2d - Handle unexpected blockers

If the sub-agent's result reports an unexpected blocker or unresolved decision: **stop and ask the user**. Do not proceed to the next task until resolved. Then decide whether to re-run this task or continue.

### 2e - On error or abort

If the sub-agent reports failure or the task must be aborted:
- Append the following section to the task file:


```
## Ausführung

**Status:** Teilweise ausgeführt - abgebrochen  
**Datum:** <today's date>  
**Abgebrochen bei:** <step or action where it stopped>  
**Grund:** <brief reason>
```

- Leave the file in its current location (do not move to `done/`)
- Stop processing further tasks

### 2f - On success

When the sub-agent reports success:

1. Append the following section to the task file (using the sub-agent's result summary):

```
## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** <today's date>  
**Zusammenfassung:** <2-3 sentences: what was created or done>
```

If `DIFF_ENABLED=true`: run the following after the task completes:

```bash
git diff $BASELINE_HASH --stat
git diff $BASELINE_HASH
```

Append the results to the `## Ausführung` section:

```
**Geänderte Dateien:**
<output of git diff --stat>

**Code-Änderungen:**
<output of git diff, limited to the most relevant parts - truncate large diffs to the key hunks, omit generated files, lockfiles, and binary files>
```

If the diff is longer than ~100 lines: summarize the less important parts in prose and only include the most significant hunks verbatim.

Then invoke the `engineering:code-review` skill, passing **only the diff** as input - no additional file reads or codebase exploration. The review must be strictly limited to the changed lines. Provide the task description as context so the reviewer understands the intent.

Append the review result to `## Ausführung`:

```
**Code-Review:**
<findings from engineering:code-review, based solely on the diff>
```

2. **If `AFTER_CMD` is set:** run it now. If it fails, report the error but do **not** undo the task - it already completed successfully. Just note the failure in the `## Ausführung` section.

3. Ensure `done/` subdirectory exists in the same directory as the task file
4. Move the task file into `done/`

### 2g - Continue

Proceed to the next task in the list. If a task failed (Step 2e), stop - do not execute remaining tasks.

## Step 3 - Intent alignment check

If all tasks completed successfully AND the last task file contains an `## Intent` section: check whether the executed work actually achieves the stated Intent.

Spawn a `general-purpose` subagent (Critic) with this prompt:

```
You are doing a final alignment check after a set of tasks was executed.
Below is the Intent (the goal the tasks were supposed to achieve) and a summary of what was actually done.

## Intent
<insert intent text>

## Was ausgeführt wurde
<insert Ausführung summaries from all task files>

Answer only: Does the executed work achieve the stated Intent?
- Yes -> one sentence why
- Partially -> what is missing or not yet covered
- No -> what is misaligned or missing

Output (no intro text):
| Alignment | Begründung |
```

Append the result to the last task file under `## Ausführung`:

```
**Intent-Alignment:** Ja / Teilweise / Nein - <Begründung>
```

If alignment is **not Yes**: print a clear warning to the user before the final summary:

```
WARNUNG: Intent nicht vollständig erreicht: <Begründung>
```

If no Intent is present: skip this step silently.

## Step 4 - Final summary

After all tasks are processed, print a brief summary:

```
Ausgeführt:   <n> Tasks
Erfolgreich:  <list of filenames>
Abgebrochen:  <filename if any> - <reason>
Übersprungen: <filenames if any>
Intent:       Ja / Teilweise / Nein / - (kein Intent)
```
