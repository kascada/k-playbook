---
name: review-code
title: Code-Review
interval-weeks: 12
scope-hint: Quellcode in Go, Python, Shell, JavaScript und TypeScript; Ausschluss - Konfiguration, Container, IaC, generierter Code, priv/, secure/, tasks/
handoff: /k-remediation
result-family: code
audit:
  enabled: true
  mode: evidence
  ruleIds:
    - code-logic-error
    - code-error-dropped
    - code-race
    - code-concurrency-risk
    - code-injection
    - code-missing-authz
    - code-hardcoded-secret
    - code-untrusted-input
    - code-unbounded-work
    - code-resource-leak
    - code-misleading-name
    - code-missing-test
  scope:
    paths:
      - "**/*.go"
      - "**/*.py"
      - "**/*.sh"
      - "**/*.js"
      - "**/*.mjs"
      - "**/*.cjs"
      - "**/*.jsx"
      - "**/*.ts"
      - "**/*.tsx"
review:
  enabled: true
---

# Review: Code-Review

Findet Fehler im Quellcode in vier Richtungen — Korrektheit, Sicherheit, Performance und
Wartbarkeit: Stellen, an denen der Code etwas anderes tut als beabsichtigt, angreifbar ist,
unnötig Last erzeugt oder den Leser in die Irre führt.

Der generische Ablauf gehört dem jeweiligen Aufrufer. Dieses Rezept hat drei
Ergebnisformen:

- den Report für `/k-remediation`, ausgeführt über `/k-review`,
- das SARIF im Audit-Lauf von `/k-audit` (`audit.mode: evidence`),
- den Abschnitt für den Diff eines Tasks, den `/k-task-run` nutzt.

