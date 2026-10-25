---
title: Knowledge — Current Direction
description: Working document for moving k-playbook onto Kephalaion — the goal of a binary-only installation whose shipped content lives in a public collection k-playbook on every hub, filled by the hub from the release, next to each project's own collection; the decisions taken so far, the open questions and the ordered work list.
---

# Knowledge — Current Direction

Working document for the duration of the switch, in the manner of
[`migration.md`](./migration.md): it records what has been discussed and decided, the open
questions and the work still to do. What has been implemented moves into the regular
documentation and leaves this file; the file is deleted when nothing remains open.

As of: 2026-10-18.

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
and `VERSION` is added from the root when packing, because the release only bumps that one. Skills, scripts, `bin` and `checks` stay out: they
have to lie on disk for the assistants and to run, so they stay in the local `k-playbook/` as
today. Commands are on their way in since 2026-10-18: their text goes into
`defaults/commands/`, only a hull stays with the assistant (see "Commands as a hull with a
reference" below); the first is `/k-upgrade`, the rest still lies at the top as today. The old places stay for the installation; two copies cannot drift because the shipped
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

**`k_playbook_doc_read` and `k-playbook read`** (2026-10-14, task 087; all three prefixes
since 2026-10-18, task 093). The MCP tool and the subcommand are thin hulls over one function,
`ResolveDocRef` (`installer/internal/project/docref.go`), which resolves `k-playbook:`
through an exchangeable source — the disk today —, `project:` from disk below the project
root, and `keph:` through the Kephalaion client (`Client.Read`, the node's `read` from the
replica, access opened like for the `doc` tools, `~` replaced by `kephalaion.user`). A
single file comes with frontmatter; an empty local file disables a `k-playbook:` file without
an error; a directory is assembled without frontmatter; an unknown reference is `not_found`
with no fallback; `kephalaion_unavailable` when the binding is not ready. `unsupported` is
left for `k-playbook:` outside its kinds. The `ref` in the prefix scheme is the lasting
contract of the tool; see [`mcp.md`](./mcp.md#doc-read-contract) and
[`verweisschema.md`](./verweisschema.md#access-route-per-prefix). **Put to use later:**
`instructions/always/010-verweise.md` still says "`keph:` you read directly with Kephalaion
`read`" — that stays true and is extended by `k_playbook_doc_read` in the follow-up task that
moves the commands; that task also decides which of the two routes is the default for
`keph:`. Until then [`verweisschema.md`](./verweisschema.md) documents both.

**Includes** (2026-10-18, task 093). An include is a reference at a particular place in a
document; `ResolveDocRef` replaces the line by the body of the target, so that commands,
rules and instructions no longer copy shared pieces. Decided and built: (1) **Includes apply
generally**, in every document the resolution reads, with a parameter to switch them off
(`k-playbook read --raw`, `k_playbook_doc_read` with `includes: false`,
`DocReadOptions{Raw}`); there is no list of directories where they are "on". (2) **Syntax
`include:`** at the start of a line, exactly one argument — a reference with prefix or a
relative path in the same prefix —, not inside fenced code blocks, no HTML comment so that
an unresolved include stays readable in the raw text. (3) **Recursive and bounded:** depth
10, a cycle is detected on the open chain at once; the codes are `include_cycle` and
`include_too_deep`, every include error carries the chain `a.md → b.md → c.md` once.
(4) **Two error levels:** a failed include in a single document fails the whole read; in an
assembled directory (`instructions/always/`, and so in the briefing) the failing part lands
with its chain under `problems`, the others stay — a faulty local always-instruction does not
stop the session start. (5) **Kinds `includes` and `testdata`:** `defaults/includes/` holds
the blocks (`k-playbook:/includes/<name>.md`, local override wins, no block yet),
`defaults/testdata/includes/` the fixed cases that the Go tests read from disk and that
every hub carries after the release; neither appears in a catalog, the briefing or the help.
(6) `DocResult.includes` lists the inserted targets with `ref`, `origin`, `depth`,
`disabled`. Nothing shipped carries an include line yet ("build first, then put to use").
**Note for the follow-up task:** `keph:` includes are allowed by the mechanism, but they
make a document depend on the network and on `kephalaion.ok` — in always-instructions and
briefing texts only with care. Details: [`verweisschema.md`](./verweisschema.md#includes).

**Commands as a hull with a reference** (2026-10-18, task 092 — built, measured in the
assistants after the next release). A command consists of two parts: a **hull** with the
assistant — `commands/<name>.md`, frontmatter (`description`, `argument-hint`,
`allowed-tools`) plus one paragraph that names the reference `k-playbook:/commands/<name>.md`
and says to read it by the reference rule (MCP `k_playbook_doc_read`, as a fallback
`k-playbook read`) — and the **text** in `defaults/commands/<name>.md`, which `ResolveDocRef`
delivers: `commands` is a kind in `docRefKinds`, and the shipped layer reads it from
`defaults/` like `instructions` (`NewShippedDocSource`). The prefix is `k-playbook:`, not
`keph:`: shipped content is never read directly from the public collection, and the
override by project-owned content happens in the resolution. The first such command is
`/k-upgrade`: today only an info (installed version from `<playbook.dir>/VERSION`, nothing to
pull after), later the binary takes over the pulling as a subcommand and the command only
calls it and relays the message — the subcommand is not named in the text until it exists.
Includes — text blocks k-playbook inserts into a document before handing it out — are
resolved by k-playbook in `ResolveDocRef`, recursively with a limit, not by Kephalaion;
built in task 093 (see "Includes" below), with `includes` as a kind and the fixed test cases
in `defaults/testdata/`, which lands read-only on every hub. Three things are deliberate:
(1) **The hull lies by hand at the old place** `commands/`, against "nothing new goes into
the old places" — the registry links only `k-playbook/commands/`; the hull goes when the
binary generates hulls from `defaults/commands/` (own task). (2) **The hull has no
`## Erster Schritt`** and does not apply `_shared/context.md`, although
`rules/command-authoring.md` demands the section word for word; the rule stays unchanged,
the reviews in `/k-task-run` do not count it as a finding, and the hull form becomes a rule
only when the binary generates hulls. (3) **Two layers apply at once:** a project-owned
`k-playbook-local/commands/<name>.md` replaces the hull (registry) and the text (resolution)
with the same file. A local version therefore carries the text itself and must not point at
`k-playbook:/commands/<name>.md` — that would resolve to itself, because the local layer
wins. Commands appear neither in the briefing (`documents`) nor in `catalogs`; the second way
for a command is the registry, and a test checks that name, origin and disabled agree over
both ways. The check in the assistants — does Claude Code, OpenCode or Cursor read the
reference, with which tool, does it follow the text — is step 4 of task 092 after release
and `make -C k-playbook-local nachziehen`; results go to
`k-playbook-local/material/befunde/command-huelle-verweis.md`. Measured after that, not
before: the shell embedding `` !`k-playbook read …` `` in Claude Code and OpenCode as an
optimisation of the hull.

**What every session needs comes with the briefing; what a command needs becomes an include**
(2026-10-18, task 094 — built, in the working tree). The module `commands/_shared/context.md`,
which 24 commands and skills apply as their first step, is split by recipient instead of
being rebuilt as one piece. (1) **Rules for every session** go into an always part,
`defaults/instructions/always/030-briefing.md`, between the references and the project
knowledge: load once, the handful of fields (`context.now.date`, the three roots,
`documents.*`, `work`), read nothing yourself, derive every place from `local.dir`. **No
table in `always/`:** it arrives with every briefing in every session, so it carries what a
session has to do, in a few lines, with a reference for what one looks up. (2) **What one
looks up** stands behind it: the full field table and the complete list of reload conditions
in `defaults/instructions/on-demand/briefing-felder.md`, on the briefing's names
(`documents.rules`, `context.gh`, …); the test `TestDefaultsBriefingFelderBlattIstVollstaendig`
keeps the sheet complete against the briefing — every top-level key and every key of
`context`, obtained the way the briefing obtains them (`briefingContextFields`), never by
reflection over `Context`. The storage rules ("derived from `local.dir`, never searched"; "a
missing directory is asked for or created by `/k-gui`, never silently, except the rows
'entsteht beim ersten Lauf von …'") went to `guidelines/betrieb.md`. (3) **The command
include is small:** `defaults/commands/_shared/erster-schritt.md` only says to take the briefing of
the session or to fetch it (`k_playbook_briefing`, else `k-playbook briefing --json`; stop
when k-playbook is not installed) — the rules come with the briefing's own always text.
`gh.md` and `base-tools.md` beside it are bound only by the commands that call `gh` or base
tools. The three lie in `defaults/commands/_shared/`, not in `defaults/includes/`:
`commands/_shared/` is the namespace `rules/command-authoring.md` already fixes for command
modules, a relative include from a command stays in the kind `commands` and takes part in the
commands' overlay (a local `k-playbook-local/commands/_shared/erster-schritt.md` replaces the block);
`includes/` stays for blocks that more than one kind uses. (4) **`/k-upgrade` is the first
reader:** its text begins with `include: _shared/erster-schritt.md` and takes `context.playbook.dir`
from the briefing instead of calling `k-playbook context`; `k-playbook read
k-playbook:/commands/k-upgrade.md` shows the inserted body, `--raw` the include line
(`TestDefaultsKUpgradeBindetContextEin`, `TestReadKUpgradeAusDefaultsMitInclude`). The old
module and its 24 readers are untouched; they move with the commands, one by one.
(5) **`betrieb.md` has two copies on purpose:** `guidelines/betrieb.md` at the top of the
repo is the version that is read (`pick` in `docref.go` takes the kind `guidelines` from the
top of the installation), `defaults/guidelines/betrieb.md` is a byte-identical copy
maintained by hand for the hub — no script (`pack-defaults.sh`, workflows, Makefile) copies
it; `diff` between the two being empty is the check. The check against the installation —
`k-playbook read k-playbook:/instructions/always/`, `k-playbook briefing`,
`k_playbook_doc_read`, and one run of `/k-upgrade` in Claude Code — follows the next release
and `make -C k-playbook-local nachziehen`; the Go tests against the working tree `defaults/`
(`installer/internal/project/shipped_defaults_test.go`) are the acceptance until then.
**Put to use later:** `rules/command-authoring.md:31` still says `commands/_shared/` holds
"genau eine Datei: `context.md`" — reworded when the rule gains the hull form (work list 15);
`verweisschema.md` ("Nothing shipped carries an include line yet") and the "Includes"
paragraph above are superseded by `/k-upgrade`'s include line since this task.

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

**Writing the findings comes first, reading them later** (2026-10-24). The findings rule, the
skill `befunde` and `/k-danke` are switched to `queue/findings/` on their own; the scheduled run
that takes the entries over and `queue_list` for subdirectories follow as a step of their own.
The switch does not wait for the `/k-docs-*` commands: they are outdated and are rebuilt in
item 11 of the work list, so the step order of `docs/knowledge-layout.md` ("Migration") no
longer binds the findings. With it the conflicts noted for the switch of the generators to
`publish` on 2026-09-15 drop (`/k-docs-tools` updating tool by tool, the scope of
`/k-docs-code`, `overlay-repo-analyse` writing into `code/`), and so does `/k-danke` handing
over to `/k-docs-extract befunde`.

The first step is built (2026-10-24, in the working tree, not yet measured in a session). The
rule `rules/befunde.md` is gone; the format is an on-demand instruction,
`k-playbook:/instructions/on-demand/befunde.md`, because it is guidance for writing a finding,
not something `/k-enforcement` can check against a diff. `always/200-befunde-festhalten.md`
names it, as `030-briefing.md` names `on-demand/briefing-felder.md`; the skill `ks-befunde` and
`/k-danke` only point at it. The decisions behind it:

- **One document per finding**, created with `k_playbook_doc_create` (kind `findings`) and
  written once with Kephalaion `write`; nothing is appended. Related findings share the header
  field `thema`, by which the takeover bundles them. A wrong finding is not changed; a new one
  with `sicherheit: widerlegt` names the thesis in words.
- **Findings are not referenced.** They wait for the takeover and disappear afterwards. What a
  running effort needs as evidence goes into its workbench topic, what code needs into the
  comment at the code.
- **Header:** `title`, `thema`, `sicherheit`, `sitzung`, `entstanden` (`arbeit` or `k-danke`),
  `ziel` — still German keys and no `type:`, so a finding never looks like a finished document.
- **One kind, not two:** what `/k-danke` adds is an ordinary finding with `entstanden: k-danke`,
  and the user's decision is the field `ziel` (`wissen`, `falle`, `regel`, `verwerfen`), set
  only after confirmation. `/k-danke` no longer writes `guidelines/fallen.md`, `rules/` or
  anything under `knowledge/`, and no longer hands over to `/k-docs-extract`; the takeover from
  the queue acts on `ziel`. Until it exists, nothing reaches `fallen.md` or `rules/`.
- **Without Kephalaion** the session does not fall back to the disk; it says so and gives the
  finding in full in its answer.

Step two moves the old findings in `material/befunde/` and the references to them (tasks, Go
comments, docs). It is a conversion in `/k-upgrade` (`defaults/commands/k-upgrade.md`, step 3.1,
written 2026-10-24), because judging what a finding is still worth needs the session, not the
program: referenced findings become workbench topics and the references point there, the rest
becomes workbench topics (something left to do), findings with `entstanden: migration` (only
useful) or is dropped, each after the user confirms; processed files are deleted. Done tasks are
not searched, their references stay as history. Until then `material/befunde/` stays as it is and is no longer written. The README text the
setup writes into `material/` (`installer/internal/project/local.go`) and this repo's
`material/README.md` say so since 2026-10-24; in an existing project, whose README the setup
never rewrites, 3.1 replaces the old paragraph.
The identifier for `sitzung` is the briefing's field `session` (briefing schema `"5"`,
2026-10-24): k-playbook assigns it with every briefing, because only Claude Code puts one into
the environment (`CLAUDE_CODE_SESSION_ID`) and OpenCode none (`OPENCODE`, `OPENCODE_PID` only,
measured with `opencode run`). The plain text carries it in its first line since 2026-10-25,
together with the request to read the output in full and the titles of the always-parts
(`briefingTextHead`), because a Claude Code session read the briefing with `| head -80` and so
missed both the findings instruction and the identifier.

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
| todos | `todos/` | the directory including `done/` | 001 |

**Todos become documents in the collection** (decided 2026-10-19, built 2026-10-20 in task
097, not yet released). The todos were one file, `k-playbook-local/data/todos.json`, owned by
Go and reached only through `k_playbook_todo_*` and `k-playbook todo` (`/k-todo`, the GUI page
`/workflows/todos`). `docs/knowledge-layout.md` ("What stays outside") left that file on disk
as control, not knowledge — the same line also kept `tasks/` outside, and those moved with task
088. The todos followed: one document per entry under `todos/` with `done/`, project-wide and
not under `user/<name>/`, because a todo belongs to the project, not to a person. Numbering as
in the table (the directory itself, start 001; after deleting the highest number it is handed
out again); `created` comes from the frontmatter for migrated entries (even empty), else from
Kephalaion's stamp; the date of ticking off is a `done:` frontmatter field written before the
`rename` into `done/`. Nothing changed in Kephalaion. Built: `Write` and `Delete` in the Go
client, the kind `todos` in the kind table (kept out of the `doc` tools), the four
`k_playbook_todo_*` tools and `k-playbook todo` on the collection with a `search`, the GUI with
creation, ticking off, reopening and a search (todo #14), and `k-playbook todo migrate`, which
took over the 28 entries of this repo with their ids. Without a usable Kephalaion there are no
todos, in the GUI as well. Contract: `docs/mcp.md`, "Todo Contract and Migration". Still to
follow: `commands/k-todo.md` (`kephalaion_unavailable`, `todos_not_migrated`, no `migrated`,
`search`), `docs/knowledge-layout.md` and the other descriptions of the old store; taking over a
not yet migrated `data/todos.json` in `k-playbook upgrade`; the MCP tools at an open session
after the release. Until the release the installed binary still writes to `data/todos.json`;
a new `k-playbook todo migrate` takes such a file over as a latecomer.

**The briefing** (2026-10-16, task 086 — built, not yet put to use). `k-playbook briefing
[--json]` and `k_playbook_briefing` are the one call at the start of a session: **data, not
prose** — which instructions, rules, reviews and guidelines apply, and what is pending in the
project's collection. References in the prefix scheme (`k-playbook:/rules/befunde.md`,
`keph:/user/kamran/tasks/100-….md`, `project:/k-playbook-local/docs/README.md`), never paths;
each document with title, description (frontmatter, else the first heading) and origin (`dist`,
`local`, `override`, `disabled`); the own open tasks and active workbench topics (not in
`done/`, what `k_playbook_doc_list` returns without `done`) with title, and the number of open
findings in `queue/findings/`; when Kephalaion is not usable, that state instead of the lists.
Since 2026-10-18 the briefing also carries the text of the always-instructions itself
(`documents.always.content`, the plain text starts with it; schema `"3"`) — the one content in
the briefing, assembled by the same function as `k-playbook read` (`joinDocDirParts`), because
every session reads it first. All other contents are read afterwards with `k_playbook_doc_read`, which since task 086 resolves
`rules/`, `reviews/` and `guidelines/` as well, through the one overlay that also builds
`catalogs` (`overlayDocDir`). Deliberately left out: the local knowledge store
`k-playbook-local/knowledge/`, contents, commands, skills, checks, scripts, k-playbook's own
docs. Environment facts — paths, tools, `gh`, the Kephalaion binding — stood in
`k-playbook context` alone; since task 091 (2026-10-17) they are in the briefing too, but only
in its transitional section `context`, the rest of the context output that the briefing does
not cover itself (see "Target for `k-playbook.md` and `AGENTS.md`" below); nothing stands in
both the briefing's own sections and the context output. `guidelines` is an overlay kind for
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
  todos/             project todos, numbered, done/     (2026-10-19, built in task 097)
  user/
    <name>/            one directory per person           (2026-10-14)
      tasks/           numbered project-wide, done/
      workbench/       current state of work, numbered project-wide, done/
```

The shipped content is not part of it; it lies in `<hub>:k-playbook/defaults/`.

**Two assistants: Claude Code and OpenCode** (decided 2026-10-18, task 095 — built
2026-10-19, in the working tree). Cursor is no longer a supported assistant. It could not be
measured — there is no Cursor CLI, the chat window was the only observation point — so every
check in the assistants turned into a step for the user, and two assistants are easier to
keep in step. The registry in the binary knows `.claude/` and `.opencode/` only, MCP setup
writes `.mcp.json` and `opencode.json[c]`, the interface offers two assistants, and the
`CLAUDE.md` stub says "erreicht OpenCode nicht". Projects that still carry a `.cursor/` from
earlier setups run unchanged: k-playbook does not touch directories it no longer manages, and
old symlinks in `.cursor/commands/` keep pointing at `k-playbook/commands/…`. Their
`CLAUDE.md` keeps the old stub wording, because `applyIncludeFile` and `k-playbook upgrade`
never rewrite an existing file; the check stays `ok` and only reports "daneben eigener
Inhalt". Process detection for the session lock (`KindCursor` in `branches/sessions.go`)
stays, like `KindCodex`: an open Cursor session still blocks a branch switch. What is left
is item 19 of the work list.

**Files that must lie on disk are installed once per machine** (2026-10-25). The end goal
stays: the binary plus Kephalaion, no clone `<project>/k-playbook/`. Everything that can be
read over MCP goes to Kephalaion. What the assistants need as a file on disk — today the
commands and the skills — the binary carries embedded and unpacks into
`~/.local/share/k-playbook/` (`$XDG_DATA_HOME` when set; the same path on macOS, because
`~/Library/Application Support` has a space and the binary lies in `~/.local/bin` there too).
Like today's `k-playbook/` it is shipped, read-only and the same for every project, so it is
installed and updated once per machine, not per project. Size as of v0.16.11: commands and
skills are 386 KB (139 KB packed) next to an 18.6 MB binary. The directory already holds
`security-tools/`. A DevContainer mounts it or runs its own install. The commands' text keeps
moving to Kephalaion (items 15 and 16 of the work list), the skills' later; what stays on disk
then is the hull. This replaces the cache pulled from the hub that "Open" assumed. Three
stages, item 20 of the work list.

## Target for `k-playbook.md` and `AGENTS.md` (task 086)

The goal: a session gets its entry completely through k-playbook — `k_playbook_briefing` for
what applies, what is pending and the environment, `k_playbook_doc_read` for the contents —,
and `k-playbook.md` and the block in `AGENTS.md` are only a minimal prompt.

**The briefing replaces everything** (2026-10-16): `k-playbook.md`, the `AGENTS.md` blocks
down to the minimal prompt, and `k-playbook context` as well. The context output is taken
apart field by field — each field moves into the briefing or drops — and at the end the
command `k-playbook context` and the tool `k_playbook_context` go. Until then both work.
Todo #26. Since task 091 (2026-10-17) the briefing carries the rest of the context output in
its section `context` — one call is enough, and the rest shows what is left to take apart;
the table "Context field → future place" below is the authority for where each field goes.
This table (2026-10-16) assigns every section of `k-playbook.md` and every sentence of
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
| `k-playbook.md` | "Zwei Verzeichnisse, zwei Eigentümer": the tree, never write into `k-playbook/`, everything project-owned into `k-playbook-local/`, override with a file of the same name | `always/020-zwei-ebenen.md` was the proposal for the two layers and the two rules. Since task 094 (2026-10-18) the part "never write into `k-playbook/`, the roots come from the briefing (`context.project.dir`, `context.local.dir`, `context.playbook.dir`), everything project-owned under `local.dir`" stands in `always/030-briefing.md`; the detail of where a file goes stays in `k-playbook:/guidelines/betrieb.md`. What is left for a `020-zwei-ebenen.md` is only the overlay rule (a file of the same name replaces, an empty one disables) — whether that still needs an always part of its own or is covered by `origin` in the briefing and `on-demand/ueberlagern.md` is noted here, not decided and not built | partly done (094); rest open |
| `k-playbook.md` | "Überlagern und Abschalten": replace by name, empty disables, the unit per kind, commands and skills must be registered | `on-demand/ueberlagern.md`, for the session that creates, overrides or disables an entry; what a session needs every time — read `k-playbook:` only through k-playbook, origin per entry — is already in `always/010-verweise.md` and in the briefing (`origin`) | open |
| `k-playbook.md` | "Wo was liegt": no assumptions about paths, `k-playbook context` resolves directories and catalogs | prompt: the one call stays the `anstoss` block; the catalogs with origin are in the briefing | open |
| `k-playbook.md` | "Wie gearbeitet wird", first paragraph: anything beyond a trivial step becomes a task (`/k-task-create`, `/k-task-refine`, `/k-task-run`) | `always/300-arbeiten-in-tasks.md` | open |
| `k-playbook.md` | "Wie gearbeitet wird", overview of the shipped commands and the interface (`k-playbook`, `/k-gui`) | `on-demand/commands.md`; the briefing deliberately lists no commands or skills, the assistant has them registered, and `k-playbook/docs/commands.md` keeps the detail | open |
| `k-playbook.md` | "Wie gearbeitet wird", last paragraph: what applies in a project is the catalog from `context`, a local command replaces or disables the shipped one | drops into `on-demand/ueberlagern.md` (same rule) | open |
| `k-playbook.md` | "Umgang mit Befunden": `remediation.mode`, the three modes, `configured: false` means ask | `on-demand/remediation.md`, read by `/k-remediation` and when a review finding is handled; the values themselves are the field `remediation` — its place is in the table "Context field → future place" below | open |
| `k-playbook-local/k-playbook.md` | the project's own rules (this repo: role, memory rule, spelling, scratch pad, docs first) | local parts `k-playbook-local/instructions/always/1000-….md`, one per topic, by the project; no shipped step | open |
| `AGENTS.md` `anstoss` | "Für dieses Projekt gilt k-playbook. Rufe zu Beginn `k-playbook context` auf" | prompt — stays; at the switch reworded to one call, the briefing (`k_playbook_briefing`, else `k-playbook briefing`): what applies, what is pending, the text of the always-instructions (`documents.always.content`, since 2026-10-18) and — until the rest is taken apart — the environment from its section `context`. What the switch has to rebuild is under "Where `k-playbook context` is anchored" below | done (task 096, 2026-10-19: `instructionsBlock()` is the briefing version, the `AGENTS.md` of this repo; the `k-playbook context` version is the second superseded one) |
| `AGENTS.md` `anstoss` | "lies die Dateien aus `instructions` in der angegebenen Reihenfolge" | drops: `documents.always.content` in the briefing is the text itself, already in reading order (since 2026-10-18); see the name clash below | done (task 096, 2026-10-19: `instructionsBlock()` is the briefing version, the `AGENTS.md` of this repo; the `k-playbook context` version is the second superseded one) |
| `AGENTS.md` `anstoss` | "Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und die effektiven Kataloge" | drops: the directories are the fields `project`, `playbook` and `local` — their place is in the table "Context field → future place" below —, the catalogs with origin are in the briefing | done (task 096, 2026-10-19: `instructionsBlock()` is the briefing version, the `AGENTS.md` of this repo; the `k-playbook context` version is the second superseded one) |
| `AGENTS.md` `session-memory` | "Projektwissen zuerst": read `k-playbook-local/docs/README.md` first, code only when the docs fail | `always/100-projektwissen-zuerst.md`; the entry is also `projectDocs` in the briefing | done (`NACHZIEHEN:` for the `keph:` entry later) |
| `AGENTS.md` `befunde` | "Befunde festhalten": the finishing condition, the place, the fields, `/k-danke` | `always/200-befunde-festhalten.md`; the format since 2026-10-24 in `k-playbook:/instructions/on-demand/befunde.md` (was `k-playbook:/rules/befunde.md`) | done; the place is `queue/findings/` since 2026-10-24 |
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

**Context field → future place** (task 091, 2026-10-17). This table is the authority for the
place of every field of the context output; where the table above still names one of these
fields, it points here. Since task 091 the briefing carries everything the context output
has and the briefing's own sections do not: section `context` (JSON) resp. the part behind
the separator "Noch aus k-playbook context — wird abgebaut" (plain text), built from the same
function as `k-playbook context` (`ContextForDir`), minus the fields listed at exactly one
place in the code (`briefingCoveredContextFields` in `project/briefing.go`). State `in the
briefing` means: the field has its own place in the briefing and is left out of the rest;
`in the rest` means: it is only in `context` for now. Every later step takes one field out of
the rest and gives it its own place — or lets it drop — and reworks the readers named in the
row; when the rest is empty, `k-playbook context` and `k_playbook_context` go. The future
places are this table's proposal, the names are confirmed when the field moves. Readers are
the commands and skills that name the field today (`grep -rn <field> commands skills`;
`_shared/context.md` is the entry of every command and names almost every field).

| Context field | Future place in the briefing | Readers today | State |
|---|---|---|---|
| `schemaVersion` (the configuration's version) | drops: the briefing has its own `schemaVersion`; the configuration's version is the tool's concern (`CheckSchema` stops before anything is built), no command reads it | — | in the rest |
| `now` (`date`, `timestamp`) | `now`, same shape — a session has to take the date from here, not guess it | `_shared/context.md`, since 094 as `context.now.date` also `always/030-briefing.md` and `on-demand/briefing-felder.md`; `/k-audit`, `/k-docs-extract`, `/k-remediation`, `/k-review`, `/k-task-refine`, `/k-task-run` | in the rest |
| `instructions` (the `k-playbook.md` files in reading order) | drops: `documents.always` is the reading order, `k-playbook-local/k-playbook.md` becomes local parts (see "Name clash"); the standard header "lies die Dateien aus `instructions`" of every command then reads `k-playbook:/instructions/always/` instead | `_shared/context.md` and the header of 20 commands and 4 skills (`/k-audit`, `/k-danke`, `/k-doc-inventory`, `/k-docs*`, `/k-enforcement`, `/k-gui`, `/k-pr-review`, `/k-remediation`, `/k-review`, `/k-task-*`, `/k-test-check`, `/k-todo`, `/k-verlauf`, `/k-vscode-project-color`; `ks-ai-session-memory`, `ks-befunde`, `ks-enforcement`, `ks-overlay-repo-analyse`) | in the rest |
| `project` (`dir`, `repoRoot`, `vcs`, `config`, `languages`) | `project`, same shape — the one place a command takes the project root, the repository and the languages from | `_shared/context.md`, since 094 as `context.project.dir` also `always/030-briefing.md` and `on-demand/briefing-felder.md`; `_docs/code.md`, `/k-audit`, `/k-docs`, `/k-docs-extract`, `/k-docs-index`, `/k-enforcement`, `/k-pr-review`, `/k-remediation`, `/k-task-refine`, `/k-task-run`, `/k-todo`; `ks-enforcement` (`PLAYBOOK.md`) | in the rest |
| `playbook` (`dir`) | `playbook`, same shape — every shipped path a command needs (`bin/k-check`, `scripts/`) derives from it ("Where the rest lives" in `_shared/context.md`) | `_shared/context.md`, since 094 as `context.playbook.dir` also `always/030-briefing.md`, `on-demand/briefing-felder.md` and `defaults/commands/k-upgrade.md` (the first reader on the briefing); `_docs/inventory.md`, `/k-docs-index`, `/k-enforcement`, `/k-pr-review`, `/k-task-run`; `ks-ai-session-memory`, `ks-enforcement`, `ks-overlay-repo-analyse` | in the rest |
| `local` (`dir`) | `local`, same shape — every project-owned path derives from it | `_shared/context.md`, since 094 as `context.local.dir` also `always/030-briefing.md`, `on-demand/briefing-felder.md` and `guidelines/betrieb.md`; `_docs/code.md`, `_docs/inventory.md`, `_docs/tools.md`, `/k-audit`, `/k-danke`, `/k-docs`, `/k-docs-extract`, `/k-docs-index`, `/k-enforcement`, `/k-pr-review`, `/k-remediation`, `/k-review`, `/k-task-create`, `/k-task-refine`, `/k-task-run`; `ks-enforcement` | in the rest |
| `remediation` (`mode`, `target`, `grouping`, `quickWins`, `branchPrefix`, `prRequired`, `directFixes`, `configured`) | `remediation`, same shape; the meaning of the modes moves to `on-demand/remediation.md` (row "Umgang mit Befunden" above) | `_shared/context.md`, `_audit/review-scan-triage.md`, `/k-audit`, `/k-pr-review` (`remediation.target`), `/k-remediation`, `/k-review` | in the rest |
| `gh` (`status`, `configured`, `installed`, `path`, `loggedIn`, `account`, `accounts`, `tokenFromEnv`, `ready`, `host`) | `gh`, same shape — the decision and the host finding a PR review has to check before calling `gh` | `_shared/context.md`, `/k-pr-review`; since 094 as `context.gh.*` the include `defaults/commands/_shared/gh.md` (not yet bound by any command) and `on-demand/briefing-felder.md` | in the rest |
| `mcp` (`required`, `configured`) | drops: no command reads it; the required servers are the interface's concern, and the interface reads `BuildContext` directly (`webui/context.go`), not the output | — | in the rest |
| `git` (`switch`, `allow`, `environments`, `configured`) | drops: no command reads it (commands check `project.vcs` and `baseTools.missing`, not `git`); the branch switch is the interface's, which reads `BuildContext` directly | — | in the rest |
| `baseTools` (`matrix`, `present`, `error`, `missing`, `ok`, `installCommand`) | `baseTools`, same shape — the warning before a command calls a base tool | `_shared/context.md`, `/k-task-run`, `/k-verlauf`; since 094 as `context.baseTools.*` the include `defaults/commands/_shared/base-tools.md` (not yet bound by any command) and `on-demand/briefing-felder.md` | in the rest |
| `catalogs.checks` (`name`, `key`, `path`, `origin`, `disabled`) | `checks`: own list beside `documents`, entries with `key`, `origin` and `disabled` — checks are run through `bin/k-check`, not read, so they are no `documents` entry and keep no `ref`; whether `path` stays is decided when the field moves | `_shared/context.md`, `/k-enforcement` | in the rest |
| `catalogs.rules` | `documents.rules` — `ref`, `title`, `description`, `origin` instead of `path`; readers switch from the path to `k_playbook_doc_read` | `_shared/context.md`, `/k-danke`, `/k-enforcement`, `/k-pr-review`, `/k-review`; `ks-befunde`, `ks-enforcement` (`SKILL.md`, `PLAYBOOK.md`) | in the briefing |
| `catalogs.reviews` (with `audit`/`review` modes per recipe) | `documents.reviews` — but the briefing entry carries no `audit`/`review` block (`parseReviewModes`); `/k-audit` and `/k-review` choose recipes by these modes, so before `catalogs` goes the review entries in the briefing get the modes, or the readers take them from the recipe's frontmatter — rework to decide at the switch | `_shared/context.md`, `_audit/review-scan-triage.md`, `/k-audit`, `/k-pr-review`, `/k-review`, `/k-task-run` | in the briefing |
| `guidelines` (files of `k-playbook-local/guidelines/` only) | `documents.guidelines` — a different meaning: the context field names only the local files, `documents.guidelines` the overlay of shipped (`k-playbook/guidelines/`, today `betrieb.md`) and local ([mcp.md](mcp.md#briefing-contract), `guidelinesKind` in `context.go`); readers see more after the switch — that is rework and belongs to this row | `_shared/context.md` (the field); `/k-danke` and `ks-befunde` write `<local.dir>/guidelines/fallen.md`, the directory, not the field | in the briefing |
| `versionSources` (`path`, `present`, `schemaVersion`, `roots`, `sources`, `exclude`, `helmValues`, `error`) | `versionSources`, same shape — the contract says no command reads the file itself ([version-inventory.md](version-inventory.md)) | `_shared/context.md`, `_docs/inventory.md`, `/k-docs` | in the rest |
| `kephalaion` (`configured`, `hub`, `collection`, `account`, `address`, `state`, `ok`, `message`, `rights`, `scopes`, `user`, `node`, `nodeVersion`, `sync`) | `work`: `work.user` is there, `work.available`/`work.message` stand for `ok`/`message`; `address` gets `work.collection` so that `keph:` references can be read; the rest of the status (hub, account, state, rights, node, sync) drops — the interface shows it | no command; `instructions/always/010-verweise.md` names `kephalaion.address`, `kephalaion.ok` and `kephalaion.user` (in the briefing under `context`, reworded to `work` at the switch) | in the rest |
| `links` (`healed`, `open`, `note`; only when there is something to report) | `links`, same shape and only when there is something to report. The healing itself (`HealLinks`) runs in `ContextForDir` today; the briefing heals because its rest comes through that function. Before `context` goes, the healing has to sit in `BuildBriefing` itself — otherwise the briefing would stop healing the moment the rest is removed | no command reads it; the `note` addresses the session | in the rest |

**Where `k-playbook context` is anchored** (task 091, 2026-10-17). The call is anchored and
checked at these places; per place, what the switch to the briefing has to rebuild. None of it
is rebuilt by task 091 — "build first, then put to use".

- **The `anstoss` block in `AGENTS.md`** (`instructionsBlock()` in
  `installer/internal/project/instructions.go`, "Rufe zu Beginn `k-playbook context` auf").
  Checked is not the text but the marker `<!-- k-playbook:anstoss -->` (`instructionsMarker`,
  `CheckRootInstructions` → `HasMarker`). "Outdated" is defined narrowly: only a block that
  still calls the superseded wrapper `/bin/k-playbook context` (`legacyInstructionsCommand`)
  and whose end can be determined — literally against `legacyInstructionsBlockLines`, or at the
  next heading or HTML comment (`instructionsBlockEnd`) — is replaced by itself
  (`replaceOutdatedInstructionsBlock`), at the GUI start (`RepairRootInstructions`,
  `cmd/k-playbook/gui.go`) and at setup (`ApplyAssistantSetup`, `setup.go`, reported as
  `RootRefreshed`); every other version belongs to the project and stays. The assistant card
  of the interface (`webui/assistant.go`, `describeSetup`, "Der veraltete Anstoß … wurde auf den
  aktuellen Aufruf gebracht") reports a replaced block after setup, and
  `HasOutdatedAnstoss` marks one whose end could not be determined
  (`errAnstossEndeUnbestimmt` — the hint names `k-playbook context` as the target by hand).
  **To rebuild:** the current block becomes the second superseded version — a second list of
  lines beside `legacyInstructionsBlockLines`, line by line as shipped, with its call line
  (`k-playbook context`, `InstalledCommandName`) as the one line that may vary —, so that
  `knownInstructionsBlockEnd` finds its end positively and existing projects get the new
  block without losing project text; `legacyInstructionsCommand` becomes a list of superseded
  calls, and `errAnstossEndeUnbestimmt` names the new call. The session-memory block
  (`sessionMemoryBlock()`, marker `k-playbook:session-memory`) and the findings block
  (`findingsBlock()`, marker `k-playbook:befunde`) are superseded the same way: their content
  is `instructions/always/100-…` and `200-…` (rows above, state done), so the template
  (`rootInstructionsTemplate()`) shrinks to the one block, and the two blocks are recognized
  by their markers and removed from existing projects — `ApplyRootInstructions` stops adding
  them (`applyRootInstructions`, the check on `sessionMemoryMarker`). Tests:
  `instructions_test.go`, `setup_test.go`; the shipped `AGENTS.md` of this repo and of the
  installation carry the current block.
- **`commands/_shared/context.md`**, the entry of every command: "Load the project context"
  calls `k-playbook context`, "Load it once per session" names the fields that age or change,
  "What it answers" lists the fields, "Where the rest lives" derives every path from
  `local.dir` and `playbook.dir`, "Rules" read `instructions`, `now.date`, `catalogs`, `gh`,
  `baseTools`. About 20 commands and four skills apply the module and name `k-playbook
  context` themselves (`grep -rln "k-playbook context" commands skills`). **To rebuild:** the
  module calls the briefing (`k_playbook_briefing`, else `k-playbook briefing --json`), the
  field table follows the table "Context field → future place", and the rules switch from
  `instructions` to `k-playbook:/instructions/always/` and from `catalogs` paths to
  `documents` references; the commands' standard header drops the call. Per field, the
  readers are in the table above; they switch when the field moves, not before.
  **Rebuilt in task 094 (2026-10-18), beside the old module:** the rebuild lies in
  `defaults/commands/_shared/` — `context.md` (take or fetch the briefing, a few lines),
  `gh.md`, `base-tools.md` — and in `instructions/always/030-briefing.md` plus
  `instructions/on-demand/briefing-felder.md`; "Where the rest lives" went to
  `guidelines/betrieb.md`. `/k-upgrade` is the first command on it (`include:
  _shared/context.md`). The old `commands/_shared/context.md` stays unchanged until the last
  of its 24 readers has moved (work list 16); each command switches to the include when it
  moves. `rules/command-authoring.md:31` ("genau eine Datei: `context.md`") is reworded with
  the hull form, not before (work list 15).
- **`k-playbook.md`, "Wo was liegt"** (shipped, and read through the context field
  `instructions`): "`k-playbook context` liefert die aufgelösten Verzeichnisse und die
  effektiven Kataloge". **To rebuild:** row "Wo was liegt" in the table above — the section
  drops into the prompt; the directories are the fields `project`, `playbook`, `local`.
- **`instructions/always/010-verweise.md`** (shipped as `defaults/instructions/always/`):
  names `kephalaion.address`, `kephalaion.ok` and `kephalaion.user` and says (since
  2026-10-18) that they stand in the briefing under `context` and in the output of
  `k-playbook context`. **To rebuild:** reworded to the briefing's `work` (`work.collection`,
  `work.available`, `work.user`; row `kephalaion` above) when the field moves — a text change
  in the shipped part, which `AGENTS.md` does not duplicate.
- **MCP `k_playbook_context`** (`mcpserver/server.go`, `contextTool`; `toolDescription`
  says "dieselbe Antwort wie das Subkommando"), the subcommand `context` (`printContext` in
  `cmd/k-playbook/main.go`) and the usage text: the two facades on `ContextForDir`. The
  review tools (`mcpserver/review.go`) and the interface (`webui/context.go`) use
  `BuildContext` directly and are not affected. **To rebuild:** both go when the rest is
  empty; the healing of the assistant links moves into `BuildBriefing` first (row `links`
  above); `docs/mcp.md` (intro, tool table, "Briefing Contract"), `docs/k-playbook-format.md`
  ("The resolved working state") and `k-playbook-local/docs/manual/assistenten-verlinkung.md`
  ("Wo die Rechnung ausgelöst wird") follow.

**Inventory of `k-playbook context` and `k_playbook_context`** (task 096, 2026-10-19; raised with
`grep -rn 'k-playbook context\|k_playbook_context'`, tests excluded). Three states: **switched
in 096** — the place names the briefing now; **stays until its reader moves** — the commands,
skills and shipped texts that still call `k-playbook context` and switch one by one ("build
first, then put to use"); **stays on purpose** — the call itself, its facades, and texts that
describe the rest of the context output while it exists. Nothing the binary **writes** into a
project names `k-playbook context` any more (checked: `rootInstructionsTemplate()`,
`instructionsBlock()`, the READMEs of `EnsureLocal`, `.mcp.json`/`opencode.json` entries,
`claudeIncludeStub()`).

| Place | What it does | State |
|---|---|---|
| `installer/internal/project/instructions.go` | template of the `AGENTS.md` block, hint `errAnstossEndeUnbestimmt`; `legacyInstructionsBlocks` names `k-playbook context` only as a superseded call to recognise | switched in 096 |
| `installer/internal/project/local.go:206` | README text of `version-sources.yaml` that `EnsureLocal` writes into the project | switched in 096 (names the briefing) |
| `installer/internal/project/local.go:441`, `languages.go:52/112`, `links.go:72/274/644`, `webui/inventory.go:36/139`, `webui/mcp_servers.go:64`, `versionsources/versionsources.go:5/209` | code comments about behaviour of the context output | stays on purpose |
| `installer/internal/webui/assistant.go` | card text after setup ("auf den aktuellen Aufruf gebracht") | switched in 096 (names `k-playbook briefing`) |
| `installer/internal/webui/static/app.js` | assistant card: state of `AGENTS.md` | switched in 096 (shows outdated prompt, obsolete blocks, hint) |
| `installer/internal/webui/static/setup.html:201`, `mcp.html:79`, `mcp-servers.html:61` | describe the context card, the MCP server (`k_playbook_context`) and the field `mcp` of the context output | stays on purpose |
| `installer/internal/project/briefing.go`, `installer/cmd/k-playbook/briefing.go` (`briefingContextSeparator`, usage), `installer/cmd/k-playbook/main.go` (usage), `installer/cmd/k-playbook/doc.go:191` (usage) | the rest section of the briefing and the help texts naming it | stays on purpose |
| `installer/internal/project/context.go`, `installer/internal/mcpserver/server.go` (`k_playbook_context`), `mcpserver/briefing.go`, `mcpserver/docs.go` | the call itself and its facades | stays on purpose (go when the rest is empty, todo #26) |
| `installer/docs/architecture.md` "Instruktionen" (prompt, superseded versions, obsolete blocks, the three entries) | architecture of the `AGENTS.md` handling | switched in 096 |
| `installer/docs/architecture.md` other hits (`:144`, `:342`, `:1100`, `:1342`, `:1359`, `:1846`, `:2101`, `:2657`, `:3246`, `:3338-3345`, `:3726`, `:3732`) | describe the context output, the MCP tool and measurements | stays on purpose |
| `README.md` ("What applies", "An existing CLAUDE.md", "Updating") | entry of a project: briefing instead of context | switched in 096 |
| `installer/README.md` | call list and the paragraph on `context` | switched in 096 (briefing and upgrade added, context marked as being taken apart) |
| `docs/k-playbook-format.md` (the `AGENTS.md` block) | the prompt | switched in 096 |
| `docs/k-playbook-format.md` (`:210`, `:249` "The resolved working state", `:296-309`, `:521-524`) | the context output and the rest section | stays until the switch of the context output |
| `docs/installation.md:166/191/311`, `docs/faq.md:103/115`, `docs/manual.md:119`, `docs/commands.md:21` | the prompt and "the call at the start of every session" | switched in 096 |
| `docs/installation.md:265/408/527/601`, `docs/faq.md:85`, `docs/manual.md:88`, `docs/commands.md:105`, `docs/overview.md`, `docs/version-inventory.md`, `docs/verweisschema.md`, `docs/kephalaion-client.md`, `docs/mcp.md` | describe fields of the context output | stays until the switch of the context output |
| `commands/*.md` (21 files), `skills/*/SKILL.md` and `PLAYBOOK.md` (5), `commands/_shared/context.md` | the old entry module and the commands' standard header | stays until its reader moves (work list 16) |
| `rules/command-authoring.md:101` (= `defaults/rules/`), `guidelines/betrieb.md:15` (= `defaults/guidelines/`) | "sonst rufe `k-playbook context` auf", "Pfade aus `k-playbook context` ableiten" | stays until its reader moves (work list 15; `betrieb.md` when `playbook`/`local` leave the rest) |
| `k-playbook.md` (= `defaults/k-playbook.md`) `:63`, `:125` | "Wo was liegt", "Umgang mit Befunden" | stays until its reader moves (rows of the target table above) |
| `defaults/instructions/always/010-verweise.md:14` | names the `kephalaion.*` fields in the rest | stays until its reader moves (row `kephalaion` above) |
| `defaults/instructions/always/030-briefing.md:33`, `defaults/instructions/on-demand/briefing-felder.md:22` | name the section `context` as the rest | stays on purpose |
| `checks/README.md:70/101` | checks do not call the context | stays on purpose |
| `.claude/settings.local.json` | permission pattern | stays on purpose (local settings) |
| `docs/knowledge-aktuell.md` | this file | stays on purpose |

## Open

**The workbench** (2026-10-14, defined 2026-10-19). Meant for the current state of work: where
one stands, what is open, what comes next. Unlike a task it is no assignment, and unlike a
finding it carries no proof. Built 2026-10-14/15 (task 088): one file per topic, numbered
project-wide, a finished topic moves to `done/`. Defined 2026-10-19 in the always part
`defaults/instructions/always/210-workbench.md` (not yet released, not yet measured in either
client): a topic is created as soon as a plan with open points outlives the session, updated
whenever a point changes, closed with `doc_done` when the work is finished; the file is
overwritten as a whole with `base_revision`, never appended; frontmatter `title` is required
and nothing else; the first paragraph states where the work stands. The findings rule and the
always part 200 now exclude the state of work and point to the workbench; `betrieb.md` has the
row. First measured 2026-10-19 from Claude Code: create, write, read via `~`, briefing title,
done and reopen all work; `keph:/user/kamran/workbench/001-workbench-einfuehren.md` holds the
protocol. Open: `/k-danke` does not ask about active topics yet; whether a finished topic is
also resolved (confirmed parts as findings, open parts as tasks); the OpenCode measurement.

**How commands and skills are read once the local files are gone.** Claude Code and OpenCode
find commands and skills only as files on disk, and checks and scripts must run. Decided
2026-10-25 for commands and skills: embedded in the binary, unpacked into
`~/.local/share/k-playbook/` (see "Files that must lie on disk are installed once per
machine"). Still open: checks, scripts and `bin/` — whether they go the same way. The earlier
idea, to `pull` `<hub>:k-playbook/defaults/` into a cache, would have lost the executable bit
(`pull` creates every file with mode `0644`) and symlinks (`push` skips them).

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
   **Task 091 (2026-10-17, in the working tree):** the briefing carries the rest of the
   context output in its section `context` (plain text behind the separator "Noch aus
   k-playbook context — wird abgebaut"), from `ContextForDir`, minus `catalogs.rules`,
   `catalogs.reviews` and `guidelines` (`briefingCoveredContextFields`, one place in the
   code); a failed context is a state (`context.available`), not an error; the briefing heals
   the assistant links as a side effect (`context.links`); briefing schema `"2"`. One call is
   enough for the start of a session; `k-playbook context`, `k_playbook_context`,
   `k-playbook.md`, the `AGENTS.md` block and the commands are unchanged. The table "Context
   field → future place" assigns every context field, and "Where `k-playbook context` is
   anchored" names what the switch has to rebuild — including the current `anstoss` block as
   the second superseded version beside `legacyInstructionsBlockLines`. Next steps: take the
   fields out of the rest one by one along the table; the switch itself is the last step.
   **2026-10-18:** the briefing carries the text of the always-instructions itself
   (`documents.always.content`, from `joinDocDirParts`, the one function behind `k-playbook read`
   for the directory; plain text starts with it, without the list of parts; schema `"3"`). The
   contract's "no contents" now has this one named exception
   ([mcp.md](mcp.md#briefing-contract)); a session that calls the briefing needs no second call
   for the always-instructions.
10. **User area and findings queue** — built 2026-10-15 (task 088, in the working tree):
    `k_playbook_doc_create`, `_list`, `_done`, `_reopen` and `k-playbook doc`, measured at hub
    `vm`; see "Creating a named document". Not yet put to use: `/k-task-create`, `/k-task-run`,
    `/k-danke` and the findings rule still write to the disk, and `k-playbook.md`, `AGENTS.md`
    and the context output do not mention the tools. Calling the MCP tools from an open session
    is a follow-up test after the next release. Still open: the rules for what goes where, the
    scheduled run for `queue/findings/`, and `queue_list` for subdirectories. Next (2026-10-24):
    the findings rule, the skill `befunde` and `/k-danke` write into `queue/findings/`; reading
    the queue comes afterwards (see "Writing the findings comes first, reading them later").
11. **Generators** — `/k-docs-*` write single documents into the project collection,
    including the removal of stale ones. The commands are outdated and get rebuilt here; the
    six steps of `docs/knowledge-layout.md` ("Migration") are not carried out on them.
12. **Delete the local files** — the old directories in the repo and the clone in the
    projects; setup, update and interface without `<project>/k-playbook/`.

Added 2026-10-18 (task 092); these come before step 12, in this order:

13. **First command as a hull** — built 2026-10-18 (task 092, in the working tree):
    `commands` is a kind of `k_playbook_doc_read`, read from `defaults/commands/`;
    `/k-upgrade` is hull plus text; `defaults/includes/` exists with a README. Open: the
    check in Claude Code and OpenCode after the next release (step 4 of the task).
14. **All prefixes and includes** — built 2026-10-18 (task 093, in the working tree):
    `ResolveDocRef` resolves `project:` and `keph:` (the client gained `Read`), include
    lines are inserted in every document with `--raw`/`includes: false` to switch off,
    `includes` and `testdata` are kinds, the fixed cases lie in `defaults/testdata/includes/`.
    Open for the follow-up tasks: extend `010-verweise.md` by `k_playbook_doc_read` for `keph:`
    and decide the default route; `keph:` includes in always-instructions only with care
    (network). The first include came with task 094 (2026-10-18): `defaults/commands/k-upgrade.md`
    binds `_shared/erster-schritt.md`; see "What every session needs comes with the briefing" above.
    Measured the same day (findings `command-huelle-verweis.md`): an `AGENTS.md` block that names
    the shell command `k-playbook briefing` first is followed by Claude Code (2 of 2), the
    MCP-first wording is not (0 of 2); OpenCode calls the MCP tool first either way. Sub-agents
    never call the briefing on their own (3 of 3 in Claude, with the block in their context), so
    `030-briefing.md` tells whoever delegates to put the call at the top of the sub-agent's brief;
    the include is named `_shared/erster-schritt.md` because it is the first step of every command.
15. **Hulls generated by the binary** — from `defaults/commands/` instead of files under
    `commands/` and symlinks; `/k-upgrade`'s hand-made hull goes with it, and the hull form
    becomes a rule in `rules/command-authoring.md`.
16. **Move the commands, then the skills** — text by text into `defaults/commands/` and
    `defaults/skills/`; `$ARGUMENTS` and frontmatter stay in the hull. `/k-upgrade` has its
    subcommand since task 096 (2026-10-19) and only relays its message.
17. **`AGENTS.md` template on the briefing, `k-playbook upgrade`** — built 2026-10-19
    (task 096, in the working tree): `rootInstructionsTemplate()` is heading plus the
    briefing prompt, byte-identical to this repo's `AGENTS.md` (test); the `k-playbook
    context` block is the second superseded version (`legacyInstructionsBlocks`), the
    session-memory and findings blocks are recognised by their markers and removed
    (`obsoleteRootBlocks`, positive boundary or hint) at setup, at the GUI start and with
    `k-playbook upgrade`; `k-playbook upgrade [--check] [--json]` pulls, installs only when
    the running program is older, re-executes the new program for the project step, and
    `/k-upgrade` relays its message. The inventory of `k-playbook context` is the table
    above. Released as v0.16.9 (without the three fixes from the review) and v0.16.10 (with
    them). Measured 2026-10-19 after v0.16.10 with a real binary change (v0.16.9 installed,
    clone at v0.16.10): `/k-upgrade` in Claude Code (headless `-p`) and in OpenCode
    (`opencode run`) both run `k-playbook upgrade`, `bin/install` replaces the binary from
    inside both clients, the message and the restart hint come through unchanged
    (`k-playbook-local/material/befunde/command-huelle-verweis.md`, 2026-10-19). Not
    measured: the "download/install not possible" path.
18. **Todos into the collection** — decided 2026-10-19, built 2026-10-20 (task 097, not yet
    released): `todos/` with `done/`, one document per entry; client `Write`/`Delete`, todo
    tools with `search`, GUI endpoints, creation, ticking off and search in the GUI,
    `k-playbook todo migrate` (run once in this repo, 28 entries). Open: `commands/k-todo.md`,
    `docs/knowledge-layout.md`, upgrade path, MCP follow-up test after the release; see
    "Todos become documents in the collection" above.

19. **Cursor leftovers** — added 2026-10-19 after task 095, three points: `/k-upgrade`
    cleans up `.cursor/` in existing projects; `/k-upgrade` brings the stub wording of
    `CLAUDE.md` in existing projects up to date, but only when the file is word for word the
    old stub; the temporary guard line `/.cursor/` in this repo's `.gitignore` goes as soon as
    the binary without the Cursor target is installed — until then the installed binary
    recreates `.cursor/commands` on every `k-playbook context`.

20. **Commands and skills from the binary into `~/.local/share/k-playbook/`** — decided
    2026-10-25, three stages, each tested before the next:
    1. Pack commands and skills at build time, embed the archive in the binary, unpack it into
       `~/.local/share/k-playbook/` on install and update. Copies only, no links; nothing in the
       projects changes yet. **Built 2026-10-25 (in the working tree):** `share/` holds copies of
       `commands/` and `skills/` and is its own Go module, embedded through a `replace` in
       `installer/go.mod`; no version directories (decided the same day: commands and skills
       differ little between versions). Every command except `version` and `help` compares a
       content checksum and unpacks on a difference; files `0444`. `make share-sync` and a test
       keep the copies equal; the VERSION guard watches `share/**`. Described in
       [`architecture.md`](../installer/docs/architecture.md#mitgelieferte-dateien-unter-localsharek-playbook).
       Measured with an isolated `HOME`; not yet installed on a real machine.
    2. Link from there into the assistants. Planned globally (`~/.claude/…`); dropped the same
       day after the measurement below, so `legacy.RemoveGlobalLinks` stays as it is.
       Projects do override shipped entries (2026-10-25: `squad-km-dev-setup` and `testomni2`
       replace `k-docs*.md` and `_docs/`), and overriding by the same name stays as it is: the
       link function resolves local before shipped (`ResolveRegistry`), only the shipped source
       moves from the clone to `~/.local/share/k-playbook/`. **Measured 2026-10-25:** with the
       same name globally and in the project, Claude Code 2.1.310 takes the global entry,
       OpenCode 1.18.35 the project's — for commands and skills alike. Global links therefore
       cannot be overridden per project in Claude Code; the shipped entries stay linked per
       project. **Built 2026-10-25 (in the working tree):** every link points relatively through
       one link `.k-playbook/share` → `~/.local/share/k-playbook/`, the only absolute one, kept
       out of git by `.k-playbook/.gitignore`; the registry reads the shipped side at its real
       place, links from the clone are repointed or removed by the existing check. Tried on a
       copy of this repo: 33 entries repointed, the local `_audit/` override and `bachforelle`
       stay in `k-playbook-local/`; Claude Code activates `regenbogenforelle` and
       `bachforelle`, OpenCode lists the commands and all skills through the link. Not yet
       installed. Described in
       [`architecture.md`](../installer/docs/architecture.md#der-umweg-link-k-playbookshare).
    3. Remove the directories no longer used and go through docs, links and the interface.

21. **Project-own commands and skills into `<project>/.k-playbook/`** — wanted 2026-10-25,
    after item 20: `.k-playbook/` holds only what is new, and `k-playbook-local/` goes once its
    content has moved (to Kephalaion or here). Same names override as before. Not yet planned.

## Findings

Measured state and evidence: `k-playbook-local/material/befunde/kephalaion-anbindung.md`.
