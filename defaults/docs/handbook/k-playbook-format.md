# K-PLAYBOOK.yaml format

`.k-playbook/K-PLAYBOOK.yaml` is the configuration of a project that uses
k-playbook. It is also the **anchor**: the directory that holds
`.k-playbook/K-PLAYBOOK.yaml` is the project's root directory.

## Fundamental decision

The installation is the binary alone, one per machine. Nothing of the tool lies
in the project, and no configuration names a path to it. A project carries its
configuration and what it owns itself:

```text
<project>/                 arbitrary name
└── .k-playbook/
    ├── K-PLAYBOOK.yaml    the anchor
    ├── commands/          project-owned commands, committed (only when used)
    └── skills/            project-owned skills, committed (only when used)
```

Everything else the project owns lies in its Kephalaion collection and is named by a
`keph:` reference.

Because nothing shipped lies in the project, an update of the program cannot
touch anything the project owns. In the former model the installation was a
clone of the k-playbook repository under `<project>/k-playbook/`, beside the
configuration. The program no longer reads such a directory, and `/k-upgrade`
deletes one that is left (conversion 3.8).

The name of the project directory is irrelevant.

## No paths in configuration

Earlier versions had a `paths:` block with nine keys. It no longer exists. All
locations derive from the project root, the directory that holds
`.k-playbook/K-PLAYBOOK.yaml`:

| What | Where |
|---|---|
| shipped commands | `~/.local/share/k-playbook/commands/`, linked through `.k-playbook/share` |
| shipped skills | `~/.local/share/k-playbook/skills/`, linked through `.k-playbook/share` |
| shipped review recipes | `<hub>:k-playbook/defaults/reviews/` (Kephalaion, since task 131) |
| shipped guidelines | `<hub>:k-playbook/defaults/guidelines/` (Kephalaion, since task 131) |
| shipped instructions, assembled and on demand | `<hub>:k-playbook/defaults/instructions/always/`, `<hub>:k-playbook/defaults/instructions/on-demand/` (Kephalaion, since task 131) |
| scripts | `~/.local/share/k-playbook/scripts/` |
| security tool matrix | `~/.local/share/k-playbook/scripts/security-tools.tsv` |
| base tool matrix | `~/.local/share/k-playbook/scripts/base-tools.tsv` |
| shared script library | `~/.local/share/k-playbook/scripts/lib/` |
| project-owned commands | `.k-playbook/commands/` — setup does not create it; it appears when the project puts a command there |
| project-owned skills | `.k-playbook/skills/` — setup does not create it; it appears when the project puts a skill there |
| project-owned review recipes | `keph:/k-playbook/reviews/` — in the project collection |
| checks | none, neither shipped nor project-owned |
| review results | `<ergebnisort>` stands for the location of review results. It is not decided yet; reviews are paused until then. |
| results kept to present | `keph:/results/` — in the project collection, project-wide, `.md` only, names start with the date (`<YYYY-MM-DD>-<name>.md`); written only when the user asks or a command provides for it, never as a duty, and cleaned up by the user. The rules are `k-playbook:/instructions/on-demand/ergebnisse.md`. Not the same as `<ergebnisort>`, where `/k-review` and `/k-audit` write |
| project documentation | `keph:/docs/` — in the project collection; the briefing names the entry under `projectDocs`. `/k-docs-code` writes to `keph:/docs/generated/code/` and `/k-docs-tools` to `keph:/docs/generated/libs/`, both through `k-playbook docs checkout|publish` |
| tool profiles | `keph:/docs/generated/libs/` — in the project collection, written by `/k-docs-tools` through `k-playbook docs checkout|publish libs` |
| version inventory | `keph:/docs/generated/versions/inventory.md` — in the project collection |
| manually maintained docs | `keph:/docs/manual/` — in the project collection |
| project-owned guidelines | `keph:/docs/guidelines/` — in the project collection. How guidelines are read from the collection is todo #40 |
| project-owned instructions | `keph:/k-playbook/instructions/always/`, `keph:/k-playbook/instructions/on-demand/` — in the project collection |
| project-owned includes | `keph:/k-playbook/includes/` — in the project collection |
| open tasks | `keph:/user/<user>/tasks/` — in the project collection; form in `k-playbook:/instructions/on-demand/tasks.md` |
| completed tasks | `keph:/user/<user>/tasks/done/` — in the project collection |
| findings | `keph:/queue/findings/` — in the project collection |
| project todos | `todos/` in the project collection |
| version sources | `keph:/k-playbook/inventory.md` — in the project collection, settings in the frontmatter |

