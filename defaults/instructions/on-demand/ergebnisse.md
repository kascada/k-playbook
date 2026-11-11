---
title: Ergebnisse ablegen
description: Wo ein Ergebnis liegt, das der Nutzer mittelfristig sichern und zeigen will — keph:/results/, projektweit, nur .md, Name mit Datum vorn; geschrieben nur auf Wunsch des Nutzers oder wenn ein Command es vorsieht, nie als Pflicht; aufgeräumt wird vom Nutzer.
---

## Ergebnisse ablegen

`keph:/results/` ist der Platz für Ergebnisse, die jemand mittelfristig sichern und anderen
zeigen will: das Ergebnis eines Reviews, eines Testlaufs, einer Messung oder eines Auftrags.
Der Platz ist ein Angebot, keine Pflicht. Er füllt sich nur mit dem, was jemand dort haben
will.

### Wann

- **Nur auf Wunsch.** Du schreibst hier, wenn der Nutzer darum bittet oder ein Command es
  ausdrücklich vorsieht. Sonst nicht.
- **Nicht von dir aus.** Ein fertiger Task, ein abgeschlossenes Review oder ein bestandener
  Test ist kein Anlass. Biete das Ablegen auch nicht am Ende einer Arbeit an: Wer einen Platz
  braucht, fragt danach.

### Wohin

- **Projektweit, nicht je User.** Ein Ergebnis gehört dem Projekt. Wer es abgelegt hat, hält
  Kephalaion selbst fest (`created.by`); dafür gibt es weder ein Feld noch ein
  User-Verzeichnis.
- **Name mit Datum vorn**, nur ASCII: `results/<YYYY-MM-DD>-<kurzname>.md`, das Datum aus
  `context.now.date`. Gehören mehrere Dateien zusammen, liegen sie in einem Verzeichnis
  `results/<YYYY-MM-DD>-<kurzname>/`. Eine Ebene davor für die Art ist erlaubt, etwa
  `results/review/<YYYY-MM-DD>-<kurzname>.md`, aber keine Pflicht.
- **Nur `.md`.** Alles andere — JSON, Rohausgaben von Scannern, Logs, Bilder — kommt nicht
  nach Kephalaion. Auch eine `.md` gibt kein Geheimnis im Klartext wieder, etwa aus der
  Ausgabe eines Secret-Scanners.
- **Anlegen mit Kephalaion `create`**, `collection` gleich `kephalaion.address`, `name` der
  Pfad ohne führenden `/`. Antwortet es mit `name_taken`, wählst du einen anderen Kurznamen;
  ein vorhandenes Ergebnis wird nicht überschrieben.
- **Ist Kephalaion nicht erreichbar** (`kephalaion.ok` nicht `true`), weichst du nicht auf die
  Platte aus. Sag es dem Nutzer.

### Kopf

```markdown
---
title: "<was das Ergebnis zeigt, in einem Satz>"
---
```

`title` ist Pflicht und steht in doppelten Anführungszeichen, ein `"` im Wert wird `\"`.
Alles Weitere in Kopf und Rumpf ist frei; eine `description` in einem Satz hilft, wenn das
Verzeichnis gelistet wird.

### Danach

- **Verweisen ist erlaubt.** Anders als auf einen Befund darfst du auf ein Ergebnis
  verweisen, etwa aus einem Task, einem Befund oder einem Workbench-Thema.
- **Geändert, verschoben oder gelöscht wird es nur auf Wunsch des Nutzers.** Aufgeräumt wird
  vom Nutzer; eine Sitzung löscht unter `results/` nichts von sich aus.
- **Nicht indiziert.** `results/` liegt außerhalb von `docs/`: Die Volltextsuche findet es
  nicht, Kephalaion `list` schon.

### Abgrenzung

| Was | Inhalt | Wie lange |
|---|---|---|
| Ergebnis, `keph:/results/` | was ein Lauf, Test oder Auftrag zu einem Zeitpunkt ergeben hat, zum Sichern und Zeigen | bis der Nutzer aufräumt |
| Befund, `keph:/queue/findings/` | eine belegte Erkenntnis | bis zur Übernahme in die Wissensablage |
| Workbench-Thema, `keph:/user/~/workbench/` | wo ein laufendes Vorhaben steht | bis das Vorhaben fertig ist |
| Task, `keph:/user/~/tasks/` | ein Auftrag samt dem, was sein Lauf anhängt | bis er erledigt ist, dann `done/` |
| Projektdoku, `keph:/docs/` | was dauerhaft gilt | solange es stimmt |

Was `/k-task-run` an einen Task anhängt, bleibt dort.
