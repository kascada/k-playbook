# k-playbook — das Werkzeug

Go-Modul hinter dem direkt installierten Befehl `k-playbook`. Die Erstinstallation ist
`bin/install` — eigenständig über `curl … | bash` aufrufbar —; das Go-Programm richtet das
Projekt selbst ein und aktualisiert sich selbst aus dem Release.

Das Werkzeug heißt `k-playbook`, nicht `k-playbook-installer`. Einrichten ist nicht seine
einzige Aufgabe; die jeweilige steckt im Subkommando.

## Aufruf

```bash
k-playbook                          # öffnet die Oberfläche im Browser
k-playbook config create
                                     # legt .k-playbook/K-PLAYBOOK.yaml im aktuellen Verzeichnis an
k-playbook briefing                 # was für dieses Projekt gilt und was ansteht — der Einstieg jeder Sitzung
k-playbook upgrade [--check]        # Programm aktualisieren und das Projekt nachziehen
k-playbook context                  # gibt den aufgelösten Arbeitsstand als JSON aus (wird abgebaut)
k-playbook stop                     # beendet den Hintergrunddienst dieses Projekts
k-playbook help
```

`bin/install` bestimmt das neueste veröffentlichte Release (die Weiterleitung von
`releases/latest`, über HTTPS, ohne `git`), lädt das
Release-Binary dieser Plattform, prüft es gegen die `SHA256SUMS` des Tags und legt es
nach `~/.local/bin/k-playbook`; danach installiert es Kephalaion, wenn es fehlt:

```bash
curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash
```

Aus einem Clone aufgerufen (`make -C k-playbook install`) tut es dasselbe; `VERSION` und
`SHA256SUMS` des Clones liest es nicht.

Ohne Argument öffnet das Programm die lokale Oberfläche. Der Server dahinter läuft als
Hintergrunddienst je Projekt weiter, das Terminal ist sofort wieder frei; ein zweiter
Aufruf findet ihn und öffnet nur den Browser, `stop` beendet ihn. Davor räumt der Aufruf
die globale Assistenten-Verlinkung des alten Modells sowie Reste der früheren
Wrapper-Installation weg.

`briefing` ist der Einstieg jeder Sitzung — der Aufruf, den `AGENTS.md` nennt: der Text
der immer geltenden Anweisungen, die Reviews und Leitlinien als Verweise, die
eigene offene Arbeit und, bis zum Abbau, der Rest der Kontextausgabe. `context` löst
Verzeichnisse und die Kataloge als JSON auf, damit kein Command
die Overlay-Regeln selbst anwenden muss; die Commands rufen es noch, bis sie einzeln auf
das Briefing umziehen. Beide räumen bewusst nichts auf — deren Ausgabe würde die
Antwort stören. `upgrade` zieht Kephalaion, Programm und Projekt nach: Pull im Clone,
solange es ihn gibt; das Binary nur, wenn das laufende älter ist als der neueste
Release-Tag auf GitHub — dann direkt aus dem Release, geprüft gegen die `SHA256SUMS` des
Tags, auch ohne Clone —; dann `AGENTS.md`, `CLAUDE.md`, Verlinkung und
MCP-Einträge; `--check` meldet nur, was ein Lauf täte.

Es gibt genau zwei Versionsquellen: die eigene Version im Binary und den neuesten
Release-Tag. `k-playbook version` nennt die erste, `k-playbook version --all` zusätzlich
die mitgereiste VS-Code-Erweiterung und die zweite; die Umgebungsprüfung
(`k-playbook health`, das Briefing) warnt, wenn das Programm älter ist als das neueste
Release. `VERSION`, `SHA256SUMS` und `bin/install` eines Clones liest und ruft das
Programm dafür nicht mehr (Pakete `internal/release`, `internal/version`).

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
| Konfiguration anlegen | `.k-playbook/K-PLAYBOOK.yaml` im Hauptverzeichnis |
| Assistenten verlinken | `.claude/`, `.opencode/`, Anstoß in `AGENTS.md`, `CLAUDE.md` als Include-Datei mit der Zeile `@AGENTS.md`; eine mitgebrachte echte `CLAUDE.md` wird dabei nach `AGENTS.md` umbenannt, ein Symlink aus einer älteren Fassung durch den Include ersetzt, nicht auflösbare Lagen werden als Konflikt gemeldet |
| VS-Code-Erweiterung installieren | „k-playbook Workspace Tools“ im VS-Code-Server der Umgebung; beim Start des Dienstes im Hintergrund, ausdrücklich mit `k-playbook vscode install` |

Dazu kommen ein rein lesender Block für die Security-Tools, die Remediation-Policy, die
mitgelieferte Doku, der aufgelöste Kontext und das Aktualisieren per
`git pull --ff-only`. Die Installation wird dabei als read-only Vendor-Clone behandelt:
der Start der Oberfläche sperrt Schreibrechte, ein Update macht sie nur für den Pull
temporär beschreibbar.

Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt. Eine vorhandene
`K-PLAYBOOK.yaml` wird nie überschrieben.

`k-playbook-local/` legt das Werkzeug nicht mehr an, weder beim Start noch auf Knopfdruck
noch als Nebenwirkung eines Werkzeugs; was dort noch liegt, bleibt unberührt. Die
schreibenden Review-Wege (`k-playbook scan`, `k-playbook merge` und die MCP-Werkzeuge
`k_playbook_review_create`, `_scan`, `_merge`, `_write_ai_entry`) sind vorübergehend
gesperrt, bis die Ablage der Review-Ergebnisse umgebaut ist.

Wenn die Oberfläche wegen eines Ankers in einem übergeordneten Verzeichnis nicht die
Ersteinrichtung zeigt, legt `k-playbook config create` den Anker bewusst ohne Suche an.
Optional nimmt der Befehl ein Hauptverzeichnis und mit `--repo-root` ein relativ dazu
liegendes Repository entgegen.

## Struktur

```text
installer/
├── cmd/k-playbook/main.go
├── internal/project/          Anker, Config, Verlinkung, Kontext, Update, Tools
├── internal/legacy/           host-globale Altlasten des alten Modells entfernen
├── internal/guiproc/          Laufzeitdatei des Hintergrunddienstes, Prozessidentität, Abkoppeln
├── internal/webui/            Server, Endpunkte, eingebettete Oberfläche
├── internal/vscodeext/        die eingebettete VS-Code-Erweiterung: Erkennen, CLI wählen,
│                              installieren, Nachzug beim Dienststart
├── vscode/                    Quelle der Erweiterung (JavaScript, ohne Abhängigkeiten)
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

Nach einer Änderung unter `installer/vscode/` zusätzlich, aus dem Wurzelverzeichnis:

```bash
make vscode-test     # node --test über installer/vscode/test/
make vscode-vsix     # VSIX neu bauen; die Datei gehört in denselben Commit
```

Node braucht nur diese beiden Ziele. `make dist`, `make test` und die Release-CI kommen
ohne aus, weil die VSIX eingecheckt ist; `TestVSIXPasstZurQuelle` macht `make test` rot,
wenn sie nicht zur Quelle passt. Einzelheiten in [`../docs/vscode.md`](../docs/vscode.md).
