# Code-Review-Flow

Diese Seite fasst den k-playbook-Flow fuer Review-Rezepte, Result-Summaries und Remediation zusammen. Sie beschreibt Reviews ausserhalb konkreter GitHub-PRs; PR-spezifische Reviews stehen in [`pr-review.md`](./pr-review.md).

## Ueberblick

```mermaid
flowchart TD
    A["/k-review <name>"] --> B{"Review-Modus"}
    B -->|interaktiv| C["Fundstellen zeigen"]
    C --> D["Aenderungsvorschlaege bestaetigen"]
    D --> E["Bestaetigte Edits ausfuehren"]
    B -->|Report-Mode| F["Result-Familie schreiben"]
    F --> G["assessment.md"]
    F --> H["findings.md"]
    F --> I["raw/ und run-metadata"]
    G --> J["/k-results"]
    H --> J
    J --> K["summary-YYYY-MM-DD.md"]
    G --> L["/k-remediation <assessment-or-summary>"]
    K --> L
    L --> M{"Remediation-Policy"}
    M -->|task-branch-pr oder task-first| N["Task-Dateien erzeugen"]
    N --> O["/k-review-loop"]
    O --> P["/k-run"]
    M -->|direct-allowed| Q["kleine freigegebene Fixes"]
    L --> R["findings.md / assessment.md Status pflegen"]
```

Der Standardablauf fuer Report-Reviews ist:

```text
/k-review <name>
/k-results
/k-remediation <summary-or-assessment>
/k-review-loop
/k-run
```

## /k-review

`/k-review` fuehrt globale oder projektlokale Review-Rezepte gegen das aktuelle Zielprojekt aus. Der Command ist der Einstieg fuer strukturierte Reviews ausserhalb eines konkreten GitHub-PRs.

Aufrufe:

```text
/k-review
/k-review tech
/k-review codeql-security
/k-review secret-scanning
```

Ohne Argument zeigt der Command die verfuegbaren Review-Rezepte aus dem globalen Katalog und aus dem projektlokalen Reviews-Verzeichnis.

Der Command:

- loest Projektpfade aus `K-PLAYBOOK.yaml` auf.
- nutzt `paths.reviews` fuer Logs und Result-Artefakte.
- trennt generischen Ablauf von konkreten Review-Kriterien.
- erlaubt projektlokale Rezepte, die globale Rezepte mit gleichem Namen ueberlagern.
- beruecksichtigt `known-decisions.md`, damit bewusste Entscheidungen nicht wiederholt als neue Findings auftauchen.

Interaktive Reviews moderieren Stelle fuer Stelle:

- Kandidaten suchen.
- kompakte Fundliste zeigen.
- bei unklaren Punkten Rueckfragen gesammelt stellen.
- pro Stelle Vorschlag zeigen und auf Freigabe warten.
- nur bestaetigte Aenderungen ausfuehren.

Report-Mode-Reviews erzeugen Ergebnisartefakte:

```text
<paths.reviews>/results/<family>/<YYYY-MM-DD>/assessment.md
<paths.reviews>/results/<family>/<YYYY-MM-DD>/findings.md
<paths.reviews>/results/<family>/<YYYY-MM-DD>/raw/
```

Report-Mode-Reviews ohne eigene `result-family`, z. B. `tech`, schreiben direkt eine Summary:

```text
<paths.reviews>/results/summary-YYYY-MM-DD.md
```

Typische Review-Familien:

- `/k-review codeql-security`
- `/k-review k-check-security`
- `/k-review secret-scanning`
- `/k-review dependency-cve`
- `/k-review dependabot-alerts`
- `/k-review iac-container`
- `/k-review tech`
- `/k-review python-comment-hardspots`

## /k-results

`/k-results` erzeugt aus vorhandenen Review-Result-Familien eine projektweite, priorisierte Summary. Der Command ist der Zwischenschritt zwischen Report-Reviews und Remediation.

Aufrufe:

```text
/k-results
/k-results latest
/k-results 2026-08-03
```

Der Command:

- liest vorhandene `assessment.md`- und `findings.md`-Dateien unter `<paths.reviews>/results/`.
- startet keine Scanner.
- veraendert keine Raw-Artefakte.
- dedupliziert Findings ueber Familien hinweg.
- beruecksichtigt `known-decisions.md` und vorhandene Tasks, soweit vorhanden.
- schreibt eine Summary unter `<paths.reviews>/results/summary-YYYY-MM-DD.md`.

Die Summary enthaelt:

- verwendete Quellen.
- priorisierte Uebersicht.
- Dedupe-Entscheidungen.
- konkrete Empfehlungen.
- Handoff fuer Remediation.

Einzelne Result-Dateien koennen direkt an `/k-remediation` uebergeben werden. `/k-results` ist fuer die projektweite Priorisierung mehrerer Familien zustaendig.

## /k-remediation

