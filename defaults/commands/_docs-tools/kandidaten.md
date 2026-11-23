---
title: Kandidaten und Auswahl für die Steckbriefe
description: Modul für /k-docs-tools, vom Hauptkontext gelesen im vierten Schritt — die Kandidaten aus dem Inventar bilden (Art, Versionen, Pin, Scope), die Nutzung im Code zählen (Go importgenau), Score und Klasse A, B, C, die Known-Pitfall-Liste, die kompakte Liste mit den Kommandos der Auswahl und die Regel, nichts zu schreiben, bevor die Auswahl bestätigt ist.
---

**Kandidaten sind alle Gegenstände des Inventars.** Lies dazu den Rumpf des Inventars, die
Abschnitte je Umgebung (`## lokal`, `## dev`, `## devcontainer`, `## ci`, `## deployment`,
ohne Einträge `## Einträge`) mit den Spalten `Gegenstand`, `Art`, `Version`, `Pin`, `Scope`
und `Herkunft`. Ein Gegenstand (`<ökosystem>/<name>`, etwa `python/celery` oder
`go/github.com/yuin/goldmark`) steht dort je Quelle und Umgebung in einer eigenen Zeile, auch
mit verschiedenen Versionen. Als Kandidat fasst du ihn über seine Zeilen zusammen:

- **Art** — die Werte aus `Art` (`package`, `tool`, `runtime`, `image`, `chart`,
  `chart-dependency`, `action`).
- **Versionen** — die verschiedenen Werte aus `Version`, leere ausgelassen.
- **Pin** — die verschiedenen Werte aus `Pin`, nach der Taxonomie des Vertrags.
- **Scope** — die verschiedenen Werte aus `Scope` (`main`, `dev`, `optional`, `test`,
  `build`), dazu die Umgebungen, in denen er vorkommt.

Das Inventar führt die Angaben der Manifeste; aus Lockfiles kommen nur die direkten
Abhängigkeiten, transitive Pakete werden nicht ergänzt. Indirekte Angaben bringt nur
`go.mod` mit: Es nennt auch indirekte Go-Module (`// indirect`), und das Inventar führt sie
als gekennzeichnete Zeilen. Erkennungsmerkmal ist die Spalte `Herkunft`: Sie enthält
`indirekt (// indirect in go.mod)`. Abweichungen zwischen den Zeilen sind kein Fehler
dieses Commands; das Inventar weist sie selbst aus.

**Nutzung im Code** zählst du je Kandidat mit Grep, innerhalb des Scope aus dem Preflight (`SCAN_ROOT` samt Zusatzpfad und Ausschlüssen) —
der Scope begrenzt nur dieses Zählen, nicht die Liste. Gesucht wird nach dem Import, passend
zur Sprache (Python `import <name>|from <name>`, JavaScript `require\(['"]<name>|from
['"]<name>`, Go nach der Regel unten, …):

- `file_count` — wie viele Dateien importieren,
- `import_count` — Vorkommen insgesamt.

**Go** zählst du importgenau, ohne `go` vorauszusetzen:

- **Importzeile.** Gezählt werden nur Zeilen, die aus nichts bestehen als einem optionalen
  `import`, einem optionalen Alias (`_`, `.` oder Bezeichner), dem Pfad in Anführungszeichen
  und einem optionalen Kommentar — die Einzelform `import "<p>"` und die Zeilen eines Blocks
  `import ( … )`. Eine Zeichenkette sonst im Code zählt nicht. Grenze: Ob die Zeile in einem
  Importblock steht, prüft Grep nicht; eine Zeile dieser Form außerhalb, etwa eine einzelne
  Zeichenkette in einem mehrzeiligen Literal, zählt mit. Das ist hingenommen.
- **Zugehörigkeit.** Ein Importpfad gehört zu einem Modul, wenn er gleich dem Modulpfad ist
  oder mit `<modulpfad>/` beginnt. So zählen Unterpakete mit (`…/go-sdk/mcp` für
  `github.com/modelcontextprotocol/go-sdk`), und ein Modul zählt nicht für ein anderes, das
  nur mit denselben Zeichen beginnt (`github.com/yuin/goldmark` nicht für
  `github.com/yuin/goldmark-emoji`).
- **Muster.** `<modulpfad>` ist der Gegenstand ohne `go/`, Punkte als `\.` maskiert:

  ```text
  ^[[:space:]]*(import[[:space:]]+)?([A-Za-z_.][A-Za-z0-9_]*[[:space:]]+)?"<modulpfad>(/[^"]*)?"[[:space:]]*(//.*)?$
  ```

  `grep -rHE --include='*.go' '<muster>' <scope>` liefert die Treffer als `datei:zeile`;
  `import_count` ist die Zahl der Treffer, `file_count` die Zahl der verschiedenen Dateien
  davor.
- **Verschachtelte Module.** Beginnt der Pfad eines anderen Go-Kandidaten mit
  `<modulpfad>/` (`cloud.google.com/go/storage` unter `cloud.google.com/go`), zählt eine
  Importzeile nur für das längste passende Modul. Für das kürzere ziehst du deshalb vor dem
  Zählen die Treffer ab, die auch das Muster des längeren trifft — je längerem Modul ein
  `grep -vE '"<längerer modulpfad>(/[^"]*)?"'` hinter dem Grep des Musters. Eine Datei, von der danach
  kein Treffer bleibt, zählt nicht für das kürzere.

