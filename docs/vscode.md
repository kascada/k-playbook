# The VS Code extension

k-playbook ships its own VS Code extension, **k-playbook Workspace Tools**
(`kascada.k-playbook-workspace-tools`). It is a collection of workspace actions; the
first one opens OpenCode in a terminal tab inside the editor area.

The extension travels inside the program binary. You are not meant to install it
yourself: starting the interface once installs it and keeps it current.

## What it contributes

| Command | Title | Keybinding |
|---|---|---|
| `kPlaybook.openCode` | k-playbook: OpenCode im neuen Tab | `ctrl+alt+shift+o`, mac `cmd+alt+shift+o` |

The command opens a terminal **in the editor area** — a tab next to your files, not the
panel at the bottom — and starts the OpenCode attach program in it. The working directory
is the chosen workspace folder: with one folder it does not ask, with several it offers a
pick, and cancelling the pick does nothing. The command also sits as a button in the
editor title bar and appears in the command palette under the category `k-playbook`.
Interface texts of the extension are German, like the rest of the interface.

The extension runs in the workspace context (`extensionKind: ["workspace"]`), which is
what makes it work the same under WSL, Remote SSH and in a dev container: it is installed
into the VS Code *server* of that environment, not into the desktop client. It has no
dependencies, no webview and no server of its own, and it is independent of any other
extension.

## Settings

Both settings have `scope: "machine"`. No workspace may redirect the executed path — a
repository you open must not be able to decide what gets run on your machine.

| Setting | Default | Meaning |
|---|---|---|
| `kPlaybook.openCode.executable` | `opencode-attach` | The program to start. A value without `/` is looked up in `PATH`, a value with `/` is taken as a path. |
| `kPlaybook.openCode.args` | `[]` | Extra arguments. The folder is not passed as an argument; it becomes the terminal's working directory. |

The path is resolved in the extension host and handed to the terminal as an absolute
path. That is deliberate: measured on 2026-10-08, the extension host carries the `PATH`
of a login shell (under WSL including `~/.local/bin` and `~/.opencode/bin`), while the
`ptyHost` that starts terminals does not. A bare program name as `shellPath` would depend
on the environment VS Code hands to the individual terminal; an absolute path does not.
Resolving there also means a missing program is noticed where a message can be shown.

A missing program, a wrong path or a window without a workspace folder produces an error
message with an **Einstellung öffnen** button, not just a line in a terminal that may or
may not stay readable. What the started program writes afterwards stays in its terminal;
the extension does not read that output and is not meant to.

In a dev container the default only works if the program sits in one of the `PATH`
directories of the container. `devcontainer.json` decides that through `remoteEnv.PATH`;
if the program lives elsewhere, set `kPlaybook.openCode.executable` to an absolute path.

## How it gets installed

**Automatically, when the service starts.** Starting the interface (`k-playbook`, or
`/k-gui` from an assistant) starts a background service per project. That service checks
whether the extension is missing or carries a different version than the one embedded in
the program, and if so installs it — in a goroutine, so the start is never held up. The
result is one line in the service log. Nothing happens if there is no VS Code in the
environment: no installation, no message.

This is a deliberate exception to the rule that k-playbook does not install tools by
itself (see `installer/docs/architecture.md`). It was made because an explicit first
installation gets lost among the other setup steps. The exception is narrow: the only
trigger is the start of the service.

Only a **stamped** program does this. A binary built without build flags — an ad-hoc
`go build`, or the test binary of `cmd/k-playbook`, which starts itself in service mode for
one of its tests — carries no version and does not touch the VS Code of the machine.
`make dev-install`, `make dist` and the release build all stamp the version, so neither the
development loop nor an installed program is affected.

**`K_PLAYBOOK_NO_VSCODE_INSTALL` blocks every call of a VS Code CLI** while it is set to a
non-empty value. Set it if you want no automatic catch-up at all: the service then says
`gesperrt` as the result of its last catch-up instead of installing, and
`k-playbook vscode install` refuses with that reason.

