---
description: Arbeitet Befunde aus einer Review-Ergebnisdatei strukturiert ab. Plant zuerst sinnvolle Remediation-Buendel nach Risiko, Aufwand und Kopplung, beachtet die projektlokale Remediation-Policy und erzeugt je nach Modus Tasks statt direkte Fixes.
argument-hint: [result-datei.md]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-remediation

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Arbeitet Befunde aus einer Ergebnisdatei strukturiert ab — üblicherweise die Datei, die `/k-review` im Report-Modus erzeugt hat. Vor der Umsetzung wird immer ein Remediation-Plan gebildet: welche Findings zusammen gehoeren, was zuerst kommt, was Quick-Win ist und was einen eigenen Task/Branch/PR braucht.

Unterstützt drei Formate:

- Result-Summaries wie `k-playbook-local/results/summary-YYYY-MM-DD.md` mit priorisierten Remediation-Gruppen.
- Legacy-Ergebnisdateien wie `k-playbook-local/results/result-*.md` mit Statuszeichen-Tabellen.
- Result-Familien wie `k-playbook-local/results/<family>/<date>/assessment.md` mit zugehoerigem `findings.md`, z. B. `dependency-cve` oder `k-check`.

Der `remediation`-Block der Context-Ausgabe legt fest, wie gearbeitet wird:

- `mode` - `task-branch-pr`, `task-first` oder `direct-allowed`.
- `target` - tatsaechlicher Code-/Git-Root, z. B. `./app` bei Wrapper-Repos.
- `grouping` - ob Findings vor der Umsetzung zu sinnvollen Buendeln zusammengefasst werden.
- `quickWins` - ob einfache, wirkungsstarke Buendel hervorgehoben werden.
- `branchPrefix` - empfohlener Branch-Prefix fuer Remediation-Branches.
- `prRequired` - ob ein PR Teil des erwarteten Workflows ist.
- `directFixes` - ob direkte Code-Fixes ohne Task erlaubt sind.

Wenn `configured` false ist, stoppe und bitte darum, die Remediation-Policy per `/k-gui` zu setzen, oder frage fuer die aktuelle Session explizit.

---

## Schritt 1 — Pfade auflösen

Aus der Context-Ausgabe:

- `RESULTS_DIR` = `<local.dir>/results`
- `TASKS_DIR` = `<local.dir>/tasks` — Zielverzeichnis für neue Task-Dateien.
- `KNOWN_DECISIONS` = `<RESULTS_DIR>/known-decisions.md`
- `DONE_DIR` = `<RESULTS_DIR>/done/`
- `REMEDIATION_MODE`, `REMEDIATION_TARGET_DIR`, `REMEDIATION_TARGET_DISPLAY`, `REMEDIATION_GROUPING`, `REMEDIATION_QUICK_WINS`, `REMEDIATION_BRANCH_PREFIX`, `REMEDIATION_PR_REQUIRED`, `REMEDIATION_DIRECT_FIXES` aus dem optionalen Remediation-Block.
- `REMEDIATION_BASE_BRANCH` = aktueller Branch im Target-Repo, falls `target:` ein Git-Repo ist; sonst unset. Bestimme ihn mit `git branch --show-current` im Target-Root. Wenn leer: nutze keinen geratenen Wert, sondern schreibe `Base branch: <manual>` in erzeugte Tasks.

Command-specific policy:

- Wenn `RESULTS_DIR` oder `TASKS_DIR` nicht existiert: fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen. Keinen anderen Pfad verwenden.
- Wenn `mode: task-branch-pr` oder `mode: task-first` gesetzt ist, muessen Remediation-Schritte als Tasks/Buendel geplant werden. Direkte Code-Aenderungen sind nur erlaubt, wenn `direct_fixes: true` und der User den konkreten Fix nach Code-Sichtung bestaetigt.
- Wenn `target:` gesetzt ist, muss der Pfad existieren. Code-Verifikation und Branch-/Git-Hinweise beziehen sich auf diesen Target-Root, nicht zwingend auf `project.repoRoot`.
- Wenn `mode: task-branch-pr` gilt und `target:` ein Git-Repo ist, pruefe vor Task-Erzeugung den aktuellen Branch und Dirty-State des Target-Repos. Bei Dirty-State keine Branch-/Task-Policy raten: User informieren und bestaetigen lassen, ob Tasks trotzdem erzeugt werden sollen. `/k-remediation` wechselt selbst keinen Branch fuer spaetere Umsetzung; es schreibt den erforderlichen Ausfuehrungskontext in die Task-Dateien.

