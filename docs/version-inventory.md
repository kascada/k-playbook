# Version inventory

The contract for the version inventory: data model, pin taxonomy, source types,
normalization, deviations, trust boundary, source configuration, and the rule for
timestamps and byte stability.

This page is the **single** definition. The collector, subcommand, command, interface,
and tests refer to it and do not redefine any of it. What is not stated here is not
defined.

## What the inventory is, and is not

The inventory is a complete, reproducible overview of a project's **declared** versions:
packages, tools, runtimes, container images, Helm charts, and Helm dependencies, collected
from declarative sources.

It reads files only. It does not query the network, install anything, or execute a
discovered tool. It does not assess which version is actually active at runtime, and it
does not consolidate environments: it documents every discovered assertion with context
and origin.

Distinction from `docs/libs/`: `/k-docs-tools` produces **curated, pitfall-oriented**
profiles for selected tools. The inventory is **complete and source-oriented** and does
not select. Both share the pin taxonomy so that two languages for the same thing do not
emerge. If version declarations differ, the inventory provides the answer; `docs/libs/`
is not automatically rewritten for that reason, but the difference is reported as a
notice.

## Names

These names are fixed and used unchanged everywhere.

| What | Name |
|---|---|
| Subcommand | `k-playbook inventory` |
| Command | `/k-doc-inventory` (file `commands/k-doc-inventory.md`) |
| Docs module | `commands/_docs/inventory.md` |
| Inventory file | `k-playbook-local/docs/versions/inventory.md` |
| Source configuration | `k-playbook-local/version-sources.yaml` |
| Generator in frontmatter | `generated.by: k-doc-inventory` |

The command deliberately uses the singular `doc`, although the rest of the family uses
`k-docs-`. This is an explicit decision, not a typo; anyone who wants to align it changes
it in every location at once, not silently in one place.

`generated.by` is `k-doc-inventory` for **both** invocation paths, including when the
subcommand was called directly. The origin identifies the generator, not the invocation
path. `docs/versions/` is the only origin to which this generator writes, and it writes to
no other.

## Data model for an inventory row

An inventory row is exactly **one assertion from exactly one source**. Two sources that
say the same thing are two rows; they are merged only in the presentation.

| Field | Required | Meaning |
|---|---|---|
| `ecosystem` | yes | Which world the assertion concerns: `python`, `go`, `node`, `rust`, `ruby`, `php`, `java`, `elixir`, `container`, `helm`, `ci`, `runtime`. `runtime` carries the language and tool runtimes themselves: `runtime/python`, `runtime/node`, `runtime/go`, so that the same runtime from `.python-version`, a manifest, and a setup action ends up in **one** group. |
| `name` | yes | The item, canonically normalized (see "Normalization"). |
| `kindOfThing` | yes | What the item is: `package`, `tool`, `runtime`, `image`, `chart`, `chart-dependency`, `action`. |
| `version` | yes | The version declaration **verbatim as in the source**, without rewriting. If it is entirely absent, this is the empty string. |
| `versionNormalized` | no | The comparable form, if one could be produced (see "Normalization"). Empty otherwise. |
| `pin` | yes | The pin type from the taxonomy below. |
| `digest` | no | The content digest, if the source specifies one: `sha256:…` or a full commit SHA. |
| `context` | yes | The environment label: `lokal`, `dev`, `devcontainer`, `ci`, `deployment`. |
| `contextOrigin` | yes | Where the label comes from: `default` (derived from the source type) or `configured` (from the source configuration). |
| `scope` | no | The declared usage scope within the source, where it defines one: `main`, `dev`, `optional`, `test`, `build`. |
| `sourceFile` | yes | The source file, relative to the project. If it is outside the project root, this is the absolute path. |
| `sourceKey` | yes | Section or key within the file, as a path: `dependencies.fastapi`, `jobs.test.steps[2].uses`, `services.db.image`. |
| `sourceLine` | no | The line number, if the assertion can be assigned to exactly one line. Empty otherwise; then `sourceKey` provides findability. |
| `group` | yes | The group key for forming deviations: `<ecosystem>/<name>`. |
| `deviation` | no | Reference to the deviation to which this row belongs; empty if the item has only one assertion or if all assertions are equal. |
| `note` | no | A visible note for exactly this row, for example `Wert aus Variable, nicht auflösbar: ${IMAGE_TAG}` ("value from variable, cannot be resolved"). |

Together, `sourceFile`, `sourceKey`, and, where present, `sourceLine` must be sufficient
to find the assertion again **without searching**. This is the criterion for every new
parser.

One consequence is intentional: if a line moves in a source file, the inventory changes.
This is not instability, but changed origin.

## Pin taxonomy

The taxonomy is the one from `commands/_docs/tools.md`, **extended**. The three existing
values retain their literal meanings there; they are quoted here, not reworded:

> `exact` (`==1.2.3`, `1.2.3`) / `range` (`^1`, `>=1,<2`, `~1.2`) / `floating` (`*`, no
> declaration).

Three values are added:

| Value | Meaning |
|---|---|
| `digest` | The source binds to immutable content instead of a version: `image@sha256:…`, an action at a full commit SHA, or a Helm chart with `digest`. If a tag is also present, it remains in `version`; the binding is still the digest. |
| `local` | There is no external version: built locally or taken from the working tree. `build:` instead of `image:` in Compose, `replace … => ../x` in `go.mod`, an editable or path dependency, or `FROM <stage>` referring to a stage in the same file. |
| `unknown` | A declaration is present but cannot be classified: an unresolvable variable (`${IMAGE_TAG}`), an unreadable expression, or invalid syntax in otherwise readable surroundings. |

`floating` and `unknown` are not confused: `floating` means "deliberately not pinned",
`unknown` means "cannot be determined". Where `unknown` appears, it must have a `note`
that says why.

A stated container tag is `exact`, even if it is not a version number: `ubuntu-22.04`
specifies as firmly as `1.2.3`. This is the same interpretation that this contract already
uses for CI actions ("a tag is `exact`"). `floating` remains reserved for tags that
explicitly must not specify: `latest`, `main`, `master`, `edge`, `stable`, `nightly`,
`dev`, `devel`, and a missing tag.

No second taxonomy emerges. If a parser needs another value, the taxonomy is extended
here; a seventh value is not invented locally.

## Context and environment labels

The labels are a **closed** set:

`lokal`, `dev`, `devcontainer`, `ci`, `deployment`

Without configuration, the label is determined from the source type
(`contextOrigin: default`):

| Source | Default label |
|---|---|
| Package manifests and lockfiles of all ecosystems | `lokal` |
| `.tool-versions`, `.python-version`, `.nvmrc`, `.ruby-version` | `lokal` |
| `.devcontainer/**` | `devcontainer` |
| `Dockerfile`, `Dockerfile.*`, `*/Dockerfile` | `deployment` |
| `docker-compose*.y{a,}ml`, `compose*.y{a,}ml` | `dev` |
| Helm: `Chart.yaml`, `Chart.lock`, `values*.yaml` | `deployment` |
| CI: `.github/workflows/*.y{a,}ml`, `.gitlab-ci.yml` | `ci` |

