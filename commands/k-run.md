---
description: Execute one or more task files. Pass a single .md file or a directory. Multiple tasks are executed in order by their numeric prefix. On success, appends an execution summary and moves the file to done/. On partial execution or error, appends a status note and leaves the file in place.
argument-hint: <file-or-directory>
---

# k-run

Execute task files described in `$ARGUMENTS`.

## Step 1 — Collect tasks

If `$ARGUMENTS` is a single `.md` file: use that file as a one-item list.

If `$ARGUMENTS` is a directory:
- Find all `.md` files directly in that directory (not subdirectories)
- Sort by the leading number in the filename (e.g. `013_foo.md` before `014_bar.md`)
- Skip files without a leading number

Announce the list of tasks to be executed before starting.

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

2. Ensure `done/` subdirectory exists in the same directory as the task file
3. Move the task file into `done/`

### 2f — Continue

Proceed to the next task in the list. If a task failed (Step 2d), stop — do not execute remaining tasks.

## Step 3 — Final summary

After all tasks are processed, print a brief summary:

```
Ausgeführt:   <n> Tasks
Erfolgreich:  <list of filenames>
Abgebrochen:  <filename if any> — <reason>
Übersprungen: <filenames if any>
```
