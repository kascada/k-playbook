---
title: Knowledge Layout
description: The three storage zones under k-playbook-local — inbox for what arrives, queue for what is outstanding, knowledge for what holds — with the ownership rule, the frontmatter contract, the table of what lands where, the write tools every deposit goes through, and how the migration happens: as the switch of the writers, step by step, with the table of the reads that last until then.
---

# Knowledge Layout

[`knowledge-gate.md`](knowledge-gate.md) describes how a deposit is asked for. This page
describes where it lies. It defines the storage zones from scratch, deliberately beside
the directories in use today rather than on top of them: the new names are free, so the
migration is a move and never a rename during operation.

**Status: built.** The three zones, the write tools and the index over `knowledge/` exist
since task 056 (the zones are created by setup, the tools are `k-playbook knowledge …` and
`k_playbook_knowledge_*`, see [`mcp.md`](mcp.md), "Knowledge Contract"). The migration is
under way, and it is not a move of its own: it is the switch of the writers to the gate, one
step per writer (see "Migration"). Until the last step `docs/` holds what the writers not yet
switched produce and stays the entry point, and `knowledge/` fills with every step. The reading
side (`briefing`, filters on `state`) is a separate step.

## Why zones, and why by lifetime

Everything a project knows currently lands in one pot called `docs/`, and that pot mixes
three things with nothing in common: raw material nobody has read yet, half-finished work,
and knowledge that holds. They differ in what may be done to them. Raw material must never
answer a question. Outstanding work must disappear when it is done. Knowledge must never
disappear at all.

Sorting by topic cannot express any of that, because a topic says nothing about how long
something lives or who may overwrite it. Sorting by lifetime does, and it yields exactly
three zones.

| Zone | Holds | Lifetime | Indexed |
|---|---|---|---|
| `inbox/` | what arrives | forever, until a person removes it | no |
| `queue/` | what is outstanding | until it has been processed | no |
| `knowledge/` | what holds | forever | yes |

## `inbox/` — what arrives

Chat transcripts, notes, PDFs, screenshots, HTML dumps, exports, anything a person drops in.
Subdivided by source alone: `inbox/<source>/…`, where `<source>` is free — `confluence`,
`chat`, `mail`, `scan`.

Any format. No naming convention, no frontmatter, no obligation of any kind beyond dropping
the file. That is not laziness, it is the condition for anything being deposited at all: a
drop zone that demands preparation gets used once.

The inbox is an **archive, not a queue**. Something may lie here for months without anybody
looking at it, and nothing here is outstanding merely by being here. It is never indexed and
never appears as a search hit, so it cannot drown the distillate made from it.

**Processing does not consume it.** A raw piece stays after it has been worked up, which is
what makes an extract auditable and repeatable: a poor extract can be drawn again from the
same source. Deleting from the inbox is a deliberate human act and never a side effect of a
processing run.

This is also where non-Markdown originals stay for good. The store holds Markdown; an image,
a PDF, a spreadsheet stays in the inbox and the knowledge document points at it. That saves
the store an attachment mechanism it would otherwise need, and it keeps the promise that
every file under `knowledge/` is full-text indexable.

## `queue/` — what is outstanding

One entry is one piece of work: *this raw piece should become knowledge*. An entry is a small
Markdown file that **references** its source in the inbox rather than copying it, and names
the target directory and the reason.

After a successful takeover into the store the entry is **deleted** — not moved, not archived,
not ticked off. An empty `queue/` therefore means: nothing outstanding. That is the entire
point of the zone. A backlog that can only be established by counting or filtering is one
nobody reads.

A run that fails leaves its entry lying, extended by a line saying what went wrong. The zone
holds work, not history; the history of what was made from a source is in the finished
document's `origin`.

## `knowledge/` — what holds

The truth. Only what lies here is indexed, searched, read and cited in an answer.

**Markdown with YAML frontmatter, always.** What arrives in another format is converted at
the entrance, or stays in the inbox with a Markdown stub here describing it. Keeping the
conversion at the entrance is what keeps the fast part fast: the index never learns a second
format.