An entry in the source configuration overrides the label for its source
(`contextOrigin: configured`). A label other than those five is an error and is visibly
rejected; see "Error cases".

## Default sources and their semantics

Without configuration, the collector searches for these sources below the project root.
It reads everything it finds completely; it silently ignores what it does not know. Only a
**found but unreadable** source is reported.

From a lockfile, only the **direct** dependencies of its associated manifest are included
in every ecosystem. For workspaces, the reference set is the union of the root manifest
and all declared member manifests; it applies only if every member is resolved, not
excluded, and readable. Transitive lockfile entries never substitute for a missing
reference set.

Directories populated by a tool are not entered: `.git`,
`.hg`, `.svn`, `node_modules`, `vendor`, `bower_components`, `.venv`, `venv`,
`__pycache__`, `.tox`, `.nox`, `.mypy_cache`, `.pytest_cache`, `.ruff_cache`, `target`,
`.gradle`, `.terraform`, `.next`, `.cache`, `_build`, `deps`. Without this boundary,
"below the project root" would also mean every manifest of every downloaded dependency,
and this contract explicitly includes only direct dependencies from lockfiles. A directory
with maintained content is not on this list.

### Where default detection does not search

Two areas are within the project yet are not searched automatically. This is not a reading
ban, as the trust boundary continues to allow them, but a statement about what counts as a
source **of the project** without intervention.

1. **The `<project>/k-playbook/` installation.** It is a clone of the tool, is located in
   the same place in every target project, contains the same manifests there, and is
   completely replaced with every update. Its `go.mod` describes the dependencies of
   k-playbook, not those of the project; the deviation between it and a package of the same
   name in the project would be the same in every project and could not be fixed in any of
   them: `k-playbook/` is never written to. An assertion that is the same everywhere and
   fixable nowhere is not an insight.
2. **Every pattern from `exclude:`** in the source configuration. This lets a project
   exclude its test fixtures and example projects: maintained content whose versions are
   deliberately old or contradictory and say nothing about the project. Only the project
   knows where such material is located and what it is called: `testdata/`,
   `tests/fixtures/`, `spec/fixtures/`. This is why it belongs in the configuration rather
   than in a list of guesses in the tool.

Both apply **only** to default detection. Anyone who wants a source from them in the
inventory writes it in `sources:`; an explicit entry overrides every exclusion and then
carries its own environment label.

**No exclusion is silent.** Every rule appears in the "Nicht durchsuchte Bereiche" (areas
not searched) section of the inventory file, with pattern, origin (`installation` or
`configured`), reason, and the number of sources skipped as a result. The total
additionally appears in frontmatter under `inventory.sources-excluded` and in the
"Übersicht" (overview) section as `Nicht durchsuchte Quellen`. A rule without matches is
included too: "nothing was found here" and "this location was not searched" are two
different assertions, and the difference belongs in the file.

The difference from the `skippedDirs` list above is the reason, not the effect: it lists
what a tool populated and thus belongs to no one; this lists what belongs to someone, just
not to this project.

### Python

`pyproject.toml`, `requirements*.txt`, `constraints*.txt`, `setup.py`, `setup.cfg`,
`Pipfile`, `Pipfile.lock`, `poetry.lock`, `uv.lock`, `.python-version`.

- A manifest is the intention, a lockfile the resolved state. Both are read and represented
  as separate rows; if they contradict each other, that is a deviation.
- Extras and markers remain in `version`, but do not become part of the name.
- `-e .`, path dependencies, and VCS dependencies are `local`. The single exception is
  `-e .` itself: it means the project and has neither an item name nor a version, so it is
  not represented as a row.
- `.python-version` is a `runtime` entry, not a package.

### Go

`go.mod`, `go.sum`, `tools.go`, `.go-version`.

- `require` produces `package` rows with an `exact` pin: Go modules are always exact.
- `replace` to a path is `local`; `replace` to another version is a separate row with a
  `note`.
- The `go` directive and `toolchain` are `runtime` entries.
- `go.sum` is not included as a version source: it repeats `go.mod` and would only add
  noise.
- `tools.go` names tools but no versions, which are in `go.mod`. It therefore likewise
  produces no rows of its own.

### Node and JavaScript

`package.json`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `.nvmrc`,
`.node-version`.

- `dependencies`, `devDependencies`, `optionalDependencies`, and `peerDependencies` go
  into `scope`.
- `engines.node` and `packageManager` are `runtime` entries.
- Lockfiles follow the general direct-dependency rule in this section. `package-lock.json`
  takes its direct dependencies from `packages[""]` of the same lockfile, `pnpm-lock.yaml`
  from `importers`, and only `yarn.lock` from its associated `package.json`.
- A `package.json` and the `package-lock.json` in the same directory form pairs for
  classifying deviations; see "Pairs from `package.json` and `package-lock.json`" under
  "Deviations". The rows themselves stay separate.

### Other manifest types

The same ones that `/k-docs-tools` already recognizes: `Cargo.toml`/`Cargo.lock`,
`Gemfile`/`Gemfile.lock`, `composer.json`/`composer.lock`, `pom.xml`,
`build.gradle`/`build.gradle.kts`, `mix.exs`/`mix.lock`. Semantics as above: both manifest
and lockfile; lockfiles follow the general direct-dependency rule in this section; scope
comes from the section.

- `mix.exs` has no sections. There, the scope comes from `only:` of the individual
  dependency, evaluated over the complete dependency entry even if it spans several lines.
  The environment names are mapped like the Python group names (`dev`, `develop`,
  `development` → `dev`; `test`, `tests`, `testing` → `test`; `build` → `build`; `main`,
  `default` → `main`; anything else → `optional`), with one exception: `:prod` is `main`.
  A dependency without `only:` is `main`. If `only:` names several environments, the order
  of precedence decides, not the order in the source: `main` before `dev` before `test`
  before `build` before `optional`. If `only:` names no readable atom, `scope` stays empty.
- `mix.lock` takes the scope from its direct dependency in `mix.exs`.

### Containers

`Dockerfile`, `Dockerfile.*`, `docker-compose*.y{a,}ml`, `compose*.y{a,}ml`.

- `FROM [--flag=value …] <image>:<tag> [AS <stage>]` is an `image` entry; `@sha256:…`
  makes it `digest`. Flags are not part of the image reference, and a stage alias registers
  the local stage.
- `FROM [--flag=value …] <stage> [AS <stage>]` referring to a stage in the same file is
  `local` and is represented as such, not omitted.
- `ARG` values are resolved only if their default is in the same file. Otherwise they are
  `unknown` with a `note`.
- In Compose, `services.<name>.image` is an `image` entry; `services.<name>.build`
  without `image` is `local`.
- Explicit tool versions from `RUN` lines are **not** guessed. `apt-get install curl` is
  not a version assertion, whereas `RUN pip install x==1.2.3` is; only what literally
  names a version is included.

### DevContainer

