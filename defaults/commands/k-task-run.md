# k-task-run

Führt Tasks aus der Projekt-Collection aus. Ohne Argument die eigenen offenen Tasks in
Nummernreihenfolge; mit Argument einen `keph:`-Verweis oder eine Tasknummer.

Der Hauptkontext steuert und fragt, Sub-Agenten arbeiten und schreiben. Ausführung, Diff
und Code-Review laufen je Task in Sub-Agenten, die ihre Ergebnisse selbst an das
Task-Dokument anhängen und dem Hauptkontext nur ein festes, knappes Format zurückgeben. Der
Hauptkontext sieht pro Task keinen Diff, keine Dateiinhalte außer den Abschnitten des Tasks,
die er braucht, keine Logs und kein vollständiges Review. Alle Rückfragen an den Nutzer
bleiben im Hauptkontext.

Produces:

- An jedem ausgeführten Task den Abschnitt `## Ausführung` mit Kennzeile, Status und
  Zusammenfassung, die Diff- und Review-Abschnitte des Review-Sub-Agenten und, bei Etappen,
  die Fortschrittszeilen des Ausführungs-Sub-Agenten — alles angehängt.
- Erfolgreich abgeschlossene Tasks unter `keph:/user/~/tasks/done/`, verschoben mit
  `k_playbook_doc_done`.
- Je Task eine Ref `refs/k-task-run/<TASK_STEM>` in der Ausführungswurzel, bis der Task
  nach `done/` wandert.
- Bei `PR required: true` einen Pull Request über `gh`.

## Schritt 1 — Briefing und Form

include: _shared/erster-schritt.md

Verbindlich für Ablage, Nummernauflösung, Kennzeilen, Fortschrittszeilen und den Schreibweg
eines Tasks ist `k-playbook:/instructions/on-demand/tasks.md`. Lies sie mit
`k_playbook_doc_read`, bevor du einen Task liest; dieser Text wiederholt sie nicht, auch
nicht die Kennzeilen-Tabelle und den Ersatz ohne MCP in der Shell. Der Command braucht die
Collection: Steht `context.kephalaion.ok` nicht auf `true`, endet er mit
`context.kephalaion.message` (Abschnitt „Ohne Kephalaion“ dort).

**Hauptkontext und Task-Dokument.** Für den Umgang des Hauptkontexts mit einem Task gilt im
ganzen Command:

- **Lesen nur mit `k_playbook_doc_read`** und `section` (eine Überschriftenzeile, etwa
  `## Intent`) oder `lines` (ein Zeilenbereich) — nie das ganze Dokument, nie mit Kephalaion
  `read`. Jede Antwort nennt die `revision`.
- **Suchen mit `k_playbook_doc_grep`**: Kennzeilen, Überschriften, Fortschrittszeilen; mit
  `count_only` nur die Anzahl und die Revision.
- **Schreiben nur mit `k_playbook_doc_append`**, jedes Anhängen mit `since_revision`. Der
  Hauptkontext ändert keine bestehende Zeile.
- **Nach dem Kopf** (5f, Schritt 2) liest der Hauptkontext keinen Abschnitt des Tasks mehr;
  er zählt nur noch mit `k_playbook_doc_grep` und hängt an.

Der Grund: Diff und Review stehen im Task-Dokument, und der Hauptkontext darf sie nicht
sehen — sonst wäre jede Rückfrage an den Nutzer von Dateiinhalten und Befunden überlagert,
und ein zweiter Lauf desselben Tasks läse den ganzen ersten mit. Die Operationen lassen den
Inhalt im Programm.

**Revisionskette.** Ein Task hat zu einer Zeit einen Schreiber (`on-demand/tasks.md`, „Lesen
und schreiben“). Der Hauptkontext merkt sich je Task `REVISION`, die Revision seines eigenen
letzten Schreibens; vor dem ersten Schreiben die Revision aus dem ersten `k_playbook_doc_grep`
in 5a. Vor jeder Übergabe an einen Sub-Agenten zählt er mit `k_playbook_doc_grep` und
`count_only` die Kennzeile, die der Sub-Agent schreiben soll, und merkt sich Anzahl und
Revision aus dieser Antwort; weicht die Revision von `REVISION` ab, gilt es wie
`foreign_change`. Jeder Sub-Agent bekommt den Verweis und `REVISION`, hängt mit
`since_revision` an und gibt in seiner Rückgabe `Revision: <n>` zurück; damit setzt der
Hauptkontext `REVISION` neu. Bekommt ein Sub-Agent `foreign_change`, hält er an, schreibt
nichts weiter und gibt statt `Revision: <n>` die Zeile
`Foreign-Change: <revision> <updatedBy> <updatedAt>` zurück. Fehlen beide Zeilen, zählt der
Hauptkontext erneut und übernimmt die Revision aus dieser Antwort nur, wenn die Anzahl
gestiegen ist; sonst gilt es wie `foreign_change`.

