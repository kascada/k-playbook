# k-playbook — das Werkzeug

Go-Modul hinter `bin/k-playbook`. Es richtet die k-playbook-Installation eines Projekts
ein und fuehrt dabei durch drei Schritte.

Das Werkzeug heisst `k-playbook`, nicht `k-playbook-installer`. Einrichten ist derzeit
seine einzige Aufgabe, aber nicht die einzige, die es bekommen soll; die Aufgabe steckt
kuenftig im Subkommando.

## Aufruf

```bash
k-playbook/bin/k-playbook
```

Der Wrapper waehlt anhand von `uname` das passende Binary aus `dist/` und startet es. Es
gibt keine Subkommandos: das Programm oeffnet die lokale Browser-Oberflaeche und
blockiert, bis der Client sich abmeldet, verschwindet oder `Ctrl+C` gedrueckt wird.

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
| Assistenten verlinken | `.claude/`, `.opencode/`, `.cursor/`, `CLAUDE.md` |

Dazu kommt ein rein lesender Block fuer die Security-Tools und der
`remediation:`-Block der Konfiguration.

Geschrieben wird ausschliesslich nach Bestaetigung, Schritt fuer Schritt. Eine vorhandene
`K-PLAYBOOK.yaml` wird nie ueberschrieben.

## Struktur

```text
installer/
├── cmd/k-playbook/main.go
├── internal/project/          Anker, Config, Struktur, Verlinkung, Tools
├── internal/webui/            Server, Endpunkte, eingebettete Oberflaeche
├── docs/architecture.md
├── go.mod
└── README.md
```

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