Bei Gegenständen ohne Import — `image`, `chart`, `chart-dependency`, `action`, `runtime` —
entfällt die Zählung; die Liste zeigt dort `—`.

**Score und Klasse.** Je Kandidat ein Score, transparent gehalten:

| Signal | Score |
|---|---|
| Framework oder Fundament, formt die Architektur (etwa FastAPI, Django, React, Celery, SQLAlchemy, NestJS, Rails) | +3 |
| Externe Kopplung mit Auth, Netz oder Zustand (Cloud-SDKs, DB-Treiber, Message-Broker, Auth-Anbieter, ML-Runtimes) | +3 |
| Steht auf der Known-Pitfall-Liste (unten) | +2 |
| Breit im Code (`file_count` ≥ 20 % aller Quelldateien im Scope) | +2 |
| Pin in allen Zeilen `exact` oder `digest` | +1 |
| Test, Lint oder Format ohne aufwendige Konfiguration (pytest, mypy, ruff, black, prettier, eslint) | −2 |
| Triviales Hilfsmittel (requests ohne Auth-Sonderfall, click, python-dotenv, chalk, colorama) | −2 |

- **A — empfohlen:** Score ≥ 3 oder auf der Known-Pitfall-Liste.
- **B — optional:** Score 1–2.
- **C — übersprungen:** Score ≤ 0.
- **Rein indirekt:** Ein Kandidat, dessen Zeilen alle das Merkmal
  `indirekt (// indirect in go.mod)` in der Spalte `Herkunft` tragen, kommt nach C — ohne
  Ausnahme, unabhängig vom Score und ohne Hinweis. Wer ihn will, holt ihn mit
  `add <gegenstand>` aus C.

**Known-Pitfall-Liste:** JWT, cryptography, aiohttp, pytz und Zeitzonen-Handling, Kafka,
LangChain, boto3, azure-sdk-*, pandas mit großen Frames, nebenläufige asyncio-Libraries, ORMs,
WebSocket-Libraries, gRPC. Sie ist eine Konstante dieses Commands: hier ergänzbar, nicht
projektweise abschaltbar.

**Vorhanden.** Hat ein Kandidat schon ein Dokument, für das dieser Lauf zuständig ist
(es hat `generated.by: k-docs-tools` oder den Altwert `k-tools-scan`), steht er mit „vorhanden“ und dem bisherigen `generated.at` in der Liste.
Umschreiben ist eine Auswahl wie jede andere; vorhandene Kandidaten sind deshalb **nicht
vorausgewählt**, auch nicht in A.

Zeige die Liste **kompakt**: A und B einzeln, C als eine zusammengefasste Zeile.

```text
Kandidaten aus dem Inventar (<N> Gegenstände, erhoben <generated.at>):

Empfohlen (A):
  [x] python/fastapi            0.115.0          exact   in 47 Dateien
      → Framework, Reihenfolge der Middleware, Async-Fallen
  [ ] python/sqlalchemy         2.0.35           exact   in 28 Dateien   vorhanden (2026-10-02)
      → Session-Semantik 1.x → 2.0, Async-Unterschiede
  [x] container/postgres        16.4, 15         exact   —
      → Major-Upgrade braucht Dump/Restore, verschiedene Versionen je Umgebung

Optional (B):
  [ ] python/pydantic           2.9.2            exact   in 34 Dateien
  [ ] python/httpx              0.27.0           range   in 8 Dateien, mit Auth

Übersprungen (C): python/pytest, python/ruff, python/black, … (18 Gegenstände) — auf Wunsch
  ausklappen.

Ohne Gegenstand im Inventar: python-requests.md — bleibt liegen | drop

Kommandos:
  all             alle A und B aufnehmen
  A-only          nur A ohne „vorhanden“ (Standard)
  add <gegenstand>     aufnehmen, auch „vorhanden“ (umschreiben) oder aus C
  remove <gegenstand>  aus der Auswahl streichen
  show C          C vollständig zeigen
  drop <datei>    ein eigenes Dokument ohne Gegenstand im Inventar entfernen
  proceed         mit der aktuellen Auswahl weiter
```

- Ein eigenes Dokument, dessen Gegenstand nicht mehr im Inventar steht, zeigt die Liste
  unter „Ohne Gegenstand im Inventar“. Es bleibt liegen, außer der Nutzer wählt `drop`; hat
  es `hints`, braucht die Löschung die Bestätigung, dass sie mit ihm gehen (Kopf in der
  Docs-Ablage). `publish` deutet eine fehlende Datei als Löschung.
- Kein Hin und Her je Tool: Der Nutzer wählt in **einer** Nachricht, dann geht es weiter.
- Fällt alles in C und wählt der Nutzer nichts aus, sag das und ende mit dem Abschluss.
- **Nichts schreiben, bevor die Auswahl bestätigt ist.**
