# FAQ

## When do I call `/k-gui`?

`/k-gui` starts the interface. It is useful:

- after installing k-playbook, for the three setup steps in a project.
- when linking is stuck and you want to see why: it is brought up to date automatically, but an actual project file in the way or a conflict in `CLAUDE.md` does not resolve itself. The assistants section explains the resolution.
- when symlinks are broken.

If you only want to know whether everything is correct, open the interface and look without confirming any of the steps: it writes only after confirmation.

## Must I call `/k-gui` from a particular directory?

Preferably from the project you mean. Starting at the working directory, the tool searches upwards for `.k-playbook/K-PLAYBOOK.yaml` and uses the directory of the first match as the project root.

If it finds nothing, for example in a project that is not set up yet, it does not guess. Instead, it proposes a location and lets you confirm it. The strongest indication is the Git repository in which the command is called: whoever starts the tool is usually in the project they mean.

## I have several projects. Must I install k-playbook several times?

No. The installation is the binary alone, once per machine: once on the host and once in every DevContainer. The same `k-playbook` serves every project; which one it means follows from the directory in which it is called. A project needs only its anchor, `.k-playbook/K-PLAYBOOK.yaml`.

All projects of a machine therefore run with the same program version, and the shipped texts come from the Kephalaion hub of the project, in the release that hub carries.

## Does anything of the tool lie in the project?

No. Commands, skills and scripts are unpacked by the binary into `~/.local/share/k-playbook/`, and the shipped texts come from the hub. The project carries only its anchor and what it owns itself.

## Where is `K-PLAYBOOK.yaml`?

In the project root, under `.k-playbook/`: `.k-playbook/K-PLAYBOOK.yaml`. It belongs to the project, is committed with it and is never overwritten by an update:

```text
project/
└── .k-playbook/
    └── K-PLAYBOOK.yaml    the anchor
```