`/k-remediation` arbeitet Findings aus Review-Ergebnissen strukturiert ab. Der Command plant zuerst sinnvolle Buendel und entscheidet danach anhand der Projekt-Policy, ob Tasks erzeugt oder kleine direkte Fixes erlaubt sind.

Aufrufe:

```text
/k-remediation
/k-remediation k-playbook/reviews/results/summary-YYYY-MM-DD.md
/k-remediation k-playbook/reviews/results/<family>/<date>/assessment.md
```

Unterstuetzte Inputs:

- Result-Summaries von `/k-results` oder Report-Reviews ohne eigene `result-family`.
- Result-Familien wie `<paths.reviews>/results/<family>/<date>/assessment.md` mit zugehoerigem `findings.md`.
- Legacy-Dateien wie `<paths.reviews>/result-*.md`.

Der Command:

- laedt offene Findings.
- beruecksichtigt `known-decisions.md`.
- buendelt Findings nach Risiko, Aufwand, Kopplung und Verifikation.
- zeigt die Remediation-Policy aus `K-PLAYBOOK.yaml`.
- ueberfuehrt bestaetigte Buendel in Tasks oder freigegebene direkte Fixes.
- pflegt Status und Task-Verweise nachvollziehbar in `findings.md` oder Summary.

`raw/` und Run-Metadaten bleiben read-only. Sie sind auditierbare Belege und duerfen nicht umgeschrieben werden.

## Remediation-Policy

Die Policy steht in `K-PLAYBOOK.yaml` im Block `remediation:`. Wichtige Felder sind:

- `mode`: `task-branch-pr`, `task-first` oder `direct-allowed`.
- `target`: tatsaechlicher Code-/Git-Root.
- `grouping`: ob Findings vor der Umsetzung gebuendelt werden.
- `quick_wins`: ob einfache wirkungsstarke Buendel hervorgehoben werden.
- `branch_prefix`: empfohlener Prefix fuer Remediation-Branches.
- `pr_required`: ob PR-Handoff erwartet wird.
- `direct_fixes`: ob direkte Fixes ueberhaupt erlaubt sind.

Im Modus `task-branch-pr` erzeugt `/k-remediation` keine direkten Code-Fixes. Bestaetigte Buendel werden als Task-Dateien mit Ausfuehrungskontext geschrieben.

## Artefakte Und Status

Jede Report-/Scan-Familie soll diese Dateien erzeugen:

- `assessment.md`: kuratierte Gesamtbewertung, Kurzfazit, Priorisierung, Handoff.
- `findings.md`: mutable, statusfaehige Arbeitsliste aller Findings oder bewusst gruppierter Baseline-Findings.
- `raw/`: auditierbare Originalausgaben, z. B. SARIF, JSON oder Tool-Logs.
- `run-metadata.json` oder aequivalent: auditierbare Laufmetadaten.

Standard-Statuswerte in `findings.md`:

| Status | Bedeutung | Remediation-Relevanz |
|---|---|---|
| `open` | neu oder noch nicht geprueft | ja |
| `confirmed` | validierter echter Befund | ja |
| `context-needed` | weitere Kontextpruefung noetig | ja |
| `likely-false-positive` | plausibler Fehlalarm | nur nach expliziter Auswahl |
| `accepted` | bewusste Entscheidung oder akzeptiertes Restrisiko | nein |
| `fixed` | behoben und verifiziert | nein |

Finding-IDs muessen stabil bleiben. Einmal vergebene IDs duerfen bei Re-Runs, Statusaenderungen oder Remediation nicht umbenannt werden.

## Handoff

Nach einem Report-Mode-Review nennt `/k-review` den naechsten Handoff, typischerweise:

```text
/k-results
/k-remediation <paths.reviews>/results/summary-YYYY-MM-DD.md
/k-remediation <paths.reviews>/results/<family>/<YYYY-MM-DD>/assessment.md
```

Wenn `/k-remediation` Tasks erzeugt, ist der naechste Schritt nicht direkte Umsetzung im Chat, sondern der normale Task-Flow:

```text
/k-review-loop
/k-run
```

Die erzeugten Tasks sollen Branch-/PR-Hinweise enthalten, wenn die Policy das verlangt. `/k-run` wertet den Abschnitt `## Ausfuehrungskontext` aus und fuehrt vor der Delegation Branch- und Dirty-Worktree-Preflights aus.

## Abgrenzung

- `/k-pr-review` ist fuer konkrete GitHub-PRs zustaendig.
- `/k-review` bewertet oder erzeugt Findings, setzt groessere Remediation aber nicht direkt um.
- `/k-results` ist read-mostly, erzeugt keine Tasks und fuehrt keine Remediation aus.
- `/k-remediation` startet keine Scanner und priorisiert nicht projektweit neu.
- Groessere Umsetzung laeuft ueber Tasks, `/k-review-loop` und `/k-run`.
- Direkte Fixes sind nur bei passender Policy und expliziter Freigabe erlaubt.
