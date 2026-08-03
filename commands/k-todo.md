---
description: "Add a todo item to the project todo file from paths.todo, or list all todos. Pass text directly to add; call without arguments to list."
argument-hint: [todo text]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob]
---

# k-todo

Manage the project todo file.

`/k-todo` does not guess project paths. The project must have `K-PLAYBOOK.yaml`; the todo file comes from `paths.todo`. If that key is missing, ask for it, write it to `K-PLAYBOOK.yaml`, and then continue.

## Step 1 — Resolve todo file

Determine `TARGET_DIR`:

- If the current working directory contains `K-PLAYBOOK.yaml`, use it as `TARGET_DIR`.
- Else walk upward from the current working directory until a parent containing `K-PLAYBOOK.yaml` is found; use that parent as `TARGET_DIR`.
- Else abort and tell the user to run `/k-gui`.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve the configured `todo` path:

- `TODO_PATH = <TARGET_DIR>/<paths.todo>`.
- `TODO_DISPLAY_PATH = <paths.todo>`.

Command-specific policy:

- If `K-PLAYBOOK.yaml` is missing: abort and tell the user to run `/k-gui`.
- If `paths.todo` is missing: ask for the project-relative todo file, recommend `k-playbook/TODO.md`, validate the answer, add it to `K-PLAYBOOK.yaml`, then continue.
- If the parent directory of `TODO_PATH` does not exist: abort and tell the user to run `/k-gui`.
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
