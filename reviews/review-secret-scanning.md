---
name: review-secret-scanning
title: Secret-Scanning Assessment
interval-weeks: 4
scope-hint: gitleaks/trufflehog-Ergebnisse für Git-Historie und Arbeitsbaum; keine Remediation, keine Secret-Rotation aus diesem Review heraus
handoff: /k-remediation
result-family: secret-scanning
audit:
  enabled: true
  title: Secret-Scanning Assessment
  resultRequired: true
  defaultResult: review-secret-scanning.md
review:
  enabled: true
---

# Review: Secret-Scanning Assessment

Erzeuge eine kuratierte, bewertete Liste aus Secret-Scanning-Ergebnissen. Dieses Review nutzt host-lokal installierte Security-Tools und schreibt projektlokale Review-Artefakte unter `k-playbook-local/results/`.

## Zweck

- Echte Secrets, produktive Credentials und Token-Leaks priorisiert sichtbar machen.
- Tool-Rohdaten von Review-Bewertung trennen.
- False Positives, Test-Fixtures und bekannte Entscheidungen nachvollziehbar markieren.
- `review-input.json` als Belegvertrag und `review-triage.md` als Handoff erzeugen.
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

- `review-input.json` - strukturierter Belegvertrag mit Scope, Gruppen, Evidence und Known-Decision-Coverage.
- `review-triage.md` - kuratierte Gesamtbewertung mit Bündeln, nicht gebündelten Findings und Handoff.
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

Review-Status im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - echter Secret-Fund.
- `context-needed` - Aktivität/Scope/Rotation unklar.
- `likely-false-positive` - plausibler Fehlalarm.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings aus mehreren Tools deduplizieren, wenn Datei, Zeile, Secret-Fingerprint oder semantischer Credential-Typ gleich sind.

## Review-Triage-Format

`review-triage.md` enthält die Pflichtabschnitte aus `commands/_audit/review-scan-triage.md`:

```markdown
# Review-Triage secret-scanning/YYYY-MM-DD

## Quellen

- Gitleaks: `raw/gitleaks-*.json`
- TruffleHog: `raw/trufflehog.json`
- Quelle: `review-input.json`

## Bündel

| ID | Priorität | Kategorie | Kurzbegründung | Gruppen |
|---|---|---|---|---|

## Bündel-Details

### B1 — <Titel>

Begründung: ...

Betroffene Belege: ...

Nächster Schritt: ...

## Nicht gebündelt

## Deckung aus known-decisions

## Handoff

`/k-remediation k-playbook-local/results/secret-scanning/YYYY-MM-DD/review-triage.md`
```

## Review-Input-Format

`review-input.json` enthält pro dedupliziertem Befund Evidence mit Datei, Zeile, Quelle und Tool:

```json
{
  "scope": { "type": "review", "family": "secret-scanning" },
  "groups": [
    {
      "id": "secret-001",
      "title": "<Credential-Typ> in <Ort>",
      "priority": "P1|P2|P3",
      "findings": ["secret-001"],
      "evidence": [
        {
          "file": "path",
          "line": 12,
          "source": "tool:gitleaks|trufflehog",
          "message": "<Secret-Typ>, Fingerprint, Raw-Quelle raw/..."
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
/k-remediation k-playbook-local/results/secret-scanning/YYYY-MM-DD/review-triage.md
```

Remediation und Secret-Rotation sind ausdrücklich nicht Teil dieses Reviews.
