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
go run ./cmd/k-playbook-installer status --path-contract
go run ./cmd/k-playbook-installer status <path>
go run ./cmd/k-playbook-installer
go run ./cmd/k-playbook-installer gui
go run ./cmd/k-playbook-installer projects list
go run ./cmd/k-playbook-installer projects scan
go run ./cmd/k-playbook-installer projects add <path> --env plain
go run ./cmd/k-playbook-installer projects status [path]
```

Der fruehere interaktive Terminal-UI-Command `ui` wurde bewusst entfernt. Die interaktive Oberflaeche ist jetzt die lokale Browser-GUI. Ohne Subcommand startet `k-playbook-installer` direkt die GUI; `gui` bleibt als expliziter Alias erhalten. Die skriptbaren CLI-Kommandos bleiben erhalten. `status` gibt fuer das aktuelle Verzeichnis denselben read-only Projektstatus als JSON aus, den die GUI fuer Projektkarten nutzt; `status <path>` tut dasselbe fuer einen expliziten Pfad. `projects status [path]` bleibt als strukturierter Alias erhalten. Der alte Pfadvertrag-Status ist explizit `status --path-contract`; `status --fix` bleibt auf die Pfadvertrag-Reparatur beschraenkt. `projects add` legt beim Speichern `K-PLAYBOOK.yaml` im Zielprojekt an, falls sie fehlt.

## Designentscheidungen

- Go bleibt die einzige Runtime fuer den Installer.
- Die GUI ist eine lokale Web-UI, keine native Desktop-App und kein Electron/Wails-Setup.
- Der HTTP-Server bindet nur auf `127.0.0.1` und verwendet einen zufaelligen freien Port.
- Der Browser wird automatisch geoeffnet. Plattformlogik: macOS `open`, Windows `rundll32`, Linux/Unix `xdg-open`.
- Die Web-Assets sind per `embed` im Go-Binary enthalten.
- Der Installer fuehrt keine Shell-Pipelines aus. Fachlogik ruft Go-Funktionen oder gezielte Prozesse wie `git pull --ff-only` auf.
- Der Git-Pull ist absichtlich `--ff-only`, damit keine Merge-Commits oder interaktiven Konfliktzustaende entstehen.
- Nach einem erfolgreichen Git-Pull vergleicht die GUI alle vorhandenen Release-Artefakte unter `dist/k-playbook-installer-*` per Hash vor/nach dem Pull. Wenn sich mindestens ein Artefakt geaendert hat, werden alle vorhandenen Artefakte nach `bin/k-playbook-installer-<os>-<arch>` gespiegelt, der Wrapper `bin/k-playbook-installer` installiert und `~/.local/bin/k-playbook-installer` als Symlink auf den Wrapper gesetzt. Die GUI zeigt dann einen Neustart-Hinweis.
- Lokale Installer-Daten liegen unter `~/dev/k-playbook/.k-playbook-local/` und sind nicht versioniert.
- Der Pfadvertrag ist Voraussetzung fuer Store-, Docs- und Pull-Funktionen.
- Pfad- und Repo-Erkennung basiert auf k-playbook-Markerdateien, nicht nur auf `.git`.

## Wichtige Pakete

### `internal/cli`

Cobra-basierte CLI. Registriert aktuell:

- `status`
- `gui`
- `projects`

`status` nutzt standardmaessig `projects.Status(path)` und schreibt eingeruecktes JSON nach stdout. Mit `--path-contract` nutzt es `pathcontract.Check()`, mit `--fix` optional `pathcontract.Repair()`. `projects` nutzt `projects` und `store`. `gui` startet `webui.Run()`.

Der Projektstatus enthaelt nur leichte read-only Checks: Projekt-Metadaten plus `playbook`, `setup`, `structure`, `docs`, `remediation`, `tasks`, `todo`, `reviews`, `enforcement`, `git`, `recommendations` und bei DevContainer-Projekten `devcontainer`. Der Status-Command darf keine Tests, Builds, Smoke-Tests, Scanner, CodeQL-Analysen oder andere aufwendige Checks starten.

Die GUI ergaenzt beim Laden gespeicherter Projekte sichere fehlende Defaults in vorhandener `K-PLAYBOOK.yaml`, bevor sie den Projektstatus bildet. Aktuell betrifft das den fehlenden `remediation:`-Block mit `direct-allowed`; bestehende Werte werden nicht ueberschrieben. Wenn eine kanonische `K-PLAYBOOK.yaml` gelesen oder angelegt wird, wird die alte Root-Datei `K-PLAYBOOK.MD` geloescht. Diese Schreiblogik liegt bewusst in `projects.EnsureConfigDefaults` bzw. `projects.EnsureConfig`, nicht im read-only Status selbst.

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
- `Status(path)`
- `StatusFromProject(project)`
- `EnsureConfigDefaults(path)`

`Status("")` bzw. CLI `status` nutzt das aktuelle Verzeichnis. Wenn der aktuelle Pfad das projektlokale `k-playbook/`-Unterverzeichnis ist und der Parent `K-PLAYBOOK.yaml` enthaelt, wird auf den Projekt-Root korrigiert.

Erkennung:

- DevContainer: `.devcontainer/devcontainer.json`
- Normal: Projektmarker wie `.git`, `K-PLAYBOOK.yaml`, `pyproject.toml`, `package.json`, `go.mod` oder Python-venv-Verzeichnisse `.venv/` und `venv/`
- Projektmarker: `.git`, `K-PLAYBOOK.yaml`, `pyproject.toml`, `package.json`, `go.mod`, `.devcontainer/devcontainer.json`

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
      "detected": ["go.mod"],
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
| `GET` | `/api/projects` | Gespeicherte Projekt-Auswahl laden, inklusive Projekt-Setup-Status (`K-PLAYBOOK.yaml`) und projektlokalem `k-playbook/docs`-Status |
| `DELETE` | `/api/projects` | Projekt nach bestaetigter GUI-Aktion aus der gespeicherten Installer-Liste entfernen; Projektdateien bleiben unveraendert |
| `POST` | `/api/projects` | Einzelnes Projekt speichern und fehlende `K-PLAYBOOK.yaml` minimal anlegen |
| `GET` | `/api/projects/config?path=...` | `K-PLAYBOOK.yaml` eines gespeicherten Projekts read-only fuer die Detailseite lesen |
| `POST` | `/api/projects/preview` | Einzelnes manuelles Projekt normalisieren und Umgebung erkennen, ohne zu speichern |
| `POST` | `/api/projects/structure` | Fehlende feste Projektstruktur und Initialdateien unter `k-playbook/` fuer ein gespeichertes Projekt anlegen |
| `POST` | `/api/projects/remediation` | `remediation:`-Block eines gespeicherten Projekts in `K-PLAYBOOK.yaml` auf den gewaehlten Modus setzen |
| `GET` | `/api/projects/scan?root=dev|home` | Projektkandidaten scannen |
| `GET` | `/api/git/status` | Read-only per `git ls-remote` pruefen, ob der Upstream-Branch von k-playbook einen anderen Commit zeigt |
| `POST` | `/api/git/pull` | `git pull --ff-only` im k-playbook-Repo ausfuehren; bei geaenderten `dist`-Installer-Binaries alle vorhandenen Artefakte nach `bin/` spiegeln, Wrapper und globalen Symlink installieren und Neustartbedarf melden |
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
   Wenn die OpenCode-/Claude-Registrierung aktualisiert werden muss, zeigt der Header nach dem initialen `/api/opencode/status` zusaetzlich den hervorgehobenen Button `Registrierung aktualisieren`. Das ist dieselbe Aktion wie im Assistenten-Registrierungsblock und wird bei OK-Status ausgeblendet.
2. Pfadvertrag.
3. Wenn OK: Pfadvertrag nur als kompakter Einzeiler.
4. Gespeicherte `Projekt-Auswahl` mit je einem gerahmten Block pro Projekt. Die Eyebrow lautet `Projekte`, nicht `Schritt 2`. Pro Projekt nutzt die GUI denselben erweiterten read-only Projektstatus wie die CLI `status`: `playbook`, `setup`, `structure`, `docs`, `remediation`, `tasks`, `todo`, `reviews`, `enforcement`, `git`, `recommendations` und bei DevContainer-Projekten `devcontainer`. Die Startseite zeigt diese Werte nur als kurze Statusliste, z. B. `Remediation-Policy: Direkt erlaubt`, `Dokumentation: vorhanden/fehlt`, `Tasks: keine offen`, `Git: sauber`.
   Jeder Projektblock ist als Ganzes anklickbar und wechselt auf die Detailseite. Bearbeitbare Werte, Hilfen und Entfernen-Aktionen liegen nicht in der Startseitenliste, sondern auf der Projekt-Detailseite.
5. Nur fuer gespeicherte Projekte mit Umgebung `devcontainer` enthaelt die kompakte Statusliste den Punkt `Playbook im Container`. Dort wird geprueft, ob `.devcontainer/devcontainer.json` den Mount `source=${localEnv:HOME}/dev/k-playbook,target=/workspaces/k-playbook,type=bind`, `postCreateCommand`, `postStartCommand` und `.devcontainer/setup-k-playbook.sh` enthaelt. Fehlende Eintraege werden auf der Detailseite mit `Eintrag setzen` reparierbar gemacht.
6. Button `Projekt hinzufuegen`.
7. Assistenten-Registrierungsblock fuer OpenCode und Claude.
8. Security-Tool-Preflight mit einer Zeile pro Tool und Status `OK ✓`, `FEHLT !` oder `OPTIONAL`. Dieser Block prueft nur `PATH`, Versionen und Projekt-venv-Scope; er installiert nichts.
9. Repository-Block mit `Git pull`; bei verfuegbarer neuer Version wird auch dieser Button hervorgehoben und zu `Zur neuen Version aktualisieren`. Nach erfolgreichem Pull laeuft `refreshAll()`, wodurch Git-Status, Pfadstatus, Projekt-Auswahl, DevContainer-Status, Assistenten-Registrierung, Security-Tools und Docs neu geprueft werden. Wenn sich dabei mindestens ein Installer-Artefakt unter `dist/` geaendert hat, spiegelt die GUI alle vorhandenen `dist/k-playbook-installer-*` nach `bin/`, installiert den Wrapper, setzt den globalen Symlink und zeigt den Hinweis, dass die GUI neu gestartet werden muss.
10. Docs-Block mit Markdown-Dateiliste. Ein Klick auf eine Datei oeffnet die gerenderte Markdown-Anzeige in einem Vollbild-Overlay, damit der Inhalt nicht durch die begrenzte Hauptspalte eingeengt wird. Das Overlay ist per `Schliessen`, Hintergrund-Klick oder `Escape` schliessbar.
11. Button `Schliessen`, der den lokalen Server beendet. Der Browser-Tab zeigt danach nur noch den Hinweis, dass das Fenster geschlossen werden kann.

### Projekt-Detailseite

Ein Klick auf einen Projektblock wechselt auf eine Projekt-Detailansicht.

Die Detailseite zeigt:

- Header mit Projektname, absolutem Pfad und `Zur Projektliste`.
- Einen eigenen Projektstatus-/Editor-Block mit denselben zentral abgeleiteten Statuswerten wie die Startseite. Bestehende Controls gibt es fuer Remediation, Projektstruktur, Dokumentation und DevContainer-Integration; die zusaetzlichen CLI-Statusfelder wie Tasks, TODO, Reviews, Enforcement, Git und Empfehlungen werden zunaechst read-only angezeigt.
- `Entfernen` loescht nach Bestaetigung nur den Eintrag aus der lokalen Installer-Projektliste, keine Projektdatei, und fuehrt danach zur Projektliste zurueck.
- Wenn `K-PLAYBOOK.yaml` in einem gespeicherten Projekt fehlt, zeigt der Editor einen Fehler und empfiehlt, das Projekt aus der Installer-Liste zu entfernen und neu einzubinden, weil neue Einbindungen die Datei direkt anlegen.
- Wenn `K-PLAYBOOK.yaml` vorhanden ist, zeigt der Editor eine Remediation-Policy-Auswahl mit Hilfe-Button; Aenderungen schreiben nur den `remediation:`-Block der projektlokalen YAML.
- Wenn die feste Struktur unvollstaendig ist, zeigt der Editor `Vervollstaendigen`.
- Wenn Docs fehlen oder leer sind, zeigt der Editor `/k-code2docs` mit Kopierbutton und Hilfe. Hinweise werden nicht durch die GUI ausgefuehrt, weil die Slash-Commands projektlokalen Kontext und Rueckfragen brauchen.
- Bei fehlender DevContainer-Integration zeigt der Editor `Eintrag setzen` und nutzt das vorhandene Host-Script fuer genau dieses Projekt.
- Darunter die komplette `K-PLAYBOOK.yaml` als read-only YAML-Anzeige.
- `YAML neu laden` liest die Datei erneut ueber `/api/projects/config?path=...`.
- `In VS Code oeffnen` nutzt einen `vscode://file/...`-Link auf die geladene `K-PLAYBOOK.yaml`; die GUI startet keinen Editor-Prozess selbst.

