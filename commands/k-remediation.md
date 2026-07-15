---
description: Arbeitet Befunde aus einer Review-Ergebnisdatei (typisch aus /k-review im Report-Modus) strukturiert ab. Verifiziert jeden Befund gegen den echten Code, kategorisiert ihn, behebt sichere Fälle direkt und erstellt Tasks für komplexe. Nur saubere Lösungen — keine Quick-and-Dirty-Fixes.
argument-hint: [result-datei.md]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-remediation

Arbeitet Befunde aus einer Ergebnisdatei strukturiert ab — üblicherweise die Datei, die `/k-review` im Report-Modus (z. B. `review-tech`) erzeugt hat.

Die Pfade werden — wie bei `/k-review` — aus `K-PLAYBOOK.MD` gelesen, damit beide Commands dieselben Verzeichnisse verwenden.

---

## Schritt 1 — Pfade aus K-PLAYBOOK.MD auflösen

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `reviews:` → `PROJECT_REVIEWS_DIR`. `PROJECT_REVIEWS_DIR` ist der aufgelöste absolute Pfad; `-` / fehlend → unset.
- `tasks:` → `TASKS_DIR`. `TASKS_DIR` ist der aufgelöste absolute Pfad; `-` / fehlend → unset.

Daraus abgeleitet:

- Wenn `PROJECT_REVIEWS_DIR` gesetzt und vorhanden:
  - `KNOWN_DECISIONS` = `<PROJECT_REVIEWS_DIR>/known-decisions.md`
  - `DONE_DIR` = `<PROJECT_REVIEWS_DIR>/done/`
- Wenn `TASKS_DIR` gesetzt: dorthin werden Task-Dateien geschrieben.
- Wenn `TASKS_DIR` fehlt / inaktiv: User fragen, ob und wohin Task-Dateien geschrieben werden sollen, sobald in Schritt 6 die erste Kategorie **T** ansteht.

Command-specific policy:

- Wenn `K-PLAYBOOK.MD` fehlt: Hinweis auf `/k-setup`, dann mit Defaults weiterarbeiten (`./tasks`, `./reviews`), falls der User zustimmt — sonst abbrechen.
- Wenn `PROJECT_REVIEWS_DIR` gesetzt aber nicht vorhanden ist: warnen und für Review-Ergebnisdateien interaktiv nachfragen.
- Wenn `TASKS_DIR` gesetzt aber nicht vorhanden ist: erst bei der ersten Kategorie **T** fragen, ob es angelegt oder ein anderer Zielordner verwendet werden soll.

---

## Schritt 2 — Ergebnisdatei bestimmen

Wenn `$ARGUMENTS` angegeben: diese Datei einlesen.

Wenn nicht:

1. In `<PROJECT_REVIEWS_DIR>` nach `result-*.md` suchen (nicht im `done/`-Unterordner).
2. Wenn genau eine: sie vorschlagen und Bestätigung abwarten.
3. Wenn mehrere: als Liste zeigen und den User wählen lassen.
4. Wenn keine: fragen:
   > "Welche Ergebnisdatei soll abgearbeitet werden?"

**Format-Check:** Die Datei sollte eine Befundtabelle mit Statuszeichen (`☐` für offen, sonst `✓`, `~`, `✗`) enthalten, üblicherweise mit Priorität. Wenn das Format nicht plausibel erkennbar ist: sauber abbrechen mit Hinweis, was erwartet wurde, statt zu raten.

Offene Punkte sind mit `☐` markiert (oder haben keine Statusspalte). Alle anderen (✓, ~, ✗) überspringen.

---

## Schritt 3 — known-decisions.md laden

Wenn `KNOWN_DECISIONS` existiert:

- Einlesen und intern als `KNOWN_DECISIONS`-Inhalt bereithalten.
- Kurz bestätigen: „`known-decisions.md` geladen — <N> Einträge."

Wenn die Datei nicht existiert:

- Warnen: „Keine `known-decisions.md` unter `<Pfad>`. Bewusste Entscheidungen können deshalb erneut als Befund auftauchen. Die Datei wird von `/k-setup` initialisiert."
- Weiter — kein automatisches Anlegen an dieser Stelle.

---

## Schritt 4 — Kategorien und Autonomie klären

**Kategorien:**

| Kürzel | Name | Bedeutung |
|--------|------|-----------|
| **S** | Sofort | Klarer Fehler, kleiner gezielter Fix (< 30 min, kein Architektur-Impact, kein Verhaltens-Einfluss für Enduser). Beispiele: falsche Konstante, stilles Ignorieren von Fehlern, veraltete Konfigurationswerte. |
| **T** | Task | Klarer Fehler, aber größerer Eingriff — eigenständige Umsetzung braucht eine Task-Datei. |
| **K** | Klärung | Unklar ob wirklich ein Problem, oder Architekturentscheidung nötig. Erst besprechen. |
| **F** | Feature | Neue Funktionalität, kein Bugfix. Immer erst nachfragen — nie autonom umsetzen. |
| **A** | Akzeptiert | Bekanntes Design, bewusste Entscheidung oder externe Behandlung. Mit Grund dokumentieren und schließen. |
| **X** | Falsch | Befund ist nicht korrekt oder nicht relevant. Dokumentieren und schließen. |

Frage den User:
> "Welche Kategorien darf ich autonom abarbeiten?
> - **S (Sofort)** — direkt fixen
> - **T (Task)** — Task-Datei anlegen
> - **A/X** — dokumentieren und schließen
>
> Oder soll ich jeden Punkt erst vorstellen und du entscheidest?"

Warte auf Antwort. Merke welche Kategorien autonom behandelt werden dürfen (`AUTO_CATEGORIES`).

---

## Schritt 5 — Befunde einlesen und sortieren

Alle offenen Befunde aus der Datei sammeln. Falls die Datei eine Prioritätsspalte enthält: nach Priorität absteigend sortieren (höchste zuerst).

Übersicht ausgeben:
```
Befunde geladen: <N> offen
Autonom: <liste der freigegebenen Kategorien>
```

---

## Schritt 6 — Jeden Befund abarbeiten

Für jeden offenen Befund der Reihe nach:

### 6a — Code lesen und verifizieren

**Immer zuerst den echten Code lesen** — nie auf Basis der Befundbeschreibung allein handeln.

- Datei und Zeile aus dem Befund aufsuchen (Read/Grep).
- Prüfen: Ist das Problem real? Ist die Beschreibung korrekt?
- Prüfen: Hat sich der Code seit der Analyse geändert (Problem vielleicht schon behoben)?

Wenn das Problem nicht reproduzierbar oder bereits behoben ist → Kategorie **X**, weiter.

### 6b — Kategorisieren

**Vorprüfung gegen known-decisions.md:** Wenn `KNOWN_DECISIONS` geladen ist, zuerst prüfen ob der Befund inhaltlich zu einem Eintrag passt (Bereich, Thema, Beschreibung). Wenn ja → Kategorie automatisch **A (Akzeptiert)**, Grund aus dem KD-Eintrag übernehmen. Den User kurz informieren:
> "Befund #N → A (akzeptiert) — trifft auf KD-NNN: <Titel> zu."

Wenn kein KD-Treffer: Kategorie anhand der Definitionen (Schritt 4) bestimmen. Im Zweifel konservativer einordnen (K statt S, T statt S).

**Qualitätsleitlinien für Sofort-Fixes:**
- Kein Quick-and-Dirty. Wenn es keine saubere Lösung gibt, wird aus **S** ein **T**.
- Bei mehreren Lösungsoptionen: die elegantere und sicherere wählen. Beispiel: eine etablierte Library einem selbst geschriebenen Workaround vorziehen.
- Fix muss build- und testbar sein (`go build ./...` bzw. entsprechendes).

### 6c — Handeln

