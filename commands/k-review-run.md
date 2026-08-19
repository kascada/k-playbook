---
description: Führt einen Review-Lauf über MCP an oder setzt ihn fort; das optionale Argument wählt new, latest oder ein Datum YYYY-MM-DD.
argument-hint: [YYYY-MM-DD|latest|new]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-review-run

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

`/k-review-run` ist der Chat-Einstieg für das Laufmodell. Der Command hält keinen
eigenen Zustand: Jeder Aufruf liest den Ist-Zustand über die MCP-Werkzeuge aus dem
Laufverzeichnis und setzt genau dort fort.

Ergebnisse dieses Commands:

- ein Review-Lauf unter `k-playbook-local/results/YYYY-MM-DD/`, wenn ein neuer Lauf
  bestätigt und angelegt wird,
- Scanner-Fortschritt und AI-Entry-Fortschritt unter `entries/`,
- `review-input.json` und `review-input.md` nach dem Merge,
- `review-triage.md` nach Anwendung des Moduls
  `commands/_review-run/review-scan-triage.md`.

## Schritt 1 — Pfade und Lauf bestimmen

Löse aus der Context-Ausgabe:

- `RESOLVED_PROJECT_DIR` = `project.dir`
- `RESOLVED_LOCAL_DIR` = `local.dir`
- `RESOLVED_RESULTS_DIR` = `<local.dir>/results`
- `RESULTS_DISPLAY_PATH` = `k-playbook-local/results`
- `TODAY` = `now.date`

Command-specific policy:

- Das Laufverzeichnis wird nie geraten. Für bestehende Läufe kommt es aus
  `k_playbook_review_status`, für neue Läufe aus `k_playbook_review_create`.
- `scan-triage` ist ein AI-Eintrag aus dem Command-Modul
  `commands/_review-run/review-scan-triage.md`, kein Eintrag aus
  `catalogs.reviews`.
- `review-triage.md` wird direkt in den vom MCP-Status gelieferten Laufordner
  geschrieben. `k_playbook_review_write_ai_entry` schreibt danach nur
  `entries/scan-triage.json`.
- Fehlt `RESOLVED_RESULTS_DIR`, frage, ob genau dieses Verzeichnis angelegt werden
  soll oder ob `/k-gui` die Struktur reparieren soll. Kein Ersatzpfad.

Argumente:

- Leer oder `new`: den heutigen Lauf `TODAY` verwenden. Existiert er nicht, nach
  Schritt 3 einen neuen Lauf anlegen.
- `latest`: den jüngsten Lauf mit `run.json` verwenden.
- `YYYY-MM-DD`: genau diesen Lauf verwenden. Existiert er nicht, nach Schritt 3
  anbieten, ihn mit diesem Datum anzulegen.

## Schritt 2 — Status lesen

Rufe immer zuerst `k_playbook_review_status` auf.

- Für bestehende Läufe: `projectDir: RESOLVED_PROJECT_DIR`, `mode: existing`,
  `run: <lauf>`.
- Wenn noch kein Lauf feststeht oder ein neuer Lauf entstehen soll:
  `projectDir: RESOLVED_PROJECT_DIR`, `mode: available`.

Melde kompakt:

```text
Lauf: 2026-08-19
Werkzeuge: 5 done, 1 failed, 0 running, 0 start
AI-Einträge: 1 done, 0 failed, 0 running, 3 start
Merge: review-input.json vorhanden / fehlt
Triage: review-triage.md vorhanden / fehlt
Nächster Schritt: <konkret>
```

Wenn der Status einen inkonsistenten `scan-triage`-Eintrag zeigt, gilt Schritt 7:

- `done` mit `result: review-triage.md` und vorhandener Datei ist erledigt.
- `done` ohne vorhandene Ergebnisdatei ist reparaturbedürftig; führe die Bewertung
  erneut aus und schreibe den Eintrag danach neu.
- Vorhandenes gültiges `review-triage.md` bei fehlendem oder gestarteten Eintrag
  wird durch `k_playbook_review_write_ai_entry` repariert.

## Schritt 3 — Auswahl für einen neuen Lauf klären

Wenn ein neuer Lauf angelegt werden soll, verwende die Auswahlbasis aus
`k_playbook_review_status` im Modus `available`.

Zeige kompakt:

- verfügbare Werkzeuge, gruppiert nach Sprache und Installationsstatus,
- aktive AI-Review-Rezepte aus `catalogs.reviews`,
- den Command-Moduleintrag `scan-triage`, wenn er im effektiven
  Command-Namensraum vorhanden und nicht durch leeres Overlay abgeschaltet ist,
- die Standardvorbelegung aus `selection.defaultEntries`.

Frage genau einmal nach Einschränkungen:

```text
Soll die Standardauswahl vollständig laufen? Leere Antwort bedeutet ja; nenne sonst
die Einträge, die laufen sollen oder wegfallen sollen.
```

Bei leerer Antwort oder ausdrücklicher Bestätigung lege den Lauf mit der vollen
Standardauswahl über `k_playbook_review_create` an. Nennt der Nutzer Einschränkungen,
zeige die daraus entstehende Auswahl und hole vor `k_playbook_review_create` eine
ausdrückliche Bestätigung ein.

`scan-triage` gehört zur Standardauswahl, wenn das Modul verfügbar ist. Es darf nicht
aus `catalogs.reviews` abgeleitet oder in der GUI-Auswahl für Review-Rezepte erwartet
werden.

## Schritt 4 — Scanner starten

Wenn Tool-Einträge auf `start` stehen, rufe `k_playbook_review_scan` auf:

```json
{ "projectDir": "RESOLVED_PROJECT_DIR", "run": "<lauf>" }
```

