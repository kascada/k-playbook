# Task flow

The task flow is the standard path for planned work that should not be completed directly in a short chat step.

## Standard flow

```text
/k-task-create
/k-task-refine
/k-task-run
```

Tasks can arise directly from the conversation or be created by `/k-remediation`. In both cases: review task files first, then execute them.

Tasks can also be read in the interface, on the **Tasks** page of the Workflows section (`/workflows/tasks`): it lists open tasks together with their number; clicking shows the task below it. Completed tasks appear in a collapsed section after that. This is read-only; tasks are created and executed through the commands.

## /k-task-create

`/k-task-create [short-name]` creates a structured task file under `k-playbook-local/tasks/`.

The command:

- derives the location from the position of `K-PLAYBOOK.yaml`.
- determines the next available number from open tasks and `done/`.
- creates a filename such as `014-audiosocket-server.md`.
- includes relevant references and special tools in the task file.
- shows the draft first and saves only after confirmation.

Tasks should be written so that `/k-task-run` can execute them without further chat context.

Substantial work is not split into many small files, but structured in one file under `## To build` as `### Stage N - Title`. Split only what is substantively separate: different context, meaningful on its own, verifiable on its own. The reason to split solely because of size does not apply because `/k-task-run` tracks progress.

## /k-task-refine

`/k-task-refine [path]` hardens task or instruction files before execution.

The command uses a structured Critic/Editor dialogue:

- Critic and Editor are read-only.
- The moderator is the only writer.
- The actual file state takes precedence after every change.
- Accepted edits and decisions are recorded in the review log.
- At the end, it checks against the specified `## Intent`, if present.

Every reviewed file receives a `## Review log`, including one that needed no changes, with the note "no changes". The log is the only evidence that a review took place; `/k-task-run` uses it to identify whether a task was reviewed.

Without an argument, the command reviews the open task files under `k-playbook-local/tasks/`.

## /k-task-run

`/k-task-run [file-or-directory]` executes task files sequentially.

The command:

- uses `k-playbook-local/tasks/` without an argument.
- sorts tasks by numeric prefix.
- never executes tasks in parallel.
- asks when a task file has no review log and therefore has never gone through `/k-task-refine`.
- clarifies open questions before delegating to subagents, from the task file alone.
- has each task executed and then reviewed by subagents that write into the task file themselves.
- stops before a task moves to `done/` when its review reports a critical finding.
- moves successfully completed tasks to `done/`.
- leaves aborted or partially executed tasks open.

If a task file contains `### Stage` headings, `/k-task-run` maintains a `## Progress` table in it. The subagent enters every stage immediately after it is completed, not only at the end, so progress survives a hard interruption. A later `/k-task-run` reads the table and offers to resume at the first open stage. The table is the only source of progress; nothing is inferred from the code or `git log`.

If a task file contains `## Execution context`, `/k-task-run` evaluates `Target repo`, `Base branch`, `Work branch`, and `PR required`, among other values. The branch/dirty-worktree preflight and, where applicable, PR handoff are then part of the flow.

### A lean main context

Several tasks can run in one session without the main context growing noticeably with each
one. The main context steers and asks; subagents work and write. Per task it sees no diff, no
file contents apart from the task file itself, no logs, and no full review — only the fixed
short returns of the subagents. The execution subagent gets the path of the task file, not its
content, and returns status, summary, validation, where it stopped, and blockers. The main
context reads task files only per shell with `sed -n` and writes to them only by appending per
shell; after the execution note it no longer reads the file at all. Reading with the
assistant's read tool or with `cat` would make Claude Code report later changes by the
subagents, including the changed lines, back into the main context.

### Diff and review

With `project.vcs: git` and `git` available, every task gets a diff and a code review:

- **Snapshot per task.** Before its first delegation, the main context records a snapshot of
  the working tree in the execution root, including uncommitted and untracked changes of
  earlier tasks. It is a tree object built through a temporary index; the real index and the
  worktree stay untouched. A ref `refs/k-task-run/<task>` keeps it safe from `git gc` until
  the task moves to `done/`. Its ID is appended to the task file as a line
  `<!-- k-task-run: snapshot <file> <id> -->`, so a restart after a blocker or a later run
  reuses it. If the ID is missing although the task ran before, or can no longer be resolved,
  a new snapshot is taken and the review covers only the changes since the resumption; the run
  says so and asks before the task moves to `done/`. Files excluded by `.gitignore` are in
  neither the snapshot nor the diff.
- **Review subagent.** A separate subagent, not the one that wrote the code, compares the
  state after the task with the snapshot, including newly created files. It follows the
  module [`commands/_task-run/diff-review.md`](../commands/_task-run/diff-review.md) and
  appends `### Diff: Geänderte Dateien`, `### Diff: Code-Änderungen`, and
  `### Review: Befunde` to `## Ausführung`. Criteria and the classification into critical,
  important, and note are defined only in the recipe
  [`reviews/review-code.md`](../reviews/review-code.md), section
  `## Ergebnisform für den Diff eines Tasks`; this page does not repeat them. The recipe is
  the catalog entry with key `code`, so a project-owned overlay under
  `k-playbook-local/reviews/` takes effect. No assistant-specific skill is involved; the
  review behaves the same in Claude Code and OpenCode.
- **Switching it off.** Only an empty project-owned `k-playbook-local/reviews/review-code.md`
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
task file gets the line `**Status nach Review:** angehalten — <reason>` and stays where it is.

### Executed but not completed

Together with the success note, the run appends a marker line
`<!-- k-task-run: ausgeführt <file> -->` carrying the task's own file name. A task file with
this marker has been executed but not completed: it was stopped at the review question, or a
session ended between the note and the move to `done/`. A later `/k-task-run` recognizes the
marker before reading the file, neither reads nor executes the task again, and asks:

- **(a) complete** — open points are done or accepted; the file gets
  `**Status nach Review:** abgeschlossen vom Nutzer (<date>)` and moves to `done/`. The PR
  handoff does not run; if the task requires a PR, the run names it as an open step.
- **(b) stop** — the run ends here, no further tasks.

There is no automatic fix, no new review, and no re-execution. A task file that merely quotes
the marker template carries a placeholder or another file name there and does not count as
executed.
