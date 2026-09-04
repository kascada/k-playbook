---
description: Härtet Task- und Instruction-Dateien vor Ausführung mit einem read-only Critic/Editor-Dialog; ohne Argument wird das Task-Verzeichnis verwendet.
argument-hint: [path]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Task, Agent]
---

# k-task-refine

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Härtet Task-/Instruction-Dateien vor Ausführung über einen strukturierten Zwei-Agenten-Dialog zwischen **Critic** und **Editor**. Critic und Editor sind read-only Advisors. Der Moderator routet zwischen ihnen, entscheidet Deadlocks, wendet akzeptierte Edits an und hängt ein Discussion Log an. Ein finaler Alignment-Check prüft das Ergebnis gegen den angegebenen Intent.

## Invocation

`/k-task-refine` — offene Task-Dateien aus `k-playbook-local/tasks/` härten.
`/k-task-refine <path>` — eine explizite Datei oder ein Verzeichnis mit `.md`-Task-/Instruction-Dateien härten.

---

## Process invariants

- **Subagents are read-only.** Critic and Editor must not modify files, run formatters, or call write/edit tools. They only return findings, reasoning, and proposed file contents.
- **The Moderator is the only writer.** Apply changes only after checking that they address routed issues and do not introduce unrelated edits.
- **Actual file state wins.** After every Moderator-applied edit, reread modified files and use that content for follow-up Critic rounds and the final alignment check.
- **Keep an audit trail.** Record all Critic issues, Moderator routing decisions, Editor decisions, skipped items, deadlocks, and final alignment in the Review-Log. Every reviewed file gets a log, including files that needed no change — the log is the only evidence that a review happened at all.
- **Use the fast path when possible.** If one Critic round yields only clear, non-controversial fixes and the Editor proposal is clean, apply once, reread, run the final alignment check, and stop.

---

## Execution

### Step 1 — Resolve target path

The context load from the first step is the preflight, even for explicit file or
directory arguments: it establishes which project is being worked in.

From the context output:

- `RESOLVED_TASKS_DIR = <local.dir>/tasks`.
- `TASKS_DISPLAY_PATH = k-playbook-local/tasks`.

Command-specific policy:

- If `$ARGUMENTS` is provided: treat it as the explicit review target.
  - If it is a file: use that file.
  - If it is a directory: use that directory.
  - If it does not exist: abort with a clear error.
  - If the target lies outside `RESOLVED_TASKS_DIR`, continue, but announce that this is an explicit one-off target rather than the standard task queue.
- If `$ARGUMENTS` is empty:
  - If `RESOLVED_TASKS_DIR` is missing on disk: abort and tell the user to run `/k-gui`.
  - Otherwise use it as the review target.

Remember the chosen absolute target as `REVIEW_TARGET` and the display path as `REVIEW_TARGET_DISPLAY`.

### Step 2 — Collect files

If `REVIEW_TARGET` is a directory:
- Collect all `.md` files directly in that directory (not subdirectories).
- Exclude `done/`, `old/`, and any archived/completed task subdirectories.
- Sort by the leading number in the filename (`001-...`, `014-...`; also accept `_`).
- Skip files without a leading number. `README.md` and other notes in the task directory are not task files.
- If no `.md` files are found: report „Keine offenen Task-Dateien gefunden" and stop.

If `REVIEW_TARGET` is a file: use that file.

Read all collected files and set `CURRENT_FILE_STATE` to the actual on-disk contents. For each file, check for an `## Intent` section.
The Intent can be specified in two ways — both are valid, also in combination:

**Option A — Inline text** (direkt im Task):
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

If file references are present: read those files and use their content as Intent.
If both inline text and file references are present: combine both.
If no `## Intent` section exists: skip the alignment check (Step 10).

### Step 3 — Print startup summary

Output to the user before doing anything else:

```
Review gestartet
─────────────────────────────
Tasks:   <list of task filenames>
Pfad:    <REVIEW_TARGET_DISPLAY>
Projekt: <project.dir>; Tasks: <TASKS_DISPLAY_PATH>
Intent:  <list of intent filenames, or "— (kein Intent angegeben)">
Runden:  max. 5
```

---

### Step 4 — Critic round

Spawn a general-purpose subagent (OpenCode: `general`, Claude Code: `general-purpose`)
as Critic with this prompt:

```
You are the Critic in a structured review of task/instruction files before they are executed by an AI agent.
You are read-only. Do not modify files or call write/edit tools.
Review at a HIGH LEVEL — does the overall approach make sense? Do NOT check implementation details.
The executing agent will fill in details; focus on the big picture.

<if Intent is present>
## Intent (the goal these files must achieve)
<insert intent text>
</if>

## Files to review
<insert file contents with filenames>

Check for:
- FEHLER   — fundamentally wrong approach, contradiction, or missing critical piece
- WARNUNG  — potentially problematic or unclear framing
- FEHLEND  — important constraint or context not specified

Output format (table, no intro text):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

### Step 5 — Moderator: route to Editor

For each issue from the Critic, decide:
- **pass** — clear mistake or blocking missing constraint; send to Editor.
- **skip** — non-blocking WARNUNG/FEHLEND; record but do not send.
- **decide** — design decision the Moderator can resolve; record the decision.
- **ask-user** — requires product/process input; pause loop, ask user, then continue.

Skip WARNUNGs and FEHLENDs unless they block execution. Store the routing table for the final Review-Log:

| ID | Kategorie | Route | Begründung | Weitergabe an Editor? |

### Step 6 — Editor round

Spawn a general-purpose subagent (OpenCode: `general`, Claude Code: `general-purpose`)
as Editor with this prompt:

```
You are the Editor in a structured review loop. The Critic has identified issues in these task/instruction files.
You are read-only. Do not modify files, run formatters, or call write/edit tools.
For each issue: either propose a concrete fix OR explain clearly why you disagree or see it differently.
Do not fix silently — for every issue, state what you did (fixed / not fixed + reason).

