---
description: Execute one or more task files. If no path is given, uses the project's task directory. Pass a single .md file or a directory to override. Multiple tasks are executed in order by their numeric prefix. Execution, diff and code review run in sub-agents that write into the task file themselves. On success, the file moves to done/ unless a critical review finding stops the run first. On partial execution or error, appends a status note and leaves the file in place.
argument-hint: [file-or-directory]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, Task, Agent]
---

# k-task-run

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Execute task files. If `$ARGUMENTS` is empty, use the project's task directory.

Der Hauptkontext steuert und fragt, Sub-Agenten arbeiten und schreiben. Ausführung, Diff
und Code-Review laufen je Task in Sub-Agenten, die ihre Ergebnisse selbst in die Task-Datei
schreiben und dem Hauptkontext nur ein festes, knappes Format zurückgeben. Der Hauptkontext
sieht pro Task keinen Diff, keine Dateiinhalte außer der Task-Datei selbst, keine Logs und
kein vollständiges Review. Alle Rückfragen an den Nutzer bleiben im Hauptkontext.

Produces:
- In jeder ausgeführten Task-Datei den Abschnitt `## Ausführung` mit Kennzeile, Status und
  Zusammenfassung, die Diff- und Review-Abschnitte des Review-Sub-Agenten und, bei
  Etappen, die Tabelle `## Fortschritt`.
- Erfolgreich abgeschlossene Task-Dateien unter `<TASKS_DISPLAY_PATH>/done/`.
- Je Task eine Ref `refs/k-task-run/<TASK_STEM>` in der Ausführungswurzel, bis der Task
  nach `done/` wandert.
- Bei `PR required: true` einen Pull Request über `gh`.

**Hauptkontext und Task-Datei.** Für den Umgang des Hauptkontexts mit Task-Dateien gilt im ganzen Command:

- **Lesen nur per Shell mit `sed -n`** — die ganze Datei mit `sed -n '1,$p' "<datei>"`,
  einen Abschnitt mit `sed -n '<von>,<bis>p' "<datei>"` —, nie mit dem Lese-Werkzeug des
  Assistenten und nicht mit `cat`. Claude Code merkt sich mit dem Lese-Werkzeug gelesene
  Dateien, ebenso per `cat` gelesene, und spielt spätere fremde Änderungen samt der
  geänderten Zeilen als Hinweis in den Hauptkontext ein — hier `## Fortschritt`, Diff und
  Review. Nach `sed -n` blieb dieser Hinweis aus.
- **Suchen mit dem Such-Werkzeug** des Assistenten (Grep). Es merkt sich nichts und bleibt
  immer erlaubt.
- **Schreiben nur per Shell**, ohne die Datei dafür zu lesen, nie mit Edit/Write in Claude
  Code oder `edit` in OpenCode: Diese verlangen ein Lesen mit dem Lese-Werkzeug und nach
  einer fremden Änderung ein erneutes, das Diff und Review in den Hauptkontext holte. Neues
  wird ans Dateiende angehängt, per Heredoc mit einer Endmarke in Anführungszeichen und
  einer Leerzeile vorweg, damit der Eintrag am Zeilenanfang beginnt. Freier Text —
  Zusammenfassung, Grund, Begründung — steht nie in doppelten Anführungszeichen, etwa als
  Argument von `printf` oder `echo`: Die Shell führte Backticks und `$(…)` darin als Befehl
  aus, und Begründungen von Sub-Agenten tragen oft Bezeichner in Backticks.
- **Nach dem Kopf** (4f, Schritt 2) liest der Hauptkontext die Task-Datei nicht mehr, auf
  keinem Weg, und ändert keine bestehende Zeile mehr; er sucht nur noch per Grep und hängt
  per Shell an.

**Kennzeilen.** `/k-task-run` erkennt den Zustand einer Task-Datei an festen Zeilen, je eine
eigene Zeile am Zeilenanfang. `<datei>` ist der Dateiname der Task-Datei, etwa
`014-setup-tts.md`:

| Zeile | Schreibt | Bedeutung |
|---|---|---|
| `<!-- k-task-run: snapshot <datei> <kennung> -->` | Hauptkontext, 4b.1 | Snapshot des Tasks |
| `<!-- k-task-run: ausgeführt <datei> -->` | Hauptkontext, mit dem Kopf in 4f | ausgeführt; solange die Datei nicht in `done/` liegt, nicht abgeschlossen |
| `### Diff: Geänderte Dateien` | Review-Sub-Agent | Diff-Abschnitte eines Laufs |
| `### Review: Befunde` | Review-Sub-Agent | Review-Abschnitt eines Laufs |

Gesucht wird am Zeilenanfang und am Zeilenende verankert, Punkte im Dateinamen maskiert. Die
beiden Kommentarzeilen tragen den eigenen Dateinamen: Eine Task-Datei, die die Vorlagen
dieses Commands zitiert, trägt dort `<datei>` oder einen fremden Namen und gilt deshalb nicht
als ausgeführt. Die beiden Überschriftenzeilen sind der Vertrag mit dem Modul
`commands/_task-run/diff-review.md`; es beschreibt auch das Rückgabeformat des
Review-Sub-Agenten.

