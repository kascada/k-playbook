---
description: Detect the tools/libraries/stacks used in a project, rank them by "worth researching", let the user pick, then produce a curated pitfall-focused file per selected tool under k-playbook-local/docs/libs/. Focuses on pitfalls and idioms, NOT copy-paste snippets.
argument-hint: [scope-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite]
---

# k-tools-scan

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Turns a raw dependency list into a curated set of **pitfall-focused** reference docs, one
per non-trivial tool. The index over these files is built by `/k-docs-index`; this command
does not write it.

Produces:
- `k-playbook-local/docs/libs/<name>.md` — pro Tool eine Datei mit Frontmatter (`lib`, `version`, `severity`, `last-reviewed`).
- `k-playbook-local/docs/libs/README.md` — nur der kurze Erklärtext des Verzeichnisses, und nur wenn die Datei fehlt. Keine Übersichtstabelle: die steht im Index.

Nothing else. This command writes only inside its own directory `docs/libs/`.

**Fokus:** Pitfalls, Auth-Quirks, Concurrency-Fallen, Version-Migrations-Notes, empfohlene Idiome. **Kein** Copy-Paste-Tutorial — dafür gibt es die offiziellen Docs.

## Schritt 1 — Pfade auflösen

From the context output:

- `RESOLVED_DOCS_DIR = <local.dir>/docs`
- `DOCS_DISPLAY_PATH = k-playbook-local/docs`
- `LIBS_DIR = <RESOLVED_DOCS_DIR>/libs`
- `LIBS_DISPLAY_PATH = k-playbook-local/docs/libs`

Use `LIBS_DIR` for all doc writes.

Command-specific policy:

- If `RESOLVED_DOCS_DIR` is missing on disk: ask whether to create exactly that directory
  now or to run `/k-gui`. Do not use a fallback path and do not abort hard.
- `LIBS_DIR` is this command's own producer directory. Create it without asking if it is
  missing — before the first run that is the normal state, not a broken installation.
- There is no precondition on `/k-code2docs` and none on the main index. The index is
  built separately by `/k-docs-index`, so this command runs standalone.
- Write nothing outside `LIBS_DIR`. `docs/README.md`, `AGENTS.md` and `opencode.json`
  belong to `/k-docs-index`; `docs/code/`, `docs/extracted/` and `docs/manual/` belong to
  other producers and are not touched here.

## Schritt 2 — Scope klären

Ask (bundled, one message):

1. Welche Package-/Manifest-Files sollen ausgewertet werden? (Default: alle gefundenen — `pyproject.toml`, `requirements*.txt`, `setup.py`, `Pipfile*`, `package.json`, `go.mod`, `Cargo.toml`, `Gemfile`, `composer.json`, `pom.xml`, `build.gradle*`, `mix.exs`.)
2. Zusätzliche Quellen einbeziehen? (Default an: `Dockerfile*`, `docker-compose*.y{a,}ml`, `.github/workflows/*.yml`, `.gitlab-ci.yml`. Signal: welche System-Tools/Runtimes/Services sind implizit dabei.)
3. Ausschlüsse — wie in `/k-code2docs`: gleiches Default-Set + `.gitignore`.

## Schritt 3 — Erkennen (automatisch, still)

Sammle für **jedes** gefundene Paket / Tool:

- **Name** (kanonisch, lowercase).
- **Version** aus dem Manifest — plus Klassifikation: `exact` (`==1.2.3`, `1.2.3`) / `range` (`^1`, `>=1,<2`, `~1.2`) / `floating` (`*`, keine Angabe).
- **Herkunft** (welche Manifest-Datei, welche Sektion — `dependencies` / `dev` / `optional`).
- **Direkt vs. transitive** — nur Direkte gehen ins Scoring; Transitive nur listen, wenn Score-relevant.
- **Nutzung im Code** — grep nach Import-Statement (Sprache-passend: Python `import <name>|from <name>`, JS `require\(['"]<name>|from ['"]<name>`, Go `"<pfad>"` in `import`, …). Zähle:
  - `file_count` — wie viele Dateien importieren
  - `import_count` — Gesamtvorkommen
- **Doku-Signale** — Doku im Repo, die ausdrücklich das Tool erwähnt (best-effort grep im Projekt-`README.md` und in `<RESOLVED_DOCS_DIR>/code/`; nur lesend).

