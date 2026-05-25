---
description: Add a todo item to TODO.md in the project root, or list all todos. Pass text directly to add; call without arguments to list.
argument-hint: [todo text]
allowed-tools: [Read, Write, Edit, Bash]
---

# k-todo

Manage the `TODO.md` file in the current project root.

## Step 1 — Determine path

`TODO_PATH` = `$CWD/TODO.md`

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