## Schritt 1 — Zielpfad auflösen und Tasks sammeln

The context load from the first step is the preflight, even for explicit file or
directory arguments: task execution resolves `## Ausführungskontext` paths relative to
`project.repoRoot` from that output.

From the context output:

- `RESOLVED_TASKS_DIR = <local.dir>/tasks`.
- `TASKS_DISPLAY_PATH = k-playbook-local/tasks`.

Command-specific policy:

- If `$ARGUMENTS` is provided: treat it as the explicit execution target.
  - If it is a single `.md` file: use that file as a one-item list.
  - If it is a directory: use that directory.
  - If it does not exist: abort with a clear error.
- If `$ARGUMENTS` is empty:
  - If `RESOLVED_TASKS_DIR` is missing on disk: abort and tell the user to run `/k-gui`. Do not create it from `/k-task-run`; there are no tasks to execute. Das ist eine
    begründete Abweichung von `rules/command-authoring.md`, Abschnitt „Fehlendes
    Verzeichnis": Ohne Task-Verzeichnis gibt es nichts auszuführen, deshalb keine
    Rückfrage nach dem Anlegen.
  - Otherwise use it as the execution target.

Remember the chosen absolute target as `RUN_TARGET` and the display path as `RUN_TARGET_DISPLAY`.

Collect tasks:

- If `RUN_TARGET` is a file: use that file as a one-item list.
- If `RUN_TARGET` is a directory:
  - Find all `.md` files directly in that directory (not subdirectories).
  - Exclude `done/`, `old/`, and any archived/completed task subdirectories.
  - Sort by the leading number in the filename (e.g. `013-foo.md` before `014-bar.md`; also accept `_`).
  - Skip files without a leading number.
  - If no runnable `.md` files are found: report "Keine offenen Task-Dateien gefunden" and stop.

Announce the list of tasks to be executed before starting. Check the **last** task file for an `## Intent` section and include it in the announcement if present:

```
Tasks:
  Pfad: <TASKS_DISPLAY_PATH>/
  1. 014-setup-tts.md
  2. 015-integrate-tts.md  <- letzte

Intent (aus letzter Datei):
  <intent text>

Tools (zusätzlich): ...
```

If any task file contains a `## Tools` section: collect all listed tools across all tasks and show them to the user upfront before executing anything. This allows the user to grant additional permissions before the run starts.

`## Intent` der letzten und `## Tools` jeder Task-Datei werden nur als Abschnitt gelesen, nie
die ganze Datei — eine Datei kann schon Diff und Review tragen. Die Überschriftenzeile
(`^## Intent`, `^## Tools`) per Grep mit Zeilennummer finden, ebenso die Zeilen `^## `
dahinter, dann per Shell nur von der gesuchten bis vor die nächste Zeile `^## ` lesen
(`sed -n '<von>,<bis>p' <datei>`). Maßgeblich ist der erste Treffer; trägt die Datei die
Kennzeile „ausgeführt", zählen nur Treffer vor ihr, damit ein Ausschnitt in
„Code-Änderungen" nicht als Abschnitt gilt. Den Intent behält der Hauptkontext für Schritt 5.

## Schritt 2 — Task-Refine-Status prüfen

Ein Task, der nie durch `/k-task-refine` gegangen ist, wurde nie gegengelesen. Prüfe für
jede gesammelte Task-Datei per Grep (`^## Review-Log`), ob sie eine `## Review-Log`-Sektion
enthält; gelesen wird dafür keine Datei.

Wenn **alle** Dateien ein Review-Log tragen: weiter, ohne Rückfrage.

Wenn mindestens eine keines hat: die betroffenen Dateien in **einer** Nachricht nennen
und fragen:

```
Ohne Task-Refine:
  - 014-setup-tts.md
  - 015-integrate-tts.md

Diese Tasks wurden nicht mit /k-task-refine gegengelesen.
Trotzdem ausführen? (ja / nein / zuerst reviewen)
```

- `ja` — weiter mit Schritt 3.
- `nein` — abbrechen, nichts ausführen.
- `zuerst reviewen` — abbrechen und wörtlich `/k-task-refine <RUN_TARGET_DISPLAY>`
  nennen, danach `/k-task-run` erneut starten. Den Task-Refine-Command nicht selbst aufrufen.

Bei explizitem Datei-Argument gilt dieselbe Prüfung, nur für die eine Datei.

