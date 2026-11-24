---
title: Review-Dialog
description: Modul für /k-task-refine und die Sub-Agenten seiner Gruppen — der Critic/Editor-Dialog über Tasks und Instruction-Dateien bis zum Review-Log, samt den Regeln und dem Rückgabeformat für den Sub-Agenten einer Gruppe. Für sich vollständig lesbar.
---

# Review-Dialog

Wer dieses Modul liest, ist der **Moderator** des Dialogs zwischen **Critic** und **Editor**,
für die `REVIEW_TARGETS`, die ihm übergeben sind: der Hauptkontext von `/k-task-refine` bei
einem Ziel und im Rückfall, sonst der Sub-Agent einer Gruppe. Critic und Editor sind read-only
Advisors; der Moderator routet zwischen ihnen, entscheidet Deadlocks, wendet akzeptierte Edits
an und hängt ein Review-Log an. Ein Alignment-Check prüft das Ergebnis gegen den Intent.
Critic, Editor und Alignment-Critic sind General-Purpose-Sub-Agenten (OpenCode: `general`,
Claude Code: `general-purpose`); ihre Prompts stehen unten und sind vollständig.

**Eingaben.** `REVIEW_TARGETS` (je Ziel der `keph:`-Verweis oder der absolute Pfad, die
Anzeigeform, ob Task in der Collection oder Datei, bei einem Task `REVISION`), `projectDir`
(`project.dir`); der Sub-Agent einer Gruppe bekommt sie im Auftrag, in Claude Code
lädt er die Werkzeuge `mcp__k-playbook__*` zuerst mit der Werkzeugsuche. Verbindlich für den
Schreibweg eines Tasks ist `k-playbook:/instructions/on-demand/tasks.md`; lies sie mit
`k_playbook_doc_read`.

## Regeln

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

include: k-playbook:/commands/_task/revisionskette.md

Im Dialog heißt das: Jedes Ändern eines Tasks (`k_playbook_doc_replace`) und jedes Anhängen
trägt `since_revision` = `REVISION` dieses Tasks. Nach „weiter“ liest du den Task neu und
wendest die Edits erneut gegen den neuen Stand an; sonst endet die Arbeit für diesen Task
ohne Review-Log.

## Als Sub-Agent einer Gruppe

Die Tasks der Gruppe sind die `REVIEW_TARGETS`; mehrere werden gemeinsam geprüft.

- Du fragst den Nutzer nicht und sendest keine Fertig-Meldung, auch nicht am Ende. Wo der
  Dialog eine Rückfrage vorsieht (ask-user beim Routen, unklare Änderung beim Anwenden, nicht
  erreichter Intent), hältst du an, hängst an keinen Task der Gruppe ein Review-Log an und gibst
  die Frage mit deinem Zwischenstand zurück.
- Die Gruppe ist eine Einheit: Brauchst du eine Entscheidung oder meldet ein Task
  `foreign_change`, hältst du für die ganze Gruppe an und hängst an keinen ihrer Tasks ein
  Review-Log an. Kommt `foreign_change` erst beim Anhängen des Review-Logs, tragen die Tasks,
  an die schon angehängt ist, ihr Review-Log; an die übrigen hängst du nichts mehr an.
- Kannst du keine Sub-Agenten starten, gibst du sofort `Ergebnis: keine-subagenten` je Task
  zurück und schreibst nichts.
- Bei einer Fortsetzung bekommst du den `Zwischenstand:` des vorigen Sub-Agenten, die
  Revisionen aus seinen Zeilen `Revision:` bzw. `Foreign-Change:` und die Antwort des Nutzers
  als Entscheidung. Du setzt an der Stelle des Zwischenstands fort, zählst die Runden weiter
  (höchstens 5 insgesamt) und hängst je Task ein einziges, vollständiges Review-Log an, das auch
  den Zwischenstand enthält.

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
Review-Log hängt am Task. `rückfrage` heißt: wartet auf die Antwort des Nutzers. `abgebrochen`
heißt: ohne Review-Log angehalten, ohne eigene Frage — wegen eines `foreign_change` in der
Gruppe oder weil die Collection nicht erreichbar war.