**One document per topic, appended.** Not one per event — otherwise the store grows with
time instead of with knowledge, and the fifth note on the same subject competes with the
four before it.

**Exactly one owner per directory, and only the owner writes there.** A generator rewrites
its directory completely on the next run; anything a different producer left there is gone
without a trace. The rule is therefore not tidiness but the condition under which a generator
run is safe at all.

**Nothing is deleted.** What has been replaced gets `state: superseded`, drops out of search
and stays readable. A store that deletes cannot say what it used to claim.

### What lands where

The path carries the **owner**, the frontmatter carries the **subject**. That split is what
lets a generator wipe its own directory while the index and the search still sort by what a
document is about rather than by who produced it.

| Path | Owner | What lands here |
|---|---|---|
| `knowledge/code/` | `/k-docs-code` | derived from this project's source |
| `knowledge/libs/` | `/k-docs-tools` | references for the tools, libraries and stacks in use |
| `knowledge/versions/` | `/k-doc-inventory` | the version inventory |
| `knowledge/extracted/` | `/k-docs-extract` | distilled from `inbox/` material |
| `knowledge/external/<system>/` | one connector per system | extracts of outside sources |
| `knowledge/findings/` | the session, through the gate | what an analysis or a hunt established, including what was ruled out |
| `knowledge/pitfalls/` | a person, on explicit confirmation | what has to be known *before* acting |
| `knowledge/manual/` | a person | everything written by hand that has no generator |
| `knowledge/README.md` | `/k-docs-index` | the index over all of it |

The first three are wiped and rewritten per run. The rest is appended to, and a run that
touches them writes single documents, never the directory.

`findings/` and `pitfalls/` are the reason the store is more than documentation. What a
session established about the code, and what one has to know before touching something, are
knowledge in exactly the sense that matters here — they answer questions, and today they
answer them nowhere a search can reach.

### The frontmatter

| Field | Meaning |
|---|---|
| `title` | the document's own wording |
| `subject` | what it is about — the axis the index and the search sort by |
| `origin` | actual provenance: the system, the identifier there, the address, when it was fetched |
| `state` | `raw`, `condensed`, `reviewed` or `superseded` |
| `format` | what the original was: `markdown`, `text`, `html`, `image`, `pdf` |
| `sources` | the raw pieces a document was distilled from, where there are any: paths relative to `k-playbook-local/` with their zone, `inbox/<source>/<name>` or, while it is still read (see "Transitional reads"), `material/<path>`. The tools name inbox paths without the zone — `inbox_list` and a queue entry's `origin` say `chat/2026-09-12.md` —, so a caller prepends `inbox/` before it writes `sources` or compares with them |
| `successor` | set by `supersede`: the path of the document that replaces this one, relative to `knowledge/` |
| `superseded_reason` | set by `supersede`: why it was replaced |
| `updated` | the day it last changed |

`state` has teeth or it is decoration: `raw` and `superseded` stay out of search results by
default and remain readable on request. `write` accepts `raw`, `condensed` and `reviewed`;
`superseded` only ever comes from `supersede`. `format` is `markdown` unless given.

**There is no `kind` field.** The kind is the directory, and a field that repeats the path is
a field that can contradict it. It is read from the path and never from the document.

## The path through

```
inbox/<source>/<file>           stays, forever
        │  someone decides it is worth it
        ▼
queue/<entry>.md                references the raw piece — deleted after the takeover
        │  processing
        ▼
knowledge/<owner>/<topic>.md    holds, with origin pointing back at the inbox
```

A producer that already writes structured output — every generator — starts at zone three
and never touches the first two.

## What stays outside

`results/` (review runs and their artifacts), `tasks/`, `rules/`, `data/todos.json` and
`priv/`. That is control and record, not knowledge: it steers a run or documents that one
happened. None of it answers a question about the project, and none of it moves.

## Access: everything through the gate

**No producer writes into a zone directly — not a person, not an assistant, not a generator.**
Every deposit is a tool call, and the tool is the only thing that knows the rules: which
directory an owner may touch, which frontmatter fields are mandatory, that a queue entry
disappears exactly when its document exists.

