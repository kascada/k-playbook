---
title: Ausführungskontext und Branch-Preflight
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn ein Task einen Abschnitt `## Ausführungskontext` trägt — die Werte (Target repo, Base branch, Work branch, PR required, Dirty worktree policy), die Ausführungswurzel und der Branch-Preflight bis zum Rahmen für den Sub-Agenten.
---

# Ausführungskontext und Branch-Preflight

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5a.1, wenn der Task einen Abschnitt
`## Ausführungskontext` trägt. Es ersetzt den kurzen Preflight des Commands für diesen Task.

Trägt der Task einen Abschnitt `## Ausführungskontext`, nimm daraus, soweit vorhanden:

- `Target repo:`
- `Base branch:`
- `Work branch:`
- `PR required:`
- `Dirty worktree policy:`

Behalte die Werte für den Erfolgspfad. Ist `PR required` wahr, muss der Command nach dem
lokalen Commit einen PR eröffnen oder genau den fehlenden Schritt nennen, der das verhindert.

`Target repo` wird relativ zu `project.repoRoot` aufgelöst, wenn es nicht absolut
ist. Steht es als Verweis `project:/<pfad>` da — so schreibt es `/k-remediation` —, gilt
`<project.dir>/<pfad>`; `project:/` allein ist die Projektwurzel. Ohne
`Target repo` ist die Projektwurzel die Ausführungswurzel (`EXEC_ROOT`).

Vor der Delegation der Branch-Preflight in der Ausführungswurzel:

1. Prüfe, dass sie existiert und ein Git-Repository ist (`git rev-parse --is-inside-work-tree`).
   Wenn nicht: anhalten und den Nutzer fragen.
2. Prüfe den Dirty-State mit `git status --short`.
3. Ist er nicht leer: anhalten und die Dateien zeigen. Weiter nur, wenn der Nutzer ausdrücklich
   bestätigt, dass diese Dateien für den Task erwartet sind; sonst nicht delegieren.
4. Ist `Work branch` gesetzt (vor einem Wechsel `check` der Sperre):
   - schon darauf: weiter;
   - existiert er lokal: `git switch <Work branch>`;
   - sonst, wenn `Base branch` gesetzt und nicht `<manual>` ist: auf `Base branch` wechseln und
     `git switch -c <Work branch>`;
   - sonst anhalten und fragen, welcher Base-Branch gelten soll.
5. Sind `Base branch` und `Work branch` gesetzt und `Base branch` ist nicht `<manual>`: prüfe
   mit `git merge-base --is-ancestor <Base branch> HEAD`, dass der Work-Branch darauf aufsetzt.
   Schlägt das fehl: anhalten und fragen.
6. Halte das Ergebnis fest und gib es dem Sub-Agenten mit. Ausführungswurzel und Branch sind
   für ihn verbindlicher Rahmen.
