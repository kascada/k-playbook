# k-playbook Installer Architektur

Diese Datei ist die Session-Memory fuer die Entwicklung des Installers unter `installer/`. Kuenftige AI-Sessions sollen diese Datei zuerst lesen, bevor sie den Installer-Code erneut analysieren.

## Ziel

Der Installer ist ein eigenstaendiges Go-Modul fuer die gefuehrte Einrichtung von k-playbook. Er soll auch dann funktionieren, wenn das k-playbook-Repo noch nicht unter dem verbindlichen Pfad `~/dev/k-playbook` liegt.

Der Installer lebt bewusst unter `installer/`, ist aber als separates Modul gebaut:

```text
installer/
├── cmd/k-playbook-installer/main.go
├── internal/cli/
├── internal/pathcontract/
├── internal/projects/
├── internal/store/
├── internal/ui/
├── internal/webui/
├── docs/architecture.md
├── go.mod
└── README.md
```

## Aktuelle Commands

```bash
go run ./cmd/k-playbook-installer status
go run ./cmd/k-playbook-installer status --fix
go run ./cmd/k-playbook-installer
go run ./cmd/k-playbook-installer gui
go run ./cmd/k-playbook-installer projects list
go run ./cmd/k-playbook-installer projects scan
go run ./cmd/k-playbook-installer projects add <path> --env plain
```

Der fruehere interaktive Terminal-UI-Command `ui` wurde bewusst entfernt. Die interaktive Oberflaeche ist jetzt die lokale Browser-GUI. Ohne Subcommand startet `k-playbook-installer` direkt die GUI; `gui` bleibt als expliziter Alias erhalten. Die skriptbaren CLI-Kommandos bleiben erhalten.

## Designentscheidungen

- Go bleibt die einzige Runtime fuer den Installer.
- Die GUI ist eine lokale Web-UI, keine native Desktop-App und kein Electron/Wails-Setup.
- Der HTTP-Server bindet nur auf `127.0.0.1` und verwendet einen zufaelligen freien Port.
- Der Browser wird automatisch geoeffnet. Plattformlogik: macOS `open`, Windows `rundll32`, Linux/Unix `xdg-open`.
- Die Web-Assets sind per `embed` im Go-Binary enthalten.
- Der Installer fuehrt keine Shell-Pipelines aus. Fachlogik ruft Go-Funktionen oder gezielte Prozesse wie `git pull --ff-only` auf.
- Der Git-Pull ist absichtlich `--ff-only`, damit keine Merge-Commits oder interaktiven Konfliktzustaende entstehen.
- Lokale Installer-Daten liegen unter `~/dev/k-playbook/.k-playbook-local/` und sind nicht versioniert.
- Der Pfadvertrag ist Voraussetzung fuer Store-, Docs- und Pull-Funktionen.
- Pfad- und Repo-Erkennung basiert auf k-playbook-Markerdateien, nicht nur auf `.git`.

## Wichtige Pakete

### `internal/cli`

Cobra-basierte CLI. Registriert aktuell:

- `status`
- `gui`
- `projects`

`status` nutzt `pathcontract.Check()` und optional `pathcontract.Repair()`. `projects` nutzt `projects` und `store`. `gui` startet `webui.Run()`.

### `internal/pathcontract`

Prueft und repariert den verbindlichen Pfad:

```text
~/dev/k-playbook
```

Wichtige Funktionen:

- `ExpectedPath()`
- `Check()`
- `Repair(result)`
- `DiscoverCurrentRoot(start)`
- `IsKPlaybookRoot(path)`

`Repair()` darf nur automatisch einen Symlink anlegen, wenn `~/dev/k-playbook` fehlt und das aktuelle k-playbook-Repo sicher erkannt wurde.

### `internal/projects`

Findet und klassifiziert Zielprojekte.

Wichtige Funktionen:

- `NormalizePath(value)`
- `ProjectFromPath(path)`
- `DetectEnvironment(path)`
- `ScanDefaultDev()`
- `Scan(root)`

Erkennung:

- DevContainer: `.devcontainer/devcontainer.json`
- Python venv: `.venv/` oder `venv/`
- Projektmarker: `.git`, `K-PLAYBOOK.MD`, `pyproject.toml`, `package.json`, `go.mod`, `.devcontainer/devcontainer.json`

