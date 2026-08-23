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
        └── gitleaks-git.sarif
```

Der Name ist das Datum, `YYYY-MM-DD`. **Existiert das Verzeichnis bereits, bricht das
Anlegen ab** statt einen zweiten Lauf danebenzustellen: ein Tag, ein Lauf. Wer erneut
starten will, räumt das vorhandene Verzeichnis weg oder benennt es um.

**`raw/` legt der Ausführer an**, beim ersten Job, den er startet. Das Anlegen eines
Laufs kennt das Verzeichnis nicht: ein Lauf ohne Werkzeug-Eintrag braucht es nicht.

**Es gibt zwei Orte für Rohdaten.** Die Ergebnisfamilien unter
`k-playbook-local/results/<familie>/YYYY-MM-DD/raw/` bleiben, wie sie sind, und
`/k-results` liest weiter sie; `ListRuns()` zeigt sie ohnehin mit an. Das
Laufverzeichnis ist dagegen familienlos, weil ein Lauf gerade über die Familien hinweg
klammert. Katalog-Perspektiven im Laufmodell legen keine eigene Rohdatenablage an; sie
lesen den gemeinsamen Merge-Beleg aus diesem Laufordner.

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
      "name": "tech",
      "kind": "ai",
      "state": "start",
      "recipeKey": "tech",
      "recipePath": "/projekt/k-playbook/reviews/review-tech.md",
      "recipeOrigin": "dist",
      "title": "Technischer Review",
      "resultRequired": true,
      "defaultResult": "review-tech.md",
      "scope": {
        "tools": ["semgrep", "gosec"]
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
  title: "Technischer Review"
  resultRequired: true
  defaultResult: "review-tech.md"
  scope:
    tools: [semgrep, gosec]
review:
  enabled: true
---
```

`audit.enabled` ist standardmäßig `false`; nur `true` nimmt das Rezept in `/k-audit`-/MCP-Läufe auf.
`review.enabled` ist standardmäßig `true`; `false` entfernt das Rezept aus der `/k-review`-Auswahl.
`title` fällt ohne Angabe auf die erste Überschrift oder den Katalog-Schlüssel zurück.
`resultRequired` ist standardmäßig `true` und bestimmt, ob ein `done`-Status ein Ergebnis
braucht. `defaultResult` ist ein relativer Vorschlag im Laufverzeichnis.
`scope.tools` ist der beim Anlegen eingefrorene Tool-Scope des AI-Eintrags. Spätere
Rezeptänderungen ändern bestehende Läufe nicht.

Der Moduleintrag `scan-triage` erhält dieselben Laufmetadaten aus dem effektiven
Command-Namensraum: `recipePath` zeigt auf
`commands/_audit/review-scan-triage.md`, `defaultResult` ist
`review-triage.md`, `resultRequired` ist `true`. Ein leeres lokales Overlay unter
`k-playbook-local/commands/_audit/review-scan-triage.md` schaltet diesen
Eintrag ab.

## Katalog-Rezepte als Perspektiven

Aktive Katalog-Rezepte laufen im Audit-Laufmodell nicht als eigene Scanner. Sie sind
Perspektiven auf den Merge-Output `review-input.json` und schreiben genau eine
Markdown-Datei direkt in den Laufordner, z. B. `review-secret-scanning.md`. Das
vollständige Beispiel für diesen Vertrag steht im Rezept
[`review-secret-scanning.md`](../reviews/review-secret-scanning.md).

Frontmatter-Vertrag:

```yaml
---
name: review-<key>
title: <Titel>
audit:
  enabled: true
  defaultResult: review-<key>.md
  resultRequired: true
  scope:
    tools: [<tool>, <tool>]
review:
  enabled: true
---
```

Der `audit`-Block ist optional. Fehlt er, bleibt das Rezept im Audit-Laufmodell inaktiv.
`audit.enabled: false` deaktiviert nur die `/k-audit`-/MCP-Auswahl; `review.enabled`
steuert weiterhin die gezielte `/k-review`-Auswahl.

