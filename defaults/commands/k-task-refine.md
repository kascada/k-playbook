# k-task-refine

Härtet Tasks und Instruction-Dateien vor der Ausführung über einen strukturierten
Zwei-Agenten-Dialog zwischen **Critic** und **Editor**. Critic und Editor sind read-only
Advisors. Der Moderator routet zwischen ihnen, entscheidet Deadlocks, wendet akzeptierte
Edits an und hängt ein Review-Log an. Ein finaler Alignment-Check prüft das Ergebnis gegen
den angegebenen Intent.

- `/k-task-refine` — die eigenen offenen Tasks, die noch kein Review-Log tragen. Der
  Hauptkontext teilt sie in Gruppen zusammenhängender Tasks; jede Gruppe prüft ein eigener
  Sub-Agent, alle parallel. Zum Schluss sucht ein Critic Überschneidungen zwischen den
  Gruppen und mit den schon geprüften offenen Tasks.
- `/k-task-refine <ziel>` — ein `keph:`-Verweis auf einen Task, eine Tasknummer oder eine
  Instruction-Datei bzw. ein Verzeichnis mit `.md`-Dateien auf der Platte. Trägt das Ziel
  schon ein Review-Log, fragt der Command vorher nach.
- `/k-task-refine <ziel> nochmal` — dasselbe ohne diese Rückfrage. Statt `nochmal` gelten
  auch `erneut` und `again`.
- `/k-task-refine nochmal` — das Wort allein, ohne Ziel: alle eigenen offenen Tasks, auch die
  schon geprüften, in Gruppen wie ohne Argument.

Produces:

- Ein `## Review-Log (<context.now.date>)` am Ende jedes geprüften Tasks, angehängt mit
  `k_playbook_doc_append`, bzw. jeder geprüften Datei — auch ohne Änderung, dann mit dem
  Vermerk „keine Änderungen“.
- Änderungen an den geprüften Tasks und Dateien, angewendet allein vom Moderator: an einem
  Task mit `k_playbook_doc_replace`, an einer Plattendatei mit dem Edit-Werkzeug.
- Eine Zusammenfassung über alle Ziele am Ende, im Lauf mit Gruppen samt den gefundenen
  Überschneidungen zwischen Tasks. Sie wird nur ausgegeben, nicht abgelegt.

Process invariants:

- **Subagents are read-only.** Critic and Editor must not modify documents or files, run
  formatters, or call write/edit tools. They only return findings, reasoning, and proposed
  edits.
- **The Moderator is the only writer.** Apply changes only after checking that they address
  routed issues and do not introduce unrelated edits. Moderator ist im Lauf mit Gruppen der
  Sub-Agent der Gruppe für genau ihre Tasks (Schritt 5), sonst der Hauptkontext.
- **Actual state wins.** After every Moderator-applied edit, reread the modified task or
  file and use that content for follow-up Critic rounds and the final alignment check.
- **Keep an audit trail.** Record all Critic issues, Moderator routing decisions, Editor
  decisions, skipped items, deadlocks, and final alignment in the Review-Log. Every reviewed
  task gets a log, including those that needed no change — the log is the only evidence that
  a review happened at all.
- **Use the fast path when possible.** If one Critic round yields only clear,
  non-controversial fixes and the Editor proposal is clean, apply once, reread, run the final
  alignment check, and stop.

## Schritt 1 — Briefing und Form

include: _shared/erster-schritt.md

Verbindlich für Ablage, Nummernauflösung, Kennzeilen und den Schreibweg eines Tasks ist
`k-playbook:/instructions/on-demand/tasks.md`. Lies sie mit `k_playbook_doc_read`, bevor du
einen Task liest oder änderst; dieser Text wiederholt sie nicht. Der Moderator hält die
Revisionskette, wie dort unter „Lesen und schreiben“ beschrieben: Jedes Anhängen und Ändern
trägt `since_revision`, ein `foreign_change` hält den Command an.

Ist das Ziel ein Task oder leer, braucht der Command die Collection: Steht
`context.kephalaion.ok` nicht auf `true`, endet er mit `context.kephalaion.message`
(Abschnitt „Ohne Kephalaion“ dort). Eine Instruction-Datei auf der Platte lässt sich auch
ohne Collection prüfen.

## Schritt 2 — Ziel auflösen

**Wort für die erneute Prüfung.** Ist das letzte Wort des Arguments `nochmal`, `erneut` oder
`again` (Groß- und Kleinschreibung gleich), nimm es ab und merke `ERNEUT = ja`; der Rest ist
das Ziel. Sonst `ERNEUT = nein`.

