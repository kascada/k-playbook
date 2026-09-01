# Dokumentation

k-playbook ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und
Checks. Er wird in ein Unterverzeichnis des Projekts geklont, das er begleiten soll.

## Einstieg

| Dokument | Inhalt |
|---|---|
| [`handbuch.md`](./handbuch.md) | Zweck, Grundmodell, Standardabläufe, Betriebsregeln. Die zentrale Seite. |
| [`installation.md`](./installation.md) | Clone, die vier Einrichtungsschritte, Security-Tools, Aktualisieren, Fehlersuche. |
| [`k-playbook-format.md`](./k-playbook-format.md) | Der Kontrakt: `K-PLAYBOOK.yaml`, Verzeichnisaufteilung, Overlay-Regeln. |
| [`commands.md`](./commands.md) | Index der Slash-Commands und ihrer Zuständigkeiten. |
| [`faq.md`](./faq.md) | Kurze Antworten zu Installation, Pfaden, Overlay und Security-Tools. |

## Detailseiten

| Dokument | Inhalt |
|---|---|
| [`code-review.md`](./code-review.md) | Ablauf von `/k-review`, `/k-remediation` und den Handoffs. |
| [`reviews-and-results.md`](./reviews-and-results.md) | Artefaktmodell: Ergebnisfamilien, Findings, Statuswerte, Priorisierung. |
| [`review-runs.md`](./review-runs.md) | Das Laufmodell: `run.json`, Einträge, Betriebsarten der Katalog-Rezepte, Merge, known-decisions. |
| [`task-flow.md`](./task-flow.md) | `/k-task-create`, `/k-task-refine`, `/k-task-run`. |
| [`pr-review.md`](./pr-review.md) | `/k-pr-review` für konkrete GitHub-Pull-Requests. |
| [`mcp.md`](./mcp.md) | Der MCP-Server: registrieren, Freigabe bei Claude Code, warum der Eintrag relativ ist. |
| [`local-github-ssh.md`](./local-github-ssh.md) | Host-spezifische GitHub-SSH-Aliases und Deploy-Keys. Kein Teil des Installationsvertrags. |
| [`schreibweise.md`](./schreibweise.md) | Umlaute statt ASCII-Umschreibung, und wo ASCII bleibt. Gilt für alle Texte des Repos. |

## Werkzeug und Kataloge

| Dokument | Inhalt |
|---|---|
| [`../installer/docs/architecture.md`](../installer/docs/architecture.md) | Architektur des Go-Werkzeugs: Anker finden, Verlinkung, Web-API, Designentscheidungen. |
| [`../installer/README.md`](../installer/README.md) | Kurzeinstieg und Prüfungen für Arbeiten am Werkzeug. |
| [`../checks/README.md`](../checks/README.md) | Schnittstelle und Nutzung von `bin/k-check`. |
| [`../rules/README.md`](../rules/README.md) | Die mitgelieferten Regeln. |
| [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv) | Kanonische Security-Tool-Matrix für Skript, Oberfläche und Review-Rezepte. |

## Skills

| Skill | Zweck |
|---|---|
| [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md) | Docs als autoritative Quelle für AI-Sessions verankern. |
| [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md) | Mitgelieferte und projekteigene Regeln während der Arbeit anwenden. |
| [`../skills/overlay-repo-analyse/PLAYBOOK.md`](../skills/overlay-repo-analyse/PLAYBOOK.md) | Docker-Overlay-Repos systematisch analysieren und dokumentieren. |

## Umstellung

[`umbau.md`](./umbau.md) ist die Arbeitsdatei zur Umstellung. Das projektlokale Modell ist
umgesetzt und in den Seiten oben beschrieben; in der Arbeitsdatei steht nur noch, was
festgelegt, aber noch nicht umgesetzt ist — derzeit der Umbau der Scan-Reviews auf SARIF.
Wenn nichts mehr offen ist, wird sie gelöscht.

## Stichwort-Index

