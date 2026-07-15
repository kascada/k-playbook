---
description: Detect the tools/libraries/stacks used in a project, rank them by "worth researching", let the user pick, then produce a curated pitfall-focused file per selected tool under <docs>/libs/, plus an index. Uses paths from K-PLAYBOOK.MD. Focuses on pitfalls and idioms, NOT copy-paste snippets.
argument-hint: [scope-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite]
---

# k-tools-scan

Second step after `/k-code2docs`. Turns a raw dependency list into a curated set of **pitfall-focused** reference docs, one per non-trivial tool.

Produces:
- `<docs>/libs/<name>.md` — pro Tool eine Datei mit Frontmatter (`lib`, `version`, `severity`, `last-reviewed`).
- `<docs>/libs/README.md` — Index-Datei für Libs (Übersichtstabelle + Kurzbeschreibung).
- Ergänzt `<docs>/README.md` um eine Sektion „Libs & Stack" mit Link auf `libs/README.md`.

**Fokus:** Pitfalls, Auth-Quirks, Concurrency-Fallen, Version-Migrations-Notes, empfohlene Idiome. **Kein** Copy-Paste-Tutorial — dafür gibt es die offiziellen Docs.

## Step 1 — Resolve paths from K-PLAYBOOK.MD

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `docs:` → `DOCS_DIR`

Command-specific policy:

- If `DOCS_DIR` is missing, `-`, or `K-PLAYBOOK.MD` is missing: default to `./docs` and remind the user that `/k-setup` can register it.
- Use `RESOLVED_DOCS_DIR` for all reads and writes.

`LIBS_DIR` = `<RESOLVED_DOCS_DIR>/libs`.

If `<RESOLVED_DOCS_DIR>/README.md` does not exist or is unpopulated: warnen und dem User empfehlen, zuerst `/k-code2docs` zu laufen — dann bricht die Lib-Doku in eine leere Index-Struktur ein und wird schlechter auffindbar. Weiter nur nach ausdrücklicher Bestätigung.

## Step 2 — Scope klären

Ask (bundled, one message):

1. Welche Package-/Manifest-Files sollen ausgewertet werden? (Default: alle gefundenen — `pyproject.toml`, `requirements*.txt`, `setup.py`, `Pipfile*`, `package.json`, `go.mod`, `Cargo.toml`, `Gemfile`, `composer.json`, `pom.xml`, `build.gradle*`, `mix.exs`.)
2. Zusätzliche Quellen einbeziehen? (Default an: `Dockerfile*`, `docker-compose*.y{a,}ml`, `.github/workflows/*.yml`, `.gitlab-ci.yml`. Signal: welche System-Tools/Runtimes/Services sind implizit dabei.)
3. Ausschlüsse — wie in `/k-code2docs`: gleiches Default-Set + `.gitignore`.

## Step 3 — Detect (automatisch, still)

Sammle für **jedes** gefundene Paket / Tool:

- **Name** (kanonisch, lowercase).
- **Version** aus dem Manifest — plus Klassifikation: `exact` (`==1.2.3`, `1.2.3`) / `range` (`^1`, `>=1,<2`, `~1.2`) / `floating` (`*`, keine Angabe).
- **Herkunft** (welche Manifest-Datei, welche Sektion — `dependencies` / `dev` / `optional`).
- **Direkt vs. transitive** — nur Direkte gehen ins Scoring; Transitive nur listen, wenn Score-relevant.
- **Nutzung im Code** — grep nach Import-Statement (Sprache-passend: Python `import <name>|from <name>`, JS `require\(['"]<name>|from ['"]<name>`, Go `"<pfad>"` in `import`, …). Zähle:
  - `file_count` — wie viele Dateien importieren
  - `import_count` — Gesamtvorkommen
- **Doku-Signale** — README/Doku im Repo, das ausdrücklich das Tool erwähnt (best-effort grep im `<RESOLVED_DOCS_DIR>` und `README.md`).

