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

A task is a document in the project's Kephalaion collection under `keph:/user/<user>/tasks/`; a completed task is moved to `done/` below it. There is no task on disk any more: without a usable Kephalaion binding (`context.kephalaion.ok` not `true`) the task commands stop and say so, instead of falling back to a directory. The shipped always-instruction `300-arbeiten-in-tasks.md` says when work becomes a task; everything about the form of a task -- location, name and number, header, structure, life cycle, marker lines, progress lines, the write path -- stands once, in `k-playbook:/instructions/on-demand/tasks.md`. The four commands and `/k-remediation` refer to it and do not repeat it.

In short:

- **Name and number.** `k_playbook_doc_create` (kind `tasks`) creates `<NNN>-<short-name>.md`; the number is the highest across all `user/*/tasks/` including `done/` plus one, starting at `001`. Only the file name has to be unique, not the number, so a number given as an argument is resolved among the own open tasks first, then `done/`, and the user chooses when several match.
- **Header.** Frontmatter with `title`, the value in double quotes (a `"` inside as `\"`), so that every YAML reader gets exactly this text, also with `: ` or ` #` -- unquoted, `: ` makes the frontmatter unreadable and ` #` starts a comment that cuts the title off (task 136; the same rule holds for every free-text value, such as `thema` in findings or `anlass` in documentation orders). The briefing and `k_playbook_doc_list` show the title. A document whose frontmatter Kephalaion cannot read is listed with its file name as title and the node's reason under `problem`; a readable title that looks cut off at ` #` gets a hint there. Nothing is repaired. The state is the location (`done/`) and the marker lines in the body, not a frontmatter field.
- **Write path.** Only the creator writes the whole document (Kephalaion `write` right after `create`). Afterwards everything is appended with `k_playbook_doc_append`, changed with `k_playbook_doc_replace` or `k_playbook_doc_set_field`, read with `k_playbook_doc_read` (`section`, `lines`) and searched with `k_playbook_doc_grep` -- each with `since_revision`, the revision of the writer's own last write. A `foreign_change` answer means somebody else wrote in between: the command stops and asks the user who continues. The history of a task is its Kephalaion revisions, not Git.
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
- shows the draft first and only then creates the document with `k_playbook_doc_create` and writes it with Kephalaion `write`; the answer names the `keph:` reference.

Tasks should be written so that `/k-task-run` can execute them without further chat context.

Substantial work is not split into many small tasks, but structured in one document under `## Zu bauen` as `### Etappe N — Titel`. Split only what is substantively separate: different context, meaningful on its own, verifiable on its own. The reason to split solely because of size does not apply because `/k-task-run` tracks progress per stage.

## /k-task-refine

`/k-task-refine [ref-or-path] [nochmal]` hardens tasks or instruction files before execution. Without an argument it takes only the own open tasks from the collection (`k_playbook_doc_list`) that carry no review log yet -- a line starting with `## Review-Log`, the same check `/k-task-run` uses. If there are open tasks but none unreviewed, it says so and ends without writing anything. With an argument it takes a `keph:` reference, a task number or -- as before -- an instruction file or directory on disk; if that target already carries a review log, it asks first whether to review it again, unless the call ends in `nochmal`, `erneut` or `again`. The word alone, without a target, takes all own open tasks, reviewed ones included.

Without a target the main context splits the tasks into groups on every run. Two tasks belong together when one names the other as prerequisite, dependency or shared goal, or when both change the same file; a file a task only reads or names as a model does not count, and in doubt tasks count as related. The relation is transitive. The start overview shows the groups with the reason for each group of more than one task. Each group is reviewed by its own subagent, all in parallel; the subagent is the moderator for its tasks, starts Critic and Editor itself and appends the review logs. It never asks the user: it stops and returns the question with its intermediate state, and the main context asks and starts a new subagent that continues from there. Where a subagent cannot start subagents -- OpenCode v2.3.0 with its default settings, measured -- the main context forms the same groups and reviews them itself one after another. When all groups are done, a lean Critic checks once for overlaps between groups and with the already reviewed open tasks (same file, same contract, a required order); its hits appear in the final summary with a one-line recommendation each, without a further review log and without a question.

The command uses a structured Critic/Editor dialogue:

- Critic and Editor are read-only; they read a task themselves with `k_playbook_doc_read`.
- The moderator is the only writer: edits to a task go through `k_playbook_doc_replace` with `since_revision`, edits to a file on disk through the edit tool.
- The actual state takes precedence after every change.
- Accepted edits and decisions are recorded in the review log, appended with `k_playbook_doc_append`.
- At the end, it checks against the specified `## Intent`, if present.

Every reviewed task receives a `## Review-Log (<date>)`, including one that needed no changes, with the note "keine Änderungen". The log is the only evidence that a review took place; `/k-task-run` uses the heading line to identify whether a task was reviewed.

## /k-task-run

`/k-task-run [ref-or-number]` executes tasks sequentially. Without an argument it takes the own open tasks from the collection, sorted by number; with an argument a `keph:` reference or a task number. Directories are no longer an argument.

The command:

- never executes tasks in parallel.
- asks when a task has no review log and therefore has never gone through `/k-task-refine`.
- clarifies open questions before delegating to subagents, from the task alone.
- has each task executed and then reviewed by subagents that append their results to the task themselves.
- stops before a task moves to `done/` when its review reports a critical finding.
- moves successfully completed tasks to `done/` with `k_playbook_doc_done`; the intent check is appended to the new reference under `done/`.
- leaves aborted or partially executed tasks open.
- queues the follow-up of the generated documentation when the run changed code (task 124): before releasing a task's snapshot the main context notes the changed file names against it (names only, no diff), and at the end it queues one documentation order per producer in `keph:/queue/docs/` following `k-playbook:/instructions/on-demand/doku-auftraege.md` -- not when there is no generated documentation yet or an order of this session already exists; for `code` an open raw order of the session is extended with the changed paths instead of being left as it is, because the paths decide which documents `/k-docs-code` writes (task 167). The execution subagents do not load the briefing and do not queue anything.

If a task contains `### Etappe` headings, the execution subagent appends a progress line immediately after every completed stage, not only at the end, so progress survives a hard interruption. A later `/k-task-run` reads the lines and offers to resume at the first open stage. The lines are the only source of progress; nothing is inferred from the code or `git log`.

If a task contains `## Ausführungskontext`, `/k-task-run` evaluates `Target repo`, `Base branch`, `Work branch`, and `PR required`, among other values. The branch/dirty-worktree preflight and, where applicable, PR handoff are then part of the flow.

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

With `context.project.vcs: git` and `git` available, every task gets a diff and a code review:

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
  `Wichtig: keine` when empty); notes stay a number. A return without the `Wichtig:` block
  counts as malformed, like any other.
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
