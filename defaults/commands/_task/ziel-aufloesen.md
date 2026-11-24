---
title: Ziel auflösen
description: "Baustein für /k-task-run und /k-task-refine, eingebunden als include — der gemeinsame Teil beim Auflösen des Arguments zu Tasks: leer, keph:-Verweis mit Prüfung auf Fremdes, Nummer, mehrere Treffer, Treffer unter done/."
---

- **Leer:** die eigenen offenen Tasks aus `k_playbook_doc_list` (Sorte `tasks`, ohne `done`,
  mit `compact: true`; ist die Antwort `truncated`, mit `next.after` als `after` weiterholen),
  sortiert nach der Nummer am Anfang des Dateinamens. Keine → „Keine offenen Tasks gefunden“
  und Schluss.
- **Ein `keph:`-Verweis** (`keph:/user/…/tasks/<datei>`): genau dieser Task — nach den
  Prüfungen aus `k-playbook:/instructions/on-demand/tasks.md`, „Fremde und erledigte
  Verweise“, bevor etwas geschrieben oder Code geändert wird. Ein fremder Verweis (User-Pfadteil
  weder `~` noch `kephalaion.user`) wird abgewiesen, mit `foreign_user` in der Meldung; Schluss.
- **Eine Nummer:** aufgelöst nach `k-playbook:/instructions/on-demand/tasks.md`, „Wohin und
  Name“ — ein Aufruf mit `number` und `done: true` über die eigenen offenen und erledigten
  Tasks. Bei
  mehreren Treffern alle zeigen und den Nutzer wählen lassen (Fertig-Meldung
  `Rückfrage: mehrere Treffer`).
- **Erledigt** heißt: ein Treffer oder Verweis unter `done/`, der Task ist abgeschlossen; die
  Rückfrage dazu (Fertig-Meldung `Rückfrage: erledigter Task`) stellt der Command.
