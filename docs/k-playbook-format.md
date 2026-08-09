# K-PLAYBOOK.yaml Format

`K-PLAYBOOK.yaml` ist die projektlokale Maschinen-Konfiguration fuer k-playbook.
Sie liegt im k-playbook-Verzeichnis des Projekts, konventionell `<projekt>/k-playbook/`.

## Grundentscheidung

k-playbook wird in ein Unterverzeichnis des Zielprojekts installiert. Es gibt keine
zentrale Basisinstallation und keinen festen Hostpfad mehr. Ein Projekt ist damit
selbstgenuegsam: Commands, Regeln, Reviews und Checks liegen im Projekt selbst.

Innerhalb des k-playbook-Verzeichnisses gilt eine harte Trennung:

- **Installation** liegt ausschliesslich unter `_dist/`. Sie wird mitgeliefert, ist
  read-only und wird bei jedem Update **vollstaendig ersetzt**. Nichts darin darf
  von Hand editiert werden.
- **Projekt-Eigentum** ist alles andere: `K-PLAYBOOK.yaml`, Tasks, Reviews,
  Ergebnisse, Docs, eigene Regeln, eigene Checks, eigene Commands. Ein Update
  fasst diese Dateien nie an.

Daraus folgt:

- Das k-playbook-Verzeichnis ist das Verzeichnis, in dem `K-PLAYBOOK.yaml` liegt.
- Projektlokale Artefaktpfade stehen unter `paths.*`, relativ zu diesem Verzeichnis.
- Der eigentliche Code-/Repo-Root steht in `project.repo_root`, ebenfalls relativ
  zu diesem Verzeichnis, und liegt normalerweise darueber (`..`).
- Commands duerfen Pfade nicht raten. Fehlt ein benoetigter `paths.*`-Eintrag,
  muss der Command nachfragen und den bestaetigten Wert in `K-PLAYBOOK.yaml`
  ergaenzen.
- Die Config speichert ausserdem Setup-Metadaten, Projekt-Policies, die
  Overlay-Entscheidungen und Tool-Entscheidungen.

## Verzeichnislayout

```text
mein-projekt/
├── .claude/
│   ├── commands -> ../k-playbook/_dist/commands
│   └── skills   -> ../k-playbook/_dist/skills
├── .gitignore                     enthaelt: k-playbook/_dist/
├── k-playbook/
│   ├── K-PLAYBOOK.yaml            Projekt
│   ├── _dist/                     Installation, gitignored, read-only
│   │   ├── VERSION
│   │   ├── commands/
│   │   ├── skills/
│   │   ├── rules/
│   │   ├── reviews/
│   │   ├── checks/
│   │   ├── scripts/
│   │   ├── security-tools.tsv
│   │   └── bin/k-check
│   ├── commands/                  Projekt: eigene Commands
│   ├── tasks/                     Projekt
│   │   └── done/
│   ├── reviews/                   Projekt: Logs, Decisions, Results, eigene Rezepte
│   ├── checks/                    Projekt: eigene Checks
│   ├── enforcement/               Projekt: eigene Regeln, Overrides
│   ├── guidelines/                Projekt
│   ├── docs/                      Projekt
│   └── TODO.md                    Projekt
└── src/
```

`_dist/` steht in der `.gitignore` des Zielprojekts. Nach einem `git clone` fehlt es
und wird mit `k-playbook-installer restore` aus der in `k_playbook.version`
gespeicherten Version wiederhergestellt.

## Projektlokale Pfade

Commands lesen diese Pfade aus `K-PLAYBOOK.yaml`:

| Zweck | YAML-Key | Konventioneller Wert |
|---|---|---|
| Tasks | `paths.tasks` | `tasks` |
| erledigte Tasks | `paths.completed_tasks` | `tasks/done` |
| TODO | `paths.todo` | `TODO.md` |
| Checks | `paths.checks` | `checks` |
| Reviews | `paths.reviews` | `reviews` |
| Guidelines | `paths.guidelines` | `guidelines` |
| Enforcement-Regeln | `paths.enforcement` | `enforcement` |
| Docs | `paths.docs` | `docs` |
| eigene Commands | `paths.commands` | `commands` |

Alle Werte muessen relativ sein und duerfen nicht mit `/` beginnen. Die
konventionellen Werte sind die empfohlenen Defaults fuer GUI und Reparaturfragen,
aber keine stillen Fallbacks.

Zwei Grenzen gelten:

- Ein Wert darf das k-playbook-Verzeichnis mit `../` verlassen, muss aber innerhalb
  von `project.repo_root` bleiben. Das erlaubt es, ein bereits etabliertes
  Projektverzeichnis weiterzuverwenden, statt es umziehen zu muessen — typisch fuer
  `paths.docs`, wenn ein Projekt seine Doku schon woanders pflegt.
