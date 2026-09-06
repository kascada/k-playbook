# Shared Context

Use this module at the start of every command, before any other step. It is the only
shared module; there is nothing else to apply.

## Load the project context

```bash
k-playbook context
```

`k-playbook` is installed once per host or DevContainer and is called by its name, from
the project's main directory — normally the working directory. There is no path to
resolve and nothing to search for: the binary finds `K-PLAYBOOK.yaml` by walking up from
the working directory, so it does not matter where the binary itself lives.

If the command is not found, k-playbook is not installed in this environment. Stop and
report it; the bootstrap is `make -C k-playbook install` (without make:
`k-playbook/bin/install`). Do not look for a project-local copy.

It writes JSON to stdout and exits non-zero if there is no installation — in that case,
stop and report it. Do not guess paths and do not create anything.

## Load it once per session

The resolved state does not change while you work, and it is the same for every command.
If you already ran this command in this session and its output is still in the
conversation, reuse that output — do not run it again, and do not re-read the files from
`instructions`. Chained commands share one load.

Run it again only when one of these applies:

- `K-PLAYBOOK.yaml` was written since the last call.
- A rule, review, check or guideline file was added, removed or emptied since the last
  call, or k-playbook was updated.
- The work moved to a different project. The command searches upwards for
  `K-PLAYBOOK.yaml`, so another working directory can resolve to another installation —
  check `project.dir` against the one you loaded.
- `now` is the one field that ages: it is the moment of the call, not of the write. A
  session that runs past midnight still carries yesterday's `now.date`. Load again when
  that would put a wrong date into a file.

When one of these applies, run it again and use the new output from then on. Never mix
fields from two loads.

## What it answers

| Field | Use |
|---|---|
| `now.date`, `now.timestamp` | The moment of this call, in the machine's time zone. `now.date` is `YYYY-MM-DD`, `now.timestamp` is RFC 3339. |
| `instructions` | Files to read first, in that order: the shipped level, then this project's. |
| `project.dir` | The main directory — where `K-PLAYBOOK.yaml` sits. |
| `project.repoRoot`, `project.vcs` | The code repository for Git operations, and whether it is under version control. |
| `project.languages` | The languages this project needs tools for. Always filled — a missing entry falls back to the default, so there is nothing to interpret. |
| `playbook.dir` | The installation. Read-only; replaced on every update. |
| `local.dir` | Everything the project owns. |
| `catalogs` | Effective `rules`, `reviews` and `checks` — shipped and project-local already merged, `origin` recorded, switched-off entries marked `disabled`. |
| `remediation` | How findings are to be worked off. |
| `gh` | Whether this project uses the GitHub CLI, and whether it is usable on this machine. |
| `cleanliness` | The local state of the installation: `clean`, `modified`, `untracked`, `ahead`, `devSync`, `message`. |
| `guidelines` | Project guideline files. |
| `versionSources` | State of the version-inventory source configuration `<local.dir>/version-sources.yaml`: whether it is there, which roots outside the project it opens up, which extra sources it configures and which areas it excludes from the default scan. Read it from here — never read that file yourself. |

`versionSources` carries the whole answer, so no command has to open the file:
`present` says whether it exists, `path` where it is, `schemaVersion` which contract
version it declares, `roots` the additional readable roots and `sources` the configured
extra sources (`path`, `kind`, `env`, `note`, `optional` per entry) and `exclude` the
patterns the default scan skips. The installation `<playbook.dir>` is always skipped and
is not part of `exclude` — that rule comes from the contract, not from the file. `error`
is filled only when the file is there but unreadable or of an unknown schema version —
then `roots`, `sources` and `exclude` are empty and the state is a visible finding, not an
empty result. The
field is absent on an installation older than it. The full contract is in
`<playbook.dir>/docs/versionsinventar.md`.

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
| documentation index | `<local.dir>/docs/README.md` |
| code documentation | `<local.dir>/docs/code/` — entsteht beim ersten Lauf von `/k-docs-code` oder des Skills `ks-overlay-repo-analyse` |
| tool profiles | `<local.dir>/docs/libs/` — entsteht beim ersten Lauf von `/k-docs-tools` |
| extracted documentation | `<local.dir>/docs/extracted/` — entsteht beim ersten Lauf von `/k-docs-extract` |
| version inventory | `<local.dir>/docs/versions/` — entsteht beim ersten Lauf von `/k-doc-inventory` |
| hand-written documentation | `<local.dir>/docs/manual/` |
| version inventory sources | `<local.dir>/version-sources.yaml` — handgepflegt; ihr Zustand steht in `versionSources` |
| raw material, never indexed | `<local.dir>/material/` |
| known decisions | `<local.dir>/known-decisions.md` |
| review results | `<local.dir>/results/<family>/<date>/` |
| review log | `<local.dir>/results/log.md` |
| unversioned working files | `<local.dir>/priv/` |
| project-local commands | `<local.dir>/commands/` |
| project-local skills | `<local.dir>/skills/` |
| the check runner, project-local by contract | `<playbook.dir>/bin/k-check` |
| scripts | `<playbook.dir>/scripts/` |
| security tool matrix | `<playbook.dir>/scripts/security-tools.tsv` |

