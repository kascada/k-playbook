# PLAYBOOK: Enforcement

**Ziel:** Globale und projektlokale Enforcement-Regeln laden, auf die aktuelle Arbeit anwenden und prüfbar berichten, ob sie eingehalten wurden.

**Rollen:**

- Skill `ks-enforcement`: laufende Anwendung während der Erstellung.
- Command `/k-enforcement`: expliziter Check danach oder zwischendurch.

Beide verwenden dieselbe Regelquelle: die Context-Ausgabe von
`k-playbook/bin/k-playbook context`.

## Regelquellen

Die effektive Regelmenge steht in `catalogs.rules` der Context-Ausgabe. Sie fuehrt die
mitgelieferten Regeln aus `<playbook.dir>/rules/` und die projekteigenen aus
`<local.dir>/rules/` bereits zusammen und haelt je Eintrag die Herkunft fest.

`<playbook.dir>` ist read-only. Mitgelieferte Regeln werden nie editiert; wer eine
aendern will, legt eine gleichnamige Datei unter `<local.dir>/rules/` an.

## Ablauf

### Schritt 1: Ziel bestimmen

Die Regeln werden auf `project.repoRoot` aus der Context-Ausgabe angewendet, nicht auf
das Playbook-Verzeichnis.

### Schritt 2: Regeln laden

Lies die Dateien aus `catalogs.rules` in der gegebenen Reihenfolge. Die Aufloesung ist
bereits erfolgt:

- Jede projekteigene Regel ist aktiv.
- Eine mitgelieferte Regel ist aktiv, ausser eine projekteigene traegt denselben
  Schluessel.
- Eine projekteigene Regel **ersetzt** die gleichnamige mitgelieferte vollstaendig.
  Die mitgelieferte Datei wird dann gar nicht gelesen.
- Eine **leere** projekteigene Regel — nur Leerzeilen und Kommentare — schaltet die
  mitgelieferte ab und ist als `disabled` markiert. Der Kommentar traegt den Grund.

Berichte die effektive Menge mit Herkunft je Eintrag (`dist`, `local`, `override`) sowie
die abgeschalteten Eintraege, bevor du mit der Pruefung beginnst.

Wenn die effektive Menge leer ist, nicht raten und nicht auf die mitgelieferten Regeln
zurueckfallen. Melde den Zustand; eine leere Menge ist eine bewusste Projektentscheidung.

### Schritt 3: Relevanz bestimmen

Für jede Regel entscheiden:

- **relevant:** betrifft die aktuelle Arbeit oder den aktuellen Diff.
- **nicht relevant:** klar außerhalb des Scopes.
- **unklar:** kurze Rückfrage stellen, wenn die Entscheidung das Ergebnis beeinflusst.

Regeln gelten für Änderungen, nicht rückwirkend für den gesamten Legacy-Bestand, außer eine Regel sagt ausdrücklich etwas anderes.

### Schritt 4: Anwenden oder prüfen

Für jede relevante Regel:

- Prüfen, ob die aktuelle Arbeit die Regel einhält.
- Bei Verstoß: entweder direkt beheben, wenn es innerhalb des aktuellen Auftrags liegt, oder den User fragen.
- Bei Unklarheit: nicht spekulieren, sondern kurz fragen.

Spezialfall Code-Änderungen:

- Immer prüfen, ob Docs betroffen sind.
- Relevante Docs im selben Arbeitsgang aktualisieren.
- Wenn keine Doc-Änderung nötig ist, kurz begründen.
- Wenn unklar ist, welche Docs betroffen sind oder ob Doku gewünscht ist, den User fragen.

### Schritt 5: Abschlussbericht

Kurz berichten:

```text
Enforcement
─────────────────────────────
Regeln geladen:  <N global>, <M projektlokal>
Relevant:        <liste oder Anzahl>
Ergebnis:        ok | offen | Verstoß behoben | Rückfrage nötig
Docs-Sync:       angepasst | nicht nötig (<Grund>) | offen
```

Wenn offene Punkte bleiben, diese konkret benennen und nicht als erledigt darstellen.