**Nicht** ins Detail gehen bei transitiven Paketen und bei Standard-Test/Lint/Format-Tools — die werden gleich als `C` klassifiziert.

## Step 4 — Score & Classify

Pro Kandidat einen Score berechnen (Heuristik, transparent halten):

| Signal                                                             | Score |
|--------------------------------------------------------------------|-------|
| Framework/Fundament (formt Architektur; z. B. FastAPI, Django, React, Celery, SQLAlchemy, NestJS, Rails) | +3 |
| Externe Kopplung mit Auth/Netz/State (Cloud-SDKs, DB-Treiber, Message-Broker, Auth-Provider, ML-Runtimes) | +3 |
| Known-Pitfall-Liste (JWT, cryptography, aiohttp, pytz/tz-Handling, Kafka, LangChain, boto3, azure-sdk-*, pandas mit großen Frames, asyncio-nebenläufige Libs, ORMs, WebSocket-Libs, gRPC) | +2 |
| Breit im Code (`file_count` ≥ 20 % aller Source-Files)             | +2 |
| Version exakt gepinnt                                              | +1 |
| Test/Lint/Format ohne heavy config (pytest, mypy, ruff, black, prettier, eslint) | −2 |
| Trivial-Utility (requests ohne Auth-Sonderfall, click, python-dotenv, chalk, colorama) | −2 |
| Transitive-only                                                    | −5 (aus Auswahl raus) |

Daraus die Klassifikation:

- **A — empfohlen**: Score ≥ 3, **oder** in Known-Pitfall-Liste.
- **B — optional**: Score 1–2.
- **C — skip (default)**: Score ≤ 0.

Die Known-Pitfall-Liste ist eine **eingebettete Konstante** dieses Commands — sie darf hier ergänzt werden, aber nicht projektspezifisch ausgehebelt werden. Ergänzungen bewusst pflegen.

## Step 5 — Präsentation & User-Auswahl

**Kompakte** Tabelle. Nur A und B einzeln, C als aggregierte Zeile. Format:

```
Empfohlen (A):
  [x] fastapi        0.115.0   in 47 Files, exakt gepinnt
      → Framework, Middleware-Reihenfolge & Async-Fallen
  [x] sqlalchemy     2.0.35    in 28 Files, exakt gepinnt
      → 1.x → 2.0 Session-Semantik, Async-Unterschiede
  [x] boto3          1.35.x    in 12 Files
      → Auth-Chains, Region-Handling, Retries
  [x] cryptography   43.0.0    in 4 Files, exakt gepinnt
      → Cipher-/Padding-Fallen, key rotation

Optional (B):
  [ ] pydantic       2.9.2     in 34 Files
  [ ] httpx          0.27.0    in 8 Files, mit Auth
  [ ] structlog      24.4.0    in wenigen Files, aber projektweite Config

Übersprungen (C): pytest, ruff, black, mypy, click, python-dotenv, colorama,
  isort, pre-commit, coverage, hypothesis, requests (18 Pakete) — auf Wunsch
  ausklappen.

Kommandos:
  all           alle A+B einbeziehen
  A-only        nur A (Default)
  add <name>    zusätzliche Kandidaten aus C aufnehmen
  remove <name> aus Auswahl streichen
  show C        C-Liste vollständig zeigen
  proceed       mit aktueller Auswahl weiter
```

Kein Ping-Pong pro Tool — der User macht seine Auswahl in einer Nachricht, dann weiter.

## Step 6 — Re-Run-Verhalten klären (falls Dateien existieren)

Bevor recherchiert wird: prüfen welche `<LIBS_DIR>/<name>.md` bereits existieren, für die jetzt eine Auswahl steht. Wenn ≥ 1 Kollision: **einmalig global** fragen (nicht pro Datei):