`/k-task-refine` hängt sein Log an **jede geprüfte** Datei an, auch an die, an der nichts
zu ändern war (dort mit dem Vermerk „keine Änderungen"). Ein fehlender Block heißt
deshalb: diese Datei ist nie durch Task-Refine gegangen.

Trotzdem bleibt es eine Rückfrage und kein Abbruch. Eine von Hand geschriebene oder aus
einem anderen Projekt übernommene Task-Datei kann sachlich in Ordnung sein, ohne je
durch Task-Refine gegangen zu sein — das zu entscheiden ist Sache des Users, nicht des
Commands.

## Schritt 3 — Diff und Review vorbereiten

Ob ein Task einen Diff und ein Review bekommt, folgt aus der Context-Ausgabe — nichts zu
konfigurieren und nichts zu suchen:

- `project.vcs` ist `git`, und `git` steht nicht in `baseTools.missing`: `DIFF_ENABLED=true`.
- `project.vcs` ist `git`, aber `git` fehlt: `DIFF_ENABLED=false`. Einmal sagen, dass damit
  Diff, Review und der Stopp bei kritischen Befunden entfallen, und den Befehl aus
  `baseTools.installCommand` nennen.
- Sonst: `DIFF_ENABLED=false`, still weiter.

`DIFF_ENABLED` ist kein Guard im Sinn von `rules/command-authoring.md`; jeder `git`-Aufruf
in einem Shell-Codeblock dieses Commands hat seinen eigenen.

Mit `DIFF_ENABLED=true`:

- `DIFF_REVIEW_MODULE = <playbook.dir>/commands/_task-run/diff-review.md`; eine
  gleichnamige Datei unter `<local.dir>/commands/_task-run/` ersetzt es nach
  `rules/command-authoring.md`. Der Hauptkontext liest das Modul nicht, er gibt nur den Pfad
  weiter. Fehlt es oder ist es leer, vor dem ersten Task abbrechen und `/k-gui` oder ein
  Update nennen.
- Grundlage des Diffs ist ein Snapshot je Task (4b.1), kein Commit je Lauf. Er schließt nicht
  committete und ungetrackte Änderungen ein, auch die der Vorgänger im selben Lauf. Ein
  schmutziger Arbeitsbaum verfälscht den Diff deshalb nicht mehr, und einen Hinweis darauf
  gibt es hier nicht; ob schmutzige Dateien für einen Task erwartet sind, klärt der
  Branch-Preflight in 4a.1. Nicht erfasst sind Dateien, die `.gitignore` ausschließt:
  Änderungen daran erscheinen weder im Diff noch im Review.

## Schritt 4 — Tasks nacheinander ausführen

For each task file, **in strict sequential order** (never parallel - two agents must not modify code simultaneously):

### 4a — Lesen und verstehen

Zuerst per Grep die Kennzeile „ausgeführt" dieser Datei suchen
(`^<!-- k-task-run: ausgeführt <datei> -->$`).

**Trifft sie**, ist der Task ausgeführt, aber nicht abgeschlossen: nach der Stopp-Abfrage
in 4f angehalten, oder eine Sitzung endete zwischen Kopf und Verschieben nach `done/`. Die
Datei wird weder gelesen noch erneut ausgeführt. Frage:

```
<task>.md: bereits ausgeführt, nicht abgeschlossen — Review, sein Ergebnis oder
kritische Befunde offen (siehe ## Ausführung).

Wie weiter?
  (a) abschließen  - offene Punkte erledigt oder akzeptiert, Task wandert nach done/
  (b) anhalten     - der Lauf endet hier, keine weiteren Tasks
```

- (a): Per Shell anhängen:

  ```bash
  printf '\n%s\n' "**Status nach Review:** abgeschlossen vom Nutzer (<now.date>)" >> "<TASK_FILE>"
  ```

  Dann weiter wie 4f, Schritt 6 (nach `done/` verschieben, Snapshot freigeben); die
  Ausführungswurzel dafür ist `Target repo` aus `## Ausführungskontext` (per Grep, nur
  Treffer vor der Kennzeile), sonst die Projektwurzel. 4f.1 läuft nicht; steht dort
  `PR required: true`, nennt der Command den PR als offenen Schritt. Weiter mit dem nächsten
  Task.
- (b): Der Lauf endet, keine weiteren Tasks.

Kein automatisches Beheben, kein erneutes Review, keine erneute Ausführung.

**Trifft sie nicht**, die Task-Datei ganz per Shell lesen (`sed -n '1,$p' "<TASK_FILE>"`). Das
`## Review-Log` kann übersprungene Punkte und Deadlocks aus dem Refine tragen, die für die
Rückfragen in 4b gebraucht werden.

### 4a.1 — Ausführungskontext und Branch-Preflight

If the task contains a `## Ausführungskontext` section, parse these fields when present:

- `Target repo:`
- `Base branch:`
- `Work branch:`
- `PR required:`
- `Dirty worktree policy:`

Also keep these parsed values for the success path. If `PR required` is true, the command must either open a PR after the local commit exists or report the exact missing step that prevents PR creation.

Resolve `Target repo` relative to `project.repoRoot` from the context output unless it is absolute. If no `Target repo` is present, use the current project root as execution root.

Before delegating to a sub-agent, perform the branch preflight in the execution root:

1. Verify the execution root exists and is a Git repo with `git rev-parse --is-inside-work-tree`. If not, stop and ask the user.
2. Check dirty state with `git status --short`.
3. If dirty state is non-empty, stop and show the dirty files. Continue only if the user explicitly confirms that these files are expected for this task; otherwise do not delegate.
4. If `Work branch` is set:
   - If already on `Work branch`, continue.
   - Else if the local `Work branch` exists, switch to it with `git switch <Work branch>`.
   - Else if `Base branch` is set and not `<manual>`, switch to `Base branch` and create the work branch with `git switch -c <Work branch>`.
   - Else stop and ask which base branch should be used.
5. If both `Base branch` and `Work branch` are set and `Base branch` is not `<manual>`, verify the work branch is based on the intended base with `git merge-base --is-ancestor <Base branch> HEAD`. If this fails, stop and ask before continuing.
6. Record the preflight result and pass it to the sub-agent. The sub-agent must treat the selected execution root and branch as mandatory context.

If the task does not contain `## Ausführungskontext`, continue with the existing behavior.

### 4a.2 — Etappen-Fortschritt prüfen

Ein Task darf seine Arbeit in `## Zu bauen` als `### Etappe N — Titel` gliedern. Nur
dann greift die Fortschrittsverfolgung; ein Task ohne Etappen läuft wie bisher als
Ganzes.

Enthält die Task-Datei Etappen:

1. Den Stand gibt eine vorhandene `## Fortschritt`-Sektion aus dem Lesen in 4a. Sie ist die
   einzige Quelle für den Stand — nicht der Code, nicht `git log`.
2. Fehlt sie, per Shell anhängen: eine Zeile je Etappe, alle auf `offen`.

```bash
cat >> "<TASK_FILE>" <<'K_TASK_RUN_EOF'

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — <Titel> | offen | | |
| 2 — <Titel> | offen | | |
K_TASK_RUN_EOF
```

So sieht ein fortgeschriebener Stand aus:

```
## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Struktur festschreiben | erledigt | 2026-08-12 | local.go + Test grün |
| 2 — Command-Authoring-Regel | erledigt | 2026-08-12 | |
| 3 — /k-docs-index | offen | | |
```

3. Stehen bereits Etappen auf `erledigt`, den Stand zeigen und fragen:

```
014-docs-umbau.md: Etappen 1-2 von 6 erledigt (zuletzt 2026-08-12).

Wie weiter?
  (a) ab Etappe 3 fortsetzen   (Default)
  (b) von vorn beginnen        - erledigte Etappen erneut ausführen
  (c) diesen Task überspringen
```

Bei `(b)` alle Zeilen per Shell auf `offen` zurücksetzen, bevor delegiert wird:

```bash
sed -i.k-task-run.bak -E '/^## Fortschritt *$/,/^(## |---)/ s/^([|][^|]*[|]) *erledigt *[|][^|]*[|][^|]*[|] *$/\1 offen | | |/' "<TASK_FILE>" && rm -f "<TASK_FILE>.k-task-run.bak"
```

Der Stand wird **nicht** aus einem Abbruch heraus geraten. Steht eine Etappe auf `offen`,
gilt sie als nicht ausgeführt, auch wenn Teile davon im Code sichtbar sind — der
Sub-Agent prüft zu Beginn der Etappe selbst, was schon da ist.

### 4b — Vor der Delegation klären

**Before** spawning the sub-agent: identify anything in the task file that is unclear, ambiguous, or requires a decision that cannot be inferred from the task description. The questions rest on the task file alone; the main context reads no code for them. What can only be clarified at the code is left to the execution sub-agent — if it hits a real decision there, it reports it as a blocker (4d).

If any such questions exist: **stop and ask the user**. Wait for answers before continuing. Do not skip this step, do not guess, and do not make quick-and-dirty decisions to avoid asking - ambiguities must be resolved in the main context where the user can answer them.

Only proceed once all open questions are resolved.

### 4b.1 — Snapshot des Tasks

Nur mit `DIFF_ENABLED=true`. Der Snapshot hält den Arbeitsbaum der Ausführungswurzel vor dem
Task fest, samt nicht committeter und ungetrackter Änderungen. Das Review vergleicht später
den Stand nach dem Task damit, einschließlich neu angelegter Dateien. In den Hauptkontext
gelangt nur seine Kennung.

Der Snapshot gehört zum Task, nicht zu einer Delegation. Er entsteht beim ersten Start des
Tasks — unmittelbar vor seiner ersten Delegation, nach dem Branch-Preflight, in der
Ausführungswurzel — und gilt für jeden weiteren Start: nach 4d und in jedem späteren Lauf,
ob der offene Etappen fortsetzt oder von vorn beginnt. Ein neuer Snapshot enthielte schon
die Änderungen des früheren Versuchs; ein kritischer Fehler daraus ginge ungeprüft nach
`done/`.

`TASK_STEM` ist der Dateiname ohne `.md`, `EXEC_ROOT` die Ausführungswurzel aus 4a.1.

1. Per Grep die Snapshot-Zeile dieser Datei suchen
   (`^<!-- k-task-run: snapshot <datei> [0-9a-f]+ -->$`); maßgeblich ist der letzte Treffer.
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

   Dann die Snapshot-Zeile per Shell anhängen:

   ```bash
   printf '\n%s\n' "<!-- k-task-run: snapshot <datei> <SNAPSHOT> -->" >> "<TASK_FILE>"
   ```

   `REVIEW_SCOPE` ist `ab Task-Beginn`, wenn der Task noch nie lief. Lief er schon — die
   Datei trägt einen Vermerk aus 4e, eine Etappe stand auf `erledigt`, oder eine
   Snapshot-Zeile ließ sich nicht mehr auflösen —, ist er `nur seit Fortsetzung`: Das
   Review deckt dann nur die Änderungen seit diesem Snapshot ab, sagt das in Abschnitt und
   Rückgabe, und 4f fragt vor dem Verschieben nach `done/` (4f, Schritt 5).

Schlägt ein Aufruf fehl, hält der Command vor der Delegation an und fragt den Nutzer, wie in
4a.1. Führt der Nutzer den Task ohne Snapshot aus, läuft er ohne Diff und Review, wie ohne
`DIFF_ENABLED`, und Schritt 6 nennt das.

### 4c — An den Sub-Agenten delegieren

Starte einen General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code:
`general-purpose`) für die Ausführung. Er bekommt den Pfad, nicht den Inhalt:

- den absoluten Pfad der Task-Datei — er liest sie selbst,
- das Arbeitsverzeichnis,
- bei `## Ausführungskontext`: die aufgelöste Ausführungswurzel, Base- und Work-Branch,
  PR-Pflicht, die Entscheidung zum schmutzigen Arbeitsbaum und das Ergebnis des
  Branch-Preflights als verbindlichen Rahmen,
- die Anweisung, vor Beginn alle `CLAUDE.md` im Projektbaum zu lesen,
- alle Klärungen aus 4b als zusätzlichen Kontext,
- bei `### Etappe`-Abschnitten: welche Etappen noch `offen` sind, und die Anweisung, die
  Zeile in `## Fortschritt` **unmittelbar nach Abschluss jeder Etappe** selbst zu setzen —
  vor der nächsten, nicht am Ende des Laufs,
- die Anweisung, in der Task-Datei keinen Abschnitt `## Ausführung` und keine Kennzeile
  anzulegen oder zu ändern — die schreibt der Hauptkontext —; was der Task selbst in seiner
  Datei verlangt, etwa einen eigenen Abschnitt, schreibt der Sub-Agent wie verlangt,
- das Rückgabeformat unten.

Kein Diff und keine Kennung eines Snapshots: Diff und Review sind Sache des
Review-Sub-Agenten in 4f.