---

## Schritt 2 — Ergebnisdatei bestimmen

Wenn `$ARGUMENTS` angegeben: diese Datei einlesen.

Akzeptierte direkte Argumente:

- `<RESULTS_DIR>/result-*.md`
- `<RESULTS_DIR>/summary-*.md`
- `<RESULTS_DIR>/<family>/<date>/assessment.md`
- Projektrelative Varianten davon, z. B. `k-playbook-local/results/k-check/2026-07-23/assessment.md`

Wenn nicht:

1. In `<RESULTS_DIR>` nach `result-*.md` und `summary-*.md` suchen (nicht im `done/`-Unterordner) sowie nach `<RESULTS_DIR>/*/*/assessment.md`.
2. Wenn genau eine: sie vorschlagen und Bestätigung abwarten.
3. Wenn mehrere: als Liste zeigen und den User wählen lassen.
4. Wenn keine: fragen:
    > "Welche Ergebnisdatei soll abgearbeitet werden?"

**Result-Family-Erkennung:** Wenn die Datei `assessment.md` heisst und der Pfad auf `<RESULTS_DIR>/<family>/<date>/assessment.md` endet:

- Setze `RESULT_FORMAT=result-family`.
- Setze `RESULT_FAMILY=<family>` und `RESULT_DATE=<date>`.
- Erwarte `findings.md` im selben Verzeichnis.
- Lies `findings.md` als primaeres Arbeitsregister.
- `assessment.md` bleibt Quelle/Kurzbewertung und darf nur nachvollziehbar fuer Summary, Handoff-Status oder explizite Remediation-Abschnitte aktualisiert werden.
- `raw/` und `run-metadata.*` im selben Verzeichnis sind auditierbar und duerfen nicht veraendert werden.

**Summary-Erkennung:** Wenn die Datei `summary-*.md` heisst und direkt unter `<RESULTS_DIR>/` liegt:

- Setze `RESULT_FORMAT=summary`.
- Die Summary-Datei ist die Quelle fuer priorisierte Remediation-Gruppen und die Arbeitsdatei fuer Handoff-/Task-Status.
- Verlinkte `assessment.md`-/`findings.md`-Quellen aus der Summary duerfen mitgeladen werden. `findings.md` bleibt das Arbeitsregister der jeweiligen Result-Familie, wenn konkrete Finding-IDs aktualisiert werden.

**Legacy-Format-Erkennung:** Sonst `RESULT_FORMAT=legacy` und die Ergebnisdatei selbst als Arbeitsregister verwenden.

**Format-Check Legacy:** Die Datei sollte eine Befundtabelle mit Statuszeichen (`☐` für offen, sonst `✓`, `~`, `✗`) enthalten, üblicherweise mit Priorität. Wenn das Format nicht plausibel erkennbar ist: sauber abbrechen mit Hinweis, was erwartet wurde, statt zu raten.

**Format-Check Result-Family:** `findings.md` muss Markdown-Headings fuer einzelne Findings enthalten und darunter mindestens ein Statusfeld in der Form `- Status: `<wert>``. Wenn `findings.md` fehlt oder kein Statusfeld erkennbar ist: sauber abbrechen. Nicht aus `assessment.md` neue Finding-IDs erraten.

Offene Punkte im Legacy-Format sind mit `☐` markiert (oder haben keine Statusspalte). Alle anderen (✓, ~, ✗) überspringen.

Offene Punkte im Result-Family-Format sind Findings mit Status `open`, `confirmed` oder `context-needed`. `likely-false-positive` ist review-relevant, aber nur nach expliziter User-Auswahl remediation-relevant. `accepted` und `fixed` sind Endzustaende und duerfen nicht automatisch in neue Fix-Tasks ueberfuehrt werden.

---

