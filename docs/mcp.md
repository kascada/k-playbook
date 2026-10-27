# The MCP Server

k-playbook includes an MCP server. It gives an assistant the same information that
`k-playbook context` provides on the command line: the resolved paths, the instruction
files in reading order, the remediation policy, the guidelines, and the effective catalogs
for rules, reviews, and checks -- shipped and project-owned content already merged.

`k_playbook_context` and `k_playbook_briefing` are the only tools that also write as a side
effect: they synchronize the project's assistant linking with the catalog, just as the
subcommands do -- the briefing because its section `context` comes through the same function
(`ContextForDir`). They write only when writing changes something, and only to symlinks created
by k-playbook itself. The response lists what happened under `links` (`k_playbook_context`)
resp. `context.links` (`k_playbook_briefing`); if the field is absent, everything was already
up to date.

The server offers the working state, the review, todo and knowledge tools,
`k_playbook_doc_read` for references, the four `doc` tools for numbered documents in the
project's Kephalaion collection, `k_playbook_briefing` for the start of a session,
`k_playbook_health` for the errors and warnings about the project's environment,
`k_playbook_gui_status` for the state and the address
of the project's interface, and `k_playbook_notify` for a desktop notification on the
machine the server runs on.
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
message in all families -- `projectDir fehlt — nichts ausgeführt. Wiederhole den Aufruf
mit projectDir, dem Projektverzeichnis mit K-PLAYBOOK.yaml (absoluter Pfad).` A `projectDir`
that is given but leads to no project stays `project_not_found`. The price: clients that check
required fields before calling (Claude Code) no longer do so for `projectDir`. Other schema
violations -- another required field missing, a wrong type, also together with a missing
`projectDir` -- are still rejected by the SDK with its bare text.

