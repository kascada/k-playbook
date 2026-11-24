---
title: Erledigten Task wieder öffnen
description: Modul für /k-task-run, vom Hauptkontext gelesen in Schritt 2, wenn der Nutzer einen Task unter done/ wieder öffnen lässt — k_playbook_doc_reopen, die Kennzeile „wieder geöffnet“ und der Umgang mit dem Replica-Nachlauf.
---

# Erledigten Task wieder öffnen

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 2, nachdem der Nutzer das
Wieder-Öffnen eines Tasks unter `done/` bejaht hat.

Rufe `k_playbook_doc_reopen`. Nach dem Wieder-Öffnen gelten der Verweis und die `revision` aus
der Antwort als `TASK_REF` und `REVISION`. Dann mit `k_playbook_doc_append` und
`since_revision: REVISION` die Kennzeile anhängen (vorher gezählt, Revisionskette im Command):

```text
<!-- k-task-run: wieder geöffnet <TASK_FILE> <now.date> -->
```

Ihre Revision ist das neue `REVISION`. Ab dieser Zeile zählt eine frühere Kennzeile
„ausgeführt“ nicht mehr (5a). Meldet ein Lesen oder Zählen unter dem neuen Verweis direkt
danach `not_found` oder eine ältere Revision, ist das der Replica-Nachlauf aus
`k-playbook:/instructions/on-demand/tasks.md`, kein fremder Schreiber.
