# Regel: Review-Rezepte erzeugen und pflegen

## Zweck

Ein Review-Rezept beschreibt nur die reviewspezifischen Kriterien. Der generische Ablauf gehört in `/k-review`, nicht in einzelne Review-Dateien.

## Ablage

Globale Review-Rezepte liegen unter:

`<playbook.dir>/reviews/`

Projekteigene Review-Rezepte liegen unter:

`<local.dir>/reviews/`

Review-Ergebnisse liegen unter `<local.dir>/results/`, nicht bei den Rezepten und nicht unter `checks/`. `reviews/` enthält nur Rezepte, `checks/` nur ausführbare Prüfroutinen.

Konvention für Report-/Scan-Familien:

`<local.dir>/results/<scan-family>/YYYY-MM-DD/`

Typische aktuelle Dateien darin:

- `review-input.json` — der Belegvertrag. Sein Schema steht in
  `commands/_review-run/review-input-contract.md` und wird hier nicht wiederholt.
- `review-triage.md` — einheitliches Endartefakt mit Kopf, Bündel-Tabelle,
  Bündel-Details, Nicht gebündelt und Deckung aus known-decisions.
- `raw/` — maschinenlesbare Rohdaten wie SARIF, JSON oder Tool-Logs.

## Dateinamen

- Review-Rezepte heißen `review-<name>.md`.
- Der Name im Frontmatter entspricht dem Dateinamen ohne `.md`.
- Projektlokale Review-Rezepte dürfen globale Rezepte mit gleichem Dateinamen überlagern.

## Frontmatter

Jedes Review-Rezept enthält YAML-Frontmatter mit mindestens:

```yaml
---
name: review-<name>
title: <lesbarer Titel>
interval-weeks: <zahl>
scope-hint: <kurzer Scope-Hinweis>
---
```

Optional, mit einer Kopplung: `handoff` und `result-family` gehören zusammen. Wer
`handoff` setzt, macht das Rezept zum Report-Rezept — und ein Report-Rezept **braucht**
eine `result-family`, sonst hat sein Ergebnis keinen Ort. `/k-review` bricht dann ab,
statt einen Ersatzpfad zu wählen.

```yaml
language: python
handoff: /k-remediation
result-family: <family-name>
audit:
  enabled: <true|false>
  mode: <perspective|evidence>
review:
  enabled: <true|false>
```

`result-family` kennzeichnet Report-/Scan-Familien, deren Ergebnisse unter `<local.dir>/results/<family-name>/YYYY-MM-DD/` liegen und typischerweise `review-input.json`, `review-triage.md`, `raw/` und ggf. Run-Metadaten enthalten. Einen zweiten Ergebnisweg daneben gibt es nicht: Der Family-Ordner ist der einzige Ort, an den ein Report-Rezept schreibt, und sein `review-triage.md` geht direkt an `/k-remediation` — ohne Zusammenführung mit anderen Familien oder mit einem Audit-Lauf. `audit.enabled` steuert Kandidaten für `/k-audit`-/MCP-Läufe; `review.enabled` steuert die gezielte `/k-review`-Auswahl. Welche weiteren Felder der `audit`-Block trägt, hängt an `audit.mode` und steht im nächsten Abschnitt.

## Zwei Betriebsarten im Audit-Lauf

`audit.mode` sagt, wie ein aktives Rezept im Lauf arbeitet. Ohne Angabe gilt `perspective`; Rezepte ohne das Feld bleiben also gültig. Die Felder des `audit`-Blocks gehören jeweils zu genau einer Betriebsart — der Lauf weist ein Rezept mit widersprüchlichem Block als nicht auswählbar aus, statt es still zu verbiegen.

**`mode: perspective`** — der Eintrag läuft **nach** dem Merge und bewertet die Gruppen aus `review-input.json`. Er ist kein eigener Scanner. Sein Ergebnis ist genau eine Markdown-Datei im Laufordner.

