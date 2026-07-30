---
name: review-k-check-security
title: k-check Security Assessment
interval-weeks: 4
scope-hint: k-check Runner-Ergebnisse fuer globale und projektlokale Checks; keine Remediation, keine Produktcode-Aenderungen
handoff: /k-remediation
result-family: k-check
---

# Review: k-check Security Assessment

Erzeuge eine kuratierte Security-Bewertung aus `k-check`-Ergebnissen. `k-check` bleibt Runner/Executor; Review, Priorisierung und Remediation-Handoff gehoeren in diese Review-Familie und danach in `/k-remediation`.

## Zweck

- Globale und projektlokale Check-Ausgaben dauerhaft als Review-Artefakte sichern.
- Runner-Rohdaten von kuratierter Bewertung trennen.
- Findings mit stabilen IDs und Status-Lifecycle fuer spaetere Remediation vorbereiten.
- `skip`, `ok`, `fail` und technische `error`-Faelle getrennt dokumentieren.
- Keine Produktcode-Aenderungen durch dieses Review.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook/reviews/results/k-check/YYYY-MM-DD/`

Dateien:

- `assessment.md` - kuratierte Gesamtbewertung.
- `findings.md` - vollstaendiges, statusfaehiges Arbeitsregister.
- `raw/k-check-<mode>.txt` - auditierbare Originalausgabe des Runners.
- `run-metadata.json` - auditierbare Laufmetadaten.

`raw/` und `run-metadata.json` sind append-only/auditierbar: nach dem Schreiben nicht kuerzen, ueberschreiben oder inhaltlich korrigieren. Korrekturen erfolgen durch neue Raw-Dateien und eine aktualisierte Bewertung. `findings.md` ist das mutable Arbeitsregister fuer Status, Owner, Remediation-Notizen, Akzeptierungen und Fix-Verweise. `assessment.md` ist kuratiert und darf spaeter nur nachvollziehbar aktualisiert werden, z. B. fuer Summary, Handoff-Status oder explizite Remediation-Abschnitte.

## Voraussetzungen

Pfad- und Statusaufloesung:

- Lies und verwende `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.
- Wenn `K-PLAYBOOK.MD` fehlt: abbrechen und `/k-setup` nennen.
- Wenn `k-playbook/reviews` fehlt: abbrechen und `/k-setup` nennen; dieses Review braucht ein lokales `reviews`-Ziel.
- Wenn `k-playbook/checks` leer ist, werden nur globale Checks ausgefuehrt.
- Das Runner-Script ist `<PLAYBOOK_REPO>/global/bin/k-check`.

## Ausfuehrungsarten

Frage vor Runner-Ausfuehrung, was passieren soll. Langlaufende Baseline-Laeufe niemals ohne sichtbares Kommando starten.

Optionen:

- **Vorhandene Raw-Ausgabe auswerten (Default)**: Nutzt `raw/k-check-*.txt` aus dem Result-Verzeichnis oder eine explizit angegebene Raw-Datei. Keine neue Runner-Ausfuehrung.
- **`k-check` ausfuehren**: Nur nach Bestaetigung. Zeige vorher den exakten Befehl inklusive `--config-root`, `--target-root`, `--mode`, `--output` und `--metadata-output`.
- **Nur Preflight**: Keine Report-Erzeugung, nur Pfade, Runner und geplante Artefakte zeigen.
- **Abbrechen**.

Typischer Lauf:

```bash
~/dev/k-playbook/global/bin/k-check \
  --config-root <TARGET_DIR> \
  --target-root <target-root> \
  --mode <changed|baseline> \
  --output k-playbook/reviews/results/k-check/YYYY-MM-DD/raw/k-check-<mode>.txt \
  --metadata-output k-playbook/reviews/results/k-check/YYYY-MM-DD/run-metadata.json
```

Das normale stdout/stderr-Verhalten bleibt erhalten; `--output` schreibt zusaetzlich den vollstaendigen Raw-Stream.

## Parser-Regeln

Raw-k-check-Ausgaben werden nach diesen Regeln ausgewertet:

- Check-Abschnitte beginnen mit `== <scope>:<check.sh> ==`, z. B. `== global:check_no_obvious_secrets.sh ==`.
- Innerhalb eines Abschnitts genau eine `K_CHECK_STATUS=<ok|skip|fail>`-Zeile erfassen.
- Optional `K_CHECK_REASON=<text>` erfassen.
- Alles zwischen Abschnittsheader und Status/Summary als Check-Ausgabe behandeln.
- Die Summary beginnt bei `K_CHECK_SUMMARY` und enthaelt `config_root`, `target_root`, `mode`, `file_source`, `files`, `ok`, `skip`, `fail`, `error`.
- Summary-Zeilen `OK|SKIP|FAIL|ERROR <scope>:<check.sh> reason=...` als Check-Gesamtstatus erfassen.
- `ok` und `skip` in `assessment.md` separat dokumentieren; `skip` mit Reason und Wiedervorlage.
- `fail`-Findings nach Check-Familie gruppieren.
- Technische `error`-Faelle sind keine Security-Findings, aber blockieren die Bewertbarkeit des betroffenen Checks.

Finding-Zeilen sind check-spezifisch. Generische globale Checks schreiben typischerweise:

```text
path:line: message
```

Lokale Legacy-Runner koennen Sammelzeilen oder abweichende Formate liefern. Wenn globale und lokale Checks dieselbe semantische Stelle melden, globale Finding-ID behalten und lokale Legacy-Meldung als Beleg/Notiz deduplizieren statt eine zweite Remediation-ID zu erzeugen.

## Status-Lifecycle

Statuswerte in `findings.md`:

- `open` - neu oder noch nicht geprueft.
- `confirmed` - validierter echter Befund.
- `context-needed` - ohne weiteren Code-/Betriebskontext nicht belastbar bewertbar.
- `likely-false-positive` - plausibler Fehlalarm; review-relevant, aber nur nach expliziter Auswahl remediation-relevant.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Remediation-relevant sind `open`, `confirmed` und `context-needed`. `accepted` und `fixed` sind Endzustaende und duerfen nicht automatisch in neue Fix-Tasks ueberfuehrt werden.

## Stabile Finding-IDs

Einmal vergebene IDs duerfen bei Re-Runs, Statusaenderungen oder Remediation nicht umbenannt werden.

Schema fuer neue k-check-IDs:

`kcheck-<area>-NNN`

Beispiele:

- `kcheck-logging-003`
- `kcheck-secrets-001`
- `kcheck-user-scope-014`

Neue IDs werden nur fuer neue semantische Findings vergeben. Wiedergefundene oder deduplizierte Findings behalten die bestehende ID. Importierte Tool-IDs anderer Familien duerfen ihr natives Praefix behalten.

## Priorisierung

- P1: echte Secrets, produktive Credentials, raw Provider-/Upstream-Responses in Logs.
- P1/P2: user-facing Authz-/Ownership-Findings.
- P2: sensitive Logging, Operational-Event-Kontraktverletzungen.
- P3: Legacy-Baseline, Test-Fixtures, wahrscheinliche Heuristik-False-Positives.

## Assessment-Format

`assessment.md` enthaelt mindestens:

```markdown
# k-check Assessment - YYYY-MM-DD

## Quellen

- Runner: `<befehl>`
- Raw: `raw/k-check-<mode>.txt`
- Run-Metadaten: `run-metadata.json`
- Finding-Register: `findings.md`

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
- P1/P2/P3-Einschaetzung: ...

## Ergebnisuebersicht

| Check | Status | Anzahl | Bewertung |
|---|---:|---:|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook/reviews/results/k-check/YYYY-MM-DD/assessment.md`
```

## Finding-Register-Format

`findings.md` enthaelt alle remediation-faehigen Findings oder bewusst gruppierte Legacy-Baselines:

```markdown
### kcheck-logging-003

- Status: `open`
- Prioritaet: `P1|P2|P3`
- Quelle: `global:check_logging_privacy_generic.sh`
- Ort: `path:line`
- Message: ...
- Raw-Quelle: `raw/k-check-baseline.txt`
- Review-Bewertung: _offen_
- Triage-Notiz: _offen_
- Owner: _offen_
- Remediation: _offen_
```

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation k-playbook/reviews/results/k-check/YYYY-MM-DD/assessment.md
```

Remediation ist ausdruecklich nicht Teil dieses Reviews.