The reason is not ceremony. A contract that only lives in a document is a contract every
producer re-implements, each slightly differently, and the differences surface as a corrupt
index months later. A contract that lives in one tool is implemented once. This is also why
the write tools take the frontmatter as **arguments** and compose it themselves rather than
accepting a finished file: a field the caller may format is a field the caller may get wrong.

Three consequences, and they hold from the first line of code:

- **An assistant never edits a file under `knowledge/` with an editor or a shell**, even when
  it would be one line. It calls the tool.
- **A generator does not write its directory file by file.** It hands over the complete set
  and the tool performs the exchange, so a wipe is never half done.
- **Deleting does not exist for `knowledge/`.** What is out of date is superseded, which is a
  tool call of its own.

This cannot be enforced against a producer that has a filesystem, and pretending otherwise
would be the wrong design. What can be done is what the index already does: it carries a hash
per file and notices when the tree disagrees with it. A write past the gate is therefore
detectable; it is repaired silently, and `status` says that it happened (`stale`,
`staleFiles`) without naming the file. `stale` can also mean an interrupted or a concurrently
running `publish` rather than a write past the gate: `publish` writes its index before its swap
(see "The write tools"), so in that moment, or after a crash in it, the index describes a set
the disk does not hold, and the next access returns to what is on disk. A drift report that names the file and stays until
somebody acknowledges it was considered and deliberately not built — see "Decisions".

## The write tools

One core in Go, a CLI subcommand and a thin MCP wrapper over the same logic — the shape the
knowledge tools already have. A generator on the command line and a session over MCP therefore
cannot drift apart, because there is only one implementation to drift from.

### Who may write where

Every write names its **producer**, from a closed list, and the tool checks it against the
target directory. A mismatch is refused.

| Producer | May write |
|---|---|
| `docs-code` | `knowledge/code/` |
| `docs-tools` | `knowledge/libs/` |
| `inventory` | `knowledge/versions/` |
| `docs-extract` | `knowledge/extracted/` |
| `connector:<system>` | `knowledge/external/<system>/` |
| `session` | `knowledge/findings/` |
| `person` | `knowledge/manual/`, `knowledge/pitfalls/` |
| `docs-index` | `knowledge/README.md` |

This is a declaration, not a proof: a producer that lies is not caught. What it buys is that a
wrong target becomes an error at the call instead of a silent overwrite discovered a month
later, and that the mapping exists in exactly one place rather than in eight callers.

### The tools

| Tool | Arguments | Result |
|---|---|---|
| `inbox_put` | `source`, `name`, `content` or `file`, `note` | the inbox path |
| `inbox_list` | `source` | what lies there, with format, size, date and note |
| `inbox_read` | `path` | the raw piece, for text formats |
| `queue_add` | `origin` (an inbox path or an outside address), `target`, `reason` | the entry id |
| `queue_list` | — | the backlog |
| `queue_drop` | `id`, `reason` | — |
| `write` | `producer`, `path`, `title`, `subject`, `origin`, `state`, `format`, `sources`, `body`, `queue` | the written path |
| `publish` | `producer`, `documents` | how many written, how many removed |
| `supersede` | `path`, `successor`, `reason` | the superseded path and the successor, both cleaned |

Over MCP the names are `k_playbook_knowledge_<tool>`; on the command line `inbox` and `queue`
are groups (`k-playbook knowledge inbox put`, `… queue add`) and the rest are subcommands
directly below `knowledge`. `publish` takes `--from <dir>` on the command line: every
Markdown file below the directory carries the fields of `write` in its frontmatter, the
tool checks them and recomposes the header; over MCP it takes the `documents` array. The
exact arguments and results are in [`mcp.md`](mcp.md), "Knowledge Contract".

`write` takes the frontmatter as **fields and the body as Markdown without a header**. The
server composes the frontmatter and sets `updated` itself. A caller that could hand over a
finished file could hand over a malformed one, and the index would be the place where that
turns up.

