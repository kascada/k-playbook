# Shared Overlay Resolution

Use this module when a command needs the effective set of rules, review recipes, or
checks for a project.

k-playbook ships catalogs under `_dist/` and lets each project deviate without ever
editing shipped files. `_dist/` is replaced wholesale on update, so a project that
edited it would lose its changes. Overlay is the supported way to deviate.

This module requires `PLAYBOOK_DIR`, `DIST_DIR`, and the relevant resolved path from
`<DIST_DIR>/commands/_shared/path-resolution.md`. Apply that module first.

## The Three Kinds

| Kind | Shipped base | Project-local | File pattern |
|---|---|---|---|
| `rules` | `<PLAYBOOK_DIR>/rules/` | `<LOCAL_DIR>/rules/` | `*.md` |
| `reviews` | `<PLAYBOOK_DIR>/reviews/` | `<LOCAL_DIR>/reviews/` | `review-*.md` |
| `checks` | `<PLAYBOOK_DIR>/checks/` | `<LOCAL_DIR>/checks/` | `*.sh` (top level only) |

## Comparison Unit

Entries are matched by **filename**. Both sides use the same naming convention, so no
derived key is needed. `docs-sync.md` in the local directory replaces `docs-sync.md`
in the shipped one. Filenames are compared case-sensitively.

Alongside the filename, every entry has a **key** for addressing it on the command
line: the filename without its extension and without the kind's prefix, so
`review-codeql-security.md` is called as `codeql-security`. The key is a label, not
the comparison unit.

## Non-Entries

The following are never catalog entries and must be skipped in both directories:

- `README.md` in any catalog directory — it documents the catalog.
- Anything inside a `lib/` subdirectory — shared helper code for checks, not a check.
- Files not matching the kind's file pattern from the table above.
- Dotfiles.

## Resolution

Given a kind:

```
base   = entries in the shipped directory
local  = entries in the project-local directory (empty if missing)

effective = local ∪ { b ∈ base | key(b) ∉ keys(local) }
```

In words:

1. Every project-local entry is active.
2. A shipped entry is active unless a project-local entry has the same key.

Consequences that must hold:

- A project-local entry **replaces** a shipped entry with the same key. It does not
  merge with it, and it is not partially applied. The shipped file is not read at all.
- To drop a project-local entry, delete the file.

## Switching a Shipped Entry Off

There is no disable list. A shipped entry is switched off by placing an **empty**
project-local file with the same key next to it. Since a local entry replaces the
shipped one completely, an empty file leaves nothing behind.

Empty means: no content other than blank lines and comment lines. That way the file
can carry the reason:

```bash
# Switched off: this project does not use Django.
```

| Kind | What the empty file does |
|---|---|
| `rules`, `reviews` | The entry stays in the catalog, but its content is what you read: a file saying it is switched off, and why. Mark it `disabled` when reporting. |
| `checks` | The entry drops out of the catalog entirely and is never executed. |

The difference is deliberate. `rules` and `reviews` are read by an assistant, so the
text speaks for itself. A check is executed: an empty script would exit 0 and look
like a check that passed, which is worse than no check at all.

## Origin Tracking

Record an origin for every effective entry, because the user must be able to tell
shipped behaviour from project behaviour:

| Origin | Meaning |
|---|---|
| `dist` | shipped, active as-is |
| `local` | project-local, no shipped entry with this key |
| `override` | project-local, replaces a shipped entry with the same key |

Also record, for reporting only:

| State | Meaning |
|---|---|
| `disabled` | switched off by an empty project-local file |

## Reporting

Every command that applies this module reports the resolution in its preflight,
before doing work. Keep it to one block:

```text
Regeln (rules): 4 aktiv
  [dist]     codeql
  [dist]     review-authoring
  [override] docs-sync            (ueberlagert _dist/rules/docs-sync.md)
  [local]    my-api-rules
  [disabled] tool-install-scope   (leere lokale Datei)
```

If the effective set is empty, say so and stop; do not fall back to the shipped
catalog. An empty set is a deliberate project decision.

## Writing Rules

- Never write to `DIST_DIR`. To change a shipped entry, create a project-local file
  with the same key.
- When a command offers to switch a shipped entry off, it creates the empty
  project-local file only after explicit confirmation, and puts the reason in a
  comment line.
- When a command offers to override a shipped entry, it copies the shipped file into
  the project-local directory as a starting point and tells the user that the copy is
  now frozen — later updates to the shipped file will not reach it.

## Required Output From This Step

- `EFFECTIVE_<KIND>` as a list of absolute paths
- an origin per entry: `dist`, `local`, or `override`
- `DISABLED_<KIND>` as the list of suppressed shipped keys
- `STALE_DISABLE_<KIND>` as the list of disable entries matching nothing
