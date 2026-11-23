---
title: Liste neu prüfen
description: Modul für /k-task-run, vom Hauptkontext gelesen direkt nach dem Nehmen der Sperre — die Task-Liste aus Schritt 2 gegen inzwischen geänderten Stand prüfen (erledigte oder inzwischen ausgeführte Tasks fallen heraus, Revisionsabgleich, leere Liste beendet den Lauf).
---

# Liste neu prüfen

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 5, direkt nach dem Nehmen der
Sperre (auch nach dem Warten). Er hat die Liste aus Schritt 2 mit `SCHON_AUSGEFUEHRT` je Task
und `LOCK_TOKEN`.

Nach dem Nehmen, auch nach dem Warten, prüft der Hauptkontext die
Liste aus Schritt 2 neu. Heraus fällt nur, was sich seit Schritt 2 geändert hat: ein Task, der
inzwischen unter `done/` liegt oder nicht mehr gefunden wird (ein `not_found` direkt nach dem
Wieder-Öffnen ist Replica-Nachlauf, kein Grund), und ein Task mit `SCHON_AUSGEFUEHRT = nein`,
an dem die Kennzeile „ausgeführt“ jetzt trifft (Suche und Regel aus 5a). Ein solcher Task fällt
ohne Rückfrage heraus und wird in der Ausgabe genannt; alles andere bleibt und läuft durch 5a,
auch ein schon vor Schritt 2 ausgeführter, nicht abgeschlossener Task. Hält der Hauptkontext
für einen Task schon eine eigene `REVISION` (nach dem Wieder-Öffnen), vergleicht er sie mit der
Revision dieser Prüfung; weicht sie nach dem Replica-Nachlauf ab, gilt `foreign_change`
(Schritt 1), kein stilles Neuholen. Für die übrigen Tasks setzt 5a `REVISION`.

Hat sich die Liste geändert, folgt „Intent und Tools lesen“ (Schritt 2) erneut. Ist sie leer,
gibt er die Sperre frei, meldet das und endet mit der Fertig-Meldung `fertig` — Schritt 6 und 7
entfallen.
