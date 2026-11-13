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
| `catalogs` | Effective `reviews` and `checks` — reviews shipped and project-local already merged, `origin` recorded, switched-off entries marked `disabled`; checks are shipped only, `origin` always `dist`. |
| `remediation` | How findings are to be worked off. |
| `gh` | Whether the GitHub CLI is installed and logged in on this machine (a host finding). |
| `guidelines` | Project guideline files. |
| `versionSources` | State of the version-inventory source configuration `keph:/k-playbook/inventory.md`: whether it is there and readable, which extra sources it configures, which areas it excludes from the default scan and which Helm values it names explicitly. Read it from here — never read that document yourself. |

`versionSources` carries the whole answer, so no command has to open the document: `ref`
says where it lives — always `keph:/k-playbook/inventory.md`, even when it is missing —,
`state` which state it is in, `present` whether it exists, `schemaVersion` which contract
version it declares (only `3` is read), `sources` the configured extra sources (`path`,
`kind`, `env`, `note`, `optional` per entry), `exclude` the patterns the default scan skips
and `helmValues` the configured Helm values (`path`, `key`, `item` per entry). Every path is
relative to the project root; nothing outside it is read. The default scan does not search the
installation `<playbook.dir>`, and it is not part of `exclude` — that rule comes from the
contract, not from the document.

`state` is one of four values:

- `present` — the document exists and is readable.
- `missing` — it is absent; the default sources apply. Not an error.
- `invalid` — it exists but is unreadable, declares a version other than `3`, or carries the
  key `roots`. `error` gives the reason with the line; `sources`, `exclude` and `helmValues`
  are empty, and a run of `k-playbook inventory` would abort.
- `unavailable` — Kephalaion is not reachable; nothing is known about the document. `error`
  gives the reason. There is no fallback to the disk and none to the default sources.

The old file `<local.dir>/version-sources.yaml` is no longer read by anything. If it is still
there, the health check reports it as a leftover; `/k-upgrade` moves its content. The field is
absent on an installation older than it. The full contract is in
`<playbook.dir>/docs/version-inventory.md`.

`gh` is a host finding for this machine only: `gh.installed`, `gh.loggedIn`, `gh.account`,
`gh.accounts` and `gh.ready`. There is no project decision about gh. Whether gh is in play is
decided by the way to GitHub alone: these fields count only where no service of the personal
GitHub access file matches the repository; where one does, its `selected` says which access
applies. `gh.ready` means both: the CLI is there and an account is on file. It is read from
gh's own configuration, not verified against the server, so a token can still be expired.

## Where the rest lives

The layout is fixed. Everything below is derived from `local.dir`, `playbook.dir` and
`project.dir` from the output above — never from configuration, and never by searching.

| Artifact | Path |
|---|---|
| tasks | `keph:/user/~/tasks/` in der Projekt-Collection, ein Dokument je Task; Form nach `k-playbook:/instructions/on-demand/tasks.md`. Der bisherige Ort auf der Platte (`tasks/` unter `local.dir`) wird nicht mehr beschrieben; `/k-upgrade` (3.3) zieht ihn um |
| completed tasks | `keph:/user/~/tasks/done/` — `/k-task-run` verschiebt sie dorthin mit `k_playbook_doc_done` |
| todo | über `/k-todo` bzw. das Subkommando `k-playbook todo`; die Ablage besitzt Go |
| project documentation | `keph:/docs/` — in der Projekt-Collection, nicht unter `local.dir`; das Briefing nennt den Einstieg unter `Projektdoku`, gelesen mit Kephalaion `list` (`recursive`, `frontmatter`), ohne Kephalaion nicht erreichbar |
| code documentation | `keph:/docs/generated/code/` — in der Projekt-Collection, nicht unter `local.dir`; geschrieben von `/k-docs-code` und dem Skill `ks-overlay-repo-analyse` nur über `k-playbook docs checkout\|publish code`, ohne Kephalaion gibt es keine |
| tool profiles | `<local.dir>/docs/libs/` — entsteht beim ersten Lauf von `/k-docs-tools` |
| version inventory | `keph:/docs/generated/versions/inventory.md` — in der Projekt-Collection, nicht unter `local.dir`; entsteht beim ersten Lauf von `/k-doc-inventory`, ohne Kephalaion gibt es kein Inventar |
| hand-written documentation | `keph:/docs/manual/` — in der Projekt-Collection, nicht unter `local.dir` |
| version inventory sources | `keph:/k-playbook/inventory.md` — in der Projekt-Collection, nicht unter `local.dir`; handgepflegt, ergänzt nur von `/k-doc-inventory` nach Bestätigung; ihr Zustand steht in `versionSources` |
| findings from analysis and debugging | `keph:/queue/findings/` in der Projekt-Collection, ein Dokument je Befund; Form nach `k-playbook:/instructions/on-demand/befunde.md`. Der bisherige Ort `<local.dir>/material/befunde/` wird nicht mehr beschrieben |
| known decisions | `<local.dir>/known-decisions.md` |
| review results | `<local.dir>/results/<family>/<date>/` |
| review log | `<local.dir>/results/log.md` |
| unversioned working files | `<local.dir>/priv/` |
| project-local commands | `<project.dir>/.k-playbook/commands/` — versioniert; das Einrichten legt es nicht an, es entsteht mit dem ersten eigenen Command |
| project-local skills | `<project.dir>/.k-playbook/skills/` — versioniert; das Einrichten legt es nicht an, es entsteht mit dem ersten eigenen Skill |
| the check runner, project-local by contract | `<playbook.dir>/bin/k-check` |
| scripts | `~/.local/share/k-playbook/scripts/` |
| security tool matrix | `~/.local/share/k-playbook/scripts/security-tools.tsv` |

