---
title: Revisionskette
description: Baustein für /k-task-run und /k-task-refine und ihre Module, eingebunden als include — wie ein Schreiber an einem Task die Revisionskette hält (since_revision, Zählen, Replica-Nachlauf, outcome_unknown), was foreign_change für den Hauptkontext und für einen Sub-Agenten heißt und welche Rückgabezeilen ein schreibender Sub-Agent liefert.
---

**Revisionskette.** Ein Task hat zu einer Zeit einen Schreiber. Verbindlich ist
`k-playbook:/instructions/on-demand/tasks.md`, „Lesen und schreiben“ (lies sie mit
`k_playbook_doc_read`). Für diesen Lauf gilt:

- **`REVISION`** ist die Revision deines letzten eigenen Schreibens an diesem Task — aus der
  Antwort von `append`, `replace`, `done` oder `reopen`; vor dem ersten Schreiben die
  übergebene bzw. die aus `k_playbook_doc_read` oder `k_playbook_doc_grep` mit `count_only`.
  Jedes Anhängen und Ändern trägt `since_revision: REVISION`.
- **Zählen.** Wird die Ankunft eines Anhängens später an einer Zeile geprüft, zählst du sie
  vorher mit `k_playbook_doc_grep` und `count_only`. Die Revision der Zählung muss `REVISION`
  sein; liegt sie darunter, läuft die Replica nach (kurz warten, bis zu dreimal neu zählen),
  eine höhere oder weiter abweichende gilt wie `foreign_change`.
- **`outcome_unknown`:** nicht blind wiederholen, sondern nachlesen („`outcome_unknown` je
  Operation“ in `tasks.md`); angekommen ist ein Anhängen, wenn die gezählte Zahl gestiegen ist.
- **`foreign_change`** heißt: Ein anderer hat geschrieben, nichts wurde geschrieben.
  - **Hauptkontext** (führt das Gespräch mit dem Nutzer): Fertig-Meldung
    (`Rückfrage: fremder Schreiber`), dann die Rückfrage mit `revision`, `updatedBy` und
    `updatedAt` aus der Antwort, wer weitermacht. Bei „weiter“ gilt die Revision aus dieser
    Antwort als `REVISION`, und der unterbrochene Schritt läuft erneut; sonst endet die Arbeit
    an diesem Task ohne weiteren Eintrag.
  - **Sub-Agent:** anhalten, nichts weiter schreiben, statt `Revision: <n>` die Zeile
    `Foreign-Change: <revision> <updatedBy> <updatedAt>` zurückgeben, den Nutzer nicht fragen.
- **Übergabe an einen schreibenden Sub-Agenten:** Der Hauptkontext zählt vorher die Zeile, die
  er schreiben soll, und gibt `TASK_REF` und `REVISION` mit. Der Sub-Agent gibt `Revision: <n>`
  zurück — die Revision seines letzten Schreibens, ohne Schreiben die übergebene; daraus setzt
  der Hauptkontext `REVISION`. Fehlen beide Zeilen, zählt er erneut (`tasks.md`,
  „Sub-Agenten“, Punkt 3). `Foreign-Change:` behandelt er wie ein eigenes `foreign_change`.
