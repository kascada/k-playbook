# K-PLAYBOOK.yaml Format

`K-PLAYBOOK.yaml` ist die Konfiguration eines Projekts, das k-playbook nutzt. Sie ist
zugleich der **Anker**: ihr Ort bestimmt, was das Hauptverzeichnis des Projekts ist.

## Grundentscheidung

Jedes Projekt traegt seine eigene Installation. Es gibt keine zentrale Basisinstallation
und keinen festen Hostpfad. Die Installation liegt neben der Konfiguration:

```text
<projekt>/                 beliebig benannt
├── K-PLAYBOOK.yaml        der Anker
├── k-playbook/            die Installation, vollstaendig ersetzbar
└── k-playbook-local/      projekteigen, committed
```

Weil die Konfiguration **neben** und nicht **in** der Installation liegt, enthaelt
`k-playbook/` nichts Projekteigenes. Das Verzeichnis ist dadurch komplett updatebar —
per `git pull` ebenso wie per `rm -rf` und neuem Clone.

Das Playbook-Verzeichnis heisst immer `k-playbook`. Wie das Projektverzeichnis darueber
heisst, spielt keine Rolle.

## Keine Pfade in der Konfiguration

Fruehere Versionen trugen einen `paths:`-Block mit neun Schluesseln. Den gibt es nicht
mehr. Alle Orte ergeben sich aus dem Ort der `K-PLAYBOOK.yaml`:

| Was | Wo |
|---|---|
| mitgelieferte Commands | `k-playbook/commands/` |
| mitgelieferte Skills | `k-playbook/skills/` |
| mitgelieferte Regeln | `k-playbook/rules/` |
| mitgelieferte Review-Rezepte | `k-playbook/reviews/` |
| mitgelieferte Checks | `k-playbook/checks/` |
| Check-Runner | `k-playbook/bin/k-check` |
| Skripte | `k-playbook/scripts/` |
| Security-Tool-Matrix | `k-playbook/scripts/security-tools.tsv` |
| projekteigene Regeln | `k-playbook-local/rules/` |
| projekteigene Review-Rezepte | `k-playbook-local/reviews/` |
| projekteigene Checks | `k-playbook-local/checks/` |
| Review-Ergebnisse | `k-playbook-local/results/` |
| Projekt-Dokumentation | `k-playbook-local/docs/` |
| Tool-Steckbriefe | `k-playbook-local/docs/libs/` |
| Guidelines | `k-playbook-local/guidelines/` |
| offene Tasks | `k-playbook-local/tasks/` |
| erledigte Tasks | `k-playbook-local/tasks/done/` |
| Projekt-TODO | `k-playbook-local/TODO.md` |
| Privates | `k-playbook-local/priv/` |
| Instruktionen, mitgeliefert | `k-playbook/k-playbook.md` |
| Instruktionen, projekteigen | `k-playbook-local/k-playbook.md` |

Ein Schluessel, dessen Wert immer derselbe ist, waere nur eine Fehlerquelle gewesen.
Commands raten damit keinen Pfad mehr und lesen auch keinen: sie leiten ihn ab.

Das gilt auch fuer die Projekt-Dokumentation. Frueher durfte `paths.docs` als einziger
Wert mit `../` aus dem k-playbook-Verzeichnis herauszeigen, damit ein Projekt seine schon
vorhandene Doku weiterverwenden konnte. Dieser Sonderfall entfaellt: `/k-code2docs`
schreibt nach `k-playbook-local/docs/`. Was ein Projekt sonst noch an Dokumentation
pflegt, bleibt davon unberuehrt — k-playbook beansprucht nur sein eigenes Verzeichnis.

## Anker finden

Der Ablauf gilt gleichermassen fuer das Werkzeug und fuer einen Assistenten:

1. Wurde ein Verzeichnis uebergeben, gilt dieses; geprueft wird `<arg>/K-PLAYBOOK.yaml`.
2. Sonst ab `realpath(CWD)` aufwaerts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
3. Fund: `PROJECT_DIR = <dir>`, `PLAYBOOK_DIR = <dir>/k-playbook`.
4. Grenze der Aufwaertssuche sind `$HOME` und `/`, jeweils einschliesslich.
5. Nichts gefunden: melden, dass keine Installation vorliegt. Nicht raten, nichts anlegen.

Die Aufwaertssuche darf **nicht** am Git-Worktree-Root abbrechen. `<projekt>/k-playbook/`
ist ein eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, kaeme sonst
nie an die Konfiguration eine Ebene darueber.

