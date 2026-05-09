---
description: Create a new task file from the current conversation. Determines the next number from tasks/, names the file <number>-<short-name>.md, includes relevant reference documents, and asks for confirmation before saving.
argument-hint: [short-name]
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

## Step 4 — Identify reference documents

Look at the current conversation for any file paths, documentation files, or source files that were relevant to the discussed task. Include them as references if they add context for the executing agent.

## Step 5 — Draft the task

Write a task draft from the conversation context. Structure:

```
# Task <number> — <Title>

<One sentence describing what this task achieves.>

## Referenzen

- `path/to/file.md` — <why it is relevant>   (only if applicable)

## Ziel

<What needs to be built or done. Be specific enough that an agent can execute without the conversation context.>

## Kontext

<Relevant constraints, architecture decisions, or background from the conversation that the executing agent needs to know.>

## Zu bauen

<Concrete list of files, modules, configs, or steps to be created or changed.>
```

Omit any section that is empty or not applicable.

## Step 6 — Confirm

Show the draft to the user and ask: "Passt das so, oder soll ich etwas anpassen?"

Wait for confirmation or corrections before saving.

## Step 7 — Save

Write the confirmed task to `tasks/<filename>` using the Write tool.  
Confirm: "Task gespeichert: tasks/<filename>"
