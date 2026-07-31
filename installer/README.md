# k-playbook Installer

Deterministischer Installer fuer k-playbook. Das Tool lebt bewusst isoliert unter `installer/`, weil das Ziel-Repo selbst unter `~/dev/k-playbook` erreichbar sein muss.

## Aktueller Umfang

- `status` prueft ohne Argument den Pfadvertrag `~/dev/k-playbook`.
- `status <path>` gibt den read-only Projektstatus als JSON aus.
- `status --fix` legt nur dann einen Symlink an, wenn `~/dev/k-playbook` fehlt und das aktuelle k-playbook-Repo sicher erkannt wurde.
- `gui` startet eine lokale Browser-Oberflaeche, zeigt den Pfadstatus, bietet denselben Symlink-Fix nach Bestaetigung an und sammelt danach Projekt-Auswahlen. Der Projekt-Scan kann `~/dev` oder das Home-Verzeichnis `~` durchsuchen. Ausserdem kann die GUI `git pull --ff-only` ausfuehren, Security-Tools read-only pruefen und Markdown-Dateien aus `docs/` gerendert anzeigen.
- Die GUI prueft und aktualisiert die Assistenten-Registrierung fuer OpenCode und Claude: `commands/k-*.md`, OpenCode `skills.paths` und Claude-Skill-Symlinks.
- Die GUI zeigt einen Security-Tool-Preflight anhand von `global/security-tools.tsv`; sie installiert dabei nichts.
- `projects list`, `projects scan`, `projects add` und `projects status <path>` verwalten bzw. pruefen die lokale Projekt-Auswahl. `projects status <path>` ist ein Alias fuer die JSON-Ausgabe von `status <path>`.

Lokale Installer-Daten liegen unter:

```text
~/dev/k-playbook/.k-playbook-local/
```

Dieses Verzeichnis ist ignoriert und wird nicht versioniert.

## Entwicklung

Empfohlene Installation nach einem Clone:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
cd ~/dev/k-playbook
make install
# alternativ ohne make: ./scripts/install-installer.sh
k-playbook-installer
```

`make install` braucht kein lokal installiertes Go. Es ruft `scripts/install-installer.sh` auf; das Script nutzt zuerst ein passendes vorhandenes Binary aus `dist/`, `bin/` oder `installer/bin/` und laedt sonst das passende Release-Binary von GitHub. Installiert wird standardmaessig nach `~/.local/bin/k-playbook-installer`.

Source-Builds fuer Entwickler brauchen Go. `make install-from-source` baut das Binary nach `./bin/k-playbook-installer` und verlinkt `~/.local/bin/k-playbook-installer` per Symlink darauf. `make gui` startet immer das repo-lokale Binary aus `./bin/` und funktioniert deshalb auch, wenn `~/.local/bin` noch nicht im `PATH` liegt. Falls `~/.local/bin` noch nicht im `PATH` liegt, fragt `make install-from-source` interaktiv, ob das passende Shell-Profil automatisch ergaenzt werden soll. Unter macOS mit zsh ist das standardmaessig `~/.zprofile`, unter Linux meistens `~/.profile`.

Entwicklungsaufrufe ohne Installation:

```bash
go run ./cmd/k-playbook-installer status
go run ./cmd/k-playbook-installer status /pfad/zum/projekt
go run ./cmd/k-playbook-installer
go run ./cmd/k-playbook-installer projects scan
go run ./cmd/k-playbook-installer projects status /pfad/zum/projekt
```

Die GUI bindet nur an `127.0.0.1` und oeffnet den Browser automatisch. Falls das automatische Oeffnen nicht funktioniert, gibt der Installer die lokale URL im Terminal aus.

Architektur, Designentscheidungen, Web-API, Frontend-Flows und offene naechste Schritte stehen in [`docs/architecture.md`](./docs/architecture.md). Diese Datei ist die zentrale Session-Memory fuer weitere Installer-Arbeiten.

Release-Artefakte werden auf Maintainer-Seite mit `make -C priv release-artifacts` nach `dist/` gebaut, z. B. `dist/k-playbook-installer-linux-amd64` und `dist/k-playbook-installer-darwin-arm64`.

Die Repo-Erkennung nutzt k-playbook-Markerdateien wie `docs/README.md`, `installer/go.mod` und mindestens ein `commands/k-*.md`, nicht nur `.git`. Dadurch funktionieren Symlinks, Worktrees, Command-Umbenennungen und spaetere nicht-Git-Kopien robuster.
