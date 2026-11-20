# Documentation

k-playbook is a toolkit of slash commands, skills, and review recipes.
It is installed as one binary per machine; a project carries only its anchor and what it owns.

The handbook pages live in [`../defaults/docs/handbook/`](../defaults/docs/handbook/) and are
shipped with every release; the interface shows them under "Docs". This directory keeps only
what is not shipped: this index and `knowledge-aktuell.md`.

## Getting started

| Document | Content |
|---|---|
| [`overview.md`](../defaults/docs/handbook/overview.md) | What k-playbook is for: the three kinds of work — building, diagnosis, quality — the means that apply to all of them, and the project documentation in `keph:/docs/` they share. One diagram. |
| [`manual.md`](../defaults/docs/handbook/manual.md) | Purpose, core model, standard workflows, operating rules. The central page. |
| [`installation.md`](../defaults/docs/handbook/installation.md) | Installing the binary, the three setup steps, security tools, updating, troubleshooting. |
| [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md) | The contract: `K-PLAYBOOK.yaml`, directory layout, overlay rules. |
| [`commands.md`](../defaults/docs/handbook/commands.md) | Index of slash commands and their responsibilities. |
| [`faq.md`](../defaults/docs/handbook/faq.md) | Short answers about installation, paths, overlays, and security tools. |

## Detail pages

| Document | Content |
|---|---|
| [`code-review.md`](../defaults/docs/handbook/code-review.md) | Both review paths, `/k-audit` and `/k-review`: flow, division of labour, result families, artifacts, status values, and the handoff to `/k-remediation`. |
| [`review-runs.md`](../defaults/docs/handbook/review-runs.md) | The run model: `run.json`, entries, operating modes of catalog recipes, merge, known decisions. |
| [`task-flow.md`](../defaults/docs/handbook/task-flow.md) | `/k-task-create`, `/k-task-refine`, `/k-task-run`. |
| [`mcp.md`](../defaults/docs/handbook/mcp.md) | The MCP server: registering it, approval in Claude Code, why *Set up* writes an absolute path and when the bare command name is written instead. |
| [`vscode.md`](../defaults/docs/handbook/vscode.md) | The VS Code extension *k-playbook Workspace Tools*: what it contributes (Kephalaion collections in the workspace), how it is installed automatically when the service starts, which CLI is called, why the VSIX is checked in, and how to add another action. |
| [`version-inventory.md`](../defaults/docs/handbook/version-inventory.md) | The version inventory contract: data model, pin taxonomy, sources, trust boundary, source configuration `keph:/k-playbook/inventory.md`. |
| [`github-access.md`](../defaults/docs/handbook/github-access.md) | The personal GitHub access file `~/.config/k-playbook/github.yaml`: services, access types (`gh`, `token`, `ssh`, `public`), `disabled`, the problems the reader reports, what every read says when the file is missing and `k-playbook github init`, which alone writes the template, `k-playbook github check` / `show` / `test` with their JSON and exit codes, the availability of an access without network, the test against GitHub (`k_playbook_github_test`, the interface page `/github-access`), the health area `github`, and `k-playbook github run`, which starts one command with the access of a service set up for that process. Steps 1, 2 and 3 (`run`) are implemented. |
| [`local-github-ssh.md`](../defaults/docs/handbook/local-github-ssh.md) | Host-specific GitHub SSH aliases and deploy keys. Not part of the installation contract; absorbed into `github-access.md` in a later step, after availability. |
| [`kephalaion-client.md`](../defaults/docs/handbook/kephalaion-client.md) | The Kephalaion tools k-playbook calls itself (`whoami`, `list`, `create`, `rename`), what it expects of them — `create`/`rename` through the hub, `name_taken`, `list` from the replica and the replica lag measured, `request_id` with one retry, `replayed`, `outcome_unknown` versus `node_unreachable`, minimum node v0.3.1 —, and where the tools themselves are documented. |
| [`verweisschema.md`](../defaults/docs/handbook/verweisschema.md) | The reference scheme: one path with a prefix — `project:`, `k-playbook:`, `keph:` —, the access route per prefix, and how k-playbook resolves `k-playbook:` references and assembles directories such as `k-playbook:/instructions/always/`. |

