---
description: Configure the project-local CodeQL decision in K-PLAYBOOK.MD. Separately asks for GitHub CodeQL and local CodeQL database usage, records the result, and only creates CodeQL files after explicit confirmation.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-setup-codeql

Set up the project-local CodeQL decision for security, quality, and enforcement checks.

CodeQL-specific rules live in `<PLAYBOOK_REPO>/global/rules/codeql.md` and must be treated as authoritative for this command.

This command is the CodeQL-specific companion to `/k-setup`:

- `/k-setup` owns the fixed complete `k-playbook/` project-local layout in `K-PLAYBOOK.MD`.
- `/k-setup-codeql` owns only the CodeQL decision block in `K-PLAYBOOK.MD`.
- GitHub CodeQL and local CodeQL databases are independent choices; ask for both separately.
- If GitHub CodeQL is active or planned, offer only a local CodeQL CLI install for preflight/status checks. Use `scripts/install-codeql-local.sh --parent "<CODEQL_PARENT_DIR>" --cli-only`; this GitHub path must not create local databases, SARIF results, or run analysis.
- Do not run scans, create databases, or change GitHub Actions automatically. Generate files only after explicit confirmation.

## Step 1 — Target bestimmen

Determine `TARGET_DIR` from the slash-command argument string:

- If the argument string is non-empty: treat it as the target directory, resolve with `realpath`, and abort if it does not exist.
- If the argument string is empty: `TARGET_DIR = realpath(CWD)`.
- Before using that value, apply the fixed-layout guard from `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`: if the selected directory is `<project>/k-playbook`, correct `TARGET_DIR` to the parent project root and show that correction in preflight.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Derive the mandatory local CodeQL artifact parent:

- `CODEQL_PARENT_DIR = <TARGET_DIR>/k-playbook`
- `CODEQL_PARENT_DISPLAY_PATH = k-playbook`

Use `CODEQL_PARENT_DIR` for every `--parent` argument this command shows or runs. Do not ask for a separate parent in `/k-setup-codeql`; the project-local playbook base is the canonical parent for CLI-only CodeQL artifacts.

Command-specific policy:

- If `K-PLAYBOOK.MD` is missing, stop and ask the user to run `/k-setup` first. This command writes only into an existing pointer/config file.
- If `CODEQL_PARENT_DIR` does not exist, stop and ask the user to run `/k-setup` first. Do not create the playbook base from this command.
- Determine a CodeQL analysis target directory (`codeql target`) separately from `TARGET_DIR`. Default is `.` (the project root). If the project root is a wrapper and exactly one nested Git worktree contains the detected application manifests, suggest that nested worktree, e.g. `./app`. Normalize as a project-relative path. Do not infer outside-target paths without asking. Store the resolved path as `CODEQL_TARGET_DIR` and the display path as `CODEQL_TARGET_DISPLAY_PATH`.

## Step 2 — Detect current state

Inspect the target project for CodeQL signals:

- Existing CodeQL managed block in `K-PLAYBOOK.MD` between:
  - `<!-- k-setup-codeql:managed:begin -->`
  - `<!-- k-setup-codeql:managed:end -->`
- Existing CodeQL `target:` value inside the managed block, if present.
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
Projekt:           <TARGET_DIR>
CodeQL Target:     <CODEQL_TARGET_DISPLAY_PATH>
K-PLAYBOOK.MD:     gefunden | fehlt
Bestehender Block: ja | nein
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
   - `ja` — record `github: true`.
   - `nein` — record `github: false`.
   - `später` — record `github: planned`.

2. Lokale CodeQL-Datenbank aktivieren?
   - `ja` — record `local-database: true`.
   - `nein` — record `local-database: false`.
   - `später` — record `local-database: planned`.

3. Welche Sprachen sollen registriert werden?
   - Suggest detected languages.
   - Allow the user to edit the list.

4. Welches Verzeichnis soll CodeQL analysieren?
   - Default: existing `target:` from the CodeQL block if present.
   - Else if a nested Git/app root was detected, suggest that path, e.g. `./app`.
   - Else default to `.`.
   - Normalize relative paths as `./...` except `.`.
   - The target path must exist and should normally be the Git/worktree root whose source manifests are analyzed.

If local database is `true` or `planned`, ask for the database path:

- Default: `./k-playbook/codeql-db/`.
- Normalize relative paths as `./...`.

If GitHub CodeQL is `true` or `planned`, ask whether a workflow path should be recorded:

- Default: `<target>/.github/workflows/codeql.yml`, normalized project-relative. If `target:` is `.`, this is `./.github/workflows/codeql.yml`; if `target:` is `./app`, this is `./app/.github/workflows/codeql.yml`.
- Record the path even if the file does not exist yet.
- Do not create the workflow unless Step 5 explicitly confirms file generation.

Ask which query suite should be the default:

- `security-extended` (recommended)
- `security-and-quality`
- custom query pack / suite string

## Step 4 — Draft CodeQL block

Compose a managed CodeQL block and show it to the user before writing.

Exact format:

```markdown
<!-- k-setup-codeql:managed:begin -->

## CodeQL

- enabled:        true
- target:         .
- github:        true
- workflow:      ./.github/workflows/codeql.yml
- local-database: false
- database:      -
- languages:     python,javascript-typescript
- queries:       security-extended
- setup-run:     2026-07-20

<!-- k-setup-codeql:managed:end -->
```

Rules:

- `enabled:` is `true` when either `github` or `local-database` is `true` or `planned`; otherwise `false`.
- `target:` is the normalized project-relative CodeQL analysis root. Use `.` for the project root or `./<path>` for nested app/worktree roots.
- `github:` is one of `true`, `false`, `planned`.
- `workflow:` is a normalized path when GitHub CodeQL is `true` or `planned`; otherwise `-`.
- `local-database:` is one of `true`, `false`, `planned`.
- `database:` is a normalized path when local database is `true` or `planned`; otherwise `-`.
- `languages:` is a comma-separated list without spaces, or `-` if intentionally unset.
- `queries:` is a suite/query-pack string, or `-` if intentionally unset.
- `setup-run:` is today's ISO date.

Ask:

> "Passt das so, oder soll ich etwas anpassen?"

Wait for confirmation.

## Step 5 — Optional file generation

After the user confirms the block, ask one separate question only if file generation would be useful:

- If GitHub CodeQL is `true` and no workflow exists at the recorded workflow path:
  - Offer to create a minimal GitHub Actions workflow.
  - Default recommendation: create only if the project uses GitHub Actions.
- If GitHub CodeQL is `true` and a workflow already exists:
  - Do not edit it silently. Offer to leave it unchanged or inspect/update after confirmation.
- If local database is `true` and the database parent directory does not exist:
  - Offer to create only the parent directory.
  - Do not run `codeql database create`.
- If GitHub CodeQL is `true` or `planned` and `codeql version` is missing:
  - Offer to install only the local CodeQL CLI with `bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" --parent "<CODEQL_PARENT_DIR>" --cli-only`.
  - Default recommendation: install it, because `/k-status` and other preflight checks can verify the configured CodeQL setup locally even when analysis runs in GitHub Actions.
  - Do not call `/k-install-codeql` full local-database mode from this GitHub path.
  - Do not create local databases, do not create SARIF results, and do not run CodeQL analysis in this path.
- If GitHub CodeQL is `true` or `planned` and `codeql version` already works:
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

## Step 6 — Write K-PLAYBOOK.MD

Update `K-PLAYBOOK.MD`:

- If a CodeQL managed block exists, replace only that block.
- If no CodeQL managed block exists, append the new block after the existing `/k-setup` managed block when present; otherwise append it to the end of the file.
- Preserve all content outside the CodeQL managed markers.
- Do not modify the `/k-setup` managed block.

Then perform any explicitly confirmed optional file generation from Step 5.

If the user explicitly confirmed CLI-only installation, run:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" --parent "<CODEQL_PARENT_DIR>" --cli-only
```

This is the only install command `/k-setup-codeql` may run when GitHub CodeQL is `true` or `planned`. It may create `<CODEQL_PARENT_DIR>/codeql-cli/` only. It must not create databases, SARIF results, or run analysis.

## Step 7 — Summary

Print a short summary:

```text
CodeQL-Setup abgeschlossen
──────────────────────────
K-PLAYBOOK.MD:  aktualisiert
CodeQL Target:  <path>
GitHub CodeQL:  true | false | planned
Workflow:       <path> | - | unverändert | erzeugt
CLI:            ok (<path>) | installiert (<path>) | fehlt/nicht installiert
CodeQL Parent:  <CODEQL_PARENT_DISPLAY_PATH>
Lokale DB:      true | false | planned
Datenbankpfad:  <path> | -
Sprachen:       <languages>
Queries:        <queries>

Hinweis: Es wurde keine Analyse ausgeführt und keine lokale CodeQL-Datenbank erzeugt.
```

If the command file itself was just added to k-playbook, remind the user that `/k-install` must be run once on each host so `/k-setup-codeql` appears in OpenCode autocomplete.
