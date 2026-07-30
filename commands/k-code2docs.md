---
description: Initial code-to-docs analysis. Scans a project semantically (by meaning/subsystem, not file-by-file), proposes a thematic doc structure, writes numbered topic docs plus an index README, and registers everything in MEMORY (AGENTS.md + opencode.json) so future AI sessions consult the docs first. Defaults to the current directory, or uses [target-dir] if given. Uses k-playbook/docs.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-code2docs

Turn an existing codebase into a curated, indexed documentation set that the AI can consult in ≤2 lookups. Explicitly **not** a grep replacement — the docs describe **meaning**, not surface facts.

`/k-code2docs` does not guess project paths. The project must have `K-PLAYBOOK.yaml`. The docs directory is always `<project>/k-playbook/docs`.

Produces:
- `k-playbook/docs/<NN>-<slug>.md` — one file per coherent topic.
- `k-playbook/docs/README.md` — TOC + alphabetical keyword index + question→file mapping.
- `AGENTS.md` at project root — session-injected pointer to the configured docs path.
- `opencode.json` at project root — registers `AGENTS.md` + `./k-playbook/docs`.

## Step 0 — Target bestimmen und bestätigen

Bestimme zuerst das Projekt, in dem gearbeitet wird. Alle späteren Pfade sind relativ zu `TARGET_DIR`, nicht zwingend zum aktuellen Arbeitsverzeichnis. Nutze dafür die `TARGET_DIR`-Regeln aus `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

**Target-Auflösung:**

- Wenn `$ARGUMENTS` gesetzt ist: als Zielverzeichnis behandeln.
  - Existiert das Verzeichnis: `TARGET_DIR = realpath($ARGUMENTS)`.
  - Existiert es nicht: abbrechen mit klarer Fehlermeldung.
- Wenn `$ARGUMENTS` leer ist: `TARGET_DIR = realpath(CWD)`.
- Danach vor dem Snapshot den fixed-layout guard aus `path-resolution.md` anwenden: Wenn `TARGET_DIR` versehentlich `<project>/k-playbook` ist, auf den Parent als Projekt-Root korrigieren und diese Korrektur im Preflight anzeigen.

**Preflight-Snapshot anzeigen:**

Für den Snapshot `K-PLAYBOOK.yaml` in `TARGET_DIR` lesen, um `layout`, `k_playbook.repo` und Setup-Datum kompakt anzeigen zu können. Wenn die Datei fehlt, abbrechen und `/k-setup` aufrufen lassen.

```text
/k-code2docs — Preflight
─────────────────────────────────────
Ziel:          <absolute TARGET_DIR>
Quelle:        Argument | CWD
K-PLAYBOOK.yaml: gefunden (layout: fixed-project-k-playbook) | fehlt
Git-Repo:      ja (branch: <branch>) | nein
Doc-Dir:       <DOCS_DIR> (existiert, <N> Dateien) | fehlt
```

Wenn `$ARGUMENTS` gesetzt war: keine Rückfrage — das explizite Ziel gilt.

Wenn `$ARGUMENTS` leer war: frage:

> "Ist das das richtige Repo? (ja/nein)"

Bei „nein": abbrechen mit Hinweis:

> "Abgebrochen. In das gewünschte Repo wechseln (`cd <pfad>`) oder Ziel als Argument angeben: `/k-code2docs <pfad>`."

Bei „ja": weiter mit Step 1.

## Step 1 — Resolve paths from K-PLAYBOOK.yaml

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve the fixed `docs` path:

- `RESOLVED_DOCS_DIR = <TARGET_DIR>/k-playbook/docs`.
- `DOCS_DISPLAY_PATH = k-playbook/docs`.

Command-specific policy:

- If `K-PLAYBOOK.yaml` is missing: abort and tell the user to add the project with the k-playbook Installer first.
- If `k-playbook/docs` is missing on disk: abort and tell the user to open the k-playbook Installer and use `Vervollstaendigen` in the project block.

`AGENTS_FILE` = `<TARGET_DIR>/AGENTS.md` and `OPENCODE_CONFIG` = `<TARGET_DIR>/opencode.json` (or `.jsonc` if that variant already exists — do not create both).

Use `RESOLVED_DOCS_DIR` for all doc reads and writes.

Derived paths for Memory registration:

- `DOCS_DISPLAY_PATH` = `k-playbook/docs`.
- `DOCS_README_FROM_AGENTS` = `k-playbook/docs/README.md`.
- `AGENTS_LINK_FROM_DOCS_README` = `../../AGENTS.md`.
- `DOCS_REFERENCE_PATH` = `./k-playbook/docs`.

## Step 2 — Clarify scope

Ask the user (bundle in one message):

1. Which top-level directories should be analyzed? (Default: everything under the project root that looks like source.)
2. Any explicit inclusions beyond source (e.g. `infra/`, `deploy/`)?

**Exclusions — apply automatically, no need to ask:**

- Default set: `.git/`, `venv/`, `.venv/`, `env/`, `node_modules/`, `dist/`, `build/`, `target/`, `_bases/`, `.next/`, `.nuxt/`, `__pycache__/`, `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/`, `coverage/`, `htmlcov/`, `.tox/`, `tests/fixtures/`, `**/*.min.js`, `**/*.lock`, `**/*.bundle.*`.
- Additionally: everything matched by `.gitignore` under `TARGET_DIR` (parse the file if present; treat entries as glob patterns; respect nested `.gitignore` files as best-effort).

Announce the final effective exclusion set before scanning, in one compact list. Give the user one chance to add more.

## Step 3 — Semantic scan

**Explicit rule of engagement:** do **not** produce a file-by-file dump. The output of this phase is an internal understanding organized by **meaning**, not by directory layout.

Look for:

- **Fachliche Subsysteme.** Not „welche Ordner gibt es", sondern welche eigenständigen Verantwortungsbereiche (z. B. „Auftragsannahme", „Abrechnung", „Routing"). Cluster über Datenmodelle, Aufrufgraphen, geteilte Vokabulare.
- **Cross-Cutting-Concerns.** Auth, Config/Secrets, Logging, Error-Handling, Persistence-Layer, HTTP-Surface, Background-Jobs, Feature-Flags, Retries/Timeouts.
- **Einstiegspunkte.** `main`/`app`/`cli`/`server`/Route-Registrierung/Entry-Scripts. Was startet was?
- **Externe Kopplungen.** DBs, Message-Queues, externe APIs, Auth-Provider, Cloud-Dienste. Wo sitzen sie im Code, wo werden sie konfiguriert?
- **Datenmodell.** Welche zentralen Entitäten? Wo definiert (ORM, Protobuf, Schema, TypedDict)? Wo transformiert?
- **Was ist Kernlogik, was ist Framework-Standard, was ist Boilerplate.** Nur Kernlogik verdient tiefe Dokumentation.
- **Was ist Legacy / Deprecated / TODO.** Wenn im Code sichtbar (Kommentare, deprecated-Marker, alte Muster), separat notieren.

**Nicht** aufnehmen:

- Framework-typische Standardstruktur ohne Eigenanteil („hier ist ein Django-Model, wie alle Django-Models").
- Kommentare, die den Code paraphrasieren.
- Detail-Signaturen einzelner Funktionen — die stehen im Code.

**Bei sehr großen Repos:** in Sub-Agents parallelisieren (einer pro Top-Level-Subsystem-Kandidat), Ergebnisse mergen. Für jeden Sub-Agent explizite Ausschlüsse + „nur Bedeutung, keine Zeilenlisten"-Regel mitgeben.

## Step 4 — Thematische Struktur vorschlagen → User bestätigt

Zeige dem User eine **kompakte** Themenliste (nicht die Doku selbst). Format:

```
Ich schlage folgende Doc-Struktur vor:

  00-overview            Was macht das System, für wen, grober Ablauf
  01-stack               Sprachen, Frameworks, DBs, Cloud-Dienste
  02-architektur         Komponenten und wie sie zusammenspielen
  03-datenmodell         Kern-Entitäten und ihre Beziehungen
  04-config-und-secrets  Wo/wie konfiguriert, welche Env-Vars
  05-authentifizierung   Login, Sessions, Tokens
  06-<subsystem-1>       ...
  07-<subsystem-2>       ...
  ...
  90-bekannte-macken     Bugs, Deprecations, TODOs die im Code sichtbar sind

Bemerkungen:
  - Aus dem Scan: <2-3 relevante Beobachtungen, die den Vorschlag begründen>
  - Nicht klar: <1-2 Punkte, wo Klärung sinnvoll wäre>

OK / zusammenlegen / trennen / streichen / umbenennen?
```

Warte auf Bestätigung. **Nichts schreiben bevor die Struktur bestätigt ist.**

Behalte die Zwanziger-Schritte (`00`, `01`, …) als Sortier-Hilfe. Große Themen dürfen später zwischen Zehner-Blöcken ergänzt werden ohne Umsortierung.

## Step 5 — Docs schreiben

Pro bestätigtem Thema eine Datei `<RESOLVED_DOCS_DIR>/<NN>-<slug>.md`. Rahmen pro Datei:

```markdown
---
type: Project Concept
title: <Titel>
description: <Ein Satz: was diese Datei erklaert.>
tags: [<kurze-tags>]
status: stable
generated: { by: k-code2docs, at: <ISO-8601-datetime> }
---

# <Titel>

<1-2 Sätze: worum es hier geht, aus welcher Perspektive.>

## Wozu

<Der fachliche/technische Zweck. Warum existiert diese Komponente/dieses Konzept überhaupt.>

## Wie es funktioniert

<Die Kern-Idee, keine Zeile-für-Zeile-Beschreibung. Datenfluss, Abhängigkeiten,
Aufruf-Pfad. Verweise auf konkrete Stellen im Code als `path/to/file.py:123`.>

## Wichtige Stellen im Code

- `path/to/file.py:45` — <was dort steht, in einem Satz>
- ...

## Randfälle / Was zu wissen ist

<Nicht-offensichtliche Verhaltensweisen, versteckte Verträge, dokumentierte Bugs.>

## Verwandte Themen

- [<datei>](./<datei>.md) — <in welchem Zusammenhang>
```

**Regeln:**

- Jede Themen-Datei bekommt OKF-kompatibles YAML-Frontmatter: `type`, `title`, `description`, `tags`, `status`, `generated`. `type` ist typischerweise `Project Concept`; bei passenderem Inhalt sind auch sprechende Typen wie `Architecture`, `Data Model`, `API Surface`, `Runtime Configuration`, `Operational Playbook` erlaubt. Keine zentrale Typ-Liste erfinden.
- `description` ist ein konkreter Ein-Satz-Summary fuer Index/Search/Agenten. `tags` sind kurz, lowercase, domänen- oder technikbezogen; keine Keyword-Flut.
- Wenn belastbare Quellen ausser Code genutzt wurden, `sources:` im Frontmatter ergaenzen, jeweils als OKF-Objekt mit mindestens `resource`. Wenn ein Mensch eine Datei spaeter fachlich bestaetigt, darf `verified: { by: human:<id>, at: <ISO-8601-datetime> }` nachgetragen werden. Nicht automatisch Human-Review behaupten.
- Code-Referenzen konsequent als `pfad:zeile` — sonst kann die spätere Session nicht ohne Grep zurückspringen.
- Verwandte Themen **immer** verlinken (relative Pfade). Isolierte Doku-Files sind ein Bug.
- Keine erfundenen Erklärungen. Wenn eine Stelle unklar ist: **fragen, nicht raten.** Rückfragen bündeln (pro Doku-Datei ein Fragenblock).
- Keine Fließtext-Wände — Struktur mit Zwischenüberschriften, Aufzählungen, kurzen Absätzen.
- Sprache: **wie die AGENTS.md-Vorlage vorsieht** (i. d. R. Deutsch, wenn nichts anderes vereinbart).

**OKF-Kompatibilitaet:**

- Diese Docs bleiben normale Markdown-Dateien mit `README.md` als Hauptindex. Es wird **kein** OKF-`index.md` als Ersatz fuer `README.md` erzeugt.
- Das Frontmatter folgt dem Open Knowledge Format leichtgewichtig, damit Menschen und Agenten Dateien nach Typ, Tags, Status und Erzeugungszeit sortieren koennen.
- Bestehende Dateien ohne Frontmatter sind nicht kaputt. Bei Aktualisierung einer solchen Datei Frontmatter nur mit Bestätigung ergaenzen.

Nach jeder geschriebenen Datei kurz melden welche Datei geschrieben wurde (Dateiname + Zeilen-Zahl, nicht Inhalt), damit der User Fortschritt sieht.

## Step 6 — `README.md` als Index bauen

`<RESOLVED_DOCS_DIR>/README.md` mit vier Blöcken:

```markdown
# <Projektname> — Dokumentation

<Ein Absatz: was das Projekt ist, was in diesen Docs steht.>

> **Für AI-Sessions:** Diese Docs sind **autoritativ**. Nutze sie zuerst,
> bevor du Code liest. Siehe [`AGENTS.md`](<AGENTS_LINK_FROM_DOCS_README>) im Projekt-Root.

## Übersicht der Dokumente

| Datei | Inhalt |
|-------|--------|
| [`00-overview.md`](00-overview.md) | ... |
| ... | ... |

## Stichwort-Index

Alphabetisch. Format: **Begriff** → `datei.md` §Abschnitt.

### A
- **<Begriff>** → `<datei>.md` §<abschnitt>

### B
...

## Häufige Fragen → direkter Sprung

| Frage | Datei |
|-------|-------|
| Was macht das System insgesamt? | `00-overview.md` |
| Welchen Stack nutzt es? | `01-stack.md` |
| Wo werden Secrets verwaltet? | `04-config-und-secrets.md` |
| Wie läuft die Authentifizierung? | `05-authentifizierung.md` |
| ... | ... |
```

**Regeln für den Stichwort-Index:**

- Aufnehmen: **Domänenbegriffe, Fach-Vokabular, Env-Var-Namen, Feature-Namen, Bug-Namen, externe Systeme, zentrale Klassen/Modul-Rollen.**
- **Nicht** aufnehmen: generische Programmier-Wörter („Klasse", „Function", „loop"). Der Index ist **kein Grep**.
- Jeder Eintrag muss auf einen konkreten Abschnitt einer konkreten Datei zeigen (nicht nur „`api.md`").
- Beim Aufbau: für jede Doc-Datei die 3–10 Kernbegriffe extrahieren, dann alphabetisch mergen.

**Regeln für „Häufige Fragen":**

- Jede Frage ist eine, die ein neuer Entwickler oder eine neue AI-Session realistisch in Woche 1 stellt.
- Antwort ist **eine** Datei. Wenn die Antwort auf 3 Dateien verteilt ist, ist die Doku falsch geschnitten — dann strukturell nachbessern.

## Step 7 — Verifikation

Interner Selbsttest, sichtbar für den User:

1. Wähle **3 Konzepte aus dem Code, die nicht direkt aus einem Doc-Titel folgen** (z. B. eine spezielle Env-Var, ein interner Job-Name, ein bestimmter API-Endpunkt).
2. Für jedes: versuche über `<RESOLVED_DOCS_DIR>/README.md` (TOC + Index + Q→Datei) in ≤2 Schritten zur Antwort zu kommen.
3. Wenn eines der drei fehlschlägt: Index/Themen nachbessern und Test wiederholen.

Ergebnis dem User zeigen. Erst weiter, wenn alle drei durchkommen.

## Step 8 — MEMORY registrieren

Der Kern dieses Schrittes: die entstandenen Docs sind wertlos, wenn Folge-Sessions sie nicht automatisch als autoritativ behandeln.

**8a — `AGENTS.md`:**

- Existiert nicht → aus `<PLAYBOOK_REPO>/ks-ai-session-memory/vorlagen/AGENTS.md.template` erzeugen und Platzhalter füllen (`<Projektname>`, „Was ist dieses Projekt?" aus `00-overview.md` ableiten, Themenbereiche aus der geschriebenen Doc-Struktur füllen, Kurzverweis-Tabelle aus dem README-„Häufige Fragen"-Block spiegeln). Ersetze dabei alle template-seitigen `docs/`-Beispiele durch `DOCS_DISPLAY_PATH` bzw. `DOCS_README_FROM_AGENTS`; keine hart kodierten `docs/README.md`-Verweise stehen lassen. Erwaehne knapp, dass die Doc-Dateien normales Markdown mit OKF-kompatiblem YAML-Frontmatter sind; `README.md` bleibt der Einstieg.
- Existiert → prüfen ob folgende Punkte enthalten sind: „Docs zuerst", Verweis auf `DOCS_README_FROM_AGENTS`, Ausnahmen-Regel. Fehlende oder auf einen alten Docs-Pfad zeigende Punkte **mit Bestätigung** einfügen/korrigieren. Rest unangetastet lassen.

**8b — `opencode.json` (oder `.jsonc` falls schon vorhanden):**

- Existiert nicht → aus `<PLAYBOOK_REPO>/ks-ai-session-memory/vorlagen/opencode.json.template` erzeugen. `references.docs.path` auf `DOCS_REFERENCE_PATH` setzen, nicht auf den Template-Default `./docs`. `description` **konkret** befüllen: Projektname + Liste der wichtigsten Themen aus der Doc-Struktur + Hinweis auf `DOCS_README_FROM_AGENTS` als Index (nicht die Template-Platzhalter stehen lassen).
- Existiert → prüfen ob `instructions` `AGENTS.md` enthält, `references.docs.path` nach Auflösung relativ zur Config-Datei auf `RESOLVED_DOCS_DIR` zeigt, und die `description` konkret ist. Fehlendes ergänzen, falsche/alte Docs-Pfade korrigieren, konkret machen — mit Bestätigung.

**8c — Restart-Hinweis:**

Explizit dem User sagen:

> OpenCode liest die Konfig einmal beim Start. Damit die neue Session-Memory greift, bitte OpenCode beenden (`/exit` oder Ctrl+C) und neu starten.

## Step 9 — Abschluss

Kompakte Zusammenfassung:

- Geschriebene Doc-Dateien (Anzahl + Gesamt-Zeilen).
- OKF-Frontmatter: neu / ergänzt / unverändert.
- Anzahl Stichwort-Einträge im Index.
- Anzahl Q→Datei-Einträge.
- MEMORY: `AGENTS.md` (neu / ergänzt / unverändert), `opencode.json` (neu / ergänzt / unverändert).
- Restart-Hinweis.
- Nächster Schritt: „`/k-setup` erneut aufrufen, um zu bestätigen dass alles registriert ist (optional)."
- Folge-Command: **`/k-tools-scan`** — erzeugt `k-playbook/docs/libs/` mit einer pitfall-fokussierten Datei je nicht-trivialer Library. Empfohlen als zweiter Schritt nach diesem Command.

## Wartungs-Hinweise (dem User beim ersten Lauf einmal zeigen)

Damit das Setup wirkt bleibt:

- Neue Doc-Datei → sofort TOC + Stichwort-Index in `README.md` nachziehen.
- Wenn Code der Doku widerspricht → Doku updaten (nicht schweigend im Code weiterarbeiten).
- Der Command darf jederzeit **erneut** aufgerufen werden — bestätige dann pro Themen-Vorschlag ob neu / aktualisieren / überspringen. Existierende Doc-Dateien werden nicht ohne Bestätigung überschrieben.

## Anti-Muster (nicht tun)

- **Datei-für-Datei-Beschreibung.** „module_x.py enthält Klasse Y mit Methode Z" — wertlos, das steht im Code. Docs beschreiben **Bedeutung**, nicht Oberfläche.
- **Grep-Ersatz-Index.** Jedes Wort aus dem Code als Stichwort → verwässert den Index unbrauchbar. Nur **Fachbegriffe** aufnehmen.
- **Themen-Splitter.** 30 winzige Docs statt 10 sinnvolle. Wenn eine Frage über 3 Dateien verteilt ist: falsch geschnitten.
- **Framework-Standards ausformulieren.** „Wir nutzen Standard-Django-Views" gehört nicht dokumentiert.
- **Isolierte Docs.** Ohne Cross-Links wird jede Datei eine Insel. Verwandte Themen **immer** verlinken.
- **Silent overwrite.** Existierende Dateien nur mit expliziter Bestätigung anfassen.
- **Templates un-ausgefüllt schreiben.** `<Projektname>` und `<konkrete Themen>` im finalen `opencode.json` oder `AGENTS.md` sind ein Fehler.