`foreign_change` — selbst bekommen oder so gemeldet — führt immer zu demselben Schritt:
Fertig-Meldung (`Rückfrage: fremder Schreiber`), dann die Rückfrage mit `revision`,
`updatedBy` und `updatedAt`, wer weitermacht. Sagt der Nutzer „weiter“, gilt die Revision
aus dieser Antwort als `REVISION`, und der unterbrochene Schritt läuft erneut; sonst endet
der Lauf für diesen Task, ohne weiteren Eintrag, und keine weiteren Tasks laufen. Nach
`outcome_unknown` hängt der Hauptkontext nicht noch einmal an, sondern zählt die Zeile, die
er geschrieben hat, und nimmt die Revision aus dieser Antwort.

## Fertig-Meldung

Vor jeder Rückfrage an den Nutzer und am Ende des Commands meldet sich der Hauptkontext auf
dem Rechner des Nutzers. Die Stellen in diesem Command: die Rückfrage in Schritt 3, die
Optionen in 5a und 5a.2, der Blocker in 5d, jede Rückfrage nach `foreign_change`, die
Stopp-Abfrage in 5f und die Zusammenfassung in Schritt 7.

include: _task/fertig-meldung.md

## Schritt 2 — Tasks sammeln

Das Argument entscheidet:

- **Leer:** die eigenen offenen Tasks aus `k_playbook_doc_list` (Sorte `tasks`, ohne `done`),
  sortiert nach der Nummer am Anfang des Dateinamens. Keine → „Keine offenen Tasks gefunden“
  und Schluss.
- **Ein `keph:`-Verweis** (`keph:/user/…/tasks/<datei>`): genau dieser Task, als Liste mit
  einem Eintrag.
- **Eine Nummer:** aufgelöst nach `on-demand/tasks.md`, „Wohin und Name“ — zuerst unter den
  eigenen offenen, dann unter den eigenen erledigten Tasks; bei mehreren Treffern alle zeigen
  und den Nutzer wählen lassen. Ein Treffer unter `done/` ist schon abgeschlossen: sag das
  und frage, ob er mit `k_playbook_doc_reopen` wieder geöffnet werden soll; sonst Schluss.
- Verzeichnisse und Dateipfade gibt es als Argument nicht mehr; ein solches Argument ist ein
  Fehler mit dem Hinweis auf `/k-upgrade` (Umstellung 3.3), der Tasks von der Platte umzieht.

Je Task merkst du dir `TASK_REF` (der Verweis), `TASK_FILE` (der letzte Pfadteil, etwa
`014-setup-tts.md`) und `TASK_STEM` (ohne `.md`).

Kündige die Liste an, bevor etwas beginnt. Prüfe den **letzten** Task auf einen Abschnitt
`## Intent` (`k_playbook_doc_read` mit `section: "## Intent"`; `section_not_found` heißt:
kein Intent) und nenne ihn:

```
Tasks:
  1. keph:/user/kamran/tasks/014-setup-tts.md
  2. keph:/user/kamran/tasks/015-integrate-tts.md  <- letzter

Intent (aus letztem Task):
  <intent text>

Tools (zusätzlich): ...
```

Lies je Task den Abschnitt `## Tools` (`section: "## Tools"`), sammle alle genannten
Werkzeuge und zeige sie dem Nutzer vorweg, damit er Rechte erteilen kann, bevor der Lauf
beginnt. `section` liefert den ersten Treffer; ein Ausschnitt in späteren Diff-Abschnitten
zählt deshalb nicht. Den Intent behält der Hauptkontext für Schritt 6.

## Schritt 3 — Task-Refine-Status prüfen

Ein Task, der nie durch `/k-task-refine` gegangen ist, wurde nie gegengelesen. Prüfe für
jeden gesammelten Task mit `k_playbook_doc_grep` (`^## Review-Log`, `count_only`), ob er ein
`## Review-Log` trägt; gelesen wird dafür nichts.

Tragen **alle** Tasks ein Review-Log: weiter, ohne Rückfrage.

Fehlt es mindestens einem: erst die Fertig-Meldung (`Rückfrage: ohne Task-Refine`), dann die
betroffenen Tasks in **einer** Nachricht nennen und fragen:

```
Ohne Task-Refine:
  - 014-setup-tts.md
  - 015-integrate-tts.md

Diese Tasks wurden nicht mit /k-task-refine gegengelesen.
Trotzdem ausführen? (ja / nein / zuerst reviewen)
```

- `ja` — weiter mit Schritt 4.
- `nein` — abbrechen, nichts ausführen.
- `zuerst reviewen` — abbrechen und wörtlich `/k-task-refine <TASK_REF>` je Task nennen,
  danach `/k-task-run` erneut starten. Den Task-Refine-Command nicht selbst aufrufen.

`/k-task-refine` hängt sein Log an **jeden geprüften** Task an, auch an den, an dem nichts
zu ändern war (dort mit dem Vermerk „keine Änderungen“). Ein fehlender Block heißt deshalb:
dieser Task ist nie durch Task-Refine gegangen. Trotzdem bleibt es eine Rückfrage und kein
Abbruch: Ein von Hand geschriebener oder aus einem anderen Projekt übernommener Task kann
sachlich in Ordnung sein — das zu entscheiden ist Sache des Nutzers, nicht des Commands.

## Schritt 4 — Diff und Review vorbereiten

