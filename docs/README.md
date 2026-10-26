# Documentation

k-playbook is a toolkit of slash commands, skills, review recipes, rules, and
checks. It is cloned into a subdirectory of the project it is meant to support.

## Getting started

| Document | Content |
|---|---|
| [`overview.md`](./overview.md) | What k-playbook is for: the three kinds of work — building, diagnosis, quality — the means that apply to all of them, and the knowledge store they share. One diagram. |
| [`manual.md`](./manual.md) | Purpose, core model, standard workflows, operating rules. The central page. |
| [`installation.md`](./installation.md) | Clone, the four setup steps, security tools, updating, troubleshooting. |
| [`k-playbook-format.md`](./k-playbook-format.md) | The contract: `K-PLAYBOOK.yaml`, directory layout, overlay rules. |
| [`commands.md`](./commands.md) | Index of slash commands and their responsibilities. |
| [`faq.md`](./faq.md) | Short answers about installation, paths, overlays, and security tools. |

## Detail pages

| Document | Content |
|---|---|
| [`code-review.md`](./code-review.md) | Both review paths, `/k-audit` and `/k-review`: flow, division of labour, result families, artifacts, status values, and the handoff to `/k-remediation`. |
| [`review-runs.md`](./review-runs.md) | The run model: `run.json`, entries, operating modes of catalog recipes, merge, known decisions. |
| [`task-flow.md`](./task-flow.md) | `/k-task-create`, `/k-task-refine`, `/k-task-run`. |
| [`knowledge-gate.md`](./knowledge-gate.md) | **Entry page for the knowledge store.** Why it exists, what is built, how we proceed, how a deposit is classified by kind, origin, state and format, the query surface, the staged plan for the index, and the measurements the decisions rest on. Links the two pages below. |
| [`knowledge-layout.md`](./knowledge-layout.md) | The storage layout under `k-playbook-local`: the three zones `inbox/`, `queue/` and `knowledge/`, the ownership rule, the frontmatter contract, the table of what lands where, the write tools every deposit goes through, and the migration. |
| [`knowledge-storage.md`](./knowledge-storage.md) | The picture of the knowledge store: how knowledge flows through the three zones and back out through the index and the MCP server. Vectors and a database of their own are deferred. Shown in the interface under Knowledge. |
| [`pr-review.md`](./pr-review.md) | `/k-pr-review` for specific GitHub pull requests. |
| [`mcp.md`](./mcp.md) | The MCP server: registering it, approval in Claude Code, why *Set up* writes an absolute path and when the bare command name is written instead. |
| [`vscode.md`](./vscode.md) | The VS Code extension *k-playbook Workspace Tools*: what it contributes, its settings, how it is installed automatically when the service starts, which CLI is called, why the VSIX is checked in, and how to add another action. |
| [`version-inventory.md`](./version-inventory.md) | The version inventory contract: data model, pin taxonomy, sources, trust boundary, `version-sources.yaml`. |
| [`github-access.md`](./github-access.md) | The personal GitHub access file `~/.config/k-playbook/github.yaml`: services, access types (`gh`, `token`, `ssh`, `public`), `disabled`, the problems the reader reports, the template written when the file is missing, `k-playbook github check` / `show` with their JSON and exit codes, and the health area `github`. Step 1 (reading and checking) is implemented; availability and running under a service are outlook. |
| [`local-github-ssh.md`](./local-github-ssh.md) | Host-specific GitHub SSH aliases and deploy keys. Not part of the installation contract; absorbed into `github-access.md` in a later step, after availability. |
| [`kephalaion-client.md`](./kephalaion-client.md) | The Kephalaion tools k-playbook calls itself (`whoami`, `list`, `create`, `rename`), what it expects of them — `create`/`rename` through the hub, `name_taken`, `list` from the replica and the replica lag measured, `request_id` with one retry, `replayed`, `outcome_unknown` versus `node_unreachable`, minimum node v0.3.1 —, and where the tools themselves are documented. |
| [`verweisschema.md`](./verweisschema.md) | The reference scheme: one path with a prefix — `project:`, `k-playbook:`, `keph:` —, the access route per prefix, and how k-playbook resolves `k-playbook:` references and assembles directories such as `k-playbook:/instructions/always/`. |
| [`writing-style.md`](./writing-style.md) | Umlauts instead of ASCII transliteration, and where ASCII remains. Applies to all repository text. |

## Tool and catalogs

| Document | Content |
|---|---|
| [`../installer/docs/architecture.md`](../installer/docs/architecture.md) | Architecture of the Go tool: finding anchors, linking, web API, design decisions. |
| [`../installer/README.md`](../installer/README.md) | Quick start and checks for working on the tool. |
| [`../checks/README.md`](../checks/README.md) | Interface and use of `bin/k-check`. |
| [`../rules/README.md`](../rules/README.md) | The shipped rules. |
| [`../share/scripts/security-tools.tsv`](../share/scripts/security-tools.tsv) | Canonical security tool matrix for the script, interface, and review recipes. |