**Geprüft** heißt allein: Eine Zeile, die mit `## Review-Log` beginnt, steht im Dokument.
Gezählt wird bei einem Task mit `k_playbook_doc_grep` (`ref`, Muster `^## Review-Log`,
`count_only`) — dasselbe Muster wie `/k-task-run` in Schritt 3 —, bei einer Datei auf der Platte
mit dem Suchwerkzeug. Ob der Task danach geändert oder ausgeführt wurde, spielt keine Rolle.
Die `revision` aus der Zählung eines Tasks merkst du dir als `REVISION` dieses Tasks.

Das Ziel entscheidet:

- **Leer:** die eigenen offenen Tasks aus `k_playbook_doc_list` (Sorte `tasks`, ohne `done`),
  sortiert nach der Nummer am Anfang des Dateinamens. Keine → „Keine offenen Tasks gefunden“
  und Schluss. Sonst zählst du je Task wie oben und nimmst als Ziele nur die ungeprüften
  (Anzahl 0). Mit `ERNEUT = ja` nimmst du alle. Bleibt keiner übrig, gib aus:

  ```
  Alle <n> offenen Tasks sind schon geprüft.
  Erneut prüfen: /k-task-refine <nummer> nochmal
  ```

  und Schluss; geschrieben wird nichts. Die schon geprüften offenen Tasks, die nicht Ziel
  sind, merkst du dir als `UEBRIGE_TASKS` für Schritt 15. Der Lauf ohne Ziel ist der **Lauf
  mit Gruppen** (Schritte 3 bis 5 und 15).
- **Ein `keph:`-Verweis** (`keph:/user/…/tasks/<datei>`): genau dieser Task — nach den
  Prüfungen aus `k-playbook:/instructions/on-demand/tasks.md`, „Fremde und erledigte
  Verweise“, bevor etwas geschrieben wird. Ein fremder Verweis (User-Pfadteil weder `~` noch
  `kephalaion.user`) wird abgewiesen, mit `foreign_user` in der Meldung; Schluss. Ein Verweis
  unter `done/` führt zur Rückfrage unten.
- **Eine Nummer:** aufgelöst nach `k-playbook:/instructions/on-demand/tasks.md`, „Wohin und
  Name“ — zuerst unter den eigenen offenen, dann unter den eigenen erledigten Tasks; bei
  mehreren Treffern alle zeigen und den Nutzer wählen lassen.
- **Sonst ein Pfad auf der Platte:** eine Datei oder ein Verzeichnis mit `.md`-Dateien, relativ
  zu `context.project.dir`. Gibt es ihn nicht, brich mit einer klaren Meldung ab. Sag dazu,
  dass dies ein ausdrückliches Einzelziel ist und nicht die Task-Warteschlange. Bei einem
  Verzeichnis: alle `.md` direkt darin, ohne `done/`, `old/` und Archive, sortiert nach der
  führenden Nummer (auch mit `_`), Dateien ohne Nummer übersprungen. Keine → „Keine
  Dateien gefunden“ und Schluss. Der Grund: Ein Verzeichnis als Ziel ist eine Folge
  nummerierter Anweisungen, die in dieser Reihenfolge gelten — so lagen Tasks einst auf der
  Platte, und so liegen Instruction-Dateien heute. `done/`, `old/` und Archive sind
  Erledigtes, eine Datei ohne Nummer (etwa ein `README.md`) gehört nicht zur Folge; wer sie
  prüfen will, nennt sie als Einzelziel. Ein Verzeichnis wird gemeinsam im Hauptkontext
  geprüft, nicht in Gruppen: Es ist eine Folge.

Ein ausdrücklich genanntes Ziel — Verweis, Nummer, Datei, Verzeichnis — zählst du wie oben,
dann gelten diese Rückfragen, jede mit der Fertig-Meldung vorher:

- **Schon geprüft** — das Ziel trägt ein Review-Log und `ERNEUT = nein`: Fertig-Meldung
  (`Rückfrage: schon geprüft`), dann fragen: erneut prüfen oder abbrechen. Die Frage nennt die
  Daten der vorhandenen Review-Logs, gelesen aus den Zeilen `^## Review-Log` (bei einem Task
  `k_playbook_doc_grep` ohne `count_only`). Bei einem Verzeichnis ist es eine Frage, die die
  geprüften Dateien mit ihren Daten nennt. Mit `ERNEUT = ja` entfällt die Frage.