A key whose value is always the same would only be a source of errors.
Commands therefore no longer guess or read paths: they derive them.

This also applies to project documentation. Previously, `paths.docs` was the
only value allowed to point out of the k-playbook directory with `../`, so a
project could continue using its existing docs. This special case is removed:
the project documentation lives in `keph:/docs/`, and other documentation
maintained by a project remains untouched.

The docs producers do not write to disk: `/k-docs-code` and the
`ks-overlay-repo-analyse` skill go through `k-playbook docs checkout|publish code` into
`keph:/docs/generated/code/` (task 109), `/k-docs-tools` through
`k-playbook docs checkout|publish libs` into `keph:/docs/generated/libs/` (task 120).
Within `docs/`, the directory reveals the origin: a tool writes exclusively to
directories of its own origin, and the frontmatter field `generated.by` identifies which
tool wrote an individual file. The `README.md` of each directory that has a template
`k-playbook:/docs/<path>/README.md` (`defaults/docs/` in the source repo, shipped to the hub)
is written by the program from that template, the same in every project — by `publish`,
the inventory, setup and `k-playbook upgrade`, and `k-playbook docs readme` (task 137).

The source configuration of the version inventory is not created by setup. Since
task 144 it is the document `keph:/k-playbook/inventory.md` in the project collection, its
settings in the frontmatter (`schema_version: 3`, `sources`, `exclude`, `helm_values`;
`roots` no longer exists, only paths below the project root are read); `/k-doc-inventory`
creates it on the first confirmed addition, and without it the default sources apply. It
is maintained manually and never overwritten by an update. Its status appears as
`environment.versionSources` in the partial call of the briefing (`ref`, `state`), so no command
reads it itself. The interface only displays the configuration in
the "Inventory" section, showing the document, its state, and the count of sources,
exclusions, and configured Helm values; it does not edit it.
That section also offers the inventory prompt, the same run as `k-playbook
inventory` and `/k-doc-inventory`, writing into the project collection. The contract is in
[`version-inventory.md`](./version-inventory.md); after a version jump in the current pass
the inventory is updated right away through this run, and the profiles are queued
(`erzeuger: tools`).

## Finding the anchor

The procedure applies equally to the tool and to an assistant:

1. If a directory was passed, it applies; check `<arg>/.k-playbook/K-PLAYBOOK.yaml`.
2. Otherwise search upwards from `realpath(CWD)`, one candidate per level: `<dir>/.k-playbook/K-PLAYBOOK.yaml`.
3. On finding it: `PROJECT_DIR = <dir>`.
4. `$HOME` and `/`, inclusive, are the boundary of the upward search.
5. If nothing is found, report that no project is set up there. Do not guess or create anything.

The anchor is the **file** `.k-playbook/K-PLAYBOOK.yaml`, not the directory
`.k-playbook/`. The link maintenance creates the directory even without a
configuration (for the `share` link), so a `.k-playbook/` can exist where no
project is. The k-playbook repository itself no longer versions anything under
`.k-playbook/` — in its own checkout that directory belongs to a separate private
development repository. A clone `<project>/k-playbook/` left over from the former
model can still carry a `.k-playbook/`, from an older state of the repository or
from the link maintenance. The conclusion stands: a search for the bare directory
could find a project where there is none.

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

The upward search must **not** stop at the Git worktree root. The project
repository can lie below the project root (`project.repo_root`) and is then its
own worktree; a search beginning there would otherwise never reach the
configuration one level above.

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
fallback to the shipped layer alone.

The origin values stay `dist`, `local`, `override` and `disabled`. A project-owned part from
the collection carries its place as well: the field `location` with the `keph:` reference
(`keph:/k-playbook/reviews/review-x.md`) in the briefing, in `k-playbook read --json`, in
`k_playbook_doc_read` — for an assembled directory per part; the plain-text briefing shows it
next to the origin (`[local keph:/k-playbook/…]`).

The kind for rules is gone since task 135: nothing is shipped, read or listed for it.

Checks no longer exist since task 155: k-playbook ships none, there is no catalog `checks`
and no runner `k-check`. Project-owned checks were gone before (task 134).

Project-owned commands and skills live in `.k-playbook/commands/` and `.k-playbook/skills/`,
versioned next to the configuration.

For commands, it is the path below `commands/`, including the namespace: a local
`commands/_shared/gh.md` replaces exactly that file, while the rest of
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

Disable a project-owned file by deleting it.

