---
description: Führt einen vollständigen Audit-Sweep über MCP an oder setzt ihn fort; das optionale Argument wählt new, latest oder ein Datum YYYY-MM-DD.
argument-hint: [YYYY-MM-DD|latest|new]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_review_status, mcp__k-playbook__k_playbook_review_create, mcp__k-playbook__k_playbook_review_scan, mcp__k-playbook__k_playbook_review_merge, mcp__k-playbook__k_playbook_review_write_ai_entry]
---

# k-audit

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

`/k-audit` ist der Chat-Einstieg für den vollständigen Scan-und-Bewertungs-Sweep. Der Command hält keinen
eigenen Zustand: Jeder Aufruf liest den Ist-Zustand über die MCP-Werkzeuge aus dem
Laufverzeichnis und setzt genau dort fort.

Ergebnisse dieses Commands:

- ein Review-Lauf unter `k-playbook-local/results/YYYY-MM-DD/`, wenn ein neuer Lauf
  bestätigt und angelegt wird,
- Scanner-Fortschritt und AI-Entry-Fortschritt unter `entries/`,
- SARIF der Evidence-Rezepte unter `raw/<entry>.sarif`,
- `review-input.json` und `review-input.md` nach dem Merge,
- Perspektiven-Reports aktiver Katalog-Rezepte im Laufordner,
- optional `review-triage.md` nach Anwendung des Moduls
  `commands/_audit/review-scan-triage.md`.

Argument-Behandlung ist Pflicht: der Command darf nie eigenmächtig ein anderes
Datum wählen als angegeben.

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
  `commands/_audit/review-scan-triage.md`, kein Eintrag aus
  `catalogs.reviews`.
- `review-triage.md` wird direkt in den vom MCP-Status gelieferten Laufordner
  geschrieben. `k_playbook_review_write_ai_entry` schreibt danach nur
  `entries/scan-triage.json`.
- Fehlt `RESOLVED_RESULTS_DIR`, frage, ob genau dieses Verzeichnis angelegt werden
  soll oder ob `/k-gui` die Struktur reparieren soll. Kein Ersatzpfad.

Das Argument steht in `$ARGUMENTS`. Lies es wörtlich; ein leerer Wert ist ein
eigener Fall und kein Freibrief für ein anderes Datum.

Argumente:

- `$ARGUMENTS` ist leer oder `new`: den heutigen Lauf `TODAY` verwenden.
  Existiert er nicht, nach Schritt 3 einen neuen Lauf anlegen.
- `$ARGUMENTS` ist `latest`: den jüngsten Lauf mit `run.json` verwenden.
- `$ARGUMENTS` sieht aus wie `YYYY-MM-DD`: genau diesen Lauf verwenden. Existiert
  er nicht, nach Schritt 3 anbieten, ihn mit diesem Datum anzulegen.
- `$ARGUMENTS` passt auf keinen dieser Fälle: nicht raten. Den Wert wörtlich
  zurückgeben, sagen, dass er keinem Fall entspricht, und nach dem gemeinten Lauf
  fragen. Kein Statusaufruf, bis die Rückfrage beantwortet ist. Auch ein Wert, der
  ein Datum enthält, aber zusätzlichen Text trägt, fällt hierunter.

## Schritt 2 — Status lesen

Melde vor dem ersten `k_playbook_review_status` Teil 1 des Argument-Meldeblocks:

```text
Argument: <der Wert wörtlich, oder `leer`>
Absicht: <Lauf TODAY (YYYY-MM-DD) | jüngster Lauf | Lauf YYYY-MM-DD | unbestimmt>
Erster Statusaufruf: <mode: existing, run: YYYY-MM-DD | mode: available | keiner>
```

Rufe danach `k_playbook_review_status` auf. Welcher erste Statusaufruf zu welchem
Argument gehört:

- `$ARGUMENTS` sieht aus wie `YYYY-MM-DD`: `projectDir: RESOLVED_PROJECT_DIR`,
  `mode: existing`, `run: <dieses Datum>`. Bei `run_not_found` erst dann
  `mode: available`, um die Auswahlbasis für Schritt 3 zu holen — der Ziel-Lauf
  bleibt dabei das genannte Datum, nicht `TODAY`.
- `$ARGUMENTS` ist leer oder `new`: `projectDir: RESOLVED_PROJECT_DIR`,
  `mode: available`; dessen `todayExists` beantwortet bestehend oder neu in einem
  Aufruf.