**Kategorie S (Sofort) — in `AUTO_CATEGORIES`:**
1. Fix direkt anwenden
2. Build/Tests prüfen
3. Status in Ergebnisdatei auf `✓ behoben` setzen
4. Im Änderungslog (Schritt 7) eintragen

**Kategorie S — NICHT in `AUTO_CATEGORIES`:**

**Pflicht: Code zeigen, dann vorstellen — niemals blind eine Liste abfragen.**

Für jeden Befund einzeln:

1. **Code lesen** (Schritt 6a wurde bereits gemacht)
2. **Vorstellen mit konkretem Code-Ausschnitt:**
   - Den relevanten Code-Block (Ist-Stand) zeigen
   - Das Problem in 1–2 Sätzen erklären
   - Den geplanten Fix als Code-Diff oder konkreten Codeblock zeigen
3. **Fragen:**
   > "Soll ich das so beheben?"
4. Antwort abwarten, dann entsprechend handeln.

Nicht erlaubt:
- Mehrere S-Befunde in einer Auswahlliste bündeln ohne Code-Details
- Nur Befundtitel oder Beschreibung aus dem Audit nennen ohne den tatsächlichen Code-Stand zu zeigen
- Batch-Fragen wie „Welche davon soll ich fixen?" ohne dass der User den Code kennt

**Kategorie T (Task) — in `AUTO_CATEGORIES`:**

Task-Datei nach den Regeln von `/k-task-create` anlegen. Siehe `commands/k-task-create.md` — die Datei dort ist maßgeblich; hier nur der Minimalkern, damit der Flow nicht bricht:

1. Ziel-Verzeichnis: `TASKS_DIR` (aus Schritt 1). Wenn nicht gesetzt: einmal fragen und für den Rest des Laufs merken.
2. Nummer: nächste freie über `<TASKS_DIR>/*.md` und `<TASKS_DIR>/old/*.md` bestimmen, zero-padded auf 3 Stellen (siehe `k-task-create.md`, Step 2).
3. Dateiname: `<NNN>-<kurzname>.md` — Kurzname aus Befundtitel abgeleitet (lowercase, hyphens; siehe `k-task-create.md`, Step 3).
4. Inhalt: Struktur aus `k-task-create.md`, Step 6 (Intent, Referenzen, Tools, Ziel, Kontext, Zu bauen). Kontext = Befundtext + Verweis auf die Ergebnisdatei. Ziel = die saubere Lösung (kein Quick-and-Dirty).
5. Status in Ergebnisdatei auf `✓ Task NNN` setzen.
6. Im Änderungslog eintragen.

**Kategorie T — NICHT in `AUTO_CATEGORIES`:**
Befund vorstellen und fragen ob Task anlegen.

**Kategorie K (Klärung):**
Befund vorstellen:
- Was genau ist das Problem (nach Code-Lektüre)
- Warum unklar / welche Architekturentscheidung steht dahinter
- Mögliche Optionen (ohne eine zu empfehlen)

Auf User-Entscheidung warten. Je nach Entscheidung als S/T/A/X weiterbehandeln.

**Kategorie F (Feature):**
Immer vorstellen und fragen — auch wenn `AUTO_CATEGORIES` alles enthält.
> "Das ist eine Funktionserweiterung. Soll ich dafür einen Task anlegen?"

**Kategorie A (Akzeptiert) — in `AUTO_CATEGORIES`:**
Status auf `~ akzeptiert` setzen. Kurzen Grund in den Änderungslog schreiben.

Danach — sofern `KNOWN_DECISIONS` vorhanden ist und der Befund *nicht* durch einen KD-Treffer automatisch geschlossen wurde — fragen:
> "Soll diese Entscheidung in `known-decisions.md` eingetragen werden, damit sie bei zukünftigen Reviews automatisch als 'Akzeptiert' gilt?"

Wenn ja: nächste freie ID bestimmen (KD-NNN), Eintrag am Ende der Datei ergänzen:

```markdown
### KD-NNN — <Kurztitel>
**Bereich:** <Datei / Modul / Konzept>
**Entscheidung:** <Begründung aus dem Befund>
**Datum:** YYYY-MM-DD
```

Kurz bestätigen: „KD-NNN eingetragen."

**Kategorie X (Falsch) — in `AUTO_CATEGORIES`:**
Status auf `✗ falsch` setzen. Kurze Begründung notieren.

---

## Schritt 7 — Ergebnisdatei aktualisieren

Nach jedem bearbeiteten Befund:

**Statusspalte:** Falls die Tabelle noch keine `**Status**`-Spalte hat, diese hinzufügen.

Statuswerte:
| Symbol | Bedeutung |
|--------|-----------|
| `✓ behoben` | Direkt gefixt |
| `✓ Task NNN` | Task-Datei angelegt |
| `~ akzeptiert` | Bekannt/bewusst, kein Handlungsbedarf |
| `✗ falsch` | Befund nicht korrekt |
| `☐` | Noch offen |

**Änderungslog:** Am Ende der Datei einen Abschnitt pflegen (anlegen wenn nicht vorhanden):

```markdown
---

## Änderungslog

| Datum | # | Kategorie | Aktion | Notiz |
|-------|---|-----------|--------|-------|
| YYYY-MM-DD | 12 | Sofort | ✓ behoben | TLS MinVersion 1.2 → 1.3 |
| YYYY-MM-DD | 3  | Task   | ✓ Task 002 | Log-Streaming, tasks/002-log-streaming.md |
| YYYY-MM-DD | 13 | Akzeptiert | ~ | Rate-Limiting extern behandelt (SecurityConfig) |
```

---

## Schritt 8 — Ergebnisdatei archivieren

Wenn alle Befunde abgearbeitet sind (keine ☐ mehr offen):

1. Ziel-Verzeichnis bestimmen:
   - Wenn `DONE_DIR` (`<PROJECT_REVIEWS_DIR>/done/`) gesetzt ist: dort archivieren. Verzeichnis bei Bedarf anlegen.
   - Wenn nicht gesetzt (kein `PROJECT_REVIEWS_DIR`): fragen:
     > "Wohin soll die abgeschlossene Datei verschoben werden? (Vorschlag: `done/` neben der Datei)"

2. Datei verschieben:
   - Neuer Name: `YYYY-MM-DD-<originalname>` (heutiges Datum voranstellen)
   - Beispiel: `result-review-tech.md` → `<DONE_DIR>/2026-07-12-result-review-tech.md`

3. Kurz bestätigen: „Archiviert: `<DONE_DIR>/YYYY-MM-DD-<name>`"

Wenn noch offene Befunde (☐) vorhanden: Datei **nicht** verschieben — sie bleibt offen.

---

## Schritt 9 — Abschlusszusammenfassung

Nach allen Befunden ausgeben:

```
Remediation abgeschlossen
─────────────────────────────────────
Bearbeitet:   <N>
✓ behoben:    <n>
✓ Tasks:      <n>  (<Task-Nummern>)
~ akzeptiert: <n>
✗ falsch:     <n>
☐ offen:      <n>  (K/F — warten auf Klärung)
Archiviert:   <DONE_DIR>/YYYY-MM-DD-<name>  (oder: — offen, nicht archiviert)
```

Wenn noch offene K- oder F-Punkte vorhanden: diese auflisten mit kurzer Begründung warum sie offen blieben.

---

## Fehlerfälle

- **Ergebnisdatei nicht gefunden / nicht plausibel**: verfügbare `result-*.md` in `<PROJECT_REVIEWS_DIR>` auflisten, User wählen lassen. Bei Formatabweichung: abbrechen statt raten.
- **`K-PLAYBOOK.MD` fehlt**: Hinweis auf `/k-setup`, dann Defaults nutzen (falls User zustimmt) oder abbrechen.
- **`TASKS_DIR` inaktiv**: einmal fragen, wohin Tasks geschrieben werden sollen; Antwort für den Rest des Laufs merken.
