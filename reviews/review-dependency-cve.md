---
name: review-dependency-cve
title: Dependency-CVE Assessment
interval-weeks: 4
scope-hint: Dependency-CVE-Ergebnisse aus Manifesten und Lockfiles; keine Upgrades oder Remediation aus diesem Review heraus
handoff: /k-remediation
result-family: dependency-cve
audit:
  enabled: true
  title: Dependency-CVE Assessment
  resultRequired: true
  defaultResult: review-dependency-cve.md
review:
  enabled: true
---

# Review: Dependency-CVE Assessment

Erzeuge eine kuratierte, bewertete Liste aus Dependency-CVE-Scans. Dieses Review nutzt host-lokal installierte Security-Tools und schreibt projektlokale Review-Artefakte unter `k-playbook-local/results/`.

## Zweck

- Bekannte CVEs in direkten und relevanten transitiven Dependencies sichtbar machen.
- Tool-Severity von projektspezifischer Review-Priorität trennen.
- Exploitability, Reachability-Hinweise und Upgrade-Aufwand für die Triage erfassen.
- `review-input.json` als Belegvertrag und `review-triage.md` als Handoff erzeugen.
- Keine Dependency-Upgrades durch dieses Review.

## Voraussetzungen

- Pfade kommen aus der Context-Ausgabe, die `/k-review` bereits geladen hat: `RESULTS_DIR` = `<local.dir>/results`.
- Lies `<playbook.dir>/scripts/security-tools.tsv` als kanonische Security-Tool-Matrix; dieses Review nutzt daraus `pip-audit`, `trivy` und `grype`.
- Wenn der Context-Aufruf fehlschlägt: abbrechen und `/k-gui` empfehlen.
- Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein Ergebnisverzeichnis.
- Prüfe `pip-audit --version`, `trivy --version` und `grype --version`.
- Wenn Pflicht-Tools fehlen: abbrechen und auf den Preflight verweisen:
  `bash <playbook.dir>/scripts/install-security-tools.sh` nennt den passenden
  Installationsbefehl.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook-local/results/dependency-cve/YYYY-MM-DD/`

Dateien:

- `review-input.json` - strukturierter Belegvertrag mit Scope, Gruppen, Evidence und Known-Decision-Coverage.
- `review-triage.md` - kuratierte Gesamtbewertung mit Bündeln, nicht gebündelten Findings und Handoff.
- `raw/pip-audit.json` - Python-CVE-Rohdaten, falls anwendbar.
- `raw/trivy-fs.json` - Trivy-Filesystem-/Dependency-Rohdaten.
- `raw/grype.json` - Grype-Rohdaten, falls ausgeführt.
- `run-metadata.json` - Befehle, Exit-Codes, Zeitpunkt, Scope, Tool-Versionen.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und dürfen nach dem Schreiben nicht gekürzt oder überschrieben werden.

## Ausführungsentscheidung

Frage vor Tool-Ausführung, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Keine neuen Scans.
- **Dependency-CVE-Scan ausführen**: Nur nach Bestätigung. Zeige vorher alle Befehle.
- **Nur Preflight**: Pfade, Tools, erkannte Manifest-/Lockfiles und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestätigung:

```bash
pip-audit --format json --output <result>/raw/pip-audit.json --path <PROJECT_REPO_ROOT_DIR>
trivy fs --format json --output <result>/raw/trivy-fs.json <PROJECT_REPO_ROOT_DIR>
grype dir:<PROJECT_REPO_ROOT_DIR> -o json > <result>/raw/grype.json
```

`pip-audit` nur ausführen, wenn Python-Manifeste oder Python-Lockfiles gefunden wurden. `grype` gehört zur Pflicht-Toolchain; ob der konkrete Grype-Scan in diesem Review sinnvoll ist, hängt vom Scope und den vorhandenen Manifests/SBOMs ab.

## Bewertungskriterien

Tool-Severity allein ist nicht die Review-Priorität. Priorisiere nach:

- P1: kritisch/hoch, remote ausnutzbar, produktionsnah, direkt erreichbar oder in Auth/Parsing/Network-Pfad.
- P2: hohe oder mittlere CVEs in direkt genutzten Dependencies oder in zentralen Runtime-Komponenten.
- P3: transitive CVEs ohne sichtbare Nutzung, Dev-/Test-only Dependencies, unklare oder niedrige Ausnutzbarkeit.

Berücksichtige:

- Direkt vs. transitiv.
- Manifest-/Lockfile-Quelle.
- Fix-Version und Upgrade-Pfad.
- Ob die betroffene Komponente im Produktpfad genutzt wird.
- Bekannte Entscheidungen aus `known-decisions.md`.

Review-Status im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - CVE betrifft eine relevante Dependency im Produktkontext.
- `context-needed` - Reachability oder Laufzeitpfad unklar.
- `likely-false-positive` - Tool-Mapping oder Umgebung wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings deduplizieren, wenn CVE-ID, Package, Version und Manifest-/Lockfile-Quelle gleich sind.

## Review-Triage-Format

`review-triage.md` enthält mindestens die Pflichtabschnitte aus `commands/_audit/review-scan-triage.md`:

```markdown
# Dependency-CVE Assessment - YYYY-MM-DD

## Quellen

- pip-audit: `raw/pip-audit.json`
- Trivy: `raw/trivy-fs.json`
- Grype: `raw/grype.json`
- Quelle: `review-input.json`

## Kurzfazit

- Rohmeldungen: <n>
- Deduplizierte CVE-Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Upgrade-/Patch-Punkt: <kurz>

## Bewertete Liste

| Prio | Finding-ID | Status | Package | CVE | Betroffene Version | Fix-Version | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook-local/results/dependency-cve/YYYY-MM-DD/review-triage.md`
```

## Review-Input-Format

`review-input.json` enthält pro dedupliziertem Befund Evidence mit Datei, Zeile, Quelle und Tool:

```json
{
  "scope": { "type": "review", "family": "dependency-cve" },
  "groups": [
    {
      "id": "depcve-001",
      "title": "<package> <version> - <CVE/GHSA>",
      "priority": "P1|P2|P3",
      "findings": ["depcve-001"],
      "evidence": [
        {
          "file": "pyproject.toml|requirements.txt|package-lock.json|...",
          "line": null,
          "source": "tool:<tool-name>",
          "message": "<CVE/GHSA>, Severity, Fix-Version, Raw-Quelle raw/..."
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
/k-remediation k-playbook-local/results/dependency-cve/YYYY-MM-DD/review-triage.md
```

Remediation und Dependency-Upgrades sind ausdrücklich nicht Teil dieses Reviews.