`.devcontainer/devcontainer.json`, `.devcontainer/**/devcontainer.json`, plus a
`Dockerfile` or Compose file to which it refers.

- `image` is an `image` entry.
- `features` are `tool` entries; the feature name is the item, and the version follows the
  `:` or is in `version`.
- If the file refers to a `Dockerfile` or a Compose file, their findings receive the
  `devcontainer` label rather than their own default.

### Helm

`Chart.yaml`, `Chart.lock`, `values*.yaml`, plus image references in any `values*.yaml`.

- In `Chart.yaml`, `version` is a `chart` entry, `appVersion` a `runtime` entry, and every
  entry from `dependencies` a `chart-dependency`. The `appVersion` entry is in the
  `runtime` ecosystem, not in `helm`: otherwise the chart version and application version
  would be in the same group and produce a deviation that is not one.
- `Chart.lock` provides the resolved state of the same dependencies: separate rows; a
  contradiction with `Chart.yaml` is a deviation.
- In `values*.yaml`, the key pairs `image.repository` + `image.tag` and a single `image`
  string count as an image reference. `image.digest` makes it `digest`. The same two forms
  count under every key that ends in `Image` in camelCase, i.e. `Image` preceded by a
  lowercase letter or a digit: `gatewayImage.repository` + `gatewayImage.tag`, or a
  `browserImage` string. `imagePullSecrets`, `IMAGE`, or `myimage` are not such keys. Other
  keys are not guessed: a `tag:` outside an image reference yields no row.
- A mapping is **not** an image reference merely because it has a `repository` key: in
  values files, `repository` also names chart and Git sources, and a registry without an
  image name (`redis.repository: registry.example.io` next to `redis.standalone.tag`)
  cannot be completed without guessing the name from a subchart template. Such values are
  collected only when configured; see "Configured Helm values" under "Source
  Configuration".
- Anchors in `values*.yaml` are skipped as labels: `tag: &v "1.2.3"` yields `1.2.3`, and a
  block below `image: &img` is read like any other block. The anchor changes neither the
  value nor the source line. Aliases stay literal: `image: *img` is the string `*img`, and
  the merge key `<<` is an ordinary key with that string as its value. Neither is resolved,
  and neither aborts the file. An alias is a reuse of an assertion that is already
  written once, not a new assertion, so resolving it would only repeat rows; and the
  source line of a resolved alias would be the anchor's line, contradicting
  `sourceFile`/`sourceKey`/`sourceLine` as the place where the assertion is written. A
  literal `*name` in a row is therefore visible, not a silently wrong value.
- Multiple `values-*.yaml` files are multiple assertions of the same item and thus the
  normal case of a deviation.

### CI

`.github/workflows/*.y{a,}ml`, `.gitlab-ci.yml`, `.gitlab/**/*.yml`.

- `uses: owner/repo@ref` is an `action` entry. A full commit SHA is `digest`, a tag is
  `exact`, and a branch is `floating`.
- `container:`, `services.*.image`, and `image:` are `image` entries.
- Setup actions with version input (`actions/setup-go` with `go-version`, `setup-node`
  with `node-version`, `setup-python` with `python-version`) additionally produce a
  `runtime` entry.

## Normalization

Only what establishes comparability is normalized. The raw declaration is always
preserved.

**Names.** Lowercase; in the Python ecosystem, additionally convert `_` and `.` to `-`
(PEP 503). Node scopes are preserved (`@scope/package`). Go module paths remain complete,
without removing `/v2`. Container images are normalized to `registry/namespace/name`; a
missing `docker.io/library` is **not** added, otherwise two identical assertions would look
different depending on who wrote them. The group key is `<ecosystem>/<name>` and is thus
local to an ecosystem: Python `redis` and an image `redis` are two items.

**Versions.** `version` is the source verbatim. `versionNormalized` is produced only for
`pin: exact`: leading `v`, `==`, and whitespace are removed; the rest remains. For
`range`, `floating`, `digest`, `local`, and `unknown`, `versionNormalized` remains empty:
a normalized range would be an interpretation, and the inventory does not interpret.
The single place where a range is evaluated is the pair rule for `package-lock.json` under
"Deviations"; it only decides how rows are classified and never rewrites `version`,
`versionNormalized`, or `pin`.

**Digests.** In the form `sha256:<64 hex>` or as a full 40-character commit SHA. Short
SHAs are not extended and count as `unknown`.

**Local builds.** `pin: local`, empty `version`, and `note` states the origin: stage, path,
or build context.

**Source and line declarations.** `sourceFile` is relative to the project with `/` as the
separator, including on Windows. `sourceKey` is a dot path; list indices are in square
brackets and start at `0`. `sourceLine` starts at `1` and is set only when the parser can
assign the assertion to exactly one line.

## Deviations

Grouping uses `group` (`<ecosystem>/<name>`), not the display name.

Within a group, classification compares **statements**. A statement is usually one row,
with its `version` and `pin`. The one exception is a consistent pair from `package.json`
and `package-lock.json` (see below): it is a single statement made of the set of its
declarations (`version` and `pin` of each manifest row) **and** the resolved lock version.

1. If all statements are the same, there is no deviation. For a single row, "the same"
   means the same `version` **and** the same `pin`; two consistent pairs are the same only
   if both their sets of declarations and their lock versions are equal, and a pair is
   never the same as a single row. The rows are included in the context table, with their
   origins listed alongside each other.
2. If they differ, a deviation of one of the two types arises:
   - `umgebungsbedingt` means the deviating statements have **different** `context`
     values. This is the normal case and usually intentional: a newer version in `lokal`
     than in `deployment`.
   - `widersprüchlich` means the deviating statements have the **same** `context`. This is
     the case that raises a question: two Compose files for the same environment,
     `Chart.yaml` versus `Chart.lock`, two manifests of the same context that declare the
     same package differently, or a manifest versus a lockfile outside the pair rule, for
     example `yarn.lock`, `pnpm-lock.yaml`, `poetry.lock`, or a `package-lock.json` whose
     version violates a range.
3. A deviation is **never** resolved, consolidated, or reduced to a "correct" value.
   Treating a consistent pair as one statement applies **only** to classification; a
   deviation is still reported with **all** involved rows and their origins, manifest and
   lockfile rows alike.

### Pairs from `package.json` and `package-lock.json`

A pair exists only in the `node` ecosystem, between a `package-lock.json` and the
`package.json` **in the same directory**, and only within the same `context`. It consists
of the lock row of a package and **all** rows of that package from this `package.json`,
for example `peerDependencies` `>=3` and `devDependencies` `^3.17.1`. Pairing only by
package name and context is not enough: two directories with their own manifest and
lockfile in the same context would then mix, and a lock version would be checked against
another directory's range.

The pair is **consistent** if the lock version satisfies **every** range of its
declarations under npm range semantics. A consistent pair counts as one statement, as
described above. Otherwise its rows stay separate statements, exactly as without the rule:

- **Violated range.** If the lock version does not satisfy a declaration, the rows stay
  separate, which is always a deviation, and the lock row carries this `note`, once per
  violated declaration and joined by `; `:
  `Lock-Version <lock version> erfüllt den Range <range> nicht (<manifest file>:<line>, <sourceKey>)`
  ("lock version does not satisfy the range").