- `$ARGUMENTS` ist `latest`: `projectDir: RESOLVED_PROJECT_DIR`,
  `mode: available`; die Liste `runs` liefert die Auswahl, danach `mode: existing`
  auf den jüngsten Lauf mit `run.json`.

Nicht tun:

- Steht im Argument ein Datum, nicht `mode: available` als ersten Statusaufruf
  verwenden. Erst `mode: existing` mit genau diesem Datum; nur bei
  `run_not_found` in den Neuanlage-Pfad wechseln, wo `mode: available` die
  Auswahlbasis für Schritt 3 liefert.

Melde direkt nach dem ersten Statusaufruf und vor der Statusmeldung Teil 2 des
Argument-Meldeblocks:

```text
Ziel-Lauf: <YYYY-MM-DD>
Modus: bestehend | neu
```

Melde kompakt:

```text
Lauf: 2026-08-19
Werkzeuge: 5 done, 1 failed, 0 running, 0 start
Evidence-Quellen: 1 done, 0 failed, 0 running, 1 start — offen: tech
Perspektiven: 1 done, 0 failed, 0 running, 2 start
Merge: review-input.json vorhanden / fehlt
Triage: review-triage.md aktuell / veraltet / fehlt
Nächster Schritt: <konkret>
```

Die beiden AI-Zeilen kommen aus `evidenceEntries` und `perspectiveEntries` der
Statusausgabe; jeder AI-Eintrag trägt seine Betriebsart zusätzlich als `mode`. Offene
Evidence-Quellen werden namentlich genannt: sie blockieren den Merge nicht und fehlen
sonst unbemerkt in `review-input.json`.

Die Triage-Zeile kommt aus `triage.state` der Statusausgabe: `current`, `stale` oder
`missing`. Den Vergleich rechnet das Werkzeug, nicht der Command — ermittle keine
Änderungszeiten selbst. Bei `stale` nennt `triage.reason` den Fall; Schritt 6 erklärt ihn.

Wenn der Status einen inkonsistenten AI-Eintrag zeigt, gilt je nach Betriebsart Schritt
5, 7 oder 8. Der Status trennt beide Fälle bereits: eine Perspektive wird an ihrer
Ergebnisdatei gemessen, eine Evidence-Quelle an ihrem SARIF.

Perspektiven (`mode: perspective`, Schritt 7; `scan-triage` in Schritt 8):

- `done` mit `result` und vorhandener, nicht leerer Datei ist erledigt.
- `done` ohne vorhandene Ergebnisdatei ist inkonsistent (`resultMissing`); führe den
  AI-Eintrag erneut aus und schreibe den Eintrag danach neu.
- Vorhandene gültige Ergebnisdatei bei fehlendem oder offenem Entry-Status darf durch
  `k_playbook_review_write_ai_entry` repariert werden (`repairable`).

Evidence-Quellen (`mode: evidence`) messen an `raw/<entry>.sarif` statt an einer
Ergebnisdatei; ihr Reparaturvertrag steht in Schritt 5.

## Schritt 3 — Auswahl für einen neuen Lauf klären

Wenn ein neuer Lauf angelegt werden soll, verwende die Auswahlbasis aus
`k_playbook_review_status` im Modus `available`.

Zeige kompakt:

- verfügbare Werkzeuge, gruppiert nach Sprache und Installationsstatus,
- aktive AI-Review-Rezepte aus `catalogs.reviews`, getrennt nach Betriebsart:
  Evidence-Quellen aus `selection.evidenceCandidates` je mit ihrem Pfad-Scope
  `scope.paths`, Perspektiven aus `selection.perspectiveCandidates` je mit ihrem
  Tool-Scope `scope.tools`,
- den Command-Moduleintrag `scan-triage`, wenn er im effektiven
  Command-Namensraum vorhanden und nicht durch leeres Overlay abgeschaltet ist,
- die Standardvorbelegung aus `selection.defaultEntries`.

Beide Scopes werden mit dem Lauf eingefroren und im Lauf nicht mehr verhandelt: sie
kommen aus dem Rezept und stehen danach in `run.json`. Passt ein Pfad-Scope nicht, wird
das Rezept geändert und danach ein neuer Lauf angelegt — der Scope im Lauf wird nicht
gebogen. `scope-hint` bleibt Freitext für `/k-review` und erweitert `scope.paths` nicht.

