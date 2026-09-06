# k-playbook

`k-playbook` ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und Checks. Er wird in ein Unterverzeichnis des Zielprojekts geklont, `<projekt>/k-playbook/`. Daneben liegen die projekteigenen Artefakte. Ein Projekt ist damit selbstgenügsam.

## Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt trägt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git
make -C k-playbook install
k-playbook
```

Ohne `make` geht derselbe Bootstrap direkt:

```bash
k-playbook/bin/install
```

Das Zielverzeichnis muss `k-playbook` heißen, denn Commands und Skills sprechen es so an.
Ohne Zielargument ergibt sich der Name aus dem Repo-Namen; gib keinen anderen an. Nur
wenn du aus einem Fork oder Mirror unter abweichendem Namen klonst, hänge `k-playbook`
als zweites Argument an.

**Go wird nicht gebraucht.** `bin/install` lädt das zur Plattform passende Release-Binary
und installiert es nach `~/.local/bin/k-playbook`. Auf dem Host und im DevContainer wird
es jeweils in der eigenen Umgebung ausgeführt und installiert daher das passende Binary.

**Die Installation braucht Netz.** Die Binaries liegen nicht im Repo, sondern als Assets
am Release, das die `VERSION` im Wurzelverzeichnis nennt. `bin/install` lädt das passende
Asset und prüft es gegen das mitgelieferte `SHA256SUMS`.

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
├── CLAUDE.md             Include-Datei mit der Zeile @AGENTS.md; die Richtung ist fest
├── .claude/
│   ├── commands/ ──┐     je ein Symlink pro Command
│   └── skills/     │     je ein Symlink pro Skill; OpenCode liest hier mit
├── .opencode/      │
│   └── commands/ ──┤
├── .cursor/        │
│   └── commands/ ──┤
├── k-playbook/   ←─┤     die Installation, vollständig ersetzbar
│   ├── commands/ skills/ rules/ reviews/ checks/
│   ├── bin/ scripts/
│   ├── k-playbook.md     mitgelieferte Instruktionsebene
│   └── installer/ docs/
└── k-playbook-local/ ←─┘ projekteigen, committed
    ├── rules/            Overlay zu k-playbook/rules/
    ├── reviews/          Overlay zu k-playbook/reviews/
    ├── checks/           Overlay zu k-playbook/checks/
    ├── commands/         Overlay zu k-playbook/commands/
    ├── skills/           Overlay zu k-playbook/skills/
    ├── results/          alles, was Reviews erzeugen; nicht versioniert
    ├── docs/             Projektwissen für AI-Sessions, nach Herkunft getrennt
    │   └── manual/       handgepflegte Doku; kein Command schreibt hier hinein
    ├── guidelines/
    ├── tasks/done/
    ├── priv/             Notizen und Zwischenstände
    ├── material/         Rohmaterial als Quelle für Docs, nie indiziert
    ├── k-playbook.md     projekteigene Instruktionsebene
    ├── TODO.md
    └── version-sources.yaml   Versionsquellen des Versionsinventars, handgepflegt
```

`k-playbook-local/` gehört ins Repository des Projekts. Drei Verzeichnisse stehen darin
zur Wahl — `results/`, `priv/` und `material/`; eines davon, `results/`, wird bei der
Einrichtung schon privat angelegt, alle drei bleiben umschaltbar. Review-Ergebnisse sind
ein Stand von einem Rechner und können gefundene Secrets im Klartext enthalten; bei
`priv/` und `material/` entscheidet das Projekt, und k-playbook schreibt dort von sich
aus keine `.gitignore`. Was gilt, zeigt und schaltet der Block **Lokale Einstellungen**
der Oberfläche — gemessen mit `git check-ignore`, nicht geraten.

Gleicher Name in `k-playbook-local/` ersetzt den mitgelieferten Eintrag vollständig; ein
leerer schaltet ihn ab. Das gilt für alle fünf Sorten — `rules/`, `reviews/`, `checks/`,
`commands/` und `skills/`.

Deshalb sind `.claude/commands/` und die anderen drei Ziele **echte Verzeichnisse mit
Einzel-Symlinks**, kein Verzeichnis-Symlink: nur so kommen beide Quellen an. Die
Oberfläche vergleicht den aufgelösten Katalog mit dem, was registriert ist, nennt
Abweichungen beim Namen und bietet an, sie zu beheben.

Was am Ende gilt, rechnet kein Command selbst aus:

```bash
k-playbook context
```

Verlinkt wird für Claude Code, OpenCode und Cursor. Skills stehen nur einmal unter
`.claude/skills`, weil OpenCode dieses Verzeichnis mitdurchsucht und Cursor kein
Skill-Konzept hat. `CLAUDE.md` ist eine kleine Include-Datei mit der Zeile `@AGENTS.md`:
Claude Code liest ausschließlich `CLAUDE.md`, OpenCode und Cursor bevorzugen `AGENTS.md`
— so gibt es genau eine Instruktionsdatei, und Claude Code lädt sie über den Import.

