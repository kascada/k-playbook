---
description: Install the CodeQL CLI locally if needed, optionally create local CodeQL databases for the current project, and verify them with a CodeQL analysis query. Uses scripts/install-codeql-local.sh.
argument-hint: [target-dir|--cli-only]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep]
---

# k-install-codeql

Install and verify local CodeQL for a project.

This command is local-only. It does not configure GitHub CodeQL and does not edit `K-PLAYBOOK.MD`; `/k-setup-codeql` owns the project-local CodeQL decision block.

Use `--cli-only` when the project uses GitHub CodeQL but still wants a local CLI for fast preflight/status checks without local databases or SARIF results.

It calls:

`<PLAYBOOK_REPO>/scripts/install-codeql-local.sh`

## Step 1 — Target and playbook paths

Determine mode and `TARGET_DIR`:

- If `$ARGUMENTS` is exactly `--cli-only`, set `CLI_ONLY=true` and `TARGET_DIR = realpath(CWD)`.
- If `$ARGUMENTS` contains a target directory and `--cli-only` in a future extension, resolve the target directory and set `CLI_ONLY=true`.
- Otherwise set `CLI_ONLY=false`.
- If `$ARGUMENTS` is a target directory without `--cli-only`: resolve it with `realpath`, and abort if it does not exist.
- If `$ARGUMENTS` is empty or exactly `--cli-only`: `TARGET_DIR = realpath(CWD)`.
- Before using that value, apply the project-local base guard from `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`: if the selected directory has no `K-PLAYBOOK.MD`, but its parent does and the parent file's `base:` resolves to the selected directory, correct `TARGET_DIR` to the parent project root and show that correction in preflight. If the parent file has no `base:`, do not infer it; stop and ask the user to run `/k-setup` for the parent project first.

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Resolve:

- `base:` → `PLAYBOOK_BASE_DIR`, if present.

Also parse the optional CodeQL managed block in `<TARGET_DIR>/K-PLAYBOOK.MD` between:

- `<!-- k-setup-codeql:managed:begin -->`
- `<!-- k-setup-codeql:managed:end -->`

Extract when present:

- `local-database:`
- `database:`
- `languages:`
- `queries:`

Command-specific policy:

- If `K-PLAYBOOK.MD` is missing, stop and ask the user to run `/k-setup` and `/k-setup-codeql` first.
- If `base:` is missing, stop and ask the user to run `/k-setup` first. Do not infer legacy base paths here.
- If `base:` exists, offer it as the default parent directory for local CodeQL artifacts.
- If the CodeQL block has `database: <path>`, offer the parent of that path as the default parent directory.
- If neither a registered `database:` nor `base:` exists, stop; this should only happen when `/k-setup` has not migrated the project yet.
- In `CLI_ONLY=true`, `languages:` and `database:` are not required; use `base:` as the default parent unless the user chooses another parent.

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
  - Whether `TARGET_DIR` is a Git worktree.
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
Ziel:          <TARGET_DIR>
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
    - Default: parent of registered `database:` if set, else `base:`.
    - The script will create below it:
      - `codeql-cli/`
      - `databases/`
      - `results/`

In `CLI_ONLY=true`, ask only for the parent directory. The script will create or reuse only `codeql-cli/` and will not create `databases/` or `results/`.

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
  --project "<TARGET_DIR>" \
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
Parent:     <PARENT_DIR>
Sprachen:   <languages>
Datenbanken:
- <language>: <path> ok | fehlt
Ergebnisse:
- <language>: <path>.sarif ok | fehlt

Hinweis: `K-PLAYBOOK.MD` wurde nicht geändert. Falls die Pfade dauerhaft registriert werden sollen: `/k-setup-codeql` ausführen.
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