**Nicht** ins Detail gehen bei transitiven Paketen und bei Standard-Test/Lint/Format-Tools — die werden gleich als `C` klassifiziert.

## Schritt 4 — Score und Klassifikation

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

## Schritt 5 — Präsentation und Auswahl

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

## Schritt 6 — Re-Run-Verhalten klären

Nur wenn Dateien existieren. Bevor recherchiert wird: prüfen welche `<LIBS_DIR>/<name>.md` bereits existieren, für die jetzt eine Auswahl steht. Wenn ≥ 1 Kollision: **einmalig global** fragen (nicht pro Datei):

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

## Schritt 7 — Recherche pro Tool

Für jedes ausgewählte Tool gestaffelt (in Batches, damit der User Fortschritt sieht; ~3 pro Zwischen-Nachricht):

**Basis (immer):**
- Rolle im Projekt aus Code-Nutzung + LLM-Wissen ableiten.
- 2–3 Kernstellen im Code identifizieren (`pfad:zeile`).
- Version-Klassifikation aus Schritt 3 übernehmen.

**Klassifikation A — zusätzlich Web-Recherche (gezielt, sparsam):**
- `WebFetch` auf **offizielle Doku-Startseite** der Version.
- `WebFetch` auf **Changelog / Release-Notes** rund um die genutzte Version (major.minor).
- Eine gezielte Recherche (Web-Suche über WebFetch auf bekannte Aggregatoren wie GitHub-Issues des Repos) nach „common pitfalls / gotchas" für die Pitfall-Liste. **Keine** breite Web-Suche — gezielt an die offizielle Quelle.

**Klassifikation B — nur Basis + Frontmatter.** Pitfalls dünn, aus LLM-Wissen. Web nur, wenn beim Schreiben eine konkrete Frage aufkommt, die nicht sicher zu beantworten ist.

**Wenn Recherche eine Frage aufwirft, die aus dem Repo nicht klärbar ist:** in einem Fragen-Block am Ende der Datei markieren (`> **Offen:** ...`) — nicht raten. Der User kann später ergänzen.

**Keine breite Web-Recherche.** Timeout / begrenzt / gezielt. Ein A-Tool sollte insgesamt ≤ 3 WebFetch-Aufrufe brauchen.

## Schritt 8 — Dateien schreiben

Feste Vorlage pro Tool, `<LIBS_DIR>/<name>.md`. Nichts erfinden was nicht da ist.

```markdown
---
type: Tool Reference
title: <Name>
description: Projektrelevante Pitfalls und Idiome für <Name>.
tags: [library, <ecosystem>, <name>]
status: stable
generated: { by: k-tools-scan, at: <ISO-8601-datetime> }
lib: <name>
version: "<version-string>"
version-pin: <exact|range|floating>
severity: <high|medium|low>
last-reviewed: <YYYY-MM-DD>
sources:
  - resource: <URL offizielle Doku>
    title: <Titel der Quelle>
  - resource: <URL Changelog/Releases>
    title: <Titel der Quelle>
  - resource: <weitere Quelle, wenn genutzt>
    title: <Titel der Quelle>
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

- [../code/<NN>-<slug>.md](../code/<NN>-<slug>.md) — <in welchem Zusammenhang>
- [./<other-lib>.md](./<other-lib>.md)

## Offene Fragen

> _(nur wenn welche entstanden sind — sonst Sektion weglassen)_
```

