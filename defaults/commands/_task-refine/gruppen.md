---
title: Gruppenprüfung
description: Modul für /k-task-refine und /k-task-run (Modus refine), vom Hauptkontext gelesen — mehrere Tasks in Gruppen zusammenhängender Tasks bilden, je Gruppe einen Sub-Agenten prüfen lassen (Rückfall je Client, Übergabe, Auswerten der Rückgaben, Fortsetzung) und danach den Überschneidungs-Critic laufen lassen; samt Tabelle, was je Aufrufer gilt.
---

# Gruppenprüfung

Dieses Modul liest der Hauptkontext, wenn mehrere Tasks gemeinsam durch den Review-Dialog
gehen. Den Dialog selbst führt der Sub-Agent der Gruppe nach
`k-playbook:/commands/_task-refine/dialog.md`; Rückgabeformat und Regeln des Sub-Agenten
stehen dort. Der Hauptkontext moderiert nicht selbst, außer im Rückfall.

**Was je Aufrufer gilt.** `REVIEW_TARGETS` sind die zu prüfenden Tasks, `UEBRIGE_TASKS` die
schon geprüften offenen Tasks, die nicht Ziel sind.

| | `/k-task-refine` ohne Ziel | `/k-task-run … refine` ohne Ziel | `/k-task-run <ziel> refine` |
|---|---|---|---|
| Gruppen | `REVIEW_TARGETS` in Gruppen | die ungeprüften Tasks der Liste in Gruppen | eine Gruppe mit einem Task |
| Überschneidungs-Critic | ja | ja | nein, auch keine Hinweise zur Reihenfolge |
| Rückfall (Client kann keinen Sub-Agenten in einem Sub-Agenten starten) | Hauptkontext prüft die Gruppen selbst nach `dialog.md`, je Gruppe nacheinander | kein Selbstprüfen: Alle ungeprüften Tasks fallen auf die Rückfrage „Ohne Task-Refine“ zurück, Grund „refine: dieser Client kann keine Sub-Agenten in Sub-Agenten starten“ | wie in der Spalte davor |
| `keine-subagenten`, `abgebrochen` ohne `Foreign-Change:`, fehlende oder formwidrige Rückgabe, abgelehnte Fortsetzung | `keine-subagenten`: Hauptkontext prüft die Gruppe selbst; `abgebrochen`: keine Fortsetzung, der Grund steht in der Zusammenfassung | dieser Task fällt auf die Rückfrage „Ohne Task-Refine“ zurück, die den Grund nennt | wie in der Spalte davor |
| Ohne Rückfrage weiter | – | nur ein Task mit `Ergebnis: geprüft`, auch nach einer Fortsetzung | wie in der Spalte davor |

Startübersicht und Zusammenfassung gehören dem Command: `/k-task-refine` hat sie in seinem
Text, `/k-task-run` hat eigene.

## Gruppen bilden

Der Hauptkontext teilt `REVIEW_TARGETS` bei jedem Lauf neu in Gruppen. Zum Urteilen liest er
je Task nur `## Referenzen`, `## Ziel` und `## Zu bauen` (`k_playbook_doc_read` mit
`section`; fehlt einer, gilt `section_not_found` als leer), nicht den ganzen Task.

**Zusammenhang** — eng, im Zweifel gemeinsam. Zwei Tasks hängen zusammen, wenn

- einer den anderen als Voraussetzung, Abhängigkeit oder gemeinsames Ziel nennt (Tasknummer
  oder `keph:`-Verweis), oder
- beide dieselbe Datei ändern.

Eine Nummer, die nur als Beispiel oder als Herkunft dasteht, zählt nicht. Eine Datei, die ein
Task nur liest oder als Vorbild nennt, zählt auch nicht: Nennt Task A
`project:/defaults/commands/k-task-run.md` nur als Vorbild und ändert Task B sie, verbindet
das die beiden nicht. Ob ein Task eine Datei ändert, steht nicht maschinenlesbar im Task; du
urteilst nach den drei Abschnitten. Lässt es sich nicht sicher sagen, gelten die Tasks als
zusammenhängend.

