---
name: review-tech
title: Tech-Debt-Analyse
interval-weeks: 24
scope-hint: Quell- und Infrastruktur-Verzeichnisse; Ausschluss - priv/, secure/, tasks/, virtuelle Umgebungen
handoff: /k-remediation
result-family: tech
audit:
  enabled: true
  mode: evidence
  ruleIds:
    - tech-duplicated-logic
    - tech-magic-value
    - tech-dead-code
    - tech-swallowed-error
    - tech-boundary-violation
    - tech-test-gap
    - tech-doc-drift
    - tech-manual-operation
  scope:
    paths:
      - "**/*.go"
      - "**/*.py"
      - "**/*.pyi"
      - "**/*.sh"
      - "**/*.toml"
      - "**/*.yaml"
      - "**/*.yml"
      - "**/Makefile"
      - "**/Dockerfile*"
review:
  enabled: true
---

# Review: Tech-Debt

Findet Tech-Debt-Kandidaten in Quell- und Infrastrukturdateien: Stellen, an denen die
Struktur des Codes die nächste Änderung teurer macht, als sie sein müsste.

Der generische Ablauf wird von `/k-review` orchestriert. Dieses Rezept arbeitet in zwei
Betriebsarten: als Report für `/k-remediation` und als Evidence-Quelle im Audit-Lauf
(`audit.mode: evidence`). Die Prüfkriterien unten gelten in beiden gleich; unterschiedlich
ist allein die Ergebnisform am Ende der Datei.

## Was als Fund zählt

Ein Fund ist eine konkrete Stelle in einer Datei des Scopes, die sich einer der Rule-IDs
unten zuordnen lässt. Er braucht drei Dinge:

- einen Ort — Datei und Zeile, an der das Problem sichtbar ist,
- eine Rule-ID aus der Liste,
- einen Satz, der sagt, was dort steht und warum es künftige Arbeit teurer macht.

Lässt sich eine dieser drei Angaben nicht machen, ist es kein Fund dieses Rezepts.

## Was nicht als Fund zählt

- Stilfragen, Formatierung und Namensgeschmack.
- Alles, was ein Linter oder Scanner des Laufs bereits meldet — `ruff`, `golangci-lint`,
  `gosec`, `semgrep`. Deren Funde stehen ohnehin in `review-input.json`.
- Abhängigkeitsalter und CVEs. Dafür gibt es `review-dependency-cve` und die
  Dependency-Scanner; eine Schätzung aus dem Code heraus wäre schlechter als deren Messung.
- Fehlende Features, Wünsche und Umbauideen ohne belegten Schaden.
- Vermutungen über Absicht. Wo unklar ist, ob eine Stelle bewusst so steht, gilt: nicht
  aufnehmen. Eine leere Fundliste ist ein gültiges Ergebnis.

## Rule-IDs und `level`

Die Liste ist abschließend. Eine Rule-ID außerhalb dieser Liste macht das Ergebnis
ungültig — sie steht so auch in `audit.ruleIds` und wird beim Melden geprüft.

| Rule-ID | Was der Fund zeigt | `level` |
|---|---|---|
| `tech-duplicated-logic` | Dieselbe fachliche Entscheidung steht an mehreren Stellen und muss bei jeder Änderung mehrfach nachgezogen werden. | `warning` |
| `tech-magic-value` | Ein unbenannter Wert steuert Verhalten, und seine Herkunft ist aus dem Code nicht erschließbar. | `note` |
| `tech-dead-code` | Code, den kein Pfad mehr erreicht: unbenutzte Funktion, nie erfüllter Zweig, Rest einer abgelösten Lösung. | `note` |
| `tech-swallowed-error` | Ein Fehler wird verworfen oder pauschal abgefangen, ohne dass der Aufrufer davon erfährt. | `error` |
| `tech-boundary-violation` | Eine Schicht greift an ihrer Schnittstelle vorbei: direkter Zugriff auf Interna eines fremden Bereichs, Zyklus zwischen Modulen. | `error` |
| `tech-test-gap` | Ein riskanter Pfad ohne Test — Fehlerbehandlung, Grenzfall, Migration —, dessen Bruch im Betrieb nicht auffiele. | `warning` |
| `tech-doc-drift` | Ein Kommentar oder Doku-Block **in einer Datei des Scopes** behauptet etwas, was der Code daneben nicht mehr tut. | `note` |
| `tech-manual-operation` | Ein Betriebsschritt ist nur von Hand ausführbar oder nur mündlich überliefert, obwohl er zum Bauen, Ausrollen oder Prüfen gehört. | `note` |

`level` ist die einzige Wertung, die dieses Rezept vergibt, und es steht so im Ergebnis,
wie es hier festgelegt ist:

- `error` und `note` sind verbindlich. Der Merge übernimmt sie unverändert
  (`severitySource: native`).
- `warning` ist ausdrücklich **kein** Urteil, sondern die Übergabe an das
  Severity-Mapping des Projekts (`scripts/severity.tsv`): wie schwer eine Dublette oder
  eine Testlücke wiegt, hängt am Projekt und nicht am Fund. Fehlt dort ein Eintrag,
  bleibt es bei `warning`.

## Ausschlüsse

Die zentralen Ausschlüsse der Modulsuche — `k-playbook/`, `k-playbook-local/`, `vendor/`,
`node_modules/`, `testdata/` und Punkt-Verzeichnisse — gelten geerbt und stehen deshalb
nicht noch einmal in `scope.paths`. Zusätzlich bleiben außen vor:

- Generierter Code und abgelegte Kopien alter Stände (`_old/`, `*_generated.go`, `*_pb2.py`).
- Virtuelle Umgebungen und Build-Ausgaben (`venv/`, `dist/`, `build/`).
- Private und vertrauliche Bereiche: `priv/`, `secure/`.
- Aufgabenlisten, Notizen und Ergebnisordner: `tasks/`, `results/`.

Eine Folge der geerbten Punkt-Regel, die leicht durchrutscht: CI-Konfigurationen unter
`.github/` liegen außerhalb jedes Pfad-Scopes. `**/*.yml` trifft sie nicht, und ein Fund
dort würde beim Melden verworfen.

## Ergebnisform in `/k-review` (Report-Modus)

Dieses Review moderiert keine Freigaben pro Fund. Es erzeugt ein vollständiges
Ergebnis, das anschließend im Rahmen von `/k-remediation` einzeln durchgegangen wird.

Die Ergebnisfamilie ist `tech`; der Lauf schreibt in den Family-Ordner
`k-playbook-local/results/tech/<datum>/` und dort genau zwei Dateien — `review-input.json`
nach dem Belegvertrag und `review-triage.md` als Endartefakt. Den Ablauf beschreibt
`/k-review`, Step 5b; rezeptspezifisch ist:

- Jeder Fund geht mit Ort, Rule-ID, `level` und dem Satz, der ihn trägt, in
  `review-input.json`. Die Rule-IDs sind dieselben wie im Audit-Lauf.
- Bündelung, Priorität und Kategorie vergibt das Triage-Modul beim Schreiben von
  `review-triage.md`, nicht dieses Rezept. Auch hier gilt: keine eigene Reihenfolge in der
  Fundliste.
- Keine Code-Änderungen aus diesem Review heraus.

## Ergebnisform im Audit-Lauf (`mode: evidence`)

Der Eintrag heißt `tech` und schreibt genau ein Artefakt: `raw/tech.sarif`. Ein
Markdown-Ergebnis, ein Family-Ordner oder ein zweites `review-input.*` entstehen nicht.
Den Ablauf beschreibt `/k-audit`, Schritt 5; rezeptspezifisch ist:

- `tool.driver.name` ist `tech` — der Eintragsname, nicht der Rezeptdateiname.
- Jeder Fund trägt eine `ruleId` aus der Liste oben, das dort festgelegte `level`, genau
  einen Fundort mit projektrelativem Pfad und Startzeile und eine `message.text` von
  einem Satz.
- Mehrere Instanzen desselben Problems in derselben Datei sind mehrere Funde. Fasse sie
  nicht selbst zusammen: der Merge bündelt sie zu einer Gruppe je Rule-ID und Datei, und
  ihre Zahl ist die Aussage, die die Triage daraus liest.
- Keine Priorität `P1`–`P3`, keine Kategoriebuchstaben `S`/`T`/`K`/`F`/`A`/`X`, keine
  Bündelung, keine Reihenfolgeempfehlung. Das ist Sache von
  `commands/_audit/review-scan-triage.md`.

**Mengendeckel: höchstens 40 Funde je Lauf.** Die Triage belegt jeden KI-Fund am Code,
bevor sie ihn priorisiert; jenseits dieser Größenordnung wird aus dem Beleg eine
Abarbeitung, und die Scanner-Funde desselben Laufs gehen daneben unter. Der Deckel ist
eine Anweisung an dieses Rezept und wird beim Melden nicht erzwungen — eine Kappung dort
verwürfe Funde, die danach niemand mehr sieht.

Auswahlregel bei Überschreitung:

1. Gezählt wird der einzelne Fund, nicht die Gruppe.
2. Es entfallen **ganze** Bündel aus Rule-ID und Datei, nie einzelne Instanzen daraus.
   Eine gekürzte Instanzzahl wäre eine falsche Aussage über den Umfang.
3. Es bleibt stehen, was zuerst kommt: `error` vor `warning` vor `note`; bei gleichem
   `level` das Bündel mit mehr Instanzen; bei gleicher Zahl alphabetisch nach Pfad, dann
   nach Rule-ID. Damit trifft ein zweiter Lauf über denselben Stand dieselbe Auswahl.
4. Nenne die Zahl der ausgelassenen Bündel und Funde im `reason` der Fertigmeldung. Ein
   stillschweigend gekürztes Ergebnis sähe aus wie ein vollständiges.

## Handoff

Nach Abschluss der Analyse und dem Log-Eintrag nennt `/k-review` Pfad und exakten
Handoff-Befehl. Remediation ist ausdrücklich nicht Teil dieses Reviews. Im Audit-Lauf gibt
es keinen Handoff aus diesem Rezept: die Funde gehen über den Merge in die Triage.

**Eigenständiger `/k-review`-Lauf nach dem Umbau.** Dieses Rezept läuft im Audit als
Evidence-Quelle mit; dort steckt sein Beleg schon im gemeinsamen Merge. Über
`review.enabled: true` bleibt es daneben einzeln aufrufbar, und ein solcher Lauf legt
einen eigenen Family-Ordner **außerhalb** jedes Laufordners an:

```text
k-playbook-local/results/tech/<datum>/
```

Sein `review-triage.md` geht direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/tech/<datum>/review-triage.md
```

Es gibt dabei **keine Zusammenführung mit dem Audit-Lauf und keine Dedupe gegen dessen
Befunde**. `/k-remediation` nimmt genau eine Ergebnisdatei; ein Befund, den derselbe Tag
auch im Audit-Lauf trägt, steht dann in beiden Ergebnissen einmal. Wer beide Seiten
zusammen sehen will, nimmt den Audit-Lauf — dort und nur dort sitzt die Zusammenführung.
