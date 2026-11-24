# Installation

The installation is the binary alone: one `k-playbook` per machine, once on the host and once in every DevContainer. A project carries no copy of the tool; it has its anchor `.k-playbook/K-PLAYBOOK.yaml` and what it owns itself.

```bash
curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash
cd /path/to/project
k-playbook
```

Without `curl`, the same bootstrap runs with `wget`:

```bash
wget -qO- https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash
```

`bin/install` is self-contained: it determines the newest release of k-playbook, downloads the release binary for the platform, verifies it against the `SHA256SUMS` of that tag and installs it to `~/.local/bin/k-playbook`. Everything else comes with the binary: it unpacks commands, skills and scripts into `~/.local/share/k-playbook/`, and it reads the shipped texts — instructions, review recipes, guidelines, the texts of the commands and this handbook — from the public collection `k-playbook` on the project's Kephalaion hub.

**Go is not required.** `bin/install` downloads the release binary suitable for the platform and installs it to `~/.local/bin/k-playbook`. On macOS and in the DevContainer, the installer runs in its own environment and therefore installs the matching macOS or Linux binary.

**`git` is not required.** The newest release is the newest published GitHub release, not the highest tag: `bin/install` reads the redirect of `https://github.com/kascada/k-playbook/releases/latest` to `…/releases/tag/<tag>` without following it — one request over HTTPS, anonymous, without the GitHub API. A release whose tag carries a suffix (`v1.2.3-rc1`) does not count; then the newest release is unknown and nothing is installed. `bin/install` needs `curl` or `wget` and `sha256sum` or `shasum`.

**Installation requires network access.** The binaries are not in the repository but are release assets. `bin/install` downloads exactly the asset of the newest release and verifies it against the `SHA256SUMS` committed at its tag, fetched from the repository over HTTPS: the expected checksum comes through the Git state, the binary through the release, so a swapped asset is noticed. The release workflow publishes a release only once all assets are attached, so a tag whose assets are still being built is not visible yet. Should an asset be missing anyway, `bin/install` says that the release is not finished and replaces nothing.

