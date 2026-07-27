# k-playbook Installation

Diese Anleitung beschreibt, wie k-playbook auf einem Host fuer OpenCode eingerichtet wird und wie Zielprojekte anschliessend projektlokal registriert werden.

Primärumgebung ist OpenCode. Claude Code kann optional nur fuer Slash-Commands angebunden werden.

## Voraussetzungen

Das Repo liegt standardmaessig unter `~/dev/k-playbook/`:

```bash
gh repo clone kascada/k-playbook ~/dev/k-playbook
```

Alternative:

```bash
git clone git@github.com:kascada/k-playbook.git ~/dev/k-playbook
```

Dieser Pfad ist der verbindliche Pfadvertrag fuer k-playbook. Projektdateien wie `K-PLAYBOOK.MD`, OpenCode `skills.paths` und Command-Symlinks zeigen auf `~/dev/k-playbook`, nicht auf einen host-spezifischen absoluten Pfad.

Wenn das echte Repo woanders liegt, wird nicht die Projektkonfiguration angepasst. Stattdessen muss `~/dev/k-playbook` als Symlink auf den echten Klon zeigen:

```bash
mkdir -p ~/dev
ln -sfn /pfad/zum/k-playbook ~/dev/k-playbook
```

Warum das Basis-Repo dauerhaft gebraucht wird:

- OpenCode-Commands werden als Symlinks nach `~/.config/opencode/command/` registriert; die Symlink-Ziele bleiben Dateien unter `~/dev/k-playbook/commands/`.
- Skills werden ueber `skills.paths: ["~/dev/k-playbook"]` aus dem Basis-Repo geladen.
- Commands und Skills lesen bei der Arbeit im Zielprojekt `K-PLAYBOOK.MD`; dessen `repo:`-Eintrag dient als Rueckverweis auf das Basis-Repo fuer Shared-Module, globale Regeln, globale Checks und Skripte.

## Devcontainer-Pfadvertrag

Wenn ein Zielprojekt in einem Devcontainer laeuft, muss derselbe logische Pfad `~/dev/k-playbook` auch im Container funktionieren. Das empfohlene Layout ist:

```text
Host:
  ~/dev/k-playbook                         echtes Basis-Repo

Container:
  /workspaces/k-playbook                   Bind-Mount des Host-Repos
  /home/vscode/dev/k-playbook              Symlink auf /workspaces/k-playbook
```

Damit bleibt dieser Eintrag in `K-PLAYBOOK.MD` auf Host und Container gleich:

```markdown
- repo: ~/dev/k-playbook
```

Der Devcontainer des Zielprojekts sollte diesen Symlink beim Start erzeugen, z. B.:

```bash
mkdir -p /home/vscode/dev
ln -sfn /workspaces/k-playbook /home/vscode/dev/k-playbook
```

Wenn `repo: ~/dev/k-playbook` im Container steht, bedeutet das also **nicht**, dass das Repo physisch dort kopiert wird. Der Pfad wird ueber den Symlink auf den Bind-Mount `/workspaces/k-playbook` aufgeloest.

## Schnellstart Neuer Host

Auf einem neuen Host sind zwei Dinge getrennt:

1. Host-global installieren: macht Commands, Skills und Security-Tools fuer alle Projekte verfuegbar.
2. Projektlokal einrichten: registriert in jedem Zielprojekt dessen eigene Pfade per `/k-setup`.

Host-global:

```text
/k-install
/k-install-security-tools --install missing
```

Empfohlen wird der Aufruf direkt im k-playbook-Repo nach dem Klonen oder nach einem Pull. Aus einem Zielprojekt heraus ist `/k-install` ebenfalls moeglich; der Command nutzt dennoch den festen Pfad `~/dev/k-playbook` und aktualisiert nur die host-globale OpenCode-Registrierung.

Danach OpenCode neu starten.

Projektlokal pro Projekt:

```text
/k-setup
```

Optional pro Projekt:

```text
/k-setup-codeql
```

Merksatz: `/k-install*` gehoert zum Host, `/k-setup*` gehoert zum Projekt.

Haeufige Fragen zum Aufrufort und zu Projekt-venvs stehen in [`faq.md`](./faq.md).