Die Fortschrittszeile schreibt der Sub-Agent selbst, nicht der Hauptkontext. Nur so
überlebt der Stand einen harten Abbruch: bricht die Sitzung mitten in Etappe 4 ab, stehen
die Etappen 1 bis 3 bereits in der Datei. Ein Fortschritt, den erst der Hauptkontext nach
Rückkehr des Sub-Agenten schreibt, wäre genau in dem Fall verloren, für den er gedacht
ist.

The sub-agent must not ask the user questions - by this point all ambiguities are resolved. If the sub-agent encounters an unexpected blocker or decision it cannot cleanly resolve, it must **not** make a quick-and-dirty decision - instead it must stop and report the issue under „Blocker" so the main agent can escalate to the user.

Die Rückgabe des Sub-Agenten ist genau dieses Format, ausdrücklich ohne Diffs,
Dateiinhalte und Logs:

```
Status: erfolgreich | teilweise | blockiert
Zusammenfassung: <2–3 Sätze>
Validierung: <Befehl → Ergebnis, je eine Zeile>
Abgebrochen bei: – | <Schritt oder Etappe, an der die Arbeit stoppte>
Blocker: keine | <Beschreibung + benötigte Entscheidung>
```

- `erfolgreich` führt nach 4f, `blockiert` nach 4d, `teilweise` nach 4e. Eine Rückgabe ohne
  gültige Zeile `Status:` gilt als `blockiert`.
- Die Zeilen „Validierung" gehen in 4f an das Review und speisen den PR-Text in 4f.1.

Wait for the sub-agent to finish before proceeding to the next task.

### 4d — Unerwartete Blocker behandeln

If the sub-agent's result reports `Status: blockiert`: **stop and ask the user**, showing the „Blocker" line. Do not proceed to the next task until resolved. Then decide whether to re-run this task or to abort it (4e).

Erneut ausführen heißt: 4c noch einmal, mit der Antwort des Nutzers als weiterer Klärung.
Der Snapshot aus 4b.1 bleibt derselbe.

### 4e — Bei Fehler oder Abbruch

