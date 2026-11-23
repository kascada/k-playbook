---
title: Abschluss eines Tasks
description: Modul für /k-task-run, vom Hauptkontext gelesen in 5f, Schritt 6 — die Befunde des Tasks für Schritt 7 sammeln (samt Start des Befund-Lesers), den Task mit k_playbook_doc_done nach done/ verschieben (neue Revision, outcome_unknown), die geänderten Pfade gegen den Snapshot für den Doku-Nachzug merken und die Snapshot-Ref freigeben.
---

# Abschluss eines Tasks

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5f, Schritt 6, für jeden Task, der nach
`done/` geht. Er hat `TASK_REF`, `TASK_STEM`, `EXEC_ROOT`, `SNAPSHOT` (oder keinen) und die
Liste `CHANGED_PATHS` des Laufs.

## Befunde sammeln

Für jeden Task, der nach `done/` geht, vor dem Verschieben. Der Hauptkontext
merkt sich je Task die Befundzeilen für die Zusammenfassung und die Abfrage in Schritt 7,
unverändert in der Form `<datei:zeile oder Prüfbefehl> — <ein Satz>`, jede kritische mit dem
Merkmal „kritisch“; was gesammelt wird, sagt die Spalte „Für Schritt 7 gesammelt“. Geht ein
Task mit ungültigem Review (nach „(b) trotzdem weiter“) nach `done/`, sucht der Hauptkontext
mit `k_playbook_doc_grep` (mit Zeilennummern) die Kennzeile
`^<!-- k-task-run: ausgeführt <TASK_FILE> -->$` — der letzte Treffer ist die dieses Laufs —
und startet mit ihrer Zeilennummer den Befund-Leser (5a). Gesammelt werden die Zeilen unter
`Kritisch:` und `Wichtig:` aus dessen Rückgabe; bei `Review: kein Abschnitt` oder einer
Rückgabe in anderer Form nichts. Ein angehaltener Task (c) behält seine Befunde im Dokument;
sie werden nicht gesammelt. Schließt ein späterer Lauf ihn in 5a mit „(a) abschließen“ ab,
sammelt der Befund-Leser sie dort.

## Befund-Leser

Ein General-Purpose-Sub-Agent (OpenCode: `general`, Claude Code: `general-purpose`), reines Nachschlagen ohne Briefing, der im Task den letzten Review-Abschnitt
liest. Er läuft hier und in `nicht-abgeschlossen.md`. Er bekommt den Verweis
`k-playbook:/commands/_task-run/befund-leser.md`, `TASK_REF`, `projectDir` und die
Zeilennummer, nach der er sucht. Seine Rückgabe ist gültig, wenn sie genau
`Review: kein Abschnitt` lautet oder der Form des Moduls folgt, mit den Blöcken `Kritisch:` und
`Wichtig:`. Er gibt keine Revision zurück; `REVISION` bleibt.

## Verschieben, Pfade, Snapshot

1. `k_playbook_doc_done` mit `TASK_REF`. Die Antwort nennt den neuen Verweis unter `done/`
   (`TASK_REF_DONE`) und `revision` — das Verschieben erzeugt eine neue Revision, und sie ist
   ab jetzt `REVISION` für alles, was noch an diesen Task angehängt wird (Schritt 6).
   `foreign_change` gibt es hier nicht. `outcome_unknown` heißt nach
   `k-playbook:/instructions/on-demand/tasks.md`, „`outcome_unknown` je Operation“: mit
   `k_playbook_doc_list` (Sorte `tasks`, `done: true`) nachsehen, ob das Dokument unter
   `done/` liegt, dann die Revision mit `k_playbook_doc_grep` und `count_only` unter
   `TASK_REF_DONE` holen; nicht noch einmal verschieben, solange es dort liegt. Ein
   `not_found` unter `TASK_REF_DONE` direkt nach dem Verschieben ist der Replica-Nachlauf.
2. Geänderte Pfade merken — nur die Namen, für den Doku-Nachzug in Schritt 7. Mit Snapshot
   vergleicht ein temporärer Index den Stand nach dem Task mit ihm, neu angelegte Dateien
   eingeschlossen; Inhalte kommen nicht in den Hauptkontext:

   ```bash
   if command -v git >/dev/null 2>&1; then
     (
       cd "<EXEC_ROOT>" || exit 1
       tmp_dir="$(mktemp -d)" || exit 1
       cp "$(git rev-parse --git-path index)" "$tmp_dir/index" 2>/dev/null || true
       GIT_INDEX_FILE="$tmp_dir/index" git add -A &&
         GIT_INDEX_FILE="$tmp_dir/index" git diff --cached --name-only "<SNAPSHOT>"
       status=$?
       rm -rf "$tmp_dir"
       exit "$status"
     )
   fi
   ```

   Die Ausgabe ergänzt `CHANGED_PATHS`, die Pfade aller Tasks dieses Laufs, relativ zur
   Ausführungswurzel. Ohne Snapshot gibt es diese Liste nicht; dann merkst du dir den Task als
   „Pfade unbekannt“.
3. Snapshot freigeben — die Ref wird mit dem Verschieben nach `done/` entfernt:

   ```bash
   if command -v git >/dev/null 2>&1; then
     git -C "<EXEC_ROOT>" update-ref -d "refs/k-task-run/<TASK_STEM>"
   fi
   ```