```
Für <N> ausgewählte Tools existiert bereits eine Doku-Datei:
  - fastapi.md         (last-reviewed: 2026-03-01)
  - sqlalchemy.md      (last-reviewed: 2026-05-20)
  ...

Wie vorgehen?
  (a) alle aktualisieren — neue Recherche, bestehende Datei ersetzen (mit Diff-Anzeige vor Speichern)
  (b) alle überspringen — nur neue Tools recherchieren
  (c) einzeln fragen  — pro Datei nachfragen
  (d) abbrechen
```

Bei `(a)` und `(c)`: **immer vor dem Überschreiben Diff zeigen**, User bestätigt Übernahme.

## Step 7 — Recherche pro Tool (gestaffelt)

Für jedes ausgewählte Tool (in Batches, damit der User Fortschritt sieht; ~3 pro Zwischen-Nachricht):

**Basis (immer):**
- Rolle im Projekt aus Code-Nutzung + LLM-Wissen ableiten.
- 2–3 Kernstellen im Code identifizieren (`pfad:zeile`).
- Version-Klassifikation aus Step 3 übernehmen.

**Klassifikation A — zusätzlich Web-Recherche (gezielt, sparsam):**
- `WebFetch` auf **offizielle Doku-Startseite** der Version.
- `WebFetch` auf **Changelog / Release-Notes** rund um die genutzte Version (major.minor).
- Eine gezielte Recherche (Web-Suche über WebFetch auf bekannte Aggregatoren wie GitHub-Issues des Repos) nach „common pitfalls / gotchas" für die Pitfall-Liste. **Keine** breite Web-Suche — gezielt an die offizielle Quelle.

**Klassifikation B — nur Basis + Frontmatter.** Pitfalls dünn, aus LLM-Wissen. Web nur, wenn beim Schreiben eine konkrete Frage aufkommt, die nicht sicher zu beantworten ist.

**Wenn Recherche eine Frage aufwirft, die aus dem Repo nicht klärbar ist:** in einem Fragen-Block am Ende der Datei markieren (`> **Offen:** ...`) — nicht raten. Der User kann später ergänzen.

**Keine breite Web-Recherche.** Timeout / begrenzt / gezielt. Ein A-Tool sollte insgesamt ≤ 3 WebFetch-Aufrufe brauchen.

## Step 8 — Datei-Struktur pro Tool

Feste Vorlage. Nichts erfinden was nicht da ist.

```markdown
---
lib: <name>
version: "<version-string>"
version-pin: <exact|range|floating>
severity: <high|medium|low>
last-reviewed: <YYYY-MM-DD>
sources:
  - <URL offizielle Doku>
  - <URL Changelog/Releases>
  - <weitere, wenn genutzt>
---

# <Name>

## Rolle im Projekt

<1–2 Absätze: wofür genutzt, wie zentral. `pfad:zeile`-Verweise auf 2–3 Kernstellen.>

## Version & Migration

- Aktuell: `<version>` (`<pin-typ>`)
- Bekannte Breaking-Changes seit ...: <knapp, mit Version-Angabe>
- Empfohlener Upgrade-Pfad (nur wenn relevant für dieses Projekt): ...

## Pitfalls (projektrelevant)

- **<Kurztitel>** — <1–3 Sätze>. Ref: `<pfad:zeile>` oder Upstream-Issue-URL.
- ...

## Empfohlene Idiome

- <kurz>
- ...

## Anti-Patterns

- <kurz>
- ...

## Verwandte Docs

- [../<NN>-<slug>.md](../<NN>-<slug>.md) — <in welchem Zusammenhang>
- [./<other-lib>.md](./<other-lib>.md)

## Offene Fragen

> _(nur wenn welche entstanden sind — sonst Sektion weglassen)_
```