## Issues to address
<insert all issues routed as pass, with IDs and Moderator decisions>

## Files (full content)
<insert file contents>

Output:
1. Proposed updated file contents (write each changed file completely in your response; do not write to disk)
2. A table of your decisions:
| ID | Aktion | Begründung |
```

### Step 7 — Moderator: validate and apply proposed edits

The Moderator reviews the Editor output before changing files:

- Confirm each proposed edit maps to a routed issue or a required consistency change.
- Reject unrelated rewrites, style-only churn, broad restructuring, or edits outside `REVIEW_TARGET` unless explicitly justified and approved.
- If the proposal is clean, apply the minimal change using the available file-editing tool.
- If the proposal is partially useful, apply only the minimal accepted subset and record the rejection reason.
- If the proposal cannot be applied cleanly, resolve the conflict minimally yourself or ask the user if the intended edit is ambiguous.
- Immediately reread every modified file and set `CURRENT_FILE_STATE` to the actual on-disk contents.

Do not trust proposed file contents as applied state until the reread confirms it.

### Step 8 — Moderator: route unresolved items back to Critic

Collect all issues the Editor did NOT fix, all Moderator-rejected edits, and any concerns noticed while rereading the actual file state. For each, pass the Editor's reasoning and the Moderator decision to the Critic.

If nothing is unresolved and no risky new change exists → use the fast path and skip to Step 10 (Intent check, if Intent exists).

### Step 9 — Critic responds (repeat up to 5 rounds total)

Spawn a new Critic subagent with this prompt:

```
You are the Critic in round <N> of a structured review loop.
Below are issues you raised that the Editor did not fix, along with the Editor's reasoning.
Also review the current state of the files for any NEW problems introduced by the Moderator-applied changes.

Respond to each unresolved item: do you accept the Editor's reasoning, clarify your point, or insist?
Then list any genuinely new problems caused by the Editor's changes.

## Unresolved items (with Editor's reasoning)
<insert items + reasoning>

## Current file state
<insert CURRENT_FILE_STATE, reread from disk after Moderator-applied edits>

Output:
1. Response table:
| ID | Antwort | Akzeptiert? (ja/nein) | Klarstellung / Neues Argument |
2. New issues table (if any):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

**Moderator decides:**
- If Critic accepts Editor's reasoning → issue closed
- If Critic insists or adds new issues → pass to Editor (Step 5), new round
- If both sides have argued the same point twice without movement → Moderator decides and notes it as a Moderator-Entscheidung
- If all issues resolved or max 5 rounds reached → proceed to Step 10
- If continuous improvement is happening → allow up to 5 rounds; stop early as soon as agreement is reached

---

### Step 10 — Intent alignment check (only if Intent was provided)

Spawn a final general-purpose Critic subagent (OpenCode: `general`, Claude Code:
`general-purpose`) with this prompt:

```
You are doing a final alignment check. Below is the Intent (the goal the task must achieve) and the current state of the task files after review.
You are read-only. Do not modify files or call write/edit tools.

## Intent
<insert intent text>

## Current task files
<insert CURRENT_FILE_STATE, reread from disk after all Moderator-applied edits>

Answer only: Can the task, as written, achieve the stated Intent?
- Yes → state briefly why
- No → list what is missing or misaligned

Output (no intro text):
| Alignment | Begründung |
```

If alignment is NO → Moderator decides: one more targeted Editor round or ask user.

---

### Step 11 — Discussion log

Append the following block to **each reviewed task file** — not only to the ones that
were changed.

Eine Datei, an der nichts zu korrigieren war, ist trotzdem geprüft worden, und diese
Prüfung muss sichtbar bleiben. `/k-task-run` Step 1.2 erkennt an genau diesem Block, ob ein
Task jemals gegengelesen wurde; fehlt er auf einer ungeänderten Datei, meldet `/k-task-run`
sie später als ungeprüft und fragt unnötig nach.

Ergab die Prüfung keine Änderung, wird derselbe Block angehängt: Tabellen ohne Zeilen
und unter `### Geänderte Dateien` der Vermerk `— keine Änderungen`.

```markdown
---
## Review-Log (<now.date>)

**Pfad:** <REVIEW_TARGET_DISPLAY>
**Intent:** <filenames or "—">
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
<Result of Step 10, or "— (kein Intent angegeben)">

### Geänderte Dateien
- <filename>: <what changed> (FEHLER-01, FEHLER-03)
- oder: — keine Änderungen

### Offen (nicht gefixt)
- <ID>: <reason>
```

---

## Review focus

Check from above:
- Is the overall approach coherent?
- Are there contradictions between steps or files?
- Is a critical constraint missing that the executing agent cannot infer?
- Is the scope/framing reasonable?
- (From round 2+) Did the Editor's changes introduce new problems?

Do NOT flag: implementation choices, missing code details, style, minor wording.
