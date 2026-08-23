---
name: review-k-check-security
title: k-check Security Assessment
interval-weeks: 4
scope-hint: k-check Runner-Ergebnisse für globale und projektlokale Checks; keine Remediation, keine Produktcode-Änderungen
handoff: /k-remediation
result-family: k-check
audit:
  enabled: false
review:
  enabled: true
---

# Review: k-check Security Assessment

Erzeuge eine kuratierte Security-Bewertung aus `k-check`-Ergebnissen. `k-check` bleibt
Runner/Executor; Review, Priorisierung und Remediation-Handoff gehören in diese
Review-Familie und danach in `/k-remediation`.

Dieses Rezept bleibt im Audit-Laufmodell deaktiviert, solange `k-check`-Ergebnisse nicht
als Evidence mit einem kanonischen Tool-Schlüssel in `review-input.json` aufgenommen
werden. Über `/k-review k-check-security` bleibt es gezielt auswählbar.

## Zweck

- Globale und projektlokale Check-Ausgaben dauerhaft als Review-Artefakte sichern.
- Runner-Rohdaten von kuratierter Bewertung trennen.
- Findings mit stabilen IDs und Status-Lifecycle für spätere Remediation vorbereiten.
- `skip`, `ok`, `fail` und technische `error`-Fälle getrennt dokumentieren.
- Keine Produktcode-Änderungen durch dieses Review.

## Parser-Regeln

`k-check`-Ausgaben werden nach diesen Regeln ausgewertet:

- Check-Abschnitte beginnen mit `== <scope>:<check.sh> ==`, z. B.
  `== global:check_no_obvious_secrets.sh ==`.
- Innerhalb eines Abschnitts genau eine `K_CHECK_STATUS=<ok|skip|fail>`-Zeile erfassen.
- Optional `K_CHECK_REASON=<text>` erfassen.
- Alles zwischen Abschnittsheader und Status/Summary als Check-Ausgabe behandeln.
- Die Summary beginnt bei `K_CHECK_SUMMARY` und enthält `config_root`, `target_root`,
  `mode`, `file_source`, `files`, `ok`, `skip`, `fail`, `error`.
- Summary-Zeilen `OK|SKIP|FAIL|ERROR <scope>:<check.sh> reason=...` als
  Check-Gesamtstatus erfassen.
- `ok` und `skip` in `review-triage.md` separat dokumentieren; `skip` mit Reason und
  Wiedervorlage.
- `fail`-Findings nach Check-Familie gruppieren.
- Technische `error`-Fälle sind keine Security-Findings, aber blockieren die Bewertbarkeit
  des betroffenen Checks.

Finding-Zeilen sind check-spezifisch. Generische globale Checks schreiben typischerweise:

```text
path:line: message
```

Lokale Legacy-Runner können Sammelzeilen oder abweichende Formate liefern. Wenn globale
und lokale Checks dieselbe semantische Stelle melden, globale Finding-ID behalten und
lokale Legacy-Meldung als Beleg/Notiz deduplizieren statt eine zweite Remediation-ID zu
erzeugen.

## Status-Lifecycle

Statuswerte im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - validierter echter Befund.
- `context-needed` - ohne weiteren Code-/Betriebskontext nicht belastbar bewertbar.
- `likely-false-positive` - plausibler Fehlalarm; review-relevant, aber nur nach
  expliziter Auswahl remediation-relevant.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Remediation-relevant sind `open`, `confirmed` und `context-needed`. `accepted` und
`fixed` sind Endzustände und dürfen nicht automatisch in neue Fix-Tasks überführt werden.

## Stabile Finding-IDs

Einmal vergebene IDs dürfen bei Re-Runs, Statusänderungen oder Remediation nicht
umbenannt werden.

Schema für neue k-check-IDs:

`kcheck-<area>-NNN`

Beispiele:

- `kcheck-logging-003`
- `kcheck-secrets-001`
- `kcheck-user-scope-014`

Neue IDs werden nur für neue semantische Findings vergeben. Wiedergefundene oder
deduplizierte Findings behalten die bestehende ID. Importierte Tool-IDs anderer Familien
dürfen ihr natives Präfix behalten.

## Priorisierung

- P1: echte Secrets, produktive Credentials, raw Provider-/Upstream-Responses in Logs.
- P1/P2: user-facing Authz-/Ownership-Findings.
- P2: sensitive Logging, Operational-Event-Kontraktverletzungen.
- P3: Legacy-Baseline, Test-Fixtures, wahrscheinliche Heuristik-False-Positives.

## Handoff

Nach Abschluss verweist `/k-review` auf `review-triage.md` im Family-Ordner. Remediation
ist ausdrücklich nicht Teil dieses Reviews.
