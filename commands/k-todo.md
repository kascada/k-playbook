---
description: "Add a todo item to the project todo file, or list all todos. Uses the todo: path from K-PLAYBOOK.MD when available. Pass text directly to add; call without arguments to list."
argument-hint: [todo text]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob]
---

# k-todo

Manage the project todo file.

## Step 1 — Resolve todo file

Determine `TARGET_DIR`:

- If the current working directory contains `K-PLAYBOOK.MD`, use it as `TARGET_DIR`.
- Else walk upward from the current working directory until a parent containing `K-PLAYBOOK.MD` is found; use that parent as `TARGET_DIR`.
- Else use the current working directory as `TARGET_DIR` and record `K_PLAYBOOK_FOUND=false`.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `todo:` → `TODO_PATH`. Treat `TODO_PATH` as the resolved absolute file path.

Command-specific policy:

- If `todo:` is set: use it.
- If `todo:` is unset or `K-PLAYBOOK.MD` is missing: tell the user that `/k-setup` can register the todo path, then use `<TARGET_DIR>/TODO.md` for this run.
- If the parent directory of `TODO_PATH` does not exist: ask before creating it.

## Step 2 — Branch on arguments

### No arguments → List

- If `TODO_PATH` does not exist: output "Keine Todos vorhanden."
- If it exists: read and display the full contents.

### With arguments → Add

**2a. Create file if missing**

If `TODO_PATH` does not exist, create it with this header:

```
# TODO

```

**2b. Append the new item**

Append a new line at the end of the file:

```
- [ ] <ARGUMENTS>
```

**2c. Confirm**

Output:
```
TODO.md: <TODO_PATH>
Hinzugefügt: <ARGUMENTS>
```
