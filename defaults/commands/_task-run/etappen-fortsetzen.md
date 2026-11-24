---
title: Etappen fortsetzen
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn ein Task mit Etappen schon erledigte Etappen trägt — die Rückfrage fortsetzen, von vorn beginnen oder überspringen, und die Zeile, die den Fortschritt zurücksetzt.
---

# Etappen fortsetzen

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5a.2, wenn die Fortschrittszeilen eines
Tasks mindestens eine Etappe als erledigt zeigen. Er hat `TASK_FILE`, `REVISION` und den Stand
(Zahl `m` der Etappen, erledigte Etappen, Datum der letzten Zeile).

Fertig-Meldung (`Rückfrage: Etappen fortsetzen`), den Stand
zeigen und fragen:

```
<TASK_FILE>: Etappen 1-2 von 6 erledigt (zuletzt 2026-08-12).

Wie weiter?
  (a) ab Etappe 3 fortsetzen   (Default)
  (b) von vorn beginnen        - erledigte Etappen erneut ausführen
  (c) diesen Task überspringen
```

Bei `(b)` vor der Delegation mit `k_playbook_doc_append` und `since_revision: REVISION` die
Zeile anhängen:

```text
<!-- k-task-run: fortschritt zurückgesetzt <now.date> -->
```
