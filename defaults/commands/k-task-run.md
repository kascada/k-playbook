# k-task-run

Führt Tasks aus der Projekt-Collection aus. Ohne Argument die eigenen offenen Tasks in
Nummernreihenfolge; mit Argument einen `keph:`-Verweis oder eine Tasknummer. Steht dahinter
— oder allein — das Wort `refine`, gehen ungeprüfte Tasks vorher ungefragt durch
`/k-task-refine` (Schritt 3).

In einem Projekt ändert zu einer Zeit höchstens ein `/k-task-run` den Arbeitsbaum: Der Lauf
nimmt vor der ersten Änderung die Sperre bei k-playbook und prüft an festen Stellen nach, ob er
sie noch hält („Sperre des Arbeitsbaums“).

Der Hauptkontext steuert und fragt; Ausführung, Diff und Code-Review laufen je Task in
Sub-Agenten, die selbst an den Task anhängen und ein knappes Format zurückgeben.

Produces:

- An jedem ausgeführten Task `## Ausführung` mit Kennzeile, Status und Zusammenfassung, die
  Diff- und Review-Abschnitte des Review-Sub-Agenten und bei Etappen die Fortschrittszeilen des
  Ausführungs-Sub-Agenten — alles angehängt.
- Erfolgreich abgeschlossene Tasks unter `keph:/user/~/tasks/done/`; je Task bis dahin eine Ref
  `refs/k-task-run/<TASK_STEM>` in der Ausführungswurzel.
- Bei `PR required: true` einen Pull Request über `gh` (5f.1); hat der Lauf Code geändert, je
  Erzeuger höchstens einen Doku-Auftrag in `keph:/queue/docs/`, und je übernommenem
  Review-Befund ein Todo (Schritt 7).
- Mit `refine` an jedem ungeprüften Task das `## Review-Log` von `/k-task-refine` (Schritt 3).

**Module.** Sonderfälle und Aufträge an Sub-Agenten stehen in Modulen, Verweisen
`k-playbook:/commands/_task-run/<datei>.md`, `_task/…` und `_task-refine/…`. Der Hauptkontext
liest ein Modul mit `k_playbook_doc_read` (`projectDir` = `context.project.dir`) erst, wenn der
Fall eintritt — nie vorsorglich —; einem Sub-Agenten, der ein Modul selbst liest, gibt er den Verweis, die Werte
des Moduls und `_task/sub-agenten.md` (Briefing-Zeile, Lesewege). Eine gleichnamige Datei unter
`<context.project.dir>/.k-playbook/commands/` ersetzt ein Modul (Überlagerung nach Dateipfad).

## Schritt 1 — Briefing und Form

include: _shared/erster-schritt.md

Verbindlich für Ablage, Nummernauflösung, Kennzeilen, Fortschrittszeilen und den Schreibweg
eines Tasks ist `k-playbook:/instructions/on-demand/tasks.md`; lies sie mit
`k_playbook_doc_read`, bevor du einen Task liest. Der Command braucht die Collection: Steht
`context.kephalaion.ok` nicht auf `true`, endet er mit `context.kephalaion.message`
(Abschnitt „Ohne Kephalaion“ dort).

**Hauptkontext und Task-Dokument.** Der Hauptkontext liest einen Task nur mit
`k_playbook_doc_read` und `section` oder `lines` (nie ganz, nie mit Kephalaion `read`), sucht
mit `k_playbook_doc_grep` (`count_only` für Anzahl und Revision) und schreibt nur mit
`k_playbook_doc_append`; er ändert keine bestehende Zeile. Nach dem Kopf (5f, Schritt 2) liest
er keinen Abschnitt des Tasks mehr — Ausnahme 5a, Option (a): `## Ausführungskontext`
(`section`, erster Treffer). `### Review: Befunde` liest er nie; die Befunde kennt er nur aus
Rückgaben: Diff und Review stehen im Task-Dokument, und der Hauptkontext sieht sie nicht.

include: _task/revisionskette.md

