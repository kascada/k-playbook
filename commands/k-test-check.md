---
description: Run unit tests, diagnose failures, and report the root cause before asking whether to fix them.
argument-hint: [path-or-command]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep]
---

# k-test-check

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Run unit tests, find the root cause of each failure, explain it briefly, then ask before making any changes.

## Invocation

`/k-test-check` — auto-detect test framework and run all tests  
`/k-test-check <path>` — run tests in a specific directory or file  
`/k-test-check <command>` — run an explicit test command (e.g. `pytest src/tests/`)

---

## Step 1 — Detect how to run tests

If `$ARGUMENTS` is a shell command or explicit path: use it directly.

Otherwise, detect the test framework from the project:

| Signal | Framework | Default command |
|---|---|---|
| `pytest.ini`, `pyproject.toml` with `[tool.pytest]`, `conftest.py` | pytest | `pytest` |
| `package.json` with `jest` or `vitest` | Jest/Vitest | `npm test` or `npx vitest` |
| `Makefile` with a `test` target | Make | `make test` |
| `go.mod` | Go | `go test ./...` |
| `Cargo.toml` | Rust | `cargo test` |

If nothing is detected: ask the user for the test command before continuing.

## Step 2 — Run tests

Execute the detected command and capture full output (stdout + stderr).

If all tests pass: report "Alle Tests grün." and stop.

## Step 3 — Parse failures

For each failed test, extract:
- Test name / file / line number
- Error message and stack trace

## Step 4 — Diagnose each failure

For each failure, read the relevant test file and the source file(s) it tests. Investigate the root cause:

- Is the test itself broken (wrong expectation, outdated fixture)?
- Is the source code broken (logic error, missing function, wrong return value)?
- Is it an environment/import issue (missing dependency, wrong path)?
- Did a recent refactor break the contract the test relies on?

Do **not** make any changes yet.

## Step 5 — Report

Print a structured report:

```
Tests: <N passed>, <M failed>

── FEHLER 1: <test name> ──────────────────────
Datei:   <test file>:<line>
Fehler:  <error message, one line>
Ursache: <1-3 sentences: root cause in plain language>
         Test kaputt / Code kaputt / Umgebung / Vertrag gebrochen

── FEHLER 2: <test name> ──────────────────────
...
```

Keep each "Ursache" concise — what is broken and why, not how to fix it.

## Step 6 — Ask

After the report, ask:

"Soll ich die Fehler korrigieren? (alle / nur bestimmte / nein)"

Wait for the answer. Do not make any changes until confirmed.
