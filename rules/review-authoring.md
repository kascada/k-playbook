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

- `review-input.json` — strukturierter Belegvertrag mit Scope, Gruppen, Evidence und Known-Decision-Coverage.
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

Optional:

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

`result-family` kennzeichnet Report-/Scan-Familien, deren Ergebnisse unter `<local.dir>/results/<family-name>/YYYY-MM-DD/` liegen und typischerweise `review-input.json`, `review-triage.md`, `raw/` und ggf. Run-Metadaten enthalten. `audit.enabled` steuert Kandidaten für `/k-audit`-/MCP-Läufe; `review.enabled` steuert die gezielte `/k-review`-Auswahl. Welche weiteren Felder der `audit`-Block trägt, hängt an `audit.mode` und steht im nächsten Abschnitt.

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

Bei `mode: evidence` kommt dazu:

- Die Rule-IDs aus `audit.ruleIds` mit je einem Satz, was ein Fund dieser Rule-ID zeigt. Die Liste ist der Vertrag über die Vergleichbarkeit; ohne Erklärung im Text ist sie nicht anwendbar.
- Das `level` je Rule-ID. Es ist die einzige Wertung, die ein Evidence-Rezept vergibt, und es entscheidet über die Schwere im Merge: `error` und `note` gelten unverändert, `warning` und `none` laufen weiter über CVSS, Tool-Metadaten und `scripts/severity.tsv`. Ein `warning` ist damit die schwächste Aussage und kein Urteil.
- Einen Mengendeckel je Lauf samt Auswahlregel, welche Funde bei Überschreitung stehen bleiben. Er bleibt Anweisung im Text und wird beim Melden nicht erzwungen — eine Kappung dort verwürfe Funde, die danach niemand mehr sieht.

Ein Evidence-Rezept liefert Funde plus `level` und sonst nichts: keine Priorität `P1`–`P3`, keine Kategoriebuchstaben `S`/`T`/`K`/`F`/`A`/`X`, keine Bündelung. Priorisierung gehört ausschließlich in `commands/_audit/review-scan-triage.md`.

## Grenzen

Ein Review-Rezept darf nicht duplizieren:

- Pfadauflösung aus `K-PLAYBOOK.yaml`.
- Laden von `known-decisions.md`.
- Logging nach `log.md`.
- Generischen Ablauf Scan, Rückfragen, Freigabe, Änderung, Abschluss.

Diese Punkte gehören in `/k-review`.

Bedient dieselbe Datei zwei Betriebsarten, darf sie **nur nach der Ergebnisform** zweigeteilt werden: ein gemeinsamer Teil mit Prüfkriterien, Rule-IDs, `level` und Ausschlüssen, darunter je Betriebsart das Stück, das nur dort gilt — der Kommentarvorschlag im interaktiven Modus, das Ergebnisdokument im Report-Modus, das SARIF im Evidence-Lauf. Der generische Ablauf wird auch dann nicht wiederholt, und die Kriterien werden nicht je Betriebsart neu geschrieben: zwei Fassungen derselben Kriterien laufen auseinander, und dann findet dasselbe Rezept je nach Aufruf etwas anderes.

## Qualitätskriterium

Ein gutes Review-Rezept ist so spezifisch, dass zwei Reviewer mit demselben Scope ungefähr dieselben Kandidaten finden, aber so knapp, dass der generische Review-Prozess nicht im Rezept versteckt wird.
