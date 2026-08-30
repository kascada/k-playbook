# K-PLAYBOOK.yaml Format

`K-PLAYBOOK.yaml` ist die Konfiguration eines Projekts, das k-playbook nutzt. Sie ist
zugleich der **Anker**: ihr Ort bestimmt, was das Hauptverzeichnis des Projekts ist.

## Grundentscheidung

Jedes Projekt trägt seine eigene Installation. Es gibt keine zentrale Basisinstallation
und keinen festen Hostpfad. Die Installation liegt neben der Konfiguration:

```text
<projekt>/                 beliebig benannt
├── K-PLAYBOOK.yaml        der Anker
├── k-playbook/            die Installation, vollständig ersetzbar
└── k-playbook-local/      projekteigen, committed
```

Weil die Konfiguration **neben** und nicht **in** der Installation liegt, enthält
`k-playbook/` nichts Projekteigenes. Das Verzeichnis ist dadurch komplett updatebar —
per `git pull` ebenso wie per `rm -rf` und neuem Clone.

Das Playbook-Verzeichnis heißt immer `k-playbook`. Wie das Projektverzeichnis darüber
heißt, spielt keine Rolle.

## Keine Pfade in der Konfiguration

Frühere Versionen trugen einen `paths:`-Block mit neun Schlüsseln. Den gibt es nicht
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
| Handgepflegte Doku | `k-playbook-local/docs/manual/` |
| Rohmaterial | `k-playbook-local/material/` |
| Guidelines | `k-playbook-local/guidelines/` |
| offene Tasks | `k-playbook-local/tasks/` |
| erledigte Tasks | `k-playbook-local/tasks/done/` |
| Projekt-TODO | `k-playbook-local/TODO.md` |
| Privates | `k-playbook-local/priv/` |
| Instruktionen, mitgeliefert | `k-playbook/k-playbook.md` |
| Instruktionen, projekteigen | `k-playbook-local/k-playbook.md` |

Ein Schlüssel, dessen Wert immer derselbe ist, wäre nur eine Fehlerquelle gewesen.
Commands raten damit keinen Pfad mehr und lesen auch keinen: sie leiten ihn ab.

Das gilt auch für die Projekt-Dokumentation. Früher durfte `paths.docs` als einziger
Wert mit `../` aus dem k-playbook-Verzeichnis herauszeigen, damit ein Projekt seine schon
vorhandene Doku weiterverwenden konnte. Dieser Sonderfall entfällt: `/k-docs-code`
schreibt nach `k-playbook-local/docs/code/`. `docs/` gliedert sich in Unterverzeichnisse
nach Herkunft; wie viele Werkzeuge in eine Herkunft schreiben, sagt das Verzeichnis nicht.
Was ein Projekt sonst noch an Dokumentation pflegt, bleibt davon unberührt — k-playbook
beansprucht nur sein eigenes Verzeichnis.

Innerhalb von `k-playbook-local/docs/` ist die Herkunft am Verzeichnis ablesbar, und
daran hängt die Eigentümerregel:

Jedes Unterverzeichnis von `docs/` steht für eine Herkunft, nicht für ein einzelnes
Werkzeug. Ein Werkzeug schreibt ausschließlich in Verzeichnisse seiner eigenen Herkunft;
welches Werkzeug eine einzelne Datei geschrieben hat, steht in ihrem Frontmatter unter
`generated.by`. `docs/README.md` gehört allein `/k-docs-index`. In `docs/manual/`
schreibt kein Command Doc-Dateien; die Struktur-README aus dem Einrichten ist davon
ausgenommen. Flache `docs/*.md` aus der Zeit vor dieser Struktur haben keinen Erzeuger:
sie werden nur gelistet, geschrieben werden sie von keinem Command.