Command-eigen: Der Hauptkontext merkt sich je Task `REVISION`; vor dem ersten Schreiben ist es
die Revision aus dem ersten `k_playbook_doc_grep` in 5a (nach dem Wieder-Öffnen die aus dessen
Antworten). Nach einem `foreign_change` ohne „weiter“ laufen keine weiteren Tasks.

## Fertig-Meldung

include: _task/fertig-meldung.md

## Sperre des Arbeitsbaums

Die Sperre hält und vergibt k-playbook: Was das Werkzeug `k_playbook_lock` (MCP-Server
`k-playbook`, `projectDir` = `context.project.dir`) sagt, gilt. Gesperrt wird je Projekt; die
Sperre nimmt und prüft allein der Hauptkontext, nie ein Sub-Agent. Ein Task mit `Target repo`
außerhalb des Projekts ist durch sie nicht geschützt.

- **Marke.** `acquire` und `wait` geben bei `state: acquired` eine Marke `token` zurück. Der
  Hauptkontext merkt sie sich als `LOCK_TOKEN` und gibt sie bei `check` und `release` mit;
  sie ist für die Sperre, was `REVISION` für den Task ist. Er nimmt nie neu, solange er eine
  Marke hält, und nie still nach einem Verlust.
- **Sonderfälle** — `acquire` meldet nicht `acquired` (etwa `busy`), `check` meldet nicht
  `held` (`lost`), `LOCK_TOKEN` ist unbekannt, oder das MCP-Werkzeug fehlt (Shell-Ersatz) —
  stehen in `k-playbook:/commands/_task-run/sperre.md`; der Hauptkontext liest es dann.
- **Prüfen.** `check` mit `LOCK_TOKEN` steht an jeder Stelle, die den Arbeitsbaum ändert,
  dort genannt; dazu nach jeder Antwort des Nutzers während Schritt 5, auch nach einer, mit der
  eine unterbrochene Sitzung fortgesetzt wird. `state: held` heißt weiter.
- **Freigeben** mit `release` und `LOCK_TOKEN` an genau einer Stelle: wenn der Lauf Schritt 5
  verlässt (5g), also nach 5f.1 des letzten Tasks und vor Schritt 6 — auch nach einem
  Anhalten, Abbruch oder `foreign_change` — und auf jedem Abbruchweg davor, sobald die Sperre
  genommen ist. Bis dahin trägt jeder ausgeführte Task seine Kennzeile „ausgeführt“ oder
  „abgebrochen“ oder liegt unter `done/`. Während einer Rückfrage in Schritt 5 bleibt die
  Sperre gehalten; Schritt 6 und 7 laufen ohne Sperre und ohne `check`. Meldet `release`
  `lost`, wird das in Schritt 7 genannt, nicht gefragt.

## Schritt 2 — Tasks sammeln

**Wort `refine`.** Ist das letzte Wort des Arguments `refine` (Groß- und Kleinschreibung
gleich), nimm es ab und merke `REFINE = ja`; der Rest ist das Argument. Sonst
`REFINE = nein`.

Das Argument entscheidet:

include: _task/ziel-aufloesen.md

- **Erledigt, zusätzlich:** Sag das und frage, ob er mit `k_playbook_doc_reopen` wieder
  geöffnet werden soll; sonst Schluss. Bei „ja“ liest der Hauptkontext
  `k-playbook:/commands/_task-run/wieder-oeffnen.md` und folgt ihm.
- Verzeichnisse und Dateipfade gibt es als Argument nicht; ein solches Argument ist ein
  Fehler.

Je Task merkst du dir `TASK_REF` (der Verweis), `TASK_FILE` (der letzte Pfadteil, etwa
`014-setup-tts.md`) und `TASK_STEM` (ohne `.md`). Dazu merkst du dir je Task
`SCHON_AUSGEFUEHRT`: ob die Kennzeile „ausgeführt“ jetzt schon trifft — Suche und Regel aus 5a,
ohne etwas zu lesen. Daran erkennt die Prüfung nach dem Nehmen der Sperre, was ein anderer Run
inzwischen ausgeführt hat (5, „Liste neu prüfen“).

