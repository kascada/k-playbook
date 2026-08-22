---
name: review-tech
title: Tech-Debt-Analyse
interval-weeks: 24
scope-hint: Quell- und Infrastruktur-Verzeichnisse; Ausschluss - priv/, secure/, tasks/, virtuelle Umgebungen
handoff: /k-remediation
audit:
  enabled: false
  title: Tech-Debt-Analyse
  resultRequired: true
  defaultResult: review-tech.md
review:
  enabled: true
---

# Review: Tech-Debt

Vollständige Analyse aller Tech-Debt-Kandidaten im Projekt mit anschließender Übergabe an `/k-remediation`.

Der generische Rahmen wird von `/k-review` orchestriert. Diese Datei beschreibt nur die reviewspezifische Analyse.

## Ablauf-Besonderheit

Dieses Review erzeugt keine interaktiven Freigaben pro Fund. Es produziert ein vollständiges Ergebnis-Dokument, das anschließend im Rahmen von `/k-remediation` einzeln durchgegangen wird.

Der Command übergibt der Analyse einen Pfad für die Ausgabe-Datei unter `k-playbook-local/results/`, üblich: `k-playbook-local/results/summary-YYYY-MM-DD.md`.

## Analyse

Nutze `/engineering:tech-debt` mit folgender Direktive:

- Analysiere die Quell- und Infrastruktur-Verzeichnisse des Projekts.
- Halte die Ausschlüsse aus `scope-hint` ein.
- Kategorisiere und priorisiere alle Tech-Debt-Kandidaten.
- Keine Code-Änderungen.
- Schreibe das vollständige Ergebnis als Markdown in die vom Command übergebene Summary-Datei.

## Handoff

Nach Abschluss der Analyse und dem Log-Eintrag nennt `/k-review` Pfad und exakten Handoff-Befehl:

`/k-remediation <ausgabe-datei>`
