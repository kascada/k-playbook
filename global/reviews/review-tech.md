---
name: review-tech
title: Tech-Debt-Analyse
interval-weeks: 24
scope-hint: Quell- und Infrastruktur-Verzeichnisse; Ausschluss - priv/, secure/, tasks/, virtuelle Umgebungen
handoff: /k-remediation
---

# Review: Tech-Debt

Vollstaendige Analyse aller Tech-Debt-Kandidaten im Projekt mit anschliessender Uebergabe an `/k-remediation`.

Der generische Rahmen wird von `/k-review` orchestriert. Diese Datei beschreibt nur die reviewspezifische Analyse.

## Ablauf-Besonderheit

Dieses Review erzeugt keine interaktiven Freigaben pro Fund. Es produziert ein vollstaendiges Ergebnis-Dokument, das anschliessend im Rahmen von `/k-remediation` einzeln durchgegangen wird.

Der Command uebergibt der Analyse einen Pfad fuer die Ausgabe-Datei unter `k-playbook/reviews/`, ueblich: `k-playbook/reviews/result-review-tech.md`.

## Analyse

Nutze `/engineering:tech-debt` mit folgender Direktive:

- Analysiere die Quell- und Infrastruktur-Verzeichnisse des Projekts.
- Halte die Ausschluesse aus `scope-hint` ein.
- Kategorisiere und priorisiere alle Tech-Debt-Kandidaten.
- Keine Code-Aenderungen.
- Schreibe das vollstaendige Ergebnis als Markdown in die vom Command uebergebene Ausgabe-Datei.

## Handoff

Nach Abschluss der Analyse und dem Log-Eintrag nennt `/k-review` Pfad und exakten Handoff-Befehl:

`/k-remediation <ausgabe-datei>`
