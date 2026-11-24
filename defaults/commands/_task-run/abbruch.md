---
title: Abbruchvermerk
description: Modul für /k-task-run, vom Hauptkontext gelesen in 5e bei Status teilweise, nach 5d abgebrochenem Task oder offener Etappe in 5f — der Kopf `## Ausführung` mit der Kennzeile „abgebrochen“, Grund, erledigte Etappen und was für den nächsten Lauf stehen bleibt.
---

# Abbruchvermerk

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5e. Er hat `TASK_FILE`, `REVISION`,
die Rückgabe des Sub-Agenten (oder den Befund aus 5f, Schritt 1) und die Zahl `m` der
Etappen-Überschriften.

Mit `k_playbook_doc_append` und
`since_revision: REVISION` anhängen, die Kennzeile „abgebrochen“ wie in 5f als erste Zeile nach
`## Ausführung` und der Leerzeile:

```text
## Ausführung

<!-- k-task-run: abgebrochen <TASK_FILE> -->
**Status:** Teilweise ausgeführt - abgebrochen  
**Datum:** <now.date>  
**Abgebrochen bei:** <Schritt oder Etappe aus der Rückgabe>  
**Grund:** <kurzer Grund>
```

„Abgebrochen bei“ kommt aus dem gleichnamigen Feld der Rückgabe, „Grund“ aus „Blocker“
oder, steht dort „keine“, aus der Zusammenfassung.

- Der Task bleibt, wo er ist (nicht nach `done/`).
- Hat der Task `### Etappe`-Abschnitte: Die Fortschrittszeilen bleiben, wie der Sub-Agent
  sie angehängt hat. Sie sind der Wiederaufnahmepunkt für den nächsten `/k-task-run` — nie
  zurücksetzen, nie eine Etappe als erledigt eintragen, die der Sub-Agent nicht selbst
  eingetragen hat. Füge dem Vermerk oben als letzte Zeile `**Etappen erledigt:** <n> von <m>`
  an; `<n>` ist die Zahl der Etappen mit Fortschrittszeile nach dem letzten Zurücksetzen
  (Zählung wie in 5a.2), `<m>` die Zahl der Etappen-Überschriften.
- Keine weiteren Tasks.