Jedes Unterverzeichnis von `docs/` steht für eine Herkunft, nicht für ein einzelnes
Werkzeug. Ein Werkzeug schreibt ausschließlich in Verzeichnisse seiner eigenen Herkunft;
welches Werkzeug eine einzelne Datei geschrieben hat, steht in ihrem Frontmatter unter
`generated.by`. `docs/README.md` auf der Platte ist der alte Platten-Index, den
`/k-upgrade` (3.2) abräumt. In `keph:/docs/manual/` schreibt kein
Command Doc-Dateien. Flache `docs/*.md` aus der Zeit vor dieser Struktur haben keinen
Erzeuger: sie werden nur gelistet, geschrieben werden sie von keinem Command.

`<local.dir>/material/` is the old source side: raw notes, chat transcripts and hand-offs.
Nothing reads or indexes it. Its subdirectory `material/befunde/` holds the findings written
before they moved to `keph:/queue/findings/`; nothing writes there any more. Everything below
`material/` is left untouched by every command.

`<local.dir>/reviews/` and `guidelines/` are the project-local side of the
catalogs. Do not read them directly — `catalogs` and `guidelines` already contain the
merged result. Checks have no project-local side; a leftover `<local.dir>/checks/` is not
read, and `/k-upgrade` removes it.

`<project.dir>/.k-playbook/commands/` and `skills/` overlay the shipped ones by the same
rule, but they are not in `catalogs`: the assistant loaded them at startup, so by the time a
command runs, the effective set is already in place. They matter only when you **write**
one — put it there, never into `playbook.dir` and no longer into `<local.dir>/commands/` or
`<local.dir>/skills/`, which nothing reads since task 129, and note that it takes effect
after the next `k-playbook briefing` or `/k-gui` has linked it and the assistant has been
restarted.

If a directory listed here is missing, ask before creating exactly that directory. Do not
stop hard, do not create it silently, and never substitute another path. `/k-gui` restores
nothing here: k-playbook no longer creates `<local.dir>` or anything in it — not on
startup, not on a button, not through a tool —, so a missing directory under `local.dir`
is the normal state, not a defect of the setup. The rows marked „entsteht beim ersten
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
  produces goes into `local.dir`.
- **Before the first call to GitHub — `gh`, or a network command of git — read
  `k-playbook:/commands/_shared/gh.md` with `k_playbook_doc_read` (without MCP:
  `k-playbook read k-playbook:/commands/_shared/gh.md`) and follow it.** It alone decides
  which access applies, what to check first, what to report when GitHub cannot be reached
  and what to name before writing; this module does not repeat it.
- **Before calling a base tool, check `baseTools` from this output.** It lists the tools
  k-playbook itself calls — `bash`, `git`, `curl` or `wget`, `tar`, `python3`, `rg`.
  Say something only when `baseTools.missing` is non-empty; when nothing is missing, say
  nothing. For each entry that a command in this run actually needs, name the tool, what
  `role` says it costs you, and `baseTools.installCommand` as the way to get it. Do not
  warn in general terms and do not list tools this run does not touch. A missing base
  tool **warns, it does not stop the run** — that is the difference from a missing
  GitHub access, which stops the run. Where a fallback exists, take it and say that you did. Never
  install anything yourself: the command is printed, never run. Note that presence is
  measured in `PATH` only, so a shell function or alias is invisible — under Claude Code
  `rg` may be reported as missing although it works; that is known and accepted.
  When `baseTools.present` is false the matrix could not be read; say nothing about base
  tools then, rather than guessing.
