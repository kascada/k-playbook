---
description: Orchestrate a complete review run through MCP: create or resume a run, start tool scans, execute AI review entries, merge evidence, and guide the final assessment. Draft command while the MCP tools are being defined.
argument-hint: [YYYY-MM-DD|latest|new]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, Task]
---

# k-review-run

## Status dieses Commands

Arbeitsentwurf. Der Command beschreibt den Zielablauf für einen zusammenhängenden
Review-Lauf über MCP. Solange die MCP-Werkzeuge noch fehlen, führt er keine Scanner aus,
sondern dokumentiert den geplanten nächsten Schritt und nennt die vorhandenen CLI-Wege.

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser Sitzung
schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

## Ziel

`/k-review-run` ist der Chat-Einstieg für das neue Laufmodell:

1. Lauf anlegen oder fortsetzen.
2. Werkzeug-Scanner über MCP starten.
3. Offene KI-Review-Einträge als Subtasks ausführen.
4. Merge über MCP starten.
5. Review-Input bewerten und den Handoff nennen.

Der Command vertraut nicht auf Chat-Gedächtnis. Nach jedem Schritt liest er den Zustand aus
dem Laufverzeichnis und entscheidet daraus, was als Nächstes möglich ist.

## Artefakte und Schreibhoheit

Der bestehende Dateikontrakt bleibt erhalten:

| Datei | Schreiber | Rolle |
|---|---|---|
| `run.json` | nur das Anlegen des Laufs | Auswahl und Sprachen, danach unverändert |
| `entries/<tool>.json` | Scanner-Ausführer | Fortschritt, Jobs, Fehler und SARIF-Verweise eines Werkzeugs |
| `entries/<review>.json` | KI-Review-Subtask über MCP | Fortschritt und Ergebnisverweise eines AI-Review-Eintrags |
| `raw/*.sarif` | Scanner-Jobs | Rohbelege |
| `review-input.json` | Merge-Werkzeug | vollständiger Audit-Beleg |
| `review-input.md` | Merge-Werkzeug | kompakte Ansicht für die Bewertung |

`run.json` wird nach dem Anlegen nicht aktualisiert. Fortsetzen entsteht aus den
Entry-Dateien und den Merge-Artefakten.

## Geplante MCP-Werkzeuge

Diese Werkzeuge sind die Zieloberfläche; ihre Namen sind Arbeitstitel:

| Werkzeug | Zweck |
|---|---|
| `k_playbook_review_create` | Lauf mit ausgewählten Tool- und AI-Einträgen anlegen |
| `k_playbook_review_status` | Lauf, Einträge, offene Schritte und Artefakte lesen |
| `k_playbook_review_scan` | Werkzeug-Einträge eines Laufs starten, fachlich `k-playbook scan <lauf>` |
| `k_playbook_review_write_ai_entry` | Status und Ergebnis eines AI-Review-Eintrags schreiben |
| `k_playbook_review_merge` | fachlich `k-playbook merge <lauf>`, schreibt `review-input.*` |
| `k_playbook_review_next_steps` | optional: serverseitig berechneter nächster Schritt |

Bis diese Werkzeuge existieren, darf der Command die CLI nennen, aber keine eigene
Ersatzlogik nachbauen.

## Schritt 1 — Lauf bestimmen

Aus der Context-Ausgabe:

- `RESULTS_DIR = <local.dir>/results`
- `RUNS_DISPLAY_DIR = k-playbook-local/results`
- `TODAY = now.date`

Wenn `RESULTS_DIR` fehlt: fragen, ob genau dieses Verzeichnis angelegt werden soll, oder
`/k-gui` nennen. Keinen Ersatzpfad verwenden.

Argumente:

- `new` oder leer: heutigen Lauf anlegen, wenn noch keiner existiert; sonst den vorhandenen
  heutigen Lauf anbieten.
- `latest`: jüngsten Lauf mit `run.json` verwenden.
- `YYYY-MM-DD`: diesen Lauf verwenden oder, wenn er fehlt, anbieten, ihn anzulegen.

Beim Anlegen werden standardmäßig alle verfügbaren Werkzeug-Einträge und alle aktiven
AI-Review-Einträge ausgewählt. Wenn der Nutzer eine Teilmenge nennt, diese Teilmenge
verwenden und im Abschluss wiederholen.

## Schritt 2 — Status lesen

Immer zuerst den Laufstatus lesen:

- ausgewählte Einträge aus `run.json`
- Ist-Zustand je Eintrag aus `entries/*.json`, fehlend = `start`
- vorhandene `raw/*.sarif`
- vorhandene `review-input.json` und `review-input.md`

