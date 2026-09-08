# Reviews and Results

This page describes the artifact model: which files a review creates, where they reside, and which status they carry. The command flow is described in [`code-review.md`](./code-review.md).

## Core Model

k-playbook separates three steps:

1. **Run a review or audit**: `/k-review <name>` assesses one family specifically; `/k-audit` performs a complete sweep.
2. **Store results**: per run or per family and date under `k-playbook-local/results/`.
3. **Work through them**: `/k-remediation` processes the assessed bundles from exactly one result file.

Consolidation happens at **one** point: in the audit run. The merge deduplicates across tools, records coverage from `known-decisions.md`, and the triage assigns priority and category. No downstream step repeats the same work at a higher level.

`/k-remediation` therefore does not aggregate on its own and accepts exactly one result file. Before implementation, it groups the findings in that file into bundles by risk, effort, quick-win potential, and shared verification, and compares them with existing tasks before creating tasks.

## Directories

Recipes and results are strictly separate:

```text
k-playbook/reviews/                    shipped recipes
k-playbook-local/reviews/              project-owned recipes, overlay
k-playbook-local/known-decisions.md    deliberately made decisions, maintained by hand
k-playbook-local/results/              everything reviews create
```

`reviews/` contains only `review-<name>.md`. This keeps it a pure overlay directory, where every file follows the same rule: same filename, the local file completely wins.

`known-decisions.md` deliberately sits one level higher, next to `rules/` and `guidelines/`: it is maintained by hand and created by no review, so it is input rather than output. Everything generated is next to it:

```text
k-playbook-local/results/
├── log.md                        when each review ran
├── YYYY-MM-DD/                   an audit run
└── <family>/YYYY-MM-DD/
    ├── review-input.json
    ├── review-triage.md
    ├── run-metadata.json
    └── raw/
```

Example:

```text
k-playbook-local/results/k-check/2026-07-24/
├── review-input.json
├── review-triage.md
├── run-metadata.json
└── raw/
    └── k-check-baseline.txt
```

`k-playbook-local/checks/` remains reserved for executable checks. Results never belong there.

### `results/` Is Not Versioned

The **entire** contents of `results/` are local, not only raw scanner output: `raw/` and `entries/`, generated documents `review-input.md`, `review-input.json`, `run.json`, and `review-triage.md`, review documents per family, plus `log.md` and, where they still exist, old `summary-YYYY-MM-DD.md` files.

The reason is the same for all of them: a review is repeatable from the code. Its result is a state from one machine at one point in time, not project knowledge. `log.md` is also personal: when someone scanned on their machine is not the project's concern. And raw output from a secret scanner contains discovered secrets in clear text; it must never enter the repository.

Whatever result is genuinely project knowledge is moved out anyway: to `k-playbook-local/known-decisions.md`, which is precisely why it sits one level higher, and to tasks created by remediation. The cost is accepted deliberately: AI assessments can no longer be read in the repository.

Because this scope is homogeneous, the usual managed ignore content (`*`, `!.gitignore`, `!README.md`) remains sufficient. `results/` is therefore the only directory that k-playbook creates privately during setup; like `priv/` and `material/`, it remains switchable through the **Local settings** section of the interface. Nothing changes automatically for existing projects: the managed `.gitignore` is created only when the directory is first created.

## Artifacts Per Family

Every new report/scan family creates these files:

- `review-input.json`: the evidence contract. Its schema is specified in exactly one place: `commands/_review-run/review-input-contract.md`. That document also describes which fields only the merge fills and what applies when they are absent.
- `review-triage.md`: a consistent final artifact with header, bundle table, bundle details, unbundled findings, and coverage from known decisions.
- `raw/`: auditable original output, for example SARIF, JSON, or tool logs.
- `run-metadata.json` or equivalent: auditable run metadata.

Raw artifacts and run metadata are auditable. They must not be shortened, overwritten, or corrected in content after writing. Corrections are made through new raw files plus an updated assessment.

`review-triage.md` is curated. It may be updated traceably, for example with a `## Remediation status` section, but the original raw evidence remains unchanged. `assessment.md` and `findings.md` are legacy artifacts of older result families and are read only when no `review-triage.md` exists.

