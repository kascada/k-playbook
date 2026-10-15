# The Kephalaion client

Which Kephalaion tools k-playbook calls itself, what it expects of them, and what was
measured at a real hub. The tools themselves are Kephalaion's; this page documents only
the part of their contract k-playbook relies on.

## The route

k-playbook talks to the Kephalaion **node** on this machine, never to a hub directly. The
node answers MCP on its loopback address (`node.listen` in Kephalaion's `config.yaml`);
k-playbook opens a session with `initialize`, then sends `tools/call` with the header pair
`X-Keph-Account-<hub>`/`X-Keph-Token-<hub>` from the project's binding. The token comes
from Kephalaion's own token file, goes only to the loopback address and never appears in
a result or an error. How the binding is found and checked: [`installation.md`](./installation.md#kephalaion-collection)
and `installer/docs/architecture.md`, section "Kephalaion-Anbindung".

All calls address the project's collection, `kephalaion.address` (`<hub>:<collection>`)
from `k-playbook context`. Code: `installer/internal/kephalaion/` (`client.go` for the
session and `whoami`, `docs.go` for `list`, `create`, `rename`).

## The tools k-playbook calls

| Tool | Where | Called by | Arguments used |
|---|---|---|---|
| `whoami` | node | the binding check behind `k-playbook context` (`kephalaion` block) and every `doc` call before it starts; again in the session of every `create`/`rename` before the first attempt (node version) | none |
| `list` | node's replica | `k_playbook_doc_create` (next number), `k_playbook_doc_list` | `collection`, `path`, `recursive: true`, `frontmatter` (only for listing), `limit: 1000`, `cursor` |
| `create` | through the hub | `k_playbook_doc_create` | `collection`, `name`, `content: ""`, `request_id` |
| `rename` | through the hub | `k_playbook_doc_done`, `k_playbook_doc_reopen` | `collection`, `name`, `new_name`, `request_id` |

Reading and writing a document's content is not on this list: a session does that
directly with Kephalaion's `read` and `write`. The `doc` tools only create, list and
move. Their contract: [`mcp.md`](./mcp.md#document-tools-in-the-collection).

## What k-playbook expects

- **`create` and `rename` go through the hub.** Their answer is the hub's row, and the
  node takes it into its replica before answering. Consequence: a `list` on the **same**
  node right after a `create` or `rename` already shows the result. Other nodes see it
  only after their next sync.
- **`name_taken`** is the structured error code (`error.code`) of `create` when a living
  document already has the name, and of `rename` when the target is taken. Nothing is
  written or overwritten. `create` uses it to try the next number (up to ten times);
  `done`/`reopen` pass it on unchanged.
- **`not_found`** from `rename`: there is no living document under the old name.
- **A tombstone does not block a name.** A deleted document is not listed and does not
  count for the next number; `create` and `rename` may reuse its name.
- **`rename` keeps the `id`** together with content and `created`; only name, revision and
  `updated` change.
- **`list` with `recursive`** returns documents only, each with its full name, `created`
  and `updated` (`at`, `by`); with `frontmatter` also the parsed frontmatter of `.md`
  documents. `created.by` is the user who created the document — the source of the user
  field for kinds without a user area (`queue/findings/`). A directory that does not
  exist yields an empty list, not an error.
- **`list` reports errors only as text**, without a structured code; `create` and
  `rename` carry `error.code`. k-playbook keeps Kephalaion's code where there is one and
  maps the rest to its own codes (see the next section).
- **`request_id`** (Kephalaion v0.3.1, Task 032) is accepted by `create` and `rename`:
  1–128 characters `[A-Za-z0-9._-]`, valid per account for 24 hours. The hub remembers
  only a success, in the same transaction. The same key with the same fields again answers
  `replayed: true` with the original revision and `ids`, without rows; the same key with
  different fields is `invalid`. A rejection is not remembered; its retry is checked anew.
- **A `replayed` answer** carries `address`, `name` (for `rename` the new name),
  `revision` and `ids`, but no `kind`, `id` or stamps, and the node does not write it into
  its replica — it triggers a sync instead.
- **A node before v0.3.1 rejects `request_id` loudly** (`isError` without a code) and
  writes nothing. k-playbook checks the version first and does not send to such a node.

Limits on the k-playbook side: 15 s per `list` call and per `create`/`rename` attempt, each
including its session, at most 16 MiB per answer, pages of 1000 entries and at most 100
pages for one listing.

## Request key, retry and the codes

`kephalaion.Client` generates a fresh `request_id` for every `Create` and `Rename` call
(`crypto/rand`); each number tried by `k_playbook_doc_create` is a call of its own and thus
gets its own key. What happens next depends on where an attempt stops:

| Where the attempt stops | Meaning | Client code | `doc` tools |
|---|---|---|---|
| address or token missing, `initialize` fails, the version check fails, node older than v0.3.1, caller gave up before sending | nothing reached the hub | `node_unreachable` | `kephalaion_unavailable` |
| node reports `unreachable`, `unsupported` (nothing stored) or `internal` (nothing sent) | nothing reached the hub | the node's code | `kephalaion_unavailable` |
| node reports a contract code (`name_taken`, `not_found`, `forbidden`, …) or an error without code | rejected, nothing written | that code (or none) | `name_taken`, `not_found`, else `kephalaion_error` |
| after sending: transport error, timeout, broken or unreadable answer, answer without structured content, HTTP 5xx, node code `outcome_unknown` | open | — retry — | — |

The retry is exactly one: the same `request_id` and the same fields, in a new session
with its own `ToolTimeout`, without a second version check. Success or `replayed` is
success; a rejection is a rejection. Every other result of the retry -- open again, the new
session not opening, or a node code `unreachable`/`unsupported`/`internal` -- ends in
`outcome_unknown`, because the first attempt is still open. If the caller's context was
cancelled after sending, there is no retry: `outcome_unknown` at once.

So `node_unreachable`/`kephalaion_unavailable` only ever means that the first attempt was
never sent or the node said that nothing was stored; every path on which the hub may have
been reached ends in success, rejection or `outcome_unknown`.

## Measured

At hub `vm`, collection `k-playbook-dev`, with the CLI of a freshly built binary
(Task 088, 2026-10-15):

- Every document created or moved appeared in the very next `k-playbook doc list`, about
  0.1 s later, and unchanged 6 to 30 s later. **No replica lag was observable** on the
  node that wrote. Task 088 accepts a lag on principle (equal numbers with different
  names are allowed), but at the node that wrote there is none to accept.
- `rename` onto a taken name returned `name_taken`; both documents stayed as they were.
- The `id` survived `done` and `reopen`.
- `created.by` was `kamran` for a document under `queue/findings/`.

At the same hub and collection, node `wsl-kleist` on v0.3.1, with a freshly built binary
(Task 090, 2026-10-15):

- `doc create`, `done` and `reopen` worked with the `request_id` that every `create` and
  `rename` now carries; the node accepted the argument on both tools.
- The same `create` sent twice with the same `request_id` and the same fields: the first
  answer created the document (revision 12), the second answered `replayed: true` with the
  same revision and the document's id in `ids`, without `kind` or stamps. A following
  `create` under the same name with a new key got `name_taken`, and `list` showed the
  document exactly once. **A retry with the same key does not create a second document.**

Not measured: a node that is cut off from its hub, a second node listing what the
first one wrote, and the message for a node before v0.3.1 at a real node (covered by a unit
test). Commands and output are in
`k-playbook-local/material/befunde/ablage-userbereich-queue.md`.

## Where the tools are documented

In the Kephalaion repository (`github.com/kephalaion/kephalaion`):

- `docs/konzept.md`, sections "Werkzeuge" → "Allgemein — lesen" and "Allgemein —
  schreiben" — the MCP tools with their arguments and answers.
- `docs/vertrag.md`, sections "Schreibvorgänge" (`create`, `rename`, "Antwort",
  "Wiederholung mit Schlüssel"), "Ausgang und Wiederholung" and "Fehler" — the contract
  between node and hub, including `name_taken`, tombstones, `request_id`/`replayed` and how
  the node takes the hub's rows into its replica.
- `docs/begriffe.md`, "write error codes" — the node's own codes `unreachable`,
  `outcome_unknown`, `unsupported` and `internal`.

The tool descriptions a node reports over MCP (`tools/list`) are the short form of the
same.
