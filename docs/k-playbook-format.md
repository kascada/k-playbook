# K-PLAYBOOK.yaml format

`K-PLAYBOOK.yaml` is the configuration of a project that uses k-playbook. It is
also the **anchor**: its location determines the project's root directory.

## Fundamental decision

Every project carries its own installation. There is no central base
installation and no fixed host path. The installation sits beside the
configuration:

```text
<project>/                 arbitrary name
├── K-PLAYBOOK.yaml        the anchor
├── k-playbook/            the installation, fully replaceable
└── k-playbook-local/      project-owned, committed
```

Because configuration is **beside**, rather than **in**, the installation,
`k-playbook/` contains nothing project-owned. The directory can therefore be
updated completely, both with `git pull` and with `rm -rf` followed by a fresh
clone.

The playbook directory is always named `k-playbook`. The name of the project
directory above it is irrelevant.

## No paths in configuration

Earlier versions had a `paths:` block with nine keys. It no longer exists. All
locations derive from the location of `K-PLAYBOOK.yaml`:

| What | Where |
|---|---|
| shipped commands | `k-playbook/commands/` |
| shipped skills | `k-playbook/skills/` |
| shipped rules | `k-playbook/rules/` |
| shipped review recipes | `k-playbook/reviews/` |
| shipped checks | `k-playbook/checks/` |
| check runner | `k-playbook/bin/k-check` |
| scripts | `k-playbook/scripts/` |
| security tool matrix | `k-playbook/scripts/security-tools.tsv` |
| base tool matrix | `k-playbook/scripts/base-tools.tsv` |
| shared script library | `k-playbook/scripts/lib/` |
| project-owned rules | `k-playbook-local/rules/` |
| project-owned review recipes | `k-playbook-local/reviews/` |
| project-owned checks | `k-playbook-local/checks/` |
| review results | `k-playbook-local/results/` |
| project documentation | `k-playbook-local/docs/` |
| tool profiles | `k-playbook-local/docs/libs/` |
| version inventory | `k-playbook-local/docs/versions/` |
| manually maintained docs | `k-playbook-local/docs/manual/` |
| raw material | `k-playbook-local/material/` |
| guidelines | `k-playbook-local/guidelines/` |
| open tasks | `k-playbook-local/tasks/` |
| completed tasks | `k-playbook-local/tasks/done/` |
| machine files owned by k-playbook | `k-playbook-local/data/` |
| derived, disposable content | `k-playbook-local/cache/` |
| project todos | `k-playbook-local/data/todos.json` |
| version sources | `k-playbook-local/version-sources.yaml` |
| private content | `k-playbook-local/priv/` |
| instructions, shipped | `k-playbook/k-playbook.md` |
| instructions, project-owned | `k-playbook-local/k-playbook.md` |

A key whose value is always the same would only be a source of errors.
Commands therefore no longer guess or read paths: they derive them.

This also applies to project documentation. Previously, `paths.docs` was the
only value allowed to point out of the k-playbook directory with `../`, so a
project could continue using its existing docs. This special case is removed:
`/k-docs-code` writes to `k-playbook-local/docs/code/`. `docs/` is organized
into subdirectories by origin; the directory does not say how many tools write
to an origin. Other documentation maintained by a project remains untouched:
k-playbook claims only its own directory.

Within `k-playbook-local/docs/`, the directory reveals the origin, which in
turn determines the ownership rule:

Every subdirectory of `docs/` represents an origin, not a single tool. A tool
writes exclusively to directories of its own origin; the frontmatter field
`generated.by` identifies which tool wrote an individual file. `docs/README.md`
belongs exclusively to `/k-docs-index`. No command writes doc files in
`docs/manual/`; the structure README created during setup is excepted. Flat
`docs/*.md` files from before this structure have no producer: no command writes
them, they are only listed.

`docs/code/`, `docs/libs/`, `docs/extracted/`, and `docs/versions/` are created
on the first run of a tool from their origin: `/k-docs-code` and the
`ks-overlay-repo-analyse` skill write to `docs/code/`, `/k-docs-tools` to
`docs/libs/`, `/k-docs-extract` to `docs/extracted/`, and `/k-doc-inventory` to
`docs/versions/`. Setup does not create them; it creates `docs/manual/` and
`material/`.

In contrast, setup creates the version inventory source configuration,
`k-playbook-local/version-sources.yaml`, as a valid empty configuration. It is
maintained manually and never overwritten by an update. Its status appears in
the `versionSources` section of `k-playbook context`, so no command reads it
itself. The interface only displays it in the "Inventory" section, showing the
path, status, and count of roots, sources, and exclusions; it does not edit it.
That section also offers the inventory prompt, the same run as `k-playbook
inventory` and `/k-doc-inventory`. The contract is in
[`version-inventory.md`](./version-inventory.md), and the requirement to update
version jumps is in `rules/docs-sync.md`.

