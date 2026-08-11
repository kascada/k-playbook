# Shared Context

Use this module at the start of every command, before any other step.

## Load the project context

Run, from anywhere inside the project:

```bash
k-playbook/bin/k-playbook context
```

The command searches upwards for `K-PLAYBOOK.yaml`, so it works from any
subdirectory. It writes JSON to stdout and exits non-zero if there is no
installation — in that case, stop and report it. Do not guess paths.

## What it answers

| Field | Use |
|---|---|
| `instructions` | Files to read first, in that order: the shipped level, then this project's. |
| `project`, `playbook`, `local` | Resolved directories. Never derive these yourself. |
| `catalogs` | Effective `rules`, `reviews` and `checks` — shipped and project-local already merged, `origin` recorded, switched-off entries marked `disabled`. |
| `remediation` | How findings are to be worked off. |
| `guidelines` | Project guideline files. |

## Rules

- **Read every file in `instructions` before doing work.** They carry what applies
  to this project; the rest of this command builds on them.
- Use the catalogs as given. The overlay rules are already applied — do not list
  directories yourself and do not re-derive which entry wins.
- Skip entries marked `disabled`. They were switched off on purpose; the file says
  why.
- Never write into the `playbook` directory. It is replaced on every update.