- **Range or lock version not checkable.** The rows stay separate statements without a
  `note`, as they were before the rule.
- **Row without a partner.** A lock row whose `package.json` is absent or does not declare
  the package, and a manifest row without a lock row, are each a statement of their own.

The link between a lock row and its `package.json` is internal to classification. It is not
a field of the inventory row and does not appear in the JSON output: the row already names
its `sourceFile`, from which the partner follows, and no consumer needs a second path.

**Checkable forms**, evaluated with the semantics of npm's `semver` package without
options:

- exact versions `1.2.3`, with or without `=` or `v`; build metadata `+…` is ignored;
- partial versions and wildcards `1`, `1.2`, `1.x`, `1.2.x`, `1.2.*`, `*`, `x`, `X`, and the
  empty string;
- `^` and `~` (also `~>`), including the special cases `^0.x`, `^0.0.x`, and `~1`;
- comparisons `>`, `>=`, `<`, `<=`, also with partial versions (`<2` means `<2.0.0-0`);
- hyphen ranges `1.2.3 - 2.3.4`;
- conditions joined by whitespace (all must hold) and alternatives joined by `||` (one
  must hold);
- prereleases by npm's rules: a prerelease lock version satisfies an alternative only if
  one of its conditions itself names a prerelease of the same `major.minor.patch`, and
  `^1.2.3` does not admit `2.0.0-beta` because its upper bound is `<2.0.0-0`.

**Not checkable**: `npm:` aliases, `workspace:`, `link:`, `file:`, Git URLs and
`owner/repo` shorthands, tarball URLs, dist tags such as `latest`, and every other
declaration that is not a valid range; likewise a lock version that is not a complete
version, for example a linked package.

**Boundaries.** The rule deliberately covers nothing else:

- `yarn.lock` stays unchanged: its assignment to manifests is one-to-many. The direct
  names come from one shared name→scope map over the root and all workspace members, and
  every lock head whose package name is direct becomes a row, including heads with
  transitive ranges. A row therefore has no single partner.
- `pnpm-lock.yaml` stays unchanged: its reader records the manifest's `specifier` as
  `version`, not the resolved version, so there is nothing to check. Such a row carries the
  same range as its manifest and does not produce a false deviation.
- npm workspaces stay unchanged: member entries `packages["<member>"]` are not read, and a
  member manifest without its own lockfile has no partner.
- Python, Rust, Elixir, and every other ecosystem stay unchanged; a manifest versus its
  lockfile remains a deviation there whenever the rows differ.

**Example.** A `package.json` without a lockfile declaring `^3.17.1` and, in the same
context, a consistent pair `^3.17.1`/`3.17.1` remain a `widersprüchlich` deviation with
three rows: the resolution of the first is unknown, and nothing is guessed. Likewise two
consistent pairs `^3.14.8`/`3.17.1` and `^3.17.1`/`3.17.1` are a deviation because their
declarations differ, and two consistent pairs `^3.14.8`/`3.14.8` and `^3.14.8`/`3.17.1`
because their lock versions differ; each shows four rows.

Both types appear in the "Abweichungen" (deviations) section of the inventory file, with
`widersprüchlich` first. The number of deviations is the number of groups with a deviation,
not the number of involved rows.

A difference between `docs/libs/` and the inventory is **not** a deviation in this sense:
it arises not from two sources but from two documents. `/k-docs` reports it as a separate,
non-remediating finding; the inventory prevails.

The frontmatter pair `version`/`version-pin` of a file from `docs/libs/` is compared with
the item of the same name in the inventory's context tables, matched by the name after the
group key `<ecosystem>/<name>`. A tool that does not appear in the inventory is not a
finding, nor is a lib file without `version`. `docs/libs/` is **not** rewritten in the
process, not even after confirmation: those files belong to `/k-docs-tools`.

## Trust Boundary

The source configuration can allow paths outside the project: host files and external
deployment repositories. They are read by a binary that also runs a local web server in
the same process model. Symlinks, `..`, and glob escapes are therefore not edge cases.

The boundary is defined **once** and applies identically to every invocation path: the
subcommand, command, and web API from Task 043. Stage 3 implements it in **exactly one
place** in `internal/`: a function that checks a requested path and returns either the
checked absolute path or an error. No reader opens a file without going through this
function.

### Allowed Roots

Exactly two kinds of root are allowed:

1. The **project root**: `project.dir` from `k-playbook context`, namely the directory
   containing `K-PLAYBOOK.yaml`. It is always allowed and does not appear in the
   configuration.
2. Every root explicitly allowed under `roots:` in `version-sources.yaml`.

Nothing else. There are no implicit roots: an absolute path in `sources:` does **not**
automatically allow its root. Anyone who wants to read outside the project writes down the
root, so that the question "what may this binary read" is answered in one place and
without evaluating globs.

`playbook.dir` is not a special root. It is below the project root and **may** be read
like any other directory; nothing is ever written there anyway. That default detection
nevertheless does not search it is a separate matter: it is stated under "Where default
detection does not search" and changes nothing about this boundary: an entry in `sources:`
that points there is read.

### Path Normalization

Before **every** check, in this order:

1. **No expansion.** `~`, `$VAR`, `%VAR%`, and shell metacharacters are not evaluated;
   they are part of the path. A path whose meaning depends on the caller's environment
   means something different on the CLI path than in the web-server process, precisely
   what a trust boundary must not allow.
2. **Make absolute.** A relative path is resolved against the project root, never against
   the process working directory.
3. **Clean lexically.** `.` and `..` are resolved and repeated separators are collapsed.
   This makes a `..` that leads beyond a root visible, and it fails the check.
4. **Resolve symlinks.** The complete path, including all parent segments, is resolved.
   The **resolved** result, not the requested path, is checked. A symlink inside the
   project that points outside is therefore an escape and is rejected unless its target is
   below an allowed root. The roots themselves are likewise resolved before comparison.
5. **Check.** The resolved path must equal a root or be below a root, compared by segment,
   not as a string prefix. `/srv/deploy` allows `/srv/deploy/x`, but not
   `/srv/deploy-alt/x`.

Globs are expanded only **after** steps 1 through 3, using their static part, and only
within an allowed root. **Every** result of expansion then individually goes through steps
4 and 5. A glob is therefore never a way around the check.

Only regular files are read. Directories, device files, FIFOs, and sockets are rejected.
The file-size limit is 8 MiB; exceeding it is a visible rejection, not a partial read.

The boundary is implemented as `inventory.Boundary` in
`installer/internal/inventory/trust.go`. It is the only place that opens a source:
`Check` checks a path, `Expand` resolves a glob, and `ReadFile` reads. It does not use
`internal/pathnorm`: its `Normalize` answers whether two SARIF paths point to the same
location, lowercases them for that purpose, and removes a leading `/`. That would be
wrong for a security check: `/etc/passwd` and `etc/passwd` must not be the same here.