Ob ein Task einen Diff und ein Review bekommt, folgt aus dem Briefing — nichts zu
konfigurieren und nichts zu suchen:

- `context.project.vcs` ist `git`, und `git` steht nicht in `context.baseTools.missing`:
  `DIFF_ENABLED=true`.
- `context.project.vcs` ist `git`, aber `git` fehlt: `DIFF_ENABLED=false`. Einmal sagen, dass
  damit Diff, Review und der Stopp bei kritischen Befunden entfallen, und den Befehl aus
  `context.baseTools.installCommand` nennen.
- Sonst: `DIFF_ENABLED=false`, still weiter.

`DIFF_ENABLED` ist kein Guard im Sinn von `k-playbook:/rules/command-authoring.md`; jeder
`git`-Aufruf in einem Shell-Codeblock dieses Commands hat seinen eigenen.

Mit `DIFF_ENABLED=true`:

- `DIFF_REVIEW_MODULE = k-playbook:/commands/_task-run/diff-review.md`; eine gleichnamige
  Datei unter `<context.local.dir>/commands/_task-run/` ersetzt es nach der Überlagerung
  von k-playbook. Der Hauptkontext liest das Modul nicht, er gibt nur den Verweis weiter.
- Grundlage des Diffs ist ein Snapshot je Task (5b.1), kein Commit je Lauf. Er schließt nicht
  committete und ungetrackte Änderungen ein, auch die der Vorgänger im selben Lauf. Ein
  schmutziger Arbeitsbaum verfälscht den Diff deshalb nicht; ob schmutzige Dateien für einen
  Task erwartet sind, klärt der Branch-Preflight in 5a.1. Nicht erfasst sind Dateien, die
  `.gitignore` ausschließt: Änderungen daran erscheinen weder im Diff noch im Review. Das
  Task-Dokument liegt in der Collection und ist nie Teil des Diffs.

## Schritt 5 — Tasks nacheinander ausführen

Für jeden Task, **streng nacheinander** (nie parallel — zwei Agenten dürfen nicht zugleich
Code ändern):

### 5a — Lesen und verstehen

Zuerst mit `k_playbook_doc_grep` die Kennzeile „ausgeführt“ dieses Tasks zählen
(`^<!-- k-task-run: ausgeführt <TASK_FILE> -->$`, Punkte maskiert, `count_only`). Die
Revision aus dieser Antwort ist das erste `REVISION`.

**Trifft sie**, ist der Task ausgeführt, aber nicht abgeschlossen: nach der Stopp-Abfrage in
5f angehalten, oder eine Sitzung endete zwischen Kopf und `done/`. Der Task wird weder
gelesen noch erneut ausgeführt. Fertig-Meldung (`Rückfrage: nicht abgeschlossen`), dann:

```
<TASK_FILE>: bereits ausgeführt, nicht abgeschlossen — Review, sein Ergebnis oder
kritische Befunde offen (siehe ## Ausführung).

Wie weiter?
  (a) abschließen  - offene Punkte erledigt oder akzeptiert, Task wandert nach done/
  (b) anhalten     - der Lauf endet hier, keine weiteren Tasks
```

- (a): Mit `k_playbook_doc_append` und `since_revision: REVISION` anhängen:

  ```text
  **Status nach Review:** abgeschlossen vom Nutzer (<context.now.date>)
  ```

  Dann weiter wie 5f, Schritt 6 (nach `done/` verschieben, Snapshot freigeben); die
  Ausführungswurzel dafür ist `Target repo` aus `## Ausführungskontext` (`section`, erster
  Treffer), sonst die Projektwurzel. 5f.1 läuft nicht; steht dort `PR required: true`, nennt
  der Command den PR als offenen Schritt. Weiter mit dem nächsten Task.
- (b): Der Lauf endet, keine weiteren Tasks.

Kein automatisches Beheben, kein erneutes Review, keine erneute Ausführung.

**Trifft sie nicht**, lies den Task bis vor seine ersten Lauf-Abschnitte: Mit
`k_playbook_doc_grep` die erste Zeile `^## Ausführung$` oder `^### Diff: ` suchen; gibt es
eine, `k_playbook_doc_read` mit `lines: "1-<zeile davor>"`, sonst `lines: "1-"`. Das
`## Review-Log` kann übersprungene Punkte und Deadlocks aus dem Refine tragen, die für die
Rückfragen in 5b gebraucht werden. Mehr liest der Hauptkontext von diesem Task nicht.

### 5a.1 — Ausführungskontext und Branch-Preflight

Trägt der Task einen Abschnitt `## Ausführungskontext`, nimm daraus, soweit vorhanden:

- `Target repo:`
- `Base branch:`
- `Work branch:`
- `PR required:`
- `Dirty worktree policy:`

Behalte die Werte für den Erfolgspfad. Ist `PR required` wahr, muss der Command nach dem
lokalen Commit einen PR eröffnen oder genau den fehlenden Schritt nennen, der das verhindert.

`Target repo` wird relativ zu `context.project.repoRoot` aufgelöst, wenn es nicht absolut
ist. Ohne `Target repo` ist die Projektwurzel die Ausführungswurzel (`EXEC_ROOT`).