The internal view -- protocol versions, the stdout rule, session termination -- is in
[`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server).

## Registering

The interface provides the **k-playbook MCP** block for this. Clicking *Set up* registers
the server in both files:

| Assistant | File | Entry |
|---|---|---|
| Claude Code | `.mcp.json` in the project root | `mcpServers` -> `k-playbook` |
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

Both files belong to the project and can contain third-party entries; the OpenCode
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
configuration without the memory block (`instructions` with `AGENTS.md`; `references.docs` is
no longer written since task 101, the entry of the project documentation comes from the briefing):
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
a directory on the way. Writing would land in the link target while the measurement saw
the link, and a link is a deliberate setup of the project, so it is skipped rather than
resolved. Only the components below the project root count; a project that lives under a
symlinked path is not affected. In both cases -- symlink and a repository git refuses to
trust -- only *Set up* writes; an obsolete entry is still corrected. Hard links are not
detected and remain a known gap. A
missing file is additionally created only if the assistant has left a **trace of its own**
in the project: an entry in `.claude/` or `.opencode/` that is not one of the symlink
targets k-playbook manages itself -- for example `.claude/settings.json` or a file of its
own under `.opencode/`. The directories alone are no trace; k-playbook creates them in
every project. Without that condition every start would create two files nobody needs.

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
`.mcp.json` and `opencode.json`; automatic correction leaves it there.

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
| `k_playbook_todo_list` | list the project todos; `includeDone` adds the completed ones, `search` filters by text |
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
| `k_playbook_doc_read` | read what lies behind a reference (`ref`) in the prefix scheme: `project:/…` a file of the project, `keph:/…` a document of the project's collection through the node (`~` for the own user), `k-playbook:/…` local and shipped already resolved against each other -- `/instructions/…`, `/rules/…`, `/reviews/…`, `/guidelines/…` and `/commands/…` (the text of a command whose hull lies with the assistant); a reference ending in `/` assembles the directory into one text, today only `k-playbook:/instructions/always/`; include lines are inserted unless `includes: false` |
| `k_playbook_doc_create` | create an empty numbered document of a kind (`tasks`, `workbench`, `findings`) in the project's Kephalaion collection; returns its `keph:` reference with the real user name |
| `k_playbook_doc_list` | list the numbered documents of a kind from the node's replica: by default the own ones, `user: "*"` all, `done` adds the completed ones |
| `k_playbook_doc_done` | move an own document into `done/` of its location |
| `k_playbook_doc_reopen` | move an own document from `done/` back into its location |
| `k_playbook_briefing` | the briefing at session start: the text of the always-instructions (`documents.always.content`), which instructions, rules, reviews and guidelines apply (references with title, description and origin), what is pending in the project's collection (own open tasks and workbench topics, the number of open findings), the environment check under `health` (the same report as `k_playbook_health`), and in the transitional section `context` the rest of `k_playbook_context` that the briefing does not cover itself; heals the assistant links as a side effect (`context.links`); like `k-playbook briefing --json` |
| `k_playbook_health` | check the project's environment and report every error and warning at once under `health` -- `ok` (no entry of level `error`) and `problems`, each `level`, `area` (`kephalaion`, `gh`, `baseTools`, `versionSources`, `github`), `message`, `hint`; repairs nothing except that the source `github` writes the template for a missing access file once; never names a token or a value from a file (service and key names may appear, values never); like `k-playbook health --json` |
| `k_playbook_gui_status` | report the state and the address of the project's interface without changing anything, like `k-playbook gui status` |
| `k_playbook_notify` | show the user a desktop notification (`title`, `body`, optional `sound`) on the machine the server runs on; `no_channel` when that machine has no way to the user, like `k-playbook notify` |

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
none of the three talks to Kephalaion itself.

The store is the project's Kephalaion collection (task 097, decision of 2026-10-19): one
document per todo, `todos/<nnn>-<short-name>.md` while open, `todos/done/<nnn>-<short-name>.md`
once ticked off -- project-wide, not under `user/<name>/`, because a todo belongs to the
project, not to a person. The kind `todos` is an entry of the kind table (`DocKinds`), with the
mark `OwnTools`: it is not in the `kind` list of the `doc` tools, and `doc_create`, `doc_list`,
`doc_done` and `doc_reopen` reject it, since `doc_create` would create an empty document and
`doc_done` would move without the `done:` field. Numbering and `done/` handling of the table
are shared. How the client talks to Kephalaion: [`kephalaion-client.md`](./kephalaion-client.md).

```markdown
---
origin: Task 026
created: 2026-08-25
done: 2026-09-08
doneMigrated: true
---

The text of the todo.
```

- **`id` is the number** in the file name. Numbers are counted from `todos/` and `todos/done/`
  together, starting at 1. A tombstone does not block a name and does not count, so **after
  deleting the todo with the highest number the next new one gets the same number again**;
  lower numbers are never handed out twice. On `name_taken` the next number is tried, as with
  `doc_create`. The short name is made from the first words of the text, normalized like
  `doc_create` (lower case, hyphens).
- **The body is `text`.** The frontmatter carries only `origin` (optional), `done` (a date, only
  under `done/`), `doneMigrated` (optional, `true`) and `created` (only for migrated entries);
  other fields are kept as they are. A todo without any of them has no frontmatter.
- **`created`:** if `created:` is in the frontmatter it applies, even when empty -- migrated
  entries without a date stay empty, timestamps are not invented. Otherwise Kephalaion's
  `created` stamp applies. Always a date `YYYY-MM-DD`.
- **Open or done is decided by the directory alone.** Ticking off writes `done:` with today's
  date **first** and then renames into `done/`, because `rename` records no date and `updated`
  moves with every text change; reopening renames back and then removes the field. Both remove
  `doneMigrated`. A leftover `done:` under `todos/` is ignored on reading; a missing one under
  `done/` falls back to the `updated` stamp. The next write of the entry cleans either up. If
  the second step fails, the tool reports its code (`outcome_unknown`, `name_taken`, ...) and
  says which step already happened.
- **Writing goes through the hub with the revision just read** (`base_revision`): a change made
  by another machine in between is `stale_revision`, nothing is overwritten -- list again and
  repeat. Every write carries its own `request_id`; `outcome_unknown` means the operation may
  have arrived: do not repeat, look with `k_playbook_todo_list` (`includeDone`).
- **Search:** Kephalaion has no search. `k_playbook_todo_list` with `search` and
  `k-playbook todo list --search` list, read every todo and filter by the text in Go,
  case-insensitively, within the list `includeDone` selects; the GUI searches open and done.
- **Without a usable Kephalaion there are no todos**, in the GUI neither:
  `kephalaion_unavailable`, as with the `doc` tools, no fallback to the disk.
- `doneMigrated` marks a `done` that came from a migration, so it is not mistaken for the day
  someone ticked the entry off.

**The former store `k-playbook-local/data/todos.json`** is no longer read by any path except
`k-playbook todo migrate`; the others only check whether it exists. While it does, every
response carries a `hint` naming `k-playbook todo migrate`, listing shows the collection
only, and add, update and delete are rejected with **`todos_not_migrated`**, so that no new todo
takes a number a former entry is meant to keep. `k-playbook todo migrate`:

- **First run** (no `data/todos.json.migriert` yet): creates one document per entry under the
  fixed name `<id, three digits>-<short-name>.md`, not through the numbering, so the former ids
  survive; done entries go straight to `todos/done/`. The frontmatter is the format above, with
  `created` always written (empty when the entry had none), `done`, `origin`, `doneMigrated`.
  An entry is skipped only if a document with its number already exists in `todos/` or
  `todos/done/` **and** has the same text; a different text aborts and names the entry. A
  document whose number does not come from `todos.json` aborts as well. Only once every entry
  has arrived is the file renamed to `data/todos.json.migriert`.
- **Latecomer** (`todos.json.migriert` exists, and a `data/todos.json` has appeared again, for
  example written by an older installed binary): its entries are appended through the normal
  numbering, done ones straight to `todos/done/`, with the same frontmatter; an entry whose text
  already exists as a document is skipped, so a repeated run after an abort creates nothing
  twice. Afterwards the file is renamed under a name of its own
  (`todos.json.nachzuegler-<date>[-<n>]`).
- `todos.json.migriert` is never overwritten; renamed files are never read again. The output
  names the created and skipped entries. `migrate` can be repeated.

A leftover `k-playbook-local/TODO.md` (the Markdown checklist before `todos.json`) is only
reported in `hint`: it is not taken over, its entries are to be transferred by hand. The
former translation on first access, `k-playbook todo import` and the response field `migrated`
are gone.

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

### Doc Read Contract

`k_playbook_doc_read` is a thin wrapper over `project.ResolveDocRef` (`project/docref.go`); the
subcommand `k-playbook read <ref> [--raw] [--json]` calls the same function and serves for
testing and as a fallback. Its input is `projectDir` (required by contract, like every tool),
`ref`, a reference in the prefix scheme of [verweisschema.md](verweisschema.md) exactly as it
stands in the text, and the optional `includes` (boolean, default `true`; `false` delivers the
raw text with its include lines, the counterpart of `--raw`). `ref` in that scheme is the
lasting contract of the tool: which source lies behind it is invisible to the caller.

All three prefixes are resolved (task 093; [verweisschema.md](verweisschema.md#access-route-per-prefix)):
`project:` a file of the project from disk below the project root, one layer; `keph:` a
document of the project's collection through the node's replica (Kephalaion `read` via the
client, access as for the `doc` tools, `~` replaced by `kephalaion.user`); `k-playbook:` the
kinds `instructions/`, `rules/`, `reviews/`, `guidelines/` and `commands/`, each a single
file local before shipped, and as include targets `includes/` and `testdata/`. Only
`instructions/always/` is assembled. Shipped `instructions/`, `commands/`, `includes/` and
`testdata/` are read from `defaults/`; for `commands/` that is the text of a command, while
the file under `share/commands/` (unpacked to `~/.local/share/k-playbook/commands/`) is only the hull the assistant registers (task 092,
[verweisschema.md](verweisschema.md#resolving-k-playbook)). The overlay is the one of
`catalogs` in `k_playbook_context` (`overlayDocDir`): for `rules` and `reviews`, name,
`origin` and `disabled` of an entry are the same over both ways, which a test checks.
`guidelines` is an overlay kind here -- shipped `k-playbook/guidelines/` and local
`k-playbook-local/guidelines/` -- while the field `guidelines` of the context output still
lists only the local files (see [k-playbook-format.md](k-playbook-format.md#what-exists-only-once)).

Include lines (`include: <ref>`, absolute or relative within the same prefix) are replaced by
the body of their target, recursively up to depth 10, in every document the tool reads;
[verweisschema.md](verweisschema.md#includes) has the rules.

A successful answer is `{ok: true, tool, projectDir, ref, directory, content, disabled, hint,
parts, problems, includes}`:

- `ref` -- the reference in canonical form (for `keph:` with the real user name);
  `directory` -- whether it ended in `/`.
- `content` -- a single file with its frontmatter, include lines replaced; for a directory
  the parts without frontmatter, separated by a blank line. Always present, `""` when empty.
- `disabled` -- `true` for a single `k-playbook:` file that an empty local file disables. That
  is not an error: `content` is `""` and `hint` says why. For a directory it stays `false`,
  even when every part is disabled; `project:` and `keph:` never disable, an empty file is
  empty content.
- `parts` -- the parts involved, in the order of the content, each with `name`, `origin`
  (`dist`, `local`, `override`, `project`, `keph`) and `disabled`; a single file has exactly
  one. Always present, `[]` for a directory without parts.
- `problems` -- what was skipped: files without a leading number in an assembled directory,
  and parts of one whose include failed (with the chain). Always present, `[]` when nothing
  was skipped. The subcommand writes `hint` and `problems` to stderr and keeps stdout for the
  content; exit 0.
- `includes` -- the inserted targets in reading order, each with `ref` (resolved), `origin`,
  `depth` (1 for an include of the main document) and `disabled`. Always present, `[]`
  without includes and with `includes: false`.

Error codes, in the envelope's `error.code` like the knowledge tools:

| Code | When |
|---|---|
| `invalid_input` | `projectDir` missing; `ref` is not a reference in the scheme -- no prefix, no `/` after it, an empty, hidden, `.` or `..` segment --; names a directory without the trailing `/`; or ends in `/` without naming an assembled directory (today only `k-playbook:/instructions/always/`): the file name is missing, decided without reading -- `k-playbook:/instructions/on-demand/`, `k-playbook:/instructions/`, `k-playbook:/rules/`, `project:/docs/`, `keph:/user/~/tasks/`, an unknown directory, a file with a trailing `/`; an include line with more than one argument |
| `unsupported` | a valid `k-playbook:` reference outside `instructions/`, `rules/`, `reviews/`, `guidelines/`, `commands/`, `includes/` and `testdata/` (for example `checks/` or `skills/`); the message names only the reader kinds |
| `not_found` | neither layer has the file or the assembled directory, the project has no such file, or the node reports `none` for the name in the collection; there is no fallback to reading anywhere else |
| `kephalaion_unavailable` | a `keph:` reference while the binding is not ready (`kephalaion.ok` not `true`), the node cannot be reached, or the session could not be opened; no fallback |
| `kephalaion_error` | the node rejected `read` with a message (collection not readable, replica never synced, invalid name); the message is the node's |
| `include_cycle` | an include target is already open in the chain -- `a.md` includes `b.md`, `b.md` includes `a.md`; the message carries the chain |
| `include_too_deep` | an include would lie deeper than 10; the message carries the chain |
| `project_not_found` | `projectDir` leads to no project |
| `read_failed` | the source could not be read |

A failed include in a single document keeps the code of its cause (`not_found`,
`unsupported`, `kephalaion_unavailable`, `invalid_input`, or one of the two include codes) and
appends the chain `a.md → b.md → c.md` to the message once; for a part of an assembled
directory the same text lands under `problems` instead, and the answer is a success without
that part.

### Document Tools in the Collection

`k_playbook_doc_create`, `_list`, `_done` and `_reopen` create numbered documents in the
project's Kephalaion collection, list them per user, and move them into `done/` and back.
They are thin wrappers over `project.UserDocs` (`project/userdocs.go`); the subcommand
`k-playbook doc create|list|done|reopen [--json]` calls the same code. Writing a document's
content is not part of them: a session does that directly with Kephalaion's `write`, with
`collection` = `kephalaion.address`. Reading goes through `k_playbook_doc_read` with the
`keph:` reference (task 093) or directly with Kephalaion's `read`. Which Kephalaion tools are
called underneath and what is expected of them: [kephalaion-client.md](kephalaion-client.md).

**Kinds.** What a kind is comes from one table, `project.DocKinds`; a new kind is a new
entry, and the allowed values of `kind` in the input schema (`enum`) and in the CLI are built
from it.

| Kind | Location in the collection | User area | Numbering range | `done/` | Start |
|---|---|---|---|---|---|
| `tasks` | `user/<name>/tasks/` | yes | all `user/*/tasks/` including `done/`, project-wide | yes | 100 |
| `workbench` | `user/<name>/workbench/` | yes | all `user/*/workbench/` including `done/`, project-wide | yes | 001 |
| `findings` | `queue/findings/` | no | the directory | no | 001 |
| `todos` | `todos/` | no | the directory including `done/` | yes | 001 |

`todos` has tools of its own (`OwnTools`, task 097) and is not reachable through the `doc`
tools: it is not in their `kind` list, and a `todos` kind or a `keph:/todos/…` reference is
`invalid_input` there. Contract: "Todo Contract and Migration".

`<name>` is `kephalaion.user` from `k-playbook context`, the person, not the account.
Everything under `queue/` has no user area; a table entry with a location under `queue/` and a
user area is rejected. Directories come into being in Kephalaion with their first document.
`tasks` starts at 100 so its numbers do not collide with the task files on disk.

**Inputs.** All four take `projectDir` (required by contract, like every tool).

| Tool | Further inputs |
|---|---|
| `k_playbook_doc_create` | `kind`; `name`, the short name -- k-playbook lowercases it, transliterates umlauts and `ß`, turns everything else into hyphens, drops a trailing `.md` and keeps at most 50 characters |
| `k_playbook_doc_list` | `kind`; `user` -- empty for the own user, `~` likewise, `*` for all, or a user name; `done` -- also list `done/` |
| `k_playbook_doc_done`, `k_playbook_doc_reopen` | `ref`, a `keph:` reference to one document, for example `keph:/user/~/tasks/100-name.md`; a segment `~` is replaced by `kephalaion.user` |

**Behaviour.**

- `create`: the number is the highest in the numbering range plus one, at least the start
  number, written with at least three digits (`<number>-<name>.md`). Highest number and list
  come from the node's replica; the document is created empty through the hub with
  Kephalaion `create`. On `name_taken` the next number is tried, up to ten times. Only the
  whole file name has to be unique: equal numbers with different names are allowed.
- `list`: for a kind with a user area, `user` filters by the user segment of the path; for a
  kind without one, `user` does not apply, all entries are listed and `hint` says so. Entries
  are sorted by reference.
- `done` moves `<location>/<file>` to `<location>/done/<file>`, `reopen` the other way, both
  with Kephalaion `rename` through the hub; the document keeps its `id`. Only own documents
  and only kinds with `done/` can be moved. An occupied target yields `name_taken`; nothing is
  overwritten.
- Without a ready binding (`kephalaion.ok` is not true) all four answer
  `kephalaion_unavailable`. There is no fallback to the disk.
- `create`, `done` and `reopen` need a Kephalaion node of at least **v0.3.1**. Before the
  first attempt k-playbook asks the node for its version (`whoami`); an older node yields
  `kephalaion_unavailable` with "Node <version> zu alt für Schreibvorgänge, mindestens
  v0.3.1 — `kephalaion upgrade`", and nothing is sent. A version that cannot be compared
  with v0.3.1 (`dev`, a suffix) is let through.
- **Request key and retry.** Every Kephalaion `create` and `rename` carries its own
  `request_id`. If the outcome stays open after sending -- transport error, timeout, a
  broken or unreadable answer, 5xx, or the node's code `outcome_unknown` -- k-playbook sends
  the same call with the same key and the same fields exactly once more, in a new session
  with its own time limit. The hub executes a key at most once: if the first attempt had
  already written, the retry answers `replayed`, which counts as success. A rejection in the
  retry counts as a rejection; for `create`, `name_taken` then leads to the next number, which
  gets a **new** key. Any other failure of the retry, or a caller that gave up after sending,
  ends in `outcome_unknown`. `create` never moves on to the next number while an outcome is
  open. `list` carries no key and is never retried.
- After `outcome_unknown` do not call the tool again: look with `k_playbook_doc_list` (for
  `done`/`reopen` with `done: true`) whether the document was created or moved.

**Answers**, in the usual envelope `{ok, tool, projectDir, …, error}`:

| Tool | Fields on success |
|---|---|
| `k_playbook_doc_create` | `ref` (the `keph:` reference with the real user name, e.g. `keph:/user/kamran/tasks/100-name.md`; without user area `keph:/queue/findings/001-name.md`), `kind`, `number`, `name` (file name), `user` (absent without user area) |
| `k_playbook_doc_list` | `kind`, `user` (the filter applied, a name or `*`; empty without user area), `done`, `items`, `hint` (only when a filter did not apply). Each item: `ref`, `title` (frontmatter `title`, else the file name), `user` (from the path; without user area the creator from Kephalaion's `created.by`, else empty), `created`, `updated`, `done` |
| `k_playbook_doc_done`, `k_playbook_doc_reopen` | `ref` (the new reference with the real name), `from` (the old one, `~` resolved), `kind` |

Error codes in `error.code`:

| Code | When |
|---|---|
| `invalid_input` | `projectDir` missing; unknown `kind` or the kind `todos`; a reference into `todos/`; a short name that is empty after adjusting it; a `user` that is not a valid name; `ref` not a reference in the scheme, not `keph:`, or a directory |
| `project_not_found` | `projectDir` leads to no project |
| `kephalaion_unavailable` | nothing reached the hub: the binding is not ready, the node cannot be reached or is older than v0.3.1, the session could not be opened, or the node reports `unreachable`, `unsupported` or `internal` (nothing stored, nothing sent); never after a `create` or `rename` may have reached the hub |
| `outcome_unknown` | `create` or `rename` may have reached the hub and the retry with the same key did not settle it; do not call again, look with `k_playbook_doc_list` |
| `name_taken` | `rename` onto an occupied target; or ten numbers in a row taken on `create` |
| `not_found` | the document to move does not exist |
| `foreign_user` | the reference lies in another user's area |
| `done_unsupported` | the kind has no `done/` |
| `wrong_dir` | `done` on a document already in `done/`, `reopen` on one outside `done/`, or a reference in no location of any kind |
| `kephalaion_error` | any other error from Kephalaion; the message names its code |

The CLI prints one line per result without `--json` (for `list`: reference, title, user,
updated, and `done`, separated by tabs) and ends with exit 1 on an error, the code in
parentheses at the end of the message.

### Briefing Contract

`k_playbook_briefing` is a thin wrapper over `project.BuildBriefing` (`project/briefing.go`);
the subcommand `k-playbook briefing [--json]` calls the same function, without `--json` as a
plain text for reading while testing. Its only input is `projectDir` (required by contract, like
every tool). The briefing is the one call at the start of a session: **data, not prose** — it
lists what applies and what is pending as references in the prefix scheme
([verweisschema.md](verweisschema.md)), never paths, and with one exception never contents:
`documents.always.content` carries the text of the always-instructions
(`k-playbook:/instructions/always/`), assembled exactly as `k_playbook_doc_read` returns it for
that reference (`joinDocDirParts` in `project/docref.go`, one function for both), because every
session has to read that text first — a second call for it would be a detour. The plain text of
`k-playbook briefing` therefore starts with this text, without the list of its parts; the parts
with their origin are in the JSON only. Every other listed document is read with
`k_playbook_doc_read` (`k-playbook:` references) or on the route of its prefix.
Environment facts — paths, tools, `gh`, the Kephalaion binding — stand in the briefing only in
the transitional section `context` (task 091): the rest of `k_playbook_context` that the
briefing's own sections do not cover, built from the same function (`ContextForDir`), never
rebuilt. So one call is enough for the start of a session, and the rest shows what is left of
`k_playbook_context`; each later step takes one field out of it and gives it a place of its own
(the authority is the table "Context field → future place" in
[knowledge-aktuell.md](knowledge-aktuell.md#target-for-k-playbookmd-and-agentsmd-task-086)),
and when the rest is empty, `k_playbook_context` and `k-playbook context` go. Until then both
work unchanged. A test (`TestBriefingTraegtKeinFeldDerKontextausgabe`) keeps the top-level
fields of the two outputs disjoint (`schemaVersion` excepted, which describes each output
itself) and the rest complete: every field of `BuildContext` that is not covered is in
`context`, the covered ones are not.

The section `health` (version 4, task 098) is the environment check: the report of
`health.Check` (`internal/health`), the same that `k_playbook_health` and `k-playbook health`
return -- see "Health Contract" below. It **evaluates** fields of the section `context`
(`kephalaion`, `gh`, `baseTools`, `versionSources`) and replaces none of them; the context
output stays unchanged. The area `github` evaluates no context field: it reads the personal
GitHub access file (`github-access.md`). In the plain text of `k-playbook briefing` the block stands right after
the text of the always-instructions and before "Auf Abruf", so that a session reads it before
it does anything: one line `Umgebung: keine Probleme` without a problem, otherwise `Umgebung:`
and one line per entry `  [level] area: message — hint`. The subcommand `briefing` exits 0
even with errors in `health` -- the briefing reports, it does not judge; the exit code belongs
to `k-playbook health`.

**Head of the plain text** (2026-10-25). `k-playbook briefing` without `--json` starts with three
lines before the text of the always-instructions: `Sitzung: <session>`, the request to read the
output in full (`Lies diese Ausgabe ganz, nicht gekürzt: …`), and `Es gelten:` with the titles of
the always-parts that are not disabled, joined by ` · ` (`briefingTextHead` in
`cmd/k-playbook/briefing.go`). They stand first because sessions cut the output short in the
shell: Claude Code read it with `| head -80` and missed both the instruction on findings and the
identifier, which then stood at the end. Whatever is cut, these lines survive and name what the
rest holds. The JSON answer is unchanged.

A successful answer is `{ok: true, tool, projectDir, schemaVersion, session, documents, work,
projectDocs, health, context}`:

| Field | Content |
|---|---|
| `schemaVersion` | the version of the briefing output, `"5"` (`"1"` had no `context`, `"2"` no `documents.always.content`, `"3"` no `health`, `"4"` no `session`) |
| `session` | the identifier of this session, new with every briefing: the time of the briefing as `YYYYMMDD-HHMMSS` and six hex characters of randomness (`newSessionID` in `project/briefing.go`), e.g. `20261024-183012-3fa9c1`. A session loads the briefing once, so the identifier stands for the session; what it writes and has to find again carries it -- a finding in `queue/findings/` under `sitzung`, which `/k-danke` looks for. It is assigned here because only Claude Code puts an identifier of its own into the environment (`CLAUDE_CODE_SESSION_ID`), OpenCode none (measured 2026-10-24). The plain text shows it as `Sitzung: <id>` in its first line (see "Head of the plain text" below) |
| `documents.always` | the assembled directory `k-playbook:/instructions/always/`: `ref`, `content` (the assembled text — parts without frontmatter, include lines replaced, disabled and empty parts skipped, joined by a blank line; `""` without parts; identical to what `k_playbook_doc_read` returns for `ref`), `parts` in the order of the text, `problems` (files without a leading number and parts whose include failed, each skipped — the briefing still comes about) |
| `documents.onDemand`, `.rules`, `.reviews`, `.guidelines` | the entries of the kind, sorted by name, each a file to read on its own; `reviews` follows the catalog pattern `review-*.md`, `README.md`, dotfiles and subdirectories never appear |
| each entry | `ref` (`k-playbook:/<kind>/<name>`), `title` (frontmatter `title`, else the first heading of any level, else the file name), `description` (frontmatter `description`, else `""`), `origin` (`dist`, `local`, `override`, or `disabled` for an entry an empty local file switched off) |
| `work.available` | whether the project's collection could be read; `false` with `work.message` when `kephalaion.ok` is not true, the node cannot be reached or a listing fails -- a state, not an error |
| `work.user` | the own Kephalaion user whose work is listed |
| `work.tasks`, `work.workbench` | the own open tasks (`user/~/tasks/`) and active workbench topics (`user/~/workbench/`), each `ref` (with the real user name), `title` (frontmatter `title`, else the file name), `updated` -- exactly what `k_playbook_doc_list` returns without `done`; "open" and "active" mean not in `done/`, a `status` frontmatter plays no role |
| `work.openFindings` | the number of documents in `queue/findings/` |
| `projectDocs` | the entry of the project documentation: the directory reference `keph:/docs/` in the project's collection (task 101). It is read with Kephalaion `list` on `docs/` (`recursive`, `frontmatter`), not with `k_playbook_doc_read`; `docs/index.md` first once Kephalaion generates it. Without a ready binding the entry is out of reach, and there is no fallback to `k-playbook-local/docs/` |
| `health` | the environment check: `ok` (`false` as soon as one entry has level `error`; warnings leave it `true`) and `problems` (always present, `[]` without a problem), each `level` (`error`, `warning`), `area` (`kephalaion`, `gh`, `baseTools`, `versionSources`, `github`), `message` (one sentence; service and key names may appear, never a token or a value from a file), `hint` (the remedy), sorted by area, then level (`error` first), then message -- exactly the report of `k_playbook_health`, see "Health Contract" |
| `context.available` | whether the context output could be built; `false` with `context.message` when there is no installation, the configuration is unreadable or of an unknown version -- a state like `work.available`, not an error: the rest of the briefing stays, `ok` stays `true`, the subcommand exits 0 |
| `context.<field>` | every top-level field of `k_playbook_context` under its name and with its value unchanged (`schemaVersion` of the configuration, `now`, `instructions`, `project`, `playbook`, `local`, `remediation`, `gh`, `mcp`, `git`, `baseTools`, `catalogs`, `versionSources`, `kephalaion`, `links` when there is something to report), in the order of the context output -- **minus** the fields the briefing covers itself, listed at exactly one place in the code (`briefingCoveredContextFields` in `project/briefing.go`): `catalogs.rules` and `catalogs.reviews` (`documents.rules`, `documents.reviews`) and `guidelines` (`documents.guidelines`; note the different meaning below). `catalogs` therefore carries only `checks` |
| `context.links` | the self-healing of the assistant links, exactly as `links` in `k_playbook_context`; absent when there was nothing to report |

All lists are always present, `[]` when empty. The documents come through the same overlay as
`catalogs` in `k_playbook_context` and `k_playbook_doc_read` (`overlayDocDir`): local before
shipped, an empty local file disables. `guidelines` is an overlay kind here -- shipped
`k-playbook/guidelines/` and local `k-playbook-local/guidelines/` --, while the field
`guidelines` of the context output still lists only the local files until the context output is
switched.

Deliberately not in the briefing's own sections: contents (except `documents.always.content`,
see above), commands, skills, checks, scripts,
k-playbook's own docs and the local knowledge store `k-playbook-local/knowledge/` -- that one joins later, with
counts per kind, state and origin and the freshness that
[knowledge-gate.md](knowledge-gate.md#querying-the-surface) described for its briefing.

Error codes in `error.code`: `invalid_input` for a missing `projectDir`, `project_not_found`
for a `projectDir` that leads to no project, and `read_failed` when a layer could not be read
-- a half briefing would look like a whole one, so the tool stops instead.

### Health Contract

`k_playbook_health` is a thin wrapper over `health.Check` (`internal/health/health.go`), the
same function behind the subcommand `k-playbook health [--json]` and behind the section
`health` of the briefing. Its only input is `projectDir`. The check is a collecting point: every
source registers itself (`health.Register`) and the check asks all of them, so a further source
joins without a change to briefing, CLI, MCP or GUI -- the fifth, the personal GitHub access
file `~/.config/k-playbook/github.yaml` (`github-access.md`, workbench 002), joined that way. The
sources themselves live where their data is read, in `project/health.go`, registered when the
package loads: `health` cannot import `project`, because `project.Briefing` carries the report;
the source `github` reads through `internal/githubaccess`, which knows neither. The check runs
**without network** beyond what `kephalaion.Check` already asks the local node, it **repairs
nothing** -- with one named exception: the source `github` writes the template for a missing
access file once, because its contract wants it, and reports that as a `warning` --, and it
**does not stop at the first problem**. No entry ever carries a token or a value from a file;
service names and key names may appear (`services.backend.access[1]: Der Pfad in file ist
relativ.`), values never. The entry for an unreadable `version-sources.yaml` names the path,
not the parser's message, which may quote a line of the file (that message stays in
`context.versionSources.error`); the entry for an unreadable `github.yaml` names the line
number, not the line.

The answer is `{ok: true, tool, projectDir, health: {ok, problems}}`. The report is nested under
`health`, not embedded, because its `ok` (no errors) is a different thing from the envelope's
`ok` (the tool ran); on the command line `--json` prints the bare report. Each entry has
`level` (`error` or `warning`), `area`, `message` and `hint`; the list is sorted by area, then
level (`error` before `warning`), then message, so two calls yield the same list. `problems`
is always present, `[]` without a problem. The sources in this version:

| Area | Condition | Level | Hint |
|---|---|---|---|
| `kephalaion` | the block `kephalaion:` is in `K-PLAYBOOK.local.yaml`, but the state of `kephalaion.Check` is not `ok` (invalid entry, no Kephalaion on this machine, node unreachable, token missing, login invalid, hub unknown or never synced, collection unreadable …) | `error` | per state: fix the entry, check the node (`kephalaion status`), check the account, sync the hub, or choose a readable collection in the GUI |
| `kephalaion` | not configured | `warning` | set up the binding in the GUI; without it `keph:` references, workbench and `work` are out of reach |
| `gh` | `tools.gh.status` is `enabled`, but `gh.ready` is false | `warning` | install gh, or `gh auth login` |
| `gh` | `tools.gh.status` has an unknown value (the context output fails on it) | `error` | set it to `unknown`, `enabled` or `disabled` |
| `baseTools` | one entry per tool in `baseTools.missing` | `warning` | the `installCommand` of the base tools |
| `versionSources` | `version-sources.yaml` is present, but unreadable or of an unknown schema version (`versionSources.error` set) | `error` | fix or remove the file; the details are in `context.versionSources.error` |
| `github` | the personal access file `~/.config/k-playbook/github.yaml` says something wrong: one entry per problem of the reader (`github-access.md`, "Reading the file"), the problem's path (`file`, `services.<name>`, `services.<name>.access[<i>]`) in front of the message because an entry has no `path` field | `error` | the reader's hint: fix that place in the file |
| `github` | the access file was missing and this call wrote the template | `warning` | fill in the template; the next call finds an empty file and reports nothing |

Exit codes of the subcommand: 0 without an error (warnings do not count), 1 with at least one
error, 2 on a call error (unknown option, no project). Error codes of the tool in `error.code`:
`invalid_input` for a missing `projectDir`, `project_not_found` for a `projectDir` that leads to
no project; the check itself never fails.

What a session does with the entries is in the always-instructions
(`defaults/instructions/always/030-briefing.md`): name them in the first answer, errors before
the work, repair nothing silently, offer the remedy from `hint`. The GUI shows the same report
on its start page, card "Umgebung", fetched from `GET /api/environment`, which answers
`{available, health: {ok, problems}}` -- the report nested under `health` like this tool's
answer, `available: false` and no report without a project configuration. `GET /api/health`
is **not** this report: it stays the window's heartbeat of the "GUI Status Contract" below and
must remain cheap, while the environment check asks the Kephalaion node. See the architecture
document, "Umgebungsprüfung: `health`".

### GUI Status Contract

`k_playbook_gui_status` answers what `k-playbook gui status` prints: the state and the
address of the project's interface. Both ways call the same function
(`guiproc.StatusForDir`); the subcommand renders its value set, the tool serializes it.

The tool **changes nothing**. It does not start a service, does not stop one, does not
remove an orphaned runtime file, and it does not even create the runtime directory -- a
missing directory is an answer, not a failure, because without it no service can be
running. Removing an orphaned file stays with `k-playbook stop`.

The value set, in the envelope `{ok, tool, projectDir, …, error}`:

| Field | Meaning |
|---|---|
| `state` | one of `none`, `running`, `otherVersion`, `orphaned`, `unresponsive` |
| `url` | the address of the interface, from the runtime file |
| `pid` | the process id of the live service |
| `version` | the version the service reports on `/api/health` |
| `build` | the build identity it reports there |

The five states: `none` -- no runtime file, and the same for a missing runtime directory.
`running` -- the service of this very program runs under `url`. `otherVersion` -- a service
runs, but from another version or another build of the same version. `orphaned` -- the
runtime file names a process that provably no longer exists. `unresponsive` -- the process
is alive and is the one from the file, but `/api/health` does not answer or answers with a
foreign key.

**Fields that were not measured are absent; they do not stand there as a null value.**
`url` and `pid` describe a process that was found alive, so they are absent for `none` and
`orphaned`. `version` and `build` come from `/api/health`, that is from the running process
itself, so they are absent whenever nobody answered. Without a service the whole answer is
therefore `{"state": "none"}`, and that is information, not an error -- on the command line
with exit code 0.

Error codes in `error.code`: `invalid_input` for a missing `projectDir`,
`project_not_found` for a `projectDir` that leads to no project, and `failed` if no runtime
directory can be resolved at all (neither `XDG_RUNTIME_DIR` nor `XDG_STATE_HOME` nor a home
directory). Note the precondition: a directory *without* an installation still has a key in
`guiproc`, so a service running on it can be asked about on the command line, while
`resolveProjectDir` rejects such a directory -- over MCP it cannot.

### Notify Contract

`k_playbook_notify` shows the user a notification on the machine the MCP server runs on: a
popup with `title` and `body`, and with `sound: true` a sound as well. It is meant for the
end of a long piece of work, when nobody is watching the screen any more -- not for
progress. Both ways call the same function (`notify.Send`); the subcommand
`k-playbook notify` renders its value set, the tool serializes it.

There is no common signal between assistants and no way for an MCP server to make a
client ring. The binary therefore notifies itself, on the machine it runs on. A stdio server
runs where the client started it: on a developer machine that is the desktop in front of
the user, in a devcontainer it is the container, which has no display and no Windows side.
The tool tries the known channels in a fixed order and reports honestly when none applies.

| Channel | Condition | How |
|---|---|---|
| `windows-toast` | WSL (`WSL_DISTRO_NAME`, `WSL_INTEROP` or a Microsoft kernel in `/proc/version`) and `powershell.exe` on `PATH` | a toast through the WinRT notification classes, sent as `-EncodedCommand`; the toast itself is silent, the sound is `C:\Windows\Media\notify.wav` through `Media.SoundPlayer`, so it does not depend on the Windows notification settings |
| `macos` | `darwin` and `osascript` | `display notification … with title …`, with `sound name "Glass"` on request |
| `linux-desktop` | `DISPLAY` or `WAYLAND_DISPLAY` set and `notify-send` on `PATH` | `notify-send`; the sound through `paplay` or `canberra-gtk-play` if present, otherwise the notification still goes out and `reason` says why there was no sound |

Input: `projectDir` (required by contract, like all other tools), `title` (required),
`body` (optional, truncated to about 200 characters), `sound` (optional, default `false`).

The value set, in the envelope `{ok, tool, projectDir, …, error}`:

| Field | Meaning |
|---|---|
| `channel` | the channel that showed the notification |
| `soundPlayed` | whether a sound was played |
| `reason` | only when a sound was requested and did not play: why not |

Error codes in `error.code`: `invalid_input` for a missing `projectDir` or an empty
`title`, `project_not_found` for a `projectDir` that leads to no project, `no_channel` when
the machine has no way to the user -- `error.message` names the reason, such as
`Container erkannt (DEVCONTAINER)` or a missing display -- and `failed` when the program
that should have shown the notification failed; its stderr is in `error.message`.
`no_channel` is information about the machine, not a reason to retry.

What is deliberately not there yet: a per-machine switch to turn the sound off, hooks in
the assistants that call `k-playbook notify` when a turn ends, and a way from a devcontainer
to the desktop in front of it.

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
*All MCP servers* in the block -- shows every MCP server both assistants know in
this project: a matrix of server names against Claude Code and OpenCode, the
required servers from `tools.mcp.required` in `K-PLAYBOOK.yaml` with their gaps, and the
files that were read. The source is those files alone: `.mcp.json`, `opencode.json` (and
`opencode.jsonc`, if both exist). Global configurations and the
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
remove the server deletes the `k-playbook` entry from `.mcp.json` and `opencode.json` or
`opencode.jsonc` -- then restarts the assistant.

## In This Repository

The k-playbook source repository is also its own target project. Of the two files, only one
is tracked:

- `.mcp.json` is in `.gitignore`.
- `opencode.json` is tracked and cannot be partially ignored; its `mcp` block is therefore in
  the repository.

Both use the **portable form** -- the bare command name `k-playbook`. The absolute path
*Set up* writes is tied to a `$HOME` and cannot be used here; the ignored `.mcp.json` contains
the bare name only so this machine has the same setting everywhere. Automatic correction
touches neither: the portable form belongs to the set of accepted forms, and only the old
wrapper path is replaced automatically.

This keeps the working tree clean with every clone update and every start. Clicking *Set up*
here writes nothing either, as long as `opencode.json` carries the memory block: the bare
name is accepted. Only if that block were missing would *Set up* rebuild the entry with the
absolute path -- a diff in a tracked file that must be reverted.
