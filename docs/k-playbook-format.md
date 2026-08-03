# K-PLAYBOOK.yaml Format

`K-PLAYBOOK.yaml` ist die projektlokale Maschinen-Konfiguration fuer k-playbook.
Sie liegt im k-playbook-Projektordner und ersetzt die bisherige Markdown-Konfiguration.

## Grundentscheidung

Die Datei enthaelt die projektlokalen Pfade fuer Tasks, Reviews, Checks, Docs,
Enforcement und weitere k-playbook-Artefakte.

Stattdessen gilt:

- Das k-playbook-Projektverzeichnis ist das Verzeichnis, in dem `K-PLAYBOOK.yaml` liegt.
- Der eigentliche Code-/Repo-Root steht in `project.repo_root`, relativ zum
  k-playbook-Projektverzeichnis.
- Projektlokale Artefaktpfade stehen unter `paths.*` und sind ebenfalls relativ
  zum k-playbook-Projektverzeichnis.
- Commands duerfen Pfade nicht raten. Fehlt ein benoetigter `paths.*`-Eintrag,
  muss der Command nachfragen und den bestaetigten Wert in `K-PLAYBOOK.yaml`
  ergaenzen.
- Die Config speichert ausserdem Setup-Metadaten, Projekt-Policies und Tool-Entscheidungen.

Damit bleiben Pfade konsistent und explizit, Tools koennen die Datei direkt parsen,
und Commands muessen keine Layouts aus dem Dateisystem erraten.

## Projektlokale Pfade

Commands lesen diese Pfade aus `K-PLAYBOOK.yaml`:

| Zweck | YAML-Key | Konventioneller Wert |
|---|---|---|
| Playbook-Basis | `paths.playbook` | `k-playbook` |
| Tasks | `paths.tasks` | `k-playbook/tasks` |
| erledigte Tasks | `paths.completed_tasks` | `k-playbook/tasks/done` |
| TODO | `paths.todo` | `k-playbook/TODO.md` |
| Checks | `paths.checks` | `k-playbook/checks` |
| Reviews | `paths.reviews` | `k-playbook/reviews` |
| Guidelines | `paths.guidelines` | `k-playbook/guidelines` |
| Enforcement-Regeln | `paths.enforcement` | `k-playbook/enforcement` |
| Docs | `paths.docs` | `k-playbook/docs` |

Alle Werte muessen relativ sein, duerfen nicht mit `/` beginnen und duerfen nicht
aus dem Projektverzeichnis herausfuehren. Die konventionellen Werte sind die
empfohlenen Defaults fuer GUI und Reparaturfragen, aber keine stillen Fallbacks.

## Minimalformat

Dieses Minimalformat ist die kleinste gueltige `K-PLAYBOOK.yaml`. Der Installer
legt genau diese Datei an, wenn ein Projekt neu eingebunden wird und die Datei
noch fehlt.

```yaml
schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks
  completed_tasks: k-playbook/tasks/done
  todo: k-playbook/TODO.md
  checks: k-playbook/checks
  reviews: k-playbook/reviews
  guidelines: k-playbook/guidelines
  enforcement: k-playbook/enforcement
  docs: k-playbook/docs

project:
  repo_root: .
  vcs: git

setup:
  updated_at: 2026-07-30

remediation:
  mode: direct-allowed
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: false
  direct_fixes: true
```

`k_playbook.repo` ist der feste logische Rueckverweis auf das globale Basis-Repo.
Der Wert ist nicht projektweise frei waehlbar. Wenn der physische Klon woanders
liegt, muss `~/dev/k-playbook` ein Symlink auf den echten Klon sein.

## Vollstaendiges Beispiel