`write` optionally names a **queue entry**, which is deleted when the document exists — not
before and not in a second call. That is the whole reason the queue can be trusted as a
backlog: there is no window in which the work is both done and outstanding.

`write` does **not overwrite a superseded document**. A target whose file carries
`state: superseded` is refused before anything is written. Otherwise a later run of the same
producer would set the state back silently and lose `successor`, and the supersession would be
gone without anyone noticing. Whoever wants to change the topic writes the successor, or
supersedes it.

`write` refuses a **path through a symbolically linked directory** under `knowledge/` before it
reads, creates a directory or writes anything: the index does not descend into a linked
directory, so the file would lie where no search reaches it. `read` and `supersede` refuse such a
path the same way; a linked *file* is indexed and stays allowed. `publish` is not affected: its
swap renames the link itself aside and puts a real directory in its place, so the published
documents land where the index sees them, and the link's target is left untouched.

`publish` is how a generator writes, and the only way it does. It hands over its **complete**
set of documents; the tool writes them and removes what is not in the set. A run that dies
halfway changes nothing at all, whereas a `clear` followed by writes would leave the store
empty for as long as the run takes.

The paths of a set are **relative to the generator's directory**: `overview.md`, not
`code/overview.md`. A path that already starts with the directory is refused, not cut: it
points at a wrongly built generator, and cutting it silently would hide that. The consequence
is that directly below a generator directory there is no subdirectory of the same name
(`code/code/`). The comparison ignores case on every platform — `Code/overview.md` is refused
as well —, because on a file system that does not distinguish case (APFS on macOS)
`code/Code/` is that same forbidden directory.

That a run which dies halfway changes nothing rests on the order of the exchange and on one
rule: **the files on disk are the truth, and drift detection may only ever lead back to the
previous state.** The new set is written into a hidden directory beside the old one
(`.<dir>-neu-*`); the index that describes it is computed from there and written; only then
are the directories swapped — the old one set aside as `.<dir>-alt-*`, the new one put in its
place, the old one removed.

- A failure before the swap — an invalid document, a full disk, an unwritable index — removes
  the hidden directory and leaves the old directory and the old index.
- A failed swap puts the old directory back and writes the old index again. If writing the
  index fails too, drift detection restores it from the disk on the next access.
- A run that dies between writing the index and the swap leaves the old directory under the
  new index. The next access returns to the old state and reports `stale: true`, although
  nobody wrote past the gate; that is accepted and not suppressed.
- A run that dies between the two renames, or whose swap and restore both fail, leaves the
  target missing and the old state hidden as `.<dir>-alt-*`; the error names both directories,
  and nothing is removed. The next access — `read` included, although it does not open the
  index — puts the orphaned `.<dir>-alt-*` back under its name before drift detection runs, and
  says so in a note: `hint` over MCP, stderr on the command line.

There is no lock, so that restore never replaces an existing target — not even an empty one,
which a concurrent `publish` may just have put in place. If the restore fails or more than one
candidate lies there, nothing is restored, a note names the directories, and the access itself
does not fail. If the restore meets a concurrent `publish` between its renames, that `publish`
reports an error, the previous state stands, and its hidden `-neu-*` stays behind. Hidden
leftovers are never deleted automatically; `status` names them. If only removing the old
directory after a successful swap fails, the run counts as successful and says so in a note.

`inbox_put` exists because a connector and a session need a way to deposit raw material. People
keep putting files there with a file manager — the inbox has no contract to violate, and a drop
zone that insists on a tool is a drop zone nobody fills.

Reading stays as it is built today: `search`, `list`, `read`, `status`, plus the `briefing`
call that [`knowledge-gate.md`](knowledge-gate.md) describes and that does not exist yet.
`search` and `list` report the `kind` — the directory, read from the path — and `origin` and
`state` from the frontmatter; `raw` and `superseded` documents and the root `README.md` are
not search hits, `list` carries them all.

### Superseding

`supersede` marks a document as replaced: `state: superseded`, `successor` and
`superseded_reason` go into the frontmatter, the body stays. It is refused in these cases:

- **A document in a generator directory.** Under `code/`, `libs/` and `versions/` a generator
  takes a document out by no longer publishing it; a supersession there would only last until
  its next run. The root `README.md` is refused too — it is navigation, not knowledge.
- **A document that is already superseded.** Whoever wants to change the successor supersedes
  the successor. That forms a chain instead of an overwritten reference.
- **A successor that search hides.** Otherwise the topic would vanish from search altogether.
  The successor is refused exactly when search hides it by its state — the same predicate
  search applies (`raw` and `superseded` today) on the same reading of the header as the index.
  A successor without a header, without `state`, or with a header the index cannot parse is a
  search hit and therefore allowed: `knowledge/manual/` belongs to a person, and documents
  written there by hand routinely carry no header. Stricter than search, and on purpose, the
  root `README.md` is never a successor, and neither is a document under `code/`, `libs/` or
  `versions/`: a generator run could remove it, and `successor` would point at nothing. The
  rule may refuse more than search hides, never less. One limit stays: a successor without a
  body yields no section and is never a hit.
- **A path through a linked directory**, for the document as for the successor; the index
  sees neither.

Generator directories and the root `README.md` are recognised without regard to case
(`Code/x.md`, `LIBS/y.md`, `readme.md`), on every platform: on a file system that does not
distinguish case they are the same files.

`supersede` takes no producer. After these rules only directories remain in which single
documents are written; there the ownership rule protects against a generator run overwriting
them, and that reason does not apply to a supersession.

**A supersession is final at the gate.** `write` refuses a superseded document, and no tool
sets the state back. A change by hand remains possible — `knowledge/manual/` belongs to a
person — but it is not a path through the gate; drift detection only re-reads such a file. A wrongly chosen successor is
corrected by superseding it with the right document; the chain stays as history.

## Migration

**Last, not first.** The zones are defined beside the directories in use, so nothing has to
move before the interface stands. Moving early would mean building the tools against a moving
target and migrating twice.

**The migration is the switch of the writers.** There is no separate migration pass beside
it. The writers go through the gate one after another, one step each, and no release runs
between the steps:

1. `/k-docs-extract`
2. `/k-doc-inventory` and `k-playbook inventory`
3. `/k-docs-code` and the skill `overlay-repo-analyse`
4. `/k-docs-tools`
5. findings — `rules/befunde.md`, the skill `befunde`, `/k-danke`
6. `/k-docs-index`, `_shared/context.md`, `/k-docs`, `rules/docs-sync.md`, the skill
   `ai-session-memory` and the entry point in `AGENTS.md`

A step also takes along what its writer leaves behind, and that splits into two halves that
need entirely different effort:

- **What a generator produces is not migrated at all.** `code/`, `libs/` and `versions/` are
  rewritten from their sources on the next run, through `publish` (steps 2 to 4). Moving them
  by hand would produce exactly the files the next run overwrites.
