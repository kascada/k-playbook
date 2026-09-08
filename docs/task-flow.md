# Task flow

The task flow is the standard path for planned work that should not be completed directly in a short chat step.

## Standard flow

```text
/k-task-create
/k-task-refine
/k-task-run
```

Tasks can arise directly from the conversation or be created by `/k-remediation`. In both cases: review task files first, then execute them.

Tasks can also be read in the interface: the **Workflows** section lists open tasks together with their number; clicking shows the task below it. Completed tasks appear in a collapsed section after that. This is read-only; tasks are created and executed through the commands.

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
- clarifies open questions before delegating to subagents.
- appends an execution note.
- moves successfully completed tasks to `done/`.
- leaves aborted or partially executed tasks open.

If a task file contains `### Stage` headings, `/k-task-run` maintains a `## Progress` table in it. The subagent enters every stage immediately after it is completed, not only at the end, so progress survives a hard interruption. A later `/k-task-run` reads the table and offers to resume at the first open stage. The table is the only source of progress; nothing is inferred from the code or `git log`.

If a task file contains `## Execution context`, `/k-task-run` evaluates `Target repo`, `Base branch`, `Work branch`, and `PR required`, among other values. The branch/dirty-worktree preflight and, where applicable, PR handoff are then part of the flow.

## Remediation tasks

Tasks created by `/k-remediation` are ordinary task-flow inputs. The following are particularly important:

- Finding IDs and sources must appear in the task.
- Branch/PR requirements from the remediation policy must appear in the execution context.
- `/k-task-refine` runs before implementation.
- Implementation then runs through `/k-task-run`.
