# Docs-Modul: Code

Dieses Modul wird von `/k-docs` und `/k-docs-code` nach dem Shared Context angewendet.
Es ist kein eigenständiger Einstieg, sondern der nachladbare Ablauf für die Herkunft
`docs/code/`.

Turn an existing codebase into a curated set of topic docs that describe **meaning**, not
surface facts — explicitly not a grep replacement. The index over these docs is built by
`/k-docs-index`; this command does not write it.

Produces:
- `k-playbook-local/docs/code/<NN>-<slug>.md` — one file per coherent topic.

Nothing else. This command writes only inside its own directory `docs/code/`.

## Schritt 1 — Pfade auflösen und Target bestätigen

From the context output:

- `RESOLVED_DOCS_DIR = <local.dir>/docs`
- `DOCS_DISPLAY_PATH = k-playbook-local/docs`
- `CODE_DIR = <RESOLVED_DOCS_DIR>/code`
- `CODE_DISPLAY_PATH = k-playbook-local/docs/code`

Use `CODE_DIR` for all doc writes.

**Target-Auflösung** — aus der Context-Ausgabe und dem Argument, nicht geraten:

- Wenn `$ARGUMENTS` gesetzt ist: es benennt das zu analysierende Verzeichnis. Existiert es nicht, abbrechen mit klarer Fehlermeldung. Liegt es außerhalb von `project.repoRoot`, ist es ein anderes Projekt — abbrechen und darauf hinweisen, dass der Command dort aufgerufen werden muss.
- Wenn `$ARGUMENTS` leer ist: das aktuelle Arbeitsverzeichnis.

Das Ergebnis wird gebunden als:

- `TARGET_DIR` — das absolute Analyseverzeichnis.
- `TARGET_DISPLAY_PATH` — derselbe Pfad relativ zu `project.dir`.

Command-specific policy:

- If `RESOLVED_DOCS_DIR` is missing on disk: ask whether to create exactly that directory
  now or to run `/k-gui`. Do not use a fallback path and do not abort hard.
- `CODE_DIR` is the directory of the origin „Code", not this command's private
  directory: the skill `ks-overlay-repo-analyse` writes its Overlay- and Base-Docs there
  too. Files from that producer are the normal case, not leftovers. Create the directory
  without asking if it is missing — before the first run that is the normal state, not a
  broken installation.
- Write nothing outside `CODE_DIR`. `docs/README.md`, `AGENTS.md` and `opencode.json`
  belong to `/k-docs-index`; `docs/libs/`, `docs/extracted/`, `docs/versions/` and
  `docs/manual/` belong to other producers and are not touched here — not even read for
  repair.

**Preflight-Snapshot anzeigen:**

```text
/k-docs-code — Preflight
─────────────────────────────────────
Ziel:          <TARGET_DIR>
Projekt:       <project.dir>
Quelle:        Argument | CWD
Git-Repo:      ja (branch: <branch>) | nein
Code-Docs:     k-playbook-local/docs/code (existiert, <N> Dateien) | fehlt
```

Wenn `$ARGUMENTS` gesetzt war: keine Rückfrage — das explizite Ziel gilt.

Wenn `$ARGUMENTS` leer war: frage:

> "Ist das das richtige Repo? (ja/nein)"

Bei „nein": abbrechen mit Hinweis:

> "Abgebrochen. In das gewünschte Repo wechseln (`cd <pfad>`) oder Ziel als Argument angeben: `/k-docs-code <pfad>`."

Bei „ja": weiter mit Schritt 2.

## Schritt 2 — Scope klären

Ask the user (bundle in one message):

1. Which top-level directories should be analyzed? (Default: everything under the project root that looks like source.)
2. Any explicit inclusions beyond source (e.g. `infra/`, `deploy/`)?

**Exclusions — apply automatically, no need to ask:**