## Schritt 3 — known-decisions.md laden

Wenn `KNOWN_DECISIONS` existiert:

- Einlesen und intern als `KNOWN_DECISIONS`-Inhalt bereithalten.
- Kurz bestätigen: „`known-decisions.md` geladen — <N> Einträge."

Wenn die Datei nicht existiert:

- Warnen: „Keine `known-decisions.md` unter `<Pfad>`. Bewusste Entscheidungen können deshalb erneut als Befund auftauchen. Die Datei wird ueber `/k-gui` initialisiert."
- Weiter — kein automatisches Anlegen an dieser Stelle.

---

## Schritt 4 — Arbeitsmodus, Buendelung und Autonomie klaeren

Zeige zuerst die geladene Remediation-Policy:

```text
Remediation-Policy
──────────────────
Modus:        task-branch-pr | task-first | direct-allowed
Target:       <REMEDIATION_TARGET_DISPLAY>
Base branch:  <REMEDIATION_BASE_BRANCH or "<manual>">
Grouping:     true | false
Quick-Wins:   true | false
Branch:       <branch_prefix><task-or-bundle>
PR required:  true | false
Direct fixes: true | false
```

Modus-Semantik:

- `task-branch-pr`: Keine direkten Code-Fixes aus `/k-remediation`. Findings werden zu Remediation-Buendeln gruppiert; jedes akzeptierte Buendel erzeugt eine Task-Datei mit Branch-/PR-Hinweis. Umsetzung erfolgt spaeter ueber `/k-run` oder einen dedizierten Dev-Flow auf Branch + PR.
- `task-first`: Standard ist Task-Erzeugung pro Buendel; direkte Fixes nur nach expliziter User-Freigabe fuer einzelne kleine Buendel.
- `direct-allowed`: Kleine sichere `S`-Findings duerfen nach Code-Sichtung direkt behoben werden, wenn der User die Kategorien freigibt.

Wenn `mode` fehlt oder unbekannt ist: stoppe und bitte um `/k-gui` oder explizite Auswahl fuer diese Session.

### Buendelung vor Einzelarbeit

Vor dem Abarbeiten einzelner Befunde muss `/k-remediation` eine Planungsuebersicht erzeugen:

1. Alle remediation-relevanten Findings laden.
2. Findings nach Kopplung gruppieren:
   - gleicher Package-/Dependency-Upgrade-Pfad, z. B. `Django >= 5.2.15`.
   - gleicher Codebereich oder gleiche Datei/Komponente.
   - gleiche Root Cause / gleiche Konfiguration.
   - gleiche Verifikationsroute, z. B. ein Test-/Build-Lauf deckt mehrere Findings ab.
3. Pro Buendel einschaetzen:
   - Risiko: `P1/P2/P3`, critical/high/medium/low, Runtime vs. Dev/Build.
   - Aufwand: `S` klein, `M` mittel, `L` gross/unsicher.
   - Kopplung: welche Findings sollten gemeinsam geloest werden.
   - Quick-Win: hohe Wirkung bei kleinem Aufwand und klarer Verifikation.
4. Reihenfolge vorschlagen:
   - Zuerst P1 + kleiner/mittlerer Aufwand.
   - Dann P1 gross oder P2 mit klarer Verifikation.
   - Dann Dev-/Build-only und P3.
5. Dem User die Buendel-Liste zeigen und bestaetigen lassen, bevor Tasks erzeugt oder Fixes gemacht werden.

Beispiel-Ausgabe:

```text
Remediation-Plan
────────────────
1. P1/S Quick-Win: python-jose >= 3.4.0 (depbot-7, depbot-8)
2. P1/M: Django Patch-Level (24 Findings)
3. P1/M: Pillow Patch-Level (17 Findings)
4. P2/S: azure-core >= 1.38.0

Vorschlag: Fuer die ersten 3 Buendel Tasks erzeugen.
```

Im Modus `task-branch-pr` sind diese Buendel die Einheit fuer Task/Branch/PR, nicht zwingend einzelne Findings.

### Kategorien

Kategorien gelten nach der Buendelplanung fuer einzelne Findings oder ganze Buendel:

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

