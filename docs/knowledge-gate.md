---
title: Knowledge Gate
description: The concept for depositing knowledge through the MCP server and querying it back — the classification of what is stored, the query surface over it, the staged plan for the index, and the measurements and reasons behind those decisions.
---

# Knowledge Gate

**This is the entry page for the knowledge store.** It says why the store exists, what is
built, how we proceed from here, how something is deposited and asked for, and which
decisions rest on which measurements. Everything else about the store is linked from here,
so a conversation about it needs to point at this one page.

| Page | What it holds |
|---|---|
| this page | purpose, status, next steps, the concept of depositing and querying, decisions, open points |
| [`knowledge-layout.md`](knowledge-layout.md) | where things lie: the zones `inbox/`, `queue/` and `knowledge/`, who may write where, the frontmatter, the write tools, the migration |
| [`knowledge-storage.md`](knowledge-storage.md) | the picture: how knowledge flows through the zones and back out; shown in the interface under Knowledge |
| [`mcp.md`](mcp.md#knowledge-contract), "Knowledge Contract" | the exact arguments, results and error codes of the tools |

The page records decisions together with the measurements they rest on, so a later change
knows what it is overturning.

## Why the gate exists

The gate is not primarily a search feature. It is an attempt to change what an assistant
does first.

An assistant that wants to know something about a project reaches for the code: it greps,
it opens files, it reconstructs from source what somebody already wrote down elsewhere.
That is slow, it burns context, and it misses everything that is not in the repository —
the tool research, the Confluence page, what a previous session found out.

The gate wins that reflex only if asking is *faster* than searching, and if the assistant
is told that the gate knows more. Both halves matter. Speed is the engineering
requirement; the instruction is what makes it happen. Everything below follows from that.

## Status

| Part | State |
|---|---|
| Chunking, BM25 index, drift detection | built |
| `search`, `list`, `read`, `status` over CLI and MCP | built |
| The three zones `inbox/`, `queue/`, `knowledge/` and the write tools `write`, `publish`, `supersede`, `inbox_*`, `queue_*` | built, task 056; see [`knowledge-layout.md`](knowledge-layout.md) |
| Classification of deposits (kind from the path, origin, state, format in the frontmatter) | built, task 056 |
| `raw` and `superseded` out of search by default | built, task 056 |
| Migration of `docs/` into `knowledge/` | not built; `knowledge/` is empty until then |
| Briefing call, filters on `kind`/`state`/`subject` | concept, this page |
| Automatic learning from finished sessions | concept, see [`knowledge-storage.md`](knowledge-storage.md) |
| Test of the write side against the built tools | run on 2026-09-13; the findings it reproduced are fixed by task 063 and re-measured in the project on 2026-09-14. The edges found in the review of task 063 — linked directories at `write` and `supersede`, the successor rule, case in generator directories and the root `README.md`, the restore at `read`, `CreateLocal` with an unwritable index — are fixed by task 064 and re-measured in the project on 2026-09-15 over MCP and the command line |
| Document view for `knowledge/` in the interface | not built; until it exists the anchor check of the write test cannot be satisfied, see "Open points" |
| Ranking correction | built, task 056: the root `README.md` is out of the search index |
| Vectors, local model | deliberately not built, tier two |
| LanceDB or another dedicated database | deferred; revisited only if the corpus or the retrieval quality demands it |
| External source connectors | deliberately not built, tier three |

## How we proceed

The write side is built, and `knowledge/` is still empty in every project. The order from
here:

1. **Test the write side against the real tools.** Before any document moves, every write
   tool is exercised in a project and the result is checked on disk, in the index and in the
   interface. The criteria are listed below.
2. **Migrate once.** What a generator produces is not moved: the generators run again and
   `publish` into `knowledge/`. Everything written by hand — pages, extracts, findings,
   pitfalls — is moved in a single pass by an assistant, document by document through
   `write`, with subject, origin and state decided per document. That pass is the second and
   larger test of the contract. Details are in [`knowledge-layout.md`](knowledge-layout.md#migration).
3. **Switch the entry point.** `/k-docs-index` writes `knowledge/README.md` through `write`
   with `producer: docs-index`, and the generated `AGENTS.md` tells a session to ask the store
   first. Until then `docs/README.md` stays the authoritative entry point (see "Open points").
4. **Build the reading side.** `briefing` and the filters on `kind`, `state` and `subject`, as
   described under "Querying".
5. **Tier two only on demand.** Vectors from a local model, or a database of its own, only if
   the corpus or the retrieval quality after the migration asks for it.

### What the write test checks

- **Ownership.** Each producer from the table in `knowledge-layout.md` writes into its own
  directory; a target outside it is refused with an error and leaves no file behind.
- **Frontmatter.** The written file carries the fields that were passed, composed by the
  tool, with `updated` set; `list` and `read` return them, the title included.
- **State.** A `raw` and a `superseded` document are readable through `read` and `list` but
  are not search hits; `supersede` sets `successor` and `superseded_reason`.
- **Queue.** A `write` naming a queue entry removes the entry once the document exists; a
  write that fails leaves the entry in place.
- **Publish.** A generator's `publish` replaces its directory as a whole; a `publish` without
  any document is refused, and a failed run leaves the previous state.
- **Inbox.** What `inbox_put` stores is listed by `inbox_list` and never appears in search.
- **Index.** After a write, `status` counts the new chunks, `search` finds the document, and a
  hit's anchor opens the right heading in the interface. The second half cannot currently be
  satisfied: the interface renders only `docs/` and has no document view for `knowledge/` (see
  "Open points"). What the write side measures is that the hit carries the anchor of the right
  heading.
- **Past the gate.** A file changed by an editor or a `git pull` is picked up as drift on the
  next access.

## The store

The zones, the ownership rule and the frontmatter contract are defined in
[`knowledge-layout.md`](knowledge-layout.md); this section describes only what the store is
made of. It is a directory of Markdown files under `k-playbook-local/`, versioned
in git. That is the whole storage layer, and it stays that way. There is no SQL, no
document store, no service. The Markdown files are the truth; everything else is derived
and disposable.

The index lives beside it under `k-playbook-local/cache/knowledge/`, is never versioned,
and can be deleted at any time. It rebuilds itself on the next access. It also carries a
hash per file and compares it against the tree, so a change made past the gate — a `git
pull`, an editor, a generator run — is detected and corrected rather than silently
outlasted.

## Depositing: what a write has to declare

A write names its producer, its path, the frontmatter as fields and the body. Four things
are declared, and they answer four different questions.

### Kind — what this is

The kind is the directory under `knowledge/`, read from the path and never from the
document. Which directory belongs to which producer is the table "Who may write where" in
[`knowledge-layout.md`](knowledge-layout.md); it is the only place that table lives, so this
page does not repeat it. The vocabulary is closed because as soon as a connector writes, the
folder cannot be inferred from who is calling.

**Ownership follows the kind.** A generator rewrites its directory completely on the next
run. Anything a different producer put there disappears without a trace. That is why a
directory has exactly one owner, only the owner may write there, and the tool refuses a
target outside the producer's directory.

### Origin — where it came from

Free-form in its values, fixed in its shape. Not the directory, but the actual provenance:
the system, the identifier there, the address, and when it was fetched. For a Confluence
extract that is space and page id and the URL and a date; for a session finding it is the
task or the topic.

Origin is what makes an extract auditable and what a refresh needs in order to know what it
is replacing. It goes into the document's frontmatter, not into the path.

### State — how far along it is

A closed vocabulary, and the one that has to have teeth. A state that does not change
retrieval is decoration.

| State | Meaning | In search by default |
|---|---|---|
| `raw` | as fetched, not condensed | no |
| `condensed` | machine-made summary, unchecked | yes |
| `reviewed` | a person or a review pass confirmed it | yes |
| `superseded` | kept for history, replaced by something newer | no |

Raw material stays out of results because it would drown the condensed version of itself;
it remains readable on request. `superseded` is what a refresh produces instead of a
deletion, which the gate does not have.

**This also settles precedence.** When a session finding contradicts generated
documentation, the higher state wins, and at equal state the more recent one. Without that
rule the contradiction is only visible, never resolved. *This is a proposal; the ordering
is a decision about intent and belongs to the project owner.*

### Format — what it originally was

Markdown is the storage format, always. `format` records what the original was: `markdown`,
`text`, `html`, `image`, `pdf`. HTML is converted on the way in. An image cannot be
full-text indexed at all, so it is stored as a file with a Markdown stub that describes it;
the stub is what the index sees.

Keeping conversion at the entrance is what preserves the promise that the Markdown files
are the truth. The alternative, storing every format natively and teaching the index about
each one, moves complexity into the part that has to stay fast.

## Querying: the surface

The aim is broad coverage without an opaque box. Two principles shape it.

**The knowledge store and the working tree are different corpora.** They differ in
freshness, in trust, and in what a hit even means. Merging them into one ranked list makes
the rank meaningless and takes away the ability to say "ask the store before you grep".
They therefore stay separate tools.

**The store's own search stays method-agnostic.** A hit names path, heading, excerpt,
origin and rank. No field says whether a term index or a vector index produced it. That is
what keeps the index replaceable.

| Tool | Answers | Notes |
|---|---|---|
| `briefing` | what is in the store at all | one call at session start; counts per kind, state and origin, the document titles, how fresh. Data, not prose. Not the content itself. |
| `search` | which sections match a question | the contract above; filters on kind, state, origin |
| `list` | which documents exist | path, title, kind |
| `read` | one document in full | Markdown, never HTML |
| `search_code` | where something appears in the source | separate corpus, separate fields: file, line, matched text. Lexical by nature and tied to the working tree. |
| `facets` | which filter values exist | makes the filters discoverable and improves the briefing |

**The order is the policy, and it belongs in the instructions, not in a tool.** Briefing at
the start; then ask the store; then read what looks right; only then search the code. A
tool cannot enforce this. `AGENTS.md` has to say it, which is also where the assistant is
told that the store knows things the repository does not.

**The feedback loop closes here.** When the store answers poorly and the deeper search
finds something, the finding is written back into `findings/` by the session. The next session gets it locally
and fast. Note what judges "answered poorly": the assistant that read the hits, not a
threshold on a score. That is deliberate. A score threshold would break the moment the
index behind it changes, which is exactly the change this design is built to allow.

## The staged plan

One solution for every project, differing at most in stages. A per-project decision about
retrieval, or a per-project database, is the thing to avoid.

**Tier one, always, no installation.** BM25 in the Go process, plus list and read for an
assistant that navigates rather than searches. No dependency, identical in every project,
works offline. Nothing falls below this.

**Tier two, per machine, optional.** One embedding model on the host, in the manner of the
base tools under `~/.local/share/k-playbook/`, not one per project. Vectors go into the
same index and the two result lists are fused. Without the model a project silently stays
on tier one. The decision is made once per machine, never per project.

**Tier three, per project.** Connectors for outside sources. These have to differ, because
one project has Confluence and the next has something else. They change nothing about the
access path; they only fill it.

`status` already carries `indexKind`, `model` and `dims`. Tier one reports `bm25` with an
empty model, tier two reports the model and its dimension. No caller changes a line when a
project moves up. That is what those fields are for.

## Decisions and what they rest on

**A dedicated vector database is not needed.** Measured: this repository holds 6 documents
and 46 chunks, which is too small to tell anything; the reference project
`squad-km-dev-setup`, with the generators having run, holds 60 documents and 506 chunks,
distributed over `manual` 222, `libs` 115, `code` 103, `versions` 40 and the root README 26.
Tool references are meant to grow considerably, learned knowledge accumulates, outside
sources are added. That points at a few thousand chunks, not tens of thousands, because
`knowledge-storage.md` explicitly discards the conversation itself and keeps only the
distillate. BM25 in-process handles that range without effort.

**Reading scales, writing does not, and that is acceptable.** Measured against a throwaway
copy grown to 640 documents and 2246 chunks:

| Corpus | Write per document | Search |
|---|---|---|
| 80 documents | 18 ms | |
| 300 documents | 35 ms | |
| 640 documents, 2246 chunks | 48 ms | 55 ms |

Search stays an order of magnitude faster than a grep over a codebase even at four times
the current corpus. Writing costs more with every increment, because each write loads the
index, checks every hash and writes the whole file back; the cost per document grows
linearly with the corpus and an import therefore grows quadratically. This is accepted:
writing happens rarely and in separate processes — a research run, a comparison on pull,
adding a source, a learning pass — and each of those needs far more preparation than the
write itself. Should it ever hurt, the fix is a batch mode that rebuilds the index once per
import run instead of once per document.

**Remote embedding models are ruled out, and not on cost.** A query needs an embedding too,
not only the corpus. That is a network round trip of 100 to 500 ms per question, ten to
twenty-five times the entire current answer time of roughly 20 ms at 506 chunks. The gate
would become slower than the reflex it is meant to replace, which destroys its only reason
to exist. Vectors are therefore possible with a local model or not at all. This is also why
`status` carries `model` and `dims` from the start: an index built with one model and
queried with another returns nonsense, and it has to be visible rather than silent.

**No score in the contract.** BM25 values and cosine distances are not translatable into
one another, so a caller that tests for "> 0.8" breaks when the index is replaced. The
order is the statement, and `rank` counts it from 1. The feedback loop confirms the choice
from a second direction: what decides whether an answer was good enough is the assistant
that read it, and that judgement is method-independent.

**Anchors come from one Goldmark run over the whole file.** The heading ids are generated
by the same configuration that renders the documentation, shared through
`internal/markdown`. Two properties, measured rather than assumed: non-ASCII is dropped
from ids without replacement, and uniqueness is counted across the whole parser run. Both
mean that chunking per section, or a self-built slug rule, would produce anchors that miss
silently. The heading in its own wording is the leading field; the anchor is a display aid.

## Where the built part came from

The parts marked "built" above were made by task 055, released as v0.7.0 in commit `2915328`.
The task file itself, with its review log, its stage record and the eight review findings that
were fixed before the tag, lives in this repository under
`k-playbook-local/tasks/done/055-wissenstor-mcp.md`. That directory is a separate, ignored
repository, so the file travels with this development machine and not with the distribution.
It is worth reading before changing anything on this page: several things that look arbitrary
here are the outcome of an argument recorded there.

Two of its decisions are load-bearing for everything above. The response contract deliberately
carries no score, which is what lets the index be replaced. And the anchors come from a single
Goldmark pass over the whole file, which is what lets a hit point at a heading the renderer
actually produced.

## Settled since

Two open points of the first version were closed by task 056 and are recorded here so a
later reader knows what was decided and why.

**The ranking put pointers above their targets.** The keyword index in this repository's
`README.md` took rank 1 for `ApplyLinks`, `SHA256SUMS` and `Symlink-Konflikt`; in the third
case the actual target did not appear in the top five at all. The cause is structural: BM25
rewards short, term-dense documents twice over, and an index of keywords is exactly that by
construction. Of the three ways out — exclude the keyword section, keep the README out of
the search index, weight the root down — the second was taken: the root `README.md` of the
store is generated navigation without content of its own, it gives no chunks to the index
at all (so it does not skew the term statistics either), and `list` covers navigation.

**`source` meant two different things in the same tool family.** `search` and `list`
returned `source` as the origin directory — the *kind* — while `write` took `source` as a
free-text provenance note — the *origin*. The field was split before anything consumed it:
`kind` is read from the path, `origin` lives in the frontmatter, and the tools report both.

## Open points

**The documentation index still lives in `docs/`.** `/k-docs-index` writes
`k-playbook-local/docs/README.md`, which `AGENTS.md` declares the authoritative entry point,
and knows nothing of `knowledge/`. In the store its place is `knowledge/README.md`, written
by the same command through `write` with `producer: docs-index`; that switch is part of the
migration, together with the instruction in the generated `AGENTS.md`.

**An empty store answers silently.** Until the migration `knowledge/` is empty, and `search`
returns an empty hit list without any hint — indistinguishable from "nothing on this topic",
although `docs/` holds the material. `status` shows it (no chunks), `search` does not. A hint
when the index is empty would keep an assistant from drawing the wrong conclusion in the
meantime.

**Outside sources change upstream.** Drift detection notices a local edit. It cannot notice
that the Confluence page an extract came from has changed. Whether the gate refreshes such
sources periodically, or whether an extract stays a dated snapshot, is undecided.

**Deleting and renaming do not exist, and `supersede` is only half the answer.** A second
import run finds pages that are gone; it can supersede them, but nothing yet does. A store
that only ever grows will eventually hold things that vanished upstream long ago, and which
run marks them is undecided.

**There is no document view for `knowledge/`.** The interface renders only
`k-playbook-local/docs/` (`GET /api/docs/file`); the page `/knowledge` shows the picture from
[`knowledge-storage.md`](knowledge-storage.md). A hit's anchor therefore has no place to open
in, and the anchor check of the write test stays unsatisfiable until such a view exists.

**A successor can still drop out of search later.** `supersede` refuses a successor that search hides by its state, but a later `write` onto the
successor may set `state: raw`. The
topic then leaves search again, without any refusal. Whether `write` should guard the state of
a document that is another document's successor is undecided.
