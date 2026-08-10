# Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt traegt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git k-playbook
k-playbook/bin/k-playbook
```

Das Argument hinter der URL bestimmt den Verzeichnisnamen. Es muss `k-playbook` lauten —
Commands und Skills sprechen das Verzeichnis so an.

**Go wird nicht gebraucht.** `bin/k-playbook` ist ein Wrapper, der das zur Plattform
passende Binary aus `dist/` startet; die Binaries liegen fertig im Repo. Fuer macOS und
Linux gleichermassen, was auch den Fall abdeckt, dass Host und Container unterschiedliche
Plattformen sind.

## Die drei Schritte

Der letzte Aufruf startet die Oberflaeche im Browser. Sie fuehrt durch drei Schritte und
schreibt jeden erst nach Bestaetigung.

### 1. Konfiguration anlegen

Beim ersten Mal findet die Oberflaeche noch keine `K-PLAYBOOK.yaml` — nach einem frischen
Clone kann es sie nicht geben. Statt zu raten, schlaegt sie einen Ort vor und laesst ihn
bestaetigen. Kandidaten in dieser Reihenfolge:

1. das Git-Repository, in dem der Aufruf stattfindet,
2. der Ort abgeleitet aus der Lage des Binaries,
3. das Arbeitsverzeichnis.

Mitvorgeschlagen wird, wo das Projekt-Repository liegt — entweder das Hauptverzeichnis
selbst oder ein Unterverzeichnis daneben, etwa wenn der Code parallel zum Playbook
ausgecheckt ist. Das Ergebnis:

```text
projekt/
├── K-PLAYBOOK.yaml     der Anker; sein Ort bestimmt das Hauptverzeichnis
└── k-playbook/         die Installation
```

Eine vorhandene `K-PLAYBOOK.yaml` wird nie ueberschrieben. Das Format steht in
[`k-playbook-format.md`](./k-playbook-format.md).

### 2. Projekteigene Struktur anlegen

Daneben entsteht `k-playbook-local/` mit allem, was dem Projekt gehoert:

```text
k-playbook-local/
├── rules/         Overlay zu k-playbook/rules/
├── reviews/       Overlay zu k-playbook/reviews/
├── checks/        Overlay zu k-playbook/checks/
├── results/       alles, was Reviews erzeugen
├── docs/          Projektwissen fuer AI-Sessions
├── guidelines/
├── tasks/done/
├── priv/          Inhalt gitignored, Verzeichnis versioniert
└── TODO.md
```

Jedes Verzeichnis traegt eine `README.md` mit seinem Zweck — auch weil Git leere
Verzeichnisse nicht speichert und sie sonst nach einem Clone des Projekts fehlen wuerden.
Vorhandene Dateien bleiben unberuehrt, auch READMEs mit eigenem Text.

`k-playbook-local/` gehoert ins Repository des Projekts und wird committet.

### 3. Assistenten verlinken

Verlinkt wird fuer Claude Code, OpenCode und Cursor:

```text
projekt/
├── AGENTS.md             Instruktionen, eine Quelle fuer alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md
├── .claude/
│   ├── commands  ──┐     Symlink
│   └── skills      │     Symlink; OpenCode liest hier mit
├── .opencode/      │
│   └── commands  ──┤     Symlink
└── .cursor/        │
    └── commands  ──┘     Symlink