Kompakt melden:

```text
Lauf: 2026-08-19
Werkzeuge: 5 done, 1 failed, 0 running, 0 start
KI-Reviews: 0 done, 3 start
Merge: fehlt
Nächster Schritt: KI-Reviews ausführen
```

## Schritt 3 — Scanner starten

Wenn Tool-Einträge auf `start` stehen:

1. Über MCP `k_playbook_review_scan` starten.
2. Während der Lauf läuft, Fortschritt aus MCP-Antworten oder erneutem Statuslesen melden.
3. Nach Abschluss Status erneut lesen.

Technische Fehler sind Entry-Zustände, nicht Laufzustände. Ein `failed`-Tool stoppt den
Command nicht automatisch; der Nutzer entscheidet, ob trotz Fehlern weitergemacht wird.
Fehlergründe stehen in `entries/<tool>.json` und werden im Chat kurz zitiert.

Solange MCP fehlt, den vorhandenen CLI-Weg nennen:

```bash
k-playbook/bin/k-playbook scan <lauf>
```

## Schritt 4 — KI-Review-Einträge ausführen

Wenn AI-Einträge auf `start` stehen:

1. Je AI-Eintrag den passenden Review-Katalogeintrag laden.
2. Einen Subtask starten, der genau diesen Review-Eintrag bewertet.
3. Der Subtask schreibt sein Ergebnis nicht frei irgendwohin, sondern über
   `k_playbook_review_write_ai_entry` in den Eintrag oder in ein vom Eintrag referenziertes
   Artefakt.
4. Danach Status erneut lesen.

Subtasks dürfen parallel laufen, wenn sie nur lesen oder jeweils ihre eigene Entry-Datei
schreiben. Sie dürfen keine fremden Entry-Dateien ändern.

Solange das Schreibwerkzeug fehlt, nicht improvisieren. Stattdessen sagen, dass der
AI-Entry noch nicht maschinenfest ausführbar ist, und den geplanten Review nennen.

## Schritt 5 — Merge starten

Wenn alle gewünschten Tool- und AI-Einträge in einem Endzustand sind, Merge anbieten.

Nicht automatisch mergen, wenn noch AI-Einträge auf `start` stehen. Dann zuerst fragen:

- offene AI-Reviews ausführen,
- bewusst nur die Tool-Belege mergen,
- oder abbrechen und später fortsetzen.

Bei Freigabe über MCP `k_playbook_review_merge` starten. Danach Status erneut lesen und die
Pfade nennen:

```text
Geschrieben:
- k-playbook-local/results/<lauf>/review-input.json
- k-playbook-local/results/<lauf>/review-input.md
```

## Schritt 6 — Bewertung

Nach dem Merge liest der Command `review-input.md` und bei Bedarf `review-input.json`.
Bewertet wird im Chat durch den Assistenten, nicht durch das Merge-Werkzeug.

Ergebnisziel ist noch offen. Bis ein eigener Bewertungs-Kontrakt festgelegt ist, im Chat
klar trennen:

- technische Ausführung und Belege,
- bewertete Findings,
- empfohlener Handoff, z. B. `/k-remediation <pfad>`.

## Fortschritt dieses Entwurfs

- [x] Entscheidung: als Command, nicht als Skill.
- [x] Entscheidung: GUI ist für diese Alternative nicht nötig.
- [x] Entscheidung: MCP orchestriert Scanner und Merge; KI bewertet im Assistenten.
- [x] Entscheidung: `run.json` bleibt Festlegung, Fortschritt steht in Entry-Dateien.
- [ ] MCP-Werkzeuge fachlich genau spezifizieren.
- [ ] MCP-Werkzeuge implementieren.
- [ ] AI-Entry-Dateiformat festlegen.
- [ ] Bewertungsartefakt festlegen.
- [ ] Diese Skizze nach Umsetzung in normale Doku überführen.

## Fehlerfälle

- Kein k-playbook-Projekt: abbrechen und die fehlende Installation nennen.
- Lauf fehlt: anbieten, ihn anzulegen, wenn das Argument ein Datum oder `new` ist.
- `run.json` unlesbar: nicht reparieren; Fehler und Pfad nennen.
- Entry-Datei unlesbar: nicht überschreiben; Fehler und Pfad nennen.
- Scanner technisch fehlgeschlagen: als Tool-Fehler melden, nicht als Befund bewerten.
- MCP-Werkzeug fehlt: geplanten Schritt nennen und vorhandenen CLI-Fallback anzeigen.
