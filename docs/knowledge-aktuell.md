---
title: Knowledge — Current Direction
description: Working document for moving k-playbook onto Kephalaion — the goal of a binary-only installation whose shipped content lives in a public collection k-playbook on every hub, filled by the hub from the release, next to each project's own collection; the decisions taken so far, the open questions and the ordered work list.
---

# Knowledge — Current Direction

Working document for the duration of the switch, in the manner of
[`migration.md`](./migration.md): it records what has been discussed and decided, the open
questions and the work still to do. What has been implemented moves into the regular
documentation and leaves this file; the file is deleted when nothing remains open.

As of: 2026-10-17.

## Goal

k-playbook is installed as a binary only. There is no clone under `<project>/k-playbook/`
any more. What is shipped today in that clone — commands, skills, rules, reviews, checks,
scripts, guidelines, docs — lives in a public collection `k-playbook` on every Kephalaion
hub, in one directory `defaults/` holding the latest release. What is the project's own lives in the project's collection.

Why:

- No repository inside the project: no read-only directory replaced on every update, no
  gap between the installed version and the working copy, fewer links to heal.
- Commands, skills and rules are knowledge like the docs and are read the same way, over
  MCP. Kephalaion is built for it (`list` with `mask: "SKILL.md"` and `frontmatter`).
- The shipped content exists once per hub instead of once per project, and no person needs
  a right to write it.

## Decided

**One collection per project** (2026-10-09). Usually named after the project (`omni`,
`kephalaion`). It holds what is the project's own; the shipped content comes from the public
collection. The development collection of this repo is renamed from `k-playbook` to
`k-playbook-dev`, because `k-playbook` is the name of the public collection.

**The shipped content lives in a public collection on every hub** (2026-10-09, variant D;
refined 2026-10-10). Every hub has a collection `k-playbook`; every account of that hub may
read it; only the hub writes it, from the k-playbook release, into `defaults/` (see below).
k-playbook needs no further address: the public collection lies on the same hub as the
project's, `<hub>:k-playbook`. Kephalaion and k-playbook should stay separate; this
dependency is a deliberate exception, and on the Kephalaion side it sits in exactly one place:
a list embedded in the Kephalaion binary names `k-playbook` with its fixed properties —
public, source `git` with the repo URL, a cap on the total size. Creating a collection of that
name applies them. "Public" (readable by every account of the hub without a grant) becomes a
general, settable property of any collection; the source `git` exists only through that list.

Dropped on the way, the same day:

- A dedicated release account writing `vendor/k-playbook/` into every project collection:
  only the token holder can update, one copy per project, the work grows with every
  collection.
- Any user asking the hub to write `vendor/k-playbook/` into the project collection: still
  one copy per project.
- One central public hub: needs anonymous reading and a node that syncs a hub without an
  account, a public service to run, and it may be blocked in company networks. It remains a
  later extension of D — a central hub can become the source for the hubs instead of GitHub.

**Setting up a project, in this order** (2026-10-09): find a Kephalaion hub, create the
collection there, create an account per machine and grant it the collection, enter the
users. Then name the collection in k-playbook. Access runs through the person operating the
machine, with that person's account.

