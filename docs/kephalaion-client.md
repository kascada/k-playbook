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
| `whoami` | node | the binding check behind `k-playbook context` (`kephalaion` block) and every `doc` call before it starts | none |
| `list` | node's replica | `k_playbook_doc_create` (next number), `k_playbook_doc_list` | `collection`, `path`, `recursive: true`, `frontmatter` (only for listing), `limit: 1000`, `cursor` |
| `create` | through the hub | `k_playbook_doc_create` | `collection`, `name`, `content: ""` |
| `rename` | through the hub | `k_playbook_doc_done`, `k_playbook_doc_reopen` | `collection`, `name`, `new_name` |

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
  maps the rest to its own codes (`kephalaion_unavailable` when the node cannot be
  reached, `kephalaion_error` otherwise).

Limits on the k-playbook side: 15 s per call including the session, at most 16 MiB per
answer, pages of 1000 entries and at most 100 pages for one listing.

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

Not measured: a node that is cut off from its hub, and a second node listing what the
first one wrote. Commands and output are in
`k-playbook-local/material/befunde/ablage-userbereich-queue.md`.

## Where the tools are documented

In the Kephalaion repository (`github.com/kephalaion/kephalaion`):

- `docs/konzept.md`, sections "Werkzeuge" → "Allgemein — lesen" and "Allgemein —
  schreiben" — the MCP tools with their arguments and answers.
- `docs/vertrag.md`, sections "Schreibvorgänge" (`create`, `rename`, "Antwort") and
  "Fehler" — the contract between node and hub, including `name_taken`, tombstones and
  how the node takes the hub's rows into its replica.

The tool descriptions a node reports over MCP (`tools/list`) are the short form of the
same.
