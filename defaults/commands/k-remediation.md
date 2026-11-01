# k-remediation

Arbeitet Befunde aus einer Ergebnisdatei strukturiert ab — üblicherweise die Datei, die
`/k-review` im Report-Modus erzeugt hat. Vor der Umsetzung wird immer ein Remediation-Plan
gebildet: welche Findings zusammengehören, was zuerst kommt, was Quick-Win ist und was einen
eigenen Task/Branch/PR braucht. Tasks entstehen in der Projekt-Collection, nicht auf der
Platte.

Unterstützt aktuelle und historische Formate:

- Audit-Läufe wie `k-playbook-local/results/<date>/review-triage.md` mit `review-input.json` — der Hauptweg.
- Aktuelle Result-Familien wie `k-playbook-local/results/<family>/<date>/review-triage.md` mit optionalem `review-input.json`.
- Legacy-Result-Summaries wie `k-playbook-local/results/summary-YYYY-MM-DD.md` mit priorisierten Remediation-Gruppen.
- Legacy-Result-Familien wie `k-playbook-local/results/<family>/<date>/assessment.md` mit zugehörigem `findings.md`, nur wenn im selben Ordner kein `review-triage.md` vorhanden ist.

**`summary-*.md` ist ein reiner Lesepfad.** Seit dem Wegfall des nachgelagerten
Priorisierungsschritts erzeugt nichts mehr eine Summary: weder `/k-audit` noch `/k-review`
schreiben sie, und dieser Command legt keine an. Was in einem bereits eingerichteten
Zielprojekt an Summaries liegt, bleibt aber lesbar und abarbeitbar. Alle Summary-Stellen
unten sind so zu verstehen — Legacy-Eingabe, nie Ausgabe.

Der Block `context.remediation` des Briefings legt fest, wie gearbeitet wird:

- `mode` - `task-branch-pr`, `task-first` oder `direct-allowed`.
- `target` - tatsächlicher Code-/Git-Root, z. B. `./app` bei Wrapper-Repos.
- `grouping` - ob Findings vor der Umsetzung zu sinnvollen Bündeln zusammengefasst werden.
- `quickWins` - ob einfache, wirkungsstarke Bündel hervorgehoben werden.
- `branchPrefix` - empfohlener Branch-Prefix für Remediation-Branches.
- `prRequired` - ob ein PR Teil des erwarteten Workflows ist.
- `directFixes` - ob direkte Code-Fixes ohne Task erlaubt sind.

Wenn `configured` false ist, stoppe und bitte darum, die Remediation-Policy per `/k-gui` zu
setzen, oder frage für die aktuelle Session explizit.

Produces:

- Tasks in `keph:/user/~/tasks/`, einer je bestätigtem Bündel oder Befund der Kategorie T,
  angelegt mit `k_playbook_doc_create` und geschrieben mit Kephalaion `write`.
- `## Remediation-Status` bzw. Statusfelder und Remediation-Log in der Arbeitsdatei
  (`review-triage.md`, Legacy-`findings.md`, `assessment.md` oder Summary); Task-Verweise
  dort sind `keph:`-Verweise.
- Eine Zeile im Protokoll von `LOG_FILE` je Lauf.
- Nur nach Freigabe: direkte Fixes im Code (Kategorie S) und Einträge in
  `known-decisions.md` (Kategorie A).

---

## Schritt 1 — Briefing, Form und Pfade

include: _shared/erster-schritt.md

Verbindlich für Ablage, Name und Nummer, Kopf und Aufbau eines Tasks ist
`k-playbook:/instructions/on-demand/tasks.md`. Lies sie mit `k_playbook_doc_read`, bevor du
einen Task anlegst oder gegen bestehende Tasks abgleichst; dieser Text wiederholt sie nicht.

Aus dem Briefing:

- `RESULTS_DIR` = `<context.local.dir>/results`
- `KNOWN_DECISIONS` = `<context.local.dir>/known-decisions.md`
- `LOG_FILE` = `<RESULTS_DIR>/log.md`
- `REMEDIATION_MODE`, `REMEDIATION_TARGET_DIR`, `REMEDIATION_TARGET_DISPLAY`,
  `REMEDIATION_GROUPING`, `REMEDIATION_QUICK_WINS`, `REMEDIATION_BRANCH_PREFIX`,
  `REMEDIATION_PR_REQUIRED`, `REMEDIATION_DIRECT_FIXES` aus `context.remediation`.
- `REMEDIATION_BASE_BRANCH` = aktueller Branch im Target-Repo, falls `target` ein Git-Repo
  ist; sonst unset. Bestimme ihn mit `git branch --show-current` im Target-Root. Wenn leer:
  nutze keinen geratenen Wert, sondern schreibe `Base branch: <manual>` in erzeugte Tasks.

Command-specific policy:

- Tasks brauchen die Collection. Steht `context.kephalaion.ok` nicht auf `true`, endet der
  Command hier mit `context.kephalaion.message` (Abschnitt „Ohne Kephalaion“ in
  `on-demand/tasks.md`): kein Task auf der Platte, kein Abgleich, kein Ersatzverzeichnis.
