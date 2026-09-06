# k-playbook — das Werkzeug

Go-Modul hinter dem direkt installierten Befehl `k-playbook`. Der Bootstrap liegt als
`bin/install` im Projekt-Clone; das Go-Programm richtet das Projekt selbst ein.

Das Werkzeug heißt `k-playbook`, nicht `k-playbook-installer`. Einrichten ist nicht seine
einzige Aufgabe; die jeweilige steckt im Subkommando.

## Aufruf

```bash
k-playbook                          # öffnet die Oberfläche im Browser
k-playbook config create
                                     # legt K-PLAYBOOK.yaml im aktuellen Verzeichnis an
k-playbook context                  # gibt den aufgelösten Arbeitsstand als JSON aus
k-playbook stop                     # beendet den Hintergrunddienst dieses Projekts
k-playbook help
```

`bin/install` lädt das passende Release-Binary, prüft es gegen `SHA256SUMS` und legt es
nach `~/.local/bin/k-playbook`.

Ohne Argument öffnet das Programm die lokale Oberfläche. Der Server dahinter läuft als
Hintergrunddienst je Projekt weiter, das Terminal ist sofort wieder frei; ein zweiter
Aufruf findet ihn und öffnet nur den Browser, `stop` beendet ihn. Davor räumt der Aufruf
die globale Assistenten-Verlinkung des alten Modells sowie Reste der früheren
Wrapper-Installation weg.

`context` ist der Einstieg für Commands und Skills: es löst Verzeichnisse,
Instruktionsdateien und die drei Kataloge auf, damit kein Command die Overlay-Regeln
selbst anwenden muss. Es räumt bewusst nichts auf — dessen Ausgabe würde das JSON
stören.

Zum Entwickeln:

```bash
make dist        # baut alle Plattformen nach dist/, der Weg vor einem Release
make dev-install # baut nur diese Plattform und ersetzt ~/.local/bin/k-playbook
make gui         # dev-install und starten
```

Alle drei bauen mit denselben Flags; eigener Build und Auslieferung liefern dasselbe
Ergebnis. `make dev-install` baut nur die eigene Plattform, weil im Entwicklungs-Loop
ohnehin nur die gestartet wird.

## Was es macht

| Schritt | Ergebnis |
|---|---|
| Konfiguration anlegen | `K-PLAYBOOK.yaml` im Hauptverzeichnis |
| Projekteigene Struktur anlegen | `k-playbook-local/` mit READMEs je Verzeichnis |
| Assistenten verlinken | `.claude/`, `.opencode/`, `.cursor/`, Anstoß in `AGENTS.md`, `CLAUDE.md` als Include-Datei mit der Zeile `@AGENTS.md`; eine mitgebrachte echte `CLAUDE.md` wird dabei nach `AGENTS.md` umbenannt, ein Symlink aus einer älteren Fassung durch den Include ersetzt, nicht auflösbare Lagen werden als Konflikt gemeldet |

Dazu kommen ein rein lesender Block für die Security-Tools, die Remediation-Policy, die
mitgelieferte Doku, der aufgelöste Kontext und das Aktualisieren per
`git pull --ff-only`. Die Installation wird dabei als read-only Vendor-Clone behandelt:
der Start der Oberfläche sperrt Schreibrechte, ein Update macht sie nur für den Pull
temporär beschreibbar.

Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt. Eine vorhandene
`K-PLAYBOOK.yaml` wird nie überschrieben.

Wenn die Oberfläche wegen eines Ankers in einem übergeordneten Verzeichnis nicht die
Ersteinrichtung zeigt, legt `k-playbook config create` den Anker bewusst ohne Suche an.
Optional nimmt der Befehl ein Hauptverzeichnis und mit `--repo-root` ein relativ dazu
liegendes Repository entgegen.

## Struktur

```text
installer/
├── cmd/k-playbook/main.go
├── internal/project/          Anker, Config, Struktur, Verlinkung, Kontext, Update, Tools
├── internal/legacy/           host-globale Altlasten des alten Modells entfernen
├── internal/guiproc/          Laufzeitdatei des Hintergrunddienstes, Prozessidentität, Abkoppeln
├── internal/webui/            Server, Endpunkte, eingebettete Oberfläche
├── docs/architecture.md
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details.

Architektur, Endpunkte, Abläufe und Designentscheidungen stehen in
[`docs/architecture.md`](./docs/architecture.md). Diese Datei ist der Einstieg für
weitere Arbeiten am Werkzeug.

Der Stand vor dem Umbau ist aus dem Baum entfernt und nur noch in der Git-Historie
erreichbar, zuletzt in Commit `02f78d3` unter `installer/_old/`.

## Prüfungen nach Änderungen

```bash
cd installer
gofmt -w ./cmd ./internal
go mod tidy
go test ./...
go build -o /tmp/k-playbook ./cmd/k-playbook
```

Nicht `go build ./cmd/k-playbook` ohne `-o` verwenden — das legt ein Binary im Repo ab.
