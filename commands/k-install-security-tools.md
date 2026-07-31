---
description: Host-local preflight and installer for shared security review tools from global/security-tools.tsv. Installs only after explicit confirmation. Uses scripts/install-security-tools.sh and writes no project files.
argument-hint: [--preflight|--install missing|--install all]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep]
---

# k-install-security-tools

Installiere oder pruefe host-lokale Security-Review-Tools fuer alle Projekte, die dieses globale k-playbook nutzen.

Dieser Command ist **host-lokal**. Er veraendert keine Projektdateien, kein `K-PLAYBOOK.yaml`, keine Review-Artefakte und startet keine Scans. Er richtet nur wiederverwendbare CLI-Tools bzw. Docker-Fallback-Images auf dem aktuellen Host ein.

Er darf nicht in einem aktiven Projekt-venv laufen. Wenn `VIRTUAL_ENV` gesetzt ist, abbrechen und den User auffordern, zuerst `deactivate` auszufuehren. Python-CLI-Tools duerfen nur via `pipx` oder dediziertem k-playbook Tool-venv installiert werden, nie in `.venv/`, `venv/` oder `env/` eines Projekts.

Er nutzt:

`<PLAYBOOK_REPO>/scripts/install-security-tools.sh`

Die kanonische Tool-Matrix liegt in:

`<PLAYBOOK_REPO>/global/security-tools.tsv`

## Tool-Scope

Pflicht-Tools laut `global/security-tools.tsv`:

- `gitleaks` - Secret-Scanning.
- `trufflehog` - tieferes Secret-Scanning.
- `pip-audit` - Python Dependency-CVEs.
- `trivy` - Filesystem/Container/IaC/CVE-Scans.
- `syft` - SBOM-Erzeugung.
- `grype` - SBOM-/Dependency-CVE-Auswertung.

## Schritt 1 - Playbook-Repo bestimmen

Bestimme `PLAYBOOK_REPO` wie in `/k-install`:

1. Fester logischer Pfad: `~/dev/k-playbook`.
2. Wenn dieser Pfad fehlt, nicht nach einem alternativen Dauerpfad fragen. Den User auffordern, das Repo dorthin zu klonen/verschieben oder einen Symlink dorthin anzulegen.
3. Wenn `/workspaces/k-playbook` existiert, aber `~/dev/k-playbook` fehlt, ist dies ein Devcontainer-Symlink-Problem; erst den Symlink `~/dev/k-playbook -> /workspaces/k-playbook` herstellen lassen.

Pruefe danach:

- `<PLAYBOOK_REPO>/scripts/install-security-tools.sh` existiert.
- Das Script ist ausfuehrbar oder kann mit `bash` gestartet werden.

Wenn nicht: abbrechen mit klarer Fehlermeldung.

## Schritt 2 - Preflight immer ausfuehren

Fuehre immer zuerst aus:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-security-tools.sh" --preflight
```

Zeige die Ausgabe kompakt. Sie muss enthalten:

- verwendete Tool-Matrix.
- Version/Pfad fuer vorhandene Tools.
- Fehlende Pflicht-Tools.
- Docker-Verfuegbarkeit.
- Docker-Fallback-Images, wo definiert.
- User-lokale Installationspfade (`~/.opencode/bin` oder `~/.local/bin` und dediziertes pip-audit Tool-venv).
- Hinweis, dass das pip-audit-venv ein dediziertes Tool-venv ist, nicht das Projekt-venv.

Wenn alle Pflicht-Tools vorhanden sind und kein explizites Install-Argument gesetzt ist: mit Status `ok` abschliessen.

## Schritt 3 - Argumente auswerten

Wenn der User keine Argumente nach dem Slash-Command angegeben hat:

- Nur Preflight ausfuehren.
- Wenn Pflicht-Tools fehlen, die empfohlenen Folgekommandos nennen.
- Nicht automatisch installieren.

Wenn der User Argumente nach dem Slash-Command angegeben hat:

- Erlaubte Weitergabe an das Script:
  - `--install missing`
  - `--install required`
  - `--install all`
  - `--install <tool>` fuer installierbare Tools aus `<PLAYBOOK_REPO>/global/security-tools.tsv`
  - optional `--method auto|native|docker|pipx|venv`
  - optional `--dry-run`
- `--yes` darf nur weitergegeben werden, wenn der User es explizit im Slash-Command angegeben hat oder danach bestaetigt hat.
- Unbekannte Argumente nicht raten; abbrechen und die erlaubten Formen zeigen.

## Schritt 4 - Installationsentscheidung

Wenn eine Installation gewuenscht ist, zeige vor Ausfuehrung den exakten Befehl.

Default-Empfehlung:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-security-tools.sh" --install missing --method auto
```

Installationswege:

- `--method auto`: native GitHub-Release-Binaries fuer Go/Rust-Tools; `pip-audit` via `pipx`, falls vorhanden, sonst dediziertes k-playbook Tool-venv.
- `--method native`: native/user-lokale Installation; `pip-audit` nutzt ein dediziertes Tool-venv, wenn `pipx` fehlt.
- `--method docker`: pullt die offiziellen Docker-Fallback-Images, soweit definiert; `pip-audit` wird mangels sinnvoller Docker-Wrapper-Integration per dediziertem Tool-venv bereitgestellt.
- `--method pipx`: nur sinnvoll fuer `pip-audit`.
- `--method venv`: nur sinnvoll fuer `pip-audit`; meint ein dediziertes Tool-venv ausserhalb von Projekt-venvs.

Frage dann:

> "Soll ich diesen host-lokalen Installationsbefehl jetzt ausfuehren?"

Ohne Bestaetigung nicht installieren.

## Schritt 5 - Ausfuehren und verifizieren

Nach Bestaetigung:

1. Den exakten Befehl mit `bash` ausfuehren.
2. Vollstaendige Ausgabe erfassen.
3. Danach erneut `--preflight` ausfuehren.
4. Ergebnis zusammenfassen:

```text
Security-Tools host-lokal geprueft/installiert
----------------------------------------------
Pflicht:    ok | <n> fehlen
Docker:     ok | fehlt
Pfad:       ~/.opencode/bin | ~/.local/bin | anderer Pfad

Naechste Nutzung:
  /k-review secret-scanning
  /k-review dependency-cve
  /k-review dependabot-alerts
  /k-review iac-container
```

## Grenzen

- Keine Projektdateien schreiben.
- Keine Scans starten.
- Keine Review-Ergebnisse erzeugen.
- Keine Paketmanager mit `sudo` verwenden.
- Keine systemweite Installation erzwingen.
- Keine fremden Dateien unter dem gewaehlten User-Bin-Verzeichnis ueberschreiben, ausser vorhandene symlink-/binary-Ziele der gleichen Toolnamen.