For runs in the new run model (`k-playbook-local/results/YYYY-MM-DD/`), a second artifact pair is added: `review-input.json` and `review-input.md` from `k-playbook merge`. They consolidate `raw/` and `entries/` and serve as input for assessment by the assistant. Details are in [`review-runs.md`](./review-runs.md#consolidating-with-k-playbook-merge).

The curated final product of both assessment paths is `review-triage.md`: directly under `k-playbook-local/results/YYYY-MM-DD/` for audits, and under `k-playbook-local/results/<family>/YYYY-MM-DD/` for targeted report reviews. Only the scope differs. Active audit catalog recipes contribute differently depending on `audit.mode`. A **perspective** (`mode: perspective`) additionally writes one perspective file directly into the run directory, for example `review-secret-scanning.md`; it reads the same merge evidence, filters through its stored `scope.tools`, and serves only as context for `scan-triage`. An **evidence source** (`mode: evidence`) writes no Markdown at all; it runs before the merge and stores `raw/<entry>.sarif`, whose findings subsequently appear as groups with the `ai-<entry>-` prefix in `review-input.json` and thus follow the same path as scanner findings. `/k-remediation` works against `review-triage.md`; legacy `assessment.md`/`findings.md` remain fallback only for family directories without `review-triage.md`.

## Status Model

Status values in legacy `findings.md`:

| Status | Meaning | Relevant to remediation |
|---|---|---|
| `open` | new or not yet checked | yes |
| `confirmed` | validated real finding | yes |
| `context-needed` | further context review required | yes |
| `likely-false-positive` | plausible false positive | only after explicit selection |
| `accepted` | deliberate decision or accepted residual risk | no |
| `fixed` | fixed and verified | no |

Finding IDs must remain stable. IDs assigned once must not be renamed during re-runs, status changes, or remediation.

Deliberate project-wide decisions are recorded in `k-playbook-local/known-decisions.md` and written by `k-playbook merge` as coverage on findings and groups. The format, location, and flow rule are described in [`review-runs.md`](./review-runs.md#effect-of-known-decisionsmd). A decision neither replaces a status value nor filters anything from raw data; it only makes visible that a finding is covered by a documented decision.

Schema for k-check:

```text
kcheck-<area>-NNN
```

Examples: `kcheck-logging-003`, `kcheck-secrets-001`, `kcheck-user-scope-014`.

Scanner families may retain their tool's native rule prefixes so a finding remains traceable to its raw report.

## Scanner Tools vs. k-check

Security scanners such as `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, and `grype` are **not** modeled as checks under `checks/*.sh`.

Reason:

- They create their own structured raw data such as JSON, SARIF-like reports, or SBOMs.
- Non-zero exit codes often mean substantive findings, not technical errors.
- Results must be deduplicated, prioritized, and assessed.
- Raw artifacts must permanently reside under `k-playbook-local/results/<family>/YYYY-MM-DD/raw/`.
- Remediation needs stable finding IDs, status values, and source evidence.

These tools therefore run through report-mode reviews:

| Tool | Review | Result family |
|---|---|---|
| `gitleaks` | `/k-review secret-scanning` | `secret-scanning` |
| `trufflehog` | `/k-review secret-scanning` | `secret-scanning` |
| `pip-audit` | `/k-review dependency-cve` | `dependency-cve` |
| `trivy` | `/k-review dependency-cve` and `/k-review iac-container` | `dependency-cve` / `iac-container` |
| `syft` | `/k-review iac-container` | `iac-container` |
| `grype` | `/k-review dependency-cve` or `/k-review iac-container` | `dependency-cve` / `iac-container` |
| GitHub Dependabot Alerts | `/k-review dependabot-alerts` | `dependabot-alerts` |

`checks/*.sh` remains reserved for fast, generic k-check heuristics and preflight-like checks. Small tool-availability checks may reside there, but not the actual scanner run with persistent assessment.

## The Families in Detail

### k-check

| | |
|---|---|
| Recipe | `k-playbook/reviews/review-k-check-security.md` |
| Runner | `k-playbook/bin/k-check` |
| Results | `k-playbook-local/results/k-check/YYYY-MM-DD/` |

Typical auditable run:

```bash
k-playbook/bin/k-check \
  --mode baseline \
  --output k-playbook-local/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt \
  --metadata-output k-playbook-local/results/k-check/YYYY-MM-DD/run-metadata.json
```

`--output` receives stdout/stderr and additionally writes the complete raw stream. `--metadata-output` writes the command, exit code, timestamp, roots, mode, check configuration, and version or Git commit where available.

Existing target files are not overwritten. Use unique names for repeat runs on the same day, for example `k-check-baseline-e2e.txt` and `run-metadata-e2e.json`.

### Secret Scanning

| | |
|---|---|
| Recipe | `k-playbook/reviews/review-secret-scanning.md` |
| Results | `k-playbook-local/results/secret-scanning/YYYY-MM-DD/` |
| Audit perspective | `k-playbook-local/results/YYYY-MM-DD/review-secret-scanning.md` |
| Audit scope | `gitleaks`, `trufflehog` |

Typical artifacts: `review-input.json`, `review-triage.md`, `raw/gitleaks-*.json`, `raw/trufflehog.json`.

The tools are provided locally on the host by `k-playbook/scripts/install-security-tools.sh`. Missing tools are not installed into the project.

### Dependency CVE

| | |
|---|---|
| Recipe | `k-playbook/reviews/review-dependency-cve.md` |
| Results | `k-playbook-local/results/dependency-cve/YYYY-MM-DD/` |
| Audit perspective | `k-playbook-local/results/YYYY-MM-DD/review-dependency-cve.md` |
| Audit scope | `pip-audit`, `trivy`, `grype`, `osv-scanner`, `govulncheck` |

Typical artifacts: `review-input.json`, `review-triage.md`, `raw/pip-audit.json`, `raw/trivy-fs.json`, and, where needed, `raw/grype.json`.

This recipe is active in the audit run model. It assesses the groups from `review-input.json` that carry at least one evidence item from an audit-scope tool.

### GitHub Dependabot Alerts

| | |
|---|---|
| Recipe | `k-playbook/reviews/review-dependabot-alerts.md` |
| Results | `k-playbook-local/results/dependabot-alerts/YYYY-MM-DD/` |
| Audit run model | disabled; rationale and examined alternatives are in the recipe |

Typical artifacts: `review-input.json`, `review-triage.md`, `raw/dependabot-alerts-open.jsonl` as an auditable import, and `raw/dependabot-alerts-summary.tsv` for rapid triage.

This family uses GitHub as its source. It is particularly suitable when a project does not use local dependency scanners or first wants to assess the alert set present in GitHub. Intentionally disabled Dependabot PRs, for example `open-pull-requests-limit: 0`, are not a finding.

### IaC/Container

| | |
|---|---|
| Recipe | `k-playbook/reviews/review-iac-container.md` |
| Results | `k-playbook-local/results/iac-container/YYYY-MM-DD/` |
| Audit perspective | `k-playbook-local/results/YYYY-MM-DD/review-iac-container.md` |
| Audit scope | `trivy`, `syft`, `grype` |

Typical artifacts: `review-input.json`, `review-triage.md` for container, image, IaC, and filesystem findings, `raw/trivy-*.json`, and, where needed, `raw/syft-*.json` and `raw/grype-*.json`.

This recipe is active in the audit run model. It assesses the groups from `review-input.json` that carry at least one evidence item from an audit-scope tool.

### Tech Debt

| | |
|---|---|
| Recipe | `k-playbook/reviews/review-tech.md` |
| Results | `k-playbook-local/results/tech/YYYY-MM-DD/` |
| Audit entry | Evidence source `tech`, SARIF under `k-playbook-local/results/YYYY-MM-DD/raw/tech.sarif` |

Typical artifacts in report mode: `review-input.json` and `review-triage.md`. This recipe operates in two modes with the same review criteria; only the result form differs. See also **Recipes as evidence sources** below.

### Recipes as Evidence Sources

Two catalog recipes provide their own evidence from code in a run rather than filtering existing evidence. They run before the merge, read only their frozen `scope.paths`, and write SARIF to `raw/<entry>.sarif`:

| Recipe | Entry | What it reads |
|---|---|---|
| `review-tech.md` | `tech` | Source and infrastructure files; tech-debt candidates with `tech-*` rule IDs. |
| `review-python-comment-hardspots.md` | `python-comment-hardspots` | Python sources; locations without reconstructable rationale, with `hardspot-*` rule IDs. |

Both remain selectable through `/k-review`: `review-tech` in report mode with the `tech` result family, and `review-python-comment-hardspots` interactively. The review criteria are the same in both modes; only the result form differs.

### Family-Only Recipes in the Audit Run Model

Some catalog recipes remain selectable through `/k-review` but are disabled for `/k-audit` until their inputs are available as evidence in `review-input.json` or a separate scope contract exists:

| Recipe | Reason |
|---|---|
| `review-k-check-security.md` | `k-check` results are not yet modeled as evidence in the merge. |
| `review-dependabot-alerts.md` | Input arrives externally through `gh api` and is not yet a tool entry in the run merge. |

Both recipes contain their detailed examination themselves: their **Position in the audit run model** section states which part of the evidence contract blocks them and what a conversion would require. Until then, their `review-triage.md` goes directly to `/k-remediation`, **without** consolidation across families and **without** deduplication against other sources. The same applies to every independent `/k-review` run of a family recipe: the family directory is outside every run directory and is not combined with the audit run.

## Review Log

`/k-review` maintains the log beside the results:

```text
k-playbook-local/results/log.md
```

For each family, it contains the latest run, when the next is due, mode and focus, and a log line with scope, output, and handoff.

Example handoff:

```text
/k-remediation k-playbook-local/results/k-check/2026-07-24/review-triage.md
```

Alongside this, `k-playbook-local/known-decisions.md` records what was deliberately decided. The merge step reads this single file and writes its effect visibly into `review-input.json` and `review-input.md`; the assessment then takes this information solely from the JSON.

## Remediation

`/k-remediation` understands these inputs:

- an audit run, `k-playbook-local/results/<date>/review-triage.md`: the primary path,
- a family, `k-playbook-local/results/<family>/<date>/review-triage.md`,
- legacy: an existing summary, `k-playbook-local/results/summary-YYYY-MM-DD.md`,
- legacy: a family with `assessment.md` and `findings.md` when no `review-triage.md` exists there.

`review-triage.md` is the primary working file everywhere. `raw/` and `run-metadata.*` are read-only.

**Summaries are no longer created.** Their creator was removed with the downstream prioritization step; neither `/k-audit` nor `/k-review` writes them. Whatever remains in an already configured project stays readable for `/k-remediation`: input from the past, not output from a run.

Before creating tasks, `/k-remediation` compares every finding with `tasks/` and `tasks/done/`. An existing task covers a finding when **source and bundle/group ID** match; title similarity alone is not enough. A match in `tasks/` prevents a second task; a match in `tasks/done/` is reported but does not close the finding: its reappearance in the result means it has returned.

A generated remediation task must contain:

- the source, `k-playbook-local/results/<family>/<date>/review-triage.md`,
- the bundle or group IDs from `review-triage.md`,
- the `review-triage.md` working register,
- the raw source, if present,
- the original location/message information,
- all findings to be resolved together when they share a fix/verification path,
- the remediation mode from `K-PLAYBOOK.yaml`,
- concrete verification steps.

The result path in a committed task is a **provenance statement, not a resolvable reference**: `results/` is not versioned, but tasks are. Anyone reading the task from the repository does not have the result file, and even on the machine that created it, it is overwritten by the next run. Inline evidence therefore remains required: group IDs, location, and message belong in the task itself, not only in a reference.

Project-wide policy:

```yaml
remediation:
  mode: task-branch-pr
  target: app
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false
```

Modes, from strictest to most open:

- `task-branch-pr`: no direct fixes. Every confirmed bundle becomes a task with a branch and PR note; implementation happens later through `/k-task-run`.
- `task-first`: tasks are the default. Direct fixes only after explicit approval for individual small bundles. **This is the default.**
- `direct-allowed`: small, safe findings may be fixed immediately after code inspection when the categories are approved.

`pr_required` and `direct_fixes` are derived from `mode` and recorded.

## Security Tools

Project virtual environments are normal for project dependencies. Read-only status may measure an active project virtual environment and labels that measurement context. Tool installation and Docker fallbacks remain separate and local to the host/user; no project virtual environment may be active before installation. Python CLI tools are recommended through `pipx` or, with `--method venv`, in dedicated k-playbook tool virtual environments.

```bash
k-playbook/scripts/install-security-tools.sh                    # status
k-playbook/scripts/install-security-tools.sh --install missing  # asks before installation
k-playbook/scripts/install-security-tools.sh --install missing --method venv  # dedicated tool virtual environments
```

The required tools are canonically listed in [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv). The script, interface, and review recipes read the same matrix.
