# k-playbook Dokumentation

Diese Dokumentation beschreibt k-playbook als globales Repo fuer AI-Assistant-Workflows und als projektlokal registrierten Werkzeugkasten.

## Einstieg

| Dokument | Inhalt |
|---|---|
| [`handbuch.md`](./handbuch.md) | Zentrale Beschreibung: Zweck, Konzepte, Standardablaeufe, Betriebsregeln. |
| [`installation.md`](./installation.md) | Host-Installation, OpenCode-Setup, Security-Tools, optional Claude Code. |
| [`local-github-ssh.md`](./local-github-ssh.md) | Host-spezifische GitHub-SSH-Aliases und Deploy-Keys fuer `k-playbook` und `KamranApps`. |
| [`faq.md`](./faq.md) | Kurze Antworten zu `/k-gui`, Aufrufort, Projekt-venvs und Setup-Abgrenzung. |
| [`commands.md`](./commands.md) | Zuständigkeiten und Details der wichtigsten `/k-*`-Commands. |
| [`pr-review.md`](./pr-review.md) | Detailguide fuer PR-Reviews. |
| [`code-review.md`](./code-review.md) | Detailguide fuer `/k-review`, `/k-results`, `/k-remediation` und Handoffs. |
| [`task-flow.md`](./task-flow.md) | Detailguide fuer `/k-task-create`, `/k-review-loop` und `/k-run`. |
| [`k-playbook-format.md`](./k-playbook-format.md) | YAML-Format der projektlokalen `K-PLAYBOOK.yaml`-Konfiguration. |
| [`reviews-and-results.md`](./reviews-and-results.md) | Review-Familien, Result-Artefakte, Findings, Priorisierung und Remediation. |
| [`../installer/docs/architecture.md`](../installer/docs/architecture.md) | Installer-Architektur, Browser-GUI, Web-API, Designentscheidungen und Session-Memory fuer weitere Installer-Arbeiten. |
| [`../prompts/README.md`](../prompts/README.md) | Kopierbare AI-Assistenten-Prompts fuer gefuehrte Ablaeufe. |
| [`../global/checks/README.md`](../global/checks/README.md) | Schnittstelle und Nutzung von `global/bin/k-check`. |
| [`../global/rules/README.md`](../global/rules/README.md) | Globale Enforcement-Regeln. |
| [`../global/security-tools.tsv`](../global/security-tools.tsv) | Kanonische Security-Tool-Matrix fuer `/k-install-security-tools`, Installer-GUI und Security-Review-Rezepte. |

## Playbooks

| Playbook | Zweck |
|---|---|
| [`../ks-ai-session-memory/PLAYBOOK.md`](../ks-ai-session-memory/PLAYBOOK.md) | Docs als autoritative Quelle fuer AI-Sessions verankern. |
| [`../ks-enforcement/PLAYBOOK.md`](../ks-enforcement/PLAYBOOK.md) | Globale und projektlokale Regeln waehrend der Arbeit anwenden. |
| [`../ks-overlay-repo-analyse/PLAYBOOK.md`](../ks-overlay-repo-analyse/PLAYBOOK.md) | Docker-Overlay-Repos systematisch analysieren und dokumentieren. |

## Stichwort-Index

- `AGENTS.md` -> [`ks-ai-session-memory/PLAYBOOK.md`](../ks-ai-session-memory/PLAYBOOK.md)
- `Basisinstallation` / `Base-Repo` -> [`../AGENTS.md`](../AGENTS.md), [`handbuch.md`](./handbuch.md)
- `CodeQL` -> [`commands.md`](./commands.md), [`../global/rules/codeql.md`](../global/rules/codeql.md)
- `Commands` -> [`handbuch.md`](./handbuch.md#wichtige-commands), [`commands.md`](./commands.md)
- `Docs zuerst` -> [`handbuch.md`](./handbuch.md), [`../ks-ai-session-memory/PLAYBOOK.md`](../ks-ai-session-memory/PLAYBOOK.md)
- `Enforcement` -> [`handbuch.md`](./handbuch.md), [`../ks-enforcement/PLAYBOOK.md`](../ks-enforcement/PLAYBOOK.md)
- `Installation` -> [`installation.md`](./installation.md)
- `Installer` / `Browser-GUI` / `Web-API` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md)
- `Installer-Binary` / `Wrapper` / `bin/k-playbook-installer` -> [`installation.md`](./installation.md#installer-binary-und-launcher)
- `GitHub SSH` / `Deploy-Key` / `KamranApps` -> [`local-github-ssh.md`](./local-github-ssh.md)
- `K-PLAYBOOK.yaml` -> [`k-playbook-format.md`](./k-playbook-format.md), [`handbuch.md`](./handbuch.md), [`commands.md`](./commands.md)
- `k-gui` -> [`faq.md`](./faq.md), [`installation.md`](./installation.md), [`commands.md`](./commands.md)
- `k-check` -> [`../global/checks/README.md`](../global/checks/README.md)
- `Multi-Project` / `DevContainer-Installation` -> [`installation.md`](./installation.md), [`commands.md`](./commands.md)
- `Prompts` -> [`../prompts/README.md`](../prompts/README.md)
- `PR-Review` -> [`pr-review.md`](./pr-review.md)
- `Remediation` -> [`code-review.md`](./code-review.md), [`reviews-and-results.md`](./reviews-and-results.md)
- `Regeln in diesem Repo` -> [`../AGENTS.md`](../AGENTS.md), [`../global/rules/README.md`](../global/rules/README.md)
- `Reviews` -> [`code-review.md`](./code-review.md), [`reviews-and-results.md`](./reviews-and-results.md)
- `Security-Tools` -> [`installation.md`](./installation.md), [`commands.md`](./commands.md)
- `Security-Tool-Matrix` -> [`../global/security-tools.tsv`](../global/security-tools.tsv)
- `Results` -> [`code-review.md`](./code-review.md), [`reviews-and-results.md`](./reviews-and-results.md)
- `Tasks` -> [`task-flow.md`](./task-flow.md), [`handbuch.md`](./handbuch.md), [`commands.md`](./commands.md)