Kündige die Liste an, bevor etwas beginnt, samt Intent und Werkzeugen:

```
Tasks:
  1. keph:/user/kamran/tasks/014-setup-tts.md
  2. keph:/user/kamran/tasks/015-integrate-tts.md  <- letzter

Intent (aus letztem Task):
  <intent text>

Tools (zusätzlich): ...
```

**Intent und Tools lesen.** Prüfe den **letzten** Task auf einen Abschnitt `## Intent`
(`k_playbook_doc_read` mit `section: "## Intent"`; `section_not_found` heißt: kein Intent).
Lies je Task `## Tools` (`section: "## Tools"`; der erste Treffer zählt, ein Ausschnitt in
späteren Diff-Abschnitten nicht), sammle die genannten Werkzeuge und zeige sie dem Nutzer
vorweg, damit er Rechte erteilen kann. Den Intent behält der Hauptkontext wörtlich für
Schritt 6; Verweise darin löst er nicht auf. Der Block läuft nach dem Refine (Schritt 3) und
nach „Liste neu prüfen“ (Schritt 5) erneut; hat sich die Werkzeugliste geändert, nennt der
Hauptkontext sie erneut.

## Schritt 3 — Task-Refine-Status prüfen

Prüfe für jeden gesammelten Task mit `k_playbook_doc_grep` (`^## Review-Log`, `count_only`), ob er ein
`## Review-Log` trägt; gelesen wird dafür nichts. Tragen **alle** Tasks eins: weiter, ohne
Rückfrage.

Fehlt es mindestens einem: erst die Fertig-Meldung (`Rückfrage: ohne Task-Refine`), dann liest
der Hauptkontext `k-playbook:/commands/_task-run/ohne-refine.md` und fragt wie dort (`ja`:
weiter mit Schritt 4, `nein`: abbrechen, `zuerst reviewen`: abbrechen und wörtlich
`/k-task-refine <TASK_REF>` je Task nennen).

**Modus `refine`.** Mit `REFINE = ja` gehen die ungeprüften Tasks zuerst durch die
Gruppenprüfung `k-playbook:/commands/_task-refine/gruppen.md`, vor dem Nehmen der Sperre. Der
Hauptkontext liest dafür `k-playbook:/commands/_task-run/refine-modus.md` und folgt ihm: ohne
Argument die ungeprüften Tasks der Liste in Gruppen mit Überschneidungs-Critic, mit Argument
eine Gruppe mit einem Task; danach die Zeilen `Refine: …` (auch für Schritt 7) und das erneute
„Intent und Tools lesen“. Er moderiert nicht selbst. Die Rückfrage oben gilt dann nur den Tasks,
die dort zurückfielen, mit dem Grund je Task; sonst weiter mit Schritt 4. Ohne `refine` bleibt Schritt 3 wie oben.

## Schritt 4 — Diff und Review vorbereiten

Ob ein Task einen Diff und ein Review bekommt, folgt aus dem Briefing:

- `context.project.vcs` ist `git` und `git` steht nicht in `context.baseTools.missing`:
  `DIFF_ENABLED=true`. Fehlt `git`: `DIFF_ENABLED=false`; einmal sagen, dass damit Diff, Review
  und der Stopp bei kritischen Befunden entfallen, und den Befehl aus
  `context.baseTools.installCommand` nennen.
- Sonst: `DIFF_ENABLED=false`, still weiter.

Mit `DIFF_ENABLED=true` gilt
`DIFF_REVIEW_MODULE = k-playbook:/commands/_task-run/diff-review.md`; der Hauptkontext liest
es nicht, er gibt nur den Verweis weiter. Grundlage des Diffs ist ein Snapshot je Task (5b.1),
kein Commit je Lauf: Er schließt nicht committete und ungetrackte Änderungen ein, auch die der
Vorgänger im selben Lauf; nicht erfasst sind Dateien, die `.gitignore` ausschließt. Das
Task-Dokument ist nie Teil des Diffs.

## Schritt 5 — Tasks nacheinander ausführen

### Sperre nehmen

