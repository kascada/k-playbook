---
title: Task ausgeführt, nicht abgeschlossen
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn in 5a die Kennzeile „ausgeführt“ eines Tasks trifft — die Rückfrage mit den Optionen abschließen oder anhalten, und was „abschließen“ tut (Befund-Leser, Status nach Review, Abschluss ohne erneute Ausführung).
---

# Task ausgeführt, nicht abgeschlossen

Dieses Modul liest der Hauptkontext von `/k-task-run`, wenn in 5a die Kennzeile „ausgeführt“
eines Tasks **trifft**: Der Task ist ausgeführt, aber nicht abgeschlossen — nach der
Stopp-Abfrage in 5f angehalten, oder eine Sitzung endete zwischen Kopf und `done/`. Er hat
`TASK_REF`, `TASK_FILE`, `REVISION` und die Zeilennummer `ZEILE` der letzten Kennzeile
„ausgeführt“, die trifft. Der Task wird weder gelesen noch erneut ausgeführt.

Fertig-Meldung (`Rückfrage: nicht abgeschlossen`), dann:

```
<TASK_FILE>: bereits ausgeführt, nicht abgeschlossen — Review, sein Ergebnis oder
kritische Befunde offen (siehe ## Ausführung).

Wie weiter?
  (a) abschließen  - offene Punkte erledigt oder akzeptiert, Task wandert nach done/
  (b) anhalten     - der Lauf endet hier, keine weiteren Tasks
```

- (a): Zuerst die Befunde des letzten Laufs holen, ohne sie selbst zu lesen: Der
  Hauptkontext startet den **Befund-Leser** (`abschluss.md`, Abschnitt „Befund-Leser“) mit der Zeilennummer der letzten
  Kennzeile „ausgeführt“, die oben trifft. Aus seiner Rückgabe sammelt er die Zeilen unter
  `Kritisch:` und `Wichtig:` für diesen Task, dazu getrennt die unter `Hinweise:` (nur zur Anzeige), wie nach „(b) trotzdem weiter“ (Tabelle „Ausgänge des
  Reviews“ im Command, `abschluss.md`, „Befunde sammeln“); bei `Review: kein Abschnitt` oder einer Rückgabe in anderer Form
  sammelt er nichts. Der Leser schreibt nichts, `REVISION` bleibt. Dann mit
  `k_playbook_doc_append` und `since_revision: REVISION` anhängen:

  ```text
  **Status nach Review:** abgeschlossen vom Nutzer (<now.date>)
  ```

  Dann weiter wie 5f, Schritt 6 (nach `done/` verschieben, Snapshot freigeben); die
  Ausführungswurzel dafür ist `Target repo` aus `## Ausführungskontext` (`section`, erster
  Treffer), sonst die Projektwurzel. 5f.1 läuft nicht; steht dort `PR required: true`, nennt
  der Command den PR als offenen Schritt. Weiter mit dem nächsten Task.
- (b): Der Lauf endet, keine weiteren Tasks.

Kein automatisches Beheben, kein erneutes Review, keine erneute Ausführung.
