---
name: review-tech
title: Tech-Debt-Analyse
interval-weeks: 24
scope-hint: Quell- und Infrastruktur-Verzeichnisse; Ausschluss - priv/, secure/, tasks/, virtuelle Umgebungen
handoff: /k-remediation
---

# Review: Tech-Debt

Vollständige Analyse aller Tech-Debt-Kandidaten im Projekt mit anschließender Übergabe an `/k-remediation`.

> Der generische Rahmen (known-decisions lesen, Log-Eintrag nach Abschluss) wird von `/k-review` orchestriert. Diese Datei beschreibt nur die reviewspezifische Analyse.

## Ablauf-Besonderheit

Dieses Review erzeugt **keine** interaktiven Freigaben pro Fund (anders als die meisten Reviews). Es produziert ein vollständiges Ergebnis-Dokument, das anschließend im Rahmen von `/k-remediation` einzeln durchgegangen wird.

Der Command übergibt der Analyse einen Pfad für die Ausgabe-Datei (aufgelöst aus K-PLAYBOOK.MD, üblich: `<reviews>/result-review-tech.md`).

## Analyse

Nutze `/engineering:tech-debt` mit folgender Direktive:

- Analysiere die Quell- und Infrastruktur-Verzeichnisse des Projekts.
- Halte die Ausschlüsse aus `scope-hint` ein (`priv/`, `secure/`, `tasks/`, virtuelle Umgebungen).
- Kategorisiere und priorisiere alle Tech-Debt-Kandidaten.
- **Keine Code-Änderungen.**
- Schreibe das vollständige Ergebnis als Markdown in die vom Command übergebene Ausgabe-Datei.

## Handoff

Nach Abschluss der Analyse (und dem Log-Eintrag, den der Command setzt):

- Neuen Chat starten.
- `/k-remediation <ausgabe-datei>` aufrufen.

Der Command teilt dem User Pfad und exakten Handoff-Befehl am Ende mit.