Der Zusammenhang gilt transitiv: Hängt A mit B und B mit C zusammen, sind alle drei eine
Gruppe. Ein Task ohne Zusammenhang ist eine Gruppe für sich. Halte je Gruppe mit mehr als
einem Task den Grund in einem Halbsatz fest. Die Tasks einer Gruppe bleiben nach Nummer
sortiert, die Gruppen nach ihrer kleinsten Nummer.

## Gruppen prüfen

**Rückfall je Client.** Den Lauf je Gruppe in einem Sub-Agenten gibt es nur, wo ein
Sub-Agent selbst Sub-Agenten starten kann, denn er startet Critic und Editor:

- **Claude Code:** Ein `general-purpose`-Sub-Agent hat das Werkzeug `Agent`, und der
  Sub-Agent, den er startet, ruft `mcp__k-playbook__*` auf (gemessen 2026-11-20). Der Lauf je
  Gruppe gilt.
- **OpenCode:** Ein Sub-Agent `general` kann keinen Sub-Agenten starten — ihm fehlt das
  Werkzeug `subagent` (gemessen 2026-11-20, v2.3.0, Grundeinstellung). Hier gilt der
  Rückfall der Tabelle oben; bei `/k-task-refine` führt der Hauptkontext den Dialog nach
  `dialog.md` von Anfang an selbst, je Gruppe nacheinander, mit den Tasks der Gruppe als
  `REVIEW_TARGETS`, und startet keinen Sub-Agenten je Gruppe. Critic und Editor startet er wie
  sonst. Danach folgt der Überschneidungs-Critic wie sonst.

Ein anderer Client folgt dem Lauf je Gruppe; gibt ein Sub-Agent `keine-subagenten` zurück,
gilt die Zeile der Tabelle.

**Übergabe.** Vor der Übergabe hält der Hauptkontext die Revisionskette nach
`k-playbook:/instructions/on-demand/tasks.md`, „Sub-Agenten“: je Task `^## Review-Log`
gezählt (`count_only`) und Anzahl und Revision gemerkt — eine frühere Zählung genügt, wenn
seitdem niemand in den Task geschrieben hat. Dann startet er je Gruppe einen Sub-Agenten, alle
in einer Nachricht.

include: k-playbook:/commands/_task/sub-agenten.md

Der Auftrag an den Sub-Agenten einer Gruppe

- nennt als seine Anleitung `k-playbook:/commands/_task-refine/dialog.md`, gelesen mit
  `k_playbook_doc_read`; er ist darin der Moderator für genau die Tasks seiner Gruppe — bei
  mehreren gemeinsam, als `REVIEW_TARGETS` —, startet Critic und Editor selbst, wendet Edits an
  und hängt an jeden Task der Gruppe das Review-Log an. Den Command-Text liest er nicht,
- nennt je Task der Gruppe den `keph:`-Verweis und die Revision sowie `projectDir`
  (`context.project.dir`),
- bei einer Fortsetzung zusätzlich: den `Zwischenstand:` des vorigen Sub-Agenten, die
  Revisionen aus seinen Zeilen `Revision:` bzw. `Foreign-Change:` und die Antwort des Nutzers
  als Entscheidung.

**Auswerten.** Der Hauptkontext sammelt die Rückgaben aller Gruppen und arbeitet sie danach
Gruppe für Gruppe ab:

- `geprüft` — fertig; die Zahlen gehen in die Zusammenfassung des Commands.
- `rückfrage` — Fertig-Meldung (`Rückfrage: <worum es geht>`), dann die Frage der Gruppe
  einzeln an den Nutzer. Mit seiner Antwort startet er für diese Gruppe einen neuen
  Sub-Agenten, für ihre Tasks, die noch kein Review-Log tragen, mit Zwischenstand, Revisionen
  und der Antwort als Entscheidung (Übergabe oben).