`README.md` in one of these directories is never an entry, nor are dotfiles.

`scripts/lib/` is not an entry either: it contains shared code next to executable
scripts. Today it contains `install-common.sh`, which
`install-security-tools.sh` and `install-base-tools.sh` both source. Both use
the same release path and apply the same guard to the installation destination.
A second copy would undermine the promise that the security matrix asset
patterns continue resolving to the same asset unchanged as soon as one resolver
changes without the other.

For skills, `SKILL.md` decides disabling: if it is empty, the skill is disabled
and not registered.

To learn what ultimately applies, do not query the filesystem but the tool:

```bash
k-playbook briefing
```

Its output lists the instructions, reviews and guidelines that apply, already merged, and
names the origin of every entry that is not shipped: `local`, `override`, or `disabled`
(`--json` carries `origin` for every entry, `dist` included). See [The
briefing](#the-briefing).

Because commands and skills come from two sources, assistant destinations,
`.claude/commands`, `.claude/skills`, and `.opencode/commands`, are real
directories with **one symlink per entry**, not a
directory symlink: it would point at exactly one source.

### What exists only once

Everything else has no counterpart on the other side:

| | Directories |
|---|---|
| project-owned only | in the project collection: `docs/`, `queue/`, `results/`, `todos/`, `user/` |
| shipped only | `docs/`, `scripts/`, `bin/`, `installer/` |

The two `docs/` are not a pair: the shipped handbook (`k-playbook:/docs/handbook/`, in the
source repository `defaults/docs/handbook/`) documents k-playbook itself, `keph:/docs/` the
project. They are two different objects and nothing to merge.

`guidelines/` is not a pair either (task 148): the kind exists shipped only, in
`<hub>:k-playbook/defaults/guidelines/` (since task 131; before, `k-playbook/guidelines/`).
`k_playbook_doc_read` (`k-playbook:/guidelines/<name>.md`) and the briefing
(`documents.guidelines`) resolve only that layer, so the origin is always `dist`.
Project-owned guidelines live in `keph:/docs/guidelines/`; how a session is pointed to
them is todo #40. From task 086 (2026-10-16) until task 148 the kind was a pair, merged
by the same rule as `reviews/`.

Nothing shipped may be changed. `~/.local/share/k-playbook/` belongs to the
program and is replaced when the program is updated; the shipped texts on the
hub are replaced by the next release. A clone `k-playbook/` left over from the
former model is not written to either; `/k-upgrade` deletes it.

## The briefing

```bash
k-playbook briefing
```

The briefing (task 086, 2026-10-16) is the one call at the start of a session:
`k-playbook briefing [--json]` and the MCP tool `k_playbook_briefing`. It says what a
session has to know: the date and the project (`now`, `project`), whether the project's
collection is reachable (`kephalaion`), which instructions, reviews and guidelines apply, as
references in the prefix scheme with title, description and origin
(`k-playbook:/instructions/always/` with its parts, `on-demand/`, `reviews/`, `guidelines/`),
what is pending in the project's collection -- own open tasks and workbench topics, at most 10
each, and the number of open findings --, which service of the personal GitHub access file
belongs to the repository, and what is wrong with the environment (`health`). References,
never paths; no contents, with one exception: the text of the always-instructions itself
(`documents.always.content`; the plain text starts with it after its head: session, the lines
`now`, `project` and `kephalaion`, read-in-full request, titles of the parts), assembled as
`k-playbook read k-playbook:/instructions/always/` assembles it, because every session reads
it first. Documents go through the same overlay as references (`overlayDocDir` in
`project/docref.go`), so an entry has the same origin wherever it appears. This means no
command has to apply overlay rules itself. There is one answer, and everyone receives the
same one.

Environment facts only single commands need -- `gh`, `github`, `baseTools`,
`versionSources`, `docs`, `remediation` -- are not in the briefing. A command fetches them
where it needs them, with the partial call `k-playbook briefing --part environment` (MCP:
`k_playbook_briefing` with `part: "environment"`), and reads them as `environment.<field>`.
The partial call assigns no session identifier and is no reload of the briefing.

`links` is normally absent, intentionally: the briefing brings assistant
linking in line with the catalog, as described below, and a message that says
the same thing on every invocation is ignored. If it is present, look at it:
`healed` names commands and skills that have just been registered. They exist
on disk, but the running assistant read its list at startup and does not yet
know them; `note` says exactly that. `open` names destinations that cannot be
resolved automatically: a real project file in the way, or a conflict in
`CLAUDE.md`.

`now` is present for another reason: commands place dates into persistent files,
review logs, result directories, and summary-file names. Not every assistant's
host tells it today's date, and a guessed date in a log is worse than none. It
is the only field that ages: it identifies invocation time, not write time.

The search starts at the working directory and proceeds upward. Without
`.k-playbook/K-PLAYBOOK.yaml`, the call stops with a message; a `schema_version` other than
`3` is a state under `project` (`available: false` with `message`), and nothing is linked
then. Contract: [mcp.md](mcp.md#briefing-contract).

## The resolved working state

The context view of the interface (`GET /api/context`, the block that loads when it is
opened) shows the resolved working state, built by `BuildContext`; the review tools read their
catalog from the same function (`BuildContextWithLayers`). The subcommand `k-playbook context`
and the MCP tool `k_playbook_context`, which output it before, were removed with task 188: a
call of the subcommand ends with one sentence on stderr naming `k-playbook briefing` and
`k-playbook briefing --part environment`, and exit code 1. What commands need comes with the
briefing and its partial call `environment`. The paused commands `/k-review` and `/k-audit`
took `catalogs.reviews` with the review modes from that output; they carry a `NACHZIEHEN` note
for their rebuild.

The view's answer carries under `context`:

| Field | Content |
|---|---|
| `schemaVersion` | the validated configuration version |
| `now` | invocation time: `date` as `YYYY-MM-DD`, `timestamp` as RFC 3339 |
| `project` | project root, `repoRoot`, `vcs`, configuration location |
| `remediation` | the policy, including a default when the block is missing |
| `docs` | the `docs:` block: `exclude` is the list of valid patterns from `docs.exclude` (empty without the block), `problems` the rejected entries, each with `pattern` and `reason` |
| `gh` | the GitHub CLI host finding for this machine |
| `git` | the `git:` block: `switch` (`unknown`, `offer`, `off`), `allow`, `environments` as a list of `{name, branch}` in file order, and `configured`, which says whether the block was present |
| `mcp` | the required MCP servers: `required` is the list from `tools.mcp.required`, `configured` says whether the block was present; an object so further MCP settings can join without renaming the field |
| `catalogs` | `reviews`, merged with the project-owned side; the catalog `checks` is gone since task 155. The shipped side of `reviews` comes from `<hub>:k-playbook/defaults/` (task 131), the project-owned side from `keph:/k-playbook/reviews/` in the project collection (task 143) |
| `catalogsProblem` | only when a layer of `reviews` cannot be read (Kephalaion not ready, `<hub>:k-playbook` missing or `defaults/` empty, the project collection not readable): why that list is empty -- it is never shown from one layer alone; the rest of the output stays, and `kephalaion` says more |
| `kephalaion` | the project's Kephalaion binding and the result of its check: `configured`, `hub`, `collection`, `account`, `address` (`<hub>:<collection>`), `state`, `ok`, `message`; when `ok`, also `rights`, `scopes`, `user`, `node`, `nodeVersion` and `sync` |

Each catalog entry has `name` (the filename), `key` (the invocation name without
extension and kind prefix), `path`, `origin` (`dist`, `local`, or `override`), `location` for a project-owned recipe
(its `keph:` reference under `keph:/k-playbook/reviews/`, also when it disables), and
`disabled`, where applicable. `path` for every recipe —
shipped, project-owned or overriding — is the effective reference
`k-playbook:/reviews/<name>`, read with `k_playbook_doc_read` or `k-playbook read`, because
both layers lie in collections (tasks 131 and 143).

From task 091 (2026-10-17) until task 187 the briefing carried the rest of this output in a
transitional section `context`. That section is gone: `now`, `project`, `kephalaion` and
`links` are sections of the briefing, `gh`, `github`, `baseTools`, `versionSources`, `docs`
and `remediation` come with its partial call `environment`, and `schemaVersion`, `mcp`, `git`
and the further fields of `kephalaion` and `gh` have no reader and no place there.

`kephalaion` takes hub and collection from the `kephalaion:` block of
`.k-playbook/K-PLAYBOOK.yaml` (see `kephalaion` under "Fields"); the account, its token file
and the node's address come from Kephalaion's client information (`kephalaion node client
--json --format 1 --no-probe --hub <alias>`), and `choice` names where the account comes from
(`override`, `default` or `only`). The check asks the node of this machine over MCP
(`whoami`, with the header pair `X-Keph-Account-<hub>`/`X-Keph-Token-<hub>` and the token
from Kephalaion's own file `<tokensDir>/<hub>/<account>.token`); it runs only when the block
is present and never stops the call — every failure is a `state` with its own `message`,
which names the remedy: `not-configured`, `invalid-entry`, `kephalaion-missing`,
`client-failed`, `node-address`, `account-choice-missing`, `token-missing`,
`node-unreachable`, `hub-unknown`, `hub-never-synced`, `login-invalid`, `login-missing`,
`replica-unreadable`, `collection-unreadable` or `ok`. `sync` carries `lastSuccess`,
`revision`, `lastError` and `neverSynced` as the node reports them. The field is filled by the
interface from the cache of its status page; internal callers that only need the catalogs do
not ask the node.
Details: `installer/docs/architecture.md`, section "Kephalaion-Anbindung".

`gh` is a finding for this particular machine only: `installed`, `path`,
`loggedIn`, `account`, `accounts`, and `tokenFromEnv`. `ready` summarizes what a
command needs to know: gh exists and an account is configured. There is no
project decision about gh: whether gh is used follows from the personal GitHub
access file (`k-playbook:/docs/handbook/github-access.md`) and, without a service for the repository,
from `ready`. The partial call `environment` carries `ready` and `account` only.

The finding is read from gh configuration, not checked against the server: a
stored token may have expired. If certainty is needed, run `gh auth status`;
that requires network access and does not belong here.

The invocation is deliberately cheap: it omits the security-tool preflight,
because it starts `--version` for every tool and takes noticeable time. The gh finding costs
nothing: it only checks `PATH` and `~/.config/gh/hosts.yml`, without a subprocess.

The interface uses the project it was started for. Without `.k-playbook/K-PLAYBOOK.yaml` the
view stays empty; a `schema_version` other than `3` stops `BuildContext`, and the view shows
the message instead.

## Instructions

What an assistant must read before working comes with the briefing: the text of
`k-playbook:/instructions/always/`, assembled from two levels.

| Where | Applies to | On update |
|---|---|---|
| `<hub>:k-playbook/defaults/instructions/` | every project using k-playbook | the hub fetches the new release |
| `keph:/k-playbook/instructions/` in the project collection | this project only | remains |

The shipped parts come first, the project-owned ones after them; a project-owned
document of the same name replaces the shipped one, an empty one disables it (see
"Merge shipped and project-owned content").

The entry point for a session is `k-playbook:/instructions/on-demand/k-playbook.md`: it
sums up the layout, how instructions are assembled, overridden and disabled, and how an
`AGENTS.md` is split up. The always part `030-briefing.md` tells a session to read it
before it answers questions about k-playbook or creates, changes, moves or disables a
project rule.

`AGENTS.md` receives only a **prompt**: a short block naming the one call of a
session, `k-playbook briefing`, with the MCP tool `k_playbook_briefing` as its
equivalent — exactly the `AGENTS.md` of the source repository. If the file does
not exist, it is created from that template; if it exists, the block is appended
without touching existing content. A marker, `<!-- k-playbook:anstoss -->`,
prevents a second run from appending it again. A block that still carries a
superseded call (`k-playbook context`, before that the wrapper
`k-playbook/bin/k-playbook context`) is replaced by the current one. Replaced is
only what can be bounded positively (the shipped shape line by line, else the next
heading or HTML comment, never the end of the file); project text before and
behind the block stays, and a block whose end cannot be determined stays with a
hint. This runs at setup, at the start of the interface and with
`k-playbook upgrade`.

`k-playbook upgrade` is the one catch-up in the terminal, the subcommand behind
`/k-upgrade`: first Kephalaion, then the program — the release binary of the
newest release, only when the running program is older than it; a clone
`k-playbook/` is neither pulled nor read (task 156) —, then the project: `AGENTS.md`, `CLAUDE.md` as the
include file, the linking of commands and skills, the MCP entries. After a
binary change the newly installed program performs the project step; after
that the calling program, when `opencode service status` reports a running
OpenCode service, reloads the service's MCP servers with `opencode reload`; the message ends with the hint to restart the
session, which `/k-upgrade` drops in OpenCode after such a reload unless another
reason for a restart is reported, in particular one from Kephalaion; `--check` writes nothing
and reports what a run would do. It never touches project prose in `AGENTS.md`.

Claude Code does not read `AGENTS.md` automatically. Setup therefore creates a
neighboring `CLAUDE.md` containing only the import line `@AGENTS.md`: a regular
file, not a symlink. [`installation.md`](./installation.md#an-existing-claudemd)
explains how the pair is classified and what happens to a supplied `CLAUDE.md`.

## Minimal format

This is what the tool creates when a new project is connected:

```yaml
# k-playbook
#
# The location of this file determines the project's root directory:
# the directory above .k-playbook/.

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

maintenance:
  # Maintenance run `k-playbook maintain`: daily is the daily programme without --only.
  # backup mirrors keph:/docs/ into target (relative to the project root, a
  # folder there reserved for the backup). Applies to all users; with
  # several people git.commit stays false, and only the person in charge
  # runs `k-playbook maintain --commit`.
  daily:
    - docs-backup
  backup:
    enabled: false
    source: docs
    target: keph-backup/docs
    git:
      commit: false
```

The `docs:` block is written as a comment only: without it nothing changes. The
`maintenance:` block is written with the backup switched off (section
[`maintenance`](#maintenance)). The comments in the file the tool writes are German; they are
translated here.

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

maintenance:
  daily:
    - docs-backup
  backup:
    enabled: true
    source: docs
    target: keph-backup/docs
    git:
      commit: false

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
in the project root and no paths in configuration. Neither the move of the anchor
from `<project>/K-PLAYBOOK.yaml` into `.k-playbook/` nor the end of the clone
`k-playbook/` beside it (task 156) changed the number: the file and its fields
stay the same.

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
- `app` or another directory name when the code is checked out in a subdirectory of the project root, such as in a DevContainer.

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
`environment.remediation` (the partial call of the briefing) reports `mode: task-first`
alongside `configured: false`, and `configured` is the field that decides: `/k-remediation`
must not guess a mode but explicitly ask for the current session, or have the policy set
through `/k-gui`. `configured` belongs to that output, not to `K-PLAYBOOK.yaml`;
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
`environment.gh.ready` from the partial call of the briefing. An existing block is ignored, also with an
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
must match `^[A-Za-z0-9][A-Za-z0-9._-]*$`; any other name is an error when the
configuration is read as a whole. The interface does not stop: the **/mcp-servers**
page shows the required list as unreadable with the error, its matrix pill
warns, and a server's detail page reports "Pflicht: unbekannt" instead of "no".
Only `required` directly under `tools.mcp` counts; a `required` key deeper
inside `mcp` is not the list.

The block is optional. Without it, nothing is required; the **/mcp-servers** page of the
interface then shows a snippet to copy. With it, the page marks every required name that is missing
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
branch name, or the same environment in two spellings (`Prod:` and `prod:`) is an error when
the configuration is read as a whole; environment names are compared case-insensitively. The interface does not stop: it shows the list and reports the
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
leaves `project.repo_root` with `..` is not used: it is reported under
`environment.docs.problems` with its reason, and reading continues. A leading `/` counts as absolute even though
`.gitignore` would read it as anchored: the list names paths in the project, and an absolute
path must not silently pass as a pattern. A leading `!` is removed for this check. An entry
that contains a line break, carriage return or other control character (tab, NUL, DEL, NEL)
or a Unicode line or paragraph separator is rejected the same way: a pattern stands on one
line. On reading this only hits such a character in the middle of a line or as an escape in a
quoted entry.

The partial call `environment` of the briefing carries the list as `environment.docs.exclude`
and the rejected entries as `environment.docs.problems`. Commands read it from there, never from
the file. `environment.docs.exclude` also carries the backup target of the [`maintenance`](#maintenance)
block whenever that block is present, its target passes the checks and lies below
`project.repo_root`, enabled or not: as an entry anchored at `project.repo_root` and marked as
a directory, for example `/keph-backup/docs/` or `/sub/keph-backup/docs/` when the project
root lies below the repository root. The export is not source code, so it stays out of the
code scan, and a change inside it queues no documentation job. This entry is computed and
never written into `docs.exclude` of the file; the leading `/` that the file rejects as
absolute is intended here. A target outside `project.repo_root` (in the project root next to
a repository checked out in a subdirectory) gets no entry, because the form is bound to
`project.repo_root`; the regular scan does not reach it, but a scan given an additional path
(the argument of `/k-docs-tools`; `/k-docs-code` takes no argument since task 167) may cover
the mirror, and a change in it may then queue a documentation job. That gap is accepted.
`/k-docs-code` applies the list to every scan, including additional sources from `hints` and
the paths of a documentation order. It takes no argument any more, so a path excluded here
cannot be reached for `code`; a hidden path still gets in through a `hint`. For
`/k-docs-tools` a path given explicitly as the argument is scanned anyway. Without the block,
`exclude` is empty and nothing changes.

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

The block is written by the setup step of the interface after a passed check. It writes
only the lines `hub:` and `collection:` -- replaced in place with a comment behind them kept,
or inserted behind the last indented line of the block -- and appends a missing block with a
comment; every other line stays byte for byte, and no account is ever written. A block with a
value in its header line, the block twice, or `hub:`/`collection:` twice is not rewritten.
The file is versioned, so the change wants to be committed.

### `maintenance`

The maintenance run `k-playbook maintain` (task 161): the daily programme and the backup of
the project documentation from Kephalaion into a folder of the project root. Optional; a new
file carries it with the backup switched off, and `k-playbook upgrade` adds what is missing.

| Field | Type | Default | Meaning |
|---|---|---|---|
| `daily` | list of step names | `docs-backup` | the steps a run without `--only` executes |
| `backup.enabled` | `true`/`false` | `false` | whether `docs-backup` exports at all |
| `backup.source` | string | `docs` | what is exported; only `docs` (everything under `keph:/docs/`, recursively) |
| `backup.target` | path | `keph-backup/docs` | the folder, relative to the project root (`project.dir`) |
| `backup.git.commit` | `true`/`false` | `false` | commit the target after a complete export, as if `--commit` were given |

Hub and collection come from the [`kephalaion`](#kephalaion) block; there is no second
address. Queues, tasks, workbench and other collection content are not exported. The backup
is a mirror of the documentation in Git, not a backup of the collection and no replacement for
database or hub backups.

The block is read line by line like `docs:`. Only the block list is read for `daily`, one
`- <step>` per line; a one-line value such as `daily: [docs-backup]` is reported, not read as
empty, and `daily: []` is the empty programme. Without `daily` the fixed default
`docs-backup` applies. A value other than `true`/`false`, a `source` other than `docs`, a
mapping written on one line (`git: {commit: true}`), a key twice, tabs in the indentation or
children in different columns are reported with their line; a maintenance run then starts no
step. Unknown keys stay and are ignored.

`enabled` and `git.commit` hold for everybody who works on the project, like the whole file.
In a project with several people `git.commit` stays `false`, and only the person in charge
(or their cron job) runs `k-playbook maintain --commit`. With `enabled: true` everybody who
runs `maintain` exports; whoever does not commit then has changed versioned files under the
target in their working tree, and a later `git pull` of the backup commit may fail on them.

**The target** is resolved against the project root. After resolving symlinks (for a target
that does not exist yet, on its nearest existing ancestor) it must lie strictly below the
project root (`project.dir`); it may lie inside `project.repo_root` or, when the repository is
checked out in a subdirectory, next to it. The export needs no Git repository: without a
requested commit the run calls no Git at all and checks no Git state, so the target may lie
outside any repository or be ignored by Git. Whether the target is kept out of the code scan
depends only on its path (section [`docs`](#docs)). The following checks look at paths only. The
target must not be the project root, `project.repo_root`, `.git` (of the project root or
of `project.repo_root`) or `.k-playbook/`, must not be an ancestor of any
of them and must not lie inside `.git` or `.k-playbook/`; an absolute
path or one starting with `~` is rejected. The rejection mentions symlinks only when
resolving them actually moved the target. The folder is reserved for the export: the first
run takes place only if the target is missing or empty and then creates the marker file `.k-playbook-backup` in
it. Every later run requires the marker, and every export passes
`--exclude .k-playbook-backup`, so `--delete` never removes it. A non-empty target without
marker stops the step before the export.

**`k-playbook maintain [--only <step>[,<step>…]] [--dry-run] [--commit]`** runs the steps
without AI and without asking, so a cron job can call it later; without a terminal it
behaves the same. The steps run in the fixed order the code keeps, whatever order `--only`
or `daily` names them in; today there is only `docs-backup`. An unknown name in `--only` or
`daily` is rejected before anything is written, and the message names the valid ones. Each
step reports `erfolgreich`, `übersprungen` or `Fehler` with a reason, then the overall result;
the exit code is 0 without a failed step (skipped is no failure), 1 once a step fails, 2 for
an invalid call or an unknown step.

`docs-backup` does nothing and reports `übersprungen` when the backup is off or the block is
missing. Otherwise it needs `kephalaion.ok` as in the briefing, checks the target and
its marker, locks the target against a parallel run (a second run on the same target is
rejected) and calls, without a shell,

```sh
kephalaion node dir pull <hub>:<collection> docs <target> --delete --exclude .k-playbook-backup
```

as a client of the local node with this machine's default account. `--dry-run` passes
`--dry-run` on, creates no marker, writes no file, configuration or Git state and says
whether a commit would be requested. The exit code of the export decides:

| Code | Kephalaion | Report of the step |
|---|---|---|
| 0 | done, complete | success; a commit may follow |
| 1 | error (preflight, node unreachable, forbidden, not readable) | error with Kephalaion's message, no commit |
| 2 | wrong call | "Kephalaion zu alt oder falscher Aufruf → /k-upgrade", no commit |
| 3 | incomplete (aborted, timeout, reported conflicts) | "partiell, erneut ausführen, kein Commit" |

Every code other than 0, an interruption (SIGINT or SIGTERM is passed on to Kephalaion once)
and an open outcome may leave a partly updated target; the step says so and never commits.
After exit 0 it commits only if `--commit` or `git.commit: true` asks for it: `git add -A`
and `git commit --only`, both limited to the target, so changes and deletions in the target
are committed while changes already staged elsewhere stay staged and the rest of the working
tree is untouched. No change means no commit; there is no push, reset or stash. Git is
called only here, for a requested commit. The commit goes into the repository the target lies
in, found with `git rev-parse --show-toplevel` in the target (or its nearest existing
ancestor), not into `project.repo_root` as such, wherever that repository's root lies: the
project root, a repository checked out in a subdirectory, one above the project root, or the
target itself. When the target is not inside a Git repository, or Git ignores it, a requested
commit is a reported error, and the export has run anyway; the message names the target (or
its nearest existing ancestor), and for an ignored target the source of the rule as
`git check-ignore -v` reports it (file and line, such as `.git/info/exclude:4`). A failing
commit (a hook, for instance) leaves the target's changes staged.

**Upgrade.** `k-playbook upgrade` (and setting up in the interface) adds the block with the
defaults above when it is missing, and only the missing fields when it is partly there. It
never overwrites a value, an explicit `enabled: false` included, keeps foreign keys and
comments byte for byte and changes nothing on a second run. A block it cannot place
unambiguously (see the reported forms above) is reported, not written.
`k-playbook upgrade --check` reports the addition as `würde ergänzt` and writes nothing. The
file is versioned, so the addition wants to be committed.

## Writing rules

- An existing `K-PLAYBOOK.yaml` is never overwritten. It belongs to the project and may contain values the tool does not know.
- The only thing the tool does unasked is the move from the former place `<project>/K-PLAYBOOK.yaml` into `.k-playbook/` ("Finding the anchor"); the content stays byte for byte.
- Write only after confirmation, step by step.
- The tool owns `schema_version` and `project.*`.
- The interface writes nothing under `tools:`; `tools.mcp` and `git` are only read. The one exception is removing the obsolete block `tools.gh` while setting up (section `tools.gh` (removed)); other blocks under `tools:` and their comments stay, and an empty `tools:` is removed. A new file has no `tools:` line.
- The remediation policy is set during onboarding; later `/k-remediation` may change it after asking. Only the `remediation:` block is written.
- `docs.exclude` is written only by `k-playbook docs exclude add` and `k_playbook_docs_exclude_add`, after the user confirmed; only lines for new entries are inserted into the `docs:` block, nothing in it is rendered again, and a form that cannot be extended safely is refused (`unsupported`). A missing block is appended; unindented comments before the next block belong to what follows and stay, such as the commented `docs:` example of a new file.
- The `maintenance:` block is only added to: setup and `k-playbook upgrade` insert missing fields with their defaults (backup off) behind the last line of their parent, never overwrite a value and write nothing when the block cannot be placed unambiguously; a missing block is appended. `k-playbook maintain` never writes the file.
- Unknown top-level fields remain and are not changed unprompted. Writing occurs line by line so comments and order remain intact.
- Host-local installation states do not belong in this file. The Kephalaion binding is not
  one of them: hub alias and collection are the same for every machine and live in the
  `kephalaion:` block; the account, which differs per machine, comes from Kephalaion and is
  written nowhere. The interface writes only the lines `hub:` and `collection:` of that block,
  after a passed check.
- Nothing shipped is written: neither `~/.local/share/k-playbook/` nor a leftover clone `k-playbook/`.

## Filename

The canonical filename is `K-PLAYBOOK.yaml`, and it sits in `.k-playbook/`. Do not
create `K-PLAYBOOK.yml`, so the tool and commands need to check only one name. A
`K-PLAYBOOK.yaml` directly in the project root is the former place; the tool moves
it ("Finding the anchor").