Statusaktionen wie Remediation-Policy aendern oder Projektstruktur vervollstaendigen aktualisieren danach die Detailkarte und laden die YAML erneut.

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

Der Block bildet den read-only Teil von `/k-install` und `/k-install-security-tools --preflight` ab. Er liest dieselbe kanonische Tool-Matrix wie das Shell-Installationsscript:

```text
global/security-tools.tsv
```

- Pflicht-Tools stehen in der Matrix und werden nicht im Go-Code dupliziert.
- Optional angezeigt: `docker` als Fallback-Kontext, aber nicht als durch k-playbook installierbares Tool.

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

`Projekt hinzufuegen` wechselt auf eine separate Scan-Ansicht.

Scan-Roots:

- `~/dev` als Default.
- `~` als explizite, potenziell langsamere Alternative.

Auswahlverhalten:

- Ganze Zeile ist klickbar und markiert die Zeile optisch.
- Jede Scan-Zeile hat rechts neben der Art-Auswahl einen eigenen Button `Hinzufuegen`.
- Immer nur ein Projekt wird pro Aktion hinzugefuegt.
- Ausgewaehlte Zeile wird optisch markiert.
- Erkannte Art wird vorausgewaehlt: `Normal` fuer normale Projekte, `DevContainer` fuer `.devcontainer/devcontainer.json`.
- Wenn die Art unbekannt ist, zeigt die Auswahl `Unbekannt - bitte auswaehlen`; `Hinzufuegen` fordert dann zur Auswahl von `Normal` oder `DevContainer` auf.
- Es gibt keinen zusaetzlichen Bestaetigungsdialog. Bei gesetzter Art legt `Hinzufuegen` direkt los.
- Beim Speichern legt Backend/CLI sofort die minimale `K-PLAYBOOK.yaml` gemaess `docs/k-playbook-format.md` an, falls sie fehlt, und vervollstaendigt die feste Projektstruktur inklusive `k-playbook/TODO.md` und `k-playbook/reviews/known-decisions.md`. Die initiale Remediation-Policy ist `direct-allowed` und kann danach in der Projektauflistung umgestellt werden.
- Nach dem Speichern springt die GUI zur Startseite zurueck.
- Manuelles Projekt-Hinzufuegen liegt ebenfalls auf der Scan-Seite und springt nach Speichern zur Startseite zurueck. Wenn die Art nicht erkannt wird, muss der Nutzer sie auswaehlen.