**Collection binding — built** (2026-10-09, task 085). Hub alias, collection and account are
machine-local in `k-playbook-local/K-PLAYBOOK.local.yaml`; the check runs over MCP (`whoami`
with the token from Kephalaion's token file); `k-playbook context` carries the result; the
interface shows it and offers the setup step that tests and then writes. Described in
[`installation.md`](./installation.md#kephalaion-collection) and
[`../installer/docs/architecture.md`](../installer/docs/architecture.md#kephalaion-anbindung).

**The hub fetches a release asset** (2026-10-10). The release workflow packs `defaults/` into
an archive `k-playbook-defaults.tar.gz` (files at the archive root, no links, reproducible
`tar` flags) and uploads it with the binaries. The hub finds the latest release through the
redirect of `github.com/<repo>/releases/latest` (no API, no token) and downloads
`releases/download/<tag>/<asset>`. A release without the asset is "not present at the
source"; the hub keeps what it has. No checksum: next to the asset in the same release it would
only guard the transfer, which TLS already does. Trust rests on whoever can publish releases of
`kascada/k-playbook` — accepted: the users rely on exactly that service. Rejected: the tarball
GitHub generates for the tag (whole repo, links to resolve in Kephalaion).

**One directory `defaults/` with the latest release** (2026-10-10; replaces "one directory
per version" from 2026-10-09). `<hub>:k-playbook/defaults/` always holds exactly one release:
the latest published one without a suffix. A fetch replaces its content as a whole, in one
revision, so a node sees either the old or the new state. There is no directory per version:
a project with an older binary reads the newer content. If versioning is needed later, it is
done in the repo itself or added then. Overriding stays as today: a file of the same name in
the project's own collection replaces the one from `defaults/`. The hub records which release
it fetched in its own database, not as a file in the collection. Documents it writes carry
`system` as author — a user of that name cannot exist.

When the hub fetches:

- when the collection is created (without network the collection is created empty, and the
  message names the command to fetch later);
- by hand, with a command of the hub admin;
- on request through the node by any account of the hub — provisional, to be restricted or
  limited later (Kephalaion todo); an upgrade of k-playbook may trigger it right away;
- later once a day, through a scheduler on the hub that is still to be built (planned for the
  knowledge base as well).

Reading never triggers a fetch: if `defaults/` is missing, it is missing. Nobody writes the
collection — no account, no node, not even the hub admin; changes come only through the
release. If the collection `k-playbook` does not exist on a hub, nothing happens. The
Kephalaion side is task 031 in the Kephalaion repo
(`k-playbook-local/tasks/031-collection-aus-release-fuellen.md`).

**The binary stays on GitHub** (2026-10-09). Kephalaion stores text up to 1 MiB per document,
every node syncs the whole collection, and a hub that hands out executables would become a
supply-chain point for every connected machine. The install script fetches the release binary
and checks it against `SHA256SUMS`, as `bin/install` does today.

**The source repo gets a directory `defaults/`** (2026-10-09 as `vendor/`, renamed
2026-10-10). It marks exactly what is shipped; its content becomes `<hub>:k-playbook/defaults/`. The repo is
public and stays public, and the binary is fetched from there already.

**`defaults/` holds the shipped Markdown, frozen** (2026-10-10). `defaults/` holds real
copies of the tracked files of `guidelines`, `reviews`, `rules` and `k-playbook.md` — 19
files, no links; `docs/` was taken out before v0.12.0 (the docs are taken over later, anew),
and `VERSION` is added from the root when packing, because the release only bumps that one. Commands, skills, scripts, `bin` and `checks` stay out: they
have to lie on disk for the assistants and to run, so they stay in the local `k-playbook/` as
today. The old places stay for the installation; two copies cannot drift because the shipped
content is not changed until the switch is complete. `defaults/instructions/` came in on
2026-10-14 and is the exception: it is maintained continuously and, since 2026-10-17, the
only place of the shipped instructions (see "A new kind `instructions/`" below). From here
on nothing new goes into the old places at the top of the repo; they are taken out kind by
kind. Who delivers the Markdown — k-playbook
from the release or Kephalaion — and how local entries override it is decided together with
the briefing. Against links at the old places, should files move later: `filepath.WalkDir`
does not descend into a symlinked root (`installer/internal/project/docs.go:40`), and a file
search that does not follow links (ripgrep, behind the Glob tool) would find nothing there.

**A reference scheme for the Markdown files** (2026-10-14, task 087). A reference in what
k-playbook ships or produces is one path with a prefix: `project:/…` (a file in the project,
read from disk), `k-playbook:/…` (resolved by k-playbook, local before shipped; ending in `/`
an assembled directory) and `keph:/…` (the project's collection, read over Kephalaion with
`collection` = `kephalaion.address`). `project:` and not `local:`, because "local" names the
layer `k-playbook-local/`; `keph:` names neither hub nor collection, because both are
machine-local. The public collection `<hub>:k-playbook` is no reference target: its
`defaults/` is the shipped layer behind `k-playbook:`. Defined in
[`verweisschema.md`](./verweisschema.md); the version for sessions is
`instructions/always/010-verweise.md`.

**A new kind `instructions/`** (2026-10-14, task 087). `instructions/always/` holds numbered
parts that are assembled into one text (numerically across both layers, shipped below 1000,
local from 1000; the number is part of the name that is overridden; files without a number are
left out and reported), `instructions/on-demand/` unnumbered files with frontmatter `title` and
`description`. Built, but deliberately not put to use: `k-playbook.md`, `AGENTS.md`, its block
template and the context output are unchanged, `catalogs` does not list the kind, and the text
of the first parts stands twice for now, in the Go code that writes `AGENTS.md` and in
`instructions/always/`. Whether `always/` replaces or supplements `k-playbook.md` is decided by
the briefing (task 086). **"Frozen" does not apply here:** `instructions/` is maintained
continuously. Until 2026-10-17 it lay twice — at the top of the repo for the installation and
as a copy in `defaults/instructions/` for the hub, kept equal by a test. **Since 2026-10-17 it
lies only in `defaults/instructions/`:** the installation reads it there (`NewShippedDocSource`
in `docref.go`), the release packs the same files, the top-level copy, the test and the drift
check in `pack-defaults.sh` are gone. `rules/`, `reviews/` and `guidelines/` are still read at
the top of the repo until their own switch.

**`k_playbook_doc_read` and `k-playbook read`** (2026-10-14, task 087). The MCP tool and the
subcommand resolve `k-playbook:/instructions/…` through an exchangeable source — the disk today
(`installer/internal/project/docref.go`). A single file comes unchanged with frontmatter; an
empty local file disables it without an error; a directory is assembled without frontmatter;
an unknown reference is `not_found` with no fallback to the disk; `project:`, `keph:` and other
`k-playbook:` kinds are `unsupported` for now. The `ref` in the prefix scheme is the lasting
contract of the tool; see [`mcp.md`](./mcp.md#doc-read-contract).

**A user area in the project's collection** (2026-10-14, built 2026-10-15 — task 088, in the
working tree, not yet put to use). Per person there is
`user/<name>/<kind>/`, for now with `tasks/` and `workbench/`. The tasks are public to everyone
in the project, but a session lists only its own by default; that is a view k-playbook
provides, not a right. One directory per person needs one Kephalaion scope (`dir user/<name>/`)
should writing ever be limited to one's own area, instead of one scope per kind. `<name>` is the
Kephalaion **user** (`kephalaion.user`), not the account: the account belongs to one machine,
so the same person on two machines would otherwise get two directories. In shipped text the
segment `~` stands for the own user (`keph:/user/~/tasks/`); see
[`verweisschema.md`](./verweisschema.md#placeholder--for-the-own-user). The tools that create,
list and move documents there: [`mcp.md`](./mcp.md#document-tools-in-the-collection).

**Findings go to `queue/findings/`** (2026-10-14; creating and listing built 2026-10-15 in
task 088, the scheduled run and the switch of the findings rule not built). What sessions find no longer
lands in `material/befunde/` but as a queue entry of its own in the collection; a scheduled run
takes the entries over into the knowledge store once the cron function exists. Two things
differ from today's queue (`docs/knowledge-layout.md`): `queue_list` reads only the top level
and skips subdirectories (`installer/internal/project/knowledge_queue.go:165`), and a queue entry
today only points at a raw piece in the inbox. A finding in the queue carries its content
itself, so after the takeover it is gone and `sources` has nothing left to point at, unless the
run moves it to the inbox first. Everything under `queue/` has no user area: `k_playbook_doc_list`
ignores `user` there and takes the user from Kephalaion's `created.by`.

**Creating a named document** (2026-10-14, built 2026-10-15 — task 088, in the working tree). One procedure for tasks, findings and
whatever needs it later: the caller passes the intended short name, k-playbook normalises it
(lower case, hyphens), takes the highest number in the counting range plus one and creates
`<number>-<name>.md` through Kephalaion's `create`. That call goes through the hub and answers
`name_taken` for a live document of the same name, and then the next number is tried. **Only the
whole file name has to be unique, not the number;** two documents with the same number and
different names are accepted. The answer is the `keph:` reference with the real user name, and
the session reads and writes the document directly over Kephalaion. A kind is one entry in a
table; below the highest number the kind's start number applies. Tasks start at 100 so they do
not collide with the task files on disk — counting those files along was dropped (2026-10-14).
Moving into `done/` and back is `rename` through the hub. Contract:
[`mcp.md`](./mcp.md#document-tools-in-the-collection); what k-playbook expects of Kephalaion:
[`kephalaion-client.md`](./kephalaion-client.md).

| Kind | Stored in | Counting range | Start |
|---|---|---|---|
| tasks | `user/<name>/tasks/` | all `user/*/tasks/` including `done/` — project-wide | 100 |
| findings | `queue/findings/` | the directory itself | 001 |
| workbench | `user/<name>/workbench/` | all `user/*/workbench/` including `done/` — project-wide | 001 |

**The briefing** (2026-10-16, task 086 — built, not yet put to use). `k-playbook briefing
[--json]` and `k_playbook_briefing` are the one call at the start of a session: **data, not
prose** — which instructions, rules, reviews and guidelines apply, and what is pending in the
project's collection. References in the prefix scheme (`k-playbook:/rules/befunde.md`,
`keph:/user/kamran/tasks/100-….md`, `project:/k-playbook-local/docs/README.md`), never paths;
each document with title, description (frontmatter, else the first heading) and origin (`dist`,
`local`, `override`, `disabled`); the own open tasks and active workbench topics (not in
`done/`, what `k_playbook_doc_list` returns without `done`) with title, and the number of open
findings in `queue/findings/`; when Kephalaion is not usable, that state instead of the lists.
Contents are read afterwards with `k_playbook_doc_read`, which since this task resolves
`rules/`, `reviews/` and `guidelines/` as well, through the one overlay that also builds
`catalogs` (`overlayDocDir`). Deliberately left out: the local knowledge store
`k-playbook-local/knowledge/`, contents, commands, skills, checks, scripts, k-playbook's own
docs, and every environment fact — paths, tools, `gh`, the Kephalaion binding stay in
`k-playbook context` alone, nothing stands in both outputs. `guidelines` is an overlay kind for
`k_playbook_doc_read` and the briefing; the field `guidelines` of the context output stays
local-only as the one named exception until the context output is switched. The briefing of
[`knowledge-gate.md`](./knowledge-gate.md#querying-the-surface) merges into this one: the
document list exists now; the counts per kind, state and origin and the freshness come with the
knowledge store later. Contract: [`mcp.md`](./mcp.md#briefing-contract); what moves where:
"Target for `k-playbook.md` and `AGENTS.md`" below.

**`/k-docs-*` write single documents** (2026-10-09). The generators write directly, document
by document, with `create`/`write` into the collection, rather than replacing whole
directories through a staging directory and `push`. Who removes stale documents is decided
when that step is built.

**Directory layout of a collection** (2026-10-09, agreed as a draft):

```
<hub>:<collection>/
  README.md            index                              /k-docs-index
  generated/           produced by the generators
    code/                                                 /k-docs-code
    libs/                                                 /k-docs-tools
    versions/                                             /k-doc-inventory
  manual/              written by hand
  extracted/                                              /k-docs-extract
  findings/                                               befunde, /k-danke
  pitfalls/
  queue/
    findings/          outstanding findings, numbered     (2026-10-14)
  user/
    <name>/            one directory per person           (2026-10-14)
      tasks/           numbered project-wide, done/
      workbench/       current state of work, numbered project-wide, done/
```

The shipped content is not part of it; it lies in `<hub>:k-playbook/defaults/`.

## Target for `k-playbook.md` and `AGENTS.md` (task 086)

The goal: a session gets its entry completely through k-playbook — `k_playbook_briefing` for
what applies, what is pending and the environment, `k_playbook_doc_read` for the contents —,
and `k-playbook.md` and the block in `AGENTS.md` are only a minimal prompt.

**The briefing replaces everything** (2026-10-16): `k-playbook.md`, the `AGENTS.md` blocks
down to the minimal prompt, and `k-playbook context` as well. The context output is taken
apart field by field — each field moves into the briefing or drops — and at the end the
command `k-playbook context` and the tool `k_playbook_context` go. Until then both work.
Todo #26. Where the table below still says "context" as a future place, that is an
intermediate state, not the target. This table (2026-10-16) assigns every section of `k-playbook.md` and every sentence of
the three `AGENTS.md` blocks (markers `k-playbook:anstoss`, `k-playbook:session-memory`,
`k-playbook:befunde`, templates in `installer/internal/project/instructions.go`) to its future
place. Nothing is moved by this task; each row is its own step later ("build first, then put
to use"), and only the last step switches `k-playbook.md`, the `AGENTS.md` block and the context
output over.

Places: `always/<n>` is a part of `instructions/always/` (shipped numbers below 1000),
`on-demand/<name>` a file in `instructions/on-demand/`, read only when a reference points at it;
"briefing" and "context" are fields of those outputs; "prompt" is the minimal `anstoss` block.

| Source | Section / sentence | Future place | State |
|---|---|---|---|
| `k-playbook.md` | intro: "gilt in jedem Projekt … `k-playbook-local/k-playbook.md` wird nach dieser Datei gelesen" | drops: the reading order is the order of `documents.always` in the briefing; local parts follow shipped ones by number | open |
| `k-playbook.md` | "Zwei Verzeichnisse, zwei Eigentümer": the tree, never write into `k-playbook/`, everything project-owned into `k-playbook-local/`, override with a file of the same name | `always/020-zwei-ebenen.md`: the two layers and the two rules in a few lines; the paths come from `context` (`playbook.dir`, `local.dir`), the detail of where a file goes stays in `k-playbook:/guidelines/betrieb.md` | open |
| `k-playbook.md` | "Überlagern und Abschalten": replace by name, empty disables, the unit per kind, commands and skills must be registered | `on-demand/ueberlagern.md`, for the session that creates, overrides or disables an entry; what a session needs every time — read `k-playbook:` only through k-playbook, origin per entry — is already in `always/010-verweise.md` and in the briefing (`origin`) | open |
| `k-playbook.md` | "Wo was liegt": no assumptions about paths, `k-playbook context` resolves directories and catalogs | prompt: the one call stays the `anstoss` block; the catalogs with origin are in the briefing | open |
| `k-playbook.md` | "Wie gearbeitet wird", first paragraph: anything beyond a trivial step becomes a task (`/k-task-create`, `/k-task-refine`, `/k-task-run`) | `always/300-arbeiten-in-tasks.md` | open |
| `k-playbook.md` | "Wie gearbeitet wird", overview of the shipped commands and the interface (`k-playbook`, `/k-gui`) | `on-demand/commands.md`; the briefing deliberately lists no commands or skills, the assistant has them registered, and `k-playbook/docs/commands.md` keeps the detail | open |
| `k-playbook.md` | "Wie gearbeitet wird", last paragraph: what applies in a project is the catalog from `context`, a local command replaces or disables the shipped one | drops into `on-demand/ueberlagern.md` (same rule) | open |
| `k-playbook.md` | "Umgang mit Befunden": `remediation.mode`, the three modes, `configured: false` means ask | `on-demand/remediation.md`, read by `/k-remediation` and when a review finding is handled; the values themselves are `remediation` in `context` | open |
| `k-playbook-local/k-playbook.md` | the project's own rules (this repo: role, memory rule, spelling, scratch pad, docs first) | local parts `k-playbook-local/instructions/always/1000-….md`, one per topic, by the project; no shipped step | open |
| `AGENTS.md` `anstoss` | "Für dieses Projekt gilt k-playbook. Rufe zu Beginn `k-playbook context` auf" | prompt — stays; at the switch reworded: `context` for the environment, `k_playbook_briefing` for what applies, then read `k-playbook:/instructions/always/` | stays |
| `AGENTS.md` `anstoss` | "lies die Dateien aus `instructions` in der angegebenen Reihenfolge" | drops: `documents.always` in the briefing is the reading order; see the name clash below | open |
| `AGENTS.md` `anstoss` | "Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und die effektiven Kataloge" | drops: directories stay in `context`, the catalogs with origin are in the briefing | open |
| `AGENTS.md` `session-memory` | "Projektwissen zuerst": read `k-playbook-local/docs/README.md` first, code only when the docs fail | `always/100-projektwissen-zuerst.md`; the entry is also `projectDocs` in the briefing | done (`NACHZIEHEN:` for the `keph:` entry later) |
| `AGENTS.md` `befunde` | "Befunde festhalten": the finishing condition, the place, the fields, `/k-danke` | `always/200-befunde-festhalten.md`; `k-playbook:/rules/befunde.md` resolves since task 086 | done (`NACHZIEHEN:` for the place `queue/findings/`) |
| — | how references are written and reached | `always/010-verweise.md` | done |

**What remains as the minimal prompt.** The `anstoss` block in `AGENTS.md`, reworded at the
switch. `k-playbook.md` does not stay: when its last section has moved, the shipped file goes,
`k-playbook-local/k-playbook.md` becomes local parts, and the context field `instructions` is
empty — which is the moment to resolve the name clash. This is the table's proposal; it is
confirmed or reversed when the switch step is planned, not before.

**Name clash `instructions` — open, deliberately.** The context field `instructions` lists the
`k-playbook.md` files in reading order; the directory `instructions/` is the kind behind
`k-playbook:/instructions/always/`, which the briefing lists under `documents.always`. Both are
right today and mean different things. The clash is resolved together with the switch of the
context output (the field changes or goes), not in task 086.

## Open

**The workbench** (2026-10-14). Meant for the current state of work: where one stands, what
is open, what comes next. Unlike a task it is no assignment, and unlike a finding it carries
no proof. Today such things end up in findings or in assistant memory; this file is itself an
example. Decided and built (2026-10-14/15, task 088): one file per topic, numbered
project-wide, and a finished topic moves to `done/`. Proposed: overwritten rather than
appended, with frontmatter `title` and `status` so that the briefing can show the titles of
one's own active topics. Undecided: whether a finished topic is also resolved (confirmed parts
as findings, open parts as tasks), and the format of its content.

**How commands and skills are read once the local files are gone.** Claude Code, OpenCode and
Cursor find commands and skills only as files on disk, and checks and scripts must run.
Without the clone the binary would probably `pull` `<hub>:k-playbook/defaults/` into a cache outside
the project and link from there. `pull` creates every file with mode `0644` (16 executable
files lose their bit) and `push` skips symlinks. Whether this carries in all three assistants
has to be measured before the local files go.

## Work list

The order as of 2026-10-09, after the decision for variant D. The old `k-playbook/` stays
until the last step.

1. **Collection binding** — built 2026-10-09 (task 085, in the working tree); see "Collection
   binding" above. Task 082 is adapted to use the same check.
2. **Rename the development collection** — done by 2026-10-13: `vm:k-playbook-dev` exists
   with `read`/`write` for `kamran-wsl`, `K-PLAYBOOK.local.yaml` points at it, and
   `vm:k-playbook` is now the public collection.
3. **`defaults/` in the source repo** — `vendor/` with links committed (`4072607`); renamed
   to `defaults/` and filled with the shipped Markdown on 2026-10-10 in the working tree
   (commands, skills and scripts stay local); released with v0.12.0.
4. **Decide what the hub fetches** — decided 2026-10-10: a release asset, see "The hub
   fetches a release asset".
5. **Release packs the content** — built 2026-10-12: `.github/scripts/pack-defaults.sh`,
   called by the release action and by `make defaults-archive`, packs `defaults/` plus `VERSION`
   reproducibly into `dist/k-playbook-defaults.tar.gz` and checks the hub's limits first;
   `make release-publish` requires the asset. Released with v0.13.0 (2026-10-13), the first
   release carrying `k-playbook-defaults.tar.gz`.
6. **Kephalaion: a collection with an external source** — readable by all accounts of the
   hub, written only by the hub, the latest release in `defaults/`, fetched at creation, by
   hand or on request.
   Built as Kephalaion task 031 (Kephalaion v0.3.0, running on the VM). First run on hub `vm`
   2026-10-13: `fetch_release vm:k-playbook` fetched v0.13.0, 20 documents under `defaults/`,
   author `system`.
7. **AGENTS.md reads the public collection** — the instructions point at
   `<hub>:k-playbook/defaults/` over MCP instead of the local `k-playbook/`.
8. **Project files into the project collection** — decide how the files of
   `k-playbook-local/` that correspond to the shipped ones (rules, commands, skills, checks,
   reviews, `k-playbook.md`) get there; adapt AGENTS.md.
9. **Briefing** — task 086 (2026-10-10; built 2026-10-16, in the working tree): the k-playbook
   MCP server lists the documents that apply, shipped and local already resolved against each
   other, and reads them by reference; the source behind it (disk today, Kephalaion later) is
   invisible to the AI. Groundwork from task 087 (2026-10-14): the reference scheme, the kind
   `instructions/` and `k_playbook_doc_read`/`k-playbook read` for `k-playbook:/instructions/…`.
   **Built in 086:** `k_playbook_doc_read` resolves `rules/`, `reviews/` and `guidelines/`
   through the one overlay that also builds `catalogs` (`overlayDocDir`, with a test comparing
   both ways); `k-playbook briefing [--json]` and `k_playbook_briefing` (see "The briefing"
   above); the table "Target for `k-playbook.md` and `AGENTS.md`" assigning every section and
   every block sentence to its future place. **Still open, deliberately:** the name clash
   `instructions` (context field with the `k-playbook.md` paths versus the directory
   `instructions/` the briefing lists), putting the briefing into `AGENTS.md`, and taking the
   context output apart into the briefing until `k-playbook context` can go (todo #26) — all
   three belong to the switch step at the end, after the sections of
   the table have moved one by one. The check against the real installation follows the next
   release.
10. **User area and findings queue** — built 2026-10-15 (task 088, in the working tree):
    `k_playbook_doc_create`, `_list`, `_done`, `_reopen` and `k-playbook doc`, measured at hub
    `vm`; see "Creating a named document". Not yet put to use: `/k-task-create`, `/k-task-run`,
    `/k-danke` and the findings rule still write to the disk, and `k-playbook.md`, `AGENTS.md`
    and the context output do not mention the tools. Calling the MCP tools from an open session
    is a follow-up test after the next release. Still open: the rules for what goes where, the
    scheduled run for `queue/findings/`, and `queue_list` for subdirectories.
11. **Generators** — `/k-docs-*` write single documents into the project collection,
    including the removal of stale ones.
12. **Delete the local files** — the old directories in the repo and the clone in the
    projects; setup, update and interface without `<project>/k-playbook/`.

## Findings

Measured state and evidence: `k-playbook-local/material/befunde/kephalaion-anbindung.md`.
