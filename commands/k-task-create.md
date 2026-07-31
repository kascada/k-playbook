---
description: "Create a new task file from the current conversation. Uses the fixed k-playbook/tasks path, determines the next number, names the file <number>-<short-name>.md, includes relevant reference documents, and asks for confirmation before saving."
argument-hint: [short-name]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Bash, Glob]
---

# k-task-create

Create a new task file based on what was discussed in the current conversation.

`/k-task-create` does not guess project paths. The project must have `K-PLAYBOOK.yaml` and the task directory is always `<project>/k-playbook/tasks`.

## Step 1 — Resolve task directory

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve the fixed `tasks` path:

- `RESOLVED_TASKS_DIR = <TARGET_DIR>/k-playbook/tasks`.
- `TASKS_DISPLAY_PATH = k-playbook/tasks`.

Command-specific policy:

- If `K-PLAYBOOK.yaml` is missing: abort and tell the user to run `/k-gui`.
- If `k-playbook/tasks` does not exist: abort and tell the user to run `/k-gui`.
- If `k-playbook/tasks` exists: use it.

Remember the chosen absolute directory as `RESOLVED_TASKS_DIR` and the display path as `TASKS_DISPLAY_PATH`.

## Step 2 — Determine next number

Scan `.md` files in **both** `RESOLVED_TASKS_DIR/` and `RESOLVED_TASKS_DIR/done/` (completed tasks land there via `/k-run`). Find the highest leading number across both directories (e.g. `013-foo.md` → 13). The new task gets the next number, zero-padded to 3 digits (e.g. `014`).

If neither directory has numbered files, start at `001`.

## Step 3 — Determine filename

If `$ARGUMENTS` is provided: use it as the short name (lowercase, words separated by hyphens).  
If not: derive a short name (2-4 words, lowercase, hyphens) from the conversation topic.

Filename: `<number>-<short-name>.md`  
Example: `014-audiosocket-server.md`

## Step 4 — Identify reference documents and tools

Look at the current conversation for:
- File paths or documentation files relevant to the task → add to `## Referenzen`
- Tools beyond the standard set (Read, Write, Edit, Bash, Glob, Grep) that will likely be needed → add to `## Tools`

Standard tools (Read, Write, Edit, Bash, Glob, Grep) do not need to be listed — they are always pre-approved in `/k-run`. Only list extras like MCP tools or special permissions.

## Step 5 — Determine Intent

Derive the Intent from the conversation: what frame or outcome do the task(s) as a whole need to deliver?

**Format:** 1–2 sentences setting the frame, followed by 2–5 bullet points listing the decisive constraints or success criteria. No implementation details — only what must hold true at the end.

**When to include:**
- Single task file → always include `## Intent`
- Multiple tasks in a series → include `## Intent` **only in the last file**; if it is unclear whether this is the last file, ask the user before drafting

If the Intent cannot be meaningfully derived from the conversation (e.g. purely mechanical task with no outcome criteria), omit the section.

## Step 6 — Draft the task

Write a task draft from the conversation context. Structure:

```
# Task <number> — <Title>

<One sentence describing what this task achieves.>

## Intent

<1–2 sentences setting the frame — what must be true when all tasks are done.>
- <decisive constraint or success criterion>
- <decisive constraint or success criterion>
- ...

(Place Intent first, before Referenzen and Tools, so it is visible immediately.)

## Referenzen

- `path/to/file.md` — <why it is relevant>   (only if applicable)

## Tools

- `Bash` — Verzeichnisse anlegen, Dateien verschieben
- `mcp__akte-sql__mysql_query` — Datenbankabfragen   (nur wenn relevant)

(Nur aufführen wenn über Standard Read/Write/Edit/Bash/Glob/Grep hinaus etwas benötigt wird.)

## Ziel

<What needs to be built or done. Be specific enough that an agent can execute without the conversation context.>

## Kontext

<Relevant constraints, architecture decisions, or background from the conversation that the executing agent needs to know.>

## Zu bauen

<Concrete list of files, modules, configs, or steps to be created or changed.>
```

Omit any section that is empty or not applicable.

## Step 7 — Confirm

Show the draft to the user and ask: "Passt das so, oder soll ich etwas anpassen?"

Wait for confirmation or corrections before saving.

## Step 8 — Save

Write the confirmed task to `<RESOLVED_TASKS_DIR>/<filename>` using the Write tool.  
Confirm: "Task gespeichert: <TASKS_DISPLAY_PATH>/<filename>"