- Ein Wert darf nie auf `k_playbook.dist` oder ein Unterverzeichnis davon zeigen.
  Installation und Projekt-Eigentum duerfen sich nicht ueberlappen, sonst wuerde ein
  Update Projektdateien loeschen.

Ein Wert, der aus dem Projekt herausfuehrt, ist ein Fehler und keine Option. Commands
schreiben ausschliesslich innerhalb des Projekts.

## Minimalformat

Dieses Minimalformat ist die kleinste gueltige `K-PLAYBOOK.yaml`. Der Installer
legt genau diese Datei an, wenn ein Projekt neu eingebunden wird.

```yaml
schema_version: 2
layout: project-local

k_playbook:
  dist: _dist
  version: 0.4.0
  installed_at: 2026-08-08

paths:
  tasks: tasks
  completed_tasks: tasks/done
  todo: TODO.md
  checks: checks
  reviews: reviews
  guidelines: guidelines
  enforcement: enforcement
  docs: docs
  commands: commands

project:
  repo_root: ..
  vcs: git

overlay:
  rules:
    disabled: []
  reviews:
    disabled: []
  checks:
    disabled: []

setup:
  updated_at: 2026-08-08

remediation:
  mode: direct-allowed
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: false
  direct_fixes: true
```

## Vollstaendiges Beispiel

```yaml
schema_version: 2
layout: project-local

k_playbook:
  dist: _dist
  version: 0.4.0
  installed_at: 2026-08-08

paths:
  tasks: tasks
  completed_tasks: tasks/done
  todo: TODO.md
  checks: checks
  reviews: reviews
  guidelines: guidelines
  enforcement: enforcement
  docs: docs
  commands: commands

project:
  repo_root: ../app
  vcs: git

overlay:
  rules:
    disabled:
      - tool-install-scope
  reviews:
    disabled: []
  checks:
    disabled:
      - check_django_baseline

setup:
  updated_at: 2026-08-08

remediation:
  mode: task-branch-pr
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false

tools:
  codeql:
    target: .
    languages:
      - python
      - javascript-typescript
    queries: security-extended
    github:
      status: enabled
      workflow: ./.github/workflows/codeql.yml
    local_database:
      status: disabled
      path: null
```

## Felder

### `schema_version`

Pflichtfeld. Aktuelle Version: `2`.

