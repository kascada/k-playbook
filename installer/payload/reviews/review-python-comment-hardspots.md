---
name: review-python-comment-hardspots
title: Python - Nicht-rekonstruierbare Entscheidungen kommentieren
language: python
interval-weeks: 16
scope-hint: Python-Quellen; Ausschluss: virtuelle Umgebungen, tests/fixtures
---

# Review: Python-Comment-Hardspots

Finde in Python-Code Stellen, an denen selbst ein erfahrener Reviewer mit Projektkenntnis nicht erkennen kann, warum der Code so geschrieben wurde wie er ist. Klaere den Grund, notfalls per Rueckfrage, und schlage dort einen aussagekraeftigen Kommentar vor.

Der generische Ablauf wird von `/k-review` orchestriert. Diese Datei beschreibt nur die reviewspezifischen Kriterien, Stil-Wahl und Beispiele.

## Zielgruppe des Codes

Sehr erfahrene Programmierer, die das Projekt kennen.

- Lokale Komplexitaet braucht keine Erklaerung.
- Domaenenwissen ist vorausgesetzt.
- Python-Idiome werden verstanden.

Es bleibt eine eng umrissene Restmenge: Stellen, an denen der Code anders aussieht als die naheliegende Loesung und der Grund dafuer nicht aus Code, Kontext oder Domaenenwissen ableitbar ist.

## Was kommentiert werden soll

Kandidaten sind Stellen, an denen die Antwort auf "warum nicht einfach so?" ausserhalb des sichtbaren Universums liegt.

Typische Faelle:

- Workarounds fuer externe Bugs oder Quirks.
- Reihenfolge-Abhaengigkeiten, die nicht aus dem Code folgen.
- Bewusste Abweichungen vom Naheliegenden.
- Magische Werte, deren Ursprung nicht erschliessbar ist.
- Unterdrueckte Fehler mit nicht-offensichtlichem Grund.
- Auskommentierter Code, der bewusst bleiben soll.
- Nicht-portierbare oder versionsabhaengige Konstrukte.
- Vertraege mit Code an anderer Stelle.

## Was nicht kommentiert werden soll

- Alles, was ein erfahrener Reviewer aus dem Code selbst erschliessen kann.
- Domaenenlogik.
- Standard-Patterns.
- Performance-Optimierungen, deren Wirkung offensichtlich ist.
- Sicherheitsmassnahmen, die als solche erkennbar sind.

Bei begruendetem Zweifel: nicht aufnehmen. Eine leere Fundliste ist ein gueltiges Ergebnis.

## Stil-Wahl beim Kommentar

- Inline am Zeilenende fuer sehr kurze Hinweise an einer einzelnen Zeile oder Konstante.
- Einzeiliger Kommentar darueber, wenn der Hinweis kurz ist und sich auf wenige Zeilen bezieht.
- Block-Kommentar ueber dem Abschnitt, wenn mehrere Saetze noetig sind.
- Docstring nur, wenn die nicht-rekonstruierbare Entscheidung das Wesen der gesamten Funktion betrifft.

## Kommentar-Qualitaetskriterien

- Erklaert das Warum, das man nicht erschliessen kann.
- Nennt die externe Ursache konkret.
- Verweist auf Quellen, wo moeglich.
- Ist so kurz wie moeglich.
- Altert gut.
- Steht in der Sprache des umgebenden Codes.

## Anti-Muster

- Lokale Komplexitaet kommentieren.
- Domaenenwissen kommentieren.
- Leere Liste vermeiden wollen.
- Vermutungen als Fakten formulieren.
- Das Was erklaeren statt das Warum.
