---
title: Steckbriefe recherchieren und schreiben
description: "Modul für /k-docs-tools und seine Sub-Agenten, vom Hauptkontext und vom Sub-Agenten selbst als Auftrag gelesen — je ausgewähltem Kandidaten recherchieren (Basis aus Repo und Inventar, Klasse A zusätzlich gezielt im Web, höchstens drei Abrufe) und den Steckbrief im Arbeitsverzeichnis schreiben: Dateiname, Kopf, Vorlage, sources mit dem Inventar, Pitfalls konkret, und die Regeln eines schreibenden Laufs."
---

Du recherchierst und schreibst Steckbriefe zu Libraries und Tools des Projekts, einen je
ausgewähltem Kandidaten, in das Arbeitsverzeichnis `WORK_DIR`, mit den Datei-Werkzeugen —
nicht nach Kephalaion, kein `publish`. Der Auftrag nennt dir den Kandidaten mit seinen Zeilen
aus dem Inventar (Gegenstand, Art, Versionen, Pin, Scope, Umgebungen), die Klasse (A oder
B), den Scope (`SCAN_ROOT`, Ausschlüsse), `WORK_DIR`, `GENERATED_AT` und die bisherige
Fassung seines Dokuments samt `hints`. Die Regeln des Laufs:

include: k-playbook:/commands/_docs/schreiblauf.md

## Recherche

Recherchiere den Kandidaten deines Auftrags:

**Basis (immer):**

- Die Rolle im Projekt aus der Nutzung im Code und eigenem Wissen ableiten.
- Zwei bis drei Kernstellen im Code nennen, als `pfad:zeile`, innerhalb des Scope.
- Versionen und Pin aus dem Auftrag übernehmen.

**Klasse A — zusätzlich gezielt im Web:**

- `WebFetch` auf die Startseite der offiziellen Doku zur genutzten Version.
- `WebFetch` auf Changelog oder Release-Notes rund um die genutzte Version (major.minor).
- Höchstens ein gezielter Abruf nach bekannten Pitfalls an der offiziellen Quelle (etwa die
  Issues des Repos). Keine breite Suche: insgesamt **höchstens drei** Abrufe je Gegenstand.

**Klasse B — nur die Basis.** Ins Web nur, wenn beim Schreiben eine konkrete Frage aufkommt,
die nicht sicher zu beantworten ist.

Was sich aus Repo und Quellen nicht klären lässt, ist eine Unklarheit: sammeln und vor
`publish` vorlegen, nicht raten.


## Schreiben

Je ausgewähltem Kandidaten eine Datei `<WORK_DIR>/<name>.md`, mit den Datei-Werkzeugen.
`<name>` ist der Gegenstand, klein geschrieben; jedes Zeichen außer `a–z`, `0–9`, `.`, `_`,
`-` wird `-`. Aus `go/github.com/yuin/goldmark` wird `go-github.com-yuin-goldmark`, aus
`python/celery` wird `python-celery`. Ein vorhandenes Dokument, das umgeschrieben wird, behält
seinen Dateinamen. Keine `README.md`, keine anderen Dateien als `.md`.

Der Kopf trägt genau diese Felder: `title` und `description` sind Pflicht, `generated` nennt
`by: k-docs-tools` und `GENERATED_AT`, `hints` werden unverändert übernommen (nie geändert;
neue nur als Vorschlag), `questions` beantwortet genau dieser Steckbrief, `sources` führt die
Quellen, `verified` setzt nur ein Mensch; weitere Kopffelder gibt es nicht. Die Vorlage:

```markdown
---
title: "<Name, wie das Projekt ihn nennt>"
description: "Projektrelevante Pitfalls und Idiome für <Name>."
generated: { by: k-docs-tools, at: <GENERATED_AT> }
hints: [ <unverändert übernommen; das Feld fehlt, wenn das Dokument keine hatte> ]
questions: [ "<Frage, die genau dieser Steckbrief beantwortet>", … ]
sources:
  - resource: keph:/docs/generated/versions/inventory.md
    item: <gegenstand>
    version: <version>
  - resource: <URL der offiziellen Doku, wenn gelesen>
  - resource: <URL von Changelog oder Release-Notes, wenn gelesen>
---

# <Name>

## Rolle im Projekt

<1–2 Absätze: wofür es genutzt wird und wie zentral es für dieses Projekt ist. Zwei bis
drei Kernstellen als `pfad:zeile`.>

## Version und Migration

- Deklariert: `<version>` (`<pin>`) in <Umgebungen>; weitere Versionen je Umgebung, wenn
  das Inventar sie nennt.
- Bekannte Breaking Changes seit …: <knapp, mit Version>
- Upgrade-Pfad, nur wenn er für dieses Projekt zählt: …

## Pitfalls (projektrelevant)

- **<Kurztitel>** — <1–3 Sätze mit Ursache und Symptom>. Ref: `<pfad:zeile>` oder URL.

## Empfohlene Idiome

- <kurz>

## Anti-Patterns

- <kurz>

## Verwandte Themen

- `keph:/docs/generated/code/<NN>-<slug>.md` — <in welchem Zusammenhang>
- `<anderer-steckbrief>.md` — <in welchem Zusammenhang>
```

Regeln:

- **`sources` beginnt mit dem Inventar:** je Version, die das Inventar für den Gegenstand
  nennt, ein Eintrag mit `resource: keph:/docs/generated/versions/inventory.md`,
  `item: <gegenstand>` und `version: <version>`; `item` ist in allen diesen Einträgen
  gleich. Zeilen ohne Version geben keinen eigenen Eintrag. Nennt das Inventar für den
  Gegenstand gar keine Version, steht ein Eintrag ohne `version`. Danach folgen die
  Herstellerquellen, die du tatsächlich gelesen hast, mit ihrer URL — eine nicht gelesene
  URL steht nicht in `sources`.
- **Wie kritisch** ein Gegenstand für das Projekt ist, zeigen die Reihenfolge der Pitfalls
  (das Wichtigste zuerst) und ein Satz unter „Rolle im Projekt“ — kein Feld im Kopf.
- Pitfalls sind **konkret**, mit Ursache und Symptom, nicht allgemein („kann Fehler
  werfen“). Ist keiner bekannt, steht dort ehrlich: „Kein projektrelevanter Pitfall bekannt
  in dieser Version.“
- Keine Tutorials, keine „Getting Started“-Schnipsel, keine abgeschriebene Doku.
- Keinen Abschnitt „Offene Fragen“: Unklarheiten legst du vor
  `publish` vor, statt sie ins Dokument zu schreiben.
- Verwandte Themen im selben Verzeichnis mit ihrem Dateinamen, die Code-Doku und andere
  Teile der Projektdoku als `keph:`-Verweis.
- Neue `hints` schlägst du nur vor; eingetragen werden sie erst nach Bestätigung durch
  den Nutzer.

Nach jeder geschriebenen Datei kurz melden: Dateiname und Zeilenzahl, nicht der Inhalt.
