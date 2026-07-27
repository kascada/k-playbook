# PLAYBOOK: Enforcement

**Ziel:** Globale und projektlokale Enforcement-Regeln laden, auf die aktuelle Arbeit anwenden und prüfbar berichten, ob sie eingehalten wurden.

**Rollen:**

- Skill `ks-enforcement`: laufende Anwendung während der Erstellung.
- Command `/k-enforcement`: expliziter Check danach oder zwischendurch.

Beide verwenden dieselben Regelquellen und dieselbe Pfadauflösung.

## Regelquellen

### Global

Global gelten alle Markdown-Dateien unter:

`<PLAYBOOK_REPO>/global/rules/*.md`

`PLAYBOOK_REPO` folgt dem festen Pfadvertrag:

- Erwartet ist `~/dev/k-playbook`.
- `<TARGET_DIR>/K-PLAYBOOK.MD` darf denselben Wert unter `## Playbook-Quelle` → `repo:` sichtbar enthalten.
- Wenn das physische Repo woanders liegt, muss ein Symlink dafuer sorgen, dass `~/dev/k-playbook` funktioniert.
- Wenn `~/dev/k-playbook` fehlt: warnen und den User auffordern, `/k-install` oder das Devcontainer-Setup auszufuehren.

### Projektlokal

Projektlokale Regeln liegen im `enforcement:`-Pfad aus `<TARGET_DIR>/K-PLAYBOOK.MD`.

Auflösung:

- Wenn `enforcement:` absolut ist: direkt verwenden.
- Wenn relativ: gegen `TARGET_DIR` auflösen.
- Wenn leer, `-` oder fehlend: keine projektlokalen Enforcement-Regeln.
- Wenn der Pfad gesetzt ist, aber fehlt: warnen und nur auf Nachfrage anlegen; für Checks grundsätzlich mit den globalen Regeln fortfahren.

## Ablauf

### Schritt 1: Ziel bestimmen

`TARGET_DIR` ist das Projekt, auf das die Regeln angewendet werden.

- Wenn explizit angegeben: diesen Pfad verwenden, sofern er existiert.
- Sonst: aktuelles Arbeitsverzeichnis.

### Schritt 2: Regeln laden

Lade:

- alle globalen Enforcement-Dateien, sortiert nach Dateiname.
- alle projektlokalen Enforcement-Dateien, sortiert nach Dateiname, falls vorhanden.

Wenn keine Regeln gefunden werden, nicht raten. Melde den Zustand und schlage vor, mit `/k-setup` bzw. einem ersten Eintrag unter `enforcement/` zu starten.

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
