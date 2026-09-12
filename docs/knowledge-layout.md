---
title: Knowledge Layout
description: The three storage zones under k-playbook-local — inbox for what arrives, queue for what is outstanding, knowledge for what holds — with the ownership rule, the frontmatter contract, the table of what lands where, the write tools every deposit goes through, and when the migration happens.
---

# Knowledge Layout

[`knowledge-gate.md`](knowledge-gate.md) describes how a deposit is asked for. This page
describes where it lies. It defines the storage zones from scratch, deliberately beside
the directories in use today rather than on top of them: the new names are free, so the
migration is a move and never a rename during operation.

**Status: built.** The three zones, the write tools and the index over `knowledge/` exist
since task 056 (the zones are created by setup, the tools are `k-playbook knowledge …` and
`k_playbook_knowledge_*`, see [`mcp.md`](mcp.md), "Knowledge Contract"). What is not yet
built is the migration: `docs/` still holds everything and is still shipped, `knowledge/`
is empty in every project until the migration moves the documents, and the reading side
(`briefing`, filters on `state`) is a separate step.

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
| `sources` | the inbox paths a document was distilled from, where there are any |
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
`staleFiles`) without naming the file. A drift report that names the file and stays until
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
| `supersede` | `path`, `successor`, `reason` | the superseded path |

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

`publish` is how a generator writes, and the only way it does. It hands over its **complete**
set of documents; the tool writes them and removes what is not in the set. A run that dies
halfway changes nothing at all, whereas a `clear` followed by writes would leave the store
empty for as long as the run takes.

`inbox_put` exists because a connector and a session need a way to deposit raw material. People
keep putting files there with a file manager — the inbox has no contract to violate, and a drop
zone that insists on a tool is a drop zone nobody fills.

Reading stays as it is built today: `search`, `list`, `read`, `status`, plus the `briefing`
call that [`knowledge-gate.md`](knowledge-gate.md) describes and that does not exist yet.
`search` and `list` report the `kind` — the directory, read from the path — and `origin` and
`state` from the frontmatter; `raw` and `superseded` documents and the root `README.md` are
not search hits, `list` carries them all.

## Migration

**Last, not first.** The zones are defined beside the directories in use, so nothing has to
move before the interface stands. Moving early would mean building the tools against a moving
target and migrating twice.

It splits into two halves that need entirely different effort:

- **What a generator produces is not migrated at all.** `code/`, `libs/` and `versions/` are
  rewritten from their sources on the next run. Moving them by hand would produce exactly the
  files the next run overwrites.
- **Everything else is moved once, by an assistant, in a single pass.** Hand-written pages,
  extracts, findings, pitfalls: they need a subject, an origin and a state, which is a judgement
  per document and not a rule a script can apply. That pass is worth doing exactly once, and
  only when the write interface exists — every document it touches goes in through the tool,
  which is at the same time the first real test of the contract.

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