Der Scan ueberspringt typische grosse oder irrelevante Verzeichnisse wie `.git`, `.venv`, `node_modules`, `dist`, `target`, `vendor`, `results`.

### `internal/store`

Persistiert die lokale Projekt-Auswahl in:

```text
~/dev/k-playbook/.k-playbook-local/projects.json
```

Schema:

```json
{
  "version": 1,
  "projects": [
    {
      "path": "/abs/path",
      "name": "repo-name",
      "environment": "plain|venv|devcontainer|unknown",
      "selected": true,
      "detected": [".venv/"],
      "addedAt": "...",
      "updatedAt": "..."
    }
  ]
}
```

### `internal/ui`

Keine interaktive UI mehr. Dieses Paket enthaelt nur noch textuelle Renderer fuer CLI-Ausgaben:

- `RenderPathStatus(result, styled)`
- `RenderProjects(file, styled)`

`styled` ist aktuell absichtlich wirkungslos, weil die Charmbracelet-Abhaengigkeiten entfernt wurden.

### `internal/webui`

Lokaler HTTP-Server plus eingebettete statische Assets.

Wichtige Dateien:

```text
internal/webui/server.go
internal/webui/static/index.html
internal/webui/static/styles.css
internal/webui/static/app.js
```

## Web-API

Aktuelle Endpunkte:

| Methode | Pfad | Zweck |
|---|---|---|
| `GET` | `/api/status` | Pfadvertrag pruefen |
| `POST` | `/api/repair-path` | Fixbaren Pfadvertrag reparieren |
| `GET` | `/api/projects` | Gespeicherte Projekt-Auswahl laden, inklusive Projekt-Setup-Status (`K-PLAYBOOK.MD`) und projektlokalem Docs-Status |
| `DELETE` | `/api/projects` | Projekt nach bestaetigter GUI-Aktion aus der gespeicherten Installer-Liste entfernen; Projektdateien bleiben unveraendert |
| `POST` | `/api/projects` | Manuelles Projekt speichern |
| `GET` | `/api/projects/scan?root=dev|home` | Projektkandidaten scannen |
| `POST` | `/api/projects/scan` | Ausgewaehlte Scan-Projekte speichern |
| `GET` | `/api/git/status` | Read-only per `git ls-remote` pruefen, ob der Upstream-Branch von k-playbook einen anderen Commit zeigt |
| `POST` | `/api/git/pull` | `git pull --ff-only` im k-playbook-Repo ausfuehren |
| `GET` | `/api/docs` | Markdown-Dateien unter `docs/` listen |
| `GET` | `/api/docs/file?path=docs/...md` | Markdown-Datei lesen und gerendert ausgeben |
| `GET` | `/api/opencode/status` | OpenCode- und Claude-Command-/Skill-Registrierung pruefen |
| `POST` | `/api/opencode/install` | Fehlende/falsche k-playbook-Command-/Skill-Symlinks anlegen, verwaiste eigene Symlinks entfernen und OpenCode `skills.paths` konservativ ergaenzen |
| `GET` | `/api/security-tools/status` | Host-lokalen Security-Tool-Preflight als strukturierte Liste pruefen; installiert nichts |
| `GET` | `/api/devcontainer/status` | Gespeicherte DevContainer-Projekte auf k-playbook-Mount und Setup-Hooks pruefen |
| `POST` | `/api/devcontainer/install` | Fuer eine konkrete oder alle fehlenden DevContainer-Integrationen `scripts/install-devcontainer-k-playbook.sh <projekt>` ausfuehren |
| `GET` | `/api/health` | Client-Heartbeat und Verfuegbarkeit des lokalen GUI-Servers pruefen |
| `POST` | `/api/client-gone` | Browser meldet Tab-/Fenster-Schliessen oder Navigation per `sendBeacon` |
| `POST` | `/api/shutdown` | Lokalen GUI-Server beenden |

`repoRoot()` in `webui/server.go` nutzt den Pfadvertrag und `pathcontract.IsKPlaybookRoot()`, bevor Docs oder Git-Aktionen ausgefuehrt werden.

