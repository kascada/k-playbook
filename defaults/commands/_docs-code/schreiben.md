---
title: Ein Code-Dokument schreiben
description: "Modul für /k-docs-code und seine Sub-Agenten, vom Hauptkontext und vom Sub-Agenten selbst als Auftrag gelesen — ein Dokument unter keph:/docs/generated/code/ im Arbeitsverzeichnis schreiben: nur was der Code jetzt zeigt, Prüfung vor dem Einfügen, Kopf und Vorlage, Stellen als pfad:zeile, Englisch, Dateinamen, und die Regeln eines schreibenden Laufs."
---

Du schreibst ein Dokument der Code-Doku des Projekts in das Arbeitsverzeichnis `WORK_DIR`,
mit den Datei-Werkzeugen — nicht nach Kephalaion, kein `publish`. Der Auftrag nennt dir
`SCAN_ROOT` (`project.repoRoot`), die Ausschlüsse und den beschriebenen Stand, das
Dokument (`dokument`) und seine `aktion`, die `pfade` und Anlässe, `WORK_DIR`, `GENERATED_AT`
und die bisherige Fassung samt `hints`. Die Regeln des Laufs:

include: k-playbook:/commands/_docs/schreiblauf.md

## Nur was der Code jetzt zeigt

Aufträge sammeln sich über Tage. Was ihr `anlass` oder ihr Rumpf beschreibt, kann eine
spätere Änderung überholt haben: Eingeführtes wurde wieder entfernt, ein Name hat sich
geändert, ein Pfad ist gelöscht. Dasselbe gilt für die bisherige Fassung des Dokuments. Beide
sagen nur, wo du hinschaust, nie, was in der Doku steht.

- Quelle jeder Aussage ist der beschriebene Stand von `SCAN_ROOT`. Was dort nicht mehr
  vorkommt, kommt nicht in die Doku, auch wenn ein Auftrag oder die bisherige Fassung es
  nennt.
- Entferntes wird aus dem Dokument gestrichen, nicht als Geschichte beschrieben („wurde mit
  Task … entfernt“, „früher …“). Die Doku beschreibt den Stand, nicht den Weg dorthin.
- **Vor dem Einfügen prüfen.** Bevor eine Aussage ins Dokument kommt, prüfst du sie erneut
  am Code: Jede Stelle `pfad:zeile` existiert und zeigt auf das Beschriebene; jeder
  genannte Bezeichner, jedes Feld, jeder Befehl, jede Option und jede Meldung kommt im Code
  so vor; kein Satz stützt sich allein auf einen Auftrag oder die bisherige Fassung. Was sich
  nicht bestätigen lässt, kommt nicht hinein; ist es wichtig, wird es eine Unklarheit.
- **Fremde Änderungen während des Laufs.** Taucht im Arbeitsbaum eine nicht committete
  Änderung auf, die nicht vom Lauf selbst stammt (`git status --porcelain` gegen den
  Stand, den der Auftrag nennt), beschreibt der Lauf sie nicht ungefragt: Sie kann halbfertig sein und
  gehört zu einer anderen Sitzung, die ihren eigenen Auftrag einreiht. Der Hauptkontext nennt
  sie vor `publish` und fragt, ob sie herausgenommen wird; ein Sub-Agent meldet sie. Ist sie
  inzwischen committet oder veröffentlicht, gehört sie zum Stand.
- Was ein Sub-Agent schreibt, prüft der Hauptkontext vor `publish` noch einmal nach denselben
  Punkten — mindestens alle Stellen `pfad:zeile` und alle Bezeichner — und berichtigt, was
  nicht stimmt.

## Datei und Vorlage

Dateinamen `<NN>-<slug>.md`, klein, nur `a–z`, `0–9`, `.`, `_`, `-`. Ein umgeschriebenes
Dokument behält seinen Dateinamen. Keine `README.md`, keine anderen Dateien als `.md`. Der
Kopf trägt genau diese Felder: `title` und `description` sind Pflicht, `generated` nennt
`by: k-docs-code` und `GENERATED_AT`, `hints` werden unverändert übernommen (nie
geändert; neue nur als Vorschlag), `questions` sind die Fragen, die ein Neuer in der ersten
Woche stellt und die genau dieses Dokument beantwortet, `sources` stehen nur, wenn Quellen
außer dem Code genutzt wurden (je Eintrag `resource`, dazu `item` und `version`);
`verified` setzt nur ein Mensch. Weitere Kopffelder gibt es nicht.

```markdown
---
title: "<Titel>"
description: "<Ein Satz: was dieses Dokument erklärt.>"
generated: { by: k-docs-code, at: <GENERATED_AT> }
hints: [ <unverändert übernommen; das Feld fehlt, wenn das Dokument keine hatte> ]
questions: [ "<Frage, die ein Neuer in der ersten Woche stellt>", … ]
---

# <Titel>

<1–2 Sätze: worum es geht, aus welcher Sicht.>

## Purpose

<Der fachliche oder technische Zweck.>

## How it works

<Die Kernidee: Datenfluss, Abhängigkeiten, Aufrufweg. Stellen im Code als `pfad:zeile`.>

## Key places in the code

- `pfad/zur/datei.go:45` — <was dort steht, in einem Satz>

## Edge cases

<Nicht Offensichtliches, versteckte Verträge, bekannte Fehler.>

## Related topics

- `<NN>-<slug>.md` — <in welchem Zusammenhang>
```

Regeln:

- Neue `hints` schlägst du nur vor; eingetragen werden sie erst nach Bestätigung durch den
  Nutzer.
- Stellen im Code immer als `pfad:zeile`, mit dem Pfad ab `SCAN_ROOT` und an jeder Stelle
  ganz — nie nur der Dateiname, auch nicht, wenn der volle Pfad im selben Abschnitt schon
  stand: Namen wie `server.go` gibt es in mehreren Paketen, und ein Leser springt über die
  Suche mitten in einen Abschnitt. Verwandte Themen im selben Verzeichnis mit ihrem
  Dateinamen, andere Teile der Projektdoku als `keph:`-Verweis — ein isoliertes Dokument ist
  ein Fehler. Auf einen Auftrag verweist kein Dokument.
- Keine erfundenen Erklärungen: Unklarheiten legst du vor, statt zu raten.
- **Englisch schreiben.** Kopf (`title`, `description`, `questions`) und Text sind englisch,
  auch wenn Code, Kommentare oder der bisherige Text deutsch sind; die Überschriften der
  Vorlage stehen wörtlich so. Bezeichner, Pfade, Command-Namen und Meldungen aus dem Code
  bleiben unverändert in Code-Schrift. Bestehende `hints` übersetzt du nicht, neue schlägst
  du englisch vor.
- Keine Fließtextwände.

Melde nach dem Dokument: Dateiname und Zeilenzahl, nicht der Inhalt; dazu Unklarheiten, überholte Stellen der bisherigen Fassung und vorgeschlagene neue `hints`.