Vor dem ersten Task, vor der ersten Änderung am Arbeitsbaum — nach Schritt 3, im Modus
`refine` also nach dem Refine —, nimmt der Hauptkontext die Sperre: `k_playbook_lock` mit
`action: acquire`, `purpose` = `/k-task-run <TASK_FILE> [, <TASK_FILE> …]` und `session` = die
Kennung aus der Zeile `Sitzung:` des Briefings.

- `acquired`: `LOCK_TOKEN` merken. Nennt die Antwort `takenOver`, sagt der Hauptkontext in
  einer Zeile, wessen verwaiste Sperre übernommen wurde.
- Sonst (`busy`): `sperre.md`, Abschnitt „Acquire meldet busy“. Die Wartezeit merkt sich der
  Hauptkontext für Schritt 7.

**Liste neu prüfen.** Direkt nach dem Nehmen, auch nach dem Warten, liest der Hauptkontext
`k-playbook:/commands/_task-run/liste-neu-pruefen.md` und folgt ihm: Ein inzwischen erledigter
oder ausgeführter Task fällt ohne Rückfrage heraus, alles andere bleibt; bei Abweichung der
eigenen `REVISION` gilt `foreign_change`; eine leere Liste gibt die Sperre frei und endet mit
der Fertig-Meldung `fertig`.

### Die Tasks

Für jeden Task, **streng nacheinander** (nie parallel — zwei Agenten dürfen nicht zugleich
Code ändern):

### 5a — Lesen und verstehen

Zuerst mit `k_playbook_doc_grep` (mit Zeilennummern, ohne `count_only`) die Kennzeilen
„ausgeführt“ und „wieder geöffnet“ dieses Tasks suchen:
`^<!-- k-task-run: (ausgeführt <TASK_FILE>|wieder geöffnet <TASK_FILE> [0-9-]+) -->$`, Punkte
im Dateinamen maskiert. Die Revision aus dieser Antwort ist das erste `REVISION`; nach dem
Wieder-Öffnen in Schritt 2 muss sie gleich `REVISION` sein (Revisionskette). Die Kennzeile „ausgeführt“ **trifft**, wenn
eine solche Zeile nach der letzten Zeile „wieder geöffnet“ steht; gibt es keine Zeile
„wieder geöffnet“, trifft jede.

**Trifft sie**, ist der Task ausgeführt, aber nicht abgeschlossen; er wird weder gelesen noch
erneut ausgeführt. Der Hauptkontext liest `k-playbook:/commands/_task-run/nicht-abgeschlossen.md`
und folgt ihm; er hat `TASK_REF`, `TASK_FILE`, `REVISION` und die Zeilennummer der letzten
Kennzeile „ausgeführt“, die trifft. „(a) abschließen“ schließt den Task ab, der Lauf geht mit
dem nächsten weiter; „(b) anhalten“ beendet den Lauf. Den Befund-Leser, der dort und beim
Abschluss läuft, beschreibt `abschluss.md`.

**Trifft sie nicht**, lies den Task bis vor seine ersten Lauf-Abschnitte, geschnitten nach
`k-playbook:/instructions/on-demand/tasks.md`, „Rumpf und Lauf-Abschnitte“: mit
`k_playbook_doc_grep` (mit Zeilennummern) `^## Ausführung$` und
`^<!-- k-task-run: (ausgeführt|abgebrochen) <TASK_FILE> -->$` suchen; der Schnitt ist die
letzte Zeile `## Ausführung` vor der ersten Kennzeile, ohne Kennzeile die erste Zeile
`## Ausführung`. Dann `k_playbook_doc_read` mit `lines: "1-<zeile vor dem Schnitt>"`, ohne
Schnitt `lines: "1-"`. Das `## Review-Log` kann übersprungene Punkte und Deadlocks tragen, die
für die Rückfragen in 5b gebraucht werden. Mehr liest der Hauptkontext von diesem Task nicht.

### 5a.1 — Ausführungskontext und Branch-Preflight

