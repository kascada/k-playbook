# k-task-refine

Härtet Tasks und Instruction-Dateien vor der Ausführung über einen strukturierten
Zwei-Agenten-Dialog zwischen **Critic** und **Editor**. Critic und Editor sind read-only
Advisors. Der Moderator routet zwischen ihnen, entscheidet Deadlocks, wendet akzeptierte
Edits an und hängt ein Review-Log an. Ein finaler Alignment-Check prüft das Ergebnis gegen
den angegebenen Intent.

- `/k-task-refine` — die eigenen offenen Tasks, die noch kein Review-Log tragen. Der
  Hauptkontext teilt sie in Gruppen zusammenhängender Tasks; jede Gruppe prüft ein eigener
  Sub-Agent, alle parallel. Zum Schluss sucht ein Critic Überschneidungen zwischen den
  Gruppen und mit den schon geprüften offenen Tasks.
- `/k-task-refine <ziel>` — ein `keph:`-Verweis auf einen Task, eine Tasknummer oder eine
  Instruction-Datei bzw. ein Verzeichnis mit `.md`-Dateien auf der Platte. Trägt das Ziel
  schon ein Review-Log, fragt der Command vorher nach.
- `/k-task-refine <ziel> nochmal` — dasselbe ohne diese Rückfrage. Statt `nochmal` gelten
  auch `erneut` und `again`.
- `/k-task-refine nochmal` — das Wort allein, ohne Ziel: alle eigenen offenen Tasks, auch die
  schon geprüften, in Gruppen wie ohne Argument.

Produces:

- Ein `## Review-Log (<now.date>)` am Ende jedes geprüften Tasks, angehängt mit
  `k_playbook_doc_append`, bzw. jeder geprüften Datei — auch ohne Änderung, dann mit dem
  Vermerk „keine Änderungen“.
- Änderungen an den geprüften Tasks und Dateien, angewendet allein vom Moderator: an einem
  Task mit `k_playbook_doc_replace`, an einer Plattendatei mit dem Edit-Werkzeug.
- Eine Zusammenfassung über alle Ziele am Ende, im Lauf mit Gruppen samt den gefundenen
  Überschneidungen zwischen Tasks. Sie wird nur ausgegeben, nicht abgelegt.

Prozessregeln (Sub-Agenten nur lesend, der Moderator als einziger Schreiber, der Stand nach
jedem Edit neu gelesen, Audit-Spur, schneller Weg) stehen in
`k-playbook:/commands/_task-refine/dialog.md`.

## Schritt 1 — Briefing und Form

include: _shared/erster-schritt.md

Verbindlich für Ablage, Nummernauflösung, Kennzeilen und den Schreibweg eines Tasks ist
`k-playbook:/instructions/on-demand/tasks.md`. Lies sie mit `k_playbook_doc_read`, bevor du
einen Task liest oder änderst; dieser Text wiederholt sie nicht.

include: _task/revisionskette.md

Der Moderator hält die Revisionskette; ein `foreign_change` hält den Command für diesen Task
an. Ist das Ziel ein Task oder leer, braucht der Command die Collection: Steht
`kephalaion.ok` nicht auf `true`, endet er mit `kephalaion.message`
(Abschnitt „Ohne Kephalaion“ dort). Eine Instruction-Datei auf der Platte lässt sich auch
ohne Collection prüfen.

## Schritt 2 — Ziel auflösen

**Wort für die erneute Prüfung.** Ist das letzte Wort des Arguments `nochmal`, `erneut` oder
`again` (Groß- und Kleinschreibung gleich), nimm es ab und merke `ERNEUT = ja`; der Rest ist
das Ziel. Sonst `ERNEUT = nein`.

