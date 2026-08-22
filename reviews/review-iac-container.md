---
name: review-iac-container
title: IaC and Container Assessment
interval-weeks: 4
scope-hint: Trivy-/Syft-/Grype-Ergebnisse für Containerfiles, IaC, Compose, Images und Filesystem; keine Remediation aus diesem Review heraus
handoff: /k-remediation
result-family: iac-container
audit:
  enabled: true
  title: IaC and Container Assessment
  resultRequired: true
  defaultResult: review-iac-container.md
review:
  enabled: true
---

# Review: IaC and Container Assessment

Erzeuge eine kuratierte, bewertete Liste aus IaC-, Container- und Filesystem-Security-Ergebnissen. Dieses Review nutzt host-lokal installierte Security-Tools und schreibt projektlokale Review-Artefakte unter `k-playbook-local/results/`.

## Zweck

- Container-, IaC- und OS-/Image-Risiken priorisiert sichtbar machen.
- Tool-Meldungen von projektspezifischer Review-Bewertung trennen.
- Fehlkonfigurationen, kritische Base-Image-CVEs und riskante Dockerfile-/Compose-Muster bewerten.
- `review-input.json` als Belegvertrag und `review-triage.md` als Handoff erzeugen.
- Keine Infrastruktur- oder Container-Änderungen durch dieses Review.

## Voraussetzungen

- Pfade kommen aus der Context-Ausgabe, die `/k-review` bereits geladen hat: `RESULTS_DIR` = `<local.dir>/results`.
- Lies `<playbook.dir>/scripts/security-tools.tsv` als kanonische Security-Tool-Matrix; dieses Review nutzt daraus `trivy`, `syft` und `grype`.
- Wenn der Context-Aufruf fehlschlägt: abbrechen und `/k-gui` empfehlen.
- Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein Ergebnisverzeichnis.
- Prüfe `trivy --version`, `syft --version` und `grype --version`.
- Wenn Pflicht-Tools fehlen: abbrechen und auf den Preflight verweisen:
  `bash <playbook.dir>/scripts/install-security-tools.sh` nennt den passenden
  Installationsbefehl.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook-local/results/iac-container/YYYY-MM-DD/`

Dateien:

- `review-input.json` - strukturierter Belegvertrag mit Scope, Gruppen, Evidence und Known-Decision-Coverage.
- `review-triage.md` - kuratierte Gesamtbewertung mit Bündeln, nicht gebündelten Findings und Handoff.
- `raw/trivy-fs.json` - Filesystem-/Dependency-/Secret-/Misconfig-Rohdaten, falls genutzt.
- `raw/trivy-config.json` - IaC-/Config-Rohdaten, falls separat genutzt.
- `raw/trivy-image-<name>.json` - Image-Rohdaten, falls Images gescannt wurden.
- `raw/syft-<target>.json` - SBOM-Rohdaten, falls ausgeführt.
- `raw/grype-<target>.json` - Grype-Rohdaten, falls ausgeführt.
- `run-metadata.json` - Befehle, Exit-Codes, Zeitpunkt, Scope, Tool-Versionen.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und dürfen nach dem Schreiben nicht gekürzt oder überschrieben werden.

## Ausführungsentscheidung

Frage vor Tool-Ausführung, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Keine neuen Scans.
- **IaC/Container-Scan ausführen**: Nur nach Bestätigung. Zeige vorher alle Befehle.
- **Nur Preflight**: Pfade, Tools, erkannte Docker-/IaC-Dateien, Images und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestätigung:

```bash
trivy fs --format json --output <result>/raw/trivy-fs.json <PROJECT_REPO_ROOT_DIR>
trivy config --format json --output <result>/raw/trivy-config.json <PROJECT_REPO_ROOT_DIR>
trivy image --format json --output <result>/raw/trivy-image-<name>.json <image-ref>
syft <target> -o json > <result>/raw/syft-<target>.json
grype <target> -o json > <result>/raw/grype-<target>.json
```

Image-Scans nur für explizit erkannte oder vom User bestätigte Image-Refs ausführen. Keine Images bauen, pullen oder pushen, außer der User bestätigt genau diesen Schritt.

## Bewertungskriterien

Priorität:

- P1: kritische Runtime-/Base-Image-CVEs mit produktionsnaher Exposition, Secrets in Image/Config, privilegierte Container, hostPath-/Docker-Socket-Mounts, Public-Exposure mit schwacher Auth.
- P2: hohe CVEs in Runtime-Layern, root User ohne Grund, fehlende Read-only-/Capability-Reduktion, unsichere IaC-Defaults.
- P3: Dev-only Images, Build-Stage-only CVEs, niedrigere Misconfig-Findings, fehlende Labels/Metadata ohne Sicherheitswirkung.

Berücksichtige:

- Runtime vs. Build-only.
- Produktivpfad vs. lokales Dev-Setup.
- Base-Image und Update-Pfad.
- Exponierte Ports, Volumes, Privilegien, Capabilities und Netzwerkmodus.
- Bekannte Entscheidungen aus `known-decisions.md`.

Review-Status im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - relevanter IaC-/Container-Befund.
- `context-needed` - Deployment-Kontext oder Image-Nutzung unklar.
- `likely-false-positive` - Tool-Mapping oder Zielkontext wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings deduplizieren, wenn Tool-Regel/CVE, Target, Layer/Datei und betroffene Komponente gleich sind.

## Review-Triage-Format

`review-triage.md` enthält mindestens die Pflichtabschnitte aus `commands/_audit/review-scan-triage.md`:

```markdown
# IaC and Container Assessment - YYYY-MM-DD

## Quellen

- Trivy FS/Config/Image: `raw/trivy-*.json`
- Syft: `raw/syft-*.json`
- Grype: `raw/grype-*.json`
- Quelle: `review-input.json`

## Kurzfazit

- Rohmeldungen: <n>
- Deduplizierte Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Container-/IaC-Punkt: <kurz>

## Bewertete Liste

| Prio | Finding-ID | Status | Typ | Target | Ort/Layer | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook-local/results/iac-container/YYYY-MM-DD/review-triage.md`
```

## Review-Input-Format

`review-input.json` enthält pro dedupliziertem Befund Evidence mit Datei, Zeile, Quelle und Tool:

```json
{
  "scope": { "type": "review", "family": "iac-container" },
  "groups": [
    {
      "id": "iaccont-001",
      "title": "<Typ> in <Target>",
      "priority": "P1|P2|P3",
      "findings": ["iaccont-001"],
      "evidence": [
        {
          "file": "<Dockerfile|compose|IaC-Datei|Image>",
          "line": null,
          "source": "tool:trivy|syft|grype",
          "message": "<Typ>, Ort/Layer, Regel/CVE, Raw-Quelle raw/..."
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
/k-remediation k-playbook-local/results/iac-container/YYYY-MM-DD/review-triage.md
```

Remediation und Infrastruktur-Änderungen sind ausdrücklich nicht Teil dieses Reviews.
