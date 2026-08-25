---
description: Führt ein einzelnes Review-Rezept aus; ohne Argument Auswahl aus aktivierten Review-Rezepten, mit Handoff im Report-Modus als review-input.json und review-triage.md.
argument-hint: [review-name]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-review

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Führe ein gezieltes Review gegen das aktuelle Projekt aus, anhand eines Review-Rezepts
aus dem effektiven Katalog der Context-Ausgabe.

This command owns the **generic** review process. Review files describe **only** what is specific to each review (criteria, style choices, examples, anti-patterns for that review). The rules for writing review recipes live in the `review-authoring` entry of `catalogs.rules`.

## Step 1 — Resolve paths

Recipes come from `catalogs.reviews`; `/k-review` darf nur Einträge mit
`review.enabled: true` auswählen oder explizit ausführen. Fehlt der Block, gilt
`review.enabled: true`; `audit.enabled` steuert ausschließlich `/k-audit` und MCP-Läufe.
Everything a review produces goes under
`<local.dir>/results/`. `known-decisions.md` is not produced by a review — it is a
hand-maintained input and lives one level up. From the context output:

- `RESULTS_DIR` = `<local.dir>/results`
- `RESULTS_DISPLAY_PATH` = `k-playbook-local/results`
- `LOG_FILE` = `<RESULTS_DIR>/log.md`
- `KNOWN_DECISIONS` = `<local.dir>/known-decisions.md`

If `RESULTS_DIR` does not exist: ask whether to create it now or run `/k-gui`; do not
use any fallback path.

## Step 2 — Determine the review to run

Take `catalogs.reviews` from the context output. It already merges shipped and
project-local recipes and marks entries switched off by an empty local file. Each entry
carries its `key` — the filename without `.md` and without the `review-` prefix, so
`review-tech.md` has the key `tech`.

If `$ARGUMENTS` is non-empty: treat it as the review name.

**Name resolution:**
- Normalize the argument to an overlay key: strip a leading `review-` and a trailing `.md`.
- Look the key up in the effective catalog. A project-local recipe already won there,
  so no separate fallback is needed.
- If the key is not in the effective catalog but a local file switches it off,
  say so explicitly instead of reporting it as unknown.
- If the key resolves to a recipe with `review.enabled: false`, abort with: Dieses
  Rezept ist für `/k-review` deaktiviert; es kann höchstens im Audit-Modus aktiv sein.

If `$ARGUMENTS` is empty: build a selection list from the effective catalog and include
only recipes with `review.enabled: true`. Do not show audit-only recipes as selectable.

- For each entry, read its YAML frontmatter (`title`, `interval-weeks`) and, if available, the last log entry (see Step 6) to show `Letzter Lauf`.
- Present as:

```
Verfügbare Reviews:

  [P] review-python-comment-hardspots — Python: Kommentare an Hardspots
       Letzter Lauf: 2025-11-04   Fällig ab: 2026-02-24
  [D] review-tech                   — Tech-Debt-Analyse
       Letzter Lauf: —              Fällig ab: —

  P = projektlokal, D = mitgeliefert, Ü = überlagert eine mitgelieferte Vorlage

Welches Review ausführen?
```

Wait for the user to pick one.

## Step 3 — Load review + known-decisions

Load the resolved review file. Parse the YAML frontmatter into:

- `name`
- `title`
- `interval-weeks` (integer; if missing, default to 12 and note this in the summary)
- `scope-hint` (free text; may be missing)
- `language` (optional; e.g. `python`)
- `handoff` (optional; e.g. `/k-remediation` — see Step 5)
- `result-family` (optional; e.g. `dependency-cve` — for report-mode reviews that use `<RESULTS_DISPLAY_PATH>/<result-family>/<YYYY-MM-DD>/`)
- `review.enabled` (optional; default `true` for this command)
- `audit.enabled` (optional; default `false`, ignored by this command)

If `KNOWN_DECISIONS` is set and the file exists:

- Read it and hold its content in session context.
- Announce briefly: „`known-decisions.md` geladen (N Einträge in Kategorien: …). Findings, die dort gedeckt sind, werden nicht als Fund gemeldet."

