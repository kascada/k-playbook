# k-playbook

`k-playbook` ist eine kuratierte Sammlung wiederverwendbarer AI-Assistant-Bausteine: Slash-Commands, Skills/Playbooks, globale Regeln, Review-Rezepte und Checks. Das Repo bildet den globalen Werkzeugkasten; projektlokale Regeln, Tasks, Reviews und Docs liegen im jeweiligen Projekt unter `k-playbook/` und werden ueber `K-PLAYBOOK.yaml` eingebunden.

## Schnellstart

### Normalinstallation ohne Go

Empfohlen fuer Nutzer nach einem frischen Clone:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
cd ~/dev/k-playbook
make install
k-playbook-installer
```

`make install` baut nicht aus Source. Es installiert die plattformspezifischen Installer-Binaries nach `bin/`, legt dort den Wrapper `bin/k-playbook-installer` an und verlinkt `~/.local/bin/k-playbook-installer` auf diesen Wrapper. Quelle sind vorhandene Release-Artefakte unter `dist/` oder die GitHub Releases. Dafuer ist kein lokal installiertes Go noetig.

Falls `~/.local/bin` noch nicht im `PATH` liegt, gibt das Script einen Hinweis aus. Die GUI kann dann auch direkt gestartet werden:

```bash
~/.local/bin/k-playbook-installer
```

Der Installer prueft den Pfadvertrag `~/dev/k-playbook`, kann ihn bei Bedarf reparieren, verwaltet die Projekt-Auswahl, zeigt die Docs an und kann `git pull --ff-only` ausfuehren.

### Installation aus Source mit Go

Fuer Entwickler oder lokale Source-Aenderungen:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
cd ~/dev/k-playbook
make install-from-source
make gui
```

`make install-from-source` ruft `make build` auf, baut alle plattformspezifischen Binaries nach `./bin/`, installiert den Wrapper `./bin/k-playbook-installer` und verlinkt `~/.local/bin/k-playbook-installer` auf diesen Wrapper. Danach aktualisiert ein spaeteres `make build` automatisch auch den globalen Aufruf ueber den Symlink.

### Lokal testen als Entwickler

Nach Codeaenderungen am Installer:

```bash
make build
./bin/k-playbook-installer status
./bin/k-playbook-installer
```

`/k-status` bevorzugt den kanonischen Launcher `~/dev/k-playbook/bin/k-playbook-installer` und nutzt im DevContainer denselben logischen Pfad ueber den Symlink nach `/workspaces/k-playbook`. Ein `make build` erzeugt alle unterstuetzten Plattform-Binaries und reicht deshalb auch fuer DevContainer, wenn das Repo nach `/workspaces/k-playbook` gemountet ist.

Oder direkt GUI starten und vorher automatisch neu bauen:

```bash
make installer-run
```

`make installer-run` ist ein Alias fuer `make gui`; `gui` haengt von `build` ab und startet danach `./bin/k-playbook-installer`.

Details zum Binary-Vertrag stehen in [`docs/installer-binaries.md`](./docs/installer-binaries.md).

Maintainer erzeugen Release-Artefakte mit:

```text
make -C priv release-artifacts
```

OpenCode-Kommandos bleiben zusaetzlich verfuegbar:

```text
/k-gui
/k-install-security-tools --install missing
```

Nach einem frischen Clone ist `/k-gui` eventuell noch nicht sichtbar. Fuer die einfache gefuehrte Installation nutze `make install`/`make gui` oder folge [`docs/multi-project-installation.md`](./docs/multi-project-installation.md).

Die Command-Zustaendigkeiten und die empfohlene Reihenfolge stehen in [`docs/commands.md`](./docs/commands.md) und [`docs/multi-project-installation.md`](./docs/multi-project-installation.md).

Wichtig: `/k-install-security-tools` und host-globale Tool-Preflights nicht aus einem aktiven Projekt-venv starten. Falls `VIRTUAL_ENV` gesetzt ist, zuerst `deactivate` ausfuehren.

In jedem Zielprojekt ueber die GUI einbinden:

```text
/k-gui
```

Merksatz: `/k-gui` ist der Normalweg fuer Host-Registrierung und Projekt-Onboarding; Spezialcommands bleiben fuer Security-Tools und CodeQL.

## Dokumentation

Der Einstieg ist das Handbuch:

- [`docs/handbuch.md`](./docs/handbuch.md) - Zweck, Konzepte, Standardablaeufe und Betriebsregeln.
- [`docs/installation.md`](./docs/installation.md) - Installation fuer OpenCode, Security-Tools und optional Claude Code.
- [`docs/multi-project-installation.md`](./docs/multi-project-installation.md) - empfohlene Reihenfolge fuer Host, mehrere Zielprojekte und DevContainer.
- [`docs/faq.md`](./docs/faq.md) - Kurze Antworten zu `/k-gui`, Aufrufort und Projekt-venvs.
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

Die Installer-GUI registriert dieses Repo host-lokal fuer OpenCode. Die Command-Dateien bleiben im Repo; OpenCode bekommt Symlinks:

```text
~/.config/opencode/command/
├── k-gui.md              -> ~/dev/k-playbook/commands/k-gui.md
├── k-status.md           -> ~/dev/k-playbook/commands/k-status.md
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

- Schlank und nachvollziehbar: feste Struktur, klare Zuständigkeiten.
- Globales Repo, lokale Projektkonfiguration: keine Pfade raten, sondern `K-PLAYBOOK.yaml` lesen.
- Docs zuerst: Projektwissen soll in `k-playbook/docs/` liegen und per `AGENTS.md`/OpenCode-Konfig in Sessions wirken.
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
make -C priv release-artifacts
```

Private Maintainer-Targets liegen lokal unter `priv/Makefile`, nicht im user-facing Root-`Makefile`.
