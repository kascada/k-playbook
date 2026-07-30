# k-playbook

`k-playbook` ist eine kuratierte Sammlung wiederverwendbarer AI-Assistant-Bausteine: Slash-Commands, Skills/Playbooks, globale Regeln, Review-Rezepte und Checks. Das Repo bildet den globalen Werkzeugkasten; projektlokale Pfade, Regeln, Tasks, Reviews und Docs werden im jeweiligen Projekt ueber `K-PLAYBOOK.MD` registriert.

## Schnellstart

Gefuehrter Start mit Browser-GUI:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
cd ~/dev/k-playbook
./scripts/install-installer.sh
k-playbook-installer
```

Das Install-Script braucht kein lokal installiertes Go. Es nutzt ein passendes Binary aus `dist/`, falls vorhanden, oder laedt es aus den GitHub Releases.

Source-Builds fuer Entwickler brauchen Go:

```text
make install
make gui
```

Der Installer prueft den Pfadvertrag `~/dev/k-playbook`, kann ihn bei Bedarf reparieren, verwaltet die Projekt-Auswahl, zeigt die Docs an und kann `git pull --ff-only` ausfuehren.

OpenCode-Kommandos bleiben zusaetzlich verfuegbar:

```text
/k-install
/k-install-security-tools --install missing
```

Nach einem frischen Clone ist `/k-install` eventuell noch nicht sichtbar. Fuer die einfache gefuehrte Installation nutze die Prompts unter [`prompts/installation/`](./prompts/installation/) oder folge [`docs/multi-project-installation.md`](./docs/multi-project-installation.md).

Die Command-Zustaendigkeiten und die empfohlene Reihenfolge stehen in [`docs/commands.md`](./docs/commands.md) und [`docs/multi-project-installation.md`](./docs/multi-project-installation.md).

Wichtig: `/k-install*` nicht aus einem aktiven Projekt-venv starten. Falls `VIRTUAL_ENV` gesetzt ist, zuerst `deactivate` ausfuehren.

In jedem Zielprojekt einmal:

```text
/k-setup
```

Merksatz: `/k-install*` ist host-global; `/k-setup*` ist projektlokal.

## Dokumentation

Der Einstieg ist das Handbuch:

- [`docs/handbuch.md`](./docs/handbuch.md) - Zweck, Konzepte, Standardablaeufe und Betriebsregeln.
- [`docs/installation.md`](./docs/installation.md) - Installation fuer OpenCode, Security-Tools und optional Claude Code.
- [`docs/multi-project-installation.md`](./docs/multi-project-installation.md) - empfohlene Reihenfolge fuer Host, mehrere Zielprojekte und DevContainer.
- [`docs/faq.md`](./docs/faq.md) - Kurze Antworten zu `/k-install`, Aufrufort und Projekt-venvs.
- [`docs/commands.md`](./docs/commands.md) - Zuständigkeiten, Gruppen und Details der `/k-*`-Commands.
- [`docs/reviews-and-results.md`](./docs/reviews-and-results.md) - Review-, Results- und Remediation-Flow.
- [`prompts/README.md`](./prompts/README.md) - kopierbare AI-Assistenten-Prompts fuer gefuehrte Ablaeufe.
- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.

## Bausteine

| Bereich | Zweck | Nutzung |
|---|---|---|
| `commands/` | Manuelle Slash-Commands | `/k-<name>` |
| `prompts/` | Kopierbare AI-Assistenten-Auftraege | als Prompt einfuegen |
| `ks-<name>/` | Skills/Playbooks mit Anleitung, Checkliste und Templates | automatisch durch OpenCode oder manuell |
| `global/rules/` | Projektuebergreifende Enforcement-Regeln | Skill `ks-enforcement` oder `/k-enforcement` |
| `global/reviews/` | Wiederverwendbare Review-Rezepte | `/k-review <name>` |
| `global/checks/` | Schnelle generische Checks | `global/bin/k-check` |

## Symlink-Struktur

`/k-install` registriert dieses Repo host-lokal fuer OpenCode. Die Command-Dateien bleiben im Repo; OpenCode bekommt Symlinks:

```text
~/.config/opencode/command/
├── k-install.md          -> ~/dev/k-playbook/commands/k-install.md
├── k-status.md           -> ~/dev/k-playbook/commands/k-status.md
├── k-setup.md            -> ~/dev/k-playbook/commands/k-setup.md
└── k-*.md                -> ~/dev/k-playbook/commands/k-*.md
```

Skills werden nicht einzeln verlinkt. Stattdessen muss das Repo in der OpenCode-Konfiguration stehen:

```jsonc
{
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

Security-Tools liegen separat host-/user-lokal, typischerweise unter `~/.opencode/bin` oder `~/.local/bin`. Python-CLI-Tools wie `pip-audit` nutzen `pipx` oder ein dediziertes k-playbook Tool-venv unter `~/.local/share/k-playbook/`, nie ein Projekt-venv.

`/k-status` prueft read-only, ob die OpenCode-Symlinks auf die erwarteten Dateien in diesem Repo zeigen und ob `skills.paths` plausibel gesetzt ist. Repariert wird mit `/k-install`.

DevContainer brauchen dieselbe Struktur im Container: Das Host-Repo muss z. B. nach `/workspaces/k-playbook` gemountet werden, und `~/dev/k-playbook` im Container zeigt per Symlink darauf. Das richtet pro Zielprojekt dieses Script ein:

```bash
~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh /pfad/zum/zielprojekt
```

Details stehen in [`docs/installation.md`](./docs/installation.md#devcontainer-pfadvertrag).

## Grundprinzipien

- Schlank und nachvollziehbar: nur Bausteine, die im Alltag genutzt werden.
- Globales Repo, lokale Projektkonfiguration: keine Pfade raten, sondern `K-PLAYBOOK.MD` lesen.
- Docs zuerst: Projektwissen soll in `docs/` liegen und per `AGENTS.md`/OpenCode-Konfig in Sessions wirken.
- Review-Ergebnisse auditierbar speichern: `assessment.md`, `findings.md`, `raw/` und Metadaten.
- Security-Tools host-lokal installieren, nie in Projekt-venvs; Review-Artefakte projektlokal ablegen.

## Repository-Struktur

```text
~/dev/k-playbook/
├── README.md
├── docs/                         Handbuch und Detaildokumentation
├── commands/                     Slash-Commands
├── prompts/                      kopierbare AI-Assistenten-Prompts
├── global/                       globale Regeln, Reviews, Checks
├── ks-ai-session-memory/         Playbook: Docs fuer AI-Sessions verankern
├── ks-enforcement/               Playbook: globale + lokale Regeln anwenden
├── ks-overlay-repo-analyse/      Playbook: Docker-Overlay-Projekte analysieren
├── scripts/                      host-lokale Hilfsskripte
└── Makefile
```

## Versionierung

Remote:

```text
git@github.com:kascada/k-playbook.git
```

Nach Änderungen:

```bash
cd ~/dev/k-playbook
make installer-test
make installer-build
make release
```

Private Maintainer-Targets liegen lokal unter `priv/Makefile`, nicht im user-facing Root-`Makefile`.
