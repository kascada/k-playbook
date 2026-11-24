---
title: Review starten
description: Modul für /k-task-run, vom Hauptkontext gelesen in 5f, Schritt 3 und 4 — das wirksame Review-Rezept aus dem Briefing bestimmen, den Review-Sub-Agenten mit seinen Werten starten, die Rückgabe auf Gültigkeit prüfen und die Revision setzen.
---

# Review starten

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5f, wenn der Task den Kopf aus Schritt 2
trägt und `DIFF_ENABLED=true` mit Snapshot gilt. Er hat `TASK_REF`, `REVISION`, `EXEC_ROOT`,
`SNAPSHOT`, `REVIEW_SCOPE`, `DIFF_REVIEW_MODULE` und die Zeilen „Validierung“ aus der letzten
Rückgabe von Ausführung oder Behebung.

**3. Rezept bestimmen**, ohne es zu lesen: aus `documents.reviews` des Briefings der Eintrag
mit `ref` `k-playbook:/reviews/review-code.md`. So greift ein Overlay unter
`keph:/k-playbook/reviews/`.

- Vorhanden, Herkunft nicht `disabled`: `REVIEW_RECIPE = <ref>` — der Sub-Agent führt Teil 1
  und Teil 2 des Moduls aus.
- Herkunft `disabled` (leere projekteigene Datei): abgeschaltet. Der Sub-Agent führt nur
  Teil 1 aus; die Diff-Abschnitte bleiben, das Review entfällt, und Schritt 7 weist
  „Review: abgeschaltet“ aus. Eine Stopp-Abfrage aus dem Review gibt es dann nicht; meldet
  Teil 1 `Diff: fehlgeschlagen`, nennt Schritt 7 das dahinter.
- Fehlt der Eintrag: Der Sub-Agent führt ebenfalls nur Teil 1 aus. Das gilt als ungültiges
  Review (5f, Schritt 5).

`review.enabled` und `audit.enabled` steuern nur `/k-review` und `/k-audit` und wirken hier
nicht.

**4. Review-Sub-Agent.** Vor dem Start mit `k_playbook_doc_grep` und `count_only` zählen,
wie oft `^### Review: Befunde$` und `^### Diff: Geänderte Dateien$` im Task stehen; die
Revision aus dieser Antwort muss `REVISION` sein (Replica-Nachlauf nach
`k-playbook:/instructions/on-demand/tasks.md`). Dann einen neuen General-Purpose-Sub-Agenten
starten — nicht den ausführenden: Wer den Code geschrieben hat, prüft ihn nicht selbst. Er
bekommt:

- den Verweis `DIFF_REVIEW_MODULE` mit der Anweisung, das Modul mit `k_playbook_doc_read` zu
  lesen und zu befolgen,
- `TASK_REF` und `REVISION`, `projectDir` (`project.dir`), die Ausführungswurzel,
  `SNAPSHOT` und `REVIEW_SCOPE`,
- die Zeilen „Validierung“ aus der letzten Rückgabe von Ausführung oder Behebung,
- `REVIEW_RECIPE` oder die Anweisung „nur Teil 1“.

Er fragt den Nutzer nicht und hängt seine Abschnitte selbst an den Task an. Seine Rückgabe
beschreibt das Modul; sie endet mit `Revision: <n>` oder `Foreign-Change: …`. Danach
dieselben Zeilen erneut zählen:

- Das Review ist **gültig**, wenn die Rückgabe dem Format des Moduls folgt — mit den Blöcken
  `Kritisch:` und `Wichtig:`, jeder mit Befundzeilen oder `keine` — und `### Review: Befunde`
  genau einmal mehr dasteht als vorher. Eine Rückgabe ohne `Wichtig:`-Block ist wie jede
  formwidrige kein gültiges Review. Nie gilt ein Lauf ohne das als „0 kritisch“. Der Block
  `Hinweise:` gehört zum Format, entscheidet aber nicht über die Gültigkeit: Fehlt er oder ist
  er formwidrig, oder stimmt die Zahl der Hinweise in `Review: …` nicht mit seinen Zeilen
  überein, bleibt das Review gültig; es gibt dann keine Hinweiszeilen, und die Zusammenfassung
  setzt hinter die Zeile „Review“ den Zusatz „— Hinweise nicht übermittelt“.
- Die Diff-Abschnitte hat der Lauf geschrieben, wenn `### Diff: Geänderte Dateien` genau
  einmal mehr dasteht.
- `REVISION` wird nach der Revisionskette gesetzt; `Foreign-Change:` führt zur Rückfrage aus
  Schritt 1, und nach „weiter“ läuft Schritt 4 erneut.