Gilt bei `Status: teilweise`, bei einem nach 4d abgebrochenen Task und wenn 4f eine offene
Etappe findet — immer vor dem Kopf aus 4f. Per Shell anhängen:

```bash
cat >> "<TASK_FILE>" <<'K_TASK_RUN_EOF'

## Ausführung

**Status:** Teilweise ausgeführt - abgebrochen  
**Datum:** <now.date>  
**Abgebrochen bei:** <step or action where it stopped>  
**Grund:** <brief reason>
K_TASK_RUN_EOF
```

„Abgebrochen bei" kommt aus dem gleichnamigen Feld der Rückgabe, „Grund" aus „Blocker"
oder, steht dort „keine", aus der Zusammenfassung.

- Leave the file in its current location (do not move to `done/`)
- If the task has `### Etappe` sections: leave the `## Fortschritt` table exactly as the
  sub-agent left it. It is the resume point for the next `/k-task-run` — never reset it, never
  delete it, and never mark a stage as done that the sub-agent did not mark itself. Add
  `**Etappen erledigt:** <n> von <m>` as last line of the note above; `<n>` comes from Grep
  (`^\|[^|]*\| *erledigt *\|`, nur Treffer innerhalb `## Fortschritt` wie in 4f, Schritt 1),
  nicht aus einem Lesen der Datei.
- Stop processing further tasks

Nach dem Kopf aus 4f kommt 4e nicht mehr vor: Ein Task verlässt den Lauf dann nur nach
`done/` oder mit der Zeile „angehalten" (4f, Schritt 5).

### 4f — Bei Erfolg

Bei `Status: erfolgreich`, in genau dieser Reihenfolge. So wird kein unvollständiger Task
reviewt, und innerhalb eines Laufs steht nie ein Erfolgskopf neben einem Abbruchvermerk aus
4e.

**1. Etappen prüfen.** Hat der Task `### Etappe`-Abschnitte: per Grep die Zeilen mit Status
`offen` suchen (`^\|[^|]*\| *offen *\|`). Es zählen nur Treffer nach der ersten
Zeile `^## Fortschritt` und vor der nächsten Zeile `^## ` oder `^---`. Steht eine Etappe
offen, ist der Lauf **nicht** erfolgreich: ohne Kopf und ohne Review weiter nach 4e, mit der
ersten offenen Etappe unter „Abgebrochen bei" und dem Grund, dass der Sub-Agent Erfolg meldet,
während die Etappe offen steht. Ein solcher Widerspruch wird gezeigt, nicht geglättet.

**2. Kopf und Kennzeile.** In einem Shell-Aufruf anhängen:

```bash
cat >> "<TASK_FILE>" <<'K_TASK_RUN_EOF'

## Ausführung

<!-- k-task-run: ausgeführt <datei> -->
**Status:** Erfolgreich ausgeführt  
**Datum:** <now.date>  
**Zusammenfassung:** <Zusammenfassung aus der Rückgabe>
K_TASK_RUN_EOF
```

Ab hier liest der Hauptkontext die Task-Datei nicht mehr. Mit `DIFF_ENABLED=false` oder
ohne Snapshot (4b.1) folgt direkt 4f, Schritt 6 — kein Diff, kein Review.

**3. Rezept bestimmen**, ohne es zu lesen: aus `catalogs.reviews` der Context-Ausgabe der
Eintrag mit dem Schlüssel `code` (Datei `review-code.md`). So greift ein Overlay unter
`k-playbook-local/reviews/`.

- Vorhanden, ohne `disabled: true`: `REVIEW_RECIPE = <path>` — der Sub-Agent führt Teil 1
  und Teil 2 des Moduls aus.
- `disabled: true` (leere projekteigene Datei): abgeschaltet. Der Sub-Agent führt nur Teil 1
  aus; die Diff-Abschnitte bleiben, das Review entfällt, und Schritt 6 weist
  „Review: abgeschaltet" aus. Eine Stopp-Abfrage aus dem Review gibt es dann nicht;
  meldet Teil 1 `Diff: fehlgeschlagen`, nennt Schritt 6 das dahinter.
- Fehlt der Eintrag: Der Sub-Agent führt ebenfalls nur Teil 1 aus. Das gilt als ungültiges
  Review (4f, Schritt 5).

`review.enabled` und `audit.enabled` steuern nur `/k-review` und `/k-audit` und wirken hier
nicht.

**4. Review-Sub-Agent.** Vor dem Start per Grep zählen, wie oft
`^### Review: Befunde$` und `^### Diff: Geänderte Dateien$` in der Task-Datei stehen. Dann
einen neuen General-Purpose-Sub-Agenten starten — nicht den ausführenden: Wer den Code
geschrieben hat, prüft ihn nicht selbst. Er bekommt:

- den Pfad `DIFF_REVIEW_MODULE` mit der Anweisung, das Modul zu lesen und zu befolgen,
- den Pfad der Task-Datei, die Ausführungswurzel, `SNAPSHOT` und `REVIEW_SCOPE`,
- die Zeilen „Validierung" aus der letzten Rückgabe von Ausführung oder Behebung,
- `REVIEW_RECIPE` oder die Anweisung „nur Teil 1".

