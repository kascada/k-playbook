---
title: Modus refine
description: Modul für /k-task-run, vom Hauptkontext gelesen in Schritt 3 mit dem Wort refine — ungeprüfte Tasks vor dem Nehmen der Sperre durch die Gruppenprüfung von /k-task-refine schicken (ohne und mit Argument), die Ausgabe „Refine:“, Hinweise zur Reihenfolge und das erneute Lesen von Intent und Tools.
---

# Modus refine

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 3, wenn `REFINE = ja` ist.

Mit `REFINE = ja` fragt Schritt 3 nicht, sondern lässt genau die ungeprüften Tasks nach
`k-playbook:/commands/_task-refine/gruppen.md` prüfen — vor dem Nehmen der Sperre, damit sie
während des Prüfens nicht gehalten wird. Der Hauptkontext liest dieses Modul, Aufrufer
`/k-task-run`; es sagt, was für `/k-task-run` gilt (Rückfall ohne zweite Ebene, Rückgaben,
Weiterlaufen nur nach `Ergebnis: geprüft`). Er moderiert nicht selbst und sieht keine Ausgabe
von Critic und Editor.

- **Ohne Argument** (`/k-task-run refine`): die ungeprüften Tasks der Liste sind
  `REVIEW_TARGETS`, die geprüften `UEBRIGE_TASKS`; Gruppen und Überschneidungs-Critic wie dort.
  Keine Startübersicht, keine Schluss-Fertig-Meldung, kein Folge-Command des Refine-Commands.
- **Mit Argument** (`/k-task-run <ref-or-number> refine`): Ist das Ziel ungeprüft, prüft es
  ein Sub-Agent als Gruppe mit einem Task, ohne Überschneidungs-Critic und ohne Hinweise zur
  Reihenfolge. Ein schon geprüftes Ziel geht nicht in den Refine; die Rückfrage „schon
  geprüft“ kommt nicht vor.
- **Danach** in der Ausgabe kurz je Ziel `Refine: <TASK_FILE> — <Ergebnis>, Runden <n>,
  gefixt <n>, offen <n>`, dieselben Zeilen später in Schritt 7. Hinweise zur Reihenfolge aus
  dem Überschneidungs-Critic werden angezeigt; die Reihenfolge bleibt die nach Nummer.
  Empfiehlt ein Hinweis eine andere Reihenfolge, ist das eine Rückfrage an den Nutzer
  (Fertig-Meldung vorher, `Rückfrage: Reihenfolge`): nach Nummer weiter oder abbrechen.
- Dann „Intent und Tools lesen“ (Schritt 2) erneut. Weiter mit der Rückfrage
  „Ohne Task-Refine“ im Command für zurückgefallene Tasks, sonst mit Schritt 4, ohne erneute
  Rückfrage.
