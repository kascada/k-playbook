# k-task-refine

Härtet Tasks und Instruction-Dateien vor der Ausführung über einen strukturierten
Zwei-Agenten-Dialog zwischen **Critic** und **Editor**. Critic und Editor sind read-only
Advisors. Der Moderator routet zwischen ihnen, entscheidet Deadlocks, wendet akzeptierte
Edits an und hängt ein Review-Log an. Ein finaler Alignment-Check prüft das Ergebnis gegen
den angegebenen Intent.

- `/k-task-refine` — die eigenen offenen Tasks aus der Projekt-Collection härten.
- `/k-task-refine <ziel>` — ein `keph:`-Verweis auf einen Task, eine Tasknummer oder eine
  Instruction-Datei bzw. ein Verzeichnis mit `.md`-Dateien auf der Platte.

Produces:

- Ein `## Review-Log (<context.now.date>)` am Ende jedes geprüften Tasks, angehängt mit
  `k_playbook_doc_append`, bzw. jeder geprüften Datei — auch ohne Änderung, dann mit dem
  Vermerk „keine Änderungen“.
- Änderungen an den geprüften Tasks und Dateien, angewendet allein vom Moderator: an einem
  Task mit `k_playbook_doc_replace`, an einer Plattendatei mit dem Edit-Werkzeug.

Process invariants:

- **Subagents are read-only.** Critic and Editor must not modify documents or files, run
  formatters, or call write/edit tools. They only return findings, reasoning, and proposed
  edits.
- **The Moderator is the only writer.** Apply changes only after checking that they address
  routed issues and do not introduce unrelated edits.
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

Das Argument entscheidet:

- **Leer:** die eigenen offenen Tasks aus `k_playbook_doc_list` (Sorte `tasks`, ohne `done`),
  sortiert nach der Nummer am Anfang des Dateinamens. Keine → „Keine offenen Tasks gefunden“
  und Schluss.
- **Ein `keph:`-Verweis** (`keph:/user/…/tasks/<datei>`): genau dieser Task.
- **Eine Nummer:** aufgelöst nach `on-demand/tasks.md`, „Wohin und Name“ — zuerst unter den
  eigenen offenen, dann unter den eigenen erledigten Tasks; bei mehreren Treffern alle zeigen
  und den Nutzer wählen lassen.
- **Sonst ein Pfad auf der Platte:** eine Datei oder ein Verzeichnis mit `.md`-Dateien, relativ
  zu `context.project.dir`. Gibt es ihn nicht, brich mit einer klaren Meldung ab. Sag dazu,
  dass dies ein ausdrückliches Einzelziel ist und nicht die Task-Warteschlange. Bei einem
  Verzeichnis: alle `.md` direkt darin, ohne `done/`, `old/` und Archive, sortiert nach der
  führenden Nummer (auch mit `_`), Dateien ohne Nummer übersprungen. Keine → „Keine
  Dateien gefunden“ und Schluss.

Merke die Ziele als `REVIEW_TARGETS`: je Eintrag der `keph:`-Verweis oder der absolute Pfad,
dazu die Anzeigeform (der Verweis bzw. der projektrelative Pfad) und ob es ein Task in der
Collection oder eine Datei ist.

## Schritt 3 — Inhalte lesen

Für jedes Ziel: einen Task mit `k_playbook_doc_read` (`ref`), eine Datei mit dem
Lese-Werkzeug. Setze `CURRENT_FILE_STATE` auf den tatsächlichen Inhalt. Bei einem Task merkst
du dir die `revision` aus der Antwort als `REVISION` dieses Tasks; sie ist das erste
`since_revision`.

Prüfe je Ziel auf einen Abschnitt `## Intent`. Der Intent steht auf zwei Arten, auch
zusammen:

**Option A — Inline-Text** (direkt im Task):

```markdown
## Intent
Der Entwickler soll nach Ausführung dieses Tasks die TTS-Anbindung
ohne Rückfragen implementieren können.
```

**Option B — Dateireferenz** (eine oder mehrere externe Dateien):

```markdown
## Intent
[requirements.md](../requirements.md)
[constraints.md](../constraints.md)
```

Referenzen liest du und nimmst ihren Inhalt als Intent; stehen beide, gelten beide. Ohne
`## Intent` entfällt der Alignment-Check (Schritt 11).

## Fertig-Meldung

Vor jeder Rückfrage an den Nutzer und am Ende meldet sich der Moderator auf dem Rechner des
Nutzers. Die Stellen in diesem Command: die Rückfrage in Schritt 8, die Rückfrage nach einem
`foreign_change` und das Ende nach Schritt 12.

include: _task/fertig-meldung.md

## Schritt 4 — Startübersicht ausgeben

Vor allem anderen an den Nutzer:

```
Review gestartet
─────────────────────────────
Tasks:   <Liste der Ziele in Anzeigeform>
Projekt: <context.project.dir>
Intent:  <Liste der Intent-Quellen, oder "— (kein Intent angegeben)">
Runden:  max. 5
```

---

## Schritt 5 — Critic-Runde

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

Output format (table, no intro text):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

**Review focus.** Check from above:

- Is the overall approach coherent?
- Are there contradictions between steps or files?
- Is a critical constraint missing that the executing agent cannot infer?
- Is the scope/framing reasonable?
- (From round 2+) Did the Editor's changes introduce new problems?

Do NOT flag: implementation choices, missing code details, style, minor wording.