**Regeln:**
- **Keine** Tutorials, keine „Getting Started"-Snippets.
- Pitfalls sind **konkret** (mit Ursache und Symptom), nicht allgemein („kann Fehler werfen").
- Wenn kein Pitfall gefunden — dann ehrliche Aussage: „Kein projektrelevanter Pitfall bekannt in dieser Version." Nicht Pitfalls erfinden.
- **Severity im Frontmatter** ist projektspezifisch: wie kritisch das Tool für dieses Projekt ist, nicht wie riskant die Lib generell.

## Step 9 — `libs/README.md` bauen

Nach Fertigstellung aller Lib-Files:

```markdown
# Libs & Stack

Kuratierte Referenz zu den nicht-trivialen Libraries und Tools dieses Projekts.
Fokus: Pitfalls und Idiome — kein Ersatz für offizielle Doku.

Erzeugt / aktualisiert von `/k-tools-scan`.

## Übersicht

| Lib | Version | Severity | Letzter Review |
|-----|---------|----------|----------------|
| [fastapi](fastapi.md) | 0.115.0 | high | 2026-07-12 |
| [sqlalchemy](sqlalchemy.md) | 2.0.35 | high | 2026-07-12 |
| ... | ... | ... | ... |

## Nicht dokumentiert (bewusste Auswahl)

Standard-Test/Lint/Format sowie Trivial-Utilities werden nicht gesondert
dokumentiert. Falls doch ein Pitfall auftaucht: Datei per `/k-tools-scan add`
nachziehen.

Übersprungen bei letztem Lauf (Ausschnitt): pytest, ruff, black, mypy,
click, python-dotenv, colorama, isort, pre-commit, coverage, hypothesis,
requests.
```

## Step 10 — `<RESOLVED_DOCS_DIR>/README.md` verlinken

Im Haupt-Index eine Sektion **„Libs & Stack"** anlegen oder aktualisieren:

```markdown
## Libs & Stack

Kuratierte Referenz zu Libraries und Tools. Fokus: Pitfalls, nicht Tutorials.

→ [libs/README.md](libs/README.md)
```

Und im Stichwort-Index Einträge für die Lib-Namen ergänzen (`fastapi`, `sqlalchemy`, …) → `libs/<name>.md`.

**Beim Ändern des Haupt-Index:** wie überall im Playbook — Bestehendes nicht schweigend überschreiben. Ergänze punktuell und zeige dem User was hinzukommt / sich ändert.

## Step 11 — Abschluss

Kompakte Zusammenfassung:

- Erkannt: N direkte + M transitive Pakete.
- Klassifiziert: A=x, B=y, C=z.
- Ausgewählt für Recherche: k Tools.
- Geschrieben / aktualisiert: `<LIBS_DIR>/` (Liste).
- `<LIBS_DIR>/README.md` (neu / aktualisiert).
- `<RESOLVED_DOCS_DIR>/README.md` — Sektion „Libs & Stack" (neu / aktualisiert), Stichwort-Index um x Einträge ergänzt.
- Offene Fragen: Zusammenfassung, wenn welche in den Files stehen.
- Hinweis: Bei größeren Upgrades später erneut `/k-tools-scan` laufen — Re-Run-Verhalten ist in Step 6 dokumentiert.

## Anti-Muster (nicht tun)

- **Copy-Paste-Snippets aus offizieller Doku.** Der Wert entsteht durch Pitfall + projektspezifischen Kontext, nicht durch Wiedergabe.
- **„Kann kaputtgehen wenn X"-Boilerplate.** Pitfalls müssen konkret sein.
- **Web-Recherche für alle Tools.** Nur bei A, gezielt.
- **Pitfalls erfinden.** Wenn nichts gefunden: leer lassen mit ehrlicher Aussage.
- **Silent overwrite.** Existierende Files nur mit Diff-Anzeige und Bestätigung ersetzen.
- **C-Liste einzeln durchgehen.** C ist per Definition „nicht gesondert dokumentiert" — außer der User pickt bewusst raus.
- **Trivial-Recherche für Framework.** Wenn ein A-Tool nur einen Standard-Absatz bekommt: entweder Recherche vertiefen oder auf B herabstufen.