## Skills

| Skill | Purpose |
|---|---|
| [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md) | Establish docs as the authoritative source for AI sessions. |
| [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md) | Apply shipped and project-owned rules while working. |
| [`../skills/overlay-repo-analyse/PLAYBOOK.md`](../skills/overlay-repo-analyse/PLAYBOOK.md) | Systematically analyze and document Docker overlay repositories. |

## Migration

[`migration.md`](./migration.md) is the working document for the migration. The project-local
model has been implemented and is described on the pages above; the working document
contains only what has been decided but not yet implemented, currently migrating scan
reviews to SARIF. It is deleted when nothing remains open.

[`knowledge-aktuell.md`](./knowledge-aktuell.md) is the current working document for moving
k-playbook onto Kephalaion: a binary-only installation whose shipped content lives in the
public collection `k-playbook` on every hub, under `defaults/`, with the decisions so far, the open
questions and the ordered work list.

## Keyword index

- `anchor` / `K-PLAYBOOK.yaml` -> [`k-playbook-format.md`](./k-playbook-format.md)
- `AGENTS.md` / `CLAUDE.md` / `include` / `@AGENTS.md` / `linking` / `renaming` / `conflict` -> [`installation.md`](./installation.md#4-link-assistants)
- `assistants` / `Claude Code` / `OpenCode` -> [`installation.md`](./installation.md#4-link-assistants)
- `BROWSER` / `browser does not open` / `DevContainer` -> [`installation.md`](./installation.md#browser-on-startup), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#browser-öffnen)
- `sections` / `setup` / `workflows` / `switch` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md#bereiche-und-die-linke-spalte), [`installation.md`](./installation.md#reviews-and-tasks)
- `checks` / `k-check` -> [`../checks/README.md`](../checks/README.md), [`commands.md`](./commands.md#k-check)
- `commands` -> [`commands.md`](./commands.md)
- `docs first` -> [`manual.md`](./manual.md#docs-first), [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md)
- `enforcement` / `rules` -> [`../rules/README.md`](../rules/README.md), [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md)
- `findings` / `status values` -> [`code-review.md`](./code-review.md#status-model)
- `Kephalaion` / `collection` / `K-PLAYBOOK.local.yaml` / `whoami` -> [`installation.md`](./installation.md#kephalaion-collection), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#kephalaion-anbindung)
- `Kephalaion client` / `name_taken` / `rename` / `replica lag` / `request_id` / `replayed` / `outcome_unknown` / `kephalaion_unavailable` -> [`kephalaion-client.md`](./kephalaion-client.md)
- `user area` / `user/<name>/` / `workbench` / `queue/findings/` / `k_playbook_doc_create` / `k_playbook_doc_list` / `k_playbook_doc_done` / `k_playbook_doc_reopen` / `k-playbook doc` -> [`mcp.md`](./mcp.md#document-tools-in-the-collection), [`knowledge-aktuell.md`](./knowledge-aktuell.md)
- `gh` / `GitHub CLI` / `gh auth login` -> [`installation.md`](./installation.md#github-cli), [`k-playbook-format.md`](./k-playbook-format.md#toolsgh)
- `GitHub SSH` / `deploy key` -> [`local-github-ssh.md`](./local-github-ssh.md), [`github-access.md`](./github-access.md#ssh-key-or-host)
- `github.yaml` / `GitHub access file` / `service` / `k-playbook github` / `K_PLAYBOOK_GITHUB_CONFIG` / `health github` -> [`github-access.md`](./github-access.md), [`mcp.md`](./mcp.md#health-contract)
- `installation` / `git clone` -> [`installation.md`](./installation.md)
- `MCP` / `.mcp.json` / `mcpServers` / `approval` -> [`mcp.md`](./mcp.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server)
- `k-playbook-local` / `project-owned` -> [`k-playbook-format.md`](./k-playbook-format.md), [`installation.md`](./installation.md#2-create-project-owned-structure)
- `interface` / `k-gui` / `web API` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md)
- `health` / `k_playbook_health` / `k-playbook health` / `GET /api/environment` / `environment check` / `Umgebung: keine Probleme` / `briefing health` -> [`mcp.md`](./mcp.md#health-contract), [`mcp.md`](./mcp.md#briefing-contract)
- `gui status` / `k_playbook_gui_status` / `is the interface running` / `runtime file state` -> [`mcp.md`](./mcp.md#gui-status-contract), [`installation.md`](./installation.md#browser-on-startup), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#zustand-erfragen-gui-status)
- `notify` / `k_playbook_notify` / `k-playbook notify` / `desktop notification` / `toast` / `sound` / `done signal` -> [`mcp.md`](./mcp.md#notify-contract), [`task-flow.md`](./task-flow.md), [`../commands/_task/fertig-meldung.md`](../commands/_task/fertig-meldung.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#fertig-meldung-notify)
- `priv` / `material` / `private` / `local settings` -> [`installation.md`](./installation.md#2-create-project-owned-structure), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#lokale-einstellungen)
- `read docs` / `Markdown view` / `Mermaid` -> [`installation.md`](./installation.md#read-documentation), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#doku-in-der-oberfläche)
- `overlay` / `replace rule` / `disable` -> [`k-playbook-format.md`](./k-playbook-format.md#merge-shipped-and-project-owned-content), [`faq.md`](./faq.md)
- `context` / `resolved working state` -> [`k-playbook-format.md`](./k-playbook-format.md#the-resolved-working-state), [`commands.md`](./commands.md#the-resolved-working-state)
- `k-playbook.md` / `instructions` / `prompt` -> [`k-playbook-format.md`](./k-playbook-format.md#instructions), [`faq.md`](./faq.md)
- `legacy artifacts` / `old global linking` -> [`installation.md`](./installation.md#4-link-assistants)
- `command missing` / `dead symlink` / `update links` -> [`installation.md`](./installation.md#update), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#selbstheilung-auf-dem-lesepfad)
- `overview` / `what is it for` / `three kinds of work` / `big picture` -> [`overview.md`](./overview.md)
- `paths` / `why no paths` -> [`faq.md`](./faq.md), [`k-playbook-format.md`](./k-playbook-format.md#no-paths-in-configuration)
- `PR review` -> [`pr-review.md`](./pr-review.md)
- `reference` / `Verweis` / `project:` / `k-playbook:` / `keph:` / `instructions/` / `k-playbook read` / `k_playbook_doc_read` -> [`verweisschema.md`](./verweisschema.md)
- `briefing` / `k-playbook briefing` / `k_playbook_briefing` / `what applies` / `session start` -> [`mcp.md`](./mcp.md#briefing-contract), [`k-playbook-format.md`](./k-playbook-format.md#the-resolved-working-state), [`knowledge-aktuell.md`](./knowledge-aktuell.md#target-for-k-playbookmd-and-agentsmd-task-086)
- `remediation` -> [`code-review.md`](./code-review.md#k-remediation), [`k-playbook-format.md`](./k-playbook-format.md#remediation)
- `results` -> [`code-review.md`](./code-review.md)
- `/k-audit` / `review-scan-triage` / `review-triage` -> [`review-runs.md`](./review-runs.md#assessing-with-review-scan-triage), [`commands.md`](./commands.md#review-flow)
- `/k-task-refine` / `task hardening` -> [`task-flow.md`](./task-flow.md)
- `review-input` / `merge` / `consolidation` -> [`review-runs.md`](./review-runs.md#consolidating-with-k-playbook-merge)
- `evidence contract` / `review-input.json` schema / `stableId` generation -> [`../commands/_review-run/review-input-contract.md`](../commands/_review-run/review-input-contract.md)
- `audit.mode` / `perspective` / `evidence recipe` / `scope.paths` / `ruleIds` -> [`review-runs.md`](./review-runs.md#catalog-recipes-in-the-run), [`../rules/review-authoring.md`](../rules/review-authoring.md)
- `known decisions` / `stableId` / `pathGlob` -> [`review-runs.md`](./review-runs.md#effect-of-known-decisionsmd)
- `spelling` / `umlauts` / `orthography` -> [`writing-style.md`](./writing-style.md)
- `reviews` -> [`code-review.md`](./code-review.md)
- `review-code` / `code review recipe` / `diff of a task` -> [`code-review.md`](./code-review.md#code-review), [`../rules/review-authoring.md`](../rules/review-authoring.md)
- `schema_version` -> [`k-playbook-format.md`](./k-playbook-format.md#schema_version)
- `security tools` / `tool matrix` -> [`installation.md`](./installation.md#security-tools), [`../share/scripts/security-tools.tsv`](../share/scripts/security-tools.tsv)
- `tasks` -> [`task-flow.md`](./task-flow.md)
- `update` / `git pull` -> [`installation.md`](./installation.md#update)
- `version inventory` / `inventory` / `version-sources.yaml` / `pin type` / `trust boundary` -> [`version-inventory.md`](./version-inventory.md)
- `VS Code` / `extension` / `VSIX` / `OpenCode in a tab` / `k-playbook vscode` / `extensions.json` -> [`vscode.md`](./vscode.md)