- Default set: `.git/`, `venv/`, `.venv/`, `env/`, `node_modules/`, `dist/`, `build/`, `target/`, `_bases/`, `.next/`, `.nuxt/`, `__pycache__/`, `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/`, `coverage/`, `htmlcov/`, `.tox/`, `tests/fixtures/`, `**/*.min.js`, `**/*.lock`, `**/*.bundle.*`.
- Additionally: everything matched by `.gitignore` under `project.repoRoot` (parse the file if present; treat entries as glob patterns; respect nested `.gitignore` files as best-effort).

Announce the final effective exclusion set before scanning, in one compact list. Give the user one chance to add more.

## Schritt 3 — Semantischer Scan

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

## Schritt 4 — Thematische Struktur vorschlagen

Zeige dem User eine **kompakte** Themenliste (nicht die Doku selbst). Format:

```
Ich schlage folgende Doc-Struktur vor (unter docs/code/):

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

Nummeriert wird in Einer-Schritten mit bewussten Lücken (`00`, `01`, …, `90`) — die Nummer ist Sortier-Hilfe, keine lückenlose Zählung. Ein neues Thema kommt später in eine freie Nummer, ohne dass umsortiert wird. Der Nummernkreis gilt nur innerhalb von `CODE_DIR`; andere Herkünfte zählen eigenständig.

## Schritt 5 — Docs schreiben

Pro bestätigtem Thema eine Datei `<CODE_DIR>/<NN>-<slug>.md`. Rahmen pro Datei:

```markdown
---
type: Project Concept
title: <Titel>
description: <Ein Satz: was diese Datei erklärt.>
tags: [<kurze-tags>]
status: stable
generated: { by: k-docs-code, at: <ISO-8601-datetime> }
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
- `generated.by` nennt das schreibende Werkzeug; dieser Command trägt `k-docs-code` ein. In `docs/code/` sind die Erzeuger der Herkunft „Code" gültig: `k-docs-code`, der Altwert `k-code2docs` und `ks-overlay-repo-analyse`. Der Index prüft den Wert gegen die Herkunft des Verzeichnisses; ein Wert, der zu einer anderen Herkunft gehört, ist ein Befund.
- `description` ist ein konkreter Ein-Satz-Summary für Index/Search/Agenten. `tags` sind kurz, lowercase, domänen- oder technikbezogen; keine Keyword-Flut.
- Wenn belastbare Quellen außer Code genutzt wurden, `sources:` im Frontmatter ergänzen, jeweils als OKF-Objekt mit mindestens `resource`. Wenn ein Mensch eine Datei später fachlich bestätigt, darf `verified: { by: human:<id>, at: <ISO-8601-datetime> }` nachgetragen werden. Nicht automatisch Human-Review behaupten.
- Code-Referenzen konsequent als `pfad:zeile` — sonst kann die spätere Session nicht ohne Grep zurückspringen.
- Verwandte Themen **immer** verlinken (relative Pfade). Innerhalb von `CODE_DIR` als `./<datei>.md`, in eine andere Herkunft als `../libs/<name>.md` bzw. `../manual/<datei>.md`. Isolierte Doku-Files sind ein Bug.
- Keine erfundenen Erklärungen. Wenn eine Stelle unklar ist: **fragen, nicht raten.** Rückfragen bündeln (pro Doku-Datei ein Fragenblock).
- Keine Fließtext-Wände — Struktur mit Zwischenüberschriften, Aufzählungen, kurzen Absätzen.
- Sprache: **wie die AGENTS.md-Vorlage vorsieht** (i. d. R. Deutsch, wenn nichts anderes vereinbart).

**OKF-Kompatibilität:**

- Diese Docs bleiben normale Markdown-Dateien. Es wird **kein** OKF-`index.md` erzeugt; der Index über alle Herkünfte entsteht in `/k-docs-index`.
- Das Frontmatter folgt dem Open Knowledge Format leichtgewichtig, damit Menschen und Agenten Dateien nach Typ, Tags, Status und Erzeugungszeit sortieren können.
- Bestehende Dateien ohne Frontmatter sind nicht kaputt. Bei Aktualisierung einer solchen Datei Frontmatter nur mit Bestätigung ergänzen. Eine Datei ohne `generated.by` gilt als unbekannte Herkunft und wird wie eine fremde behandelt — also nur mit Bestätigung überschrieben.

