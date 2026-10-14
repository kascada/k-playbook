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
| `k-playbook:` | resolved by k-playbook, local before shipped | `k-playbook:/instructions/on-demand/xyz.md` |
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
  supplies them as `kephalaion.address` in `k-playbook context`. If `kephalaion.ok` is not
  `true` there, `keph:` references are unreachable, and a session says so rather than
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

The user area itself — `user/<name>/<kind>/` with `tasks/` (numbered project-wide) and
`workbench/` — was decided on 2026-10-14; its layout and tools are not built yet.

<!-- NACHZIEHEN: Layout und Werkzeuge des Userbereichs (user/<name>/tasks, workbench) beschreiben, sobald sie festgelegt sind; dann hierher verlinken. -->

## Why These Names

- `project:` and not `local:`, because "local" already names the layer `k-playbook-local/`.
  A `local:` reference would read as "from `k-playbook-local/`", which it is not.
- `keph:` carries no address because the address is not a property of the content: the same
  project reaches its collection on different machines through different hub aliases.
- The scheme belongs to k-playbook. Kephalaion's own addressing (`collection` + `name`)
  stays untouched; `keph:` is translated into it at the moment of reading.

## Access Route per Prefix

| Prefix | Today |
|---|---|
| `project:` | the session reads the file directly from disk |
| `k-playbook:` | MCP `k_playbook_doc_read` with the reference as `ref`; as a fallback and for testing `k-playbook read <ref>` |
| `keph:` | Kephalaion `read` with `collection` = `kephalaion.address` from `k-playbook context` and `name` = the path without the leading `/`, a segment `~` replaced by `kephalaion.user` |

`k_playbook_doc_read` and `k-playbook read` resolve only `k-playbook:` references, and today
only those under `k-playbook:/instructions/`. For `project:` and `keph:` they answer "not yet
supported" (code `unsupported`). That limits the tool, not the reference: a `project:` or
`keph:` reference is valid and is read on its route above. A `k-playbook:` path outside
`instructions/` is refused as `unsupported` as well -- `k-playbook:/rules/befunde.md` is a valid
reference whose kind is simply not resolved yet. Paths with `..`, `.`, hidden or empty
segments are not references at all (`invalid_input`). A `k-playbook:/instructions/…`
reference ending in `/` whose directory is not on the list of assembled directories is
incomplete -- the file name is missing -- and also `invalid_input`, decided without reading:
`k-playbook:/instructions/on-demand/`, `k-playbook:/instructions/`, an unknown directory and a
file with a trailing `/` (`…/010-verweise.md/`). The message says "Dateiname fehlt" and gives
an example that fits the case. Outside `instructions/` the check for `unsupported` comes
first, so `k-playbook:/rules/` stays `unsupported`. An unknown reference -- a file, or an
assembled directory that neither layer has -- is `not_found`.
Other kinds (`rules`, `reviews`, `guidelines`) follow with the briefing (task 086). The tool's
full answer and error codes are in [mcp.md](mcp.md#doc-read-contract).

## Resolving `k-playbook:`

Two layers, compared by name:

| Layer | Directory | Origin |
|---|---|---|
| shipped | the installation `k-playbook/` — a clone of this repo, read at its top level, not `defaults/` | `dist` |
| local | `k-playbook-local/` | `local`, or `override` when it replaces a shipped file of the same name |

The shipped content therefore lies at the top of the repo (`instructions/`) and, as a real
copy, in `defaults/instructions/` for the hub. Unlike the rest of `defaults/`, it is not
frozen: `instructions/` is maintained continuously, and a Go test in the normal test run
keeps both trees equal; `.github/scripts/pack-defaults.sh` refuses to pack when they differ.

The resolution goes through an exchangeable source — a directory listing and a file read per
layer. Today the source is the disk; later it can be Kephalaion without the caller noticing.

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
