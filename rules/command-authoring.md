# Regel: Command-Dateien erzeugen und pflegen

## Zweck

Ein Command ist eine Markdown-Datei, die ein Assistent als Slash-Command ausführt. Alle
Commands sind gleich gebaut, damit man einen unbekannten Command lesen kann, ohne seine
Form erst zu erraten — und damit man eine Command-Datei gegen eine geschriebene
Konvention prüfen kann.

Diese Regel beschreibt die **Form**. Was ein einzelner Command tut, steht in ihm selbst.

## Ablage

Mitgelieferte Commands liegen unter:

`<playbook.dir>/commands/`

Projekteigene Commands liegen unter:

`<local.dir>/commands/`

Beide Seiten werden Datei für Datei verrechnet; der Pfad ab `commands/` ist die
Vergleichseinheit, einschließlich Namensraum. Eine gleichnamige projekteigene Datei
ersetzt die mitgelieferte vollständig, eine leere Datei schaltet den Command ab.

Geschrieben wird nie nach `<playbook.dir>/` — ein Update ersetzt das Verzeichnis.

`commands/_shared/` ist kein Namensraum für Commands, sondern für Module, die Commands
einbinden. Dort liegt heute genau eine Datei: `context.md`.

## Name

- Der Command heißt wie seine Datei ohne `.md`.
- Die H1 der Datei ist derselbe Name, ohne Slash: Datei `k-todo.md`, H1 `# k-todo`.
- Command-Namen bleiben ASCII — sie werden aufgerufen und verglichen, nicht gelesen.

## Frontmatter

Jede Command-Datei beginnt mit YAML-Frontmatter, in genau dieser Reihenfolge:

```yaml
---
description: <ein Satz, was der Command tut und was das Argument bewirkt>
argument-hint: [<argument>]
# model: <provider/modell>
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---
```

- Die Reihenfolge ist verbindlich: `description`, `argument-hint`, `# model:`,
  `allowed-tools`.
- Werte stehen **ohne Anführungszeichen**, solange YAML das zulässt.
- `# model:` ist ein Kommentar, keine Zuweisung: die Zeile hält fest, gegen welches
  Modell der Command entworfen wurde, ohne einem Assistenten ein Modell vorzuschreiben.
- `argument-hint` entfällt nur bei Commands ohne Argument.
- `allowed-tools` nennt genau das, was der Command wirklich braucht.

## Erster Schritt

Auf die H1 folgt die Sektion `## Erster Schritt` mit dem Shared-Include, **wortgleich**
in jeder Command-Datei:

```text
## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.
```

Wortgleich heißt zeichengleich. Wer den Text umformuliert, erzeugt eine zweite Fassung
derselben Aussage, die beim nächsten Mal auseinanderläuft.

Darunter steht ein kurzer Absatz, was der Command tut, und — wenn er Dateien erzeugt —
eine Liste seiner Ergebnisse.

## Schritte

- Jeder Arbeitsschritt ist eine H2 der Form `## Schritt N — Titel`.
- `N` zählt bei 1 hoch, ohne Lücke.
- Titel sind deutsch, das Trennzeichen ist ein Em-Dash mit Leerzeichen davor und danach.
- `## Erster Schritt` trägt diese Nummerierung nicht; er steht vor `## Schritt 1`.

## Pfade

Der erste nummerierte Schritt löst die Pfade auf. Er nimmt sie aus der
Context-Ausgabe und bindet sie an ALL-CAPS-Namen, die der Rest der Datei benutzt:

- `RESOLVED_<X>_DIR` beziehungsweise `<X>_PATH` für den echten Pfad, mit dem gelesen und
  geschrieben wird.
- `<X>_DISPLAY_PATH` für die Fassung, die dem Nutzer angezeigt wird — der kurze,
  projektrelative Pfad.

Ein Command **rät nie einen Pfad und benutzt nie einen Fallback**. Was nicht aus der
Context-Ausgabe kommt, wird nicht verwendet.

## Command-specific policy

Direkt nach der Pfadauflösung, im selben Schritt, steht ein Block

```text
Command-specific policy:
```

mit dem, was nur für diesen Command gilt: welche Dateien fehlen dürfen, was dann
geschieht, welche Vorbedingung ein anderer Command erfüllt haben muss. Alles, was für
alle Commands gilt, gehört nicht hierher, sondern in `_shared/context.md`.

## Fehlendes Verzeichnis

Fehlt ein Verzeichnis, fragt der Command, ob **genau dieses Verzeichnis** angelegt werden
soll oder ob `/k-gui` laufen soll, das die projektlokale Struktur wiederherstellt.

- Kein hartes Abbrechen.
- Kein stilles Anlegen und kein stilles Weiterlaufen.
- Kein Ersatzpfad.

Das ist deckungsgleich mit `_shared/context.md`; die Politik ist dort einmal formuliert
und wird hier nicht neu erfunden.

Sie gilt für die Verzeichnisse, die das Einrichten anlegt — die aus `LocalStructure()`.
**Sein eigenes Erzeuger-Verzeichnis legt ein Command ohne Rückfrage an**: dass es vor dem
ersten Lauf fehlt, ist der Normalzustand, keine kaputte Installation.

## Schluss

Auf den letzten Schritt folgen zwei Sektionen, beide optional, in dieser Reihenfolge:

- `## Fehlerfälle` — was schiefgehen kann und was der Command dann tut.
- `## Anti-Muster (nicht tun)` — die Fehler, die bei diesem Command erfahrungsgemäß
  passieren, je als kurzer Punkt mit Begründung.

Die Überschriften sind fest. Wer eine der beiden Sektionen weglässt, lässt sie weg — er
benennt sie nicht um.

## Handoff

Der letzte Schritt nennt den Folge-Command **wörtlich**, mit Slash, und sagt in einem
Satz, wofür er da ist. Gibt es keinen Nachfolger, sagt der Schritt das ebenso
ausdrücklich. Ein Command, der offen endet, hinterlässt die Frage „und jetzt?" beim
Nutzer.

## Schreibweise

Es gilt `docs/schreibweise.md`: Umlaute und ß in allem, was gelesen wird — Überschriften,
Fließtext, Nutzertexte, Kommentare. ASCII bleibt bei Datei- und Verzeichnisnamen,
Command-Namen, Konfigurationsschlüsseln und den ALL-CAPS-Pfadnamen.

## Qualitätskriterium

Eine gute Command-Datei ist so genau, dass zwei Assistenten mit derselben Eingabe
dieselben Dateien an denselben Stellen erzeugen, und so knapp, dass nichts darin steht,
was schon in `_shared/context.md` steht.
