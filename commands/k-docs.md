---
description: Inspect the project documentation state, report consistency gaps and offer the available docs actions: code docs, tool references, material extraction, version inventory, index rebuild and memory registration. With an argument, dispatches directly to that action.
argument-hint: [status|code|tools|extract|inventory|index]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite]
---

# k-docs

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

Prüft den Dokumentationsbestand unter `k-playbook-local/docs/` und bietet an, welche
Docs-Aktionen sinnvoll möglich sind. Die langen Produzenten liegen als nachladbare Module
unter `commands/_docs/`; dieser Command dupliziert deren Ablauf nicht.

Produces:
- Standardmäßig nichts. Ohne Auswahl ist `/k-docs` read-only.
- Bei Auswahl eines Produzenten schreibt nur das nachgeladene Modul in seine eigene
  Herkunft.

## Schritt 1 — Pfade auflösen

From the context output:

- `RESOLVED_DOCS_DIR = <local.dir>/docs`
- `DOCS_DISPLAY_PATH = k-playbook-local/docs`
- `CODE_DIR = <RESOLVED_DOCS_DIR>/code`
- `LIBS_DIR = <RESOLVED_DOCS_DIR>/libs`
- `EXTRACTED_DIR = <RESOLVED_DOCS_DIR>/extracted`
- `VERSIONS_DIR = <RESOLVED_DOCS_DIR>/versions`
- `INVENTORY_FILE = <VERSIONS_DIR>/inventory.md`
- `MANUAL_DIR = <RESOLVED_DOCS_DIR>/manual`
- `MATERIAL_DIR = <local.dir>/material`
- `INDEX_FILE = <RESOLVED_DOCS_DIR>/README.md`

Command-specific policy:

- If `RESOLVED_DOCS_DIR` is missing: ask whether to create exactly that directory or to run
  `/k-gui`. Do not use a fallback path and do not abort hard.
- `CODE_DIR`, `LIBS_DIR`, `EXTRACTED_DIR` and `VERSIONS_DIR` are producer directories.
  Missing is normal; do not create them during the status pass.
- `MANUAL_DIR` and `MATERIAL_DIR` are created by setup. If either is missing, report it as
  a setup gap and offer `/k-gui`; do not create it silently.
- Without a confirmed action this command writes nothing.

## Schritt 2 — Bestand prüfen

Collect these facts, compactly:

- Doc files per origin: `code/`, `libs/`, `extracted/`, `versions/`, `manual/`, plus flat
  `docs/*.md` except `README.md`.
- For the inventory under `VERSIONS_DIR`, the frontmatter values `generated.at` and
  `inventory.*`. Its **state** comes from there and from nowhere else — never derive it
  from the Markdown body. Reading the entry tables is a different matter and happens only
  for the authority check in Schritt 3.
- The state of the version source configuration from `versionSources` in the context
  output. Do not read `k-playbook-local/version-sources.yaml` yourself.
- Whether `INDEX_FILE` exists.
- Whether `AGENTS.md` exists and mentions `k-playbook-local/docs/README.md`.
- Whether `opencode.json` or `opencode.jsonc` exists and contains a `references.docs.path`
  pointing to `./k-playbook-local/docs`.
- Material files under `MATERIAL_DIR`, if the directory exists.
- Manifest hints for tool docs: `pyproject.toml`, `requirements*.txt`, `package.json`,
  `go.mod`, `Cargo.toml`, `Gemfile`, `composer.json`, `pom.xml`, `build.gradle*`,
  `mix.exs` under `project.repoRoot`, respecting the usual exclusions from the producer
  modules.

For every doc file except README files, read only frontmatter and Markdown links needed for
the checks below. Do not rewrite any doc file.

## Schritt 3 — Konsistenz-Befunde melden

Report, but do not repair:

- Missing or incomplete frontmatter (`title` or `description` absent).
- Dead relative Markdown links.
- `generated.by` that does not match the origin:
  - `docs/code/`: `k-docs-code`, legacy `k-code2docs`, `ks-overlay-repo-analyse`.
  - `docs/libs/`: `k-docs-tools`, legacy `k-tools-scan`.
  - `docs/extracted/`: `k-docs-extract`.
  - `docs/versions/`: `k-doc-inventory` — on both call paths, the subcommand
    `k-playbook inventory` included.
- Flat doc files under `docs/` besides `README.md`.
- `libs/README.md` with an overview table instead of only explanatory text.
- Missing index or missing memory registration in `AGENTS.md` / `opencode.json`.

**Versionsangaben aus `LIBS_DIR` gegen das Inventar.** Only if the inventory exists. For
every file under `LIBS_DIR`, take its frontmatter `version` and `version-pin` and look up
the same tool in `INVENTORY_FILE`. Read only the inventory's context tables for that —
`Gegenstand`, `Version` and `Pin` — and match by the canonical name behind the group key
`<ecosystem>/<name>`; a tool that is not in the inventory is no finding, and neither is a
lib file without `version`.

