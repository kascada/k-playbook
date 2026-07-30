# Shared Path Resolution

Use this module when a command needs project-local k-playbook paths.

The project-local layout is fixed and complete. `/k-setup` creates every standard project-local k-playbook directory/file under `<project>/k-playbook/`. Commands derive paths from the project root; they do not read configurable path values and do not check active/inactive building blocks.

## Fixed Layout

For a project root `TARGET_DIR`, derive:

| Artifact | Derived path |
|---|---|
| playbook base | `<TARGET_DIR>/k-playbook/` |
| tasks | `<TARGET_DIR>/k-playbook/tasks/` |
| completed tasks | `<TARGET_DIR>/k-playbook/tasks/done/` |
| todo | `<TARGET_DIR>/k-playbook/TODO.md` |
| checks | `<TARGET_DIR>/k-playbook/checks/` |
| reviews | `<TARGET_DIR>/k-playbook/reviews/` |
| guidelines | `<TARGET_DIR>/k-playbook/guidelines/` |
| enforcement | `<TARGET_DIR>/k-playbook/enforcement/` |
| docs | `<TARGET_DIR>/k-playbook/docs/` |

No command should ask for alternative locations for these paths. If a project still has old configurable paths or a `## Bausteine` active/inactive block in `K-PLAYBOOK.MD`, ask the user to run `/k-setup` to migrate the managed block and create the complete fixed structure.

## Loading This Module From Commands

When a command says to read and apply this module, locate it as `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Determine `PLAYBOOK_REPO`:

- The canonical logical path is always `~/dev/k-playbook`; expand `~` against the current user.
- If `<TARGET_DIR>/K-PLAYBOOK.MD` contains `repo:`, read it only as validation/documentation. Expected value is `~/dev/k-playbook`. If it differs, commands should warn and continue with `~/dev/k-playbook` when possible; `/k-setup` owns migrating the value back.
- If `~/dev/k-playbook` is missing but `/workspaces/k-playbook` exists, this is likely a Devcontainer with a missing symlink. Commands may report a setup error; setup/install flows may create `~/dev/k-playbook -> /workspaces/k-playbook`.
- If `~/dev/k-playbook` is missing, do not ask for a different permanent path. Ask the user to clone/move the repo there or create a symlink there.

## Determine `TARGET_DIR`

- If the command has already defined `TARGET_DIR`, use it.
- Else if the command received an explicit project/root directory argument, resolve it with `realpath` and validate that it exists.
- Else use the current working directory as `TARGET_DIR`.
- Before finalizing `TARGET_DIR`, guard against accidentally targeting the fixed project-local playbook base directory:
  - If `<TARGET_DIR>/K-PLAYBOOK.MD` is missing, but `<TARGET_DIR>/../K-PLAYBOOK.MD` exists and `<TARGET_DIR>` is named `k-playbook`, treat the parent directory as the project root and set `TARGET_DIR = realpath(<TARGET_DIR>/..)`. Announce this correction in the command preflight.
- Resolve all derived playbook paths against `TARGET_DIR`.

## Read `K-PLAYBOOK.MD`

- Read `<TARGET_DIR>/K-PLAYBOOK.MD` if it exists.
- If it is missing, record `K_PLAYBOOK_FOUND=false`; do not abort here.
- Parse setup metadata only: managed markers, `layout:`, `repo:`, and `setup-run:`.
- Expected layout is `fixed-project-k-playbook`.
- Legacy `## Pfade` and `## Bausteine` blocks are migration signals only. Do not use them to choose paths or availability.
- `base:` in old files is legacy metadata. Do not use it to derive paths. The effective playbook base is always `<TARGET_DIR>/k-playbook`.

## Resolve Requested Keys

For every key requested by the calling command, e.g. `tasks`, `docs`, or `reviews`:

- Store the derived absolute path as `RESOLVED_<KEY>_DIR` for directory-valued keys or `RESOLVED_<KEY>_PATH` for file-valued keys.
- For compatibility with existing command text, a command may also use historical names such as `TASKS_DIR`, `TODO_PATH`, `PROJECT_REVIEWS_DIR`, or `DOCS_DIR`; they must point to the same derived path.
- Store `<KEY>_DISPLAY_PATH` as the project-relative fixed path, e.g. `k-playbook/tasks`, `k-playbook/TODO.md`, `k-playbook/docs`.

Directory-valued keys: `tasks`, `checks`, `reviews`, `guidelines`, `enforcement`, `docs`.
File-valued keys: `todo`.

## Existence Checks

After resolving a requested key:

- If the derived path exists, record it as usable.
- If it does not exist, record it as missing; commands should ask the user to run `/k-setup` rather than creating the standard structure themselves.
- For file-valued keys, the calling command may instead check whether the parent directory exists and create the file lazily when that is part of the command's job, e.g. `/k-todo` may create `k-playbook/TODO.md`.

## Required Output From This Step

At the end of path resolution, the command should have:

- `TARGET_DIR`
- `K_PLAYBOOK_FOUND`
- `PLAYBOOK_BASE_DIR = <TARGET_DIR>/k-playbook`
- `PLAYBOOK_BASE_DISPLAY_PATH = k-playbook`
- `K_PLAYBOOK_SCHEMA = fixed|legacy-paths|legacy-blocks|missing`
- one derived resolved path per requested key, e.g. `RESOLVED_TASKS_DIR` or `RESOLVED_TODO_PATH`
- one display variable per requested key, e.g. `TASKS_DISPLAY_PATH`
- a clear command-specific decision for every missing path
