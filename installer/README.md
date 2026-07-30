# k-playbook Installer

Deterministischer Installer fuer k-playbook. Das Tool lebt bewusst isoliert unter `installer/`, weil das Ziel-Repo selbst unter `~/dev/k-playbook` erreichbar sein muss.

## Aktueller Umfang

- `status` prueft den Pfadvertrag `~/dev/k-playbook`.
- `status --fix` legt nur dann einen Symlink an, wenn `~/dev/k-playbook` fehlt und das aktuelle k-playbook-Repo sicher erkannt wurde.
- `gui` startet eine lokale Browser-Oberflaeche, zeigt den Pfadstatus, bietet denselben Symlink-Fix nach Bestaetigung an und sammelt danach Projekt-Auswahlen.
- `projects list`, `projects scan` und `projects add` verwalten die lokale Projekt-Auswahl.

Lokale Installer-Daten liegen unter:

```text
~/dev/k-playbook/.k-playbook-local/
```

Dieses Verzeichnis ist ignoriert und wird nicht versioniert.

## Entwicklung

```bash
go run ./cmd/k-playbook-installer status
go run ./cmd/k-playbook-installer gui
go run ./cmd/k-playbook-installer projects scan
```

Die GUI bindet nur an `127.0.0.1` und oeffnet den Browser automatisch. Falls das automatische Oeffnen nicht funktioniert, gibt der Installer die lokale URL im Terminal aus.

Falls Go auf dem Host noch fehlt, muss zuerst Go installiert werden. Unter Debian/Ubuntu/WSL z. B. ueber die Distribution oder den offiziellen Go-Tarball, unter macOS z. B. mit Homebrew oder dem offiziellen Installer.

Die Repo-Erkennung nutzt k-playbook-Markerdateien wie `commands/k-install.md` und `docs/README.md`, nicht nur `.git`. Dadurch funktionieren Symlinks, Worktrees und spaetere nicht-Git-Kopien robuster.
