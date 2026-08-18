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
| `ai` | ein Review-Rezept aus dem Katalog | ein Assistent, über einen Command |

Beide sind Einträge desselben Laufs. Nur der Weg dorthin unterscheidet sich — das Ergebnis
landet für beide im selben Verzeichnis und damit später in derselben Zusammenfassung.

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

**Es gibt vorerst zwei Orte für Rohdaten.** Die Ergebnisfamilien unter
`k-playbook-local/results/<familie>/YYYY-MM-DD/raw/` bleiben, wie sie sind, und
`/k-results` liest weiter sie; `ListRuns()` zeigt sie ohnehin mit an. Das
Laufverzeichnis ist dagegen familienlos, weil ein Lauf gerade über die Familien hinweg
klammert. Wann beides zusammengeht, entscheidet der Umbau der Rezepte auf reine
Bewertung ([`umbau.md`](./umbau.md), „Offen").

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
    { "name": "review-secret-scanning", "kind": "ai",   "state": "start" }
  ]
}
```

Die Schlüssel sind englisch, wie alles, was das Werkzeug sonst als JSON ausgibt —
`schemaVersion`, `missingRequired`, `installMethod`. Sie entstehen aus den Go-Feldnamen.

`languages` hält fest, mit welcher Sprachauswahl der Lauf angelegt wurde. Sie kann sich
danach ändern; was gelaufen ist, soll trotzdem nachvollziehbar bleiben.

Die `entries` in `run.json` tragen den Zustand mit, den der Lauf **festgelegt** hat. Den
tatsächlichen Fortschritt führt die Datei unter `entries/`.

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

## Die Oberfläche

Der Block **Workflows** auf der Startseite führt mit einem Knopf zur Seite `/reviews`; die
Zahl darauf ist die der vorhandenen Läufe. Aufgelistet werden sie auf der Seite selbst.
Dort wird auch ein neuer Lauf zusammengestellt:

- **Werkzeuge** aus der Tool-Matrix, gefiltert nach `project.languages`. Was nicht
  installiert ist, steht da, lässt sich aber nicht auswählen — mit dem Hinweis, wie es
  installiert wird.
- **Reviews** aus dem aufgelösten Katalog, mitgeliefert und projekteigen zusammengeführt.
  Abgeschaltete Einträge fehlen.

„Erstellen" legt das Verzeichnis und `run.json` an. Mehr nicht: **das Anlegen startet
nichts.** Gestartet wird im Terminal, mit `k-playbook scan` — einen Knopf dafür gibt es
in der Oberfläche bewusst noch nicht.