**Geprüft** heißt allein: Eine Zeile, die mit `## Review-Log` beginnt, steht im Dokument.
Gezählt wird bei einem Task mit `k_playbook_doc_grep` (`ref`, Muster `^## Review-Log`,
`count_only`) — dasselbe Muster wie `/k-task-run` —, bei einer Datei auf der Platte
mit dem Suchwerkzeug. Ob der Task danach geändert oder ausgeführt wurde, spielt keine Rolle.
Die `revision` aus der Zählung eines Tasks merkst du dir als `REVISION` dieses Tasks.

Das Ziel entscheidet:

include: _task/ziel-aufloesen.md

- **Leer, zusätzlich:** Du zählst je Task wie oben und nimmst als Ziele nur die ungeprüften
  (Anzahl 0). Mit `ERNEUT = ja` nimmst du alle. Bleibt keiner übrig, gib aus:

  ```
  Alle <n> offenen Tasks sind schon geprüft.
  Erneut prüfen: /k-task-refine <nummer> nochmal
  ```

  und Schluss; geschrieben wird nichts. Die schon geprüften offenen Tasks, die nicht Ziel
  sind, merkst du dir als `UEBRIGE_TASKS`. Der Lauf ohne Ziel ist der **Lauf mit Gruppen**.
- **Sonst ein Pfad auf der Platte:** eine Datei oder ein Verzeichnis mit `.md`-Dateien, relativ
  zu `project.dir`. Gibt es ihn nicht, brich mit einer klaren Meldung ab. Sag dazu,
  dass dies ein ausdrückliches Einzelziel ist und nicht die Task-Warteschlange. Bei einem
  Verzeichnis: alle `.md` direkt darin, ohne `done/`, `old/` und Archive, sortiert nach der
  führenden Nummer (auch mit `_`), Dateien ohne Nummer übersprungen. Keine → „Keine
  Dateien gefunden“ und Schluss. Der Grund: Ein Verzeichnis als Ziel ist eine Folge
  nummerierter Anweisungen, die in dieser Reihenfolge gelten — so lagen Tasks einst auf der
  Platte, und so liegen Instruction-Dateien heute. `done/`, `old/` und Archive sind
  Erledigtes, eine Datei ohne Nummer (etwa ein `README.md`) gehört nicht zur Folge; wer sie
  prüfen will, nennt sie als Einzelziel. Ein Verzeichnis wird gemeinsam im Hauptkontext
  geprüft, nicht in Gruppen: Es ist eine Folge.

Ein ausdrücklich genanntes Ziel — Verweis, Nummer, Datei, Verzeichnis — zählst du wie oben,
dann gelten diese Rückfragen, jede mit der Fertig-Meldung vorher:

- **Schon geprüft** — das Ziel trägt ein Review-Log und `ERNEUT = nein`: Fertig-Meldung
  (`Rückfrage: schon geprüft`), dann fragen: erneut prüfen oder abbrechen. Die Frage nennt die
  Daten der vorhandenen Review-Logs, gelesen aus den Zeilen `^## Review-Log` (bei einem Task
  `k_playbook_doc_grep` ohne `count_only`). Bei einem Verzeichnis ist es eine Frage, die die
  geprüften Dateien mit ihren Daten nennt. Mit `ERNEUT = ja` entfällt die Frage.
- **Erledigt** — ein Treffer oder Verweis unter `done/`: Der Task ist schon abgeschlossen.
  Fertig-Meldung (`Rückfrage: erledigter Task`), dann fragen: trotzdem prüfen — das
  Review-Log kommt an das Dokument unter `done/`, der Task bleibt dort — oder abbrechen.
  `/k-task-refine` öffnet keinen Task wieder; das tut `/k-task-run`. Ist der erledigte Task
  auch schon geprüft, bleibt es bei dieser einen Rückfrage; sie nennt dann die Daten der
  Review-Logs mit.

Merke die Ziele als `REVIEW_TARGETS`: je Eintrag der `keph:`-Verweis oder der absolute Pfad,
dazu die Anzeigeform (der Verweis bzw. der projektrelative Pfad), ob es ein Task in der
Collection oder eine Datei ist, und bei einem Task `REVISION`.

