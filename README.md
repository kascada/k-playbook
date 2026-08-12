# k-playbook

`k-playbook` ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und Checks. Er wird in ein Unterverzeichnis des Zielprojekts geklont, `<projekt>/k-playbook/`. Daneben liegen die projekteigenen Artefakte. Ein Projekt ist damit selbstgenügsam.

> **Umstellung.** Das projektlokale Modell ist umgesetzt und in der Doku beschrieben.
> Offen ist noch der Umbau der Scan-Reviews auf SARIF; er steht in
> [`docs/umbau.md`](./docs/umbau.md).

## Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt trägt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git k-playbook
k-playbook/bin/k-playbook
```

Das zweite Argument hinter der URL bestimmt den Verzeichnisnamen — er muss `k-playbook`
lauten, denn Commands und Skills sprechen ihn so an.

**Go wird nicht gebraucht.** `bin/k-playbook` ist ein Wrapper, der das zur Plattform
passende Binary aus `dist/` startet; die Binaries liegen fertig im Repo. Für macOS und
Linux gleichermaßen, was auch den Fall abdeckt, dass Host und DevContainer
unterschiedliche Plattformen sind.

Der letzte Aufruf startet die Oberfläche im Browser. Beim ersten Mal findet sie noch
keine `K-PLAYBOOK.yaml` und schlägt vor, wo sie angelegt wird: das Verzeichnis über dem
Clone. Mitvorgeschlagen wird, wo das Projekt-Repository liegt — entweder das
Hauptverzeichnis selbst oder ein Unterverzeichnis daneben, etwa wenn der Code parallel
zum Playbook ausgecheckt ist. Geschrieben wird erst nach Bestätigung:

```text
projekt/
├── K-PLAYBOOK.yaml     der Anker; sein Ort bestimmt das Hauptverzeichnis
└── k-playbook/         die Installation
```

Danach legt dieselbe Oberfläche die projekteigene Struktur an und richtet die Verlinkung
für die Assistenten ein.

## Verzeichnisstruktur

```text
projekt/
├── K-PLAYBOOK.yaml       der Anker
├── AGENTS.md             Instruktionen, eine Quelle für alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md
├── .claude/
│   ├── commands/ ──┐     je ein Symlink pro Command
│   └── skills/     │     je ein Symlink pro Skill; OpenCode liest hier mit
├── .opencode/      │
│   └── commands/ ──┤
├── .cursor/        │
│   └── commands/ ──┤
├── k-playbook/   ←─┤     die Installation, vollständig ersetzbar
│   ├── commands/ skills/ rules/ reviews/ checks/
│   ├── bin/ dist/ scripts/
│   ├── k-playbook.md     mitgelieferte Instruktionsebene
│   └── installer/ docs/
└── k-playbook-local/ ←─┘ projekteigen, committed
    ├── rules/            Overlay zu k-playbook/rules/
    ├── reviews/          Overlay zu k-playbook/reviews/
    ├── checks/           Overlay zu k-playbook/checks/
    ├── commands/         Overlay zu k-playbook/commands/
    ├── skills/           Overlay zu k-playbook/skills/
    ├── results/          alles, was Reviews erzeugen
    ├── docs/             Projektwissen für AI-Sessions
    ├── guidelines/
    ├── tasks/done/
    ├── priv/             Inhalt gitignored, Verzeichnis versioniert
    ├── k-playbook.md     projekteigene Instruktionsebene
    └── TODO.md
```

Gleicher Name in `k-playbook-local/` ersetzt den mitgelieferten Eintrag vollständig; ein
leerer schaltet ihn ab. Das gilt für alle fünf Sorten — `rules/`, `reviews/`, `checks/`,
`commands/` und `skills/`.

Deshalb sind `.claude/commands/` und die anderen drei Ziele **echte Verzeichnisse mit
Einzel-Symlinks**, kein Verzeichnis-Symlink: nur so kommen beide Quellen an. Die
Oberfläche vergleicht den aufgelösten Katalog mit dem, was registriert ist, nennt
Abweichungen beim Namen und bietet an, sie zu beheben.

Was am Ende gilt, rechnet kein Command selbst aus:

```bash
k-playbook/bin/k-playbook context
```

Verlinkt wird für Claude Code, OpenCode und Cursor. Skills stehen nur einmal unter
`.claude/skills`, weil OpenCode dieses Verzeichnis mitdurchsucht und Cursor kein
Skill-Konzept hat. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`: Claude Code liest
ausschließlich `CLAUDE.md`, OpenCode bevorzugt `AGENTS.md` — so landet jede Änderung in
beiden.

## Aktualisieren

Die Oberfläche prüft nach dem Start, ob die Installation hinter dem Remote liegt, und
zieht auf Knopfdruck nach. Von Hand geht es genauso:

```bash
cd /pfad/zum/projekt/k-playbook
git pull --ff-only
```

`k-playbook/` enthält nichts Projekteigenes und ist dadurch vollständig ersetzbar.

## Selbst bauen

Die mitgelieferten Binaries genügen für den normalen Betrieb. Wer am Werkzeug selbst
arbeitet oder lieber selbst baut, braucht Go:

```bash
make dist   # baut alle Plattformen nach dist/
make gui    # baut und startet die Oberfläche
```

`make dist` ist das einzige Build-Target und verwendet dieselben Flags wie die
ausgelieferten Artefakte, damit beide Wege dasselbe Ergebnis liefern. `make gui` ist der
Weg beim Entwickeln: es startet den frisch gebauten Stand.

## Grundprinzipien

- Jedes Projekt trägt seine eigene Installation in einem Unterverzeichnis. Kein fester Hostpfad, kein globaler Symlink.
- Installation und Projekt-Eigentum sind strikt getrennt: `k-playbook/` wird bei jedem Update vollständig ersetzt, alles daneben nie angefasst.
- Mitgelieferte Regeln, Reviews und Checks werden nicht editiert. Ein Projekt weicht per Overlay ab: eine gleichnamige lokale Datei ersetzt vollständig, eine leere schaltet ab.
- Pfade stehen nicht in der Konfiguration. Sie ergeben sich aus dem Ort der `K-PLAYBOOK.yaml`.
- Tasks, Reviews und Ergebnisse bleiben projekteigene Artefakte unter `k-playbook-local/`.
- Security-Tools werden host- oder user-lokal installiert, nie in Projekt-venvs. Sie sind die eine bewusste Ausnahme von der Projektlokalität.
- Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt.

## Dokumentation

- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.
- [`docs/handbuch.md`](./docs/handbuch.md) - Zweck, Grundmodell und Standardabläufe.
- [`docs/k-playbook-format.md`](./docs/k-playbook-format.md) - der Kontrakt: `K-PLAYBOOK.yaml`, Struktur, Overlay.
- [`docs/installation.md`](./docs/installation.md) - Clone, Einrichtungsschritte, Security-Tools.
- [`docs/commands.md`](./docs/commands.md) - Index der Slash-Commands.
- [`docs/umbau.md`](./docs/umbau.md) - Stand der Umstellung, Festlegungen und offene Punkte.