Die Richtung ist überall dieselbe. Bringt ein Projekt nur eine echte `CLAUDE.md` mit,
wird sie beim Einrichten nach `AGENTS.md` **umbenannt** und `CLAUDE.md` neu als Include
angelegt; der Inhalt bleibt erhalten und wird nicht verdoppelt. Eine `CLAUDE.md`, die
die Zeile `@AGENTS.md` schon trägt, ist eingerichtet — auch mit eigenen Hausregeln
daneben. Ein Symlink aus einer älteren Fassung wird beim ersten `k-playbook context`
verlustfrei ersetzt. Was sich nicht automatisch auflösen lässt — zwei echte Dateien
ohne Import-Zeile, eine bewusst gesetzte Verlinkung auf ein anderes Ziel, ein
git-ignoriertes `AGENTS.md` — wird als **Konflikt** gemeldet und nicht angefasst.
Solange der steht, sieht Claude Code vom Playbook nichts.

## Aktualisieren

Die Oberfläche prüft nach dem Start, ob die Installation hinter dem Remote liegt, und
zieht auf Knopfdruck nach. Dabei wird `k-playbook/` nur für den Pull beschreibbar gemacht
und danach wieder read-only gesetzt. Von Hand geht es genauso:

```bash
cd /pfad/zum/projekt
make -C k-playbook installer-update
```

`k-playbook/` enthält nichts Projekteigenes und ist dadurch vollständig ersetzbar.

**Hat `VERSION` dabei gewechselt, gehört zum neuen Stand ein anderes Binary.** Der
Hintergrunddienst beendet sich dann und nennt den Bootstrap; installiert wird er nicht
von selbst. Der Aufruf ist derselbe wie bei der Erstinstallation und in jeder Umgebung
einmal fällig — auf dem Host wie im DevContainer:

```bash
make -C k-playbook install
```

Ohne `make`: `k-playbook/bin/install`. Ein Zielprojekt hat kein eigenes `install`-Target;
der Aufruf geht immer über den Clone. Sind nur Commands, Regeln oder Rezepte neu, wechselt
`VERSION` nicht und der Dienst läuft weiter.

Die Verlinkung für die Assistenten zieht sich danach selbst nach — beim nächsten
`k-playbook context`, dem Aufruf am Anfang jeder Sitzung, oder beim nächsten Blick in den
Assistenten-Block der Oberfläche. Sie folgt dem Katalog des Projekts, nicht dem Weg, über
den die Installation aktualisiert wurde. Damit die neuen Commands ankommen, ist der
Assistent einmal neu zu starten; seine Liste liest er beim Start.

## Selbst bauen

Für den normalen Betrieb genügen die Release-Assets, die `bin/install` lädt. Wer am
Werkzeug arbeitet oder lieber selbst baut, braucht Go:

```bash
make dist        # baut alle Plattformen nach dist/, der Weg vor einem Release
make dist-host   # baut nur diese Plattform, deutlich schneller
make dev-install # baut diese Plattform und ersetzt ~/.local/bin/k-playbook
make gui         # dev-install und starten
```

Der Entwicklungs-Loop bleibt dadurch netzfrei: `make gui` baut, installiert und startet
den frisch gebauten Stand, ohne etwas zu laden. `dist/` ist nicht versioniert.

Gebaut wird mit denselben Flags, mit denen CI die Release-Assets baut — `-trimpath`,
`CGO_ENABLED=0`, `-buildvcs=false` und die in `installer/go.mod` festgenagelte
Toolchain —, damit beide Wege bitgleiche Binaries liefern. Genau darauf beruht das
versionierte `SHA256SUMS`.

Ein Release läuft in zwei Schritten, damit `VERSION` nie auf einen Tag ohne Downloads
zeigt:

```bash
make release VERSION=v0.2.0          # baut, committet VERSION und SHA256SUMS, pusht den Tag
# CI baut nach, prüft gegen SHA256SUMS und lädt die Assets hoch
make release-publish VERSION=v0.2.0  # bringt denselben Commit auf main
```

## Grundprinzipien

- Jedes Projekt trägt seine eigene Installation in einem Unterverzeichnis. Kein fester Hostpfad, kein globaler Symlink.
- Installation und Projekt-Eigentum sind strikt getrennt: `k-playbook/` wird bei jedem Update vollständig ersetzt, alles daneben nie angefasst.
- Mitgelieferte Regeln, Reviews und Checks werden nicht editiert. Ein Projekt weicht per Overlay ab: eine gleichnamige lokale Datei ersetzt vollständig, eine leere schaltet ab.
- Pfade stehen nicht in der Konfiguration. Sie ergeben sich aus dem Ort der `K-PLAYBOOK.yaml`.
- Tasks, Reviews und Ergebnisse bleiben projekteigene Artefakte unter `k-playbook-local/`.
- Projekte dürfen ihre eigenen venvs nutzen. Security-Tools werden davon getrennt host-/user-lokal oder in dedizierte k-playbook-Tool-venvs installiert; sie sind die eine bewusste Ausnahme von der Projektlokalität.
- Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt.

## Dokumentation

- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.
- [`docs/handbuch.md`](./docs/handbuch.md) - Zweck, Grundmodell und Standardabläufe.
- [`docs/k-playbook-format.md`](./docs/k-playbook-format.md) - der Kontrakt: `K-PLAYBOOK.yaml`, Struktur, Overlay.
- [`docs/installation.md`](./docs/installation.md) - Clone, Einrichtungsschritte, Security-Tools.
- [`docs/commands.md`](./docs/commands.md) - Index der Slash-Commands.
- [`docs/umbau.md`](./docs/umbau.md) - Stand der Umstellung, Festlegungen und offene Punkte.
