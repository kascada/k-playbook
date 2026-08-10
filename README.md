# k-playbook

`k-playbook` ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und Checks. Er wird in ein Unterverzeichnis des Zielprojekts geklont, `<projekt>/k-playbook/`. Daneben liegen die projekteigenen Artefakte. Ein Projekt ist damit selbstgenuegsam.

> **Umstellung laeuft.** Diese Datei beschreibt den Zielzustand. Einzelne Commands, Skills
> und Doku-Seiten folgen noch dem abgeloesten Layout; was im Detail nachzuziehen ist, steht
> unter „Nachzuziehen" in [`docs/umbau.md`](./docs/umbau.md).

## Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt traegt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git k-playbook
k-playbook/bin/k-playbook
```

Das zweite Argument hinter der URL bestimmt den Verzeichnisnamen — er muss `k-playbook`
lauten, denn Commands und Skills sprechen ihn so an.

**Go wird nicht gebraucht.** `bin/k-playbook` ist ein Wrapper, der das zur Plattform
passende Binary aus `dist/` startet; die Binaries liegen fertig im Repo. Fuer macOS und
Linux gleichermassen, was auch den Fall abdeckt, dass Host und DevContainer
unterschiedliche Plattformen sind.

Der letzte Aufruf startet die Oberflaeche im Browser. Beim ersten Mal findet sie noch
keine `K-PLAYBOOK.yaml` und schlaegt vor, wo sie angelegt wird: das Verzeichnis ueber dem
Clone. Mitvorgeschlagen wird, wo das Projekt-Repository liegt — entweder das
Hauptverzeichnis selbst oder ein Unterverzeichnis daneben, etwa wenn der Code parallel
zum Playbook ausgecheckt ist. Geschrieben wird erst nach Bestaetigung:

```text
projekt/
├── K-PLAYBOOK.yaml     der Anker; sein Ort bestimmt das Hauptverzeichnis
└── k-playbook/         die Installation
```

Danach legt dieselbe Oberflaeche die projekteigene Struktur an und richtet die Verlinkung
fuer die Assistenten ein.

## Verzeichnisstruktur

```text
projekt/
├── K-PLAYBOOK.yaml       der Anker
├── AGENTS.md             Instruktionen, eine Quelle fuer alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md
├── .claude/
│   ├── commands  ──┐     Symlink
│   └── skills      │     Symlink; OpenCode liest hier mit
├── .opencode/      │
│   └── commands  ──┤     Symlink
├── .cursor/        │
│   └── commands  ──┤     Symlink
├── k-playbook/   ←─┘     die Installation, vollstaendig ersetzbar
│   ├── commands/ skills/ rules/ reviews/ checks/
│   ├── bin/ dist/
│   └── installer/ docs/
└── k-playbook-local/     projekteigen, committed
    ├── rules/            Overlay zu k-playbook/rules/
    ├── reviews/          Overlay zu k-playbook/reviews/
    ├── checks/           Overlay zu k-playbook/checks/
    ├── results/          alles, was Reviews erzeugen
    ├── docs/             Projektwissen fuer AI-Sessions
    ├── guidelines/
    ├── tasks/done/
    ├── priv/             Inhalt gitignored, Verzeichnis versioniert
    └── TODO.md
```

Gleicher Dateiname in `k-playbook-local/rules/`, `reviews/` oder `checks/` ersetzt die
mitgelieferte Datei vollstaendig. Commands und Skills gibt es nur mitgeliefert.

Verlinkt wird fuer Claude Code, OpenCode und Cursor. Skills stehen nur einmal unter
`.claude/skills`, weil OpenCode dieses Verzeichnis mitdurchsucht und Cursor kein
Skill-Konzept hat. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`: Claude Code liest
ausschliesslich `CLAUDE.md`, OpenCode bevorzugt `AGENTS.md` — so landet jede Aenderung in
beiden.

## Aktualisieren

```bash
cd /pfad/zum/projekt/k-playbook
git pull
```

`k-playbook/` enthaelt nichts Projekteigenes und ist dadurch vollstaendig ersetzbar.

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

## Grundprinzipien

- Jedes Projekt traegt seine eigene Installation in einem Unterverzeichnis. Kein fester Hostpfad, kein globaler Symlink.
- Installation und Projekt-Eigentum sind strikt getrennt: `k-playbook/` wird bei jedem Update vollstaendig ersetzt, alles daneben nie angefasst.
- Mitgelieferte Regeln, Reviews und Checks werden nicht editiert. Ein Projekt weicht per Overlay ab: gleichnamige lokale Datei ersetzt vollstaendig, `overlay.<kind>.disabled` schaltet ab.
- Pfade stehen nicht in der Konfiguration. Sie ergeben sich aus dem Ort der `K-PLAYBOOK.yaml`.
- Tasks, Reviews und Ergebnisse bleiben projekteigene Artefakte unter `k-playbook-local/`.
- Security-Tools werden host- oder user-lokal installiert, nie in Projekt-venvs. Sie sind die eine bewusste Ausnahme von der Projektlokalitaet.
- Geschrieben wird ausschliesslich nach Bestaetigung, Schritt fuer Schritt.

## Dokumentation

- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.
- [`docs/handbuch.md`](./docs/handbuch.md) - Zweck, Grundmodell und Standardablaeufe.
- [`docs/k-playbook-format.md`](./docs/k-playbook-format.md) - der Kontrakt: `K-PLAYBOOK.yaml`, Struktur, Overlay.
- [`docs/installation.md`](./docs/installation.md) - Clone, Einrichtungsschritte, Security-Tools.
- [`docs/commands.md`](./docs/commands.md) - Index der Slash-Commands.
- [`docs/umbau.md`](./docs/umbau.md) - Stand der Umstellung, Festlegungen und offene Punkte.
