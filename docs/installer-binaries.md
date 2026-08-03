# Installer-Binaries

Diese Datei beschreibt den verbindlichen Binary- und Launcher-Vertrag fuer den k-playbook Installer.

## Ziel

Der kanonische Launcher ist:

```text
~/dev/k-playbook/bin/k-playbook-installer
```

Im DevContainer zeigt `~/dev/k-playbook` per Symlink auf `/workspaces/k-playbook`. Dadurch ist derselbe logische Launcher auf Host und im Container nutzbar.

`~/.local/bin/k-playbook-installer` ist nur ein Komfort-Symlink auf den kanonischen Launcher.

## Verzeichnisse

`dist/` enthaelt Release-Artefakte und bleibt Quelle fuer Installation und GUI-Update:

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

## Erzeugung

- `make build` baut alle plattformspezifischen Binaries nach `bin/` und installiert den Wrapper.
- `make dist` baut alle plattformspezifischen Release-Artefakte nach `dist/`.
- `make install` spiegelt alle unterstuetzten Artefakte aus `dist/` oder aus GitHub Releases nach `bin/`, installiert den Wrapper und setzt `~/.local/bin/k-playbook-installer` als Symlink auf den Wrapper.
- `make install-from-source` ruft `make build` auf und setzt danach denselben globalen Symlink.
- Das GUI-Update nach `git pull --ff-only` spiegelt alle vorhandenen `dist/k-playbook-installer-*` nach `bin/`, installiert den Wrapper und setzt den globalen Symlink neu.

## DevContainer

Host und DevContainer teilen sich dasselbe Repo, aber nicht zwingend dieselbe Plattform. Deshalb darf `bin/k-playbook-installer` kein einzelnes Go-Binary sein. Der Wrapper verhindert Kollisionen zwischen z. B. macOS-Host und Linux-Container.

Ein GUI-Update ist architekturunabhaengig, solange `dist/` alle Release-Artefakte enthaelt: alle vorhandenen Artefakte werden nach `bin/` gespiegelt.
