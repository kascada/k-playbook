# Dokumentation

k-playbook ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und
Checks. Er wird in ein Unterverzeichnis des Projekts geklont, das er begleiten soll.

## Einstieg

| Dokument | Inhalt |
|---|---|
| [`handbuch.md`](./handbuch.md) | Zweck, Grundmodell, Standardablaeufe, Betriebsregeln. Die zentrale Seite. |
| [`installation.md`](./installation.md) | Clone, die drei Einrichtungsschritte, Security-Tools, Aktualisieren, Fehlersuche. |
| [`k-playbook-format.md`](./k-playbook-format.md) | Der Kontrakt: `K-PLAYBOOK.yaml`, Verzeichnisaufteilung, Overlay-Regeln. |
| [`commands.md`](./commands.md) | Index der Slash-Commands und ihrer Zustaendigkeiten. |
| [`faq.md`](./faq.md) | Kurze Antworten zu Installation, Pfaden, Overlay und Security-Tools. |

## Detailseiten

| Dokument | Inhalt |
|---|---|
| [`code-review.md`](./code-review.md) | Ablauf von `/k-review`, `/k-results`, `/k-remediation` und den Handoffs. |
| [`reviews-and-results.md`](./reviews-and-results.md) | Artefaktmodell: Ergebnisfamilien, Findings, Statuswerte, Priorisierung. |
| [`task-flow.md`](./task-flow.md) | `/k-task-create`, `/k-review-loop`, `/k-run`. |
| [`pr-review.md`](./pr-review.md) | `/k-pr-review` fuer konkrete GitHub-Pull-Requests. |
| [`local-github-ssh.md`](./local-github-ssh.md) | Host-spezifische GitHub-SSH-Aliases und Deploy-Keys. Kein Teil des Installationsvertrags. |

## Werkzeug und Kataloge

| Dokument | Inhalt |
|---|---|
| [`../installer/docs/architecture.md`](../installer/docs/architecture.md) | Architektur des Go-Werkzeugs: Anker finden, Verlinkung, Web-API, Designentscheidungen. |
| [`../installer/README.md`](../installer/README.md) | Kurzeinstieg und Pruefungen fuer Arbeiten am Werkzeug. |
| [`../checks/README.md`](../checks/README.md) | Schnittstelle und Nutzung von `bin/k-check`. |
| [`../rules/README.md`](../rules/README.md) | Die mitgelieferten Regeln. |
| [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv) | Kanonische Security-Tool-Matrix fuer Skript, Oberflaeche und Review-Rezepte. |

## Skills

| Skill | Zweck |
|---|---|
| [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md) | Docs als autoritative Quelle fuer AI-Sessions verankern. |
| [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md) | Mitgelieferte und projekteigene Regeln waehrend der Arbeit anwenden. |
| [`../skills/overlay-repo-analyse/PLAYBOOK.md`](../skills/overlay-repo-analyse/PLAYBOOK.md) | Docker-Overlay-Repos systematisch analysieren und dokumentieren. |

## Umstellung

[`umbau.md`](./umbau.md) ist die Arbeitsdatei zur laufenden Umstellung auf das
projektlokale Modell. Sie haelt fest, was festgelegt ist, was ersatzlos entfallen ist und
was noch nachzuziehen bleibt. Wenn alles umgestellt ist, wird sie geloescht.

## Stichwort-Index

- `Anker` / `K-PLAYBOOK.yaml` -> [`k-playbook-format.md`](./k-playbook-format.md)
- `AGENTS.md` / `CLAUDE.md` / `Verlinkung` -> [`installation.md`](./installation.md#3-assistenten-verlinken)
- `Assistenten` / `Claude Code` / `OpenCode` / `Cursor` -> [`installation.md`](./installation.md#3-assistenten-verlinken)
- `checks` / `k-check` -> [`../checks/README.md`](../checks/README.md), [`commands.md`](./commands.md#k-check)
- `CodeQL` -> [`commands.md`](./commands.md), [`../rules/codeql.md`](../rules/codeql.md)
- `Commands` -> [`commands.md`](./commands.md)
- `Docs zuerst` -> [`handbuch.md`](./handbuch.md#docs-first), [`../skills/ai-session-memory/PLAYBOOK.md`](../skills/ai-session-memory/PLAYBOOK.md)
- `Enforcement` / `Regeln` -> [`../rules/README.md`](../rules/README.md), [`../skills/enforcement/PLAYBOOK.md`](../skills/enforcement/PLAYBOOK.md)
- `Findings` / `Statuswerte` -> [`reviews-and-results.md`](./reviews-and-results.md#statusmodell)
- `GitHub SSH` / `Deploy-Key` -> [`local-github-ssh.md`](./local-github-ssh.md)
- `Installation` / `git clone` -> [`installation.md`](./installation.md)
- `k-playbook-local` / `projekteigen` -> [`k-playbook-format.md`](./k-playbook-format.md), [`installation.md`](./installation.md#2-projekteigene-struktur-anlegen)
- `Oberflaeche` / `k-gui` / `Web-API` -> [`../installer/docs/architecture.md`](../installer/docs/architecture.md)
- `Overlay` / `overlay.disabled` / `Regel ersetzen` -> [`k-playbook-format.md`](./k-playbook-format.md#mitgeliefertes-und-projekteigenes-zusammenfassen)
- `Pfade` / `warum keine paths` -> [`faq.md`](./faq.md), [`k-playbook-format.md`](./k-playbook-format.md#keine-pfade-in-der-konfiguration)
- `PR-Review` -> [`pr-review.md`](./pr-review.md)
- `Remediation` -> [`code-review.md`](./code-review.md#k-remediation), [`reviews-and-results.md`](./reviews-and-results.md#remediation)
- `Results` / `Ergebnisse` -> [`reviews-and-results.md`](./reviews-and-results.md)
- `Reviews` -> [`code-review.md`](./code-review.md)
- `schema_version` -> [`k-playbook-format.md`](./k-playbook-format.md#schema_version)
- `Security-Tools` / `Tool-Matrix` -> [`installation.md`](./installation.md#security-tools), [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv)
- `Tasks` -> [`task-flow.md`](./task-flow.md)
- `Update` / `git pull` -> [`installation.md`](./installation.md#aktualisieren)
