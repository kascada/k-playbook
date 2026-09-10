---
title: Knowledge Gate
description: The concept for depositing knowledge through the MCP server and querying it back — the classification of what is stored, the query surface over it, the staged plan for the index, and the measurements and reasons behind those decisions.
---

# Knowledge Gate

[`knowledge-storage.md`](knowledge-storage.md) draws where knowledge flows. This page is
the concept underneath it: how something is deposited, how it is asked for, and why the
parts are shaped the way they are. It records decisions together with the measurements
they rest on, so a later change knows what it is overturning.

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
| `search`, `list`, `read`, `write`, `status` over CLI and MCP | built |
| Classification of deposits (kind, origin, state, format) | concept, this page |
| Briefing call | concept, this page |
| Ranking correction | known defect, see Open points |
| Vectors, local model | deliberately not built, tier two |
| LanceDB or another dedicated database | deferred; revisited only if the corpus or the retrieval quality demands it |
| External source connectors | deliberately not built, tier three |

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

Today a write carries a path, the content, and a free-text origin note. That is not enough
once several kinds of producer write through the same server. Four things have to be
declared, and they answer four different questions.

### Kind — what this is

A closed vocabulary. It says what sort of knowledge this is, and it decides which directory
the document lands in.

| Kind | Meaning | Written by |
|---|---|---|
| `code` | derived from the source of this project | `/k-docs-code` |
| `libs` | reference for a tool, library or stack in use | `/k-docs-tools` |
| `extracted` | distilled from raw material | `/k-docs-extract` |
| `versions` | the version inventory | `/k-doc-inventory` |
| `manual` | written by a person | a person |
| `learned` | a finding from a finished session | the gate |
| `external` | an extract of an outside source | a connector |

The first five exist. `learned` is the gate's own destination today. `external` is the new
one and the reason the vocabulary has to be closed: as soon as a connector writes, the
folder cannot be inferred from who is calling.

**Ownership follows the kind.** A generator rewrites its directory completely on the next
run. Anything a different producer put there disappears without a trace. That is why the
gate writes only to `learned/` today, and why the rule has to generalise rather than be
repeated per case: a directory has exactly one owner, and only the owner may write there.

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
finds something, the finding is written back as `learned`. The next session gets it locally
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

## Open points

**The ranking puts pointers above their targets.** The keyword index in this repository's
`README.md` takes rank 1 for `ApplyLinks`, `SHA256SUMS` and `Symlink-Konflikt`; in the third
case the actual target does not appear in the top five at all. The cause is structural:
BM25 rewards short, term-dense documents twice over, and an index of keywords is exactly
that by construction. It competes against the documents it exists to open up, on precisely
the terms somebody searches for. Three ways out: exclude the keyword section from chunking,
keep the README out of the search index entirely since `list` covers navigation anyway, or
weight the root origin down. This is a ranking decision and belongs to whoever has the
finished index in front of them.

**`source` means two different things in the same tool family, and it should be renamed
before anything consumes it.** `search` and `list` return `source` as the origin directory,
which in the vocabulary above is the *kind*. `write` takes `source` as a free-text provenance
note and puts it in the frontmatter, which above is the *origin*. The value that goes in is
not the value that comes back. Demonstrated: a write with `--source confluence` produces a
document whose frontmatter says `confluence`, while a search for it reports `source: learned`.

The collision is invisible today only because the gate writes to exactly one directory, so the
kind is a constant. It stops being invisible the moment a connector writes as `external` or a
generator writes through the gate.

Splitting the field into `kind` and `origin` costs almost nothing right now: the tools are
released but nothing reads them yet, and the interface that will read them has not built its
listing block. Later it costs a release plus every caller. *This is a contract change and
therefore a decision for the project owner.*

**The documentation index does not know `learned/`.** Anything written through the gate is
findable by search but does not appear in `k-playbook-local/docs/README.md`, which
`AGENTS.md` declares the authoritative entry point. Either the index takes the directory in,
or the directory stays out on purpose and `AGENTS.md` says that the index is no longer
complete. Tracked as a todo.

**Outside sources change upstream.** Drift detection notices a local edit. It cannot notice
that the Confluence page an extract came from has changed. Whether the gate refreshes such
sources periodically, or whether an extract stays a dated snapshot, is undecided.

**Deleting and renaming do not exist.** A second import run finds pages that are gone. The
`superseded` state above is the proposed answer, but it has not been built, and a store that
only ever grows will eventually hold things that vanished upstream long ago.
