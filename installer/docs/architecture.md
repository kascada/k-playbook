# Architektur

Session-Memory fuer die Arbeit am Werkzeug unter `installer/`. Diese Datei zuerst lesen,
bevor der Code erneut analysiert wird.

## Ziel

`k-playbook` richtet die k-playbook-Installation eines Projekts ein. Es findet den Anker,
legt Konfiguration und projekteigene Struktur an und verlinkt die Assistenten. Alles
Weitere — Reviews, Tasks, Checks — machen Commands und Skills im Assistenten, nicht
dieses Programm.

Das Werkzeug ist ein eigenstaendiges Go-Modul unter `installer/`, wird aber als
`bin/k-playbook` aus dem Repo-Root heraus aufgerufen.

## Aufbau

```text
installer/
├── cmd/k-playbook/main.go       raeumt Altlasten weg, startet webui.Run()
├── internal/legacy/
│   └── global.go                host-globale Registrierung des alten Modells entfernen
├── internal/project/
│   ├── discover.go              Anker finden
│   ├── environment.go           was liegt hier vor
│   ├── config.go                Config lesen, Ort vorschlagen, anlegen
│   ├── local.go                 projekteigene Struktur pruefen und anlegen
│   ├── links.go                 Assistenten-Verlinkung pruefen und herstellen
│   ├── remediation.go           remediation:-Block lesen und setzen
│   └── tools.go                 Security-Tool-Preflight ueber das Skript
├── internal/webui/
│   ├── server.go                Routen, Lebenszyklus
│   ├── browser.go               Browser oeffnen, Container erkennen
│   ├── config.go local.go assistant.go tools.go remediation.go
│   └── static/                  index.html, app.js, styles.css
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details. Die
Trennung haelt die Fachlogik testbar.

## Anker finden

Der zentrale Mechanismus, in `project/discover.go`:

1. Ab `startDir` aufwaerts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
2. Fund: `ProjectDir = <dir>`, `PlaybookDir = <dir>/k-playbook`.
3. Grenze sind `$HOME` und `/`, jeweils einschliesslich.
4. Nichts gefunden: `ErrNotFound`. Nicht raten, nichts anlegen.

**Die Suche darf nicht am Git-Worktree-Root abbrechen.** `<projekt>/k-playbook/` ist ein
eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, kaeme sonst nie an
die Konfiguration eine Ebene darueber. Das ist der haeufigste Fall, nicht der Sonderfall.

`homeDir()` loest Symlinks auf, damit der Vergleich auch bei verlinktem `$HOME` greift.

Es gibt **keine Fallunterscheidung** zwischen Entwicklungsrepo und Zielprojekt. Beide
tragen ihre eigene `K-PLAYBOOK.yaml` und werden gleich behandelt. Der einzige Unterschied
ist installiert oder nicht — siehe `Environment` in `project/environment.go`.

## Ort vorschlagen, wenn nichts gefunden wurde

Nach einem `git clone` existiert noch keine Config, die Suche schlaegt also fehl. `Suggest()`
in `project/config.go` **darf** raten, anders als `Discover` — geschrieben wird ohnehin
erst nach Bestaetigung. Kandidaten in dieser Reihenfolge:

1. **Das Git-Repository, in dem der Aufruf stattfindet.** Wer das Werkzeug startet, steht
   in aller Regel in dem Projekt, das er meint. Der staerkste Hinweis.
2. **Aus dem Ort des Binaries.** Es liegt in `<X>/dist/`, also ist `X` die Installation.
   Ob `X` selbst das Hauptverzeichnis ist oder eine Ebene darunter liegt, haengt daran, ob
   die Installation geklont wurde oder das Repo selbst ist — beides kommt vor, deshalb
   stehen beide Orte zur Auswahl.
3. Das Arbeitsverzeichnis.

Ein frueherer Ansatz leitete das Hauptverzeichnis allein aus dem Binary-Pfad ab und
pruefte, ob das Zwischenverzeichnis `k-playbook` heisst. Das ging im Entwicklungsrepo
schief: dort heisst das Hauptverzeichnis selbst so, und der Vorschlag landete eine Ebene
zu hoch.

`suggestRepoRoot()` beantwortet eine andere Frage — nicht wo das Hauptverzeichnis `A`
liegt, sondern was in `project.repo_root` gehoert:

| Situation | Hauptverzeichnis | `repo_root` |
|---|---|---|
| `A/.git` vorhanden | `A` | `.` |
| `A/G/.git`, `A` selbst ohne `.git` | `A` | `G` |
| mehrere Kandidaten unter `A` | `A` | leer, der Nutzer waehlt |

## Config lesen ohne YAML-Parser

`ReadConfig()` liest zeilenweise statt mit einem YAML-Parser. Gelesen werden nur wenige
Skalare — `schema_version`, `project.repo_root`, `project.vcs` — und der zeilenweise
Zugriff laesst Kommentare, Reihenfolge und unbekannte Bloecke unangetastet, wenn spaeter
zurueckgeschrieben wird. Die Datei gehoert dem Projekt und kann Werte tragen, die das
Werkzeug nicht kennt.

`CreateConfig()` schreibt nie ueber eine vorhandene Datei. `schema_version` ist `3`.

## Projekteigene Struktur

`LocalStructure()` in `project/local.go` ist die einzige Quelle fuer das, was unter
`k-playbook-local/` liegt:

```text
rules/  reviews/  checks/  results/  docs/  guidelines/  tasks/  tasks/done/  priv/  TODO.md
```

Jedes Verzeichnis bekommt eine `README.md` mit seinem Zweck — **auch weil Git leere
Verzeichnisse nicht speichert** und sie sonst nach einem Clone des Projekts fehlen
wuerden. `priv/` bekommt zusaetzlich eine eigene `.gitignore`, die den Inhalt ausschliesst
und das Verzeichnis selbst versioniert laesst; so muss die Projekt-`.gitignore` nicht
angefasst werden.

`writeIfMissing()` schreibt nur, wenn nichts da ist. Vorhandene READMEs mit eigenem Text
bleiben unberuehrt.

Es gibt bewusst kein `commands/` und kein `skills/` darin — siehe naechster Abschnitt.

## Assistenten-Verlinkung

`Links()` in `project/links.go`:

| Link | Ziel | Wer liest |
|---|---|---|
| `.claude/commands` | `k-playbook/commands` | Claude Code |
| `.claude/skills` | `k-playbook/skills` | Claude Code, OpenCode |
| `.opencode/commands` | `k-playbook/commands` | OpenCode |
| `.cursor/commands` | `k-playbook/commands` | Cursor |
| `CLAUDE.md` | `AGENTS.md` | Claude Code |

Skills stehen nur einmal: OpenCode durchsucht neben `.opencode/skills/` auch
`.claude/skills/`, ein zweiter Link waere Dopplung. Cursor kennt kein Skill-Konzept.

`CLAUDE.md` zeigt auf `AGENTS.md`, weil Claude Code ausschliesslich `CLAUDE.md` liest und
OpenCode `AGENTS.md` bevorzugt. Ein Symlink statt eines Imports, damit eine Aenderung
immer in beiden ankommt — wer in `CLAUDE.md` schreibt, schreibt durch den Link hindurch.
Der Link ist `Optional`: fehlt `AGENTS.md`, ist nichts zu tun. Die Datei gehoert dem
Projekt und wird nicht angelegt.

**Weil die Links auf Verzeichnisse zeigen, kann es keine projekteigenen Commands geben.**
Ein Symlink zeigt auf genau eine Quelle. Gaebe es `k-playbook-local/commands/`, muesste
pro Datei verlinkt und nach jedem Update nachgezogen werden. Fuer Skills gilt dasselbe.

Zustaende in `LinkState`:

| Zustand | Bedeutung |
|---|---|
| `ok` | Symlink vorhanden, zeigt auf die Installation |
| `missing` | nichts vorhanden |
| `stale` | Symlink vorhanden, zeigt woandershin |
| `own-directory` | echtes Verzeichnis, das dem Projekt gehoert; wird per Einzeldatei-Links bestueckt |
| `blocked` | eine Datei steht im Weg; wird nicht angefasst |
| `no-source` | die Installation hat das Quellverzeichnis nicht |

`own-directory` faengt den Fall ab, dass ein Projekt bereits ein echtes `.claude/commands/`
mit eigenen Dateien hat. Dann wird nicht ersetzt, sondern bestueckt.

## Altlasten des globalen Modells

`RemoveGlobalLinks()` in `legacy/global.go` laeuft bei jedem Programmstart, vor der
Oberflaeche. Sie raeumt weg, was das abgeloeste host-globale Modell hinterlassen hat:

| Ort | Alte Form |
|---|---|
| `~/.claude/commands` | Verzeichnis-Symlink auf `<repo>/commands`, sonst Einzel-Symlinks darin |
| `~/.claude/skills` | Einzel-Symlinks auf die Skill-Verzeichnisse des Repos |
| `~/.config/opencode/command` | Einzel-Symlinks auf `<repo>/commands/*.md` |
| `~/.config/opencode/opencode.jsonc`, `.json` | `skills.paths` auf die Basisinstallation |

Bleiben diese Links liegen, sieht ein Assistent in **jedem** Projekt zusaetzlich die
Commands eines fremden Standes — genau das, was die projektlokale Installation aufloesen
soll. Deshalb der Aufraeumschritt beim Start und nicht in der Oberflaeche.

Erkannt wird an einem Pfadsegment `k-playbook` im Symlink-Ziel, unabhaengig davon, ob das
Ziel noch existiert: der Repo-Pfad war frei waehlbar, das Verzeichnis hiess aber immer so,
und ein toter Link ist genauso eine Altlast wie ein lebender. Echte Dateien und Symlinks,
die woandershin zeigen, bleiben unberuehrt; ein Verzeichnis faellt nur weg, wenn danach
nichts mehr darin steht. Aus der OpenCode-Config wird ausschliesslich der Top-Level-Key
`skills` geschnitten, wenn sein Wert ein `k-playbook` nennt — der Rest der Datei bleibt
Zeichen fuer Zeichen erhalten, Kommentare eingeschlossen. Deshalb ein eigener
JSONC-Scanner statt Parsen und Neuschreiben.

Gemeldet wird nur, wenn tatsaechlich etwas wegfaellt; auf einem sauberen Rechner ist der
Schritt still. Ein Fehler dabei haelt den Start nicht auf.

## Security-Tool-Preflight

`CheckTools()` in `project/tools.go` ruft
`k-playbook/scripts/install-security-tools.sh --json` auf und liest dessen JSON.

Die Tool-Liste steht **nicht** im Go-Code. Sie kommt aus `scripts/security-tools.tsv`,
derselben Matrix, die auch das Skript und die Review-Rezepte lesen.

Der Aufruf ist ausschliesslich lesend: `--json` prueft nur, ob die Binaries vorhanden
sind. Installiert wird bewusst im Terminal, weil das den Host veraendert und nicht das
Projekt. Ein Timeout von 30 Sekunden begrenzt den Aufruf, weil der Preflight je Tool ein
`--version` startet und eines davon haengen kann.

Bricht das Skript ab — etwa bei aktivem Projekt-venv — landet die erste stderr-Zeile in
der Fehlermeldung.

## Web-API

| Methode | Pfad | Zweck |
|---|---|---|
| `GET` | `/api/health` | Heartbeat in beide Richtungen |
| `POST` | `/api/client-gone` | Browser meldet Tab-/Fenster-Schliessen |
| `POST` | `/api/shutdown` | Server beenden |
| `GET` | `/api/config` | Anker suchen bzw. Ort vorschlagen |
| `POST` | `/api/config` | `K-PLAYBOOK.yaml` anlegen |
| `GET` | `/api/local` | projekteigene Struktur pruefen |
| `POST` | `/api/local` | fehlende Teile anlegen |
| `GET` | `/api/assistant` | Verlinkung pruefen |
| `POST` | `/api/assistant` | Verlinkung herstellen |
| `GET` | `/api/tools` | Security-Tool-Preflight, read-only |
| `GET` | `/api/remediation` | `remediation:`-Block lesen |
| `POST` | `/api/remediation` | `remediation:`-Block setzen |

Statische Assets liegen unter `/static/`, die Startseite unter `/`. `indexHandler`
rendert `static/index.html` als Template und liefert vorab mit, ob eine Installation
gefunden wurde.

## Lebenszyklus

Der Server bindet auf `127.0.0.1:0`, nimmt also einen freien Port, und gibt die URL im
Terminal aus.

Beenden funktioniert in beide Richtungen:

- Der Client ruft alle paar Sekunden `/api/health`. Bleibt er laenger als 5 Sekunden aus,
  ist das Browserfenster weg und der Server beendet sich.
- Beim Schliessen meldet der Browser `/api/client-gone` per `sendBeacon`. Danach wartet
  der Server 3 Sekunden, damit ein Reload ihn nicht abraeumt.
- Solange sich **nie** ein Client gemeldet hat, bleibt der Server stehen: der Browser kann
  noch unterwegs sein, oder die URL wird von Hand eingetragen.
- `Ctrl+C` und der `Schliessen`-Button gehen ebenfalls.

Der Browser wird nicht automatisch geschlossen, wenn der Server endet. Browser blockieren
das in vielen Faellen, und `open`/`xdg-open` liefern keinen verlaesslichen Tab-Handle.

In einem Container wird der Browser gar nicht erst geoeffnet; `containerMarker()` erkennt
das und weist auf die Port-Weiterleitung hin.

## Designentscheidungen

- Go ist die einzige Runtime. Keine Node-Toolchain, kein Build-Schritt fuer das Frontend.
- Die Oberflaeche ist eine lokale Web-UI, keine native App und kein Electron/Wails-Setup.
- Assets sind per `embed` im Binary. Ein Binary, keine Begleitdateien.
- Keine Shell-Pipelines. Fachlogik ruft Go-Funktionen; der einzige Fremdprozess ist das
  Security-Tool-Skript.
- Kein YAML-Parser. Zeilenweises Lesen erhaelt Kommentare und unbekannte Bloecke.
- Kein Projekt-Store. Es gibt keine Liste bekannter Projekte mehr — das Werkzeug arbeitet
  auf dem Projekt, in dem es liegt.
- Geschrieben wird ausschliesslich nach Bestaetigung, Schritt fuer Schritt.
- `dist/` wird mitversioniert, damit die Installation ohne Go auskommt.

## Abhaengigkeiten

Keine. `go.mod` fuehrt nur die Standardbibliothek — Cobra, Goldmark und die
Charmbracelet-Pakete des alten Stands sind mit ihm entfallen.

## Debuggen

Wenn ein laufendes Werkzeug eine URL wie `http://127.0.0.1:34531/` ausgibt:

```bash
curl -sS http://127.0.0.1:34531/api/config
curl -sS http://127.0.0.1:34531/api/local
curl -sS http://127.0.0.1:34531/api/assistant
curl -sS -X POST http://127.0.0.1:34531/api/shutdown
```

`/api/config` ist der erste Check: er sagt, ob ein Anker gefunden wurde oder ob die
Oberflaeche im Vorschlagsmodus laeuft.

Ein laufender Server nutzt weiter den Code im Speicher. Nach Backend- oder
Asset-Aenderungen neu starten.

## Der alte Stand

`installer/_old/` enthaelt rund 7800 Zeilen Go vor dem Umbau, per `git mv` verschoben und
damit in der Historie nachvollziehbar. Verzeichnisse mit `_`-Praefix ignoriert die
Go-Toolchain vollstaendig: kein Build, keine Tests, keine Imports. Der Code ist dort
**nicht baubar** — seine Imports zeigen auf Pfade, die es nicht mehr gibt. Er ist zum
Lesen und Herueberkopieren da.

| Ort | Inhalt |
|---|---|
| `_old/internal/install/config.go` | vollstaendiger Config-Vertrag: `PathKeys`, `ValidatePath`, `RenderConfig` |
| `_old/internal/install/migrate.go` | zeilenweise Migration, die Kommentare und unbekannte Bloecke erhaelt |
| `_old/internal/projects/status.go` | die elf Statuspruefungen der alten Oberflaeche |
| `_old/internal/webui/server.go` | die alte GUI mit allen Bedienablaeufen |
| `_old/payload/payload.go` | `go:embed` samt Extract-Logik des abgeloesten Modells |

## Offene Punkte

- Was der Projektstatus in der Oberflaeche zeigen soll, nachdem der Projekt-Store
  entfallen ist.
- Keine automatisierten Tests fuer die HTTP-Handler; getestet ist bisher nur
  `internal/project`.
- Release-Artefakte gibt es fuer macOS und Linux. Windows ist offen.
