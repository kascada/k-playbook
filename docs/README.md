# k-playbook Dokumentation

Diese Dokumentation beschreibt k-playbook als globales Repo fuer AI-Assistant-Workflows und als projektlokal registrierten Werkzeugkasten.

## Einstieg

| Dokument | Inhalt |
|---|---|
| [`handbuch.md`](./handbuch.md) | Zentrale Beschreibung: Zweck, Konzepte, Standardablaeufe, Betriebsregeln. |
| [`installation.md`](./installation.md) | Host-Installation, OpenCode-Setup, Security-Tools, optional Claude Code. |
| [`multi-project-installation.md`](./multi-project-installation.md) | Zentrale Installation fuer mehrere Zielprojekte, Python-/venv- und DevContainer-Workflows. |
| [`faq.md`](./faq.md) | Kurze Antworten zu `/k-install`, Aufrufort, Projekt-venvs und Setup-Abgrenzung. |
| [`commands.md`](./commands.md) | Zuständigkeiten und Details der wichtigsten `/k-*`-Commands. |
| [`reviews-and-results.md`](./reviews-and-results.md) | Review-Familien, Result-Artefakte, Findings, Priorisierung und Remediation. |
| [`../prompts/README.md`](../prompts/README.md) | Kopierbare AI-Assistenten-Prompts fuer gefuehrte Ablaeufe. |
| [`../global/checks/README.md`](../global/checks/README.md) | Schnittstelle und Nutzung von `global/bin/k-check`. |
| [`../global/rules/README.md`](../global/rules/README.md) | Globale Enforcement-Regeln. |

## Playbooks

| Playbook | Zweck |
|---|---|
| [`../ks-ai-session-memory/PLAYBOOK.md`](../ks-ai-session-memory/PLAYBOOK.md) | Docs als autoritative Quelle fuer AI-Sessions verankern. |
| [`../ks-enforcement/PLAYBOOK.md`](../ks-enforcement/PLAYBOOK.md) | Globale und projektlokale Regeln waehrend der Arbeit anwenden. |
| [`../ks-overlay-repo-analyse/PLAYBOOK.md`](../ks-overlay-repo-analyse/PLAYBOOK.md) | Docker-Overlay-Repos systematisch analysieren und dokumentieren. |

## Stichwort-Index

- `AGENTS.md` -> [`ks-ai-session-memory/PLAYBOOK.md`](../ks-ai-session-memory/PLAYBOOK.md)
- `CodeQL` -> [`commands.md`](./commands.md), [`../global/rules/codeql.md`](../global/rules/codeql.md)
- `Docs zuerst` -> [`handbuch.md`](./handbuch.md), [`../ks-ai-session-memory/PLAYBOOK.md`](../ks-ai-session-memory/PLAYBOOK.md)
- `Enforcement` -> [`handbuch.md`](./handbuch.md), [`../ks-enforcement/PLAYBOOK.md`](../ks-enforcement/PLAYBOOK.md)
- `Installation` -> [`installation.md`](./installation.md)
- `K-PLAYBOOK.MD` -> [`handbuch.md`](./handbuch.md), [`commands.md`](./commands.md)
- `k-install` -> [`faq.md`](./faq.md), [`installation.md`](./installation.md), [`commands.md`](./commands.md)
- `k-check` -> [`../global/checks/README.md`](../global/checks/README.md)
- `Multi-Project` -> [`multi-project-installation.md`](./multi-project-installation.md)
- `OKF` / `Open Knowledge Format` -> [`handbuch.md`](./handbuch.md), [`commands.md`](./commands.md)
- `Prompts` -> [`../prompts/README.md`](../prompts/README.md)
- `Remediation` -> [`reviews-and-results.md`](./reviews-and-results.md)
- `Reviews` -> [`reviews-and-results.md`](./reviews-and-results.md)
- `Security-Tools` -> [`installation.md`](./installation.md), [`commands.md`](./commands.md)
- `Tasks` -> [`handbuch.md`](./handbuch.md), [`commands.md`](./commands.md)