Vor der Delegation der Branch-Preflight in der Ausführungswurzel:

1. Prüfe, dass sie existiert und ein Git-Repository ist (`git rev-parse --is-inside-work-tree`).
   Wenn nicht: anhalten und den Nutzer fragen.
2. Prüfe den Dirty-State mit `git status --short`.
3. Ist er nicht leer: anhalten und die Dateien zeigen. Weiter nur, wenn der Nutzer ausdrücklich
   bestätigt, dass diese Dateien für den Task erwartet sind; sonst nicht delegieren.
4. Ist `Work branch` gesetzt:
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

Ohne `## Ausführungskontext` läuft alles wie bisher in der Projektwurzel.

### 5a.2 — Etappen-Fortschritt prüfen

Ein Task darf seine Arbeit in `## Zu bauen` als `### Etappe N — Titel` gliedern. Nur dann
greift die Fortschrittsverfolgung; ein Task ohne Etappen läuft als Ganzes.

Enthält der Task Etappen (`k_playbook_doc_grep`, `^### Etappe [0-9]+ — `; ihre Zahl ist `m`):

1. Den Stand geben allein die Fortschrittszeilen nach `on-demand/tasks.md`, „Fortschritt
   anhängend“: `k_playbook_doc_grep` mit
   `^<!-- k-task-run: (etappe [0-9]+ erledigt|fortschritt zurückgesetzt) ` liefert sie mit
   Zeilennummern. Es gilt je Etappe die letzte Zeile nach dem letzten Zurücksetzen; eine
   Etappe ohne Zeile ist offen. Nicht der Code, nicht `git log`, nicht eine ältere Tabelle
   `## Fortschritt`.
2. Steht keine Etappe auf erledigt: weiter mit 5b, alle Etappen offen.
3. Stehen Etappen auf erledigt: Fertig-Meldung (`Rückfrage: Etappen fortsetzen`), den Stand
   zeigen und fragen:

```
<TASK_FILE>: Etappen 1-2 von 6 erledigt (zuletzt 2026-08-12).

Wie weiter?
  (a) ab Etappe 3 fortsetzen   (Default)
  (b) von vorn beginnen        - erledigte Etappen erneut ausführen
  (c) diesen Task überspringen
```

Bei `(b)` vor der Delegation mit `k_playbook_doc_append` und `since_revision: REVISION` die
Zeile anhängen:

```text
<!-- k-task-run: fortschritt zurückgesetzt <context.now.date> -->
```

Der Stand wird **nicht** aus einem Abbruch heraus geraten. Eine offene Etappe gilt als nicht
ausgeführt, auch wenn Teile davon im Code sichtbar sind — der Sub-Agent prüft zu Beginn der
Etappe selbst, was schon da ist.

### 5b — Vor der Delegation klären

**Vor** dem Start des Sub-Agenten: Finde alles im Task, was unklar oder mehrdeutig ist oder
eine Entscheidung verlangt, die sich aus der Beschreibung nicht ableiten lässt. Die Fragen
stützen sich allein auf den gelesenen Task; der Hauptkontext liest dafür keinen Code. Was
sich nur am Code klären lässt, bleibt dem Ausführungs-Sub-Agenten — trifft er dort auf eine
echte Entscheidung, meldet er sie als Blocker (5d).

Gibt es solche Fragen: **anhalten und den Nutzer fragen** (Fertig-Meldung vorher). Auf die
Antworten warten. Diesen Schritt nicht überspringen, nicht raten und keine
Quick-and-dirty-Entscheidung treffen, um die Frage zu vermeiden — Mehrdeutiges wird im
Hauptkontext geklärt, wo der Nutzer antworten kann.

Weiter erst, wenn alle offenen Fragen beantwortet sind.

### 5b.1 — Snapshot des Tasks

Nur mit `DIFF_ENABLED=true`. Der Snapshot hält den Arbeitsbaum der Ausführungswurzel vor dem
Task fest, samt nicht committeter und ungetrackter Änderungen. Das Review vergleicht später
den Stand nach dem Task damit, einschließlich neu angelegter Dateien. In den Hauptkontext
gelangt nur seine Kennung.

Der Snapshot gehört zum Task, nicht zu einer Delegation. Er entsteht beim ersten Start des
Tasks — unmittelbar vor seiner ersten Delegation, nach dem Branch-Preflight, in der
Ausführungswurzel — und gilt für jeden weiteren Start: nach 5d und in jedem späteren Lauf,
ob der offene Etappen fortsetzt oder von vorn beginnt. Ein neuer Snapshot enthielte schon
die Änderungen des früheren Versuchs; ein kritischer Fehler daraus ginge ungeprüft nach
`done/`.

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
   Dokument trägt einen Vermerk aus 5e, eine Etappe stand auf erledigt, oder eine
   Snapshot-Zeile ließ sich nicht mehr auflösen —, ist er `nur seit Fortsetzung`: Das
   Review deckt dann nur die Änderungen seit diesem Snapshot ab, sagt das in Abschnitt und
   Rückgabe, und 5f fragt vor dem Verschieben nach `done/` (5f, Schritt 5).

