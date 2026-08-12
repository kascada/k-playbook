---
name: review-secret-scanning
title: Secret-Scanning Assessment
interval-weeks: 4
scope-hint: gitleaks/trufflehog-Ergebnisse für Git-Historie und Arbeitsbaum; keine Remediation, keine Secret-Rotation aus diesem Review heraus
handoff: /k-remediation
result-family: secret-scanning
---

# Review: Secret-Scanning Assessment

Erzeuge eine kuratierte, bewertete Liste aus Secret-Scanning-Ergebnissen. Dieses Review nutzt host-lokal installierte Security-Tools und schreibt projektlokale Review-Artefakte unter `k-playbook-local/results/`.

## Zweck

- Echte Secrets, produktive Credentials und Token-Leaks priorisiert sichtbar machen.
- Tool-Rohdaten von Review-Bewertung trennen.
- False Positives, Test-Fixtures und bekannte Entscheidungen nachvollziehbar markieren.
- Ein `assessment.md` mit priorisierter Triage-Reihenfolge und ein statusfähiges `findings.md` erzeugen.
- Keine Produktcode-Änderungen und keine Secret-Rotation durch dieses Review.

## Voraussetzungen

- Pfade kommen aus der Context-Ausgabe, die `/k-review` bereits geladen hat: `RESULTS_DIR` = `<local.dir>/results`.
- Lies `<playbook.dir>/scripts/security-tools.tsv` als kanonische Security-Tool-Matrix; dieses Review nutzt daraus `gitleaks` und `trufflehog`.
- Wenn der Context-Aufruf fehlschlägt: abbrechen und `/k-gui` empfehlen.
- Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein Ergebnisverzeichnis.
- Prüfe `gitleaks version` und `trufflehog --version`.
- Wenn Pflicht-Tools fehlen: abbrechen und auf den Preflight verweisen:
  `bash <playbook.dir>/scripts/install-security-tools.sh` nennt den passenden
  Installationsbefehl.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook-local/results/secret-scanning/YYYY-MM-DD/`

Dateien:

- `assessment.md` - kuratierte Gesamtbewertung mit priorisierter Liste.
- `findings.md` - vollständiges, statusfähiges Arbeitsregister.
- `raw/gitleaks-git.json` - Gitleaks-Git-Historienrohdata, falls ausgeführt.
- `raw/gitleaks-dir.json` - Gitleaks-Arbeitsbaumrohdata, falls ausgeführt.
- `raw/trufflehog.json` - TruffleHog-Rohdata, falls ausgeführt.
- `run-metadata.json` - Befehle, Exit-Codes, Zeitpunkt, Scope, Tool-Versionen.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und dürfen nach dem Schreiben nicht gekürzt oder überschrieben werden.

## Ausführungsentscheidung

Frage vor Tool-Ausführung, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Keine neuen Scans.
- **Secret-Scan ausführen**: Nur nach Bestätigung. Zeige vorher alle Befehle.
- **Nur Preflight**: Pfade, Tools und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestätigung:

```bash
gitleaks git --report-format json --report-path <result>/raw/gitleaks-git.json <PROJECT_REPO_ROOT_DIR>
gitleaks dir --report-format json --report-path <result>/raw/gitleaks-dir.json <PROJECT_REPO_ROOT_DIR>
trufflehog git file://<PROJECT_REPO_ROOT_DIR> --json > <result>/raw/trufflehog.json
```

Exit-Codes von Secret-Scannern können Findings signalisieren. Ein non-zero Exit-Code ist nicht automatisch ein technischer Fehler; Bewertung anhand Rohdaten und Tool-Doku vornehmen.

## Bewertungskriterien

Priorität:

- P1: produktive Secrets, private Keys, Cloud-/Payment-/Database-Credentials, CI/CD-Tokens mit Schreibrechten.
- P2: plausibel aktive Tokens mit begrenztem Scope, interne Service-Credentials, Secrets in Git-Historie ohne sichtbare Rotation.
- P3: Test-Fixtures, Beispielwerte, low-confidence Findings, bereits rotierte oder offensichtlich deaktivierte Werte.

Review-Status in `findings.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - echter Secret-Fund.
- `context-needed` - Aktivität/Scope/Rotation unklar.
- `likely-false-positive` - plausibler Fehlalarm.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings aus mehreren Tools deduplizieren, wenn Datei, Zeile, Secret-Fingerprint oder semantischer Credential-Typ gleich sind.

## Assessment-Format

`assessment.md` enthält mindestens:

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

| Prio | Finding-ID | Status | Typ | Ort | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook-local/results/secret-scanning/YYYY-MM-DD/assessment.md`
```

## Finding-Register-Format

`findings.md` enthält pro dedupliziertem Befund:

```markdown
### secret-001

- Status: `open`
- Priorität: `P1|P2|P3`
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
/k-remediation k-playbook-local/results/secret-scanning/YYYY-MM-DD/assessment.md
```

Remediation und Secret-Rotation sind ausdrücklich nicht Teil dieses Reviews.