## Fertig-Meldung

Vor jeder Rückfrage an den Nutzer und einmal am Ende meldet sich der Hauptkontext auf dem
Rechner des Nutzers; jede Stelle nennt sie selbst („Fertig-Meldung (`Rückfrage: …`)“). Ein
Sub-Agent einer Gruppe sendet keine Fertig-Meldung — weder vor einer Rückfrage noch am Ende —;
er gibt die Frage zurück.

include: _task/fertig-meldung.md

## Schritt 3 — Gruppen bilden (nur im Lauf mit Gruppen)

Lies `k-playbook:/commands/_task-refine/gruppen.md` mit `k_playbook_doc_read` und bilde die
Gruppen nach „Gruppen bilden“, Aufrufer `/k-task-refine`. Mit einem Ziel entfällt der Schritt;
der Hauptkontext ist Moderator für alle `REVIEW_TARGETS`.

## Schritt 4 — Startübersicht ausgeben

Vor allem anderen an den Nutzer:

```
Review gestartet
─────────────────────────────
Tasks:   <Liste der Ziele in Anzeigeform>
Gruppen: <je Gruppe eine Zeile, siehe unten>
Ablauf:  <je Gruppe ein Sub-Agent, parallel | Hauptkontext, Gruppen nacheinander | Hauptkontext>
Projekt: <project.dir>
Runden:  max. 5
```

Eine Gruppenzeile nennt die Nummern, bei mehr als einem Task mit dem Grund, etwa
`gemeinsam: <nr> + <nr> — beide ändern project:/defaults/commands/k-task-refine.md`, sonst
`allein: <nr>`. Ohne Gruppen (Lauf mit Ziel) entfallen die Zeilen `Gruppen:`, und `Ablauf:`
heißt `Hauptkontext`. Den Intent liest erst der Dialog; das Review-Log nennt ihn je Ziel.

## Schritt 5 — Prüfen

Die Weiche:

- **Lauf mit Gruppen** (ohne Ziel): Folge `gruppen.md` (schon gelesen), Aufrufer
  `/k-task-refine`: je Gruppe einen Sub-Agenten prüfen lassen (im Rückfall prüft der
  Hauptkontext selbst), die Rückgaben auswerten, dann den Überschneidungs-Critic. Moderator ist
  der Sub-Agent der Gruppe für genau ihre Tasks.
- **Lauf mit Ziel:** Lies `k-playbook:/commands/_task-refine/dialog.md` mit `k_playbook_doc_read`
  und führe den Dialog selbst für alle `REVIEW_TARGETS`; der Hauptkontext ist Moderator.

## Schritt 6 — Zusammenfassung

Erst die Fertig-Meldung (`fertig`; der Body nennt die Zahl der Tasks, gefixte und offene
Issues und, im Lauf mit Gruppen, die Zahl der Überschneidungen). Dann an den Nutzer, einmal
über alle Ziele:

```
Review abgeschlossen
─────────────────────────────
<je Ziel eine Zeile: Anzeigeform — Ergebnis — Runden <n>, gefixt <n>, offen <n>>
<im Lauf mit Gruppen: die Zeilen des Überschneidungs-Critics unverändert, oder "Überschneidungen: keine"
 bzw. "Überschneidungen: nicht geprüft (eine Gruppe, keine anderen offenen Tasks)">
```

Das Ergebnis je Ziel ist `geprüft`, `abgebrochen` mit Grund oder `ohne Review-Log` (der Nutzer
hat abgebrochen oder nach einem fremden Schreiber nicht weitergemacht).

Folge-Command: **`/k-task-run`** — führt die gehärteten Tasks aus und legt in jedem das
Ergebnis der Ausführung ab; mit demselben Verweis oder derselben Nummer genau diesen. Nennt die
Zusammenfassung eine Überschneidung mit Reihenfolge, führe die Tasks in dieser Reihenfolge aus.