## Frontend-Flows

### Startseite

Die Startseite zeigt:

1. Header mit Button `Status neu laden`.
   Daneben liegt `k-playbook aktualisieren`; beim Start prueft die GUI read-only per `/api/git/status`, ob der Upstream-Branch einen anderen Commit zeigt. Wenn eine neue Version verfuegbar ist, wird der Button hervorgehoben und heisst `Zur neuen Version aktualisieren`. Der Button nutzt denselben `git pull --ff-only`-Flow wie der Repository-Block weiter unten.
2. Pfadvertrag.
3. Wenn OK: Pfadvertrag nur als kompakter Einzeiler.
4. Gespeicherte `Projekt-Auswahl` mit je einem gerahmten Block pro Projekt. Die Eyebrow lautet `Projekte`, nicht `Schritt 2`. Pro Projekt wird read-only geprueft, ob `K-PLAYBOOK.MD` existiert und ob `docs/` Markdown-Dateien enthaelt. Wenn `K-PLAYBOOK.MD` fehlt, zeigt der Projektblock `/k-setup`; wenn Docs fehlen oder leer sind, zeigt er `/k-code2docs`. Beide Hinweise nutzen Kopierbutton und Hilfe-Button und werden nicht durch die GUI ausgefuehrt, weil die Slash-Commands projektlokalen Kontext und Rueckfragen brauchen.
   Jeder Projektblock bietet `Entfernen`; nach Bestaetigung wird nur der Eintrag aus der lokalen Installer-Projektliste entfernt, keine Projektdatei.
5. Nur fuer gespeicherte Projekte mit Umgebung `devcontainer` enthaelt der jeweilige Projektblock die Zeile `k-playbook im Container erreichbar`. Dort wird geprueft, ob `.devcontainer/devcontainer.json` den Mount `source=${localEnv:HOME}/dev/k-playbook,target=/workspaces/k-playbook,type=bind`, `postCreateCommand`, `postStartCommand` und `.devcontainer/setup-k-playbook.sh` enthaelt. Bei fehlenden Eintraegen zeigt diese Projektzeile `Eintrag setzen` und nutzt das vorhandene Host-Script fuer genau dieses Projekt.
6. Button `Projekte auswaehlen`.
7. Assistenten-Registrierungsblock fuer OpenCode und Claude.
8. Security-Tool-Preflight mit einer Zeile pro Tool und Status `OK ✓`, `FEHLT !` oder `OPTIONAL`. Dieser Block prueft nur `PATH`, Versionen und Projekt-venv-Scope; er installiert nichts.
9. Repository-Block mit `Git pull`; bei verfuegbarer neuer Version wird auch dieser Button hervorgehoben und zu `Zur neuen Version aktualisieren`. Nach erfolgreichem Pull laeuft `refreshAll()`, wodurch Git-Status, Pfadstatus, Projekt-Auswahl, DevContainer-Status, Assistenten-Registrierung, Security-Tools und Docs neu geprueft werden.
10. Docs-Block mit gerenderter Markdown-Anzeige.
11. Button `Schliessen`, der den lokalen Server beendet. Der Browser-Tab zeigt danach nur noch den Hinweis, dass das Fenster geschlossen werden kann.

Der Client prueft periodisch `/api/health`. Wenn der lokale Server extern beendet wird, z. B. per `Ctrl+C`, erkennt der bereits geladene Browser-Tab den Verbindungsverlust und zeigt denselben Abschluss-Hinweis. Umgekehrt meldet der Browser beim Tab-/Fenster-Schliessen oder bei Navigation `/api/client-gone` per `sendBeacon`; der Server beendet sich danach mit kurzem Timeout. Falls diese Meldung nicht ankommt, beendet sich der Server nach ausbleibenden Heartbeats. Ein Reload bleibt moeglich, weil ein neuer Heartbeat die Abmeldung wieder aufhebt.

Der Installer versucht nicht, den Browser beim Server-Ende automatisch zu schliessen. Browser blockieren das in vielen Faellen, und `open`/`xdg-open` liefern keinen verlaesslichen Tab-Handle. Der robuste Weg ist der explizite `Schliessen`-Button in der GUI oder `Ctrl+C` im Terminal.

