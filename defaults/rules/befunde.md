# Regel: Befunde aus Analyse und Fehlersuche festhalten

## Zweck

Herauszufinden, wie fremder Code funktioniert, welche Absicht dahinterstand und was womit
zusammenhängt, kostet lange Suchen und echte Testläufe. Das Ergebnis lebt nur im
Sitzungskontext und ist beim nächsten Mal weg — dieselbe Sackgasse wird ein zweites Mal
gelaufen.

Diese Regel legt fest, **wann** ein Befund festgehalten wird, **wohin** er gehört und
**wie** er aussieht.

## Grundsatz

Eine Analyse ist erst abgeschlossen, wenn ihr Ergebnis geschrieben ist. Das gilt für
Belegtes, für Widerlegtes und für ausgeschlossene Sackgassen.

Geschrieben wird **während** der Arbeit, nicht erst am Ende. Eine lange Sitzung wird
zwischendurch komprimiert; wer bis zum Schluss wartet, hat die Details der frühen
Fehlersuche dann nicht mehr.

## Die drei Stufen

Der Unterschied liegt allein im Umfang des Eintrags, nicht in der Ablage.

| Stufe | Anlass | Umfang |
|---|---|---|
| 1 | Eine Erkenntnis ist belegt. | Knapp: Befund, Beleg, Sicherheit. |
| 2 | Ein Problem wurde erkannt **und gelöst**. | Zusätzlich: Frage, Hintergrund, ausgeschlossene Wege. |
| 3 | Die Sitzung ist zu Ende. | Verdichten, befördern, prüfen — über `/k-danke`. |

Stufe 1 und 2 laufen ohne Rückfrage. Gemeldet wird in einer Zeile, welche Datei ergänzt
wurde — nicht ihr Inhalt.

Stufe 3 fragt sehr wohl, aber erst, wenn das Material bereits geschrieben dasteht.

## Ablageort

`k-playbook-local/material/befunde/<slug>.md` — eine Datei je **Thema**, fortgeschrieben.

Das Verzeichnis ist das Erzeuger-Verzeichnis dieser Regel und wird beim ersten Lauf ohne
Rückfrage angelegt. Es ist die einzige Stelle unterhalb von `material/`, in die
geschrieben wird; der Rest des Verzeichnisses bleibt Rohmaterial, das niemand anfasst.

Nicht hierher gehören zwei andere Sorten Wissen, die in einer Analyse mit anfallen:

- **Betriebsfallen** — was man vor dem Handeln wissen muss, damit man nicht hineintritt
  („dieses Verzeichnis ist gesperrt", „dieser Test sagt nichts aus"). Sie gehören nach
  `k-playbook-local/guidelines/fallen.md`.
- **Prüfbare Regeln** — was jedes Mal zu geschehen hat. Sie gehören als projekteigene
  Regel nach `k-playbook-local/rules/`.

Beide werden nur nach ausdrücklicher Bestätigung geschrieben: sie wirken dauerhaft.

## Dateiformat

Das Frontmatter ist bewusst ein anderes als das einer Doc-Datei — deutsche Schlüssel, kein
`type:`, kein `generated:`. Eine Material-Datei darf auf keinen Blick wie ein fertiges
Dokument aussehen, sonst wandert sie am Extrakt-Schritt vorbei in den Index.

```markdown
---
thema: <Titel des Themas>
begonnen: <ISO-Datum>
zuletzt: <ISO-Datum>
status: offen | geklaert
---

# <Titel>

<Ein Satz: welcher Frage dieses Thema nachgeht.>

## <ISO-Datum> — <Befund in einem Satz>

**Befund:** <die Antwort, in einem Satz>
**Beleg:** <pfad:zeile, Testlauf mit Kommando und Ausgabe, oder Log-Stelle>
**Sicherheit:** bestaetigt | unbestaetigt | widerlegt
**Frage:** <was wollten wir wissen>                    (ab Stufe 2)
**Warum es so ist:** <Absicht oder Historie>           (ab Stufe 2, nur wenn erkennbar)
**Sackgassen:** <was geprüft und ausgeschlossen wurde> (ab Stufe 2)

<!-- sitzung: <Sitzungskennung> -->
```

## Regeln zum Inhalt

- **`Sackgassen:` ist das Wertvollste am ganzen Eintrag.** Was ausgeschlossen wurde, steht
  nirgends im Code und ist genau das, was die nächste Sitzung sonst noch einmal
  durchläuft. Mit Begründung, nicht als bloße Aufzählung.