If it doesn't exist:

- Warn once: „Keine `known-decisions.md` unter `<Pfad>`. Es kann sein, dass bewusste Entscheidungen als Findings auftauchen — bitte im Zweifel korrigieren."
- Continue.

## Step 4 — Clarify scope

Show the review's `scope-hint` (if present) and ask the user to confirm or narrow the scope:

- Welche Dateien / Verzeichnisse sollen geprüft werden?
- Etwas explizit ausschließen?

Do not proceed until the scope is clear.

## Step 5 — Run the review

There are two execution modes, selected by the review's frontmatter:

### 5a — Interaktiver Modus (Standardfall — kein `handoff` gesetzt)

Generischer Ablauf, der auf jede interaktive Review-Datei angewendet wird:

1. **Scan** nach den Kriterien der Review-Datei. Kandidatenliste intern bilden.
2. **Übersicht** zeigen — kompakt, Pfad+Zeile+Ein-Satz-Beschreibung pro Fund. Bei leerer Liste: ehrlich sagen („nichts gefunden") — das ist gültiges Ergebnis.
3. **Rückfragen bündeln** — falls Gründe / Bewertung nicht ohne User-Input zu klären sind: **alle** Fragen in **einer** Nachricht sammeln, nicht Ping-Pong.
4. **Stelle-für-Stelle**:
   - Stelle mit Kontext zeigen.
   - Reviewspezifischen Vorschlag machen (Kommentar / Fix / Findings-Notiz — was auch immer die Review-Datei vorsieht).
   - **Warten auf Freigabe.** Optionen: `Okay` / `Änder so: ...` / `Skip`.
   - Nur nach `Okay` (oder nach Anpassung + erneutem `Okay`) tatsächlich ändern.
   - Diff/Snippet zeigen, dann nächste Stelle.
5. **Hypothesen-Prinzip** — wenn der User eine Rückfrage nicht beantworten kann: beste Hypothese formulieren, **im Vorschlag** klar als Hypothese markieren (nicht im final einzufügenden Text). Nach Bestätigung ohne Hypothese-Hinweis einfügen.

**Harte Regeln, immer:**

- **Nie mehrere Stellen gleichzeitig ändern.** Pro Stelle: vorschlagen → Freigabe → einfügen → nächste.
- **Kein Refactoring / keine Formatierung „nebenbei".**
- **Keine Vermutungen als Fakten formulieren.** Falsche Begründungen sind schlimmer als gar keine.
- **Reviewspezifischer Scope**, nichts anderes anfassen.

### 5b — Report-Modus (`handoff` ist im Frontmatter gesetzt)

Für Reviews, die ein Ergebnis-Dokument erzeugen statt Stelle-für-Stelle zu moderieren (z. B. `review-tech`):

1. Analyse gemäß Review-Datei durchführen.
2. Ergebnis schreiben. Alles landet unter `RESULTS_DIR`; keinen Ersatzpfad wählen.
   - Wenn `result-family` gesetzt ist: Ergebnisverzeichnis `<RESULTS_DIR>/<result-family>/<YYYY-MM-DD>/` verwenden. Dieses Verzeichnis bei Bedarf anlegen. Vor der Bewertung `review-input.json` schreiben; danach ausschließlich `review-triage.md` als aktuelles Endartefakt schreiben. Der Handoff zeigt immer auf `review-triage.md` in diesem Verzeichnis.
   - Wenn `result-family` nicht gesetzt ist: Summary-Pfad `<RESULTS_DIR>/summary-YYYY-MM-DD.md` verwenden. `RESULTS_DIR` bei Bedarf anlegen. Wenn die Datei existiert, nicht blind überschreiben: nach Bestätigung aktualisieren oder einen eindeutigen Namen vorschlagen, z. B. `summary-YYYY-MM-DD-2.md`.
3. Am Ende: dem User exakten Handoff-Befehl nennen, z. B.:
   `/k-remediation <RESULTS_DIR>/summary-YYYY-MM-DD.md` oder `/k-remediation <RESULTS_DIR>/<result-family>/<YYYY-MM-DD>/review-triage.md`
4. **Kein Log-Eintrag mit „Findings übernommen/geskippt"** — nur Analyse-Lauf + Result-Pfad protokollieren (siehe Step 6).

`review-input.json` im Report-Modus ist der Eingabevertrag für `review-triage.md` und
entspricht strukturell dem Audit-Merge:

```json
{
  "scope": { "type": "review", "family": "<result-family>" },
  "groups": [
    {
      "id": "review-<family>-001",
      "title": "...",
      "findings": ["..."],
      "evidence": [
        { "file": "path", "line": 12, "source": "review:<name>", "message": "..." }
      ],
      "coveredByKnownDecision": false,
      "partialCoverage": false,
      "knownDecisionCoverage": []
    }
  ],
  "ungroupedFindings": [],
  "knownDecisions": { "status": "loaded|missing|empty", "coverage": [] }
}
```

Stabile Bündel-IDs dürfen bei Re-Runs nicht ohne Grund wechseln. Jede Gruppe braucht
mindestens einen Evidence-Eintrag mit Datei, optionaler Zeile und Quelle. Findings ohne
sinnvolle Bündelung bleiben in `ungroupedFindings`, nicht in einer improvisierten Gruppe.

`review-triage.md` im Report-Modus verwendet dieselben Pflichtabschnitte wie das
Audit-Modul `commands/_audit/review-scan-triage.md`: Kopf, `## Bündel`,
`## Bündel-Details`, `## Nicht gebündelt`, `## Deckung aus known-decisions`.
Abschnitte ohne Treffer bleiben vorhanden und enthalten eine kurze Begründung.

## Step 6 — Log-Eintrag

Wenn `LOG_FILE` gesetzt ist:

1. Datei anlegen, falls sie noch nicht existiert (Skelett siehe unten).
2. Sektion für das Review sicherstellen (`## <title>` — anlegen falls nicht vorhanden).
3. Felder aktualisieren:
   - `Letzter Lauf`: `now.date`.
   - `Fällig ab`: `now.date` + `interval-weeks` (als `YYYY-MM-DD`).
4. Eine Zeile ans Protokoll am Dateiende anhängen:

   | Datum | Review | Scope | Output |
   |---|---|---|---|
   | 2026-07-12 | review-python-comment-hardspots | src/upload.py, src/api.py | 3 Vorschläge / 2 übernommen / 1 skip |

Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` empfehlen. Nicht nach einem Ersatzpfad für nur diesen Lauf fragen.

**Log-Skelett** (nur beim ersten Anlegen):

```markdown
# Review-Log

## Protokoll

| Datum | Review | Scope | Output |
|---|---|---|---|
```

Review-spezifische Sektionen (`## <title>` mit `Letzter Lauf` / `Fällig ab`) werden vor der Protokoll-Tabelle hinzugefügt, sobald das jeweilige Review das erste Mal läuft.

## Step 7 — Abschluss

- Kompakte Zusammenfassung: N geprüft / M übernommen / K geskippt / L nicht-adressiert.
- Wenn nicht-adressierte Findings übrig sind: dem User vorschlagen, diese über `/k-task-create` in die Task-Pipeline zu übergeben. Nur auf Bestätigung ausführen.
- Log-Datei-Pfad nennen.
- Bei Report-Modus: Handoff-Befehl noch einmal wörtlich ausgeben.

## Fehlerfälle

- **Review-Name nicht gefunden**: verfügbare Reviews auflisten und um Auswahl bitten (Step 2 wiederholen).
- **Ambiguität** (mehrere Reviews matchen einen Teilnamen): vollständige Kandidatenliste zeigen, exakten Namen erfragen.
- **Kein k-playbook-Projekt**: der Context-Aufruf schlägt fehl; abbrechen und `/k-gui` empfehlen.
- **`RESULTS_DIR` fehlt im Dateisystem**: User fragen, ob genau dieses Verzeichnis angelegt werden soll oder `/k-gui` die Struktur reparieren soll; keinen anderen Pfad verwenden.