Er fragt den Nutzer nicht und schreibt seine Abschnitte selbst in die Task-Datei. Seine
Rückgabe beschreibt das Modul. Danach dieselben Zeilen erneut zählen:

- Das Review ist **gültig**, wenn die Rückgabe dem Format des Moduls folgt und
  `### Review: Befunde` genau einmal mehr dasteht als vorher — nie gilt ein Lauf ohne das als
  „0 kritisch".
- Die Diff-Abschnitte hat der Lauf geschrieben, wenn `### Diff: Geänderte Dateien` genau
  einmal mehr dasteht.

**5. Stopp-Abfrage** — nach dem Review, vor dem Verschieben nach `done/`, vor 4f.1 und vor
dem nächsten Task. Sie kommt, sobald einer dieser Gründe vorliegt:

| Grund | Die Abfrage nennt | (a) |
|---|---|---|
| gültiges Review mit mindestens einem kritischen Befund | die Zeilen unter „Kritisch" aus der Rückgabe | beheben lassen |
| kein gültiges Review: Abbruch, Rückgabe nicht im Format oder kein neuer Review-Abschnitt | dass kein gültiges Review vorliegt, dazu eine Zeile `Abbruch:` | Review erneut laufen lassen |
| Rezept fehlt im Katalog | dass kein gültiges Review vorliegt, weil `review-code` im Katalog fehlt | entfällt |
| `REVIEW_SCOPE` ist `nur seit Fortsetzung` | „Review nur seit Fortsetzung" | entfällt |

Mehrere Gründe stehen in einer Abfrage; (a) steht zur Wahl, solange einer mit (a) dabei ist.
Fehlt das Rezept im Katalog, gilt dafür nur die dritte Zeile, nicht die zweite — ein erneutes
Review fände das Rezept ebenso wenig.
Für kritische Befunde:

```
<task>.md: <n> kritische Review-Befunde
  - <datei:zeile oder Prüfbefehl> — <ein Satz>

Wie weiter?
  (a) beheben lassen   - Sub-Agent behebt nur diese Befunde, danach erneutes Review
  (b) trotzdem weiter  - Task gilt als erledigt, Befunde bleiben dokumentiert
  (c) anhalten         - Task bleibt liegen, keine weiteren Tasks
```

Bei den anderen Gründen steht statt der Befundzeilen `<task>.md: kein gültiges Review —
<Grund>` bzw. `<task>.md: Review nur seit Fortsetzung — ein Fehler aus dem früheren Versuch
bleibt ungeprüft`.

- **(a) beheben lassen.** Ein General-Purpose-Sub-Agent bekommt den Pfad der Task-Datei,
  den Rahmen aus 4c (Ausführungswurzel, Branch, Preflight) und die kritischen Befunde. Er
  behebt nur diese, ändert an der Task-Datei nichts, fragt den Nutzer nicht, meldet eine
  offene Entscheidung unter „Blocker" und gibt das Format aus 4c zurück.
  - `erfolgreich`: zurück zu 4f, Schritt 4 — das Review läuft erneut gegen denselben Snapshot
    mit demselben Umfang und hängt seine Abschnitte an; die früheren bleiben. Bleibt etwas
    kritisch, wird wieder gefragt.
  - `blockiert`: fragen wie in 4d. „Erneut" heißt hier nur, den Behebungs-Sub-Agenten mit
    der Antwort des Nutzers noch einmal zu starten — nie zurück nach 4a. Wird er nicht
    erneut gestartet, weiter wie (c) mit dem Grund „Behebung blockiert, kritische
    Review-Befunde offen".
  - `teilweise`: weiter wie (c) mit dem Grund „Behebung unvollständig, kritische
    Review-Befunde offen".

  Die Vorlage aus 4e kommt dabei nicht vor.
- **(a) Review erneut laufen lassen** (kein gültiges Review): zurück zu 4f, Schritt 4 mit
  denselben Eingaben.
- **(b) trotzdem weiter.** Weiter mit 4f, Schritt 6. Liegt kein gültiges Review vor und hat der
  letzte Review-Lauf keine Diff-Abschnitte geschrieben (Zählung aus 4f, Schritt 4), startet der
  Hauptkontext davor einen Sub-Agenten nur mit Teil 1 des Moduls.
- **(c) anhalten.** Per Shell anhängen; die bestehende Statuszeile bleibt stehen:

  ```bash
  cat >> "<TASK_FILE>" <<'K_TASK_RUN_EOF'

  **Status nach Review:** angehalten — <Grund>
  K_TASK_RUN_EOF
  ```

  Grund ist „kritische Review-Befunde offen", „kein gültiges Review", „Review nur seit
  Fortsetzung" oder der Grund aus (a). Die Datei bleibt liegen, eine
  `## Fortschritt`-Tabelle unverändert; kein PR, keine weiteren Tasks. Die Kennzeile aus
  4f, Schritt 2 steht schon in der Datei; den nächsten Lauf regelt 4a.

