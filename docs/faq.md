# FAQ

## When do I call `/k-gui`?

`/k-gui` starts the interface. It is useful:

- after cloning k-playbook into a project, for the three setup steps.
- when linking is stuck and you want to see why: it is brought up to date automatically, but an actual project file in the way or a conflict in `CLAUDE.md` does not resolve itself. The assistants section explains the resolution.
- when symlinks are broken. It no longer creates `k-playbook-local/` or anything in it; a missing directory there is not a defect.

If you only want to know whether everything is correct, open the interface and look without confirming any of the steps: it writes only after confirmation.

## Must I call `/k-gui` from a particular directory?

Preferably from the project you mean. Starting at the working directory, the tool searches upwards for `.k-playbook/K-PLAYBOOK.yaml` and uses the directory of the first match as the project root.

If it finds nothing, for example immediately after cloning, it does not guess. Instead, it proposes a location and lets you confirm it. The strongest indication is the Git repository in which the command is called: whoever starts the tool is usually in the project they mean.

## I have several projects. Must I install k-playbook several times?

Yes, and that is intentional. Every project gets its own clone under `<project>/k-playbook/`.

The advantage is that projects can use different versions. A project that is not currently being touched remains at its version, and an update in another project does not change that. There is no central installation that affects all of them at once and no host path that must work on every machine.

## Must the directory be named `k-playbook`?

Yes. Commands and skills refer to it by that name. The name of the project directory above it does not matter.

```bash
git clone https://github.com/kascada/k-playbook.git
```

Without a destination argument, the name is derived from the repository name and is therefore correct automatically. You need your own argument only for a fork or mirror cloned under a different name; then it must be `k-playbook`.

## Why is `K-PLAYBOOK.yaml` not in `k-playbook/`?

Because `k-playbook/` is completely replaced with every update. Everything owned by the project must sit beside it, otherwise it would be gone after the next `git pull`.

Therefore, `.k-playbook/K-PLAYBOOK.yaml` is in the project root. `k-playbook-local/` beside it
held project-owned files; it is being phased out and no longer created, and `/k-upgrade`
deletes it once nothing is left in it but a `.gitignore`, a `README.md` and empty
subdirectories (3.7, task 150):

```text
project/
├── .k-playbook/
│   └── K-PLAYBOOK.yaml    the anchor
├── k-playbook/            replaceable
└── k-playbook-local/      being phased out; deleted by /k-upgrade once empty
```

Until 2026-11 the anchor was `K-PLAYBOOK.yaml` directly in the project root. The tool moves such a file into `.k-playbook/` on its first call; an older program then no longer finds the project, so every environment of the project needs the new program.

## Where are the paths for tasks, reviews, and results?

Nowhere: they derive from the project root, the directory holding `.k-playbook/K-PLAYBOOK.yaml`. Results are under `k-playbook-local/results/` and shipped checks under `k-playbook/checks/`. Tasks are not files at all any more: they are documents in the project's Kephalaion collection under `keph:/user/<user>/tasks/` (see [`task-flow.md`](./task-flow.md)); `/k-upgrade` 3.3 moves older task files from `k-playbook-local/tasks/` there, and 3.4 removes the directory once it is empty; setup no longer creates it. 3.7 finally deletes `k-playbook-local/` itself once it is empty.

Earlier versions used a `paths:` block with nine keys. It has been removed: the structure is fixed, and a key that always has the same value would only be a source of errors. The complete mapping is in [`k-playbook-format.md`](./k-playbook-format.md).

## How do I change a shipped review recipe?

You do not: the shipped layer comes from the hub. Instead, create a document with the same name in the project collection under `keph:/k-playbook/reviews/` (task 143; `k-playbook-local/reviews/` is no longer read):

```text
<hub>:k-playbook/defaults/reviews/review-code.md    is no longer read
keph:/k-playbook/reviews/review-code.md             applies on its own
```

The local file **completely** replaces the shipped one. It must therefore contain the whole recipe; individual sections are not retained from the original. The cost is that later improvements to the original no longer reach this copy.

