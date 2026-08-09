# Shared Path Resolution

Use this module when a command needs project-local k-playbook paths.

k-playbook is installed into a subdirectory of the target project, conventionally
`<project>/k-playbook/`. There is no central base installation and no fixed host
path. Everything a command needs is inside that directory.

Project-local paths are configuration, not assumptions. Commands must read the
effective paths from `K-PLAYBOOK.yaml`. If a required path key is missing, the
command must ask the user for the value, validate it, write it back to
`K-PLAYBOOK.yaml`, and only then continue. Do not silently fall back to historical
defaults.

## Installation vs. Project Ownership

Inside the k-playbook directory the split is absolute:

- `_dist/` is the **installation**. It is shipped, read-only, and replaced wholesale
  on every update. Never write to it.
- Everything else is **project-owned**: `K-PLAYBOOK.yaml`, tasks, reviews, results,
  docs, own rules, own checks, own commands. An update never touches these.

A command that needs to persist anything writes outside `_dist/`, always.

## Discover `PLAYBOOK_DIR`

This runs before anything else and needs no configuration — it is what breaks the
chicken-and-egg problem of locating configuration.

1. If the command already defined `PLAYBOOK_DIR`, use it.
2. If the command received an explicit directory argument, resolve it with `realpath`:
   - If `<arg>/K-PLAYBOOK.yaml` exists, `PLAYBOOK_DIR = <arg>`.
   - Else if `<arg>/k-playbook/K-PLAYBOOK.yaml` exists, `PLAYBOOK_DIR = <arg>/k-playbook`.
   - Else abort and report the argument as not a k-playbook project.
3. Otherwise start at `realpath(CWD)` and walk upwards:
   - If `<dir>/K-PLAYBOOK.yaml` exists, `PLAYBOOK_DIR = <dir>`. Stop.
   - Else if `<dir>/k-playbook/K-PLAYBOOK.yaml` exists, `PLAYBOOK_DIR = <dir>/k-playbook`. Stop.
   - Else move to the parent. Stop ascending at the Git worktree root, at `$HOME`,
     or at `/`, whichever comes first.
4. If nothing was found, stop and tell the user to run `k-playbook-installer init`.
   Do not guess a directory and do not create one.

Never look for the playbook in `$HOME`, `~/dev/k-playbook`, or `/workspaces/k-playbook`.
Those belong to the retired central-installation model.

Announce the discovered `PLAYBOOK_DIR` in the command preflight when it was not
passed explicitly.

## Read `K-PLAYBOOK.yaml`

- Read `<PLAYBOOK_DIR>/K-PLAYBOOK.yaml`.
- Parse `schema_version`, `layout`, `k_playbook`, `paths`, `project`, `overlay`, and
  `setup.updated_at`.
- Expected `schema_version` is `2` and expected `layout` is `project-local`.
- If `schema_version` is `1` or `layout` is `fixed-project-k-playbook`, this is a
  pre-migration project. Stop and tell the user to run
  `k-playbook-installer migrate`. Do not attempt to interpret v1 paths.
- Set `DIST_DIR = <PLAYBOOK_DIR>/<k_playbook.dist>`. If `k_playbook.dist` is missing,
  the config is invalid — stop and recommend the installer.
- If `DIST_DIR` does not exist, the installation is incomplete, typically after a
  fresh `git clone` because `_dist/` is gitignored. Stop and tell the user to run
  `k-playbook-installer restore`.
- `project.repo_root` is required for code/Git operations. It is relative to
  `PLAYBOOK_DIR` and is the only configured path that may leave it; the normal value
  is `..`. It must stay inside the Git worktree.
- `project.vcs` is required and is either `git` or `none`.
- If `project.repo_root` or `project.vcs` is missing, empty, or invalid, do not search
  for Git roots. Stop and tell the user to run `/k-gui`; the installer owns discovery
  and the explicit `vcs: none` decision.
- Store `PROJECT_REPO_ROOT_DIR` as the resolved absolute path and
  `PROJECT_REPO_ROOT_DISPLAY_PATH` as the configured relative value.

## Canonical Path Keys

`K-PLAYBOOK.yaml` stores project-local artifact paths under `paths:`. Values are
relative to `PLAYBOOK_DIR` and must not be absolute.

A value may leave `PLAYBOOK_DIR` with `../` as long as it stays inside
`PROJECT_REPO_ROOT_DIR`. That is how a project keeps an already-established directory
instead of moving it — most often `paths.docs`, when the project maintains its
documentation elsewhere. A value must never resolve into `DIST_DIR`.

