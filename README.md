# k-playbook

Persönliche Sammlung von wiederverwendbaren AI-Assistant-Bausteinen –
projektübergreifend, unter Git-Kontrolle.

Drei Arten von Inhalten:

| Verzeichnis | Was | Aufruf |
|-------------|-----|--------|
| `commands/` | Slash-Commands (Kurzbefehle, manuell aufgerufen) | `/k-<name>` |
| `ks-<name>/` | Playbooks/Skills (Prozeduren mit Prosa, Checkliste, Templates) | automatisch getriggert (OpenCode-Skill) oder von Hand befolgt |
| `global/` | Projektuebergreifende Regeln, Review-Rezepte und Checks | von Commands oder `global/bin/*` geladen |

Dieses Repo enthaelt nur globale Bausteine. Projektlokale Bausteine liegen im jeweiligen Zielprojekt, typischerweise unter `<projekt>/k-playbook/`, und werden dort ueber `K-PLAYBOOK.MD` registriert.

## Struktur

```
~/dev/k-playbook/
├── README.md                           diese Datei
├── install.md                          Setup für OpenCode und Claude Code
├── Makefile                            git-add + commit + push in einem
├── scripts/                            Host-lokale Hilfsskripte
├── global/                              globale Regeln und Review-Rezepte
│   ├── bin/
│   │   └── k-check
│   ├── checks/
│   │   ├── *.sh
│   │   └── lib/
│   ├── rules/
│   │   ├── codeql.md
│   │   ├── docs-sync.md
│   │   └── review-authoring.md
│   └── reviews/
│       ├── review-dependabot-alerts.md
│       ├── review-dependency-cve.md
│       ├── review-iac-container.md
│       ├── review-k-check-security.md
│       ├── review-codeql-security.md
│       ├── review-python-comment-hardspots.md
│       ├── review-secret-scanning.md
│       └── review-tech.md
├── commands/                            Slash-Commands
│   ├── k-enforcement.md
│   ├── k-install.md
│   ├── k-install-codeql.md
│   ├── k-install-security-tools.md
│   ├── k-logmcp.md
│   ├── k-remediation.md
│   ├── k-results.md
│   ├── k-review-loop.md
│   ├── k-run.md
│   ├── k-setup-codeql.md
│   ├── k-task-create.md
│   ├── k-test-check.md
│   ├── k-todo.md
│   ├── k-verlauf.md
│   ├── k-vscode-project-color.md
│   ├── k-zeit-review.md
│   └── todo.md → k-todo.md              Symlink-Alias
├── ks-ai-session-memory/                 Playbook: docs für AI-Sessions verankern
│   ├── SKILL.md                         Skill-Metadaten (Frontmatter)
│   ├── PLAYBOOK.md                      Prosa + Checkliste
│   └── vorlagen/                        Templates
├── ks-enforcement/                        Playbook: globale + projektlokale Enforcement-Regeln anwenden
│   ├── SKILL.md
│   └── PLAYBOOK.md
└── ks-overlay-repo-analyse/              Playbook: Docker-Overlay-Projekte analysieren
    ├── SKILL.md
    ├── PLAYBOOK.md
    ├── vorlagen/
    └── scripts/                          extract-base.sh, diff-overlay.sh
```

## Setup

→ Siehe [`install.md`](./install.md) für die vollständige Einrichtung
(OpenCode primär, Claude Code optional).

Eine kurze Übersicht der wichtigsten Setup- und Install-Commands steht in
[`docs/commands.md`](./docs/commands.md).

Der Review-/Results-/Remediation-Flow ist in
[`docs/reviews-and-results.md`](./docs/reviews-and-results.md) dokumentiert.

## Globale Checks

Der globale Check-Runner liegt unter `global/bin/k-check` und wird aus einem Projekt-Root gestartet:

```bash
~/dev/k-playbook/global/bin/k-check --mode changed
~/dev/k-playbook/global/bin/k-check --mode baseline
```

Er fuehrt `.sh`-Checks aus `global/checks/` und zusaetzlich projektlokale `.sh`-Checks aus dem in `K-PLAYBOOK.MD` registrierten `checks:`-Pfad aus. Details zur Schnittstelle, Root-Semantik, `changed`-/`baseline`-Modi und Exit-Codes stehen in [`global/checks/README.md`](./global/checks/README.md).

Fuer auditierbare Review-Laeufe kann der Runner seine vollstaendige Ausgabe und Metadaten dauerhaft schreiben:

```bash
~/dev/k-playbook/global/bin/k-check --mode baseline --output k-playbook/reviews/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt --metadata-output k-playbook/reviews/results/k-check/YYYY-MM-DD/run-metadata.json
```

Vorhandene Output-/Metadaten-Dateien werden nicht ueberschrieben.

Auf jedem Server einmal `/k-install` ausführen, sobald der Command verfügbar ist. Danach erneut ausführen, wenn neue Dateien unter `commands/k-*.md` dazukommen, damit die OpenCode-Symlinks aktualisiert werden. `/k-install` zeigt auch den host-lokalen Security-Tool-Status. Fehlende Pflicht-Tools werden separat installiert mit:

```text
/k-install-security-tools --install missing
```

Merksatz: `/k-install*` ist host-global; `/k-setup*` ist projektlokal. Details stehen in [`install.md`](./install.md) und [`docs/commands.md`](./docs/commands.md).

## Versionierung

Git-Repo mit Remote auf GitHub:

```
git@github.com:kascada/k-playbook.git
```

Nach Änderungen:

```bash
cd ~/dev/k-playbook
make push        # git add -A + git commit -m "update skills" + git push
```

Oder manuell:

```bash
git add .
git commit -m "..."
git push
```

## Konvention

- Alle User-facing Namen beginnen mit `k-` (persönliche Präfix von kleist/kascada)
- Playbook-/Skill-Ordner heißen `ks-<thema>/` (`ks-` = k-Skill, trennt Skills von Commands)
- Slash-Commands heißen `commands/k-<thema>.md`
- Globale Check-Entry-Points liegen als ausführbare Tools unter `global/bin/`; Checks selbst liegen als `.sh`-Schnittstelle unter `global/checks/`.
- `/k-results` erzeugt die priorisierte Gesamtzusammenfassung aus vorhandenen `reviews/results/*/*/assessment.md`.
- Nach dem Anlegen eines neuen `commands/k-*.md` auf jedem Host `/k-install` ausführen, damit OpenCode den Symlink unter `~/.config/opencode/command/` bekommt.
- Templates in `ks-<thema>/vorlagen/`, Skripte in `ks-<thema>/scripts/`
