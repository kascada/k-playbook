# The MCP Server

k-playbook includes an MCP server. It gives an assistant the same information that
`k-playbook context` provides on the command line: the resolved paths, the instruction
files in reading order, the remediation policy, the guidelines, and the effective catalogs
for rules, reviews, and checks -- shipped and project-owned content already merged.

`k_playbook_context` is the only tool that also writes as a side effect: it synchronizes the
project's assistant linking with the catalog, just as the subcommand does. It writes only
when writing changes something, and only to symlinks created by k-playbook itself. The
response lists what happened under `links`; if the field is absent, everything was already
up to date.

The server offers the working state and the review, todo and knowledge tools.
`k_playbook_context` has the optional `dir` parameter; all other tools require `projectDir`,
because a stdio server must not assume that its process working directory is the target
project. There is no fallback to that directory.

`projectDir` is required by contract but optional in the input schema, and its description
starts with `Pflicht.`. The reason is the answer a caller gets: a field marked required in the
schema is rejected by the SDK's schema validation before the tool runs, with a bare text
(`validating "arguments": validating root: required: missing properties: ["projectDir"]`) --
no envelope, no code, nothing that says the call did nothing. That is how a `knowledge_write`
got lost once. The tools therefore check the parameter themselves: missing, empty, or only
whitespace yields the family's envelope with `ok: false`, code `invalid_input`, and the same
message in all three families -- `projectDir fehlt — nichts ausgeführt. Wiederhole den Aufruf
mit projectDir, dem Projektverzeichnis mit K-PLAYBOOK.yaml (absoluter Pfad).` A `projectDir`
that is given but leads to no project stays `project_not_found`. The price: clients that check
required fields before calling (Claude Code) no longer do so for `projectDir`. Other schema
violations -- another required field missing, a wrong type, also together with a missing
`projectDir` -- are still rejected by the SDK with its bare text.