- **Erledigt** — ein Treffer oder Verweis unter `done/`: Der Task ist schon abgeschlossen.
  Fertig-Meldung (`Rückfrage: erledigter Task`), dann fragen: trotzdem prüfen — das
  Review-Log kommt an das Dokument unter `done/`, der Task bleibt dort — oder abbrechen.
  `/k-task-refine` öffnet keinen Task wieder; das tut `/k-task-run`. Ist der erledigte Task
  auch schon geprüft, bleibt es bei dieser einen Rückfrage; sie nennt dann die Daten der
  Review-Logs mit.

Merke die Ziele als `REVIEW_TARGETS`: je Eintrag der `keph:`-Verweis oder der absolute Pfad,
dazu die Anzeigeform (der Verweis bzw. der projektrelative Pfad), ob es ein Task in der
Collection oder eine Datei ist, und bei einem Task `REVISION`. Mit einem Ziel geht es weiter
mit Schritt 4, ohne Gruppen; der Hauptkontext ist Moderator.

## Fertig-Meldung

Vor jeder Rückfrage an den Nutzer und einmal am Ende meldet sich der Hauptkontext auf dem
Rechner des Nutzers. Die Stellen in diesem Command: die Rückfragen in Schritt 2 (mehrere
Treffer, schon geprüft, erledigter Task), jede Rückfrage einer Gruppe und jedes
`Foreign-Change:` eines Sub-Agenten in Schritt 5, jedes ask-user in Schritt 8, die Rückfrage
in Schritt 10, die Rückfrage nach einem nicht erreichten Intent in Schritt 13, die Rückfrage
nach einem `foreign_change` und das Ende in Schritt 16. Ein Sub-Agent einer Gruppe sendet
keine Fertig-Meldung — weder vor einer Rückfrage noch am Ende —; er gibt die Frage zurück.

include: _task/fertig-meldung.md

## Schritt 3 — Gruppen bilden (nur im Lauf mit Gruppen)

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

## Schritt 4 — Startübersicht ausgeben

Vor allem anderen an den Nutzer:

```
Review gestartet
─────────────────────────────
Tasks:   <Liste der Ziele in Anzeigeform>
Gruppen: <je Gruppe eine Zeile, siehe unten>
Ablauf:  <je Gruppe ein Sub-Agent, parallel | Hauptkontext, Gruppen nacheinander | Hauptkontext>
Projekt: <context.project.dir>
Runden:  max. 5
```

Eine Gruppenzeile nennt die Nummern, bei mehr als einem Task mit dem Grund, etwa
`gemeinsam: <nr> + <nr> — beide ändern project:/defaults/commands/k-task-refine.md`, sonst
`allein: <nr>`. Ohne Gruppen (Lauf mit Ziel) entfallen die Zeilen `Gruppen:`, und `Ablauf:`
heißt `Hauptkontext`. Den Intent liest erst Schritt 6; das Review-Log nennt ihn je Ziel.

## Schritt 5 — Gruppen prüfen (nur im Lauf mit Gruppen)

Ohne Gruppen weiter mit Schritt 6; der Hauptkontext ist Moderator für alle `REVIEW_TARGETS`.

**Rückfall je Client.** Den Lauf je Gruppe in einem Sub-Agenten gibt es nur, wo ein
Sub-Agent selbst Sub-Agenten starten kann, denn er startet Critic und Editor:

- **Claude Code:** Ein `general-purpose`-Sub-Agent hat das Werkzeug `Agent`, und der
  Sub-Agent, den er startet, ruft `mcp__k-playbook__*` auf (gemessen 2026-11-20). Der Lauf je
  Gruppe gilt.
- **OpenCode:** Ein Sub-Agent `general` kann keinen Sub-Agenten starten — ihm fehlt das
  Werkzeug `subagent` (gemessen 2026-11-20, v2.3.0, Grundeinstellung). Hier gilt der
  Rückfall: Der Hauptkontext bildet dieselben Gruppen (Schritt 3), führt den Dialog
  (Schritte 6 bis 14) von Anfang an selbst, je Gruppe nacheinander, mit den Tasks der Gruppe
  als `REVIEW_TARGETS`, und startet keinen Sub-Agenten je Gruppe. Critic und Editor startet
  er wie sonst. Danach folgt Schritt 15 wie sonst.

Ein anderer Client folgt dem Lauf je Gruppe; gibt ein Sub-Agent `keine-subagenten` zurück,
prüft der Hauptkontext diese Gruppe wie im Rückfall selbst.

**Übergabe.** Vor der Übergabe hält der Hauptkontext die Revisionskette nach
`k-playbook:/instructions/on-demand/tasks.md`, „Sub-Agenten“: je Task `^## Review-Log`
gezählt (`count_only`) und Anzahl und Revision gemerkt — die Zählung aus Schritt 2 genügt, wenn
seitdem niemand in den Task geschrieben hat. Dann startet er je Gruppe einen
General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code: `general-purpose`), alle in
einer Nachricht. Der Auftrag