### Assistenten-Registrierung

Der Block bildet den zentralen Teil von `/k-install` ab und erweitert ihn um Claude-Code-Symlinks:

- Alle vorhandenen `commands/k-*.md` im k-playbook-Repo werden gezaehlt.
- Fuer jede Command-Datei wird ein Symlink gleichen Namens unter `~/.config/opencode/command/` erwartet.
- Fuer Claude Code wird fuer jede Command-Datei ein Symlink gleichen Namens unter `~/.claude/commands/` erwartet.
- Fuer Claude Skills werden alle `ks-*/SKILL.md` gezaehlt. Der Symlink `~/.claude/skills/<skill-name>` zeigt auf den jeweiligen Skill-Ordner, sodass darunter `SKILL.md` sichtbar ist.
- Fehlende oder falsche Symlinks werden per `Registrierung aktualisieren` angelegt bzw. ersetzt.
- Fremde Dateien gleichen Namens, die keine Symlinks sind, werden nur gemeldet und nicht ueberschrieben.
- Verwaiste k-playbook-Symlinks werden gemeldet und bei `Registrierung aktualisieren` entfernt, sofern sie auf eine nicht mehr existierende Datei unter diesem k-playbook-`commands/` zeigen.
- `skills.paths` wird in `~/.config/opencode/opencode.jsonc` oder `.json` geprueft.
- Wenn keine Config existiert, wird eine minimale Config mit `skills.paths: ["~/dev/k-playbook"]` angelegt.
- Wenn eine einfache Config ohne `skills` existiert, wird `skills` konservativ ergaenzt.
- Wenn bereits `skills` vorhanden ist und der Pfad fehlt, wird nicht geraten; die GUI meldet, dass manuelle Bearbeitung noetig ist.

Nach Aenderungen an Commands oder Skills muessen betroffene Assistenten neu gestartet werden, weil OpenCode und Claude Code Commands/Skills beim Start bzw. beim Laden ihrer Umgebung erfassen.

UI-Regel: Reine Statusanzeigen duerfen nicht wie Buttons aussehen. Fuer klickbare Aktionen werden `button`, `.primary` und `.secondary` genutzt. Fuer nicht-klickbare Zustandsanzeigen wird `.status-label` genutzt, z. B. `WARN !` oder `OK ✓`, ohne Rahmen und ohne pill-/button-artige Flaeche.

### Security-Tool-Preflight

Der Block bildet den read-only Teil von `/k-install` und `/k-install-security-tools --preflight` ab. Er nutzt eine zentrale Tool-Liste im Go-Backend:

- Pflicht: `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype`.
- Optional angezeigt: `docker` als spaeterer Fallback-Kontext.

Die GUI prueft pro Tool nur `exec.LookPath()` und eine kurze Versionsabfrage. Sie schreibt keine Dateien, installiert keine Tools und startet keine Scans. Wenn `VIRTUAL_ENV` gesetzt ist oder `PATH` typische Projekt-venv-Segmente wie `.venv`, `venv` oder `env` enthaelt, wird der Scope als Warnung angezeigt, damit Projekt-venvs nicht als host-globale Tool-Installation gewertet werden.

Wenn Pflicht-Tools fehlen, zeigt die GUI eine generische Command-Aktionszeile mit Kopierbutton fuer `/k-install-security-tools --install missing`. Die Hilfe nennt zusaetzlich den Terminal-Fallback `bash ~/dev/k-playbook/scripts/install-security-tools.sh --install missing --method auto`.

### Command-Aktionszeilen

Die GUI startet Slash-Commands nicht selbst, wenn diese im Zielkontext Rueckfragen stellen oder Dateien schreiben. Stattdessen nutzt sie eine wiederverwendbare Command-Aktionszeile mit:

- kurzer Beschreibung,
- Button zum Kopieren des Slash-Commands,
- `Hilfe`-Button mit Kontext und optionalem Fallback-Hinweis.

Aktuelle Nutzungen:

- Fehlendes Projekt-Setup: `/k-setup` im jeweiligen Zielprojekt.
- Leere oder fehlende Projekt-Dokumentation: `/k-code2docs` im Zielprojekt; Hilfe erwaehnt `/k-tools-scan` als vorgelagerten Inventarisierungsschritt.
- Fehlende Security-Tools: `/k-install-security-tools --install missing`; Hilfe erwaehnt das Shell-Script als Terminal-Fallback.

### Scan-Seite

`Projekte auswaehlen` wechselt auf eine separate Scan-Ansicht.

Scan-Roots:

- `~/dev` als Default.
- `~` als explizite, potenziell langsamere Alternative.

Auswahlverhalten:

- Ganze Zeile ist klickbar.
- Checkbox bleibt sichtbar.
- Ausgewaehlte Zeilen werden optisch markiert.
- Speicherbutton zeigt Anzahl, z. B. `1 Projekt speichern`, `3 Projekte speichern`.
- Nach `Auswahl speichern` springt die GUI zur Startseite zurueck.
- Manuelles Projekt-Hinzufuegen liegt ebenfalls auf der Scan-Seite und springt nach Speichern zur Startseite zurueck.

## Docs-Anzeige

Die GUI listet Markdown-Dateien aus dem zentralen Repo-Verzeichnis `docs/`.

Rendering:

- Serverseitig mit `github.com/yuin/goldmark`.
- GitHub-Flavored Markdown via `extension.GFM`.
- Auto-Heading-IDs via `parser.WithAutoHeadingID()`.
- Eingebettetes Raw-HTML in Markdown wird nicht per `html.WithUnsafe()` aktiviert. Dadurch wird Markdown gerendert, aber HTML nicht als aktives HTML durchgereicht.
- Frontend setzt das vom Server erzeugte HTML mit `innerHTML` in den Viewer.

Wichtige Abgrenzung: Weil nur lokale Markdown-Dateien unter `docs/` gelesen werden und Raw-HTML nicht aktiv erlaubt ist, bleibt der Viewer bewusst einfach. Wenn spaeter externe oder projektlokale Docs aus fremden Repos angezeigt werden, sollte zusaetzlich Sanitizing explizit bewertet werden.

## Dependencies

Direkte Go-Abhaengigkeiten in `installer/go.mod`:

- `github.com/spf13/cobra` fuer CLI-Kommandos.
- `github.com/yuin/goldmark` fuer Markdown-Rendering.

Entfernte Abhaengigkeiten:

- Charmbracelet `huh`, `lipgloss` und indirekte TUI-Pakete wurden entfernt, weil die interaktive TUI nicht mehr gebraucht wird.

## Betrieb und Verifikation

### User-Installation

Empfohlener Ablauf nach einem frischen Clone:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
cd ~/dev/k-playbook
make install
# alternativ ohne make: ./scripts/install-installer.sh
k-playbook-installer
```

Dieser Weg braucht kein lokal installiertes Go. `make install` ruft `scripts/install-installer.sh` auf; das Script nutzt zuerst ein passendes Release-Artefakt aus `dist/`, falls vorhanden, und laedt sonst das passende Binary aus den GitHub Releases.

Wenn `~/.local/bin` noch nicht im PATH liegt, gibt das Script einen Hinweis aus. Alternativ kann direkt gestartet werden:

```bash
~/.local/bin/k-playbook-installer
```

Wenn das Repo an einem anderen Ort geklont wurde, funktioniert derselbe Ablauf aus diesem Clone heraus. Die GUI kann danach den Pfadvertrag reparieren und bei Bedarf `~/dev/k-playbook` als Symlink auf den echten Clone anlegen.

Das Root-`Makefile` ist user-facing. Es enthaelt Installer-Targets:

```bash
make build
make dist
make install
make install-from-source
make uninstall
make gui
make test
make clean
make path-hint
make path-setup
```

Die alten laengeren Namen bleiben als Aliase erhalten: `make installer-build`, `make installer-install`, `make installer-install-from-source`, `make installer-uninstall`, `make installer-run`, `make installer-test`, `make installer-clean`.

Build- und Installationspfade:

```text
bin/k-playbook-installer
dist/k-playbook-installer-<os>-<arch>
~/.local/bin/k-playbook-installer
```

`make dist` baut plattformspezifische Artefakte nach `dist/` fuer `linux-amd64`, `linux-arm64`, `darwin-amd64` und `darwin-arm64`. Diese Artefakte sind fuer GitHub Releases gedacht und werden nicht versioniert. Das private Maintainer-Target `make -C priv release-artifacts` ruft dieses Root-Target auf.

`make install` installiert ohne Go ein vorhandenes passendes `dist/`-Artefakt oder laedt das Asset von `https://github.com/kascada/k-playbook/releases/latest/download/`. Die erwarteten Asset-Namen entsprechen den `make dist`-Dateinamen, z. B. `k-playbook-installer-linux-amd64`. Dieser Weg kopiert das Binary nach `~/.local/bin`; er legt keinen Symlink ins Repo an.

