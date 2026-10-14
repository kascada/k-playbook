---
title: Knowledge — Current Direction
description: Working document for moving k-playbook onto Kephalaion — the goal of a binary-only installation whose shipped content lives in a public collection k-playbook on every hub, filled by the hub from the release, next to each project's own collection; the decisions taken so far, the open questions and the ordered work list.
---

# Knowledge — Current Direction

Working document for the duration of the switch, in the manner of
[`migration.md`](./migration.md): it records what has been discussed and decided, the open
questions and the work still to do. What has been implemented moves into the regular
documentation and leaves this file; the file is deleted when nothing remains open.

As of: 2026-10-14.

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
2026-10-14 and is the exception: it is maintained continuously and kept equal to
`instructions/` by a test (see "A new kind `instructions/`" below). Who delivers the Markdown — k-playbook
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
the briefing (task 086). **"Frozen" does not apply here:** `instructions/` lies at the top of the
repo for the installation and as a real copy in `defaults/instructions/` for the hub, and it is
maintained continuously; a Go test in the normal test run
(`installer/internal/project/instructions_tree_test.go`) keeps both trees equal, and
`.github/scripts/pack-defaults.sh` refuses to pack when they differ.

**`k_playbook_doc_read` and `k-playbook read`** (2026-10-14, task 087). The MCP tool and the
subcommand resolve `k-playbook:/instructions/…` through an exchangeable source — the disk today
(`installer/internal/project/docref.go`). A single file comes unchanged with frontmatter; an
empty local file disables it without an error; a directory is assembled without frontmatter;
an unknown reference is `not_found` with no fallback to the disk; `project:`, `keph:` and other
`k-playbook:` kinds are `unsupported` for now. The `ref` in the prefix scheme is the lasting
contract of the tool; see [`mcp.md`](./mcp.md#doc-read-contract).

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
```

The shipped content is not part of it; it lies in `<hub>:k-playbook/defaults/`.

## Open

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
9. **Briefing** — next step, task 086 (2026-10-10): the k-playbook MCP server lists the
   documents that apply, shipped and local already resolved against each other, and reads
   them by key; the source behind it (disk today, Kephalaion later) is invisible to the AI.
   Context output and `AGENTS.md` point at it. The Markdown files are rebuilt and integrated
   step by step afterwards; the docs are taken over later, anew.
   Groundwork built 2026-10-14 (task 087, in the working tree): the reference scheme, the kind
   `instructions/` and `k_playbook_doc_read`/`k-playbook read` for `k-playbook:/instructions/…`,
   not yet put to use. Still open for 086: listing keys as `k-playbook:/…`, the other kinds
   (`rules`, `reviews`, `guidelines`), resolving the name clash `instructions` (context field,
   `k-playbook.md`, directory `instructions/`), and putting it into `AGENTS.md`. The check
   against the real installation follows the next release; the installed `k-playbook/` here is
   older and has no `instructions/`.
10. **Generators** — `/k-docs-*` write single documents into the project collection,
    including the removal of stale ones.
11. **Delete the local files** — the old directories in the repo and the clone in the
    projects; setup, update and interface without `<project>/k-playbook/`.

## Findings

Measured state and evidence: `k-playbook-local/material/befunde/kephalaion-anbindung.md`.
