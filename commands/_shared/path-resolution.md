# Shared Path Resolution

Use this module when a command needs project-local paths from `K-PLAYBOOK.MD`.

The calling command remains responsible for command-specific policy: which keys to resolve, whether they point to files or directories, whether missing paths are allowed, whether paths may be created, and what defaults apply.

## Loading This Module From Commands

When a command says to read and apply this module, locate it as `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Determine `PLAYBOOK_REPO`:

- The canonical logical path is always `~/dev/k-playbook`; expand `~` against the current user.
- If `<TARGET_DIR>/K-PLAYBOOK.MD` exists and contains `## Playbook-Quelle` with `repo:`, read it only as validation/documentation. Expected value is `~/dev/k-playbook`. If it differs, commands should warn and continue with `~/dev/k-playbook` when possible; `/k-setup` owns migrating the value back.
- If `~/dev/k-playbook` is missing but `/workspaces/k-playbook` exists, this is likely a Devcontainer with a missing symlink. Commands may report a setup error; setup/install flows may create `~/dev/k-playbook -> /workspaces/k-playbook`.
- If `~/dev/k-playbook` is missing, do not ask for a different permanent path. Ask the user to clone/move the repo there or create a symlink there.

## Determine `TARGET_DIR`

- If the command has already defined `TARGET_DIR`, use it.
- Else if the command received an explicit project/root directory argument, resolve it with `realpath` and validate that it exists.
- Else use the current working directory as `TARGET_DIR`.
- Before finalizing `TARGET_DIR`, guard against accidentally targeting the project-local playbook base directory:
  - If `<TARGET_DIR>/K-PLAYBOOK.MD` is missing, but `<TARGET_DIR>/../K-PLAYBOOK.MD` exists, read the parent file's `base:` value.
  - If the parent file has no `base:`, do not infer it. Stop or warn according to the calling command's policy and ask the user to run `/k-setup` for the parent project first.
  - If the resolved parent `base:` path equals the current `TARGET_DIR`, treat the parent directory as the project root and set `TARGET_DIR = realpath(<TARGET_DIR>/..)`. Example: when CWD is `<project>/k-playbook` and parent `K-PLAYBOOK.MD` says `base: ./k-playbook`, the target is `<project>`, not `<project>/k-playbook`.
  - Announce this correction in the command preflight so the user can see which project root is used.
- Resolve all repo-relative playbook paths against `TARGET_DIR`.

## Read `K-PLAYBOOK.MD`

- Read `<TARGET_DIR>/K-PLAYBOOK.MD` if it exists.
- If it is missing, record `K_PLAYBOOK_FOUND=false`; do not abort here.
- Extract only the `## Pfade` block.
- Parse list entries shaped like `- key: value`.
- Treat a missing key, empty value, or `-` as unset.
- Parse `base:` as project-local playbook base metadata, not as a requested building block.
- If `base:` is set and relative, resolve it against `TARGET_DIR` and expose it as `PLAYBOOK_BASE_DIR`, with `PLAYBOOK_BASE_DISPLAY_PATH` as the shortest useful display path.
- If `base:` is missing, leave `PLAYBOOK_BASE_DIR` unset; do not infer it here. Commands that need the playbook base must stop and ask the user to run `/k-setup`. `/k-setup` is the only command that may ask for and write a missing `base:` migration.

## Resolve Requested Keys

For every key requested by the calling command, e.g. `tasks`, `docs`, or `reviews`:

- Store the raw parsed value as `<KEY>_DIR`.
- If unset, record `<KEY>_DIR` as unset and skip path resolution.
- If the value is absolute, set `RESOLVED_<KEY>_DIR` to that value.
- If the value is relative, set `RESOLVED_<KEY>_DIR` to `<TARGET_DIR>/<value>`.
- Store `<KEY>_DISPLAY_PATH` as the shortest useful display path: project-relative when the resolved path is inside `TARGET_DIR`, otherwise absolute.

Use uppercase variable names in the calling command, e.g. `TASKS_DIR`, `RESOLVED_TASKS_DIR`, `TASKS_DISPLAY_PATH`. If a command needs a historical or clearer name such as `PROJECT_REVIEWS_DIR`, state the mapping explicitly and treat that variable as the resolved absolute path.

## Existence Checks

After resolving a requested key:

- If the resolved path exists, record it as usable.
- If it does not exist, record it as missing; do not create it automatically.
- For file-valued keys, the calling command may instead check whether the parent directory exists and create the file lazily.
- The calling command decides whether to ask, create, use a default, warn, or abort.

## Missing Values

This shared module does not choose defaults. The calling command must state its policy explicitly.

General policy for commands that use project-local playbook paths:

- Missing `K-PLAYBOOK.MD` -> abort and ask the user to run `/k-setup`, unless the command operates only on an explicit file/directory argument and does not need registered project paths.
- Missing `base:` -> abort and ask the user to run `/k-setup`. Do not infer it from existing paths.
- Missing or inactive building block -> abort if the command requires that block; otherwise treat it as intentionally unavailable and do not invent a default path.
- Missing configured path on disk -> abort or ask the user to run `/k-setup`; commands should not silently create configured project structure unless their command-specific policy explicitly owns that initialization.

## Required Output From This Step

At the end of path resolution, the command should have:

- `TARGET_DIR`
- `K_PLAYBOOK_FOUND`
- `PLAYBOOK_BASE_DIR` and `PLAYBOOK_BASE_DISPLAY_PATH` if `base:` is set
- one raw variable per requested key, e.g. `TASKS_DIR`
- one resolved variable per set key, e.g. `RESOLVED_TASKS_DIR`
- one display variable per set key, e.g. `TASKS_DISPLAY_PATH`
- a clear command-specific decision for every missing or non-existing path