`make install-from-source` baut zuerst das repo-lokale Binary unter `bin/k-playbook-installer`, legt danach `~/.local/bin/k-playbook-installer` als Symlink auf dieses Binary an und prueft, ob `~/.local/bin` im `PATH` liegt. Dadurch aktualisiert ein spaeteres `make build` automatisch auch den globalen Aufruf. `make gui` startet immer das repo-lokale Binary und funktioniert deshalb auch ohne frisch geladenen PATH. Diese Source-Targets brauchen Go auf dem Host. Falls `~/.local/bin` nicht im `PATH` ist und der Aufruf in einem normalen interaktiven Terminal laeuft, fragt `make path-setup`, ob das passende Shell-Profil automatisch ergaenzt werden soll. Nicht-interaktive Aufrufe bekommen nur den Hinweis.

Profil-Auswahl im Root-`Makefile`:

- zsh: `~/.zprofile`, relevant fuer moderne macOS-Defaults.
- bash auf macOS: `~/.bash_profile`.
- bash auf Linux: `~/.profile`.
- sonst: `~/.profile`.

`PATH_PROFILE` kann beim Aufruf ueberschrieben werden, z. B. `make path-setup PATH_PROFILE=~/.zshrc`.

Der automatisch geschriebene Eintrag lautet:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Danach muss der Nutzer entweder ein neues Terminal oeffnen oder das ausgegebene Profil aktivieren, z. B.:

```bash
. ~/.profile
```

Das private Entwickler-Makefile liegt unter `priv/Makefile`. Private Targets wie `release-artifacts` und `sichern` gehoeren nicht ins Root-`Makefile`, weil das Root-`Makefile` fuer Nutzer gedacht ist.

Standard-Entwicklung:

```bash
cd installer
go run ./cmd/k-playbook-installer
```

Pruefungen nach Aenderungen:

```bash
cd installer
gofmt -w ./cmd ./internal
go mod tidy
go test ./...
go build -o /tmp/opencode/k-playbook-installer ./cmd/k-playbook-installer
```

Nicht mit `go build ./cmd/k-playbook-installer` ohne `-o` pruefen, wenn kein lokales Binary im Repo liegen soll. Dieser Befehl erzeugt sonst `installer/k-playbook-installer`.

## Bekannte offene Punkte

- Es gibt noch keine automatisierten Tests fuer API-Handler oder Frontend-Flows.
- Docs-Viewer rendert Markdown, hat aber noch keine Navigation innerhalb von Headings.
- `Git pull` zeigt nur die Prozessausgabe, noch keine strukturierte Git-Status-Anzeige.
- Es gibt noch keine Update-/Installationsaktion fuer die ausgewaehlten Projekte; aktuell wird nur die Auswahl gespeichert.
- Release-Binaries sind fuer macOS/Linux definiert; Windows und optionale `.app`-Pakete sind noch offen.
- Der Home-Scan `~` kann langsam sein und viele Treffer liefern; deshalb bleibt `~/dev` der Default.

## Naechste sinnvolle Schritte

1. API-Handler-Tests fuer Status, Docs und Projekt-Speichern ergaenzen.
2. Docs-Viewer mit Inhaltsverzeichnis oder Datei-Suche erweitern.
3. Repository-Block um `git status --short` und Remote/Branch-Anzeige ergaenzen.
4. Projekt-Auswahl um Bearbeiten erweitern.
5. Installer-Packaging erweitern: Windows und spaeter optional `.app`/native Pakete.
