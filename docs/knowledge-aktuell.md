---
title: Knowledge — Current Direction
description: Working document for moving k-playbook onto Kephalaion — the goal of a binary-only installation whose shipped content lives in a public collection k-playbook on every hub, filled by the hub from the release, next to each project's own collection; the decisions taken so far, the open questions and the ordered work list.
---

# Knowledge — Current Direction

Working document for the duration of the switch, in the manner of
[`migration.md`](./migration.md): it records what has been discussed and decided, the open
questions and the work still to do. What has been implemented moves into the regular
documentation and leaves this file; the file is deleted when nothing remains open.

As of: 2026-10-09.

## Goal

k-playbook is installed as a binary only. There is no clone under `<project>/k-playbook/`
any more. What is shipped today in that clone — commands, skills, rules, reviews, checks,
scripts, guidelines, docs — lives in a public collection `k-playbook` on every Kephalaion
hub, one directory per release. What is the project's own lives in the project's collection.

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

**The shipped content lives in a public collection on every hub** (2026-10-09, variant D).
Every hub has a collection `k-playbook`; every account of that hub may read it; only the hub
writes it, from the k-playbook release. It holds one directory per release (`v0.11.0/`, …),
because projects run different binaries and each reads the content of its own version; an
existing directory means there is nothing to fetch. k-playbook needs no further address: the
public collection lies on the same hub as the project's, `<hub>:k-playbook`. Kephalaion stays
generic: it gets a collection with an external source, readable by all accounts of the hub,
written only by the hub — nothing specific to k-playbook. Kephalaion and k-playbook should
stay separate; this dependency is a deliberate exception.

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

**One directory per version, fetched on request** (2026-10-09). A release directory is never
changed after it is written. An update of k-playbook is always an update of the binary: the
new binary looks for `<hub>:k-playbook/<version>/` on its first start and, if it is missing,
asks the hub to fetch exactly that version; the source is fixed in the collection's
configuration, so any account of the hub may ask. The hub admin can also fetch by hand.
Polling and a GitHub webhook were dropped (they fetch unused versions; a webhook needs a public
endpoint per hub). If the collection `k-playbook` does not exist on a hub, nothing happens;
if it exists, it is read-only for everyone except the hub itself. The Kephalaion side is task
031 in the Kephalaion repo (`k-playbook-local/tasks/031-collection-aus-release-fuellen.md`).

**The binary stays on GitHub** (2026-10-09). Kephalaion stores text up to 1 MiB per document,
every node syncs the whole collection, and a hub that hands out executables would become a
supply-chain point for every connected machine. The install script fetches the release binary
and checks it against `SHA256SUMS`, as `bin/install` does today.

**The source repo gets a directory `vendor/`** (2026-10-09). It marks exactly what is
shipped; its content becomes one release directory of the public collection. The repo is
public and stays public, and the binary is fetched from there already.

**`vendor/` holds links during the transition** (2026-10-09). `vendor/` contains relative
symlinks to the shipped places (`vendor/commands -> ../commands`, likewise `bin`, `checks`,
`docs`, `guidelines`, `reviews`, `rules`, `scripts`, `skills`, `k-playbook.md`, `VERSION`). The
real files stay where they are, so the installation, the Go code and the assistants' file
search see no change. Whoever packs the content dereferences the links (`cp -rL`;
`__pycache__` excluded): 107 files, no link left. In step 8 the direction flips —
the files move into `vendor/` and the old places go. The opposite direction (files in
`vendor/`, links at the old places) was dropped: `filepath.WalkDir` does not descend into a
symlinked root (`installer/internal/project/docs.go:40` walks `k-playbook/docs`), and a file
search that does not follow links (ripgrep, which backs the Glob tool) would find nothing under
`k-playbook/rules/` and the like.

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

The shipped content is not part of it; it lies in `<hub>:k-playbook/<version>/`.

## Open

**What the hub fetches.** The tarball GitHub generates for a tag carries no checksum of its
own. A release asset packed from `vendor/` with its sum in `SHA256SUMS` would let the hub
verify what it writes.

**How commands and skills are read once the local files are gone.** Claude Code, OpenCode and
Cursor find commands and skills only as files on disk, and checks and scripts must run.
Without the clone the binary would probably `pull` `<hub>:k-playbook/<version>/` into a cache outside
the project and link from there. `pull` creates every file with mode `0644` (16 executable
files lose their bit) and `push` skips symlinks. Whether this carries in all three assistants
has to be measured before the local files go.

## Work list

The order as of 2026-10-09, after the decision for variant D. The old `k-playbook/` stays
until the last step.

1. **Collection binding** — built 2026-10-09 (task 085, in the working tree); see "Collection
   binding" above. Task 082 is adapted to use the same check.
2. **Rename the development collection** — `k-playbook` → `k-playbook-dev` at the hub (there
   is no rename: add, grant accounts and nodes, then revoke and remove the old one — it is
   empty), at the node and in `K-PLAYBOOK.local.yaml`.
3. **`vendor/` in the source repo** — links in the working tree since 2026-10-09; to be
   committed.
4. **Decide what the hub fetches** — tarball of the tag or a release asset with checksum;
   see "Open".
5. **Release packs the content** — if decided: an asset built from `vendor/` with its sum in
   `SHA256SUMS`.
6. **Kephalaion: a collection with an external source** — readable by all accounts of the
   hub, written only by the hub, one directory per version, fetched on request or by hand.
   Kephalaion task 031, to be refined there.
7. **AGENTS.md reads the public collection** — the instructions point at
   `<hub>:k-playbook/<version>/` over MCP instead of the local `k-playbook/`.
8. **Project files into the project collection** — decide how the files of
   `k-playbook-local/` that correspond to the shipped ones (rules, commands, skills, checks,
   reviews, `k-playbook.md`) get there; adapt AGENTS.md.
9. **Briefing tool** — a new tool that hands an assistant its start, built only from the
   collections; ensure nothing local is used.
10. **Generators** — `/k-docs-*` write single documents into the project collection,
    including the removal of stale ones.
11. **Delete the local files** — the old directories in the repo and the clone in the
    projects; setup, update and interface without `<project>/k-playbook/`.

## Findings

Measured state and evidence: `k-playbook-local/material/befunde/kephalaion-anbindung.md`.
