---
title: Befunde festhalten
description: Wie ein Befund aus Analyse und Fehlersuche geschrieben wird — ein Dokument je Befund in queue/findings/, Kopf mit thema, sicherheit, sitzung, entstanden und ziel, Rumpf mit Befund, Beleg und Sackgassen; dazu Widerlegen, was nicht hineingehört und was /k-danke daran ändert.
---

## Befunde festhalten

Herauszufinden, wie fremder Code funktioniert, welche Absicht dahinterstand und was womit
zusammenhängt, kostet lange Suchen und echte Testläufe. Ein Befund hält das Ergebnis fest,
damit es in die Wissensablage übernommen werden kann, statt beim nächsten Mal neu erarbeitet
zu werden. Das gilt für Belegtes, für Widerlegtes und für ausgeschlossene Sackgassen.

Geschrieben wird **während** der Arbeit, nicht erst am Ende. Eine lange Sitzung wird
zwischendurch komprimiert; wer bis zum Schluss wartet, hat die Einzelheiten der frühen
Fehlersuche dann nicht mehr.

### Wann

| Stufe | Anlass | Umfang |
|---|---|---|
| 1 | Eine Erkenntnis ist belegt. | Knapp: Befund, Beleg, Sicherheit. |
| 2 | Ein Problem wurde erkannt **und gelöst**. | Zusätzlich: Frage, Hintergrund, ausgeschlossene Wege. |
| 3 | Die Sitzung ist zu Ende. | `/k-danke`: vorlegen, nachtragen, `ziel` erfragen. |

Stufe 1 und 2 laufen ohne Rückfrage. Gemeldet wird in einer Zeile, welcher Befund angelegt
wurde, nicht sein Inhalt.

### Wohin

- **Ein Dokument je Befund** in `keph:/queue/findings/`. Anlegen mit `k_playbook_doc_create`,
  Sorte `findings`, Kurzname: der Befund in wenigen Worten. Die Antwort nennt den Verweis.
  Den Inhalt schreibst du gleich danach mit Kephalaion `write`, `collection` gleich
  `kephalaion.address`.
- **Ein Befund wird nach dem Schreiben nicht mehr geändert.** Einzige Ausnahme ist `ziel`, das
  `/k-danke` nach Bestätigung durch den Nutzer setzt. Wer etwas ergänzen will, schreibt einen
  neuen Befund.
- **Auf einen Befund wird nicht verwiesen.** Er wartet darauf, in die Wissensablage übernommen
  zu werden, und verschwindet danach. Was ein laufendes Vorhaben als Beleg braucht, steht in
  seinem Workbench-Thema; was der Code braucht, steht im Kommentar am Code.
- **Ist Kephalaion nicht erreichbar** (`kephalaion_unavailable`, `kephalaion.ok` nicht `true`),
  weichst du nicht auf die Platte aus. Sag es und gib den Befund vollständig in deiner Antwort
  wieder, damit er nicht verloren geht.

Nicht hierher gehören:

- **Arbeitsstände** — wo ein laufendes Vorhaben steht, was offen ist, was als Nächstes kommt.
  Sie gehören in die Workbench (`keph:/user/~/workbench/`).
- **Betriebsfallen und prüfbare Regeln als fertiger Text.** Was eine Falle oder eine Regel
  werden soll, ist zuerst ein Befund. Was daraus wird, entscheidet der Nutzer in `/k-danke`
  über `ziel`.

### Kopf

Der Kopf trägt bewusst deutsche Schlüssel und weder `type:` noch `generated:`. Ein Befund darf
auf keinen Blick wie ein fertiges Dokument der Wissensablage aussehen.

```markdown
---
title: <der Befund in einem Satz>
thema: <wenige Worte, unter denen verwandte Befunde zusammengehören>
sicherheit: bestaetigt | unbestaetigt | widerlegt
sitzung: <session aus dem Briefing>
entstanden: arbeit | k-danke | migration
ziel:
---
```

- `title` zeigen `k_playbook_doc_list` und das Briefing an.
- `thema` gruppiert. Verwende dasselbe Thema wie ein vorhandener Befund, wenn es dieselbe
  Frage ist; die Übernahme bündelt danach.
- `sitzung` ist die Kennung deiner Sitzung, das Feld `session` des Briefings (im Klartext die
  Zeile `Sitzung:`). Daran findet `/k-danke` die Befunde dieser Sitzung. Hast du das Briefing
  neu geladen, nimm die neue Kennung.
- `entstanden` ist `arbeit` für alles, was während der Arbeit geschrieben wird, `k-danke` für
  das, was `/k-danke` nachträgt, und `migration` für das, was `/k-upgrade` aus den früheren
  Thema-Dateien unter `material/befunde/` übernimmt.
- `ziel` bleibt beim Schreiben leer. Nur `/k-danke` setzt es, und nur nach Bestätigung durch
  den Nutzer: `wissen` (Projektwissen für die Wissensablage), `falle` (was man vor dem Handeln
  wissen muss), `regel` (was jedes Mal zu geschehen hat) oder `verwerfen` (nicht übernehmen).

### Rumpf

```markdown
**Befund:** <die Antwort, in einem Satz>
**Beleg:** <pfad:zeile, Testlauf mit Kommando und Ausgabe, oder Log-Stelle>
**Frage:** <was wollten wir wissen>                    (ab Stufe 2)
**Warum es so ist:** <Absicht oder Historie>           (ab Stufe 2, nur wenn erkennbar)
**Sackgassen:** <was geprüft und ausgeschlossen wurde> (ab Stufe 2)
```

- **`Sackgassen:` ist das Wertvollste am ganzen Befund.** Was ausgeschlossen wurde, steht
  nirgends im Code und ist genau das, was die nächste Sitzung sonst noch einmal durchläuft.
  Mit Begründung, nicht als bloße Aufzählung.
- **`sicherheit`** hat genau drei Werte:
  - `bestaetigt`: am Code oder an einem Testlauf belegt. Der Beleg steht als `pfad:zeile`
    oder als Kommando mit Ausgabe unter `Beleg:`. **`bestaetigt` ohne Beleg ist unzulässig.**
  - `unbestaetigt`: plausibel, aber nicht geprüft. Das ist der Standard.
  - `widerlegt`: die These wurde geprüft und ist falsch. Ein widerlegter Befund ist ein
    vollwertiger Befund.
- Keine erfundenen Zusammenhänge. Was nicht belegt ist, bekommt `unbestaetigt` und keine
  Ausschmückung.

### Widerlegen

Stellt sich ein früherer Befund als falsch heraus, wird er nicht geändert. Ein neuer Befund
mit `sicherheit: widerlegt` und demselben `thema` nennt die These in Worten, nicht per Verweis,
und sagt, was stattdessen gilt und woran der Irrtum lag. Die Übernahme sieht beide.

### Was nicht festgehalten wird

- Zwischenschritte auf dem Weg zu einer Erkenntnis. Erst das Ergebnis zählt.
- Alles, was der Code selbst schon sagt.
- Was in der Projektdokumentation bereits richtig steht.
- Ablaufplauderei, Terminabsprachen, Werkzeugbedienung.

### Abgeschlossen

Eine Analyse gilt nur dann als abgeschlossen, wenn eines davon zutrifft:

- Der Befund steht in `keph:/queue/findings/`, und die Antwort nennt seinen Verweis.
- Kephalaion war nicht erreichbar, und die Antwort gibt den Befund vollständig wieder.
- Es wurde geprüft und gesagt, warum nichts festzuhalten war.
- Der Nutzer hat entschieden, dass nichts geschrieben werden soll.