Nach den Blöcken folgt einmal für die Gruppe, bei `rückfrage`, die Zeile
`Rückfrage: <die Frage in einem Satz>`. Bei `rückfrage` und bei `Foreign-Change:` folgt zuletzt
ein Block `Zwischenstand:` mit den Abschnitten des Review-Logs bis dahin — Critic-Issues,
Routing, Editor-Entscheidungen, die schon angewandten Edits — und der laufenden Runde.

## Inhalte lesen

Für jedes Ziel: einen Task mit `k_playbook_doc_read` (`ref`, `projectDir`), eine Datei mit dem
Lese-Werkzeug. Setze `CURRENT_FILE_STATE` auf den tatsächlichen Inhalt. Bei einem Task ist die
`revision` aus der Antwort das erste `since_revision`; weicht sie von der übergebenen
`REVISION` ab, gilt der Replica-Nachlauf (Revisionskette) und danach `foreign_change`. Sonst
bleibt sie `REVISION`.

Prüfe je Ziel auf einen Abschnitt `## Intent`. Der Intent steht auf zwei Arten, auch
zusammen:

**Option A — Inline-Text** direkt im Task unter `## Intent`.

**Option B — Verweise** im Intent, etwa `project:/docs/requirements.md` oder
`keph:/docs/manual/tts.md`: Was dafür gilt, steht in `k-playbook:/instructions/on-demand/tasks.md`,
„Intent“.

Die Verweise liest du mit (`k-playbook:` und `keph:` mit `k_playbook_doc_read`, `project:/`
von der Platte) und nimmst ihren Inhalt zum Intent; stehen Text und Verweise, gelten beide.
Ohne `## Intent` entfällt der Alignment-Check.

## Critic-Runde

Starte einen Sub-Agenten als Critic mit diesem Prompt:

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
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <project.dir>), a path with your file reading tool.

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

## An den Editor routen

For each issue from the Critic, decide:

- **pass** — clear mistake or blocking missing constraint; send to Editor.
- **skip** — non-blocking WARNUNG/FEHLEND; record but do not send.
- **decide** — design decision the Moderator can resolve; record the decision.
- **ask-user** — requires product/process input; pause loop, ask user (Fertig-Meldung
  `Rückfrage: <worum es geht>` vorher), then continue. A reference whose prefix is ambiguous by
  `k-playbook:/instructions/on-demand/verweise-nachziehen.md` is always ask-user.

Skip WARNUNGs and FEHLENDs unless they block execution. Store the routing table for the
final Review-Log:

| ID | Kategorie | Route | Begründung | Weitergabe an Editor? |

## Editor-Runde

Starte einen Sub-Agenten als Editor mit diesem Prompt:

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
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <project.dir>), a path with your file reading tool.

