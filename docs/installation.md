# Installation

k-playbook is cloned into the project it is meant to support. There is no central installation and no fixed host path; each project has its own.

```bash
cd /path/to/project
git clone git@github.com:kascada/k-playbook.git
make -C k-playbook install
k-playbook
```

The same bootstrap also works directly without `make`:

```bash
k-playbook/bin/install
```

The target directory must be named `k-playbook`: commands and skills address it by that name. Without a destination argument, the name derives from the repository name and is therefore correct automatically; you need a custom argument only when cloning a fork or mirror under another name. It must then be `k-playbook`.

**Go is not required.** `bin/install` downloads the release binary suitable for the platform and installs it to `~/.local/bin/k-playbook`. On macOS and in the DevContainer, the installer runs in its own environment and therefore installs the matching macOS or Linux binary.

**Installation requires network access.** The binaries are not in the clone but are release assets. `bin/install` downloads exactly the matching asset and verifies it against the shipped `SHA256SUMS`.

**`~/.local/bin` must be in PATH**: this is a requirement, not an aside. k-playbook is called exclusively by its name; without the PATH entry, it would be installed but not discoverable by anyone. If it is missing, the bootstrap stops before downloading anything and prints the line for the shell profile:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

On Linux, the entry usually already exists. On macOS, it does **not**: `/etc/paths` does not include it and `path_helper` does not add it. The line belongs in `~/.zprofile` (zsh) or `~/.bashrc` (bash); then open a new shell and call the bootstrap again.

One special case exists on the first run on a fresh host or in a DevContainer: Debian and Ubuntu add `~/.local/bin` in `~/.profile` only when the directory already exists at login. The bootstrap therefore creates it **before** checking PATH and, when stopping, says that the profile does not need changing: log in again (or run `. ~/.profile`) and call the bootstrap again.

**Host and DevContainer with a shared home.** `~/.local/bin/k-playbook` is a real file for each platform. If both environments share the same `$HOME`, they overwrite each other there. `bin/install` detects a binary for another platform and reports during replacement that the other environment also needs to run the bootstrap again. Calling the binary directly when it belongs to the wrong platform is not handled: the shell then reports `cannot execute binary file`. Separate homes are the clean state.

## The four steps

The final call opens the interface in the browser. It guides you through four steps and writes each one only after confirmation.

### 1. Create configuration

On its first run, the interface does not yet find `K-PLAYBOOK.yaml`: after a fresh clone, it cannot exist. Rather than guessing, it proposes a location and lets you confirm it. Candidates in this order:

1. the Git repository in which the command is called,
2. the location derived from the binary's position,
3. the working directory.

It also proposes where the project repository is located, either the project root itself or a sibling subdirectory, for example when code is checked out alongside the playbook. The result:

```text
project/
├── K-PLAYBOOK.yaml     the anchor; its location determines the project root
└── k-playbook/         the installation
```

An existing `K-PLAYBOOK.yaml` is never overwritten. Its format is described in [`k-playbook-format.md`](./k-playbook-format.md).

### Create configuration in the terminal

When an anchor of a parent project masks initial setup in the interface, an anchor can be created directly without searching:

```bash
k-playbook config create
```

The command writes to the current directory and then reports the exact path, detected repository, and version control. Optionally, another project directory can be given; if the repository is not at its root, `--repo-root` sets its relative path:

```bash
k-playbook config create --repo-root app /path/to/project
```

An existing `K-PLAYBOOK.yaml` remains unchanged through this path as well.

### 2. Create project-owned structure

Alongside it, `k-playbook-local/` is created with everything owned by the project:

```text
k-playbook-local/
├── rules/         overlay for k-playbook/rules/
├── reviews/       overlay for k-playbook/reviews/
├── checks/        overlay for k-playbook/checks/
├── commands/      overlay for k-playbook/commands/
├── skills/        overlay for k-playbook/skills/
├── results/       everything reviews generate; see below
├── data/          machine files owned by k-playbook, versioned; holds todos.json
├── cache/         derived content this machine can rebuild at any time; see below
├── docs/          project knowledge for AI sessions, separated by origin
│   └── manual/    manually maintained documentation; no command writes here
├── guidelines/
├── tasks/done/
├── priv/          private notes; see below
├── material/      raw material as a source for docs; see below
├── k-playbook.md  project-owned instruction layer
└── version-sources.yaml   version sources for the version inventory, maintained manually
```

