---
description: Execute one or more task files. Pass a single .md file or a directory. Multiple tasks are executed in order by their numeric prefix. On success, appends an execution summary and moves the file to done/. On partial execution or error, appends a status note and leaves the file in place.
argument-hint: <file-or-directory>
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-run

Execute task files described in `$ARGUMENTS`.

## Step 1 — Collect tasks

If `$ARGUMENTS` is a single `.md` file: use that file as a one-item list.

If `$ARGUMENTS` is a directory:
- Find all `.md` files directly in that directory (not subdirectories)
- Sort by the leading number in the filename (e.g. `013_foo.md` before `014_bar.md`)
- Skip files without a leading number

Announce the list of tasks to be executed before starting. Check the **last** task file for an `## Intent` section and include it in the announcement if present:

```
Tasks:
  1. 014-setup-tts.md
  2. 015-integrate-tts.md  ← letzte

Intent (aus letzter Datei):
  <intent text>

Tools (zusätzlich): ...
```

If any task file contains a `## Tools` section: collect all listed tools across all tasks and show them to the user upfront before executing anything. This allows the user to grant additional permissions before the run starts.

## Step 1.5 — Git-Diff-Option

Check whether the working directory (or any parent) contains a `.git` folder:

```bash
git -C <working-dir> rev-parse --git-dir 2>/dev/null
```

If **no git repository** is found: skip this step silently, set `DIFF_ENABLED=false`.

If a **git repository is found**:

1. Ask the user:
   > "Git-Repository gefunden. Soll nach der Ausführung ein Diff der Änderungen in die Auftragsdatei geschrieben werden?"

2. If **no**: set `DIFF_ENABLED=false`, continue.

3. If **yes**:
   - Ask: "Wurde bereits ein Commit als Baseline gemacht? (Falls nicht, bitte jetzt committen und dann bestätigen.)"
   - Wait for confirmation.
   - Record the current HEAD: `BASELINE_HASH=$(git rev-parse HEAD)`
   - Set `DIFF_ENABLED=true`

## Step 2 — Execute each task

For each task file, in order:

### 2a — Read and understand

Read the task file completely. Understand what needs to be created or done (code, config, HTML, infrastructure changes, etc.).

### 2b — Execute

Carry out the described work in the main context. Spawn subagents for heavy implementation work (writing files, generating code) where it keeps the main context manageable — but keep coordination, decisions, and user interaction in the main context.

### 2c — Ask when needed

If anything is unclear or a decision is needed that cannot be inferred: **stop and ask the user**. Wait for the answer before continuing. Do not guess or skip.

### 2d — On error or abort

If execution fails or must be aborted mid-task:
- Note which step was reached
- Append the following section to the task file:

```
## Ausführung

**Status:** Teilweise ausgeführt — abgebrochen  
**Datum:** <today's date>  
**Abgebrochen bei:** <step or action where it stopped>  
**Grund:** <brief reason>
```

- Leave the file in its current location (do not move to `done/`)
- Stop processing further tasks

### 2e — On success

When a task is fully completed:

1. Append the following section to the task file:

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
<output of git diff, limited to the most relevant parts — truncate large diffs to the key hunks, omit generated files, lockfiles, and binary files>
```

If the diff is longer than ~100 lines: summarize the less important parts in prose and only include the most significant hunks verbatim.

Then invoke the `engineering:code-review` skill, passing **only the diff** as input — no additional file reads or codebase exploration. The review must be strictly limited to the changed lines. Provide the task description as context so the reviewer understands the intent.

Append the review result to `## Ausführung`:

```
**Code-Review:**
<findings from engineering:code-review, based solely on the diff>
```

2. Ensure `done/` subdirectory exists in the same directory as the task file
3. Move the task file into `done/`

### 2f — Continue

Proceed to the next task in the list. If a task failed (Step 2d), stop — do not execute remaining tasks.

## Step 3 — Intent alignment check

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
- Yes → one sentence why
- Partially → what is missing or not yet covered
- No → what is misaligned or missing

Output (no intro text):
| Alignment | Begründung |
```

Append the result to the last task file under `## Ausführung`:

```
**Intent-Alignment:** Ja / Teilweise / Nein — <Begründung>
```

If alignment is **not Yes**: print a clear warning to the user before the final summary:

```
⚠ Intent nicht vollständig erreicht: <Begründung>
```

If no Intent is present: skip this step silently.

## Step 4 — Final summary

After all tasks are processed, print a brief summary:

```
Ausgeführt:   <n> Tasks
Erfolgreich:  <list of filenames>
Abgebrochen:  <filename if any> — <reason>
Übersprungen: <filenames if any>
Intent:       Ja / Teilweise / Nein / — (kein Intent)
```
