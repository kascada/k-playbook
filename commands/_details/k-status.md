---
description: Fast read-only health overview for the current project, backed by k-playbook-installer status JSON.
argument-hint: [full|codeql|reviews|json|strict]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-status

Show a fast, read-only health overview for the current project.

This command is a status preflight, not a repair command:

- Start with the binary-backed project status: `k-playbook-installer status`.
- Treat the binary JSON as the authoritative base for project health.
- Prefer small existence and metadata checks over scans.
- Do not create files, change config, install tools, run tests, run builds, run smoke tests, start scanners, create CodeQL databases, run CodeQL analysis, upload SARIF, call GitHub APIs, or print Git diffs.
- If more checks are needed later, add them to the Go status implementation first where feasible, not as duplicated prompt logic.

## Modes

Interpret `$ARGUMENTS` as one optional mode:

- Empty: compact human-readable report from the binary JSON.
- `json`: print the binary JSON unchanged.
- `full`: compact report plus `K-PLAYBOOK.yaml` content when present.
- `strict`: compact report plus a health-gate summary where warnings count as failed gates.
- `codeql`: compact report plus lightweight CodeQL config metadata only when present in `K-PLAYBOOK.yaml`; do not run CodeQL.
- `reviews`: compact report focused on review status from the binary JSON; do not run reviews.

If `$ARGUMENTS` is anything else, print the supported modes and stop without running deeper checks.

## Step 1 - Target

Determine `TARGET_DIR` with `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`:

- For the current argument set, modes are not target paths; use `TARGET_DIR = realpath(CWD)`.
- Apply the fixed-layout guard from the shared module: if `TARGET_DIR` has no `K-PLAYBOOK.yaml`, but its parent has one and `TARGET_DIR` is named `k-playbook`, correct `TARGET_DIR` to the parent project root and show this correction in the compact report.
- If `<TARGET_DIR>/K-PLAYBOOK.yaml` is still missing after that correction, abort and tell the user to run `/k-gui`.

## Step 2 - Resolve Installer Binary

Resolve `INSTALLER_BIN` before running status. Try these candidates in order and use the first executable file:

- `~/dev/k-playbook/bin/k-playbook-installer`.
- `/workspaces/k-playbook/bin/k-playbook-installer`.
- `k-playbook-installer` from `PATH`.
- `~/.local/bin/k-playbook-installer`.

Do not build the binary from `/k-status`. In particular, do not run `make build`, `go build`, or `go run` from this command. If no candidate is executable, report the binary as unavailable and recommend one of these explicit setup actions:

- On a normal host: `make install` or `make install-from-source` in `~/dev/k-playbook`.
- For local developer testing: `make build` in `~/dev/k-playbook`, then re-run `/k-status`.
- In a DevContainer: ensure `/workspaces/k-playbook/bin/k-playbook-installer` exists via the mounted repo or run `make install` in `~/dev/k-playbook`.

## Step 3 - Binary Status

Run the installer status command from `TARGET_DIR`:

```bash
"<INSTALLER_BIN>" status
```

If the shell cannot reliably run with `TARGET_DIR` as working directory, use:

```bash
"<INSTALLER_BIN>" status "<TARGET_DIR>"
```

Expected JSON shape:

```json
{
  "path": "/path/to/project",
  "name": "project",
  "environment": "plain",
  "selected": true,
  "detected": ["go.mod"],
  "playbook": {},
  "projectRoot": {},
  "setup": {},
  "structure": {},
  "docs": {},
  "remediation": {},
  "tasks": {},
  "todo": {},
  "reviews": {},
  "enforcement": {},
  "git": {},
  "recommendations": [],
  "devcontainer": {}
}
```

If `INSTALLER_BIN` is missing, report that the binary is unavailable and include the candidate paths checked. Do not recreate the old manual status implementation as a fallback unless the user explicitly asks for degraded best-effort output.

## Compact Output

Default output should be short and derived from the JSON:

```text
/k-status
------------------------
Projekt:       /path/to/project
K-PLAYBOOK:    OK, updated_at 2026-07-20
Project-Root:  OK, . (git)
Setup:         OK, K-PLAYBOOK.yaml vorhanden
Struktur:      WARN, 2 Pfade fehlen
Docs:          WARN, docs-Verzeichnis enthaelt noch keine Markdown-Dateien
Remediation:   OK, direct-allowed
Tasks:         WARN, 3 offen, naechste: 002-example.md
TODO:          OK, 0 offen
Reviews:       WARN, known-decisions fehlt
Enforcement:   OK, 1 Regel
Git:           WARN, dirty (4 geaendert, 1 untracked)
Devcontainer:  WARN, 2 Eintraege fehlen

Naechste Aktionen:
1. /k-gui
2. /k-run
3. /k-code2docs
```

Only show `Devcontainer` when the JSON contains `devcontainer`.

Use these labels consistently:

- `OK`: corresponding JSON object has `ok: true`.
- `WARN`: corresponding JSON object has `ok: false`, but the project remains inspectable.
- `FAIL`: the binary command failed, `K-PLAYBOOK.yaml` is missing/invalid, or a configured required path is missing.
- `projectRoot.ok: false` is a failed configuration gate. Do not search for a Git root in `/k-status`; report the message from the JSON and recommend `/k-gui`.

## JSON Mode

In `json` mode, print the `"<INSTALLER_BIN>" status` JSON unchanged. Do not prepend warnings or prose unless the binary failed.

## Full Mode

In `full` mode:

- Print the compact report.
- If `<TARGET_DIR>/K-PLAYBOOK.yaml` exists, read and print it under a separate `K-PLAYBOOK.yaml` heading.
- Do not print large generated files, diffs, logs, task contents, review contents, or docs contents.

## Strict Mode

In `strict` mode:

- Print the compact report.
- Add `Health-Gates: OK` only when every shown status object is OK.
- Add `Health-Gates: FAIL (<warn> warn gates, <fail> fail gates)` when warnings or failures exist.
- Do not change exit behavior and do not modify files.

## CodeQL Mode

In `codeql` mode:

- Start with the binary status JSON.
- Read only lightweight `tools.codeql` metadata from `K-PLAYBOOK.yaml` when present.
- Check configured paths only with existence metadata when needed.
- Do not run `codeql version`, `codeql database create`, `codeql database analyze`, uploads, or GitHub API calls.

If deeper CodeQL status is needed, add it to the installer binary as a lightweight status field first.

## Reviews Mode

In `reviews` mode:

- Start with the binary status JSON.
- Report the `reviews` object and recommendations relevant to reviews.
- Do not run review recipes or read full review result contents.

If due-date calculation or richer review metadata is needed, add it to the installer binary as lightweight status fields first.