```yaml
audit:
  enabled: true
  mode: perspective
  defaultResult: review-<name>.md
  resultRequired: <true|false>
  scope:
    tools: [<tool>, <tool>]
```

**`mode: evidence`** — der Eintrag läuft **vor** dem Merge, liest Code im eingefrorenen Pfad-Scope und liefert selbst Belege. Sein Pflichtartefakt ist SARIF unter `raw/<entry>.sarif`; ein Markdown-Ergebnis entsteht nicht. `defaultResult` und `resultRequired` beschreiben eine Ergebnisdatei, die es hier nicht gibt, und sind deshalb verboten.

```yaml
audit:
  enabled: true
  mode: evidence
  ruleIds: [<rule-id>, <rule-id>]
  scope:
    paths: ["<glob>", "<glob>"]
```

`scope.paths` ist der verbindliche Scope des Evidence-Laufs und wird beim Melden erzwungen: Funde außerhalb werden verworfen und gezählt. Die zentralen Ausschlüsse der Modulsuche — `k-playbook/`, `k-playbook-local/`, `vendor/`, `node_modules/`, `testdata/` und Punkt-Verzeichnisse — erbt er ohnehin; sie gehören nicht noch einmal ins Rezept. `scope-hint` bleibt Freitext für `/k-review` und darf `scope.paths` weder erweitern noch überstimmen.

`ruleIds` ist die abschließende Liste der Rule-IDs, die das Rezept vergeben darf. Sie wird beim Melden geprüft: eine Rule-ID außerhalb der Liste macht den Eintrag `failed`. Sie hält die Funde über Läufe hinweg vergleichbar — frei erfundene Rule-IDs je Lauf zerstörten das.

## Inhalt

Ein Review-Rezept soll enthalten:

- Ziel des Reviews.
- Was als Finding zählt.
- Was ausdrücklich nicht als Finding zählt.
- Bewertungskriterien oder Anti-Muster.
- Bei interaktiven Reviews: welche Vorschläge gemacht werden dürfen.
- Bei Report-Reviews: wohin der Handoff geht.
- Bei `review-code.md`: den Abschnitt für den Diff eines Tasks (Abschnitt „Ergebnisform ‚Diff eines Tasks'" unten).

Bei `mode: evidence` kommt dazu:

- Die Rule-IDs aus `audit.ruleIds` mit je einem Satz, was ein Fund dieser Rule-ID zeigt. Die Liste ist der Vertrag über die Vergleichbarkeit; ohne Erklärung im Text ist sie nicht anwendbar.
- Das `level` je Rule-ID. Es ist die einzige Wertung, die ein Evidence-Rezept vergibt, und es entscheidet über die Schwere im Merge: `error` und `note` gelten unverändert, `warning` und `none` laufen weiter über CVSS, Tool-Metadaten und `scripts/severity.tsv`. Ein `warning` ist damit die schwächste Aussage und kein Urteil.
- Einen Mengendeckel je Lauf samt Auswahlregel, welche Funde bei Überschreitung stehen bleiben. Er bleibt Anweisung im Text und wird beim Melden nicht erzwungen — eine Kappung dort verwürfe Funde, die danach niemand mehr sieht.

Ein Evidence-Rezept liefert Funde plus `level` und sonst nichts: keine Priorität `P1`–`P3`, keine Kategoriebuchstaben `S`/`T`/`K`/`F`/`A`/`X`, keine Bündelung. Priorisierung gehört ausschließlich in `commands/_audit/review-scan-triage.md`.

## Ergebnisform „Diff eines Tasks"

Neben Kommentarvorschlag, Ergebnisdokument und SARIF gibt es eine vierte Ergebnisform: das Review des Diffs, den ein Task hinterlassen hat. Abnehmer ist `/k-task-run`. Sie ist keine Betriebsart mit eigenem Frontmatter-Feld, sondern ein Abschnitt im Rezept.

- **Genau ein Katalogeintrag.** `/k-task-run` nutzt nur den Eintrag mit dem Schlüssel `code`, also die Datei `review-code.md`. Kein anderes Rezept trägt diesen Abschnitt.
- **Feste Überschrift.** Der Abschnitt steht unter einer festen Überschrift, an der das Modul des Commands ihn eindeutig erkennt. Welche das ist, legt das Rezept fest; die Regel verlangt, dass es sie gibt.
- **Inhalt.** Der Abschnitt enthält nur Gegenstand, Maßstab, die Einstufung der Funde für den Abnehmer und die aufgabengebundenen Kriterien samt ihrem Beleg. Aufgabengebunden heißen Kriterien, deren Maßstab der Task selbst oder die übergebenen Validierungsergebnisse sind. Welche das konkret sind und wie eingestuft wird, steht nur im Rezept; die Regel zählt sie nicht auf und definiert die Einstufung nicht.
- **Einzige Ausnahme vom Verbot eigener Kriterien.** Die aufgabengebundenen Kriterien sind die einzigen Kriterien, die nur in einer Ergebnisform gelten — nur hier ist der Task der Maßstab. Die gemeinsamen Kriterien werden auch in diesem Abschnitt nicht neu geschrieben; er wendet sie an.
- **Nicht im Rezept.** Ablauf, Form des Abschnitts in der Task-Datei und Rückgabeformat gehören ins Modul des Commands.
- **Overlay.** Ein projekteigenes `review-code.md` ersetzt das mitgelieferte vollständig und muss den Abschnitt deshalb selbst tragen, unter derselben Überschrift. Fehlt er, hält `/k-task-run` jeden Task fail-closed an.
- **Abschalten.** Einen eigenen Schalter gibt es nicht: `review.enabled` wirkt nur auf `/k-review`, `audit.enabled` nur auf `/k-audit`. Die Nutzung durch `/k-task-run` schaltet allein ein leeres projekteigenes `review-code.md` ab (`disabled: true` im Katalogeintrag).

## Grenzen

Ein Review-Rezept darf nicht duplizieren:

- Pfadauflösung aus `K-PLAYBOOK.yaml`.
- Laden von `known-decisions.md`.
- Logging nach `log.md`.
- Generischen Ablauf Scan, Rückfragen, Freigabe, Änderung, Abschluss.

Diese Punkte gehören in `/k-review`.

Zwei weitere Beschreibungen gehören ebenfalls nicht ins Rezept: das Schema von
`review-input.json` steht in `commands/_review-run/review-input-contract.md`, die Triage
mit Bündelung, Priorität und Kategorie in `commands/_audit/review-scan-triage.md`. Ein
Rezept verweist darauf und schreibt keine zweite Fassung — auch nicht als Auszug.

Bedient dieselbe Datei mehrere Ergebnisformen, darf sie **nur nach der Ergebnisform** geteilt werden: ein gemeinsamer Teil mit Prüfkriterien, Rule-IDs, `level` und Ausschlüssen, darunter je Ergebnisform das Stück, das nur dort gilt — der Kommentarvorschlag im interaktiven Modus, das Ergebnisdokument im Report-Modus, das SARIF im Evidence-Lauf, der Abschnitt für den Diff eines Tasks. Der generische Ablauf wird auch dann nicht wiederholt, und die Kriterien werden nicht je Betriebsart neu geschrieben: zwei Fassungen derselben Kriterien laufen auseinander, und dann findet dasselbe Rezept je nach Aufruf etwas anderes. Die einzige Ausnahme sind die aufgabengebundenen Kriterien im Abschnitt für den Diff eines Tasks (Abschnitt „Ergebnisform ‚Diff eines Tasks'" oben).

Für den Abschnitt „Diff eines Tasks" gehört der Ablauf nicht in `/k-review`, sondern ins Modul von `/k-task-run`; das Rezept wiederholt ihn nicht.

## Qualitätskriterium

Ein gutes Review-Rezept ist so spezifisch, dass zwei Reviewer mit demselben Scope ungefähr dieselben Kandidaten finden, aber so knapp, dass der generische Review-Prozess nicht im Rezept versteckt wird.
