# Shared Context

Use this module at the start of every command, before any other step. It is the only
shared module; there is nothing else to apply.

## Load the project context

```bash
k-playbook/bin/k-playbook context
```

The path is relative to the project's main directory, which is normally the working
directory. If the wrapper is not there, walk upwards until a directory contains
`k-playbook/bin/k-playbook`, and call it from there. The binary then finds
`K-PLAYBOOK.yaml` on its own, so it does not matter where it was started from.

It writes JSON to stdout and exits non-zero if there is no installation — in that case,
stop and report it. Do not guess paths and do not create anything.

## Load it once per session

The output does not change while you work, and it is the same for every command. If you
already ran this command in this session and its output is still in the conversation,
reuse that output — do not run it again, and do not re-read the files from
`instructions`. Chained commands share one load.

Run it again only when one of these applies:

- `K-PLAYBOOK.yaml` was written since the last call.
- A rule, review, check or guideline file was added, removed or emptied since the last
  call, or k-playbook was updated.
- The work moved to a different project. The command searches upwards for
  `K-PLAYBOOK.yaml`, so another working directory can resolve to another installation —
  check `project.dir` against the one you loaded.

When one of these applies, run it again and use the new output from then on. Never mix
fields from two loads.

## What it answers

| Field | Use |
|---|---|
| `instructions` | Files to read first, in that order: the shipped level, then this project's. |
| `project.dir` | The main directory — where `K-PLAYBOOK.yaml` sits. |
| `project.repoRoot`, `project.vcs` | The code repository for Git operations, and whether it is under version control. |
| `playbook.dir` | The installation. Read-only; replaced on every update. |
| `local.dir` | Everything the project owns. |
| `catalogs` | Effective `rules`, `reviews` and `checks` — shipped and project-local already merged, `origin` recorded, switched-off entries marked `disabled`. |
| `remediation` | How findings are to be worked off. |
| `gh` | Whether this project uses the GitHub CLI, and whether it is usable on this machine. |
| `guidelines` | Project guideline files. |

`gh` carries two separate things. `gh.status` is the project's decision — `enabled`,
`disabled` or `unknown` — and lives in `K-PLAYBOOK.yaml`. `gh.installed`, `gh.loggedIn`,
`gh.account` and `gh.ready` are a host finding for this machine only. `gh.ready` means
both: the CLI is there and an account is on file. It is read from gh's own configuration,
not verified against the server, so a token can still be expired.

## Where the rest lives

The layout is fixed. Everything below is derived from `local.dir` and `playbook.dir`
from the output above — never from configuration, and never by searching.

| Artifact | Path |
|---|---|
| tasks | `<local.dir>/tasks/` |
| completed tasks | `<local.dir>/tasks/done/` |
| todo | `<local.dir>/TODO.md` |
| project documentation | `<local.dir>/docs/` |
| tool profiles | `<local.dir>/docs/libs/` |
| review results | `<local.dir>/results/<family>/<date>/` |
| review log | `<local.dir>/results/log.md` |
| known decisions | `<local.dir>/results/known-decisions.md` |
| results summary | `<local.dir>/results/summary-YYYY-MM-DD.md` |
| unversioned working files | `<local.dir>/priv/` |
| project-local commands | `<local.dir>/commands/` |
| project-local skills | `<local.dir>/skills/` |
| check runner | `<playbook.dir>/bin/k-check` |
| scripts | `<playbook.dir>/scripts/` |
| security tool matrix | `<playbook.dir>/scripts/security-tools.tsv` |

`<local.dir>/rules/`, `reviews/`, `checks/` and `guidelines/` are the project-local side
of the catalogs. Do not read them directly — `catalogs` and `guidelines` already contain
the merged result.

`<local.dir>/commands/` and `skills/` overlay the shipped ones by the same rule, but they
are not in `catalogs`: the assistant loaded them at startup, so by the time a command
runs, the effective set is already in place. They matter only when you **write** one —
put it there, never into `playbook.dir`, and note that it takes effect after `/k-gui`
has linked it and the assistant has been restarted.

If a directory listed here is missing, say so and stop. Do not create it silently and do
not substitute another one; `/k-gui` restores the project-local structure.

## Rules

- **Read every file in `instructions` before doing work.** They carry what applies to
  this project; the rest of the command builds on them. Once per session is enough — if
  you already read them, work from what you have.
- **Do not read `K-PLAYBOOK.yaml` yourself.** Its content reaches you through this
  output. Reading it directly means reading a second, possibly different answer.
- Use the catalogs as given. Shipped and project-local are already merged — do not list
  directories yourself and do not re-derive which entry wins.
- Skip entries marked `disabled`. They were switched off on purpose; the file says why.
- Never write into `playbook.dir`. It is replaced on every update. Everything a command
  produces goes into `local.dir`.
- **Before calling `gh`, check `gh` from this output.** Stop and report instead of
  calling it when `gh.status` is `disabled` (the project decided against it), when
  `gh.status` is `unknown` (nobody decided yet — point to `/k-gui`), or when `gh.ready`
  is false. In the last case name which half is missing: install `gh`, or run
  `gh auth login --hostname github.com`. Never install and never sign in yourself —
  both change the host, and signing in needs a browser.
- **Name `gh.account` before writing to GitHub.** Approving, merging or commenting is
  externally visible and acts as whoever is signed in. The active account is machine-wide
  and may have been switched in another project since.
