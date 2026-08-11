# Regel: Review-Rezepte erzeugen und pflegen

## Zweck

Ein Review-Rezept beschreibt nur die reviewspezifischen Kriterien. Der generische Ablauf gehoert in `/k-review`, nicht in einzelne Review-Dateien.

## Ablage

Globale Review-Rezepte liegen unter:

`<playbook.dir>/reviews/`

Projekteigene Review-Rezepte liegen unter:

`<local.dir>/reviews/`

Review-Ergebnisse liegen unter `<local.dir>/results/`, nicht bei den Rezepten und nicht unter `checks/`. `reviews/` enthaelt nur Rezepte, `checks/` nur ausfuehrbare Pruefroutinen.

Konvention fuer Report-/Scan-Familien:

`<local.dir>/results/<scan-family>/YYYY-MM-DD/`

Typische Dateien darin:

- `assessment.md` — kuratierte Gesamtbewertung.
- `findings.md` — vollstaendiges Finding-Register.
- `raw/` — maschinenlesbare Rohdaten wie SARIF, JSON oder Tool-Logs.

## Dateinamen

- Review-Rezepte heissen `review-<name>.md`.
- Der Name im Frontmatter entspricht dem Dateinamen ohne `.md`.
- Projektlokale Review-Rezepte duerfen globale Rezepte mit gleichem Dateinamen ueberlagern.

## Frontmatter

Jedes Review-Rezept enthaelt YAML-Frontmatter mit mindestens:

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
```

`result-family` kennzeichnet Report-/Scan-Familien, deren Ergebnisse unter `<local.dir>/results/<family-name>/YYYY-MM-DD/` liegen und typischerweise `assessment.md`, `findings.md`, `raw/` und ggf. Run-Metadaten enthalten.

## Inhalt

Ein Review-Rezept soll enthalten:

- Ziel des Reviews.
- Was als Finding zaehlt.
- Was ausdruecklich nicht als Finding zaehlt.
- Bewertungskriterien oder Anti-Muster.
- Bei interaktiven Reviews: welche Vorschlaege gemacht werden duerfen.
- Bei Report-Reviews: wohin der Handoff geht.

## Grenzen

Ein Review-Rezept darf nicht duplizieren:

- Pfadauflosung aus `K-PLAYBOOK.yaml`.
- Laden von `known-decisions.md`.
- Logging nach `log.md`.
- Generischen Ablauf Scan, Rueckfragen, Freigabe, Aenderung, Abschluss.

Diese Punkte gehoeren in `/k-review`.

## Qualitaetskriterium

Ein gutes Review-Rezept ist so spezifisch, dass zwei Reviewer mit demselben Scope ungefaehr dieselben Kandidaten finden, aber so knapp, dass der generische Review-Prozess nicht im Rezept versteckt wird.