- **`Sicherheit:`** hat genau drei Werte:
  - `bestaetigt` — am Code oder an einem Testlauf belegt. Der Beleg steht als `pfad:zeile`
    oder als Kommando mit Ausgabe unter `Beleg:`. **`bestaetigt` ohne Beleg ist
    unzulässig.**
  - `unbestaetigt` — plausibel, aber nicht geprüft. Das ist der Standard.
  - `widerlegt` — die These wurde geprüft und ist falsch. Ein widerlegter Befund ist ein
    vollwertiger Befund und wird nicht gelöscht.
- **Die Sitzungskennung** steht als HTML-Kommentar am Ende jedes Abschnitts. Daran erkennt
  ein späterer Aufräumer, welche Sitzungen Einträge geschrieben, aber nie abgeschlossen
  haben — ohne einen Chatverlauf lesen zu müssen.
- Keine erfundenen Zusammenhänge. Was nicht belegt ist, bekommt `unbestaetigt` und keine
  Ausschmückung.

## Anhängen, nie umschreiben

Ein neuer Befund ist ein neuer `##`-Abschnitt am Ende der Datei. Frühere Abschnitte werden
nicht korrigiert.

Stellt sich ein früherer Befund als falsch heraus, wird er **nicht** überschrieben:
Ein neuer Abschnitt benennt ihn, setzt `Sicherheit: widerlegt` und sagt, was stattdessen
gilt. Nur so bleibt sichtbar, dass ein Irrtum vorlag — und woran er lag.

Im Frontmatter werden ausschließlich `zuletzt:` und `status:` fortgeschrieben.

## Schreibkonflikt

Vor jedem Anhängen wird der Inhalt der Datei gegen den Stand geprüft, den diese Sitzung
zuletzt gelesen hat. Weicht er ab, hat inzwischen eine andere Sitzung geschrieben.

Dann geht der Eintrag nach `<slug>--<ISO-Datum>.md`, bei Bedarf mit `--<n>` dahinter, und
die Meldung sagt ausdrücklich, dass eine Zweitdatei entstanden ist. Die Thema-Datei wird
in diesem Fall **nicht** angefasst.

Mehrere Sitzungen im selben Arbeitsbaum sind der Normalfall, nicht die Ausnahme.

## Was nicht festgehalten wird

- Zwischenschritte auf dem Weg zu einer Erkenntnis. Erst das Ergebnis zählt.
- Alles, was der Code selbst schon sagt. Eine Doku, die den Code paraphrasiert, veraltet
  mit ihm und hilft niemandem.
- Was in einer bestehenden Datei unter `k-playbook-local/docs/` bereits richtig steht.
- Ablaufplauderei, Terminabsprachen, Werkzeugbedienung.

## Prüfkriterium

Eine Analyse gilt nur dann als abgeschlossen, wenn eines davon zutrifft:

- Der Befund steht unter `material/befunde/`, und die Antwort nennt die geschriebene Datei.
- Es wurde geprüft und gesagt, warum nichts festzuhalten war.
- Der Nutzer hat entschieden, dass nichts geschrieben werden soll.

## Anti-Muster (nicht tun)

- **Bis zum Sitzungsende warten.** Bis dahin ist der Kontext womöglich komprimiert und die
  Einzelheiten sind weg. Die Stufen 1 und 2 sind genau dagegen da.
- **Behauptung als Tatsache.** `Sicherheit: bestaetigt` ohne `pfad:zeile` oder Testausgabe
  ist eine Lüge im Eintrag.
- **Einen Irrtum stillschweigend korrigieren.** Ein überschriebener Befund sieht aus, als
  wäre er nie falsch gewesen — und der nächste läuft wieder hinein.
- **Die Sackgassen weglassen**, weil sie ja zu nichts geführt haben. Sie sind der Teil,
  den man sonst ein zweites Mal bezahlt.
- **In `docs/` schreiben.** Diese Regel schreibt ausschließlich nach `material/befunde/`.
  Der Weg in die Wissensablage (`knowledge/extracted/`) führt über `/k-docs-extract`, und
  der hat sein eigenes Gate.
- **Das übrige `material/` anfassen.** Verschieben, umbenennen oder aufräumen nimmt die
  Quelle weg, gegen die man später prüfen würde.