Rezepte mit widersprüchlichem `audit`-Block stehen in `selection.unavailableCandidates`
mit `unavailableReason` und sind nicht auswählbar. Nenne den Grund, statt sie
stillschweigend wegzulassen.

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

## Schritt 5 — Evidence-Rezepte ausführen

Führe diesen Schritt **vor** dem Merge aus. Evidence-Rezepte lesen `review-input.json`
nicht — sie liefern einen Teil davon.

Betroffen sind die AI-Einträge mit `mode: evidence` — in der Statusausgabe die Liste
`evidenceEntries` — im Zustand `start` oder `running`. Je Eintrag:

1. Lade den im Lauf gespeicherten `recipePath`.
2. Verwende den im Lauf gespeicherten `scope`-Snapshot, insbesondere `scope.paths`. Der
   Pfad-Scope ist verbindlich und wird nicht aus dem Chat-Gedächtnis erweitert; innerhalb
   der Globs gelten zusätzlich die zentralen Ausschlüsse der Modulsuche (`k-playbook`,
   `k-playbook-local`, `vendor`, `node_modules`, `testdata`, Punkt-Verzeichnisse).
3. Führe das Rezept auf dem Code in diesem Scope aus. Die zulässigen Rule-IDs stehen im
   Rezept unter `audit.ruleIds`; sie kommen aus dem Rezept und nicht aus `run.json`.
4. Schreibe das Ergebnis als SARIF nach `raw/<entry>.sarif`: `tool.driver.name` ist der
   Eintragsname, jeder Fund trägt eine `ruleId` aus `audit.ruleIds`, einen Fundort im
   Scope und das im Rezept je Rule-ID festgelegte `level`. Ein Ergebnisdokument in
   Markdown entsteht nicht.
5. Melde den Eintrag mit `k_playbook_review_write_ai_entry`:

   ```json
   {
     "projectDir": "RESOLVED_PROJECT_DIR",
     "run": "<lauf>",
     "entry": "<entry>",
     "state": "done",
     "job": {
       "sarif": "raw/<entry>.sarif",
       "started": "<RFC3339>",
       "finished": "<RFC3339>"
     }
   }
   ```

6. Lies danach den Status erneut.

`result` bleibt leer: das Pflichtartefakt ist das SARIF. Der Job gehört zur
Fertigmeldung — bei `state: running` wird er abgewiesen. Ein SARIF ohne Ergebnisse ist ein
gültiges `done`: ein leerer Scope-Befund ist ein Ergebnis, kein Fehler.

Evidence-Quellen schreiben `raw/<entry>.sarif` und sonst nichts: keinen Family-Ordner,
kein zweites `review-input.*` und keine Bewertung. Priorität und Kategorie vergibt allein
Schritt 8.

Lies die Antwort des Werkzeugs, statt `done` zu unterstellen:

- `evidence.findings` ist die Zahl der übernommenen Funde.
- `evidence.droppedFindings` und `evidence.droppedPaths` nennen die Funde außerhalb von
  `scope.paths`. Sie werden verworfen, `raw/<entry>.sarif` wird bereinigt
  zurückgeschrieben (`evidence.sarifRewritten`), der Eintrag bleibt gültig. Melde die
  Zahl — sie sagt, dass das Rezept über seinen Scope hinausgegriffen hat.
- `stateOverridden: true` mit `requestedState: done` heißt: das Artefakt war ungültig,
  geschrieben wurde `failed` mit Grund.

Zwei Fehlerklassen, die auseinandergehalten werden:

- **Fehler des Aufrufs** — SARIF-Pfad außerhalb von `raw/`, Datei fehlt oder ist leer,
  `result` an einem Evidence-Eintrag, Job bei `state: running`, Rezept ohne gültigen
  Evidence-Vertrag. Das Werkzeug schreibt dann **nichts**. Korrigiere den Aufruf; das
  Rezept muss dafür nicht erneut laufen.
- **Fehler des Artefakts** — unlesbares SARIF, `tool.driver.name` ungleich Eintragsname,
  eine Rule-ID außerhalb von `audit.ruleIds`. Der Eintrag wird `failed` mit Grund
  geschrieben. Nur ein erneuter Rezeptlauf behebt das; ein zweiter Statusaufruf nicht.

Reparaturvertrag für Evidence-Quellen:

