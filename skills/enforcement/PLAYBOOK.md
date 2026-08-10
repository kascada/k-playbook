# PLAYBOOK: Enforcement

**Ziel:** Globale und projektlokale Enforcement-Regeln laden, auf die aktuelle Arbeit anwenden und prüfbar berichten, ob sie eingehalten wurden.

**Rollen:**

- Skill `ks-enforcement`: laufende Anwendung während der Erstellung.
- Command `/k-enforcement`: expliziter Check danach oder zwischendurch.

Beide verwenden dieselben Regelquellen und dieselbe Pfadauflösung.

## Regelquellen

Es gibt zwei Quellen, die per Overlay zu einer effektiven Regelmenge kombiniert werden.

### Mitgeliefert

Mitgelieferte Regeln liegen unter `<DIST_DIR>/rules/*.md`.

`DIST_DIR` wird nicht geraten und folgt keinem Hostpfad. Es kommt aus der Discovery
in `<DIST_DIR>/commands/_shared/path-resolution.md`: `PLAYBOOK_DIR` wird vom
Arbeitsverzeichnis aus aufwärts gesucht, `DIST_DIR` ergibt sich aus
`k_playbook.dist` in `K-PLAYBOOK.yaml`.

`<DIST_DIR>` ist read-only. Mitgelieferte Regeln werden nie editiert.

### Projektlokal

Projektlokale Regeln liegen im konfigurierten `paths.enforcement`.

## Ablauf

### Schritt 1: Ziel bestimmen

Wende `<DIST_DIR>/commands/_shared/path-resolution.md` an und löse `enforcement` auf.

Ergebnis: `PLAYBOOK_DIR`, `DIST_DIR`, `RESOLVED_ENFORCEMENT_DIR` und
`PROJECT_REPO_ROOT_DIR`. Die Regeln werden auf `PROJECT_REPO_ROOT_DIR` angewendet.

### Schritt 2: Regeln laden

Wende `<DIST_DIR>/commands/_shared/overlay-resolution.md` für die Art `rules` an.

Damit gilt:

- Jede projektlokale Regel ist aktiv.
- Eine mitgelieferte Regel ist aktiv, außer eine projektlokale Regel trägt denselben
  Schlüssel oder der Schlüssel steht in `overlay.rules.disabled`.
- Eine projektlokale Regel **ersetzt** die gleichnamige mitgelieferte vollständig.
  Die mitgelieferte Datei wird dann gar nicht gelesen.

Berichte die effektive Menge mit Herkunft je Eintrag (`dist`, `local`, `override`)
sowie abgeschaltete und veraltete `disabled`-Einträge, bevor du mit der Prüfung
beginnst.

Wenn die effektive Menge leer ist, nicht raten und nicht auf die mitgelieferten
Regeln zurückfallen. Melde den Zustand; eine leere Menge ist eine bewusste
Projektentscheidung.

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
