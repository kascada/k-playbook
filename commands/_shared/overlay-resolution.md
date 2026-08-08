# Shared Overlay Resolution

Use this module when a command needs the effective set of rules, review recipes, or
checks for a project.

k-playbook ships catalogs under `_dist/` and lets each project deviate without ever
editing shipped files. `_dist/` is replaced wholesale on update, so a project that
edited it would lose its changes. Overlay is the supported way to deviate.

This module requires `PLAYBOOK_DIR`, `DIST_DIR`, and the relevant resolved path from
`<DIST_DIR>/commands/_shared/path-resolution.md`. Apply that module first.

## The Three Kinds

| Kind | Shipped base | Project-local | Disable list | File pattern |
|---|---|---|---|---|
| `rules` | `<DIST_DIR>/rules/` | `RESOLVED_ENFORCEMENT_DIR` (`paths.enforcement`) | `overlay.rules.disabled` | `*.md` |
| `reviews` | `<DIST_DIR>/reviews/` | `RESOLVED_REVIEWS_DIR` (`paths.reviews`) | `overlay.reviews.disabled` | `review-*.md` |
| `checks` | `<DIST_DIR>/checks/` | `RESOLVED_CHECKS_DIR` (`paths.checks`) | `overlay.checks.disabled` | `check_*.sh` |

Note the asymmetry: the project-local directory for `rules` is `paths.enforcement`,
not `paths.rules`. That name is historical and stays.

## Entry Key

Every catalog entry is identified by its **key**, not its filename. Two entries with
the same key are the same logical entry, regardless of which directory they came from.

- `rules`: filename without the `.md` extension. `docs-sync.md` -> `docs-sync`
- `reviews`: filename without `.md` and without the leading `review-`.
  `review-codeql-security.md` -> `codeql-security`
- `checks`: filename without the `.sh` extension. `check_no_obvious_secrets.sh` ->
  `check_no_obvious_secrets`

Keys are compared case-sensitively.

## Non-Entries

The following are never catalog entries and must be skipped in both directories:

- `README.md` in any catalog directory — it documents the catalog.
- Anything inside a `lib/` subdirectory — shared helper code for checks, not a check.
- Files not matching the kind's file pattern from the table above.
- Dotfiles.

## Resolution

Given a kind:

```
base     = entries in the shipped directory
local    = entries in the project-local directory (empty if unset or missing)
disabled = the kind's disable list from K-PLAYBOOK.yaml (empty if absent)

effective = local
          ∪ { b ∈ base | key(b) ∉ keys(local) ∧ key(b) ∉ disabled }
```

In words:

1. Every project-local entry is active.
2. A shipped entry is active unless a project-local entry has the same key, or the
   key is listed in `disabled`.

Consequences that must hold:

- A project-local entry **replaces** a shipped entry with the same key. It does not
  merge with it, and it is not partially applied. The shipped file is not read at all.
- `disabled` only affects shipped entries. A project-local entry is never disabled by
  it; to drop a local entry, delete the file.
- A shipped entry that is both overlaid and listed in `disabled` is simply replaced by
  the local one. Report the redundant `disabled` entry as stale.

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
| `disabled` | shipped, suppressed via `overlay.<kind>.disabled` |
| `stale-disable` | key in `disabled` that matches no shipped entry |

## Reporting

Every command that applies this module reports the resolution in its preflight,
before doing work. Keep it to one block:

```text
Regeln (rules): 4 aktiv
  [dist]     codeql
  [dist]     review-authoring
  [override] docs-sync            (ueberlagert _dist/rules/docs-sync.md)
  [local]    my-api-rules
  [disabled] tool-install-scope   (via overlay.rules.disabled)
```

If the effective set is empty, say so and stop; do not fall back to the shipped
catalog. An empty set after a full `disabled` list is a deliberate project decision.

Report `stale-disable` keys as a warning and offer to remove them from
`K-PLAYBOOK.yaml`. Do not remove them silently.

## Writing Rules

- Never write to `DIST_DIR`. To change a shipped entry, create a project-local file
  with the same key.
- When a command offers to disable a shipped entry, it writes to
  `overlay.<kind>.disabled` only after explicit confirmation.
- When a command offers to override a shipped entry, it copies the shipped file into
  the project-local directory as a starting point and tells the user that the copy is
  now frozen — later updates to the shipped file will not reach it.

## Required Output From This Step

- `EFFECTIVE_<KIND>` as a list of absolute paths
- an origin per entry: `dist`, `local`, or `override`
- `DISABLED_<KIND>` as the list of suppressed shipped keys
- `STALE_DISABLE_<KIND>` as the list of disable entries matching nothing