The internal view -- protocol versions, the stdout rule, session termination -- is in
[`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server).

## Registering

The interface provides the **k-playbook MCP** block for this. Clicking *Set up* registers
the server in all three files:

| Assistant | File | Entry |
|---|---|---|
| Claude Code | `.mcp.json` in the project root | `mcpServers` -> `k-playbook` |
| Cursor | `.cursor/mcp.json` | same schema, same key |
| OpenCode | `opencode.json`, or `opencode.jsonc` if only that one exists | `mcp` -> `k-playbook` |

*Set up* always registers the same command, in two spellings -- one per schema -- as the
absolute path of the installed `k-playbook`, resolved when writing:

```json
{
  "mcpServers": {
    "k-playbook": {
      "command": "/home/wer/.local/bin/k-playbook",
      "args": ["mcp"]
    }
  }
}
```

```json
{
  "mcp": {
    "k-playbook": {
      "type": "local",
      "command": ["/home/wer/.local/bin/k-playbook", "mcp"],
      "enabled": true
    }
  }
}
```

For OpenCode, `command` is an **array** of command and arguments, not two fields.

The three files belong to the project and can contain third-party entries; the OpenCode
configuration also contains entirely different settings. Therefore, exactly the
`k-playbook` key is touched, while everything else remains unchanged -- **including
comments and trailing commas**. Files are read in JWCC format (JSON with commas and
comments), and an existing file is not rewritten from its content; only that one key is
patched. Third-party entries, their order, and comments remain unchanged.

One side effect remains visible: after patching, the file is **reindented** once using tabs.
For a committed configuration with space indentation, this creates a diff across the whole
file. This happens once, during the first setup -- once the entry is correct, nothing is
written at all.

**Two extensions, one file.** OpenCode reads `opencode.json` and `opencode.jsonc` and deeply
merges both when they exist side by side. Therefore, setup never creates a second file:
`opencode.jsonc` is the target when it exists and `opencode.json` is absent -- otherwise,
the target is `opencode.json`. If both really exist, only `opencode.json` is maintained, and
the interface reports "two configurations" instead of "registered": what wins during the
merge cannot be seen from outside, and one of the two must be resolved.

If the OpenCode configuration is created **new**, the schema reference
`"$schema": "https://opencode.ai/config.json"` is written with it. Otherwise, OpenCode adds
it itself at the next start and rewrites the file for that purpose; with the entry, the file
remains as setup left it. It is **not** added to an already existing file -- that file belongs
to the project.

The `k-playbook` key belongs to k-playbook. If it contains a third-party command, that is not
a conflict but an incorrect state: the interface reports "points elsewhere" and overwrites
it during setup. What counts as a correct state is not a single value but a set of forms --
see [Why the Entry Is an Absolute Path](#why-the-entry-is-an-absolute-path). Only a file that
cannot be read even as JWCC -- a missing bracket, a fragment -- is reported and **not**
touched. Comments alone are not such a case.

The registration *Set up* writes is an absolute path and is therefore tied to an environment;
it cannot be committed as-is. To share it, enter the [portable form](#the-portable-form-the-bare-command-name)
manually -- the bare command name. What is there is not touched by automatic correction as
long as it belongs to the set of accepted forms. k-playbook writes the bare name itself in
one case only: when automatic correction replaces an obsolete entry in a file tracked by git
-- see [Existing Projects Synchronize Automatically](#existing-projects-synchronize-automatically).

## Approval and Restart

Written is not the same as available. The assistant reads its configuration at startup, so it
must be restarted once after setup.

Claude Code additionally requires **approval**: project-specific servers from a `.mcp.json`
apply only after explicit consent in an interactive session. The question appears once at the
next startup. Since v2.1.196, Workspace Trust is also required; a freshly cloned project
cannot approve its own servers.

## Why the Entry Is an Absolute Path

The value *Set up* registers is the **absolute path of the installed binary resolved when
writing**, typically `~/.local/bin/k-playbook`, expanded. It is neither the bare command name
nor the project-owned wrapper used previously. The one exception is automatic correction of
an obsolete entry in a tracked file: it writes the bare name
([below](#the-portable-form-the-bare-command-name)).

The reason is the case in which the entry is needed. A client started from the Dock or Finder
-- Cursor, VS Code, Claude Desktop -- does **not** inherit the PATH of a login shell;
`~/.local/bin` is typically absent. A bare `k-playbook` would be unavailable precisely in
those environments, while the same entry works from a terminal. An absolute path depends on
no inherited environment.

### A Set of Accepted Forms, Not a Target Value

Each write puts exactly one form into a file, chosen by a fixed rule: the absolute path,
except when automatic correction replaces an obsolete entry in a tracked file -- then the bare
name. **Validation** checks against a set:

| Form | Counts as |
|---|---|
| any absolute path whose filename is `k-playbook` | current |
| the same path from another `$HOME` | current |
| the bare command name `k-playbook` -- the portable form | current |
| `k-playbook/bin/k-playbook`, `bin/k-playbook`, and any path ending in either | obsolete |
| anything else, including `./k-playbook` | incorrect state, reported |

Comparing for equality with a single target value would declare every other valid spelling
incorrect and rewrite it on every run. Two cases make this concrete: a committed
registration cannot contain an absolute home path, and the host and Dev Container have
different HOMEs.

"Obsolete" is therefore defined **narrowly**: the old wrapper path, and nothing else. Only
it is overwritten automatically. An entry that neither fits the set nor is the old wrapper
is reported and overwritten only when *Set up* is clicked explicitly.

### The Portable Form: the Bare Command Name

An absolute path contains a `$HOME` and is therefore tied to an environment. Anyone who
wants to **commit** their registration cannot use it: the same file applies on the host and
in the Dev Container, whose HOMEs differ. The bare name is provided for this purpose:

```json
{
  "mcpServers": {
    "k-playbook": {
      "command": "k-playbook",
      "args": ["mcp"]
    }
  }
}
```

It is the only spelling that names no environment at all. Each environment resolves it via
its own PATH to its own binary -- exactly what bootstrap ensures: `~/.local/bin` **must** be
in PATH, or `bin/install` aborts. For the same reason, host and container do not play
ping-pong with this file: neither finds anything in it to correct.

*Set up* never writes it, and neither does adding a missing entry or creating a missing file
-- all of them write the resolved absolute path. k-playbook writes it in exactly one case:
when automatic correction replaces an obsolete wrapper entry in a file git tracks, or where
that question cannot be answered ([details](#existing-projects-synchronize-automatically)).
The old relative wrapper entry named no environment, so the bare name is its faithful
translation. Otherwise the portable form is entered manually in the file and then remains
there. Exactly the name is accepted -- `./k-playbook` and every path ending in the name are
not.

### Three Explicit Boundaries

**The portable form does not cover the Dock/Finder case.** That is the cost, and it is stated
here rather than concealed: a client started from the Dock or Finder -- Cursor, VS Code,
Claude Desktop -- does not inherit the login-shell PATH. It finds nothing under a bare name,
and the server remains unavailable. This exact case is why *Set up* writes the absolute path.
It applies to a bare name entered manually and to one written by automatic correction in a
tracked file alike.

Anyone sharing a committed registration shares a form that works from the terminal and in a
Dev Container, but not from the Dock. *Set up* does not help here: the bare name belongs to
the accepted forms, so *Set up* leaves it in place. The only exception is an OpenCode
configuration without the memory block (`instructions` with `AGENTS.md`, `references.docs`):
there *Set up* rewrites the entry and writes the absolute path with it. The way out is to
**enter the absolute path manually** in that environment. It is an accepted form as well, so
neither automatic correction nor *Set up* replaces it -- in a repository that tracks the
file, that is a local diff that must not be committed.

The `/mcp` self-test also does not cover the portable form: it starts what *Set up* would
write -- the resolved absolute path -- rather than what is in the file. It therefore answers
"does the installed binary respond?", not "does the client find the entry?".


**An absolute path from a foreign `$HOME` counts as current.** This is a decision, not an
oversight: otherwise, host and Dev Container would mutually declare the shared file obsolete
and rewrite it on every switch. The cost: if host and container share the same repository but
have separate HOMEs, MCP remains unavailable in the other environment without automatic
correction intervening. Then only the `/mcp` self-test reports a problem. *Set up* does not
replace a foreign absolute path either; anyone working in this situation enters the portable
form manually.

The same applies to a file that host and Dev Container **share without git tracking it** --
through a bind mount, for example. Automatic correction then writes the absolute path of the
environment it runs in, and the other environment stays without MCP, because the foreign
path counts as current. The bare name is written automatically only into tracked files:
"tracked" means committed, not shared. This boundary is named, not solved.

**The server finds the project through its working directory.** The registered path says
which binary starts -- not which project is intended. At runtime, the server resolves this by
searching upward for `K-PLAYBOOK.yaml`, beginning in its working directory. That is the
assistant's working directory. Anyone opening the assistant in a subdirectory -- for example
in the code repository, which may sit beside the playbook according to
[`installation.md`](./installation.md#1-create-configuration) -- gets a server that does
not find their project. The interface states the condition in the block and clearly points it
out if it itself was not started in the project root.

## Existing Projects Synchronize Automatically

A project whose registration still names the retired wrapper is corrected without manual
work in two places:

- during a **clone update** through the interface. `git pull` alone does not reach the files:
  they are in the project root, not in `k-playbook/`.
- at **every start** of `k-playbook`. This is the fallback path for everything that bypasses
  it -- a manual `git pull` or `make -C k-playbook installer-update`.

Clicking *Set up* is not required for this; it remains the explicit write path alongside it.

This correction is **narrow and idempotent**. Replacing the obsolete entry is a repair of
content k-playbook wrote itself, so it runs regardless of whether the file is committed;
only the form written depends on it (see below).
No accepted form is touched, and an entry that is neither obsolete nor accepted (`stale`)
is left alone -- it may come from a foreign `$HOME` and be valid there. Nothing is deleted.

The two automatic paths differ in how far they go. The clone update replaces the obsolete
entry and nothing else. The start goes further: it also **adds a missing entry and creates
a missing file -- but only when the target file is not tracked by git.** That is the
condition that keeps a clone's working tree clean, and it is measured, not guessed,
in the project root: `K-PLAYBOOK.yaml` unreadable counts as tracked (the question is
unanswered, so nothing is written); `project.vcs` other than `git` means nothing is
tracked; if `git rev-parse --show-toplevel` fails, nothing is tracked only when git
explicitly reports "not a git repository (or any of the parent directories)" or "(or any
parent up to mount point …)" **and** no `.git` entry exists in the project root or above
it, along the given path and along the resolved one; any other failure -- "dubious
ownership" in a repository owned by someone else, an orphaned `.git` file, a `.git`
directory without `HEAD`, a timeout or a git that could not run -- counts as tracked. The
call runs with `LC_ALL=C` so the wording stays comparable. After a successful `rev-parse`,
`git ls-files --error-unmatch` decides -- exit 0 tracked, exit 1 not tracked, anything
else counts as tracked. The start also adds nothing and creates nothing when the path to
the target file **goes through a symlink** -- the file itself (even a dangling link) or
the assistant directory. Writing would land in the link target while the measurement saw
the link, and a link is a deliberate setup of the project, so it is skipped rather than
resolved. Only the components below the project root count; a project that lives under a
symlinked path is not affected. In both cases -- symlink and a repository git refuses to
trust -- only *Set up* writes; an obsolete entry is still corrected. Hard links are not
detected and remain a known gap. A
missing file is additionally created only if the assistant has left a **trace of its own**
in the project: an entry in `.claude/`, `.cursor/` or `.opencode/` that is not one of the
symlink targets k-playbook manages itself -- for example `.claude/settings.json` or
`.cursor/rules/`. The directories alone are no trace; k-playbook creates them in every
project. Without that condition every start would create three files nobody needs.

Which form is written depends on the case. **Adding** a missing entry and **creating** a
missing file write what *Set up* writes: the absolute path. The start message says so -- the
file carries a machine-specific path and should stay unversioned. Whether the project
commits it anyway is the project's decision; from then on the file is tracked and the
automatic path adds nothing to it any more.

**Replacing** an obsolete entry follows the same tracking measurement, per target file and in
both automatic paths: a file git **tracks** -- or one for which the question stays
unanswered, such as with an unreadable `K-PLAYBOOK.yaml` -- gets the bare name `k-playbook`;
a file that is **not tracked** gets the absolute path. The old relative wrapper entry named no
environment, and a committed replacement must not name one either. The absolute legacy path
`…/k-playbook/bin/k-playbook` follows the same rule. A later start writes nothing, neither on
the host nor in a second environment, because both forms are accepted. The measurement runs
only when an obsolete entry is actually replaced.

Behind a **symlink**, the question is answered for the file that is actually written: the
link target. That rule applies only after the chain above has established that the project
uses git and its root lies in a repository -- with `project.vcs` other than `git`, a file
behind a link stays untracked and gets the absolute path. The link target is resolved and
compared physically with the physical repository root, since the project root itself may be
reached through a symlinked path. A target in the same repository is measured with
`git ls-files`; a target that is not tracked there gets the absolute path. A target outside
that repository -- in no repository or in a nested one -- or one that cannot be resolved
leaves the question unanswered and gets the bare name.

When the bare name was written, the start message and the clone-update response say so and
name its cost: a client launched from the Dock or Finder needs the absolute path, entered
manually.

**Projects already migrated and committed stay as they are.** Earlier versions wrote the
absolute path into tracked files as well. A path from a foreign `$HOME` counts as current and
is not replaced again. Affected projects enter the bare name `k-playbook` manually in
`.mcp.json`, `.cursor/mcp.json` and `opencode.json`; automatic correction leaves it there.

The prerequisite remains the one-time bootstrap per host or Dev Container --
`make -C k-playbook install`, or without make `k-playbook/bin/install`: both correction paths
run in the installed binary. Without an installed `k-playbook`, no path can be registered --
then nothing is written and the interface says so.

## Tools

| Tool | Purpose |
|---|---|
| `k_playbook_context` | return the resolved working state like `k-playbook context` and synchronize assistant linking |
| `k_playbook_review_status` | read the selection basis for new runs or the status of an existing run |
| `k_playbook_review_create` | create a review run or return validated `run.json` in a dry run |
| `k_playbook_review_scan` | execute a run's tool entries through `review.Execute` |
| `k_playbook_review_merge` | consolidate a run through `merge.Run` into `review-input.json` and `review-input.md` |
| `k_playbook_review_write_ai_entry` | write the status and result of an AI review entry |
| `k_playbook_todo_list` | list the project todos; `includeDone` adds the completed ones |
| `k_playbook_todo_add` | add a todo and assign the next id |
| `k_playbook_todo_update` | change the text, tick a todo off, or reopen it |
| `k_playbook_todo_delete` | remove a todo permanently |
| `k_playbook_knowledge_search` | search the knowledge store `k-playbook-local/knowledge/` section by section; hits carry `path`, `heading`, `excerpt`, `kind`, `origin`, `state`, `rank`, `anchor`; documents in state `raw` or `superseded` and the root `README.md` are not hits |
| `k_playbook_knowledge_list` | list the files of the store with `path`, `title` (frontmatter `title`, else the first heading, else the file name), `kind`, `origin`, `state`; the README comes first, raw and superseded documents included |
| `k_playbook_knowledge_read` | read one file of the store as Markdown |
| `k_playbook_knowledge_write` | write one document: `producer` from the closed list, `path` inside the producer's directory, the frontmatter as fields (`title`, `subject`, `origin`, `state`, `format`, `sources`), `body` without a header, optional `queue` entry that is deleted once the document exists |
| `k_playbook_knowledge_publish` | a generator (`docs-code`, `docs-tools`, `inventory`) hands over its complete `documents` and the store swaps the directory atomically; reports how many were written and removed |
| `k_playbook_knowledge_supersede` | set `state: superseded` on a document, record `successor` and `superseded_reason`; nothing is deleted |
| `k_playbook_knowledge_inbox_put` | drop a raw piece into `k-playbook-local/inbox/<source>/<name>` from `content` or `file`, with an optional `note` stored beside it |
| `k_playbook_knowledge_inbox_list` | list the raw pieces of the inbox with `path`, `source`, `name`, `format`, `size`, `modified`, `note` |
| `k_playbook_knowledge_inbox_read` | read one raw piece as text; text formats only |
| `k_playbook_knowledge_queue_add` | add a queue entry from `origin`, `target`, `reason`; the tool assigns the `id` |
| `k_playbook_knowledge_queue_list` | list the backlog, oldest first; empty means nothing outstanding |
| `k_playbook_knowledge_queue_drop` | delete a queue entry without a takeover; the `reason` is not recorded |
| `k_playbook_knowledge_status` | report kind and size of the search index: `indexKind`, `model`, `dims`, `fileCount`, `chunkCount`, `byKind`, `builtAt`, `indexVersion`, `stale`, `staleFiles` |

There is deliberately no `k_playbook_review_next_steps` tool yet. The orchestrating command
reads the status and makes its own decision from it.

### Review Response Contract

All review tools return the same envelope. Success:

```json
{
  "ok": true,
  "tool": "k_playbook_review_status",
  "project": {
    "inputDir": "/provided/path",
    "root": "/project",
    "playbookDir": "/project/k-playbook",
    "localDir": "/project/k-playbook-local",
    "reviewRunsDir": "/project/k-playbook-local/results",
    "languages": ["go"]
  },
  "data": {},
  "warnings": []
}
```

Domain error:

```json
{
  "ok": false,
  "tool": "k_playbook_review_status",
  "project": { "inputDir": "/provided/path" },
  "error": {
    "code": "project_not_found",
    "message": "No k-playbook project found.",
    "details": {}
  },
  "warnings": []
}
```

Domain errors remain tool results with `ok: false`; the MCP server remains available for the
next call. MCP protocol errors are reserved for malformed JSON-RPC/MCP messages. Error codes
are stable and in `snake_case`, including `invalid_input`, `project_not_found`, `run_not_found`, `run_exists`,
`invalid_mode`, `invalid_selection`, `selection_unknown`, `selection_unavailable`,
`entry_not_found`, `entry_kind_invalid`, `entry_state_invalid`, `result_required`,
`result_path_invalid`, `read_failed`, `write_failed`, `preflight_failed`, `execution_failed`,
and `merge_failed`.

`invalid_input` is the code for a missing, empty, or whitespace-only `projectDir`, with empty
`details`; nothing was executed, and the call is to be repeated with `projectDir`.
`project_not_found` is reserved for a `projectDir` that leads to no k-playbook project, a
missing installation, or an unreadable working directory when resolving a relative path.

The evidence-mode codes are also provided, all from `k_playbook_review_write_ai_entry`:
`entry_job_invalid` (a job on a perspective or on a report that is not `done`),
`entry_result_invalid` (`result` on an evidence entry, which has no result file),
`sarif_required` (`done` without a job), `sarif_path_invalid` (path outside `raw/`, missing
file, or empty file), and `recipe_contract_invalid` (the recipe no longer fulfills the
evidence contract, so the rule-ID list cannot be validated). They are all **invocation
errors**: the tool writes nothing, and the recipe run need not be repeated. An invalid
**artifact** -- unreadable SARIF, an incorrect `tool.driver.name`, an unknown rule ID -- is,
by contrast, not an error code, but a written entry status of `failed` with a reason and
`stateOverridden: true` in the response.

### Selection Validation

`k_playbook_review_status` in `available` mode and `k_playbook_review_create` use the same
selection basis. Candidates include at least `name`, `kind`, `title`, `selectable`,
`defaultSelected`, and `unavailableReason`.

Tool candidates come from the tool matrix and the preflight, filtered by `project.languages`.
Tools that are not installed or do not match the language remain visible, but are not
`defaultSelected`. AI candidates come from the effective review catalog; disabled local
review files and recipes with `audit.enabled: false` are absent. A recipe with a
contradictory `audit` block -- for example, `mode: evidence` without `ruleIds` or with
`scope.tools` -- is not adjusted: it appears under `unavailableCandidates` with
`selectable: false` and an `unavailableReason` that names the violated rule.

AI candidates additionally carry `mode`. Alongside them, the selection basis provides
`evidenceCandidates` and `perspectiveCandidates`, or `evidenceEntries` and
`perspectiveEntries` in an existing run -- the run's order must be apparent from the output
without a command deriving it again from the recipes.

Without `entries`, `k_playbook_review_create` selects every candidate with
`defaultSelected: true`. Unknown names produce `selection_unknown`, explicitly requested
unselectable candidates produce `selection_unavailable`, and duplicate or incorrectly typed
entries produce `invalid_selection`.

### Scan Semantics

`k_playbook_review_scan` does not shell out to the `k-playbook` CLI. It calls the Go domain
logic `review.Execute` directly; only it may start the configured external scanner binaries.
MCP input contains no shell commands and cannot override scanner command lines.

A scanner failure is normally an entry status in `data.entries[]`; the tool call remains
`ok: true` when run, selection, and status files were read and written consistently.
`ok: false` is for orchestration errors such as missing runs, invalid selection, unreadable
run files, unwritable entry files, or a global preflight failure.

### AI Recipe Metadata

Review recipes can contain separate `audit`/`review` frontmatter at the beginning of the
file:

```yaml
---
audit:
  enabled: true
  mode: perspective
  title: "Secret-Scanning Assessment"
  resultRequired: true
  defaultResult: "review-secret-scanning.md"
  scope:
    tools: [gitleaks, trufflehog]
review:
  enabled: true
---
```

`audit.mode` determines which of the remaining fields apply. Without a value, it is
`perspective`. For `mode: evidence`, `ruleIds` and `scope.paths` replace `scope.tools`,
`resultRequired`, and `defaultResult`:

```yaml
---
audit:
  enabled: true
  mode: evidence
  title: "Tech Debt Analysis"
  ruleIds: [tech-swallowed-error, tech-duplicated-logic]
  scope:
    paths: ["**/*.go", "**/*.py"]
review:
  enabled: true
---
```

The MCP tools continue to be called `k_playbook_review_*` because they form the technical
review-catalog/run API. The visible user role of the complete sweep is nevertheless
`/k-audit`.

When creating a run, `recipeKey`, `recipePath`, `recipeOrigin`, `title`, `mode`,
`resultRequired`, `defaultResult`, and `scope` are copied into `run.json`. The write tool for
AI entries later validates against this copy, not against recipe text that may have changed.
`scope.tools` and `scope.paths` are therefore a snapshot: existing runs retain their scope
even if the recipe changes later. `mode` is written explicitly, including for `perspective`,
so the run itself says how an entry was intended; an empty field in an old run is read as
`perspective`.

For `mode: evidence`, `resultRequired` is forced to `false` -- the required artifact is
`raw/<entry>.sarif`, and with `true`, `k_playbook_review_status` would report every
successful evidence entry as `resultMissing` and `inconsistent`.

`ruleIds` is the one exception to the snapshot rule: the list is **not** copied, but read
fresh from the recipe when reporting. It is the recipe's contract, not a run definition --
anyone changing it changes it for all runs, and a recipe that no longer fulfills its evidence
contract should be detected when reporting rather than pass with an incomplete check.

For an evidence entry, `k_playbook_review_write_ai_entry` additionally accepts `job` with
`sarif` (relative to the run directory, below `raw/`) and optional `started` and `finished`.
State, finding count, and job name are created when reporting. Under `evidence`, the response
contains the number of accepted findings, those discarded outside the scope along with their
first paths, and whether the SARIF was written back sanitized.

### Timeouts and Progress Notifications

`k_playbook_review_scan` runs for several minutes in real runs. During execution, the server
sends MCP progress notifications to the calling client provided its `CallTool` call includes
a `progressToken`. Without a token, the server remains silent; the final response is the same
in both cases.

A progress event is sent on every state change of a job or entry and additionally as a
heartbeat at least every 15 seconds, so even a single long-running scanner can be shown to be
alive. A one-second debounce prevents a fast scanner from producing multiple events.

What the MCP specification guarantees: the standard `notifications/progress` format with
`progressToken`, `progress`, `total`, and an optional `message`.

What the MCP specification **does not** guarantee: that a client resets its request timeout
when receiving such a notification. That is client behavior and must be verified for each
target client.

For **OpenCode**, the behavior was verified on 2026-08-22 with the server shipped in this
version: a 158-second scan run completed in an OpenCode client with
`mcp.k-playbook.timeout: 90000` (90 seconds), without aborting the tool call; no scanner
persisted `reason: "cancelled"`. The evidence is based on persisted entry states
(`k-playbook-local/results/<run>/entries/*.json`).

The value belongs in the client configuration and is not written by k-playbook. This
repository's committed `opencode.json` does **not** set it; it contains only the registration
itself. A manually added `timeout` survives automatic correction, however --
`mcpEntryCommand()` evaluates only command and arguments and leaves additional keys intact.

For clients with `progressToken` support and a verified timeout reset, a moderate request
timeout (60-90 seconds) is sensible: it limits a genuinely hung server to a noticeable wait
time without killing a normal long run.

For clients **without** `progressToken`, there is no alive guarantee. The server sends them
no notifications, and their timeout expires without warning. Recommendations:

- higher request timeout (five to ten minutes depending on the scanner set), or
- the CLI path (`k-playbook scan <run>`), which is decoupled from client timeouts, or
- a future background-scan/polling solution.

An explicit client cancellation after scanning starts still aborts the scanners hard; the
last scanner that was running persists `reason: "cancelled"`. Progress notifications do not
change this -- they address only the timeout failure mode, not cancellation.

### Todo Contract and Migration

The four todo tools are thin wrappers. The load-bearing layer is the subcommand
`k-playbook todo`: the MCP server is registered **per project** (see "Registering"), so a
project without that entry does not have the tools -- while the binary is there with the
installation. A missing tool never even appears at the client and cannot be reported; a binary
that is too old answers `k-playbook todo` with `unbekanntes Kommando`, which is detectable. The
interface, the subcommand, and the tools all call the same functions in `internal/project`;
none of the three touches the file itself.

The store is `k-playbook-local/data/todos.json`:

```json
{
  "schemaVersion": 1,
  "nextId": 14,
  "migratedOn": "2026-09-08",
  "todos": [
    { "id": 3, "text": "...", "created": "2026-08-25", "done": "2026-09-08", "origin": "Task 026" },
    { "id": 4, "text": "...", "created": "", "done": "2026-09-08", "doneMigrated": true }
  ]
}
```

`id` is never reused; `nextId` guarantees that even after a delete. `done` is absent on open
entries -- there is no separate status field. `doneMigrated` marks a `done` that came from a
migration or an import, so it is not mistaken for the day someone ticked the entry off.
`migratedOn` is the provenance note of the **document**, not of a single entry.

An older `k-playbook-local/TODO.md` is translated on the first access -- through any of the three
layers, reading or writing -- and removed afterward. The response then carries
`migrated: <count>`. Text, order, and done state are preserved; timestamps are not invented, so
`created` stays empty for translated entries. If both files exist, access does not fail: the JSON
document is read and written, and the leftover file is reported in `hint`. Only the migration
itself refuses, and its message names the way out: `k-playbook todo import
k-playbook-local/TODO.md` appends the Markdown entries to the existing document and removes the
file.

Ticking off keeps the entry, deleting removes it -- two separate tools, so a model cannot throw
away history by accident.

The envelope is `{ok, tool, projectDir, ...}` with an `error` of `code` and `message` on
failure. A missing, empty, or whitespace-only `projectDir` is `invalid_input` and nothing is
written; a `projectDir` that leads to no project is `project_not_found`.

### Knowledge Contract

The knowledge tools follow the same pattern: thin wrappers over `project.Knowledge`, the one
place that chunks, indexes, searches and writes; the load-bearing layer is the subcommand
`k-playbook knowledge`, and a binary that is too old answers it with `unbekanntes Kommando`. The
envelope is `{ok, tool, projectDir, ...}` with an `error` of `code` and `message` on failure.
The store is the zone `k-playbook-local/knowledge/`; `inbox/` and `queue/` lie beside it and
are never indexed. [knowledge-layout.md](knowledge-layout.md) defines the zones, the owner per
directory and the frontmatter; this section only fixes what the tools return.

**Reading.** A search hit names `path`, `heading` (the heading verbatim, the leading field),
`excerpt` (the start of the section, at most 400 characters), `kind` (the directory under
`knowledge/`, which is the owner: `code`, `libs`, `versions`, `extracted`, `external`,
`findings`, `pitfalls`, `manual`, or `root` for a flat file), `origin` and `state` from the
document's frontmatter, `rank` (from 1; the order is the statement, there is no score) and
`anchor` (the Goldmark heading id, a display aid). `kind` is read from the path and never from
the document; `origin` is the actual provenance and lives only in the frontmatter -- the two
were one field called `source` in v0.7.0 and were split before anything consumed them.
Documents in state `raw` or `superseded` are not hits, and neither is the root `README.md`,
which is generated navigation whose keyword index would otherwise outrank the documents it
points at; `list` carries all of them with their `state`, and `read` returns any of them.
`read` only returns what the index can see: its path is checked like a write path -- relative,
inside the zone, no hidden segment, a Markdown file -- and a path through a symbolically
linked directory, which the index does not descend into, is refused as `invalid_input`; a
linked file is indexed and readable. Like every access, `read` first puts an orphaned
`.<dir>-alt-*` of an interrupted `publish` back, and its result reports that in `hint` (the
subcommand on stderr). There is no filter on `state`; that belongs to the reading
side. The `title` that `list`
reports is the frontmatter `title` when the document has one, otherwise its first heading,
otherwise the file name -- what a caller had to give `write` comes back on reading, and a
file without a header still gets a readable name. `search` always carries
`hits` and `list` always carries `entries`, empty ones included -- the same as the `--json`
output of the subcommand. The other tools omit both keys rather than sending `null`.
When the store has no section that could be a hit at all -- no chunk outside the root
`README.md`, and none from a document in state `raw` or `superseded` --, `search` says so in
`hint` (the subcommand on stderr, its stdout stays `Keine Treffer …` or the plain JSON), whatever
the query and the `kind` filter: the store has nothing searchable yet, and project knowledge
lies under `k-playbook-local/docs/` until the migration. Without it an empty store would read
like "nothing on this topic". A store with sections that the query does not hit carries no
such hint. The pointer at `docs/` is a transitional read and goes in step 6 of the switch of
the writers ([knowledge-layout.md](knowledge-layout.md#transitional-reads)).

**Writing.** Every write names its `producer` from the closed list in the layout, and the
`path` -- relative to `knowledge/`, reported back exactly as `read`, `list` and `search`
name it -- must lie in that producer's directory. Three refusals, three messages: an unknown
producer, a path that leads out of the zone, and a target inside the zone but outside the
producer's directory. The frontmatter is composed by the tool from `title`, `subject`,
`origin`, `state` (`raw`, `condensed`, `reviewed`; `superseded` is refused), optional
`format` (`markdown` by default, or `text`, `html`, `image`, `pdf`) and `sources`; `updated`
is set by the tool. The `body` is Markdown without a header, and a body that carries one is
refused rather than passed through -- also when blank lines, leading whitespace or CRLF line
endings come before it. `write` optionally names a `queue` entry, which is
deleted after the document exists and the index knows it -- not before, and not if the write
failed. `write` does not overwrite a document whose file carries `state: superseded`: the call
is refused before anything is written, and the file, the index and a named queue entry stay as
they are. A later run of the same producer would otherwise reset the state silently and lose
`successor`; whoever wants to change the topic writes the successor or supersedes it.
A `write` path through a symbolically linked directory is refused as `invalid_input` before
anything is read, created or written: the file would lie where the index does not look.
`publish` is not affected; its swap replaces a linked directory with a real one.
The generators `docs-code`, `docs-tools` and `inventory` cannot `write`: they
`publish` their complete set of `documents`, and the tool builds the new directory beside the
old one, writes the index that describes it and only then swaps it, so a run that dies halfway
leaves the previous state untouched; the result says how many were `written` and `removed`.
The paths of `documents` are relative to the generator's directory: a path that already starts with it, in any case (`code/overview.md` or `Code/overview.md` for
`docs-code`), is refused as `invalid_input`, on the command line
as well, instead of landing in `code/code/` -- a generator directory therefore holds no
subdirectory of its own name. If a swap is interrupted between its two renames, the next access -- `read` included -- puts the set-aside directory `.<dir>-alt-*` back under its name before drift detection runs,
provided the target is missing and exactly one candidate lies there; it never replaces an
existing target. [knowledge-layout.md](knowledge-layout.md#the-write-tools) spells out every
failure case.
An empty set -- `documents: []` or the field left out -- is refused as `invalid_input` before
anything is created: `publish` never empties a directory, on either path.
On the command line, `publish --from <dir>` reads every Markdown file below `<dir>` with the
same fields in its frontmatter, checks them and recomposes the header -- nothing is copied.
`supersede` sets `state: superseded`, `successor` and `superseded_reason`, refreshes
`updated` and leaves the body; the successor must already exist in the store and must be able to be a search hit: it is
refused exactly when search hides it by its state (`raw`, `superseded`), so a successor without a
header or without `state` is allowed. Refused as `invalid_input`: a document or a successor
under `code/`, `libs/` or `versions/` and the root `README.md` on either side -- all recognised
without regard to case (`Code/x.md`, `readme.md`) --, a document that is already superseded, a
successor that search hides, and a document or successor through a linked directory. The result names both
paths cleaned: `./findings/y.md` comes back as `findings/y.md`. A supersession is final; the
rules and the way to correct a wrong successor are in
[knowledge-layout.md](knowledge-layout.md#superseding).

**Inbox and queue.** `inbox_put` stores a raw piece under `inbox/<source>/<name>` as it is
-- any format, no frontmatter, no index -- and refuses an occupied name as well as a path on
which a file lies where a directory is needed (`README.md` as `source`, an existing piece as an
intermediate directory in `name`); a `note` is stored
beside it as `<name>.note` and shown by `inbox_list` at the piece's entry. `inbox_read` reads
text formats only (`md`, `markdown`, `txt`, `html`, `htm`, `json`, `yaml`, `yml`, `csv`, `xml`, `log`).
`queue_add` takes `origin`, `target` (a directory relative to `knowledge/`, checked for path
safety only) and `reason` and assigns the `id` itself; `queue_list` returns the backlog with
those fields plus `added` and any `notes`; `queue_drop` deletes an entry and records nothing.

**Error codes.** The `code` says whether the arguments or the environment are wrong, so a
caller can decide between correcting and giving up. `invalid_input` is reserved for input
errors, which the core (`project.InputError`) distinguishes and the wrappers only relay: an
unknown producer, a path out of the zone or outside the producer's directory, a missing or
malformed field, a body with a header, a refused `state`, an empty `documents` set, a
non-text format at `inbox_read`, an occupied inbox name or a file on its path, a `publish` path that starts with the generator's directory (in any case), a `supersede` target or
successor in a generator directory or the root `README.md` (in any case), an already superseded
target, a successor that search hides by its state, a `write` onto a superseded document, a
`read` path with a hidden segment, a `read`, `write` or `supersede` path through a linked
directory -- and "not there": a missing path
at `read` or `supersede`, a missing successor at `supersede`, an unknown queue `id` at
`queue_drop` or `write`, a missing inbox path at `inbox_read`. The caller named something
that does not exist and can correct it; no separate code. Everything else is the
environment: the writing tools (`write`, `publish`, `supersede`, `inbox_put`, `queue_add`,
`queue_drop`) answer `write_failed`, the reading tools (`read`, `search`, `list`,
`inbox_read`, `inbox_list`, `queue_list`, `status`) answer `read_failed` -- for instance an
unwritable `knowledge/`, or a path that exists but cannot be read. A missing, empty, or
whitespace-only `projectDir` is `invalid_input` as well, before anything is resolved or
written (see the top of this page). `project_not_found` stays the code for a `projectDir` that
is given but leads to no k-playbook project.

`status` reports `stale: true` and `staleFiles` when the last access found files changed
behind the tools' back and re-read them. The same report can mean an interrupted or a
concurrently running `publish`: its index is written before its swap, so for that moment, or
after a crash in it, the index describes a set the disk does not hold, and the access returns
to what is on disk -- although nobody wrote past the gate. A `hint` appears -- at `read` as well -- when an access had to skip something without failing over it: an unwritable `cache/`, an unreadable file, a
set-aside directory put back after an interrupted swap -- and at `search` when the store has nothing searchable yet (see "Reading"). `status` also names hidden leftovers of
a swap (`.<dir>-neu-*`, `.<dir>-alt-*`), which are never deleted automatically, and Markdown
files at the old location `k-playbook-local/docs/learned/`, which the store no longer reads. Unreadable is
not drift: a file whose hash cannot be taken is reported in the `hint`, its index entry is
dropped once, and it stays invisible to the comparison -- no `stale`, no rewrite of the index
-- until it is readable again; its return then counts as drift like any new file. `status` also
carries `model` and `dims`, empty while the index is lexical: they are the place where an
index built with one embedding model and queried with another would show up instead of
quietly returning nonsense.

This section describes what is built. The concept it belongs to -- how a deposit is
classified, what the query surface looks like, and which decisions rest on which measurements
-- is in [knowledge-gate.md](knowledge-gate.md).

## Inspecting What the Server Offers

The **/mcp** page -- reachable in the block through *State and tools* -- shows two things:

- the registration state per assistant, in more detail than the block,
- the tools actually offered. They are not kept in a list in code: the interface starts the
  registered command as a separate process, sends it `initialize` and `tools/list`, and
  displays what it returns. Thus, the same view also answers whether the server runs at all.

It starts the resolved absolute path -- what the assistant starts when the file holds that
form; for the bare name see [Three Explicit Boundaries](#three-explicit-boundaries) -- with
the project root as the working directory and **without the inherited shell PATH**. The last point is not a
detail. If the self-test ran with the PATH of the shell that started the interface, it would
report success while a client started from the Dock or Finder fails -- it would measure an
environment the client does not have. Instead, the minimal system PATH received by a GUI
program is set.

If the server does not respond, none is installed, or no usable JSON is returned, this is a
page result and not a failure: it reports "server does not respond" with the reason and
remains usable.

### All Servers of the Project

The **/mcp-servers** page -- the *MCP-Server* entry under *Setup* in the left column, or
*All MCP servers* in the block -- shows every MCP server the three assistants know in
this project: a matrix of server names against Claude Code, OpenCode, and Cursor, the
required servers from `tools.mcp.required` in `K-PLAYBOOK.yaml` with their gaps, and the
files that were read. The source is those files alone: `.mcp.json`, `opencode.json` (and
`opencode.jsonc`, if both exist), and `.cursor/mcp.json`. Global configurations and the
`enabledMcpjsonServers` lists in `.claude/settings*.json` are deliberately not read; the
page says so.

Each cell leads to a detail page `/mcp-servers/<assistant>/<name>` with the entry's
configuration -- transport, command or URL, resolved path, `enabled`, whether it is
required, and the key names from `env` without their values. Loading the page starts
nothing. *Measure* sends a `POST` and starts the configured command with the project root
as working directory and the **inherited** environment plus `env` from the entry -- unlike
the self-test on `/mcp`, which strips the shell PATH. Foreign servers via `npx` or `uvx`
do not live in the minimal system PATH; the page names the resolved path. The result
lists server name, version, protocol, capabilities, tools with parameters, and prompts and
resources where the server reports them. Remote servers (HTTP/SSE) are not contacted:
their login belongs to the assistant.

## Manually

The server can also be started without the interface:

```bash
k-playbook mcp
```

It then speaks JSON-RPC over stdin and stdout and waits for a client.

**Removal is manual only.** The interface sets up; it does not clean up. Anyone wanting to
remove the server deletes the `k-playbook` entry from `.mcp.json`, `.cursor/mcp.json`, and
`opencode.json` or `opencode.jsonc` -- then restarts the assistant.

## In This Repository

The k-playbook source repository is also its own target project. Of the three files, only one
is tracked:

- `.mcp.json` and `.cursor/mcp.json` are in `.gitignore`.
- `opencode.json` is tracked and cannot be partially ignored; its `mcp` block is therefore in
  the repository.

All three use the **portable form** -- the bare command name `k-playbook`. The absolute path
*Set up* writes is tied to a `$HOME` and cannot be used here; the two ignored files contain
the bare name only so this machine has the same setting everywhere. Automatic correction
touches neither: the portable form belongs to the set of accepted forms, and only the old
wrapper path is replaced automatically.

This keeps the working tree clean with every clone update and every start. Clicking *Set up*
here writes nothing either, as long as `opencode.json` carries the memory block: the bare
name is accepted. Only if that block were missing would *Set up* rebuild the entry with the
absolute path -- a diff in a tracked file that must be reverted.
