---
name: review-k-check-security
title: k-check Security Assessment
interval-weeks: 4
scope-hint: k-check Runner-Ergebnisse für globale und projektlokale Checks; keine Remediation, keine Produktcode-Änderungen
handoff: /k-remediation
result-family: k-check
audit:
  enabled: false
  title: k-check Security Assessment
  resultRequired: true
  defaultResult: review-k-check-security.md
review:
  enabled: true
---

# Review: k-check Security Assessment

Erzeuge eine kuratierte Security-Bewertung aus `k-check`-Ergebnissen. `k-check` bleibt Runner/Executor; Review, Priorisierung und Remediation-Handoff gehören in diese Review-Familie und danach in `/k-remediation`.

## Zweck

- Globale und projektlokale Check-Ausgaben dauerhaft als Review-Artefakte sichern.
- Runner-Rohdaten von kuratierter Bewertung trennen.
- Findings mit stabilen IDs und Status-Lifecycle für spätere Remediation vorbereiten.
- `skip`, `ok`, `fail` und technische `error`-Fälle getrennt dokumentieren.
- Keine Produktcode-Änderungen durch dieses Review.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook-local/results/k-check/YYYY-MM-DD/`

Dateien:

- `review-input.json` - strukturierter Belegvertrag mit Scope, Gruppen, Evidence und Known-Decision-Coverage.
- `review-triage.md` - kuratierte Gesamtbewertung mit Bündeln, nicht gebündelten Findings und Handoff.
- `raw/k-check-<mode>.txt` - auditierbare Originalausgabe des Runners.
- `run-metadata.json` - auditierbare Laufmetadaten.

`raw/` und `run-metadata.json` sind append-only/auditierbar: nach dem Schreiben nicht kürzen, überschreiben oder inhaltlich korrigieren. Korrekturen erfolgen durch neue Raw-Dateien plus aktualisierte `review-input.json` und `review-triage.md`. `review-triage.md` darf später nur nachvollziehbar aktualisiert werden, z. B. für Handoff-Status oder explizite Remediation-Abschnitte.

## Voraussetzungen

Pfad- und Statusauflösung:

- Pfade kommen aus der Context-Ausgabe, die `/k-review` bereits geladen hat: `RESULTS_DIR` = `<local.dir>/results`.
- Wenn der Context-Aufruf fehlschlägt: abbrechen und `/k-gui` empfehlen.
- Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein Ergebnisverzeichnis.
- Wenn `<local.dir>/checks/` leer ist, werden nur die mitgelieferten Checks ausgeführt.
- Das Runner-Script ist `<playbook.dir>/bin/k-check`.

## Ausführungsarten

Frage vor Runner-Ausführung, was passieren soll. Langlaufende Baseline-Läufe niemals ohne sichtbares Kommando starten.

Optionen:

- **Vorhandene Raw-Ausgabe auswerten (Default)**: Nutzt `raw/k-check-*.txt` aus dem Result-Verzeichnis oder eine explizit angegebene Raw-Datei. Keine neue Runner-Ausführung.
- **`k-check` ausführen**: Nur nach Bestätigung. Zeige vorher den exakten Befehl inklusive `--config-root`, `--target-root`, `--mode`, `--output` und `--metadata-output`.
- **Nur Preflight**: Keine Report-Erzeugung, nur Pfade, Runner und geplante Artefakte zeigen.
- **Abbrechen**.

Typischer Lauf:

```bash
<playbook.dir>/bin/k-check \
  --config-root <project.dir> \
  --target-root <target-root> \
  --mode <changed|baseline> \
  --output k-playbook-local/results/k-check/YYYY-MM-DD/raw/k-check-<mode>.txt \
  --metadata-output k-playbook-local/results/k-check/YYYY-MM-DD/run-metadata.json
