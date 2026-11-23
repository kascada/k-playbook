---
title: Abschließende Zusammenfassung
description: Modul für /k-task-run, vom Hauptkontext gelesen am Ende von Schritt 7 — das Format der Zusammenfassung (Ausgeführt, Erfolgreich, Abgeschlossen, Angehalten, Abgebrochen, Refine, Sperre, Intent, Todos, Doku) und wie die Zeilen „Review“, „Sperre“ und „Refine“ gefüllt werden.
---

# Abschließende Zusammenfassung

Dieses Modul liest der Hauptkontext von `/k-task-run` am Ende von Schritt 7, nach der
Fertig-Meldung `fertig` oder `abgebrochen`. Er hat die Ergebnisse aller Tasks, die gesammelten
Befunde und die Zeilen „Todos“ und „Doku“ aus Schritt 7. Die knappe Zusammenfassung:

```
Ausgeführt:    <n> Tasks
Erfolgreich:   <Verweise unter done/>
  <TASK_FILE>  Review: <n> kritisch, <n> wichtig, <n> Hinweis
    - kritisch: <datei:zeile oder Prüfbefehl> — <ein Satz>
    - <datei:zeile oder Prüfbefehl> — <ein Satz>
    - Hinweis: <datei:zeile oder Prüfbefehl> — <ein Satz>
Abgeschlossen: <Tasks, die der Nutzer in 5a, Option (a), abgeschlossen hat>
  <TASK_FILE>  Review: <n> kritisch, <n> wichtig, <n> Hinweis — letzter Lauf
    - kritisch: <datei:zeile oder Prüfbefehl> — <ein Satz>
    - Hinweis: <datei:zeile oder Prüfbefehl> — <ein Satz>
Angehalten:    <TASK_FILE> - <Grund>
Abgebrochen:   <TASK_FILE, falls vorhanden> - <Grund>
Übersprungen:  <TASK_FILEs, falls vorhanden>
Refine:        <je Ziel: TASK_FILE — Ergebnis, Runden, gefixt, offen> | – (ohne refine)
Nachbessern:   <n> Runden, behoben <n>, offen <n> | – (keine Schleife)
Sperre:        sofort genommen | nach <dauer> Warten auf <holder> | übernommen von <holder>
               [; verloren — <reason>] [; Freigabe meldete lost]
Intent:        Ja / Teilweise / Nein / - (kein Intent)
Todos:         angelegt <kennung> [, <kennung>] | keine übernommen | – (keine Befunde)
Doku:          eingereiht <verweis> [, <verweis>] | schon eingereiht <verweis> | kein Code geändert
               | keine erzeugte Doku | offen — <Grund>, /k-enforcement  [; Inventar: /k-doc-inventory]
```

Die Zeile „Review“ steht je erfolgreichem Task und gibt das letzte gültige Review wieder, wie
die letzte Spalte der Tabelle „Ausgänge des Reviews“ (5f) sie nennt. Unter ihr stehen die für
diesen Task gesammelten Befundzeilen, kritische mit dem Vorsatz `kritisch:`, danach die Hinweiszeilen mit dem Vorsatz `Hinweis:`;
ohne gesammelte Zeilen keine. Fehlt der Block `Hinweise:` oder ist er formwidrig, oder stimmt
die Zahl in der Zeile nicht mit den Hinweiszeilen überein, steht hinter der Zeile „Review“ der
Zusatz „— Hinweise nicht übermittelt“.

Je Task unter „Abgeschlossen“ steht die Zeile `Review:` aus der Rückgabe des Befund-Lesers mit
dem Zusatz „— letzter Lauf“, darunter die gesammelten Zeilen und Hinweise wie oben. Bei
`Review: kein Abschnitt` lautet sie `Review: kein Abschnitt im letzten Lauf`, bei einer
Rückgabe in anderer Form `Review: Befunde nicht gelesen`.

Die Zeile „Nachbessern“ steht je Task, für den die Schleife aus `nachbessern.md` lief, mit den
Zahlen, die der Hauptkontext dort gemerkt hat.

Die Zeile „Sperre“ nennt die Wartezeit aus „Sperre nehmen“, einen übernommenen Halter und
einen Verlust samt Grund; „Refine“ die Zeilen aus dem Modus `refine` (Schritt 3), auch für
Tasks, die auf die Rückfrage zurückfielen.

Bei „Abgeschlossen“ steht ein offener PR-Schritt dabei, wenn der Task `PR required: true`
trägt. Ein nach `foreign_change` liegen gebliebener Task steht unter „Angehalten“ mit dem
Grund „fremder Schreiber“.