Shipped checks cannot be changed this way: there are no project-owned checks since task 134. `k-check` runs only the shipped checks from `k-playbook/checks/`, and a leftover `k-playbook-local/checks/` is not read; `/k-upgrade` removes it.

## How do I disable a shipped review recipe?

With an **empty** file of the same name. Because the local file completely replaces the shipped one, nothing remains for an empty file.

"Empty" means nothing except blank lines and comments. This lets the file state its own reason:

```text
# keph:/k-playbook/reviews/review-dependabot-alerts.md
# Disabled: this project is not on GitHub.
```

The entry remains visible in the catalog and its content states that it is disabled.

A shipped check cannot be disabled for a single project: checks have no project-owned side. A check that does not apply skips on its own.

There is no list for this in the configuration.

## How do I know what ultimately applies?

```bash
k-playbook context
```

It outputs the resolved working state as JSON: directories, remediation policy, and the catalogs, with shipped and project-owned content already merged, an origin for each entry, and disabled entries marked.

The interface presents the same information in readable form in the `Resolved context` section.

## Where do the instructions for a session live?

In the briefing. `AGENTS.md` in the project root contains only a brief prompt that names `k-playbook briefing`, the one call at the start of a session. The briefing carries the instructions that always apply as one text: the shipped parts, updated with every release, and after them the project-owned parts from the project collection under `keph:/k-playbook/instructions/always/`. Instructions for a particular job are read on demand through a reference (`k-playbook:/instructions/on-demand/<topic>.md`); their project-owned side lies under `keph:/k-playbook/instructions/on-demand/`.

Content that applies only to one project — structure and special characteristics, conventions, recurring flows — belongs there or into the project documentation under `keph:/docs/`. The former instruction file `k-playbook.md`, once shipped and once project-owned, is gone: nothing reads it any more, and the context output has no field `instructions` (task 148).

## Can I add my own slash commands?

Yes. `.k-playbook/commands/` holds them, and `.k-playbook/skills/` holds custom skills, both versioned next to the configuration. Setup does not create the two directories; create them when you put the first entry there. The same rule applies as for reviews: the same name replaces the shipped item, and an empty entry disables it.

```text
.k-playbook/commands/k-own.md    new, only in this project
.k-playbook/commands/k-todo.md   replaces the shipped item
.k-playbook/skills/my-skill/SKILL.md
```

The former place, `k-playbook-local/commands/` and `k-playbook-local/skills/`, is no longer read since task 129; `/k-upgrade` names what is left there with its target.

The new command is registered automatically: at the next `k-playbook briefing`, the call at the beginning of every session, or when the assistants section of the interface is next displayed. Then restart the assistant; it reads commands at startup.

For commands, the path from `commands/` counts: a local `commands/_shared/x.md` replaces exactly that file, while the rest of the namespace remains shipped. A skill, on the other hand, is replaced as a whole: `SKILL.md` and accompanying files must match each other.

## Why are `.claude/commands/` and `.opencode/commands/` full of symlinks?

Because entries come from two sources. A directory symlink points to exactly one of them, so it would provide either only the shipped entries or only `.k-playbook/commands/`. The target is therefore an actual directory with one link for each command or skill, pointing to the version that applies under the overlay rule.

Older installations still have one directory symlink there. It is detected and replaced with individual links at the next synchronization.

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

No. The `k-playbook/bin/install` bootstrap is a pure shell script. It determines the newest published release (the redirect of `releases/latest` on GitHub, over HTTPS; `curl` or `wget` suffice, `git` is not needed), selects the release asset of its tag for the platform, macOS or Linux, verifies it against the `SHA256SUMS` committed at the tag, and installs it as an actual file under `~/.local/bin/k-playbook`. It does not read `VERSION` or `SHA256SUMS` of a clone and also runs without one: `curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash`. It runs once per host or DevContainer and requires network access; every invocation then runs through the installed `k-playbook`.

After that the program keeps itself current from the release: it compares its own version with the newest release tag, the health check warns when it is older, and `k-playbook upgrade` or the update button of the interface install the release binary of that tag, verified against its `SHA256SUMS`. `k-playbook version --all` shows both versions. Neither needs the clone or Go.

You need Go only if you work on the tool itself:

```bash
make dist
make gui
```
