---
name: review-python-comment-hardspots
title: Python - Nicht-rekonstruierbare Entscheidungen kommentieren
language: python
interval-weeks: 16
scope-hint: Python-Quellen; Ausschluss: virtuelle Umgebungen, tests/fixtures
audit:
  enabled: false
review:
  enabled: true
---

# Review: Python-Comment-Hardspots

Finde in Python-Code Stellen, an denen selbst ein erfahrener Reviewer mit
Projektkenntnis nicht erkennen kann, warum der Code so geschrieben wurde wie er ist.
Kläre den Grund, notfalls per Rückfrage, und schlage dort einen aussagekräftigen
Kommentar vor.

Der generische Ablauf wird von `/k-review` orchestriert. Dieses Rezept bleibt im
Audit-Laufmodell deaktiviert, weil Code-Hotspots derzeit nicht als Evidence in
`review-input.json` vorliegen und dieser Task keinen alternativen Scope-Typ einführt.

## Zielgruppe des Codes

Sehr erfahrene Programmierer, die das Projekt kennen.

- Lokale Komplexität braucht keine Erklärung.
- Domänenwissen ist vorausgesetzt.
- Python-Idiome werden verstanden.

Es bleibt eine eng umrissene Restmenge: Stellen, an denen der Code anders aussieht als
die naheliegende Lösung und der Grund dafür nicht aus Code, Kontext oder Domänenwissen
ableitbar ist.

## Was kommentiert werden soll

Kandidaten sind Stellen, an denen die Antwort auf „warum nicht einfach so?" außerhalb
des sichtbaren Universums liegt.

Typische Fälle:

- Workarounds für externe Bugs oder Quirks.
- Reihenfolge-Abhängigkeiten, die nicht aus dem Code folgen.
- Bewusste Abweichungen vom Naheliegenden.
- Magische Werte, deren Ursprung nicht erschließbar ist.
- Unterdrückte Fehler mit nicht-offensichtlichem Grund.
- Auskommentierter Code, der bewusst bleiben soll.
- Nicht-portierbare oder versionsabhängige Konstrukte.
- Verträge mit Code an anderer Stelle.

## Was nicht kommentiert werden soll

- Alles, was ein erfahrener Reviewer aus dem Code selbst erschließen kann.
- Domänenlogik.
- Standard-Patterns.
- Performance-Optimierungen, deren Wirkung offensichtlich ist.
- Sicherheitsmaßnahmen, die als solche erkennbar sind.

Bei begründetem Zweifel: nicht aufnehmen. Eine leere Fundliste ist ein gültiges Ergebnis.

## Stil-Wahl beim Kommentar

- Inline am Zeilenende für sehr kurze Hinweise an einer einzelnen Zeile oder Konstante.
- Einzeiliger Kommentar darüber, wenn der Hinweis kurz ist und sich auf wenige Zeilen
  bezieht.
- Block-Kommentar über dem Abschnitt, wenn mehrere Sätze nötig sind.
- Docstring nur, wenn die nicht-rekonstruierbare Entscheidung das Wesen der gesamten
  Funktion betrifft.

## Kommentar-Qualitätskriterien

- Erklärt das Warum, das man nicht erschließen kann.
- Nennt die externe Ursache konkret.
- Verweist auf Quellen, wo möglich.
- Ist so kurz wie möglich.
- Altert gut.
- Steht in der Sprache des umgebenden Codes.

## Anti-Muster

- Lokale Komplexität kommentieren.
- Domänenwissen kommentieren.
- Leere Liste vermeiden wollen.
- Vermutungen als Fakten formulieren.
- Das Was erklären statt das Warum.

## Handoff

Dieses Rezept bleibt über `/k-review python-comment-hardspots` auswählbar. Eine spätere
Aktivierung im Audit-Laufmodell braucht einen separaten Vertrag, der Code-Hotspots als
Evidence in `review-input.json` bringt.
