# k-playbook

`k-playbook` ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und Checks. Er wird in ein Unterverzeichnis des Zielprojekts geklont, `<projekt>/k-playbook/`. Daneben liegen die projekteigenen Artefakte. Ein Projekt ist damit selbstgenuegsam.

> **Umstellung laeuft.** Alles bis zum Abschnitt [Altes Modell](#altes-modell) ist
> aktuell; was darunter steht, wird der Reihe nach ersetzt. Der Stand steht in
> [`docs/umbau.md`](./docs/umbau.md).

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

Danach richtet dieselbe Oberflaeche die Verlinkung fuer die Assistenten ein
(`.claude/commands` und `.claude/skills`).

### Aktualisieren

```bash
cd /pfad/zum/projekt/k-playbook
git pull
```

`k-playbook/` enthaelt nichts Projekteigenes und ist dadurch vollstaendig ersetzbar.

### Selbst bauen

Wer die Binaries lieber selbst erzeugt statt die mitgelieferten zu nehmen, braucht Go:

```bash
cd /pfad/zum/projekt/k-playbook
make dist
```

`make dist` ist das einzige Build-Target und verwendet dieselben Flags wie die
ausgelieferten Artefakte, damit beide Wege dasselbe Ergebnis liefern.

## Grundprinzipien

- Jedes Projekt traegt seine eigene Installation in einem Unterverzeichnis. Kein fester Hostpfad, kein globaler Symlink.
- Installation und Projekt-Eigentum sind strikt getrennt: `k-playbook/` wird bei jedem Update vollstaendig ersetzt, alles daneben nie angefasst.
- Mitgelieferte Regeln, Reviews und Checks werden nicht editiert. Ein Projekt weicht per Overlay ab: gleichnamige lokale Datei ersetzt, `overlay.<kind>.disabled` schaltet ab.
- Pfade werden aus `K-PLAYBOOK.yaml` gelesen und nicht geraten.
- Docs, Tasks, Reviews und Results bleiben projektlokale Artefakte.
- Security-Tools werden host- oder user-lokal installiert, nie in Projekt-venvs. Sie sind die eine bewusste Ausnahme von der Projektlokalitaet.

## Dokumentation

- [`docs/umbau.md`](./docs/umbau.md) - Stand der Umstellung, Festlegungen und offene Punkte.
- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.
- [`docs/pr-review.md`](./docs/pr-review.md) - PR-Review-Flow.
- [`docs/code-review.md`](./docs/code-review.md) - Review-Rezepte, Results-Summary, Remediation und Handoffs.
- [`docs/task-flow.md`](./docs/task-flow.md) - Task-Erzeugung, Review-Loop und Ausfuehrung.
- [`docs/reviews-and-results.md`](./docs/reviews-and-results.md) - Artefaktmodell fuer Reviews, Findings und Remediation.

---

## Altes Modell

**Alles ab hier beschreibt den abgeloesten Stand** mit zentraler Basisinstallation,
`_dist/` und dem Binary `k-playbook-installer`. Keiner dieser Aufrufe funktioniert im
aktuellen Stand. Die Abschnitte bleiben stehen, bis sie der Reihe nach ersetzt sind; was
schon entschieden ist, steht in [`docs/umbau.md`](./docs/umbau.md).

### Weitere Kommandos

| Kommando | Zweck |
|---|---|
| `k-playbook-installer update` | ersetzt `_dist`; alles daneben bleibt unangetastet |
| `k-playbook-installer restore` | stellt `_dist` nach einem `git clone` wieder her |
| `k-playbook-installer migrate` | stellt ein Projekt von `schema_version: 1` auf `2` um |
| `k-playbook-installer status` | read-only Projektstatus als JSON |
| `k-playbook-installer version` | Payload-Version dieses Binaries |

Alle Kommandos finden das Projekt selbst, indem sie vom Arbeitsverzeichnis aufwaerts nach
`k-playbook/K-PLAYBOOK.yaml` suchen. Ein Pfad als Argument ist optional.

`_dist/` steht in der `.gitignore` und fehlt darum nach einem frischen Clone. Die Version
steht in `K-PLAYBOOK.yaml`, `restore` stellt den Zustand daraus wieder her.

### Bestehende Projekte migrieren

```bash
cd /pfad/zum/projekt
k-playbook-installer migrate --dry-run   # zeigt die Aenderungen
k-playbook-installer migrate
```

Die Migration verschiebt `K-PLAYBOOK.yaml` nach `k-playbook/`, kuerzt die `paths.*`-Werte um
das `k-playbook/`-Praefix und hebt `project.repo_root` eine Ebene an. Tasks, Reviews,
Ergebnisse und Docs liegen bereits richtig und werden nicht bewegt. Unbekannte Felder
bleiben erhalten.

### Browser-GUI

```bash
k-playbook-installer gui
```

Die GUI bildet noch das alte zentrale Modell ab. Auf Projekten mit `schema_version: 2`
verweigert sie jede schreibende Aktion mit einem klaren Hinweis, statt die Migration
rueckgaengig zu machen. Fuer migrierte Projekte ist die Kommandozeile der Weg.

### DevContainer

Die DevContainer-Integration setzt noch auf den alten Bind-Mount des Basis-Repos und wird
mit der GUI umgebaut. Im neuen Modell braucht sie ihn nicht mehr: `_dist/` liegt im Projekt
und ist im Container dadurch automatisch vorhanden.

### Bausteine

Die Payload unter `installer/payload/` wird per `go:embed` ins Binary gepackt und im
Zielprojekt nach `k-playbook/_dist/` entpackt.

| Bereich | Zweck | Im Projekt unter |
|---|---|---|
| `installer/payload/commands/` | Slash-Commands `/k-<name>` | `_dist/commands/` |
| `installer/payload/skills/` | Skills und Playbooks | `_dist/skills/` |
| `installer/payload/rules/` | mitgelieferte Enforcement-Regeln | `_dist/rules/` |
| `installer/payload/reviews/` | Review-Rezepte fuer `/k-review` | `_dist/reviews/` |
| `installer/payload/checks/` | generische Checks | `_dist/checks/` |
| `installer/payload/bin/k-check` | Check-Runner | `_dist/bin/k-check` |
| `installer/` | Installer-Quellcode | nicht mitgeliefert |
| `docs/`, `prompts/` | Doku und Auftraege fuer die Entwicklung | nicht mitgeliefert |
