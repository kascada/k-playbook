# K-PLAYBOOK.yaml format

`.k-playbook/K-PLAYBOOK.yaml` is the configuration of a project that uses
k-playbook. It is also the **anchor**: the directory that holds
`.k-playbook/K-PLAYBOOK.yaml` is the project's root directory.

## Fundamental decision

Every project carries its own installation. There is no central base
installation and no fixed host path. The installation sits beside the
configuration:

```text
<project>/                 arbitrary name
├── .k-playbook/
│   ├── K-PLAYBOOK.yaml    the anchor
│   ├── commands/          project-owned commands, committed (only when used)
│   └── skills/            project-owned skills, committed (only when used)
├── k-playbook/            the installation, fully replaceable
└── k-playbook-local/      being phased out: never created any more, only read
```

Because configuration is **beside**, rather than **in**, the installation,
`k-playbook/` contains nothing project-owned. The directory can therefore be
updated completely, both with `git pull` and with `rm -rf` followed by a fresh
clone.

The playbook directory is always named `k-playbook`. The name of the project
directory above it is irrelevant.

## No paths in configuration

Earlier versions had a `paths:` block with nine keys. It no longer exists. All
locations derive from the project root, the directory that holds
`.k-playbook/K-PLAYBOOK.yaml`:

| What | Where |
|---|---|
| shipped commands | `~/.local/share/k-playbook/commands/`, linked through `.k-playbook/share` |
| shipped skills | `~/.local/share/k-playbook/skills/`, linked through `.k-playbook/share` |
| shipped review recipes | `<hub>:k-playbook/defaults/reviews/` (Kephalaion, since task 131) |
| shipped checks | `k-playbook/checks/` |
| shipped guidelines | `<hub>:k-playbook/defaults/guidelines/` (Kephalaion, since task 131) |
| shipped instructions, assembled and on demand | `<hub>:k-playbook/defaults/instructions/always/`, `<hub>:k-playbook/defaults/instructions/on-demand/` (Kephalaion, since task 131) |
| check runner | `k-playbook/bin/k-check` |
| scripts | `~/.local/share/k-playbook/scripts/` |
| security tool matrix | `~/.local/share/k-playbook/scripts/security-tools.tsv` |
| base tool matrix | `~/.local/share/k-playbook/scripts/base-tools.tsv` |
| shared script library | `~/.local/share/k-playbook/scripts/lib/` |
| project-owned commands | `.k-playbook/commands/` — setup does not create it; it appears when the project puts a command there |
| project-owned skills | `.k-playbook/skills/` — setup does not create it; it appears when the project puts a skill there |
| project-owned review recipes | `keph:/k-playbook/reviews/` — in the project collection since task 143. Setup no longer creates `k-playbook-local/reviews/`; nothing reads it, `/k-upgrade` 3.4 removes it once it is empty and otherwise names every file with its target |
| project-owned checks | none since task 134: `k-check` runs only the shipped checks from `k-playbook/checks/`; `/k-upgrade` 3.4 removes a leftover `k-playbook-local/checks/` |
| review results | none at present: review runs are paused until their results get a new place (task 147). `k-playbook-local/results/` is not created any more, and `/k-upgrade` 3.4 deletes a leftover one |
| results kept to present | `keph:/results/` — in the project collection, project-wide, `.md` only, names start with the date (`<YYYY-MM-DD>-<name>.md`); written only when the user asks or a command provides for it, never as a duty, and cleaned up by the user. The rules are `k-playbook:/instructions/on-demand/ergebnisse.md`. Not the same as `k-playbook-local/results/`, which `/k-review` and `/k-audit` still use |
| project documentation | `keph:/docs/` — in the project collection; the briefing names the entry under `projectDocs`. Setup no longer creates `k-playbook-local/docs/`; `/k-docs-code` writes to `keph:/docs/generated/code/` and `/k-docs-tools` to `keph:/docs/generated/libs/`, both through `k-playbook docs checkout|publish` |
| tool profiles | `keph:/docs/generated/libs/` — in the project collection, written by `/k-docs-tools` through `k-playbook docs checkout|publish libs`, not under `k-playbook-local/` |
| version inventory | `keph:/docs/generated/versions/inventory.md` — in the project collection, not under `k-playbook-local/` |
| manually maintained docs | `keph:/docs/manual/` — in the project collection, not under `k-playbook-local/` |
| project-owned guidelines | `keph:/docs/guidelines/` — in the project collection. Setup no longer creates `k-playbook-local/guidelines/`; `/k-upgrade` 3.4 deletes a leftover one even with content and names every file (todo #40) |
| project-owned instructions | `keph:/k-playbook/instructions/always/`, `keph:/k-playbook/instructions/on-demand/` — in the project collection since task 143; `k-playbook-local/instructions/` is no longer read, `/k-upgrade` 3.4 removes it or names each file with its target |
| project-owned includes | `keph:/k-playbook/includes/` — in the project collection since task 143; `k-playbook-local/includes/` is no longer read, `/k-upgrade` 3.4 handles it like `instructions/` |
| open tasks | `keph:/user/<user>/tasks/` — in the project collection, not under `k-playbook-local/`; form in `k-playbook:/instructions/on-demand/tasks.md` |
| completed tasks | `keph:/user/<user>/tasks/done/` — in the project collection; a `k-playbook-local/tasks/` from before is moved by `/k-upgrade` 3.3 and removed by 3.4 once empty |
| project todos | `todos/` in the project collection; a `k-playbook-local/data/todos.json` from before is taken over by `k-playbook todo migrate` |
| version sources | `keph:/k-playbook/inventory.md` — in the project collection since task 144, settings in the frontmatter; a `k-playbook-local/version-sources.yaml` is no longer read, `/k-upgrade` 3.6 moves it |
| private content | no place created by k-playbook any more (task 147); a `k-playbook-local/priv/` from before stays untouched |

A key whose value is always the same would only be a source of errors.
Commands therefore no longer guess or read paths: they derive them.

This also applies to project documentation. Previously, `paths.docs` was the
only value allowed to point out of the k-playbook directory with `../`, so a
project could continue using its existing docs. This special case is removed:
the project documentation lives in `keph:/docs/`, and other documentation
maintained by a project remains untouched: k-playbook claims only its own
directory.

### What setup no longer creates

Since task 107, setup creates none of `docs/`, `tasks/`, `tasks/done/`, `data/` and
`material/`, and the setup page does not count them as missing: their content lives in
the project collection. Since task 129 the same holds for `commands/` and `skills/`: their
content lives in `.k-playbook/commands/` and `.k-playbook/skills/`. In an existing project, `/k-upgrade` 3.4 deletes each of them that holds
nothing but `README.md` files and the managed `.gitignore` — directly under `data/` the
renamed leftovers of `k-playbook todo migrate` (`todos.json.migriert`,
`todos.json.nachzuegler-<date>[-<n>]`) count the same, while a `data/todos.json` not yet
taken over keeps it —, without asking (task 117); anything else keeps the directory, and the
command only names what lies there. For `commands/` and `skills/` it names each entry with its
target below `.k-playbook/` and moves only when the user explicitly asks for it.

The docs producers no longer write to disk: `/k-docs-code` and the
`ks-overlay-repo-analyse` skill go through `k-playbook docs checkout|publish code` into
`keph:/docs/generated/code/` (task 109), `/k-docs-tools` through
`k-playbook docs checkout|publish libs` into `keph:/docs/generated/libs/` (task 120).
Within `docs/`, the directory reveals the origin: a tool writes exclusively to
directories of its own origin, and the frontmatter field `generated.by` identifies which
tool wrote an individual file. The `README.md` of each directory that has a template
`k-playbook:/docs/<path>/README.md` (`defaults/docs/` in the source repo, shipped to the hub)
is written by the program from that template, the same in every project — by `publish`,
the inventory, setup and `k-playbook upgrade`, and `k-playbook docs readme` (task 137).

Since task 146, setup does not create `results/` either. Review results can be reproduced
from the code and are the state of one machine, not project knowledge; the review commands
are switched off until they get a new place for their results. In an existing project,
`/k-upgrade` 3.4 deletes a leftover `results/` without asking, even with content, and names
every deleted file: its content was usually not versioned and cannot be recovered from Git
afterwards. Nothing creates the directory again — the review tools report a missing one as
an error and write only into an existing one.

The former knowledge directories on disk, with their search index, are gone since task
123; nothing writes there any more. In an existing project `/k-upgrade` 3.0 clears what
is left of them and offers each knowledge file for a move to `keph:/docs/manual/` or for
deletion.

What lies on disk from before keeps its meaning until `/k-upgrade` has moved it:
`docs/README.md` is the old disk index and is cleared by 3.2, a
`docs/manual/` or `docs/versions/` is a state from before the move, `docs/extracted/`
is a state of that time, `material/befunde/` is emptied by 3.1, and `tasks/` is moved
by 3.3. The version inventory lives in the project collection as
`keph:/docs/generated/versions/inventory.md`, and hand-written documentation in
`keph:/docs/manual/`.

The source configuration of the version inventory is not created by setup either. Since
task 144 it is the document `keph:/k-playbook/inventory.md` in the project collection, its
settings in the frontmatter (`schema_version: 3`, `sources`, `exclude`, `helm_values`;
`roots` no longer exists, only paths below the project root are read); `/k-doc-inventory`
creates it on the first confirmed addition, and without it the default sources apply. It
is maintained manually and never overwritten by an update. Its status appears in the
`versionSources` section of `k-playbook context` (`ref`, `state`), so no command reads it
itself. A `k-playbook-local/version-sources.yaml` from before is no longer read: the health
check reports it as a legacy file, and `/k-upgrade` (conversion 3.6) moves its settings into
the document and deletes it after asking. The interface only displays the configuration in
the "Inventory" section, showing the document, its state, and the count of sources,
exclusions, and configured Helm values; it does not edit it.
That section also offers the inventory prompt, the same run as `k-playbook
inventory` and `/k-doc-inventory`, writing into the project collection. The contract is in
[`version-inventory.md`](./version-inventory.md); after a version jump in the current pass
the inventory is updated right away through this run, and the profiles are queued
(`erzeuger: tools`).

`k-playbook-local/` itself is being phased out: k-playbook no longer creates the
directory or anything in it (task 147), and the interface has neither the
**Project-owned structure** block nor the **Local settings** block with the
switch for `priv/` any more. What lies there stays untouched; a managed
`.gitignore` in `priv/` or `material/` stays and keeps working, and whether that
content is versioned is decided by hand. `/k-upgrade` 3.4 deletes a leftover
`results/`.
Details are in [`installation.md`](./installation.md#k-playbook-local-is-no-longer-created).

## Finding the anchor

The procedure applies equally to the tool and to an assistant:

1. If a directory was passed, it applies; check `<arg>/.k-playbook/K-PLAYBOOK.yaml`.
2. Otherwise search upwards from `realpath(CWD)`, one candidate per level: `<dir>/.k-playbook/K-PLAYBOOK.yaml`.
3. On finding it: `PROJECT_DIR = <dir>`, `PLAYBOOK_DIR = <dir>/k-playbook`.
4. `$HOME` and `/`, inclusive, are the boundary of the upward search.
5. If nothing is found, report that no installation exists. Do not guess or create anything.

The anchor is the **file** `.k-playbook/K-PLAYBOOK.yaml`, not the directory
`.k-playbook/`. The link maintenance creates the directory even without a
configuration (for the `share` link), so a `.k-playbook/` can exist where no
project is. The k-playbook repository itself no longer versions anything under
`.k-playbook/` — in its own checkout that directory belongs to a separate private
development repository —, so after the release following v0.16.30 an installation
`k-playbook/` carries a `.k-playbook/` only when the link maintenance creates it
there. The conclusion stands: a search for the bare directory could find a project
inside the installation.

**Moving from the former place.** Until 2026-11 the anchor was `<project>/K-PLAYBOOK.yaml`,
directly in the project root. The tool moves it before it checks a level: if
`.k-playbook/K-PLAYBOOK.yaml` already exists, the file at the former place is
deleted (a symlink: only the link); otherwise it is moved (a symlink: the file it
points to). There is no phase in which both places are read. After the first call of
the new program, an older program no longer finds the project: every environment
of the project -- a second machine, a DevContainer, a colleague's checkout -- then
needs the new program. The environment check (`k-playbook health`, area `anchor`)
warns when the moved anchor is neither tracked nor ignored, or when the former
place is still in the index.

The upward search must **not** stop at the Git worktree root.
`<project>/k-playbook/` is its own clone and therefore its own worktree; a
search beginning there would otherwise never reach the configuration one level
above.

## Merge shipped and project-owned content

Five kinds exist twice. What applies is the union of both sides:

| Kind | Shipped | Project-owned | Unit |
|---|---|---|---|
| instructions | `<hub>:k-playbook/defaults/instructions/` | `keph:/k-playbook/instructions/` | `*.md`, by path below `instructions/` |
| includes | `<hub>:k-playbook/defaults/includes/` | `keph:/k-playbook/includes/` | `*.md`, by path below `includes/` |
| review recipes | `<hub>:k-playbook/defaults/reviews/` | `keph:/k-playbook/reviews/` | `review-*.md` |
| commands | `~/.local/share/k-playbook/commands/` | `.k-playbook/commands/` | `*.md`, recursive |
| skills | `~/.local/share/k-playbook/skills/` | `.k-playbook/skills/` | directory with `SKILL.md` |

The comparison unit is the **name**. Both sides use the same naming convention,
so no derived key is required.

**The project-owned side of instructions, includes and review recipes lies in the
project collection** (task 143), under `keph:/k-playbook/<kind>/` beside `docs/`,
`queue/`, `todos/` and `user/`, with the same tree as below `k-playbook:/`: a project-owned
`k-playbook:/instructions/on-demand/muster.md` is `keph:/k-playbook/instructions/on-demand/muster.md`.
It applies to everyone who uses the project; write access is that of the rest of the
collection, without a scope of its own. The rule is the same as for every layer: the same
name replaces, an empty document disables, a new name is added; `k-playbook:/instructions/always/`
is still assembled from both layers in one go, numerically sorted across them. Without a
`k-playbook/` directory in the collection the project-owned layer is empty, without an
error. An error of the collection — not ready, not listable, not readable — is
`kephalaion_unavailable`, also when `<hub>:k-playbook` is readable: there is no silent
fallback to the shipped layer alone. The switch was hard: `k-playbook-local/instructions/`,
`includes/` and `reviews/` are no longer read; `/k-upgrade` 3.4 removes them.

The origin values stay `dist`, `local`, `override` and `disabled`. A project-owned part from
the collection carries its place as well: the field `location` with the `keph:` reference
(`keph:/k-playbook/reviews/review-x.md`) in the briefing, in `k-playbook read --json`, in
`k_playbook_doc_read` — for an assembled directory per part — and in the catalog entries
of the context output; the plain-text briefing shows it next to the origin
(`[local keph:/k-playbook/…]`).

The kind for rules is gone since task 135: nothing is shipped, read or listed for it, and
`/k-upgrade` 3.4 removes a leftover local directory once it is empty.

Checks have no project-owned side since task 134. The catalog `checks` lists the shipped
checks from `k-playbook/checks/` (`*.sh`, top level only) and nothing else: `origin` is
always `dist`, nothing is overlaid and nothing disabled, and a leftover
`k-playbook-local/checks/` is not read; `/k-upgrade` 3.4 removes it once it is empty and
otherwise notes that what lies there no longer runs. A new form for project-owned checks
is still open.

`k-playbook-local/tmp/` was never created by the setup: sessions laid it out by hand as a
scratch pad in the days of the knowledge store, with the managed `.gitignore` and a README
that promised a cleaner which never existed. Nothing reads it and nothing cleans it;
`/k-upgrade` 3.4 removes it once it is empty and otherwise says so — notes and interim
states belong in `priv/`.

Project-owned commands and skills live in `.k-playbook/` since task 129, versioned next to
the configuration; the former place `k-playbook-local/commands/` and
`k-playbook-local/skills/` is no longer read, neither by the linking nor by
`k-playbook:/commands/<name>.md`. A file left there has no effect; `/k-upgrade` 3.4 names it
with its target.

For commands, it is the path below `commands/`, including the namespace: a local
`commands/_shared/context.md` replaces exactly that file, while the rest of
`_shared/` remains shipped. A skill, however, is replaced as a whole:
`SKILL.md`, `PLAYBOOK.md`, and templates must fit each other; a partially
replaced skill cannot sensibly be composed.

**For the same filename, the project-owned file wins, completely.** The shipped
one is not read at all, and no individual sections from it are retained. To
change a shipped entry, copy it and change the copy, at the cost that later
improvements to the original no longer reach that copy. The advantage outweighs
this: what applies is in exactly one file.

**Content is disabled with an empty file**, not with a list in configuration.
Because a same-named local file completely replaces the shipped one, nothing
remains when it is empty. "Empty" means nothing except blank lines and comments,
which lets the file record its own reason:

```bash
# Disabled: this project does not use Django.
```

A disabled review stays visible in the catalog, and its content says it is disabled.
Checks cannot be disabled since they have no project-owned side.

Disable a project-owned file by deleting it.

`README.md` in one of these directories is never an entry, nor are dotfiles or
anything under `checks/lib/`.

The same applies to `scripts/lib/`: it contains shared code next to executable
scripts, not a separate entry. Today it contains `install-common.sh`, which
`install-security-tools.sh` and `install-base-tools.sh` both source. Both use
the same release path and apply the same guard to the installation destination.
A second copy would undermine the promise that the security matrix asset
patterns continue resolving to the same asset unchanged as soon as one resolver
changes without the other.

For skills, `SKILL.md` decides disabling: if it is empty, the skill is disabled
and not registered.

To learn what ultimately applies, do not query the filesystem but the tool:

```bash
k-playbook context
```

Its output lists the merged catalogs with the origin of each entry: `dist`,
`local`, or `override`, and marks disabled content. See [The resolved working
state](#the-resolved-working-state).

Because commands and skills come from two sources, assistant destinations,
`.claude/commands`, `.claude/skills`, and `.opencode/commands`, are real
directories with **one symlink per entry**, not a
directory symlink: it would point at exactly one source.

### What exists only once

Everything else has no counterpart on the other side:

| | Directories |
|---|---|
| project-owned only | `priv/` |
| shipped only | `docs/`, `scripts/`, `bin/`, `installer/` |

`docs/`, `tasks/`, `data/`, `cache/`, `material/` and `results/` were project-owned only as well;
setup no longer creates them (see [What setup no longer creates](#what-setup-no-longer-creates)).
A `k-playbook-local/docs/` that a producer still writes is not a pair with
`k-playbook/docs/` either: `k-playbook/docs/` documents k-playbook itself, the
other the project. They are two different objects with the same name and nothing
to merge.

`guidelines/` is a pair since task 086 (2026-10-16): shipped
`<hub>:k-playbook/defaults/guidelines/` (since task 131; before, `k-playbook/guidelines/`)
and project-owned `k-playbook-local/guidelines/`, merged by the same rule as `reviews/`
-- a file of the same name replaces, an empty one disables. `k_playbook_doc_read`
(`k-playbook:/guidelines/<name>.md`) and the briefing resolve it that way. The field
`guidelines` of the context output is the one named exception: it still lists only
the project-owned files until the context output is switched (see the next section);
`catalogs` does not list the kind. The local layer is on its way out: setup no longer
creates `k-playbook-local/guidelines/`, `/k-upgrade` deletes it, and project-owned
guidelines live in `keph:/docs/guidelines/`. The overlay code stays until todo #40 and
yields nothing without the directory.

Nothing below `k-playbook/` may be written, including by commands that read
recipes or checks there. An update replaces the directory completely.

## The resolved working state

```bash
k-playbook context
```

Outputs as JSON what a command would otherwise have to calculate itself from
configuration and the filesystem:

| Field | Content |
|---|---|
| `schemaVersion` | the validated configuration version |
| `now` | invocation time: `date` as `YYYY-MM-DD`, `timestamp` as RFC 3339 |
| `project` | project root, `repoRoot`, `vcs`, configuration location |
| `playbook`, `local` | the two resolved directories |
| `remediation` | the policy, including a default when the block is missing |
| `docs` | the `docs:` block: `exclude` is the list of valid patterns from `docs.exclude` (empty without the block), `problems` the rejected entries, each with `pattern` and `reason` |
| `gh` | the GitHub CLI host finding for this machine |
| `git` | the `git:` block: `switch` (`unknown`, `offer`, `off`), `allow`, `environments` as a list of `{name, branch}` in file order, and `configured`, which says whether the block was present |
| `mcp` | the required MCP servers: `required` is the list from `tools.mcp.required`, `configured` says whether the block was present; an object so further MCP settings can join without renaming the field |
| `catalogs` | `reviews`, merged with the project-owned side, and `checks`, shipped only (task 134); the shipped side of `reviews` comes from `<hub>:k-playbook/defaults/` (task 131), the project-owned side from `keph:/k-playbook/reviews/` in the project collection (task 143) |
| `catalogsProblem` | only when a layer of `reviews` cannot be read (Kephalaion not ready, `<hub>:k-playbook` missing or `defaults/` empty, the project collection not readable): why that list is empty -- it is never shown from one layer alone; the rest of the output stays, and `kephalaion` says more |
| `guidelines` | files from `k-playbook-local/guidelines/` only -- not merged with the shipped guidelines, unlike `k_playbook_doc_read` and the briefing; the one named exception until the context output is switched (task 086) |
| `kephalaion` | the project's Kephalaion binding and the result of its check: `configured`, `hub`, `collection`, `account`, `address` (`<hub>:<collection>`), `state`, `ok`, `message`; when `ok`, also `rights`, `scopes`, `user`, `node`, `nodeVersion` and `sync` |
| `links` | only when there is something to report: what assistant-link self-healing updated (`healed`), what remained open (`open`), and what that means for this session (`note`) |

Each catalog entry has `name` (the filename), `key` (the invocation name without
extension and kind prefix), `path`, `origin` (`dist`, `local`, or `override`; for
`checks` always `dist`), `location` for a project-owned recipe (its `keph:` reference
under `keph:/k-playbook/reviews/`, also when it disables), and `disabled`, where applicable
(never for `checks`). `path` is a file on disk only for a shipped check; for every recipe —
shipped, project-owned or overriding — it is the effective reference
`k-playbook:/reviews/<name>`, read with `k_playbook_doc_read` or `k-playbook read`, because
both layers lie in collections (tasks 131 and 143).

This means no command has to apply overlay rules itself. There is one answer,
and everyone receives the same one.

**Beside the context output stands the briefing** (task 086, 2026-10-16):
`k-playbook briefing [--json]` and the MCP tool `k_playbook_briefing`. The context
output is the environment -- paths, configuration, tools, the merged catalogs with
their paths. The briefing is what a session has to know at the start: which
instructions, reviews and guidelines apply, as references in the prefix
scheme with title, description and origin (`k-playbook:/instructions/always/` with
its parts, `on-demand/`, `reviews/`, `guidelines/`), and what is pending in
the project's collection -- own open tasks and workbench topics, the number of open
findings. References, never paths; no contents, with one exception: the text of the
always-instructions itself (`documents.always.content`, the plain text starts with
it after a three-line head: session, read-in-full request, titles of the parts), assembled as `k-playbook read k-playbook:/instructions/always/` assembles it,
because every session reads it first. Both go through the same overlay
(`overlayDocDir` in `project/docref.go`), so an entry has the same origin wherever
it appears.

Since task 091 (2026-10-17) the briefing also carries **the rest of the context
output**: everything in the table above that its own sections do not cover, in the
section `context` (`--json`, MCP) resp. behind the separator line "Noch aus
k-playbook context — wird abgebaut" (plain text). The rest comes from the same
function as `k-playbook context` (`ContextForDir`), minus `catalogs.reviews`
and `guidelines`, which are `documents.reviews` and `.guidelines` -- the list stands at one place in the code
(`briefingCoveredContextFields`). Nothing stands twice: environment facts are in
the briefing only there. So one call is enough for the start of a session, and
because the rest comes through `ContextForDir`, the briefing heals the assistant
links too (`context.links`). If the context output cannot be built, that is a state
(`context.available: false` with `context.message`), not an error. The rest is a
transition: each later step takes one field out and gives it its own place in the
briefing -- the authority is the table "Context field → future place" in
[knowledge-aktuell.md](knowledge-aktuell.md#target-for-k-playbookmd-and-agentsmd-task-086)
--, and when it is empty, `k-playbook context` and `k_playbook_context` go. The first
field has gone that way: `instructions`, the list of the `k-playbook.md` files, no longer
exists (task 148), because the reading order is `documents.always`. Today the `AGENTS.md`
block, `k-playbook context` and `catalogs` are unchanged; the switch comes step by step. Contract: [mcp.md](mcp.md#briefing-contract).

`links` is normally absent, intentionally: the invocation brings assistant
linking in line with the catalog, as described below, and a message that says
the same thing on every invocation is ignored. If it is present, look at it:
`healed` names commands and skills that have just been registered. They exist
on disk, but the running assistant read its list at startup and does not yet
know them; `note` says exactly that. `open` names destinations that cannot be
resolved automatically: a real project file in the way, or a conflict in
`CLAUDE.md`.

`kephalaion` takes hub and collection from the `kephalaion:` block of
`.k-playbook/K-PLAYBOOK.yaml` (see `kephalaion` under "Fields"); the account, its token file
and the node's address come from Kephalaion's client information (`kephalaion node client
--json --format 1 --no-probe --hub <alias>`), and `choice` names where the account comes from
(`override`, `default` or `only`). The check asks the node of this machine over MCP
(`whoami`, with the header pair `X-Keph-Account-<hub>`/`X-Keph-Token-<hub>` and the token
from Kephalaion's own file `<tokensDir>/<hub>/<account>.token`); it runs only when the block
is present and never stops `context` — every failure is a `state` with its own `message`,
which names the remedy: `not-configured`, `invalid-entry`, `kephalaion-missing`,
`client-failed`, `node-address`, `account-choice-missing`, `token-missing`,
`node-unreachable`, `hub-unknown`, `hub-never-synced`, `login-invalid`, `login-missing`,
`replica-unreadable`, `collection-unreadable` or `ok`. `sync` carries `lastSuccess`,
`revision`, `lastError` and `neverSynced` as the node reports them. The field is filled by the
subcommand and the MCP tool; internal callers that only need the catalogs do not ask the node.
Details: `installer/docs/architecture.md`, section "Kephalaion-Anbindung".

`now` is present for another reason: commands place dates into persistent files,
review logs, result directories, and summary-file names. Not every assistant's
host tells it today's date, and a guessed date in a log is worse than none. It
is the only field that ages: it identifies invocation time, not write time.

`gh` is a finding for this particular machine only: `installed`, `path`,
`loggedIn`, `account`, `accounts`, and `tokenFromEnv`. `ready` summarizes what a
command needs to know: gh exists and an account is configured. There is no
project decision about gh: whether gh is used follows from the personal GitHub
access file (`docs/github-access.md`) and, without a service for the repository,
from `ready`.

The finding is read from gh configuration, not checked against the server: a
stored token may have expired. If certainty is needed, run `gh auth status`;
that requires network access and does not belong here.

The invocation is deliberately cheap: it omits the security-tool preflight,
because it starts `--version` for every tool and takes noticeable time.
`context` must be usable at the start of every command. The gh finding costs
nothing: it only checks `PATH` and `~/.config/gh/hosts.yml`, without a subprocess.

The search starts at the working directory and proceeds upward. Without
`.k-playbook/K-PLAYBOOK.yaml`, the call stops with a message; it does the same for a
`schema_version` other than `3`.

## Instructions

What an assistant must read before working comes with the briefing: the text of
`k-playbook:/instructions/always/`, assembled from two levels.

| Where | Applies to | On update |
|---|---|---|
| `<hub>:k-playbook/defaults/instructions/` | every project using k-playbook | the hub fetches the new release |
| `keph:/k-playbook/instructions/` in the project collection | this project only | remains |

The shipped parts come first, the project-owned ones after them; a project-owned
document of the same name replaces the shipped one, an empty one disables it (see
"Merge shipped and project-owned content"). The former instruction file
`k-playbook.md` — once shipped at the top of the installation, once in
`k-playbook-local/` — is gone (task 148): the program reads neither, the shipped
file is no longer part of the repository, and the context output has no field
`instructions`.

`AGENTS.md` receives only a **prompt**: a short block naming the one call of a
session, `k-playbook briefing`, with the MCP tool `k_playbook_briefing` as its
equivalent — exactly the `AGENTS.md` of the source repository. If the file does
not exist, it is created from that template; if it exists, the block is appended
without touching existing content. A marker, `<!-- k-playbook:anstoss -->`,
prevents a second run from appending it again. A block that still carries a
superseded call (`k-playbook context`, before that the wrapper
`k-playbook/bin/k-playbook context`) is replaced by the current one, and the two
blocks the template used to add behind it — `k-playbook:session-memory` and
`k-playbook:befunde` — are removed: their content comes with the briefing as
`instructions/always/100-…` and `200-…`. Replaced and removed is only what can be
bounded positively (the shipped shape line by line, else the next heading or
HTML comment, never the end of the file); project text before, between and
behind the blocks stays, and a block without a boundary stays with a hint. This
runs at setup, at the start of the interface and with `k-playbook upgrade`.

`k-playbook upgrade` is the one catch-up in the terminal, the subcommand behind
`/k-upgrade`: first the program — `git pull --ff-only` in the clone (a clone
still on `git@github.com:kascada/k-playbook` is switched to HTTPS first), then the
binary through `k-playbook/bin/install`, only when the running program is older
than the clone's `VERSION` —, then the project: `AGENTS.md`, `CLAUDE.md` as the
include file, the linking of commands and skills, the MCP entries. After a
binary change the newly installed program performs the project step, and the
message ends with the hint to restart the session; `--check` writes nothing
and reports what a run would do. It never touches project prose in `AGENTS.md`
or anything under `k-playbook-local/`.

Claude Code does not read `AGENTS.md` automatically. Setup therefore creates a
neighboring `CLAUDE.md` containing only the import line `@AGENTS.md`: a regular
file, not a symlink. [`installation.md`](./installation.md#an-existing-claudemd)
explains how the pair is classified and what happens to a supplied `CLAUDE.md`.

## Minimal format

This is what the tool creates when a new project is connected:

```yaml
# k-playbook
#
# The location of this file determines the project's root directory.
# The installation is beside it under k-playbook/ and can be fully
# replaced; project-owned files do not belong in it.

schema_version: 3

project:
  # Project repository location, relative to this file.
  repo_root: .
  vcs: git

remediation:
  # How findings from reviews are addressed.
  mode: task-first
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  # Derived from the mode; commands read these directly.
  pr_required: false
  direct_fixes: true

# docs:
#   # Excluded from the code scan of the docs producers, in addition to the
#   # fixed exclusions. Patterns as in .gitignore, relative to project.repo_root.
#   # Add with `k-playbook docs exclude add <pattern>…`.
#   exclude:
#     - docs/
#     - vendor/
```

The `docs:` block is written as a comment only: without it nothing changes.

## Complete example

```yaml
schema_version: 3

project:
  repo_root: app
  vcs: git

remediation:
  mode: task-branch-pr
  target: app
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false

docs:
  exclude:
    - docs/
    - vendor/

kephalaion:
  hub: vm
  collection: app

tools:
  mcp:
    required:
      - k-playbook

git:
  switch: offer
  allow:
    - development
    - remediation/*
  environments:
    dev: development
    prod: master
```

## Fields

### `schema_version`

Required field. Current version: `3`.

`3` describes the model documented here: an anchor `.k-playbook/K-PLAYBOOK.yaml`
in the project root, `k-playbook/` and `k-playbook-local/` beside it, and no paths
in configuration. The move of the anchor from `<project>/K-PLAYBOOK.yaml` into
`.k-playbook/` did not change the number: the file and its fields stay the same.

Older values belong to retired models and are no longer supported:

| Value | Model |
|---|---|
| `1` | central base installation under `~/dev/k-playbook` |
| `2` | anchor in the k-playbook directory, installation under `_dist/`, `paths.*` |

The tool stops for every other version rather than continuing. Silently
continuing to read would be most dangerous: the values could be read but would
mean something else. A number higher than `3` is reported as "installation
older than configuration"; a missing `schema_version` is also an error.

There is no `migrate` command: the models describe different directory layouts,
and converting their fields into each other would mean maintaining a translation
that grows with every model. Instead, the interface resets it: it saves the old
file as `K-PLAYBOOK.yaml.v1-alt` and creates a fresh one. The process is
described in
[`installation.md`](./installation.md#a-configuration-from-a-retired-model).

### `project.repo_root`

Required field. The project repository location relative to the project root --
the directory that holds `.k-playbook/`, not the directory of the file itself.

Typical values:

- `.` when the project root itself is the repository.
- `app` or another directory name when code is checked out alongside the installation, such as in a DevContainer.

The repository is deliberately in configuration and is not derived from the
filesystem. Commands may read and validate the value but may not search for Git
roots themselves.

### `project.vcs`

Required field. Either `git` or `none`. `none` is an explicit project decision,
so it belongs in the file instead of being guessed by commands.

### `remediation`

Block for `/k-remediation`. The tool creates it with new projects.

| Field | Type | Meaning |
|---|---|---|
| `mode` | enum | `task-branch-pr`, `task-first`, or `direct-allowed` |
| `target` | string | remediation target relative to the project root (like `project.repo_root`); default is `project.repo_root` |
| `grouping` | boolean | group findings into meaningful bundles before implementation |
| `quick_wins` | boolean | highlight simple, high-impact bundles |
| `branch_prefix` | string | recommended prefix for remediation branches |
| `pr_required` | boolean | derived from `mode` |
| `direct_fixes` | boolean | derived from `mode` |

The modes, from strictest to most permissive:

| Mode | Meaning | `pr_required` | `direct_fixes` |
|---|---|---|---|
| `task-branch-pr` | No direct fixes. Every confirmed bundle becomes a task with branch and PR guidance; it is implemented later with `/k-task-run`. | `true` | `false` |
| `task-first` | Tasks are the standard. Direct fixes only when explicitly approved for individual small bundles. | `false` | `true` |
| `direct-allowed` | Small, safe findings may be fixed immediately after code inspection when the categories are approved. | `false` | `true` |

**The default is `task-first`.** Tasks as the standard are the safe default:
nothing changes in code without action, while direct fixes remain possible after
approval. That default is a display value, not a decision. If the block is absent,
`k-playbook context` reports `mode: task-first` alongside `configured: false`, and
`configured` is the field that decides: `/k-remediation` must not guess a mode but
explicitly ask for the current session, or have the policy set through `/k-gui`.
`configured` belongs to the `k-playbook context` output, not to `K-PLAYBOOK.yaml`;
it is therefore not in the field table above.

`pr_required` and `direct_fixes` are additionally in the file so commands can
read them without interpreting the mode. They are written when setting the mode
and are not maintained independently.

### `tools`

Optional block for project-local tool decisions.

Important: this contains project decisions, not host facts. Whether `gitleaks`
or `trivy` is installed on this machine belongs in a preflight report, not in a
versioned project configuration.

#### `tools.gh` (removed)

Until task 128 a block `tools.gh` with `status` (`unknown`, `enabled`,
`disabled`) held the project's decision whether it uses the GitHub CLI. It is
gone: whether gh is in play is decided by the way to GitHub alone — with a
service of the personal GitHub access file, its selected access; without one,
`gh.ready` from the context output. An existing block is ignored, also with an
unknown value, and nothing fails on it. Setting up the project (`k-playbook
upgrade`, or setting up and updating in the interface) removes it together with
its comments, without asking, and says so in its answer
(`AssistantSetup.configBlocksRemoved`); other blocks under `tools:` stay, and an
empty `tools:` goes as well.

#### `tools.mcp`

| Field | Type | Meaning |
|---|---|---|
| `required` | list of strings | MCP server names the project expects in every assistant |

Which MCP servers the project requires -- by the key under which they appear in
`.mcp.json` and `opencode.json`. Both YAML list forms are
read: block form with dashes and flow form `[k-playbook, atlassian]`. Names
must match `^[A-Za-z0-9][A-Za-z0-9._-]*$`; any other name stops `context`, as
an invalid language name does. The interface does not stop: the **/mcp-servers**
page shows the required list as unreadable with the error, its matrix pill
warns, and a server's detail page reports "Pflicht: unbekannt" instead of "no".
Only `required` directly under `tools.mcp` counts; a `required` key deeper
inside `mcp` is not the list.

The block is optional. Without it, nothing is required and `context` reports
`mcp.configured: false`; the **/mcp-servers** page of the interface then shows
a snippet to copy. With it, the page marks every required name that is missing
in one of the two assistant files. Nothing writes this block: it is edited by
hand.

### `git`

Optional block for the **Branches** page of the interface: whether it offers switching the
branch of the code repository (`project.repo_root`), where to, and which long-lived branch
serves which environment.

| Field | Type | Meaning |
|---|---|---|
| `switch` | enum | `unknown`, `offer`, or `off` |
| `allow` | list of strings | branch patterns a switch may target; `*` matches any characters including `/`; empty means every branch |
| `environments` | mapping | environment name → branch name, in file order |

**The default is `unknown`.** It is an explicit state, not a silent no: without the block the page lists branches and environments but offers no switch and says
where the decision belongs. `off` is the decision not to switch from the interface.

`environments` is the only authoritative source for which branch serves an environment.
Without an entry, the page shows a suggestion — from the branch name, or, when the project uses
`gh`, from the latest deployment — and labels it as such. A branch may serve several
environments; several branches per environment are not expressible. The format is flat and
keyed on purpose, so that a later per-user layer can override single keys.

An unknown value or key, an empty `allow` entry, an environment without a branch, an invalid
branch name, or the same environment in two spellings (`Prod:` and `prod:`) stops `context`;
environment names are compared case-insensitively. The interface does not stop: it shows the list and reports the
switch permission as not checkable. Both list forms are read for `allow`.

Adding the block does not change `schema_version`: it is optional, and older installations
ignore unknown blocks. Nothing writes this block; it is edited by hand.

### `docs`

Optional block for the docs producers such as `/k-docs-code` (task 116).

| Field | Type | Meaning |
|---|---|---|
| `exclude` | list of strings | patterns excluded from the code scan, in addition to the fixed exclusions of the command text |

The patterns mean the same as in a `.gitignore` placed in `project.repo_root`, the scan root
of `/k-docs-code` -- for example `docs/` excludes every directory named `docs`, `vendor/x.md`
is bound to the root. Only the block form is read, one `- <pattern>` per line, indented
deeper than `exclude:` or -- equally valid YAML -- in the same column as `exclude:`; quotes
are removed, and a comment after an entry, empty entries and
duplicates are skipped. A one-line value such as `exclude: [docs/]` is not read but reported;
`exclude: []` is the empty list.

An entry that is absolute (leading `/` or `\`, a drive letter such as `C:\`, or `~/`) or that
leaves `project.repo_root` with `..` is not used: `context` reports it under
`docs.problems` with its reason and continues. A leading `/` counts as absolute even though
`.gitignore` would read it as anchored: the list names paths in the project, and an absolute
path must not silently pass as a pattern. A leading `!` is removed for this check. An entry
that contains a line break, carriage return or other control character (tab, NUL, DEL, NEL)
or a Unicode line or paragraph separator is rejected the same way: a pattern stands on one
line. On reading this only hits such a character in the middle of a line or as an escape in a
quoted entry.

`k-playbook context` and the briefing carry the list as `context.docs.exclude` and the
rejected entries as `context.docs.problems`. Commands read it from there, never from the file.
`/k-docs-code` applies it to every scan, including additional sources from `hints`; a path
given explicitly as the argument is scanned anyway. Without the block, `exclude` is empty and
nothing changes.

The block is written by `k-playbook docs exclude add <pattern>… [--json]` and the MCP tool
`k_playbook_docs_exclude_add` (`projectDir`, `patterns`), both over the same logic; a docs
command calls the tool only after the user confirmed saving. Each pattern is checked as when
reading and rejected with a reason if it is absolute, leaves `project.repo_root` or contains
a line break or other control character -- written raw, a line break would put a line outside
the block; a pattern already in the list is skipped; if nothing is new the file stays byte for byte. Otherwise the
new entries are inserted as lines of their own behind the last entry of the list, in its
column; every other line stays byte for byte -- entries already there (including rejected
ones), neighbouring keys such as `include:`, comments in and outside the block. A `docs:`
block without `exclude:` gets the key behind its last indented line; a missing block is
appended at the end. `exclude: []` becomes the block list (the `[]` goes, a comment behind it
stays). A form that cannot be extended safely -- a one-line value such as the flow form
`exclude: [docs/]`, `docs:` with a value in its header line or as a list, `exclude:` twice,
or lines under `exclude:` that are not `- <pattern>` in one column -- is not rewritten: the
answer is `unsupported` with the line and the reason, and nothing is written. The answer names `exclude` (the list afterwards), `added`, `rejected`
(`pattern`, `reason`), `changed`, and `config`, the file path.

Adding the block does not change `schema_version`.

### `kephalaion`

The project's binding to its Kephalaion collection (task 138). Hub alias and collection are
the same for everyone working on the project, so the block is versioned with it; Kephalaion
hands the hub alias out with the setup code, so it reads the same on every machine.

| Field | Type | Meaning |
|---|---|---|
| `hub` | string | alias of the hub entry at the node (`kephalaion status`) |
| `collection` | string | the project's collection at that hub; address for clients `<hub>:<collection>` |

Both follow Kephalaion's naming rule (lowercase, `a`–`z`, `0`–`9`, `.`, `_`, `-`, starting
with a letter or digit, at most 63 characters, no `system` prefix). They are read line by
line as the direct children of the block; quotes and a comment behind the value are removed.
Without the block, or with neither field, the binding is `not-configured`. The block names no
account: the account of this machine, its token file and the node's address come from
Kephalaion (`kephalaion node client`), see
[`installation.md`](./installation.md#kephalaion-collection). An `account:` line in the block
is not read; the health check warns about it.

The block is written by the setup step of the interface after a passed check and by the
project step of `k-playbook upgrade` when it moves the old machine-local binding. Both write
only the lines `hub:` and `collection:` -- replaced in place with a comment behind them kept,
or inserted behind the last indented line of the block -- and append a missing block with a
comment; every other line stays byte for byte, and no account is ever written. A block with a
value in its header line, the block twice, or `hub:`/`collection:` twice is not rewritten.
The file is versioned, so the change wants to be committed.

## Writing rules

- An existing `K-PLAYBOOK.yaml` is never overwritten. It belongs to the project and may contain values the tool does not know.
- The only thing the tool does unasked is the move from the former place `<project>/K-PLAYBOOK.yaml` into `.k-playbook/` ("Finding the anchor"); the content stays byte for byte.
- Write only after confirmation, step by step.
- The tool owns `schema_version` and `project.*`.
- The interface writes nothing under `tools:`; `tools.mcp` and `git` are only read. The one exception is removing the obsolete block `tools.gh` while setting up (section `tools.gh` (removed)); other blocks under `tools:` and their comments stay, and an empty `tools:` is removed. A new file has no `tools:` line.
- The remediation policy is set during onboarding; later `/k-remediation` may change it after asking. Only the `remediation:` block is written.
- `docs.exclude` is written only by `k-playbook docs exclude add` and `k_playbook_docs_exclude_add`, after the user confirmed; only lines for new entries are inserted into the `docs:` block, nothing in it is rendered again, and a form that cannot be extended safely is refused (`unsupported`). A missing block is appended; unindented comments before the next block belong to what follows and stay, such as the commented `docs:` example of a new file.
- Unknown top-level fields remain and are not changed unprompted. Writing occurs line by line so comments and order remain intact.
- Host-local installation states do not belong in this file. The Kephalaion binding is not
  one of them: hub alias and collection are the same for every machine and live in the
  `kephalaion:` block; the account, which differs per machine, comes from Kephalaion and is
  written nowhere. The interface writes only the lines `hub:` and `collection:` of that block,
  after a passed check. The machine-local `k-playbook-local/K-PLAYBOOK.local.yaml` of earlier
  versions is no longer read; `k-playbook upgrade` moves it and `/k-upgrade` deletes it.
- Nothing below `k-playbook/` may be written.

## Filename

The canonical filename is `K-PLAYBOOK.yaml`, and it sits in `.k-playbook/`. Do not
create `K-PLAYBOOK.yml`, so the tool and commands need to check only one name. A
`K-PLAYBOOK.yaml` directly in the project root is the former place; the tool moves
it ("Finding the anchor").
