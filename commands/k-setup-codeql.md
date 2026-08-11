---
description: Configure the project-local CodeQL decision in K-PLAYBOOK.yaml. Separately asks for GitHub CodeQL and local CodeQL database usage, records the result, and only creates CodeQL files after explicit confirmation.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-setup-codeql

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Set up the project-local CodeQL decision for security, quality, and enforcement checks.

CodeQL-specific rules live in `<playbook.dir>/rules/codeql.md` and must be treated as authoritative for this command.

This command is the CodeQL-specific companion to the project config created by `/k-gui`:

- `/k-gui` owns `K-PLAYBOOK.yaml` and the project-local structure.
- `/k-setup-codeql` owns only `tools.codeql` in `K-PLAYBOOK.yaml`.
- GitHub CodeQL and local CodeQL databases are independent choices; ask for both separately.
- If GitHub CodeQL is active or planned, offer only a local CodeQL CLI install for preflight/status checks. Use `scripts/install-codeql-local.sh --parent "<CODEQL_PARENT_DIR>" --cli-only`; this GitHub path must not create local databases, SARIF results, or run analysis.
- Do not run scans, create databases, or change GitHub Actions automatically. Generate files only after explicit confirmation.

## Step 1 — Target bestimmen

`PLAYBOOK_DIR`, `DIST_DIR` and `PROJECT_REPO_ROOT_DIR` come from the context output as `playbook.dir`, `playbook.dir` and `project.repoRoot`. Interpret the slash-command argument string:

- If the argument string is non-empty: treat it as the target directory, resolve with `realpath`, and abort if it does not exist.
- If the argument string is empty: run discovery from the current working directory.
- The context call searches upwards on its own, so it also works from inside the k-playbook directory; do not implement a separate guard.

Derive the mandatory local CodeQL artifact parent:

- `CODEQL_PARENT_DIR = <PLAYBOOK_DIR>`
- `CODEQL_PARENT_DISPLAY_PATH = k-playbook`

Use `CODEQL_PARENT_DIR` for every `--parent` argument this command shows or runs. Do not ask for a separate parent in `/k-setup-codeql`; the project-local playbook base is the canonical parent for CLI-only CodeQL artifacts.

Command-specific policy:

- If the context call failed, this is not a k-playbook project; stop and ask the user to run `/k-gui` first. This command writes only into an existing project config file.
- If `CODEQL_PARENT_DIR` does not exist, stop and ask the user to run `/k-gui`. Do not create the playbook base from this command.
- Determine a CodeQL analysis target directory (`codeql target`) separately from `PROJECT_REPO_ROOT_DIR`. Default is `.` (the project root). If the project root is a wrapper and exactly one nested Git worktree contains the detected application manifests, suggest that nested worktree, e.g. `./app`. Normalize as a project-relative path. Do not infer outside-target paths without asking. Store the resolved path as `CODEQL_TARGET_DIR` and the display path as `CODEQL_TARGET_DISPLAY_PATH`.

## Step 2 — Detect current state

Inspect the target project for CodeQL signals:

- Existing `tools.codeql` object in `K-PLAYBOOK.yaml`, if present.
- Existing CodeQL `target` value inside that object, if present.
- Nested Git worktrees or app roots that may be the actual CodeQL analysis target.
- GitHub Actions workflows:
  - `<CODEQL_TARGET_DIR>/.github/workflows/codeql.yml`
  - `<CODEQL_TARGET_DIR>/.github/workflows/codeql.yaml`
  - any workflow under `<CODEQL_TARGET_DIR>/.github/workflows/` containing `github/codeql-action/`
- Local CodeQL config:
  - `.github/codeql/codeql-config.yml`
  - `.github/codeql/codeql-config.yaml`
  - `codeql-config.yml`
  - `codeql-config.yaml`
- Local CodeQL databases or intended database directories:
  - `k-playbook/codeql/`
  - `k-playbook/codeql-db/`
  - `.codeql/`
- Local CLI-only install:
  - `k-playbook/codeql-cli/codeql/codeql`
- CodeQL CLI availability:
  - `codeql version`

Detect likely languages from common project signals below `CODEQL_TARGET_DIR`, not necessarily below the wrapper project root:

