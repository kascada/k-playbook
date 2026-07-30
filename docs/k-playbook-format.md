# K-PLAYBOOK.yaml Format

`K-PLAYBOOK.yaml` ist die projektlokale Maschinen-Konfiguration fuer k-playbook.
Sie liegt im Projekt-Root und ersetzt die bisherige Markdown-Konfiguration.

## Grundentscheidung

Die Datei enthaelt keine konfigurierbaren Standardpfade fuer Tasks, Reviews, Checks,
Docs oder Enforcement.

Stattdessen gilt:

- Das Projekt-Root ist das Verzeichnis, in dem `K-PLAYBOOK.yaml` liegt.
- Die projektlokale k-playbook-Struktur liegt fest unter `k-playbook/`.
- Jedes Command kennt seine festen Unterverzeichnisse.
- Die Config speichert Setup-Metadaten, Projekt-Policies und Tool-Entscheidungen.

Damit bleiben Pfade konsistent, Tools koennen die Datei direkt parsen, und Commands
muessen keine alternativen Layouts oder aktive/inaktive Bausteine interpretieren.

## Feste Abgeleitete Pfade

Commands leiten diese Pfade aus dem Projekt-Root ab:

| Zweck | Pfad |
|---|---|
| Playbook-Basis | `k-playbook/` |
| Tasks | `k-playbook/tasks/` |
| erledigte Tasks | `k-playbook/tasks/done/` |
| TODO | `k-playbook/TODO.md` |
| Checks | `k-playbook/checks/` |
| Reviews | `k-playbook/reviews/` |
| Guidelines | `k-playbook/guidelines/` |
| Enforcement-Regeln | `k-playbook/enforcement/` |
| Docs | `k-playbook/docs/` |

Diese Werte gehoeren nicht als `paths:`-Block in `K-PLAYBOOK.yaml`.

## Minimalformat

Dieses Minimalformat ist die kleinste gueltige `K-PLAYBOOK.yaml`. Der Installer
legt genau diese Datei an, wenn ein Projekt neu eingebunden wird und die Datei
noch fehlt.

```yaml
schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: 2026-07-30
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

Dieser Wert bestaetigt, dass Commands die feste `k-playbook/`-Struktur verwenden
und keine Pfadliste aus der Config lesen.

### `k_playbook.repo`

Pflichtfeld. Erwarteter Wert: `~/dev/k-playbook`.

Der Wert dient als sichtbarer Rueckverweis fuer globale Commands, Skills, Regeln,
Reviews, Checks und Skripte. Er ist ein Pfadvertrag, keine Projektoption.

### `setup.updated_at`

Pflichtfeld. ISO-Datum `YYYY-MM-DD`, an dem `/k-setup` die Datei zuletzt
geschrieben oder aktualisiert hat.

### `remediation`

Optionaler Block fuer `/k-remediation`.

| Feld | Typ | Bedeutung |
|---|---|---|
| `mode` | enum | `task-branch-pr`, `task-first` oder `direct-allowed` |
| `target` | string | Code-/Git-Root relativ zum Projekt-Root, z. B. `.` oder `./app` |
| `grouping` | boolean | Findings vor Umsetzung zu sinnvollen Buendeln gruppieren |
| `quick_wins` | boolean | einfache, wirkungsstarke Buendel hervorheben |
| `branch_prefix` | string | empfohlener Prefix fuer Remediation-Branches |
| `pr_required` | boolean | PR als erwarteter Workflow fuer erzeugte Tasks |
| `direct_fixes` | boolean | direkte Code-Fixes ohne Task grundsaetzlich erlaubt |

Wenn der Block fehlt, soll `/k-remediation` nicht raten, sondern `/k-setup` zur
Ergaenzung der Policy empfehlen oder fuer die aktuelle Session explizit fragen.

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

- `/k-setup` besitzt `schema_version`, `layout`, `k_playbook` und `setup`.
- `/k-setup` besitzt ausserdem die Remediation-Policy, sofern sie gesetzt wird.
- `/k-setup-codeql` besitzt nur `tools.codeql`.
- Commands duerfen unbekannte Top-Level-Felder erhalten, aber nicht ungefragt aendern.
- Standardpfade duerfen nicht als konfigurierbarer `paths:`-Block eingefuehrt werden.
- Host-lokale Installationszustaende duerfen nicht in `K-PLAYBOOK.yaml` geschrieben werden.

## Dateiname

Der kanonische Dateiname ist `K-PLAYBOOK.yaml`.

`K-PLAYBOOK.yml` soll nicht neu erzeugt werden, damit Tools und Commands nur einen
kanonischen Namen pruefen muessen.