Output:
1. Per changed target, the edits as pairs — ALT: the exact text to replace, long enough to occur exactly once in the document; NEU: its replacement. Insertions name the unique line before which or after which the text goes. Do not write to disk or to the collection.
2. A table of your decisions:
| ID | Aktion | Begründung |
```

## Vorschläge prüfen und anwenden

The Moderator reviews the Editor output before changing anything:

- Confirm each proposed edit maps to a routed issue or a required consistency change.
- Reject unrelated rewrites, style-only churn, broad restructuring, or edits outside
  `REVIEW_TARGETS` unless explicitly justified and approved.
- If the proposal is clean, apply the minimal change:
  - an einem Task mit `k_playbook_doc_replace` (`ref`, `old` = ALT, `new` = NEU,
    `since_revision` = `REVISION` dieses Tasks); die Antwort nennt die neue Revision, die ab
    jetzt `REVISION` ist. `text_not_unique` heißt: ALT mit mehr Umgebung erneut; `text_not_found`
    heißt: gegen `CURRENT_FILE_STATE` prüfen, ob die Stelle inzwischen anders heißt.
    `outcome_unknown` heißt: steht NEU jetzt im Task, gilt die Revision aus dem Nachlesen
    als `REVISION`; sonst der feste Weg aus „`outcome_unknown` je Operation“ in `tasks.md`.
  - an einer Plattendatei mit dem Edit-Werkzeug des Assistenten.
- If the proposal is partially useful, apply only the minimal accepted subset and record the
  rejection reason.
- If the proposal cannot be applied cleanly, resolve the conflict minimally yourself or ask
  the user if the intended edit is ambiguous — vorher die Fertig-Meldung (`Rückfrage: unklare
  Änderung`).
- Immediately reread every modified target (Task: `k_playbook_doc_read`, Datei: Lese-Werkzeug)
  and set `CURRENT_FILE_STATE` to the actual content.

Do not trust proposed edits as applied state until the reread confirms it.

## Offenes zurück an den Critic

Collect all issues the Editor did NOT fix, all Moderator-rejected edits, and any concerns
noticed while rereading the actual state. For each, pass the Editor's reasoning and the
Moderator decision to the Critic.

If nothing is unresolved and no risky new change exists → use the fast path and skip to the
Intent-Alignment-Check (if Intent exists), otherwise to das Review-Log.

## Critic antwortet (bis zu 5 Runden insgesamt)

Starte einen neuen Critic-Sub-Agenten mit diesem Prompt:

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
Read each target yourself, now, after the Moderator applied the edits: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <project.dir>), a path with your file reading tool.

Output:
1. Response table:
| ID | Antwort | Akzeptiert? (ja/nein) | Klarstellung / Neues Argument |
2. New issues table (if any):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

**Moderator decides:**

- If Critic accepts Editor's reasoning → issue closed
- If Critic insists or adds new issues → pass to Editor (Abschnitt „An den Editor routen“), new round
- If both sides have argued the same point twice without movement → Moderator decides and
  notes it as a Moderator-Entscheidung
- If all issues resolved or max 5 rounds reached → proceed to the Intent-Alignment-Check
- If continuous improvement is happening → allow up to 5 rounds; stop early as soon as
  agreement is reached

## Intent-Alignment-Check (nur mit Intent)

Starte einen letzten Critic-Sub-Agenten mit diesem Prompt:

```
You are doing a final alignment check. Below is the Intent (the goal the task must achieve) and the targets after review.
You are read-only. Do not modify documents or files or call write/edit tools.

## Intent
<insert intent text>

## Current task files
<one line per target: the keph: reference or the absolute path>
Read each target yourself: a keph: reference with the MCP tool k_playbook_doc_read (ref = the reference, projectDir = <project.dir>), a path with your file reading tool.

Answer only: Can the task, as written, achieve the stated Intent?
- Yes → state briefly why
- No → list what is missing or misaligned

Output (no intro text):
| Alignment | Begründung |
```

If alignment is NO → Moderator decides: one more targeted Editor round or ask user
(Fertig-Meldung `Rückfrage: Intent nicht erreicht` vorher).

## Review-Log anhängen

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
`now.date`.

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
<Result of the Intent-Alignment-Check, or "— (kein Intent angegeben)">

### Geänderte Dateien
- <Ziel>: <what changed> (FEHLER-01, FEHLER-03)
- oder: — keine Änderungen

### Offen (nicht gefixt)
- <ID>: <reason>
```

Vor dem Anhängen an einen Task zählt der Moderator die Zeile `^## Review-Log \(<datum>\)$`
(Revisionskette, „Zählen“); die Revision aus der Zählung muss `REVISION` sein. Nach
`outcome_unknown` zählt er dieselbe Zeile erneut: Ist die Zahl gestiegen, ist der Block
angekommen.

Hier endet der Dialog. Der Sub-Agent einer Gruppe gibt jetzt seine Rückgabe zurück. Der
Hauptkontext macht dort weiter, wo ihn der Command hergeschickt hat.