## OpenCode Setup

### Empfohlen: `/k-install`

Wenn `/k-install` bereits im Slash-Command-Menue sichtbar ist:

```text
/k-install
```

Der Command erledigt host-lokal:

- `commands/k-*.md` nach `~/.config/opencode/command/` symlinken.
- `skills.paths` in der OpenCode-Konfig pruefen oder ergaenzen.
- Security-Review-Tools per Preflight pruefen.
- verwaiste alte Command-Links melden.
- daran erinnern, OpenCode neu zu starten.

Nach neuen Dateien unter `commands/k-*.md` auf jedem Host erneut `/k-install` ausfuehren.

Wenn `/k-install` aus einem Projekt heraus gestartet wird, waehlt der Command keinen projektspezifischen Repo-Pfad. Er prueft `~/dev/k-playbook`. Falls der aktuelle Arbeitsordner selbst ein k-playbook-Klon ist, aber nicht unter `~/dev/k-playbook` liegt, soll der Command vorschlagen, den Klon dorthin zu verschieben oder nach Bestaetigung einen Symlink nach `~/dev/k-playbook` anzulegen.

### Bootstrap Wenn `/k-install` Noch Nicht Sichtbar Ist

Minimal einmal den Install-Command direkt verlinken:

```bash
mkdir -p ~/.config/opencode/command
ln -sf ~/dev/k-playbook/commands/k-install.md ~/.config/opencode/command/k-install.md
```

OpenCode neu starten und dann `/k-install` ausfuehren.

### Manuelle Referenz

OpenCode-Skills werden ueber `skills.paths` registriert. Datei `~/.config/opencode/opencode.jsonc`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

Slash-Commands werden als Symlinks registriert:

```bash
mkdir -p ~/.config/opencode/command
for f in ~/dev/k-playbook/commands/k-*.md; do
  ln -sf "$f" ~/.config/opencode/command/
done
```

OpenCode liest Konfiguration nur beim Start. Nach Änderungen daher OpenCode neu starten.

## Security-Tools

`/k-install` installiert Security-Tools nicht selbst, sondern zeigt einen Preflight. Fehlende Pflicht-Tools werden separat installiert:

```text
/k-install-security-tools --install missing
```

