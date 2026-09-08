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

The server offers the working state and the review-run tools. `k_playbook_context` has the
optional `dir` parameter; all review tools require `projectDir`, because a stdio server must
not assume that its process working directory is the target project.

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

The same command is always registered, in only two spellings -- the absolute path of the
installed `k-playbook`, resolved when writing:

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

The written registration is an absolute path and is therefore tied to an environment; it
cannot be committed as-is. To share it, enter the [portable form](#the-portable-form-the-bare-command-name)
manually -- the bare command name. What is there is not touched by automatic correction as
long as it belongs to the set of accepted forms.

## Approval and Restart

Written is not the same as available. The assistant reads its configuration at startup, so it
must be restarted once after setup.

Claude Code additionally requires **approval**: project-specific servers from a `.mcp.json`
apply only after explicit consent in an interactive session. The question appears once at the
next startup. Since v2.1.196, Workspace Trust is also required; a freshly cloned project
cannot approve its own servers.

## Why the Entry Is an Absolute Path

The registered value is the **absolute path of the installed binary resolved when writing**,
typically `~/.local/bin/k-playbook`, expanded. It is neither the bare command name nor the
project-owned wrapper used previously.

The reason is the case in which the entry is needed. A client started from the Dock or Finder
-- Cursor, VS Code, Claude Desktop -- does **not** inherit the PATH of a login shell;
`~/.local/bin` is typically absent. A bare `k-playbook` would be unavailable precisely in
those environments, while the same entry works from a terminal. An absolute path depends on
no inherited environment.

### A Set of Accepted Forms, Not a Target Value

Exactly one form is always written. **Validation** checks against a set:

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

It is never written. *Set up* continues to write the resolved absolute path; the portable
form is entered manually in the file and then remains there. Exactly the name is accepted --
`./k-playbook` and every path ending in the name are not.

### Three Explicit Boundaries

**The portable form does not cover the Dock/Finder case.** That is the cost, and it is stated
here rather than concealed: a client started from the Dock or Finder -- Cursor, VS Code,
Claude Desktop -- does not inherit the login-shell PATH. It finds nothing under a bare name,
and the server remains unavailable. This exact case is why *Set up* writes the absolute path.

Anyone sharing a committed registration shares a form that works from the terminal and in a
Dev Container, but not from the Dock. The path to a solution is the same as for separate
HOMEs: click *Set up* once in that environment. It writes the absolute path into the file --
in a repository that tracks it, that is a diff that must not be committed.

The `/mcp` self-test also does not cover the portable form: it starts what *Set up* would
write -- the resolved absolute path -- rather than what is in the file. It therefore answers
"does the installed binary respond?", not "does the client find the entry?".


**An absolute path from a foreign `$HOME` counts as current.** This is a decision, not an
oversight: otherwise, host and Dev Container would mutually declare the shared file obsolete
and rewrite it on every switch. The cost: if host and container share the same repository but
have separate HOMEs, MCP remains unavailable in the other environment without automatic
correction intervening. Then only the `/mcp` self-test reports a problem. Anyone working in
this situation sets up explicitly once in each environment.

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

This correction is **narrow and idempotent**. It writes only for an existing entry that is
obsolete in the narrow sense. A missing file is not created, a missing entry is not added,
and no accepted form is touched -- otherwise every start would dirty a project's committed
MCP files. One configuration entry is replaced; nothing is deleted.

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
are stable and in `snake_case`, including `project_not_found`, `run_not_found`, `run_exists`,
`invalid_mode`, `invalid_selection`, `selection_unknown`, `selection_unavailable`,
`entry_not_found`, `entry_kind_invalid`, `entry_state_invalid`, `result_required`,
`result_path_invalid`, `read_failed`, `write_failed`, `preflight_failed`, `execution_failed`,
and `merge_failed`.

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

## Inspecting What the Server Offers

The **/mcp** page -- reachable in the block through *State and tools* -- shows two things:

- the registration state per assistant, in more detail than the block,
- the tools actually offered. They are not kept in a list in code: the interface starts the
  registered command as a separate process, sends it `initialize` and `tools/list`, and
  displays what it returns. Thus, the same view also answers whether the server runs at all.

It starts exactly what the assistant starts: the same absolute path, with the project root as
the working directory and **without the inherited shell PATH**. The last point is not a
detail. If the self-test ran with the PATH of the shell that started the interface, it would
report success while a client started from the Dock or Finder fails -- it would measure an
environment the client does not have. Instead, the minimal system PATH received by a GUI
program is set.

If the server does not respond, none is installed, or no usable JSON is returned, this is a
page result and not a failure: it reports "server does not respond" with the reason and
remains usable.

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

All three use the **portable form** -- the bare command name `k-playbook`. The written
absolute path is tied to a `$HOME` and cannot be used here; the two ignored files contain it
only so this machine has the same setting everywhere. Automatic correction touches neither:
the portable form belongs to the set of accepted forms, and only the old wrapper path is
written automatically.

This keeps the working tree clean with every clone update and every start. Clicking *Set up*
here, by contrast, puts the absolute path in `opencode.json` -- a diff in a tracked file that
must be reverted.