- Wenn `RESULTS_DIR` nicht existiert: fragen, ob genau dieses Verzeichnis angelegt werden
  soll, oder `/k-gui` nennen. Keinen anderen Pfad verwenden.
- Wenn `mode: task-branch-pr` oder `mode: task-first` gesetzt ist, müssen
  Remediation-Schritte als Tasks/Bündel geplant werden. Direkte Code-Änderungen sind nur
  erlaubt, wenn `directFixes: true` und der User den konkreten Fix nach Code-Sichtung
  bestätigt.
- Wenn `target` gesetzt ist, muss der Pfad existieren. Code-Verifikation und
  Branch-/Git-Hinweise beziehen sich auf diesen Target-Root, nicht zwingend auf
  `context.project.repoRoot`.
- Wenn `mode: task-branch-pr` gilt und `target` ein Git-Repo ist, prüfe vor der
  Task-Erzeugung den aktuellen Branch und Dirty-State des Target-Repos. Bei Dirty-State keine
  Branch-/Task-Policy raten: User informieren und bestätigen lassen, ob Tasks trotzdem erzeugt
  werden sollen. `/k-remediation` wechselt selbst keinen Branch für spätere Umsetzung; es
  schreibt den erforderlichen Ausführungskontext in die Tasks.

---

## Schritt 2 — Ergebnisdatei bestimmen

Wenn ein Argument angegeben ist: diese Datei einlesen.

Akzeptierte direkte Argumente:

- `<RESULTS_DIR>/<date>/review-triage.md`
- `<RESULTS_DIR>/<family>/<date>/review-triage.md`
- Legacy: `<RESULTS_DIR>/summary-*.md`, soweit im Projekt noch eine liegt
- Legacy: `<RESULTS_DIR>/<family>/<date>/assessment.md`, wenn dort kein `review-triage.md` liegt
- Projektrelative Varianten davon, z. B. `k-playbook-local/results/2026-07-23/review-triage.md` oder `k-playbook-local/results/k-check/2026-07-23/review-triage.md`

Wenn nicht:

1. In `<RESULTS_DIR>` nach `<RESULTS_DIR>/*/review-triage.md` und `<RESULTS_DIR>/*/*/review-triage.md` suchen.
2. Legacy-Fallback: zusätzlich `<RESULTS_DIR>/summary-*.md` anbieten, soweit vorhanden, und Family-Ordner ohne `review-triage.md` über `<RESULTS_DIR>/*/*/assessment.md`. Beides als Legacy kennzeichnen, damit erkennbar bleibt, dass hier nichts Neues mehr entsteht.
3. Wenn genau eine: sie vorschlagen und Bestätigung abwarten.
4. Wenn mehrere: als Liste zeigen und den User wählen lassen.
5. Wenn keine: fragen:
    > "Welche Ergebnisdatei soll abgearbeitet werden?"

**Aktuelle Audit-Erkennung:** Wenn die Datei `review-triage.md` heißt und der Pfad auf `<RESULTS_DIR>/<date>/review-triage.md` endet:

- Setze `RESULT_FORMAT=audit-run`.
- Setze `RESULT_DATE=<date>`.
- Lies `review-triage.md` als primäre Arbeitsdatei. Bündel aus `## Bündel` und `## Bündel-Details` sind die Remediation-Einheiten.
- Lade `review-input.json`, wenn vorhanden, für Evidence-Details und stabile Gruppen-IDs.
- `review-triage.md` darf nur nachvollziehbar um `## Remediation-Status` oder Statushinweise ergänzt werden; die ursprünglichen Belege bleiben unverändert.
- `raw/`, `entries/` und `run.json` im selben Verzeichnis sind auditierbar und dürfen nicht verändert werden.

**Aktuelle Result-Family-Erkennung:** Wenn die Datei `review-triage.md` heißt und der Pfad auf `<RESULTS_DIR>/<family>/<date>/review-triage.md` endet:

- Setze `RESULT_FORMAT=result-family`.
- Setze `RESULT_FAMILY=<family>` und `RESULT_DATE=<date>`.
- Lies `review-triage.md` als primäre Arbeitsdatei. Bündel aus `## Bündel` und `## Bündel-Details` sind die Remediation-Einheiten.
- Lade `review-input.json`, wenn vorhanden, für Evidence-Details und stabile Gruppen-IDs.
- `review-triage.md` darf nur nachvollziehbar um `## Remediation-Status` oder Statushinweise ergänzt werden; die ursprünglichen Belege bleiben unverändert.
- `raw/` und `run-metadata.*` im selben Verzeichnis sind auditierbar und dürfen nicht verändert werden.

**Legacy-Result-Family-Erkennung:** Wenn die Datei `assessment.md` heißt und der Pfad auf `<RESULTS_DIR>/<family>/<date>/assessment.md` endet:

- Prüfe zuerst, ob im selben Ordner `review-triage.md` existiert. Wenn ja, nutze diese Datei und ignoriere `assessment.md` als Primärinput.
- Andernfalls setze `RESULT_FORMAT=legacy-result-family`.
- Erwarte `findings.md` im selben Verzeichnis.
- Lies `findings.md` als primäres Arbeitsregister.
- `assessment.md` bleibt Quelle/Kurzbewertung und darf nur nachvollziehbar für seine Kurzfassung, den Handoff-Status oder explizite Remediation-Abschnitte aktualisiert werden.
- `raw/` und `run-metadata.*` im selben Verzeichnis sind auditierbar und dürfen nicht verändert werden.

