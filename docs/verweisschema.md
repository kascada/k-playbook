# Reference Scheme

A reference in Markdown that k-playbook ships or produces is **one path with a prefix**.
The prefix says where the content lives and how it is read. A session can map every
reference to its access route without asking back.

Decided 2026-10-14 (task 087). The version for sessions is
`instructions/always/010-verweise.md`; both state the same thing and are kept in step.

## The Prefixes

| Prefix | Meaning | Example |
|---|---|---|
| `project:` | a file in the project, path from the project root | `project:/scripts/123.sh` |
| `k-playbook:` | resolved by k-playbook, local before shipped | `k-playbook:/instructions/on-demand/xyz.md`, `k-playbook:/rules/befunde.md` |
| `k-playbook:` with a trailing `/` | a directory, assembled into one text -- only directories on a fixed list, today only `instructions/always/` | `k-playbook:/instructions/always/` |
| `keph:` | the project's collection in Kephalaion | `keph:/manual/abc.md` |

More examples:

- `project:/k-playbook-local/docs/README.md`
- `k-playbook:/instructions/on-demand/<topic>.md`
- `keph:/manual/<topic>.md`

## Rules

- **After the prefix there is always a `/`.** The path runs from the root of the space the
  prefix names: the project root, the k-playbook space, the project's collection.
