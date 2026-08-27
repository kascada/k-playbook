# Review-Läufe

Wie ein Review-Lauf entsteht, was er auf die Platte legt und wie die Beteiligten ihren
Fortschritt darin festhalten. Für die Artefakte, die ein einzelnes Review erzeugt, siehe
[`reviews-and-results.md`](./reviews-and-results.md); für den Umbau, aus dem das hier
hervorgeht, [`umbau.md`](./umbau.md).

## Der Lauf ist die Klammer

Bisher startete jedes Review seine Werkzeuge selbst, und jedes legte seine Ergebnisse an
seinem eigenen Ort ab. Ein Lauf fasst zusammen, was zusammengehört: **eine Auswahl, ein
Verzeichnis, ein Zustand.**

Ein Lauf besteht aus **Einträgen**, und ein Eintrag hat eine **Art**:

| Art | Was es ist | Wer führt es aus |
|---|---|---|
| `tool` | ein Security-Tool aus der Matrix | das Werkzeug, über eine CLI |
| `ai` | ein Review-Rezept aus dem Katalog oder ein Command-Moduleintrag | ein Assistent, über einen Command |

Beide sind Einträge desselben Laufs. Nur der Weg dorthin unterscheidet sich — das Ergebnis
landet für beide im selben Verzeichnis und damit später in derselben Zusammenfassung.
Der Standard-Moduleintrag `scan-triage` kommt aus
`commands/_audit/review-scan-triage.md`; er gehört bewusst nicht zu
`catalogs.reviews` und erscheint deshalb nicht in der GUI-Auswahl für Review-Rezepte.

## Das Laufverzeichnis

```text
k-playbook-local/results/
└── 2026-08-12/                  der Lauf, benannt nach dem Tag
    ├── run.json                 die Festlegung: was ausgewählt wurde
    ├── entries/
    │   ├── semgrep.json         Fortschritt und Probleme je Eintrag
    │   └── review-secret-scanning.json
    └── raw/                     die SARIF-Dateien der Scan-Jobs
        ├── semgrep.sarif
        ├── gitleaks-git.sarif
        └── tech.sarif            das Artefakt einer Evidence-Quelle
```

Der Name ist das Datum, `YYYY-MM-DD`. **Existiert das Verzeichnis bereits, bricht das
Anlegen ab** statt einen zweiten Lauf danebenzustellen: ein Tag, ein Lauf. Wer erneut
starten will, räumt das vorhandene Verzeichnis weg oder benennt es um.

**`raw/` legt der Ausführer an**, beim ersten Job, den er startet. Das Anlegen eines
Laufs kennt das Verzeichnis nicht: ein Lauf ohne Werkzeug-Eintrag braucht es nicht. Eine
Evidence-Quelle schreibt ebenfalls hierhin, nach `raw/<entry>.sarif`, und legt das
Verzeichnis an, wenn vor ihr kein Werkzeug lief.

**Es gibt zwei Orte für Rohdaten.** Die Ergebnisfamilien unter
`k-playbook-local/results/<familie>/YYYY-MM-DD/raw/` bleiben, wie sie sind: Ein gezielter
`/k-review`-Lauf legt sie weiter an, und `/k-remediation` arbeitet die Triage daneben ab;
`ListRuns()` zeigt sie ohnehin mit an. Das
Laufverzeichnis ist dagegen familienlos, weil ein Lauf gerade über die Familien hinweg
klammert. Katalog-Rezepte im Laufmodell legen keine eigene Rohdatenablage an: eine
Perspektive liest den gemeinsamen Merge-Beleg aus diesem Laufordner, eine Evidence-Quelle
schreibt ihr SARIF in dessen `raw/` — kein Rezept öffnet dafür einen Family-Ordner.

## Wer was schreibt

Der entscheidende Punkt, wenn mehrere Beteiligte gleichzeitig arbeiten: **niemand schreibt
in eine fremde Datei.**

| Datei | Schreiber |
|---|---|
| `run.json` | nur das Anlegen des Laufs; danach niemand |
| `entries/<name>.json` | nur der Eintrag, dem sie gehört |
| `raw/<job>.sarif` | nur der Job, der sie erzeugt; weggeräumt wird sie vom Eintrag, dem er gehört |

Schreiber ist der Job, Aufräumer der Eintrag — und der räumt nur seine eigenen Dateien
weg, die aus der vorigen `entries/<tool>.json`. „Niemand schreibt in eine fremde Datei"
bleibt davon unangetastet.

Damit können Werkzeuge parallel laufen, ohne sich gegenseitig zu überschreiben. Eine
einzelne gemeinsame Datei wäre einfacher zu lesen, aber der zweite Schreiber löschte den
Fortschritt des ersten — bei parallelen Scans ist das kein Randfall, sondern der Normalfall.
Geschrieben wird **atomar**: erst eine Temp-Datei im selben Verzeichnis, dann `rename`.
Sonst sähe ein Leser, der während des Laufs nachschaut, irgendwann eine halbe Datei.

Dass das Ausführen `run.json` nicht anfasst, erspart eine Sperre je Lauf. Zwei
gleichzeitige Aufrufe kollidieren nur, wenn sie denselben Eintrag nennen — das ist
derselbe Fall wie der wiederholte Aufruf, und er gilt als überschreibend, nicht als
ergänzend.

**Der Gesamtzustand wird beim Lesen abgeleitet**, aus den Dateien unter `entries/`;
eine fehlende Datei zählt als `start`. Sind alle Einträge `start`, ist der Lauf
`created`; sind alle in einem Endzustand, ist er `done`; sonst `running`.

**Vorrangregel:** Weichen der Zustand in `run.json` und der unter `entries/` voneinander
ab, gilt `entries/`. `run.json` hält fest, was ausgewählt wurde, nicht, wie weit es ist.

## `run.json`

```json
{
  "schemaVersion": 1,
  "created": "2026-08-12T14:03:11+02:00",
  "state": "created",
  "languages": ["python", "go"],
  "entries": [
    { "name": "semgrep",                "kind": "tool", "state": "start" },
    {
      "name": "secret-scanning",
      "kind": "ai",
      "state": "start",
      "recipeKey": "secret-scanning",
      "recipePath": "/projekt/k-playbook/reviews/review-secret-scanning.md",
      "recipeOrigin": "dist",
      "title": "Secret-Scanning Assessment",
      "mode": "perspective",
      "resultRequired": true,
      "defaultResult": "review-secret-scanning.md",
      "scope": {
        "tools": ["gitleaks", "trufflehog"]
      }
    },
    {
      "name": "tech",
      "kind": "ai",
      "state": "start",
      "recipeKey": "tech",
      "recipePath": "/projekt/k-playbook/reviews/review-tech.md",
      "recipeOrigin": "dist",
      "title": "Tech-Debt-Analyse",
      "mode": "evidence",
      "resultRequired": false,
      "scope": {
        "paths": ["**/*.go", "**/*.py"]
      }
    }
  ]
}
```

Die Schlüssel sind englisch, wie alles, was das Werkzeug sonst als JSON ausgibt —
`schemaVersion`, `missingRequired`, `installMethod`. Sie entstehen aus den Go-Feldnamen.

`languages` hält fest, mit welcher Sprachauswahl der Lauf angelegt wurde. Sie kann sich
danach ändern; was gelaufen ist, soll trotzdem nachvollziehbar bleiben.

Die `entries` in `run.json` tragen den Zustand mit, den der Lauf **festgelegt** hat. Den
tatsächlichen Fortschritt führt die Datei unter `entries/`.

AI-Einträge kopieren beim Anlegen die Metadaten aus dem Review-Rezept in `run.json`.
Spätere Änderungen am Rezept ändern den Lauf nicht mehr. Die optionalen Metadaten stehen
im YAML-Frontmatter des Rezepts:

```yaml
---
audit:
  enabled: true
  mode: perspective
  title: "Secret-Scanning Assessment"
  resultRequired: true
  defaultResult: "review-secret-scanning.md"
  scope:
    tools: [gitleaks, trufflehog]
review:
  enabled: true
---
```

```yaml
---
audit:
  enabled: true
  mode: evidence
  title: "Tech-Debt-Analyse"
  ruleIds: [tech-swallowed-error, tech-duplicated-logic]
  scope:
    paths: ["**/*.go", "**/*.py"]
review:
  enabled: true
---
```