```

Das normale stdout/stderr-Verhalten bleibt erhalten; `--output` schreibt zusätzlich den vollständigen Raw-Stream.

## Parser-Regeln

Raw-k-check-Ausgaben werden nach diesen Regeln ausgewertet:

- Check-Abschnitte beginnen mit `== <scope>:<check.sh> ==`, z. B. `== global:check_no_obvious_secrets.sh ==`.
- Innerhalb eines Abschnitts genau eine `K_CHECK_STATUS=<ok|skip|fail>`-Zeile erfassen.
- Optional `K_CHECK_REASON=<text>` erfassen.
- Alles zwischen Abschnittsheader und Status/Summary als Check-Ausgabe behandeln.
- Die Summary beginnt bei `K_CHECK_SUMMARY` und enthält `config_root`, `target_root`, `mode`, `file_source`, `files`, `ok`, `skip`, `fail`, `error`.
- Summary-Zeilen `OK|SKIP|FAIL|ERROR <scope>:<check.sh> reason=...` als Check-Gesamtstatus erfassen.
- `ok` und `skip` in `review-triage.md` separat dokumentieren; `skip` mit Reason und Wiedervorlage.
- `fail`-Findings nach Check-Familie gruppieren.
- Technische `error`-Fälle sind keine Security-Findings, aber blockieren die Bewertbarkeit des betroffenen Checks.

Finding-Zeilen sind check-spezifisch. Generische globale Checks schreiben typischerweise:

```text
path:line: message
```

Lokale Legacy-Runner können Sammelzeilen oder abweichende Formate liefern. Wenn globale und lokale Checks dieselbe semantische Stelle melden, globale Finding-ID behalten und lokale Legacy-Meldung als Beleg/Notiz deduplizieren statt eine zweite Remediation-ID zu erzeugen.

## Status-Lifecycle

Statuswerte im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - validierter echter Befund.
- `context-needed` - ohne weiteren Code-/Betriebskontext nicht belastbar bewertbar.
- `likely-false-positive` - plausibler Fehlalarm; review-relevant, aber nur nach expliziter Auswahl remediation-relevant.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Remediation-relevant sind `open`, `confirmed` und `context-needed`. `accepted` und `fixed` sind Endzustände und dürfen nicht automatisch in neue Fix-Tasks überführt werden.

## Stabile Finding-IDs

Einmal vergebene IDs dürfen bei Re-Runs, Statusänderungen oder Remediation nicht umbenannt werden.

Schema für neue k-check-IDs:

`kcheck-<area>-NNN`

Beispiele:

- `kcheck-logging-003`
- `kcheck-secrets-001`
- `kcheck-user-scope-014`

Neue IDs werden nur für neue semantische Findings vergeben. Wiedergefundene oder deduplizierte Findings behalten die bestehende ID. Importierte Tool-IDs anderer Familien dürfen ihr natives Präfix behalten.

## Priorisierung

- P1: echte Secrets, produktive Credentials, raw Provider-/Upstream-Responses in Logs.
- P1/P2: user-facing Authz-/Ownership-Findings.
- P2: sensitive Logging, Operational-Event-Kontraktverletzungen.
- P3: Legacy-Baseline, Test-Fixtures, wahrscheinliche Heuristik-False-Positives.

## Review-Triage-Format

`review-triage.md` enthält mindestens die Pflichtabschnitte aus `commands/_audit/review-scan-triage.md`:

```markdown
# k-check Assessment - YYYY-MM-DD

## Quellen

- Runner: `<befehl>`
- Raw: `raw/k-check-<mode>.txt`
- Run-Metadaten: `run-metadata.json`
- Quelle: `review-input.json`

## Run-Metadaten Kurzfassung

- Arbeitsverzeichnis: ...
- Exit-Code: ...
- config_root: ...
- target_root: ...
- mode: ...
- Check-Konfiguration: globale Checks ..., lokale Checks ...
- k-check-Version/Git-Commit: ...

## Kurzfazit

- Technische Runner-Fehler: ...
- Security-relevante Fail-Gruppen: ...
- P1/P2/P3-Einschätzung: ...

## Ergebnisübersicht

| Check | Status | Anzahl | Bewertung |
|---|---:|---:|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook-local/results/k-check/YYYY-MM-DD/review-triage.md`
```

## Review-Input-Format

`review-input.json` enthält remediation-fähige Findings oder bewusst gruppierte Legacy-Baselines:

```json
{
  "scope": { "type": "review", "family": "k-check" },
  "groups": [
    {
      "id": "kcheck-logging-003",
      "title": "<Check-Familie> - <Kurzproblem>",
      "priority": "P1|P2|P3",
      "findings": ["kcheck-logging-003"],
      "evidence": [
        {
          "file": "path",
          "line": 12,
          "source": "global:check_logging_privacy_generic.sh",
          "message": "<Check-Message>, Raw-Quelle raw/k-check-baseline.txt"
        }
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

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation k-playbook-local/results/k-check/YYYY-MM-DD/review-triage.md
```

Remediation ist ausdrücklich nicht Teil dieses Reviews.