- Eintrag ist `done`, Job vorhanden, `raw/<entry>.sarif` existiert und ist nicht leer:
  konsistent, keine Reparatur nötig.
- Eintrag ist `done`, aber ohne Job oder ohne vorhandenes SARIF: Der Status meldet
  `sarifMissing` und `inconsistent`. Führe das Rezept erneut aus und melde neu. Ein
  nachgeschriebener Status ohne Artefakt bleibt eine leere Zusage — der Merge liest die
  Datei, nicht den Zustand.
- Gültiges SARIF bei fehlendem oder offenem Entry-Status: Der Status meldet `repairable`.
  Hier genügt `k_playbook_review_write_ai_entry` mit dem Job; ein erneuter Rezeptlauf ist
  nicht nötig.
- Eintrag ist `failed`: durch einen erneuten Rezeptlauf ersetzen, nicht durch
  Nachschreiben des Status.
- Rezept nach Laufstart geändert: Der Lauf behält seinen `scope`-Snapshot, die
  Rule-ID-Liste wird beim Melden aber frisch aus dem Rezept gelesen. Erfüllt das Rezept
  den Evidence-Vertrag nicht mehr, scheitert das Melden mit `recipe_contract_invalid` —
  ein Fehler des Aufrufs, es wird nichts geschrieben.
- Alter Lauf ohne Eintrag für ein später hinzugefügtes Rezept: keine nachträgliche
  Ergänzung der alten `run.json`.

Ein Evidence-Eintrag, der offen bleibt oder scheitert, blockiert den Merge nicht. Nenne
ihn namentlich und mache weiter; Schritt 6 sagt, was dann gilt.

## Schritt 6 — Merge starten

Wenn die gewünschten Tool-Einträge in einem Endzustand stehen, starte den Merge über
`k_playbook_review_merge`:

```json
{ "projectDir": "RESOLVED_PROJECT_DIR", "run": "<lauf>" }
```

Offene, fehlgeschlagene oder noch nicht ausgeführte AI-Einträge blockieren den Merge
nicht. Der Merge sammelt Tool-Einträge **und** Evidence-Einträge in einem Endzustand ein:
gelesen wird, wessen Eintrag auf `done` steht und wessen Job ein SARIF im Laufordner
nennt. Perspektiven haben kein SARIF und tragen deshalb nichts bei — sie lesen das
Ergebnis, statt es zu liefern. Der Merge schreibt danach:

- `<lauf>/review-input.json`,
- `<lauf>/review-input.md`.

Rohdaten, `run.json` und vorhandene Entry-Dateien werden durch den Merge nicht
verändert. Lies danach den Status erneut.

Nenne dabei ausdrücklich, welche Evidence-Quellen noch offen sind: ihre Funde fehlen in
`review-input.json`. Ein erneuter Merge, nachdem die Evidence eingetroffen ist, ist der
reguläre Weg und kein Reparaturfall — `k_playbook_review_merge` rechnet beide Artefakte
neu und überschreibt sie.

Ein erneuter Merge entwertet eine bereits geschriebene Bewertung: `review-triage.md`
beschreibt dann einen Stand, den es nicht mehr gibt. Der Eintragszustand zeigt das nicht
— er misst nur, ob die Ergebnisdatei da und nicht leer ist, und bleibt deshalb `done` und
konsistent. `k_playbook_review_status` vergleicht darum die Änderungszeit von
`review-input.json` mit `finishedAt` des Eintrags `scan-triage` und meldet das Ergebnis
unter `triage`:

- `current` — die Bewertung ist nach dem letzten Merge entstanden.
- `stale` — `review-input.json` ist jünger, oder die Aktualität ist nicht belegbar, weil
  `review-input.json` fehlt oder der Eintrag keine brauchbare Endzeit nennt.
  `triage.reason` sagt, welcher Fall vorlag.
- `missing` — es gibt noch keine `review-triage.md`.

Bei `stale` melde die Bewertung als veraltet und führe Schritt 8 erneut aus. Ein Lauf mit
veralteter Bewertung ist nicht vollständig.

## Schritt 7 — Katalog-Perspektiven ausführen

Führe diesen Schritt erst aus, wenn `review-input.json` im Laufordner vorhanden ist.

Für AI-Einträge mit `mode: perspective` aus `catalogs.reviews` — in der Statusausgabe die
Liste `perspectiveEntries` — mit Zustand `start` oder `running`:

