---
description: "Add a todo item to the fixed project todo file, or list all todos. Uses k-playbook/TODO.md. Pass text directly to add; call without arguments to list."
argument-hint: [todo text]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob]
---

# k-todo

Manage the project todo file.

`/k-todo` does not guess project paths. The project must have `K-PLAYBOOK.MD`. The todo file is always `<project>/k-playbook/TODO.md`.

## Step 1 — Resolve todo file

Determine `TARGET_DIR`:

- If the current working directory contains `K-PLAYBOOK.MD`, use it as `TARGET_DIR`.
- Else walk upward from the current working directory until a parent containing `K-PLAYBOOK.MD` is found; use that parent as `TARGET_DIR`.
- Else abort and tell the user to run `/k-setup` first.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve the fixed `todo` path:

- `TODO_PATH = <TARGET_DIR>/k-playbook/TODO.md`.
- `TODO_DISPLAY_PATH = k-playbook/TODO.md`.

Command-specific policy:

- If `K-PLAYBOOK.MD` is missing: abort and tell the user to run `/k-setup` first.
- If the parent directory of `TODO_PATH` does not exist: abort and tell the user to run `/k-setup` to create/migrate the configured file parent.
- If the parent directory exists: use `TODO_PATH`.

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
TODO.md: <TODO_DISPLAY_PATH>
Hinzugefügt: <ARGUMENTS>
```