The variable is what seals the test run, and it is why a test run cannot install into the
live editor — the mistake that "never test against the real `code`" forbids. Every test
binary sets it for itself, and every process it starts inherits it, however deep: a test
run builds and starts real, *stamped* programs as services, so the version stamp alone is
not enough, and keeping a clean `PATH` per test only works where someone remembered to.
`TestDienstRuehrtImTestlaufKeineCLIAn` is the counter-probe: it starts such a service with
a fake `code` in its `PATH` and goes red if anything calls it.

No window reload is needed. Measured on 2026-10-08: a freshly installed extension's
commands appear in the command palette without *Developer: Reload Window*, and the
extension is already active.

Two consequences worth knowing:

- **If you start the interface, you never install the extension by hand.** Whoever never
  starts the interface installs it explicitly with `k-playbook vscode install`.
- **Removing the extension does not stick.** The next start of the service installs it
  again, because the service compares versions and does not record a decision to remove
  it. Respecting a deliberate uninstall is a separate piece of work (todo #23). Until
  then, set `kPlaybook.openCode.executable` aside and simply ignore the command, or stop
  using the interface for that project.

### The explicit way

```bash
k-playbook vscode install          # install or catch up
k-playbook vscode status           # embedded version, installations, chosen CLI
k-playbook vscode vsix -o ext.vsix # write the VSIX only
k-playbook version --all           # program version plus the extension version
```

`k-playbook version` stays **one line** — the version and nothing else. The interface reads
it from the file at the install target to recognise a program change, and `ReadVersion`
discards an answer that carries more than one line. The extension version therefore sits
behind `version --all`, which is also where the committed VSIX is inspected, and that
counts as the way to see it.

`k-playbook vscode status` and the card **VS-Code-Erweiterung** on the setup page of the
interface show the same thing: the embedded version, what `extensions.json` lists, the CLI
a run would choose, the directory that CLI writes to, and the last automatic catch-up of
the running service. The same data is available as JSON at `GET /api/vscode`. A failed
catch-up is shown there with its reason instead of staying silent.

**"Needs a catch-up" refers to the directory the catch-up writes to**, not to "somewhere".
Which directory that is follows from the kind of CLI: the remote CLI and `code-server`
serve the server of the environment and write to `~/.vscode-server/extensions`, a `code`
from `PATH` writes to `~/.vscode/extensions`. If the extension sits in *both* directories
in different versions, a catch-up through one CLI cannot change what the other one serves.
That second installation is named separately — `Abweichend` on the card,
`Abweichend, von dieser CLI nicht erreichbar` in `vscode status` — instead of leaving the
state as a permanent "needs a catch-up" that no run can ever clear. It is caught up from
the environment that serves the other directory, or by hand from the VSIX.

Installing refuses under `sudo`: the extension belongs to the user who operates the
editor. Running as `root` without `sudo` — the normal case in a container — is allowed.

### Which CLI gets called

Installation always means `<cli> --install-extension <file> --force`, with a temporary
copy of the embedded VSIX. The CLI is chosen in this order:

1. **The remote CLI of the running server**, `~/.vscode-server/bin/<commit>/bin/remote-cli/code`,
   but only when `VSCODE_IPC_HOOK_CLI` points at a socket that still exists. That is the
   case in a terminal of VS Code, and it is the path a person takes.
2. **`code-server`**, the newest `~/.vscode-server/bin/<commit>/bin/code-server`. This is
   the server binary itself: it writes into the extension directory and `extensions.json`
   directly, without a window and without an IPC socket. This is the path the service
   takes, and the measurement that made the automatic installation possible — started
   detached and without any `VSCODE_*` variable, with a window open, it finished with exit
   0 in about one second.
3. **`code` from `PATH`**, but never below `/mnt/`.

Why the last restriction, as a precaution rather than a claim: under WSL the Windows shim
`/mnt/c/.../bin/code` detects WSL and forwards to exactly the remote CLI of the WSL
server, so it installs on the right side. But it only does so while
`ms-vscode-remote.remote-wsl` is installed on the Windows side — without it the script
falls through to the real Windows CLI — and on the way it will download a server through
`wslDownload.sh` if one is missing. An automatic step of a background service must not
trigger that side effect blindly, and the call goes through Windows interop and is far
slower than `code-server`. The order `code-server` before desktop `code` is right either
way.

If no CLI is found, nothing is installed and the interface names the explicit way: write
the VSIX with `k-playbook vscode vsix -o …` and use *Extensions: Install from VSIX…*.

Detection reads `extensions.json` only, never directory names. After
`--uninstall-extension` the entry disappears at once while the directory
`<id>-<version>/` stays behind for a while; a check on directory names would report a
removed extension as present.

Several project services can start at the same time. The installation therefore runs
under a `flock` on a file in the service's runtime directory (`$XDG_RUNTIME_DIR`, else
`$XDG_STATE_HOME`, else `~/.local/state`), and whoever waited re-checks afterwards —
usually finding nothing left to do.

