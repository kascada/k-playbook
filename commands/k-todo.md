---
description: "Add a todo item to the project todo file from paths.todo, or list all todos. Pass text directly to add; call without arguments to list."
argument-hint: [todo text]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob]
---

# k-todo

## Erster Schritt

Fuehre zuerst `commands/_shared/context.md` aus: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe.


Manage the project todo file.

`/k-todo` does not guess project paths. The project must have `K-PLAYBOOK.yaml`; the todo file comes from `paths.todo`. If that key is missing, ask for it, write it to `K-PLAYBOOK.yaml`, and then continue.

## Step 1 — Resolve todo file

Read and apply `<DIST_DIR>/commands/_shared/path-resolution.md`. Its discovery step
finds `PLAYBOOK_DIR` by walking upward from the current working directory; do not
implement a separate search here.

For this command, resolve the configured `todo` path:

- `TODO_PATH = <PLAYBOOK_DIR>/<paths.todo>`.
- `TODO_DISPLAY_PATH = <paths.todo>`.

Command-specific policy:

- If discovery finds no `K-PLAYBOOK.yaml`: abort; the directory is not a k-playbook project. Recommend `k-playbook-installer init`.
- If `paths.todo` is missing: ask for the todo file relative to `PLAYBOOK_DIR`, recommend `TODO.md`, validate the answer, add it to `K-PLAYBOOK.yaml`, then continue.
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