Schlägt ein Aufruf fehl, hält der Command vor der Delegation an und fragt den Nutzer, wie in
5a.1. Führt der Nutzer den Task ohne Snapshot aus, läuft er ohne Diff und Review, wie ohne
`DIFF_ENABLED`, und Schritt 7 nennt das.

### 5c — An den Sub-Agenten delegieren

Vorher die Fortschrittszeilen zählen (`k_playbook_doc_grep`,
`^<!-- k-task-run: etappe [0-9]+ erledigt `, `count_only`) und Anzahl und Revision merken;
die Revision muss `REVISION` sein (Revisionskette, Schritt 1).

Starte einen General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code:
`general-purpose`) für die Ausführung. Er bekommt den Verweis, nicht den Inhalt:

- an den Anfang seines Auftrags: Führe zuerst `k-playbook briefing` aus und lies die
  Ausgabe; lies dann alle `CLAUDE.md` und `AGENTS.md` im Projektbaum,
- `TASK_REF` und `REVISION` — er liest den Task selbst mit `k_playbook_doc_read` und
  `k-playbook:/instructions/on-demand/tasks.md` dazu,
- das Arbeitsverzeichnis,
- bei `## Ausführungskontext`: die aufgelöste Ausführungswurzel, Base- und Work-Branch,
  PR-Pflicht, die Entscheidung zum schmutzigen Arbeitsbaum und das Ergebnis des
  Branch-Preflights als verbindlichen Rahmen,
- alle Klärungen aus 5b als zusätzlichen Kontext,
- bei `### Etappe`-Abschnitten: welche Etappen offen sind, und die Anweisung, **unmittelbar
  nach Abschluss jeder Etappe** — vor der nächsten, nicht am Ende des Laufs — die
  Fortschrittszeile `<!-- k-task-run: etappe <n> erledigt <datum> --> <beleg>` mit
  `k_playbook_doc_append` und `since_revision` anzuhängen, Form nach `on-demand/tasks.md`,
  `<datum>` aus `context.now.date` seines Briefings,
- die Anweisung, am Task keinen Abschnitt `## Ausführung` und keine andere Kennzeile
  anzulegen oder zu ändern — die schreibt der Hauptkontext —; was der Task selbst in seinem
  Dokument verlangt, etwa einen eigenen Abschnitt, hängt der Sub-Agent wie verlangt an,
- die Regeln zu `foreign_change` und `outcome_unknown` aus `on-demand/tasks.md`: bei
  `foreign_change` anhalten, nichts weiter schreiben, `Foreign-Change:` zurückgeben,
- das Rückgabeformat unten.

Kein Diff und keine Kennung eines Snapshots: Diff und Review sind Sache des
Review-Sub-Agenten in 5f.

Die Fortschrittszeile schreibt der Sub-Agent selbst, nicht der Hauptkontext. Nur so
überlebt der Stand einen harten Abbruch: bricht die Sitzung mitten in Etappe 4 ab, stehen
die Zeilen für 1 bis 3 bereits im Dokument. Ein Fortschritt, den erst der Hauptkontext nach
Rückkehr des Sub-Agenten schreibt, wäre genau in dem Fall verloren, für den er gedacht ist.

Der Sub-Agent fragt den Nutzer nicht — alle Mehrdeutigkeiten sind geklärt. Trifft er auf
einen unerwarteten Blocker oder eine Entscheidung, die er nicht sauber auflösen kann, trifft
er **keine** Quick-and-dirty-Entscheidung, sondern hält an und meldet sie unter „Blocker“,
damit der Hauptkontext den Nutzer fragen kann.

Die Rückgabe des Sub-Agenten ist genau dieses Format, ausdrücklich ohne Diffs,
Dateiinhalte und Logs:

```
Status: erfolgreich | teilweise | blockiert
Zusammenfassung: <2–3 Sätze>
Validierung: <Befehl → Ergebnis, je eine Zeile>
Abgebrochen bei: – | <Schritt oder Etappe, an der die Arbeit stoppte>
Blocker: keine | <Beschreibung + benötigte Entscheidung>
Revision: <n>
```

- `Revision` ist die Revision seines letzten eigenen Anhängens; hat er nichts angehängt, die
  übergebene. Nach `foreign_change` steht stattdessen
  `Foreign-Change: <revision> <updatedBy> <updatedAt>`, und `Status` ist `blockiert`.
- `erfolgreich` führt nach 5f, `blockiert` nach 5d, `teilweise` nach 5e. Eine Rückgabe ohne
  gültige Zeile `Status:` gilt als `blockiert`.
- Die Zeilen „Validierung“ gehen in 5f an das Review und speisen den PR-Text in 5f.1.
- `REVISION` wird nach der Revisionskette aus Schritt 1 gesetzt: aus `Revision:`, nach
  `Foreign-Change:` erst nach der Rückfrage, ohne beides durch erneutes Zählen.

Warte, bis der Sub-Agent fertig ist, bevor der nächste Schritt beginnt.

### 5d — Unerwartete Blocker behandeln