- beginnt mit: Führe zuerst `k-playbook briefing` aus und lies die Ausgabe,
- nennt den Command-Text als Verweis, nicht als Kopie: `k-playbook:/commands/k-task-refine.md`,
  gelesen mit `k_playbook_doc_read`; der Sub-Agent ist darin der Moderator und führt die
  Schritte 6 bis 14 für genau die Tasks seiner Gruppe aus — bei mehreren gemeinsam, als
  `REVIEW_TARGETS` —, startet Critic und Editor selbst, wendet Edits an und hängt an jeden Task
  der Gruppe das Review-Log an,
- nennt je Task der Gruppe den `keph:`-Verweis und die Revision sowie `projectDir`
  (`context.project.dir`); in Claude Code sind die Werkzeuge `mcp__k-playbook__*` im
  Sub-Agenten oft verzögert und werden vor dem ersten Aufruf mit der Werkzeugsuche geladen,
- nennt die Regeln zu `foreign_change`, `outcome_unknown` und dem Replica-Nachlauf aus
  `k-playbook:/instructions/on-demand/tasks.md` und die Regeln unten,
- nennt das Rückgabeformat unten,
- bei einer Fortsetzung zusätzlich: den `Zwischenstand:` des vorigen Sub-Agenten, die
  Revisionen aus seinen Zeilen `Revision:` bzw. `Foreign-Change:` und die Antwort des Nutzers
  als Entscheidung.

**Regeln für den Sub-Agenten einer Gruppe:**

- Er fragt den Nutzer nicht und sendet keine Fertig-Meldung, auch nicht am Ende von
  Schritt 14. Wo der Dialog eine Rückfrage vorsieht (ask-user in Schritt 8, unklare Änderung in
  Schritt 10, nicht erreichter Intent in Schritt 13), hält er an, hängt an keinen Task der
  Gruppe ein Review-Log an und gibt die Frage mit seinem Zwischenstand zurück.
- Die Gruppe ist eine Einheit: Braucht er eine Entscheidung oder meldet ein Task
  `foreign_change`, hält er für die ganze Gruppe an und hängt an keinen ihrer Tasks ein
  Review-Log an. Kommt `foreign_change` erst beim Anhängen in Schritt 14, tragen die Tasks, an
  die schon angehängt ist, ihr Review-Log; an die übrigen hängt er nichts mehr an.
- Kann er keine Sub-Agenten starten, gibt er sofort `Ergebnis: keine-subagenten` je Task
  zurück und schreibt nichts.
- Bei einer Fortsetzung setzt er an der Stelle des Zwischenstands fort, mit der Entscheidung
  aus dem Auftrag, zählt die Runden weiter (höchstens 5 insgesamt) und hängt je Task ein
  einziges, vollständiges Review-Log an, das auch den Zwischenstand enthält.

**Rückgabe**, genau dieses Format, ohne Task-Inhalte und Diffs — je Task der Gruppe ein Block:

```
Ziel: <ref>
Ergebnis: geprüft | rückfrage | abgebrochen | keine-subagenten
Runden: <n>
Gefixt: <n>
Offen: <n>
Revision: <n>
```

Statt `Revision: <n>` steht nach einem `foreign_change` an diesem Task
`Foreign-Change: <revision> <updatedBy> <updatedAt>`. `Revision` ist die Revision des letzten
eigenen Schreibens an diesem Task, ohne Schreiben die übergebene. `geprüft` heißt: Das
Review-Log hängt am Task. `rückfrage` heißt: wartet auf die Antwort des Nutzers.
`abgebrochen` heißt: ohne Review-Log angehalten, ohne eigene Frage — wegen eines
`foreign_change` in der Gruppe oder weil die Collection nicht erreichbar war.

Nach den Blöcken folgt einmal für die Gruppe, bei `rückfrage`, die Zeile
`Rückfrage: <die Frage in einem Satz>`. Bei `rückfrage` und bei `Foreign-Change:` folgt zuletzt
ein Block `Zwischenstand:` mit den Abschnitten des Review-Logs bis dahin — Critic-Issues,
Routing, Editor-Entscheidungen, die schon angewandten Edits — und der laufenden Runde.

**Auswerten.** Der Hauptkontext sammelt die Rückgaben aller Gruppen und arbeitet sie danach
Gruppe für Gruppe ab:

- `geprüft` — fertig; die Zahlen gehen in die Zusammenfassung (Schritt 16).
- `rückfrage` — Fertig-Meldung (`Rückfrage: <worum es geht>`), dann die Frage der Gruppe
  einzeln an den Nutzer. Mit seiner Antwort startet er für diese Gruppe einen neuen
  Sub-Agenten, für ihre Tasks, die noch kein Review-Log tragen, mit Zwischenstand, Revisionen
  und der Antwort als Entscheidung (Übergabe oben).
- `Foreign-Change:` — Fertig-Meldung (`Rückfrage: fremder Schreiber`), dann fragen, wer
  weitermacht, mit `revision`, `updatedBy` und `updatedAt` aus der Zeile. Sagt der Nutzer
  „weiter“, startet er einen neuen Sub-Agenten für die Tasks der Gruppe ohne Review-Log, mit
  der Revision aus der Zeile `Foreign-Change:` als nächstem `since_revision` und dem
  Zwischenstand. Sonst bleibt die Gruppe ohne Review-Log.
- `keine-subagenten` — der Hauptkontext prüft die Gruppe selbst, Schritte 6 bis 14, wie im
  Rückfall.
- `abgebrochen` ohne `Foreign-Change:` — keine Fortsetzung; der Grund steht in der
  Zusammenfassung.
- Fehlen bei einem Task beide Zeilen `Revision:` und `Foreign-Change:`, zählt der Hauptkontext
  `^## Review-Log` erneut, nach `k-playbook:/instructions/on-demand/tasks.md`, „Sub-Agenten“,
  Punkt 3.

Sind alle Gruppen fertig — auch nach Rückfragen und Fortsetzungen —, weiter mit Schritt 15.

## Schritt 6 — Inhalte lesen

Für jedes Ziel: einen Task mit `k_playbook_doc_read` (`ref`), eine Datei mit dem
Lese-Werkzeug. Setze `CURRENT_FILE_STATE` auf den tatsächlichen Inhalt. Bei einem Task ist die
`revision` aus der Antwort das erste `since_revision`; weicht sie von `REVISION` aus Schritt 2
bzw. aus dem Auftrag ab, gilt der Replica-Nachlauf aus
`k-playbook:/instructions/on-demand/tasks.md` und danach `foreign_change`. Sonst bleibt sie
`REVISION`.

Prüfe je Ziel auf einen Abschnitt `## Intent`. Der Intent steht auf zwei Arten, auch
zusammen:

**Option A — Inline-Text** (direkt im Task):

```markdown
## Intent
Der Entwickler soll nach Ausführung dieses Tasks die TTS-Anbindung
ohne Rückfragen implementieren können.
```

**Option B — Verweise** im Intent: Was dafür gilt, steht in
`k-playbook:/instructions/on-demand/tasks.md`, „Intent“; dieser Text erklärt es nicht neu.
Ein Beispiel:

```markdown
## Intent
Die TTS-Anbindung erfüllt die Anforderungen aus `project:/docs/requirements.md`.
- Es gilt der Rahmen aus `keph:/docs/manual/tts.md`.
```

Die Verweise liest du auf den Lesewegen dort mit und nimmst ihren Inhalt zum Intent; stehen
Text und Verweise, gelten beide. Ohne `## Intent` entfällt der Alignment-Check (Schritt 13).

---

## Schritt 7 — Critic-Runde

Spawn a general-purpose subagent (OpenCode: `general`, Claude Code: `general-purpose`)
as Critic with this prompt:

```
You are the Critic in a structured review of task/instruction files before they are executed by an AI agent.
You are read-only. Do not modify documents or files or call write/edit tools.
Review at a HIGH LEVEL — does the overall approach make sense? Do NOT check implementation details.
The executing agent will fill in details; focus on the big picture.

<if Intent is present>
## Intent (the goal these files must achieve)
<insert intent text>
</if>

## Files to review
<one line per target: the keph: reference or the absolute path>
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <context.project.dir>), a path with your file reading tool.

Check for:
- FEHLER   — fundamentally wrong approach, contradiction, or missing critical piece
- WARNUNG  — potentially problematic or unclear framing
- FEHLEND  — important constraint or context not specified

References: Read k-playbook:/instructions/on-demand/verweise-nachziehen.md with k_playbook_doc_read and check every reference in the targets against it. A relative link or a bare path where a reference is meant is an issue like any other (WARNUNG); recommend the form with prefix. If the prefix is ambiguous by that file, say so instead of guessing. Leave evidence untouched as that file says (Review-Log, ## Ausführung and what follows it).

Output format (table, no intro text):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

**Review focus.** Check from above:

- Is the overall approach coherent?
- Are there contradictions between steps or files?
- Is a critical constraint missing that the executing agent cannot infer?
- Is the scope/framing reasonable?
- (From round 2+) Did the Editor's changes introduce new problems?

Do NOT flag: implementation choices, missing code details, style, minor wording. A reference
without prefix is no wording question; it is flagged (see the prompt).

## Schritt 8 — Moderator: an den Editor routen

For each issue from the Critic, decide:

- **pass** — clear mistake or blocking missing constraint; send to Editor.
- **skip** — non-blocking WARNUNG/FEHLEND; record but do not send.
- **decide** — design decision the Moderator can resolve; record the decision.
- **ask-user** — requires product/process input; pause loop, ask user (Fertig-Meldung
  vorher), then continue. A reference whose prefix is ambiguous by
  `k-playbook:/instructions/on-demand/verweise-nachziehen.md` is always ask-user.

Skip WARNUNGs and FEHLENDs unless they block execution. Store the routing table for the
final Review-Log:

| ID | Kategorie | Route | Begründung | Weitergabe an Editor? |

## Schritt 9 — Editor-Runde

Spawn a general-purpose subagent (OpenCode: `general`, Claude Code: `general-purpose`)
as Editor with this prompt:

```
You are the Editor in a structured review loop. The Critic has identified issues in these task/instruction files.
You are read-only. Do not modify documents or files, run formatters, or call write/edit tools.
For each issue: either propose a concrete fix OR explain clearly why you disagree or see it differently.
Do not fix silently — for every issue, state what you did (fixed / not fixed + reason).

## Issues to address
<insert all issues routed as pass, with IDs and Moderator decisions>

For an issue about a reference, propose the form with prefix by k-playbook:/instructions/on-demand/verweise-nachziehen.md (read it with k_playbook_doc_read). Where the prefix is ambiguous by that file, do not guess; say so.

## Files
<one line per target: the keph: reference or the absolute path>
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <context.project.dir>), a path with your file reading tool.

Output:
1. Per changed target, the edits as pairs — ALT: the exact text to replace, long enough to occur exactly once in the document; NEU: its replacement. Insertions name the unique line before which or after which the text goes. Do not write to disk or to the collection.
2. A table of your decisions:
| ID | Aktion | Begründung |
```

## Schritt 10 — Moderator: Vorschläge prüfen und anwenden

The Moderator reviews the Editor output before changing anything:

- Confirm each proposed edit maps to a routed issue or a required consistency change.
- Reject unrelated rewrites, style-only churn, broad restructuring, or edits outside
  `REVIEW_TARGETS` unless explicitly justified and approved.
- If the proposal is clean, apply the minimal change:
  - an einem Task mit `k_playbook_doc_replace` (`ref`, `old` = ALT, `new` = NEU,
    `since_revision` = `REVISION` dieses Tasks); die Antwort nennt die neue Revision, die ab
    jetzt `REVISION` ist. `text_not_unique` heißt: ALT mit mehr Umgebung erneut; `text_not_found`
    heißt: gegen `CURRENT_FILE_STATE` prüfen, ob die Stelle inzwischen anders heißt.
    `outcome_unknown` heißt: nicht blind wiederholen, sondern nachlesen nach
    `k-playbook:/instructions/on-demand/tasks.md`, „`outcome_unknown` je Operation“ — steht
    NEU jetzt im Task, gilt die Revision aus dem Nachlesen als `REVISION`; sonst der feste Weg
    dort.
  - an einer Plattendatei mit dem Edit-Werkzeug des Assistenten.
- If the proposal is partially useful, apply only the minimal accepted subset and record the
  rejection reason.
- If the proposal cannot be applied cleanly, resolve the conflict minimally yourself or ask
  the user if the intended edit is ambiguous — vorher die Fertig-Meldung (`Rückfrage: unklare
  Änderung`).
- Immediately reread every modified target (Task: `k_playbook_doc_read`, Datei: Lese-Werkzeug)
  and set `CURRENT_FILE_STATE` to the actual content.

Do not trust proposed edits as applied state until the reread confirms it.

**`foreign_change`** bei einem `k_playbook_doc_replace`: Seit deinem letzten Schreiben hat
jemand anderes in den Task geschrieben, nichts wurde geschrieben. Halte an, ändere nichts
weiter an diesem Task, sende die Fertig-Meldung (`Rückfrage: fremder Schreiber`) und frage den
Nutzer mit `revision`, `updatedBy` und `updatedAt` aus der Antwort, wer weitermacht. Sagt er
„weiter“, gilt die Revision aus der Antwort als `REVISION`, du liest den Task neu und wendest
die Edits erneut gegen den neuen Stand an; sonst endet der Command für diesen Task ohne
Review-Log. Der Sub-Agent einer Gruppe fragt nicht, sondern hält an und gibt
`Foreign-Change:` zurück (Schritt 5).

## Schritt 11 — Moderator: Offenes zurück an den Critic

Collect all issues the Editor did NOT fix, all Moderator-rejected edits, and any concerns
noticed while rereading the actual state. For each, pass the Editor's reasoning and the
Moderator decision to the Critic.

If nothing is unresolved and no risky new change exists → use the fast path and skip to
Schritt 13 (Intent check, if Intent exists).

## Schritt 12 — Critic antwortet (bis zu 5 Runden insgesamt)

Spawn a new Critic subagent with this prompt:

```
You are the Critic in round <N> of a structured review loop.
You are read-only. Do not modify documents or files or call write/edit tools.
Below are issues you raised that the Editor did not fix, along with the Editor's reasoning.
Also review the current state of the files for any NEW problems introduced by the Moderator-applied changes.