- `Anker` / `K-PLAYBOOK.yaml` -> [`k-playbook-format.md`](./k-playbook-format.md)
- `AGENTS.md` / `CLAUDE.md` / `Verlinkung` / `Umbenennen` / `Konflikt` -> [`installation.md`](./installation.md#4-assistenten-verlinken)
- `Assistenten` / `Claude Code` / `OpenCode` / `Cursor` -> [`installation.md`](./installation.md#4-assistenten-verlinken)
- `BROWSER` / `Browser öffnet nicht` / `DevContainer` -> [`installation.md`](./installation.md#browser-beim-start), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#browser-öffnen)
- `Bereiche` / `Setup` / `Workflows` / `Umschalter` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md#bereiche-und-die-linke-spalte), [`installation.md`](./installation.md#reviews-und-tasks)
- `checks` / `k-check` -> [`../checks/README.md`](../checks/README.md), [`commands.md`](./commands.md#k-check)
- `Commands` -> [`commands.md`](./commands.md)
- `Docs zuerst` -> [`handbuch.md`](./handbuch.md#docs-first), [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md)
- `Enforcement` / `Regeln` -> [`../rules/README.md`](../rules/README.md), [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md)
- `Findings` / `Statuswerte` -> [`reviews-and-results.md`](./reviews-and-results.md#statusmodell)
- `gh` / `GitHub CLI` / `gh auth login` -> [`installation.md`](./installation.md#github-cli), [`k-playbook-format.md`](./k-playbook-format.md#toolsgh)
- `GitHub SSH` / `Deploy-Key` -> [`local-github-ssh.md`](./local-github-ssh.md)
- `Installation` / `git clone` -> [`installation.md`](./installation.md)
- `MCP` / `.mcp.json` / `mcpServers` / `Freigabe` -> [`mcp.md`](./mcp.md), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server)
- `k-playbook-local` / `projekteigen` -> [`k-playbook-format.md`](./k-playbook-format.md), [`installation.md`](./installation.md#2-projekteigene-struktur-anlegen)
- `Oberfläche` / `k-gui` / `Web-API` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md)
- `priv` / `material` / `privat` / `Lokale Einstellungen` -> [`installation.md`](./installation.md#2-projekteigene-struktur-anlegen), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#lokale-einstellungen)
- `Doku lesen` / `Markdown-Ansicht` / `Mermaid` -> [`installation.md`](./installation.md#doku-lesen), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#doku-in-der-oberfläche)
- `Overlay` / `Regel ersetzen` / `abschalten` -> [`k-playbook-format.md`](./k-playbook-format.md#mitgeliefertes-und-projekteigenes-zusammenfassen), [`faq.md`](./faq.md)
- `context` / `aufgelöster Arbeitsstand` -> [`k-playbook-format.md`](./k-playbook-format.md#der-aufgelöste-arbeitsstand), [`commands.md`](./commands.md#der-aufgelöste-arbeitsstand)
- `k-playbook.md` / `Instruktionen` / `Anstoß` -> [`k-playbook-format.md`](./k-playbook-format.md#instruktionen), [`faq.md`](./faq.md)
- `Altlasten` / `alte globale Verlinkung` -> [`installation.md`](./installation.md#4-assistenten-verlinken)
- `Command fehlt` / `toter Symlink` / `Verlinkung nachziehen` -> [`installation.md`](./installation.md#aktualisieren), [`../installer/docs/architecture.md`](../installer/docs/architecture.md#selbstheilung-auf-dem-lesepfad)
- `Pfade` / `warum keine paths` -> [`faq.md`](./faq.md), [`k-playbook-format.md`](./k-playbook-format.md#keine-pfade-in-der-konfiguration)
- `PR-Review` -> [`pr-review.md`](./pr-review.md)
- `Remediation` -> [`code-review.md`](./code-review.md#k-remediation), [`reviews-and-results.md`](./reviews-and-results.md#remediation)
- `Results` / `Ergebnisse` -> [`reviews-and-results.md`](./reviews-and-results.md)
- `/k-audit` / `review-scan-triage` / `review-triage` -> [`review-runs.md`](./review-runs.md#bewerten-mit-review-scan-triage), [`commands.md`](./commands.md#review-flow)
- `/k-task-refine` / `Task-Härtung` -> [`task-flow.md`](./task-flow.md)
- `review-input` / `merge` / `Zusammenfassen` -> [`review-runs.md`](./review-runs.md#zusammenfassen-mit-k-playbook-merge)
- `Belegvertrag` / `review-input.json`-Schema / `stableId`-Bildung -> [`../commands/_review-run/review-input-contract.md`](../commands/_review-run/review-input-contract.md)
- `audit.mode` / `Perspektive` / `Evidence-Rezept` / `scope.paths` / `ruleIds` -> [`review-runs.md`](./review-runs.md#katalog-rezepte-im-lauf), [`../rules/review-authoring.md`](../rules/review-authoring.md)
- `known-decisions` / `stableId` / `pathGlob` -> [`review-runs.md`](./review-runs.md#wirkung-von-known-decisionsmd)
- `Schreibweise` / `Umlaute` / `Rechtschreibung` -> [`schreibweise.md`](./schreibweise.md)
- `Reviews` -> [`code-review.md`](./code-review.md)
- `schema_version` -> [`k-playbook-format.md`](./k-playbook-format.md#schema_version)
- `Security-Tools` / `Tool-Matrix` -> [`installation.md`](./installation.md#security-tools), [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv)
- `Tasks` -> [`task-flow.md`](./task-flow.md)
- `Update` / `git pull` -> [`installation.md`](./installation.md#aktualisieren)