Reihenfolge in `/k-audit`:

1. Tool-Einträge laufen und schreiben `entries/<tool>.json` sowie `raw/<job>.sarif`.
2. Der Merge schreibt `review-input.json` und `review-input.md` aus den Tool-Einträgen.
3. Aktive Katalog-Rezepte lesen `review-input.json` als Perspektiven.
4. Optional läuft danach `scan-triage` und darf Perspektiven-Reports als Kontext nutzen.

Offene, fehlgeschlagene oder noch nicht ausgeführte Perspektiven blockieren den Merge
nicht. Sie können den AI-Teil offen lassen, aber `review-input.json` muss aus den
Tool-Scans erzeugbar bleiben.

Scope-Semantik:

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

Reparaturmatrix für AI-Einträge:

| Zustand | Verhalten |
|---|---|
| Eintrag existiert in `run.json`, Ergebnisdatei fehlt, Entry ist offen | Status bleibt offen; Rerun führt den AI-Eintrag erneut aus und schreibt die Ergebnisdatei. |
| Eintrag existiert in `run.json`, Ergebnisdatei fehlt, Entry steht auf abgeschlossen | Statusausgabe markiert den Eintrag als inkonsistent und `resultRequired` nicht erfüllt; Rerun schreibt die Ergebnisdatei neu und repariert den Status. |
| Eintrag existiert in `run.json`, Ergebnisdatei existiert, Entry-Status fehlt oder ist offen | Statusausgabe darf den Eintrag als reparabel markieren; Rerun muss keine neue Datei erzwingen, sondern darf den Status über `k_playbook_review_write_ai_entry` auf abgeschlossen setzen, wenn die Datei nicht leer ist. |
| Eintrag existiert in `run.json`, Ergebnisdatei existiert, Entry ist abgeschlossen | Keine Reparatur nötig. |
| Rezept wurde nach Laufstart deaktiviert oder geändert | Der bestehende Lauf nutzt weiter den Snapshot aus `run.json`. |
| Alter Lauf enthält keinen Eintrag für ein später hinzugefügtes Rezept | Keine automatische nachträgliche Ergänzung der alten `run.json`. Neue Rezept-Einträge entstehen nur beim Erzeugen eines neuen Laufs. |

Manuelle Verifikation nach Änderungen am Perspektivenmodell:

1. `/k-audit 2026-08-21` in einer neuen Assistenten-Session starten und Status lesen.
2. Prüfen, dass `secret-scanning` nach dem Merge `review-secret-scanning.md` ohne
   Alignment-Hinweis erzeugt und den Scope `gitleaks`, `trufflehog` nennt.
3. Prüfen, dass `python-comment-hardspots` nicht als aktiver Audit-Eintrag ausgewählt wird
   und im Rezept als Family-only begründet ist.
4. In einem CVE-lastigen Ziel einen kleinen Lauf mit Dependency-Tools anlegen und prüfen,
   dass `dependency-cve` als Perspektive über `review-input.json` läuft.
5. In Statusausgabe und Create-Dry-Run prüfen, dass aktive Rezept-Einträge ihren
   gespeicherten `scope` zeigen.
6. Je einen beschädigten AI-Eintrag aus der Reparaturmatrix simulieren und prüfen, dass
   Status oder Rerun die erwartete Reparatur zeigt.

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
einem schlanken Schema:

```json
{
  "name": "tech",
  "kind": "ai",
  "state": "done",
  "result": "review-tech.md",
  "reason": "",
  "startedAt": "2026-08-19T10:00:00Z",
  "finishedAt": "2026-08-19T10:15:00Z"
}
```

`result` ist relativ zum Laufverzeichnis. Wenn `resultRequired` in der kopierten
`run.json`-Metadatenstruktur `true` ist, braucht `state: done` dieses Ergebnis und die
Datei muss existieren. `failed` und `skipped` brauchen einen `reason`; `running` darf kein
`finishedAt` tragen.

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