**Regeln:**
- Das Frontmatter ist OKF-kompatibel: `type: Tool Reference` plus `title`, `description`, `tags`, `status`, `generated`. Die bestehenden Tool-Felder (`lib`, `version`, `version-pin`, `severity`, `last-reviewed`, `sources`) bleiben für `/k-tools-scan` erhalten.
- `generated.by` ist immer `k-tools-scan`. Der Index prüft das gegen das Verzeichnis; ein anderer Wert in `docs/libs/` ist ein Befund.
- `lib`, `version`, `severity` und `last-reviewed` sind Pflicht: `/k-docs-index` baut daraus die Übersichtstabelle „Libs & Stack". Fehlt eines, kommt die Datei trotzdem in den Index — die Lücke wird dort als Konsistenz-Befund gemeldet, nicht stillschweigend übergangen.
- `sources` im Frontmatter nur für tatsächlich genutzte Quellen eintragen, jeweils als OKF-Objekt mit mindestens `resource`. Offizielle Doku und Changelog/Releases bevorzugen.
- **Keine** Tutorials, keine „Getting Started"-Snippets.
- Pitfalls sind **konkret** (mit Ursache und Symptom), nicht allgemein („kann Fehler werfen").
- Wenn kein Pitfall gefunden — dann ehrliche Aussage: „Kein projektrelevanter Pitfall bekannt in dieser Version." Nicht Pitfalls erfinden.
- **Severity im Frontmatter** ist projektspezifisch: wie kritisch das Tool für dieses Projekt ist, nicht wie riskant die Lib generell.

**Erklärtext des Verzeichnisses.** Fehlt `<LIBS_DIR>/README.md`, lege sie mit genau diesem Inhalt an. Existiert sie, bleibt sie unangetastet — auch dann, wenn sie noch eine alte Übersichtstabelle enthält; die räumt `/k-docs-index` nach Bestätigung auf.

```markdown
# Libs & Stack

Kuratierte Referenz zu den nicht-trivialen Libraries und Tools dieses Projekts.
Fokus: Pitfalls und Idiome — kein Ersatz für offizielle Doku.

Erzeugt von `/k-tools-scan`. Die Übersichtstabelle steht im Index unter
[`../README.md`](../README.md).
```

## Schritt 9 — Abschluss

Kompakte Zusammenfassung:

- Erkannt: N direkte + M transitive Pakete.
- Klassifiziert: A=x, B=y, C=z.
- Ausgewählt für Recherche: k Tools.
- Geschrieben / aktualisiert in `LIBS_DISPLAY_PATH` (Liste).
- `<LIBS_DIR>/README.md`: neu angelegt / unverändert vorhanden.
- Offene Fragen: Zusammenfassung, wenn welche in den Files stehen.
- Ausdrücklich: außerhalb von `LIBS_DISPLAY_PATH` wurde nichts geschrieben.
- Hinweis: Bei größeren Upgrades später erneut `/k-tools-scan` laufen — das Re-Run-Verhalten steht in Schritt 6.
- Folge-Command: **`/k-docs-index`** — nimmt `version`, `severity` und `last-reviewed` aus den geschriebenen Dateien und baut daraus die Sektion „Libs & Stack" im einzigen Index `k-playbook-local/docs/README.md`. Ohne diesen Lauf tauchen die neuen Lib-Dateien nirgends auf.

## Fehlerfälle

- `RESOLVED_DOCS_DIR` fehlt → fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen. Kein Ersatzpfad, kein harter Abbruch.
- Kein Manifest gefunden → melden, welche Dateinamen gesucht wurden, und stoppen. Keine Dependencies aus Import-Statements erfinden.
- Alle Kandidaten fallen in Klasse C → das sagen und stoppen. Ein Lib-Doc über `click` ist keine Arbeit wert.
- `WebFetch` schlägt fehl oder liefert nichts Brauchbares → die Datei aus LLM-Wissen schreiben, die Lücke im Fragen-Block benennen und `sources` nicht mit einer nicht gelesenen URL füllen.
- Diff bestätigt der User nicht → Datei unverändert lassen, im Abschluss als übersprungen führen.

## Anti-Muster (nicht tun)

- **Copy-Paste-Snippets aus offizieller Doku.** Der Wert entsteht durch Pitfall + projektspezifischen Kontext, nicht durch Wiedergabe.
- **„Kann kaputtgehen wenn X"-Boilerplate.** Pitfalls müssen konkret sein.
- **Web-Recherche für alle Tools.** Nur bei A, gezielt.
- **Pitfalls erfinden.** Wenn nichts gefunden: leer lassen mit ehrlicher Aussage.
- **Silent overwrite.** Existierende Files nur mit Diff-Anzeige und Bestätigung ersetzen.
- **C-Liste einzeln durchgehen.** C ist per Definition „nicht gesondert dokumentiert" — außer der User pickt bewusst raus.
- **Trivial-Recherche für Framework.** Wenn ein A-Tool nur einen Standard-Absatz bekommt: entweder Recherche vertiefen oder auf B herabstufen.
- **Eine zweite Übersichtstabelle bauen.** `libs/README.md` ist ein Erklärtext, kein Index. Es gibt genau einen Index, und den schreibt `/k-docs-index`.
- **Den Haupt-Index anfassen.** `docs/README.md` gehört einem anderen Command; eine hier eingefügte Sektion ist beim nächsten `/k-docs-index` weg.