Bei einem erneuten Lauf pro Themen-Vorschlag bestätigen lassen, ob neu / aktualisieren / überspringen. Existierende Dateien werden nicht ohne Bestätigung überschrieben. Eine Datei mit fremdem `generated.by` — etwa `ks-overlay-repo-analyse` — ist dabei kein Altbestand aus einem früheren Lauf, sondern gehört einem anderen Erzeuger derselben Herkunft: sie wird nie ohne Bestätigung überschrieben.

Nach jeder geschriebenen Datei kurz melden welche Datei geschrieben wurde (Dateiname + Zeilen-Zahl, nicht Inhalt), damit der User Fortschritt sieht.

## Schritt 6 — Abschluss

Kompakte Zusammenfassung:

- Geschriebene Doc-Dateien in `CODE_DISPLAY_PATH` (Anzahl + Gesamt-Zeilen), getrennt nach neu / aktualisiert / übersprungen.
- OKF-Frontmatter: neu / ergänzt / unverändert.
- Offene Fragen, die in den Dateien als Fragenblock stehen.
- Ausdrücklich: außerhalb von `CODE_DISPLAY_PATH` wurde nichts geschrieben.
- Folge-Command: **`/k-docs-index`** — baut aus allen Herkünften den einzigen Index `k-playbook-local/docs/README.md` und registriert die Docs in `AGENTS.md` und `opencode.json`. Ohne diesen Lauf sind die neuen Dateien nicht verlinkt und für Folge-Sessions nicht auffindbar.

## Fehlerfälle

- `$ARGUMENTS` zeigt auf ein nicht existierendes Verzeichnis → abbrechen, den Pfad nennen.
- `$ARGUMENTS` liegt außerhalb von `project.repoRoot` → abbrechen und darauf hinweisen, dass der Command im anderen Projekt aufgerufen werden muss. Keine projektübergreifende Analyse.
- `RESOLVED_DOCS_DIR` fehlt → fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen. Kein Ersatzpfad, kein harter Abbruch.
- Eine Zieldatei existiert bereits und die Bestätigung zum Überschreiben bleibt aus → Datei unverändert lassen, im Abschluss als übersprungen führen.
- Der Scan findet keine Kernlogik, nur Framework-Standard → das sagen und keine Doku erfinden.

## Anti-Muster (nicht tun)

- **Datei-für-Datei-Beschreibung.** „module_x.py enthält Klasse Y mit Methode Z" — wertlos, das steht im Code. Docs beschreiben **Bedeutung**, nicht Oberfläche.
- **Themen-Splitter.** 30 winzige Docs statt 10 sinnvolle. Wenn eine Frage über 3 Dateien verteilt ist: falsch geschnitten.
- **Framework-Standards ausformulieren.** „Wir nutzen Standard-Django-Views" gehört nicht dokumentiert.
- **Isolierte Docs.** Ohne Cross-Links wird jede Datei eine Insel. Verwandte Themen **immer** verlinken.
- **Silent overwrite.** Existierende Dateien nur mit expliziter Bestätigung anfassen.
- **Fremdes Verzeichnis beschreiben.** `docs/README.md`, `docs/libs/`, `docs/extracted/`, `docs/versions/` und `docs/manual/` haben andere Eigentümer. Wer hier hineinschreibt, zerstört beim nächsten Lauf des Eigentümers seine eigene Arbeit — oder schlimmer, fremde.
- **Den Index nebenbei nachziehen.** Der Index wird nicht „schnell noch" ergänzt; dafür läuft `/k-docs-index` über alle Herkünfte.