**Dedupe-Modell.** Findings werden nie stumm entfernt. Drei Regeln fassen zusammen:

- gleiche `fingerprint` oder `partialFingerprint` innerhalb vergleichbarer Quelle,
- gleiche Datei, Zeile, Regel-ID und normalisierte Message,
- gleiche Dependency-ID (CVE/GHSA/OSV) plus Package plus Version bzw. Manifest.

Gleiche Datei/Zeile mit ähnlicher Regel-Familie wird nicht zusammengefasst, sondern
wechselseitig als `possible-duplicate` markiert. Der Assistent entscheidet, ob er die
Belege bündelt.

**Zwei Artefakte, klare Rollen.** `review-input.json` trägt Provenienz (`schemaVersion`,
`generated`, `kPlaybookVersion`, Laufkontext, Entries, Findings, Gruppen) und alle
Belege — deshalb darf es groß werden. `review-input.md` bleibt bewusst kompakt:
Gruppen werden gebündelt, die Detailliste ist hart gedeckelt und verweist auf die
JSON, sobald sie das Limit überschreitet.

**Wiederholbar.** Ein erneuter Aufruf überschreibt beide Artefakte. Bei fehlenden
Entry-Dateien für einen in `run.json` ausgewählten Eintrag gilt der Zustand `start`;
kein Fehler, sondern eine sichtbare Auskunft im Statusblock.

## Wirkung von `known-decisions.md`

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

Suchpfad:

- `RUN_DIR/known-decisions.md` für laufspezifische Decisions.
- `k-playbook-local/results/known-decisions.md` für projektweite Decisions.
- Beide Dateien werden kombiniert. Bei gleicher `id` gewinnt die laufspezifische Decision;
  die verdrängte projektweite Fassung steht sichtbar im Metablock.
- Wenn keine Datei existiert, meldet der Merge sichtbar „keine known-decisions geladen".

JSON-Wirkung:

- Gedeckte Findings tragen `coveredByKnownDecision` mit `id`, `category` und `matchedBy`.
- Gruppen tragen `coveredByKnownDecision` nur, wenn alle Findings derselben primären
  Decision unterliegen.
- Teils gedeckte Gruppen tragen `partialCoverage: true` und `knownDecisionCoverage` mit
  IDs, Kategorien und Finding-Zahlen.
- Der Metablock `knownDecisions` nennt Quellen, geladene IDs, Herkunft, Ablauf,
  `applied`, `notAppliedReason` und Loader-Warnungen.

`review-input.md` zeigt die Anzahl vollständig und teilweise gedeckter Gruppen, eine
Known-Decision-Spalte in der Gruppentabelle und abgelaufene Decisions als eigenen Hinweis.
Das Bewertungsmodul `review-scan-triage` liest diese Deckung anschließend ausschließlich
aus `review-input.json`.

## Bewerten mit `review-scan-triage`

Nach dem Merge bewertet `/k-audit` den Lauf über das Command-Modul
`commands/_audit/review-scan-triage.md`. Das Modul ist kein Review-Rezept und
wird nicht aus `reviews/` geladen. Es gehört zum Command-Namensraum von
`/k-audit` und wird als AI-Eintrag `scan-triage` im Lauf geführt. Der
`/k-review`-Report-Modus nutzt dasselbe Eingabe- und Ausgabeformat, schreibt aber keinen
MCP-Entry-Status.

Eingaben sind ausschließlich Dateien im Laufkontext:

- `review-input.json` mit vollständigen Belegen, Provenienz und stabilen Gruppen-IDs,
- `review-input.md` als kompakte Ansicht,
- die vom Merge berechnete Deckung unter `knownDecisions`,
  `coveredByKnownDecision`, `partialCoverage` und `knownDecisionCoverage`.

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
