---
title: Eindeutige Review-Befunde nachbessern
description: Modul für /k-task-run, vom Hauptkontext gelesen in 5f nach einem gültigen Review mit kritischen oder wichtigen Befunden — die Schleife aus Behebung und erneutem Review (höchstens drei Runden, Kreis-Erkennung, Sperrprüfung und REVISION je Runde, Abbruchgründe) und wie sie endet.
---

# Eindeutige Review-Befunde nachbessern

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5f, nach dem Review und vor der
Stopp-Abfrage, wenn das Review **gültig** ist (Ausgang „gültig, ohne kritischen Befund“ oder
„gültig, mit kritischem Befund“ der Tabelle „Ausgänge des Reviews“), `Kritisch:` oder `Wichtig:`
mindestens eine Befundzeile trägt und `REVIEW_SCOPE` nicht `nur seit Fortsetzung` ist. Bei
einem ungültigen Review, fehlendem oder abgeschaltetem Rezept und ohne Diff läuft keine
Schleife. Er hat `TASK_REF`, `REVISION`, den Rahmen aus 5c (Ausführungswurzel, `projectDir`,
Branch, Preflight), `SNAPSHOT`, `REVIEW_SCOPE`, `DIFF_REVIEW_MODULE` und die Rückgabe des
Reviews mit ihren Befundzeilen — mehr sieht er von den Befunden nicht.

**Idee.** Was eindeutig behebbar ist, wird ohne Rückfrage behoben und von einem zweiten,
unabhängigen Sub-Agenten geprüft; was Zweifel lässt, bleibt als Rest. Maßstab ist
„eindeutig“, nicht „klein“.

**Eine Runde** besteht aus Behebung und erneutem Review. Es gibt höchstens **drei Runden**.

1. **Sperre prüfen.** `check` der Sperre vor dem Start der Behebung. Ist sie verloren, endet
   die Schleife (Abbruch unten).
2. **Behebung.** Der Hauptkontext startet einen General-Purpose-Sub-Agenten (OpenCode:
   `general`, Claude Code: `general-purpose`) — nicht den Review-Sub-Agenten — nach
   `k-playbook:/commands/_task-run/ausfuehrung.md`, Abschnitt „Behebung“; dort steht der
   Auftrag, hier keine zweite Fassung. Er bekommt den Verweis des Moduls, `TASK_REF`,
   `REVISION`, den Rahmen aus 5c und die kritischen und wichtigen Befundzeilen der letzten
   Rückgabe des Reviews, die noch in der Schleife sind (Kreis-Erkennung unten). Seine
   Rückgabe hat das Format aus 5c; `REVISION` wird danach nach der Revisionskette fortgeschrieben
   (die übergebene, da er nichts schreibt). Nicht Behobenes nennt er mit Grund unter
   `Zusammenfassung:`.
   - `erfolgreich`: alles Eindeutige ist behoben. Befunde, die seine `Zusammenfassung:` als
     nicht behoben nennt, zählt der Hauptkontext als nicht behoben und damit als Rest; sie
     scheiden aus der Schleife aus.
   - `teilweise` oder `blockiert`: technisches Scheitern; die Schleife endet sofort, ohne
     weitere Runde.
3. **Sperre prüfen, dann erneutes Review.** `check` der Sperre, dann liest der Hauptkontext
   `k-playbook:/commands/_task-run/review-starten.md` und folgt ihm: Das Review läuft erneut
   gegen denselben Snapshot mit demselben Umfang und hängt seine Abschnitte an; die früheren
   bleiben. `REVISION` wird nach jeder Rückgabe fortgeschrieben. Ist das Review ungültig,
   endet die Schleife; das letzte gültige Review bleibt maßgeblich.
4. **Weiter oder Ende.** Gibt es nach dem Review noch kritische oder wichtige Befunde, die
   nicht ausgeschieden sind, und waren es weniger als drei Runden, beginnt die nächste Runde.

**Kreis-Erkennung.** Der Hauptkontext vergleicht die Rückgaben der Reviews. Ein Befund ist
derselbe, wenn Datei (ohne Zeilennummer) und Kernaussage übereinstimmen; im Zweifel gilt
„gleich“. Ist ein Befund nach seiner Behebung noch oder wieder da, scheidet er aus der
Schleife aus: Er geht nicht mehr an den nächsten Behebenden und zählt am Ende als Rest.
Die übrigen Befunde laufen weiter. Befunde, die erst durch eine Behebung entstehen, laufen
in der nächsten Runde mit oder bleiben als Rest.

**Abbruch.** Die Schleife endet sofort, ohne weitere Runde, bei `teilweise` oder `blockiert`
der Behebung, bei ungültigem Review, bei verlorener Sperre und bei `Foreign-Change:`. Die
beiden letzten führen immer, auch ohne kritischen Rest, in die vorhandene Rückfrage (Sperre
verloren bzw. 5d, fremder Schreiber); der Hauptkontext schreibt dabei nichts weiter und setzt
nach der Antwort 5f dort fort, wo die Rückfrage es vorsieht.

**Ende.** Das letzte gültige Review der Schleife ist maßgeblich. Der Hauptkontext merkt sich
für Schritt 7 die Zahl der Runden, der behobenen und der offenen Befunde.

- Verbleiben kritische Befunde (auch ausgeschiedene und nicht eindeutige): weiter mit der
  Stopp-Abfrage (`stopp-abfrage.md`); sie nennt, was versucht wurde. Ein Abbruch ohne
  kritischen Rest geht nicht in die Abfrage.
- Verbleiben nur wichtige: „Befunde sammeln“ und Todos wie bisher; behobene wichtige Befunde
  werden nicht mehr gesammelt.
- Verbleibt nichts: weiter mit 5f, Schritt 6.

Die Schleife läuft genau einmal je Task und Review; nach „(a) beheben lassen“ in der
Stopp-Abfrage kommt sie nicht wieder.
