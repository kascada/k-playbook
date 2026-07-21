---
description: Execute a review from the review catalog (global playbook or project-local). Handles all generic orchestration - known-decisions lookup, one-by-one moderation, log update - so review files only describe review-specific content. Pass a review name as argument, or omit to pick from a list.
argument-hint: [review-name]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-review

Run a code review against the current project, using a review recipe either from the global playbook (`~/dev/k-playbook/review/`) or from the project-local `reviews:` path in `K-PLAYBOOK.MD`.

This command owns the **generic** review process. Review files describe **only** what is specific to each review (criteria, style choices, examples, anti-patterns for that review).

`/k-review` does not guess project paths. The project must have `K-PLAYBOOK.MD` and `base:` configured by `/k-setup`. The `reviews:` block may be inactive; in that case only global review recipes are available and no project-local log or known-decisions file is used.

## Step 1 — Resolve paths from K-PLAYBOOK.MD

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `reviews:` → `PROJECT_REVIEWS_DIR`. Treat `PROJECT_REVIEWS_DIR` as the resolved absolute path. If value is `-` or entry is missing, `PROJECT_REVIEWS_DIR` is unset.

Also require `base:` from `K-PLAYBOOK.MD`; use it only as validation metadata, not to infer `reviews:`.

Also set:

- `GLOBAL_REVIEWS_DIR` = `<PLAYBOOK_REPO>/k-playbook/review/` if that directory exists, otherwise `<PLAYBOOK_REPO>/review/` if that exists. If neither exists, continue with project-local reviews only if configured; otherwise abort because no review catalog is available.
- If `PROJECT_REVIEWS_DIR` is set:
  - `LOG_FILE` = `<PROJECT_REVIEWS_DIR>/log.md`
  - `KNOWN_DECISIONS` = `<PROJECT_REVIEWS_DIR>/known-decisions.md`
  - `RESULT_DIR` = `<PROJECT_REVIEWS_DIR>/` (for reviews that produce output files, e.g. tech-debt)
- Else:
  - `LOG_FILE` = unset — announce that no log will be written. Do not ask for a one-off log path.
  - `KNOWN_DECISIONS` = unset — warn but continue.
  - `RESULT_DIR` = unset. Report-mode reviews that need to write a result require an active existing `reviews:` directory; otherwise abort with a `/k-setup` hint.

Command-specific policy:

- If `K-PLAYBOOK.MD` is missing: abort and tell the user to run `/k-setup` first.
- If `base:` is missing: abort and tell the user to run `/k-setup` first. Do not infer it from existing paths.
- If `reviews:` is unset or inactive (`-`): continue with global reviews only. Do not invent a project-local review path.
- If `PROJECT_REVIEWS_DIR` is set but missing: abort and tell the user to run `/k-setup` to create/migrate the configured directory.

## Step 2 — Determine the review to run

If `$ARGUMENTS` is non-empty: treat it as the review name.

**Name resolution:**
- Try `<PROJECT_REVIEWS_DIR>/review-<name>.md` first (project-local wins on collision).
- Fall back to `<GLOBAL_REVIEWS_DIR>/review-<name>.md`.
- Also accept the bare filename (`review-<name>.md`) or the full name including the `review-` prefix.

If `$ARGUMENTS` is empty: build a selection list.

- Collect all `review-*.md` from `<PROJECT_REVIEWS_DIR>` (if set) and `<GLOBAL_REVIEWS_DIR>`.
- Merge by base filename. On collision: project-local wins; mark the global entry as *überlagert*.
- For each entry, read its YAML frontmatter (`title`, `interval-weeks`) and, if available, the last log entry (see Step 6) to show `Letzter Lauf`.
- Present as:

```
Verfügbare Reviews:

  [P] review-python-comment-hardspots — Python: Kommentare an Hardspots
       Letzter Lauf: 2025-11-04   Fällig ab: 2026-02-24
  [G] review-tech                   — Tech-Debt-Analyse
       Letzter Lauf: —              Fällig ab: —

  P = projekt-lokal, G = global

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
2. Ergebnis in `<RESULT_DIR>/result-<name>.md` schreiben. If `RESULT_DIR` is unset, abort and ask the user to activate/create `reviews:` via `/k-setup` first.
3. Am Ende: dem User exakten Handoff-Befehl nennen, z. B.:
   `/k-remediation <RESULT_DIR>/result-review-tech.md`
4. **Kein Log-Eintrag mit „Findings übernommen/geskippt"** — nur Analyse-Lauf + Result-Pfad protokollieren (siehe Step 6).

## Step 6 — Log-Eintrag

Wenn `LOG_FILE` gesetzt ist:

1. Datei anlegen, falls sie noch nicht existiert (Skelett siehe unten).
2. Sektion für das Review sicherstellen (`## <title>` — anlegen falls nicht vorhanden).
3. Felder aktualisieren:
   - `Letzter Lauf`: heute (`YYYY-MM-DD`).
   - `Fällig ab`: heute + `interval-weeks` (als `YYYY-MM-DD`).
4. Eine Zeile ans Protokoll am Dateiende anhängen:

   | Datum | Review | Scope | Output |
   |---|---|---|---|
   | 2026-07-12 | review-python-comment-hardspots | src/upload.py, src/api.py | 3 Vorschläge / 2 übernommen / 1 skip |

Wenn `LOG_FILE` nicht gesetzt ist: dem User zeigen, was hätte geschrieben werden sollen. Nicht nach einem Ersatzpfad fragen; Projektpfade werden über `/k-setup` registriert.

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
- **`K-PLAYBOOK.MD` fehlt oder `base:` fehlt**: abbrechen und `/k-setup` aufrufen lassen.