**Legacy-Summary-Erkennung:** Wenn die Datei `summary-*.md` heißt und direkt unter `<RESULTS_DIR>/` liegt:

- Setze `RESULT_FORMAT=summary`.
- Sag einmal dazu, dass das ein Legacy-Format ist: Summaries werden nicht mehr erzeugt, diese hier stammt aus der Zeit, als es noch einen nachgelagerten Priorisierungsschritt gab. Sie wird abgearbeitet, aber nicht fortgeschrieben und nicht ersetzt.
- Die Summary-Datei ist die Quelle für priorisierte Remediation-Gruppen und die Arbeitsdatei für Handoff-/Task-Status.
- Verlinkte `review-triage.md`-Quellen aus der Summary dürfen mitgeladen werden. Legacy-`assessment.md`-/`findings.md`-Quellen dürfen nur geladen werden, wenn kein `review-triage.md` vorhanden ist.
- Liegt zu demselben Stand ein neueres `review-triage.md` vor, dieses vorschlagen: es ist der aktuelle Weg, die Summary nur noch der Rest eines alten.

**Sonst:** kein unterstütztes Format — sauber abbrechen mit Hinweis auf `review-triage.md` und die Legacy-Eingaben `summary-*.md` und `assessment.md`, statt zu raten.

**Format-Check aktuelle Audit- und Result-Family-Triage:** `review-triage.md` muss die Pflichtabschnitte `## Bündel`, `## Bündel-Details`, `## Nicht gebündelt` und `## Deckung aus known-decisions` enthalten. Wenn sie fehlen: sauber abbrechen, nicht aus Rohdaten improvisieren.

**Format-Check Legacy-Result-Family:** `findings.md` muss Markdown-Headings für einzelne Findings enthalten und darunter mindestens ein Statusfeld in der Form `- Status: `<wert>``. Wenn `findings.md` fehlt oder kein Statusfeld erkennbar ist: sauber abbrechen. Nicht aus `assessment.md` neue Finding-IDs erraten.

Offene Punkte im aktuellen Audit- oder Result-Family-Format sind Bündel aus `review-triage.md`, die nicht in `## Deckung aus known-decisions` vollständig gedeckt sind und keinen erledigten Remediation-Status tragen. Im Legacy-Format sind offene Punkte Findings mit Status `open`, `confirmed` oder `context-needed`. `likely-false-positive` ist review-relevant, aber nur nach expliziter User-Auswahl remediation-relevant. `accepted` und `fixed` sind Endzustände und dürfen nicht automatisch in neue Fix-Tasks überführt werden.

---

## Schritt 3 — known-decisions.md laden

Wenn `KNOWN_DECISIONS` existiert:

- Einlesen und intern als `KNOWN_DECISIONS`-Inhalt bereithalten.
- Kurz bestätigen: „`known-decisions.md` geladen — <N> Einträge."

Wenn die Datei nicht existiert:

- Warnen: „Keine `known-decisions.md` unter `<Pfad>`. Bewusste Entscheidungen können deshalb erneut als Befund auftauchen. Die Datei wird von Hand angelegt; ihr Format steht in `docs/review-runs.md`."
- Weiter — kein automatisches Anlegen an dieser Stelle.

---

## Schritt 4 — Arbeitsmodus, Bündelung und Autonomie klären

Zeige zuerst die geladene Remediation-Policy:

```text
Remediation-Policy
──────────────────
Modus:        task-branch-pr | task-first | direct-allowed
Target:       <REMEDIATION_TARGET_DISPLAY>
Base branch:  <REMEDIATION_BASE_BRANCH or "<manual>">
Grouping:     true | false
Quick-Wins:   true | false
Branch:       <branchPrefix><task-or-bundle>
PR required:  true | false
Direct fixes: true | false
```

Modus-Semantik:

- `task-branch-pr`: Keine direkten Code-Fixes aus `/k-remediation`. Findings werden zu Remediation-Bündeln gruppiert; jedes akzeptierte Bündel erzeugt einen Task mit Branch-/PR-Hinweis. Umsetzung erfolgt später über `/k-task-run` oder einen dedizierten Dev-Flow auf Branch + PR.
- `task-first`: Standard ist Task-Erzeugung pro Bündel; direkte Fixes nur nach expliziter User-Freigabe für einzelne kleine Bündel.
- `direct-allowed`: Kleine sichere `S`-Findings dürfen nach Code-Sichtung direkt behoben werden, wenn der User die Kategorien freigibt.

Wenn `mode` fehlt oder unbekannt ist: stoppe und bitte um `/k-gui` oder explizite Auswahl für diese Session.

### Bündelung vor Einzelarbeit

Vor dem Abarbeiten einzelner Befunde muss `/k-remediation` eine Planungsübersicht erzeugen:

1. Alle remediation-relevanten Findings laden.
2. Findings nach Kopplung gruppieren:
   - gleicher Package-/Dependency-Upgrade-Pfad, z. B. `Django >= 5.2.15`.
   - gleicher Codebereich oder gleiche Datei/Komponente.
   - gleiche Root Cause / gleiche Konfiguration.
   - gleiche Verifikationsroute, z. B. ein Test-/Build-Lauf deckt mehrere Findings ab.
3. Pro Bündel einschätzen:
   - Risiko: `P1/P2/P3`, critical/high/medium/low, Runtime vs. Dev/Build.
   - Aufwand: `S` klein, `M` mittel, `L` groß/unsicher.
   - Kopplung: welche Findings sollten gemeinsam gelöst werden.
   - Quick-Win: hohe Wirkung bei kleinem Aufwand und klarer Verifikation.
4. Reihenfolge vorschlagen:
   - Zuerst P1 + kleiner/mittlerer Aufwand.
   - Dann P1 groß oder P2 mit klarer Verifikation.
   - Dann Dev-/Build-only und P3.
5. Dem User die Bündel-Liste zeigen und bestätigen lassen, bevor Tasks erzeugt oder Fixes gemacht werden.

Beispiel-Ausgabe:

```text
Remediation-Plan
────────────────
1. P1/S Quick-Win: python-jose >= 3.4.0 (depbot-7, depbot-8)
2. P1/M: Django Patch-Level (24 Findings)
3. P1/M: Pillow Patch-Level (17 Findings)
4. P2/S: azure-core >= 1.38.0

Vorschlag: Für die ersten 3 Bündel Tasks erzeugen.
```

Im Modus `task-branch-pr` sind diese Bündel die Einheit für Task/Branch/PR, nicht zwingend einzelne Findings.

### Kategorien

Kategorien gelten nach der Bündelplanung für einzelne Findings oder ganze Bündel:

| Kürzel | Name | Bedeutung |
|--------|------|-----------|
| **S** | Sofort | Klarer Fehler, kleiner gezielter Fix (< 30 min, kein Architektur-Impact, kein Verhaltens-Einfluss für Enduser). Beispiele: falsche Konstante, stilles Ignorieren von Fehlern, veraltete Konfigurationswerte. |
| **T** | Task | Klarer Fehler, aber größerer Eingriff — eigenständige Umsetzung braucht einen Task. |
| **K** | Klärung | Unklar ob wirklich ein Problem, oder Architekturentscheidung nötig. Erst besprechen. |
| **F** | Feature | Neue Funktionalität, kein Bugfix. Immer erst nachfragen — nie autonom umsetzen. |
| **A** | Akzeptiert | Bekanntes Design, bewusste Entscheidung oder externe Behandlung. Mit Grund dokumentieren und schließen. |
| **X** | Falsch | Befund ist nicht korrekt oder nicht relevant. Dokumentieren und schließen. |

Frage den User:
> "Welche Kategorien darf ich autonom abarbeiten?
> - **S (Sofort)** — direkt fixen
> - **T (Task)** — Task anlegen
> - **A/X** — dokumentieren und schließen
>
> Oder soll ich jeden Punkt erst vorstellen und du entscheidest?"

Wenn `mode: task-branch-pr` gilt, ersetze die Frage durch:

> "Dieses Projekt nutzt `task-branch-pr`. Ich werde keine direkten Fixes machen. Soll ich für die bestätigten Bündel Tasks mit Branch-/PR-Hinweis anlegen?"

Warte auf Antwort. Merke welche Kategorien autonom behandelt werden dürfen (`AUTO_CATEGORIES`).

---

## Schritt 5 — Befunde einlesen, gruppieren und sortieren

Alle offenen Befunde aus der Arbeitsdatei sammeln. Bei einer Legacy-Summary ist die Arbeitsdatei die Summary-Datei mit ihren priorisierten Gruppen. Bei aktuellen Result-Familien ist die Arbeitsdatei `review-triage.md`; `review-input.json` wird, falls vorhanden, als Evidence-Quelle mitgeladen. Bei Legacy-Result-Familien bleibt `findings.md` die Arbeitsdatei; `assessment.md` wird als Quelle mitgeladen.

Aktuelles Result-Family-Parsing:

- Bündel-ID ist die Markdown-Heading-ID aus `## Bündel-Details`, z. B. `### B1 — Dependency-Upgrade`.
- Erfasse Priorität, Kategorie, Kurzbegründung, betroffene Gruppen, Belege und nächsten Schritt aus `review-triage.md`.
- Wenn `review-input.json` vorhanden ist, nutze die dortigen Evidence-Einträge zur Code-Sichtung; erfinde keine Evidence aus der Triage-Prosa.
- Vollständig durch `## Deckung aus known-decisions` gedeckte Gruppen sind nicht automatisch remediation-relevant; teilgedeckte Gruppen bleiben sichtbar.

Legacy-Result-Family-Parsing:

- Finding-ID ist die Markdown-Heading-ID, z. B. `### kcheck-logging-003` oder `### py/full-ssrf-001`.
- Die ID muss aus `findings.md` stammen; Task-Erzeugung darf keine neue ID für ein bestehendes Finding erzeugen.
- Erfasse `Status`, `Priorität`, `Quelle`, `Ort`, `Message`, `Raw-Quelle`, `Review-Bewertung`, `Triage-Notiz`, sofern vorhanden.
- Status-Lifecycle: `open` -> `confirmed` oder `context-needed`; `likely-false-positive`, `accepted` und `fixed` sind dokumentierte End-/Seitenausgänge. Neue Findings starten als `open`.
- Remediation-relevant: `open`, `confirmed`, `context-needed`; `likely-false-positive` nur nach expliziter Auswahl; `accepted` und `fixed` nie automatisch.

