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
session and `whoami`, `docs.go` for `list`, `read`, `create`, `write`, `rename`, `delete`).

## The tools k-playbook calls

| Tool | Where | Called by | Arguments used |
|---|---|---|---|
| `whoami` | node | the binding check behind `k-playbook context` (`kephalaion` block) and every `doc` and `todo` call before it starts; again in the session of every `create`/`write`/`rename`/`delete` before the first attempt (node version) | none |
| `list` | node's replica | `k_playbook_doc_create` (next number), `k_playbook_doc_list`, every todo access (task 097) | `collection`, `path`, `recursive: true`, `frontmatter` (only for listing), `limit: 1000`, `cursor` |
| `read` | node's replica | `k_playbook_doc_read` and `k-playbook read` for a `keph:` reference (task 093), also as the target of an include; every todo access, one call per todo (task 097) | `collection`, `name` (the path without the leading `/`, `~` already replaced) |
| `create` | through the hub | `k_playbook_doc_create`; `todo add` and `todo migrate` (task 097) | `collection`, `name`, `content` (`""` for `doc_create`, the todo document for todos), `request_id` |
| `write` | through the hub | `todo update` (text, `done:` field) (task 097) | `collection`, `name`, `content`, `base_revision` (the revision from the `read` just before), `request_id` |
| `rename` | through the hub | `k_playbook_doc_done`, `k_playbook_doc_reopen`; `todo update` with done/reopen (task 097) | `collection`, `name`, `new_name`, `request_id` |
| `delete` | through the hub | `todo delete` (task 097) | `collection`, `name`, `base_revision`, `request_id`; never `recursive` |

