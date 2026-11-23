---
title: Stopp-Abfrage nach dem Review
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn nach dem Review ein Stopp-Grund vorliegt — die Abfrage mit den Optionen beheben lassen oder Review erneut, trotzdem weiter, anhalten, und was jede Option tut.
---

# Stopp-Abfrage nach dem Review

Dieses Modul liest der Hauptkontext von `/k-task-run`, wenn in 5f, Schritt 5, ein Stopp-Grund
vorliegt (Tabelle „Ausgänge des Reviews“ im Command). Die Fertig-Meldung
(`Rückfrage: <Grund>`, der Body nennt die Zahlen) ist schon gesendet. Er hat `TASK_REF`,
`REVISION`, den Rahmen aus 5c und die Rückgabe des Reviews.

**Gründe.** Mehrere Gründe stehen in einer Abfrage; (a) steht zur Wahl, solange einer mit (a)
dabei ist.

| Grund | Die Abfrage nennt | (a) |
|---|---|---|
| gültiges Review mit mindestens einem kritischen Befund | die Zeilen unter „Kritisch“ aus der Rückgabe | beheben lassen |
| kein gültiges Review: Abbruch, Rückgabe nicht im Format oder kein neuer Review-Abschnitt | dass kein gültiges Review vorliegt, dazu eine Zeile `Abbruch:` | Review erneut laufen lassen |
| Rezept fehlt im Briefing | dass kein gültiges Review vorliegt, weil `review-code` unter `documents.reviews` fehlt (nur diese Zeile, nicht die vorige — ein erneutes Review fände das Rezept ebenso wenig) | entfällt |
| `REVIEW_SCOPE` ist `nur seit Fortsetzung` | „Review nur seit Fortsetzung“ | entfällt |

Für kritische Befunde (nach der Schleife aus `nachbessern.md` die Reste):

```
<TASK_FILE>: <n> kritische Review-Befunde
  - <datei:zeile oder Prüfbefehl> — <ein Satz>
```

Lief die Schleife, nennt die Abfrage darunter in einer Zeile, was versucht wurde (Runden,
behoben, offen, welche Befunde nicht behoben oder ausgeschieden sind), dann:

```
Wie weiter?
  (a) beheben lassen   - Sub-Agent behebt nur diese Befunde, danach erneutes Review
  (b) trotzdem weiter  - Task gilt als erledigt, Befunde bleiben dokumentiert
  (c) anhalten         - Task bleibt liegen, keine weiteren Tasks
```

Bei den anderen Gründen steht statt der Befundzeilen `<TASK_FILE>: kein gültiges Review —
<Grund>` bzw. `<TASK_FILE>: Review nur seit Fortsetzung — ein Fehler aus dem früheren
Versuch bleibt ungeprüft`.

- **(a) beheben lassen.** Nach der Schleife aus `nachbessern.md` ist das eine ausdrückliche
  Entscheidung des Nutzers: genau ein Behebungslauf mit anschließendem Review, nie zurück in
  die Schleife oder ihre Weiche. Nach `check` der Sperre startet der Hauptkontext einen
  General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code: `general-purpose`) nach
  `k-playbook:/commands/_task-run/ausfuehrung.md`, Abschnitt „Behebung“ — der Auftrag steht nur
  dort. Er bekommt den Verweis
  des Moduls, `TASK_REF`, `REVISION`, den Rahmen aus 5c (Ausführungswurzel, `projectDir`,
  Branch, Preflight) und die kritischen Befunde. Seine Rückgabe hat das Format aus 5c
  (`Revision:` ist dann die übergebene).
  - `erfolgreich`: zurück zu 5f, Schritt 4, ohne 4a — das Review läuft erneut gegen denselben Snapshot
    mit demselben Umfang und hängt seine Abschnitte an; die früheren bleiben. Bleibt etwas
    kritisch, wird wieder gefragt.
  - `blockiert`: fragen wie in 5d. „Erneut“ heißt hier nur, den Behebungs-Sub-Agenten mit
    der Antwort des Nutzers noch einmal zu starten — nie zurück nach 5a. Wird er nicht
    erneut gestartet, weiter wie (c) mit dem Grund „Behebung blockiert, kritische
    Review-Befunde offen“.
  - `teilweise`: weiter wie (c) mit dem Grund „Behebung unvollständig, kritische
    Review-Befunde offen“.

  Die Vorlage aus 5e kommt dabei nicht vor.
- **(a) Review erneut laufen lassen** (kein gültiges Review): zurück zu 5f, Schritt 4 mit
  denselben Eingaben.
- **(b) trotzdem weiter.** Weiter mit 5f, Schritt 6. Liegt kein gültiges Review vor und hat der
  letzte Review-Lauf keine Diff-Abschnitte geschrieben (Zählung aus 5f, Schritt 4), startet der
  Hauptkontext davor einen Sub-Agenten nur mit Teil 1 des Moduls (`DIFF_REVIEW_MODULE`).
- **(c) anhalten.** Mit `k_playbook_doc_append` und `since_revision: REVISION` anhängen; die
  bestehende Statuszeile bleibt stehen:

  ```text
  **Status nach Review:** angehalten — <Grund>
  ```

  Grund ist „kritische Review-Befunde offen“, „kein gültiges Review“, „Review nur seit
  Fortsetzung“ oder der Grund aus (a). Der Task bleibt liegen, die Fortschrittszeilen
  unverändert; kein PR, keine weiteren Tasks. Die Kennzeile aus 5f, Schritt 2 steht schon im
  Dokument; den nächsten Lauf regelt 5a.
