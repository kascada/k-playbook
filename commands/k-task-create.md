---
description: "Create a new task file from the current conversation. Writes to the project's task directory, determines the next number, names the file <number>-<short-name>.md, includes relevant reference documents, and asks for confirmation before saving."
argument-hint: [short-name]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Bash, Glob]
---

# k-task-create

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Create a new task file based on what was discussed in the current conversation.

## Step 1 — Resolve task directory

From the context output:

- `RESOLVED_TASKS_DIR = <local.dir>/tasks`.
- `TASKS_DISPLAY_PATH = k-playbook-local/tasks`.

Command-specific policy:

- If `RESOLVED_TASKS_DIR` does not exist: ask whether to create exactly that directory now or run `/k-gui`; do not use any fallback path.

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