- `Foreign-Change:` — Fertig-Meldung (`Rückfrage: fremder Schreiber`), dann fragen, wer
  weitermacht, mit `revision`, `updatedBy` und `updatedAt` aus der Zeile. Sagt der Nutzer
  „weiter“, startet er einen neuen Sub-Agenten für die Tasks der Gruppe ohne Review-Log, mit
  der Revision aus der Zeile `Foreign-Change:` als nächstem `since_revision` und dem
  Zwischenstand. Sonst bleibt die Gruppe ohne Review-Log.
- `keine-subagenten`, `abgebrochen` ohne `Foreign-Change:`, fehlende oder formwidrige
  Rückgabe, abgelehnte Fortsetzung — die Zeile der Tabelle oben.
- Fehlen bei einem Task beide Zeilen `Revision:` und `Foreign-Change:`, zählt der Hauptkontext
  `^## Review-Log` erneut, nach `k-playbook:/instructions/on-demand/tasks.md`, „Sub-Agenten“,
  Punkt 3.

Sind alle Gruppen fertig — auch nach Rückfragen und Fortsetzungen —, läuft der
Überschneidungs-Critic.

## Überschneidungen prüfen

Ein Critic je Gruppe sieht die anderen Gruppen nicht, und die schon geprüften offenen Tasks
sieht ohne diesen Schritt kein Critic. Das Urteil über den Zusammenhang stützt sich nur auf
drei Abschnitte je Task, und ein gemeinsamer Vertrag in verschiedenen Dateien verbindet zwei
Tasks dort gar nicht. Deshalb läuft einmal ein schlanker Critic über die Ziele des Laufs und
`UEBRIGE_TASKS`.

Er läuft einmal, wenn alle Gruppen fertig sind, auch nach Rückfragen und Fortsetzungen, und
liest die Tasks im Stand danach — sobald es zu einer Gruppe etwas zum Vergleichen gibt: eine
weitere Gruppe des Laufs oder einen Task in `UEBRIGE_TASKS`. Er entfällt nur bei genau einer
Gruppe und leerem `UEBRIGE_TASKS`, weil ihr Critic dann schon alles gemeinsam gesehen hat, und
in der letzten Spalte der Tabelle. Im Rückfall gilt derselbe Schritt. Der Hauptkontext startet
ihn selbst auf der ersten Ebene, die jeder Client kann, und übergibt den Auftrag unten
**wörtlich**, mit eingesetzten Werten; der Critic liest dieses Modul nicht. Er schreibt nichts,
hält deshalb keine Revisionskette und sendet keine Fertig-Meldung.

```
Führe zuerst `k-playbook briefing` aus und lies die Ausgabe.
You are the overlap Critic of a task review run.
You are read-only. Do not modify documents or files or call write/edit tools.

## Own open tasks
<one line per own open task: the keph: reference>
Read each task yourself with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <context.project.dir>); ignore everything from the first line "## Review-Log" on.

## Groups of this run
<one line per group: the task numbers>
Tasks in no group were reviewed earlier; they are not targets of this run.

Report only overlaps and contradictions
- between tasks of different groups, and
- between a target of this run and a task in no group.
Never report a pair without a target of this run, and never a pair inside one group.
An overlap is: the same file changed by both; the same contract (marker line, return format, call form) defined differently; an order one task needs from the other.
Say nothing about single tasks.

Output, exactly this format, no intro text — one line per pair:
Überschneidung: <nr> + <nr> — <was> — Empfehlung: <one line, e.g. "<nr> vor <nr> ausführen">
or, if there are none:
Überschneidungen: keine
```

Die Treffer gehen unverändert in die Zusammenfassung des Commands. Der Hauptkontext ändert
daraufhin nichts, hängt kein weiteres Review-Log an und fragt nicht nach: Die Treffer ändern
kein Review-Log, und die Reihenfolge entscheidet der Nutzer beim Ausführen.