```

Skills stehen nur einmal unter `.claude/skills`: OpenCode durchsucht dieses Verzeichnis
mit, Cursor kennt kein Skill-Konzept. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, weil
Claude Code ausschliesslich `CLAUDE.md` liest und OpenCode `AGENTS.md` bevorzugt — so
landet jede Aenderung in beiden. Fehlt `AGENTS.md`, wird nichts angelegt; die Datei
gehoert dem Projekt.

Die Verlinkung ist projektlokal. Es wird nichts in `~/.config/opencode/` oder
`~/.claude/` geschrieben. Dadurch kann ein Rechner mehrere Projekte mit
unterschiedlichen k-playbook-Staenden tragen, ohne dass sie sich gegenseitig
ueberschreiben.

Nach Aenderungen an Commands oder Skills muss der jeweilige Assistent neu gestartet
werden — beide erfassen sie beim Start.

## Aktualisieren

```bash
cd /pfad/zum/projekt/k-playbook
git pull
```

`k-playbook/` enthaelt nichts Projekteigenes und ist dadurch vollstaendig ersetzbar —
auch per `rm -rf` und neuem Clone. `K-PLAYBOOK.yaml` und `k-playbook-local/` liegen
daneben und bleiben unberuehrt.

Nach einem Update mit neuen Commands die Oberflaeche noch einmal starten, damit die
Verlinkung nachgezogen wird.

## Security-Tools

Security-Tools werden host- oder user-lokal installiert, nie in ein Projekt-venv. Sie
sind die eine bewusste Ausnahme von der Projektlokalitaet: ein Scanner gehoert zur
Arbeitsumgebung, nicht zum Projekt.

Die kanonische Matrix liegt in [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv).
Sie wird vom Installationsskript und von der Oberflaeche gelesen; die Liste steht nicht
zusaetzlich im Go-Code.

Pflicht-Tools:

| Tool | Rolle |
|---|---|
| `gitleaks` | Secret-Scanning |
| `trufflehog` | tiefes Secret-Scanning |
| `pip-audit` | Python Dependency-CVEs |
| `trivy` | Filesystem-, Container- und IaC-CVEs |
| `syft` | SBOM-Erzeugung |
| `grype` | SBOM-/Dependency-CVE-Auswertung |

`docker` ist optional und wird als Fallback-Kontext angezeigt, aber nicht durch
k-playbook installiert.

Die Oberflaeche zeigt den Status read-only und installiert nichts. Alles Weitere macht
das Skript selbst:

```bash
k-playbook/scripts/install-security-tools.sh                       # Status, das ist der Default
k-playbook/scripts/install-security-tools.sh --install missing     # fragt vor der Installation
k-playbook/scripts/install-security-tools.sh --help                # erklaert die Methoden
```

`--method` waehlt zwischen `auto`, `native`, `docker`, `pipx` und `venv`. Ohne `--yes`
zeigt das Skript den Plan und fragt.

**Vor der Installation darf kein Projekt-venv aktiv sein.** Sonst wird ein Tool aus dem
venv faelschlich als host-global vorhanden erkannt. Falls `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

Python-CLI-Tools gehoeren in `pipx` oder in ein dediziertes k-playbook-Tool-venv unter
`~/.local/share/k-playbook/`, nicht in `<projekt>/.venv`.

## Selbst bauen

Die mitgelieferten Binaries genuegen fuer den normalen Betrieb. Wer am Werkzeug selbst
arbeitet oder lieber selbst baut, braucht Go:

```bash
make dist   # baut alle Plattformen nach dist/
make gui    # baut und startet die Oberflaeche
```

`make dist` ist das einzige Build-Target und verwendet dieselben Flags wie die
ausgelieferten Artefakte, damit beide Wege dasselbe Ergebnis liefern. `make gui` ist der
Weg beim Entwickeln: es startet den frisch gebauten Stand.

Optional laesst sich das Werkzeug host-weit verfuegbar machen:

```bash
make install-from-source   # verlinkt ~/.local/bin/k-playbook auf bin/k-playbook
```

Das ist reiner Komfort. Der Regelfall bleibt der Aufruf ueber `bin/k-playbook` im
jeweiligen Clone, denn jedes Projekt hat seinen eigenen.

## Verifikation

Checkliste fuer ein Projekt:

- [ ] `K-PLAYBOOK.yaml` liegt im Hauptverzeichnis, nicht in `k-playbook/`.
- [ ] `schema_version: 3` ist gesetzt.
- [ ] `project.repo_root` zeigt auf das Projekt-Repository, `project.vcs` ist `git` oder `none`.
- [ ] `k-playbook/` ist ein eigener Clone und enthaelt nichts Projekteigenes.
- [ ] `k-playbook-local/` existiert vollstaendig und ist im Projekt-Repository committet.
- [ ] `.claude/commands`, `.claude/skills`, `.opencode/commands` und `.cursor/commands`
      sind Symlinks nach `k-playbook/`.
- [ ] `CLAUDE.md` ist ein Symlink auf `AGENTS.md`.

## Fehlersuche

**Slash-Commands tauchen nicht auf.** Die Symlinks in `.claude/`, `.opencode/` bzw.
`.cursor/` pruefen, danach den Assistenten neu starten. Wenn die Symlinks fehlen, die
Oberflaeche noch einmal starten und die Verlinkung herstellen lassen.

**Skills werden nicht getriggert.** `.claude/skills` muss auf `k-playbook/skills` zeigen,
und unter jedem Skill-Ordner muss `SKILL.md` liegen. Danach den Assistenten neu starten.

**Das Werkzeug findet kein Projekt.** Dann fehlt die `K-PLAYBOOK.yaml` oberhalb des
Aufrufortes. Die Suche laeuft ab dem Arbeitsverzeichnis aufwaerts bis `$HOME` bzw. `/`
und raet bewusst nicht. Die Oberflaeche schlaegt dann einen Ort vor.

**Das Binary fehlt.** `bin/k-playbook` meldet, welches Artefakt es unter `dist/` erwartet
hat. Entweder `git pull` oder `make dist`.
