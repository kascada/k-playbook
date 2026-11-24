# Task flow

The task flow is the standard path for planned work that should not be completed directly in a short chat step.

## Standard flow

```text
/k-task-create
/k-task-refine
/k-task-run
```

Tasks can arise directly from the conversation or be created by `/k-remediation`. In both cases: review tasks first, then execute them.

## Where a task lives

A task is a document in the project's Kephalaion collection under `keph:/user/<user>/tasks/`; a completed task is moved to `done/` below it. There is no task on disk any more: without a usable Kephalaion binding (`kephalaion.ok` not `true`) the task commands stop and say so, instead of falling back to a directory. The shipped always-instruction `300-arbeiten-in-tasks.md` says when work becomes a task; everything about the form of a task -- location, name and number, header, structure, life cycle, marker lines, progress lines, the write path -- stands once, in `k-playbook:/instructions/on-demand/tasks.md`. The four commands and `/k-remediation` refer to it and do not repeat it.

In short:

- **Name and number.** `k_playbook_task_create` (CLI `k-playbook task create`) creates `<NNN>-<short-name>.md` in one step -- number, frontmatter, the heading `# Task <NNN> — <Title>` and the structure checks are the tool's, the session supplies title, short name and body (task 178); the number is the highest across all `user/*/tasks/` including `done/` plus one, starting at `001`. Only the file name has to be unique, not the number, so a number given as an argument is resolved with one `k_playbook_doc_list` call (`number`, `done: true`, task 181) over the own open and completed tasks -- the `done` field of each hit tells which is which -- and the user chooses when several match.
- **Header.** Frontmatter with `title`, the value in double quotes (a `"` inside as `\"`), so that every YAML reader gets exactly this text, also with `: ` or ` #` -- unquoted, `: ` makes the frontmatter unreadable and ` #` starts a comment that cuts the title off (task 136; the same rule holds for every free-text value, such as `thema` in findings or `anlass` in documentation orders). The briefing and `k_playbook_doc_list` show the title. A document whose frontmatter Kephalaion cannot read is listed with its file name as title and the node's reason under `problem`; a readable title that looks cut off at ` #` gets a hint there. Nothing is repaired. The state is the location (`done/`) and the marker lines in the body, not a frontmatter field.
- **Write path.** The whole document is written once, by `k_playbook_task_create`. Afterwards everything is appended with `k_playbook_doc_append`, changed with `k_playbook_doc_replace` or `k_playbook_doc_set_field`, read with `k_playbook_doc_read` (`section`, `lines`) and searched with `k_playbook_doc_grep` -- each with `since_revision`, the revision of the writer's own last write. A `foreign_change` answer means somebody else wrote in between: the command stops and asks the user who continues. The history of a task is its Kephalaion revisions, not Git.
- **Progress.** A task with `### Etappe N` headings gets one appended line per completed stage, `<!-- k-task-run: etappe <n> erledigt <date> --> <evidence>`; a reset is a line `<!-- k-task-run: fortschritt zurückgesetzt <date> -->`. Per stage the last line after the last reset counts. There is no progress table (task 108).
- **References.** Every reference in a task carries a prefix (`project:/`, `k-playbook:/`, `keph:/`), the intent included; relative links and bare paths are reported, not guessed. What counts as a reference and how an old one is brought up to date is `k-playbook:/instructions/on-demand/verweise-nachziehen.md`; `/k-task-refine` checks it (see [verweisschema.md](verweisschema.md#bringing-references-up-to-date)).
- **Foreign and completed references.** A command that changes, appends to, moves or changes code on the strength of an existing task accepts only references whose user segment is `~` or the own user; a foreign one is rejected up front with `foreign_user`. Reading and searching (the `/k-remediation` comparison over all users) are exempt. A reference under `done/` leads to the same question as a number that only matches a completed task: `/k-task-run` offers to reopen it, `/k-task-refine` asks whether to review it anyway or stop.
- **Replica lag.** The node reads from its replica, which trails the hub. The write operations wait for it; `k_playbook_doc_grep` and `k_playbook_doc_read` do not. A read or count right after one's own write that shows a revision **below** one's own last one is the replica lagging -- wait briefly and read again, a few times; only a higher revision, or one that still differs after the retries, counts as `foreign_change`. The same holds for `not_found` right after one's own `k_playbook_doc_done`/`_reopen` under the new reference. Every append whose arrival is later checked by a heading or marker is counted first, and every `outcome_unknown` has a fixed path per operation (re-read, take the revision from that answer, write again only when nothing arrived). The rule stands once, in `tasks.md`, section "Lesen und schreiben".

The interface has no page for tasks: under Workflows it offers the pages Reviews and Todos, and the card "Tasks" of the overview explains the flow without a counter or a link. Tasks are read and worked on through `/k-task-create`, `/k-task-refine` and `/k-task-run` in the assistant.

## /k-task-create

`/k-task-create [short-name]` creates a task from the current conversation. The command is a hull with the assistant and a text under `k-playbook:/commands/k-task-create.md`.

The command:

- reads `on-demand/tasks.md` first and follows it for location, name, header, structure, intent rule and scope.
- derives the short name from the argument or from the topic of the conversation.
- includes relevant references and special tools in the draft.
- writes the body, renders the draft with `k_playbook_task_create` and `dryRun: true` (violations of the structure come back at once and are worked in before the user sees anything), shows it with the warnings, and only after confirmation creates the task with the same call without `dryRun`; the answer names the `keph:` reference. There is no empty intermediate document and no second write.

Tasks should be written so that `/k-task-run` can execute them without further chat context.

Substantial work is not split into many small tasks, but structured in one document under `## Zu bauen` as `### Etappe N — Titel`. Split only what is substantively separate: different context, meaningful on its own, verifiable on its own. The reason to split solely because of size does not apply because `/k-task-run` tracks progress per stage.

## /k-task-refine

`/k-task-refine [ref-or-path] [nochmal]` hardens tasks or instruction files before execution. Without an argument it takes only the own open tasks from the collection (`k_playbook_doc_list` with `compact`, paged by `next.after` when the answer is `truncated`) that carry no review log yet -- a line starting with `## Review-Log`, the same check `/k-task-run` uses. If there are open tasks but none unreviewed, it says so and ends without writing anything. With an argument it takes a `keph:` reference, a task number or -- as before -- an instruction file or directory on disk; if that target already carries a review log, it asks first whether to review it again, unless the call ends in `nochmal`, `erneut` or `again`. The word alone, without a target, takes all own open tasks, reviewed ones included.

Without a target the main context splits the tasks into groups on every run. Two tasks belong together when one names the other as prerequisite, dependency or shared goal, or when both change the same file; a file a task only reads or names as a model does not count, and in doubt tasks count as related. The relation is transitive. The start overview shows the groups with the reason for each group of more than one task. Each group is reviewed by its own subagent, all in parallel; the subagent is the moderator for its tasks, starts Critic and Editor itself and appends the review logs. It never asks the user: it stops and returns the question with its intermediate state, and the main context asks and starts a new subagent that continues from there. Where a subagent cannot start subagents -- OpenCode v2.3.0 with its default settings, measured -- the main context forms the same groups and reviews them itself one after another. When all groups are done, a lean Critic checks once for overlaps between groups and with the already reviewed open tasks (same file, same contract, a required order); its hits appear in the final summary with a one-line recommendation each, without a further review log and without a question.

The command uses a structured Critic/Editor dialogue:

- Critic and Editor are read-only; they read a task themselves with `k_playbook_doc_read`.
- The moderator is the only writer: edits to a task go through `k_playbook_doc_replace` with `since_revision`, edits to a file on disk through the edit tool.
- The actual state takes precedence after every change.
- Accepted edits and decisions are recorded in the review log, appended with `k_playbook_doc_append`.
- At the end, it checks against the specified `## Intent`, if present.

Every reviewed task receives a `## Review-Log (<date>)`, including one that needed no changes, with the note "keine Änderungen". The log is the only evidence that a review took place; `/k-task-run` uses the heading line to identify whether a task was reviewed.

## /k-task-run

`/k-task-run [ref-or-number] [refine]` executes tasks sequentially. Without an argument it takes the own open tasks from the collection, sorted by number; with an argument a `keph:` reference or a task number. Directories are no longer an argument. With the word `refine` -- alone or after the reference or number -- unreviewed tasks first go through `/k-task-refine` without a question (see [Mode refine](#mode-refine)).

The command:

- never executes tasks in parallel, and never two runs in one project at the same time: before the first change to the working tree it takes the working-tree lock (see [The working-tree lock](#the-working-tree-lock)).
- asks when a task has no review log and therefore has never gone through `/k-task-refine` -- unless `refine` was given and the review succeeded.
- clarifies open questions before delegating to subagents, from the task alone.
- has each task executed and then reviewed by subagents that append their results to the task themselves.
- stops before a task moves to `done/` when its review reports a critical finding.
- moves successfully completed tasks to `done/` with `k_playbook_doc_done`; the intent check is appended to the new reference under `done/`.
- leaves aborted or partially executed tasks open.
- queues the follow-up of the generated documentation when the run changed code (task 124): before releasing a task's snapshot the main context notes the changed file names against it (names only, no diff), and at the end it queues one documentation order per producer in `keph:/queue/docs/` following `k-playbook:/instructions/on-demand/doku-auftraege.md` -- not when there is no generated documentation yet or an order of this session already exists; for `code` an open raw order of the session is extended with the changed paths instead of being left as it is, because the paths decide which documents `/k-docs-code` writes (task 167). The execution subagents do not load the briefing and do not queue anything.

If a task contains `### Etappe` headings, the execution subagent appends a progress line immediately after every completed stage, not only at the end, so progress survives a hard interruption. A later `/k-task-run` reads the lines and offers to resume at the first open stage. The lines are the only source of progress; nothing is inferred from the code or `git log`.

If a task contains `## Ausführungskontext`, `/k-task-run` evaluates `Target repo`, `Base branch`, `Work branch`, and `PR required`, among other values. The branch/dirty-worktree preflight and, where applicable, PR handoff are then part of the flow.

### The working-tree lock

At any time at most one `/k-task-run` changes the working tree of a project; a second one
waits instead of writing in between (task 171). The lock is held and granted by k-playbook
(`k_playbook_lock`, `k-playbook lock`, see [mcp.md](./mcp.md#working-tree-lock)), not by the
assistant: what the tool says holds, even against what a session believes about itself.

- **Taking.** After step 3 -- with `refine` after the review --, before the first change to
  the working tree, the main context calls `acquire` and keeps the token as `LOCK_TOKEN`.
  When the lock is busy it reports the holder and the user's ways out (abort, or free it with
  `k-playbook lock release --force`) and then calls `wait` in a loop without a time limit; a
  waiting run continues by itself once the lock is free. A holder that is certainly orphaned
  is taken over by `acquire` itself. For an uncertain one (the shared OpenCode service, a lock
  taken through the CLI) the user is asked once per holder: wait, free and take over, or abort.
- **Checking the list again.** After taking, also after waiting, the run drops only what
  changed since it collected its list: a task that moved to `done/` or disappeared, or one
  whose marker "ausgeführt" appeared since then. A task it reopened itself and that another
  writer changed meanwhile is a `foreign_change`. An empty list releases the lock and ends.
- **Checking.** `check` with the token before a branch switch, before every snapshot, before
  every delegation to the execution or fix subagent, before the push and after every answer
  of the user. Subagents never take or check the lock; the check before their delegation
  covers their commits. A lost lock (`lost` with the reason: released by the user, taken over
  as orphaned, file missing) stops the run before the next change and asks the user -- wait
  and take again, or abort --, saying that writes may have happened since the last check; a
  running subagent is not interrupted. A session that no longer knows its token (after a
  context summary) asks `status` and then always the user; nothing is taken over or taken
  again silently.
- **Releasing** happens at one fixed point: when the run leaves step 5 (no further task, also
  after a stop or abort), before the intent check and the summary, which only write to the
  collection. By then every task the run executed carries its marker or lies under `done/`,
  so a waiting run that checks its list finds that state.
- **Without the MCP tool** the run takes the lock through `k-playbook lock` in the shell. Such
  a lock knows no long-lived holder and is never treated as orphaned; if the session ends
  without releasing it, the user frees it with `k-playbook lock release --force`.

The lock protects only the project's own working tree: a task whose `Target repo` lies
outside the project is not covered. `/k-task-refine`, the release, `/k-docs-code` and a
briefing line are not part of it yet.

### Mode refine

`/k-task-run refine` and `/k-task-run <ref-or-number> refine` replace the question of step 3:
the unreviewed tasks are reviewed by `/k-task-refine` first, before the lock is taken.

- `/k-task-run refine` works like `/k-task-refine` without an argument: the main context forms
  the groups, one subagent reviews each group, and the overlap Critic runs at the end. Its hints
  on order are shown; the order stays by number, and a hint that contradicts it becomes a
  question.
- `/k-task-run <ref-or-number> refine` reviews the one target like `/k-task-refine <target>`,
  but in a subagent as a group of one, without the overlap Critic. An already reviewed target
  is not reviewed again.
- The main context never moderates itself and sees nothing from Critic or Editor. Only a task
  that comes back `geprüft` goes on without a question. A question of the review or a
  `foreign_change` is asked and continued as in `/k-task-refine`. `keine-subagenten`,
  `abgebrochen`, or a missing or malformed return lets that task fall back to the question of
  step 3, with the reason.
- In OpenCode a subagent cannot start subagents with the default settings, so `refine` there
  always falls back to the question of step 3 and says why. A way of its own for OpenCode is
  follow-up work.
- Afterwards the main context reads the intent and the tools again and names a changed tool
  list; the final summary carries one line per reviewed task and a line on the lock.

### A lean main context

Several tasks can run in one session without the main context growing noticeably with each
one. The main context steers and asks; subagents work and write. Per task it sees no diff, no
file contents apart from the sections of the task it needs, no logs, and no full review -- only
the fixed short returns of the subagents. The execution subagent gets the reference of the task
and its current revision, not the content, and returns status, summary, validation, where it
stopped, blockers and the revision of its last write. The main context reads a task only with
`k_playbook_doc_read` and `section` or `lines`, searches with `k_playbook_doc_grep` and writes
only with `k_playbook_doc_append`; after the execution note it reads no section at all. The
document operations keep the content in the program, which is why they exist (task 104).

### The revision chain

A task has one writer at a time. The main context remembers the revision of its own last
write and passes it to every subagent; the subagent appends with `since_revision` and returns
`Revision: <n>`. Before every handover the main context counts the marker line the subagent
is going to write (`k_playbook_doc_grep`, `count_only`) and remembers count and revision;
afterwards it takes the revision from the return, or -- when the return carries none -- counts
again and accepts the revision from that answer only when the count has grown. A subagent that
gets `foreign_change` stops, writes nothing further and returns
`Foreign-Change: <revision> <updatedBy> <updatedAt>`; the main context then sends the
notification and asks the user who continues. After "weiter" the revision from that answer is
the next `since_revision`.

### Diff and review

With `project.vcs: git` and `git` available (`environment.baseTools.missing` does not name it), every task gets a diff and a code review:

- **Snapshot per task.** Before its first delegation, the main context records a snapshot of
  the working tree in the execution root, including uncommitted and untracked changes of
  earlier tasks. It is a tree object built through a temporary index; the real index and the
  worktree stay untouched. A ref `refs/k-task-run/<task>` keeps it safe from `git gc` until
  the task moves to `done/`. Its ID is appended to the task as a line
  `<!-- k-task-run: snapshot <file> <id> -->`, so a restart after a blocker or a later run
  reuses it. If the ID is missing although the task ran before, or can no longer be resolved,
  a new snapshot is taken and the review covers only the changes since the resumption; the run
  says so and asks before the task moves to `done/`. Files excluded by `.gitignore` are in
  neither the snapshot nor the diff. The task document lives in the collection and is never
  part of the diff.
- **Review subagent.** A separate subagent, not the one that wrote the code, compares the
  state after the task with the snapshot, including newly created files. It follows the
  module `k-playbook:/commands/_task-run/diff-review.md`
  ([`defaults/commands/_task-run/diff-review.md`](../../../defaults/commands/_task-run/diff-review.md)),
  which it reads itself, and appends `### Diff: Geänderte Dateien`, `### Diff: Code-Änderungen`,
  and `### Review: Befunde` to the task with `k_playbook_doc_append` and `since_revision`.
  Its return to the main context carries the counts, one line per critical finding under
  `Kritisch:` and one line per important finding under `Wichtig:` (`Kritisch: keine` and
  `Wichtig: keine` when empty) and one line per note under `Hinweise:` (`Hinweise: keine`
  when empty). A return without the `Wichtig:` block counts as malformed, like any other; a
  missing or malformed `Hinweise:` block does not invalidate the review, the summary then adds
  "Hinweise nicht übermittelt". The summary shows the notes under the task's "Review" line;
  they are never offered as todos and trigger neither the stop prompt nor the fix loop.
  Criteria and the classification into critical, important, and note are defined only in the
  recipe [`reviews/review-code.md`](../../../defaults/reviews/review-code.md), section
  `## Ergebnisform für den Diff eines Tasks`; this page does not repeat them. The recipe is
  the entry `k-playbook:/reviews/review-code.md` under `documents.reviews` of the briefing, so
  a project-owned overlay under `keph:/k-playbook/reviews/` in the project collection takes
  effect. No
  assistant-specific skill is involved; the review behaves the same in Claude Code and
  OpenCode.
- **Notification.** Before every question to the user and at the end of the run, the main
  context sends a desktop notification through `k_playbook_notify` (title
  `<project>: <task> – fertig|Rückfrage: …|abgebrochen`, body what happened, with sound), as
  the module `k-playbook:/commands/_task/fertig-meldung.md`
  ([`defaults/commands/_task/fertig-meldung.md`](../../../defaults/commands/_task/fertig-meldung.md))
  describes; `/k-task-refine` includes the same module. `<task>` is the file stem of the task
  document. A `no_channel` answer, for example in a devcontainer, changes nothing: the
  question and the summary stay in the chat.
- **Switching it off.** Only an empty project-owned `keph:/k-playbook/reviews/review-code.md`
  switches the review off for `/k-task-run`. The diff sections are still written, and the
  final summary shows "Review: abgeschaltet". `review.enabled` and `audit.enabled` only
  affect `/k-review` and `/k-audit`. An overlay that lacks the section for the diff of a task
  stops every task.

Without git there is no diff, no review, and no stop.

### Stop on a critical finding

After the review and before the task moves to `done/`, before the PR handoff, and before the
next task, the run asks when the review reports at least one critical finding:

```text
(a) fix       - a subagent fixes only these findings, then a new review
(b) continue  - the task counts as done, the findings stay documented
(c) stop      - the task stays open, no further tasks
```

A review that aborted, returned no valid result, or wrote no new section counts as critical,
never as "0 critical"; (a) then means running the review again. A review that covers only the
changes since the resumption is asked about the same way, with (b) and (c). With (c) the
task gets the line `**Status nach Review:** angehalten — <reason>` and stays where it is.
An important finding never stops the run (task 170): `/k-task-run` often runs unattended over
several tasks, and every stop holds up the whole run.

### Fixing clear findings automatically

After a valid review with at least one critical or important finding (and a review scope other than "only since the resumption"), `/k-task-run` fixes what is clear before it asks (task 177, module `commands/_task-run/nachbessern.md`, step 4a of 5f). A round is one fix by a general-purpose subagent, never the reviewing one, followed by a new review against the same snapshot and scope. The order for the fix lives only in `ausfuehrung.md`, section "Behebung": only unambiguous findings are fixed; what is not fixed is named with the reason under `Zusammenfassung:`, in the unchanged return format. Every round checks the lock before the fix and before the review and carries `REVISION` forward; `Foreign-Change:` or a lost lock ends the loop and leads into the usual question.

There are at most three rounds. A finding that is still or again present after its fix (same file without line number, same core statement; when in doubt it counts as the same) drops out of the loop and counts as remainder, as does every finding the fixer left alone. A technical failure of the fix ends the loop at once. At the end the last valid review of the loop counts: critical remainder goes to the stop question, which names what was tried; important remainder is collected as todos; fixed findings are not collected. "(a) fix" in the stop question stays as an explicit user choice with exactly one fix run and a new review, never back into the loop. The summary carries a line "Nachbessern: <n> Runden, behoben <n>, offen <n>".

### Findings as todos

Review findings that a run does not fix do not stay hidden in a task under `done/` (task
170). For every task that moves to `done/`, the main context collects finding lines from the
return of the **last** review of this run -- after "(a) fix" that is the new review:

- the lines under `Wichtig:`; after "(b) continue" also those under `Kritisch:`;
- when that return is not valid and the task still moves to `done/` (after "(b) continue"),
  a lookup subagent, the *finding reader*, reads the last `### Review: Befunde` of this run
  -- the one after the marker "ausgeführt" of this run -- and returns the same two blocks;
  the main context collects from that return and still never reads the section itself;
- a task stopped with (c) keeps its findings in the document; nothing is collected.

The final summary shows the collected lines below each task's `Review:` line, critical ones
prefixed with `kritisch:`. After the documentation follow-up and before the final
notification, the run asks once -- with its own notification `Rückfrage: Befunde als Todo`
-- listing all collected findings numbered across the whole run:

```text
Als Todo übernehmen? (alle / keine / alle außer <nummern> / nur <nummern>)
```

`alle` is the default. Each accepted finding becomes one `k_playbook_todo_add`: the text is
the finding in one self-contained sentence with its location, the origin is
`Task <NNN> — <reference under done/>`; the summary names the created IDs in the line
`Todos`. On `outcome_unknown` the run looks the todo up with `k_playbook_todo_list`
(`search`) instead of adding it again. The question also comes when the run stopped or
aborted earlier (5e, a stop, `foreign_change`), for the tasks completed until then; without
collected findings there is no question. An automatic todo per finding was rejected: review
findings are not always right, and unchecked todos fill the briefing with noise.

### Executed but not completed

Together with the success note, the run appends a marker line
`<!-- k-task-run: ausgeführt <file> -->` carrying the task's own file name. A task with
this marker has been executed but not completed: it was stopped at the review question, or a
session ended between the note and the move to `done/`. A later `/k-task-run` recognizes the
marker before reading the task, neither reads nor executes it again, and asks:

- **(a) complete** -- open points are done or accepted; the task gets
  `**Status nach Review:** abgeschlossen vom Nutzer (<date>)` and moves to `done/`. Before
  that line is appended, the finding reader (see "Findings as todos") reads the last
  `### Review: Befunde` after the matching marker "ausgeführt" and returns its `Kritisch:` and
  `Wichtig:` lines. They are collected like after "(b) continue", appear under "Abgeschlossen"
  in the summary below the line `Review: … — letzter Lauf` and go into the todo question.
  Without such a section the line reads `Review: kein Abschnitt im letzten Lauf`; a malformed
  return gives `Review: Befunde nicht gelesen`. The section may predate a fix and name
  something already fixed; the question lets the user deselect it. The PR handoff does not
  run; if the task requires a PR, the run names it as an open step.
- **(b) stop** -- the run ends here, no further tasks.

There is no automatic fix, no new review, and no re-execution; the finding reader only
reads. A task that merely quotes
the marker template carries a placeholder or another file name there and does not count as
executed. All marker lines are listed in `on-demand/tasks.md`, section "Kennzeilen".

Since task 108 two more marker lines exist:

- `<!-- k-task-run: wieder geöffnet <file> <date> -->` -- appended by the main context after
  `k_playbook_doc_reopen`, when a completed task is run again. The marker "ausgeführt" only
  counts after the last such line, so a reopened task is executed again instead of being
  offered for completion.
- `<!-- k-task-run: abgebrochen <file> -->` -- the head of an aborted run (5e), written like
  "ausgeführt" as the first line after `## Ausführung` and the blank line.

A reader that wants the task without the sections of earlier runs cuts at the last
`## Ausführung` line before the first marker "ausgeführt" or "abgebrochen"; without such a
marker at the first `## Ausführung`, without that the whole task is its body.

## Core and modules

`/k-task-run` and `/k-task-refine` are each a core text that describes the normal flow completely, plus modules that are read only when a case occurs or that a subagent reads itself as its order. The core names at every fork the condition, the module and the values handed over; a module is a reference `k-playbook:/commands/_…/<file>.md` read with `k_playbook_doc_read`. A project-owned file of the same path under `.k-playbook/commands/` replaces it. Tests keep the core at 32,000 bytes and every module at 20,000 bytes with includes resolved.

- `commands/_task/` -- shared by both commands: `fertig-meldung.md`, `revisionskette.md` (since_revision, counting, `foreign_change`, the return lines `Revision:` and `Foreign-Change:`), `sub-agenten.md` (subagent type per client, briefing line, read paths), `ziel-aufloesen.md`.
- `commands/_task-run/` -- special cases of the main context (`sperre.md`, `nicht-abgeschlossen.md`, `nachbessern.md`, `stopp-abfrage.md`, `snapshot.md`, `abschluss.md`, `review-starten.md`, `pr.md`, `abbruch.md`, `etappen-fortsetzen.md`, `ausfuehrungskontext.md`, `wieder-oeffnen.md`, `liste-neu-pruefen.md`, `ohne-refine.md`, `refine-modus.md`, `intent-alignment.md`, `doku-nachzug.md`, `befunde-als-todo.md`, `zusammenfassung.md`) and orders that a subagent reads itself (`ausfuehrung.md`, `befund-leser.md`, `intent-check.md`, `diff-review.md`).
- `commands/_task-refine/` -- `dialog.md` (the Critic/Editor dialogue; whoever moderates reads only this, also a group subagent) and `gruppen.md` (forming groups, reviewing them in subagents, the overlap Critic; read by the main context of both commands, with a table of what applies per caller).

The step numbers of `/k-task-run` stay; modules carry sections named by content, and only the core of `/k-task-refine` has step numbers. How the clients follow the module forks is checked by the user after a release.
