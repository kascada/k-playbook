# Shared Path Resolution

Use this module when a command needs project-local paths from `K-PLAYBOOK.MD`.

The calling command remains responsible for command-specific policy: which keys to resolve, whether they point to files or directories, whether missing paths are allowed, whether paths may be created, and what defaults apply.

## Loading This Module From Commands

When a command says to read and apply this module, locate it as `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Determine `PLAYBOOK_REPO` best-effort:

- If `<TARGET_DIR>/K-PLAYBOOK.MD` exists and contains `## Playbook-Quelle` with `repo:`, use that path.
- Else if the slash-command file path can be resolved, use the parent repo that contains `commands/`.
- Else try `~/dev/k-playbook`.
- If still unclear, ask the user where the k-playbook repo is.

## Determine `TARGET_DIR`

- If the command has already defined `TARGET_DIR`, use it.
- Else if the command received an explicit project/root directory argument, resolve it with `realpath` and validate that it exists.
- Else use the current working directory as `TARGET_DIR`.
- Resolve all repo-relative playbook paths against `TARGET_DIR`.

## Read `K-PLAYBOOK.MD`

- Read `<TARGET_DIR>/K-PLAYBOOK.MD` if it exists.
- If it is missing, record `K_PLAYBOOK_FOUND=false`; do not abort here.
- Extract only the `## Pfade` block.
- Parse list entries shaped like `- key: value`.
- Treat a missing key, empty value, or `-` as unset.

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

This shared module does not choose defaults. The calling command must state its policy explicitly, for example:

- `tasks:` missing -> ask the user and suggest `./tasks`.
- `docs:` missing -> default to `./docs` and remind that `/k-setup` can register it.
- `reviews:` missing -> continue with global reviews only.
- `todo:` missing -> default to `./TODO.md` or ask the user, depending on the command.

## Required Output From This Step

At the end of path resolution, the command should have:

- `TARGET_DIR`
- `K_PLAYBOOK_FOUND`
- one raw variable per requested key, e.g. `TASKS_DIR`
- one resolved variable per set key, e.g. `RESOLVED_TASKS_DIR`
- one display variable per set key, e.g. `TASKS_DISPLAY_PATH`
- a clear command-specific decision for every missing or non-existing path
