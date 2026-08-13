# Review-Läufe

Wie ein Review-Lauf entsteht, was er auf die Platte legt und wie die Beteiligten ihren
Fortschritt darin festhalten. Für die Artefakte, die ein einzelnes Review erzeugt, siehe
[`reviews-and-results.md`](./reviews-and-results.md); für den Umbau, aus dem das hier
hervorgeht, [`umbau.md`](./umbau.md).

## Der Lauf ist die Klammer

Bisher startete jedes Review seine Werkzeuge selbst, und jedes legte seine Ergebnisse an
seinem eigenen Ort ab. Ein Lauf fasst zusammen, was zusammengehört: **eine Auswahl, ein
Verzeichnis, ein Zustand.**

Ein Lauf besteht aus **Einträgen**, und ein Eintrag hat eine **Art**:

| Art | Was es ist | Wer führt es aus |
|---|---|---|
| `tool` | ein Security-Tool aus der Matrix | das Werkzeug, über eine CLI |
| `ai` | ein Review-Rezept aus dem Katalog | ein Assistent, über einen Command |

Beide sind Einträge desselben Laufs. Nur der Weg dorthin unterscheidet sich — das Ergebnis
landet für beide im selben Verzeichnis und damit später in derselben Zusammenfassung.

## Das Laufverzeichnis

```text
k-playbook-local/results/
└── 2026-08-12/                  der Lauf, benannt nach dem Tag
    ├── run.json                 die Festlegung und der Gesamtzustand
    └── entries/
        ├── semgrep.json         Fortschritt und Probleme je Eintrag
        └── review-secret-scanning.json
```

Der Name ist das Datum, `YYYY-MM-DD`. **Existiert das Verzeichnis bereits, bricht das
Anlegen ab** statt einen zweiten Lauf danebenzustellen: ein Tag, ein Lauf. Wer erneut
starten will, räumt das vorhandene Verzeichnis weg oder benennt es um.

## Wer was schreibt

Der entscheidende Punkt, wenn mehrere Beteiligte gleichzeitig arbeiten: **niemand schreibt
in eine fremde Datei.**

| Datei | Schreiber |
|---|---|
| `run.json` | nur das Werkzeug |
| `entries/<name>.json` | nur der Eintrag, dem sie gehört |

Damit können Werkzeuge parallel laufen, ohne sich gegenseitig zu überschreiben. Eine
einzelne gemeinsame Datei wäre einfacher zu lesen, aber der zweite Schreiber löschte den
Fortschritt des ersten — bei parallelen Scans ist das kein Randfall, sondern der Normalfall.

Wer den Gesamtstand wissen will, liest `run.json` für die Festlegung und die Dateien unter
`entries/` für den Fortschritt. Der Zustand in `run.json` ist der des Laufs, nicht die
Summe der Einträge.

## `run.json`

```json
{
  "schemaVersion": 1,
  "created": "2026-08-12T14:03:11+02:00",
  "state": "created",
  "languages": ["python", "go"],
  "entries": [
    { "name": "semgrep",                "kind": "tool", "state": "start" },
    { "name": "review-secret-scanning", "kind": "ai",   "state": "start" }
  ]
}
```

Die Schlüssel sind englisch, wie alles, was das Werkzeug sonst als JSON ausgibt —
`schemaVersion`, `missingRequired`, `installMethod`. Sie entstehen aus den Go-Feldnamen.

`languages` hält fest, mit welcher Sprachauswahl der Lauf angelegt wurde. Sie kann sich
danach ändern; was gelaufen ist, soll trotzdem nachvollziehbar bleiben.

Die `entries` in `run.json` tragen den Zustand mit, den der Lauf **festgelegt** hat. Den
tatsächlichen Fortschritt führt die Datei unter `entries/`.

## Zustände

Für den Lauf:

| Zustand | Bedeutung |
|---|---|
| `created` | angelegt, noch nichts gestartet |
| `running` | mindestens ein Eintrag läuft oder ist fertig |
| `done` | alle Einträge sind durch |
| `failed` | der Lauf wurde abgebrochen |

Für einen Eintrag:

| Zustand | Bedeutung |
|---|---|
| `start` | ausgewählt, noch nicht gestartet |
| `running` | läuft gerade |
| `done` | fertig, Ergebnis liegt vor |
| `failed` | technisch fehlgeschlagen |
| `skipped` | übersprungen, etwa weil das Werkzeug fehlt |

`failed` meint immer den **technischen** Fehlschlag, nie einen Befund. Ein Scanner, der
Probleme findet, ist `done` — das ist seine Aufgabe.

## Die Oberfläche

Der Block **Workflows** auf der Startseite führt mit einem Knopf zur Seite `/reviews`; die
Zahl darauf ist die der vorhandenen Läufe. Aufgelistet werden sie auf der Seite selbst.
Dort wird auch ein neuer Lauf zusammengestellt:

- **Werkzeuge** aus der Tool-Matrix, gefiltert nach `project.languages`. Was nicht
  installiert ist, steht da, lässt sich aber nicht auswählen — mit dem Hinweis, wie es
  installiert wird.
- **Reviews** aus dem aufgelösten Katalog, mitgeliefert und projekteigen zusammengeführt.
  Abgeschaltete Einträge fehlen.

„Erstellen" legt das Verzeichnis und `run.json` an. Mehr nicht: **das Anlegen startet
nichts.** Wie ein einzelner Eintrag läuft, ist ein eigener Schritt.