Danach lies den Status erneut. Technische Fehler bleiben Entry-Zustände: Ein
`failed`-Tool stoppt den Command nicht automatisch. Melde den Grund und frage, ob trotz
Fehlern weitergemacht, der Scan wiederholt oder abgebrochen werden soll.

Wenn nur bestimmte Tool-Einträge wiederholt werden sollen, übergib sie in `entries`.
AI-Einträge werden nie an `k_playbook_review_scan` übergeben.

## Schritt 5 — AI-Review-Einträge ausführen

Für AI-Einträge aus `catalogs.reviews` mit Zustand `start` oder `running`:

1. Lade den im Lauf gespeicherten `recipePath`.
2. Führe den Review als eigenen Arbeitsschritt aus.
3. Schreibe das Ergebnis in den im Eintrag genannten `defaultResult` oder einen
   bestätigten relativen Ergebnisnamen im Laufordner.
4. Setze den Eintrag mit `k_playbook_review_write_ai_entry` auf `done`, oder bei
   technischem Abbruch auf `failed` mit Grund.
5. Lies danach den Status erneut.

Der Eintrag `scan-triage` wird hier noch nicht ausgeführt. Er gehört zu Schritt 7,
weil er `review-input.*` aus dem Merge braucht.

## Schritt 6 — Merge starten

Wenn alle gewünschten Tool-Einträge und alle AI-Review-Rezepte in einem Endzustand
stehen, starte den Merge über `k_playbook_review_merge`:

```json
{ "projectDir": "RESOLVED_PROJECT_DIR", "run": "<lauf>" }
```

Sind noch AI-Review-Rezepte offen, frage vor dem Merge:

- offene AI-Reviews zuerst ausführen,
- bewusst nur die vorhandenen Tool-Belege mergen,
- oder abbrechen und später fortsetzen.

Nach dem Merge lies den Status erneut und nenne:

- `<lauf>/review-input.json`,
- `<lauf>/review-input.md`.

Rohdaten, `run.json` und vorhandene Entry-Dateien werden durch den Merge nicht
verändert.

## Schritt 7 — Bewertung schreiben

Führe diesen Schritt erst aus, wenn `review-input.json` und `review-input.md` im
Laufordner vorhanden sind.

Wende `commands/_review-run/review-scan-triage.md` wortlaut-treu an:

- Verwende `review-input.json` als Audit-Beleg und `review-input.md` als Ansicht.
- Suche `known-decisions.md` nur an den dort genannten deterministischen Pfaden.
- Bündele Gruppen nach gemeinsamer Root-Cause.
- Vergib Priorität `P1`/`P2`/`P3` und Kategorie `S`/`T`/`K`/`F`/`A`/`X`.
- Verweise auf stabile Gruppen-IDs.
- Schreibe ausschließlich `<RUN_DIR>/review-triage.md`.

Danach rufe `k_playbook_review_write_ai_entry` auf:

```json
{
  "projectDir": "RESOLVED_PROJECT_DIR",
  "run": "<lauf>",
  "entry": "scan-triage",
  "state": "done",
  "result": "review-triage.md",
  "startedAt": "<RFC3339>",
  "finishedAt": "<RFC3339>"
}
```

Wenn `review-triage.md` bereits existiert, prüfe die Pflichtabschnitte aus dem Modul.
Ist die Datei gültig und fehlt nur der Eintragszustand, repariere ausschließlich den
Eintrag über `k_playbook_review_write_ai_entry`. Ist die Datei ungültig oder zeigt der
Eintrag auf ein fehlendes Ergebnis, führe die Bewertung erneut aus.

## Schritt 8 — Abschluss und Handoff

Lies zum Abschluss den Status erneut und melde:

- Lauf und Laufordner,
- Tool-Zustände,
- AI-Entry-Zustände,
- Pfad zu `review-input.json`, `review-input.md` und `review-triage.md`,
- offene technische Fehler oder bewusst übersprungene Einträge,
- den nächsten fachlichen Schritt.

Handoff: Der Nachfolger für `review-triage.md` ist noch nicht endgültig definiert.
Bis dahin endet der Command ausdrücklich mit dem Pfad zu
`k-playbook-local/results/<lauf>/review-triage.md` und dem Hinweis, dass daraus später
Tasks oder Remediation-Bündel abgeleitet werden.

## Fehlerfälle

- Kein k-playbook-Projekt: abbrechen und die fehlende Installation nennen.
- `k_playbook_review_status` meldet `project_not_found`: kein Ersatzpfad, stattdessen
  `/k-gui` nennen.
- Lauf fehlt: bei `new` oder Datum nach Auswahlklärung anlegen; bei `latest` die
  verfügbaren Läufe zeigen.
- `run.json` unlesbar: nicht reparieren; Fehler und Pfad nennen.
- Entry-Datei unlesbar: nicht überschreiben; Fehler und Pfad nennen.
- `scan-triage` fehlt in der Auswahlbasis: sagen, dass das Command-Modul im effektiven
  Namensraum fehlt oder abgeschaltet ist; Bewertung nicht improvisieren.
- `review-input.*` fehlt: zuerst Schritt 6 ausführen.
- `done` für `scan-triage` ohne vorhandenes `review-triage.md`: Zustand als
  reparaturbedürftig melden und Schritt 7 erneut ausführen.

## Anti-Muster (nicht tun)

- Keinen Laufzustand aus Chat-Gedächtnis ableiten; immer MCP-Status lesen.
- `scan-triage` nicht als Review-Katalog-Rezept behandeln.
- `review-triage.md` nicht über `k_playbook_review_write_ai_entry` schreiben; das
  Werkzeug schreibt nur den Entry-Zustand.
- Keine freien Suchpfade für `known-decisions.md` verwenden.
- Nicht nach `k-playbook/` schreiben.
