---
description: Install the CodeQL CLI locally if needed, optionally create local CodeQL databases for the current project, and verify them with a CodeQL analysis query. Uses scripts/install-codeql-local.sh.
argument-hint: [target-dir|--cli-only]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep]
---

# k-install-codeql

Install and verify local CodeQL for a project.

CodeQL-specific rules live in `<PLAYBOOK_REPO>/global/rules/codeql.md` and must be treated as authoritative for this command.

This command is local-only. It does not configure GitHub CodeQL and does not edit `K-PLAYBOOK.yaml`; `/k-setup-codeql` owns the project-local `tools.codeql` decision.

Use `--cli-only` when the project uses GitHub CodeQL but still wants a local CLI for fast preflight/status checks without local databases or SARIF results.

It calls:

`<PLAYBOOK_REPO>/scripts/install-codeql-local.sh`

## Step 1 — Target and playbook paths

Determine mode and `TARGET_DIR` from the slash-command argument string:

- If the argument string is exactly `--cli-only`, set `CLI_ONLY=true` and `TARGET_DIR = realpath(CWD)`.
- If the argument string contains a target directory and `--cli-only` in a future extension, resolve the target directory and set `CLI_ONLY=true`.
- Otherwise set `CLI_ONLY=false`.
- If the argument string is a target directory without `--cli-only`: resolve it with `realpath`, and abort if it does not exist.
- If the argument string is empty or exactly `--cli-only`: `TARGET_DIR = realpath(CWD)`.
- Before using that value, apply the fixed-layout guard from `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`: if the selected directory is `<project>/k-playbook`, correct `TARGET_DIR` to the parent project root and show that correction in preflight.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Derive `PLAYBOOK_BASE_DIR = <TARGET_DIR>/k-playbook` and `PLAYBOOK_BASE_DISPLAY_PATH = k-playbook`.

Also parse optional `tools.codeql` metadata from `<TARGET_DIR>/K-PLAYBOOK.yaml` when present.

Extract when present:

- `target`
- `local_database.status`
- `local_database.path`
- `languages`
- `queries`

Command-specific policy:

- If `K-PLAYBOOK.yaml` is missing, stop and ask the user to add the project with the k-playbook Installer and then run `/k-setup-codeql` first.
- If `PLAYBOOK_BASE_DIR` is missing, stop and ask the user to open the k-playbook Installer and use `Vervollstaendigen` in the project block. Do not create the playbook base from this command.
- Offer `PLAYBOOK_BASE_DIR` as the default parent directory for local CodeQL artifacts.
- If `tools.codeql.local_database.path` is set, offer the parent of that path as the default parent directory.
- In `CLI_ONLY=true`, `languages` and `local_database.path` are not required. Use `PLAYBOOK_BASE_DIR` as the parent for local CodeQL artifacts; do not ask for a separate parent unless the user explicitly requests a non-standard location.
- If `target` is present, use it as the default project path for full local database mode. If missing, default to `TARGET_DIR`. The resolved CodeQL project path must exist.

## Step 2 — Preflight

Check:

- Script exists and is executable or can be run with `bash`:
  - `<PLAYBOOK_REPO>/scripts/install-codeql-local.sh`
- Existing CodeQL CLI:
  - `codeql version`
- Basic download/extract prerequisites:
  - `curl` or `wget`
  - `unzip`
- Git repo status:
  - Whether the resolved CodeQL project path from `target` is a Git worktree.
- Likely languages, using the same detection table as `/k-setup-codeql`:
  - `package.json`, `*.js`, `*.jsx`, `*.ts`, `*.tsx` → `javascript-typescript`
  - `pyproject.toml`, `requirements*.txt`, `*.py` → `python`
  - `go.mod`, `*.go` → `go`
  - `pom.xml`, `build.gradle*`, `*.java`, `*.kt` → `java-kotlin`
  - `*.csproj`, `*.sln`, `*.cs` → `csharp`
  - `Cargo.toml`, `*.rs` → `rust`
  - `*.cpp`, `*.cc`, `*.cxx`, `*.c`, `*.h`, `*.hpp` → `cpp`

Show a compact preflight summary:

```text
/k-install-codeql — Preflight
─────────────────────────────
Projekt:       <TARGET_DIR>
CodeQL Target: <CODEQL_PROJECT_DIR>
Script:        <path> ok | fehlt
CodeQL CLI:    ok (<version>) | fehlt, wird lokal installiert
Parent:        <suggested parent>
Sprachen:      <languages from CodeQL block or detected>
Queries:       <queries or security-extended>
Voraussetzung: curl/wget ok, unzip ok | fehlt: <tools>
```

If `curl`/`wget` and `unzip` are missing while `codeql` is not already available, stop and tell the user which host packages are required.

## Step 3 — User choices

Ask in one bundled interaction:

1. Parent-Verzeichnis für lokale CodeQL-Artefakte?
    - Default: parent of registered `local_database.path` if set, else `k-playbook/`.
    - The script will create below it:
      - `codeql-cli/`
      - `databases/`
      - `results/`

In `CLI_ONLY=true`, use `PLAYBOOK_BASE_DIR` as `PARENT_DIR`. Ask only for confirmation to install/check the CLI there. The script will create or reuse only `codeql-cli/` and will not create `databases/` or `results/`.

For `CLI_ONLY=true`, show this command:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" \
  --parent "<PARENT_DIR>" \
  --cli-only
```

Then ask:

> "Soll ich die CodeQL CLI jetzt installieren/prüfen?"

Do not run the script before confirmation.

For full local database mode, continue with the remaining choices:

2. Welche Sprachen?
   - Default: `languages:` from CodeQL block if present, else detected languages.
   - Allow custom comma-separated values.

3. Welche Queries?
   - Default: `queries:` from CodeQL block if present, else `security-extended`.
   - The script maps `security-extended` and `security-and-quality` to language-specific CodeQL suite files for local `database analyze`.

4. Existing databases?
   - Default: reuse existing databases.
   - Optional: recreate with `--force`.

Then show the exact command that will be executed:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" \
  --project "<CODEQL_PROJECT_DIR>" \
  --parent "<PARENT_DIR>" \
  --languages "<LANGUAGES>" \
  --queries "<QUERIES>"
```

Ask:

> "Soll ich das jetzt ausführen?"

Do not run the script before confirmation.

## Step 4 — Execute

After confirmation:

1. Verify the parent directory's parent exists; create only the selected parent directory as needed.
2. Run the script with `bash`.
3. Capture the full output.
4. If the script fails, diagnose the failure briefly and stop.

In `CLI_ONLY=true`, stop after the script succeeds and continue directly to the report. Do not verify databases or SARIF results.

## Step 5 — Verify

After a successful script run, verify:

- `codeql version` works either from PATH or from `<PARENT_DIR>/codeql-cli/codeql/codeql`.
- For every selected language:
  - `<PARENT_DIR>/databases/<language>/` exists.
  - `<PARENT_DIR>/results/<language>.sarif` exists and is non-empty.
- Run one explicit CodeQL check query/analysis verification:
  - Prefer the SARIF generated by the script from `codeql database analyze`.
  - If SARIF is missing, run `codeql database analyze <db> <queries> --format=sarif-latest --output=<result>` once for the first selected language.

Do not upload results anywhere.

Skip the database and SARIF checks in `CLI_ONLY=true`.

## Step 6 — Report

Print:

```text
Lokales CodeQL installiert/geprüft
─────────────────────────────────
CLI:        <path> (<version>)
Projekt:    <TARGET_DIR>
CodeQL:     <CODEQL_PROJECT_DIR>
Parent:     <PARENT_DIR>
Sprachen:   <languages>
Datenbanken:
- <language>: <path> ok | fehlt
Ergebnisse:
- <language>: <path>.sarif ok | fehlt

Hinweis: `K-PLAYBOOK.yaml` wurde nicht geändert. Falls die CodeQL-Entscheidung dauerhaft registriert werden soll: `/k-setup-codeql` ausführen.
```

For `CLI_ONLY=true`, print:

```text
Lokale CodeQL CLI installiert/geprüft
────────────────────────────────────
CLI:      <path> (<version>)
Projekt:  <TARGET_DIR>
Parent:   <PARENT_DIR>

Hinweis: Es wurden keine lokalen Datenbanken erzeugt und keine Analyse ausgeführt.
```
