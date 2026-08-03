# Shared Path Resolution

Use this module when a command needs project-local k-playbook paths.

Project-local paths are configuration, not assumptions. Commands must read the effective paths from `K-PLAYBOOK.yaml`. If a required path key is missing, the command must ask the user for the value, validate it, write it back to `K-PLAYBOOK.yaml`, and only then continue. Do not silently fall back to historical defaults.

## Canonical Path Keys

`K-PLAYBOOK.yaml` stores project-local artifact paths under `paths:`. Values are relative to the directory containing `K-PLAYBOOK.yaml`, must not be absolute, and must not escape that directory.

| Artifact | YAML key | Conventional value |
|---|---|
| playbook base | `paths.playbook` | `k-playbook` |
| tasks | `paths.tasks` | `k-playbook/tasks` |
| completed tasks | `paths.completed_tasks` | `k-playbook/tasks/done` |
| todo | `paths.todo` | `k-playbook/TODO.md` |
| checks | `paths.checks` | `k-playbook/checks` |
| reviews | `paths.reviews` | `k-playbook/reviews` |
| guidelines | `paths.guidelines` | `k-playbook/guidelines` |
| enforcement | `paths.enforcement` | `k-playbook/enforcement` |
| docs | `paths.docs` | `k-playbook/docs` |

The conventional values are suggestions for onboarding and repair prompts, not implicit runtime defaults. A command may display the conventional value as the recommended answer, but it must not use it until the value is present in `K-PLAYBOOK.yaml`.

## Loading This Module From Commands

When a command says to read and apply this module, locate it as `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Determine `PLAYBOOK_REPO`:

- Read it from `<TARGET_DIR>/K-PLAYBOOK.yaml` as `k_playbook.repo` when available; expand `~` against the current user.
- If `k_playbook.repo` is missing, ask the user for the global k-playbook repo path, recommend `~/dev/k-playbook`, validate that the path exists, and write the chosen value to `K-PLAYBOOK.yaml`.
- If `~/dev/k-playbook` is missing but `/workspaces/k-playbook` exists, this is likely a Devcontainer with a missing symlink. Commands may report a setup error; setup/install flows may create `~/dev/k-playbook -> /workspaces/k-playbook`.
- If the configured repo path is missing, do not guess another permanent path. Ask the user whether to correct `k_playbook.repo`, create the expected symlink, or run `/k-gui`.

## Determine `TARGET_DIR`

- If the command has already defined `TARGET_DIR`, use it.
- Else if the command received an explicit project/root directory argument, resolve it with `realpath` and validate that it exists.
- Else use the current working directory as `TARGET_DIR`.
- Before finalizing `TARGET_DIR`, guard against accidentally targeting the fixed project-local playbook base directory:
  - If `<TARGET_DIR>/K-PLAYBOOK.yaml` is missing, but `<TARGET_DIR>/../K-PLAYBOOK.yaml` exists and `<TARGET_DIR>` is named `k-playbook`, treat the parent directory as the project root and set `TARGET_DIR = realpath(<TARGET_DIR>/..)`. Announce this correction in the command preflight.
- Resolve all configured project-local paths against `TARGET_DIR`.

## Read `K-PLAYBOOK.yaml`

- Read `<TARGET_DIR>/K-PLAYBOOK.yaml` if it exists.
- If it is missing, record `K_PLAYBOOK_FOUND=false`; do not abort here.
- Parse setup metadata: `schema_version`, `layout`, `k_playbook.repo`, `project.repo_root`, `project.vcs`, `paths`, and `setup.updated_at`.
- Expected layout is `fixed-project-k-playbook`. Existing configs with this layout are incomplete for path-aware commands until the required `paths.*` keys have been added.
- Read standard artifact paths from `paths:`. Do not derive a requested path from `TARGET_DIR` unless the value has first been added to `K-PLAYBOOK.yaml`.
- `project.repo_root` is required for code/Git operations. It is relative to `TARGET_DIR`, must not be absolute, and must not escape `TARGET_DIR`.
- `project.vcs` is required and is either `git` or `none`.
- If `project.repo_root` or `project.vcs` is missing, empty, or invalid, do not search for Git roots. Stop and tell the user to run `/k-gui`; the GUI owns discovery or the explicit `vcs: none` decision.
- Store `PROJECT_REPO_ROOT_DIR` as the resolved absolute path and `PROJECT_REPO_ROOT_DISPLAY_PATH` as the configured relative value.

## Resolve Requested Keys

For every key requested by the calling command, e.g. `tasks`, `docs`, or `reviews`:

- Read `paths.<key>` from `K-PLAYBOOK.yaml`.
- If `paths.<key>` is missing or empty, ask the user for the project-relative path. Show the conventional value from the table as the recommended answer. After confirmation, write the value to `K-PLAYBOOK.yaml` before continuing.
- Validate that the configured value is relative, normalized, and stays inside `TARGET_DIR`.
- Store the resolved absolute path as `RESOLVED_<KEY>_DIR` for directory-valued keys or `RESOLVED_<KEY>_PATH` for file-valued keys.
- For compatibility with existing command text, a command may also use historical names such as `TASKS_DIR`, `TODO_PATH`, `PROJECT_REVIEWS_DIR`, or `DOCS_DIR`; they must point to the same YAML-configured path.
- Store `<KEY>_DISPLAY_PATH` as the configured project-relative path, e.g. the value of `paths.tasks`, `paths.todo`, or `paths.docs`.

Directory-valued keys: `tasks`, `checks`, `reviews`, `guidelines`, `enforcement`, `docs`.
File-valued keys: `todo`.

## Existence Checks

After resolving a requested key:

- If the YAML-configured path exists, record it as usable.
- If `K-PLAYBOOK.yaml` is missing, stop and tell the user to run `/k-gui`.
- If the YAML-configured path does not exist, ask whether to create it now or run `/k-gui` to repair the project structure. Do not substitute another path.
- For file-valued keys, the calling command may instead check whether the parent directory exists and create the file lazily when that is part of the command's job, e.g. `/k-todo` may create `k-playbook/TODO.md`.

## Required Output From This Step

At the end of path resolution, the command should have:

- `TARGET_DIR`
- `K_PLAYBOOK_FOUND`
- `PLAYBOOK_BASE_DIR` from `paths.playbook` if requested or present
- `PLAYBOOK_BASE_DISPLAY_PATH` from `paths.playbook` if requested or present
- `K_PLAYBOOK_SCHEMA = yaml|invalid|missing`
- one YAML-configured resolved path per requested key, e.g. `RESOLVED_TASKS_DIR` or `RESOLVED_TODO_PATH`
- one display variable per requested key, e.g. `TASKS_DISPLAY_PATH`
- a clear command-specific decision for every missing path
