# Review Runs

How a review run is created, what it writes to disk, and how participants record their
progress in it. For the artifacts created by an individual review, see
[`code-review.md`](./code-review.md); for the migration from which this
emerged, see [`migration.md`](./migration.md).

## The Run Is the Container

Previously, every review started its own tools and stored its results in its own location.
A run brings together what belongs together: **one selection, one directory, one state.**

A run consists of **entries**, and an entry has a **kind**:

| Kind | What it is | Who runs it |
|---|---|---|
| `tool` | a security tool from the matrix | the tool, through a CLI |
| `ai` | a review recipe from the catalog or a command module entry | an assistant, through a command |

Both are entries in the same run. Only the path to get there differs: the result for both
ends up in the same directory and therefore later in the same consolidation. The standard
module entry `scan-triage` comes from `commands/_audit/review-scan-triage.md`; it
intentionally does not belong to `catalogs.reviews` and therefore does not appear in the
GUI selection for review recipes.

## The Run Directory

```text
k-playbook-local/results/
└── 2026-08-12/                  the run, named after the day
    ├── run.json                 the definition: what was selected
    ├── entries/
    │   ├── semgrep.json         progress and issues per entry
    │   └── review-secret-scanning.json
    └── raw/                     the SARIF files of the scan jobs
        ├── semgrep.sarif
        ├── gitleaks-git.sarif
        └── tech.sarif            the artifact of an evidence source
```

The name is the date, `YYYY-MM-DD`. **If the directory already exists, creation stops**
rather than placing a second run beside it: one day, one run. To start again, remove or
rename the existing directory.

**`raw/` is created by the runner** when it starts its first job. Creating a run does not
know this directory: a run without a tool entry does not need it. An evidence source also
writes here, to `raw/<entry>.sarif`, and creates the directory if no tool ran before it.

**There are two locations for raw data.** The result families under
`k-playbook-local/results/<family>/YYYY-MM-DD/raw/` remain as they are: a targeted
`/k-review` run continues to create them, and `/k-remediation` works through the triage
beside them; `ListRuns()` displays them as well. The run directory, by contrast, has no
family because a run is precisely the container across families. Catalog recipes in the
run model do not create their own raw-data storage: a perspective reads the shared merge
evidence from this run directory, while an evidence source writes its SARIF to its `raw/`;
no recipe opens a family directory for this purpose.

## Who Writes What

The decisive point when multiple participants work at the same time: **no one writes to
another's file.**

| File | Writer |
|---|---|
| `run.json` | only run creation; no one afterward |
| `entries/<name>.json` | only the entry to which it belongs |
| `raw/<job>.sarif` | only the job that produces it; it is removed by the entry to which it belongs |

The job is the writer, the entry is the cleaner, and it removes only its own files from
the previous `entries/<tool>.json`. "No one writes to another's file" remains unaffected.

This allows tools to run in parallel without overwriting one another. A single shared file
would be easier to read, but the second writer would delete the first one's progress: with
parallel scans, this is not an edge case but the normal case. Writes are **atomic**: first
a temporary file in the same directory, then `rename`. Otherwise, a reader checking during
the run would eventually see a partial file.

Because execution does not touch `run.json`, no lock per run is needed. Two simultaneous
calls collide only when they name the same entry: that is the same case as a repeated call,
and is considered overwriting rather than additive.

**The overall state is derived on read** from the files under `entries/`; a missing file
counts as `start`. If all entries are `start`, the run is `created`; if all are in a final
state, it is `done`; otherwise it is `running`.

**Precedence rule:** If the state in `run.json` differs from that under `entries/`,
`entries/` applies. `run.json` records what was selected, not how far it has progressed.

## `run.json`

```json
{
  "schemaVersion": 1,
  "created": "2026-08-12T14:03:11+02:00",
  "state": "created",
  "languages": ["python", "go"],
  "entries": [
    { "name": "semgrep",                "kind": "tool", "state": "start" },
    {
      "name": "secret-scanning",
      "kind": "ai",
      "state": "start",
      "recipeKey": "secret-scanning",
      "recipePath": "/projekt/k-playbook/reviews/review-secret-scanning.md",
      "recipeOrigin": "dist",
      "title": "Secret-Scanning Assessment",
      "mode": "perspective",
      "resultRequired": true,
      "defaultResult": "review-secret-scanning.md",
      "scope": {
        "tools": ["gitleaks", "trufflehog"]
      }
    },
    {
      "name": "tech",
      "kind": "ai",
      "state": "start",
      "recipeKey": "tech",
      "recipePath": "/projekt/k-playbook/reviews/review-tech.md",
      "recipeOrigin": "dist",
      "title": "Tech Debt Analysis",
      "mode": "evidence",
      "resultRequired": false,
      "scope": {
        "paths": ["**/*.go", "**/*.py"]
      }
    }
  ]
}
```

The keys are English, like everything else the tool outputs as JSON: `schemaVersion`,
`missingRequired`, `installMethod`. They are derived from the Go field names.

`languages` records the language selection with which the run was created. It may change
afterward; what ran should nevertheless remain traceable.

The `entries` in `run.json` carry the state the run **defined**. The file under `entries/`
tracks actual progress.

When created, AI entries copy metadata from the review recipe into `run.json`. Later
changes to the recipe no longer change the run. The optional metadata is in the recipe's
YAML frontmatter:

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