**6. Abschluss.**

1. Ensure `done/` subdirectory exists in the same directory as the task file.
2. Move the task file into `done/` (per Shell, `mv`).
3. Snapshot freigeben — die Ref wird mit dem Verschieben nach `done/` entfernt:

   ```bash
   if command -v git >/dev/null 2>&1; then
     git -C "<EXEC_ROOT>" update-ref -d "refs/k-task-run/<TASK_STEM>"
   fi
   ```

### 4f.1 — PR-Handoff bei `PR required: true`

Erst nach der Stopp-Abfrage aus 4f. If the task's `## Ausführungskontext` has `PR required: true`, handle PR creation after the task has completed and a local commit exists.

Preflight:

1. Work in the resolved execution root from 4a.1.
2. Verify the branch is `Work branch` if one was specified.
3. Verify the worktree is clean with `git status --short`; if not clean, stop and report that a commit is required before PR creation.
4. Verify the branch has an upstream. If it has none, ask before pushing. Do not push silently.

PR body generation:

- Build the PR body as real multiline Markdown in a temporary file, not as a quoted CLI string with `\n` escapes.
- Use `/tmp/k-task-run-pr-body-<task-number-or-branch-slug>.md` unless a better existing temp path is already available.
- Include:
  - short summary of the change.
  - finding IDs or task reference if present.
  - validation commands/results: die Zeilen „Validierung" aus der letzten Rückgabe von Ausführung oder Behebung.
  - known residual risks or tests that could not run.

Create the PR with `gh pr create --body-file <file>`:

```bash
gh pr create \
  --base <Base branch> \
  --head <Work branch> \
  --title "<concise title>" \
  --body-file /tmp/k-task-run-pr-body-<slug>.md
```

Do not use `--body "...\n..."`; Bash will pass literal backslash-n in normal double quotes. If a one-off inline body is unavoidable, use a shell-safe multiline mechanism such as a here-document or ANSI-C quoting, but prefer `--body-file`.

If `gh` is unavailable or not authenticated, print the exact `gh pr create --body-file ...` command and the body-file contents for manual use.

### 4g — Weiter

Proceed to the next task in the list. If a task failed (4e) or was stopped (4f, Schritt 5, or 4a, Option (b)), stop - do not execute remaining tasks.

## Schritt 5 — Intent-Alignment-Check

If all tasks went to `done/` AND the last task file has an `## Intent` section: check whether the executed work actually achieves the stated Intent.

Schritt 5 liest keine Task-Dateien. Den Intent hat der Hauptkontext aus Schritt 1; was ausgeführt
wurde, sind die Zusammenfassungen aus den Rückgaben der Sub-Agenten in diesem Lauf. Für
einen Task, der nach 4a (a) ohne Rückgabe in diesem Lauf abgeschlossen wurde, holt der
Hauptkontext die erste Zeile `^\*\*Zusammenfassung:\*\*` nach seiner Kennzeile per Grep.

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

Append the result per Shell to the last task file, now under `done/`:

```bash
cat >> "<TASK_FILE_IN_DONE>" <<'K_TASK_RUN_EOF'

**Intent-Alignment:** <Ja / Teilweise / Nein> - <Begründung>
K_TASK_RUN_EOF
```

If alignment is **not Yes**: print a clear warning to the user before the final summary:

```
WARNUNG: Intent nicht vollständig erreicht: <Begründung>
```

If no Intent is present: skip this step silently.

## Schritt 6 — Abschließende Zusammenfassung

After all tasks are processed, print a brief summary:

```
Ausgeführt:    <n> Tasks
Erfolgreich:   <list of filenames>
  <filename>   Review: <n> kritisch, <n> wichtig, <n> Hinweis
Abgeschlossen: <filenames closed by the user in 4a, option (a)>
Angehalten:    <filename> - <Grund>
Abgebrochen:   <filename if any> - <reason>
Übersprungen:  <filenames if any>
Intent:        Ja / Teilweise / Nein / - (kein Intent)
```

Die Zeile „Review" steht je erfolgreichem Task und gibt das letzte gültige Review wieder:

- bei `REVIEW_SCOPE` `nur seit Fortsetzung` mit dem Zusatz „— nur seit Fortsetzung",
- `Review: abgeschaltet` bei abgeschaltetem Rezept,
- `Review: kein gültiges Review` nach (b) ohne gültiges Review,
- `Review: – (ohne Diff)` ohne `DIFF_ENABLED` oder ohne Snapshot.

Bei „Abgeschlossen" steht ein offener PR-Schritt dabei, wenn der Task `PR required: true`
trägt.

Folge-Command: **`/k-danke`** — schließt die Sitzung ab: legt die Befunde vor, die während
der Ausführung festgehalten wurden, und prüft den Docs-Nachzug.
