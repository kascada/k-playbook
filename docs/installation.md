# k-playbook Installation

Der Installationsweg steht im Root-[`README.md`](../README.md#installation): Binary bauen, dann `k-playbook-installer init` im Zielprojekt.

Diese Datei beschreibt die Details dahinter: Uebergangsstand, Source-Build, Launcher-/Binary-Struktur, DevContainer-Integration und Verifikation.

Primaerumgebung ist OpenCode. Claude Code kann optional fuer Slash-Commands angebunden werden.

## Status: Umstellung auf projektlokale Installation

k-playbook wird in ein Unterverzeichnis des Zielprojekts installiert
(`<projekt>/k-playbook/_dist/`) statt in eine zentrale Basisinstallation unter
`~/dev/k-playbook`. Der Pfadvertrag ist damit entfallen.

Fertig und in Benutzung:

- Konfigurationskontrakt `schema_version: 2`, siehe [`k-playbook-format.md`](./k-playbook-format.md).
- Aufloesung in `commands/_shared/path-resolution.md` und `commands/_shared/overlay-resolution.md`.
- Alle Commands, Skills, Review-Rezepte und `k-check`.
- Die Installer-Kommandozeile: `init`, `update`, `restore`, `migrate`, `status`, `version`.
  Die Payload steckt per `go:embed` im Binary; `dist/` enthaelt die aktuellen Artefakte,
  `make install` braucht daher kein Go.

Noch offen:

- Die Browser-GUI bildet weiterhin das alte zentrale Modell ab. Sie laeuft, verweigert aber
  jede schreibende Aktion auf einem Projekt mit `schema_version: 2`, damit sie eine
  Migration nicht rueckgaengig macht.
- Die DevContainer-Integration setzt noch auf den Bind-Mount des Basis-Repos.
- `prompts/installation/` beschreibt noch das alte Verfahren.

Alles unterhalb dieses Abschnitts beschreibt diesen noch nicht umgebauten Rest.

## Pfadvertrag (ausserlaufend)

Der logische k-playbook-Pfad ist fest:

```text
~/dev/k-playbook
```

Projektdateien wie `K-PLAYBOOK.yaml`, OpenCode `skills.paths` und Command-Symlinks zeigen auf diesen Pfad, nicht auf einen host-spezifischen absoluten Ort.

Warum das wichtig ist:

- Eine zentrale Basisinstallation kann viele Zielprojekte bedienen.
- Projektkonfigurationen bleiben portabel zwischen Host und DevContainer.
- Commands, Skills, globale Regeln, globale Checks und Skripte koennen immer denselben Rueckverweis nutzen.
- Wenn das echte Repo physisch woanders liegt, muss nur ein Symlink repariert werden, nicht jede Projektkonfiguration.

Wenn das Repo nicht unter `~/dev/k-playbook` liegt, soll der Pfadvertrag ueber einen Symlink erfuellt werden:

```bash
mkdir -p ~/dev
ln -sfn /pfad/zum/k-playbook ~/dev/k-playbook
```

Die Installer-GUI erkennt diesen Fall und kann den Symlink nach Bestaetigung anlegen, wenn das aktuelle k-playbook-Repo sicher erkannt wurde.

## Source-Build fuer Entwickler

Die automatische Installation mit `make install` nutzt vorhandene Release-Artefakte oder GitHub Releases und braucht kein lokal installiertes Go. Fuer lokale Installer-Entwicklung oder unveroeffentlichte Source-Aenderungen gibt es die Build-Variante:

```bash
cd ~/dev/k-playbook
make install-from-source
make gui
```

`make install-from-source` braucht Go. Es ruft `make build` auf, baut alle unterstuetzten Plattform-Binaries nach `bin/`, installiert den Wrapper `bin/k-playbook-installer`, verlinkt `~/.local/bin/k-playbook-installer` auf diesen Wrapper und stellt sicher, dass `~/dev/k-playbook/bin` im Shell-Profil steht.

`make gui` baut ebenfalls vorher und startet danach den repo-lokalen Wrapper aus `bin/`. Das ist fuer Entwickler praktisch, weil der Aufruf nicht davon abhaengt, ob der neu gesetzte PATH schon in der aktuellen Shell aktiv ist.

Weitere nuetzliche Entwickler-Targets:

```bash
make build
make test
make dist
make clean
```

Aliases existieren fuer aeltere Aufrufe, z. B. `make installer-build`, `make installer-test` und `make installer-run`.

## Installer-Binary und Launcher

Der kanonische Launcher ist:

```text
~/dev/k-playbook/bin/k-playbook-installer
```

Der PATH-Vertrag fuer den Shell-Aufruf ist `~/dev/k-playbook/bin`. `~/.local/bin/k-playbook-installer` ist nur ein Komfort-Symlink auf denselben Launcher und bleibt fuer direkte Aufrufe bzw. Kompatibilitaet bestehen.

`dist/` enthaelt Release-Artefakte und bleibt Quelle fuer `make install` und GUI-Update:

```text
dist/k-playbook-installer-linux-amd64
dist/k-playbook-installer-linux-arm64
dist/k-playbook-installer-darwin-amd64
dist/k-playbook-installer-darwin-arm64
```

`bin/` enthaelt lokale Runtime-Binaries plus Wrapper:

```text
bin/k-playbook-installer
bin/k-playbook-installer-linux-amd64
bin/k-playbook-installer-linux-arm64
bin/k-playbook-installer-darwin-amd64
bin/k-playbook-installer-darwin-arm64
```

`bin/k-playbook-installer` ist ein Shell-Wrapper. Er erkennt per `uname` Betriebssystem und Architektur und startet per `exec` das passende plattformspezifische Binary im selben Verzeichnis.

Host und DevContainer teilen sich dasselbe Repo, aber nicht zwingend dieselbe Plattform. Deshalb darf `bin/k-playbook-installer` kein einzelnes Go-Binary sein. Der Wrapper verhindert Kollisionen zwischen z. B. macOS-Host und Linux-Container.

Nach einem GUI-Update per `git pull --ff-only` spiegelt die GUI vorhandene `dist/k-playbook-installer-*`-Artefakte nach `bin/`, installiert den Wrapper, setzt den Komfort-Symlink neu und stellt sicher, dass `~/dev/k-playbook/bin` im Shell-Profil steht. Das ist architekturunabhaengig, solange `dist/` alle Release-Artefakte enthaelt.

## DevContainer-Pfadvertrag

Wenn ein Zielprojekt in einem DevContainer laeuft, muss derselbe logische Pfad `~/dev/k-playbook` auch im Container funktionieren. Das empfohlene Layout ist:

```text
Host:
  ~/dev/k-playbook                         echtes Basis-Repo

Container:
  /workspaces/k-playbook                   Bind-Mount des Host-Repos
  /home/vscode/dev/k-playbook              Symlink auf /workspaces/k-playbook
```

Wichtig: Der Symlink allein reicht nicht. Das Host-Verzeichnis `~/dev/k-playbook` muss zuerst in den DevContainer gemountet werden, sonst zeigt `/home/vscode/dev/k-playbook` im Container ins Leere.

Die DevContainer-Integration wird pro Zielprojekt ueber die Installer-GUI eingerichtet. Technisch nutzt sie dieses Script aus dem Host-Repo:

```bash
~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh /pfad/zum/zielprojekt
```

Das Script erwartet im Zielprojekt eine vorhandene `.devcontainer/devcontainer.json` und richtet dort die Integration ein.

Es passt `.devcontainer/devcontainer.json` an:

- Bind-Mount `~/dev/k-playbook` nach `/workspaces/k-playbook`.
- `postCreateCommand` fuer das initiale Container-Setup inklusive optionaler Security-Tool-Installation.
- `postStartCommand`, damit persistierte OpenCode-Volumes bei jedem Container-Start repariert werden.

Es kopiert `scripts/templates/devcontainer-setup-k-playbook.sh` nach `.devcontainer/setup-k-playbook.sh` im Zielprojekt. Diese Datei ist projektlokale DevContainer-Infrastruktur und bleibt bewusst im Zielprojekt, damit der Container beim Start ohne Zugriff auf das globale Installationsscript reparierbar bleibt.

`.devcontainer/setup-k-playbook.sh` erledigt im Container:

- Symlink `/home/vscode/dev/k-playbook` -> `/workspaces/k-playbook`.
- PATH-Eintrag `~/dev/k-playbook/bin` in den Shell-Profilen des Container-Users.
- Command-Links fuer `k-*.md` nach `/home/vscode/.config/opencode/command/` und `/home/vscode/.config/opencode/commands/`.
- Minimale OpenCode-User-Config mit `skills.paths: ["~/dev/k-playbook"]`, falls noch keine Container-Config existiert.
- Bei `--install-security-tools`: Installation fehlender Pflicht-Tools laut `global/security-tools.tsv` in das Home des Container-Users `vscode`.

Danach den DevContainer neu bauen oder neu starten und OpenCode im Container neu starten.

Wenn `/k-gui` danach nicht sichtbar ist, im Container pruefen:

```bash
ls -l /workspaces/k-playbook/commands/k-gui.md
ls -l ~/dev/k-playbook/commands/k-gui.md
ls -l ~/.config/opencode/command/k-gui.md
ls -l ~/.config/opencode/commands/k-gui.md
```

Alle vier Pfade muessen existieren. Wenn der erste fehlt, ist der Bind-Mount nicht gesetzt. Wenn nur der zweite fehlt, fehlt der Symlink. Wenn nur die letzten Pfade fehlen, fehlt der OpenCode-Bootstrap im Container-Home.

Wenn `k_playbook.repo: ~/dev/k-playbook` im Container steht, bedeutet das nicht, dass das Repo physisch dort kopiert wird. Der Pfad wird ueber den Symlink auf den Bind-Mount `/workspaces/k-playbook` aufgeloest.

## OpenCode-Registrierung

Die Installer-GUI registriert OpenCode-Commands und Skills. Sie verlinkt `commands/k-*.md` nach `~/.config/opencode/command/` und stellt sicher, dass `skills.paths` das k-playbook-Repo enthaelt.

Manuelle Referenz fuer OpenCode-Skills:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

OpenCode liest Konfiguration nur beim Start. Nach Registrierung oder Aenderungen an Commands/Skills OpenCode neu starten.

## Projekt-Onboarding

Zielprojekte werden ueber die Installer-GUI eingebunden. Wenn der Assistant bereits registriert ist, im Assistant aufrufen:

```text
/k-gui
```

Die GUI:

- legt oder aktualisiert `K-PLAYBOOK.yaml` im Projekt-Root.
- schreibt die projektlokalen Artefaktpfade unter `paths.*`.
- legt die konfigurierte projektlokale Struktur an, konventionell unter `k-playbook/`.
- erzeugt bestaetigte Verzeichnisse oder Initialdateien.
- zeigt und aktualisiert die Remediation-Policy pro Projekt.
- verwaltet DevContainer-Integration pro Projekt.

`K-PLAYBOOK.yaml` ist die zentrale Config-Datei. Spaetere Commands lesen projektlokale Pfade aus `paths.*`. Fehlt ein benoetigter Pfad, muss der Command nachfragen und den bestaetigten Wert in `K-PLAYBOOK.yaml` ergaenzen; er darf nicht still aus `k-playbook/` oder dem Dateisystem raten.

## Security-Tools

Security-Tools werden host- oder user-lokal installiert, nie in Projekt-venvs. Die kanonische Tool-Matrix liegt in `global/security-tools.tsv`; sie wird von `/k-install-security-tools` und der Installer-GUI gelesen.

Aktuelle Pflicht-Tools:

- `gitleaks` und `trufflehog` fuer Secret-Scanning.
- `pip-audit` fuer Python Dependency-CVEs.
- `trivy` fuer Filesystem-, Container-, IaC- und CVE-Scans.
- `syft` fuer SBOMs.
- `grype` fuer SBOM-/Dependency-CVE-Auswertung.

Vor `/k-install-security-tools` darf kein Projekt-venv aktiv sein. Falls `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

## Optional: Claude Code

Claude Code sucht Slash-Commands unter `~/.claude/commands/`. Optionaler Symlink:

```bash
ln -sfn ~/dev/k-playbook/commands ~/.claude/commands
```

Danach sind Commands per `/k-<name>` in Claude Code verfuegbar.

Hinweis: Claude Code kennt kein direktes Aequivalent zu OpenCode-Skills mit automatisch getriggertem `SKILL.md`. Die `ks-<name>/PLAYBOOK.md`-Dateien muessen dort manuell konsultiert werden.

## Verifikation

Checkliste fuer einen Host:

- [ ] `~/dev/k-playbook/` existiert und ist ein Git-Repo oder Symlink auf das echte Repo.
- [ ] `~/dev/k-playbook/bin/k-playbook-installer` existiert und ist ausfuehrbar.
- [ ] `~/dev/k-playbook/bin` steht im Shell-Profil und nach neu geladener Shell im `PATH`.
- [ ] `~/.local/bin/k-playbook-installer` zeigt als Komfort-Symlink auf den kanonischen Launcher oder der Launcher wird direkt genutzt.
- [ ] `~/.config/opencode/opencode.jsonc` oder `.json` enthaelt `skills.paths` mit dem Repo-Pfad.
- [ ] Symlinks unter `~/.config/opencode/command/` zeigen auf `commands/k-*.md`.
- [ ] `/k-status` zeigt `OpenCode: OK` oder empfiehlt `/k-gui` fuer fehlende/falsche Symlinks.
- [ ] OpenCode wurde neu gestartet.

Checkliste fuer ein Projekt (`schema_version: 1`, aktuell):

- [ ] `K-PLAYBOOK.yaml` existiert im Projekt-Root.
- [ ] `layout: fixed-project-k-playbook` ist gesetzt.
- [ ] `k_playbook.repo` zeigt auf `~/dev/k-playbook`.
- [ ] Die benoetigten `paths.*`-Eintraege sind gesetzt und die konfigurierten Pfade existieren.
- [ ] `/k-status` zeigt keine unerwarteten `FAIL`-Eintraege.

Checkliste fuer ein Projekt (`schema_version: 2`, nach dem Installer-Umbau):

- [ ] `K-PLAYBOOK.yaml` existiert unter `<projekt>/k-playbook/`.
- [ ] `layout: project-local` ist gesetzt.
- [ ] `k_playbook.dist` zeigt auf ein existierendes Verzeichnis, konventionell `_dist`.
- [ ] `k_playbook.version` ist gesetzt, damit `restore` nach einem `git clone` funktioniert.
- [ ] `k-playbook/_dist/` steht in der `.gitignore` des Projekts und ist nicht eingecheckt.
- [ ] `project.repo_root` zeigt aus dem k-playbook-Verzeichnis heraus, normalerweise `..`.
- [ ] Kein `paths.*`-Wert zeigt in `_dist/` hinein.
- [ ] `.claude/commands` und `.claude/skills` zeigen auf die Verzeichnisse unter `_dist/`.
- [ ] `/k-status` zeigt keine unerwarteten `FAIL`-Eintraege.

Checkliste fuer einen DevContainer:

- [ ] `/workspaces/k-playbook/commands/` existiert durch den Bind-Mount.
- [ ] `~/dev/k-playbook` existiert im Container und zeigt auf `/workspaces/k-playbook`.
- [ ] `~/.config/opencode/command/k-gui.md` ist ein Symlink auf `~/dev/k-playbook/commands/k-gui.md` oder den aufgeloesten Bind-Mount-Pfad.
- [ ] `~/.config/opencode/opencode.jsonc` oder `.json` enthaelt `skills.paths` mit `~/dev/k-playbook`.
- [ ] `/k-status` meldet die DevContainer-Pfadstruktur nicht als Warnung.

## Fehlersuche

Wenn Slash-Commands nicht auftauchen:

- Symlinks in `~/.config/opencode/command/` pruefen.
- OpenCode neu starten.
- Frontmatter der Command-Datei pruefen.
- Installer-GUI direkt mit `k-playbook-installer` starten und Registrierung aktualisieren.

Wenn Skills nicht automatisch getriggert werden:

- `skills.paths` in der OpenCode-Konfig pruefen.
- `SKILL.md` im richtigen `ks-<name>/`-Ordner pruefen.
- `description` im Skill konkret genug formulieren.
- OpenCode neu starten.

Wenn Projektpfade nicht stimmen:

- Im Projekt `/k-status` ausfuehren.
- Wenn `K-PLAYBOOK.yaml` fehlt oder ungueltig ist, `/k-gui` ausfuehren.
- Fehlende Verzeichnisse nicht manuell raten, sondern ueber die Installer-GUI vervollstaendigen.
