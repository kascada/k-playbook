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
  defaultResult: review-<name>.md
  resultRequired: <true|false>
  scope:
    tools: [<tool>, <tool>]
review:
  enabled: <true|false>
```

`result-family` kennzeichnet Report-/Scan-Familien, deren Ergebnisse unter `<local.dir>/results/<family-name>/YYYY-MM-DD/` liegen und typischerweise `review-input.json`, `review-triage.md`, `raw/` und ggf. Run-Metadaten enthalten. `audit.enabled` steuert Kandidaten für `/k-audit`-/MCP-Läufe; `review.enabled` steuert die gezielte `/k-review`-Auswahl. Aktive Audit-Rezepte brauchen `defaultResult`, `resultRequired` und `scope.tools`; sie laufen als Perspektive auf `review-input.json`, nicht als eigener Scanner.

## Inhalt

Ein Review-Rezept soll enthalten:

- Ziel des Reviews.
- Was als Finding zählt.
- Was ausdrücklich nicht als Finding zählt.
- Bewertungskriterien oder Anti-Muster.
- Bei interaktiven Reviews: welche Vorschläge gemacht werden dürfen.
- Bei Report-Reviews: wohin der Handoff geht.

## Grenzen

Ein Review-Rezept darf nicht duplizieren:

- Pfadauflosung aus `K-PLAYBOOK.yaml`.
- Laden von `known-decisions.md`.
- Logging nach `log.md`.
- Generischen Ablauf Scan, Rückfragen, Freigabe, Änderung, Abschluss.

Diese Punkte gehören in `/k-review`.

## Qualitätskriterium

Ein gutes Review-Rezept ist so spezifisch, dass zwei Reviewer mit demselben Scope ungefähr dieselben Kandidaten finden, aber so knapp, dass der generische Review-Prozess nicht im Rezept versteckt wird.