### Rejection Is Visible

Every rejection produces a message in the result: in the "Abgelehnte Quellen und
Hinweise" (rejected sources and notices) section of the inventory file, in the subcommand
output, and in the API response. It states the requested path, resolved path, and reason.

**There is no silent skipping.** A configured source that could not be read is a gap in
the inventory; a gap no one sees is worse than an error.

The run therefore does **not** abort: the remaining sources are collected, the inventory
is written, and it contains the rejections. Only an unreadable source configuration
aborts; see "Error cases".

## Source Configuration

`k-playbook-local/version-sources.yaml`. YAML, because `K-PLAYBOOK.yaml` uses it too and
a second format would provide no benefit.

**Write rule.** The file is maintained manually. The command, subcommand, and every
future interface may write to it only with the user's explicit confirmation, and then
only additively: existing entries, comments, and ordering remain untouched. Without
confirmation, nothing is written. The single in-place change allowed is raising
`schema_version: 1` to `2` when the confirmed addition is the first `helm_values` entry;
it is part of the diff shown for confirmation.

**State through `context`.** Commands read configuration exclusively from
`k-playbook context`. The state of this file, whether present or absent, the allowed
roots, and the configured sources, therefore belongs in the context output. The schema
and field names are stated below under "State in the context output"; Stage 3 builds the
reader as the **only** implementation used by both the collector and
`installer/internal/project/context.go`.

### Fields

| Key | Required | Meaning |
|---|---|---|
| `schema_version` | yes | `1` or `2`. `2` is required only by `helm_values`; see "Schema version" below. Another value aborts the run instead of interpreting fields that could mean something else. |
| `roots` | no | List of absolute paths that may be read in addition to the project root. Empty or absent means only the project root. |
| `sources` | no | List of additional sources. Empty or absent means only the default sources below the project root. |
| `exclude` | no | List of patterns where default detection does not search. Each pattern is a path relative to the project root; `*` means any number of characters within a segment, `**` any number of segments. A pattern without a wildcard matches the path itself and everything below it. An absolute pattern is visibly rejected: it would depend on the machine and match nothing on another one. |
| `helm_values` | no | List of configured Helm values; see "Configured Helm values" below. Requires `schema_version: 2`. |

For each entry in `sources`:

| Key | Required | Meaning |
|---|---|---|
| `path` | yes | File or glob. Relative to the project root or absolute; absolute only within a root from `roots`. |
| `kind` | yes | Source type: `auto`, `python`, `go`, `node`, `rust`, `ruby`, `php`, `java`, `elixir`, `dockerfile`, `compose`, `devcontainer`, `helm`, `ci`, `tool-versions`. `auto` determines the type from the file name as for default sources. |
| `env` | yes | Environment label: `lokal`, `dev`, `devcontainer`, `ci`, `deployment`. |
| `note` | no | Display text for the inventory's source list. |
| `optional` | no | `true` means a missing file is not a notice. Without the key, a configured but missing source is a visible notice. |

`sources` **supplements**, rather than replaces, the default sources. There is no switch
that disables default detection: an inventory that does not include the project sources
would not be one. `exclude` does not disable it either: it removes named areas, visibly
and individually, and an entry in `sources` includes each of them again.

### Configured Helm values

Some version declarations in `values*.yaml` are not image references and cannot be
recognized without guessing, for example a subchart that takes only a tag
(`redis.standalone.tag: v7.4.11`) and builds the image name in its own template.
`helm_values` names such a value explicitly. For each entry:

| Key | Required | Meaning |
|---|---|---|
| `path` | yes | File or glob of the values file, relative to the project root or absolute, like `path` in `sources`. |
| `key` | yes | Dot path to the value, for example `helm-chart-generic.redis.standalone.tag`. A segment may carry a list index in square brackets starting at `0`: `services[1].tag`. A key that itself contains a `.` cannot be addressed. |
| `item` | yes | The item the value belongs to, as a group key `container/<name>`, for example `container/redis`. The name is normalized like an image name. Only the `container` ecosystem is allowed. |

**Checks.** An entry without `path`, `key`, or `item`, with a `key` that has an empty
segment or an invalid index, or with an `item` that is not `container/<name>`, is visibly
rejected under "Abgelehnte Quellen und Hinweise" with the file's line and the reason, like
an entry in `sources` with an unknown `kind`. The run continues.

**Row.** The value of the key becomes `version` verbatim, and the pin rules for container
tags apply: an empty value or a moving tag such as `latest` is `floating`, an unresolvable
variable is `unknown`, and every other value is `exact`. `kindOfThing` is `image`,
`sourceKey` is the configured `key`, `sourceLine` the line of the value, and `note` is
`konfiguriert in version-sources.yaml` ("configured in version-sources.yaml"). The row
carries the `context` of the source as which the file is read, including an `env` from
`sources`. A configured entry does not make a file a source: the file must be read as Helm
values anyway, by default detection or through `sources`.

**No silent gap.** Every configured entry that yields no row produces a visible notice for
the file, in the form
`konfigurierter Helm-Wert <key> → <item> (version-sources.yaml, Zeile <n>) nicht angewandt: <reason>`
("configured Helm value … not applied"). The reasons:

| Case | Reason text |
|---|---|
| The path is outside the allowed roots | the rejection reason of the trust boundary |
| A glob matches no file | `das Muster trifft keine Datei` |
| The file does not exist | `die Datei liegt nicht auf der Platte` |
| The file is excluded from default detection and not in `sources` | ``die Datei ist durch die Ausschlussregel `<pattern>` von der Standarderkennung ausgenommen; als Quelle unter sources: eintragen`` |
| The file is not a source at all, for example because of its name | `die Datei ist keine Quelle des Inventars; als Quelle unter sources: mit kind: helm eintragen` |
| The file is read with another source type, or as `Chart.yaml`/`Chart.lock` | `die Datei wird als <kind or file name> gelesen, nicht als Helm-values` |
| The file was rejected when read | `die Datei wurde abgelehnt: <reason>` |
| The file could not be evaluated | `die Datei ist nicht auswertbar` |
| The key is missing | `der Schlüssel fehlt in der Datei` |
| The key has a mapping or list instead of a scalar | `der Schlüssel hat keinen Skalar` |

With a glob, every matched file is checked on its own: a file in which the key is missing
produces its own notice, while the other files yield their rows.

### Schema version

`helm_values` is the first key added after `schema_version: 1`. An older binary checks only
the version and does not recognize unknown top-level keys; under `schema_version: 1` it
would skip `helm_values` silently and write an inventory without the configured rows. A
configuration that is applied halfway and silently dropped halfway is worse than an error.

**Decided:** `helm_values` requires `schema_version: 2`.

- This binary reads both `1` and `2`. A file without `helm_values` works unchanged with
  `1`; nothing needs to be migrated, and the template keeps `schema_version: 1`, so a newly
  created configuration remains readable for older installations of the same project.
- Under `schema_version: 1`, `helm_values` is not applied. The section is visibly
  rejected once with its line and the reason
  ``helm_values verlangt schema_version: 2; unter schema_version: 1 wird der Abschnitt nicht angewandt``,
  and the run continues.