## Tool and catalogs

| Document | Content |
|---|---|
| [`../installer/docs/architecture.md`](../installer/docs/architecture.md) | Architecture of the Go tool: finding anchors, linking, web API, design decisions. |
| [`../installer/README.md`](../installer/README.md) | Quick start and checks for working on the tool. |
| [`../share/scripts/security-tools.tsv`](../share/scripts/security-tools.tsv) | Canonical security tool matrix for the script, interface, and review recipes. |

## Skills

| Skill | Purpose |
|---|---|
| [`../defaults/commands/_skills/overlay-repo-analyse.md`](../defaults/commands/_skills/overlay-repo-analyse.md) | Systematically analyze and document Docker overlay repositories; writes into `keph:/docs/generated/code/` through `k-playbook docs checkout\|publish code`. The skill under `share/skills/overlay-repo-analyse/` is a hull that reads this text as `k-playbook:/commands/_skills/overlay-repo-analyse.md`. |

## Migration

[`migration.md`](../defaults/docs/handbook/migration.md) is the working document for the migration. The project-local
model has been implemented and is described on the pages above; the working document
contains only what has been decided but not yet implemented, currently migrating scan
reviews to SARIF. It is deleted when nothing remains open.

[`knowledge-aktuell.md`](./knowledge-aktuell.md) is the current working document for moving
k-playbook onto Kephalaion: a binary-only installation whose shipped content lives in the
public collection `k-playbook` on every hub, under `defaults/`, with the decisions so far, the open
questions and the ordered work list.

## Keyword index