**Kephalaion is installed along with it.** k-playbook needs a Kephalaion node for every `keph:` document. If no `kephalaion` is on the `PATH`, `bin/install` then loads the `install.sh` of the latest Kephalaion release and runs it; that script downloads the binary and `SHA256SUMS`, verifies the checksum and installs to `~/.local/bin/kephalaion`. An existing `kephalaion` is left alone by the script, whatever its version; updating is the job of `k-playbook upgrade` (see [Update](#update)). Only the binary is installed. Setting up the node, the service and the token stays with Kephalaion (`kephalaion node init`, `kephalaion service install`); in a DevContainer, where the configuration comes read-only from the host and the node is the host's, that is exactly right. If the step fails, `bin/install` still ends with exit code 0, because k-playbook itself is installed, and names the command to catch up. `k-playbook upgrade` loads and runs the same `install.sh` in its Kephalaion step when `kephalaion` is missing, and `k-playbook health` names the installation as the remedy while `kephalaion` is missing and the connection is not ok.

**`~/.local/bin` must be in PATH**: this is a requirement, not an aside. k-playbook is called exclusively by its name; without the PATH entry, it would be installed but not discoverable by anyone. If it is missing, the bootstrap stops before downloading anything and prints the line for the shell profile:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

On Linux, the entry usually already exists. On macOS, it does **not**: `/etc/paths` does not include it and `path_helper` does not add it. The line belongs in `~/.zprofile` (zsh) or `~/.bashrc` (bash); then open a new shell and call the bootstrap again.

One special case exists on the first run on a fresh host or in a DevContainer: Debian and Ubuntu add `~/.local/bin` in `~/.profile` only when the directory already exists at login. The bootstrap therefore creates it **before** checking PATH and, when stopping, says that the profile does not need changing: log in again (or run `. ~/.profile`) and call the bootstrap again.

**Host and DevContainer with a shared home.** `~/.local/bin/k-playbook` is a real file for each platform. If both environments share the same `$HOME`, they overwrite each other there. `bin/install` and the installation by `k-playbook upgrade` detect a binary for another platform and report during replacement that the other environment also needs to install again, with the `curl … | bash` line above, because the binary there can no longer be run. Calling the binary directly when it belongs to the wrong platform is not handled: the shell then reports `cannot execute binary file`. Separate homes are the clean state.

## The three steps

The final call opens the interface in the browser. It guides you through three steps and writes each one only after confirmation.

### 1. Create configuration

On its first run in a project, the interface does not yet find `K-PLAYBOOK.yaml`. Rather than guessing, it proposes a location and lets you confirm it. Candidates in this order:

1. the Git repository in which the command is called,
2. the working directory.

It also proposes where the project repository is located, either the project root itself or a subdirectory of it, for example when the code is checked out one level below. The result:

```text
project/
└── .k-playbook/
    └── K-PLAYBOOK.yaml the anchor; the directory holding .k-playbook/ is the project root
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

### Where project-owned content lives

What a project owns is a document in the project collection or lies under `.k-playbook/` in
the project root:

- tasks, the workbench, todos and the project documentation in the project collection
  (`keph:/user/<user>/tasks/`, `keph:/user/<user>/workbench/`, `todos/`, `keph:/docs/`);
- project-owned commands and skills in `.k-playbook/commands/` and `.k-playbook/skills/`,
  versioned next to the configuration. Setup does not create these two; they appear when
  the project puts something there;
- project-owned instructions, includes and review recipes in the project collection under
  `keph:/k-playbook/`;
- the source configuration of the version inventory as the document
  `keph:/k-playbook/inventory.md`, created by `/k-doc-inventory` on the first confirmed
  addition.

The location of review results is not decided yet; until it is, reviews are paused, see
[Reviews and tasks](#reviews-and-tasks).

### 2. Register the MCP server

The **k-playbook MCP** section registers the shipped MCP server with both assistants. This gives an assistant the briefing, the documents of the project's collection, the todos and the other tools of k-playbook as MCP tools instead of making it retrieve them through the command line:

```text
project/
├── .mcp.json          Claude Code:  mcpServers -> k-playbook
└── opencode.json      OpenCode:     mcp -> k-playbook
```

It registers `k-playbook mcp`, resolved when written to the **absolute path** of the installed binary. Clients launched from the Dock or Finder do not inherit the shell PATH; a bare command name would be dead there. To commit the entry, enter the bare name `k-playbook` manually: automatic correction does not change it. A client launched from the Dock or Finder then needs the absolute path, entered manually -- *Set up* does not replace an accepted form. Both are described in [`mcp.md`](./mcp.md#why-the-entry-is-an-absolute-path). One condition always applies: the entry works only when the assistant is opened in the project root, the directory where `.k-playbook/K-PLAYBOOK.yaml` is located.

Both files belong to the project. Exactly the `k-playbook` key is touched; foreign entries remain. Registration is complete only after restarting the assistant; Claude Code asks for approval once.

Everything else, the two schemas, handling foreign values, manual removal, and the `/mcp` page with the tools actually offered, is described in [`mcp.md`](./mcp.md).

### 3. Link assistants

Links are created for Claude Code and OpenCode:

```text
project/
├── AGENTS.md             instructions, one source for all assistants
├── CLAUDE.md             include file with the line @AGENTS.md; the direction is fixed
├── .k-playbook/
│   ├── share             -> ~/.local/share/k-playbook, machine-local, not versioned
│   ├── .gitignore        contains /share
│   ├── commands/         project-owned commands, versioned (only when used)
│   └── skills/           project-owned skills, versioned (only when used)
├── .claude/
│   ├── commands/         one symlink per command
│   └── skills/           one symlink per skill; OpenCode reads here too
└── .opencode/
    └── commands/
```

The three targets are **actual directories with individual symlinks**, not directory symlinks. A directory symlink points to exactly one source, so it would provide either only the shipped entries or only `.k-playbook/commands/`; it would also swallow real files a project puts directly into `.claude/commands/`, which today win over the catalog. Each link points to the version that applies under the overlay rule:

```text
.claude/commands/
  k-todo.md    -> ../../.k-playbook/share/commands/k-todo.md   shipped
  k-review.md  -> ../../.k-playbook/commands/k-review.md       project-owned, replaces
  k-own.md     -> ../../.k-playbook/commands/k-own.md          project-owned only
```

The shipped commands and skills are unpacked by the binary once per machine into `~/.local/share/k-playbook/` (`$XDG_DATA_HOME/k-playbook/` when set). The individual links reach them through the one link `.k-playbook/share`: that link is absolute and depends on the machine's home directory, so `.k-playbook/.gitignore` keeps it out of version control, while every link through it stays relative and can be committed. Linking the shipped entries globally (`~/.claude/commands/`) is not an option: with the same name in both places Claude Code takes the global one, so a project could no longer replace a shipped command.

**Host and DevContainer sharing the project.** If both work in the same project directory with different home directories, each one derives a different target for `.k-playbook/share` and points the link at its own home whenever it heals the links: on every briefing and interface start. The other environment then loses all shipped commands and skills without an error, because Claude Code silently skips links without a target. Mounting `~/.local/share/k-playbook` under the host's path alone does not help: the binary in the container still derives its target from its own home.

The fix is the variable `K_PLAYBOOK_SHARE_DIR`. It names the directory itself and takes precedence over `XDG_DATA_HOME` and the home directory; the binary unpacks there, links there and reads its scripts from there. k-playbook never sets it. The container sets it to the host's path, so both environments compute the same link target. On the host it stays unset. A relative value is an error that the link check reports by name, not a silent fallback to the home directory.

```jsonc
// .devcontainer/devcontainer.json
"containerEnv": {
  "K_PLAYBOOK_SHARE_DIR": "${localEnv:HOME}/.local/share/k-playbook"
},
"mounts": [
  "source=k-playbook-share-${devcontainerId},target=${localEnv:HOME}/.local/share/k-playbook,type=volume"
]
```

- **Own volume, not the host's directory.** Host and container unpack the version embedded in their own binary; sharing one directory would make them overwrite each other whenever the versions differ. The volume also keeps the security tools across rebuilds.
- **Writable for the container user.** Docker creates a new mount point owned by root. Change its owner once, for example `sudo chown "$(id -u):$(id -g)" "$K_PLAYBOOK_SHARE_DIR"` in `postCreateCommand`.
- **Host with `XDG_DATA_HOME`.** If the host sets it, its directory is `$XDG_DATA_HOME/k-playbook`, and the container's variable must name that path instead.

Inside a container whose project directory is mounted from the host, the environment check warns while the variable is missing (area `share`, in the briefing and in `k-playbook health`). On the host it stays silent; there the commands simply disappear.

The interface compares this expected state with what is actually registered and reports deviations by name: what is missing, what points to the wrong source, what is orphaned, and what belongs to the project and therefore remains. It synchronizes them at the press of a button. An actual file that someone placed there themselves always takes precedence and is never replaced.

Skills appear only once under `.claude/skills`: OpenCode searches this directory too. `CLAUDE.md` is an include file with the import line `@AGENTS.md`, because Claude Code reads only `CLAUDE.md` while OpenCode prefers `AGENTS.md`. This leaves exactly one actual instruction file, and Claude Code loads it at startup through the import. Setup writes a short stub: above the import line, a note that project rules belong in `AGENTS.md`, then `@AGENTS.md` alone on a line.

`AGENTS.md` also receives a brief **prompt**: a block that names `k-playbook briefing` (and the MCP tool `k_playbook_briefing`) as the one call at the start of a session. If the file is missing, it is created; if it exists, the block is appended and existing content is not touched. A `<!-- k-playbook:anstoss -->` marker prevents a second run from appending it again.

### An existing CLAUDE.md

The direction is fixed: `CLAUDE.md` includes `AGENTS.md`, never the other way around. So that a second, divergent instruction file does not arise alongside it, setup first classifies the `CLAUDE.md`/`AGENTS.md` pair and resolves what can be resolved:

| Initial situation | What happens |
|---|---|
| `CLAUDE.md` contains the `@AGENTS.md` line, `AGENTS.md` is an actual file | the **expected state**, nothing to do. Content beside the import line belongs to the project and remains untouched |
| `CLAUDE.md` is a symlink to `AGENTS.md` | the symlink is **replaced without loss** by the include file; the content is already in `AGENTS.md`, including on the read path, see below |
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

**A symlink `CLAUDE.md`** is replaced with the include file by the next `k-playbook briefing` or by the assistants section of the interface, which reports this in `links.note`. Afterwards, `git status` shows a one-time modified `CLAUDE.md` with a changed mode (symlink -> regular file, `120000` -> `100644`); this change must be committed. It is the one place where a read-only path changes a versioned file in the project root, and it is named as such. No include to nowhere: as long as `AGENTS.md` is absent, such a symlink waits for its target and is replaced only when it exists. If the include file exists while `AGENTS.md` is missing, the detail text identifies the import to nowhere: Claude Code loads nothing from it until **Setup** creates `AGENTS.md`.

**The cost of separation.** Without a symlink, there are two files that can diverge: content Claude Code writes to `CLAUDE.md` through `/memory` or `#` does not reach OpenCode, and the check does not report project content beside the include as a conflict. This is deliberate: reversing the direction would mean treating every piece of content beside the include as a conflict. As a counterweight, the stub above the import line says that project rules belong in `AGENTS.md`.

**What Claude Code must support.** `@` imports in `CLAUDE.md` have existed since Claude Code **0.2.107** (CHANGELOG entry "CLAUDE.md files can now import other files"); an older version silently loads nothing from the stub. Import depth is limited: according to the documentation, Claude Code follows imports up to **four levels** deep ("Imported files can recursively import other files, with a maximum depth of four hops", code.claude.com, page "How Claude remembers your project", as of 2026-09-05). The stub consumes one level, so an `AGENTS.md` that itself imports with `@` has one level less than it would have when read directly.

Anything else an assistant should read is not in `AGENTS.md`: it comes with `k-playbook briefing`, the one call `AGENTS.md` names. The briefing carries the instructions that always apply, the shipped parts first and the project-owned parts from the project collection (`keph:/k-playbook/instructions/always/`) after them. What a project wants every session to know belongs into its collection.

Linking is project-local. Nothing is written to `~/.config/opencode/` or `~/.claude/`. This allows one machine to have several projects with different k-playbook versions without them overwriting each other.

After changing commands or skills, restart the respective assistant: both discover them at startup.

## Browser on startup

At interface startup, the URL is printed in the terminal and the browser opens. The server behind it continues as a background service per project: the call returns as soon as the browser is open, and a second call in the same project only opens another window on the same server. It is stopped through `Stop service` in the interface or `k-playbook stop`; without any request, it stops itself after 60 minutes.

**Asking without acting.** `k-playbook gui status` reports the state and the address of this project's service as JSON and changes nothing at all: it starts nothing, stops nothing, opens no browser, and does not clean up an orphaned runtime file. Without a service the whole answer is `{"state": "none"}`, with exit code 0. An assistant gets the same answer through the MCP tool `k_playbook_gui_status` (see [`mcp.md`](mcp.md), "GUI Status Contract").

The program that opens the browser is selected in this order:

1. **`$BROWSER`**, if set. The freedesktop convention: a `:`-separated list of commands in which `%s` stands for the URL. If the placeholder is absent, the URL is appended.
2. Otherwise, the usual platform candidates: `open` on macOS, otherwise `wslview`, `xdg-open`, `gio open`, and others until one starts.

**In a container, only `$BROWSER` counts.** Every guessed candidate would run in the container rather than on the user's machine; worse, `x-www-browser` and `sensible-browser` in slim images often point to a terminal browser, which then takes over the terminal. An explicitly set `$BROWSER` knows better: it points to a helper that passes the URL through to the host.

The VS Code DevContainer sets this up automatically: the variable points to a script that calls `code --openExternal` and therefore opens the browser on the host. VS Code also forwards the port. It is the same path by which `gh auth login` opens its browser.

If `$BROWSER` is not set in the container, the existing behavior remains: the terminal names the detected container marker and the URL to enter manually. To add the helper, set the variable yourself:

```bash
export BROWSER=/path/to/helper.sh   # receives the URL as an argument
```

## Reviews and tasks

The **Workflows** section brings together the work queues. Two kinds have a page of their own: reviews and todos, the latter from the project collection (`todos/`). The two pages are listed under Workflows in the switcher on the left, on every page.

`/workflows` itself is the overview: one card per kind explains what tasks, reviews and todos are and when to use each. The card "Reviews" shows "Pausiert" and the reason instead of a counter, the card "Todos" shows how many are open, and each of the two links to its page. The card "Tasks" carries neither a counter nor a link.

**Tasks** have no page in the interface. They are documents in the project collection (`keph:/user/<user>/tasks/`, see [`task-flow.md`](./task-flow.md)) and are created, hardened and executed through `/k-task-create`, `/k-task-refine` and `/k-task-run` in the assistant.

**Reviews** (`/workflows/reviews`) lists no runs: reviews are paused until the location of their results is decided (task 147), and the page shows the message of the pause. `/k-review`, `/k-audit` and `/k-remediation` only say so, `k-playbook scan` and `k-playbook merge` end with the same message, and all five MCP tools, `k_playbook_review_status`, `_create`, `_scan`, `_merge` and `_write_ai_entry`, answer `reviews_paused` and write nothing ([`mcp.md`](./mcp.md#review-tools-are-paused)). **Todos** (`/workflows/todos`) lists the open items, with the checked-off ones in a collapsed section below.

Each of the three pages opens with a short help block. Its head carries a small link into the shipped documentation — the overview and the todos to `commands.md`, the reviews to `code-review.md` and `review-runs.md`. It opens the file in the Docs section, with the index and its cross-references; the address behind it is `/docs?file=<file>`, which any page can use.

## Read documentation

The **Docs** section shows the handbook of k-playbook, the same documentation you are reading now. It comes from the shipped layer: in the source repository it lives in `defaults/docs/handbook/`, and with every release the hub carries it in the public collection under `<hub>:k-playbook/defaults/docs/handbook/`; it is read through Kephalaion, not from disk and not from the binary. The index is on the left in the menu and the open file on the right; without a selection, it shows the start page `overview.md`. Links in the text lead to the next file, and anchors jump within the open file.

The **Handbuch** section shows the handbook of the project in the same view: the pages under `keph:/docs/manual/handbook/` in the project collection, a subdirectory of the manual documentation. Its start page `README.md` comes from a template and is always shown first; a project adds pages by creating Markdown files there.

Neither section has a fallback. Without Kephalaion both show a message instead of pages. Docs also says when the shipped content on the hub predates the handbook — the hub fetches a new release only when the collection is created, by hand or on request — and names the remedy: the hub fetches the k-playbook release again (Kephalaion `fetch_release` for `<hub>:k-playbook`). Handbuch says when `keph:/docs/manual/handbook/` does not exist; `k-playbook docs readme manual/handbook` writes its start page.

Another page can request a specific file: `/docs?file=task-flow.md`, optionally followed by an anchor. That is how the help links of the Workflows pages work. If the requested file is not in the index, the answer says so rather than falling back to the start page — the shipped content on the hub can carry a different state than the binary that wrote the link.

Mermaid diagrams are rendered if the machine has network access: the library is loaded when needed. Without network access, the diagram source remains visible and the text is still fully readable.

## Inspect what applies

At the bottom is the **Resolved context** section. When expanded, it shows what a command sees: resolved paths, the effective catalog for reviews with its origin, shipped, project-owned, or replaced; a catalog for checks no longer exists (task 155). Guidelines are not listed there any more (task 148): they come shipped only, and the briefing names them. Disabled entries are included so their existence remains visible.

The section loads only when expanded and changes nothing.

An assistant can receive the same information as a tool instead of retrieving it from the command line. That is what the MCP server is for; it is configured in the **k-playbook MCP** section, and the `/mcp` page shows the registration state along with the tools the server actually offers. Everything about it is in [`mcp.md`](./mcp.md).

## Update

One call updates everything: `k-playbook upgrade` in the terminal, or `/k-upgrade` in an assistant session, whose hull runs the same call first. It brings Kephalaion up to date, then the program, then the project (`AGENTS.md`, `CLAUDE.md`, the linking and the MCP entries); the details are in [Updating the program](#updating-the-program). In the interface, the update button offers the program update when the running program is older than the newest release.

**What lies in the project root is caught up on the next call to `k-playbook`.** Startup is the general fallback path: it adds a missing MCP entry in a file not tracked by Git, creates a missing `AGENTS.md` from the template, and reports what it did. This is not an additional everyday step, but the same call with which a session starts anyway. `.k-playbook/K-PLAYBOOK.yaml` remains untouched by all of this.

### Updating the program

The program has exactly two version sources: its own version, stamped into the binary, and the newest release tag on GitHub. Every comparison and every installation of the program starts from these two; no `VERSION`, `SHA256SUMS` or `bin/install` in the project is read or called for it.

- **`k-playbook version`** prints the program's own version, one line. **`k-playbook version --all`** adds the shipped VS Code extension and the newest release tag; without network that line says `unbekannt` with the reason, and the exit code stays 0. In a terminal, `version` points to `--all` on stderr.
- **The health check warns** when the program is older than the newest release, with `k-playbook upgrade` as the remedy. It is the same check as in the briefing, `k-playbook health`, `k_playbook_health` and the status page. Without network, without a version of its own, or when the program is equally old or newer, it stays silent. The check reads the newest release from a cache, `latest-release.json` under `$XDG_CACHE_HOME/k-playbook/` or else `~/.cache/k-playbook/` (the same on Linux and macOS): a found release counts for six hours, a failure (no network, no answer in time) for thirty minutes, and only an older entry makes it ask GitHub. So it may lag behind a new release by a few hours. The note at interface startup and the status page when it loads read the same cache.
- **`k-playbook upgrade`** in the terminal and **the update button** in the interface always ask GitHub right away — as do `k-playbook upgrade --check` and `k-playbook version --all` — and write the answer to the cache. They install the release binary of the newest release directly from GitHub, verified against the `SHA256SUMS` of its tag (fetched from the repository over HTTPS, not as a release asset), and replace `~/.local/bin/k-playbook` atomically.

If the running binary is **older** than the newest release tag, the interface shows `Programm aktualisieren (vA → vB)`. On the click the service installs the release binary, hands over its runtime file to a new service started from that file, and the open page follows to the new address, where the header shows the new version. Nothing is downloaded on startup or during a check: only an explicit click or an explicit `k-playbook upgrade` installs. If the binary is equally old or newer — a build above the newest tag, as `make dev-install VERSION=…` produces — nothing is installed and the service continues. The comparison is semantic, and only "older" triggers it. If the newest release cannot be read — no network, no published release, no answer within three seconds — it is unknown: the health check stays silent, and `k-playbook upgrade` says so and still catches up the project.

The newest release is the newest published one, and the release workflow publishes it only after all assets are uploaded. Between pushing a tag and the finished release, everything still sees the previous release; there is no window in which the health check warns but nothing can be installed yet. Should a published release lack an asset anyway, `k-playbook upgrade` and the button report that the release is not finished; nothing is replaced.

If the installation fails -- no network, checksum mismatch, release not finished -- the old service keeps running and the page names the error, the output, and the command to catch up by hand: `k-playbook upgrade`. If `k-playbook upgrade` itself fails to install, it names the first installation instead: `curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash`. The same holds if `k-playbook` in the `PATH` does not resolve to `~/.local/bin/k-playbook`: then there is no button and no installation, only a note with both paths. Installing anyway would put a new binary where the next call to `k-playbook` never looks; that call would start the old program again, and its startup would stop the newly started service. Remove the other file or move `~/.local/bin` to the front of the `PATH`, then run the first installation in the terminal. Afterwards, `k-playbook` starts the new version. After a program update in the terminal, an old service initially keeps running; it is detected and replaced only when the next call to `k-playbook` comes from a newly installed binary. The binary file itself is compared, not only its version, so a newly built binary with the same version also replaces the old service. If only commands, rules, or recipes are new, the service continues, `Reload` in the interface loads the version, and restarting the assistant is enough.

**The MCP registration is caught up on every path.** Every start of the interface and the project step of `k-playbook upgrade` add a missing MCP entry, or create a missing file, when the target file is not tracked by Git. An existing entry stays, also one in no accepted form; *Set up* overwrites it. `k-playbook upgrade --check` does not report MCP entries.

**Linking catches itself up.** Because commands and skills are linked individually, a newly shipped command does not arrive by itself. It is therefore caught up on the read path, not only at the press of a button: the assistants section synchronizes registration when displayed and reports what changed (`Linking synchronized: 3 added, 1 removed.`), and `k-playbook briefing` does the same, the call already made at the beginning of every assistant session. How the program reached its version is irrelevant: through the interface, `k-playbook upgrade`, or a build of one's own. What cannot resolve itself, an actual project file in the way or a conflict in `CLAUDE.md`, remains and continues to appear in the assistants section.

For the new commands to reach the assistant, restart it afterwards: Claude Code and OpenCode discover them at startup.

**Kephalaion comes first in `k-playbook upgrade`.** In the terminal and in `/k-upgrade`, `k-playbook upgrade` first brings Kephalaion up to date, then the program, then the project. The order is deliberate: k-playbook depends on Kephalaion and not the other way round — new k-playbook releases rely on new Kephalaion features, while Kephalaion keeps serving older clients — and the project step and `/k-upgrade` need the node. If `kephalaion` is missing, the step loads the `install.sh` of the latest Kephalaion release and runs it; if that fails, the line names the installation by hand. If it is there, the step calls `kephalaion upgrade --auto --json` and reports only what Kephalaion answers: Kephalaion updates itself and restarts its node only where it considers that safe, on a machine with a per-user node running as a per-user service. A hub, a global installation, a dev build, a node whose hub does not carry the new release yet, or a node inside a DevContainer are never touched; the line names the reason and Kephalaion's hint for doing it by hand. An older `kephalaion` that does not know `--auto` yet (exit code 2) gets a line from `kephalaion upgrade --check --json` instead: installed version, newer release and how to update by hand. `k-playbook upgrade --check` only calls `--check --json` and changes nothing. Nothing in this step stops the run: on failure, missing JSON or a timeout the program and project steps still follow. A call that runs past its time limit (3 minutes for `--auto`) is asked to stop with SIGTERM and given time to clean up; its line then says "outcome unknown" and names `kephalaion status` as the check.

When a run of `k-playbook upgrade` replaced the binary, its last line says to call `/k-upgrade` in an assistant session — a new one if a restart is due — for the conversions that need judgement, and `--check` ends with a preview line when a run would fetch something new; both are left out under `K_PLAYBOOK_VIA_COMMAND=1`, which the hull of `/k-upgrade` and its fallback call set.

After an upgrade of Kephalaion, and whenever its node was restarted, the output asks for a restart of the assistant session, like after a program change, and `/k-upgrade` stops there; other sessions using the same node may have lost their connection to Kephalaion as well.

**READMEs of the project documentation.** Setup and update — the interface buttons and `k-playbook upgrade` — end by writing the `README.md` of every directory under `keph:/docs/` that has a template `k-playbook:/docs/<path>/README.md`, when it is missing or differs (task 137). These READMEs are the same in every project and come from the hub's delivery; one changed by hand is overwritten. Without a ready Kephalaion nothing is attempted, a slow hub holds the step up for at most 10 seconds, and a missing template is only a hint: none of it stops setup or update. `k-playbook docs readme` writes them on demand, `k-playbook docs readme --check` only reports.

### A project's own conversion script

`/k-upgrade` runs one script that belongs to the project: if `scripts/k-upgrade-umbau.sh` exists in the project root, the session runs it with `bash scripts/k-upgrade-umbau.sh` from the project root, right after `k-playbook upgrade` and before the conversions of step 3. It runs without asking, also when the binary changed and the command then stops for a restart. Its output is passed on unchanged. If it ends with an exit code other than 0, the session names the code and asks whether the conversions should run anyway. If the file is missing, nothing is said about it.

The script belongs to the project and converts what concerns only this project. The conversions take stock only after it has run.

Only `/k-upgrade` runs the script. The update button in the interface and `k-playbook upgrade` in the terminal do not call it. k-playbook neither ships nor creates the file; the project writes it and keeps it under version control like any other script.

### A directory symlink in the linking

If a target of the linking such as `.claude/commands` is a symlink to a whole directory, only that one source would ever arrive. The assistants section reports it, and one click handles it; nothing is deleted or overwritten:

| Where | What the interface reports | What to do |
|---|---|---|
| Assistant linking | `Directory symlink, replaced with individual links` | **Set up**: the symlink is replaced with individual links |

Individual links belong in the project's repository and are committed. A fresh checkout of the project then has commands registered immediately.

### A configuration with an older schema_version

A `.k-playbook/K-PLAYBOOK.yaml` with an older or missing `schema_version` is not read:

```text
K-PLAYBOOK.yaml hat schema_version 1, dieses Werkzeug erwartet 3 — bitte neu einrichten: ...
```

It is not converted ([`k-playbook-format.md`](./k-playbook-format.md#schema_version) explains why); the project is set up again. Start the interface: the **Project configuration** section then appears again, names the found version and offers **Reset and create again**. It:

- places the old file alongside it as `K-PLAYBOOK.yaml.v1-alt`, rather than deleting it, so that custom values stay readable,
- writes a fresh `K-PLAYBOOK.yaml` with `schema_version: 3`, prefilled with the old `project.repo_root`,
- never overwrites an existing backup, instead appending `-2`, `-3`.

Then go through the remaining sections as for a new installation and retrieve custom values from the backup. Files from an earlier model are neither detected nor reported; they are removed by hand.

## One tool for all projects

After bootstrapping, this is enough everywhere:

```bash
cd /path/to/project
k-playbook
```

It is the same tool for every project. Which project it means derives from the directory in which it is called, not from the program's location. Nothing of the tool lies in the project: commands, skills and scripts come with the binary, the shipped texts from the hub. There is no entry point in the project itself; k-playbook is called exclusively by its name, and commands do the same.

`~/.local/bin/k-playbook` is an actual file, not a symlink or runtime resolution. Every work environment installs the version for its own platform: the macOS host a Darwin binary and the DevContainer a Linux binary. A DevContainer therefore bootstraps once for itself and again after a rebuild.

**There is no dedicated DevContainer integration**: nothing to mount and no setup script in `.devcontainer/`. The container runs the same bootstrap as the host and gets its own binary; the project brings nothing of the tool with it.

**One release, one binary.** A release is a tag `vX.Y.Z`; its assets are the binaries, and the `SHA256SUMS` committed at that tag contains their checksums. `VERSION` at the root of the repository names the same release and is what a build stamps into the binary; the installed program reads no `VERSION` file in a project. If the running binary is older than the newest release tag, it is replaced by `k-playbook upgrade` or by the update button, and only on an explicit call or click; the program never downloads anything on startup or during a check. The download is verified against the `SHA256SUMS` of the tag before the file at `~/.local/bin/k-playbook` is replaced (see [Updating the program](#updating-the-program)).

## Kephalaion collection

Each project works with its own collection in Kephalaion, usually named after the project.
Setting it up happens in this order: an admin finds a Kephalaion hub, creates the collection
there, creates one account per machine, grants it the collection and hands out a setup token
(`kephalaion hub account add` or `kephalaion hub account token`). On the machine the account is
set up with `kephalaion node account setup <hub> <account>`; the token lands in
`~/.config/kephalaion/tokens/<hub>/<account>.token`. If Kephalaion is not set up on the machine
at all, `kephalaion node setup <code>` comes first. Then k-playbook names the hub and the
collection.

**The binding is versioned.** Hub alias and collection are the same for everyone working on a
project, so they live in the block `kephalaion:` of `.k-playbook/K-PLAYBOOK.yaml` and are
committed with the project:

```yaml
kephalaion:
  hub: vm                 # alias of the hub entry; Kephalaion hands it out with the setup code
  collection: k-playbook  # address for clients: vm:k-playbook
```

The block names no account and no token. An `account:` line there is not read; the health check
warns about it.

**Account, token file and node come from Kephalaion.** k-playbook does not search for the
Kephalaion config or the token itself. It asks the client information of the Kephalaion
binary, `kephalaion node client --json --format 1 --no-probe --hub <alias>`, with a time limit
of 5 seconds. The binary is looked up in `PATH`, then `~/.local/bin/kephalaion`, then
`/usr/local/bin/kephalaion`; an MCP server started by a GUI assistant does not always inherit
`PATH`. From the answer (format 1, see `kephalaion node client --help`) k-playbook takes:

- the node address from `node.url`, accepted only for `127.0.0.1`, `[::1]` or `localhost`; any
  other address is reported and never contacted;
- the account from `hubs[].account` and its origin from `choice` — the client's override, the
  default account (`kephalaion node account default`), or the only token file. k-playbook calls
  without `--account`; there is no per-project choice of account;
- the token file from `hubs[].tokenFile`, read only if it is exactly
  `<tokensDir>/<hub>/<account>.token`, a regular file and not a symlink, holding a token of the
  form `keph_` plus 43 characters.

`config.source: none` is not an error: without a Kephalaion config the node's default address
applies. A `config.problem` is named in the message. A token never appears in any output,
message or log, and it is only sent to the loopback address.

**The check.** With these values k-playbook asks the node over MCP: `initialize` and
`tools/call whoami` on `/mcp`, with the header pair `X-Keph-Account-<hub>` and
`X-Keph-Token-<hub>`. It passes when the node answers, reports `login: ok` for the hub and
lists the collection among the readable ones. That shows more than `kephalaion node whoami`:
that the node answers on `/mcp` and that the token is valid at the hub. It does not show
whether the headers in an assistant's own MCP configuration carry the same, still valid token.
The request to the node takes at most 3 seconds.

The briefing carries the result in its section `kephalaion` (`ok`, `message`, `address`,
`user`; see [`k-playbook-format.md`](./k-playbook-format.md#the-briefing)); the check never
makes a call fail. Every state in which
the binding does not work has a message with its remedy:

| State | Meaning | Remedy in the message |
|---|---|---|
| `not-configured` | the block `kephalaion:` is missing in `.k-playbook/K-PLAYBOOK.yaml` | enter the block and commit it, or use the setup step |
| `invalid-entry` | hub or collection breaks Kephalaion's naming rule, or the file is unreadable | fix the block |
| `kephalaion-missing` | no `kephalaion` binary, or one without the client information in format 1 (exit 2, no JSON, other `format`) | install Kephalaion, or `kephalaion upgrade` |
| `client-failed` | the client information could not be produced (exit 1, the reason from stderr), timed out or crashed | the reason; check by hand with `kephalaion node client --json --format 1 --no-probe` |
| `node-address` | `node.url` is not a loopback address | set `listen` under `node:` in the Kephalaion config to a loopback address |
| `account-choice-missing` | several token files for the hub and no default | `kephalaion node account default <hub> <account>`; if the token directory is mounted read-only, as in the DevContainer, set the default on the host |
| `token-missing` | no token file for the hub, only a waiting token (`.pending`), a token file elsewhere, or an unusable one | `kephalaion node account setup <hub> <account>` with a setup token from the admin, or first `kephalaion node setup <code>`; for a lone `.pending` the same `setup` call settles it |
| `node-unreachable`, `hub-unknown`, `hub-never-synced`, `login-invalid`, `login-missing`, `replica-unreadable`, `collection-unreadable` | what the node or `whoami` reports | `kephalaion status`, `kephalaion node sync`, or another collection |

If a `.pending` lies next to the token file (an open `rotate`), the binding still works and the
message adds `kephalaion node account check <hub> <account> --token-file <file>` to clean up.

**The setup step.** The status page of the interface shows the collection, the account with the
origin of its choice and the result of the check. When nothing is entered or the check fails,
the setup step is shown first. It shows the Kephalaion config, the node address and the token
directory from the client information, then:

1. Choose the hub — the hubs come from the client information without `--hub`, each with the
   account Kephalaion chooses for it; a hub without a chosen account shows why.
2. Choose the collection — the node lists the readable ones for the hub's account; the one named
   like the project directory is preselected.
3. **Testen** — runs the check for this choice and writes nothing.
4. **Eintragen** — checks again and writes only when the check passes.

Writing touches only the lines `hub:` and `collection:` of the block `kephalaion:` in
`.k-playbook/K-PLAYBOOK.yaml`; a missing block is appended with a comment, other blocks and
comments stay byte for byte. No account is written. The file is versioned, so the answer says
that the change wants to be committed. Without hubs the page lists the admin steps above; when
the node does not answer, it says to start Kephalaion. The rest of the interface stays reachable
without a collection. "Collection ändern" on the status page opens the setup step again.

## GitHub CLI

The **GitHub CLI** card shows the **finding for this machine**: whether `gh` is in PATH and an account is stored. It appears only in the card and in the partial call `environment` of the briefing (`environment.gh`), never in configuration, because it differs on the next machine. There is nothing to decide on the card: whether `gh` is used follows from the personal GitHub access file (`k-playbook:/docs/handbook/github-access.md`) — with a service for the repository its selected access, without one this finding. Commands without a service stop when `gh` is missing or not logged in and name the access file as the remedy.

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

Deliberately a command, not a button. Switching applies to every terminal and project on this machine, not only this one, and every later `gh` call without a service then runs under the new name.

Only `github.com`. Enterprise instances would have their own accounts per host and a separate decision per project; that would be different from this.

## Security tools

Projects may work with their own `.venv`. Security tools are installed separately at host or user scope, never in a project venv. They are one of two deliberate exceptions to project locality: a scanner belongs to the work environment, not the project. Base tools are the second exception; see below.

The canonical matrix is in [`../share/scripts/security-tools.tsv`](../../../share/scripts/security-tools.tsv). The installation script and interface read it; the list is not duplicated in Go code.

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
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh --languages python,go --preflight
```

The interface shows the status read-only and installs nothing. The script itself does everything else:

```bash
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh                       # status; this is the default
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh --install missing     # asks before installing
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh --help                # explains the methods
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
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh --install missing --method venv
```

This also does not install into `<project>/.venv`, but into dedicated k-playbook tool venvs under `~/.local/share/k-playbook/security-tools/<tool>-venv`. `--method venv` applies only to Python CLI tools; GitHub-release and Go tools continue to use their native installation path. Each Python tool gets its own venv so their dependencies do not interfere; `--venv-root` can relocate the root.

## Base tools

The second deliberate exception to project locality, and a different type from security tools: base tools are not scanners but the ground on which commands stand: `bash`, `git`, `curl` or `wget`, `tar`, `python3`, and `rg`. Either `curl` or `wget` is sufficient.

The matrix is in [`../share/scripts/base-tools.tsv`](../../../share/scripts/base-tools.tsv), separate from the security matrix: `scanners.tsv` refers to the latter through its `tool` column, and an `rg` in it would appear in every review run as a skipped entry.

**A missing base tool warns; it does not block.** The environment check of the briefing names it, and `k-playbook briefing --part environment` reports the state under `environment.baseTools`; a command names the gap, uses a fallback where one exists, and continues. This distinguishes them from `gh`, whose absence stops a command that needs it.

Inspect and install state:

```bash
bash ~/.local/share/k-playbook/scripts/install-base-tools.sh --preflight
bash ~/.local/share/k-playbook/scripts/install-base-tools.sh --install
```

The script decides per tool: as root with `apt-get`, it installs system-wide through the package manager. Otherwise, it takes the user-local path from a GitHub release. Currently this applies only to `rg`, and this path needs no root. For `git`, `curl`, `wget`, `tar`, and `python3`, there is no useful user-local path; the script prints the `sudo apt-get` command and exits with status `3`, which distinguishes "there is no path for this tool here" from failure.

**k-playbook never escalates to root itself.** It shows the `sudo` command but never runs it, and the script does not restart itself through `sudo`.

The target of the user-local path can be relocated through `--prefix` and `--bin-dir`, as well as `K_BASE_TOOLS_PREFIX` and `K_BASE_TOOLS_BIN_DIR`. A writing invocation whose resolved target does not belong to the executing user is rejected. This catches the `sudo` typo that would otherwise leave binaries with incorrect ownership.

## VS Code extension

If the environment has VS Code, starting the interface also installs the extension
**k-playbook Workspace Tools** into the VS Code server of that environment — in the
background, without holding up the start, and without a window reload. With Kephalaion
set up, it binds the project's collection into the workspace on every window start. The extension
travels inside the program binary, so a program update brings the matching version with
it.

This is a deliberate exception to "k-playbook does not install tools by itself": an
explicit first installation gets lost among the other setup steps. The only trigger is
the start of the service, nothing happens in an environment without VS Code, and whoever
never starts the interface installs it explicitly:

```bash
k-playbook vscode install   # install or catch up
k-playbook vscode status    # embedded version, installations, chosen CLI
```

The setup page shows the same state as a card, and a failed catch-up appears there with
its reason. Full detail — settings, which CLI is called, why the VSIX is checked in, how
to add another action — is in [`vscode.md`](./vscode.md).

## Build it yourself

For normal operation, the release asset downloaded by `bin/install` is enough. Anyone working on the tool or preferring to build it themselves needs Go and a checkout of the source repository, at any place outside the project:

```bash
git clone https://github.com/kascada/k-playbook.git
cd k-playbook
make dist         # all platforms into dist/
make dist-host    # only this machine's platform
make dev-install  # builds and installs this platform
```

All build targets use the same flags CI uses to build release assets, so every path produces bit-identical binaries. `dist-host` saves the three foreign platforms and is enough when only this machine needs to run the version. A self-built binary is not used automatically: `dev-install` places it at `~/.local/bin/k-playbook`, and only afterwards does the `k-playbook` call use it. This is also the way to work without network access: build rather than download.

## Verification

Checklist for a project:

- [ ] `.k-playbook/K-PLAYBOOK.yaml` is in the project root ([`k-playbook-format.md`](./k-playbook-format.md#finding-the-anchor)).
- [ ] `schema_version: 3` is set.
- [ ] `project.repo_root` points to the project repository; `project.vcs` is `git` or `none`.
- [ ] `.claude/commands`, `.claude/skills`, and `.opencode/commands` are directories with individual symlinks through `.k-playbook/share/` or to `.k-playbook/commands/` and `.k-playbook/skills/`; the interface reports them as configured.
- [ ] `CLAUDE.md` is a regular file with the `@AGENTS.md` line outside backticks and code blocks, and `AGENTS.md` carries the prompt. An actual carried-over `CLAUDE.md` was renamed to `AGENTS.md`; a symlink `CLAUDE.md` was replaced with the include file. If `Conflict` appears instead, resolve it manually; until then, Claude Code cannot see the prompt.
- [ ] `k-playbook version` prints the version of the installed program; `k-playbook version --all` names the newest release tag, and the program matches it or is newer.
- [ ] `k-playbook briefing` completes, names the expected instructions and reviews, and reports `Umgebung: keine Probleme`.
- [ ] With VS Code in the environment: `k-playbook vscode status` names the embedded
      version and finds it installed in `~/.vscode-server/extensions`.

The final item checks all preceding items at once: the command stops if configuration is missing or has a different `schema_version`.

## Troubleshooting

**Slash commands do not appear.** Start the interface: it compares the catalog with what is registered and names missing commands. Restart the assistant after setup.

**A new command from `.k-playbook/commands/` is missing.** It is not registered automatically: the interface reports it as missing and creates the link at the press of a button. The same applies when a project-owned file has newly replaced a shipped command; then the existing link still points to the old source.

**Skills are not triggered.** Every skill directory must contain `SKILL.md`; without it, the directory does not count as a skill and is not linked. Then restart the assistant.

**`schema_version` does not match.** If the number is below `3` or missing, configuration is older than the tool. The project is set up again; see [A configuration with an older schema_version](#a-configuration-with-an-older-schema_version). If it is higher, the reverse is true: the program is behind. Then `k-playbook upgrade` helps, not a reset, which would discard the newer file.

**The tool finds no project.** Then `.k-playbook/K-PLAYBOOK.yaml` is missing above the invocation location. Starting at the working directory, the search goes upwards to `$HOME` or `/` and deliberately does not guess. The interface then proposes a location. A `K-PLAYBOOK.yaml` directly in the project root is not an anchor.

**`k-playbook: command not found`.** Either the bootstrap has not yet run in this environment, in which case run it: `curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash`. Or `~/.local/bin` is missing from PATH. The bootstrap checks PATH itself and stops in that case before downloading anything.

**`cannot execute binary file`.** `~/.local/bin/k-playbook` contains the binary for another platform, typical with a host and DevContainer sharing `$HOME`. Run the bootstrap again in this environment; it detects the case and reports that the other environment also needs to run it again afterwards.

**The program is older than the newest release.** The running program is older than the newest release tag on GitHub; the health check warns, `k-playbook` says so on startup, and the interface offers `Programm aktualisieren (v0.9.3 → v0.9.4)`. One click installs the release binary of that tag, verified against its `SHA256SUMS`, and restarts the service from the new program; in the terminal, `k-playbook upgrade` does the same. If there is a note with two paths instead of a button, `k-playbook` in the `PATH` resolves to a different file than `~/.local/bin/k-playbook`; resolve that first, otherwise the next call would start the old program again. `k-playbook version` prints which version a file carries.

**The installation finds no asset.** The newest published release lacks the asset of this platform — created by hand, or being re-equipped by the release workflow right now. Nothing is replaced, and the message says the release is not finished. Wait until the release is complete and run `k-playbook upgrade` again, or build it yourself; see [Build it yourself](#build-it-yourself).