- An older binary that understands only `1` aborts on a file with `2`, and its context
  output reports `error`. The failure is visible, which is the point: the file relies on
  a section that binary cannot apply.

The rejected alternative was to keep `helm_values` under `schema_version: 1` and to
document that older binaries skip it. It avoids the abort but leaves exactly the silent
gap this contract rules out everywhere else.

### Template

The following content is both the **example file and the template** created by
`LocalStructure()` in Stage 2: a valid, empty configuration with an explanatory comment.
The template is German, as is the inventory file it configures; the block below is its
exact wording. Stage 2 copies it verbatim into `versionSourcesTemplate()` in
`installer/internal/project/local.go`, which the `fileTemplate()` branch returns for
`version-sources.yaml`, rather than drafting it again.

```yaml
# Versionsquellen für `k-playbook inventory`
#
# Diese Datei ist handgepflegt. k-playbook schreibt nur nach ausdrücklicher
# Bestätigung in sie, und dann ausschließlich ergänzend: bestehende Einträge,
# Kommentare und Reihenfolge bleiben erhalten.
#
# Ohne Einträge erhebt `k-playbook inventory` die Standardquellen unterhalb der
# Projektwurzel — Manifeste, Lockfiles, Dockerfiles, Compose, DevContainer,
# Helm und CI. Hier stehen nur zusätzliche Quellen und die Wurzeln außerhalb
# des Projekts, die dafür gelesen werden dürfen.
#
# Vollständige Beschreibung: k-playbook/docs/version-inventory.md

schema_version: 1

# Zusätzlich lesbare Wurzeln, je ein absoluter Pfad. Die Projektwurzel ist
# immer erlaubt und gehört nicht hierher. Was nicht unter einer dieser Wurzeln
# liegt, wird abgelehnt — sichtbar gemeldet, nicht stillschweigend übergangen.
#
#   roots:
#     - /srv/deploy
roots: []

# Zusätzliche Quellen. Je Eintrag:
#   path: Datei oder Glob, relativ zur Projektwurzel oder absolut
#   kind: auto, python, go, node, rust, ruby, php, java, elixir, dockerfile,
#         compose, devcontainer, helm, ci, tool-versions
#   env:  lokal, dev, devcontainer, ci, deployment
#   note: optionaler Anzeigetext
#
#   sources:
#     - path: /srv/deploy/values-prod.yaml
#       kind: helm
#       env: deployment
#       note: Produktionswerte aus dem Deployment-Repo
sources: []

# Bereiche, in denen die Standarderkennung nicht suchen soll — je ein Muster
# relativ zur Projektwurzel, `*` für ein Segment, `**` für beliebig viele.
# Gedacht für Testfixtures und Beispielprojekte: gepflegter Inhalt, dessen
# Versionen nichts über dieses Projekt aussagen.
#
# Gesperrt ist damit nichts. Jeder Ausschluss steht mit der Zahl der
# übergangenen Quellen im Inventar, und eine Quelle daraus kommt wieder hinein,
# sobald sie unter `sources:` steht.
#
# Die Installation `k-playbook/` ist immer ausgenommen und gehört nicht hierher:
# sie ist ein Clone des Werkzeugs und sagt nichts über dieses Projekt.
#
#   exclude:
#     - tests/fixtures/**
exclude: []

# Versionen in Helm-values, die keine Image-Referenz sind — etwa ein Subchart,
# das nur einen Tag entgegennimmt. Je Eintrag:
#   path: values-Datei oder Glob; sie muss ohnehin als Helm-values gelesen werden
#   key:  Punktpfad zum Wert, Listenindex in eckigen Klammern
#   item: Gegenstand als container/<name>
#
# Der Abschnitt verlangt `schema_version: 2`; unter 1 wird er abgelehnt. Jeder
# Eintrag, der keine Zeile ergibt, steht als Hinweis mit Grund im Inventar.
#
#   helm_values:
#     - path: helm/values.yaml
#       key: redis.standalone.tag
#       item: container/redis
```

### State in the Context Output

`k-playbook context` exposes the state of this file under the `versionSources` key. The
context output thereby answers the question completely, and no command needs to open
`version-sources.yaml` itself, the same rule that already applies to `K-PLAYBOOK.yaml`.

```json
"versionSources": {
  "path": "/pfad/zum/projekt/k-playbook-local/version-sources.yaml",
  "present": true,
  "schemaVersion": 1,
  "roots": ["/srv/deploy"],
  "sources": [
    {
      "path": "/srv/deploy/values-prod.yaml",
      "kind": "helm",
      "env": "deployment",
      "note": "Production values from the deployment repository",
      "optional": false
    }
  ],
  "exclude": ["tests/fixtures/**"],
  "helmValues": [
    {
      "path": "helm/values.yaml",
      "key": "redis.standalone.tag",
      "item": "container/redis"
    }
  ],
  "error": ""
}
```

| Field | Always present | Meaning |
|---|---|---|
| `path` | yes | Absolute path of the file, including when it is absent, so it is clear where it belongs. |
| `present` | yes | Whether the file exists. |
| `schemaVersion` | no | The version declared by the file. Absent until something has been read. |
| `roots` | no | The allowed roots, verbatim as in the file. The project root is **not** included because it is always allowed. |
| `sources` | no | The configured additional sources, in file order. Each entry has `path`, `kind`, `env`, `note`, `optional`, the same names as the YAML keys. An entry with an unknown `kind` or `env` is **included**: the context output shows the file as it stands, and omitting it would represent it differently. It is rejected only during the collection run, visibly there. |
| `exclude` | no | The patterns where default detection does not search, verbatim as in the file. The fixed installation rule is **not** included because it does not come from the file. An absolute pattern that is therefore rejected is likewise not included: unlike `sources`, a pattern is not an entry one can inspect, but a rule that either applies or does not. |
| `helmValues` | no | The configured Helm values, in file order, each with `path`, `key`, and `item`. As with `sources`, an entry the collection run rejects is **included**, including every entry under `schema_version: 1`, where the section is not applied. |
| `error` | no | Set when the file exists but is unreadable or has an unknown version. |

The field names are the file's YAML keys, in camelCase as everywhere else in the context
output; only `schema_version` becomes `schemaVersion` and `helm_values` becomes
`helmValues`. There is no renaming and no second terminology.

Three states, and no more:

- **present and valid**: `present: true`, `error` empty. `roots`, `sources`, `exclude`,
  and `helmValues` contain the content; empty lists mean "nothing configured", not "not
  read".
- **absent**: `present: false`, `error` empty, and `roots`, `sources`, `exclude`, and
  `helmValues` empty. This is not an error: the default sources below the project root
  apply.
- **invalid**: `present: true`, `error` populated, and `roots`, `sources`, `exclude`, and
  `helmValues` empty.

The context invocation does **not** abort for an invalid file. It runs at the start of
every command; an invalid additional configuration must not disable every command. The
state nevertheless remains visible, which is what `error` is for. The inventory
**collection run** does abort, as stated under "Error cases"; this is not a contradiction,
but the distinction between reporting and collecting.

