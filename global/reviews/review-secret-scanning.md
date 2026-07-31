---
name: review-secret-scanning
title: Secret-Scanning Assessment
interval-weeks: 4
scope-hint: gitleaks/trufflehog-Ergebnisse fuer Git-Historie und Arbeitsbaum; keine Remediation, keine Secret-Rotation aus diesem Review heraus
handoff: /k-remediation
result-family: secret-scanning
---

# Review: Secret-Scanning Assessment

Erzeuge eine kuratierte, bewertete Liste aus Secret-Scanning-Ergebnissen. Dieses Review nutzt host-lokal installierte Tools aus `/k-install-security-tools` und schreibt projektlokale Review-Artefakte unter `k-playbook/reviews/`.

## Zweck

- Echte Secrets, produktive Credentials und Token-Leaks priorisiert sichtbar machen.
- Tool-Rohdaten von Review-Bewertung trennen.
- False Positives, Test-Fixtures und bekannte Entscheidungen nachvollziehbar markieren.
- Ein `assessment.md` mit priorisierter Triage-Reihenfolge und ein statusfaehiges `findings.md` erzeugen.
- Keine Produktcode-Aenderungen und keine Secret-Rotation durch dieses Review.

## Voraussetzungen

- Lies und verwende `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.
- Lies `<PLAYBOOK_REPO>/global/security-tools.tsv` als kanonische Security-Tool-Matrix; dieses Review nutzt daraus `gitleaks` und `trufflehog`.
- Wenn `K-PLAYBOOK.yaml` fehlt: abbrechen und `/k-gui` nennen.
- Wenn `k-playbook/reviews` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein lokales `reviews`-Ziel.
- Pruefe `gitleaks version` und `trufflehog --version`.
- Wenn Pflicht-Tools fehlen: abbrechen und `/k-install-security-tools --install missing` nennen.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook/reviews/results/secret-scanning/YYYY-MM-DD/`

Dateien:

- `assessment.md` - kuratierte Gesamtbewertung mit priorisierter Liste.
- `findings.md` - vollstaendiges, statusfaehiges Arbeitsregister.
- `raw/gitleaks-git.json` - Gitleaks-Git-Historienrohdata, falls ausgefuehrt.
- `raw/gitleaks-dir.json` - Gitleaks-Arbeitsbaumrohdata, falls ausgefuehrt.
- `raw/trufflehog.json` - TruffleHog-Rohdata, falls ausgefuehrt.
- `run-metadata.json` - Befehle, Exit-Codes, Zeitpunkt, Scope, Tool-Versionen.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und duerfen nach dem Schreiben nicht gekuerzt oder ueberschrieben werden.

## Ausfuehrungsentscheidung

Frage vor Tool-Ausfuehrung, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Keine neuen Scans.
- **Secret-Scan ausfuehren**: Nur nach Bestaetigung. Zeige vorher alle Befehle.
- **Nur Preflight**: Pfade, Tools und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestaetigung:

```bash
gitleaks git --report-format json --report-path <result>/raw/gitleaks-git.json <TARGET_DIR>
gitleaks dir --report-format json --report-path <result>/raw/gitleaks-dir.json <TARGET_DIR>
trufflehog git file://<TARGET_DIR> --json > <result>/raw/trufflehog.json
```

Exit-Codes von Secret-Scannern koennen Findings signalisieren. Ein non-zero Exit-Code ist nicht automatisch ein technischer Fehler; Bewertung anhand Rohdaten und Tool-Doku vornehmen.

## Bewertungskriterien

Prioritaet:

- P1: produktive Secrets, private Keys, Cloud-/Payment-/Database-Credentials, CI/CD-Tokens mit Schreibrechten.
- P2: plausibel aktive Tokens mit begrenztem Scope, interne Service-Credentials, Secrets in Git-Historie ohne sichtbare Rotation.
- P3: Test-Fixtures, Beispielwerte, low-confidence Findings, bereits rotierte oder offensichtlich deaktivierte Werte.

Review-Status in `findings.md`:

- `open` - neu oder noch nicht geprueft.
- `confirmed` - echter Secret-Fund.
- `context-needed` - Aktivitaet/Scope/Rotation unklar.
- `likely-false-positive` - plausibler Fehlalarm.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings aus mehreren Tools deduplizieren, wenn Datei, Zeile, Secret-Fingerprint oder semantischer Credential-Typ gleich sind.

## Assessment-Format

`assessment.md` enthaelt mindestens:

```markdown
# Secret-Scanning Assessment - YYYY-MM-DD

## Quellen

- Gitleaks: `raw/gitleaks-*.json`
- TruffleHog: `raw/trufflehog.json`
- Finding-Register: `findings.md`

## Kurzfazit

- Rohmeldungen: <n>
- Deduplizierte Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Befund: <kurz>

## Bewertete Liste

| Prio | Finding-ID | Status | Typ | Ort | Bewertung | Naechster Schritt |
|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook/reviews/results/secret-scanning/YYYY-MM-DD/assessment.md`
```

## Finding-Register-Format

`findings.md` enthaelt pro dedupliziertem Befund:

```markdown
### secret-001

- Status: `open`
- Prioritaet: `P1|P2|P3`
- Tool(s): `gitleaks`, `trufflehog`
- Typ: ...
- Ort: `path:line` oder Git-Commit
- Fingerprint: ...
- Raw-Quelle: `raw/...`
- Review-Bewertung: _offen_
- Rotation/Revocation: _offen_
- Remediation: _offen_
```

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation k-playbook/reviews/results/secret-scanning/YYYY-MM-DD/assessment.md
```

Remediation und Secret-Rotation sind ausdruecklich nicht Teil dieses Reviews.