Report every difference as its **own, non-repairing** finding, one line each:

```text
libs/<name>.md: version <wert> (<pin>) — Inventar: <wert> (<pin>), <herkunft>
```

The inventory is the authority on versions; `docs/libs/` stays curated pitfall
documentation whose `version` is a snapshot as of its `last-reviewed`. Do **not** rewrite
`docs/libs/`, not even after confirmation — that belongs to `/k-docs-tools` — and do not
count these lines as inventory deviations: a deviation arises from two sources, this
difference from two documents. Where both disagree, the inventory holds.

The two origins do not blend: `docs/libs/` is **curated and pitfall-oriented** (a
selection, one profile per tool, written by `/k-docs-tools`), `docs/versions/` is
**complete and source-oriented** (every declared version from every source, with its
origin per line, written by `/k-doc-inventory` through `k-playbook inventory`). Each
finding above therefore names both sides and the way to close it: refresh the inventory
with `/k-doc-inventory` if it is older than the manifest, or refresh the profile with
`/k-docs-tools` if its content — not just its number — is stale. When a version bump is
what caused the difference, the obligation to update the inventory in the same working
step is stated in `rules/docs-sync.md`, section „Nachzug bei Versionssprüngen".

If there are no findings, say so in one line.

## Schritt 4 — Möglichkeiten anbieten

Build the option list from the facts, not from guesses:

```text
/k-docs — Stand
─────────────────────────────────────
code/       <N> Dateien | fehlt
libs/       <N> Dateien | fehlt
extracted/  <N> Dateien | fehlt
versions/   <N> Dateien | fehlt
manual/     <N> Dateien | fehlt
unsortiert  <N> Dateien
Index       vorhanden | fehlt
Memory      ok | fehlt AGENTS.md | fehlt opencode.json | unvollständig
Material    <N> Dateien
Manifeste   <N> gefunden
Inventar    erhoben <generated.at>, <sources-read> Quellen gelesen,
            <sources-configured> konfiguriert, <deviations> Abweichungen | fehlt
Quellkonfig version-sources.yaml vorhanden (<N> Wurzeln, <N> Quellen,
            <N> Ausschlüsse) | fehlt | defekt

Mögliche Aktionen:
  1. code       Code semantisch analysieren → /k-docs-code
  2. tools      Libraries/Tools dokumentieren → /k-docs-tools
  3. extract    Rohmaterial verdichten → /k-docs-extract
  4. inventory  Versionsinventar erheben → /k-doc-inventory
  5. index      Index bauen und Memory registrieren → /k-docs-index
  6. status     nur diesen Bericht anzeigen

Was soll ich tun?
```

Default without `$ARGUMENTS`: show the report and wait for the user's choice. Do not start
a producer automatically.

Argument dispatch:

- `status` or empty after the report → stop read-only.
- `code` → apply `k-playbook/commands/_docs/code.md` with the remaining arguments.
- `tools` → apply `k-playbook/commands/_docs/tools.md` with the remaining arguments.
- `extract` → run the flow from `/k-docs-extract` with the remaining arguments.
- `inventory` → apply `k-playbook/commands/_docs/inventory.md`. It takes no arguments; the
  collection itself is the subcommand `k-playbook inventory`.
- `index` → run the flow from `/k-docs-index`.

When dispatching to a module or command, use the context already loaded in this session.
Do not rerun `context` unless the shared context says it is stale.

## Schritt 5 — Abschluss

For read-only status:

- Count docs per origin.
- Count consistency findings by category.
- State which actions are available and which are currently blocked by missing setup
  directories.
- Say explicitly: no files were changed.

For dispatched actions, use the dispatched module's or command's own Abschluss.

## Fehlerfälle

- Unknown argument → show valid arguments: `status`, `code`, `tools`, `extract`,
  `inventory`, `index`.
- A required module under `commands/_docs/` is missing → abort that action and report an
  incomplete or outdated k-playbook installation. Do not reconstruct the module in chat.
- `opencode.json` and `opencode.jsonc` both exist → report the ambiguity. The target rule
  still decides and `opencode.json` is the one that gets written; do not ask which one to
  use.

## Anti-Muster (nicht tun)

- **Produzenten automatisch starten.** Status ist read-only; Schreiben beginnt erst nach
  Auswahl.
- **Abläufe duplizieren.** Code- und Tool-Erzeugung stehen in `_docs/code.md` und
  `_docs/tools.md`.
- **Index nebenbei reparieren.** Dafür gibt es `/k-docs-index`.
- **Doc-Dateien im Statuslauf korrigieren.** Befunde melden, nicht still ändern.