| Signal | Language |
|--------|----------|
| `package.json`, `*.js`, `*.jsx`, `*.ts`, `*.tsx` | `javascript-typescript` |
| `pyproject.toml`, `requirements*.txt`, `*.py` | `python` |
| `go.mod`, `*.go` | `go` |
| `pom.xml`, `build.gradle*`, `*.java`, `*.kt` | `java-kotlin` |
| `*.csproj`, `*.sln`, `*.cs` | `csharp` |
| `Cargo.toml`, `*.rs` | `rust` |
| `*.cpp`, `*.cc`, `*.cxx`, `*.c`, `*.h`, `*.hpp` | `cpp` |

Present a compact status table:

```text
/k-setup-codeql — Preflight
────────────────────────────
Projekt:           <PROJECT_REPO_ROOT_DIR>
CodeQL Target:     <CODEQL_TARGET_DISPLAY_PATH>
K-PLAYBOOK.yaml:   gefunden | fehlt
tools.codeql:      ja | nein
GitHub Workflow:   <path> | fehlt
CodeQL Config:     <path> | fehlt
Lokale DB:         <path> | fehlt
CodeQL Parent:     <CODEQL_PARENT_DISPLAY_PATH>
CodeQL CLI:        ok (<version>) | fehlt
Sprachen:          <detected languages> | unklar
```

## Step 3 — Entscheidungen abfragen

Ask the user in one bundled interaction:

1. GitHub CodeQL aktivieren?
   - `ja` — record `github.status: enabled`.
   - `nein` — record `github.status: disabled`.
   - `später` — record `github.status: planned`.

2. Lokale CodeQL-Datenbank aktivieren?
   - `ja` — record `local_database.status: enabled`.
   - `nein` — record `local_database.status: disabled`.
   - `später` — record `local_database.status: planned`.

3. Welche Sprachen sollen registriert werden?
   - Suggest detected languages.
   - Allow the user to edit the list.

4. Welches Verzeichnis soll CodeQL analysieren?
   - Default: existing `target:` from the CodeQL block if present.
   - Else if a nested Git/app root was detected, suggest that path, e.g. `./app`.
   - Else default to `.`.
   - Normalize relative paths as `./...` except `.`.
   - The target path must exist and should normally be the Git/worktree root whose source manifests are analyzed.

If local database status is `enabled` or `planned`, ask for the database path:

- Default: `./k-playbook/codeql-db/`.
- Normalize relative paths as `./...`.

If GitHub CodeQL status is `enabled` or `planned`, ask whether a workflow path should be recorded:

- Default: `<target>/.github/workflows/codeql.yml`, normalized project-relative. If `target:` is `.`, this is `./.github/workflows/codeql.yml`; if `target:` is `./app`, this is `./app/.github/workflows/codeql.yml`.
- Record the path even if the file does not exist yet.
- Do not create the workflow unless Step 5 explicitly confirms file generation.

Ask which query suite should be the default:

- `security-extended` (recommended)
- `security-and-quality`
- custom query pack / suite string

## Step 4 — Draft `tools.codeql`

Compose the `tools.codeql` YAML object and show it to the user before writing.

Exact format:

```yaml
tools:
  codeql:
    target: .
    languages:
      - python
      - javascript-typescript
    queries: security-extended
    github:
      status: enabled
      workflow: ./.github/workflows/codeql.yml
    local_database:
      status: disabled
      path: null
```

Rules:

- `target` is the normalized project-relative CodeQL analysis root. Use `.` for the project root or `./<path>` for nested app/worktree roots.
- `github.status` is one of `enabled`, `disabled`, `planned`.
- `github.workflow` is a normalized path when GitHub CodeQL is `enabled` or `planned`; otherwise `null`.
- `local_database.status` is one of `enabled`, `disabled`, `planned`.
- `local_database.path` is a normalized path when local database is `enabled` or `planned`; otherwise `null`.
- `languages` is a YAML list, or an empty list if intentionally unset.
- `queries` is a suite/query-pack string, or `null` if intentionally unset.
- Do not store a redundant `enabled` field; derive it from the two status fields.

Ask:

> "Passt das so, oder soll ich etwas anpassen?"

Wait for confirmation.

## Step 5 — Optional file generation

After the user confirms the block, ask one separate question only if file generation would be useful:

- If GitHub CodeQL status is `enabled` and no workflow exists at the recorded workflow path:
  - Offer to create a minimal GitHub Actions workflow.
  - Default recommendation: create only if the project uses GitHub Actions.
- If GitHub CodeQL status is `enabled` and a workflow already exists:
  - Do not edit it silently. Offer to leave it unchanged or inspect/update after confirmation.
- If local database status is `enabled` and the database parent directory does not exist:
  - Offer to create only the parent directory.
  - Do not run `codeql database create`.
- If GitHub CodeQL status is `enabled` or `planned` and `codeql version` is missing:
  - Offer to install only the local CodeQL CLI with `bash "<playbook.dir>/scripts/install-codeql-local.sh" --parent "<CODEQL_PARENT_DIR>" --cli-only`.
  - Default recommendation: install it, because `/k-status` and other preflight checks can verify the configured CodeQL setup locally even when analysis runs in GitHub Actions.
  - Do not call `/k-install-codeql` full local database mode from this GitHub path.
  - Do not create local databases, do not create SARIF results, and do not run CodeQL analysis in this path.
- If GitHub CodeQL status is `enabled` or `planned` and `codeql version` already works:
  - Report the detected CLI and do not install another copy.

Minimal workflow skeleton, only when explicitly confirmed:

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '30 2 * * 1'

jobs:
  analyze:
    name: Analyze (${{ matrix.language }})
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      packages: read
      actions: read
      contents: read
    strategy:
      fail-fast: false
      matrix:
        language: [<github-codeql-language-list>]
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: ${{ matrix.language }}
          queries: <queries>
      - name: Autobuild
        uses: github/codeql-action/autobuild@v3
      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v3
```

Language mapping for GitHub workflow:

| Registered value | GitHub CodeQL workflow value |
|------------------|------------------------------|
| `javascript-typescript` | `javascript-typescript` |
| `python` | `python` |
| `go` | `go` |
| `java-kotlin` | `java-kotlin` |
| `csharp` | `csharp` |
| `cpp` | `c-cpp` |
| `rust` | `rust` |

If the default branch is not clearly `main`, inspect `git symbolic-ref refs/remotes/origin/HEAD` or ask before writing workflow branches.

## Step 6 — Write K-PLAYBOOK.yaml

Update `K-PLAYBOOK.yaml`:

- Replace only the `tools.codeql` object.
- If `tools` does not exist, create it.
- Preserve all other YAML keys, including setup fields and unrelated tool decisions.
- Do not modify fields owned by `/k-gui` except if a valid YAML rewrite requires stable formatting.

Then perform any explicitly confirmed optional file generation from Step 5.

If the user explicitly confirmed CLI-only installation, run:

```bash
bash "<playbook.dir>/scripts/install-codeql-local.sh" --parent "<CODEQL_PARENT_DIR>" --cli-only
```

This is the only install command `/k-setup-codeql` may run when GitHub CodeQL status is `enabled` or `planned`. It may create `<CODEQL_PARENT_DIR>/codeql-cli/` only. It must not create databases, SARIF results, or run analysis.

## Step 7 — Summary

Print a short summary:

```text
CodeQL-Setup abgeschlossen
──────────────────────────
K-PLAYBOOK.yaml: aktualisiert
CodeQL Target:  <path>
GitHub CodeQL:  enabled | disabled | planned
Workflow:       <path> | - | unverändert | erzeugt
CLI:            ok (<path>) | installiert (<path>) | fehlt/nicht installiert
CodeQL Parent:  <CODEQL_PARENT_DISPLAY_PATH>
Lokale DB:      enabled | disabled | planned
Datenbankpfad:  <path> | -
Sprachen:       <languages>
Queries:        <queries>

Hinweis: Es wurde keine Analyse ausgeführt und keine lokale CodeQL-Datenbank erzeugt.
```

If the command file itself was just added to k-playbook, remind the user that `/k-gui` must refresh registration once on each host so `/k-setup-codeql` appears in OpenCode autocomplete.