- **A reference ending in `/` means a directory.** Only `k-playbook:` knows directories, and
  only those on a fixed list are assembled -- today only `k-playbook:/instructions/always/`.
  The reader gets all files in it as one text, in the fixed order (see
  [Assembling a directory](#assembling-a-directory)). Every other directory, such as
  `instructions/on-demand/` or `instructions/` itself, is read file by file with its name; a
  reference to it ending in `/` lacks the file name and is an error (`invalid_input`).
- **`k-playbook:` is read only through k-playbook**, never directly from `k-playbook/` or
  `k-playbook-local/`. k-playbook decides whether a local version replaces or disables the
  shipped one; whoever looks on disk can pick the wrong version.
- **A reference k-playbook does not know is an error.** The tool reports it with an error
  code; it does not fall back to reading from disk, and the session reports it instead of
  searching for the file on its own.
- **A `keph:` reference names neither hub nor collection.** Both are machine-local in
  `k-playbook-local/K-PLAYBOOK.local.yaml` and can differ from machine to machine. k-playbook
  supplies them as `kephalaion.address` in `k-playbook context` and uses them itself when
  it resolves a `keph:` reference. If `kephalaion.ok` is not `true` there, `keph:`
  references are unreachable (`kephalaion_unavailable`), and a session says so rather than
  working around it.
- **The public collection `<hub>:k-playbook` is not a reference target.** Its `defaults/` is
  the shipped layer behind `k-playbook:`; a direct reference would bypass the override by
  local content.

## Placeholder `~` for the Own User

Shipped content has to name "your tasks" or "your workbench" without writing a user name
into the reference, just as `keph:` writes no hub or collection. A path segment `~` stands
for the own user:

```text
keph:/user/~/tasks/
keph:/user/~/workbench/
```

- `~` is replaced by `kephalaion.user` from `k-playbook context`: the **user** (the person),
  not the **account** (`kephalaion.account`, one per machine). The same person on two
  machines lands in the same directory.
- `~` cannot clash with a real name: Kephalaion names match `^[a-z0-9][a-z0-9._-]{0,62}$`
  (`installer/internal/kephalaion/binding.go:149`).
- Only a whole segment counts. `~kamran` or `a~b` is no placeholder.
- References that k-playbook hands out itself, for example when it creates a task, carry the
  real name (`keph:/user/kamran/tasks/088-name.md`). The placeholder is for text written once
  for every user.

The user area itself — `user/<name>/<kind>/` with `tasks/` and `workbench/`, each numbered
project-wide and with a `done/` — and the tools that create, list and move its documents are
described in [mcp.md](mcp.md#document-tools-in-the-collection). `k_playbook_doc_done` and
`k_playbook_doc_reopen` accept `~` in their `ref`; every reference they hand back carries the
real name.

## Why These Names

- `project:` and not `local:`, because "local" already names the layer `k-playbook-local/`.
  A `local:` reference would read as "from `k-playbook-local/`", which it is not.
- `keph:` carries no address because the address is not a property of the content: the same
  project reaches its collection on different machines through different hub aliases.
- The scheme belongs to k-playbook. Kephalaion's own addressing (`collection` + `name`)
  stays untouched; `keph:` is translated into it at the moment of reading.

## Access Route per Prefix

One function resolves all three prefixes: `project.ResolveDocRef`
(`installer/internal/project/docref.go`), behind MCP `k_playbook_doc_read` and the
subcommand `k-playbook read <ref>` (task 093). Both are thin hulls and behave the same.

| Prefix | Route |
|---|---|
| `project:` | `k_playbook_doc_read` / `k-playbook read`: the file read from disk below the project root (the directory of `K-PLAYBOOK.yaml`), one layer, no overlay; a session may also read the file directly, it is the same file |
| `k-playbook:` | `k_playbook_doc_read` / `k-playbook read`, local before shipped; never directly from disk |
| `keph:` | two routes until `010-verweise.md` is updated in the follow-up task, which also decides the default: `k_playbook_doc_read` / `k-playbook read`, which call the node's `read` through the Kephalaion client (`collection` = the binding's address, `name` = the path without the leading `/`, a segment `~` replaced by `kephalaion.user`); or Kephalaion `read` directly with `collection` = `kephalaion.address` from `k-playbook context` |

What each prefix answers:

- **`project:`** reads one file, path from the project root, through `os.DirFS` so that no
  path leaves the root (`..` is already rejected by the parser). No overlay, no assembly: an
  empty file is empty content, not `disabled`. `parts` carries one entry with `origin:
  project`. A missing file is `not_found`; a reference ending in `/` is `invalid_input`
  ("Dateiname fehlt").
- **`keph:`** reads one document of the project's collection through the node's replica
  (Kephalaion `read`, see [kephalaion-client.md](kephalaion-client.md)). Access is opened like
  for the `doc` tools: if `kephalaion.ok` is not `true`, the answer is `kephalaion_unavailable`
  without fallback; the access is opened at most once per resolution, however many `keph:`
  references an include chain contains. `~` as a whole segment is replaced by
  `kephalaion.user`; the `ref` in the answer carries the real name. A name the node reports as
  `none` is `not_found`, a directory `invalid_input` -- `keph:` names single documents, nothing
  is assembled. `parts` carries `origin: keph`. Other errors of the node keep their code where
  there is one (`kephalaion_error` otherwise).
- **`k-playbook:`** resolves the kinds `instructions/`, `rules/`, `reviews/`, `guidelines/`
  and `commands/` (`docReaderKinds` in `docref.go`; `rules`, `reviews` and `guidelines` since
  task 086, `commands` since task 092) and, as targets of includes, `includes/` and
  `testdata/` (see below). A `k-playbook:` path outside these kinds is refused as
  `unsupported` -- `k-playbook:/checks/check_x.sh` is a valid reference whose kind is not
  resolved: checks are scripts that run, skills live with the assistant; nobody reads them
  through a reference. The message names only the reader kinds. Commands lie with the
  assistant only as a hull (frontmatter plus the reference `k-playbook:/commands/<name>.md`);
  their text is resolved like a rule, as a single file.

Paths with `..`, `.`, hidden or empty segments are not references at all (`invalid_input`).
A `k-playbook:` reference ending in `/` whose directory is not on the list of assembled
directories is incomplete -- the file name is missing -- and also `invalid_input`, decided
without reading: `k-playbook:/instructions/on-demand/`, `k-playbook:/instructions/`,
`k-playbook:/rules/`, an unknown directory and a file with a trailing `/`
(`…/010-verweise.md/`); the same holds for every `project:` and `keph:` reference ending in
`/`. The message says "Dateiname fehlt" and gives an example that fits the case. Outside the
kinds the check for `unsupported` comes first, so `k-playbook:/checks/` stays `unsupported`.
An unknown reference -- a file, or an assembled directory that neither layer has -- is
`not_found`. The tool's full answer and error codes are in [mcp.md](mcp.md#doc-read-contract).

Two kinds exist only as targets: **`includes/`** holds text blocks for include lines
(`defaults/includes/`, overridable by `k-playbook-local/includes/`, an empty local file
disables), and **`testdata/`** holds fixed test cases that come out the same on every
installation (`defaults/testdata/`). Both are resolved like `instructions/`, but appear in no
catalog, no briefing and no help text: nobody reads them for their own sake.

## Includes

An include is a reference at a particular place in a document: the line is replaced by the
body of the target before the content is handed out (decision 2026-10-18, task 093). The
mechanism belongs to the resolution and applies to **every** document it reads -- there is no
list of directories where includes are "on" -- and it can be switched off per call.

**Syntax.** A line that begins with `include:` without leading whitespace, followed by exactly
one argument and nothing else:

```markdown
include: k-playbook:/includes/context.md
include: _shared/context.md
```

The argument is a reference with prefix, or a relative path without one. Relative means:
against the directory of the including document, in the same prefix, then through that
prefix's normal resolution -- for `k-playbook:` with the overlay, so a local version of the
target wins. In an assembled directory each part's directory is its own. No relative path
leaves the kind (`..` is `invalid_input`); a block in `includes/` is reachable from a command
or an instruction only through the absolute reference `k-playbook:/includes/…`, which is why
`includes` is a kind. The syntax is deliberately not an HTML comment: an include that was not
resolved must stay readable in the raw text so that a session which receives the raw text
knows what to read. Lines starting with `@` or `` ! `` are out, Claude Code reinterprets
them. Inside fenced code blocks (``` or ~~~) nothing is inserted -- otherwise the syntax could
not be documented. A frontmatter block at the top of the main document is skipped.

**What is inserted:** the body of the target without its frontmatter (the same cut as for an
assembled directory), with the target's own includes already resolved, in place of the include
line. A disabled target (empty local file) inserts nothing and is no error; it appears in
`includes` with `disabled: true`, and the line simply disappears -- as does the line of an
empty target. A directory reference ending in `/` is allowed as a target when it can be
assembled (`instructions/always/`), otherwise `invalid_input`; its text is inserted, its own
problems are reported with the outer answer.

**Limits and errors.** Depth 10: the including main document is depth 0, what it includes
depth 1; an include at depth 11 is `include_too_deep`. A cycle is detected on the chain of
references currently open and reported at once (`include_cycle`), not at the depth limit; for
`keph:` the real name is checked as well, so `~` cannot hide a cycle. A target the resolution
cannot find or may not read keeps its code (`not_found`, `unsupported`,
`kephalaion_unavailable`, `invalid_input`); a line with two arguments is `invalid_input`. Every
such message carries the chain once, `… — Include-Kette: a.md → b.md → c.md`. No include
fails silently.

**Two error levels.** For a single document (a file reference) a failed include is the failure
of the whole read; there is no partial result. In an assembled directory
(`instructions/always/`) the existing mechanism applies: the include error of a part lands
with its chain under `problems`, the part is left out, the other parts stay -- so a faulty
include line in a local always-instruction does not stop the session start, and nothing
fails silently. The briefing builds `documents.always.content` through the same code and
behaves the same.

**Parameter.** Includes are inserted by default. `k-playbook read --raw` and
`k_playbook_doc_read` with `includes: false` deliver the raw text; in the code
`DocReadOptions{Raw: true}` with `ResolveDocRefWith`, while `ResolveDocRef(layers, ref)`
keeps the default. With the raw text there are no include errors.

**`includes` in the answer.** The inserted targets in reading order, each with `ref`
(canonical, resolved -- relative made absolute, `~` replaced), `origin` (`dist`, `local`,
`override`, `project`, `keph`; empty for an assembled directory as target, whose parts are
not listed individually), `depth` and `disabled`. Always present, `[]` without includes and
with `includes: false`. `parts` remains what it is: the parts of a directory.

**`keph:` includes** are allowed by the mechanism. That they make a document depend on the
network and on `kephalaion.ok` is a question of use, not of the mechanism: in
always-instructions and briefing texts only with care (note for the follow-up task in
[knowledge-aktuell.md](knowledge-aktuell.md)).

**Test cases.** `defaults/testdata/includes/` holds fixed cases -- simple, nested, relative,
cycle, too deep, dead target, code block -- with a README; the Go tests read them from disk
with the expectations in the test, and after a release `k-playbook read
k-playbook:/testdata/includes/<case>.md` answers the same on every hub and installation. No
case includes `project:` or `keph:`, because such a target would exist nowhere after the
release. Nothing shipped carries an include line yet; the first use comes with the move of
the commands.

## Resolving `k-playbook:`

Two layers, compared by name:

| Layer | Directory | Origin |
|---|---|---|
| shipped | the installation `k-playbook/` — a clone of this repo; `instructions/`, `commands/`, `includes/` and `testdata/` are read from `defaults/`, the other kinds at the top level | `dist` |
| local | `k-playbook-local/` | `local`, or `override` when it replaces a shipped file of the same name |

The shipped instructions exist once, in `defaults/instructions/`: the installation reads
them there (`NewShippedDocSource` in `docref.go`), and the release packs the same files for
the hub. Until 2026-10-17 a copy lay at the top of the repo as well, kept equal by a test;
that copy is gone. `rules/`, `reviews/` and `guidelines/` still lie at the top of the repo
until their own switch, because the catalogs of `k-playbook context` read them there.

`commands/` is read from `defaults/commands/` as well, but the file at the top of the repo
is not a copy: `commands/<name>.md` is the hull the assistant registers, frontmatter plus the
reference, and `defaults/commands/<name>.md` is the text behind the reference. A hull without
a text in `defaults/` is `not_found`; there is no fallback to the hull. A local
`k-playbook-local/commands/<name>.md` replaces both at once — the hull in the registry and the
text here — so it carries the text itself and must not point at the same reference, which
would resolve to that file. First command on this route: `/k-upgrade` (task 092).

The resolution goes through an exchangeable source — a directory listing and a file read per
layer. Today the source is the disk; later it can be Kephalaion without the caller noticing.

There is **one** overlay, not two: `overlayDocDir` in `docref.go` lays the two layers of a
directory over each other -- same name wins locally, an empty local file disables -- and the
catalogs of `k-playbook context` (`resolveCatalog`), the references here and the briefing all
go through it. A test compares both ways for `rules` and `reviews`: name, origin and
`disabled` of every catalog entry are what `k-playbook:/<kind>/<name>` answers, and the
content behind the reference is the file the catalog names. The one named exception is the
field `guidelines` of the context output, which still lists only
`k-playbook-local/guidelines/` until the context output is switched; for references and the
briefing, `guidelines` is a regular overlay kind with shipped `k-playbook/guidelines/` and
local `k-playbook-local/guidelines/`.

### A single file

- The local file wins, otherwise the shipped one.
- It is delivered unchanged, frontmatter included.
- An **empty** local file disables the shipped one. That is not an error: the content is
  empty, and the answer carries `disabled: true` with a hint.

### Assembling a directory

Only for `k-playbook:` references that end in `/` and name a directory on the list of
assembled directories -- today only `instructions/always/`. The list is a value in code
(`docAssembledDirs` in `installer/internal/project/docref.go`), not configuration; a new entry
is a contract change and comes with its own task. Any other reference ending in `/` is
refused as `invalid_input` (see [Access Route per Prefix](#access-route-per-prefix)). For the
directories on the list:

- All files of the directory from both layers, without subdirectories and without
  `README.md`.
- A local file replaces the shipped one of the same name completely; an empty local file
  disables it.
- Only files whose name starts with a number are taken in. A file without one is left out
  and reported as a problem (envelope `problems`, on the command line a hint on stderr).
- Order: numerically by the leading number across both layers (`20-` before `100-`); with the
  same number, by name.
- The frontmatter of each part is removed before the parts are joined.
- A directory with only disabled parts, or with none, yields an empty text, not an error;
  the disabled parts are listed in `parts`.

### Numbers are part of the name

Overriding compares the whole file name, number included. A local
`100-projektwissen-zuerst.md` replaces exactly that part; a local
`150-projektwissen-zuerst.md` is a new part beside it. Hence:

- Shipped numbers stay stable. Renaming a shipped part is a contract change, because every
  local override of it silently becomes an additional part.
- Orphaned local parts — overrides of a shipped part that was renamed or removed — are not
  detected.
- Ranges: shipped parts below 1000, local parts from 1000 on, so they follow everything
  shipped. A local part that belongs in between takes a smaller number on purpose.

### What "empty" means

The same as `isEmptyFile()` for `rules` and `reviews`: nothing but blank lines and lines
starting with `#`. That lets a disabling file carry its reason:

```markdown
# Abgeschaltet: dieses Projekt führt keine Befunde.
```

**Consequence for Markdown:** a local file consisting only of headings also counts as empty
and disables the shipped one. Whoever wants to replace a part needs at least one line of
body text.

### Markers

HTML comments with the marker `NACHZIEHEN:` are not treated specially and pass through. They
belong to the ongoing rebuild only and disappear from the contents later.