- `anchor` / `K-PLAYBOOK.yaml` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md)
- `AGENTS.md` / `CLAUDE.md` / `include` / `@AGENTS.md` / `linking` / `renaming` / `conflict` -> [`installation.md`](../defaults/docs/handbook/installation.md#3-link-assistants)
- `assistants` / `Claude Code` / `OpenCode` -> [`installation.md`](../defaults/docs/handbook/installation.md#3-link-assistants)
- `.k-playbook/share` / `K_PLAYBOOK_SHARE_DIR` / `commands missing in DevContainer` -> [`installation.md`](../defaults/docs/handbook/installation.md#3-link-assistants), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-umweg-link-k-playbookshare)
- `BROWSER` / `browser does not open` / `DevContainer` -> [`installation.md`](../defaults/docs/handbook/installation.md#browser-on-startup), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#browser-öffnen)
- `sections` / `setup` / `workflows` / `switch` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md#bereiche-und-die-linke-spalte), [`installation.md`](../defaults/docs/handbook/installation.md#reviews-and-tasks)
- `commands` -> [`commands.md`](../defaults/docs/handbook/commands.md)
- `docs first` -> [`manual.md`](../defaults/docs/handbook/manual.md#docs-first)
- `enforcement` / `docs follow-up` / `/k-enforcement` -> [`commands.md`](../defaults/docs/handbook/commands.md), [`manual.md`](../defaults/docs/handbook/manual.md#docs-follow-up)
- `queue/docs/` / `documentation orders` / `Doku-Aufträge` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#document-tools-in-the-collection), [`commands.md`](../defaults/docs/handbook/commands.md)
- `findings` / `status values` -> [`code-review.md`](../defaults/docs/handbook/code-review.md#status-model)
- `Kephalaion` / `collection` / `kephalaion:` block / `kephalaion node client` / `choice` / `whoami` -> [`installation.md`](../defaults/docs/handbook/installation.md#kephalaion-collection), [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#kephalaion), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#kephalaion-anbindung)
- `Kephalaion client` / `name_taken` / `rename` / `replica lag` / `request_id` / `replayed` / `outcome_unknown` / `kephalaion_unavailable` -> [`kephalaion-client.md`](../defaults/docs/handbook/kephalaion-client.md)
- `user area` / `user/<name>/` / `workbench` / `queue/findings/` / `k_playbook_doc_create` / `k_playbook_doc_list` / `k_playbook_doc_done` / `k_playbook_doc_reopen` / `k-playbook doc` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#document-tools-in-the-collection), [`knowledge-aktuell.md`](./knowledge-aktuell.md)
- `k_playbook_doc_append` / `k_playbook_doc_replace` / `k_playbook_doc_set_field` / `k_playbook_doc_grep` / `since_revision` / `foreign_change` / `k-playbook doc append` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#changing-and-searching-documents)
- `k_playbook_docs_checkout` / `k_playbook_docs_publish` / `k_playbook_docs_exclude_add` / `k-playbook docs` / `k-playbook docs exclude add` / `docs.exclude` / `context.docs` / `.k-playbook/work/` / `checkout record` / `hints` / `allow_hints` / `/k-docs-code` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#generated-docs-checkout-and-publish), [`commands.md`](../defaults/docs/handbook/commands.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#erzeugte-doku-checkout-und-publish), [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#docs)
- `k_playbook_docs_readme` / `k-playbook docs readme` / `README templates` / `defaults/docs/` / `k-playbook:/docs/` / `template_missing` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#generated-docs-checkout-and-publish), [`verweisschema.md`](../defaults/docs/handbook/verweisschema.md#access-route-per-prefix), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#readme-aus-vorlagen)
- `k-playbook maintain` / `maintenance:` / `docs-backup` / `keph-backup/docs` / `.k-playbook-backup` / `daily programme` / `maintenance.backup` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#maintenance), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#wartungslauf-maintain)
- `gh` / `GitHub CLI` / `gh auth login` -> [`installation.md`](../defaults/docs/handbook/installation.md#github-cli), [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#toolsgh)
- `GitHub SSH` / `deploy key` -> [`local-github-ssh.md`](../defaults/docs/handbook/local-github-ssh.md), [`github-access.md`](../defaults/docs/handbook/github-access.md#ssh-key-or-host)
- `github.yaml` / `GitHub access file` / `service` / `k-playbook github` / `K_PLAYBOOK_GITHUB_CONFIG` / `health github` / `availability` / `k_playbook_github_test` / `/github-access` / `github run` -> [`github-access.md`](../defaults/docs/handbook/github-access.md), [`mcp.md`](../defaults/docs/handbook/mcp.md#github-test-contract), [`mcp.md`](../defaults/docs/handbook/mcp.md#health-contract)
- `installation` / `bootstrap` / `bin/install` -> [`installation.md`](../defaults/docs/handbook/installation.md)
- `MCP` / `.mcp.json` / `mcpServers` / `approval` -> [`mcp.md`](../defaults/docs/handbook/mcp.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server)
- `project-owned` / `.k-playbook/` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md), [`installation.md`](../defaults/docs/handbook/installation.md)
- `interface` / `k-gui` / `web API` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md)
- `health` / `k_playbook_health` / `k-playbook health` / `GET /api/environment` / `environment check` / `Umgebung: keine Probleme` / `briefing health` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#health-contract), [`mcp.md`](../defaults/docs/handbook/mcp.md#briefing-contract)
- `gui status` / `k_playbook_gui_status` / `is the interface running` / `runtime file state` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#gui-status-contract), [`installation.md`](../defaults/docs/handbook/installation.md#browser-on-startup), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#zustand-erfragen-gui-status)
- `notify` / `k_playbook_notify` / `k-playbook notify` / `desktop notification` / `toast` / `sound` / `done signal` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#notify-contract), [`task-flow.md`](../defaults/docs/handbook/task-flow.md), [`../defaults/commands/_task/fertig-meldung.md`](../defaults/commands/_task/fertig-meldung.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#fertig-meldung-notify)
- `lock` / `k_playbook_lock` / `k-playbook lock` / `working-tree lock` / `Sperre` / `wait for another run` / `orphaned` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#working-tree-lock), [`task-flow.md`](../defaults/docs/handbook/task-flow.md#the-working-tree-lock), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#sperre-des-arbeitsbaums-lock)
- `read docs` / `Markdown view` / `Mermaid` -> [`installation.md`](../defaults/docs/handbook/installation.md#read-documentation), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#doku-in-der-oberfläche)
- `overlay` / `replace rule` / `disable` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#merge-shipped-and-project-owned-content), [`faq.md`](../defaults/docs/handbook/faq.md)
- `context` / `resolved working state` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#the-resolved-working-state), [`commands.md`](../defaults/docs/handbook/commands.md#the-resolved-working-state)
- `instructions` / `prompt` / `AGENTS.md` block -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#instructions), [`faq.md`](../defaults/docs/handbook/faq.md)
- `legacy artifacts` / `old global linking` -> [`installation.md`](../defaults/docs/handbook/installation.md#3-link-assistants)
- `command missing` / `dead symlink` / `update links` -> [`installation.md`](../defaults/docs/handbook/installation.md#update), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#selbstheilung-auf-dem-lesepfad)
- `overview` / `what is it for` / `three kinds of work` / `big picture` -> [`overview.md`](../defaults/docs/handbook/overview.md)
- `paths` / `why no paths` -> [`faq.md`](../defaults/docs/handbook/faq.md), [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#no-paths-in-configuration)
- `reference` / `Verweis` / `project:` / `k-playbook:` / `keph:` / `instructions/` / `k-playbook read` / `k_playbook_doc_read` -> [`verweisschema.md`](../defaults/docs/handbook/verweisschema.md)
- `briefing` / `k-playbook briefing` / `k_playbook_briefing` / `what applies` / `session start` -> [`mcp.md`](../defaults/docs/handbook/mcp.md#briefing-contract), [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#the-resolved-working-state), [`knowledge-aktuell.md`](./knowledge-aktuell.md#the-briefing-replaces-the-context-output-task-086)
- `remediation` -> [`code-review.md`](../defaults/docs/handbook/code-review.md#k-remediation), [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#remediation)
- `results` -> [`code-review.md`](../defaults/docs/handbook/code-review.md)
- `keph:/results/` / `keeping results` / `presenting results` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#no-paths-in-configuration)
- `/k-audit` / `review-scan-triage` / `review-triage` -> [`review-runs.md`](../defaults/docs/handbook/review-runs.md#assessing-with-review-scan-triage), [`commands.md`](../defaults/docs/handbook/commands.md#review-flow)
- `/k-task-refine` / `task hardening` -> [`task-flow.md`](../defaults/docs/handbook/task-flow.md)
- `review-input` / `merge` / `consolidation` -> [`review-runs.md`](../defaults/docs/handbook/review-runs.md#consolidating-with-k-playbook-merge)
- `evidence contract` / `review-input.json` schema / `stableId` generation -> [`../share/commands/_review-run/review-input-contract.md`](../share/commands/_review-run/review-input-contract.md)
- `audit.mode` / `perspective` / `evidence recipe` / `scope.paths` / `ruleIds` -> [`review-runs.md`](../defaults/docs/handbook/review-runs.md#catalog-recipes-in-the-run)
- `known decisions` / `stableId` / `pathGlob` -> [`review-runs.md`](../defaults/docs/handbook/review-runs.md#effect-of-known-decisionsmd)
- `reviews` -> [`code-review.md`](../defaults/docs/handbook/code-review.md)
- `review-code` / `code review recipe` / `diff of a task` -> [`code-review.md`](../defaults/docs/handbook/code-review.md#code-review)
- `schema_version` -> [`k-playbook-format.md`](../defaults/docs/handbook/k-playbook-format.md#schema_version)
- `security tools` / `tool matrix` -> [`installation.md`](../defaults/docs/handbook/installation.md#security-tools), [`../share/scripts/security-tools.tsv`](../share/scripts/security-tools.tsv)
- `tasks` -> [`task-flow.md`](../defaults/docs/handbook/task-flow.md)
- `update` / `k-playbook upgrade` / `old clone k-playbook/` -> [`installation.md`](../defaults/docs/handbook/installation.md#update)
- `version inventory` / `inventory` / `keph:/k-playbook/inventory.md` / `pin type` / `trust boundary` -> [`version-inventory.md`](../defaults/docs/handbook/version-inventory.md)
- `VS Code` / `extension` / `VSIX` / `OpenCode in a tab` / `k-playbook vscode` / `extensions.json` / `code-workspace` / `keph://` / `vscode collections` -> [`vscode.md`](../defaults/docs/handbook/vscode.md)