Die Ausführungswurzel (`EXEC_ROOT`) ist die Projektwurzel. Trägt der Task einen Abschnitt
`## Ausführungskontext`, liest der Hauptkontext
`k-playbook:/commands/_task-run/ausfuehrungskontext.md` und folgt ihm: Es nennt die Werte
(`Target repo`, `Base branch`, `Work branch`, `PR required`, `Dirty worktree policy`), löst
`EXEC_ROOT` auf, führt den Branch-Preflight aus und liefert den Rahmen für den Sub-Agenten;
`PR required` und `Target repo` merkt sich der Hauptkontext für den Erfolgspfad.

Ohne `## Ausführungskontext` vor der Delegation: Prüfe, dass `EXEC_ROOT` ein Git-Repository ist
(`git rev-parse --is-inside-work-tree`; sonst anhalten und fragen), und den Dirty-State mit
`git status --short`. Ist er nicht leer: anhalten und die Dateien zeigen; weiter nur, wenn der
Nutzer ausdrücklich bestätigt, dass sie für den Task erwartet sind, sonst nicht delegieren.

### 5a.2 — Etappen-Fortschritt prüfen

Ein Task ohne `### Etappe N — Titel` läuft als Ganzes. Enthält der Task Etappen (`k_playbook_doc_grep`, `^### Etappe [0-9]+ — `; ihre Zahl ist `m`):
Den Stand geben allein die Fortschrittszeilen, gelesen und ausgewertet nach
`k-playbook:/instructions/on-demand/tasks.md`, „Fortschritt anhängend“. Steht keine Etappe auf
erledigt, weiter mit 5b, alle Etappen offen. Sonst liest der Hauptkontext
`k-playbook:/commands/_task-run/etappen-fortsetzen.md` und folgt ihm (Fertig-Meldung
`Rückfrage: Etappen fortsetzen`, Optionen, Zurücksetzen). Der Stand wird **nicht** aus einem
Abbruch heraus geraten: Eine offene Etappe gilt als nicht ausgeführt, auch wenn Teile davon im
Code sichtbar sind.

### 5b — Vor der Delegation klären

**Vor** dem Start des Sub-Agenten: Finde alles im Task, was unklar oder mehrdeutig ist oder
eine Entscheidung verlangt, die sich aus der Beschreibung nicht ableiten lässt. Die Fragen
stützen sich allein auf den gelesenen Task; der Hauptkontext liest dafür keinen Code. Was
sich nur am Code klären lässt, bleibt dem Ausführungs-Sub-Agenten — trifft er dort auf eine
echte Entscheidung, meldet er sie als Blocker (5d).

Gibt es solche Fragen: **anhalten und den Nutzer fragen** (Fertig-Meldung vorher) — nicht
überspringen, nicht raten, keine Quick-and-dirty-Entscheidung. Weiter erst, wenn alle offenen
Fragen beantwortet sind.

### 5b.1 — Snapshot des Tasks

Nur mit `DIFF_ENABLED=true`. Der Snapshot hält den Arbeitsbaum der Ausführungswurzel vor dem
Task fest, samt nicht committeter und ungetrackter Änderungen; das Review vergleicht den Stand
nach dem Task damit. Er gehört zum Task, nicht zu einer Delegation: Er entsteht beim ersten
Start unmittelbar vor der ersten Delegation und gilt für jeden weiteren Start (nach 5d und in
späteren Läufen). Vor dem Anlegen `check` der Sperre.

Der Hauptkontext liest `k-playbook:/commands/_task-run/snapshot.md` und folgt ihm: Es liefert
`SNAPSHOT` und `REVIEW_SCOPE` — `ab Task-Beginn`, wenn der Task noch nie lief, sonst
`nur seit Fortsetzung` (das Review deckt dann nur die Änderungen seit diesem Snapshot ab, sagt
das in Abschnitt und Rückgabe, und 5f fragt vor dem Verschieben nach `done/`). Schlägt ein
Aufruf fehl, hält der Command vor der Delegation an und fragt den Nutzer, wie in 5a.1. Führt der
Nutzer den Task ohne Snapshot aus, läuft er ohne Diff und Review, und Schritt 7 nennt das.

