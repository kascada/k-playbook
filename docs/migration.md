# Migration: Project-Local Installation

Working document for the duration of the migration. It records what has been discussed and
decided -- not what was merely contemplated. What has been implemented no longer appears here,
but in the regular documentation; this file is deleted when nothing remains open.

As of: 2026-08-20, branch `main`.

## What Has Already Changed

The migration to the project-local model is complete and incorporated into the documentation:

| Topic | Where it is documented |
|---|---|
| Model, anchor, directory layout, overlay rules, configuration | [`k-playbook-format.md`](./k-playbook-format.md) |
| Clone, bootstrap, the four setup steps, updating | [`installation.md`](./installation.md) |
| Core model and standard workflows | [`manual.md`](./manual.md) |
| Commands, `context` once per session | [`commands.md`](./commands.md) |
| Recipes, results, remediation policy | [`code-review.md`](./code-review.md), [`k-playbook-format.md`](./k-playbook-format.md#remediation) |
| Removed: `/k-install-security-tools`, `paths.*` | [`faq.md`](./faq.md) |
| Tool: find anchors, linking, update, direct installation, legacy artifacts, web API | [`../installer/docs/architecture.md`](../installer/docs/architecture.md) |
| Command names and review handoff: `/k-audit`, `/k-review`, `/k-task-refine`, consistent `review-triage.md` | [`commands.md`](./commands.md), [`code-review.md`](./code-review.md), [`review-runs.md`](./review-runs.md) |

## Division Of Work: Development Repository Vs. Installation

**`~/dev/k-playbook` is the development repository -- not an installation.** This is where the
skills, commands, checks, reviews and rules, the installer, and the documentation are created
and made available through Git. Work happens against the repository state; the installation
alongside it under `k-playbook/` is a separate clone and may carry a different version.

**The actual installation is different.** The reference project for testing and adapting is
`/home/kleist/dev/Aiva/kascada/`. Every change is checked there against a real, evolved
installation, not against a newly created example project.

## Reviews On Tools And SARIF

Discussed, not yet implemented. The section grows with the individual steps; what is stated
here is decided, everything else is listed under "Open" at the end.

**The problem.** Today, every scan review starts its own tools -- described in prose in the
recipe and run by the assistant. `review-dependency-cve`, `review-iac-container`, and
`review-secret-scanning` each carry their own preflight, invocation lines, and findings format.
The same orchestration is duplicated, and the formats diverge: field names switch between
`Quelle`, `Tool(s)`, `Package`, and `Target`, so every downstream reader must guess the union
of all variants.

**The target model: the tool starts tools, the assistant assesses.** All tools produce SARIF and
write it to a shared result directory. A merge step combines the files and consolidates
duplicates. Only then does the assistant follow -- once over the consolidated result, not once
per tool. It assesses false positives, prioritizes, explains context, and proposes fixes.

The raw data remains untouched, as it already does under `raw/`. Only this keeps traceable what
a tool actually reported and permits later reassessment.

The merge provides more than less duplicated work: a finding independently reported by two
scanners carries more weight than one from a single source. This information arises only through
merging -- while every review runs on its own, no one sees it.

**Operation: guided in the interface.** The flow is a sequence of steps, each of which unlocks
only when the previous one is in place:

1. Select the project's languages.
2. Select the tools, prefiltered by language.
3. Start. The tools run in parallel, each writing its SARIF file.
4. Trigger the assistant reviews. The interface cannot do this itself -- it does not start an
   assistant session. It therefore displays commands to copy; reviews obtain their settings from
   the `context` output, not from a separate source.
5. Consolidate. This step is repeatable: if reviews run only afterward, invoke it again and
   their results are included.

**The tool list.** The existing `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, and
`grype` are joined by the following for Python and Go:

| Tool | Language | Purpose |
|---|---|---|
| `ruff` | Python | quality, plus the `S` rule set (flake8-bandit); replaces bandit |
| `semgrep` | Python, Go | generic security rules |
| `gosec` | Go | Go security |
| `golangci-lint` | Go | Go quality, combines staticcheck and errcheck |
| `govulncheck` | Go | Go CVEs with reachability |
| `osv-scanner` | Python, Go | dependency CVEs |

All seven can produce SARIF themselves. Of the existing tools, `gitleaks`, `trivy`, and `grype`
can do so; `trufflehog` and `pip-audit` cannot. `syft` produces an SBOM and therefore no
findings at all -- it supplies `grype` and remains outside the merge.

**Completed.**

- The tool matrix carries `languages`, `install_method`, `install_ref`, and `asset_pattern`.
  `go install` remains reserved for tools that need Go anyway -- a purely Python project
  therefore needs no Go.
- Languages are under `project.languages`, in the `context` output, and as a selection in the
  Security Tools block. The preselection is `python`.
- The run model is in [`review-runs.md`](./review-runs.md): one run per day, `run.json` plus
  `entries/`, tools and reviews as entries in the same run. The interface creates runs but does
  not start anything.
- Execution by the tool is also documented there: `scripts/scanners.tsv` contains each job's
  invocation including exclusions, `k-playbook scan <run> [entry ...]` runs tool entries in
  parallel, writes SARIF to `raw/`, and writes progress to `entries/<name>.json`. The entry
  remains the tool; someone assembling a run does not need to know that one consists of multiple
  jobs.
- Module-bound jobs resolve their own directory: `workdir: module` in `scanners.tsv` makes the
  runner search for manifests under the target and start the job once per module. This lets
  `govulncheck`, `golangci-lint`, and `gosec` run even in projects whose `go.mod` is not at the
  root. `gosec` was initially set to `target` -- measurement showed it then checked nothing
  (0 rather than 154 findings), and it has since also been set to `module`; its exclusions for
  `k-playbook/` were removed without replacement because module search skips those directories
  anyway.

**Measured; belongs in scan jobs.** Compared at `~/dev/Aiva/kascada` (351 files, 59,000 lines):

- **95.3% of all Python findings are `S101`/`B101`** -- simple `assert`. A job that does not
  exclude this buries the other 170 findings beneath 3500. The same applies to test directories:
  they contain most `S105` hits (`{"secret": "read-secret"}`).
- **`ruff` requires `--select S --isolated`.** Its default selection is `E`/`F`; without this
  it finds not a single security finding, and with the project configuration it would find
  something other than intended.
- **`bandit` was removed**, because ruff covers it: 97.7% identical findings, and the difference
  was mostly false positives and cases already dismissed with `# noqa`, which bandit does not
  see.
- **`semgrep --config auto` is excluded, not merely undesirable.** The combination with disabled
  metrics is refused by the tool itself: "Cannot create auto config when metrics are off. Please
  allow metrics or run with a specific config." Either send usage data or use a named rule set --
  there is nothing in between, and the former is out of the question for a tool that runs in
  other projects. (`~/.semgrep/settings.yml` carries an `anonymous_user_id`.)

  Measured with Semgrep OSS 1.172.0 on one Python and one Go file each:
  `--config p/security-audit --metrics=off` runs cleanly, **one job covers both languages** --
  79 rules with Python only, 107 with Python and Go, because semgrep selects based on the file
  types found. Separate language jobs are not needed.

  The rules come from the server on **every** run; `~/.semgrep/` keeps no rule cache. The job
  therefore needs a network connection, and the rule set can change between two runs -- more
  slowly than with `auto`, but not ruled out. A custom rule set from local YAML files would be
  the solution, but would mean maintaining it; against that is that the SARIF fully documents
  the rules run: 225 entries under `tool.driver.rules` with ID, description, and `helpUri`, plus
  the tool version. What applied in a run therefore remains readable from `raw/` -- the purpose
  for which the directory is auditable.

**Completed: merge tool.** From Task 014 (`k-playbook-local/tasks/done/014-sarif-merge-review-input.md`).
A dedicated subcommand instead of an external runtime: `k-playbook merge <run>` reads `run.json`,
`entries/*.json`, and `raw/*.sarif`, normalizes findings, deduplicates them, and writes two
artifacts: `review-input.json` as complete audit evidence and `review-input.md` as a compact
view. Details are in [`review-runs.md`](./review-runs.md#consolidating-with-k-playbook-merge).
External candidates were excluded: Microsoft SARIF Multitool would pull in .NET or npm, and
`sarif-tools` (PyPI) does not cover cross-tool deduplication and `entries` context.

First real run (2026-08-19, this repository): 347 findings, 227 groups. Raw data remained
untouched. Second real run on the same day at the OMNI project
(`~/dev/squad-km-dev-setup/k-playbook-local/results/2026-08-19/`): 327 findings, 141 groups --
the deduplication effect is significantly stronger there because lockfile lines bundle many CVEs.

**Completed: merge improvements from the real run.** From Task 016
(`k-playbook-local/tasks/done/016-merge-nachbesserungen.md`). Six points that came from the
first two real runs have been cleanly incorporated into the merge:

- Same-location bundle within the same entry/job/tool pulls multiple rule IDs at one location
  into one group without losing evidence.
- Central rule-to-severity mapping in `scripts/severity.tsv`; native and CVSS values take
  precedence, and the mapping covers the rest. The OMNI run shows it clearly: the 167 findings
  previously reported as `unknown` are now assigned a normal severity.
- `entries[].source` no longer duplicates entry data; traceability is through entry/job ID, tool
  name, and raw-data path.
- `kPlaybookVersion` comes from `runtime/debug.ReadBuildInfo` (release version, development
  version, dirty suffix) instead of being hard-coded as `"unknown"`.
- The Markdown number block also includes `done` tools with 0 findings.
- Stable group IDs (`stableId`/`stableKey`) with a descriptive prefix (`scan-<tool>-...` or
  `scan-cve-<id>-...`) and deterministic collision resolution. Two consecutive merges of the
  same run produce the same set of IDs.

Real run after 016: k-playbook repository 347/173, OMNI 327/110. Fanout locations from the
`_old/` tree and `requirements.txt:29` are visibly bundled; `unknown` disappears. Raw data in
both runs was `sha256`-identical before and after the merge.

**Completed: soft skip from the catalog.** From Task 015
(`k-playbook-local/tasks/done/015-scanner-soft-skip.md`). It was triggered by `osv-scanner` in
the OMNI run on 2026-08-19: exit 128 plus "No package sources found", without SARIF -- treated
by the runner as a technical `failed` even though the tool itself reported that there was nothing
to check under the reference point. Rather than writing a special case for every scanner into the
runner, the signal is now in the catalog: a `soft_skip` column in `scripts/scanners.tsv` carries
rules in the form `<exit code>:<regex>`, multiple separated by `;`. If process exit code and the
pattern in stderr or stdout match, the runner executes the job as `skipped` with the matching
line as the reason; `failed` remains reserved for technical errors. Precedence remains clear:
readable SARIF wins and remains `done`, broken non-empty SARIF remains `failed`, as do timeouts
and runner cancellation. Details are in [`review-runs.md`](./review-runs.md#states) and the
header line of [`scripts/scanners.tsv`](../scripts/scanners.tsv).

**Completed: alternative 2, command orchestrated through MCP.** From Task 018
(`k-playbook-local/tasks/done/018-review-run-und-triage.md`). `/k-audit` is now enabled: the
command creates or continues runs through MCP, reads status before every step, starts scanners,
runs AI review entries, starts the merge, and then invokes the `review-scan-triage` assessment
module. The selection basis comes from `k_playbook_review_status` in `available` mode: tools,
active review recipes, and the `scan-triage` command-module entry are confirmed before `create`.

**Completed: MCP tools for review runs.** From Task 017
(`k-playbook-local/tasks/done/017-mcp-review-werkzeuge.md`). The MCP server provides five tools
with a consistent response/error envelope and mandatory `projectDir`:
`k_playbook_review_status`, `_create`, `_scan`, `_merge`, and `_write_ai_entry`. They call the
existing domain logic under `installer/internal/review/*` and `installer/internal/review/merge/*`
directly; there is no shelling out to the CLI. Details are in [`mcp.md`](./mcp.md).

**Completed: assessment as a command module instead of a catalog recipe.** From Task 018. The
assessment of a run is under `commands/_audit/review-scan-triage.md`, reads `review-input.json`
and `review-input.md`, considers `known-decisions.md` through a fixed search path, and writes
`review-triage.md` directly into the run directory. The `scan-triage` AI entry is managed through
the MCP contract even though it is not in `catalogs.reviews`; an empty local overlay of the module
disables it.

**Addendum from Task 032: the evidence contract is a dedicated module.** The schema of
`review-input.json` is now in `commands/_review-run/review-input-contract.md` -- in the family
namespace of `/k-audit` and `/k-review` -- and nowhere else; commands, recipes, rules, and this
documentation refer to it. The contract has two parts: a core owed by both paths, and a
merge-only part filled only by the audit path because `/k-review` does not manage MCP tools and
can never invoke the merge. In the `result-family` branch, `/k-review` report mode applies this
module and `commands/_audit/review-scan-triage.md` faithfully rather than paraphrasing both.
Three fields that existed only in prose -- top-level `scope`, `ungroupedFindings`, and
`groups[].id` -- were removed; the merge code did not change as a result.

**Completed from Task 033: one result path instead of two.** The downstream command that
summarized the triage of multiple families again at a higher level into a project-wide summary
was removed. What it actually did is now done by others: the merge deduplicates across tools,
the merge also provides coverage from `known-decisions.md`, and the assessment module assigns
`P1`-`P3` priority. The comparison against existing tasks -- the only thing that existed only
there -- now sits in `/k-remediation`, where tasks are created: before creation, using source plus
bundle/group ID as the criterion; a hit in `tasks/done/` is reported but does not close the
finding. Therefore: `review-triage.md` goes directly to `/k-remediation`, and merging happens
exclusively in the audit run.

Also decided: **every report recipe requires a `result-family`.** The branch without a family
previously wrote `summary-YYYY-MM-DD.md`; it is gone, and `review-tech` has received the `tech`
family. Nothing creates summaries anymore -- `/k-remediation` still reads existing ones as legacy
input.

Two recipes remain outside the audit run, `dependabot-alerts` and `k-check-security`. Both were
seriously examined for `audit.mode: evidence`; why this fails is documented in the respective
recipe under **Position in the Audit Run Model** and requires one task each. Until then, their
`review-triage.md` goes directly to `/k-remediation` -- without cross-family consolidation and
without deduplication against other sources. The same applies to every independent `/k-review`
run of a family recipe. This is the deliberate consequence of the migration: `/k-remediation`
accepts exactly one result file and is not extended for multiple sources.

**What the removal means for already configured projects.** In a target project that updates the
clone, it means three things:

- Existing `summary-*.md` under `k-playbook-local/results/` remain. Nothing creates them
  anymore; `/k-remediation` continues to read them as legacy input. Anyone who no longer needs
  them deletes them manually -- `results/` is not versioned anyway.
- With the update, the command disappears from the catalog, and the symlinks under
  `.claude/commands/`, `.opencode/commands/`, and `.cursor/commands/` are then orphaned.
  Setup in the interface removes them itself. The order cannot be reversed: first update the
  clone, then set up. In reverse, setup still resolves the catalog from the old state and creates
  the links again.
- **A project-local overlay otherwise remains alive.** A same-named file under
  `k-playbook-local/commands/` previously overlaid the shipped command. If the shipped one is
  removed, it no longer overlays anything, but becomes an independent project-owned command and
  remains registered. It must be deleted in the target project.

**Completed: `known-decisions.md` applies project-wide.** From Task 019
(`k-playbook-local/tasks/done/019-known-decisions.md`). The format is fixed: one `##` entry per
decision, exactly one fenced `yaml` block with required fields, followed by rationale.
`k-playbook merge` reads the file, marks covered findings and groups in `review-input.json`, and
shows complete or partial coverage in `review-input.md`. The assessment module no longer matches
on its own, but adopts `knownDecisions` and `coveredByKnownDecision` from JSON. The real run of
2026-08-19 covers the 74 `_old/` groups through `kd-old-tree`; `raw/`, `run.json`, and
`entries/*.json` remained unchanged according to SHA256.

**Addendum from Task 030: the two-level rule has been withdrawn.** Task 019 had specified two
search paths -- a run-specific `RUN_DIR/known-decisions.md` before the project-wide file, with
the run-specific version winning for the same `id`. Both were removed. The run-specific version
had no creator: no command, skill, or interface element created it, and it had not occurred in
any run of this repository. It also does not hold up conceptually -- a deliberate decision is
not run-bound, `expires` exists for that -- and it had a silent side effect: a file nobody
expects could fully override a project-wide decision.

At the same time, the project-wide file moved one level up from `k-playbook-local/results/` to
`k-playbook-local/known-decisions.md`. `results/` is "everything reviews create"; this file is
maintained by hand and is input, not output. It deliberately remains **not** an entry in
`LocalStructure()`: `CreateLocal()` creates file entries with `writeIfMissing`, so the new
location would always exist afterward -- and transitional reading of the old location would never
run. `/k-gui` therefore does not create it; its purpose is in
[`review-runs.md`](./review-runs.md#effect-of-known-decisionsmd). The old location continues to
be read until 2027-02-28 and the move is reported visibly; removal is a project todo
(`/k-todo`), triggered by the comment on `legacyResultsDirName`.

**Completed: todos move from `TODO.md` to `data/todos.json`.** From Task 053. The project todos
used to live in `k-playbook-local/TODO.md`, a Markdown checklist that the `/k-todo` command wrote
by hand. They now live in `k-playbook-local/data/todos.json`, a document owned by Go: it carries
an `id` per entry that is never reused, a `created` and a `done` date, an optional `origin`, and
`nextId` at the document level.

The translation happens once, on the first access through any of the three layers -- the
interface, `k-playbook todo`, or the MCP tools `k_playbook_todo_*`. All three go through the same
function in `internal/project`, writing accesses included: `add` in a project that still has a
`TODO.md` migrates first and never creates a second, empty document beside it. Text, order, and
done state are preserved. Timestamps are not invented: `created` stays empty, because the
Markdown file never said when an entry was written, and a done entry receives the migration date
with `doneMigrated` marking it as exactly that, not as the day someone ticked it off. The
`TODO.md` is removed afterward.

If both files exist, access does not fail: the JSON document is read and written, and the
leftover `TODO.md` is reported as a hint -- `hint` in the web response, a warning in the CLI and
MCP envelopes, never `message`. Only the migration itself refuses in that case, and its message
names the way out: `k-playbook todo import k-playbook-local/TODO.md` appends the Markdown entries
to the existing document and removes the file.

Two directories arrived with it: `k-playbook-local/data/` for machine files that belong to the
project's state, versioned and without a `.gitignore`, and `k-playbook-local/cache/` for derived
content that can be rebuilt at any time, private by default like `results/`.

**Completed: namespace convention for command modules.** Decided together with Task 018 and
anchored in `rules/command-authoring.md`: `commands/_<name>/` contains modules (not a command).
`_shared/` remains for modules shared by all commands; `_<command-name>/` collects
command-specific modules (`_audit/`), `_<family>/` collects modules of a command family (for
example `_docs/`, once the Docs commands receive shared modules). Overlay works by file path from
`commands/`; an empty file disables it. Rule in
[`../rules/command-authoring.md`](../rules/command-authoring.md#ablage).

**First real triage run.** 2026-08-19, this repository, `review-triage.md` under
`k-playbook-local/results/2026-08-19/`. 173 groups were condensed into six bundles plus one
remaining group: B1 Dependency Upgrade Go (P1/S), B2 Path Validation (P2/S), B3 Process Calls
(P2/K), B4 File Permissions and Ignored Cleanup Errors (P2/T), B5 Remove `_old/` from Scope
(P3/X), B6 Staticcheck Cleanup (P3/T). The bundle cut demonstrates the benefit of the whole
pipeline for the first time: 347 SARIF findings become six manageable assessment units, each with
a concrete next step.

## Next

As of 2026-08-20. These points have been discussed, are fixed in the current result, and are
addressed in this order:

1. **`_old/` cleanup.** Bundle B5 makes it visible: 74 groups from `_old/internal/*` are
   archived legacy code. Once `known-decisions.md` carries their coverage, we decide with a calm
   overview: delete, move, or exclude. Then perform a new merge and triage for verification.
2. **End-to-end test of the `/k-audit` flow.** The command is enabled, MCP tools are in place,
   the module and merge are complete, and `known-decisions.md` applies. What is still missing is
   the complete run in a chat session -- new run, confirm selection, scan, merge, triage, read
   `review-triage.md`. The first target projects are this repository and OMNI. After the test run
   we record what stands out in the command, module, and selection.
3. **Handoff after triage.** What happens to `review-triage.md` after assessment?
   `/k-remediation` understands `review-triage.md` as the current format;
   historical `assessment.md`/`findings.md` remain legacy fallback. Only the concrete quality of
   task creation from bundles remains open in the end-to-end test.

**Small cleanup item: installation sync.** The last two merges required `chmod u+w` on the
installation clone so that `scripts/severity.tsv` (from Task 016) was available. As soon as the
current state is pushed to the remote and installations are updated through the regular `git pull`
path, the workaround is no longer needed. Nevertheless, we will check whether there is a more
convenient way to synchronize the installation clone with the development state -- today that is
one manual step, tomorrow it is a recurring detail.

## Open, Without A Specific Date

Discuss individually before work begins:

- **OpenCode usage as an MCP tool.** OpenCode already stores session usage locally in SQLite:
  `~/.local/share/opencode/opencode.db`. The `session` table carries aggregated values per
  session: `cost`, `tokens_input`, `tokens_output`, `tokens_reasoning`, `tokens_cache_read`,
  `tokens_cache_write`, plus `model`, `agent`, `directory`, `title`, `time_created`, and
  `time_updated`. Finer values per assistant response are additionally stored as JSON in
  `event.data`, especially at `message.updated.1`; `session.updated.1` contains continuing
  session snapshots with the same totals.

  For a manual query, OpenCode's own DB command is sufficient, for example:

  ```bash
  opencode db "select id,title,directory,agent,model,cost,tokens_input,tokens_output,tokens_reasoning,tokens_cache_read,tokens_cache_write,time_updated from session order by time_updated desc limit 10" --format json
  ```

  An MCP tool for this must not expose the entire database. The same DB and its adjacent data
  directory also contain sensitive data such as accounts, credentials, `auth.json`, and
  `mcp-auth.json`. The secure scope would therefore be: open read-only (`mode=ro`), permit only
  allowlisted queries, and read only `session` plus optionally `event`. Possible tools:
  `opencode_usage_recent_sessions(limit, directory?)`, `opencode_usage_session(session_id)`,
  `opencode_usage_daily_totals(days?)`, and `opencode_usage_message_events(session_id)`.
  Responses return usage metadata only, never prompts, tool output, or credential tables.

  The core query would be:

  ```sql
  select
    id,
    title,
    directory,
    agent,
    json_extract(model, '$.providerID') as provider,
    json_extract(model, '$.id') as model,
    json_extract(model, '$.variant') as variant,
    cost,
    tokens_input,
    tokens_output,
    tokens_reasoning,
    tokens_cache_read,
    tokens_cache_write,
    datetime(time_created / 1000, 'unixepoch', 'localtime') as created_at,
    datetime(time_updated / 1000, 'unixepoch', 'localtime') as updated_at
  from session
  order by time_updated desc
  limit ?;
  ```

  Claude Code has a similar usage signal, but with a different shape. In non-interactive runs
  (`claude --print --output-format json` or `stream-json`), the final result output or SDK
  `ResultMessage` provides aggregated `usage` and `total_cost_usd`. The usage fields follow the
  Anthropic schema: `input_tokens`, `output_tokens`, `cache_read_input_tokens`, and
  `cache_creation_input_tokens`. Hooks additionally receive `session_id` and `transcript_path`;
  the transcript is JSONL and can be read as evidence. Locally, however, there is no documented
  OpenCode-like SQLite schema with session totals visible; on this machine
  `~/.claude/sessions/` is empty and the CLI is not logged in to the shell. A hook or wrapper
  export would therefore make more sense for Claude Code: store result JSON from `--print`, or
  capture `transcript_path` through hooks and extract usage metadata only. An MCP tool there must
  likewise not indiscriminately return transcripts because they can contain prompts, responses,
  and tool results.
- **Alternative 1: GUI starts only tool scans.** After creating a run, the interface starts the
  tool entries in the background, technically `k-playbook scan <run>`. The merge does not run
  automatically afterward. Once tool scans complete, the interface checks the run: if `ai`
  entries still have `start`, it shows that they must be run by an assistant and names the
  intended next steps. If no AI entries remain open, or existing tool results should be condensed
  anyway, the interface offers a button for `k-playbook merge <run>`. After the merge it shows
  the written artifacts (`review-input.json`, `review-input.md`) and the next step for assessment
  by the assistant. While a scan is running, the GUI server must not exit because a browser window
  was lost. Progress is read from `entries/*.json`; additionally, the server writes coarse status
  lines about the run, tool/job states, and completion to stdout.
- What happens to `trufflehog` and `pip-audit`, which cannot produce SARIF: convert or replace.
- **`k-check` as an MCP tool.** Today the runner emits terminal text;
  `review-k-check-security` saves it as `raw/k-check-<mode>.txt`, and the assistant reads and
  interprets it. Its parameters -- `--mode changed|baseline`, `--files-from`, `--base-ref`,
  `--exclude`, `--timeout` -- are prose in the recipe. A tool could carry them as a schema and,
  rather than returning raw output, state which checks ran and which failed with file and line.
  `--metadata-output` already writes JSON; the structure therefore exists, it just does not reach
  the assistant. Raw output remains in `raw/` -- it is auditable and not replaced.
- **The changed state as an MCP tool.** What counts and what does not in an assessment is today
  prose in `commands/k-task-run.md` ("omit generated files, lockfiles, and binary files"; for
  more than ~100 lines, summarize) and is therefore decided anew on every run. The mechanical
  part is: reference point through `git merge-base`, files and lines through `git diff --numstat`,
  Git itself reports binary, `linguist-generated` and `-diff` are in `.gitattributes`, and a
  maintained name list recognizes lockfiles. This belongs in the program -- by the same principle
  as local settings: measured, not guessed. Assessment remains with the assistant: which hunks
  count, what is summarized, whether a file marked as generated is nevertheless interesting.
  **The tool must not provide the diff text** -- the assistant retrieves it selectively using the
  path list. Otherwise the tool would need size limits and truncation rules, which are judgments
  again. The response therefore carries no content, remains small even with 200 changed files,
  and cannot silently swallow anything.
- **An empty result cannot be distinguished from no result.** A job's outcome depends on whether
  readable SARIF is available -- deliberately so, because almost all scanners end with a non-zero
  code as soon as they find something. But a tool that could not check anything writes the same
  file as one that found nothing: exit 0, valid SARIF, empty `results`. Both are `done`.

  It occurred twice, in two languages: `gitleaks` would silently have reported 0 findings in a
  project named `k-playbook` because the exclusion pattern without an anchor matched the entire
  project root (found while verifying in Task 004 and fixed there). `gosec` reported 0 rather
  than 154 from the project root because it lacked module context (Task 010). Both times it was
  noticed only because someone measured.

  **The obvious approach is blocked.** SARIF has `runs[].artifacts` and `invocations` fields for
  "what did I touch"; measured against the files of this repository, no tool fills them:
  `gosec`, `ruff`, `gitleaks`, and `golangci-lint` leave both empty, while `semgrep` writes an
  `invocations` with nothing but `executionSuccessful: true`. It therefore cannot be read from
  SARIF itself.

  What remains is on the input side: before starting, the runner knows what is there -- it knows
  the target and the run's language selection. "0 findings among 40 Go files" is different from
  "0 among 0", and this distinction requires no tool-specific rule.

  **Decided: information beside the state.** A job may legitimately find nothing; `failed` would
  be wrong for that, and a fourth state would merely shift the judgment. Every job therefore
  carries `candidates` -- the number of files that were candidates under its reference point.
  What counts as a candidate is specified by the identically named column in
  `scripts/scanners.tsv` (`source`, `any`, `manifest`, `none`), so code does not need one special
  case per tool name. The number is an upper bound, not a coverage measurement; it is assessed in
  the assessment step. Details are in [`review-runs.md`](./review-runs.md), "`candidates` -- what
  the job could have checked".

  **Additional case: the tool itself explains that there is nothing to check.** `candidates`
  applies when the runner can count what exists before invocation. But a tool can also **itself**
  decide that there is nothing to check under the reference point -- when it sees candidates but
  does not accept them as subjects. On 2026-08-19 in the OMNI run, `osv-scanner` found a candidate
  manifest (`candidates: 1`), but did not itself accept it and ended with exit 128 "No package
  sources found" and empty SARIF. Candidate counting alone was insufficient: it said something
  existed; whether the scanner treated it as a source, it did not know. Therefore the information
  is now beside `candidates` in the catalog: the `soft_skip` column in `scripts/scanners.tsv`
  permits combinations of exit code and regex per row under which the job is `skipped` with a
  reason. See "Completed: soft skip from the catalog" above for details.