## Docs-Anzeige

Die GUI listet Markdown-Dateien aus dem zentralen Repo-Verzeichnis `docs/`. Die Dateiliste bleibt in der normalen Startseiten-Card; der gerenderte Inhalt wird in einem Vollbild-Overlay angezeigt, damit lange Markdown-Dateien, Tabellen und Codebloecke die verfuegbare Browserbreite besser ausnutzen.

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

Dieser Weg braucht kein lokal installiertes Go. `make install` ruft `scripts/install-installer.sh` auf; das Script spiegelt alle unterstuetzten Release-Artefakte aus `dist/` oder aus GitHub Releases nach `bin/`, installiert dort den Wrapper `bin/k-playbook-installer` und verlinkt `~/.local/bin/k-playbook-installer` auf diesen Wrapper.

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
dist/k-playbook-installer-<os>-<arch>
bin/k-playbook-installer-<os>-<arch>
bin/k-playbook-installer
~/.local/bin/k-playbook-installer
```

`make dist` baut plattformspezifische Artefakte nach `dist/` fuer `linux-amd64`, `linux-arm64`, `darwin-amd64` und `darwin-arm64`. Diese Artefakte sind fuer GitHub Releases gedacht und werden nicht versioniert. Das private Maintainer-Target `make -C priv release-artifacts` ruft dieses Root-Target auf.

`make build` baut alle plattformspezifischen Binaries nach `bin/` und installiert `bin/k-playbook-installer` als Wrapper. Der Wrapper erkennt per `uname` die aktuelle Plattform und startet per `exec` das passende Binary im selben Verzeichnis.

`make install` installiert ohne Go alle unterstuetzten Release-Artefakte aus `dist/` oder laedt sie von `https://github.com/kascada/k-playbook/releases/latest/download/`. Die erwarteten `dist/`-Asset-Namen entsprechen den `make dist`-Dateinamen, z. B. `k-playbook-installer-linux-amd64`. Dieser Weg spiegelt die Binaries nach `bin/`, installiert den Wrapper und setzt `~/.local/bin/k-playbook-installer` als Symlink auf den Wrapper.