Falls die Datei eine Prioritätsspalte oder ein `Priorität`-Feld enthält: nach Priorität absteigend sortieren (höchste zuerst). Danach die Bündel-Planung aus Schritt 4 anwenden; die Bearbeitungsreihenfolge ist die bestätigte Bündel-Reihenfolge, nicht die rein sequentielle Finding-Reihenfolge.

### Abgleich gegen bestehende Tasks

Dieser Abgleich sitzt **vor** der Task-Erzeugung: Ob ein Task überhaupt entsteht,
entscheidet sich hier. Er läuft über alle Tasks der Projekt-Collection, offene und erledigte,
aller User — ein Befund ist auch dann in Arbeit, wenn ein anderer den Task angelegt hat.

1. Mit `k_playbook_doc_grep` über die Sorte: `kind: tasks`, `user: "*"`, `done: true`,
   `pattern` = die **Quelle** der Arbeitsdatei als Regex (Punkte maskiert, etwa
   `results/k-check/2026-07-23/review-triage\.md`). Die Antwort nennt je Treffer `ref` und
   die Zeile; mehr als 200 Treffer meldet `truncated: true` — dann die Suche mit der
   **Bündel-/Gruppen-ID** verfeinern.
2. Für jeden Verweis aus Schritt 1 mit `k_playbook_doc_grep` auf `ref` prüfen, ob die
   Bündel-/Gruppen-ID des geplanten Bündels darin steht (als Wort, etwa `\bB1\b` oder
   `\bP1-01\b`). Beides schreibt Schritt 6c in jeden erzeugten Task, Quelle und ID, damit
   dieser Abgleich ihn beim nächsten Lauf wiedererkennt.
3. Je geplantem Bündel entscheiden, ob ein bestehender Task es abdeckt.

**Treffer-Kriterium.** Ein bestehender Task deckt einen Befund ab, wenn **Quelle und
Bündel-/Gruppen-ID übereinstimmen**. Beides muss zutreffen. Titelähnlichkeit allein
reicht nicht: derselbe Titel über zwei Läufen hinweg sagt nichts darüber, ob es derselbe
Befund ist, und ein umformulierter Titel verdeckt sonst eine echte Deckung.

**Treffer in einem offenen Task** (Verweis ohne `/done/`) — der Befund ist bereits in Arbeit:

- Keinen zweiten Task anlegen. Den Treffer melden: Bündel-ID, `keph:`-Verweis des Tasks,
  Quelle.
- Das Bündel im Arbeitsregister nach den Regeln aus Schritt 7 auf den bestehenden Task
  verweisen lassen, statt einen neuen Verweis zu erfinden.

**Treffer unter `done/`** (Verweis mit `/tasks/done/`) — der Befund bleibt offen:

- Melden, mit `keph:`-Verweis und Quelle daneben.
- Aber **nicht** als erledigt werten. Ein abgeschlossener Task zu einem früheren
  Vorkommen darf einen wiederkehrenden Befund nicht stillschweigend unterdrücken: Dass
  der Befund erneut in einem aktuellen Lauf steht, heißt, dass er wieder da ist.
- Der Befund geht regulär in die Bündel-Planung und kann einen neuen Task bekommen. Der
  Hinweis auf den erledigten Task steht daneben — er gehört als Kontext in den neuen
  Task.

**Wenn der Abgleich nicht laufen kann** — `k_playbook_doc_grep` scheitert mit einem anderen
Fehler als „kein Treffer“: warnen und weiterarbeiten, aber ausdrücklich sagen, dass kein
Abgleich stattgefunden hat. Ein Abgleich, der nicht lief, darf nichts unterdrücken.

Übersicht ausgeben:
```
Befunde geladen: <N> offen
Bündel: <M> geplant
Bereits in Task:  <k>  (<Bündel-ID> -> <keph:-Verweis>)
Früher erledigt:  <k>  (<Bündel-ID> -> <keph:-Verweis>, Befund bleibt offen)
Autonom: <liste der freigegebenen Kategorien>
```

---

## Schritt 6 — Bündel oder Befunde abarbeiten

Wenn `grouping: true`, arbeite die bestätigten Bündel ab. Innerhalb eines Bündels dürfen mehrere Finding-IDs in einem Task zusammengefasst werden, wenn sie denselben Fix-/Verifikationspfad haben. Wenn `grouping: false`, arbeite die einzelnen Findings der Reihe nach ab.

Bei `mode: task-branch-pr`:

1. Keine Produktcode-Dateien ändern.
2. Pro bestätigtem Bündel einen Task anlegen.
3. Der Task muss enthalten:
    - alle Finding-IDs im Bündel.
    - Result-Pfad und, falls vorhanden, `review-input.json` oder Legacy-`findings.md`.
    - Ziel-Root (`target`), z. B. `./app`.
    - vorgeschlagener Branch: `<branchPrefix><NNN>-<slug>`, `<NNN>` die Nummer aus der Antwort von `k_playbook_doc_create`.
    - Hinweis: PR erforderlich.
    - Abschnitt `## Ausführungskontext` unmittelbar nach `## Intent` mit Target-Repo, Base-Branch, Work-Branch, PR-Pflicht und Dirty-Worktree-Policy.
    - Abschnitt `## Branch-Preflight` vor `## Zu bauen` mit klarer Pflicht: zuerst im Target-Repo den Dirty-State prüfen, dann vom Base-Branch den Work-Branch erstellen oder auf bestehenden Work-Branch wechseln, und erst danach Dateien ändern.
    - Verifikationsplan für das Bündel.
4. Bei aktuellen Result-Familien `review-triage.md` um `## Remediation-Status` mit dem `keph:`-Verweis des Tasks ergänzen oder aktualisieren. Bei Legacy-Result-Familien in `findings.md` bei allen Findings des Bündels `- Remediation: Task <NNN> - <keph:-Verweis>` ergänzen oder aktualisieren. Status bleibt offen, bis der Fix wirklich umgesetzt ist.
5. Bei einer Legacy-Summary die Summary selbst um Task-/Handoff-Status für das Bündel ergänzen oder aktualisieren. Ergänzt wird nur der Status der Abarbeitung — die Summary wird nicht neu berechnet und keine zweite angelegt.
6. `review-triage.md`, Legacy-`assessment.md` oder die Legacy-Summary bekommt/aktualisiert `## Remediation-Status` mit erzeugten Tasks und Bündeln.

Bei `mode: task-first`: analog, aber direkte Fixes können nach expliziter Einzelfreigabe erlaubt sein.

Bei `mode: direct-allowed`: direkte Fixes für `S` bleiben erlaubt.

Für jede bestätigte Einheit der Reihe nach — also je nach Policy ein Bündel oder ein einzelner Befund:

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

Dieser Zweig ist nur erlaubt, wenn `REMEDIATION_DIRECT_FIXES=true` und der Modus nicht `task-branch-pr` ist. Im Modus `task-branch-pr` wird auch ein kleiner Sofort-Fix als Task/Bündel mit Branch-/PR-Hinweis geplant.

**Kategorie S — in `AUTO_CATEGORIES` und direkte Fixes erlaubt:**
1. Fix direkt anwenden
2. Build/Tests prüfen
3. Status in der Arbeitsdatei nach den Regeln aus Schritt 7 setzen
4. Im Remediation-Log (Schritt 7) eintragen

**Kategorie S — NICHT in `AUTO_CATEGORIES`, aber direkte Fixes wären erlaubt:**

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

Den Task nach `k-playbook:/instructions/on-demand/tasks.md` anlegen — Ablage, Name und
Nummer, Kopf und Aufbau stehen dort und werden hier nicht wiederholt:

1. Abgleich prüfen: Deckt ein offener Task dieses Bündel bereits ab (Schritt 5,
   Treffer-Kriterium Quelle plus Bündel-/Gruppen-ID)? Dann keinen zweiten Task anlegen,
   sondern den Treffer melden und beim nächsten Bündel weitermachen. Ein Treffer unter
   `done/` hält den Task nicht auf; er wird im neuen Task als Vorgeschichte genannt.
2. `k_playbook_doc_create` mit `kind: tasks` und einem Kurznamen aus dem Befundtitel
   (klein, Bindestriche). Die Antwort nennt `ref` und `number`; die Nummer vergibt
   k-playbook.
3. Inhalt nach dem Abschnitt „Aufbau“ von `on-demand/tasks.md` (Frontmatter mit `title`,
   Intent, Referenzen, Tools, Ziel, Kontext, Zu bauen) mit Kephalaion `write` schreiben,
   `collection` = `context.kephalaion.address`, `name` = Pfad des Verweises ohne führenden
   `/`. Kontext = Befundtext + Verweis auf die Ergebnisdatei. Ziel = die saubere Lösung (kein
   Quick-and-Dirty).

    **Pflichtanker in jedem Task: Quelle plus Bündel-/Gruppen-ID.** Beide Angaben sind
    nicht bloß Herkunftsnachweis, sondern das Kriterium, an dem der Abgleich aus
    Schritt 5 diesen Task beim nächsten Lauf wiedererkennt. Fehlt eine davon, entsteht
    beim nächsten Lauf ein zweiter Task zum selben Befund. Sie gilt in **jedem**
    `RESULT_FORMAT`:

    - Bei `RESULT_FORMAT=audit-run` muss der Task enthalten: Quelle `k-playbook-local/results/<date>/review-triage.md`, Bündel-/Gruppen-ID(s) aus `review-triage.md`, Evidence aus `review-input.json` falls vorhanden, Raw-Quelle falls vorhanden und die ursprüngliche Ort-/Message-Angabe.
    - Bei `RESULT_FORMAT=result-family` muss der Task enthalten: Quelle `k-playbook-local/results/<family>/<date>/review-triage.md`, Bündel-/Gruppen-ID(s) aus `review-triage.md`, Evidence aus `review-input.json` falls vorhanden, Raw-Quelle falls vorhanden und die ursprüngliche Ort-/Message-Angabe.
    - Bei `RESULT_FORMAT=summary` (Legacy-Lesepfad, siehe Schritt 2) muss der Task enthalten: Quelle `k-playbook-local/results/summary-<date>.md`, die Gruppen-ID aus der Summary (z. B. `P1-01`), dazu die von der Summary verlinkten Quellen und deren Bündel-/Finding-IDs, soweit sie dort stehen.
    - Bei `RESULT_FORMAT=legacy-result-family` muss der Task enthalten: Quelle `k-playbook-local/results/<family>/<date>/assessment.md`, Finding-ID(s) aus `findings.md`, Arbeitsregister `findings.md`, Raw-Quelle falls vorhanden und die ursprüngliche `Ort`-/`Message`-Angabe.
    - Bei Bündeln muss der Task enthalten: Bündelname, alle Finding-IDs, gemeinsame Ursache/Fix-Route, Ziel-Root, vorgeschlagener Branch und PR-Pflicht aus der Remediation-Policy.
    - Bei einem Treffer unter `done/` muss der Task den erledigten Vorgänger nennen: sein `keph:`-Verweis und der Umstand, dass derselbe Befund erneut aufgetreten ist.
    - Bei `mode: task-branch-pr` muss der Task zusätzlich diese Struktur enthalten:

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