Respond to each unresolved item: do you accept the Editor's reasoning, clarify your point, or insist?
Then list any genuinely new problems caused by the Editor's changes.

## Unresolved items (with Editor's reasoning)
<insert items + reasoning>

## Current state
<one line per target: the keph: reference or the absolute path>
Read each target yourself, now, after the Moderator applied the edits: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <context.project.dir>), a path with your file reading tool.

Output:
1. Response table:
| ID | Antwort | Akzeptiert? (ja/nein) | Klarstellung / Neues Argument |
2. New issues table (if any):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

**Moderator decides:**

- If Critic accepts Editor's reasoning → issue closed
- If Critic insists or adds new issues → pass to Editor (Schritt 8), new round
- If both sides have argued the same point twice without movement → Moderator decides and
  notes it as a Moderator-Entscheidung
- If all issues resolved or max 5 rounds reached → proceed to Schritt 13
- If continuous improvement is happening → allow up to 5 rounds; stop early as soon as
  agreement is reached

---

## Schritt 13 — Intent-Alignment-Check (nur mit Intent)

Spawn a final general-purpose Critic subagent (OpenCode: `general`, Claude Code:
`general-purpose`) with this prompt:

```
You are doing a final alignment check. Below is the Intent (the goal the task must achieve) and the targets after review.
You are read-only. Do not modify documents or files or call write/edit tools.

## Intent
<insert intent text>

## Current task files
<one line per target: the keph: reference or the absolute path>
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <context.project.dir>), a path with your file reading tool.

Answer only: Can the task, as written, achieve the stated Intent?
- Yes → state briefly why
- No → list what is missing or misaligned

Output (no intro text):
| Alignment | Begründung |
```

If alignment is NO → Moderator decides: one more targeted Editor round or ask user.

---

## Schritt 14 — Review-Log anhängen

Hänge den folgenden Block an **jedes geprüfte Ziel** an — nicht nur an die geänderten: an
einen Task mit `k_playbook_doc_append` (`ref`, `content` = der Block, `since_revision` =
`REVISION`; die Antwort nennt die Revision, mit der der Task jetzt steht), an eine
Plattendatei per Shell mit einem Heredoc, dessen Endmarke in Anführungszeichen steht.

Ein Ziel, an dem nichts zu korrigieren war, ist trotzdem geprüft worden, und diese Prüfung
muss sichtbar bleiben. `/k-task-run` erkennt an genau diesem Block (Kennzeile
`## Review-Log (<datum>)`, siehe `k-playbook:/instructions/on-demand/tasks.md`), ob ein Task
jemals gegengelesen wurde; fehlt er, meldet `/k-task-run` den Task später als ungeprüft und
fragt unnötig nach.

Ergab die Prüfung keine Änderung, wird derselbe Block angehängt: Tabellen ohne Zeilen und
unter `### Geänderte Dateien` der Vermerk `— keine Änderungen`. `<datum>` ist
`context.now.date`.

```markdown
---
## Review-Log (<datum>)

**Ziel:** <Anzeigeform des Ziels>
**Intent:** <Intent-Quellen oder "—">
**Runden:** <N>

### Diskussion
<For each issue that was debated (not trivially fixed): 2-3 sentences capturing the core argument from each side and the resolution>

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |

### Editor-Entscheidungen
| ID | Aktion | Begründung |

### Moderator-Entscheidungen
<List deadlocks, skipped WARNUNG/FEHLEND items, rejected Editor edits, and user-input decisions, each with brief reasoning>

### Intent-Alignment
<Result of Schritt 13, or "— (kein Intent angegeben)">

### Geänderte Dateien
- <Ziel>: <what changed> (FEHLER-01, FEHLER-03)
- oder: — keine Änderungen

### Offen (nicht gefixt)
- <ID>: <reason>
```

