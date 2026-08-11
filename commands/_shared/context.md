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

## Load it once per session

The output does not change while you work, and it is the same for every command.
If you already ran this command in this session and its output is still in the
conversation, reuse that output — do not run it again, and do not re-read the
files from `instructions`. Chained commands share one load.

Run it again only when one of these applies:

- `K-PLAYBOOK.yaml` was written since the last call.
- A rule, review, check or guideline file was added, removed or emptied since
  the last call, or k-playbook was updated.
- The work moved to a different project. The command searches upwards for
  `K-PLAYBOOK.yaml`, so another working directory can resolve to another
  installation — check `project.dir` against the one you loaded.

When one of these applies, run it again and use the new output from then on.
Never mix fields from two loads.

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
  to this project; the rest of this command builds on them. Once per session is
  enough — if you already read them, work from what you have.
- Use the catalogs as given. The overlay rules are already applied — do not list
  directories yourself and do not re-derive which entry wins.
- Skip entries marked `disabled`. They were switched off on purpose; the file says
  why.
- Never write into the `playbook` directory. It is replaced on every update.