4. Status in der Arbeitsdatei nach den Regeln aus Schritt 7 setzen; der Task steht dort mit
   seinem `keph:`-Verweis.
5. Im Remediation-Log eintragen.

Fehlerfälle beim Anlegen: `outcome_unknown` → nicht noch einmal anlegen, sondern mit
`k_playbook_doc_list` (Sorte `tasks`) nachsehen und dann `write`; schlägt `write` fehl, bleibt
das leere Dokument liegen — sag das, nenne den Verweis und wiederhole `write`, bevor du etwas
anderes tust.

**Kategorie T — NICHT in `AUTO_CATEGORIES`:**
Befund vorstellen und fragen ob ein Task angelegt werden soll.

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
Status nach den Regeln aus Schritt 7 auf akzeptiert setzen. Kurzen Grund in das Remediation-Log schreiben.

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
Status nach den Regeln aus Schritt 7 auf falsch-positiv setzen. Kurze Begründung notieren.

---

## Schritt 7 — Ergebnisdatei aktualisieren

Nach jedem bearbeiteten Befund. Ein Task steht in jedem Statuseintrag mit seinem
`keph:`-Verweis, nie mit einem Plattenpfad.

### Result-Familien

Bei `RESULT_FORMAT=result-family` wird primär `review-triage.md` aktualisiert:

- Kategorie S nach erfolgreichem Fix und Verifikation: im betroffenen Bündel oder `## Remediation-Status` als `fixed` dokumentieren.
- Kategorie T mit Task: `## Remediation-Status` um `keph:`-Verweis, Bündel-ID und betroffene Gruppen ergänzen. Das Bündel bleibt offen, bis der Fix wirklich umgesetzt und verifiziert ist.
- Kategorie K: Kontextbedarf mit klarer `Triage-Notiz` im Bündel oder Statusabschnitt dokumentieren.
- Kategorie A: Akzeptierungsgrund und optional Known-Decision-Verweis dokumentieren.
- Kategorie X: Ausschluss oder falsch-positive Einordnung mit Begründung dokumentieren.

Am Ende von `review-triage.md` einen nachvollziehbaren Abschnitt pflegen:

```markdown
---

## Remediation-Status

| Datum | Bündel/Gruppe | Kategorie | Aktion | Notiz |
|---|---|---|---|---|
| YYYY-MM-DD | B1 | Task | Task 018 | keph:/user/kamran/tasks/018-redact-upstream-log.md |
```

Bei `RESULT_FORMAT=legacy-result-family` wird primär `findings.md` aktualisiert:

- Kategorie S nach erfolgreichem Fix und Verifikation: `- Status: `fixed``.
- Kategorie T mit Task: Status bleibt `open` oder `confirmed`, bis der Fix wirklich umgesetzt und verifiziert ist; ergänze aber `- Remediation: Task <NNN> - <keph:-Verweis>` oder aktualisiere ein vorhandenes Remediation-Feld.
- Kategorie K: `- Status: `context-needed`` mit klarer `Triage-Notiz`.
- Kategorie A: `- Status: `accepted`` mit Akzeptierungsgrund und optional Known-Decision-Verweis.
- Kategorie X: `- Status: `likely-false-positive`` mit Begründung.
- Wenn ein Befund durch Code-Lektüre bestätigt wurde, aber noch nicht behoben ist: `- Status: `confirmed``.

Am Ende von `findings.md` einen nachvollziehbaren Abschnitt pflegen:

```markdown
---

## Remediation-Log

| Datum | Finding-ID | Kategorie | Aktion | Notiz |
|---|---|---|---|---|
| YYYY-MM-DD | kcheck-logging-003 | Task | Task 018 | keph:/user/kamran/tasks/018-redact-upstream-log.md |
```

Legacy-`assessment.md` darf optional einen Abschnitt `## Remediation-Status` bekommen oder aktualisieren:

```markdown
## Remediation-Status

- YYYY-MM-DD: `/k-remediation` gestartet; <N> remediation-relevante Findings aus `findings.md` geladen.
- YYYY-MM-DD: Task(s) <keph:-Verweise> für Finding(s) <...> angelegt.
```

### Legacy-Result-Summaries

Bei `RESULT_FORMAT=summary` wird die vorhandene Summary-Datei selbst aktualisiert. Auch
hier gilt: fortgeschrieben wird allein der Abarbeitungsstand, eine neue Summary entsteht
nicht.

- Kategorie T mit Task: in der betroffenen Prioritätsgruppe `Status` oder eine kurze `Remediation:`-Zeile auf `Task <NNN> - <keph:-Verweis>` setzen.
- Kategorie K/A/X: Status mit knapper Begründung direkt in der betroffenen Gruppe dokumentieren.
- Kategorie S nach erfolgreichem Fix und Verifikation: Status auf `fixed` oder `behoben` setzen und Verifikation nennen.
- Wenn die Summary auf konkrete `review-triage.md`-Bündel oder Legacy-`findings.md`-IDs verweist, die zugehörigen Arbeitsdateien nach den jeweiligen Result-Family-Regeln synchron aktualisieren.

Am Ende der Summary einen nachvollziehbaren Abschnitt pflegen:

```markdown
---

## Remediation-Status

| Datum | Gruppe/Finding | Kategorie | Aktion | Notiz |
|---|---|---|---|---|
| YYYY-MM-DD | P1-01 | Task | Task 018 | keph:/user/kamran/tasks/018-redact-upstream-log.md |
```

`raw/` und `run-metadata.*` niemals bearbeiten.

---

## Schritt 8 — Log-Eintrag

Nichts wird archiviert oder verschoben. Lauf-Verzeichnisse bleiben stabil unter `k-playbook-local/results/<date>/`, Family-Verzeichnisse unter `k-playbook-local/results/<family>/<date>/`; vorhandene Legacy-Summaries bleiben stabil dort liegen, wo sie liegen — direkt unter `k-playbook-local/results/`. Abschluss erfolgt bei Audit-Läufen und aktuellen Result-Familien über `## Remediation-Status` in `review-triage.md`, bei Legacy-Familien über Statuswerte in `findings.md` und optional `## Remediation-Status` in `assessment.md`, bei einer Legacy-Summary direkt in der Summary.

Jeder Remediation-Lauf hinterlässt zusätzlich eine Zeile in `LOG_FILE` — auch dann, wenn er keinen einzigen Befund geändert hat:

1. Datei anlegen, falls sie noch nicht existiert (Skelett wie in `/k-review`).
2. Eine Zeile ans Protokoll am Dateiende anhängen:

   | Datum | Review | Scope | Output |
   |---|---|---|---|
   | 2026-07-12 | remediation | k-check/2026-07-12/review-triage.md | 4 Bündel / 2 Tasks / 1 behoben / 1 akzeptiert |

- `Datum`: `context.now.date`.
- `Review`: `remediation`.
- `Scope`: die abgearbeitete Ergebnisdatei, projektrelativ.
- `Output`: die Zahlen aus der Abschlusszusammenfassung in einem Satz.

Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` empfehlen. Nicht nach einem Ersatzpfad für nur diesen Lauf fragen.

---

## Schritt 9 — Abschlusszusammenfassung

Nach allen Befunden ausgeben:

```
Remediation abgeschlossen
─────────────────────────────────────
Bearbeitet:   <N>
✓ behoben:    <n>
✓ Tasks:      <n>  (<keph:-Verweise>)
~ akzeptiert: <n>
✗ falsch:     <n>
☐ offen:      <n>  (K/F — warten auf Klärung)
```

Wenn noch offene K- oder F-Punkte vorhanden: diese auflisten mit kurzer Begründung warum sie offen blieben.

Folge-Command: sind Tasks entstanden, **`/k-task-run`** — er setzt die erzeugten Tasks um,
wie Schritt 4 es nennt (nach `/k-task-refine`, wenn sie noch nicht gegengelesen sind; beide
nehmen den `keph:`-Verweis oder die Nummer als Argument). Sind keine Tasks entstanden, steht
ausdrücklich kein Folge-Command an.

---

## Fehlerfälle

- **Ergebnisdatei nicht gefunden / nicht plausibel**: verfügbare `<date>/review-triage.md` und `<family>/<date>/review-triage.md` in `<RESULTS_DIR>` auflisten; als Legacy zusätzlich vorhandene `summary-*.md` und `assessment.md` — letztere nur, wenn kein `review-triage.md` im selben Ordner existiert. Bei Formatabweichung: abbrechen statt raten.
- **Kein k-playbook-Projekt**: das Briefing schlägt fehl; abbrechen und `/k-gui` empfehlen.
- **`RESULTS_DIR` fehlt im Dateisystem**: fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen.
- **Kephalaion nicht bereit** (`context.kephalaion.ok` nicht `true`, `kephalaion_unavailable`): abbrechen mit `context.kephalaion.message`; ohne Collection gibt es keine Tasks und keinen Abgleich, und auf die Platte wird nicht ausgewichen.