- **Everything written by hand is moved by an assistant, document by document through
  `write`.** Hand-written pages, extracts, findings, pitfalls: they need a subject, an origin
  and a state, which is a judgement per document and not a rule a script can apply. Step 5
  clears `material/`: findings go through `write` into `knowledge/findings/` (see "Findings
  and pitfalls count as knowledge"), the remaining raw material goes into `inbox/`. What was
  written by hand under `docs/` — the extracts left in `docs/extracted/`, the documents in
  `docs/learned/` — is moved in one pass within step 6.

**The old location `docs/learned/` belongs to the second half.** Up to v0.7.0,
`knowledge write` wrote to `k-playbook-local/docs/learned/`. The store does not read there
and has deliberately no fallback, so those documents are invisible to `list`, `search`,
`read` and `status`. Instead, `status` carries a note as long as Markdown files lie there,
and step 6 moves them with the rest of what was written by hand under `docs/`.

### Transitional reads

Whoever reads a previous location enters it in this table; whoever makes the read superfluous
removes it, clears the location and deletes the row. When the switch is complete the table is
empty and this section goes: from then on nothing is read that does not lie in `inbox/` or
`knowledge/`.

A previous location is one a switched writer has left. A location that is still the current
target of a writer not yet switched is not one, and it has no row: `material/befunde/` for
`/k-danke`, `rules/befunde.md` and the skill `befunde` until step 5, `docs/code/`, `docs/libs/`
and `docs/versions/` for `/k-docs-index` until steps 2 to 4. Every row names exactly one step.

| Reader | Previous location | Why it is still read | Removed in step | State |
|---|---|---|---|---|
| `/k-docs-extract` | `material/` | `/k-danke` calls `/k-docs-extract befunde` until step 5, and raw material still lies there; the selection shows it as "bisheriger Ort", nothing is copied into `inbox/` | 5 — findings through `write` into `knowledge/findings/`, the remaining raw material into `inbox/` | since task 073 |
| `/k-docs`, status report | `material/` | counts raw material to offer `/k-docs-extract` beside the pieces in `inbox/` | 5 | since task 073 |
| `/k-docs-index`, follow-up hint | `material/` | recommends `/k-docs-extract` while raw material lies there | 5 | since task 073 |
| `search`, hint on an empty store | `docs/` | the hint says that project knowledge lies under `k-playbook-local/docs/` until the migration | 6 | since task 073 |
| `status`, note on the old location | `docs/learned/` | documents written there up to v0.7.0 would be invisible without it | 6 — moves the documents | since task 063 |
| `AGENTS.md` (block from setup and from the skill `ai-session-memory`), `opencode.json` | `docs/README.md` as the entry point | the index knows `docs/` only; a new extract in `knowledge/extracted/` is found through `search`, not through the keyword index | 6 | since task 073 |
| `/k-docs-index` | `docs/extracted/` | indexes the extracts written there before task 073; it is no target any more | 6 — moves them | since task 073 |
| `/k-docs`, status report | `docs/extracted/` | counts the older extracts and checks their `generated.by` | 6 | since task 073 |
| skill `ai-session-memory` | `docs/extracted/` | lists the origin directories, `extracted/` included, when it registers the docs | 6 | since task 073 |

The state names the task with which the read became transitional; the reader itself may be
older.

## Decisions and what they exclude

Taken on 2026-09-10, all four of them decisions about intent rather than about technique,
and therefore the project owner's under [`../rules/design-entscheidungen.md`](../rules/design-entscheidungen.md).

**Three zones, not two.** Merging the drop zone and the backlog would save a move, and it
would cost the only property that makes a backlog useful: that it is empty when nothing is
outstanding.

**The inbox archives.** The alternative, letting the raw piece pass through and vanish, is
cheaper in space and loses repeatability: an extract could never be drawn a second time, and
its `origin` would point at something that no longer exists.

**The path carries the owner.** Sorting the path by subject would read better and would take
the generators' wipe away from them — each would then have to know its own files individually,
which is exactly the bookkeeping that goes wrong when a run is interrupted.

**A declared producer, not a technical barrier.** Ownership is checked against a name the
caller supplies. Two stronger options were on the table and were not taken: a check at commit
time, and a drift report that names the file and stays until somebody acknowledges it. A write
past the gate therefore continues to be repaired silently, as today.

**A generator publishes its directory, it does not write into it.** The alternative, a `clear`
followed by single writes, is closer to what the generators do today and leaves the store empty
for the length of a run.

**Findings and pitfalls count as knowledge.** They move into the store instead of staying in
`material/befunde/` and `guidelines/fallen.md`. `/k-danke` consequently promotes the `state`
of a document, not its location — the finding stays where it was written and becomes
`reviewed`.

## Open points

**The queue entry's format is internal.** `queue/<id>.md` carries `origin`, `target`,
`reason` and `added` in its frontmatter and notes in the body; it may change as long as the
arguments of `queue_add` stay the same. A processing run that needs more decides that when it
is built.

**`subject` has no vocabulary yet.** Open as a free field it is a second `origin` — useful for
reading, useless for filtering. Whether the values come from a closed list, and who maintains
it, is undecided.