Wenn `mode: task-branch-pr` gilt, ersetze die Frage durch:

> "Dieses Projekt nutzt `task-branch-pr`. Ich werde keine direkten Fixes machen. Soll ich fuer die bestaetigten Buendel Task-Dateien mit Branch-/PR-Hinweis erzeugen?"

Warte auf Antwort. Merke welche Kategorien autonom behandelt werden dürfen (`AUTO_CATEGORIES`).

---

## Schritt 5 — Befunde einlesen, gruppieren und sortieren

Alle offenen Befunde aus der Arbeitsdatei sammeln. Bei Summary ist die Arbeitsdatei die Summary-Datei mit ihren priorisierten Gruppen. Bei Legacy ist die Arbeitsdatei die Ergebnisdatei. Bei Result-Familien ist die Arbeitsdatei `findings.md`; `assessment.md` wird als Quelle mitgeladen.

Result-Family-Parsing:

- Finding-ID ist die Markdown-Heading-ID, z. B. `### kcheck-logging-003` oder `### py/full-ssrf-001`.
- Die ID muss aus `findings.md` stammen; Task-Erzeugung darf keine neue ID fuer ein bestehendes Finding erzeugen.
- Erfasse `Status`, `Prioritaet`, `Quelle`, `Ort`, `Message`, `Raw-Quelle`, `Review-Bewertung`, `Triage-Notiz`, sofern vorhanden.
- Status-Lifecycle: `open` -> `confirmed` oder `context-needed`; `likely-false-positive`, `accepted` und `fixed` sind dokumentierte End-/Seitenausgaenge. Neue Findings starten als `open`.
- Remediation-relevant: `open`, `confirmed`, `context-needed`; `likely-false-positive` nur nach expliziter Auswahl; `accepted` und `fixed` nie automatisch.

Falls die Datei eine Prioritätsspalte oder ein `Prioritaet`-Feld enthält: nach Priorität absteigend sortieren (höchste zuerst). Danach die Buendel-Planung aus Schritt 4 anwenden; die Bearbeitungsreihenfolge ist die bestaetigte Buendel-Reihenfolge, nicht die rein sequentielle Finding-Reihenfolge.

Übersicht ausgeben:
```
Befunde geladen: <N> offen
Buendel: <M> geplant
Autonom: <liste der freigegebenen Kategorien>
```

---

## Schritt 6 — Buendel oder Befunde abarbeiten

Wenn `grouping: true`, arbeite die bestaetigten Buendel ab. Innerhalb eines Buendels duerfen mehrere Finding-IDs in einer Task zusammengefasst werden, wenn sie denselben Fix-/Verifikationspfad haben. Wenn `grouping: false`, arbeite einzelne Findings wie im Legacy-Flow ab.

Bei `mode: task-branch-pr`:

1. Keine Produktcode-Dateien aendern.
2. Pro bestaetigtem Buendel eine Task-Datei erzeugen.
3. Task muss enthalten:
    - alle Finding-IDs im Buendel.
    - Result-Pfad und, falls vorhanden, `findings.md`.
    - Ziel-Root (`target:`), z. B. `./app`.
    - vorgeschlagener Branch: `<branch_prefix><NNN>-<slug>`.
    - Hinweis: PR erforderlich.
    - Abschnitt `## Ausführungskontext` unmittelbar nach `## Intent` mit Target-Repo, Base-Branch, Work-Branch, PR-Pflicht und Dirty-Worktree-Policy.
    - Abschnitt `## Branch-Preflight` vor `## Zu bauen` mit klarer Pflicht: zuerst im Target-Repo den Dirty-State pruefen, dann vom Base-Branch den Work-Branch erstellen oder auf bestehenden Work-Branch wechseln, und erst danach Dateien aendern.
    - Verifikationsplan fuer das Buendel.
4. Bei Result-Familien in `findings.md` bei allen Findings des Buendels `- Remediation: Task <NNN> - <tasks/...md>` ergaenzen oder aktualisieren. Status bleibt `open` oder `confirmed`, bis der Fix wirklich umgesetzt ist.
5. Bei Summary-Dateien die Summary selbst um Task-/Handoff-Status fuer das Buendel ergaenzen oder aktualisieren.
6. `assessment.md` oder die Summary bekommt/aktualisiert `## Remediation-Status` mit erzeugten Tasks und Buendeln.