`k-playbook-local/material/` is the source side: raw material such as chat
transcripts and notes. It is never indexed and no command writes to it. Like
`priv/`, its contents are versioned normally; k-playbook does not write a
`.gitignore` for it. Raw material commonly contains tokens, paths, and names;
the project decides whether it should therefore stay outside the repository.

Four directories are optional in this respect: `results/`, `cache/`, `priv/`,
and `material/`. `results/` and `cache/` are made private during installation by
default: review results are the state of one machine, not project knowledge, and
everything under `cache/` is derived from the project and can be rebuilt at any
time. All four can be switched. The interface's **Local settings** block shows
the measured current state and switches it for **all four** directories;
manually, this is controlled through a `.gitignore` in the directory itself,
whose content is named by the relevant `README.md`. Details are in
[`installation.md`](./installation.md#2-create-project-owned-structure).

## Finding the anchor

The procedure applies equally to the tool and to an assistant:

1. If a directory was passed, it applies; check `<arg>/K-PLAYBOOK.yaml`.
2. Otherwise search upwards from `realpath(CWD)`, one candidate per level: `<dir>/K-PLAYBOOK.yaml`.
3. On finding it: `PROJECT_DIR = <dir>`, `PLAYBOOK_DIR = <dir>/k-playbook`.
4. `$HOME` and `/`, inclusive, are the boundary of the upward search.
5. If nothing is found, report that no installation exists. Do not guess or create anything.

The upward search must **not** stop at the Git worktree root.
`<project>/k-playbook/` is its own clone and therefore its own worktree; a
search beginning there would otherwise never reach the configuration one level
above.

## Merge shipped and project-owned content

Five directories exist twice. What applies is the union of both sides:

| Kind | Shipped | Project-owned | Unit |
|---|---|---|---|
| rules | `k-playbook/rules/` | `k-playbook-local/rules/` | `*.md` |
| review recipes | `k-playbook/reviews/` | `k-playbook-local/reviews/` | `review-*.md` |
| checks | `k-playbook/checks/` | `k-playbook-local/checks/` | `*.sh`, top level only |
| commands | `k-playbook/commands/` | `k-playbook-local/commands/` | `*.md`, recursive |
| skills | `k-playbook/skills/` | `k-playbook-local/skills/` | directory with `SKILL.md` |

The comparison unit is the **name**. Both sides use the same naming convention,
so no derived key is required.

For commands, it is the path below `commands/`, including the namespace: a local
`commands/_shared/context.md` replaces exactly that file, while the rest of
`_shared/` remains shipped. A skill, however, is replaced as a whole:
`SKILL.md`, `PLAYBOOK.md`, and templates must fit each other; a partially
replaced skill cannot sensibly be composed.

**For the same filename, the project-owned file wins, completely.** The shipped
one is not read at all, and no individual sections from it are retained. To
change a shipped rule, copy it and change the copy, at the cost that later
improvements to the original no longer reach that copy. The advantage outweighs
this: what applies is in exactly one file.

**Content is disabled with an empty file**, not with a list in configuration.
Because a same-named local file completely replaces the shipped one, nothing
remains when it is empty. "Empty" means nothing except blank lines and comments,
which lets the file record its own reason:

```bash
# Disabled: this project does not use Django.
```

The difference between kinds is intentional. `rules` and `reviews` are read;
their entry remains visible in the catalog and its content says it is disabled.
A check is instead **executed**: an empty script would exit 0 and look like a
passed check. It therefore disappears from the catalog entirely.

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
`.claude/commands`, `.claude/skills`, `.opencode/commands`, and
`.cursor/commands`, are real directories with **one symlink per entry**, not a
directory symlink: it would point at exactly one source.

### What exists only once

Everything else has no counterpart on the other side:

| | Directories |
|---|---|
| project-owned only | `results/`, `data/`, `cache/`, `docs/`, `guidelines/`, `tasks/`, `priv/`, `material/` |
| shipped only | `docs/`, `scripts/`, `bin/`, `installer/` |

`docs/` appears in both rows but is not a pair: `k-playbook/docs/` documents
k-playbook itself, while `k-playbook-local/docs/` documents the project. They
are two different objects with the same name and nothing to merge.

Nothing below `k-playbook/` may be written, including by commands that read
rules or recipes there. An update replaces the directory completely.

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
| `instructions` | instruction files in reading order |
| `project` | project root, `repoRoot`, `vcs`, configuration location |
| `playbook`, `local` | the two resolved directories |
| `remediation` | the policy, including a default when the block is missing |
| `gh` | the GitHub CLI decision and host finding |
| `catalogs` | `rules`, `reviews`, `checks`, merged |
| `guidelines` | files from `k-playbook-local/guidelines/` |
| `links` | only when there is something to report: what assistant-link self-healing updated (`healed`), what remained open (`open`), and what that means for this session (`note`) |

Each catalog entry has `name` (the filename), `key` (the invocation name without
extension and kind prefix), `path`, `origin` (`dist`, `local`, or `override`),
and `disabled`, where applicable.

This means no command has to apply overlay rules itself. There is one answer,
and everyone receives the same one.

`links` is normally absent, intentionally: the invocation brings assistant
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

`gh` combines two things that must remain separate: `status` and `configured`
are the project decision from `tools.gh` and versioned in the file; `installed`,
`path`, `loggedIn`, `account`, `accounts`, and `tokenFromEnv` are findings for
this particular machine. `ready` summarizes what a command needs to know: gh
exists and an account is configured.

The finding is read from gh configuration, not checked against the server: a
stored token may have expired. If certainty is needed, run `gh auth status`;
that requires network access and does not belong here.

The invocation is deliberately cheap: it omits the security-tool preflight,
because it starts `--version` for every tool and takes noticeable time.
`context` must be usable at the start of every command. The gh finding costs
nothing: it only checks `PATH` and `~/.config/gh/hosts.yml`, without a subprocess.

The search starts at the working directory and proceeds upward. Without
`K-PLAYBOOK.yaml`, the call stops with a message; it does the same for a
`schema_version` other than `3`.

## Instructions

What an assistant must read before working is in `k-playbook.md`, once at each
level:

| File | Applies to | On update |
|---|---|---|
| `k-playbook/k-playbook.md` | every project using k-playbook | is replaced |
| `k-playbook-local/k-playbook.md` | this project only | remains |

They are read in this order; the project-owned level can supplement or override
the shipped one. Under `instructions`, `context` lists only files that actually
exist: a nonexistent path is worse than no path.

The file is deliberately not called `AGENTS.md`: assistants read that name
automatically, and it is reserved for the project root.

`AGENTS.md` receives only a **prompt**: a short block pointing to
`k-playbook context`. If the file does not exist, it is created; if it exists,
the block is appended without touching existing content. A marker,
`<!-- k-playbook:anstoss -->`, prevents a second run from appending it again.

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
```

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

tools:
  gh:
    status: enabled
```

## Fields

### `schema_version`

Required field. Current version: `3`.

`3` describes the model documented here: an anchor in the project root,
`k-playbook/` and `k-playbook-local/` beside it, and no paths in configuration.

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

Required field. The project repository location relative to `K-PLAYBOOK.yaml`.

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
| `target` | string | remediation target relative to `K-PLAYBOOK.yaml`; default is `project.repo_root` |
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

#### `tools.gh`

| Field | Type | Meaning |
|---|---|---|
| `status` | enum | `unknown`, `enabled`, or `disabled` |

Whether this project uses the GitHub CLI. It is used by `/k-pr-review` and the
Dependabot review.

**The default is `unknown`.** It is an explicit state, not a silent no: without
a decision, a command cannot know whether a missing `gh` is a problem or
intentional. The interface therefore shows `unknown` as an open item, and
commands requiring `gh` stop on it. Any value other than the three stated is an
error and stops `context`: a typo must not look like a decision.

The block says nothing about whether `gh` exists on this machine. That is a host
finding and exists only in context output. Nor is there a host here: the
decision applies to `github.com`.

## Writing rules

- An existing `K-PLAYBOOK.yaml` is never overwritten. It belongs to the project and may contain values the tool does not know.
- Write only after confirmation, step by step.
- The tool owns `schema_version` and `project.*`.
- The interface owns only `tools.gh`. It writes the `gh:` sub-block; an adjacent block for another tool remains untouched. For new projects, it creates it as `unknown` so the open decision is visible in the file.
- The remediation policy is set during onboarding; later `/k-remediation` may change it after asking. Only the `remediation:` block is written.
- Unknown top-level fields remain and are not changed unprompted. Writing occurs line by line so comments and order remain intact.
- Host-local installation states do not belong in this file.
- Nothing below `k-playbook/` may be written.

## Filename

The canonical filename is `K-PLAYBOOK.yaml`. Do not create `K-PLAYBOOK.yml`, so
the tool and commands need to check only one name.