`docs/code/`, `docs/libs/` und `docs/extracted/` entstehen beim ersten Lauf eines
Werkzeugs ihrer Herkunft — in `docs/code/` schreiben `/k-docs-code` und der Skill
`ks-overlay-repo-analyse`, in `docs/libs/` `/k-docs-tools`, in `docs/extracted/`
`/k-docs-extract`. Das Einrichten legt sie nicht an; es legt `docs/manual/` und
`material/` an.

`k-playbook-local/material/` ist die Quellseite: Rohmaterial wie Chat-Mitschnitte und
Notizen. Es wird nie indiziert und von keinem Command geschrieben. Sein Inhalt wird wie
bei `priv/` ganz normal mitversioniert — k-playbook schreibt dafür keine `.gitignore`.
Rohmaterial enthält typischerweise Tokens, Pfade und Namen; ob es deshalb draußen bleiben
soll, entscheidet das Projekt.

Drei Verzeichnisse stehen so zur Wahl: `results/`, `priv/` und `material/`. `results/`
ist das einzige, das bei der Installation schon privat angelegt wird — Review-Ergebnisse
sind ein Stand von einem Rechner, kein Projektwissen. Umschaltbar sind alle drei. Der
Block **Lokale Einstellungen** der Oberfläche zeigt für **alle drei** Verzeichnisse den
gemessenen Ist-Zustand und schaltet ihn um; von Hand geht es über eine `.gitignore` im
Verzeichnis selbst, deren Inhalt die jeweilige `README.md` nennt. Details in
[`installation.md`](./installation.md#2-projekteigene-struktur-anlegen).

## Anker finden

Der Ablauf gilt gleichermaßen für das Werkzeug und für einen Assistenten:

1. Wurde ein Verzeichnis übergeben, gilt dieses; geprüft wird `<arg>/K-PLAYBOOK.yaml`.
2. Sonst ab `realpath(CWD)` aufwärts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
3. Fund: `PROJECT_DIR = <dir>`, `PLAYBOOK_DIR = <dir>/k-playbook`.
4. Grenze der Aufwärtssuche sind `$HOME` und `/`, jeweils einschließlich.
5. Nichts gefunden: melden, dass keine Installation vorliegt. Nicht raten, nichts anlegen.

Die Aufwärtssuche darf **nicht** am Git-Worktree-Root abbrechen. `<projekt>/k-playbook/`
ist ein eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, käme sonst
nie an die Konfiguration eine Ebene darüber.

## Mitgeliefertes und Projekteigenes zusammenfassen

Fünf Verzeichnisse existieren doppelt. Was gilt, ist die Vereinigung beider Seiten:

| Sorte | mitgeliefert | projekteigen | Einheit |
|---|---|---|---|
| Regeln | `k-playbook/rules/` | `k-playbook-local/rules/` | `*.md` |
| Review-Rezepte | `k-playbook/reviews/` | `k-playbook-local/reviews/` | `review-*.md` |
| Checks | `k-playbook/checks/` | `k-playbook-local/checks/` | `*.sh`, nur oberste Ebene |
| Commands | `k-playbook/commands/` | `k-playbook-local/commands/` | `*.md`, rekursiv |
| Skills | `k-playbook/skills/` | `k-playbook-local/skills/` | Verzeichnis mit `SKILL.md` |

Die Vergleichseinheit ist der **Name**. Beide Seiten benutzen dieselbe
Namenskonvention, deshalb braucht es keinen abgeleiteten Schlüssel.

Bei Commands ist es der Pfad ab `commands/`, einschließlich Namensraum: eine lokale
`commands/_shared/context.md` ersetzt genau diese Datei, der Rest von `_shared/` bleibt
mitgeliefert. Ein Skill dagegen wird als Ganzes ersetzt — `SKILL.md`, `PLAYBOOK.md` und
Vorlagen müssen zueinander passen, ein halb ersetzter Skill wäre nicht sinnvoll
zusammensetzbar.

**Bei gleichem Dateinamen gewinnt die projekteigene Datei, und zwar vollständig.** Die
mitgelieferte wird dann gar nicht erst gelesen; es werden auch keine einzelnen Abschnitte
daraus übernommen. Wer eine mitgelieferte Regel ändern will, kopiert sie und ändert die
Kopie — mit dem Preis, dass spätere Verbesserungen am Original diese Kopie nicht mehr
erreichen. Der Vorteil wiegt schwerer: was gilt, steht in genau einer Datei.

**Abgeschaltet wird über eine leere Datei**, nicht über eine Liste in der
Konfiguration. Da eine gleichnamige lokale Datei die mitgelieferte vollständig ersetzt,
bleibt bei einer leeren nichts übrig. „Leer" heißt: nichts außer Leerzeilen und
Kommentaren — so kann die Datei ihren eigenen Grund tragen:

```bash
# Abgeschaltet: dieses Projekt nutzt kein Django.
```

Der Unterschied zwischen den Sorten ist beabsichtigt. `rules` und `reviews` werden
gelesen; dort bleibt der Eintrag im Katalog sichtbar und sein Inhalt sagt, dass er
abgeschaltet ist. Ein Check wird dagegen **ausgeführt** — ein leeres Skript liefe mit
Exit 0 durch und sähe aus wie ein bestandener Check. Deshalb fällt er ganz aus dem
Katalog.

Eine projekteigene Datei schaltet man ab, indem man sie löscht.

`README.md` in einem der Verzeichnisse ist nie ein Eintrag, ebensowenig Dotfiles oder
irgendetwas unter `checks/lib/`.

Bei Skills entscheidet die `SKILL.md` über das Abschalten: ist sie leer, gilt der Skill
als abgeschaltet und wird nicht registriert.

Wer wissen will, was am Ende gilt, fragt nicht das Dateisystem, sondern das Werkzeug:

```bash
k-playbook/bin/k-playbook context
```

Die Ausgabe führt die zusammengeführten Kataloge mit Herkunft je Eintrag — `dist`,
`local` oder `override` — und markiert Abgeschaltetes. Siehe [Der aufgelöste
Arbeitsstand](#der-aufgelöste-arbeitsstand).

Weil Commands und Skills damit aus zwei Quellen kommen, sind die Assistenten-Ziele —
`.claude/commands`, `.claude/skills`, `.opencode/commands`, `.cursor/commands` — echte
Verzeichnisse mit **einem Symlink je Eintrag**, kein Verzeichnis-Symlink: der zeigte auf
genau eine Quelle.

### Was es nur einmal gibt

Alles Übrige hat kein Gegenstück auf der anderen Seite:

| | Verzeichnisse |
|---|---|
| nur projekteigen | `results/`, `docs/`, `guidelines/`, `tasks/`, `priv/`, `material/`, `TODO.md` |
| nur mitgeliefert | `docs/`, `scripts/`, `bin/`, `installer/` |

`docs/` steht in beiden Zeilen, ist aber kein Paar: `k-playbook/docs/` dokumentiert
k-playbook selbst, `k-playbook-local/docs/` das Projekt. Zwei verschiedene Gegenstände
unter demselben Namen, nichts zusammenzufassen.

Nichts unterhalb von `k-playbook/` darf geschrieben werden — auch nicht von Commands, die
dort Regeln oder Rezepte lesen. Ein Update ersetzt das Verzeichnis vollständig.

## Der aufgelöste Arbeitsstand

```bash
k-playbook/bin/k-playbook context
```

Gibt als JSON aus, was ein Command sonst selbst aus Konfiguration und Dateisystem
zusammenrechnen müsste:

| Feld | Inhalt |
|---|---|
| `schemaVersion` | die geprüfte Fassung der Konfiguration |
| `now` | der Zeitpunkt des Aufrufs: `date` als `YYYY-MM-DD`, `timestamp` nach RFC 3339 |
| `instructions` | die Instruktionsdateien in Lesereihenfolge |
| `project` | Hauptverzeichnis, `repoRoot`, `vcs`, Ort der Konfiguration |
| `playbook`, `local` | die beiden aufgelösten Verzeichnisse |
| `remediation` | die Policy, mit Default, falls der Block fehlt |
| `gh` | die Entscheidung zur GitHub CLI samt Host-Befund |
| `cleanliness` | der lokale Zustand der Installation: geänderte und zusätzliche Dateien, lokale Commits, eingespielter Arbeitsstand |
| `catalogs` | `rules`, `reviews`, `checks` — zusammengeführt |
| `guidelines` | die Dateien aus `k-playbook-local/guidelines/` |

Jeder Katalogeintrag trägt `name` (den Dateinamen), `key` (den Aufrufnamen ohne Endung
und Sortenpräfix), `path`, `origin` — `dist`, `local` oder `override` — und `disabled`,
wo zutreffend.

Damit muss kein Command die Overlay-Regeln selbst anwenden. Es gibt eine Antwort, und
alle bekommen dieselbe.

`cleanliness` steht dort, weil die Regel „in `k-playbook/` wird nie geschrieben" sich
nicht selbst durchsetzt und ihr Bruch still bleibt: Ändert sich eine lokal veränderte
Datei upstream nicht mit, läuft `git pull` sauber durch und lässt sie stehen. Zwei
`git`-Aufrufe im lokalen Clone, ohne Netz — dieselbe Prüfung, die auch die Oberfläche vor
einem Update anstellt. Damit gibt es eine Antwort auf die Frage statt zwei.

`now` steht aus einem anderen Grund dort: Commands stempeln Datumsangaben in Dateien, die
bleiben — Review-Logs, Ergebnisverzeichnisse, Namen von Summary-Dateien. Nicht jedem
Assistenten nennt sein Wirt das heutige Datum, und ein geratenes Datum in einem Protokoll
ist schlechter als gar keines. Es ist das einzige Feld, das altert: es benennt den
Zeitpunkt des Aufrufs, nicht den des Schreibens.

`gh` führt zwei Dinge zusammen, die auseinandergehalten gehören: `status` und
`configured` sind die Projektentscheidung aus `tools.gh` und stehen versioniert in der
Datei; `installed`, `path`, `loggedIn`, `account`, `accounts` und `tokenFromEnv` sind ein
Befund für genau diesen Rechner. `ready` fasst zusammen, was ein Command wissen muss:
gh ist da und ein Account ist hinterlegt.

Der Befund ist aus der gh-Konfiguration gelesen, nicht beim Server geprüft — ein
hinterlegter Token kann abgelaufen sein. Wer Gewissheit braucht, ruft `gh auth status`
auf; das kostet einen Netzzugriff und gehört deshalb nicht hierher.

Der Aufruf ist bewusst billig: der Security-Tool-Preflight fehlt darin, weil er je Tool
ein `--version` startet und spürbar dauert. `context` soll am Anfang jedes Commands
stehen können. Der gh-Befund kostet nichts — ein Blick in den PATH und in
`~/.config/gh/hosts.yml`, kein Unterprozess.

Gesucht wird ab dem Arbeitsverzeichnis aufwärts. Ohne `K-PLAYBOOK.yaml` bricht der
Aufruf mit einer Meldung ab, ebenso bei einer `schema_version`, die nicht `3` ist.

## Instruktionen

Was ein Assistent vor der Arbeit lesen soll, steht in `k-playbook.md` — je einmal pro
Ebene:

| Datei | Gilt für | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge; die projekteigene Ebene kann die mitgelieferte
ergänzen oder überstimmen. `context` nennt unter `instructions` nur die Dateien, die
tatsächlich existieren — ein Pfad ins Leere wäre schlechter als keiner.

Die Datei heißt bewusst nicht `AGENTS.md`: diesen Namen lesen die Assistenten von sich
aus, und er ist dem Hauptverzeichnis vorbehalten.

`AGENTS.md` bekommt nur einen **Anstoß**: einen kurzen Block, der auf
`k-playbook context` verweist. Fehlt die Datei, wird sie angelegt; ist sie da, wird der
Block angehängt und vorhandener Inhalt nicht angetastet. Ein Marker
`<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf ihn erneut anhängt.

## Minimalformat

Das legt das Werkzeug an, wenn ein Projekt neu eingebunden wird:

```yaml
# k-playbook
#
# Der Ort dieser Datei bestimmt das Hauptverzeichnis des Projekts.
# Die Installation liegt daneben unter k-playbook/ und ist vollständig
# ersetzbar; projekteigene Dateien gehören nicht hinein.

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

## Vollständiges Beispiel

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
  gh:
    status: enabled
```

## Felder

### `schema_version`

Pflichtfeld. Aktuelle Version: `3`.

`3` beschreibt das hier dokumentierte Modell: Anker im Hauptverzeichnis,
`k-playbook/` und `k-playbook-local/` daneben, keine Pfade in der Konfiguration.

Ältere Werte gehören zu abgelösten Modellen und werden nicht mehr unterstützt:

| Wert | Modell |
|---|---|
| `1` | zentrale Basisinstallation unter `~/dev/k-playbook` |
| `2` | Anker im k-playbook-Verzeichnis, Installation unter `_dist/`, `paths.*` |

Das Werkzeug bricht bei jeder anderen Fassung ab, statt weiterzumachen. Stillschweigend
weiterzulesen wäre das Gefährlichste: die Werte ließen sich lesen, bedeuteten aber
etwas anderes. Eine höhere Zahl als `3` wird als „Installation älter als die
Konfiguration" gemeldet, eine fehlende `schema_version` ebenfalls als Fehler.

Es gibt kein `migrate`-Kommando: die Modelle beschreiben verschiedene
Verzeichnis-Aufteilungen, und die Felder ineinander umzurechnen hieße, eine Übersetzung
zu pflegen, die mit jedem Modell wächst. Stattdessen setzt die Oberfläche zurück — sie
sichert die alte Datei als `K-PLAYBOOK.yaml.v1-alt` weg und legt eine frische an. Wie
das abläuft, steht in
[`installation.md`](./installation.md#eine-konfiguration-aus-einem-abgelösten-modell).

### `project.repo_root`

Pflichtfeld. Ort des Projekt-Repositorys, relativ zur `K-PLAYBOOK.yaml`.

Typische Werte:

- `.` wenn das Hauptverzeichnis selbst das Repository ist.
- `app` oder ein anderer Verzeichnisname, wenn der Code parallel zur Installation
  ausgecheckt ist — etwa in einem DevContainer.

Das Repo steht bewusst in der Konfiguration und wird nicht aus dem Dateisystem
abgeleitet. Commands dürfen den Wert lesen und prüfen, aber nicht selbst nach
Git-Roots suchen.

### `project.vcs`

Pflichtfeld. Entweder `git` oder `none`. `none` ist eine ausdrückliche
Projektentscheidung und steht deshalb in der Datei, statt in Commands geraten zu werden.

### `remediation`

Block für `/k-remediation`. Das Werkzeug legt ihn bei neuen Projekten gleich mit an.

| Feld | Typ | Bedeutung |
|---|---|---|
| `mode` | enum | `task-branch-pr`, `task-first` oder `direct-allowed` |
| `target` | string | Remediation-Ziel relativ zur `K-PLAYBOOK.yaml`; Default ist `project.repo_root` |
| `grouping` | boolean | Findings vor der Umsetzung zu sinnvollen Bündeln gruppieren |
| `quick_wins` | boolean | einfache, wirkungsstarke Bündel hervorheben |
| `branch_prefix` | string | empfohlener Prefix für Remediation-Branches |
| `pr_required` | boolean | aus `mode` abgeleitet |
| `direct_fixes` | boolean | aus `mode` abgeleitet |

Die Modi, vom striktesten zum offensten:

| Modus | Bedeutung | `pr_required` | `direct_fixes` |
|---|---|---|---|
| `task-branch-pr` | Keine direkten Fixes. Jedes bestätigte Bündel wird eine Task mit Branch- und PR-Hinweis; umgesetzt wird später über `/k-task-run`. | `true` | `false` |
| `task-first` | Tasks sind der Standard. Direkte Fixes nur, wenn sie für einzelne kleine Bündel ausdrücklich freigegeben werden. | `false` | `true` |
| `direct-allowed` | Kleine, sichere Befunde dürfen nach Code-Sichtung sofort behoben werden, wenn die Kategorien freigegeben sind. | `false` | `true` |

**Default ist `task-first`.** Tasks als Standard sind die sichere Vorgabe: nichts wird
ohne Zutun am Code geändert, direkte Fixes bleiben nach Freigabe trotzdem möglich.

`pr_required` und `direct_fixes` stehen zusätzlich in der Datei, damit Commands sie
lesen können, ohne den Modus deuten zu müssen. Sie werden beim Setzen des Modus
mitgeschrieben und nicht unabhängig davon gepflegt.

Fehlt der Block, soll `/k-remediation` nicht raten, sondern für die aktuelle Sitzung
ausdrücklich fragen.

### `tools`

Optionaler Block für projektlokale Tool-Entscheidungen.

Wichtig: hier stehen Projektentscheidungen, keine Host-Fakten. Ob `gitleaks` oder
`trivy` auf diesem Rechner installiert sind, gehört in einen Preflight-Bericht,
nicht in eine versionierte Projektkonfiguration.

#### `tools.gh`

| Feld | Typ | Bedeutung |
|---|---|---|
| `status` | enum | `unknown`, `enabled` oder `disabled` |

Ob dieses Projekt die GitHub CLI nutzt. Gebraucht wird sie von `/k-pr-review` und vom
Dependabot-Review.

**Default ist `unknown`.** Das ist ein ausdrücklicher Zustand und kein
stillschweigendes Nein: ohne Entscheidung weiß ein Command nicht, ob ein fehlendes `gh`
ein Problem oder gewollt ist. Die Oberfläche zeigt `unknown` deshalb als offenen Punkt,
und Commands, die `gh` brauchen, brechen darauf ab. Ein anderer Wert als die drei
genannten ist ein Fehler und lässt `context` abbrechen — ein Tippfehler soll nicht wie
eine Entscheidung aussehen.

Der Block sagt nichts darüber, ob `gh` auf diesem Rechner liegt. Das ist ein Host-Befund
und steht nur in der Kontextausgabe. Ebenso gibt es hier keinen Host: die Entscheidung
gilt für `github.com`.

## Schreibregeln

- Eine vorhandene `K-PLAYBOOK.yaml` wird nie überschrieben. Sie gehört dem Projekt und
  kann Werte tragen, die das Werkzeug nicht kennt.
- Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt.
- Das Werkzeug besitzt `schema_version` und `project.*`.
- Die Oberfläche besitzt nur `tools.gh`. Geschrieben wird der `gh:`-Unterblock; ein
  danebenliegender Block eines anderen Tools bleibt unangetastet. Bei neuen Projekten
  wird er gleich mit `unknown` angelegt, damit die offene Entscheidung in der Datei
  sichtbar ist.
- Die Remediation-Policy wird beim Einbinden gesetzt; später darf `/k-remediation` sie
  nach Rückfrage ändern. Geschrieben wird nur der `remediation:`-Block.
- Unbekannte Top-Level-Felder bleiben erhalten und werden nicht ungefragt geändert.
  Geschrieben wird zeilenweise, damit Kommentare und Reihenfolge erhalten bleiben.
- Host-lokale Installationszustände gehören nicht in diese Datei.
- Nichts unterhalb von `k-playbook/` darf geschrieben werden.

## Dateiname

Der kanonische Dateiname ist `K-PLAYBOOK.yaml`. `K-PLAYBOOK.yml` soll nicht erzeugt
werden, damit Werkzeug und Commands nur einen Namen prüfen müssen.
