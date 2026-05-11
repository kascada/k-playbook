---
description: Create a new task file from the current conversation. Determines the next number from tasks/, names the file <number>-<short-name>.md, includes relevant reference documents, and asks for confirmation before saving.
argument-hint: [short-name]
allowed-tools: [Read, Write, Glob]
---

# k-task-create

Create a new task file based on what was discussed in the current conversation.

## Step 1 — Determine target directory

Look for a `tasks/` subdirectory relative to the current working directory. If it does not exist, ask the user where to save the task before continuing.

## Step 2 — Determine next number

List all `.md` files in `tasks/` (not subdirectories). Find the highest leading number (e.g. `013-foo.md` → 13). The new task gets the next number, zero-padded to 3 digits (e.g. `014`).

If `tasks/` is empty or has no numbered files, start at `001`.

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
- `mcp__akte-db__mysql_query` — Datenbankabfragen   (nur wenn relevant)

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

Write the confirmed task to `tasks/<filename>` using the Write tool.  
Confirm: "Task gespeichert: tasks/<filename>"