`make install-from-source` baut zuerst alle repo-lokalen Binaries unter `bin/`, installiert den Wrapper, legt danach `~/.local/bin/k-playbook-installer` als Symlink auf diesen Wrapper an und prueft, ob `~/.local/bin` im `PATH` liegt. Dadurch aktualisiert ein spaeteres `make build` automatisch auch den globalen Aufruf. `make gui` startet immer den repo-lokalen Wrapper und funktioniert deshalb auch ohne frisch geladenen PATH. Diese Source-Targets brauchen Go auf dem Host. Falls `~/.local/bin` nicht im `PATH` ist und der Aufruf in einem normalen interaktiven Terminal laeuft, fragt `make path-setup`, ob das passende Shell-Profil automatisch ergaenzt werden soll. Nicht-interaktive Aufrufe bekommen nur den Hinweis.

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

### Lokale GUI debuggen

Wenn ein bereits gestarteter Installer eine URL wie `http://127.0.0.1:34531/` ausgibt, kann die laufende Browser-GUI ohne sichtbaren Browser technisch geprueft werden:

```bash
curl -sS http://127.0.0.1:34531/api/status
curl -sS http://127.0.0.1:34531/api/projects
curl -sS http://127.0.0.1:34531/
```

`/api/status` ist der erste Check. Wenn dort `OK:false` bzw. ein Code wie `EXPECTED_NOT_K_PLAYBOOK` kommt, blendet das Frontend die Projektbereiche bewusst aus und die Seite wirkt wie eine neue Installation. Dann zuerst die Pfad-/Root-Erkennung reparieren, bevor Frontend-Rendering gesucht wird.

Nach Go-Aenderungen am Installer den lokalen Entwickler-Build aktualisieren:

```bash
cd installer
go build -o bin/k-playbook-installer ./cmd/k-playbook-installer
./bin/k-playbook-installer status
./bin/k-playbook-installer
```

Ein bereits laufender GUI-Server nutzt weiter den alten Code im Speicher. Nach Backend- oder Embed-Asset-Aenderungen die GUI neu starten. Wenn der Server noch laeuft, kann er ueber seinen lokalen Shutdown-Endpunkt beendet werden:

```bash
curl -sS -X POST http://127.0.0.1:34531/api/shutdown
```

Die Repo-Erkennung darf nicht an einen einzelnen Slash-Command wie `commands/k-install.md` gekoppelt sein, weil Commands umbenannt oder ersetzt werden koennen. Stabiler sind Repo-Marker wie `AGENTS.md`, `README.md`, `docs/README.md`, `installer/go.mod` plus mindestens ein `commands/k-*.md`.

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