## Schritt 6 — Moderator: an den Editor routen

For each issue from the Critic, decide:

- **pass** — clear mistake or blocking missing constraint; send to Editor.
- **skip** — non-blocking WARNUNG/FEHLEND; record but do not send.
- **decide** — design decision the Moderator can resolve; record the decision.
- **ask-user** — requires product/process input; pause loop, ask user (Fertig-Meldung
  vorher), then continue.

Skip WARNUNGs and FEHLENDs unless they block execution. Store the routing table for the
final Review-Log:

| ID | Kategorie | Route | Begründung | Weitergabe an Editor? |

## Schritt 7 — Editor-Runde

Spawn a general-purpose subagent (OpenCode: `general`, Claude Code: `general-purpose`)
as Editor with this prompt:

```
You are the Editor in a structured review loop. The Critic has identified issues in these task/instruction files.
You are read-only. Do not modify documents or files, run formatters, or call write/edit tools.
For each issue: either propose a concrete fix OR explain clearly why you disagree or see it differently.
Do not fix silently — for every issue, state what you did (fixed / not fixed + reason).

## Issues to address
<insert all issues routed as pass, with IDs and Moderator decisions>

## Files
<one line per target: the keph: reference or the absolute path>
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <context.project.dir>), a path with your file reading tool.

Output:
1. Per changed target, the edits as pairs — ALT: the exact text to replace, long enough to occur exactly once in the document; NEU: its replacement. Insertions name the unique line before which or after which the text goes. Do not write to disk or to the collection.
2. A table of your decisions:
| ID | Aktion | Begründung |
```

## Schritt 8 — Moderator: Vorschläge prüfen und anwenden

The Moderator reviews the Editor output before changing anything:

- Confirm each proposed edit maps to a routed issue or a required consistency change.
- Reject unrelated rewrites, style-only churn, broad restructuring, or edits outside
  `REVIEW_TARGETS` unless explicitly justified and approved.
- If the proposal is clean, apply the minimal change:
  - an einem Task mit `k_playbook_doc_replace` (`ref`, `old` = ALT, `new` = NEU,
    `since_revision` = `REVISION` dieses Tasks); die Antwort nennt die neue Revision, die ab
    jetzt `REVISION` ist. `text_not_unique` heißt: ALT mit mehr Umgebung erneut; `text_not_found`
    heißt: gegen `CURRENT_FILE_STATE` prüfen, ob die Stelle inzwischen anders heißt.
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
Review-Log.

## Schritt 9 — Moderator: Offenes zurück an den Critic

Collect all issues the Editor did NOT fix, all Moderator-rejected edits, and any concerns
noticed while rereading the actual state. For each, pass the Editor's reasoning and the
Moderator decision to the Critic.

If nothing is unresolved and no risky new change exists → use the fast path and skip to
Schritt 11 (Intent check, if Intent exists).

## Schritt 10 — Critic antwortet (bis zu 5 Runden insgesamt)

Spawn a new Critic subagent with this prompt:

```
You are the Critic in round <N> of a structured review loop.
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
- If Critic insists or adds new issues → pass to Editor (Schritt 6), new round
- If both sides have argued the same point twice without movement → Moderator decides and
  notes it as a Moderator-Entscheidung
- If all issues resolved or max 5 rounds reached → proceed to Schritt 11
- If continuous improvement is happening → allow up to 5 rounds; stop early as soon as
  agreement is reached

---

## Schritt 11 — Intent-Alignment-Check (nur mit Intent)

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

## Schritt 12 — Review-Log anhängen

Hänge den folgenden Block an **jedes geprüfte Ziel** an — nicht nur an die geänderten: an
einen Task mit `k_playbook_doc_append` (`ref`, `content` = der Block, `since_revision` =
`REVISION`; die Antwort nennt die Revision, mit der der Task jetzt steht), an eine
Plattendatei per Shell mit einem Heredoc, dessen Endmarke in Anführungszeichen steht.

Ein Ziel, an dem nichts zu korrigieren war, ist trotzdem geprüft worden, und diese Prüfung
muss sichtbar bleiben. `/k-task-run` erkennt an genau diesem Block (Kennzeile
`## Review-Log (<datum>)`, siehe `on-demand/tasks.md`), ob ein Task jemals gegengelesen
wurde; fehlt er, meldet `/k-task-run` den Task später als ungeprüft und fragt unnötig nach.

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
<Result of Schritt 11, or "— (kein Intent angegeben)">

### Geänderte Dateien
- <Ziel>: <what changed> (FEHLER-01, FEHLER-03)
- oder: — keine Änderungen

### Offen (nicht gefixt)
- <ID>: <reason>
```

Ein `foreign_change` beim Anhängen wird behandelt wie in Schritt 8: anhalten,
Fertig-Meldung, Rückfrage; nach „weiter“ mit der Revision aus der Antwort erneut anhängen.
Nach `outcome_unknown` nicht noch einmal anhängen, sondern mit `k_playbook_doc_grep`
(`^## Review-Log \(<datum>\)$`, `count_only`) nachsehen, ob der Block angekommen ist.

Danach die Fertig-Meldung (`fertig`; der Body nennt Runden, gefixte und offene Issues).

Folge-Command: **`/k-task-run`** — führt die gehärteten Tasks aus und legt in jedem das
Ergebnis der Ausführung ab; mit demselben Verweis oder derselben Nummer genau diesen.