`audit.enabled` ist standardmäßig `false`; nur `true` nimmt das Rezept in `/k-audit`-/MCP-Läufe auf.
`review.enabled` ist standardmäßig `true`; `false` entfernt das Rezept aus der `/k-review`-Auswahl.
`title` fällt ohne Angabe auf die erste Überschrift oder den Katalog-Schlüssel zurück.
`audit.mode` ist standardmäßig `perspective`; Rezepte ohne das Feld bleiben unverändert
gültig. Welche weiteren Felder gelten, hängt daran — der ganze Vertrag steht unter
[Katalog-Rezepte im Lauf](#katalog-rezepte-im-lauf).

Für `mode: perspective`: `resultRequired` ist standardmäßig `true` und bestimmt, ob ein
`done`-Status ein Ergebnis braucht. `defaultResult` ist ein relativer Vorschlag im
Laufverzeichnis. `scope.tools` ist der beim Anlegen eingefrorene Tool-Scope.

Für `mode: evidence`: `scope.paths` ist der beim Anlegen eingefrorene Pfad-Scope.
`resultRequired` wird hart auf `false` gesetzt und steht so in `run.json` — nicht weil das
Rezept es so schriebe, sondern weil es das dort gar nicht darf: Pflichtartefakt ist
`raw/<entry>.sarif`, und mit der Vorgabe `true` meldete `review_status` jeden
erfolgreichen Evidence-Eintrag als `resultMissing` und `inconsistent`. `defaultResult`
fehlt aus demselben Grund.

`audit.ruleIds` wird **nicht** in `run.json` eingefroren. Der Scope hält fest, worüber der
Eintrag gelaufen ist; die Rule-ID-Liste dagegen ist der Vertrag, an dem sein Artefakt beim
Melden gemessen wird, und der wird frisch aus dem Rezept gelesen. Ein Rezept, das die
Liste nach Laufstart ändert, ändert damit die Prüfung — erfüllt es den Evidence-Vertrag
gar nicht mehr, scheitert das Melden mit `recipe_contract_invalid`.

Spätere Rezeptänderungen ändern bestehende Läufe im Übrigen nicht.

Der Moduleintrag `scan-triage` erhält dieselben Laufmetadaten aus dem effektiven
Command-Namensraum: `recipePath` zeigt auf
`commands/_audit/review-scan-triage.md`, `defaultResult` ist
`review-triage.md`, `resultRequired` ist `true`. Ein leeres lokales Overlay unter
`k-playbook-local/commands/_audit/review-scan-triage.md` schaltet diesen
Eintrag ab.

## Katalog-Rezepte im Lauf

Ein aktives Katalog-Rezept arbeitet im Lauf in einer von zwei Betriebsarten. Welche es
ist, sagt `audit.mode` im Rezept und nicht der Command; ohne Angabe gilt `perspective`,
Rezepte von vor der zweiten Betriebsart bleiben also unverändert gültig.

Der `audit`-Block ist optional. Fehlt er, bleibt das Rezept im Audit-Laufmodell inaktiv.
`audit.enabled: false` deaktiviert nur die `/k-audit`-/MCP-Auswahl; `review.enabled`
steuert weiterhin die gezielte `/k-review`-Auswahl.

Die Felder des Blocks gehören jeweils zu genau einer Betriebsart. Ein Rezept, das sie
mischt, wird nicht stillschweigend zurechtgebogen: es erscheint in der Auswahlbasis unter
`unavailableCandidates` mit `selectable: false` und einem `unavailableReason`, der die
verletzte Regel nennt.

### Perspektive (`mode: perspective`)

Eine Perspektive läuft **nach** dem Merge und ist kein eigener Scanner. Sie liest den
Merge-Output `review-input.json` und schreibt genau eine Markdown-Datei direkt in den
Laufordner, z. B. `review-secret-scanning.md`. Das vollständige Beispiel für diesen
Vertrag steht im Rezept
[`review-secret-scanning.md`](../reviews/review-secret-scanning.md).

Frontmatter-Vertrag:

```yaml
---
name: review-<key>
title: <Titel>
audit:
  enabled: true
  mode: perspective
  defaultResult: review-<key>.md
  resultRequired: true
  scope:
    tools: [<tool>, <tool>]
review:
  enabled: true
---
```

Scope-Semantik bei `mode: perspective`:

- `scope.tools` filtert auf Evidence-Ebene von `review-input.json`.
- Eine Gruppe gehört zur Perspektive, wenn mindestens eine Evidence dieser Gruppe ein
  `evidence.tool` aus `scope.tools` trägt.
- Die originale Gruppen-ID bleibt unverändert. Das Rezept dedupliziert, splittet und
  nummeriert Gruppen nicht neu.
- Die Perspektive bewertet nur Evidence aus `scope.tools` als primären Befund.
- Evidence aus anderen Tools bleibt als Kontext sichtbar, muss aber im Report eindeutig als
  „außerhalb des Scopes" markiert werden.
- Ein Finding kann in mehreren Perspektiven auftauchen. Das ist gewollt und kein
  Dedupe-Fehler.
- Leere Scope-Ergebnisse sind gültig. Das Rezept schreibt dann eine Ergebnisdatei mit
  Status „keine scoped Findings" statt eigene Scans nachzuholen.

### Evidence-Quelle (`mode: evidence`)

Eine Evidence-Quelle läuft **vor** dem Merge. Sie liest Code im eingefrorenen Pfad-Scope
und liefert einen Teil von `review-input.json`, statt ihn zu lesen. Ihr Pflichtartefakt
ist SARIF unter `raw/<entry>.sarif`; ein Markdown-Ergebnis entsteht nicht. Die beiden
umgestellten Beispiele sind [`review-tech.md`](../reviews/review-tech.md) und
[`review-python-comment-hardspots.md`](../reviews/review-python-comment-hardspots.md).

Frontmatter-Vertrag:

```yaml
---
name: review-<key>
title: <Titel>
audit:
  enabled: true
  mode: evidence
  ruleIds: [<rule-id>, <rule-id>]
  scope:
    paths: ["<glob>", "<glob>"]
review:
  enabled: true
---
```

`scope.paths` und `ruleIds` sind beide Pflicht. Ohne Pfad-Scope läse der Eintrag das ganze
Repo; ohne Rule-ID-Liste wären seine Funde von Lauf zu Lauf nicht vergleichbar.
`scope.tools`, `resultRequired` und `defaultResult` sind hier verboten: das erste
beschreibt einen Filter auf `review-input.json`, den der Eintrag gar nicht liest, die
beiden anderen eine Ergebnisdatei, die es nicht gibt.

Ergebnisvertrag, geprüft beim Melden über `k_playbook_review_write_ai_entry`:

- `tool.driver.name` im SARIF entspricht dem Eintragsnamen. Steht dort etwas anderes,
  behauptet das Artefakt eine Herkunft, die es nicht hat.
- Jede Rule-ID des SARIF steht in `audit.ruleIds`. Eine fremde macht den Eintrag `failed`
  mit Grund — kein stilles `done`.
- Funde außerhalb von `scope.paths` oder innerhalb der geerbten Ausschlüsse werden
  verworfen und gezählt, `raw/<entry>.sarif` wird bereinigt zurückgeschrieben. Der Eintrag
  bleibt gültig; die Zahl und die ersten Pfade stehen im `reason`. Ein einzelner Ausreißer
  vernichtet damit nicht das ganze Artefakt.
- Ein SARIF ohne Ergebnisse ist ein gültiges `done`. Ein leerer Scope-Befund ist ein
  Ergebnis, kein Fehler — dieselbe Regel wie „Leere Scope-Ergebnisse sind gültig" bei
  Perspektiven.

`level` je Fund ist die Wertung, die das Rezept vergibt, und der Merge nimmt sie ernst:
`error` und `note` gelten unverändert (`severitySource: native`), `warning` und `none`
laufen weiter über CVSS, Tool-Metadaten und `scripts/severity.tsv`. Das Rezept legt das
`level` je Rule-ID fest — Näheres in [`rules/review-authoring.md`](../rules/review-authoring.md).

Scope-Semantik bei `mode: evidence`:

- `scope.paths` sind Globs relativ zur Projektwurzel. Verglichen wird Segment für
  Segment; `**` überspringt beliebig viele Segmente, `*` und `?` gelten innerhalb eines
  Segments. Ein Muster trifft eine Datei auch dann, wenn es ihr Verzeichnis trifft:
  `installer` und `installer/**` decken beide `installer/internal/review/run.go`.
- Innerhalb der Globs gelten zusätzlich die zentralen Ausschlüsse der Modulsuche —
  `k-playbook/`, `k-playbook-local/`, `vendor/`, `node_modules/`, `testdata/` und
  Punkt-Verzeichnisse. Sie werden geerbt und gehören nicht noch einmal ins Rezept.
- Der Pfad-Scope ist verbindlich und wird beim Melden erzwungen. `scope-hint` bleibt
  Freitext für `/k-review` und darf ihn weder erweitern noch überstimmen.
- Die Gruppen-ID entsteht im Merge und nicht im Rezept: KI-Funde bilden eine Gruppe je
  Rule-ID und Datei, ohne Zeile und ohne Meldung, mit dem Präfix `ai-<entry>-`. Mehrere
  Instanzen desselben Problems in einer Datei sind deshalb eine Gruppe; ihre Zahl steht in
  `findingIds`. Das Rezept fasst nicht selbst zusammen.
- Eine Gruppe, in der Scanner- und KI-Belege zusammenliegen, behält die Scanner-ID.

### Reihenfolge im Lauf

Ein `/k-audit`-Lauf arbeitet die Einträge in dieser Reihenfolge ab:

1. Tool-Einträge laufen und schreiben `entries/<tool>.json` sowie `raw/<job>.sarif`.
2. AI-Einträge mit `mode: evidence` laufen im eingefrorenen Pfad-Scope und schreiben
   `raw/<entry>.sarif`.
3. Der Merge schreibt `review-input.json` und `review-input.md` aus den Tool-Einträgen
   **und** den Evidence-Einträgen in einem Endzustand: gelesen wird, wessen Eintrag auf
   `done` steht und wessen Job ein SARIF im Laufordner nennt.
4. Aktive Katalog-Rezepte mit `mode: perspective` lesen `review-input.json` als
   Perspektiven.
5. Optional läuft danach `scan-triage` und darf Perspektiven-Reports als Kontext nutzen.

Offene, fehlgeschlagene oder noch nicht ausgeführte AI-Einträge blockieren den Merge
nicht — Perspektiven so wenig wie Evidence-Quellen. Sie können den AI-Teil offen lassen,
aber `review-input.json` muss aus den Tool-Scans erzeugbar bleiben. Trifft die Evidence
später ein, ist ein erneuter Merge der reguläre Weg und kein Reparaturfall; eine
`review-triage.md`, die vor diesem Merge geschrieben wurde, ist danach veraltet und wird
neu geschrieben.

### Reparatur und Verifikation

Reparaturmatrix für AI-Einträge. Woran ein Eintrag gemessen wird, hängt an seiner
Betriebsart: bei `mode: perspective` an der Ergebnisdatei, bei `mode: evidence` am SARIF.

| Betriebsart | Zustand | Verhalten |
|---|---|---|
| `perspective` | Eintrag existiert in `run.json`, Ergebnisdatei fehlt, Entry ist offen | Status bleibt offen; Rerun führt den AI-Eintrag erneut aus und schreibt die Ergebnisdatei. |
| `perspective` | Eintrag existiert in `run.json`, Ergebnisdatei fehlt, Entry steht auf abgeschlossen | Statusausgabe markiert den Eintrag als inkonsistent und `resultRequired` nicht erfüllt; Rerun schreibt die Ergebnisdatei neu und repariert den Status. |
| `perspective` | Eintrag existiert in `run.json`, Ergebnisdatei existiert, Entry-Status fehlt oder ist offen | Statusausgabe darf den Eintrag als reparabel markieren; Rerun muss keine neue Datei erzwingen, sondern darf den Status über `k_playbook_review_write_ai_entry` auf abgeschlossen setzen, wenn die Datei nicht leer ist. |
| `perspective` | Eintrag existiert in `run.json`, Ergebnisdatei existiert, Entry ist abgeschlossen | Keine Reparatur nötig. |
| `evidence` | Entry ist `done`, Job vorhanden, `raw/<entry>.sarif` existiert und ist nicht leer | Keine Reparatur nötig. |
| `evidence` | Entry ist `done`, aber ohne Job oder ohne vorhandenes SARIF | Statusausgabe meldet `sarifMissing` und `inconsistent`; nur ein erneuter Rezeptlauf repariert das. Ein nachgeschriebener Status ohne Artefakt bleibt eine leere Zusage — der Merge liest die Datei, nicht den Zustand. |
| `evidence` | Gültiges SARIF, Entry-Status fehlt oder ist offen | Statusausgabe meldet `repairable`; `k_playbook_review_write_ai_entry` mit dem Job genügt, ein erneuter Rezeptlauf ist nicht nötig. |
| `evidence` | Entry ist `failed` | Durch einen erneuten Rezeptlauf ersetzen, nicht durch Nachschreiben des Status. |
| `evidence` | Funde außerhalb von `scope.paths` gemeldet | Teilannahme: die Funde werden verworfen, `raw/<entry>.sarif` bereinigt zurückgeschrieben, der Eintrag bleibt gültig. Zahl und erste Pfade stehen im `reason`. |
| beide | Rezept wurde nach Laufstart deaktiviert oder geändert | Der bestehende Lauf nutzt weiter den `scope`-Snapshot aus `run.json`. Die Rule-ID-Liste wird beim Melden dagegen frisch aus dem Rezept gelesen; erfüllt es den Evidence-Vertrag nicht mehr, scheitert das Melden mit `recipe_contract_invalid` und schreibt nichts. |
| beide | Alter Lauf enthält keinen Eintrag für ein später hinzugefügtes Rezept | Keine automatische nachträgliche Ergänzung der alten `run.json`. Neue Rezept-Einträge entstehen nur beim Erzeugen eines neuen Laufs. |

Manuelle Verifikation nach Änderungen am Laufmodell der Katalog-Rezepte:

1. `/k-audit 2026-08-21` in einer neuen Assistenten-Session starten und Status lesen.
2. Prüfen, dass `secret-scanning` nach dem Merge `review-secret-scanning.md` ohne
   Alignment-Hinweis erzeugt und den Scope `gitleaks`, `trufflehog` nennt.
3. Prüfen, dass `tech` und `python-comment-hardspots` vor dem Merge als Evidence-Einträge
   laufen, ihr SARIF nach `raw/<entry>.sarif` schreiben und danach als Gruppen mit dem
   Präfix `ai-<entry>-` in `review-input.json` stehen.
4. In einem CVE-lastigen Ziel einen kleinen Lauf mit Dependency-Tools anlegen und prüfen,
   dass `dependency-cve` als Perspektive über `review-input.json` läuft.
5. In Statusausgabe und Create-Dry-Run prüfen, dass aktive Rezept-Einträge ihren
   gespeicherten `scope` und ihr `mode` zeigen und die Listen `evidenceCandidates` und
   `perspectiveCandidates` die Reihenfolge des Laufs wiedergeben.
6. Je einen beschädigten AI-Eintrag aus der Reparaturmatrix simulieren und prüfen, dass
   Status oder Rerun die erwartete Reparatur zeigt.
7. Ein Rezept mit widersprüchlichem `audit`-Block anlegen und prüfen, dass es unter
   `unavailableCandidates` mit Grund erscheint, statt still zu wirken.

## Zustände

Für den Lauf:

| Zustand | Bedeutung |
|---|---|
| `created` | angelegt, noch nichts gestartet |
| `running` | mindestens ein Eintrag läuft oder ist fertig |
| `done` | alle Einträge sind durch |

Einen Laufzustand `failed` gibt es nicht: ein technischer Fehlschlag steht am Eintrag,
nicht am Lauf. Ein Lauf, in dem ein Werkzeug scheitert, ist trotzdem durch.

Für einen Eintrag:

| Zustand | Bedeutung |
|---|---|
| `start` | ausgewählt, noch nicht gestartet |
| `running` | läuft gerade |
| `done` | fertig, Ergebnis liegt vor |
| `failed` | technisch fehlgeschlagen |
| `skipped` | übersprungen, etwa weil das Werkzeug fehlt |

`failed` meint immer den **technischen** Fehlschlag, nie einen Befund. Ein Scanner, der
Probleme findet, ist `done` — das ist seine Aufgabe. Fast alle Scanner enden mit einem
Exit-Code ungleich 0, sobald sie etwas gefunden haben; maßgeblich ist deshalb nicht der
Code, sondern ob lesbares SARIF vorliegt.

`skipped` deckt auch den Fall ab, dass ein Werkzeug **selbst** signalisiert, dass es
unter dem Bezugspunkt nichts zu prüfen gab — nicht als Zufall aus einem Exit-Code, sondern
als bewusste Meldung. Der Katalog trägt in der Spalte `soft_skip` je Zeile eine oder
mehrere Regeln der Form `<Exit-Code>:<Regex>`, durch `;` getrennt. Passen Prozess-Exit-Code
und Muster in stderr oder stdout, führt der Ausführer den Job als `skipped` mit der
passenden Zeile als Grund, statt ihn als technischen Fehlschlag zu deuten. Der Marker
greift nur, wenn der Prozess regulär mit einem Exit-Code beendet ist und keine SARIF-Datei
geschrieben wurde oder die Datei leer ist. **Lesbares SARIF gewinnt** und bleibt `done`;
**kaputtes, nicht leeres SARIF bleibt `failed`**. Timeouts, Runner-Abbruch und Startfehler
bleiben ebenfalls `failed` — der Marker meint einen bewussten Ausgang, keinen technischen.
Der bisher auslösende Fall ist `osv-scanner`: Exit 128 plus „No package sources found",
ohne SARIF-Datei.

**Ein leeres Ergebnis ist ohne die Kandidatenzahl nicht zu lesen.** `done` mit 0 Befunden
heißt nur, dass lesbares SARIF vorliegt — nicht, dass etwas geprüft wurde. „Geprüft und
sauber" und „gar nichts geprüft" schreiben dieselbe Datei, und der zweite Fall ist der
schädliche: er reicht einen falsch negativen Befund als Entlastung weiter. Deshalb trägt
jeder Job, für den gezählt werden konnte, die Zahl der Dateien, die als Gegenstand in
Frage kamen (`candidates`, siehe unten). Ein neuer Zustand entsteht daraus nicht: die
Zählung trennt „nichts zu prüfen" von den beiden anderen Fällen, nicht „geprüft und
sauber" von „nichts geprüft". Das Werkzeug stellt fest, beurteilt wird im
Bewertungsschritt.

## Einen Eintrag ausführen

```text
k-playbook scan <lauf> [eintrag …]
```

Ohne Eintragsangabe laufen alle Werkzeug-Einträge, die unter `entries/` auf `start`
stehen. Das Kommando **blockiert**, bis alle ausgewählten Einträge durch sind; der
Fortschritt wird währenddessen allein aus `entries/` gelesen.

**Nur `kind: tool`.** Ein `ai`-Eintrag bleibt unangetastet auf `start` — ihn führt ein
Assistent über seinen Command aus, und dabei entsteht auch erst seine Datei. Wird er
ausdrücklich genannt, sagt das Kommando das auf stderr und lässt ihn stehen.

### Der Eintrag ist das Werkzeug, der Job ist der Aufruf

Ein Eintrag ist ein Werkzeug aus der Matrix. Wie dieses Werkzeug aufgerufen wird, steht
in [`scripts/scanners.tsv`](../scripts/scanners.tsv) — eine Zeile je **Job**, mit
Sprachen, Zeitgrenze und dem Aufruf samt Platzhaltern. Ein Werkzeug kann mehrere Jobs
haben, einen oder gar keinen:

| Ebene | Name | Datei |
|---|---|---|
| Eintrag | `trivy` | `entries/trivy.json` |
| Job | `trivy-fs`, `trivy-config` | `raw/trivy-fs.sarif`, `raw/trivy-config.sarif` |

Wer einen Lauf zusammenstellt, sieht davon nichts: die Oberfläche bietet Werkzeuge an,
keine Jobs. Ein Job, dessen Sprache nicht gewählt ist, dessen Werkzeug fehlt oder der
kein SARIF liefern kann, wird `skipped` mit Grund — nicht `failed`.

**Der Name kommt aus dem Katalog — außer bei mehreren Modulen.** Eine Katalogzeile mit
`workdir: module` nennt kein festes Arbeitsverzeichnis, sondern verlangt eins: der
Ausführer sucht die Module unter dem Ziel und startet den Job je gefundenem Modul einmal,
mit dem Modul als Arbeitsverzeichnis. Nötig ist das für Aufrufe wie `govulncheck -format
sarif ./...`, die überhaupt kein Pfad-Argument haben.

| Gefundene Module | Jobs | Name |
|---|---|---|
| keins | einer, `skipped` mit Grund | aus dem Katalog |
| genau eins | einer | aus dem Katalog, unverändert |
| mehrere | einer je Modul | Katalogname plus abgeleitetes Suffix, `govulncheck-installer` |

Das Suffix entsteht aus dem Pfad des Moduls relativ zum Ziel: Pfadtrenner werden zu `-`,
was als Dateiname nicht taugt, fällt weg. Zwei Module können dabei auf denselben Namen
führen; der zweite bekommt dann eine Ziffer angehängt, sonst überschriebe sein Job die
Datei des ersten.

Kein auffindbares Modul ist `skipped`: es fehlt der Gegenstand, nicht das Werkzeug. Eine
Suche, die selbst nicht durchführbar ist — Lesefehler, fehlende Rechte —, ist dagegen
`failed`: dann ist gerade unbekannt, ob es ein Modul gibt, und `skipped` behauptete, es
gebe nichts zu tun.

Die Suche übergeht die Installationskopie `k-playbook/`, das projekteigene
`k-playbook-local/`, dazu `vendor/`, `testdata/`, `node_modules/` und alles mit führendem
Punkt: dort liegt kein Modul des Projekts. Diese Liste steht im Code und nicht im Katalog
— anders als der Werkzeugausschluss gehört sie zu keinem Aufruf.

**Welches Modul geprüft wurde, steht am Job**, auch im Ein-Modul-Fall, wo Job- und
Dateiname unverändert bleiben. Der Name allein trüge die Auskunft sonst nur dann, wenn
aufgefächert wurde — also gerade nicht im Regelfall.

Das Programm eines Jobs kommt aus dem Preflight (`install-security-tools.sh --json`),
nicht aus einer eigenen PATH-Auflösung: sonst griffe der Lauf in einem Python-Projekt
mit aktivem venv dessen `ruff` und prüfte damit ein anderes Werkzeug als der Preflight
([`rules/tool-install-scope.md`](../rules/tool-install-scope.md)).

Jobs laufen parallel, mit einer Obergrenze über den ganzen Lauf. **Sie schreiben nicht
selbst**: sie melden ihr Ergebnis an ihren Eintrag, und der schreibt seine Datei. Damit
bleibt es bei einem Schreiber je Datei.

### Aus n Jobs wird ein Eintragszustand

Zuerst die Frage, ob der Eintrag überhaupt durch ist; erst danach die nach dem Ausgang:

0. Läuft noch ein Job oder steht einer aus → `running`.
1. Sonst mindestens ein Job `failed` → `failed`.
2. Sonst mindestens ein Job `done` → `done`.
3. Sonst → `skipped`.

Regel 0 steht voran, weil ein laufender Eintrag sonst über Regel 3 als `skipped` gälte —
und der Lauf daraus ein `done` machte, während er noch läuft. `done` steht über
`skipped`, weil ein streng schlechtester Ausgang `gitleaks` zu `skipped` machte, sobald
`gitleaks-dir` übersprungen wird, und damit die Datei versteckte, die `gitleaks-git`
tatsächlich geschrieben hat. `skipped` heißt deshalb genau eins: der Eintrag ist durch,
und kein einziger Job ist gelaufen — auch der Fall „gar kein Job", wie bei `syft`, das
eine SBOM erzeugt und keine Befunde.

### `entries/<name>.json`

```json
{
  "schemaVersion": 1,
  "name": "trivy",
  "kind": "tool",
  "state": "done",
  "started": "2026-08-13T09:12:04+02:00",
  "finished": "2026-08-13T09:13:41+02:00",
  "jobs": [
    { "job": "trivy-fs",     "state": "done",    "exitCode": 1, "sarif": "raw/trivy-fs.sarif",
      "findings": 12, "candidates": 3, "started": "…", "finished": "…" },
    { "job": "trivy-config", "state": "skipped", "reason": "Sprache nicht gewählt" },
    { "job": "govulncheck",  "state": "done",    "module": "installer", "exitCode": 0,
      "sarif": "raw/govulncheck.sarif", "findings": 0, "candidates": 2,
      "started": "…", "finished": "…" }
  ]
}
```

Der Eintrag trägt seinen abgeleiteten Zustand, darunter bleiben die Jobs einzeln
sichtbar: Der Gesamtzustand ist die Kurzfassung, nicht die einzige Auskunft. `exitCode`,
`findings` und `candidates` fehlen, wo nichts gemessen wurde — 0 hieße hier „gemessen und
null".
`reason` steht bei `skipped` und `failed`; ein Werkzeug ohne Job trägt ihn am Eintrag,
weil es keinen Job gibt, an dem er stehen könnte. `module` nennt das geprüfte Modul,
relativ zum Ziel des Laufs, die Wurzel selbst als `.`; bei `workdir: target` fehlt es —
dort gibt es kein Modul, auf das es zeigen könnte. Das Beispiel mischt bewusst: die
`trivy`-Jobs laufen projektweit, `govulncheck` an einem Modul.

#### `candidates` — was der Job hätte prüfen können

`candidates` ist die Zahl der Dateien, die unter dem **Bezugspunkt** des Jobs als
Gegenstand in Frage kamen: bei `workdir: module` das Modul, sonst das Ziel des Laufs.
Welche Dateien zählen, sagt die Spalte `candidates` in
[`scripts/scanners.tsv`](../scripts/scanners.tsv):

| Sorte | Kandidat ist | Für |
|---|---|---|
| `source` | eine Datei mit der Endung einer Sprache aus `languages` | `gosec`, `ruff`, `golangci-lint`, `semgrep` |
| `any` | jede Datei | `gitleaks`, `trufflehog` |
| `manifest` | ein Abhängigkeits-Manifest, ebenfalls nach `languages` | `trivy fs`, `govulncheck`, `osv-scanner`, `grype`, `pip-audit` |
| `none` | nichts; das Feld bleibt ungesetzt | `trivy config` |

Die Sorte steht im Katalog und nicht im Code: eine Regel, die auf einen Werkzeugnamen
prüft, wäre genau der Sonderfall, den diese Auskunft vermeiden soll. `none` ist die
ausdrückliche Ausnahme — `trivy config` sucht IaC-Konfigurationen, und die ohne trivys
eigene Erkennungslogik abzugrenzen erzeugte eher Falschalarme, als dass es einordnete.

Gezählt wird **je Bezugspunkt und Sorte einmal im ganzen Lauf**, nicht je Job: derselbe
Baumlauf über dasselbe Ziel ergibt für jeden Job derselben Sorte dasselbe. Ausgelassen
werden dabei die Verzeichnisse, die ohnehin kein Job sieht — `k-playbook/` und
`k-playbook-local/results/`, beide **am Bezugspunkt verankert**, dazu alles mit führendem
Punkt. Die Verankerung ist kein Detail: `installer/cmd/k-playbook/` ist Code dieses
Projekts, und ein Ausschluss über den bloßen Namen fräße ihn mit — derselbe Fehler wie in
Task 004, wo ein Muster ohne Anker die ganze Projektwurzel traf. Die Liste steht im Code
([`installer/internal/review/candidates.go`](../installer/internal/review/candidates.go)),
aus demselben Grund wie die der Modulsuche, und sie ist nicht dieselbe: die Modulsuche
fragt, wo ein Modul des Projekts liegt, die Zählung, was ein Werkzeug hätte sehen können —
`vendor/`, `node_modules/` und `testdata/` zählen deshalb mit.

**Die Zahl ist eine Obergrenze, keine Abdeckungsmessung.** Die werkzeugeigenen
Ausschlüsse stehen in `args`, und jedes Werkzeug schreibt sie anders; die Zählung kennt
sie nicht. Es gilt deshalb nur: Kandidaten ≥ tatsächlich geprüfte Dateien. `0` heißt
sicher „nichts zu tun", eine hohe Zahl heißt „hier hätte etwas sein können".

Das Feld fehlt, wo nicht gezählt wurde: bei einem `skipped`-Job, bei der Sorte `none` und
dann, wenn der Baumlauf selbst gescheitert ist. Ein Fehler dabei macht **keinen** Job zum
Fehlschlag — die Zählung ist eine Zusatzauskunft, kein Ergebnis. Ein `failed`-Job trägt
die Zahl dagegen, wenn für seinen Bezugspunkt gezählt wurde: sein Fehlschlag sagt über den
Gegenstand nichts aus.

`k-playbook scan` nennt die Zahl nur bei 0 Befunden — dort trennt sie „nichts zu prüfen"
von „nichts geprüft", sonst ist sie Rauschen:

```text
  ruff             ruff             fertig, 0 Befunde bei 12 Kandidaten → raw/ruff.sarif
```

**Geschrieben wird schon beim Start**, mit dem Zustand `running`, und danach bei jeder
Zustandsänderung eines Jobs. Läge die Datei erst am Ende, wäre der Fortschritt während
des Laufs nicht lesbar, und der abgeleitete Laufzustand spränge von `created` direkt auf
`done`.

**Wiederholbar.** Ein zweiter Aufruf über denselben Eintrag überschreibt seine Dateien,
statt danebenzuschreiben oder abzubrechen. Überschreiben allein genügt dafür aber nicht:
weil Job-Namen am Modulbestand hängen können, räumt der Eintrag vorher weg, was er beim
vorigen Aufruf unter `raw/` geschrieben hat — auch dann, wenn diesmal kein einziger Job
läuft. Sonst bliebe `raw/govulncheck.sarif` liegen, sobald ein zweites Modul den Job in
`govulncheck-installer` umbenennt, und gälte weiter als Ergebnis. Welche Dateien es sind,
sagt die vorige `entries/<tool>.json` — und zwar über den **Job-Namen**, nicht über den
`sarif`-Pfad: `raw/<job>.sarif` ist die Namensregel, und der Name steht dort bei jedem
Ausgang. Deshalb gilt die Zusage ohne Ausnahme, auch für die Datei eines Jobs, der beim
vorigen Aufruf gescheitert ist. Fehlt die Datei, greift die Namensregel (der Job-Name
beginnt mit dem Tool-Namen) als Rückfall, beschränkt auf `*.sarif`; dieselbe Regel prüft
auch jeden gelesenen Job-Namen, bevor er ein Löschen steuert. Die Dateien anderer Einträge
bleiben stehen.

### AI-Entry-Status

AI-Einträge schreiben ihre eigene Datei ebenfalls unter `entries/<name>.json`, aber mit
einem schlanken Schema. Eine Perspektive meldet ihre Ergebnisdatei:

```json
{
  "name": "secret-scanning",
  "kind": "ai",
  "state": "done",
  "result": "review-secret-scanning.md",
  "reason": "",
  "startedAt": "2026-08-19T10:00:00Z",
  "finishedAt": "2026-08-19T10:15:00Z"
}
```

`result` ist relativ zum Laufverzeichnis. Wenn `resultRequired` in der kopierten
`run.json`-Metadatenstruktur `true` ist, braucht `state: done` dieses Ergebnis und die
Datei muss existieren. `failed` und `skipped` brauchen einen `reason`; `running` darf kein
`finishedAt` tragen.

Eine Evidence-Quelle meldet stattdessen ihr Artefakt, und zwar als `jobs`:

```json
{
  "name": "tech",
  "kind": "ai",
  "state": "done",
  "reason": "3 Funde außerhalb von scope.paths verworfen: docs/review-runs.md, …",
  "startedAt": "2026-08-19T10:00:00Z",
  "finishedAt": "2026-08-19T10:15:00Z",
  "jobs": [
    { "job": "tech", "state": "done", "sarif": "raw/tech.sarif", "findings": 17,
      "started": "…", "finished": "…" }
  ]
}
```

`result` bleibt leer — Pflichtartefakt ist das SARIF. `jobs` ist dieselbe Darstellung wie
bei Tool-Einträgen und keine zweite daneben: der Merge liest beide über denselben Weg, und
ein eigenes Job-Format wäre dort unsichtbar. Der Job heißt wie der Eintrag; `findings` ist
die Zahl der **übernommenen** Funde. Das Feld fehlt, wo es keinen Job gibt — die Dateien
der Perspektiven behalten damit genau die Form, die sie vorher hatten, und Dateien aus der
Zeit vor `jobs` bleiben lesbar.

## Die Oberfläche

Der Block **Workflows** auf der Startseite führt mit einem Knopf zur Seite `/reviews`; die
Zahl darauf ist die der vorhandenen Läufe. Aufgelistet werden sie auf der Seite selbst.
Dort wird auch ein neuer Lauf zusammengestellt:

- **Werkzeuge** aus der Tool-Matrix, gefiltert nach `project.languages`. Was nicht
  installiert ist, steht da, lässt sich aber nicht auswählen — mit dem Hinweis, wie es
  installiert wird.
- **Reviews** aus dem aufgelösten Katalog, mitgeliefert und projekteigen zusammengeführt.
  Abgeschaltete und nicht audit-aktivierte Einträge (`audit.enabled: false`) fehlen.

„Erstellen" legt das Verzeichnis und `run.json` an. Mehr nicht: **das Anlegen startet
nichts.** Gestartet wird im Terminal, mit `k-playbook scan` — einen Knopf dafür gibt es
in der Oberfläche bewusst noch nicht.

## MCP-Werkzeuge

Der MCP-Server bietet dieselbe Fachlogik maschinenlesbar an. Die CLI bleibt der manuelle
Weg; MCP ist für die Chat-Orchestrierung ohne Shell-Out zur `k-playbook`-CLI gedacht.

| Werkzeug | Zweck | CLI-Äquivalent |
|---|---|---|
| `k_playbook_review_status` | Auswahlbasis oder bestehenden Laufstatus lesen | keines, nahe an Oberfläche `/reviews` |
| `k_playbook_review_create` | Lauf anlegen oder Dry-Run der `run.json`-Struktur erzeugen | Oberfläche „Erstellen" |
| `k_playbook_review_scan` | Tool-Einträge blockierend ausführen | `k-playbook scan <lauf>` |
| `k_playbook_review_merge` | `review-input.json` und `review-input.md` schreiben | `k-playbook merge <lauf>` |
| `k_playbook_review_write_ai_entry` | Status eines AI-Eintrags schreiben | kein CLI-Äquivalent |

Alle Werkzeuge verlangen `projectDir`, suchen von dort aufwärts nach `K-PLAYBOOK.yaml` und
geben strukturierte JSON-Hüllen zurück. Fehler sind fachliche Werkzeugergebnisse mit
`ok: false`, keine MCP-Protokollfehler.
Im Modus `available` liefert `k_playbook_review_status` neben Werkzeugen und
Katalog-Rezepten auch den Command-Moduleintrag `scan-triage`, sofern er im effektiven
Command-Namensraum aktiv ist. `k_playbook_review_create`,
`k_playbook_review_status` im bestehenden Lauf und
`k_playbook_review_write_ai_entry` akzeptieren diesen Eintrag, obwohl er nicht in
`catalogs.reviews` steht.

### Ergebnis eines AI-Eintrags melden

`k_playbook_review_write_ai_entry` nimmt `run`, `entry`, `state` und wahlweise `result`,
`reason`, `startedAt` und `finishedAt`. Für eine Evidence-Quelle kommt `job` dazu:

```json
{
  "projectDir": "…",
  "run": "2026-08-19",
  "entry": "tech",
  "state": "done",
  "job": {
    "sarif": "raw/tech.sarif",
    "started": "2026-08-19T10:00:00Z",
    "finished": "2026-08-19T10:15:00Z"
  }
}
```

`job.sarif` liegt relativ zum Laufverzeichnis unter `raw/`; `started` und `finished` sind
optional. Zustand, Fundzahl und Job-Name entstehen beim Melden und werden nicht
entgegengenommen. Der Job gehört zur **Fertigmeldung**: bei `state: running` wird er
abgewiesen, und für `mode: perspective` gibt es ihn nicht.

Die Antwort ist zu lesen, statt `done` zu unterstellen. Sie trägt unter `evidence` die
Zahl der übernommenen Funde (`findings`), die verworfenen (`droppedFindings`,
`droppedPaths`) und ob das SARIF bereinigt zurückgeschrieben wurde (`sarifRewritten`).
`stateOverridden: true` neben `requestedState: done` heißt: das Artefakt war ungültig,
geschrieben wurde `failed` mit Grund.

Zwei Fehlerklassen bleiben getrennt. **Fehler des Aufrufs** — SARIF-Pfad außerhalb von
`raw/`, Datei fehlt oder ist leer, `result` an einem Evidence-Eintrag, Job bei
`state: running`, Rezept ohne gültigen Evidence-Vertrag — schreiben **nichts**; korrigiert
wird der Aufruf. **Fehler des Artefakts** — unlesbares SARIF, falscher
`tool.driver.name`, fremde Rule-ID — schreiben `failed` mit Grund; nur ein erneuter
Rezeptlauf behebt sie.

### Aktualität der Bewertung

Der Laufstatus meldet neben den Einträgen einen Block `triage`:

```json
{
  "triage": {
    "result": "review-triage.md",
    "state": "stale",
    "finishedAt": "2026-08-19T11:00:00Z",
    "reviewInputModified": "2026-08-19T12:30:00Z",
    "reason": "review-input.json ist jünger als die Bewertung — der Merge lief danach."
  }
}
```

`state` ist `missing`, `current` oder `stale`. Der Block steht **neben** dem Zustand des
Eintrags `scan-triage` und ersetzt ihn nicht: der Eintrag sagt, ob die Bewertung
geschrieben wurde, dieser Block, ob sie noch gilt. Nötig ist er, weil die Reparaturprüfung
an `review-triage.md` nur Existenz und Größe misst — ein erneuter Merge, der reguläre Weg
bei nachträglich eintreffender Evidence, ließe den Eintrag sonst `done` und konsistent,
obwohl die Bewertung einen Stand beschreibt, den es nicht mehr gibt.

Verglichen wird die Änderungszeit von `review-input.json` mit `finishedAt` des Eintrags
`scan-triage`. Gleiche Zeiten gelten als aktuell. Nicht belegbare Fälle — `review-input.json`
fehlt, der Eintrag nennt keine oder keine lesbare Endzeit — gelten als `stale` und nie als
`current`: eine unbelegte Bewertung darf einen Lauf nicht vollständig aussehen lassen. Ein
Lauf mit `state: stale` ist nicht fertig; der Triage-Schritt läuft erneut.

## Zusammenfassen mit `k-playbook merge`

Wenn ein Lauf durch ist, verdichtet ein zweiter Schritt seine Rohdaten zu einem
kuratierbaren Review-Input:

```text
k-playbook merge <lauf>
```

Das Kommando liest `run.json`, `entries/*.json` und die `raw/*.sarif` der `done`-Jobs,
normalisiert die Findings, gruppiert erkennbare Dubletten und schreibt zwei Artefakte
neben die Rohdaten:

```text
k-playbook-local/results/<lauf>/
├── review-input.json    vollständiger Audit-Beleg, JSON
└── review-input.md      kompakte Ansicht, Markdown
```

**Was das Kommando ausdrücklich nicht tut.** Es fasst keine Rohdateien an: `raw/*.sarif`,
`run.json` und `entries/*.json` bleiben unverändert. Es bewertet auch nicht — dafür ist
der Assistent zuständig, der `review-input.json` als Eingabe bekommt.

**Dedupe-Modell.** Findings werden nie stumm entfernt. Fünf Regeln fassen zusammen:

- gleiche `fingerprint` oder `partialFingerprint` innerhalb vergleichbarer Quelle,
- gleiche Datei, Zeile, Regel-ID und normalisierte Message,
- **eine** gemeinsame Dependency-ID (CVE/GHSA/OSV/PYSEC) plus gleiches Package und
  gleiche Version,
- bei Nicht-Dependency-Funden (`same-location-tool`): dasselbe Werkzeug im selben Job an
  derselben Datei und Zeile,
- bei KI-Evidence (`ai-path-rule`): gleiches Rezept, gleiche Datei und gleiche Regel-ID —
  ohne Zeile und ohne Message.

Die vierte Regel greift für Dependency-Funde **nicht**. Bei Manifest-Funden zeigt jeder
Fund eines Werkzeugs auf dieselbe Zeile der Manifest-Datei; die Regel gruppierte dort
nach Fundort statt nach Identität und legte durchweg verschiedene Schwachstellen
zusammen — in einem gemessenen Lauf 18 verschiedene GHSA in einer Gruppe. Für Funde ohne
erkannte Dependency bleibt sie unverändert: dort ist dieselbe Zeile eine Aussage über den
Fund.

Die fünfte Regel gehört zur Schlüsselklasse `ai` und ist beabsichtigt: zwei KI-Funde
derselben Rule-ID in derselben Datei sind eine Gruppe. Alle Instanzen bleiben als
`findingIds` und `evidence` erhalten, der Repräsentant nennt aber nur **eine** Stelle —
die Zahl ist mitzulesen. Der Grund liegt in der Stabilität: der stabile Schlüssel einer
KI-Gruppe enthält weder Zeile noch Meldung, damit eine verschobene Zeile oder ein
umformulierter Text die Gruppen-ID nicht ändert. Ohne die vierte Regel bekämen zwei Funde
derselben Rule-ID in derselben Datei zwei Gruppen mit demselben Schlüssel — also
kollidierende `stableId`s. Eine Decision auf eine solche Gruppe deckt Datei und Rule-ID
ganz; eine Abweisung je Instanz gibt es nicht.

Bei der Dependency-Regel zählt jede Kennung einzeln: zwei Werkzeuge finden zusammen,
sobald sie sich eine Kennung teilen, auch wenn das eine drei Aliase nennt und das
andere nur einen. In den Schlüssel gehen dabei nur Kennungen, die den Fund benennen —
`ruleId` und benannte Alias-Felder —, nicht solche, die im Advisory-Text nebenbei
vorkommen. Das Manifest steht **nicht** im Schlüssel: Werkzeuge schreiben denselben
Pfad zu verschieden (`requirements.txt`, `/requirements.txt`,
`file:///abs/pfad/requirements.txt`). Der Preis ist im Monorepo sichtbar — gleiches
Paket mit gleicher CVE unter `services/a/` und `services/b/` wird eine Gruppe. Die
Version bleibt dagegen im Schlüssel: die Werkzeuge, die überhaupt eine strukturierte
Angabe machen, schreiben sie gleich.

**Woher Package und Version kommen — und woher nicht.** In den harten Schlüssel darf nur,
was **strukturiert** gelesen wurde. Zwei Quellen zählen dazu:

- eine benannte Property des Results oder der Rule (`package`, `packageName`, `version`,
  `installedVersion`, …) — so schreibt es pip-audit,
- ein purl in `purls` oder `purl`, auch als JSON-Array — so schreibt es grype
  (`pkg:pypi/requests@2.19.0`). Der Ecosystem-Teil fällt weg, der Namensteil bleibt
  vollständig, damit ein Go-Modulpfad wie `golang.org/x/sys` nicht verstümmelt wird.

Nennt ein Werkzeug Paket und Version nur im Fließtext, wird der Wert gelesen, landet aber
in `textPackage` / `textVersion` und **nicht** im harten Schlüssel. So ist es bei
osv-scanner (`Package 'requests@2.19.0' is vulnerable to …`) und bei trivy
(`Package: requests` / `Installed Version: …`). Der Grund ist die Trennschärfe: Paket und
Version sind im Schlüssel das Einzige, was dieselbe Kennung in zwei verschiedenen Paketen
auseinanderhält (vendored libs); ein danebenliegender Textwert verschmelzte genau die
Befunde, die getrennt bleiben müssen. Anders als bei den Kennungen gibt es hier deshalb
**keinen** Rückfall von der engen auf die breite Seite — ohne strukturierten Wert bleibt
der harte Schlüssel aus, und die Funde von osv-scanner und trivy stehen als
`possible-duplicate` daneben.

Zwei Fälle werden nicht zusammengefasst, sondern wechselseitig als
`possible-duplicate` markiert:

- gleiche Datei und Zeile mit ähnlicher Regel-Familie,
- gemeinsame Dependency-ID, aber Package oder Version fehlt oder weicht ab — oder eine
  Seite nennt gar kein Package und hat damit keinen harten Schlüssel.

Der Assistent entscheidet, ob er die Belege bündelt.

**Zwei Artefakte, klare Rollen.** `review-input.json` trägt Provenienz und alle Belege —
deshalb darf es groß werden. Welche Felder es hat, steht in
`commands/_review-run/review-input-contract.md` und nur dort.
`review-input.md` bleibt bewusst kompakt:
Gruppen werden gebündelt, die Detailliste ist hart gedeckelt und verweist auf die
JSON, sobald sie das Limit überschreitet.

**Wann `schemaVersion` steigt.** Nur bei **brechenden** Änderungen: ein Feld entfällt,
wird umbenannt, wechselt den Typ, oder die Bedeutung eines bestehenden Feldes ändert sich
so, dass ein Leser der alten Fassung es falsch versteht. Ein **neues** Feld lässt sie
stehen. Der Grund ist die Rolle der Zahl: sie sagt einem Leser, ob er die Datei noch so
verarbeiten darf wie bisher — und das darf er, denn alle Felder sind optional, der
Vertrag hält für jedes fest, was gilt, wenn es fehlt, und ein Leser überspringt, was er
nicht kennt. Stiege sie bei jedem Feld, wäre sie ein Änderungszähler, und jeder Konsument
müsste seine Prüfung nachziehen, ohne dass sich für ihn etwas geändert hätte.

**Geänderte Werte sind keine Schemaänderung.** Verschieben sich die `stableId` — siehe
„Stabile Gruppen-IDs hängen an einer eingefrorenen Pfadnormierung" weiter unten —, bleibt
`schemaVersion` stehen: das Feld heißt weiter dasselbe und bedeutet weiter dasselbe, nur
sein Wert wird anders gerechnet. Wer wissen
muss, mit welcher Fassung ein Beleg entstanden ist, liest `kPlaybookVersion`; die steht
genau dafür in jedem Merge-Artefakt. Die Zahl steht derzeit auf `1` und ist seit ihrer
Einführung nicht gestiegen — auch nicht für die Dependency-Felder, die seither
hinzugekommen sind.

**Wiederholbar.** Ein erneuter Aufruf überschreibt beide Artefakte. Bei fehlenden
Entry-Dateien für einen in `run.json` ausgewählten Eintrag gilt der Zustand `start`;
kein Fehler, sondern eine sichtbare Auskunft im Statusblock.

### Stabile Gruppen-IDs hängen an einer eingefrorenen Pfadnormierung

Ein Pfad geht an vier Stellen in Schlüssel und Klasse einer Gruppe ein. Diese vier
Stellen rufen **nicht** dieselbe Normierung wie die Gruppierung, sondern eine
eigene, festgeschriebene Kopie (`stablePath` in `merge/stable_path.go`). Die
Gruppierungs-Normierung (`pathnorm.Normalize`, geteilt mit `knowndecisions`) darf sich
weiterentwickeln, ohne die IDs zu bewegen.

**Warum getrennt.** Genau das ist einmal schiefgegangen. Beim Umbau der
Dedupe-Schlüssel wurde die Gruppierungs-Normierung verbessert — Backslashes, `file://` samt
Authority, `.`/`..`, doppelte Slashes, führendes `/` —, und weil die ID-Bildung
dieselbe Funktion rief, verschoben sich die Stable-IDs als unbeabsichtigter
Nebeneffekt. Die Messung an einem Lauf mit 74 Gruppen: **38 Gruppen verschoben,
alle 38 allein durch die Pfadform**, in jedem einzelnen Fall durch das entfernte
führende `/` (`/dist/…` → `dist/…`). Betroffen waren grype und osv-scanner
vollständig, pip-audit und trivy gar nicht — wer relative Pfade schreibt, merkte
nichts. Eine Verschiebung, die niemand wollte, die nirgends fehlschlug und die
jeden `stableId`-Eintrag in `known-decisions.md` still ins Leere laufen ließ.

Eine Stable-ID ist der Bezugspunkt, unter dem eine Triage-Entscheidung einen Lauf
überdauert. Sie darf sich ändern, wenn sich der Befund ändert — nicht, wenn ein
Werkzeug seine Pfade anders schreibt oder die Normierung dazulernt. Der Preis der
Trennung ist eine doppelt gepflegte Funktion; er ist niedriger als eine Kennung,
die bei jeder Verbesserung an anderer Stelle bricht.

**Warum nicht die alte Normierung zurück.** Sie hätte die IDs von vor dem Umbau
nicht wiedergebracht. Zwischen damals und heute liegt eine zweite Verschiebung —
gefülltes Paket aus purl-Quellen und die geänderte Gruppenzusammensetzung — die
den Pfadanteil überlagert: von 74 Gruppen tragen am Ende noch 12 ihre alte ID, und
davon bringt eine Rücknahme der Pfadform keine zurück. Zurück käme allein die
schlechtere Normierung, dauerhaft eingefroren in der ID-Bildung, während die
Gruppierung die bessere benutzt. Eine Migrationstabelle wäre ebenfalls nur ein
Stichtagsdokument gewesen; sie wird mit der nächsten Verschiebung wertlos, und
es gibt keinen Bestand, der sie rechtfertigte.

**Was das für `known-decisions.md` heißt.** Einträge mit dem Kriterium `stableId` sind
**einmal** neu abzuleiten: aus einem Lauf, der mit dieser Fassung zusammengeführt wurde.
Der einfachste Weg ist, den betroffenen Lauf neu zu mergen und die `stableId` der Gruppe
aus `review-input.json` oder `review-input.md` zu übernehmen. Die vier Änderungen —
erweiterte Pfadnormierung, Dedupe-Schlüssel je Kennung, nachgeholtes Paket samt
geänderter Gruppenzusammensetzung und die Einengung auf die enge Kennungsmenge — sind
bewusst in **einem** Schritt zusammengelegt, damit diese Ableitung einmal anfällt und
nicht viermal. Einträge mit `pathGlob`, `ruleId` oder `fingerprint` sind nicht betroffen
— sie matchen nicht über die ID. Wer eine Decision langfristig halten will, ist mit
diesen Kriterien besser bedient.

**Eine Kennung mehr verschiebt die ID nicht mehr.** Präfix und `dependencies`-Zeile des
Schlüssels entstehen aus der **engen** Kennungsmenge — den Kennungen, die den Fund selbst
benennen (`ruleId` und benannte Alias-Felder) —, nicht aus allen Kennungen, die im
Advisory-Text vorkommen. Vorher genügte eine beiläufig genannte Fremd-Kennung, um die ID
zu verschieben, und sie konnte sogar das Präfix bestimmen: im Messlauf hieß die
pyyaml-Gruppe um CVE-2019-20477 `scan-cve-cve-2017-18342-…`, nach einem anderen
pyyaml-Advisory, das der Beschreibungstext nur erwähnt. Nennt ein Werkzeug seine einzige
Kennung ausschließlich im Freitext, fällt die Bildung auf die breite Menge zurück — sonst
verlöre so ein Fund Präfix und Klasse ganz.

**Was die Entkopplung nicht leistet.** Sie deckt die Pfadform ab, nicht die
Gruppenzusammensetzung. Drei Restinstabilitäten bleiben, und sie sind bewusst in
Kauf genommen:

- Der Schlüssel entsteht über **alle** Findings einer Gruppe — Werkzeuge, Jobs,
  Fundorte, Regeln, Meldungen, Fingerprints. Kommt ein Fund hinzu, weil ein
  weiteres Werkzeug denselben Befund meldet oder dieselbe Sache an zweiter Stelle
  auftaucht, ändert sich die ID. Das ist der Regelfall bei jeder Änderung am
  Werkzeugsatz eines Laufs.
- Trägt eine Gruppe keine Kennung in der engen Menge, fällt die ID-Bildung auf
  die breite zurück. Für solche Gruppen hängt die ID weiter an der vollständigen
  Aliasliste und bricht, sobald ein Werkzeug eine Kennung mehr nennt.
- Auch mit enger Menge hilft die Einengung nur, wenn die zusätzliche Kennung
  **außerhalb** davon auftaucht. Nennt ein Werkzeug sie in einem benannten
  Alias-Feld — pip-audit schreibt seine Aliase genau dorthin —, zählt sie zur
  engen Menge und die ID verschiebt sich trotzdem.

Wer diese Fälle ausschließen muss, benutzt in `known-decisions.md` `pathGlob`
oder `ruleId` statt `stableId`.

**Wann sich das wieder ändern darf.** `stablePath` wird nicht nebenbei
angefasst. Eine Änderung dort verschiebt jede Stable-ID, deren Pfad davon berührt
wird, und entwertet die zugehörigen Decisions. Sie ist eine eigene Entscheidung,
die hier dokumentiert und in `commands/_review-run/review-input-contract.md`
nachgezogen wird; ein Test im Merge-Paket hält den Stand fest und schlägt fehl,
wenn jemand sie unbemerkt verschiebt.

## Wirkung von `known-decisions.md`

`k-playbook-local/known-decisions.md` hält fest, welche Befunde bewusst nicht mehr als
Befund gelten sollen: Falschpositive, akzeptierte Risiken, Zurückgestelltes, Verworfenes.
Sie ist eine handgepflegte **Eingabe** und kein Review-Ergebnis — deshalb liegt sie
direkt in `k-playbook-local/`, neben `rules/`, `reviews/` und `guidelines/`, und nicht in
`results/`. Kein Command legt sie an; sie wird von Hand geschrieben und ist optional.

Bewusste Entscheidungen werden vor der Bewertung zentral im Merge-Schritt angewendet. Das
Matching passiert also nicht im Chat: `k-playbook merge` lädt `known-decisions.md`, markiert
gedeckte Findings in `review-input.json` und schreibt dieselbe Information in die
Gruppen- und Markdown-Sicht. Rohdaten, `run.json` und `entries/*.json` bleiben dabei
unverändert.

Format je Eintrag:

````markdown
## kd-beispiel

```yaml
id: kd-beispiel
category: wontfix
expires: 2026-11-30
owner: team
match:
  - pathGlob: _old/**
```

Begründung als Fließtext.
````

Der `##`-Header muss der `id` entsprechen. Pflichtfelder sind `id`, `category` und
`match`; erlaubte Kategorien sind `false-positive`, `accepted-risk`, `deferred` und
`wontfix`. `expires` ist ein ISO-Datum; abgelaufene Decisions werden sichtbar gemeldet,
aber nicht angewendet.

Unterstützte Match-Kriterien:

- `stableId` für eine stabile Gruppe aus `review-input.json`.
- `ruleId` plus `location`; `ruleId` allein ist verboten.
- `cveId`, `ghsaId` oder `osvId` plus Scope über `package`, `version`, `manifestGlob` oder
  `stableId`.
- `pathGlob` für ganze Bäume wie `_old/**`.

Pfade werden als projektrelative Slash-Pfade ohne führendes `./` verglichen. Bei mehreren
Locations reicht ein Treffer; der Match-Report nennt die getroffene Location.

**Es ist dieselbe Normierung, mit der der Merge gruppiert.** Muster und Fundort laufen
beide durch sie: Backslashes werden zu `/`, ein `file://`-Präfix fällt samt Rechnername
weg, `.`- und `..`-Segmente und doppelte Slashes werden aufgelöst, das führende `/`
entfällt, verglichen wird in Kleinschreibung. Vorher hatten Merge und Matching je eine
eigene Kopie, und die waren auseinandergelaufen — eine Decision traf dann nur einen Teil
einer Gruppe, die der Merge über genau diese Schreibweisen zusammengezogen hatte, und das
zeigte sich nur als Teildeckung. Geteilt wird ausdrücklich diese eine Frage; die
Normierung der Stable-IDs ist eine andere und eingefroren (siehe oben).

Welches Kriterium wofür taugt — der Unterschied ist größer, als die Liste vermuten lässt:

- **`stableId`** deckt genau eine Gruppe und ist der Regelweg. Für KI-Evidence gilt das
  besonders: die Gruppen-ID einer KI-Gruppe hängt nur an Rezept, Rule-ID und Datei, bleibt
  also über Re-Runs stabil, auch wenn sich Zeilen verschieben oder der Text sich ändert.
- **`ruleId` plus `location`** ist der brauchbare zweite Weg. `location` ist dabei ein
  **Pfad-Glob und keine Zeile**: verglichen wird der Dateipfad des Funds, Zeile und Spalte
  gehen nicht ein. `ruleId: tech-magic-value` mit `location: installer/**` deckt damit
  diese eine Regel in diesem einen Baum — und nichts sonst. Genau deshalb ist `ruleId`
  ohne `location` verboten: allein deckte es die Regel im ganzen Projekt.
- **`pathGlob`** ist die grobe Ausnahme für ganze Bäume, die nicht mehr gepflegt werden.
  Er trifft **jeden** Fund an diesem Pfad — auch Scanner-Funde fremder Werkzeuge und
  fremder Regeln, die es dort heute noch gar nicht gibt. Wer `pathGlob: services/legacy/**`
  schreibt, schaltet dort auch den nächsten Secret-Fund stumm. Diese Nebenwirkung ist der
  Grund, ihn nicht als Ersatz für die beiden anderen Wege zu benutzen.

Ort:

- `k-playbook-local/known-decisions.md` — die eine projektweite Datei. Es gibt keinen
  zweiten Suchpfad und keine laufspezifische Fassung: eine bewusste Entscheidung ist
  nicht an einen Lauf gebunden, dafür gibt es `expires`.
- Wenn die Datei nicht existiert, meldet der Merge sichtbar „keine known-decisions geladen".
- Übergang: Projekte, die die Datei noch unter `k-playbook-local/results/` liegen haben,
  werden von dort weiter gelesen — der Merge meldet dann den Umzug als Hinweis, in
  `review-input.md`, in der CLI-Ausgabe und im Ergebnis des MCP-Werkzeugs
  `k_playbook_review_merge`. Liegen beide vor, gewinnt der neue Ort und der alte wird als
  ignoriert gemeldet. Diese Lesung des alten Orts ist befristet und wird wieder entfernt.

JSON-Wirkung:

- Gedeckte Findings tragen `coveredByKnownDecision` mit `id`, `category` und `matchedBy`.
- Gruppen tragen `coveredByKnownDecision` nur, wenn alle Findings derselben primären
  Decision unterliegen.
- Teils gedeckte Gruppen tragen `partialCoverage: true` und `knownDecisionCoverage` mit
  IDs, Kategorien und Finding-Zahlen.
- Der Metablock `knownDecisions` nennt die Quelle, geladene IDs, Herkunft, Ablauf,
  `applied`, `notAppliedReason` und Loader-Warnungen.

`review-input.md` zeigt die Anzahl vollständig und teilweise gedeckter Gruppen, eine
Known-Decision-Spalte in der Gruppentabelle, abgelaufene Decisions als eigenen Hinweis und
die Loader-Warnungen unter „Hinweise zu Known-Decisions".
Das Bewertungsmodul `review-scan-triage` liest diese Deckung anschließend ausschließlich
aus `review-input.json`.

## Bewerten mit `review-scan-triage`

Nach dem Merge bewertet `/k-audit` den Lauf über das Command-Modul
`commands/_audit/review-scan-triage.md`. Das Modul ist kein Review-Rezept und
wird nicht aus `reviews/` geladen. Es liegt im Namensraum `_audit/`, weil die Konstante
`scanTriageModule` und der Eintragsname `scan-triage` an diesem Pfad hängen; audit-exklusiv
ist es nicht. Im Lauf wird es als AI-Eintrag `scan-triage` geführt. Der
`/k-review`-Report-Modus wendet denselben Wortlaut an, schreibt aber keinen
MCP-Entry-Status.

Eingaben sind ausschließlich Dateien im Laufkontext: `review-input.json` als Beleg und
`review-input.md` als kompakte Ansicht. Was im Beleg steht — Kern, merge-only-Teil und
die Bildung der stabilen Gruppen-IDs —, beschreibt
`commands/_review-run/review-input-contract.md`.

Das Modul schreibt genau ein neues Markdown-Artefakt direkt in den Laufordner:

```text
k-playbook-local/results/<lauf>/review-triage.md
```

`review-triage.md` bündelt Gruppen nach gemeinsamer Ursache, vergibt Priorität
`P1`/`P2`/`P3`, Kategorie `S`/`T`/`K`/`F`/`A`/`X`, verweist auf die stabilen
Gruppen-IDs und nennt den nächsten Schritt je Bündel. Gruppen, die durch
`known-decisions.md` gedeckt sind, bleiben sichtbar und werden als gedeckt markiert; die
Zuordnung kommt ausschließlich aus `review-input.json`.

Nach dem Schreiben setzt `/k-audit` den AI-Eintrag mit
`k_playbook_review_write_ai_entry` auf `done` und `result: review-triage.md`.
Das Werkzeug schreibt dabei nur `entries/scan-triage.json`; der Markdown-Inhalt
bleibt ein direktes Artefakt im Laufordner. Ein `done`-Status ist nur konsistent,
wenn der relative Ergebnispfad im Laufordner bleibt und die Datei existiert.
