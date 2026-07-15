---
description: Review task/instruction files using a Critic/Editor dialogue before execution. If no path is given, uses the tasks: path from K-PLAYBOOK.MD. Two agents debate issues; the Moderator routes, decides, and appends a discussion log. Final intent alignment check at the end.
argument-hint: [path]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Glob, Task]
---

# k-review-loop

Review task/instruction files before execution using a structured two-agent dialogue between a **Critic** and an **Editor**. The Moderator routes between them, decides on deadlocks, and appends a discussion log. A final alignment check verifies the result against the stated Intent.

## Invocation

`/k-review-loop` — review open task files from the `tasks:` path in `K-PLAYBOOK.MD`.
`/k-review-loop <path>` — review an explicit file or directory containing `.md` task/instruction files.

---

## Execution

### Step 1 — Resolve target path

If `$ARGUMENTS` is provided: treat it as the explicit review target.

- If it is a file: use that file.
- If it is a directory: use that directory.
- If it does not exist: abort with a clear error.

If `$ARGUMENTS` is empty: read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `tasks:` → `TASKS_DIR`

Command-specific policy:

- If `TASKS_DIR` is set and exists: use it as the review target.
- If `TASKS_DIR` is set but missing: tell the user and ask whether to create it or abort. For review, abort is the default recommendation because there are no tasks to review.
- If `TASKS_DIR` is unset or `K-PLAYBOOK.MD` is missing: tell the user that `/k-setup` can register the task path, then ask which file or directory to review for this run.

Remember the chosen absolute target as `REVIEW_TARGET` and the display path as `REVIEW_TARGET_DISPLAY`.

### Step 2 — Collect files

If `REVIEW_TARGET` is a directory:
- Collect all `.md` files directly in that directory (not subdirectories).
- Exclude `done/`, `old/`, and any archived/completed task subdirectories.
- Sort by leading number if present (`001-...`, `014-...`), otherwise alphabetically after numbered files.
- If no `.md` files are found: report „Keine offenen Task-Dateien gefunden" and stop.

If `REVIEW_TARGET` is a file: use that file.

Read all collected files. For each file, check for an `## Intent` section.
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
If no `## Intent` section exists: skip the alignment check (Step 9).

### Step 3 — Print startup summary

Output to the user before doing anything else:

```
Review gestartet
─────────────────────────────
Tasks:   <list of task filenames>
Pfad:    <REVIEW_TARGET_DISPLAY>
Intent:  <list of intent filenames, or "— (kein Intent angegeben)">
Runden:  max. 5
```

---

### Step 4 — Critic round

Spawn a `general-purpose` subagent (Critic) with this prompt:

```
You are the Critic in a structured review of task/instruction files before they are executed by an AI agent.
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
- **Clear mistake → pass to Editor**
- **Design decision → decide yourself** and note the decision
- **Needs user input → ask** (pause loop, ask user, then continue)

Skip WARNUNGs and FEHLENDs unless they block execution.

### Step 6 — Editor round

Spawn a `general-purpose` subagent (Editor) with this prompt:

```
You are the Editor in a structured review loop. The Critic has identified issues in these task/instruction files.
For each issue: either fix it directly OR explain clearly why you disagree or see it differently.
Do not fix silently — for every issue, state what you did (fixed / not fixed + reason).

## Issues to address
<insert FEHLER list with IDs and Moderator decisions>

## Files (full content)
<insert file contents>

Output:
1. Updated file contents (Write each changed file completely)
2. A table of your decisions:
| ID | Aktion | Begründung |
```

### Step 7 — Moderator: route unresolved items back to Critic

Collect all issues the Editor did NOT fix. For each, pass the Editor's reasoning to the Critic.

If nothing is unresolved and no new issues exist → skip to Step 9 (Intent check).

### Step 8 — Critic responds (repeat up to 5 rounds total)

Spawn a new Critic subagent with this prompt:

```
You are the Critic in round <N> of a structured review loop.
Below are issues you raised that the Editor did not fix, along with the Editor's reasoning.
Also review the current state of the files for any NEW problems introduced by the Editor's changes.

Respond to each unresolved item: do you accept the Editor's reasoning, clarify your point, or insist?
Then list any genuinely new problems caused by the Editor's changes.

## Unresolved items (with Editor's reasoning)
<insert items + reasoning>

## Current file state
<insert current file contents>

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
- If all issues resolved or max 5 rounds reached → proceed to Step 9
- If continuous improvement is happening → allow up to 5 rounds; stop early as soon as agreement is reached

---

### Step 9 — Intent alignment check (only if Intent was provided)

Spawn a final Critic subagent with this prompt:

```
You are doing a final alignment check. Below is the Intent (the goal the task must achieve) and the current state of the task files after review.

## Intent
<insert intent text>

## Current task files
<insert current file contents>

Answer only: Can the task, as written, achieve the stated Intent?
- Yes → state briefly why
- No → list what is missing or misaligned

Output (no intro text):
| Alignment | Begründung |
```

If alignment is NO → Moderator decides: one more targeted Editor round or ask user.

---

### Step 10 — Discussion log

Append the following block to each modified task file:

```markdown
---
## Review-Log (<date>)

**Pfad:** <REVIEW_TARGET_DISPLAY>
**Intent:** <filenames or "—">
**Runden:** <N>

### Diskussion
<For each issue that was debated (not trivially fixed): 2-3 sentences capturing the core argument from each side and the resolution>

### Moderator-Entscheidungen
<List any points where the Moderator broke a deadlock, with brief reasoning>

### Intent-Alignment
<Result of Step 9, or "— (kein Intent angegeben)">

### Geänderte Dateien
- <filename>: <what changed> (FEHLER-01, FEHLER-03)

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