Jedes Unterverzeichnis von `docs/` steht für eine Herkunft, nicht für ein einzelnes
Werkzeug. Ein Werkzeug schreibt ausschließlich in Verzeichnisse seiner eigenen Herkunft;
welches Werkzeug eine einzelne Datei geschrieben hat, steht in ihrem Frontmatter unter
`generated.by`. `docs/README.md` gehört allein `/k-docs-index`. In `docs/manual/`
schreibt kein Command Doc-Dateien; die Struktur-README aus dem Einrichten ist davon
ausgenommen. Flache `docs/*.md` aus der Zeit vor dieser Struktur haben keinen Erzeuger:
sie werden nur gelistet, geschrieben werden sie von keinem Command.

`<local.dir>/material/` is the source side: raw notes, chat transcripts and hand-offs. It
is never indexed and no command writes into it.

`<local.dir>/rules/`, `reviews/`, `checks/` and `guidelines/` are the project-local side
of the catalogs. Do not read them directly — `catalogs` and `guidelines` already contain
the merged result.

`<local.dir>/commands/` and `skills/` overlay the shipped ones by the same rule, but they
are not in `catalogs`: the assistant loaded them at startup, so by the time a command
runs, the effective set is already in place. They matter only when you **write** one —
put it there, never into `playbook.dir`, and note that it takes effect after `/k-gui`
has linked it and the assistant has been restarted.

If a directory listed here is missing, ask whether to create exactly that directory or to
run `/k-gui`, which restores the project-local structure. Do not stop hard, do not create
it silently, and never substitute another path. This applies only to the directories that
setting up creates — those in `LocalStructure()`. The rows marked „entsteht beim ersten
Lauf von …" are exempt: there, a missing directory is the normal state before the first
run, and the producing command creates its own directory without asking.

## Rules

- **Read every file in `instructions` before doing work.** They carry what applies to
  this project; the rest of the command builds on them. Once per session is enough — if
  you already read them, work from what you have.
- **Do not read `K-PLAYBOOK.yaml` yourself.** Its content reaches you through this
  output. Reading it directly means reading a second, possibly different answer.
- **Take today's date from `now.date`.** Never guess it, and do not call `date` yourself.
  Commands stamp dates into things that stay — review logs, result and run directories —
  and not every assistant is told what day it is. A guessed date in a log is
  worse than no log. Where a command writes `YYYY-MM-DD` or `<date>`, that is the value.
  An installation older than this field has no `now` in its output: say so once, then
  fall back to `date +%F`.
- Use the catalogs as given. Shipped and project-local are already merged — do not list
  directories yourself and do not re-derive which entry wins.
- Skip entries marked `disabled`. They were switched off on purpose; the file says why.
- Never write into `playbook.dir`. It is replaced on every update. Everything a command
  produces goes into `local.dir`. `cleanliness` reports whether that rule has held: it is
  the one field that looks backwards. Do not re-derive it with your own `git` calls, and
  do not repair what it reports — name it and point to `/k-gui`. `modified` or `ahead`
  means an update will fail or silently keep the wrong file; `devSync` means someone put
  a working copy there on purpose.
- **Before calling `gh`, check `gh` from this output.** Stop and report instead of
  calling it when `gh.status` is `disabled` (the project decided against it), when
  `gh.status` is `unknown` (nobody decided yet — point to `/k-gui`), or when `gh.ready`
  is false. In the last case name which half is missing: install `gh`, or run
  `gh auth login --hostname github.com`. Never install and never sign in yourself —
  both change the host, and signing in needs a browser.
- **Before calling a base tool, check `baseTools` from this output.** It lists the tools
  k-playbook itself calls — `bash`, `git`, `curl` or `wget`, `tar`, `python3`, `rg`.
  Say something only when `baseTools.missing` is non-empty; when nothing is missing, say
  nothing. For each entry that a command in this run actually needs, name the tool, what
  `role` says it costs you, and `baseTools.installCommand` as the way to get it. Do not
  warn in general terms and do not list tools this run does not touch. A missing base
  tool **warns, it does not stop the run** — that is the difference from `gh.ready`,
  which ends a PR review. Where a fallback exists, take it and say that you did. Never
  install anything yourself: the command is printed, never run. Note that presence is
  measured in `PATH` only, so a shell function or alias is invisible — under Claude Code
  `rg` may be reported as missing although it works; that is known and accepted.
  When `baseTools.present` is false the matrix could not be read; say nothing about base
  tools then, rather than guessing.
- **Name `gh.account` before writing to GitHub.** Approving, merging or commenting is
  externally visible and acts as whoever is signed in. The active account is machine-wide
  and may have been switched in another project since.