Bei `mode: task-first`: analog, aber direkte Fixes koennen nach expliziter Einzelfreigabe erlaubt sein.

Bei `mode: direct-allowed`: legacy Flow fuer `S` bleibt erlaubt.

Für jede bestaetigte Einheit der Reihe nach — also je nach Policy ein Buendel oder ein einzelner Befund:

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

**Kategorie S (Sofort) — nur wenn direkte Fixes erlaubt sind:**

Dieser Zweig ist nur erlaubt, wenn `REMEDIATION_DIRECT_FIXES=true` und der Modus nicht `task-branch-pr` ist. Im Modus `task-branch-pr` wird auch ein kleiner Sofort-Fix als Task/Buendel mit Branch-/PR-Hinweis geplant.

**Kategorie S — in `AUTO_CATEGORIES` und direkte Fixes erlaubt:**
1. Fix direkt anwenden
2. Build/Tests prüfen
3. Status in Ergebnisdatei auf `✓ behoben` setzen
4. Im Änderungslog (Schritt 7) eintragen

**Kategorie S — NICHT in `AUTO_CATEGORIES`, aber direkte Fixes waeren erlaubt:**

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

1. Ziel-Verzeichnis: `TASKS_DIR` (aus Schritt 1). Wenn nicht gesetzt: abbrechen und `/k-gui` nennen.
2. Nummer: nächste freie über `<TASKS_DIR>/*.md` und `<TASKS_DIR>/done/*.md` bestimmen, zero-padded auf 3 Stellen (siehe `k-task-create.md`, Step 2).
3. Dateiname: `<NNN>-<kurzname>.md` — Kurzname aus Befundtitel abgeleitet (lowercase, hyphens; siehe `k-task-create.md`, Step 3).
4. Inhalt: Struktur aus `k-task-create.md`, Step 6 (Intent, Referenzen, Tools, Ziel, Kontext, Zu bauen). Kontext = Befundtext + Verweis auf die Ergebnisdatei. Ziel = die saubere Lösung (kein Quick-and-Dirty).
    - Bei Result-Familien muss der Task enthalten: Quelle `k-playbook-local/results/<family>/<date>/assessment.md`, Finding-ID(s) aus `findings.md`, Arbeitsregister `findings.md`, Raw-Quelle falls vorhanden und die urspruengliche `Ort`-/`Message`-Angabe.
    - Bei Buendeln muss der Task enthalten: Buendelname, alle Finding-IDs, gemeinsame Ursache/Fix-Route, Ziel-Root, vorgeschlagener Branch und PR-Pflicht aus der Remediation-Policy.
    - Bei `mode: task-branch-pr` muss der Task zusaetzlich diese Struktur enthalten:

```markdown
## Ausführungskontext

- Target repo: `<REMEDIATION_TARGET_DISPLAY>`
- Base branch: `<REMEDIATION_BASE_BRANCH or "<manual>">`
- Work branch: `<REMEDIATION_BRANCH_PREFIX><NNN>-<slug>`
- PR required: `<REMEDIATION_PR_REQUIRED>`
- Dirty worktree policy: abort before changing files unless the dirty files are explicitly expected for this task and the user confirms continuation.

## Branch-Preflight

- Before changing files, run the Git preflight in `Target repo`.
- Verify the target repo is clean or stop and ask the user how to proceed.
- If `Work branch` does not exist, create it from `Base branch`.
- If `Work branch` exists, switch to it and verify it is based on the intended `Base branch`, or ask before continuing.
- Only after this preflight, update code, dependencies, lockfiles, generated files, or review status files.
```

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

### Legacy-Dateien

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

### Result-Familien

Bei `RESULT_FORMAT=result-family` wird primaer `findings.md` aktualisiert:

- Kategorie S nach erfolgreichem Fix und Verifikation: `- Status: `fixed``.
- Kategorie T mit Task-Datei: Status bleibt `open` oder `confirmed`, bis der Fix wirklich umgesetzt und verifiziert ist; ergaenze aber `- Remediation: Task <NNN> - <tasks/...md>` oder aktualisiere ein vorhandenes Remediation-Feld.
- Kategorie K: `- Status: `context-needed`` mit klarer `Triage-Notiz`.
- Kategorie A: `- Status: `accepted`` mit Akzeptierungsgrund und optional Known-Decision-Verweis.
- Kategorie X: `- Status: `likely-false-positive`` mit Begruendung.
- Wenn ein Befund durch Code-Lektuere bestaetigt wurde, aber noch nicht behoben ist: `- Status: `confirmed``.

Am Ende von `findings.md` einen nachvollziehbaren Abschnitt pflegen:

```markdown
---

## Remediation-Log

| Datum | Finding-ID | Kategorie | Aktion | Notiz |
|---|---|---|---|---|
| YYYY-MM-DD | kcheck-logging-003 | Task | Task 018 | tasks/018-redact-upstream-log.md |
```

`assessment.md` darf optional einen Abschnitt `## Remediation-Status` bekommen oder aktualisieren:

```markdown
## Remediation-Status

- YYYY-MM-DD: `/k-remediation` gestartet; <N> remediation-relevante Findings aus `findings.md` geladen.
- YYYY-MM-DD: Task(s) <...> fuer Finding(s) <...> angelegt.
```

### Result-Summaries

Bei `RESULT_FORMAT=summary` wird die Summary-Datei selbst aktualisiert:

- Kategorie T mit Task-Datei: in der betroffenen Prioritaetsgruppe `Status` oder eine kurze `Remediation:`-Zeile auf `Task <NNN> - <tasks/...md>` setzen.
- Kategorie K/A/X: Status mit knapper Begruendung direkt in der betroffenen Gruppe dokumentieren.
- Kategorie S nach erfolgreichem Fix und Verifikation: Status auf `fixed` oder `behoben` setzen und Verifikation nennen.
- Wenn die Summary auf konkrete `findings.md`-IDs verweist, die zugehoerigen `findings.md`-Eintraege nach den Result-Family-Regeln synchron aktualisieren.

Am Ende der Summary einen nachvollziehbaren Abschnitt pflegen:

```markdown
---

## Remediation-Status

| Datum | Gruppe/Finding | Kategorie | Aktion | Notiz |
|---|---|---|---|---|
| YYYY-MM-DD | P1-01 | Task | Task 018 | tasks/018-redact-upstream-log.md |
```

`raw/` und `run-metadata.*` niemals bearbeiten.

---

## Schritt 8 — Ergebnisdatei archivieren

Archivierung gilt nur fuer Legacy-Ergebnisdateien.

Bei Summary- und Result-Family-Dateien wird nichts nach `done/` verschoben. Summary-Dateien bleiben stabil unter `k-playbook-local/results/`; Result-Verzeichnisse bleiben stabil unter `k-playbook-local/results/<family>/<date>/`. Abschluss erfolgt ueber Statuswerte in `findings.md` und optional `## Remediation-Status` in `assessment.md` oder der Summary.

Wenn alle Befunde abgearbeitet sind (keine ☐ mehr offen):

1. Ziel-Verzeichnis bestimmen:
   - Wenn `DONE_DIR` (`<RESULTS_DIR>/done/`) gesetzt ist: dort archivieren. Verzeichnis bei Bedarf anlegen.
   - Wenn nicht gesetzt (kein `RESULTS_DIR`): abbrechen und `/k-gui` nennen.

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

- **Ergebnisdatei nicht gefunden / nicht plausibel**: verfügbare `result-*.md` in `<RESULTS_DIR>` auflisten, User wählen lassen. Bei Formatabweichung: abbrechen statt raten.
- **Kein k-playbook-Projekt**: der Context-Aufruf schlaegt fehl; abbrechen und `/k-gui` empfehlen.
- **`RESULTS_DIR` oder `TASKS_DIR` fehlt im Dateisystem**: fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen.
- **YAML-konfigurierte Reviews- oder Tasks-Pfade fehlen im Dateisystem**: User fragen, ob genau diese Pfade angelegt werden sollen oder `/k-gui` die Struktur reparieren soll; keinen anderen Pfad verwenden.