Vor `/k-install-security-tools` darf kein Projekt-venv aktiv sein. Falls `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

Python-CLI-Tools werden bevorzugt mit `pipx` installiert. Falls `pipx` fehlt, nutzt der Installer ein dediziertes k-playbook Tool-venv unter `~/.local/share/k-playbook/`; dieses ist nicht das Projekt-venv.

Pflicht-Tools:

- `gitleaks` und `trufflehog` fuer Secret-Scanning.
- `pip-audit` fuer Python Dependency-CVEs.
- `trivy` fuer Filesystem-, Container-, IaC- und CVE-Scans.
- `syft` fuer SBOMs.
- `grype` fuer SBOM-/Dependency-CVE-Auswertung.

Der Installer schreibt keine Projektdateien, installiert nichts in `.venv/`, `venv/` oder `env/` und startet keine Scans. Review-Familien nutzen die Tools spaeter ueber `/k-review`.

## Projektlokales Setup

Im Zielprojekt:

```text
/k-setup
```

Der Command:

- legt oder aktualisiert `K-PLAYBOOK.MD` im Projekt-Root.
- registriert projektlokale Pfade wie `tasks`, `todo`, `checks`, `reviews`, `guidelines`, `enforcement`, `docs`.
- erzeugt bestaetigte Verzeichnisse oder Initialdateien.
- schreibt keine host-globale OpenCode-Konfiguration.
- installiert keine host-globalen Tools.

`K-PLAYBOOK.MD` ist die zentrale Pointer-Datei. Spaetere Commands lesen daraus, wo projektlokale Bausteine liegen.

Der `repo:`-Eintrag im Managed Block ist fest `~/dev/k-playbook`. Er ist sichtbar und wird von Commands gelesen, aber nicht als frei waehlbarer Pfad behandelt. Wenn der physische Klon woanders liegt, muss ein Symlink den Pfadvertrag erfuellen.

## Optional: CodeQL

Wenn ein Projekt CodeQL nutzen soll:

```text
/k-setup-codeql
```

Der Command dokumentiert die Entscheidung in `K-PLAYBOOK.MD` und trennt GitHub CodeQL, lokale CLI und lokale Datenbanken. Lokale Analysen werden nicht still durch Setup gestartet.

Details: [`commands.md`](./commands.md) und [`../global/rules/codeql.md`](../global/rules/codeql.md).

## Optional: Claude Code

Claude Code sucht Slash-Commands unter `~/.claude/commands/`. Optionaler Symlink:

```bash
ln -sfn ~/dev/k-playbook/commands ~/.claude/commands
```

Danach sind Commands per `/k-<name>` in Claude Code verfuegbar.

Hinweis: Claude Code kennt kein direktes Aequivalent zu OpenCode-Skills mit automatisch getriggertem `SKILL.md`. Die `ks-<name>/PLAYBOOK.md`-Dateien muessen dort manuell konsultiert werden.

## Verifikation

Checkliste fuer einen Host:

- [ ] `~/dev/k-playbook/` existiert und ist ein Git-Repo.
- [ ] `~/.config/opencode/opencode.jsonc` oder `.json` enthaelt `skills.paths` mit dem Repo-Pfad.
- [ ] Symlinks unter `~/.config/opencode/command/` zeigen auf `commands/k-*.md`.
- [ ] `/k-status` zeigt `OpenCode: OK` oder empfiehlt `/k-install` fuer fehlende/falsche Symlinks.
- [ ] OpenCode wurde neu gestartet.
- [ ] Autocomplete zeigt `/k-*`-Commands.
- [ ] Ein Test-Skill wird bei passendem Thema geladen.
- [ ] `/k-install-security-tools` meldet Pflicht-Tools als vorhanden oder installiert sie nach Bestaetigung.
- [ ] `/k-install-security-tools` wurde ohne aktives Projekt-venv ausgefuehrt.

Checkliste fuer ein Projekt:

- [ ] `K-PLAYBOOK.MD` existiert im Projekt-Root.
- [ ] `base:` ist gesetzt.
- [ ] `repo:` zeigt auf `~/dev/k-playbook`.
- [ ] Benoetigte Pfade sind aktiv und existieren.
- [ ] `/k-status` zeigt keine unerwarteten `FAIL`-Eintraege.
- [ ] Bei aktiven Docs existiert ein Docs-Index oder `/k-code2docs` ist als naechster Schritt geplant.

Checkliste fuer einen Devcontainer:

- [ ] `/workspaces/k-playbook/commands/` existiert durch den Bind-Mount.
- [ ] `~/dev/k-playbook` existiert im Container und zeigt auf `/workspaces/k-playbook`.
- [ ] `~/.config/opencode/command/k-install.md` ist ein Symlink auf `~/dev/k-playbook/commands/k-install.md` oder den aufgeloesten Bind-Mount-Pfad.
- [ ] `~/.config/opencode/opencode.jsonc` oder `.json` enthaelt `skills.paths` mit `~/dev/k-playbook`.
- [ ] `/k-status` meldet die Devcontainer-Pfadstruktur nicht als Warnung.

## Fehlersuche

### Slash-Commands Tauchen Nicht Auf

- Symlinks in `~/.config/opencode/command/` pruefen.
- OpenCode neu starten.
- Frontmatter der Command-Datei pruefen.
- `/k-install` erneut ausfuehren.

### Skills Werden Nicht Automatisch Getriggert

- `skills.paths` in der OpenCode-Konfig pruefen.
- `SKILL.md` im richtigen `ks-<name>/`-Ordner pruefen.
- `description` im Skill konkret genug formulieren.
- OpenCode neu starten.

### Projektpfade Stimmen Nicht

- Im Projekt `/k-status` ausfuehren.
- Wenn `K-PLAYBOOK.MD` fehlt oder `base:` fehlt, `/k-setup` ausfuehren.
- Fehlende Verzeichnisse nicht manuell raten, sondern ueber `/k-setup` bestaetigt erzeugen oder migrieren.