### 5c — An den Sub-Agenten delegieren

Vorher `check` der Sperre, dann die Fortschrittszeilen zählen (`k_playbook_doc_grep`,
`^<!-- k-task-run: etappe [0-9]+ erledigt `, `count_only`) und Anzahl und Revision merken;
die Revision muss `REVISION` sein (Revisionskette, Schritt 1).

Starte einen General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code:
`general-purpose`) für die Ausführung. Er liest seinen Auftrag selbst:
`k-playbook:/commands/_task-run/ausfuehrung.md`, Abschnitt „Ausführung“. Du übergibst ihm den
Verweis und diese Werte, nicht den Inhalt des Tasks:

- `TASK_REF` und `REVISION`; das Arbeitsverzeichnis und `projectDir` (`context.project.dir`,
  auch wenn die Ausführungswurzel woanders liegt),
- bei `## Ausführungskontext`: die aufgelöste Ausführungswurzel, Base- und Work-Branch,
  PR-Pflicht, die Entscheidung zum schmutzigen Arbeitsbaum und das Ergebnis des
  Branch-Preflights als verbindlichen Rahmen,
- alle Klärungen aus 5b als zusätzlichen Kontext,
- bei `### Etappe`-Abschnitten: welche Etappen offen sind.

Kein Diff und keine Kennung eines Snapshots: Diff und Review sind Sache des
Review-Sub-Agenten in 5f. Seine Rückgabe hat das Format des Moduls; ausgewertet werden:
`Status: erfolgreich | teilweise | blockiert` (`erfolgreich` führt nach 5f, `blockiert` nach
5d, `teilweise` nach 5e; eine Rückgabe ohne gültige Zeile `Status:` gilt als `blockiert`),
`Zusammenfassung:`, `Abgebrochen bei:` und `Blocker:` für den Kopf in 5f bzw. 5e und die
Rückfrage in 5d, `Validierung:` für das Review in 5f und den PR-Text, und `Revision: <n>` bzw.
nach `foreign_change` `Foreign-Change: <revision> <updatedBy> <updatedAt>` (dann ist `Status`
`blockiert`): `REVISION` wird nach der Revisionskette gesetzt, nach `Foreign-Change:` erst nach
der Rückfrage, ohne beides durch erneutes Zählen.

Warte, bis der Sub-Agent fertig ist, bevor der nächste Schritt beginnt.

### 5d — Unerwartete Blocker behandeln

Meldet die Rückgabe `Status: blockiert`: **anhalten und den Nutzer fragen**, mit der Zeile
„Blocker“ — vorher die Fertig-Meldung (`Rückfrage: Blocker`, der Body nennt ihn). Steht
`Foreign-Change:` in der Rückgabe, ist das die Rückfrage nach `foreign_change` aus Schritt 1.
Kein nächster Task, bis das geklärt ist. Dann entscheiden, ob der Task erneut läuft (5c noch
einmal, mit der Antwort des Nutzers als weiterer Klärung; derselbe Snapshot) oder abgebrochen
wird (5e).

### 5e — Bei Fehler oder Abbruch

Gilt bei `Status: teilweise`, bei einem nach 5d abgebrochenen Task und wenn 5f eine offene
Etappe findet — immer vor dem Kopf aus 5f. Der Hauptkontext liest
`k-playbook:/commands/_task-run/abbruch.md` und hängt den Abbruchvermerk an. Der Task bleibt
liegen, die Fortschrittszeilen bleiben; keine weiteren Tasks. Nach dem Kopf aus 5f kommt 5e
nicht mehr vor: Ein Task verlässt den Lauf dann nur nach `done/` oder mit der Zeile
„angehalten“ (5f, Schritt 5).

### 5f — Bei Erfolg

Bei `Status: erfolgreich`, in genau dieser Reihenfolge.