Vor dem Anhängen an einen Task zählt der Moderator mit `k_playbook_doc_grep` und `count_only`
die Zeile `^## Review-Log \(<datum>\)$` und merkt sich die Anzahl
(`k-playbook:/instructions/on-demand/tasks.md`, „Zählen vor dem Anhängen“); die Revision aus
dieser Antwort muss `REVISION` sein, sonst gilt der Replica-Nachlauf dort.

Ein `foreign_change` beim Anhängen wird behandelt wie in Schritt 10: anhalten,
Fertig-Meldung, Rückfrage; nach „weiter“ mit der Revision aus der Antwort erneut anhängen.
Nach `outcome_unknown` nicht noch einmal anhängen, sondern dieselbe Zeile erneut zählen: Ist
die Zahl gestiegen, ist der Block angekommen; sonst gilt der feste Weg aus „`outcome_unknown`
je Operation“ dort.

Hier endet der Dialog. Der Sub-Agent einer Gruppe gibt jetzt seine Rückgabe (Schritt 5)
zurück und sendet keine Fertig-Meldung. Prüft der Hauptkontext die Gruppen selbst (Rückfall),
macht er mit der nächsten Gruppe weiter, nach der letzten mit Schritt 15. Mit einem Ziel geht
es weiter mit Schritt 16.

## Schritt 15 — Überschneidungen prüfen (nur im Lauf mit Gruppen)

Ein Critic je Gruppe sieht die anderen Gruppen nicht, und die schon geprüften offenen Tasks
sieht ohne diesen Schritt kein Critic. Das Urteil über den Zusammenhang in Schritt 3 stützt
sich nur auf drei Abschnitte je Task, und ein gemeinsamer Vertrag in verschiedenen Dateien
verbindet zwei Tasks dort gar nicht. Deshalb läuft hier einmal ein schlanker Critic über die
Ziele des Laufs und `UEBRIGE_TASKS`.

Er läuft einmal, wenn alle Gruppen fertig sind, auch nach Rückfragen und Fortsetzungen, und
liest die Tasks im Stand danach — sobald es zu einer Gruppe etwas zum Vergleichen gibt: eine
weitere Gruppe des Laufs oder einen Task in `UEBRIGE_TASKS`. Er entfällt nur bei genau einer
Gruppe und leerem `UEBRIGE_TASKS`, weil ihr Critic dann schon alles gemeinsam gesehen hat.
Im Rückfall gilt derselbe Schritt. Der Hauptkontext startet ihn selbst als
General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code: `general-purpose`), auf der
ersten Ebene, die jeder Client kann. Der Critic schreibt nichts, hält deshalb keine
Revisionskette und sendet keine Fertig-Meldung.

```
Führe zuerst `k-playbook briefing` aus und lies die Ausgabe.
You are the overlap Critic of a /k-task-refine run (command text: k-playbook:/commands/k-task-refine.md, Schritt 15; read it with k_playbook_doc_read).
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

Die Treffer gehen unverändert in die Zusammenfassung. Der Hauptkontext ändert daraufhin nichts,
hängt kein weiteres Review-Log an und fragt nicht nach: Die Treffer ändern kein Review-Log, und
die Reihenfolge entscheidet der Nutzer beim Ausführen.

## Schritt 16 — Zusammenfassung

Erst die Fertig-Meldung (`fertig`; der Body nennt die Zahl der Tasks, gefixte und offene
Issues und, im Lauf mit Gruppen, die Zahl der Überschneidungen). Dann an den Nutzer, einmal
über alle Ziele:

```
Review abgeschlossen
─────────────────────────────
<je Ziel eine Zeile: Anzeigeform — Ergebnis — Runden <n>, gefixt <n>, offen <n>>
<im Lauf mit Gruppen: die Zeilen aus Schritt 15 unverändert, oder "Überschneidungen: keine"
 bzw. "Überschneidungen: nicht geprüft (eine Gruppe, keine anderen offenen Tasks)">
```

Das Ergebnis je Ziel ist `geprüft`, `abgebrochen` mit Grund oder `ohne Review-Log` (der Nutzer
hat abgebrochen oder nach einem fremden Schreiber nicht weitergemacht).

Folge-Command: **`/k-task-run`** — führt die gehärteten Tasks aus und legt in jedem das
Ergebnis der Ausführung ab; mit demselben Verweis oder derselben Nummer genau diesen. Nennt die
Zusammenfassung eine Überschneidung mit Reihenfolge, führe die Tasks in dieser Reihenfolge aus.