Meldet die Rückgabe `Status: blockiert`: **anhalten und den Nutzer fragen**, mit der Zeile
„Blocker“ — vorher die Fertig-Meldung (`Rückfrage: Blocker`, der Body nennt ihn). Steht
`Foreign-Change:` in der Rückgabe, ist das die Rückfrage nach `foreign_change` aus Schritt 1.
Kein nächster Task, bis das geklärt ist. Dann entscheiden, ob der Task erneut läuft oder
abgebrochen wird (5e).

Erneut ausführen heißt: 5c noch einmal, mit der Antwort des Nutzers als weiterer Klärung.
Der Snapshot aus 5b.1 bleibt derselbe.

### 5e — Bei Fehler oder Abbruch

Gilt bei `Status: teilweise`, bei einem nach 5d abgebrochenen Task und wenn 5f eine offene
Etappe findet — immer vor dem Kopf aus 5f. Mit `k_playbook_doc_append` und
`since_revision: REVISION` anhängen:

```text
## Ausführung

**Status:** Teilweise ausgeführt - abgebrochen  
**Datum:** <context.now.date>  
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

Nach dem Kopf aus 5f kommt 5e nicht mehr vor: Ein Task verlässt den Lauf dann nur nach
`done/` oder mit der Zeile „angehalten“ (5f, Schritt 5).

### 5f — Bei Erfolg

Bei `Status: erfolgreich`, in genau dieser Reihenfolge. So wird kein unvollständiger Task
reviewt, und innerhalb eines Laufs steht nie ein Erfolgskopf neben einem Abbruchvermerk aus
5e.

**1. Etappen prüfen.** Hat der Task `### Etappe`-Abschnitte: die Fortschrittszeilen erneut
zählen (wie 5a.2). Hat eine Etappe keine Zeile nach dem letzten Zurücksetzen, ist der Lauf
**nicht** erfolgreich: ohne Kopf und ohne Review weiter nach 5e, mit der ersten offenen
Etappe unter „Abgebrochen bei“ und dem Grund, dass der Sub-Agent Erfolg meldet, während die
Etappe offen steht. Ein solcher Widerspruch wird gezeigt, nicht geglättet.

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

**3. Rezept bestimmen**, ohne es zu lesen: aus `documents.reviews` des Briefings der Eintrag
mit `ref` `k-playbook:/reviews/review-code.md`. So greift ein Overlay unter
`<context.local.dir>/reviews/`.

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
Revision aus dieser Antwort muss `REVISION` sein. Dann einen neuen General-Purpose-Sub-Agenten
starten — nicht den ausführenden: Wer den Code geschrieben hat, prüft ihn nicht selbst. Er
bekommt:

- den Verweis `DIFF_REVIEW_MODULE` mit der Anweisung, das Modul mit `k_playbook_doc_read` zu
  lesen und zu befolgen,
- `TASK_REF` und `REVISION`, die Ausführungswurzel, `SNAPSHOT` und `REVIEW_SCOPE`,
- die Zeilen „Validierung“ aus der letzten Rückgabe von Ausführung oder Behebung,
- `REVIEW_RECIPE` oder die Anweisung „nur Teil 1“.

Er fragt den Nutzer nicht und hängt seine Abschnitte selbst an den Task an. Seine Rückgabe
beschreibt das Modul; sie endet mit `Revision: <n>` oder `Foreign-Change: …`. Danach
dieselben Zeilen erneut zählen:

- Das Review ist **gültig**, wenn die Rückgabe dem Format des Moduls folgt und
  `### Review: Befunde` genau einmal mehr dasteht als vorher — nie gilt ein Lauf ohne das als
  „0 kritisch“.
- Die Diff-Abschnitte hat der Lauf geschrieben, wenn `### Diff: Geänderte Dateien` genau
  einmal mehr dasteht.
- `REVISION` wird nach der Revisionskette gesetzt; `Foreign-Change:` führt zur Rückfrage aus
  Schritt 1, und nach „weiter“ läuft Schritt 4 erneut.

**5. Stopp-Abfrage** — nach dem Review, vor dem Verschieben nach `done/`, vor 5f.1 und vor
dem nächsten Task. Sie kommt, sobald einer dieser Gründe vorliegt:

| Grund | Die Abfrage nennt | (a) |
|---|---|---|
| gültiges Review mit mindestens einem kritischen Befund | die Zeilen unter „Kritisch“ aus der Rückgabe | beheben lassen |
| kein gültiges Review: Abbruch, Rückgabe nicht im Format oder kein neuer Review-Abschnitt | dass kein gültiges Review vorliegt, dazu eine Zeile `Abbruch:` | Review erneut laufen lassen |
| Rezept fehlt im Briefing | dass kein gültiges Review vorliegt, weil `review-code` unter `documents.reviews` fehlt | entfällt |
| `REVIEW_SCOPE` ist `nur seit Fortsetzung` | „Review nur seit Fortsetzung“ | entfällt |

Mehrere Gründe stehen in einer Abfrage; (a) steht zur Wahl, solange einer mit (a) dabei ist.
Vor der Abfrage die Fertig-Meldung (`Rückfrage: <Grund>`, der Body nennt die Zahlen).
Fehlt das Rezept, gilt dafür nur die dritte Zeile, nicht die zweite — ein erneutes Review
fände das Rezept ebenso wenig.
Für kritische Befunde:

```
<TASK_FILE>: <n> kritische Review-Befunde
  - <datei:zeile oder Prüfbefehl> — <ein Satz>

Wie weiter?
  (a) beheben lassen   - Sub-Agent behebt nur diese Befunde, danach erneutes Review
  (b) trotzdem weiter  - Task gilt als erledigt, Befunde bleiben dokumentiert
  (c) anhalten         - Task bleibt liegen, keine weiteren Tasks
```

Bei den anderen Gründen steht statt der Befundzeilen `<TASK_FILE>: kein gültiges Review —
<Grund>` bzw. `<TASK_FILE>: Review nur seit Fortsetzung — ein Fehler aus dem früheren
Versuch bleibt ungeprüft`.

- **(a) beheben lassen.** Ein General-Purpose-Sub-Agent bekommt `TASK_REF`, den Rahmen aus 5c
  (Ausführungswurzel, Branch, Preflight) und die kritischen Befunde. Er behebt nur diese,
  schreibt nichts in den Task, fragt den Nutzer nicht, meldet eine offene Entscheidung unter
  „Blocker“ und gibt das Format aus 5c zurück (`Revision:` ist dann die übergebene).
  - `erfolgreich`: zurück zu 5f, Schritt 4 — das Review läuft erneut gegen denselben Snapshot
    mit demselben Umfang und hängt seine Abschnitte an; die früheren bleiben. Bleibt etwas
    kritisch, wird wieder gefragt.
  - `blockiert`: fragen wie in 5d. „Erneut“ heißt hier nur, den Behebungs-Sub-Agenten mit
    der Antwort des Nutzers noch einmal zu starten — nie zurück nach 5a. Wird er nicht
    erneut gestartet, weiter wie (c) mit dem Grund „Behebung blockiert, kritische
    Review-Befunde offen“.
  - `teilweise`: weiter wie (c) mit dem Grund „Behebung unvollständig, kritische
    Review-Befunde offen“.

  Die Vorlage aus 5e kommt dabei nicht vor.
- **(a) Review erneut laufen lassen** (kein gültiges Review): zurück zu 5f, Schritt 4 mit
  denselben Eingaben.
- **(b) trotzdem weiter.** Weiter mit 5f, Schritt 6. Liegt kein gültiges Review vor und hat der
  letzte Review-Lauf keine Diff-Abschnitte geschrieben (Zählung aus 5f, Schritt 4), startet der
  Hauptkontext davor einen Sub-Agenten nur mit Teil 1 des Moduls.
- **(c) anhalten.** Mit `k_playbook_doc_append` und `since_revision: REVISION` anhängen; die
  bestehende Statuszeile bleibt stehen:

  ```text
  **Status nach Review:** angehalten — <Grund>
  ```

  Grund ist „kritische Review-Befunde offen“, „kein gültiges Review“, „Review nur seit
  Fortsetzung“ oder der Grund aus (a). Der Task bleibt liegen, die Fortschrittszeilen
  unverändert; kein PR, keine weiteren Tasks. Die Kennzeile aus 5f, Schritt 2 steht schon im
  Dokument; den nächsten Lauf regelt 5a.

**6. Abschluss.**

1. `k_playbook_doc_done` mit `TASK_REF`. Die Antwort nennt den neuen Verweis unter `done/`
   (`TASK_REF_DONE`) und `revision` — das Verschieben erzeugt eine neue Revision, und sie ist
   ab jetzt `REVISION` für alles, was noch an diesen Task angehängt wird (Schritt 6).
   `foreign_change` gibt es hier nicht; `outcome_unknown` heißt: mit `k_playbook_doc_list`
   (Sorte `tasks`, `done: true`) nachsehen, nicht noch einmal verschieben.
2. Snapshot freigeben — die Ref wird mit dem Verschieben nach `done/` entfernt:

   ```bash
   if command -v git >/dev/null 2>&1; then
     git -C "<EXEC_ROOT>" update-ref -d "refs/k-task-run/<TASK_STEM>"
   fi
   ```

### 5f.1 — PR-Handoff bei `PR required: true`

Erst nach der Stopp-Abfrage aus 5f. Trägt der `## Ausführungskontext` des Tasks
`PR required: true`, folgt der PR, nachdem der Task abgeschlossen ist und ein lokaler Commit
existiert.

Preflight:

1. In der aufgelösten Ausführungswurzel aus 5a.1 arbeiten.
2. Prüfen, dass der Branch `Work branch` ist, wenn einer angegeben war.
3. Prüfen, dass der Arbeitsbaum sauber ist (`git status --short`); sonst anhalten und melden,
   dass vor dem PR ein Commit nötig ist.
4. Prüfen, dass der Branch einen Upstream hat. Ohne Upstream vor dem Push fragen; nie still
   pushen.

PR-Text:

- Den PR-Text als echtes mehrzeiliges Markdown in eine temporäre Datei schreiben, nicht als
  zitierten CLI-String mit `\n`.
- `/tmp/k-task-run-pr-body-<tasknummer-oder-branch-slug>.md`, sofern kein besserer
  temporärer Pfad schon da ist.