1. Lade den im Lauf gespeicherten `recipePath`.
2. Verwende den im Lauf gespeicherten `scope`-Snapshot, insbesondere `scope.tools`.
3. Führe den Review als Perspektive auf `review-input.json` aus: Gruppen gehören in die
   Perspektive, wenn mindestens eine Evidence ein `evidence.tool` aus `scope.tools`
   trägt; fremde Evidence bleibt Kontext und wird als außerhalb des Scopes markiert.
4. Schreibe das Ergebnis in den im Eintrag genannten `defaultResult` oder einen
   bestätigten relativen Ergebnisnamen im Laufordner.
5. Setze den Eintrag mit `k_playbook_review_write_ai_entry` auf `done`, oder bei
   technischem Abbruch auf `failed` mit Grund.
6. Lies danach den Status erneut.

Leere Scope-Ergebnisse sind gültig: Das Ergebnisdokument sagt dann klar „keine scoped
Findings" und der Eintrag kann auf `done` gesetzt werden. Katalog-Perspektiven schreiben
keinen Family-Ordner, keine `raw/`-Artefakte und kein zweites `review-input.*`.

Dieses Verbot gilt der Betriebsart, nicht dem Lauf: Evidence-Quellen schreiben
`raw/<entry>.sarif`, und genau das ist ihr Pflichtartefakt (Schritt 5).

Reparaturvertrag für Katalog-Perspektiven:

- Eintrag existiert in `run.json`, Ergebnisdatei fehlt, Entry ist offen: Status bleibt
  offen; Rerun führt den AI-Eintrag erneut aus und schreibt die Ergebnisdatei.
- Eintrag existiert in `run.json`, Ergebnisdatei fehlt, Entry steht auf abgeschlossen:
  Statusausgabe markiert den Eintrag als inkonsistent und `resultRequired` nicht erfüllt;
  Rerun schreibt die Ergebnisdatei neu und repariert den Status.
- Eintrag existiert in `run.json`, Ergebnisdatei existiert, Entry-Status fehlt oder ist
  offen: Statusausgabe darf den Eintrag als reparabel markieren; Rerun muss keine neue
  Datei erzwingen, sondern darf den Status über `k_playbook_review_write_ai_entry` auf
  abgeschlossen setzen, wenn die Datei nicht leer ist.
- Eintrag existiert in `run.json`, Ergebnisdatei existiert, Entry ist abgeschlossen: keine
  Reparatur nötig.
- Rezept wurde nach Laufstart deaktiviert oder geändert: Der bestehende Lauf nutzt weiter
  den Snapshot aus `run.json`.
- Alter Lauf enthält keinen Eintrag für ein später hinzugefügtes Rezept: keine
  automatische nachträgliche Ergänzung der alten `run.json`.

Der Eintrag `scan-triage` wird hier noch nicht ausgeführt. Er gehört zu Schritt 8.

## Schritt 8 — Bewertung schreiben

Führe diesen Schritt erst aus, wenn `review-input.json` und `review-input.md` im
Laufordner vorhanden sind. `scan-triage` darf vorhandene Perspektiven-Reports als Kontext
nutzen, aggregiert sie aber nicht in einem zweiten Merge-Schritt.

Wende `commands/_audit/review-scan-triage.md` wortlaut-treu an: das Modul lesen und
befolgen, ohne seine Regeln hier zu wiederholen. Welche Belege es liest, wie es bündelt,
wie es KI-Evidence gewichtet und welche Prioritäten und Kategorien es vergibt, steht im
Modul und nirgends sonst. Hier steht nur, was das Modul nicht wissen kann:

- `RUN_DIR` ist der Laufordner aus dem MCP-Status. Er wird nicht geraten.
- Geschrieben wird ausschließlich `<RUN_DIR>/review-triage.md`.

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

Wenn `review-triage.md` bereits existiert, prüfe die Pflichtabschnitte aus dem Modul. Ist
die Datei gültig und fehlt nur der Eintragszustand, repariere ausschließlich den Eintrag
über `k_playbook_review_write_ai_entry`. Ist die Datei ungültig oder zeigt der Eintrag auf
ein fehlendes Ergebnis, führe die Bewertung erneut aus.

Meldet der Status `triage.state: stale`, ist die Bewertung veraltet oder ihre Aktualität
nicht belegbar: führe sie erneut aus und melde den Eintrag neu. Das Reparieren des
Eintragszustands genügt hier nicht — der Zustand war nie falsch, der Inhalt ist es.

