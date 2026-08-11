# k-playbook — das Werkzeug

Go-Modul hinter `bin/k-playbook`. Es richtet die k-playbook-Installation eines Projekts
ein und fuehrt dabei durch drei Schritte.

Das Werkzeug heisst `k-playbook`, nicht `k-playbook-installer`. Einrichten ist nicht seine
einzige Aufgabe; die jeweilige steckt im Subkommando.

## Aufruf

```bash
k-playbook/bin/k-playbook           # oeffnet die Oberflaeche im Browser
k-playbook/bin/k-playbook context   # gibt den aufgeloesten Arbeitsstand als JSON aus
k-playbook/bin/k-playbook help
```

Der Wrapper waehlt anhand von `uname` das passende Binary aus `dist/` und startet es.

Ohne Argument oeffnet das Programm die lokale Oberflaeche und blockiert, bis der Client
sich abmeldet, verschwindet oder `Ctrl+C` gedrueckt wird. Dabei raeumt es zuvor die
host-globale Verlinkung des alten Modells weg, falls noch welche liegt, und spiegelt sich
nach `~/.local/share/k-playbook/installation` — danach genuegt in jedem Projekt ein
blosses `k-playbook`.

`context` ist der Einstieg fuer Commands und Skills: es loest Verzeichnisse,
Instruktionsdateien und die drei Kataloge auf, damit kein Command die Overlay-Regeln
selbst anwenden muss. Es raeumt bewusst nichts auf — dessen Ausgabe wuerde das JSON
stoeren.

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
| Assistenten verlinken | `.claude/`, `.opencode/`, `.cursor/`, `CLAUDE.md`, Anstoss in `AGENTS.md` |

Dazu kommen ein rein lesender Block fuer die Security-Tools, die Remediation-Policy, die
mitgelieferte Doku, der aufgeloeste Kontext und das Aktualisieren per
`git pull --ff-only`.

Geschrieben wird ausschliesslich nach Bestaetigung, Schritt fuer Schritt. Eine vorhandene
`K-PLAYBOOK.yaml` wird nie ueberschrieben.

## Struktur

```text
installer/
├── cmd/k-playbook/main.go
├── internal/project/          Anker, Config, Struktur, Verlinkung, Kontext, Update, Tools
├── internal/hostinstall/      host-weite Kopie unter ~/.local aktuell halten
├── internal/legacy/           host-globale Altlasten des alten Modells entfernen
├── internal/webui/            Server, Endpunkte, eingebettete Oberflaeche
├── docs/architecture.md
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details.

Architektur, Endpunkte, Ablaeufe und Designentscheidungen stehen in
[`docs/architecture.md`](./docs/architecture.md). Diese Datei ist der Einstieg fuer
weitere Arbeiten am Werkzeug.

`installer/_old/` enthaelt den vollstaendigen Stand vor dem Umbau als Nachschlagewerk.
Verzeichnisse mit `_`-Praefix ignoriert die Go-Toolchain vollstaendig — der Code dort ist
nicht baubar und nur zum Lesen da.

## Pruefungen nach Aenderungen

```bash
cd installer
gofmt -w ./cmd ./internal
go mod tidy
go test ./...
go build -o /tmp/k-playbook ./cmd/k-playbook
```

Nicht `go build ./cmd/k-playbook` ohne `-o` verwenden — das legt ein Binary im Repo ab.