`audit.enabled` defaults to `false`; only `true` includes the recipe in `/k-audit`/MCP runs.
`review.enabled` defaults to `true`; `false` removes the recipe from `/k-review` selection.
Without a value, `title` falls back to the first heading or the catalog key.
`audit.mode` defaults to `perspective`; recipes without the field remain valid unchanged.
Which additional fields apply depends on it; the complete contract is under
[Catalog Recipes in the Run](#catalog-recipes-in-the-run).

For `mode: perspective`, `resultRequired` defaults to `true` and determines whether a
`done` state requires a result. `defaultResult` is a relative suggestion in the run
directory. `scope.tools` is the tool scope frozen at creation.

For `mode: evidence`, `scope.paths` is the path scope frozen at creation.
`resultRequired` is forced to `false` and recorded that way in `run.json`, not because the
recipe would declare it that way, but because it is not permitted there: the required
artifact is `raw/<entry>.sarif`, and with `true`, `review_status` would report every
successful evidence entry as `resultMissing` and `inconsistent`. `defaultResult` is absent
for the same reason.

`audit.ruleIds` is **not** frozen in `run.json`. The scope records what the entry ran
against; the rule-ID list, by contrast, is the contract against which its artifact is
measured when reporting, and it is read fresh from the recipe. A recipe that changes the
list after the run starts therefore changes the validation; if it no longer fulfills the
evidence contract, reporting fails with `recipe_contract_invalid`.

Later recipe changes otherwise do not alter existing runs.

The `scan-triage` module entry receives the same run metadata from the effective command
namespace: `recipePath` points to `commands/_audit/review-scan-triage.md`,
`defaultResult` is `review-triage.md`, and `resultRequired` is `true`. An empty local
overlay under `k-playbook-local/commands/_audit/review-scan-triage.md` disables this entry.

## Catalog Recipes in the Run

An active catalog recipe operates in one of two modes in a run. `audit.mode` in the recipe,
not the command, specifies which one; if absent, it is `perspective`, so recipes from
before the second mode remain valid unchanged.

The `audit` block is optional. If it is absent, the recipe remains inactive in the audit
run model. `audit.enabled: false` disables only `/k-audit`/MCP selection;
`review.enabled` continues to control targeted `/k-review` selection.

The block's fields each belong to exactly one mode. A recipe that mixes them is not silently
adjusted: it appears in the selection basis under `unavailableCandidates` with
`selectable: false` and an `unavailableReason` that names the violated rule.

### Perspective (`mode: perspective`)

A perspective runs **after** the merge and is not a scanner of its own. It reads the merge
output `review-input.json` and writes exactly one Markdown file directly into the run
directory, for example `review-secret-scanning.md`. The complete example of this contract
is in the [`review-secret-scanning.md`](../reviews/review-secret-scanning.md) recipe.

Frontmatter contract:

```yaml
---
name: review-<key>
title: <Title>
audit:
  enabled: true
  mode: perspective
  defaultResult: review-<key>.md
  resultRequired: true
  scope:
    tools: [<tool>, <tool>]
review:
  enabled: true
---
```

Scope semantics for `mode: perspective`:

- `scope.tools` filters at the evidence level of `review-input.json`.
- A group belongs to the perspective when at least one item of evidence in that group has
  an `evidence.tool` from `scope.tools`.
- The original group ID remains unchanged. The recipe does not deduplicate, split, or
  renumber groups.
- The perspective assesses only evidence from `scope.tools` as the primary finding.
- Evidence from other tools remains visible as context, but must be clearly marked in the
  report as "outside the scope".
- A finding can appear in multiple perspectives. This is intentional and not a
  deduplication error.
- Empty scope results are valid. The recipe then writes a result file with the status
  "no scoped findings" rather than obtaining its own scans.

### Evidence Source (`mode: evidence`)

An evidence source runs **before** the merge. It reads code in the frozen path scope and
supplies part of `review-input.json` rather than reading it. Its required artifact is SARIF
under `raw/<entry>.sarif`; no Markdown result is created. The two converted examples are
[`review-tech.md`](../reviews/review-tech.md) and
[`review-python-comment-hardspots.md`](../reviews/review-python-comment-hardspots.md).

Frontmatter contract:

```yaml
---
name: review-<key>
title: <Title>
audit:
  enabled: true
  mode: evidence
  ruleIds: [<rule-id>, <rule-id>]
  scope:
    paths: ["<glob>", "<glob>"]
review:
  enabled: true
---
```

Both `scope.paths` and `ruleIds` are required. Without a path scope, the entry would read
the entire repository; without a rule-ID list, its findings would not be comparable from
run to run. `scope.tools`, `resultRequired`, and `defaultResult` are prohibited here: the
first describes a filter on `review-input.json`, which the entry does not read, and the
other two describe a result file that does not exist.

Result contract, validated when reporting through `k_playbook_review_write_ai_entry`:

- `tool.driver.name` in the SARIF matches the entry name. If it says something else, the
  artifact claims an origin it does not have.
- Every SARIF rule ID is in `audit.ruleIds`. An unknown one makes the entry `failed` with a
  reason; it is not silently `done`.
- Findings outside `scope.paths` or within inherited exclusions are discarded and counted,
  and `raw/<entry>.sarif` is written back sanitized. The entry remains valid; the count and
  first paths are in `reason`. A single outlier therefore does not invalidate the whole
  artifact.
- SARIF without results is valid `done`. An empty scope finding is a result, not an error:
  the same rule as "Empty scope results are valid" for perspectives.

The `level` for each finding is the assessment assigned by the recipe, and the merge takes
it seriously: `error` and `note` apply unchanged (`severitySource: native`), while
`warning` and `none` continue through CVSS, tool metadata, and `scripts/severity.tsv`.
The recipe defines the `level` for each rule ID; details are in
[`rules/review-authoring.md`](../rules/review-authoring.md).

Scope semantics for `mode: evidence`:

- `scope.paths` are globs relative to the project root. Comparison is segment by segment;
  `**` skips any number of segments, while `*` and `?` apply within a segment. A pattern
  also matches a file when it matches its directory: `installer` and `installer/**` both
  cover `installer/internal/review/run.go`.
- Within the globs, the central exclusions of module search also apply: `k-playbook/`,
  `k-playbook-local/`, `vendor/`, `node_modules/`, `testdata/`, and dot directories. They
  are inherited and do not belong in the recipe again.
- The path scope is binding and enforced when reporting. `scope-hint` remains free text
  for `/k-review` and may neither extend nor override it.
- The group ID is created in the merge, not the recipe: AI findings form one group per rule
  ID and file, without line or message, using the `ai-<entry>-` prefix. Multiple instances
  of the same problem in one file are therefore one group; their count is in `findingIds`.
  The recipe does not consolidate them itself.
- A group containing scanner and AI evidence retains the scanner ID.

### Order in a Run

A `/k-audit` run processes entries in this order:

1. Tool entries run and write `entries/<tool>.json` and `raw/<job>.sarif`.
2. AI entries with `mode: evidence` run in the frozen path scope and write
   `raw/<entry>.sarif`.
3. The merge writes `review-input.json` and `review-input.md` from the tool entries
   **and** the evidence entries in a final state: it reads entries whose state is `done`
   and whose job names a SARIF in the run directory.
4. Active catalog recipes with `mode: perspective` read `review-input.json` as
   perspectives.
5. `scan-triage` can optionally run afterward and use perspective reports as context.

Open, failed, or not-yet-run AI entries do not block the merge, neither perspectives nor
evidence sources. They can leave the AI part open, but `review-input.json` must remain
creatable from tool scans. If evidence arrives later, another merge is the regular path,
not a repair case; a `review-triage.md` written before that merge is outdated afterward and
is rewritten.

### Repair and Verification

Repair matrix for AI entries. What an entry is measured against depends on its mode: for
`mode: perspective`, the result file; for `mode: evidence`, the SARIF.

| Mode | State | Behavior |
|---|---|---|
| `perspective` | Entry exists in `run.json`, result file is missing, entry is open | State remains open; rerun executes the AI entry again and writes the result file. |
| `perspective` | Entry exists in `run.json`, result file is missing, entry is complete | Status output marks the entry inconsistent and `resultRequired` unmet; rerun writes the result file again and repairs the state. |
| `perspective` | Entry exists in `run.json`, result file exists, entry state is missing or open | Status output may mark the entry repairable; rerun need not force a new file, but may set the state to complete through `k_playbook_review_write_ai_entry` if the file is not empty. |
| `perspective` | Entry exists in `run.json`, result file exists, entry is complete | No repair required. |
| `evidence` | Entry is `done`, job exists, `raw/<entry>.sarif` exists and is not empty | No repair required. |
| `evidence` | Entry is `done`, but without a job or existing SARIF | Status output reports `sarifMissing` and `inconsistent`; only another recipe run repairs it. A subsequently written state without an artifact remains an empty promise: the merge reads the file, not the state. |
| `evidence` | Valid SARIF, entry state is missing or open | Status output reports `repairable`; `k_playbook_review_write_ai_entry` with the job is sufficient, and another recipe run is not needed. |
| `evidence` | Entry is `failed` | Replace through another recipe run, not by writing the state afterward. |
| `evidence` | Findings outside `scope.paths` reported | Partial acceptance: findings are discarded, `raw/<entry>.sarif` is written back sanitized, and the entry remains valid. The count and first paths are in `reason`. |
| both | Recipe was disabled or changed after the run started | The existing run continues to use the `scope` snapshot in `run.json`. By contrast, the rule-ID list is read fresh from the recipe when reporting; if it no longer fulfills the evidence contract, reporting fails with `recipe_contract_invalid` and writes nothing. |
| both | Old run contains no entry for a recipe added later | No automatic retroactive addition to the old `run.json`. New recipe entries are created only when creating a new run. |

Manual verification after changes to the catalog-recipe run model:

1. Start `/k-audit 2026-08-21` in a new assistant session and read the status.
2. Verify that `secret-scanning` creates `review-secret-scanning.md` after the merge
   without an alignment note and names the `gitleaks`, `trufflehog` scope.
3. Verify that `tech` and `python-comment-hardspots` run as evidence entries before the
   merge, write their SARIF to `raw/<entry>.sarif`, and afterward appear as groups with
   the `ai-<entry>-` prefix in `review-input.json`.
4. In a CVE-heavy target, create a small run with dependency tools and verify that
   `dependency-cve` runs as a perspective over `review-input.json`.
5. In status output and create dry-run, verify that active recipe entries show their stored
   `scope` and `mode`, and that the `evidenceCandidates` and `perspectiveCandidates` lists
   reproduce the run's order.
6. Simulate one damaged AI entry from each repair-matrix case and verify that status or
   rerun shows the expected repair.
7. Create a recipe with a contradictory `audit` block and verify that it appears under
   `unavailableCandidates` with a reason rather than silently seeming active.

## States

For the run:

| State | Meaning |
|---|---|
| `created` | created, nothing started yet |
| `running` | at least one entry is running or complete |
| `done` | all entries have completed |

There is no `failed` run state: a technical failure belongs to the entry, not the run. A
run in which a tool fails is nevertheless complete.

For an entry:

| State | Meaning |
|---|---|
| `start` | selected, not started yet |
| `running` | currently running |
| `done` | complete, result is available |
| `failed` | technically failed |
| `skipped` | skipped, for example because the tool is missing |

`failed` always means a **technical** failure, never a finding. A scanner that finds
problems is `done`: that is its job. Almost all scanners finish with a non-zero exit code
as soon as they find something; what matters is therefore not the code, but whether
readable SARIF is available.

`skipped` also covers the case where a tool **itself** signals that there was nothing to
check under the reference point: not as an accident of an exit code, but as a deliberate
message. In its `soft_skip` column, the catalog carries one or more rules per row in the
form `<Exit-Code>:<Regex>`, separated by `;`. If the process exit code and a pattern in
stderr or stdout match, the runner executes the job as `skipped` with the matching row as
the reason, rather than interpreting it as a technical failure. The marker applies only
when the process ends normally with an exit code and no SARIF file was written or the file
is empty. **Readable SARIF wins** and remains `done`; **broken, non-empty SARIF remains
`failed`**. Timeouts, runner cancellation, and startup failures also remain `failed`: the
marker means a deliberate outcome, not a technical one. The case that triggered this so
far is `osv-scanner`: exit 128 plus "No package sources found", without a SARIF file.

**An empty result cannot be interpreted without the candidate count.** `done` with zero
findings means only that readable SARIF is available, not that anything was checked.
"Checked and clean" and "nothing checked" write the same file, and the latter case is
harmful: it passes on a false-negative finding as reassurance. Therefore, every job for
which counting was possible carries the number of files that could have been subject to
inspection (`candidates`, see below). This does not create a new state: the count separates
"nothing to check" from the other two cases, not "checked and clean" from "nothing
checked". The tool establishes the facts; assessment happens in the assessment step.

## Running an Entry

```text
k-playbook scan <lauf> [eintrag …]
```

Without an entry argument, all tool entries that are `start` under `entries/` run. The
command **blocks** until all selected entries are complete; progress is read solely from
`entries/` while it runs.

**Only `kind: tool`.** An `ai` entry remains untouched at `start`: an assistant runs it
through its command, which is also when its file is created. If it is explicitly named,
the command reports this on stderr and leaves it as is.

### The Entry Is the Tool, the Job Is the Invocation

An entry is a tool from the matrix. How that tool is invoked is specified in
[`scripts/scanners.tsv`](../scripts/scanners.tsv): one row per **job**, with languages,
time limit, and invocation including placeholders. A tool can have multiple jobs, one, or
none:

| Level | Name | File |
|---|---|---|
| Entry | `trivy` | `entries/trivy.json` |
| Job | `trivy-fs`, `trivy-config` | `raw/trivy-fs.sarif`, `raw/trivy-config.sarif` |

Someone assembling a run does not see these: the interface offers tools, not jobs. A job
whose language was not selected, whose tool is missing, or which cannot produce SARIF is
`skipped` with a reason, not `failed`.

**The name comes from the catalog, except with multiple modules.** A catalog row with
`workdir: module` does not name a fixed working directory, but requires one: the runner
searches for modules beneath the target and starts the job once for each found module,
using that module as the working directory. This is needed for invocations such as
`govulncheck -format sarif ./...`, which have no path argument at all.

| Found modules | Jobs | Name |
|---|---|---|
| none | one, `skipped` with a reason | from the catalog |
| exactly one | one | from the catalog, unchanged |
| multiple | one per module | catalog name plus derived suffix, `govulncheck-installer` |

The suffix is derived from the module path relative to the target: path separators become
`-`, and anything unsuitable for a filename is removed. Two modules can produce the same
name; a digit is then appended to the second one, otherwise its job would overwrite the
first one's file.

No discoverable module is `skipped`: the subject is missing, not the tool. A search that
cannot itself be performed, due to a read error or missing permissions, is `failed`:
whether a module exists is then unknown, and `skipped` would claim that there is nothing
to do.

The search skips the installation copy `k-playbook/`, the project-owned
`k-playbook-local/`, plus `vendor/`, `testdata/`, `node_modules/`, and everything with a
leading dot: none of these contain a project module. This list is in code rather than the
catalog: unlike a tool exclusion, it does not belong to an invocation.

**The job records which module was checked**, including the one-module case where the job
and filename remain unchanged. Otherwise, the name alone would provide that information
only when the job was split, which is not the usual case.

The executable for a job comes from the preflight (`install-security-tools.sh --json`),
not from its own PATH resolution: otherwise, a run in a Python project with an active venv
would use its `ruff` and thus check a different tool than the preflight
([`rules/tool-install-scope.md`](../rules/tool-install-scope.md)).

Jobs run in parallel, with a limit across the whole run. **They do not write directly**:
they report their result to their entry, and the entry writes its file. This preserves one
writer per file.

### Deriving an Entry State from n Jobs

First determine whether the entry is complete at all, then determine its outcome:

0. A job is still running or pending -> `running`.
1. Otherwise, at least one job `failed` -> `failed`.
2. Otherwise, at least one job `done` -> `done`.
3. Otherwise -> `skipped`.

Rule 0 comes first because otherwise rule 3 would treat a running entry as `skipped`, and
the run would derive `done` from it while it was still running. `done` ranks above
`skipped` because a strict worst outcome would make `gitleaks` `skipped` whenever
`gitleaks-dir` is skipped, hiding the file that `gitleaks-git` actually wrote. Therefore,
`skipped` means exactly one thing: the entry is complete and not a single job ran. This
also includes “no job at all”, as with `syft`, which creates an SBOM rather than findings.

### `entries/<name>.json`

```json
{
  "schemaVersion": 1,
  "name": "trivy",
  "kind": "tool",
  "state": "done",
  "started": "2026-08-13T09:12:04+02:00",
  "finished": "2026-08-13T09:13:41+02:00",
  "jobs": [
    { "job": "trivy-fs",     "state": "done",    "exitCode": 1, "sarif": "raw/trivy-fs.sarif",
      "findings": 12, "candidates": 3, "started": "…", "finished": "…" },
    { "job": "trivy-config", "state": "skipped", "reason": "language not selected" },
    { "job": "govulncheck",  "state": "done",    "module": "installer", "exitCode": 0,
      "sarif": "raw/govulncheck.sarif", "findings": 0, "candidates": 2,
      "started": "…", "finished": "…" }
  ]
}
```

The entry carries its derived state, while the individual jobs remain visible beneath it:
the overall state is the summary, not the only information. `exitCode`, `findings`, and
`candidates` are absent when nothing was measured; 0 would mean “measured and zero” here.
`reason` appears for `skipped` and `failed`; a tool without a job carries it on the entry
because there is no job to carry it. `module` names the checked module relative to the run
target, with the root itself as `.`; it is absent for `workdir: target`, where there is no
module it could refer to. The example intentionally mixes these: the `trivy` jobs run
project-wide, while `govulncheck` runs on one module.

#### `candidates`: What the Job Could Have Checked

`candidates` is the number of files that could have been subjects under the job's
**reference point**: the module for `workdir: module`, otherwise the run target. Which
files count is specified by the `candidates` column in
[`scripts/scanners.tsv`](../scripts/scanners.tsv):

| Kind | Candidate is | For |
|---|---|---|
| `source` | a file with an extension for a language from `languages` | `gosec`, `ruff`, `golangci-lint`, `semgrep` |
| `any` | every file | `gitleaks`, `trufflehog` |
| `manifest` | a dependency manifest, also based on `languages` | `trivy fs`, `govulncheck`, `osv-scanner`, `grype`, `pip-audit` |
| `none` | nothing; the field remains unset | `trivy config` |

The kind is in the catalog rather than the code: a rule that checks a tool name would be
exactly the special case this information is meant to avoid. `none` is the explicit
exception: `trivy config` searches for IaC configurations, and distinguishing them
without Trivy's own detection logic would create false positives rather than classify
anything.

Counting occurs **once per reference point and kind for the entire run**, not per job: the
same tree walk over the same target produces the same result for every job of the same
kind. It skips directories that no job sees anyway: `k-playbook/` and
`k-playbook-local/results/`, both **anchored at the reference point**, plus everything
with a leading dot. The anchoring is not a detail: `installer/cmd/k-playbook/` is code in
this project, and excluding by name alone would consume it too, the same error as in Task
004, where an unanchored pattern matched the entire project root. The list is in code
([`installer/internal/review/candidates.go`](../installer/internal/review/candidates.go)),
for the same reason as the module-search list, and it is not the same: module search asks
where a project module resides, while counting asks what a tool could have seen. Therefore,
`vendor/`, `node_modules/`, and `testdata/` are included in the count.

**The number is an upper bound, not a coverage measurement.** Tool-specific exclusions
are in `args`, and every tool expresses them differently; counting does not know them.
Therefore, only this holds: candidates >= files actually checked. `0` reliably means
“nothing to do”; a high number means “there could have been something here”.

The field is absent where counting did not occur: for a `skipped` job, kind `none`, and
when the tree walk itself failed. An error there does **not** fail any job: counting is
supplementary information, not a result. A `failed` job does carry the number when its
reference point was counted, because its failure says nothing about the subject.

`k-playbook scan` reports the number only for 0 findings: there it distinguishes “nothing
to check” from “nothing checked”; otherwise it is noise:

```text
  ruff             ruff             done, 0 findings from 12 candidates → raw/ruff.sarif
```

**Writing occurs at startup**, with state `running`, and then whenever a job changes
state. If the file were written only at the end, progress would not be readable during the
run and the derived run state would jump directly from `created` to `done`.

**Repeatable.** A second invocation for the same entry overwrites its files rather than
writing alongside them or stopping. However, overwriting alone is insufficient: because
job names can depend on the module set, the entry first removes what it wrote under `raw/`
in the previous invocation, even when no job runs this time. Otherwise,
`raw/govulncheck.sarif` would remain once a second module renames the job to
`govulncheck-installer` and would continue to count as a result. The prior
`entries/<tool>.json` specifies which files these are, through the **job name**, not the
`sarif` path: `raw/<job>.sarif` is the naming rule, and the name occurs there for every
outcome. Therefore, the guarantee applies without exception, including the file of a job
that failed in the previous invocation. If the file is absent, the naming rule (the job
name starts with the tool name) is the fallback, limited to `*.sarif`; the same rule also
validates every read job name before it controls a deletion. Files of other entries remain.

### AI Entry Status

AI entries also write their own file under `entries/<name>.json`, but with a lean schema.
A perspective reports its result file:

```json
{
  "name": "secret-scanning",
  "kind": "ai",
  "state": "done",
  "result": "review-secret-scanning.md",
  "reason": "",
  "startedAt": "2026-08-19T10:00:00Z",
  "finishedAt": "2026-08-19T10:15:00Z"
}
```

`result` is relative to the run directory. When `resultRequired` is `true` in the copied
`run.json` metadata structure, `state: done` requires this result and the file must exist.
`failed` and `skipped` require a `reason`; `running` must not carry `finishedAt`.

An evidence source instead reports its artifact as `jobs`:

```json
{
  "name": "tech",
  "kind": "ai",
  "state": "done",
  "reason": "discarded 3 findings outside scope.paths: docs/review-runs.md, ...",
  "startedAt": "2026-08-19T10:00:00Z",
  "finishedAt": "2026-08-19T10:15:00Z",
  "jobs": [
    { "job": "tech", "state": "done", "sarif": "raw/tech.sarif", "findings": 17,
      "started": "…", "finished": "…" }
  ]
}
```

`result` remains empty: SARIF is the required artifact. `jobs` uses the same representation
as tool entries rather than a second one alongside it: the merge reads both by the same
path, and a separate job format would be invisible there. The job has the same name as the
entry; `findings` is the count of **accepted** findings. The field is absent when there is
no job, so perspective files retain exactly the form they had before, and files from before
`jobs` remain readable.

## The Interface

The **Reviews** page of the Workflows section (`/workflows/reviews`) lists previous runs
from `k-playbook-local/results/`, including their count next to the list. Each run shows its
state and entry count; a directory without `run.json` is identified as such.

The interface does no more here: **it does not create or start a run.** A run is created
through `/k-audit` or `/k-review` in the assistant, and scanned in the terminal with
`k-playbook scan`.

## MCP Tools

The MCP server exposes the same domain logic in machine-readable form. The CLI remains the
manual path; MCP is intended for chat orchestration without shelling out to the
`k-playbook` CLI.

| Tool | Purpose | CLI equivalent |
|---|---|---|
| `k_playbook_review_status` | read selection basis or an existing run's status | none, close to the `/workflows` section |
| `k_playbook_review_create` | create a run or generate a dry run of the `run.json` structure | none |
| `k_playbook_review_scan` | run tool entries blocking | `k-playbook scan <lauf>` |
| `k_playbook_review_merge` | write `review-input.json` and `review-input.md` | `k-playbook merge <lauf>` |
| `k_playbook_review_write_ai_entry` | write an AI entry's status | no CLI equivalent |

All tools require `projectDir`, search upward from there for `K-PLAYBOOK.yaml`, and return
structured JSON envelopes. Errors are domain tool results with `ok: false`, not MCP
protocol errors. A missing, empty, or whitespace-only `projectDir` is `invalid_input` and
nothing is executed; a `projectDir` that leads to no project is `project_not_found` (see
[mcp.md](mcp.md)). In `available` mode, `k_playbook_review_status` returns the `scan-triage`
command-module entry alongside tools and catalog recipes when it is active in the effective
command namespace. `k_playbook_review_create`, `k_playbook_review_status` for an existing
run, and `k_playbook_review_write_ai_entry` accept this entry even though it is not in
`catalogs.reviews`.

### Reporting an AI Entry Result

`k_playbook_review_write_ai_entry` accepts `run`, `entry`, `state`, and optionally
`result`, `reason`, `startedAt`, and `finishedAt`. An evidence source additionally uses
`job`:

```json
{
  "projectDir": "…",
  "run": "2026-08-19",
  "entry": "tech",
  "state": "done",
  "job": {
    "sarif": "raw/tech.sarif",
    "started": "2026-08-19T10:00:00Z",
    "finished": "2026-08-19T10:15:00Z"
  }
}
```

`job.sarif` is relative to the run directory under `raw/`; `started` and `finished` are
optional. State, finding count, and job name are created when reporting and are not
accepted as input. The job belongs to the **completion report**: it is rejected with
`state: running`, and it does not exist for `mode: perspective`.

The response must be read rather than assuming `done`. Under `evidence`, it carries the
number of accepted findings (`findings`), discarded findings (`droppedFindings`,
`droppedPaths`), and whether the SARIF was written back sanitized (`sarifRewritten`).
`stateOverridden: true` alongside `requestedState: done` means that the artifact was
invalid and `failed` was written with a reason.

Two error classes remain separate. **Invocation errors** such as a SARIF path outside
`raw/`, a missing or empty file, `result` on an evidence entry, a job at `state: running`,
or a recipe without a valid evidence contract write **nothing**; correct the invocation.
**Artifact errors** such as unreadable SARIF, an incorrect `tool.driver.name`, or an
unknown rule ID write `failed` with a reason; only another recipe run fixes them.

### Assessment Freshness

Alongside entries, the run status reports a `triage` block:

```json
{
  "triage": {
    "result": "review-triage.md",
    "state": "stale",
    "finishedAt": "2026-08-19T11:00:00Z",
    "reviewInputModified": "2026-08-19T12:30:00Z",
    "reason": "review-input.json is newer than the assessment; the merge ran afterward."
  }
}
```

`state` is `missing`, `current`, or `stale`. The block is **alongside** the state of the
`scan-triage` entry and does not replace it: the entry says whether the assessment was
written, while this block says whether it is still valid. It is needed because repair
validation for `review-triage.md` only checks existence and size. Another merge, the
normal path when evidence arrives later, would otherwise leave the entry `done` and
consistent although the assessment describes a state that no longer exists.

The modification time of `review-input.json` is compared with `finishedAt` of the
`scan-triage` entry. Equal times count as current. Cases that cannot be evidenced, where
`review-input.json` is missing or the entry has no readable finish time, count as `stale`
and never as `current`: an unsubstantiated assessment must not make a run look complete. A
run with `state: stale` is not complete; the triage step runs again.

## Consolidating with `k-playbook merge`

When a run is complete, a second step condenses its raw data into a review input that can
be curated:

```text
k-playbook merge <lauf>
```

The command reads `run.json`, `entries/*.json`, and `raw/*.sarif` from `done` jobs,
normalizes findings, groups recognizable duplicates, and writes two artifacts alongside
the raw data:

```text
k-playbook-local/results/<lauf>/
├── review-input.json    complete audit evidence, JSON
└── review-input.md      compact view, Markdown
```

**What the command explicitly does not do.** It does not touch raw files: `raw/*.sarif`,
`run.json`, and `entries/*.json` remain unchanged. Nor does it assess them: that is the
assistant's responsibility, which receives `review-input.json` as input.

**Dedupe model.** Findings are never silently removed. Five rules consolidate them:

- the same `fingerprint` or `partialFingerprint` within a comparable source, for
  dependency findings only for fingerprints on the allowlist,
- the same file, line, rule ID, and normalized message,
- **one** shared dependency ID (CVE/GHSA/OSV/PYSEC/GO) plus the same package and version,
- for non-dependency findings (`same-location-tool`): the same tool in the same job at the
  same file and line,
- for AI evidence (`ai-path-rule`): the same recipe, file, and rule ID, without a line or
  message.

The first rule applies to dependency findings by name only. A fingerprint *can* name the
finding, as SARIF provides for, but it can also hash only its location and then groups
dependency findings by location rather than identity. Both occur under the same field name:
Grype and osv-scanner both write `partialFingerprints.primaryLocationLineHash`, and in one
measured run Grype's value was identical per package, with eight of its nine values each
covering 3 to 6 different vulnerabilities, while osv-scanner's value formed a bijection to
the identifiers. Therefore, an **allowlist of tool and fingerprint-name pairs** applies
(`namingFingerprints` in `merge/dedupe.go`): for a finding with a recognized dependency,
only a listed pair forms a key; every other pair does not. Currently,
`osv-scanner` / `primaryLocationLineHash` is listed. The list is deliberately an allowlist,
not a denylist: a fingerprint name introduced only by a tool update is not on it and cannot
silently restore location grouping. The cost is maintaining it per tool. The rule remains
unchanged for findings without a recognized dependency.

The fourth rule does **not** apply to dependency findings. For manifest findings, every
finding from a tool points to the same line in the manifest file; there, the rule would
group by location rather than identity and combine consistently different vulnerabilities,
in one measured run 18 different GHSAs in one group. It remains unchanged for findings
without a recognized dependency: there, the same line says something about the finding.

The fifth rule belongs to key class `ai` and is intentional: two AI findings with the same
rule ID in the same file are one group. All instances remain as `findingIds` and `evidence`,
but the representative names only **one** location, so the count must be read too. This is
for stability: an AI group's stable key contains neither line nor message, so a moved line
or reworded text does not change the group ID. Without the fourth rule, two findings with
the same rule ID in the same file would produce two groups with the same key, and thus
colliding `stableId`s. A decision for such a group covers the entire file and rule ID; there
is no per-instance rejection.

**Which identifier forms count.** `CVE-...`, `GHSA-...`, `OSV-...`, `PYSEC-...`, and
`GO-...` are read as dependency identifiers. The latter is the Go Vulnerability Database
identifier, under which Grype exclusively records its Go findings; without it, these
findings had no identifier at all and remained without a hard key. The number must have at
least four digits, as issued by the Go database (`GO-2026-5024`): the pattern runs
case-insensitively through free text, and a shorter form would read date values such as
`go-2026-08` as an identifier and chain unrelated findings through it. For the same reason,
forms that no tool itself *issues* and which occur only as an alias in advisory text, such
as `SNYK-...`, are absent: they connect nothing that the other forms do not already connect
and introduce only the chaining risk.

For the dependency rule, every identifier counts individually: two tools join as soon as
they share an identifier, even if one names three aliases and the other only one. Only
identifiers that name the finding enter the key, `ruleId` and named alias fields, not those
that occur incidentally in advisory text. The manifest is **not** in the key: tools write
the same path differently (`requirements.txt`, `/requirements.txt`,
`file:///abs/pfad/requirements.txt`). The cost is visible in a monorepo: the same package
with the same CVE under `services/a/` and `services/b/` becomes one group. The version,
however, remains in the key: tools that provide a structured value at all write it equally.

**Where package and version come from, and where they do not.** Only values read
**structurally** may enter the hard key. Two sources count:

- a named property of the result or rule (`package`, `packageName`, `version`,
  `installedVersion`, ...) as written by pip-audit,
- a purl in `purls` or `purl`, including as a JSON array, as written by Grype
  (`pkg:pypi/requests@2.19.0`). The ecosystem portion is dropped, while the name portion
  remains complete so a Go module path such as `golang.org/x/sys` is not mangled.

If a tool names package and version only in prose, the value is read but placed in
`textPackage` / `textVersion` and **not** in the hard key. This is the case with
osv-scanner (`Package 'requests@2.19.0' is vulnerable to ...`) and Trivy
(`Package: requests` / `Installed Version: ...`). The reason is discrimination: in the key,
package and version are the only distinction between the same identifier in two different
packages (vendored libraries); an adjacent text value would merge precisely the findings
that must remain separate. Unlike identifiers, there is therefore **no** fallback here from
the narrow to the broad side: without a structured value, the hard key is disabled, and
findings from osv-scanner and Trivy stand beside one another as `possible-duplicate`.

Two cases are not consolidated but are marked mutually as `possible-duplicate`:

- same file and line with a similar rule family,
- shared dependency ID but package or version is missing or differs, or one side names no
  package at all and therefore has no hard key.

The assistant decides whether to bundle the evidence.

**Two artifacts, clear roles.** `review-input.json` carries provenance and all evidence,
so it may be large. Its fields are specified in
`commands/_review-run/review-input-contract.md`, and only there. `review-input.md` remains
deliberately compact: groups are bundled, the detailed list has a hard cap, and it refers
to the JSON once it exceeds that limit.

**When `schemaVersion` increases.** Only for **breaking** changes: a field is removed,
renamed, changes type, or the meaning of an existing field changes in a way that makes a
reader of the old version misunderstand it. A **new** field leaves it unchanged. This is
because of the number's role: it tells a reader whether it may still process the file as
before, and it may, because all fields are optional, the contract specifies for each what
applies when absent, and a reader skips what it does not know. If it increased for every
field, it would be a change counter and every consumer would have to update its check even
though nothing had changed for it.

**Changed values are not a schema change.** If `stableId`s shift, see “Stable Group IDs
Depend on Frozen Path Normalization” below, `schemaVersion` remains unchanged: the field
continues to have the same name and meaning, only its value is calculated differently.
Anyone who needs to know which version created evidence reads `kPlaybookVersion`; it is in
every merge artifact precisely for that purpose. The number is currently `1` and has not
increased since its introduction, including for the dependency fields added since then.

**Repeatable.** Another invocation overwrites both artifacts. If entry files are missing
for an entry selected in `run.json`, its state is `start`; this is not an error, but visible
information in the status block.

### Stable Group IDs Depend on Frozen Path Normalization

A path enters the key and class of a group in four places. These four places call **not**
the same normalization as grouping, but a separate, fixed copy (`stablePath` in
`merge/stable_path.go`). The grouping normalization (`pathnorm.Normalize`, shared with
`knowndecisions`) may evolve without moving the IDs.

**Why separate them.** This is exactly what went wrong once. While rebuilding the dedupe
keys, grouping normalization was improved for backslashes, `file://` including authority,
`.`/`..`, duplicate slashes, and a leading `/`. Because ID construction called the same
function, stable IDs shifted as an unintended side effect. The measurement in a run with 74
groups: **38 groups shifted, all 38 solely because of path form**, in every case due to the
removed leading `/` (`/dist/...` -> `dist/...`). Grype and osv-scanner were affected
completely, pip-audit and Trivy not at all: tools that write relative paths were unaffected.
This was a shift nobody wanted, that failed nowhere, and that silently made every
`stableId` entry in `known-decisions.md` point nowhere.

A stable ID is the reference point by which a triage decision survives a run. It may
change when the finding changes, but not when a tool writes paths differently or
normalization learns more. The cost of separation is a function maintained twice; it is
lower than an identifier that breaks with every improvement elsewhere.

**Why not restore the old normalization.** It would not have restored the IDs from before
the rebuild. A second shift occurred between then and now, populated package from purl
sources and the changed group composition, which overlays the path portion: in that run
with 74 groups, only 12 still had their old ID in the end, and reverting the path form
would restore none of them. It would restore only the inferior normalization, frozen
permanently in ID construction while grouping uses the better one. A migration table would
also have been only a point-in-time document; it becomes worthless with the next shift, and
there is no inventory that justifies it.

**What this means for `known-decisions.md`.** Entries with the `stableId` criterion must
be derived again **once**, from a run consolidated with this version. The easiest path is
to merge the affected run again and copy the group's `stableId` from `review-input.json` or
`review-input.md`.

This has occurred **twice** so far, in two deliberately bundled steps. The first bundle
combines four changes, expanded path normalization, dedupe keys per identifier, populated
package with changed group composition, and narrowing to the narrow identifier set, so one
derivation rather than four is enough. The second comes **afterward** and carries two more:
the allowlist for fingerprints on dependency findings and the `GO-...` identifier form,
which can also set a group's class and prefix through the narrow set. Anyone who has
already derived once derives a second time for the second bundle; in a measured run with 73
groups, 64 retained their old ID. Entries with `pathGlob`, `ruleId`, or `fingerprint` are
unaffected because they do not match by ID. These criteria are better suited to keeping a
decision long term.

**One additional identifier no longer shifts the ID.** The key's prefix and `dependencies`
line arise from the **narrow** identifier set, the identifiers that name the finding itself
(`ruleId` and named alias fields), rather than every identifier that occurs in advisory
text. Previously, an incidentally mentioned foreign identifier was enough to shift the ID
and could even determine the prefix: in the measured run, the PyYAML group for
CVE-2019-20477 was named `scan-cve-cve-2017-18342-...`, after another PyYAML advisory that
the description text only mentioned. If a tool names its only identifier exclusively in
prose, construction falls back to the broad set; otherwise, such a finding would lose its
prefix and class entirely.

**What *counts* as an identifier does shift the ID, however.** The statement above says an
additional identifier **outside** the narrow set has no effect. If the narrow set itself
grows because an identifier form is newly recognized, prefix and key change. That is
exactly what adding `GO-...` did: Grype's Go findings previously had no identifier at all,
their groups were `class=location` with the `scan-grype-` prefix; since then, they are
`class=dependency` with `scan-cve-go-...-`. An expansion of identifier forms is therefore
an ID-shifting change and belongs in a bundle, not slipped in incidentally.

**What decoupling does not do.** It covers path form, not group composition. Three
remaining instabilities remain and are deliberately accepted:

- The key is created from **all** findings in a group: tools, jobs, locations, rules,
  messages, fingerprints. If a finding is added because another tool reports the same
  finding or the same issue appears in a second location, the ID changes. This is the usual
  case whenever a run's tool set changes.
- If a group carries no identifier in the narrow set, ID construction falls back to the
  broad one. For such groups, the ID still depends on the complete alias list and breaks as
  soon as a tool names one additional identifier.
- Even with a narrow set, narrowing helps only when the additional identifier occurs
  **outside** it. If a tool names it in a named alias field, where pip-audit writes its
  aliases, it counts in the narrow set and the ID still shifts.

Anyone who must exclude these cases uses `pathGlob` or `ruleId` instead of `stableId` in
`known-decisions.md`.

**When this may change again.** `stablePath` is not changed incidentally. A change there
shifts every stable ID whose path it affects and invalidates the corresponding decisions.
It is a separate decision, documented here and synchronized in
`commands/_review-run/review-input-contract.md`; a test in the merge package fixes the
state and fails if someone shifts it unnoticed.

## Effect of `known-decisions.md`

`k-playbook-local/known-decisions.md` records findings that deliberately should no longer
count as findings: false positives, accepted risks, deferred items, and rejected items. It
is hand-maintained **input**, not a review result, which is why it resides directly in
`k-playbook-local/`, next to `rules/`, `reviews/`, and `guidelines/`, rather than in
`results/`. No command creates it; it is written by hand and optional.

Known decisions are applied centrally in the merge step before assessment. Matching does
not happen in the chat: `k-playbook merge` loads `known-decisions.md`, marks covered
findings in `review-input.json`, and writes the same information to the group and Markdown
views. Raw data, `run.json`, and `entries/*.json` remain unchanged.

Format per entry:

````markdown
## kd-beispiel

```yaml
id: kd-beispiel
category: wontfix
expires: 2026-11-30
owner: team
match:
  - pathGlob: _old/**
```

Rationale as prose.
````

The `##` header must match `id`. Required fields are `id`, `category`, and `match`; allowed
categories are `false-positive`, `accepted-risk`, `deferred`, and `wontfix`. `expires` is
an ISO date; expired decisions are visibly reported but not applied.

Supported match criteria:

- `stableId` for a stable group from `review-input.json`.
- `ruleId` plus `location`; `ruleId` alone is prohibited.
- `cveId`, `ghsaId`, or `osvId` plus scope through `package`, `version`, `manifestGlob`, or
  `stableId`.
- `pathGlob` for entire trees such as `_old/**`.

Paths are compared as project-relative slash paths without a leading `./`. For multiple
locations, one match is sufficient; the match report names the matched location.

**It is the same normalization used by the merge for grouping.** Both pattern and finding
location pass through it: backslashes become `/`, a `file://` prefix including hostname is
removed, `.` and `..` segments and duplicate slashes are resolved, the leading `/` is
removed, and comparison is case-insensitive. Previously, merge and matching each had their
own copy, and they had diverged: a decision then matched only part of a group that the merge
had combined through precisely these spellings, and this appeared only as partial coverage.
This one question is explicitly shared; stable-ID normalization is different and frozen
(see above).

Which criterion suits which purpose: the difference is greater than the list suggests:

- **`stableId`** covers exactly one group and is the standard path. This applies especially
  to AI evidence: an AI group's group ID depends only on recipe, rule ID, and file, so it
  remains stable across reruns even when lines move or the text changes.
- **`ruleId` plus `location`** is the useful second path. Here, `location` is a **path glob,
  not a line**: the finding's file path is compared, while line and column are excluded.
  `ruleId: tech-magic-value` with `location: installer/**` therefore covers that one rule in
  that one tree, and nothing else. This is exactly why `ruleId` without `location` is
  prohibited: by itself, it would cover the rule across the whole project.
- **`pathGlob`** is the coarse exception for entire trees that are no longer maintained. It
  matches **every** finding at that path, including scanner findings from other tools and
  rules that may not even exist there today. Writing `pathGlob: services/legacy/**` also
  suppresses the next secret finding there. This side effect is why it is not a replacement
  for the other two paths.

Location:

- `k-playbook-local/known-decisions.md`: the one project-wide file. There is no second
  search path and no run-specific version: a known decision is not bound to a run; that is
  what `expires` is for.
- If the file does not exist, the merge visibly reports “no known decisions loaded”.
- Transition: projects that still have the file under `k-playbook-local/results/` continue
  to have it read from there. The merge then reports the move as a notice in
  `review-input.md`, CLI output, and the result of the MCP tool
  `k_playbook_review_merge`. If both exist, the new location wins and the old one is
  reported as ignored. Reading the old location is temporary and will be removed.

JSON effect:

- Covered findings carry `coveredByKnownDecision` with `id`, `category`, and `matchedBy`.
- Groups carry `coveredByKnownDecision` only when all findings are subject to the same
  primary decision.
- Partially covered groups carry `partialCoverage: true` and `knownDecisionCoverage` with
  IDs, categories, and finding counts.
- The `knownDecisions` meta block names the source, loaded IDs, origin, expiration,
  `applied`, `notAppliedReason`, and loader warnings.

`review-input.md` shows the number of fully and partially covered groups, a known-decision
column in the group table, expired decisions as a separate notice, and loader warnings
under “Known-decision notices”. The assessment module `review-scan-triage` subsequently
reads this coverage exclusively from `review-input.json`.

## Assessing with `review-scan-triage`

After the merge, `/k-audit` assesses the run through the command module
`commands/_audit/review-scan-triage.md`. The module is not a review recipe and is not
loaded from `reviews/`. It resides in the `_audit/` namespace because the
`scanTriageModule` constant and `scan-triage` entry name depend on that path; it is not
audit-exclusive. In the run, it is maintained as the `scan-triage` AI entry. The
`/k-review` report mode applies the same wording but does not write an MCP entry status.

Inputs are exclusively files in the run context: `review-input.json` as evidence and
`review-input.md` as the compact view. What the evidence contains, the core,
merge-only portion, and construction of stable group IDs, is described by
`commands/_review-run/review-input-contract.md`.

The module writes exactly one new Markdown artifact directly into the run directory:

```text
k-playbook-local/results/<lauf>/review-triage.md
```

`review-triage.md` bundles groups by common cause, assigns priority `P1`/`P2`/`P3` and
category `S`/`T`/`K`/`F`/`A`/`X`, refers to stable group IDs, and names the next step per
bundle. Groups covered by `known-decisions.md` remain visible and are marked covered; the
assignment comes exclusively from `review-input.json`.

After writing, `/k-audit` sets the AI entry to `done` with
`k_playbook_review_write_ai_entry` and `result: review-triage.md`. The tool writes only
`entries/scan-triage.json`; Markdown content remains a direct artifact in the run
directory. A `done` state is consistent only when the relative result path remains in the
run directory and the file exists.