## Schritt 9 — Abschluss und Handoff

Lies zum Abschluss den Status erneut und melde:

- Lauf und Laufordner,
- Tool-Zustände,
- AI-Entry-Zustände, getrennt nach Evidence-Quellen und Perspektiven,
- Pfad zu `review-input.json`, `review-input.md` und `review-triage.md`,
- offene technische Fehler oder bewusst übersprungene Einträge,
- den nächsten fachlichen Schritt:
  `/k-remediation k-playbook-local/results/<lauf>/review-triage.md` für die Abarbeitung
  der Triage.

Ein Lauf ist nicht vollständig, solange eine Evidence-Quelle offen ist oder `triage.state`
nicht auf `current` steht. Sage das ausdrücklich, statt den Lauf als fertig zu melden: der
erste Fall wird über Schritt 5 und einen erneuten Merge geschlossen, der zweite über
Schritt 8.

Handoff: `review-triage.md` ist das aktuelle Ergebnisartefakt und geht direkt in die
Abarbeitung — einen Zwischenschritt gibt es nicht. Nenne wörtlich:

```text
/k-remediation k-playbook-local/results/<lauf>/review-triage.md
```

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
- `review-input.*` fehlt: zuerst Schritt 6 ausführen, den Merge. Schritt 7 setzt
  `review-input.json` voraus und kann es nicht erzeugen.
- `done` für `scan-triage` ohne vorhandenes `review-triage.md`: Zustand als
  reparaturbedürftig melden und Schritt 8 erneut ausführen.
- `triage.state: stale`: Bewertung als veraltet melden, `triage.reason` mit nennen und
  Schritt 8 erneut ausführen. Kein Reparieren des Eintragszustands.
- `sarif_path_invalid`, `sarif_required`, `entry_result_invalid`, `entry_job_invalid`
  oder `recipe_contract_invalid` beim Melden einer Evidence-Quelle: Fehler des Aufrufs,
  es wurde nichts geschrieben. Aufruf korrigieren, Rezept nicht erneut laufen lassen.
- Antwort mit `stateOverridden: true`: Das SARIF war ungültig, der Eintrag steht auf
  `failed`. Grund nennen und das Rezept erneut ausführen; den Status nicht überschreiben.
- Evidence-Quelle bleibt offen oder ist `failed`: Merge nicht blockieren. Den Eintrag
  namentlich melden und sagen, dass `review-input.json` seine Funde nicht enthält.

## Anti-Muster (nicht tun)

- Keinen Laufzustand aus Chat-Gedächtnis ableiten; immer MCP-Status lesen.
- Argument nicht stillschweigend durch `new` oder `today` ersetzen.
- Bei einem Argument, das auf keinen der vier Fälle passt, nicht den
  wahrscheinlichsten Fall wählen, sondern nachfragen.
- Ohne Teil 1 des Argument-Meldeblocks nicht den ersten Statusaufruf machen.
- `scan-triage` nicht als Review-Katalog-Rezept behandeln.
- `review-triage.md` nicht über `k_playbook_review_write_ai_entry` schreiben; das
  Werkzeug schreibt nur den Entry-Zustand.
- Keine freien Suchpfade für `known-decisions.md` verwenden.
- `scope.paths` einer Evidence-Quelle nicht aus dem Chat-Gedächtnis erweitern und nicht
  durch `scope-hint` überstimmen; der Snapshot aus `run.json` gilt.
- Keine Rule-IDs erfinden, die nicht in `audit.ruleIds` des Rezepts stehen — der Fund
  wäre über Läufe hinweg nicht mehr vergleichbar und das Melden schlägt fehl.
- Ein `failed` einer Evidence-Quelle nicht durch erneutes Schreiben des Status auf `done`
  heben; nur ein erneuter Rezeptlauf behebt ein ungültiges Artefakt.
- Evidence-Funde nicht im Rezeptlauf priorisieren oder kategorisieren; das ist Schritt 8.
- Einen Lauf nicht als vollständig melden, solange eine Evidence-Quelle offen ist oder
  `triage.state` nicht `current` ist.
- Den Zeitvergleich der Bewertung nicht selbst nachrechnen; `triage.state` aus dem Status
  ist die Antwort.
- Nicht nach `k-playbook/` schreiben.