Version `1` beschreibt das alte Modell mit zentraler Basisinstallation unter
`~/dev/k-playbook`. Dateien mit `schema_version: 1` muessen mit
`k-playbook-installer migrate` umgestellt werden; siehe [Migration](#migration-von-schema_version-1).

### `layout`

Pflichtfeld. Aktueller Wert: `project-local`.

Der Wert bestaetigt, dass k-playbook in einem Unterverzeichnis des Projekts
installiert ist. Der Vorgaengerwert `fixed-project-k-playbook` gehoert zu
`schema_version: 1` und ist nicht mehr gueltig.

### `k_playbook`

Pflichtblock. Beschreibt die Installation.

| Feld | Typ | Bedeutung |
|---|---|---|
| `dist` | string | Verzeichnisname der Installation, relativ zum k-playbook-Verzeichnis. Konventionell `_dist`. |
| `version` | string | Version des installierten Werkzeugs. Quelle fuer `restore` nach einem `git clone`. |
| `installed_at` | string | ISO-Datum `YYYY-MM-DD` des letzten `init`/`update`/`restore`. |

`dist` ist konfigurierbar, damit ein Projekt bei einem Namenskonflikt ausweichen
kann. Der Wert muss ein einzelnes Verzeichnissegment sein, darf nicht `.` oder `..`
enthalten und darf mit keinem `paths.*`-Wert kollidieren.

`version` ist Projekt-Eigentum und wird committet. Sie ist die einzige Information,
die nach `git clone` noch verfuegbar ist, um die passende Installation
wiederherzustellen.

### `paths`

Pflichtblock fuer projektlokale k-playbook-Artefakte. Commands verwenden nur die
jeweils benoetigten Keys, duerfen fehlende Keys aber nicht selbst erraten.

| Feld | Typ | Bedeutung |
|---|---|---|
| `tasks` | string | Task-Dateien fuer `/k-task-create`, `/k-run`, `/k-review-loop` |
| `completed_tasks` | string | Ablage fuer erledigte Tasks |
| `todo` | string | Projekt-TODO-Datei |
| `checks` | string | Projektlokale Checks; Overlay ueber `_dist/checks` |
| `reviews` | string | Projektlokale Review-Rezepte, Logs, Decisions und Results; Overlay ueber `_dist/reviews` |
| `guidelines` | string | Projektlokale Guidelines |
| `enforcement` | string | Projektlokale Enforcement-Regeln; Overlay ueber `_dist/rules` |
| `docs` | string | Projektlokale Docs fuer Docs-First-AI-Sessions. Haeufigster Fall fuer einen `../`-Wert, wenn das Projekt seine Doku bereits an anderer Stelle pflegt |
| `commands` | string | Projekteigene Slash-Commands; gewinnen bei Namensgleichheit gegen `_dist/commands` |

Wenn ein Command einen benoetigten Key nicht findet, muss er den Nutzer nach dem
Pfad relativ zum k-playbook-Verzeichnis fragen, den Wert validieren und
`K-PLAYBOOK.yaml` ergaenzen. Er darf keinen Wert aus dem Dateisystem raten.

Ein Beispiel fuer einen Wert ausserhalb des k-playbook-Verzeichnisses:

```yaml
paths:
  tasks: tasks       # <projekt>/k-playbook/tasks
  docs: ../docs      # <projekt>/docs  — bestehende Projektdoku
```

### `project.repo_root`

Pflichtfeld. Relativer Pfad vom k-playbook-Verzeichnis zum tatsaechlichen
Code-/Repo-Root.

Typische Werte:

- `..` fuer normale Projekte, bei denen `k-playbook/` direkt im Git-/Code-Root liegt.
- `../app` fuer Wrapper-/DevContainer-Projekte, bei denen der eigentliche Code in
  einem Unterverzeichnis liegt.

Dies ist der einzige Pfad, der das k-playbook-Verzeichnis verlassen darf und muss.
Er muss innerhalb des Git-Worktrees bleiben.

Commands duerfen diesen Pfad aus der YAML lesen und validieren, aber nicht selbst
Git-Roots suchen oder raten. Wenn `project.repo_root` leer, ungueltig oder fehlend
ist, muss `/k-status` einen Fehler melden und die GUI zur Korrektur empfehlen.

### `project.vcs`

Pflichtfeld. Aktuelle Werte:

- `git` fuer Projekte mit Git-Worktree im `project.repo_root`.
- `none` fuer Projekte ohne Git. Das ist eine explizite Projektentscheidung und
  wird in der YAML gespeichert, statt in Commands geraten zu werden.

### `overlay`

Pflichtblock. Steuert, welche mitgelieferten Kataloge aktiv sind.

`_dist/rules`, `_dist/reviews` und `_dist/checks` gelten grundsaetzlich. Ein Projekt
kann davon auf zwei Wegen abweichen:

1. **Ueberlagern**: Eine gleichnamige Datei im projektlokalen Verzeichnis ersetzt den
   mitgelieferten Eintrag vollstaendig.
2. **Abschalten**: Ein Eintrag in `overlay.<kind>.disabled` deaktiviert den
   mitgelieferten Eintrag ersatzlos.

| Feld | Typ | Basisverzeichnis | Projektverzeichnis |
|---|---|---|---|
| `overlay.rules.disabled` | list[string] | `_dist/rules` | `paths.enforcement` |
| `overlay.reviews.disabled` | list[string] | `_dist/reviews` | `paths.reviews` |
| `overlay.checks.disabled` | list[string] | `_dist/checks` | `paths.checks` |

Die Listeneintraege sind Schluessel, keine Dateinamen: der Basisname ohne Endung,
bei Reviews zusaetzlich ohne `review-`-Praefix. Also `tool-install-scope`, nicht
`tool-install-scope.md`; `codeql-security`, nicht `review-codeql-security.md`.

Ein Eintrag in `disabled`, der in `_dist` nicht existiert, ist kein Fehler, muss aber
vom Command als veraltet gemeldet werden. Die Aufloesungsregel im Detail steht in
`_dist/commands/_shared/overlay-resolution.md`.

Projektlokale Dateien werden von `disabled` nicht betroffen. Wer eine eigene Regel
nicht laden will, loescht sie.

### `setup.updated_at`

Pflichtfeld. ISO-Datum `YYYY-MM-DD`, an dem die Installer-GUI oder ein dafuer
zustaendiger Command die Datei zuletzt geschrieben oder aktualisiert hat.

### `remediation`

Optionaler Block fuer `/k-remediation`.

Der Installer legt den Block bei neuen Projekteinbindungen standardmaessig mit
`mode: direct-allowed` an. Das ist fuer kleine, sichere Sofort-Fixes pragmatisch;
groessere Aenderungen bleiben Tasks. Fuer strengere Team-/PR-Prozesse kann der
Modus beim Einbinden auf `task-first` oder `task-branch-pr` gestellt werden.

| Feld | Typ | Bedeutung |
|---|---|---|
| `mode` | enum | `task-branch-pr`, `task-first` oder `direct-allowed` |
| `target` | string | optionaler Remediation-Override relativ zum k-playbook-Verzeichnis; Default ist `project.repo_root` |
| `grouping` | boolean | Findings vor Umsetzung zu sinnvollen Buendeln gruppieren |
| `quick_wins` | boolean | einfache, wirkungsstarke Buendel hervorheben |
| `branch_prefix` | string | empfohlener Prefix fuer Remediation-Branches |
| `pr_required` | boolean | PR als erwarteter Workflow fuer erzeugte Tasks |
| `direct_fixes` | boolean | direkte Code-Fixes ohne Task grundsaetzlich erlaubt |

Wenn der Block fehlt, soll `/k-remediation` nicht raten, sondern den Installer
zur Ergaenzung der Policy empfehlen oder fuer die aktuelle Session explizit fragen.

### `tools`

Optionaler Block fuer projektlokale Tool-Entscheidungen.

Wichtig: Dieser Block speichert Projektentscheidungen, keine transienten Host-Fakten.
Ob `codeql`, `gitleaks`, `trivy` oder andere Tools auf dem aktuellen Host wirklich
installiert sind, gehoert in `/k-status` oder einen Preflight-Bericht, nicht in die
versionierte Projekt-Konfiguration.

#### `tools.codeql`

| Feld | Typ | Bedeutung |
|---|---|---|
| `target` | string | CodeQL-Analyse-Root relativ zum Projekt-Root |
| `languages` | list[string] | registrierte CodeQL-Sprachen |
| `queries` | string | Query-Suite oder Query-Pack |
| `github.status` | enum | `enabled`, `disabled` oder `planned` |
| `github.workflow` | string/null | projektrelativer Workflow-Pfad oder `null` |
| `local_database.status` | enum | `enabled`, `disabled` oder `planned` |
| `local_database.path` | string/null | projektrelativer Datenbankpfad oder `null` |

`enabled` wird nicht als eigenes redundantes Feld gespeichert. Ein Tool gilt fuer
ein Projekt als aktiv oder geplant, wenn mindestens ein relevanter Status `enabled`
oder `planned` ist.

## Schreibregeln

- Der Installer besitzt `schema_version`, `layout`, `k_playbook`, `project`, `setup` und `paths`.
- Der Installer besitzt `project.repo_root` und `project.vcs`. Er darf Git-Kandidaten
  suchen oder den Nutzer fragen; andere Commands duerfen das nicht.
- Der Installer besitzt die Remediation-Policy beim Einbinden; spaeter darf
  `/k-remediation` sie nach Rueckfrage aendern.
- `/k-setup-codeql` besitzt nur `tools.codeql`.
- `overlay.*.disabled` gehoert dem Nutzer. Commands duerfen Eintraege vorschlagen und
  nach ausdruecklicher Bestaetigung schreiben, aber nie still.
- Commands duerfen unbekannte Top-Level-Felder erhalten, aber nicht ungefragt aendern.
- Commands duerfen benoetigte fehlende `paths.*`-Keys nach Rueckfrage ergaenzen.
- Commands duerfen projektlokale Pfade nicht aus dem Dateisystem oder historischen Defaults raten.
- Host-lokale Installationszustaende duerfen nicht in `K-PLAYBOOK.yaml` geschrieben werden.
- Nichts unterhalb von `k_playbook.dist` darf geschrieben werden. Das gilt auch fuer
  Commands, die dort Regeln oder Rezepte lesen.

## Migration von `schema_version: 1`

Ausgefuehrt durch `k-playbook-installer migrate <pfad>`:

1. `<root>/K-PLAYBOOK.yaml` nach `<root>/k-playbook/K-PLAYBOOK.yaml` verschieben.
2. In allen `paths.*`-Werten das Praefix `k-playbook/` entfernen.
3. `paths.playbook` loeschen. Das Verzeichnis der YAML ist die Playbook-Basis.
4. `paths.commands` mit dem Default `commands` ergaenzen.
5. `project.repo_root: .` nach `..` aendern; andere Werte `<w>` nach `../<w>`.
6. `k_playbook.repo` entfernen; `k_playbook.dist`, `version` und `installed_at` setzen.
7. `layout` auf `project-local`, `schema_version` auf `2` setzen.
8. Leeren `overlay`-Block ergaenzen.
9. `.gitignore` des Projekts um `k-playbook/_dist/` ergaenzen.

Unbekannte Top-Level-Felder bleiben unveraendert erhalten.

Die Migration ist rein mechanisch und aendert keine Projektinhalte. Tasks, Reviews,
Ergebnisse und Docs liegen bereits unter `k-playbook/` und bleiben, wo sie sind.

## Dateiname

Der kanonische Dateiname ist `K-PLAYBOOK.yaml`.

`K-PLAYBOOK.yml` soll nicht neu erzeugt werden, damit Tools und Commands nur einen
kanonischen Namen pruefen muessen.