## Why the VSIX is checked in

`installer/internal/vscodeext/vsix/k-playbook-workspace-tools.vsix` is a committed file,
embedded with `go:embed`. It is not built during `make dist`, and that is the point:

- **Reproducible builds.** `make release` writes `SHA256SUMS` from the locally built
  binaries, and CI rebuilds the tag and verifies against it. vsce writes the build time
  and the source files' mtimes into the zip, so a VSIX built per build would differ byte
  for byte and the check would go red.
- **No Node toolchain.** `make dist`, `make test` and the release CI work without Node.
  Node is a developer step for one target only.
- **Not in the clone.** Clones of different projects on one machine carry different
  states and would install older and newer versions in turn.

The extension therefore has a version of its own in `installer/vscode/package.json`,
starting at `0.1.0`, not the program's `VERSION`: the VSIX is committed before the tag
exists and cannot know the coming number.

## Building and testing

```bash
make vscode-vsix   # rebuild the VSIX with vsce (pinned), needs Node
make vscode-test   # node --test over installer/vscode/test/, needs Node
make test          # go test ./... plus vscode-test when node is present
```

Both targets need Node and say so clearly when it is missing. `make vscode-vsix` runs
vsce in `installer/vscode/`, not in a temporary directory — below `/tmp`, `vsce ls`
collected no file at all and reported a missing entrypoint instead, an error message that
points at the extension and means the working directory.

**The rebuilt VSIX belongs in the same commit as the source change.** `make test` goes red
otherwise: `TestVSIXPasstZurQuelle` compares every entry of the committed VSIX against its
source file under `installer/vscode/` and every shipped source file against the VSIX
(`package.json` by content, everything else byte for byte).

Any change under `installer/` requires a release through the VERSION guard, so a change to
the extension means a version bump. Releases need no Node.

The extension's tests use `node:test` against a stubbed `vscode` module. They never run
against a real VS Code: the remote CLI ignores `--extensions-dir` and would write into the
live environment.

## Adding another action

The extension is a collection, and a second action does not require rebuilding the first.

1. Add `installer/vscode/actions/<name>.js` exporting `{ id, run }`. The id follows
   `kPlaybook.<action>`.
2. List the module in the `actions` array in `installer/vscode/extension.js`.
3. Register it in `installer/vscode/package.json` under `contributes.commands` with the
   category `k-playbook`, plus `menus` and `keybindings` as needed.
4. Add a test under `installer/vscode/test/`.
5. Bump `version` in `installer/vscode/package.json`, run `make vscode-vsix`, and commit
   the VSIX with the change.

A test in `installer/vscode/test/extension.test.js` checks both directions: every action
is contributed as a command, and every contributed command has an action.
