# FAQ

## When do I call `/k-gui`?

`/k-gui` starts the interface. It is useful:

- after cloning k-playbook into a project, for the four setup steps.
- when linking is stuck and you want to see why: it is brought up to date automatically, but an actual project file in the way or a conflict in `CLAUDE.md` does not resolve itself. The assistants section explains the resolution.
- when parts of the project-owned structure are missing or symlinks are broken.

If you only want to know whether everything is correct, open the interface and look without confirming any of the steps: it writes only after confirmation.

## Must I call `/k-gui` from a particular directory?

Preferably from the project you mean. Starting at the working directory, the tool searches upwards for `K-PLAYBOOK.yaml` and uses the first match as the project root.

If it finds nothing, for example immediately after cloning, it does not guess. Instead, it proposes a location and lets you confirm it. The strongest indication is the Git repository in which the command is called: whoever starts the tool is usually in the project they mean.

## I have several projects. Must I install k-playbook several times?

Yes, and that is intentional. Every project gets its own clone under `<project>/k-playbook/`.

The advantage is that projects can use different versions. A project that is not currently being touched remains at its version, and an update in another project does not change that. There is no central installation that affects all of them at once and no host path that must work on every machine.

## Must the directory be named `k-playbook`?

Yes. Commands and skills refer to it by that name. The name of the project directory above it does not matter.

```bash
git clone git@github.com:kascada/k-playbook.git
```

Without a destination argument, the name is derived from the repository name and is therefore correct automatically. You need your own argument only for a fork or mirror cloned under a different name; then it must be `k-playbook`.

## Why is `K-PLAYBOOK.yaml` not in `k-playbook/`?

Because `k-playbook/` is completely replaced with every update. Everything owned by the project must sit beside it, otherwise it would be gone after the next `git pull`.

Therefore, `K-PLAYBOOK.yaml` and `k-playbook-local/` are in the project root:

```text
project/
├── K-PLAYBOOK.yaml        the anchor
├── k-playbook/            replaceable
└── k-playbook-local/      project-owned
```

## Where are the paths for tasks, reviews, and results?

Nowhere: they derive from the location of `K-PLAYBOOK.yaml`. Tasks are under `k-playbook-local/tasks/`, results under `k-playbook-local/results/`, and shipped rules under `k-playbook/rules/`.

Earlier versions used a `paths:` block with nine keys. It has been removed: the structure is fixed, and a key that always has the same value would only be a source of errors. The complete mapping is in [`k-playbook-format.md`](./k-playbook-format.md).

## How do I change a shipped rule?

You do not: `k-playbook/` is replaced during updates. Instead, create a file with the same name under `k-playbook-local/`:

```text
k-playbook/rules/docs-sync.md          is no longer read
k-playbook-local/rules/docs-sync.md    applies on its own
```

The local file **completely** replaces the shipped one. It must therefore contain the whole rule; individual sections are not retained from the original. The cost is that later improvements to the original no longer reach this copy.

The same applies to `reviews/` and `checks/`.

## How do I disable a shipped rule?

With an **empty** file of the same name. Because the local file completely replaces the shipped one, nothing remains for an empty file.

"Empty" means nothing except blank lines and comments. This lets the file state its own reason:

```bash
# k-playbook-local/checks/check_django_baseline.sh
# Disabled: this project does not use Django.
```

For `rules` and `reviews`, the entry remains visible in the catalog and its content states that it is disabled. A check disappears from the catalog entirely: an empty script would run with exit 0 and look like a passing check.

There is no list for this in the configuration.

## How do I know what ultimately applies?

```bash
k-playbook context
```

It outputs the resolved working state as JSON: directories, instruction files in read order, remediation policy, guidelines, and the three catalogs, with shipped and project-owned content already merged, an origin for each entry, and disabled entries marked.

The interface presents the same information in readable form in the `Resolved context` section.

## What is `k-playbook.md`?

The instruction file: what an assistant should read before working. It exists twice:

| File | Applies to | On update |
|---|---|---|
| `k-playbook/k-playbook.md` | every project that uses k-playbook | is replaced |
| `k-playbook-local/k-playbook.md` | this project only | remains |

They are read in this order. The project-owned layer is where content that applies only here belongs: project structure and special characteristics, conventions, recurring flows. General k-playbook rules do not belong there: they are in the shipped layer and are updated with every update.

It is deliberately not named `AGENTS.md`: assistants read that name themselves, and it is reserved for the project root. It contains only a brief prompt that refers to `k-playbook context`.

## Can I add my own slash commands?

Yes. `k-playbook-local/commands/` holds them, and `k-playbook-local/skills/` holds custom skills. The same rule applies as for rules and reviews: the same name replaces the shipped item, and an empty entry disables it.

```text
k-playbook-local/commands/k-own.md    new, only in this project
k-playbook-local/commands/k-todo.md   replaces the shipped item
k-playbook-local/skills/my-skill/SKILL.md
```

The new command is registered automatically: at the next `k-playbook context`, the call at the beginning of every session, or when the assistants section of the interface is next displayed. Then restart the assistant; it reads commands at startup.

For commands, the path from `commands/` counts: a local `commands/_shared/x.md` replaces exactly that file, while the rest of the namespace remains shipped. A skill, on the other hand, is replaced as a whole: `SKILL.md` and accompanying files must match each other.

## Why are `.claude/commands/` and `.opencode/commands/` full of symlinks?

Because entries come from two sources. A directory symlink points to exactly one of them, so it would provide either only `k-playbook/` or only `k-playbook-local/`. The target is therefore an actual directory with one link for each command or skill, pointing to the version that applies under the overlay rule.

Older installations still have one directory symlink there. It is detected and replaced with individual links at the next synchronization.

An **actual file** that you placed there yourself is never replaced. It takes precedence, and the interface identifies it as project-owned.

## May a project run with a venv?

Yes. A project venv for project dependencies is normal. The read-only status of k-playbook security tools may inspect this active venv and identifies that in the interface. Only during installation must no project venv be active so that nothing is written into it. If `VIRTUAL_ENV` is set and installation is to happen:

```bash
deactivate
```

`.venv/bin`, `venv/bin`, or `env/bin` in `PATH` therefore block installations only, not the status display. `--method auto` is recommended. If Python CLI tools should explicitly be isolated in venvs, use dedicated k-playbook tool venvs:

```bash
k-playbook/scripts/install-security-tools.sh --install missing --method venv
```

The destination is `~/.local/share/k-playbook/security-tools/<tool>-venv`, not `<project>/.venv`.

## How do I install missing security tools?

The interface displays the status read-only. Install through the script:

```bash
k-playbook/scripts/install-security-tools.sh --install missing
```

Without `--yes`, it shows the plan and asks. `--help` explains the `auto`, `native`, `docker`, `pipx`, and `venv` methods. It writes no project files and starts no scans.

There is no longer a dedicated `/k-install-security-tools` command. It would only duplicate the script in prose and would have needed updating whenever the script changed.

## Do I need Go?

No. The `k-playbook/bin/install` bootstrap is a pure shell script. It selects the release asset for the platform, macOS or Linux, verifies it against the shipped `SHA256SUMS`, and installs it as an actual file under `~/.local/bin/k-playbook`. It runs once per host or DevContainer and requires network access; every invocation then runs through the installed `k-playbook`.

You need Go only if you work on the tool itself:

```bash
make dist
make gui
```