Reading a document's content goes both ways since task 093: a session can call
Kephalaion's `read` directly, and `k_playbook_doc_read` / `k-playbook read keph:/…` call it
through this client (`Client.Read`). Writing is still done directly by the session with
Kephalaion's `write`; the `doc` tools only create, list and move. Their contract:
[`mcp.md`](./mcp.md#document-tools-in-the-collection). The todo functions (task 097) are the
first in k-playbook that write content and delete themselves (`Client.Write`,
`Client.Delete`), since a todo is the document's body plus a few frontmatter fields:
[`mcp.md`](./mcp.md#todo-contract-and-migration).

## What k-playbook expects

- **`create` and `rename` go through the hub.** Their answer is the hub's row, and the
  node takes it into its replica before answering. Consequence: a `list` on the **same**
  node right after a `create` or `rename` already shows the result. Other nodes see it
  only after their next sync.
- **`name_taken`** is the structured error code (`error.code`) of `create` when a living
  document already has the name, and of `rename` when the target is taken. Nothing is
  written or overwritten. `create` uses it to try the next number (up to ten times);
  `done`/`reopen` pass it on unchanged.
- **`not_found`** from `rename`, `write` and `delete`: there is no living document under the
  (old) name.
- **`stale_revision`** from `write` and `delete` (and `rename`, which k-playbook calls without
  it): the document no longer has the revision given as `base_revision`; nothing is written,
  the message names the current revision. The todo functions read the document from the
  replica, then write or delete on that revision, so a change another machine made in
  between is not silently overwritten.
- **`write` with unchanged content** writes nothing at the hub: no new revision, the answer
  carries the existing row.
- **`delete` sets a tombstone** (`deleted: true` in the answer, new revision). Without
  `recursive` a directory is `invalid`; k-playbook deletes single documents only.
- **A tombstone does not block a name.** A deleted document is not listed and does not
  count for the next number; `create` and `rename` may reuse its name.
- **`rename` keeps the `id`** together with content and `created`; only name, revision and
  `updated` change.
- **`list` with `recursive`** returns documents only, each with its full name, `created`
  and `updated` (`at`, `by`); with `frontmatter` also the parsed frontmatter of `.md`
  documents. `created.by` is the user who created the document — the source of the user
  field for kinds without a user area (`queue/findings/`). A directory that does not
  exist yields an empty list, not an error.
- **`read` answers from the replica**, without the network, like `list`: the state of the
  last sync, so what another machine has just written can be missing; own writes are
  there at once. Its answer (`ReadOutput` in Kephalaion's `internal/node/mcpnode/read.go`)
  carries `kind` -- `document`, `directory` or `none` --, `address`, `name`, `id`,
  `revision`, `created`, `updated`, `size`, `writable` and, for a document, `content`
  unchanged with frontmatter (`""` for an empty document; the field is absent for a
  directory and for `none`). **`none` is not an error**: the name has neither a living
  document nor a directory; k-playbook turns it into `not_found`. A directory becomes
  `invalid_input`, since `keph:` references name single documents. `k-playbook`
  sends only `collection` and `name`; `content: false`, `frontmatter` and `id` are not
  used. A name that does not satisfy Kephalaion's name rule is rejected by the node as
  text.
- **`list` and `read` report errors only as text**, without a structured code (collection
  not readable, replica never synced, invalid name); `create` and `rename` carry
  `error.code`. k-playbook keeps Kephalaion's code where there is one and maps the rest
  to its own codes (see the next section).
- **`request_id`** (Kephalaion v0.3.1, Task 032) is accepted by `create`, `write`, `rename` and
  `delete`:
  1–128 characters `[A-Za-z0-9._-]`, valid per account for 24 hours. The hub remembers
  only a success, in the same transaction. The same key with the same fields again answers
  `replayed: true` with the original revision and `ids`, without rows; the same key with
  different fields is `invalid`. A rejection is not remembered; its retry is checked anew.
- **A `replayed` answer** carries `address`, `name` (for `rename` the new name),
  `revision` and `ids`, but no `kind`, `id` or stamps, and the node does not write it into
  its replica — it triggers a sync instead.
- **A node before v0.3.1 rejects `request_id` loudly** (`isError` without a code) and
  writes nothing. k-playbook checks the version first and does not send to such a node.

Limits on the k-playbook side: 15 s per `list` or `read` call and per
`create`/`write`/`rename`/`delete` attempt, each including its session, at most 16 MiB per answer, pages of 1000 entries and at
most 100 pages for one listing.

## Request key, retry and the codes

`kephalaion.Client` generates a fresh `request_id` for every `Create`, `Write`, `Rename` and
`Delete` call
(`crypto/rand`); each number tried by `k_playbook_doc_create` is a call of its own and thus
gets its own key. What happens next depends on where an attempt stops:

| Where the attempt stops | Meaning | Client code | `doc` and `todo` tools |
|---|---|---|---|
| address or token missing, `initialize` fails, the version check fails, node older than v0.3.1, caller gave up before sending | nothing reached the hub | `node_unreachable` | `kephalaion_unavailable` |
| node reports `unreachable`, `unsupported` (nothing stored) or `internal` (nothing sent) | nothing reached the hub | the node's code | `kephalaion_unavailable` |
| node reports a contract code (`name_taken`, `not_found`, `stale_revision`, `forbidden`, …) or an error without code | rejected, nothing written | that code (or none) | `name_taken`, `not_found`, `stale_revision`, else `kephalaion_error` |
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

At the same hub and collection, with a freshly built binary (task 093, 2026-10-18):

- A document created with Kephalaion's `create` came back through
  `k-playbook read keph:/user/~/workbench/…` (with `~`) and through the real name, byte for
  byte the content that Kephalaion's `read` returned; the `ref` in the answer carried the
  real name. The whole call, status check (`whoami`) and `read` included, took 0.05 s.
- A missing name answered `not_found` (the node reports `kind: none`, no error); the
  directory name without `/` `invalid_input`. The document was deleted afterwards.
  Output in `k-playbook-local/material/befunde/command-huelle-verweis.md`.

At the same hub and collection, node `wsl-kleist` v0.3.1, with a freshly built binary
(task 097, 2026-10-20):

- `write` with `base_revision` and `delete` with `base_revision`, both with a `request_id`,
  were accepted by the node: `k-playbook todo update --text`, `--done` (`write` of `done:`,
  then `rename` into `todos/done/`), `--reopen` (`rename` back, then `write` without `done:`)
  and `todo delete` all succeeded; a deleted todo was gone from the next listing, and an
  `update` of its id answered `not_found`.
- `k-playbook todo migrate` created 28 documents (`create` with content) in about 2.5 s; the
  following `todo list --all` (one `list` plus 28 `read` calls, each in its own session) took
  0.28 s. The frontmatter came back unchanged, `created: ""` as an empty string in `list`
  with `frontmatter`.

Not measured: `stale_revision` at a real hub (covered by unit tests), a second node seeing the
todos. Commands and output: `k-playbook-local/material/befunde/todos-in-der-collection.md`.

## Where the tools are documented

In the Kephalaion repository (`github.com/kephalaion/kephalaion`):

- `docs/konzept.md`, sections "Werkzeuge" → "Allgemein — lesen" and "Allgemein —
  schreiben" — the MCP tools with their arguments and answers; `read` in "Gebaut in Task
  009" (reads the replica, errors as text) and "Frontmatter in `list` und `read`". The
  authoritative shape of its answer is `ReadOutput` in `internal/node/mcpnode/read.go`;
  `docs/vertrag.md` covers only node and hub and does not describe `read`.
- `docs/vertrag.md`, sections "Schreibvorgänge" (`create`, `rename`, "Antwort",
  "Wiederholung mit Schlüssel"), "Ausgang und Wiederholung" and "Fehler" — the contract
  between node and hub, including `name_taken`, tombstones, `request_id`/`replayed` and how
  the node takes the hub's rows into its replica.
- `docs/begriffe.md`, "write error codes" — the node's own codes `unreachable`,
  `outcome_unknown`, `unsupported` and `internal`.

The tool descriptions a node reports over MCP (`tools/list`) are the short form of the
same.
