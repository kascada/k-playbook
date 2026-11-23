---
title: Snapshot des Tasks
description: Modul für /k-task-run, vom Hauptkontext gelesen in 5b.1 mit DIFF_ENABLED=true — den Snapshot eines Tasks als Tree-Objekt anlegen oder aus der Snapshot-Zeile wiederverwenden, die Ref setzen, die Zeile anhängen und REVIEW_SCOPE bestimmen.
---

# Snapshot des Tasks

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5b.1, wenn `DIFF_ENABLED=true`. Er hat
`TASK_FILE`, `TASK_STEM`, `EXEC_ROOT` und `REVISION`. Ergebnis sind `SNAPSHOT` (die Kennung)
und `REVIEW_SCOPE`.

1. Mit `k_playbook_doc_grep` die Snapshot-Zeile dieses Tasks suchen
   (`^<!-- k-task-run: snapshot <TASK_FILE> [0-9a-f]+ -->$`); maßgeblich ist der letzte
   Treffer.
2. Trifft sie und lässt sich die Kennung auflösen, wird sie wiederverwendet:
   `SNAPSHOT=<kennung>`, `REVIEW_SCOPE=ab Task-Beginn`. Die Ref wird dabei gesetzt, falls
   sie fehlt:

   ```bash
   if command -v git >/dev/null 2>&1; then
     git -C "<EXEC_ROOT>" cat-file -e "<SNAPSHOT>^{tree}" &&
       git -C "<EXEC_ROOT>" update-ref "refs/k-task-run/<TASK_STEM>" "<SNAPSHOT>"
   fi
   ```

3. Sonst einen neuen Snapshot anlegen. Er entsteht als Tree-Objekt über einen temporären
   Index; der echte Index und der Arbeitsbaum bleiben unberührt. Gegen `git gc` bleibt er
   über die Ref `refs/k-task-run/<TASK_STEM>` erreichbar, die mit dem Verschieben nach
   `done/` wieder entfernt wird. Die Ausgabe ist allein die Kennung:

   ```bash
   if command -v git >/dev/null 2>&1; then
     (
       cd "<EXEC_ROOT>" || exit 1
       snap_dir="$(mktemp -d)" || exit 1
       cp "$(git rev-parse --git-path index)" "$snap_dir/index" 2>/dev/null || true
       GIT_INDEX_FILE="$snap_dir/index" git add -A &&
         tree="$(GIT_INDEX_FILE="$snap_dir/index" git write-tree)" &&
         git update-ref "refs/k-task-run/<TASK_STEM>" "$tree" &&
         printf '%s\n' "$tree"
       status=$?
       rm -rf "$snap_dir"
       exit "$status"
     )
   fi
   ```

   Dann die Snapshot-Zeile mit `k_playbook_doc_append` und `since_revision: REVISION`
   anhängen:

   ```text
   <!-- k-task-run: snapshot <TASK_FILE> <SNAPSHOT> -->
   ```

   `REVIEW_SCOPE` ist `ab Task-Beginn`, wenn der Task noch nie lief. Lief er schon — das
   Dokument trägt einen Vermerk aus 5e (die Kennzeile „abgebrochen“ oder, aus der Zeit vor
   ihr, die Zeile `^\*\*Status:\*\* Teilweise ausgeführt - abgebrochen`), eine Etappe stand
   auf erledigt, oder eine Snapshot-Zeile ließ sich nicht mehr auflösen —, ist er
   `nur seit Fortsetzung`: Das Review deckt dann nur die Änderungen seit diesem Snapshot ab,
   sagt das in Abschnitt und Rückgabe, und 5f fragt vor dem Verschieben nach `done/` (5f,
   Schritt 5).