Die Prüfkriterien unten gelten in allen dreien gleich. Unterschiedlich sind allein die
Ergebnisform am Ende der Datei und, ob im selben Lauf eine andere Quelle schon Belege
liefert (Abschnitt „Ausschluss je Lauf").

**Schalter.** `review.enabled` wirkt nur auf `/k-review`, `audit.enabled` nur auf
`/k-audit`. Die Nutzung durch `/k-task-run` hat keinen eigenen Schalter: Sie schaltet allein
ein leeres projekteigenes `review-code.md` ab (`disabled: true` im Katalogeintrag).

**Feste Überschrift.** Der Abschnitt für den Diff eines Tasks steht unter der Überschrift
`## Ergebnisform für den Diff eines Tasks`. `/k-task-run` erkennt ihn an genau dieser
Zeile. Ein projekteigenes `review-code.md` ersetzt diese Datei vollständig und muss den
Abschnitt deshalb unter derselben Überschrift selbst tragen; fehlt er, hält `/k-task-run`
jeden Task an.

## Gegenstand

- **Korrektheit:** Randfälle, Fehlerweitergabe, Nebenläufigkeit, Off-by-one, Typen.
- **Sicherheit:** Injektion, fehlende oder falsche Berechtigungsprüfung, Pfade, unsichere
  Deserialisierung, SSRF, Geheimnisse im Code.
- **Performance:** N+1, ungebremste Schleifen oder Abfragen, Komplexität im heißen Pfad,
  Ressourcenlecks.
- **Wartbarkeit:** irreführende Benennung oder Kommentare, fehlende Tests zum geprüften
  Verhalten.

## Was als Fund zählt

Ein Fund ist eine konkrete Stelle, die sich einer der Rule-IDs unten zuordnen lässt. Er
braucht drei Dinge:

- einen Ort — Datei und Zeile, an der das Problem sichtbar ist,
- eine Rule-ID aus der Liste,
- einen Satz, der sagt, was dort steht und welche Folge es hat.

Ein Fund einer Rule-ID mit `level: error` nennt zusätzlich den Auslöser: die Eingabe, den
Zustand oder den Aufrufpfad, unter dem das falsche Verhalten eintritt; bei
`code-hardcoded-secret` genügt die Stelle. Lässt sich der Auslöser nicht benennen, ist es
höchstens das Risiko derselben Richtung — sofern es dafür eine Rule-ID gibt —, sonst kein
Fund.

Lässt sich eine der drei Angaben nicht machen, ist es kein Fund dieses Rezepts.

## Was nicht als Fund zählt

- Stil, Formatierung und Namensgeschmack. Ein Name zählt erst, wenn er etwas Falsches
  behauptet (`code-misleading-name`).
- Strukturelle Tech-Debt ohne Fehlerbild: Dubletten, magische Werte, toter Code,
  Verstöße gegen Schichtgrenzen, Handarbeit im Betrieb. Dafür gibt es `review-tech`.
- Abhängigkeitsalter und CVEs. Dafür gibt es `review-dependency-cve` und die
  Dependency-Scanner.
- IaC-, Container- und Imagekonfiguration. Dafür gibt es `review-iac-container`.
- Komplexität außerhalb eines heißen Pfads: eine teure, aber begrenzte Stelle, die einmal
  je Aufruf läuft, ist kein Fund.
- Fehlende Features, Wünsche und Umbauideen ohne belegten Schaden.
- Vermutungen über Absicht. Wo unklar ist, ob eine Stelle bewusst so steht, gilt: nicht
  aufnehmen. Eine leere Fundliste ist ein gültiges Ergebnis.

## Rule-IDs und `level`

Die Liste ist abschließend. Eine Rule-ID außerhalb dieser Liste macht das Ergebnis
ungültig — sie steht so auch in `audit.ruleIds` und wird beim Melden geprüft.

**Schnittregel.** `level` hängt fest an der Rule-ID. Deshalb deckt keine Rule-ID zugleich
einen belegten Fehler und ein bloßes Risiko ab: Wo eine Richtung beides kennt, sind es zwei
Rule-IDs. `error` steht nur an Rule-IDs für Funde, die falsches Verhalten, Datenverlust oder
eine Sicherheitslücke zeigen; `warning` an Rule-IDs für Risiken; `note` an Rule-IDs für
Wartbarkeit. Eine spätere Rule-ID folgt derselben Regel. Passt ein Gegenstand nur mit
wechselndem `level`, wird er in zwei Rule-IDs geteilt, statt eine Rule-ID mit Ausnahme zu
führen.

| Rule-ID | Richtung | Was der Fund zeigt | `level` |
|---|---|---|---|
| `code-logic-error` | Korrektheit | Für eine benennbare Eingabe oder einen benennbaren Zustand tut der Code etwas anderes als beabsichtigt — Randfall, Off-by-one, falsche Bedingung, falscher Typ oder falsche Umwandlung —, bis hin zu verlorenen oder überschriebenen Daten. | `error` |
| `code-error-dropped` | Korrektheit | Ein Fehler wird verworfen, überdeckt oder falsch weitergegeben, und der Aufrufer arbeitet mit einem falschen oder unvollständigen Ergebnis weiter. | `error` |
| `code-race` | Korrektheit | Ein Nebenläufigkeitsfehler mit benennbarer Abfolge: gemeinsamer Zustand ohne Synchronisation, verlorene Aktualisierung, gegenseitige Blockade. | `error` |
| `code-concurrency-risk` | Korrektheit | Nebenläufiger Code verlässt sich auf eine Reihenfolge, eine Lebensdauer oder einen Abbruch, den nichts zusichert; ein Fehlablauf ist möglich, aber nicht belegt. | `warning` |
| `code-injection` | Sicherheit | Eine Eingabe aus einer nicht vertrauenswürdigen Quelle gelangt nachweislich ungeprüft in eine Abfrage, eine Shell-Zeile, einen Dateipfad, eine Deserialisierung oder das Ziel einer ausgehenden Anfrage (SSRF). | `error` |
| `code-missing-authz` | Sicherheit | Eine geschützte Operation ist auf einem benennbaren Pfad ohne oder mit falscher Berechtigungsprüfung erreichbar, oder die Prüfung steht erst hinter der Wirkung. | `error` |
| `code-hardcoded-secret` | Geheimnisse | Zugangsdaten, Token oder private Schlüssel stehen im Code. | `error` |
| `code-untrusted-input` | Sicherheit | Eine Eingabe erreicht eine der Stellen aus `code-injection`, ohne dass ihre Herkunft oder eine Prüfung belegbar ist; ein Angriffsweg ist denkbar, aber nicht nachgewiesen. | `warning` |
| `code-unbounded-work` | Performance | Die Arbeit wächst ohne Grenze mit der Eingabe: eine Abfrage je Element (N+1), eine Schleife oder Abfrage ohne Obergrenze, hohe Komplexität im heißen Pfad. | `warning` |
| `code-resource-leak` | Performance | Eine Ressource — Datei, Verbindung, Goroutine, Prozess, Sperre — wird auf mindestens einem Pfad nicht freigegeben. | `warning` |
| `code-misleading-name` | Wartbarkeit | Ein Name oder Kommentar behauptet etwas anderes, als der Code tut. | `note` |
| `code-missing-test` | Wartbarkeit | Verhalten in den geprüften Zeilen — eine Verzweigung, ein Fehlerpfad, ein behandelter Randfall — hat keinen Test, der es festhält. | `note` |

`Geheimnisse` ist ein Teil der Sicherheit, steht aber als eigene Richtung in der Tabelle:
Im Audit belegen ihn eigene Scanner (Abschnitt „Ausschluss je Lauf").

`level` ist die einzige Wertung, die dieses Rezept vergibt, und es steht so im Ergebnis,
wie es hier festgelegt ist:

- `error` und `note` sind verbindlich. Der Merge übernimmt sie unverändert
  (`severitySource: native`).
- `warning` ist ausdrücklich **kein** Urteil, sondern die Übergabe an das
  Severity-Mapping des Projekts (`scripts/severity.tsv`). Fehlt dort ein Eintrag, bleibt es
  bei `warning`.

Im Diff eines Tasks entscheidet `level` über die Einstufung; wie, steht im Abschnitt dafür.

## Ausschluss je Lauf

Die Kriterien oben hängen nicht an der Betriebsart. Was dieses Rezept meldet, hängt allein
am Lauf: Was im selben Lauf eine andere Quelle schon belegt, meldet es nicht noch einmal.
Quellen sind nur Einträge, die wie dieses Rezept **vor** dem Merge Belege liefern. Die
Perspektiven `review-sast` und `review-secret-scanning` zählen nicht: Sie laufen nach dem
Merge und bewerten nur, was die Scanner geliefert haben.

**Allein** — über `/k-review` oder für den Diff eines Tasks — gibt es keine andere Quelle.
Der Ausschluss greift nicht; alle Rule-IDs gelten in allen Richtungen.

**Im Audit** hängt der Ausschluss an den Einträgen des Laufs und ihren Zuständen. Beides
steht in der Statusausgabe unter `entries`, die `/k-audit` vor den Evidence-Rezepten gelesen
hat; maßgeblich ist der Zustand dort. Quellen sind:

- ein Scanner-Eintrag im Zustand `done`. Ein Eintrag `failed` oder `skipped` —
  übersprungen etwa, weil das Werkzeug fehlt — hat nichts belegt, ebenso einer auf `start`
  oder `running`; sein Gegenstand bleibt bei diesem Rezept.
- der Evidence-Eintrag `tech`, sobald er im Lauf ausgewählt ist, für die Dateien in seinem
  Pfad-Scope (`scope` des Eintrags in derselben Ausgabe). Er läuft im selben Schritt wie
  dieses Rezept; scheitert er, ersetzt ihn ein erneuter Lauf von `tech`, nicht dieses
  Rezept.

Welcher Scanner welche Richtung belegt:

| Scanner-Eintrag | Sprachen | Richtung |
|---|---|---|
| `gitleaks`, `trufflehog` | alle Dateien | Geheimnisse |
| `semgrep` | Python, Go, JavaScript, TypeScript | Sicherheit |
| `gosec` | Go | Sicherheit |
| `ruff` | Python | Sicherheit — der Eintrag läuft nur mit den Sicherheitsregeln |
| `njsscan` | JavaScript, TypeScript | Sicherheit |
| `golangci-lint` | Go | Korrektheit, Wartbarkeit |

Die Sprachen stehen in `scripts/security-tools.tsv` und `scripts/scanners.tsv` (Spalte
`languages`); weicht die Tabelle davon ab, gilt die Matrix.

Was im Audit bleibt:

- **Scanner derselben Richtung `done`.** In einer Datei, deren Sprache ein solcher Scanner
  abdeckt, meldet das Rezept in dieser Richtung nur Funde, deren Begründung mehr als die
  Fundstelle braucht: den Aufrufer, die Herkunft der Daten, die Berechtigungslage oder das
  beabsichtigte Verhalten. Das sind etwa eine fehlende oder an falscher Stelle stehende
  Berechtigungsprüfung, eine Eingabe, die über eine Modulgrenze ungeprüft in eine Abfrage
  gelangt, ein Fehler, der geprüft, aber falsch weitergegeben wird, oder eine Bedingung, die
  dem beabsichtigten Verhalten widerspricht. In den übrigen Richtungen meldet es in dieser
  Datei alles.
- **Performance** belegt kein Scanner. Sie bleibt in jedem Lauf vollständig bei diesem
  Rezept.
- **Geheimnisse** meldet das Rezept nur, solange weder `gitleaks` noch `trufflehog` `done`
  ist. Scanner der Richtung Sicherheit ändern daran nichts.
- **`tech` ausgewählt.** In den Dateien seines Pfad-Scopes bleibt vom Gegenstand der
  `tech`-Rule-IDs bei diesem Rezept nur: aus `code-error-dropped` der Fehler, der überdeckt
  oder falsch weitergegeben wird — der verworfene oder pauschal abgefangene gehört zu
  `tech-swallowed-error` —, aus `code-misleading-name` der Name — der veraltete Kommentar
  gehört zu `tech-doc-drift`. `code-missing-test` gehört dort ganz zu `tech-test-gap`.

## Ausschlüsse

Diese Ausschlüsse begrenzen den Scope im Evidence-Lauf und im Report. Die zentralen
Ausschlüsse der Modulsuche — `k-playbook/`, `k-playbook-local/`, `vendor/`,
`node_modules/`, `testdata/` und Punkt-Verzeichnisse — gelten geerbt und stehen deshalb
nicht noch einmal in `scope.paths`. Zusätzlich bleiben außen vor:

- Generierter oder gebündelter Code und abgelegte Kopien alter Stände (`_old/`,
  `*_generated.go`, `*_pb2.py`, `*.min.js`).
- Virtuelle Umgebungen und Build-Ausgaben (`venv/`, `dist/`, `build/`).
- Private und vertrauliche Bereiche: `priv/`, `secure/`.
- Aufgabenlisten, Notizen und Ergebnisordner: `tasks/`, `results/`.

Konfiguration, Container und IaC liegen außerhalb von `scope.paths`; ein Fund dort würde im
Evidence-Lauf beim Melden verworfen.

## Mengendeckel

**Höchstens 40 Funde je Lauf**, im Evidence-Lauf und im Report. Die Triage belegt jeden
KI-Fund am Code, bevor sie ihn priorisiert; jenseits dieser Größenordnung wird aus dem
Beleg eine Abarbeitung. Der Deckel ist eine Anweisung an dieses Rezept und wird beim Melden
nicht erzwungen. Für den Diff eines Tasks gilt kein Deckel.

Auswahlregel bei Überschreitung:

1. Gezählt wird der einzelne Fund, nicht die Gruppe.
2. Es entfallen **ganze** Bündel aus Rule-ID und Datei, nie einzelne Instanzen daraus.
   Eine gekürzte Instanzzahl wäre eine falsche Aussage über den Umfang.
3. Es bleibt stehen, was zuerst kommt: `error` vor `warning` vor `note`; bei gleichem
   `level` das Bündel mit mehr Instanzen; bei gleicher Zahl alphabetisch nach Pfad, dann
   nach Rule-ID. Damit trifft ein zweiter Lauf über denselben Stand dieselbe Auswahl.
4. Nenne die Zahl der ausgelassenen Bündel und Funde — im Evidence-Lauf im `reason` der
   Fertigmeldung, im Report in der Abschlussmeldung von `/k-review`. Ein stillschweigend
   gekürztes Ergebnis sähe aus wie ein vollständiges.

## Ergebnisform in `/k-review` (Report-Modus)

Dieses Review moderiert keine Freigaben pro Fund. Es erzeugt ein vollständiges Ergebnis,
das anschließend im Rahmen von `/k-remediation` einzeln durchgegangen wird.

Die Ergebnisfamilie ist `code`; der Lauf schreibt in den Family-Ordner
`k-playbook-local/results/code/<datum>/` und dort genau zwei Dateien — `review-input.json`
nach dem Belegvertrag und `review-triage.md` als Endartefakt. Den Ablauf beschreibt
`/k-review`, Step 5b; rezeptspezifisch ist:

- Jeder Fund geht mit Ort, Rule-ID, `level` und dem Satz, der ihn trägt, in
  `review-input.json`. Die Rule-IDs sind dieselben wie im Audit-Lauf.
- Der Ausschluss je Lauf greift nicht; der Mengendeckel gilt.
- Bündelung, Priorität und Kategorie vergibt das Triage-Modul beim Schreiben von
  `review-triage.md`, nicht dieses Rezept.
- Keine Code-Änderungen aus diesem Review heraus.

## Ergebnisform im Audit-Lauf (`mode: evidence`)

Der Eintrag heißt `code` und schreibt genau ein Artefakt: `raw/code.sarif`. Ein
Markdown-Ergebnis, ein Family-Ordner oder ein zweites `review-input.*` entstehen nicht.
Den Ablauf beschreibt `/k-audit`, Schritt 5; rezeptspezifisch ist:

- `tool.driver.name` ist `code` — der Eintragsname, nicht der Rezeptdateiname.
- Jeder Fund trägt eine `ruleId` aus der Liste oben, das dort festgelegte `level`, genau
  einen Fundort mit projektrelativem Pfad und Startzeile und eine `message.text` von
  einem Satz.
- Der Ausschluss je Lauf gilt, mit den Zuständen aus der Statusausgabe.
- Mehrere Instanzen desselben Problems in derselben Datei sind mehrere Funde. Fasse sie
  nicht selbst zusammen: der Merge bündelt sie zu einer Gruppe je Rule-ID und Datei.
- Keine Priorität `P1`–`P3`, keine Kategoriebuchstaben `S`/`T`/`K`/`F`/`A`/`X`, keine
  Bündelung, keine Reihenfolgeempfehlung. Das ist Sache von
  `commands/_audit/review-scan-triage.md`.

## Ergebnisform für den Diff eines Tasks

**Gegenstand.** Nur die geänderten Zeilen seit dem Snapshot des Tasks, und zwar der ganze
Diff: `scope.paths` und die Ausschlüsse oben gelten hier nicht. Umliegender Code darf
gelesen werden, um einen Befund zu beurteilen; Befunde außerhalb des Diffs werden nicht
gemeldet. In Dateien oder Teilen ohne Programmlogik — Doku, Anweisungstext in Markdown —
gilt nur der Verstoß gegen eine Vorgabe des Tasks. Wo eine solche Datei Logik trägt —
Shell, Shell-Codeblöcke in Markdown, Konfiguration, die Verhalten steuert —, gelten für
diese Logik zusätzlich die Rule-IDs der Richtungen Korrektheit, Sicherheit und
Geheimnisse. In Quellcode gelten alle Rule-IDs.

**Maßstab für die Absicht.** Die Task-Beschreibung. Was sie verlangt, ist das beabsichtigte
Verhalten, an dem etwa `code-logic-error` gemessen wird.

**Einstufung.**

| Grundlage | Einstufung |
|---|---|
| Rule-ID mit `level: error` | kritisch |
| Rule-ID mit `level: warning` | wichtig |
| Rule-ID mit `level: note` | Hinweis |
| Verstoß gegen eine ausdrückliche Vorgabe des Tasks | kritisch |
| kaputter Build | kritisch |
| rote Tests | kritisch |

Die letzten drei Zeilen sind die aufgabengebundenen Kriterien, die es nur hier gibt. Sie
tragen keine Rule-ID — `ruleIds` bleibt die Liste des Evidence-Laufs. Diese Zuordnung ist
der einzige Maßstab für „kritisch". Die Grenze liegt dort, weil ein folgender Task auf
solchen Fehlern aufbauen würde; das begründet sie, ist aber kein zweites Kriterium.

**Beleg der aufgabengebundenen Kriterien.**

- Verstoß gegen eine Vorgabe: die Vorgabe im Wortlaut der Task-Datei und die Stelle im
  Diff, die ihr widerspricht. Eine erschlossene Absicht ist keine ausdrückliche Vorgabe.
- Kaputter Build, rote Tests: Das Review sieht nur den Diff. Beleg sind allein die
  Validierungsergebnisse, die das Modul von `/k-task-run` übergibt oder selbst erhebt. Ohne
  solchen Beleg wird das Kriterium nicht angewandt, auch nicht vermutet.

## Handoff

Nach Abschluss der Analyse und dem Log-Eintrag nennt `/k-review` Pfad und exakten
Handoff-Befehl. Remediation ist ausdrücklich nicht Teil dieses Reviews. Im Audit-Lauf gibt
es keinen Handoff aus diesem Rezept: die Funde gehen über den Merge in die Triage. Das
Review für den Diff eines Tasks geht an `/k-task-run` zurück.

**Eigenständiger `/k-review`-Lauf.** Dieses Rezept läuft im Audit als Evidence-Quelle mit;
dort steckt sein Beleg schon im gemeinsamen Merge. Über `review.enabled: true` bleibt es
daneben einzeln aufrufbar, und ein solcher Lauf legt einen eigenen Family-Ordner
**außerhalb** jedes Laufordners an:

```text
k-playbook-local/results/code/<datum>/
```

Sein `review-triage.md` geht direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/code/<datum>/review-triage.md
```

Es gibt dabei **keine Zusammenführung mit dem Audit-Lauf und keine Dedupe gegen dessen
Befunde**. Wer beide Seiten zusammen sehen will, nimmt den Audit-Lauf — dort und nur dort
sitzt die Zusammenführung.