- Inhalt: kurze Zusammenfassung der Änderung; Befund-IDs oder der `keph:`-Verweis des Tasks;
  die Zeilen „Validierung“ aus der letzten Rückgabe von Ausführung oder Behebung; bekannte
  Restrisiken oder Tests, die nicht laufen konnten.

PR anlegen mit `gh pr create --body-file <datei>`:

```bash
gh pr create \
  --base <Base branch> \
  --head <Work branch> \
  --title "<knapper Titel>" \
  --body-file /tmp/k-task-run-pr-body-<slug>.md
```

Kein `--body "...\n..."`: Bash übergibt in normalen doppelten Anführungszeichen ein
wörtliches Backslash-n. Ist ein Inline-Body unvermeidlich, ein Heredoc oder ANSI-C-Quoting
nehmen; `--body-file` bleibt der Weg.

Ist `gh` nicht da oder nicht angemeldet (`context.gh.ready` falsch), den genauen
`gh pr create --body-file ...`-Befehl und den Inhalt der Body-Datei zum Nachmachen ausgeben.

### 5g — Weiter

Weiter mit dem nächsten Task der Liste. Ist ein Task fehlgeschlagen (5e), angehalten (5f,
Schritt 5, oder 5a, Option (b)) oder nach `foreign_change` liegen geblieben: anhalten — keine
weiteren Tasks.

## Schritt 6 — Intent-Alignment-Check

Sind alle Tasks nach `done/` gewandert **und** trägt der letzte Task einen `## Intent`:
prüfen, ob die ausgeführte Arbeit den Intent tatsächlich erreicht.

Dieser Schritt liest keinen Task. Den Intent hat der Hauptkontext aus Schritt 2; was
ausgeführt wurde, sind die Zusammenfassungen aus den Rückgaben der Sub-Agenten in diesem
Lauf. Für einen Task, der nach 5a (a) ohne Rückgabe in diesem Lauf abgeschlossen wurde, holt
der Hauptkontext die erste Zeile `^\*\*Zusammenfassung:\*\*` nach seiner Kennzeile mit
`k_playbook_doc_grep` (unter `TASK_REF_DONE`).

Spawn a general-purpose subagent (OpenCode: `general`, Claude Code: `general-purpose`)
as Critic with this prompt:

```
You are doing a final alignment check after a set of tasks was executed.
Below is the Intent (the goal the tasks were supposed to achieve) and a summary of what was actually done.

## Intent
<insert intent text>

## Was ausgeführt wurde
<insert the summaries from the sub-agents' returns, one per task>

Answer only: Does the executed work achieve the stated Intent?
- Yes -> one sentence why
- Partially -> what is missing or not yet covered
- No -> what is misaligned or missing

Output (no intro text):
| Alignment | Begründung |
```

Das Ergebnis mit `k_playbook_doc_append` an den letzten Task anhängen, jetzt unter
`TASK_REF_DONE`, mit `since_revision` = der `revision` aus der Antwort von
`k_playbook_doc_done`:

```text
**Intent-Alignment:** <Ja / Teilweise / Nein> - <Begründung>
```

Ist das Alignment **nicht Ja**: vor der abschließenden Zusammenfassung eine klare Warnung:

```
WARNUNG: Intent nicht vollständig erreicht: <Begründung>
```

Ohne Intent entfällt dieser Schritt still.

## Schritt 7 — Abschließende Zusammenfassung

Nach allen Tasks zuerst die Fertig-Meldung (`fertig` oder `abgebrochen`; der Body trägt die
Zeilen „Erfolgreich“, „Angehalten“ und „Abgebrochen“ in Kurzform), dann eine knappe
Zusammenfassung:

```
Ausgeführt:    <n> Tasks
Erfolgreich:   <Verweise unter done/>
  <TASK_FILE>  Review: <n> kritisch, <n> wichtig, <n> Hinweis
Abgeschlossen: <Tasks, die der Nutzer in 5a, Option (a), abgeschlossen hat>
Angehalten:    <TASK_FILE> - <Grund>
Abgebrochen:   <TASK_FILE, falls vorhanden> - <Grund>
Übersprungen:  <TASK_FILEs, falls vorhanden>
Intent:        Ja / Teilweise / Nein / - (kein Intent)
```

Die Zeile „Review“ steht je erfolgreichem Task und gibt das letzte gültige Review wieder:

- bei `REVIEW_SCOPE` `nur seit Fortsetzung` mit dem Zusatz „— nur seit Fortsetzung“,
- `Review: abgeschaltet` bei abgeschaltetem Rezept,
- `Review: kein gültiges Review` nach (b) ohne gültiges Review,
- `Review: – (ohne Diff)` ohne `DIFF_ENABLED` oder ohne Snapshot.

Bei „Abgeschlossen“ steht ein offener PR-Schritt dabei, wenn der Task `PR required: true`
trägt. Ein nach `foreign_change` liegen gebliebener Task steht unter „Angehalten“ mit dem
Grund „fremder Schreiber“.

Folge-Command: **`/k-danke`** — schließt die Sitzung ab: legt die Befunde vor, die während
der Ausführung festgehalten wurden, und prüft den Docs-Nachzug.