**1. Etappen prüfen.** Hat der Task `### Etappe`-Abschnitte: die Fortschrittszeilen erneut
zählen (wie 5a.2). Hat eine Etappe keine Zeile nach dem letzten Zurücksetzen, ist der Lauf
**nicht** erfolgreich: ohne Kopf und Review weiter nach 5e, mit der ersten offenen Etappe
unter „Abgebrochen bei“ und dem Grund, dass der Sub-Agent Erfolg meldet, während die Etappe
offen steht. Der Widerspruch wird gezeigt, nicht geglättet.

**2. Kopf und Kennzeile.** In einem `k_playbook_doc_append` mit `since_revision: REVISION`
anhängen:

```text
## Ausführung

<!-- k-task-run: ausgeführt <TASK_FILE> -->
**Status:** Erfolgreich ausgeführt  
**Datum:** <context.now.date>  
**Zusammenfassung:** <Zusammenfassung aus der Rückgabe>
```

Ab hier liest der Hauptkontext keinen Abschnitt des Tasks mehr. Mit `DIFF_ENABLED=false`
oder ohne Snapshot (5b.1) folgt direkt 5f, Schritt 6 — kein Diff, kein Review.

**3. Rezept bestimmen und 4. Review-Sub-Agent.** Der Hauptkontext liest
`k-playbook:/commands/_task-run/review-starten.md` und folgt ihm: Es bestimmt das Rezept aus
dem Briefing (ohne es zu lesen), startet den Review-Sub-Agenten — nicht den ausführenden —
nach `DIFF_REVIEW_MODULE`, prüft die Rückgabe auf Gültigkeit und setzt `REVISION`. Das
Ergebnis ist einer der Ausgänge der Tabelle.

**Ausgänge des Reviews.** Die Tabelle gilt für die Stopp-Abfrage (Schritt 5), das Sammeln der
Befunde und die Zeile „Review“ in Schritt 7:

| Ausgang | Stopp-Abfrage | Für Schritt 7 gesammelt | Zeile „Review“ |
|---|---|---|---|
| gültig, ohne kritischen Befund | nein | `Wichtig:` | `Review: <n> kritisch, <n> wichtig, <n> Hinweis` |
| gültig, mit kritischem Befund | ja | `Wichtig:`, nach „(b) trotzdem weiter“ dazu `Kritisch:` | wie oben |
| ungültig: Abbruch, Rückgabe nicht im Format oder kein neuer Review-Abschnitt | ja | nach „(b)“ die Zeilen des Befund-Lesers | `Review: kein gültiges Review`, mit „— Befunde nicht gelesen“, wenn der Leser nicht in seiner Form antwortete |
| Rezept fehlt im Briefing | ja, ohne (a) | nichts | `Review: kein gültiges Review` |
| Rezept abgeschaltet (Herkunft `disabled`) | nein | nichts | `Review: abgeschaltet` |
| `DIFF_ENABLED=false` oder ohne Snapshot | nein | nichts | `Review: – (ohne Diff)` |
| zusätzlich `REVIEW_SCOPE` `nur seit Fortsetzung` | ja, ohne (a) | wie sonst | Zusatz „— nur seit Fortsetzung“ |

Maßgeblich ist das letzte Review des Laufs, auch nach 4a und nach „(a)“. Ein Befund „wichtig“ ist kein Grund für die Stopp-Abfrage; er hält den Lauf nicht an.

**4a. Nachbessern.** Bei gültigem Review mit kritischem oder wichtigem Befund und `REVIEW_SCOPE`
≠ `nur seit Fortsetzung` liest der Hauptkontext `k-playbook:/commands/_task-run/nachbessern.md`
und folgt ihm (mit `TASK_REF`, `REVISION`, Rahmen aus 5c, `SNAPSHOT`, `REVIEW_SCOPE`,
`DIFF_REVIEW_MODULE`, Befundzeilen). Sonst, nach der Schleife und nach „(a) beheben lassen“: entfällt.

**5. Stopp-Abfrage** — nach dem Review, vor dem Verschieben nach `done/`, vor 5f.1 und vor
dem nächsten Task. Sie kommt, sobald ein Ausgang der Tabelle eine Abfrage verlangt: erst die
Fertig-Meldung (`Rückfrage: <Grund>`, der Body nennt die Zahlen), dann liest der Hauptkontext
`k-playbook:/commands/_task-run/stopp-abfrage.md` und folgt ihm (Abfrage, beheben lassen bzw.
Review erneut, trotzdem weiter, anhalten).

