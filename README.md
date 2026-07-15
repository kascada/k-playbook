# k-playbook

Persönliche Sammlung von wiederverwendbaren AI-Assistant-Bausteinen –
projektübergreifend, unter Git-Kontrolle.

Zwei Arten von Inhalten:

| Verzeichnis | Was | Aufruf |
|-------------|-----|--------|
| `commands/` | Slash-Commands (Kurzbefehle, manuell aufgerufen) | `/k-<name>` |
| `ks-<name>/` | Playbooks/Skills (Prozeduren mit Prosa, Checkliste, Templates) | automatisch getriggert (OpenCode-Skill) oder von Hand befolgt |

## Struktur

```
~/dev/k-playbook/
├── README.md                           diese Datei
├── install.md                          Setup für OpenCode und Claude Code
├── Makefile                            git-add + commit + push in einem
├── commands/                            Slash-Commands
│   ├── k-akte.md
│   ├── k-enforcement.md
│   ├── k-install.md
│   ├── k-logmcp.md
│   ├── k-remediation.md
│   ├── k-review-loop.md
│   ├── k-run.md
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

Auf jedem Server einmal `/k-install` ausführen, sobald der Command verfügbar ist. Danach erneut ausführen, wenn neue Dateien unter `commands/k-*.md` dazukommen, damit die OpenCode-Symlinks aktualisiert werden.

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
- Templates in `ks-<thema>/vorlagen/`, Skripte in `ks-<thema>/scripts/`