A `K-PLAYBOOK.yaml` directly in the project root is not an anchor: it is neither read nor moved, and the project counts as not set up (see [`k-playbook-format.md`](./k-playbook-format.md#finding-the-anchor)).

## Where are the paths for tasks, reviews, and results?

Nowhere: no path is configured. Tasks are not files: they are documents in the project's Kephalaion collection under `keph:/user/<user>/tasks/` (see [`task-flow.md`](./task-flow.md)). Project-owned review recipes are documents in the same collection under `keph:/k-playbook/reviews/`. What lies on disk, project-owned commands and skills under `.k-playbook/commands/` and `.k-playbook/skills/`, derives from the project root, the directory holding `.k-playbook/K-PLAYBOOK.yaml`.

Audit results belong to `<ergebnisort>`.
`<ergebnisort>` stands for the location of audit results. It is not decided yet; audit runs are paused until then.
The five run tools of the MCP server are blocked, and `/k-audit` and `/k-remediation` are temporarily unavailable. `/k-review` works: it keeps its result in a working directory, by default a new one under `/tmp/`.

`K-PLAYBOOK.yaml` has no `paths:` block: the structure is fixed, and a key that always has the same value would only be a source of errors. The complete mapping is in [`k-playbook-format.md`](./k-playbook-format.md).

## How do I change a shipped review recipe?

You do not: the shipped layer comes from the hub. Instead, create a document with the same name in the project collection under `keph:/k-playbook/reviews/` (task 143):

```text
<hub>:k-playbook/defaults/reviews/review-code.md    is no longer read
keph:/k-playbook/reviews/review-code.md             applies on its own
```

The local file **completely** replaces the shipped one. It must therefore contain the whole recipe; individual sections are not retained from the original. The cost is that later improvements to the original no longer reach this copy.

There are no checks any more (task 155): k-playbook ships none, and there are no project-owned ones either.

## How do I disable a shipped review recipe?

With an **empty** file of the same name. Because the local file completely replaces the shipped one, nothing remains for an empty file.

"Empty" means nothing except blank lines and comments. This lets the file state its own reason:

```text
# keph:/k-playbook/reviews/review-dependabot-alerts.md
# Disabled: this project is not on GitHub.
```

The entry remains visible in the catalog and its content states that it is disabled.

There is no list for this in the configuration.

## How do I know what ultimately applies?

```bash
k-playbook briefing
```

It outputs what applies: the project and its directories, the instructions, reviews and guidelines with shipped and project-owned content already merged, an origin for each entry that is not shipped, and disabled entries marked. The environment facts single commands need (remediation policy, base tools, `gh`) come with `k-playbook briefing --part environment`.

The interface presents the resolved paths and catalogs in readable form in the `Resolved context` section.

## Where do the instructions for a session live?

In the briefing. `AGENTS.md` in the project root contains only a brief prompt that names `k-playbook briefing`, the one call at the start of a session. The briefing carries the instructions that always apply as one text: the shipped parts, updated with every release, and after them the project-owned parts from the project collection under `keph:/k-playbook/instructions/always/`. Instructions for a particular job are read on demand through a reference (`k-playbook:/instructions/on-demand/<topic>.md`); their project-owned side lies under `keph:/k-playbook/instructions/on-demand/`. The entry point for a session is `k-playbook:/instructions/on-demand/k-playbook.md`: it sums up how k-playbook is laid out, how instructions are assembled, overridden and disabled, and how an `AGENTS.md` is split up, and a session reads it before it answers questions about k-playbook or creates, changes, moves or disables a project rule.

Content that applies only to one project — structure and special characteristics, conventions, recurring flows — belongs there or into the project documentation under `keph:/docs/`.

## Can I add my own slash commands?

Yes. `.k-playbook/commands/` holds them, and `.k-playbook/skills/` holds custom skills, both versioned next to the configuration. Setup does not create the two directories; create them when you put the first entry there. The same rule applies as for reviews: the same name replaces the shipped item, and an empty entry disables it.

```text
.k-playbook/commands/k-own.md    new, only in this project
.k-playbook/commands/k-todo.md   replaces the shipped item
.k-playbook/skills/my-skill/SKILL.md
```

The new command is registered automatically: at the next `k-playbook briefing`, the call at the beginning of every session, or when the assistants section of the interface is next displayed. Then restart the assistant; it reads commands at startup.

For commands, the path from `commands/` counts: a local `commands/_shared/x.md` replaces exactly that file, while the rest of the namespace remains shipped. A skill, on the other hand, is replaced as a whole: `SKILL.md` and accompanying files must match each other.

## Why are `.claude/commands/` and `.opencode/commands/` full of symlinks?

Because entries come from two sources. A directory symlink points to exactly one of them, so it would provide either only the shipped entries or only `.k-playbook/commands/`. The target is therefore an actual directory with one link for each command or skill, pointing to the version that applies under the overlay rule.

If one of these targets is a directory symlink, it is detected and replaced with individual links at the next synchronization.

An **actual file** that you placed there yourself is never replaced. It takes precedence, and the interface identifies it as project-owned.

## May a project run with a venv?

Yes. A project venv for project dependencies is normal. The read-only status of k-playbook security tools may inspect this active venv and identifies that in the interface. Only during installation must no project venv be active so that nothing is written into it. If `VIRTUAL_ENV` is set and installation is to happen:

```bash
deactivate
```

`.venv/bin`, `venv/bin`, or `env/bin` in `PATH` therefore block installations only, not the status display. `--method auto` is recommended. If Python CLI tools should explicitly be isolated in venvs, use dedicated k-playbook tool venvs:

```bash
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh --install missing --method venv
```

The destination is `~/.local/share/k-playbook/security-tools/<tool>-venv`, not `<project>/.venv`.

## How do I install missing security tools?

The interface displays the status read-only. Install through the script:

```bash
bash ~/.local/share/k-playbook/scripts/install-security-tools.sh --install missing
```

Without `--yes`, it shows the plan and asks. `--help` explains the `auto`, `native`, `docker`, `pipx`, and `venv` methods. It writes no project files and starts no scans.

There is no longer a dedicated `/k-install-security-tools` command. It would only duplicate the script in prose and would have needed updating whenever the script changed.

## Do I need Go?

No. The bootstrap `bin/install` is a pure shell script, called with `curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash`. It determines the newest published release (the redirect of `releases/latest` on GitHub, over HTTPS; `curl` or `wget` suffice, `git` is not needed), selects the release asset of its tag for the platform, macOS or Linux, verifies it against the `SHA256SUMS` committed at the tag, and installs it as an actual file under `~/.local/bin/k-playbook`. It runs once per host or DevContainer and requires network access; every invocation then runs through the installed `k-playbook`.

After that the program keeps itself current from the release: it compares its own version with the newest release tag, the health check warns when it is older, and `k-playbook upgrade` or the update button of the interface install the release binary of that tag, verified against its `SHA256SUMS`. `k-playbook version --all` shows both versions. Neither needs Go.

You need Go only if you work on the tool itself:

```bash
make dist
make gui
```
