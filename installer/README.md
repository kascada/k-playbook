# k-playbook — das Werkzeug

Go-Modul hinter `bin/k-playbook`. Es richtet die k-playbook-Installation eines Projekts
ein und führt dabei durch drei Schritte.

Das Werkzeug heißt `k-playbook`, nicht `k-playbook-installer`. Einrichten ist nicht seine
einzige Aufgabe; die jeweilige steckt im Subkommando.

## Aufruf

```bash
k-playbook/bin/k-playbook           # öffnet die Oberfläche im Browser
k-playbook/bin/k-playbook context   # gibt den aufgelösten Arbeitsstand als JSON aus
k-playbook/bin/k-playbook help
```

Der Wrapper wählt anhand von `uname` das passende Binary aus `dist/` und startet es.

Ohne Argument öffnet das Programm die lokale Oberfläche und blockiert, bis der Client
sich abmeldet, verschwindet oder `Ctrl+C` gedrückt wird. Dabei räumt es zuvor die
host-globale Verlinkung des alten Modells weg, falls noch welche liegt, und spiegelt sich
nach `~/.local/share/k-playbook/installation` — danach genügt in jedem Projekt ein
bloßes `k-playbook`.

`context` ist der Einstieg für Commands und Skills: es löst Verzeichnisse,
Instruktionsdateien und die drei Kataloge auf, damit kein Command die Overlay-Regeln
selbst anwenden muss. Es räumt bewusst nichts auf — dessen Ausgabe würde das JSON
stören.

Zum Entwickeln:

```bash
make dist   # baut alle Plattformen nach dist/
make gui    # baut und startet
```

`make dist` ist das einzige Build-Target. Eigener Build und Auslieferung verwenden
dieselben Flags und liefern dasselbe Ergebnis.

## Was es macht

| Schritt | Ergebnis |
|---|---|
| Konfiguration anlegen | `K-PLAYBOOK.yaml` im Hauptverzeichnis |
| Projekteigene Struktur anlegen | `k-playbook-local/` mit READMEs je Verzeichnis |
| Assistenten verlinken | `.claude/`, `.opencode/`, `.cursor/`, `CLAUDE.md`, Anstoß in `AGENTS.md`; eine mitgebrachte echte `CLAUDE.md` wird dabei nach `AGENTS.md` umbenannt, nicht auflösbare Lagen werden als Konflikt gemeldet |

Dazu kommen ein rein lesender Block für die Security-Tools, die Remediation-Policy, die
mitgelieferte Doku, der aufgelöste Kontext und das Aktualisieren per
`git pull --ff-only`. Die Installation wird dabei als read-only Vendor-Clone behandelt:
der Start der Oberfläche sperrt Schreibrechte, ein Update macht sie nur für den Pull
temporär beschreibbar.

Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt. Eine vorhandene
`K-PLAYBOOK.yaml` wird nie überschrieben.

## Struktur

```text
installer/
├── cmd/k-playbook/main.go
├── internal/project/          Anker, Config, Struktur, Verlinkung, Kontext, Update, Tools
├── internal/hostinstall/      host-weite Kopie unter ~/.local aktuell halten
├── internal/legacy/           host-globale Altlasten des alten Modells entfernen
├── internal/webui/            Server, Endpunkte, eingebettete Oberfläche
├── docs/architecture.md
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details.

Architektur, Endpunkte, Abläufe und Designentscheidungen stehen in
[`docs/architecture.md`](./docs/architecture.md). Diese Datei ist der Einstieg für
weitere Arbeiten am Werkzeug.

`installer/_old/` enthält den vollständigen Stand vor dem Umbau als Nachschlagewerk.
Verzeichnisse mit `_`-Präfix ignoriert die Go-Toolchain vollständig — der Code dort ist
nicht baubar und nur zum Lesen da.

## Prüfungen nach Änderungen

```bash
cd installer
gofmt -w ./cmd ./internal
go mod tidy
go test ./...
go build -o /tmp/k-playbook ./cmd/k-playbook
```

Nicht `go build ./cmd/k-playbook` ohne `-o` verwenden — das legt ein Binary im Repo ab.
