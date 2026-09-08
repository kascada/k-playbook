# Documentation

k-playbook is a toolkit of slash commands, skills, review recipes, rules, and
checks. It is cloned into a subdirectory of the project it is meant to support.

## Getting started

| Document | Content |
|---|---|
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
| [`knowledge-storage.md`](./knowledge-storage.md) | Knowledge storage: the path from knowledge input through the MCP server and the versioned Markdown files into the local vector database. Shown in the interface under Knowledge. |
| [`pr-review.md`](./pr-review.md) | `/k-pr-review` for specific GitHub pull requests. |
| [`mcp.md`](./mcp.md) | The MCP server: registering it, approval in Claude Code, why the entry uses an absolute path. |
| [`version-inventory.md`](./version-inventory.md) | The version inventory contract: data model, pin taxonomy, sources, trust boundary, `version-sources.yaml`. |
| [`local-github-ssh.md`](./local-github-ssh.md) | Host-specific GitHub SSH aliases and deploy keys. Not part of the installation contract. |
| [`writing-style.md`](./writing-style.md) | Umlauts instead of ASCII transliteration, and where ASCII remains. Applies to all repository text. |

## Tool and catalogs

| Document | Content |
|---|---|
| [`../installer/docs/architecture.md`](../installer/docs/architecture.md) | Architecture of the Go tool: finding anchors, linking, web API, design decisions. |
| [`../installer/README.md`](../installer/README.md) | Quick start and checks for working on the tool. |
| [`../checks/README.md`](../checks/README.md) | Interface and use of `bin/k-check`. |
| [`../rules/README.md`](../rules/README.md) | The shipped rules. |
| [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv) | Canonical security tool matrix for the script, interface, and review recipes. |

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

## Keyword index

- `anchor` / `K-PLAYBOOK.yaml` -> [`k-playbook-format.md`](./k-playbook-format.md)
- `AGENTS.md` / `CLAUDE.md` / `include` / `@AGENTS.md` / `linking` / `renaming` / `conflict` -> [`installation.md`](./installation.md#4-link-assistants)
- `assistants` / `Claude Code` / `OpenCode` / `Cursor` -> [`installation.md`](./installation.md#4-link-assistants)
- `BROWSER` / `browser does not open` / `DevContainer` -> [`installation.md`](./installation.md#browser-on-startup), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#browser-öffnen)
- `sections` / `setup` / `workflows` / `switch` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md#bereiche-und-die-linke-spalte), [`installation.md`](./installation.md#reviews-and-tasks)
- `checks` / `k-check` -> [`../checks/README.md`](../checks/README.md), [`commands.md`](./commands.md#k-check)
- `commands` -> [`commands.md`](./commands.md)
- `docs first` -> [`manual.md`](./manual.md#docs-first), [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md)
- `enforcement` / `rules` -> [`../rules/README.md`](../rules/README.md), [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md)
- `findings` / `status values` -> [`code-review.md`](./code-review.md#status-model)
- `gh` / `GitHub CLI` / `gh auth login` -> [`installation.md`](./installation.md#github-cli), [`k-playbook-format.md`](./k-playbook-format.md#toolsgh)
- `GitHub SSH` / `deploy key` -> [`local-github-ssh.md`](./local-github-ssh.md)
- `installation` / `git clone` -> [`installation.md`](./installation.md)
- `MCP` / `.mcp.json` / `mcpServers` / `approval` -> [`mcp.md`](./mcp.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server)
- `k-playbook-local` / `project-owned` -> [`k-playbook-format.md`](./k-playbook-format.md), [`installation.md`](./installation.md#2-create-project-owned-structure)
- `interface` / `k-gui` / `web API` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md)
- `priv` / `material` / `private` / `local settings` -> [`installation.md`](./installation.md#2-create-project-owned-structure), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#lokale-einstellungen)
- `read docs` / `Markdown view` / `Mermaid` -> [`installation.md`](./installation.md#read-documentation), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#doku-in-der-oberfläche)
- `overlay` / `replace rule` / `disable` -> [`k-playbook-format.md`](./k-playbook-format.md#merge-shipped-and-project-owned-content), [`faq.md`](./faq.md)
- `context` / `resolved working state` -> [`k-playbook-format.md`](./k-playbook-format.md#the-resolved-working-state), [`commands.md`](./commands.md#the-resolved-working-state)
- `k-playbook.md` / `instructions` / `prompt` -> [`k-playbook-format.md`](./k-playbook-format.md#instructions), [`faq.md`](./faq.md)
- `legacy artifacts` / `old global linking` -> [`installation.md`](./installation.md#4-link-assistants)
- `command missing` / `dead symlink` / `update links` -> [`installation.md`](./installation.md#update), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#selbstheilung-auf-dem-lesepfad)
- `paths` / `why no paths` -> [`faq.md`](./faq.md), [`k-playbook-format.md`](./k-playbook-format.md#no-paths-in-configuration)
- `PR review` -> [`pr-review.md`](./pr-review.md)
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
- `schema_version` -> [`k-playbook-format.md`](./k-playbook-format.md#schema_version)
- `security tools` / `tool matrix` -> [`installation.md`](./installation.md#security-tools), [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv)
- `tasks` -> [`task-flow.md`](./task-flow.md)
- `update` / `git pull` -> [`installation.md`](./installation.md#update)
- `version inventory` / `inventory` / `version-sources.yaml` / `pin type` / `trust boundary` -> [`version-inventory.md`](./version-inventory.md)
