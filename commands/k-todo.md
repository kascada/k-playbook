---
description: "Add a todo item to the project todo file, or list all todos. Pass text directly to add; call without arguments to list."
argument-hint: [todo text]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob]
---

# k-todo

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Manage the project todo file.

## Step 1 — Resolve todo file

From the context output:

- `TODO_PATH = <local.dir>/TODO.md`.
- `TODO_DISPLAY_PATH = k-playbook-local/TODO.md`.

Command-specific policy:

- If `local.dir` does not exist: abort and tell the user to run `/k-gui`.
- If `local.dir` exists but `TODO.md` does not: that is normal, Step 2 creates it.

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