```yaml
schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks
  completed_tasks: k-playbook/tasks/done
  todo: k-playbook/TODO.md
  checks: k-playbook/checks
  reviews: k-playbook/reviews
  guidelines: k-playbook/guidelines
  enforcement: k-playbook/enforcement
  docs: k-playbook/docs

project:
  repo_root: ./app
  vcs: git

setup:
  updated_at: 2026-07-30

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

Pflichtfeld. Aktuelle Version: `1`.

### `layout`

Pflichtfeld. Aktueller Wert: `fixed-project-k-playbook`.

Dieser Wert bestaetigt das k-playbook-Projektmodell. Projektlokale Pfade stehen
trotzdem explizit unter `paths.*`; alte Dateien mit diesem Layout ohne `paths.*`
muessen beim naechsten GUI-/Command-Lauf um die benoetigten Keys ergaenzt werden.

### `k_playbook.repo`

Pflichtfeld. Erwarteter Wert: `~/dev/k-playbook`.

Der Wert dient als sichtbarer Rueckverweis fuer globale Commands, Skills, Regeln,
Reviews, Checks und Skripte. Er ist ein Pfadvertrag, keine Projektoption.

### `paths`

Pflichtblock fuer projektlokale k-playbook-Artefakte. Commands verwenden nur die
jeweils benoetigten Keys, duerfen fehlende Keys aber nicht selbst erraten.

| Feld | Typ | Bedeutung |
|---|---|---|
| `playbook` | string | Basisverzeichnis fuer projektlokale k-playbook-Artefakte |
| `tasks` | string | Task-Dateien fuer `/k-task-create`, `/k-run`, `/k-review-loop` |
| `completed_tasks` | string | Ablage fuer erledigte Tasks |
| `todo` | string | Projekt-TODO-Datei |
| `checks` | string | Projektlokale Checks |
| `reviews` | string | Projektlokale Review-Rezepte, Logs, Decisions und Results |
| `guidelines` | string | Projektlokale Guidelines |
| `enforcement` | string | Projektlokale Enforcement-Regeln |
| `docs` | string | Projektlokale Docs fuer Docs-First-AI-Sessions |

Wenn ein Command einen benoetigten Key nicht findet, muss er den Nutzer nach dem
projektrelativen Pfad fragen, den Wert validieren und `K-PLAYBOOK.yaml` ergaenzen.
Er darf nicht still `k-playbook/...` verwenden, nur weil dieses Verzeichnis existiert.

### `project.repo_root`

Pflichtfeld. Relativer Pfad vom Verzeichnis der `K-PLAYBOOK.yaml` zum tatsaechlichen
Code-/Repo-Root.

Typische Werte:

- `.` fuer normale Projekte, bei denen `K-PLAYBOOK.yaml` im Git-/Code-Root liegt.
- `./app` fuer Wrapper-/DevContainer-Projekte, bei denen der eigentliche Code in
  einem Unterverzeichnis liegt.

Commands duerfen diesen Pfad aus der YAML lesen und validieren, aber nicht selbst
Git-Roots suchen oder raten. Wenn `project.repo_root` leer, ungueltig oder fehlend
ist, muss `/k-status` einen Fehler melden und die GUI zur Korrektur empfehlen.

### `project.vcs`

Pflichtfeld. Aktuelle Werte:

- `git` fuer Projekte mit Git-Worktree im `project.repo_root`.
- `none` fuer Projekte ohne Git. Das ist eine explizite Projektentscheidung und
  wird in der YAML gespeichert, statt in Commands geraten zu werden.

### `setup.updated_at`

Pflichtfeld. ISO-Datum `YYYY-MM-DD`, an dem `/k-setup` die Datei zuletzt
geschrieben oder aktualisiert hat.

### `remediation`

Optionaler Block fuer `/k-remediation`.

Der Installer legt den Block bei neuen Projekteinbindungen standardmaessig mit
`mode: direct-allowed` an. Das ist fuer kleine, sichere Sofort-Fixes pragmatisch;
groessere Aenderungen bleiben Tasks. Fuer strengere Team-/PR-Prozesse kann der
Modus beim Einbinden auf `task-first` oder `task-branch-pr` gestellt werden.

| Feld | Typ | Bedeutung |
|---|---|---|
| `mode` | enum | `task-branch-pr`, `task-first` oder `direct-allowed` |
| `target` | string | optionaler Remediation-Override relativ zum Projektverzeichnis; Default ist `project.repo_root` |
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

- `/k-setup` besitzt `schema_version`, `layout`, `k_playbook`, `project` und `setup`.
- `/k-setup` bzw. die Installer-GUI besitzt ausserdem den `paths`-Block.
- `/k-setup` besitzt ausserdem die Remediation-Policy, sofern sie gesetzt wird.
- Die Installer-GUI besitzt `project.repo_root` und `project.vcs`. Sie darf
  Git-Kandidaten suchen oder den Nutzer fragen; andere Commands duerfen das nicht.
- `/k-setup-codeql` besitzt nur `tools.codeql`.
- Commands duerfen unbekannte Top-Level-Felder erhalten, aber nicht ungefragt aendern.
- Commands duerfen benoetigte fehlende `paths.*`-Keys nach Rueckfrage ergaenzen.
- Commands duerfen projektlokale Pfade nicht aus dem Dateisystem oder historischen Defaults raten.
- Host-lokale Installationszustaende duerfen nicht in `K-PLAYBOOK.yaml` geschrieben werden.

## Dateiname

Der kanonische Dateiname ist `K-PLAYBOOK.yaml`.

`K-PLAYBOOK.yml` soll nicht neu erzeugt werden, damit Tools und Commands nur einen
kanonischen Namen pruefen muessen.