| Artifact | YAML key | Conventional value |
|---|---|---|
| tasks | `paths.tasks` | `tasks` |
| completed tasks | `paths.completed_tasks` | `tasks/done` |
| todo | `paths.todo` | `TODO.md` |
| checks | `paths.checks` | `checks` |
| reviews | `paths.reviews` | `reviews` |
| guidelines | `paths.guidelines` | `guidelines` |
| enforcement | `paths.enforcement` | `enforcement` |
| docs | `paths.docs` | `docs` |
| own commands | `paths.commands` | `commands` |

The conventional values are suggestions for onboarding and repair prompts, not
implicit runtime defaults. A command may display the conventional value as the
recommended answer, but it must not use it until the value is present in
`K-PLAYBOOK.yaml`.

## Resolve Requested Keys

For every key requested by the calling command, e.g. `tasks`, `docs`, or `reviews`:

- Read `paths.<key>` from `K-PLAYBOOK.yaml`.
- If `paths.<key>` is missing or empty, ask the user for the path relative to
  `PLAYBOOK_DIR`. Show the conventional value from the table as the recommended
  answer. After confirmation, write the value to `K-PLAYBOOK.yaml` before continuing.
- Validate that the configured value is relative and normalized, and that it resolves
  inside `PROJECT_REPO_ROOT_DIR`. It may sit outside `PLAYBOOK_DIR` via `../`.
- Reject any value that resolves outside `PROJECT_REPO_ROOT_DIR`. Commands write only
  within the project; report this as a configuration error.
- Reject any value that resolves into `DIST_DIR`. Installation and project ownership
  must not overlap, because an update replaces `DIST_DIR` wholesale and would delete
  the project's files with it.
- Store the resolved absolute path as `RESOLVED_<KEY>_DIR` for directory-valued keys
  or `RESOLVED_<KEY>_PATH` for file-valued keys.
- For compatibility with existing command text, a command may also use historical
  names such as `TASKS_DIR`, `TODO_PATH`, `PROJECT_REVIEWS_DIR`, or `DOCS_DIR`; they
  must point to the same YAML-configured path.
- Store `<KEY>_DISPLAY_PATH` as the configured relative path, e.g. the value of
  `paths.tasks`, `paths.todo`, or `paths.docs`.

Directory-valued keys: `tasks`, `completed_tasks`, `checks`, `reviews`, `guidelines`,
`enforcement`, `docs`, `commands`.
File-valued keys: `todo`.

## Shipped Catalogs

Commands that need shipped rules, review recipes, or checks resolve them under
`DIST_DIR`:

| Catalog | Path |
|---|---|
| shared modules | `<DIST_DIR>/commands/_shared/` |
| rules | `<DIST_DIR>/rules/` |
| review recipes | `<DIST_DIR>/reviews/` |
| checks | `<DIST_DIR>/checks/` |
| scripts | `<DIST_DIR>/scripts/` |
| security tool matrix | `<DIST_DIR>/security-tools.tsv` |
| check runner | `<DIST_DIR>/bin/k-check` |

These always exist once `DIST_DIR` was verified above; a command must not treat a
missing catalog as a normal case. If one is missing, the installation is damaged —
report it and recommend `k-playbook-installer restore`.

For `rules`, `reviews`, and `checks`, do not read the shipped directory directly.
Combine it with the project-local directory as described in
`<DIST_DIR>/commands/_shared/overlay-resolution.md`.

## Existence Checks

After resolving a requested key:

- If the YAML-configured path exists, record it as usable.
- If the YAML-configured path does not exist, ask whether to create it now or run
  `/k-gui` to repair the project structure. Do not substitute another path.
- For file-valued keys, the calling command may instead check whether the parent
  directory exists and create the file lazily when that is part of the command's job,
  e.g. `/k-todo` may create the configured `TODO.md`.

## Required Output From This Step

At the end of path resolution, the command should have:

- `PLAYBOOK_DIR`
- `DIST_DIR`
- `PROJECT_REPO_ROOT_DIR` and `PROJECT_REPO_ROOT_DISPLAY_PATH`
- `K_PLAYBOOK_SCHEMA = yaml|invalid|missing|needs-migration`
- one YAML-configured resolved path per requested key, e.g. `RESOLVED_TASKS_DIR` or
  `RESOLVED_TODO_PATH`
- one display variable per requested key, e.g. `TASKS_DISPLAY_PATH`
- a clear command-specific decision for every missing path
