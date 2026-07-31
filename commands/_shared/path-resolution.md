# Shared Path Resolution

Use this module when a command needs project-local k-playbook paths.

The project-local layout is fixed and complete. The k-playbook Installer creates or completes every standard project-local k-playbook directory/file under `<project>/k-playbook/`. Commands derive k-playbook artifact paths from the project root; they read the actual code/repo root only from `K-PLAYBOOK.yaml` as `project.repo_root`.

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

No command should ask for alternative locations for these paths. `K-PLAYBOOK.yaml` records setup metadata, policies, the actual code/repo root, and tool decisions; standard k-playbook paths stay derived from the fixed layout.

## Loading This Module From Commands

When a command says to read and apply this module, locate it as `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Determine `PLAYBOOK_REPO`:

- The canonical logical path is always `~/dev/k-playbook`; expand `~` against the current user.
- If `<TARGET_DIR>/K-PLAYBOOK.yaml` contains `k_playbook.repo`, read it only as validation/documentation. Expected value is `~/dev/k-playbook`. If it differs, commands should warn and continue with `~/dev/k-playbook` when possible; `/k-gui` owns creating/completing the canonical project config.
- If `~/dev/k-playbook` is missing but `/workspaces/k-playbook` exists, this is likely a Devcontainer with a missing symlink. Commands may report a setup error; setup/install flows may create `~/dev/k-playbook -> /workspaces/k-playbook`.
- If `~/dev/k-playbook` is missing, do not ask for a different permanent path. Ask the user to clone/move the repo there or create a symlink there.

## Determine `TARGET_DIR`

- If the command has already defined `TARGET_DIR`, use it.
- Else if the command received an explicit project/root directory argument, resolve it with `realpath` and validate that it exists.
- Else use the current working directory as `TARGET_DIR`.
- Before finalizing `TARGET_DIR`, guard against accidentally targeting the fixed project-local playbook base directory:
  - If `<TARGET_DIR>/K-PLAYBOOK.yaml` is missing, but `<TARGET_DIR>/../K-PLAYBOOK.yaml` exists and `<TARGET_DIR>` is named `k-playbook`, treat the parent directory as the project root and set `TARGET_DIR = realpath(<TARGET_DIR>/..)`. Announce this correction in the command preflight.
- Resolve all derived playbook paths against `TARGET_DIR`.

## Read `K-PLAYBOOK.yaml`

- Read `<TARGET_DIR>/K-PLAYBOOK.yaml` if it exists.
- If it is missing, record `K_PLAYBOOK_FOUND=false`; do not abort here.
- Parse setup metadata: `schema_version`, `layout`, `k_playbook.repo`, `project.repo_root`, `project.vcs`, and `setup.updated_at`.
- Expected layout is `fixed-project-k-playbook`.
- Do not read a `paths:` block for standard paths. The effective playbook base is always `<TARGET_DIR>/k-playbook`.
- `project.repo_root` is required for code/Git operations. It is relative to `TARGET_DIR`, must not be absolute, and must not escape `TARGET_DIR`.
- `project.vcs` is required and is either `git` or `none`.
- If `project.repo_root` or `project.vcs` is missing, empty, or invalid, do not search for Git roots. Stop and tell the user to run `/k-gui`; the GUI owns discovery or the explicit `vcs: none` decision.
- Store `PROJECT_REPO_ROOT_DIR` as the resolved absolute path and `PROJECT_REPO_ROOT_DISPLAY_PATH` as the configured relative value.

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
- If `K-PLAYBOOK.yaml` is missing or the derived path does not exist, stop and tell the user to run `/k-gui`. The GUI owns project onboarding and completing the fixed structure.
- For file-valued keys, the calling command may instead check whether the parent directory exists and create the file lazily when that is part of the command's job, e.g. `/k-todo` may create `k-playbook/TODO.md`.

## Required Output From This Step

At the end of path resolution, the command should have:

- `TARGET_DIR`
- `K_PLAYBOOK_FOUND`
- `PLAYBOOK_BASE_DIR = <TARGET_DIR>/k-playbook`
- `PLAYBOOK_BASE_DISPLAY_PATH = k-playbook`
- `K_PLAYBOOK_SCHEMA = yaml|invalid|missing`
- one derived resolved path per requested key, e.g. `RESOLVED_TASKS_DIR` or `RESOLVED_TODO_PATH`
- one display variable per requested key, e.g. `TASKS_DISPLAY_PATH`
- a clear command-specific decision for every missing path
