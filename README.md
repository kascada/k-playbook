# k-playbook

`k-playbook` ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und Checks. Er wird in ein Unterverzeichnis des Zielprojekts installiert, konventionell `<projekt>/k-playbook/`. Dort liegt die mitgelieferte Installation unter `_dist/`, daneben die projekteigenen Artefakte: `K-PLAYBOOK.yaml`, Tasks, Reviews, Ergebnisse und Docs. Ein Projekt ist damit selbstgenuegsam.

> **Umstellung laeuft.** Die Kommandozeile setzt das projektlokale Modell vollstaendig um.
> Die Browser-GUI bildet noch das alte zentrale Modell ab und wird spaeter umgebaut; sie
> verweigert Schreibzugriffe auf bereits migrierte Projekte, damit sie die Migration nicht
> rueckgaengig macht. Der Kontrakt steht in [`docs/k-playbook-format.md`](./docs/k-playbook-format.md).

## Installation

k-playbook braucht ein Binary pro Host. Es traegt die komplette Installation in sich,
es gibt kein Repo zum Klonen und keinen festen Pfad mehr.

```bash
git clone git@github.com:kascada/k-playbook.git
cd k-playbook
make install
```

**Go wird dafuer nicht gebraucht.** `make install` nimmt die fertigen Binaries aus `dist/`
und faellt auf die GitHub Releases zurueck, wenn dort keine passende Plattform liegt.
Genau dafuer ist der Installer in Go geschrieben: ein Binary, keine Laufzeitumgebung.

Das geklonte Repo wird danach nur noch fuer die Weiterentwicklung gebraucht. Zielprojekte
haengen nicht daran, und der Ort spielt keine Rolle.

Fuer Arbeit am Installer selbst gibt es `make install-from-source`; das braucht Go.

## Projekt einbinden

Im Zielprojekt:

```bash
cd /pfad/zum/projekt
k-playbook-installer init
```

Das legt an:

```text
projekt/
├── .claude/commands -> ../k-playbook/_dist/commands
├── .claude/skills   -> ../k-playbook/_dist/skills
├── .gitignore                   (+ k-playbook/_dist/)
└── k-playbook/
    ├── K-PLAYBOOK.yaml          committed
    ├── _dist/                   mitgeliefert, gitignored
    ├── tasks/ reviews/ docs/ checks/ enforcement/ guidelines/ commands/
    └── TODO.md
```

`init` ist mehrfach ausfuehrbar und ueberschreibt eine vorhandene `K-PLAYBOOK.yaml` nie.

## Weitere Kommandos

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

## Bestehende Projekte migrieren

```bash
cd /pfad/zum/projekt
k-playbook-installer migrate --dry-run   # zeigt die Aenderungen
k-playbook-installer migrate
```

Die Migration verschiebt `K-PLAYBOOK.yaml` nach `k-playbook/`, kuerzt die `paths.*`-Werte um
das `k-playbook/`-Praefix und hebt `project.repo_root` eine Ebene an. Tasks, Reviews,
Ergebnisse und Docs liegen bereits richtig und werden nicht bewegt. Unbekannte Felder
bleiben erhalten.

## Browser-GUI

```bash
k-playbook-installer gui
```

Die GUI bildet noch das alte zentrale Modell ab. Auf Projekten mit `schema_version: 2`
verweigert sie jede schreibende Aktion mit einem klaren Hinweis, statt die Migration
rueckgaengig zu machen. Fuer migrierte Projekte ist die Kommandozeile der Weg.

## DevContainer

Die DevContainer-Integration setzt noch auf den alten Bind-Mount des Basis-Repos und wird
mit der GUI umgebaut. Im neuen Modell braucht sie ihn nicht mehr: `_dist/` liegt im Projekt
und ist im Container dadurch automatisch vorhanden.

## Dokumentation

- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.
- [`docs/installation.md`](./docs/installation.md) - Installationsdetails, Uebergangsstand und Installer-Binary.
- [`docs/commands.md`](./docs/commands.md) - kompakter Command-Index.
- [`docs/pr-review.md`](./docs/pr-review.md) - PR-Review-Flow.
- [`docs/code-review.md`](./docs/code-review.md) - Review-Rezepte, Results-Summary, Remediation und Handoffs.
- [`docs/task-flow.md`](./docs/task-flow.md) - Task-Erzeugung, Review-Loop und Ausfuehrung.
- [`docs/reviews-and-results.md`](./docs/reviews-and-results.md) - Artefaktmodell fuer Reviews, Findings und Remediation.

## Bausteine

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

## Grundprinzipien

- Jedes Projekt traegt seine eigene Installation in einem Unterverzeichnis. Kein fester Hostpfad, kein globaler Symlink.
- Installation und Projekt-Eigentum sind strikt getrennt: `_dist/` wird bei jedem Update vollstaendig ersetzt, alles daneben nie angefasst.
- Mitgelieferte Regeln, Reviews und Checks werden nicht editiert. Ein Projekt weicht per Overlay ab: gleichnamige lokale Datei ersetzt, `overlay.<kind>.disabled` schaltet ab.
- Pfade werden aus `K-PLAYBOOK.yaml` gelesen und nicht geraten.
- Docs, Tasks, Reviews und Results bleiben projektlokale Artefakte.
- Security-Tools werden host- oder user-lokal installiert, nie in Projekt-venvs. Sie sind die eine bewusste Ausnahme von der Projektlokalitaet.