## Mitgeliefertes und Projekteigenes zusammenfassen

Drei Verzeichnisse existieren doppelt. Was gilt, ist die Vereinigung beider Seiten:

| Sorte | mitgeliefert | projekteigen | Dateimuster |
|---|---|---|---|
| Regeln | `k-playbook/rules/` | `k-playbook-local/rules/` | `*.md` |
| Review-Rezepte | `k-playbook/reviews/` | `k-playbook-local/reviews/` | `review-*.md` |
| Checks | `k-playbook/checks/` | `k-playbook-local/checks/` | `*.sh`, nur oberste Ebene |

Die Vergleichseinheit ist der **Dateiname**. Beide Seiten benutzen dieselbe
Namenskonvention, deshalb braucht es keinen abgeleiteten Schluessel.

**Bei gleichem Dateinamen gewinnt die projekteigene Datei, und zwar vollstaendig.** Die
mitgelieferte wird dann gar nicht erst gelesen; es werden auch keine einzelnen Abschnitte
daraus uebernommen. Wer eine mitgelieferte Regel aendern will, kopiert sie und aendert die
Kopie — mit dem Preis, dass spaetere Verbesserungen am Original diese Kopie nicht mehr
erreichen. Der Vorteil wiegt schwerer: was gilt, steht in genau einer Datei.

**Abgeschaltet wird ueber eine leere Datei**, nicht ueber eine Liste in der
Konfiguration. Da eine gleichnamige lokale Datei die mitgelieferte vollstaendig ersetzt,
bleibt bei einer leeren nichts uebrig. „Leer" heisst: nichts ausser Leerzeilen und
Kommentaren — so kann die Datei ihren eigenen Grund tragen:

```bash
# Abgeschaltet: dieses Projekt nutzt kein Django.
```

Der Unterschied zwischen den Sorten ist beabsichtigt. `rules` und `reviews` werden
gelesen; dort bleibt der Eintrag im Katalog sichtbar und sein Inhalt sagt, dass er
abgeschaltet ist. Ein Check wird dagegen **ausgefuehrt** — ein leeres Skript liefe mit
Exit 0 durch und saehe aus wie ein bestandener Check. Deshalb faellt er ganz aus dem
Katalog.

Eine projekteigene Datei schaltet man ab, indem man sie loescht.

`README.md` in einem der Verzeichnisse ist nie ein Eintrag, ebensowenig Dotfiles oder
irgendetwas unter `checks/lib/`.

Wer wissen will, was am Ende gilt, fragt nicht das Dateisystem, sondern das Werkzeug:

```bash
k-playbook/bin/k-playbook context
```