**6. Abschluss.** Der Hauptkontext liest `k-playbook:/commands/_task-run/abschluss.md` und folgt
ihm: Es sammelt die Befunde des Tasks für Schritt 7 (nach der Spalte „Für Schritt 7 gesammelt“),
verschiebt den Task mit `k_playbook_doc_done` nach `done/` (`TASK_REF_DONE`, neue Revision =
`REVISION`), ergänzt `CHANGED_PATHS` um die geänderten Pfade gegen den Snapshot (ohne Snapshot:
„Pfade unbekannt“) und gibt die Snapshot-Ref frei.

### 5f.1 — PR-Handoff bei `PR required: true`

Erst nach der Stopp-Abfrage aus 5f. Trägt der `## Ausführungskontext` des Tasks
`PR required: true`, folgt der PR, nachdem der Task abgeschlossen ist und ein lokaler Commit
existiert: Der Hauptkontext liest `k-playbook:/commands/_task-run/pr.md` und folgt ihm — Zugang
über `GH_SERVICE`, Preflight (mit `check` der Sperre vor dem Push), Push, PR-Text, PR. Ohne
`PR required: true` entfällt 5f.1.

### 5g — Weiter

Weiter mit dem nächsten Task der Liste. Ist ein Task fehlgeschlagen (5e), angehalten (5f,
Schritt 5, oder 5a, Option (b)) oder nach `foreign_change` liegen geblieben: keine weiteren
Tasks, aber Schritt 6, soweit er greift, und Schritt 7 mit der Abfrage der Befunde der bis
dahin abgeschlossenen Tasks. Kein weiterer Task — nach dem letzten oder nach einem Anhalten —:
Sperre freigeben (`release` mit `LOCK_TOKEN`), erst dann Schritt 6.

## Schritt 6 — Intent-Alignment-Check

Sind alle Tasks nach `done/` gewandert **und** trägt der letzte Task einen `## Intent`, liest
der Hauptkontext `k-playbook:/commands/_task-run/intent-alignment.md` und folgt ihm: Es startet
den Critic, hängt `**Intent-Alignment:** <Ja / Teilweise / Nein> - <Begründung>` an den letzten
Task an und warnt vor der abschließenden Zusammenfassung mit
`WARNUNG: Intent nicht vollständig erreicht: <Begründung>`, wenn das Alignment nicht Ja ist.
Ohne Intent entfällt dieser Schritt still.

## Schritt 7 — Abschließende Zusammenfassung

**Doku-Nachzug.** Vor der Zusammenfassung prüft der Hauptkontext, ob der Lauf Code geändert
hat (`CHANGED_PATHS` nicht leer, oder ein Task mit „Pfade unbekannt“). Dann liest er
`k-playbook:/commands/_task-run/doku-nachzug.md` und folgt ihm, nicht der
Ausführungs-Sub-Agent; Ergebnis ist die Zeile „Doku“.

**Befunde als Todo.** Danach, vor der Fertig-Meldung `fertig` oder `abgebrochen`: Hat der Lauf
Befunde gesammelt (5f, Schritt 6, und 5a, Option (a)), liest der Hauptkontext
`k-playbook:/commands/_task-run/befunde-als-todo.md` und folgt ihm (eine Abfrage je Lauf,
Todos über `k_playbook_todo_add`, Zeile „Todos“); sonst entfällt die Abfrage.

Nach allen Tasks zuerst die Fertig-Meldung (`fertig` oder `abgebrochen`; der Body trägt die
Zeilen „Erfolgreich“, „Angehalten“ und „Abgebrochen“ in Kurzform), dann die Zusammenfassung
nach `k-playbook:/commands/_task-run/zusammenfassung.md`, die der Hauptkontext jetzt liest.

Folge-Command: **`/k-danke`** — schließt die Sitzung ab.
