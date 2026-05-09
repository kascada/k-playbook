# Skill: review-loop

TRIGGER when: the user invokes `/review-loop <path>` where `<path>` is a file or directory containing task/instruction files.

Review task files at a high level before execution. A Reviewer agent checks the overall approach, the Author agent corrects files directly. Changes are listed at the end.

---

## Execution

### Step 1 — Collect files

If `<path>` is a directory: collect all `.md` files in it.
If `<path>` is a file: use that file.

Read all collected files into context.

### Step 2 — Reviewer round

Spawn a `general-purpose` subagent with this prompt:

```
You are reviewing task/instruction files before they are executed by an AI agent.
Review at a HIGH LEVEL — does the overall approach make sense? You are NOT checking implementation details.
The executing agent will fill in details; focus on the big picture.

Files to review:
<insert file contents with filenames>

Check for:
- FEHLER   — fundamentally wrong approach, contradiction, or missing critical piece
- WARNUNG  — potentially problematic, unclear framing
- FEHLEND  — important constraint or context not specified

Output format (table, no intro text):
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
```

### Step 3 — Moderate

For each issue decide:
- **Clear mistake → fix directly** (Author agent)
- **Design decision → decide yourself** and note the decision
- **Needs user input → ask** (stop loop, ask user, then continue)

Skip WARNUNGs and FEHLENDs unless they block execution.

### Step 4 — Author round (only if there are FEHLERs to fix)

Spawn a `general-purpose` subagent with this prompt:

```
You are correcting task/instruction files based on a review.
Change ONLY what is listed below. Write corrected files using the Write tool.
Do not summarize — write the complete corrected file content.

Issues to fix:
<insert FEHLER list with IDs and moderator decisions>

Files to correct (full content):
<insert file contents>
```

### Step 5 — Second review

Spawn a new Reviewer agent with a focused prompt:
- Show only changed sections
- Ask: "Are all original FEHLERs resolved? Have any new problems been introduced?"

If 0 FEHLERs → done. Otherwise repeat from Step 3.

**Max 3 rounds.** After 3 rounds without convergence: stop and ask user.

### Step 6 — Summary

After completion, list all changes made:
```
Geänderte Dateien:
- <filename>: <what was changed> (FEHLER-01, FEHLER-03)
Offen (nicht gefixt):
- <ID>: <reason>
```

---

## Review focus (for Reviewer prompt)

The review is "from above" — check:
- Is the overall approach coherent?
- Are there contradictions between steps or files?
- Is a critical constraint missing that the executing agent cannot infer?
- Is the scope/framing reasonable?

Do NOT flag: implementation choices, missing code details, style, minor wording.