Die Ausgabe fuehrt die zusammengefuehrten Kataloge mit Herkunft je Eintrag — `dist`,
`local` oder `override` — und markiert Abgeschaltetes. Siehe [Der aufgeloeste
Arbeitsstand](#der-aufgeloeste-arbeitsstand).

**Commands und Skills gibt es nur mitgeliefert.** Es gibt kein
`k-playbook-local/commands/` und kein `k-playbook-local/skills/`. Deshalb kann die
Verlinkung fuer die Assistenten ein einzelner Verzeichnis-Symlink bleiben.

Nichts unterhalb von `k-playbook/` darf geschrieben werden — auch nicht von Commands, die
dort Regeln oder Rezepte lesen. Ein Update ersetzt das Verzeichnis vollstaendig.

## Der aufgeloeste Arbeitsstand

```bash
k-playbook/bin/k-playbook context
```

Gibt als JSON aus, was ein Command sonst selbst aus Konfiguration und Dateisystem
zusammenrechnen muesste:

| Feld | Inhalt |
|---|---|
| `schemaVersion` | die gepruefte Fassung der Konfiguration |
| `instructions` | die Instruktionsdateien in Lesereihenfolge |
| `project` | Hauptverzeichnis, `repoRoot`, `vcs`, Ort der Konfiguration |
| `playbook`, `local` | die beiden aufgeloesten Verzeichnisse |
| `remediation` | die Policy, mit Default, falls der Block fehlt |
| `catalogs` | `rules`, `reviews`, `checks` — zusammengefuehrt |
| `guidelines` | die Dateien aus `k-playbook-local/guidelines/` |

Jeder Katalogeintrag traegt `name` (den Dateinamen), `key` (den Aufrufnamen ohne Endung
und Sortenpraefix), `path`, `origin` — `dist`, `local` oder `override` — und `disabled`,
wo zutreffend.

Damit muss kein Command die Overlay-Regeln selbst anwenden. Es gibt eine Antwort, und
alle bekommen dieselbe.

Der Aufruf ist bewusst billig: der Security-Tool-Preflight fehlt darin, weil er je Tool
ein `--version` startet und spuerbar dauert. `context` soll am Anfang jedes Commands
stehen koennen.

Gesucht wird ab dem Arbeitsverzeichnis aufwaerts. Ohne `K-PLAYBOOK.yaml` bricht der
Aufruf mit einer Meldung ab, ebenso bei einer `schema_version`, die nicht `3` ist.

## Instruktionen

Was ein Assistent vor der Arbeit lesen soll, steht in `k-playbook.md` — je einmal pro
Ebene:

| Datei | Gilt fuer | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge; die projekteigene Ebene kann die mitgelieferte
ergaenzen oder ueberstimmen. `context` nennt unter `instructions` nur die Dateien, die
tatsaechlich existieren — ein Pfad ins Leere waere schlechter als keiner.

Die Datei heisst bewusst nicht `AGENTS.md`: diesen Namen lesen die Assistenten von sich
aus, und er ist dem Hauptverzeichnis vorbehalten.

`AGENTS.md` bekommt nur einen **Anstoss**: einen kurzen Block, der auf
`k-playbook context` verweist. Fehlt die Datei, wird sie angelegt; ist sie da, wird der
Block angehaengt und vorhandener Inhalt nicht angetastet. Ein Marker
`<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf ihn erneut anhaengt.

## Minimalformat

Das legt das Werkzeug an, wenn ein Projekt neu eingebunden wird:

```yaml
# k-playbook
#
# Der Ort dieser Datei bestimmt das Hauptverzeichnis des Projekts.
# Die Installation liegt daneben unter k-playbook/ und ist vollstaendig
# ersetzbar; projekteigene Dateien gehoeren nicht hinein.

schema_version: 3

project:
  # Ort des Projekt-Repositorys, relativ zu dieser Datei.
  repo_root: .
  vcs: git

remediation:
  # Wie Befunde aus Reviews abgearbeitet werden.
  mode: task-first
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  # Aus dem Modus abgeleitet; Commands lesen sie direkt.
  pr_required: false
  direct_fixes: true
```

## Vollstaendiges Beispiel

```yaml
schema_version: 3

project:
  repo_root: app
  vcs: git

remediation:
  mode: task-branch-pr
  target: app
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false

tools:
  codeql:
    target: app
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

Pflichtfeld. Aktuelle Version: `3`.

`3` beschreibt das hier dokumentierte Modell: Anker im Hauptverzeichnis,
`k-playbook/` und `k-playbook-local/` daneben, keine Pfade in der Konfiguration.

Aeltere Werte gehoeren zu abgeloesten Modellen und werden nicht mehr unterstuetzt:

| Wert | Modell |
|---|---|
| `1` | zentrale Basisinstallation unter `~/dev/k-playbook` |
| `2` | Anker im k-playbook-Verzeichnis, Installation unter `_dist/`, `paths.*` |

Das Werkzeug bricht bei jeder anderen Fassung ab, statt weiterzumachen. Stillschweigend
weiterzulesen waere das Gefaehrlichste: die Werte liessen sich lesen, bedeuteten aber
etwas anderes. Eine hoehere Zahl als `3` wird als „Installation aelter als die
Konfiguration" gemeldet, eine fehlende `schema_version` ebenfalls als Fehler.

Es gibt kein `migrate`-Kommando; die Umstellung ist ein bewusster Schritt.

### `project.repo_root`

Pflichtfeld. Ort des Projekt-Repositorys, relativ zur `K-PLAYBOOK.yaml`.

Typische Werte:

- `.` wenn das Hauptverzeichnis selbst das Repository ist.
- `app` oder ein anderer Verzeichnisname, wenn der Code parallel zur Installation
  ausgecheckt ist — etwa in einem DevContainer.

Das Repo steht bewusst in der Konfiguration und wird nicht aus dem Dateisystem
abgeleitet. Commands duerfen den Wert lesen und pruefen, aber nicht selbst nach
Git-Roots suchen.

### `project.vcs`

Pflichtfeld. Entweder `git` oder `none`. `none` ist eine ausdrueckliche
Projektentscheidung und steht deshalb in der Datei, statt in Commands geraten zu werden.

### `remediation`

Block fuer `/k-remediation`. Das Werkzeug legt ihn bei neuen Projekten gleich mit an.

| Feld | Typ | Bedeutung |
|---|---|---|
| `mode` | enum | `task-branch-pr`, `task-first` oder `direct-allowed` |
| `target` | string | Remediation-Ziel relativ zur `K-PLAYBOOK.yaml`; Default ist `project.repo_root` |
| `grouping` | boolean | Findings vor der Umsetzung zu sinnvollen Buendeln gruppieren |
| `quick_wins` | boolean | einfache, wirkungsstarke Buendel hervorheben |
| `branch_prefix` | string | empfohlener Prefix fuer Remediation-Branches |
| `pr_required` | boolean | aus `mode` abgeleitet |
| `direct_fixes` | boolean | aus `mode` abgeleitet |

Die Modi, vom striktesten zum offensten:

| Modus | Bedeutung | `pr_required` | `direct_fixes` |
|---|---|---|---|
| `task-branch-pr` | Keine direkten Fixes. Jedes bestaetigte Buendel wird eine Task mit Branch- und PR-Hinweis; umgesetzt wird spaeter ueber `/k-run`. | `true` | `false` |
| `task-first` | Tasks sind der Standard. Direkte Fixes nur, wenn sie fuer einzelne kleine Buendel ausdruecklich freigegeben werden. | `false` | `true` |
| `direct-allowed` | Kleine, sichere Befunde duerfen nach Code-Sichtung sofort behoben werden, wenn die Kategorien freigegeben sind. | `false` | `true` |

**Default ist `task-first`.** Tasks als Standard sind die sichere Vorgabe: nichts wird
ohne Zutun am Code geaendert, direkte Fixes bleiben nach Freigabe trotzdem moeglich.

`pr_required` und `direct_fixes` stehen zusaetzlich in der Datei, damit Commands sie
lesen koennen, ohne den Modus deuten zu muessen. Sie werden beim Setzen des Modus
mitgeschrieben und nicht unabhaengig davon gepflegt.

Fehlt der Block, soll `/k-remediation` nicht raten, sondern fuer die aktuelle Sitzung
ausdruecklich fragen.

### `tools`

Optionaler Block fuer projektlokale Tool-Entscheidungen.

Wichtig: hier stehen Projektentscheidungen, keine Host-Fakten. Ob `codeql`, `gitleaks`
oder `trivy` auf diesem Rechner installiert sind, gehoert in einen Preflight-Bericht,
nicht in eine versionierte Projektkonfiguration.

#### `tools.codeql`

| Feld | Typ | Bedeutung |
|---|---|---|
| `target` | string | CodeQL-Analyse-Root relativ zur `K-PLAYBOOK.yaml` |
| `languages` | list[string] | registrierte CodeQL-Sprachen |
| `queries` | string | Query-Suite oder Query-Pack |
| `github.status` | enum | `enabled`, `disabled` oder `planned` |
| `github.workflow` | string/null | projektrelativer Workflow-Pfad oder `null` |
| `local_database.status` | enum | `enabled`, `disabled` oder `planned` |
| `local_database.path` | string/null | projektrelativer Datenbankpfad oder `null` |

Ein zusaetzliches `enabled`-Feld gibt es nicht. Ein Tool gilt als aktiv oder geplant,
wenn mindestens ein relevanter Status `enabled` oder `planned` ist.

## Schreibregeln

- Eine vorhandene `K-PLAYBOOK.yaml` wird nie ueberschrieben. Sie gehoert dem Projekt und
  kann Werte tragen, die das Werkzeug nicht kennt.
- Geschrieben wird ausschliesslich nach Bestaetigung, Schritt fuer Schritt.
- Das Werkzeug besitzt `schema_version` und `project.*`.
- `/k-setup-codeql` besitzt nur `tools.codeql`.
- Die Remediation-Policy wird beim Einbinden gesetzt; spaeter darf `/k-remediation` sie
  nach Rueckfrage aendern. Geschrieben wird nur der `remediation:`-Block.
- Unbekannte Top-Level-Felder bleiben erhalten und werden nicht ungefragt geaendert.
  Geschrieben wird zeilenweise, damit Kommentare und Reihenfolge erhalten bleiben.
- Host-lokale Installationszustaende gehoeren nicht in diese Datei.
- Nichts unterhalb von `k-playbook/` darf geschrieben werden.

## Dateiname

Der kanonische Dateiname ist `K-PLAYBOOK.yaml`. `K-PLAYBOOK.yml` soll nicht erzeugt
werden, damit Werkzeug und Commands nur einen Namen pruefen muessen.