The generated docs origins `docs/code/`, `docs/libs/`, and `docs/versions/` are not shown there: they arise on the first run of their generator. `/k-docs-extract` writes to `knowledge/extracted/` in the knowledge store instead, created by its first write; raw material for it goes into `inbox/`, and `material/` is still read as the previous location until step 5 of the switch (see [knowledge-layout.md](./knowledge-layout.md#transitional-reads)).

Every directory contains a `README.md` stating its purpose, also because Git does not store empty directories and they would otherwise be missing after cloning the project. Existing files remain untouched, including READMEs with their own content.

`k-playbook-local/` belongs in the project's repository and is committed, except for the **contents** of four directories for which the project makes that decision: `results/`, `cache/`, `priv/`, and `material/`. For `priv/` and `material/`, k-playbook still does not write a `.gitignore` itself and does not decide what a project versions. `data/` is never among them: it holds machine files that belong to the project's state, above all `todos.json`.

`results/` and `cache/` are the exception: both are created as private on initial creation. Everything under `cache/` is derived from the project and can be rebuilt at any time, so it does not belong in the repository — it would go stale there unnoticed. A review is reproducible from the code, but its result is a state from this machine, and the raw output from a secret scanner does not belong in the repository anyway. This remains switchable, and k-playbook does not undo a switch: the managed `.gitignore` is created only when the directory is first created, not on every run. Existing projects that have previously versioned `results/` therefore notice nothing from the update.

This choice is visible and switchable in the **Local settings** section of the interface. For each directory, it uses `git check-ignore` to determine whether the content is actually excluded and identifies the repository to which the statement applies. If `k-playbook-local/` contains a repository of its own, it applies to that one. One of four states is shown:

| State | Meaning |
|---|---|
| private | the content stays out |
| versioned | no rule; the content is included in the repository |
| partially private | a rule applies, but files are in the repository; the rule applies only to new files |
| private only after the next commit | files were removed from the index but have not yet been committed |

The last two look private but are not; they are therefore shown as a warning and name the affected files. Switching to private creates `.gitignore` and removes already versioned files from the index with `git rm --cached`; this takes effect only with the next commit, and anything already pushed remains in history.

If the ignore rule comes from elsewhere, the `.gitignore` at the project root, `.git/info/exclude`, or global configuration, or if the file in the directory has its own content, nothing is written: the section then shows the state and identifies the source. Manually, the path continues to be a `.gitignore` in the relevant directory; the respective `README.md` names the content.

### 3. Register the MCP server

The **k-playbook MCP** section registers the shipped MCP server with the three assistants. This gives an assistant the resolved working state as a tool instead of making it retrieve it through the command line:

```text
project/
├── .mcp.json          Claude Code:  mcpServers -> k-playbook
├── .cursor/mcp.json   Cursor:       same schema
└── opencode.json      OpenCode:     mcp -> k-playbook
```

It registers `k-playbook mcp`, resolved when written to the **absolute path** of the installed binary. Clients launched from the Dock or Finder do not inherit the shell PATH; a bare command name would be dead there. To commit the entry, enter the bare name `k-playbook` manually: automatic correction does not change it, and when it replaces an obsolete wrapper entry in a tracked file, it writes the bare name itself. A client launched from the Dock or Finder then needs the absolute path, entered manually -- *Set up* does not replace an accepted form. Both are described in [`mcp.md`](./mcp.md#why-the-entry-is-an-absolute-path). One condition always applies: the entry works only when the assistant is opened in the project root, where `K-PLAYBOOK.yaml` is located.

The three files belong to the project. Exactly the `k-playbook` key is touched; foreign entries remain. Registration is complete only after restarting the assistant; Claude Code asks for approval once.

Everything else, the two schemas, handling foreign values, manual removal, and the `/mcp` page with the tools actually offered, is described in [`mcp.md`](./mcp.md).

### 4. Link assistants

Links are created for Claude Code, OpenCode, and Cursor:

```text
project/
├── AGENTS.md             instructions, one source for all assistants
├── CLAUDE.md             include file with the line @AGENTS.md; the direction is fixed
├── .claude/
│   ├── commands/         one symlink per command
│   └── skills/           one symlink per skill; OpenCode reads here too
├── .opencode/
│   └── commands/
└── .cursor/
    └── commands/
```

The four targets are **actual directories with individual symlinks**, not directory symlinks. A directory symlink points to exactly one source, so it would provide either only the installation or only `k-playbook-local/`. Each link points to the version that applies under the overlay rule:

```text
.claude/commands/
  k-todo.md    -> ../../k-playbook/commands/k-todo.md          shipped
  k-review.md  -> ../../k-playbook-local/commands/k-review.md  project-owned, replaces
  k-own.md     -> ../../k-playbook-local/commands/k-own.md     project-owned only
```

The interface compares this expected state with what is actually registered and reports deviations by name: what is missing, what points to the wrong source, what is orphaned, and what belongs to the project and therefore remains. It synchronizes them at the press of a button. An actual file that someone placed there themselves always takes precedence and is never replaced.

Skills appear only once under `.claude/skills`: OpenCode searches this directory too, while Cursor has no skill concept. `CLAUDE.md` is an include file with the import line `@AGENTS.md`, because Claude Code reads only `CLAUDE.md` while OpenCode and Cursor prefer `AGENTS.md`. This leaves exactly one actual instruction file, and Claude Code loads it at startup through the import. Setup writes a short stub: above the import line, a note that project rules belong in `AGENTS.md`, then `@AGENTS.md` alone on a line.

`AGENTS.md` also receives a brief **prompt**: a block that refers to `k-playbook context`. If the file is missing, it is created; if it exists, the block is appended and existing content is not touched. A `<!-- k-playbook:anstoss -->` marker prevents a second run from appending it again.

### An existing CLAUDE.md

The direction is fixed: `CLAUDE.md` includes `AGENTS.md`, never the other way around. So that a second, divergent instruction file does not arise alongside it, setup first classifies the `CLAUDE.md`/`AGENTS.md` pair and resolves what can be resolved:

| Initial situation | What happens |
|---|---|
| `CLAUDE.md` contains the `@AGENTS.md` line, `AGENTS.md` is an actual file | the **expected state**, nothing to do. Content beside the import line belongs to the project and remains untouched |
| `CLAUDE.md` is still a symlink to `AGENTS.md` | the older form: the symlink is **replaced without loss**; the content is already in `AGENTS.md`, including on the read path, see below |
| only an actual `CLAUDE.md` without an import line | it is **renamed** to `AGENTS.md`, the prompt is appended to the preserved content, and `CLAUDE.md` is created again as an include file |
| only the include file, `AGENTS.md` missing | it remains; `AGENTS.md` is created from the template. Renaming it would create an `AGENTS.md` that imports itself |
| `AGENTS.md` is a symlink to `CLAUDE.md` | the reversed direction is resolved: remove the symlink; rename an actual `CLAUDE.md`, leave an include file in place; then create the include file again or create `AGENTS.md` from the template |
| `AGENTS.md` is a dead symlink | it is removed so the file is not written to its target |
| both are actual files, `CLAUDE.md` has no effective import line | **conflict**: the project decides whether the content applies to all assistants or only Claude Code: either move the content to `AGENTS.md` and reduce `CLAUDE.md` to the `@AGENTS.md` line, or put `@AGENTS.md` before the existing content and leave it there. Neither happens automatically |
| `CLAUDE.md` deliberately points to another target | **conflict**: the project's link remains, otherwise the instructions read there would immediately become ineffective |
| `AGENTS.md` deliberately points to another target | a project decision, not an error: the include file works through the link, and that is where the prompt arrives. If `CLAUDE.md` also has its own content without an import line, it is a **conflict** |
| `AGENTS.md` is ignored by Git | **conflict**: otherwise its content would silently fall out of version control; remove the ignore rule and set up again |

The import line is **effective** only outside backticks and code blocks; Claude Code ignores it there during import parsing. A `CLAUDE.md` that mentions `@AGENTS.md` only in backticks or a code block is therefore treated as a file without an import and becomes a conflict rather than the expected state.

During a conflict, nothing is moved, deleted, backed up, and no `AGENTS.md` is created. This is not cosmetic: while it persists, Claude Code sees nothing from setup because it reads only `CLAUDE.md`. The assistants card reports the state as `Conflict` and details the resolution.

The same process runs during **updates**. A project that has only an actual `CLAUDE.md` is therefore set up that way too, and a project without any `AGENTS.md` gets it for the first time.

**Existing projects** migrate themselves. The first `k-playbook context`, or the assistants section of the interface, replaces the old symlink with the include file and reports this in `links.note`. Afterwards, `git status` shows a one-time modified `CLAUDE.md` with a changed mode (symlink -> regular file, `120000` -> `100644`); this change must be committed. It is the one place where a read-only path changes a versioned file in the project root, and it is named as such. No include to nowhere: as long as `AGENTS.md` is absent, an old symlink waits for its target and is replaced only when it exists. If the include file exists while `AGENTS.md` is missing, the detail text identifies the import to nowhere: Claude Code loads nothing from it until **Setup** creates `AGENTS.md`.

**The cost of separation.** Without a symlink, there are two files that can diverge: content Claude Code writes to `CLAUDE.md` through `/memory` or `#` does not reach OpenCode and Cursor, and the check does not report project content beside the include as a conflict. This is deliberate: reversing the direction would mean treating every piece of content beside the include as a conflict. As a counterweight, the stub above the import line says that project rules belong in `AGENTS.md`.

**What Claude Code must support.** `@` imports in `CLAUDE.md` have existed since Claude Code **0.2.107** (CHANGELOG entry "CLAUDE.md files can now import other files"); an older version silently loads nothing from the stub. Import depth is limited: according to the documentation, Claude Code follows imports up to **four levels** deep ("Imported files can recursively import other files, with a maximum depth of four hops", code.claude.com, page "How Claude remembers your project", as of 2026-09-05). The stub consumes one level, so an `AGENTS.md` that itself imports with `@` has one level less than the former symlink.

Anything else an assistant should read is not in `AGENTS.md`, but in `k-playbook.md`, once per layer:

| File | Applies to | On update |
|---|---|---|
| `k-playbook/k-playbook.md` | every project that uses k-playbook | is replaced |
| `k-playbook-local/k-playbook.md` | this project only | remains |

They are read in this order; the project-owned layer supplements or overrides the shipped one.

Linking is project-local. Nothing is written to `~/.config/opencode/` or `~/.claude/`. This allows one machine to have several projects with different k-playbook versions without them overwriting each other.

**Legacy artifacts are removed.** On machines with an installation from the old model, host-global symlinks still exist under `~/.claude/commands`, `~/.claude/skills`, and `~/.config/opencode/command`, along with a `skills.paths` entry in the OpenCode user configuration. They apply to every project, so an assistant would additionally see commands from another version. `k-playbook` removes them on every start, but only items demonstrably belonging to k-playbook. If it removes something, it reports this in the terminal; otherwise it stays silent.

After changing commands or skills, restart the respective assistant: both discover them at startup.

## Browser on startup

At interface startup, the URL is printed in the terminal and the browser opens. The server behind it continues as a background service per project: the call returns as soon as the browser is open, and a second call in the same project only opens another window on the same server. It is stopped through `Stop service` in the interface or `k-playbook stop`; without any request, it stops itself after 60 minutes. The program that opens the browser is selected in this order:

1. **`$BROWSER`**, if set. The freedesktop convention: a `:`-separated list of commands in which `%s` stands for the URL. If the placeholder is absent, the URL is appended.
2. Otherwise, the usual platform candidates: `open` on macOS, otherwise `wslview`, `xdg-open`, `gio open`, and others until one starts.

**In a container, only `$BROWSER` counts.** Every guessed candidate would run in the container rather than on the user's machine; worse, `x-www-browser` and `sensible-browser` in slim images often point to a terminal browser, which then takes over the terminal. An explicitly set `$BROWSER` knows better: it points to a helper that passes the URL through to the host.

The VS Code DevContainer sets this up automatically: the variable points to a script that calls `code --openExternal` and therefore opens the browser on the host. VS Code also forwards the port. It is the same path by which `gh auth login` opens its browser.

If `$BROWSER` is not set in the container, the existing behavior remains: the terminal names the detected container marker and the URL to enter manually. To add the helper, set the variable yourself:

```bash
export BROWSER=/path/to/helper.sh   # receives the URL as an argument
```

## Reviews and tasks

The **Workflows** section brings together the work queues, one page per kind: tasks from `k-playbook-local/tasks/`, review runs from `k-playbook-local/results/`, and todos from `k-playbook-local/data/todos.json`. The three pages are listed under Workflows in the switcher on the left, on every page — the way to the tasks does not lead through the overview.

`/workflows` itself is the overview: it explains what the three kinds are, when to use each, and how much each currently holds. From there, one link per kind leads to its page.

**Tasks** (`/workflows/tasks`) lists the open tasks in numeric order; clicking displays the task as Markdown below the list. Completed tasks from `tasks/done/` appear in a collapsed section, with the highest number first, and can be read the same way. On the right of each task row, it shows whether it has already gone through `/k-task-refine`, with a date if the review log gives one. "without task refine" is not an error, but it is why `/k-task-run` asks before execution.

**Reviews** (`/workflows/reviews`) lists the previous runs with their state and entry count. **Todos** (`/workflows/todos`) lists the open items, with the checked-off ones in a collapsed section below.

Each of the four pages opens with a short help block. Its head carries a small link into the shipped documentation — the overview and the todos to `commands.md`, the tasks to `task-flow.md`, the reviews to `code-review.md` and `review-runs.md`. It opens the file in the Docs section, with the index and its cross-references; the address behind it is `/docs?file=<file>`, which any page can use.

This section is read-only. Tasks are created and executed through `/k-task-create` and `/k-task-run` in the assistant.

## Project knowledge

The **Knowledge** section, above Docs in the switcher, shows where knowledge flows in this project: from the input, through the zones `inbox/`, `queue/` and `knowledge/` and the write tools of the MCP server, into the index in the Go process and back to the AI. Vectors and a database of their own are deferred; the concept and the current status are in `knowledge-gate.md`. For now it is one file, `knowledge-storage.md` from the shipped documentation, rendered with its diagram; a listing of the stored entries will follow below it.

Docs and Knowledge are deliberately separate: Docs is the reference work of the installation, Knowledge is what accumulates in the project.

## Read documentation

The **Docs** section shows all Markdown files from `k-playbook/docs`, the same documentation you are reading now, at the version installed in the project. The index is on the left in the menu and the open file on the right; without a selection, it shows `README.md`. Links in the text lead to the next file, and anchors jump within the open file.

Another page can request a specific file: `/docs?file=task-flow.md`, optionally followed by an anchor. That is how the help links of the Workflows pages work. If the requested file is not in the index, the answer says so rather than falling back to `README.md` — the installation can carry a different state than the binary that wrote the link.

Mermaid diagrams are rendered if the machine has network access: the library is loaded when needed. Without network access, the diagram source remains visible and the text is still fully readable.

## Inspect what applies

At the bottom is the **Resolved context** section. When expanded, it shows what a command sees: resolved paths, instruction files in read order, effective catalogs for rules, reviews, and checks with their origin, shipped, project-owned, or replaced, plus guidelines. Disabled entries are included so their existence remains visible.

It provides the same information as `k-playbook context`, only presented readably.

The section loads only when expanded and changes nothing.

An assistant can receive the same information as a tool instead of retrieving it from the command line. That is what the MCP server is for; it is configured in the **k-playbook MCP** section, and the `/mcp` page shows the registration state along with the tools the server actually offers. Everything about it is in [`mcp.md`](./mcp.md).

## Update

The convenient path is the interface. After startup, it checks with `git ls-remote` whether the installation is behind the remote and updates it at the press of a button with `git pull --ff-only`. It briefly makes `k-playbook/` writable and then sets it read-only again. Deliberately `ls-remote` rather than `fetch`: the check runs without being requested and must not touch repository state. Deliberately `--ff-only`: a merge in the clone would create local history that nobody maintains.

Manually, it works the same way:

```bash
cd /path/to/project
make -C k-playbook installer-update
```

The Make target corresponds to `chmod -R u+w k-playbook && git -C k-playbook fetch origin && git -C k-playbook reset --hard origin/main && git -C k-playbook clean -fd && chmod -R a-w k-playbook` and also locks the installation again if the pull fails. The hard reset is deliberate: by contract, `k-playbook/` has no local changes; everything in it may arise only through pulling. In the development repository, `make installer-update` additionally works because the installation clone is under `./k-playbook/`.

**The Make path updates the clone, not the adjacent project.** It is a pure Git chain and deliberately runs without Go and without the installed binary. That is its purpose: it must keep working even when nothing else works in the project. It therefore does not reach things in the project root but not in the clone, the MCP registration and the prompt block in `AGENTS.md`.

This is caught up on the **next call to `k-playbook`**. Startup is the second, general fallback path: it corrects stale MCP registration itself and reports what it did. This applies to every path that bypasses the interface update button, including a manual `git pull`. Therefore, after a manual update, call once:

```bash
k-playbook
```

This is not an additional everyday step, but the same call with which a session starts anyway.

`k-playbook/` contains no project-owned content and is therefore completely replaceable, including by `rm -rf` and a new clone. `K-PLAYBOOK.yaml` and `k-playbook-local/` sit alongside it and remain untouched.

At every interface startup, an existing installation is also set read-only. This is only a local protection layer against accidental writes; the update deliberately and temporarily lifts it.

**If local work was nevertheless done there, the interface reports it and does not update.** The `Installation` section appears only in this case, names the affected files, and gives the reset command; it does not run it automatically. The reason for the check is that the mistake would otherwise stay hidden: if a locally modified file does not also change upstream, `git pull` completes successfully and leaves it in place, so the change survives every update without ever being noticed.

If `VERSION` changed and the running binary does not yet have that version, the new version requires another binary. The service then stops itself after responding. If the binary already has it, the usual case in the development repository where `make dev-install` installs it before the clone, the service continues. Install the new binary explicitly through the bootstrap, `make -C k-playbook install`, or without make `k-playbook/bin/install`; the update path does not itself download or replace a host binary. Afterwards, `k-playbook` starts the new version. After a manual `git pull` or `make -C k-playbook installer-update`, an old service initially keeps running; it is detected and replaced only when the next call to `k-playbook` comes from a newly installed binary. The binary file itself is compared, not only its version, so a newly built binary with the same `VERSION` also replaces the old service. If only commands, rules, or recipes are new, `VERSION` does not change: the service continues, `Reload` in the interface loads the version, and restarting the assistant is enough.

**The transition window when changing to direct installation.** A project still configured under the retired wrapper model needs these steps in this order, with a window between them where nothing works:

1. **Update the clone first.** Before that, `k-playbook/bin/install` does not exist in this project at all; the bootstrap is present only after the update.
2. **Then run the bootstrap**, once per host and once per DevContainer: `make -C k-playbook install`, or without make `k-playbook/bin/install`.

Between the two steps, commands, the prompt block in `AGENTS.md`, and the MCP registration still point to `k-playbook/bin/k-playbook`, the file removed by the update. In this window, no command works, nor is there a replacement invocation: the two automatic correction paths run in the installed binary and therefore require the bootstrap. It is the one remaining manual step; after it, the next call to `k-playbook` catches up registration and the prompt block itself.

**Linking catches itself up.** Because commands and skills are linked individually, a newly shipped command does not arrive by itself. It is therefore caught up on the read path, not only at the press of a button: the assistants section synchronizes registration when displayed and reports what changed (`Linking synchronized: 3 added, 1 removed.`), and `k-playbook context` does the same, the call already made at the beginning of every assistant session. How the installation reached its version is irrelevant: through the interface, manual `git pull`, or a Makefile target. What cannot resolve itself, an actual project file in the way or a conflict in `CLAUDE.md`, remains and continues to appear in the assistants section.

For the new commands to reach the assistant, restart it afterwards: Claude Code, OpenCode, and Cursor discover them at startup.

### Existing projects: two things are added

Anyone updating a project from a version through 0.4 finds two things after the update. One click handles both; nothing is deleted or overwritten:

| Where | What the interface reports | What to do |
|---|---|---|
| Project-owned structure | `Missing entries: commands, skills` | **Create**: the two overlay directories are created with their README |
| Assistant linking | `Directory symlink from an older version` | **Set up**: the symlink is replaced with individual links |

The second point is the actual conversion: `.claude/commands -> ../k-playbook/commands` becomes an actual directory with one link per command. The source in `k-playbook/` remains untouched.

Individual links belong in the project's repository and are committed. A fresh clone then has commands registered immediately.

### A configuration from a retired model

Anyone carrying forward a project from one of the first versions will eventually encounter:

```text
K-PLAYBOOK.yaml has schema_version 1 and describes a retired model ...
```

This is not an error in the project, but the consequence of a deliberate separation: the installation updates itself through `git pull`, while `K-PLAYBOOK.yaml` sits beside it and is never overwritten because it belongs to the project. At some point the tool is three models ahead while the file is still at the first. It is not converted ([`k-playbook-format.md`](./k-playbook-format.md#schema_version) explains why), but it can be reset.

Start the interface: the **Project configuration** section then appears again, names the found version and the model it describes, and offers **Reset and create again**. It:

- places the old file alongside it as `K-PLAYBOOK.yaml.v1-alt`, rather than deleting it, because `remediation`, `tools`, and `project.repo_root` exist only there,
- writes a fresh `K-PLAYBOOK.yaml` with `schema_version: 3`, prefilled with the old `project.repo_root`,
- never overwrites an existing backup, instead appending `-2`, `-3`.

Then go through the remaining sections as for a new installation and retrieve custom values from the backup.

**First move project-owned content.** Under model 1, tasks, checks, reviews, guidelines, docs, and `TODO.md` were **inside** `k-playbook/`, precisely the directory that is now the replaceable clone. Merely renewing the configuration would leave a silent trap: everything would look healthy, and the next update would take the content with it. If the interface finds project-owned content there, it writes nothing, names the paths, and remains at "Outdated":

```bash
cd /path/to/project
git mv k-playbook/tasks     k-playbook-local/tasks
git mv k-playbook/reviews   k-playbook-local/reviews
git mv k-playbook/TODO.md   k-playbook-local/TODO.md
```

The moved `TODO.md` does not stay in that form: on the first todo access -- through the interface, `/k-todo`, or `k-playbook todo` -- it is translated into `k-playbook-local/data/todos.json` and then removed. Text, order, and done state are preserved.

The old file's `paths.` section identifies the paths. Once they have moved, the button becomes available.

## One tool for all projects

After bootstrapping, this is enough everywhere:

```bash
cd /path/to/project
k-playbook
```

It is the same tool for every project. Which project it means derives from the directory in which it is called, not from the program's location. The clone under `k-playbook/` is a pure content source: commands, skills, rules, reviews, checks, and documentation. A second entry point in the project itself no longer exists; k-playbook is called exclusively by its name, and commands do the same.

`~/.local/bin/k-playbook` is an actual file, not a symlink or runtime resolution. Every work environment installs the version for its own platform: the macOS host a Darwin binary and the DevContainer a Linux binary. A DevContainer therefore bootstraps once for itself and again after a rebuild.

**There is no longer dedicated DevContainer integration**: no bind mount to `/workspaces/k-playbook`, no symlink in the container, and no setup script in `.devcontainer/`. The installation is in the project directory and enters the container with it like every other project file.

**One version, one binary.** `VERSION` at the root of the clone names the release whose assets belong to this clone version; `SHA256SUMS` beside it contains their checksums. Commits to rules, reviews, commands, or docs change neither file. If `VERSION` changes during an update, the new version requires another binary, installed explicitly through the bootstrap and never incidentally.

## GitHub CLI

`/k-pr-review` and the Dependabot review work through `gh`. The **GitHub CLI** card separates two things that are easily confused.

One is the **project's decision**: whether it uses `gh`. It is set here and stored in `K-PLAYBOOK.yaml` under `tools.gh.status`. Until it is made, its value is `unknown`, and the card shows it in red, not as a cosmetic defect, but because a command otherwise cannot tell whether a missing `gh` is a problem or intentional. Commands requiring `gh` stop at `unknown`.

The other is the **finding for this machine**: whether `gh` is in PATH and an account is stored. It appears only in the card and context output, never in configuration, because it differs on the next machine.

Install and sign in through the terminal, just as with security tools: both change the host, and `gh auth login` needs a browser. The card shows the appropriate command.

```bash
gh auth login --hostname github.com   # sign in
gh auth status                        # check the token with the server
```

The finding is read from `~/.config/gh/hosts.yml` and is **not checked with the server**: a stored token may have expired or been revoked. To be certain, run `gh auth status`.

If several accounts are stored, the card names them and shows the switch command:

```bash
gh auth switch --hostname github.com --user <account>
```

Deliberately a command, not a button. Switching applies to every terminal and project on this machine, not only this one, and an approval or merge then runs under the new name. `/k-pr-review` therefore names the active account before every write action.

Only `github.com`. Enterprise instances would have their own accounts per host and a separate decision per project; that would be different from this.

## Security tools

Projects may work with their own `.venv`. Security tools are installed separately at host or user scope, never in a project venv. They are one of two deliberate exceptions to project locality: a scanner belongs to the work environment, not the project. Base tools are the second exception; see below.

The canonical matrix is in [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv). The installation script and interface read it; the list is not duplicated in Go code.

Required tools:

| Tool | Languages | Role |
|---|---|---|
| `gitleaks` | all | secret scanning |
| `trufflehog` | all | deep secret scanning |
| `trivy` | all | filesystem, container, and IaC CVEs |
| `syft` | all | SBOM generation |
| `grype` | all | SBOM/dependency CVE analysis |
| `pip-audit` | Python | Python dependency CVEs |
| `ruff` | Python | Python quality and flake8-bandit rules |
| `semgrep` | Python, Go, JS/TS | generic security rules |
| `osv-scanner` | Python, Go, JS/TS | dependency CVEs with SARIF |
| `gosec` | Go | Go security |
| `govulncheck` | Go | Go CVEs with reachability |
| `njsscan` | JS/TS | Node/JS security |

Optional because they overlap with others: `golangci-lint` (Go quality, bundles staticcheck and errcheck). `docker` is also optional and appears as fallback context, but k-playbook does not install it.

`bandit` deliberately appears in neither list: `ruff` covers it; its `S` rule set *is* flake8-bandit. A second tool for the same rules would only produce duplicate findings.

**JavaScript and TypeScript are two separate languages** in the matrix, not a shared one. `AppliesTo` has no cost for this, but candidate counting derives which extensions count from the same setting. With only `javascript`, a pure TypeScript project would get a 0 for its `.ts` files. A project with both names both.

**Required applies per language.** A language-bound tool counts as a missing requirement only when its language was requested. Without an indication, no language binding counts as required, because without this information it cannot require what may not be needed:

```bash
k-playbook/scripts/install-security-tools.sh --languages python,go --preflight
```

The interface shows the status read-only and installs nothing. The script itself does everything else:

```bash
k-playbook/scripts/install-security-tools.sh                       # status; this is the default
k-playbook/scripts/install-security-tools.sh --install missing     # asks before installing
k-playbook/scripts/install-security-tools.sh --help                # explains the methods
```

`--method` selects between `auto`, `native`, `docker`, `pipx`, and `venv`. Without `--yes`, the script shows the plan and asks.

The tool's origin is in the matrix rather than the script: the `install_method` column names `github` (release asset), `go` (`go install`), `pipx` (pipx or a dedicated tool venv), or `none`; `install_ref` gives the appropriate reference, and `asset_pattern` gives the asset-name pattern for GitHub releases. A new tool is therefore one TSV row.

**`go install` remains limited to tools that already need Go.** Otherwise a pure Python project would have to install Go merely to acquire a scanner. Only `govulncheck` is affected: it analyzes Go sources and needs the toolchain at runtime but has no release binaries. `gosec`, `golangci-lint`, and `osv-scanner` therefore come from GitHub releases, with `osv-scanner` as a bare binary without an archive, which the script recognizes from the asset name.

A project may of course work with `.venv`. The read-only preflight then measures exactly that active venv and identifies the measurement context in the interface. **Only before installing security tools must no project venv be active**, so nothing is written into the project venv. If `VIRTUAL_ENV` is set and installation is planned:

```bash
deactivate
```

`--method auto` is recommended: native binaries, Go tools, and Python CLI tools through `pipx` or dedicated tool venvs. To generally isolate Python CLI tools in venvs, use explicitly:

```bash
k-playbook/scripts/install-security-tools.sh --install missing --method venv
```

This also does not install into `<project>/.venv`, but into dedicated k-playbook tool venvs under `~/.local/share/k-playbook/security-tools/<tool>-venv`. `--method venv` applies only to Python CLI tools; GitHub-release and Go tools continue to use their native installation path. Each Python tool gets its own venv so their dependencies do not interfere; `--venv-root` can relocate the root.

## Base tools

The second deliberate exception to project locality, and a different type from security tools: base tools are not scanners but the ground on which commands stand: `bash`, `git`, `curl` or `wget`, `tar`, `python3`, and `rg`. Either `curl` or `wget` is sufficient.

The matrix is in [`../scripts/base-tools.tsv`](../scripts/base-tools.tsv), separate from the security matrix: `scanners.tsv` refers to the latter through its `tool` column, and an `rg` in it would appear in every review run as a skipped entry.

**A missing base tool warns; it does not block.** `k-playbook context` reports the state under `baseTools`; a command names the gap, uses a fallback where one exists, and continues. This distinguishes them from `gh`, whose absence ends a PR review hard.

Inspect and install state:

```bash
k-playbook/scripts/install-base-tools.sh --preflight
k-playbook/scripts/install-base-tools.sh --install
```

The script decides per tool: as root with `apt-get`, it installs system-wide through the package manager. Otherwise, it takes the user-local path from a GitHub release. Currently this applies only to `rg`, and this path needs no root. For `git`, `curl`, `wget`, `tar`, and `python3`, there is no useful user-local path; the script prints the `sudo apt-get` command and exits with status `3`, which distinguishes "there is no path for this tool here" from failure.

**k-playbook never escalates to root itself.** It shows the `sudo` command but never runs it, and the script does not restart itself through `sudo`.

The target of the user-local path can be relocated through `--prefix` and `--bin-dir`, as well as `K_BASE_TOOLS_PREFIX` and `K_BASE_TOOLS_BIN_DIR`. A writing invocation whose resolved target does not belong to the executing user is rejected. This catches the `sudo` typo that would otherwise leave binaries with incorrect ownership.

### For a Dockerfile or DevContainer

`--yes` disables every prompt so a single RUN line can run unattended. As root with `apt-get`, the normal case in an image build, it installs everything system-wide:

```dockerfile
RUN bash /opt/project/k-playbook/scripts/install-base-tools.sh --install --yes
```

If a tool has no path on this host, the run ends with `3`. This is not a failure, but `docker build` stops for it. If you do not want that, append `|| test $? -eq 3`; if you want to see the gap during the build, leave it in place.

## Build it yourself

For normal operation, the release asset downloaded by `bin/install` is enough. Anyone working on the tool or preferring to build it themselves needs Go:

```bash
make -C k-playbook dist         # all platforms into dist/
make -C k-playbook dist-host    # only this machine's platform
make -C k-playbook dev-install  # builds and installs this platform
```

All build targets use the same flags CI uses to build release assets, so every path produces bit-identical binaries. `dist-host` saves the three foreign platforms and is enough when only this machine needs to run the version. A self-built binary is not used automatically: `dev-install` places it at `~/.local/bin/k-playbook`, and only afterwards does the `k-playbook` call use it. This is also the way to work without network access: build rather than download.

The installation is write-protected. `make -C k-playbook installer-writable` makes it writable for building, and `installer-readonly` locks it again. An update returns it to the clone state anyway.

## Verification

Checklist for a project:

- [ ] `K-PLAYBOOK.yaml` is in the project root, not in `k-playbook/`.
- [ ] `schema_version: 3` is set.
- [ ] `project.repo_root` points to the project repository; `project.vcs` is `git` or `none`.
- [ ] `k-playbook/` is its own clone and contains no project-owned content.
- [ ] `k-playbook-local/` exists completely and is committed in the project repository, except for the contents of `results/`, which are excluded from the outset in a new installation.
- [ ] `.claude/commands`, `.claude/skills`, `.opencode/commands`, and `.cursor/commands` are directories with individual symlinks to `k-playbook/` or `k-playbook-local/`; the interface reports them as configured.
- [ ] `CLAUDE.md` is a regular file with the `@AGENTS.md` line outside backticks and code blocks, and `AGENTS.md` carries the prompt. An actual carried-over `CLAUDE.md` was renamed to `AGENTS.md`; a symlink from an older version was replaced with the include file. If `Conflict` appears instead, resolve it manually; until then, Claude Code cannot see the prompt.
- [ ] `k-playbook context` completes and names the expected catalogs.

The final item checks all preceding items at once: the command stops if configuration is missing or has a different `schema_version`.

## Troubleshooting

**Slash commands do not appear.** Start the interface: it compares the catalog with what is registered and names missing commands. Restart the assistant after setup.

**A new command from `k-playbook-local/commands/` is missing.** It is not registered automatically: the interface reports it as missing and creates the link at the press of a button. The same applies when a project-owned file has newly replaced a shipped command; then the existing link still points to the old source.

**Skills are not triggered.** Every skill directory must contain `SKILL.md`; without it, the directory does not count as a skill and is not linked. Then restart the assistant.

**`schema_version` does not match.** If the number is below `3` or missing, configuration is older than the tool. The interface resets it; see [A configuration from a retired model](#a-configuration-from-a-retired-model). If it is higher, the reverse is true: the installation is behind. Then `git pull` in `k-playbook/` helps, not a reset, which would discard the newer file.

**The tool finds no project.** Then `K-PLAYBOOK.yaml` is missing above the invocation location. Starting at the working directory, the search goes upwards to `$HOME` or `/` and deliberately does not guess. The interface then proposes a location.

**An assistant sees foreign commands.** Typical after an installation from the old model: host-global symlinks apply to every project. Start the interface once; it removes them and reports what was removed.

**`k-playbook: command not found`.** Either the bootstrap has not yet run in this environment, in which case use `make -C k-playbook install` (without make: `k-playbook/bin/install`), or `~/.local/bin` is missing from PATH. The bootstrap checks PATH itself and stops in that case before downloading anything.

**`cannot execute binary file`.** `~/.local/bin/k-playbook` contains the binary for another platform, typical with a host and DevContainer sharing `$HOME`. Run the bootstrap again in this environment; it detects the case and reports that the other environment also needs to run it again afterwards.

**The bootstrap finds no asset.** If `VERSION` is missing in the clone, no release belongs to this version. Then `git pull` in `k-playbook/` or build it yourself; see [Build it yourself](#build-it-yourself).