If the field is entirely absent, the installation is older than it. This is the only case
in which a command cannot evaluate it.

## The Inventory File

`k-playbook-local/docs/versions/inventory.md`. The `docs/versions/` directory is the fifth
docs origin; it is not created during setup, but on the first run of its generator, like
`docs/code/`, `docs/libs/`, and `docs/extracted/`.

### Frontmatter

It must be complete; otherwise `/k-docs` step 3 and `/k-docs-index` step 4 report a
finding. `generated.by` alone is insufficient.

```yaml
---
type: Version Inventory
title: Versionsinventar
description: Vollständige Übersicht der deklarierten Versionen dieses Projekts, nach Umgebung getrennt und mit Herkunft je Zeile.
tags: [versions, inventory, dependencies]
status: stable
generated: { by: k-doc-inventory, at: <RFC 3339> }
inventory:
  sources-configured: <N>
  sources-read: <N>
  entries: <N>
  deviations: <N>
  rejected: <N>
  sources-excluded: <N>
  sources-unevaluable: <N>
---
```

`generated.at` is the collection time; there is only this one timestamp.

The keys and the values of `type`, `tags`, and `status` are fixed identifiers and stay
English. `title` and `description` are German, in the exact wording the renderer writes.

### Structure

The body has a deterministic structure; every section is always present, even if empty,
so that a diff does not have to distinguish between "section absent" and "section empty".
The body is German, like `title` and `description`. Headings and fixed text are quoted
here in the renderer's exact wording, with English explanations alongside:

1. `# Versionsinventar` and a paragraph that begins ``Erzeugt von `k-doc-inventory` am``:
   generator, collection time (`generated.at`), the note that the file is regenerated on
   every run and manual changes are lost, and that it lists the **declared** versions, not
   what is active at runtime.
2. `## Übersicht` (overview): a list of eight counters, in this order: `Einträge`
   (entries), `Ausgewertete Quellen` (sources read), `Konfigurierte Zusatzquellen`
   (entries in `sources` of the source configuration, rejected ones included),
   `Abweichungen` (deviations), `Abgelehnte Quellen` (rejections), `Nicht durchsuchte
   Quellen` (sources skipped by an exclusion rule), `Nicht auswertbare Quellen` (sources
   read but not evaluable as a whole, see below), and `Hinweise` (notices). The first seven
   carry the same numbers as the `inventory.*` keys in frontmatter; `Hinweise` appears only
   here.
3. `## <label>` for each environment label in the fixed order `lokal`, `dev`,
   `devcontainer`, `ci`, `deployment`; labels without entries are omitted. It contains a
   table with the columns `Gegenstand` (item: `<ecosystem>/<name>`, the group key), `Art`
   (`kindOfThing`), `Version`, `Pin` (pin type), `Scope`, and `Herkunft` (origin). The
   origin cell contains file and line, `sourceKey`, and, where present, digest and `note`,
   separated by `·`. If no label has any entries, a single `## Einträge` (entries) section
   stands in place of all of them.
4. `## Abweichungen` (deviations): a short paragraph explaining the two types, then one
   block per group, headed `` ### <type> — `<group>` ``, with type `widersprüchlich` before
   `umgebungsbedingt`. Each block is a table of the involved rows with the columns
   `Version`, `Pin`, `Kontext` (context), and `Herkunft` (origin).
5. `## Ausgewertete Quellen` (evaluated sources): a table with one row per read file and
   the columns `Datei` (file), `Quellart` (source type), `Label`, `Einträge` (number of
   entries), `Zustand` (state: `ausgewertet` or `nicht auswertbar`), and `Note`, which
   carries the `note` if one is configured for the source. A source from the source
   configuration carries `(konfiguriert)` (configured) after its source type; otherwise it
   would not be visible where a row comes from.
6. `## Nicht durchsuchte Bereiche` (areas not searched): a fixed paragraph stating that
   these areas are within the project, are not searched by default detection, and are not
   blocked; then a table with one row per exclusion rule and the columns `Muster`
   (pattern), `Herkunft` (origin: `installation` or `configured`), `Übergangene Quellen`
   (number of skipped sources), and `Grund` (reason). A rule that matched nothing still
   has its row.
7. `## Abgelehnte Quellen und Hinweise` (rejected sources and notices): a table with one
   row per rejection and the columns `Angefragt` (requested path), `Aufgelöst` (resolved
   path), and `Grund` (reason), followed by the notices as a list, each prefixed with its
   source file where there is one.

**Empty sections.** A section with nothing to list contains the single line `Keine.`
(none): `## Einträge` when there are no entries at all, `## Abweichungen` without
deviations, `## Ausgewertete Quellen` without read sources, and
`## Abgelehnte Quellen und Hinweise` when there are neither rejections nor notices. In
`## Nicht durchsuchte Bereiche`, `Keine.` follows the fixed paragraph when there is no
rule; since the installation rule always exists, this does not occur in practice.

**Sorting.** Contexts use the fixed order above. Within a context: `ecosystem`, then
`name`, then `sourceFile`, then `sourceLine`, and finally `sourceKey` and `version`, all
ascending with bytewise comparison, not locale-dependent. The last two keys decide cases
where two assertions come from the same line, for example two pinned tools in one `RUN`
line; without them the parser's order would decide. Deviations are sorted by type, then
`group`. Sources are sorted by `sourceFile`. Two runs over the same state therefore
produce the same file, regardless of the order in which the file system returns entries.

### Sources that could not be evaluated

A source that was found and read but could not be evaluated **as a whole** has the state
`nicht auswertbar` ("not evaluable"). Without it, such a source would look like a checked
source without findings: `Einträge 0` and nothing else in its row. The name is chosen over
"nicht lesbar" (unreadable) because two of its triggers concern a readable file whose
evaluation depends on something else, and over "fehlerhaft" (faulty) because an excluded
manifest is not a fault of the lockfile.

It has exactly three triggers:

1. **The source's own parse error**: invalid JSON, JSONC, YAML, or TOML of a known source
   type, or a source type the collector does not know.
2. **A missing root package**: a `package-lock.json` without `packages[""]`, whose direct
   dependencies therefore cannot be recognized.
3. **A required manifest that is absent, unreadable, or excluded**, including a workspace
   member manifest, for a lockfile that takes its direct dependencies from its manifest
   (`yarn.lock`, `poetry.lock`, `uv.lock`, `Pipfile.lock`, `Cargo.lock`, `composer.lock`,
   `mix.lock`). The state is set on the **lockfile**, which is the source that therefore
   yields no entries, not on the manifest.

Each trigger also produces a notice for the source that states the reason; the state does
not replace it. A notice about content, for example an unresolvable variable, an
unreadable `install_requires`, or a configured Helm value that was not applied, does
**not** set the state: the source was evaluated, and the notice concerns one statement.

The state appears identically in every output:

- in the file: the `Zustand` column of `## Ausgewertete Quellen`, the counter
  `Nicht auswertbare Quellen` in `## Übersicht`, and `inventory.sources-unevaluable` in
  frontmatter;
- in JSON: `unevaluable: true` on the source in `sources` of the collection result, and
  `sourcesUnevaluable` in the status read from frontmatter;
- in the interface and in the subcommand output: the same counter, and each such source
  with its file.

A source that is not evaluable still counts under `Ausgewertete Quellen`: it was found and
read. The new counter says how many of those yielded nothing for that reason.

## Byte Stability and Timestamps

The contract to which collector and tests refer equally:

1. A run collects and renders the complete result **in memory**.
2. If the inventory file exists, the result is compared with its contents, excluding
   **only `generated.at`**.
3. If they are equal, **nothing is written**. The file remains byte-identical, including
   its old timestamp and file-system modification time.
4. If they differ, the file is fully rewritten with `generated.at` set to the time of this
   run.
5. If it does not exist, it is written.

It follows that `generated.at` is the time of the last **content change**, not of the last
run. Anyone who wants to know when collection last happened collects again; the result is
the same file. This interpretation resolves the contradiction between "collection time in
the file" and "a repeated run leaves the file untouched"; it is not formulated differently
elsewhere.

Rejections from the trust boundary are content: changing the set of rejected sources
changes the file. The same applies to exclusions: an added pattern in `exclude:` changes
the inventory, visibly in two places: the skipped sources are absent, and the rule appears
with its count in its own section.

## Where 043 Gets Status

**Decided: from the frontmatter of the inventory file. No intermediate artifact is
created.**

The interface from Task 043 needs four machine-readable values: state, time of the last
collection, number of sources, and number of deviations. All four are in the frontmatter
(`generated.at`, `inventory.sources-*`, `inventory.deviations`), and the state follows
from whether the file exists. The number of sources that could not be evaluated is read
the same way, from `inventory.sources-unevaluable`; a file without that key is an
incomplete frontmatter like any other and is written again on the next run.

Only the YAML block between the two `---` lines at the beginning of the file is read. The
Markdown body is **never** parsed: no table is read backward and no heading evaluated.
That was precisely the requirement: status does not come from running text.

This does not prohibit **reading** the file. It is documentation and is read like any
other: for its comparison with `docs/libs/`, `/k-docs` inspects the context tables and
takes item, version, and pin type from there. The boundary is between status and content:
status comes from frontmatter, nowhere else, and no machine derives it from the body.

Three reasons argue against a separate JSON artifact:

- The collector needs the frontmatter reader **anyway**: the "without timestamp"
  comparison from the byte-stability rule requires it to read the contents and find
  `generated.at` there. A second storage location would add code, not save it.
- Two files that say the same thing can diverge, when deleted by hand, restored from Git,
  or after an aborted run. There would then be two answers to the same question, and the
  interface might show the wrong one.
- A machine artifact would need a location outside the docs origins, a versioning decision,
  and an entry in the structure: work for four numbers that are already written.

**Consequence for Stage 3:** The frontmatter reader is **one** function in `internal/`
that returns a status value: `inventory.ReadStatus`, which returns `inventory.Status`.
The collector, `/k-docs`, the subcommand, and the API from 043 use the same one; 043 does
not build a second reader. If the file is absent, that is a defined state ("absent"), not
an error. If it exists but its frontmatter is incomplete or invalid, that is a visible
finding, not a silent zero result.

## Error Cases

Every case has defined, visible behavior. There is no silent empty result anywhere.

| Case | Behavior |
|---|---|
| `version-sources.yaml` is absent | No error. The default sources below the project root apply; the context output reports the file as absent. |
| `version-sources.yaml` is unreadable YAML | **Abort** before any collection, with file, line, and the parser message. Nothing is written. Partially interpreting an invalid configuration would mean applying a trust boundary other than the documented one. |
| `schema_version` is absent or neither `1` nor `2` | **Abort**, as for `K-PLAYBOOK.yaml`. |
| `helm_values` under `schema_version: 1` | The **section** is rejected once, with line and reason, and not applied; see "Schema version". The run continues. |
| Invalid entry in `helm_values` | The **entry** is rejected with line and reason; the run continues. |
| Configured Helm value yields no row | Visible notice for the file with key, item, configuration line, and reason; see "Configured Helm values". |
| A root in `roots:` is not absolute | **Abort**. A relative root would depend on the caller's working directory and mean something different in the web-server process than on the CLI path; partially interpreting a trust boundary stated this way would be worse than rejecting it. |
| Unknown `env` label in an entry | The **entry** is rejected and listed under "Abgelehnte Quellen und Hinweise" (rejected sources and notices), with the found value and the five valid values. The run continues. |
| Unknown `kind` in an entry | As for the label: entry rejected, visibly, run continues. |
| Absolute or empty pattern in `exclude:` | The **pattern** is rejected and listed under "Abgelehnte Quellen und Hinweise" (rejected sources and notices), with line and reason. It then does not apply; the run continues. |
| Configured source is missing from disk | Visible notice, unless the entry has `optional: true`. |
| Path outside allowed roots | Visible rejection with requested and resolved path. The run continues. |
| Symlink points outside every root | As above; the resolved target is reported, so it is clear what would actually have been read. |
| Known source file is invalid | Visible notice with file and error; the source has the state `nicht auswertbar`. The remaining sources are collected. No invented entries, no partial result without marking. |
| `package-lock.json` without `packages[""]` | Visible notice; the lockfile contributes no entries and has the state `nicht auswertbar`. |
| Lockfile readable, associated manifest absent or unreadable | The lockfile contributes no entries and has the state `nicht auswertbar`. A visible notice states lockfile, expected manifest, and reason; transitive lockfile packages do not substitute for direct ones. |
| Lockfile readable, associated manifest excluded by `exclude:` | The lockfile contributes no entries and has the state `nicht auswertbar`. The exclusion also remains visible in the "Nicht durchsuchte Bereiche" (areas not searched) section; the notice names the matching exclusion rule. A lockfile explicitly named under `sources:` overrides the exclusion only for itself, not for its manifest. |
| Workspace lockfile with a missing, unresolvable, excluded, or unreadable member manifest | The lockfile contributes no entries at all and has the state `nicht auswertbar`. The notice states root manifest, member declaration or resolved path, and reason; a partial union of remaining members is not allowed. |
| Unknown file type below the project root | Silently skipped. Only what is searched can be absent. |
| Inventory file exists, frontmatter invalid | Visible finding. The run collects again and writes the file because comparison is not possible. |

## Where This Applies

This page is the contract for:

- the collector in `installer/internal/inventory/`, the source-configuration reader in
  `installer/internal/versionsources/`, and the YAML reader used by both in
  `installer/internal/yamllite/` (Task 042, Stage 3),
- the `k-playbook inventory` subcommand and the `/k-doc-inventory` command with module
  `commands/_docs/inventory.md` (Task 042, Stage 4),
- the inventory area of the interface and its API (Task 043),
- the fixtures and tests of both tasks.

If anything changes, it changes here, and the listed locations synchronize.
