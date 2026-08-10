---
name: review-iac-container
title: IaC and Container Assessment
interval-weeks: 4
scope-hint: Trivy-/Syft-/Grype-Ergebnisse fuer Containerfiles, IaC, Compose, Images und Filesystem; keine Remediation aus diesem Review heraus
handoff: /k-remediation
result-family: iac-container
---

# Review: IaC and Container Assessment

Erzeuge eine kuratierte, bewertete Liste aus IaC-, Container- und Filesystem-Security-Ergebnissen. Dieses Review nutzt host-lokal installierte Security-Tools und schreibt projektlokale Review-Artefakte unter `k-playbook/reviews/`.

## Zweck

- Container-, IaC- und OS-/Image-Risiken priorisiert sichtbar machen.
- Tool-Meldungen von projektspezifischer Review-Bewertung trennen.
- Fehlkonfigurationen, kritische Base-Image-CVEs und riskante Dockerfile-/Compose-Muster bewerten.
- Ein `assessment.md` mit priorisierter Liste und ein statusfaehiges `findings.md` erzeugen.
- Keine Infrastruktur- oder Container-Aenderungen durch dieses Review.

## Voraussetzungen

- Lies und verwende `<DIST_DIR>/commands/_shared/path-resolution.md`.
- Lies `<DIST_DIR>/security-tools.tsv` als kanonische Security-Tool-Matrix; dieses Review nutzt daraus `trivy`, `syft` und `grype`.
- Wenn die Discovery kein `K-PLAYBOOK.yaml` findet: abbrechen und `k-playbook-installer init` empfehlen.
- Wenn der konfigurierte `paths.reviews` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein lokales `reviews`-Ziel.
- Pruefe `trivy --version`, `syft --version` und `grype --version`.
- Wenn Pflicht-Tools fehlen: abbrechen und auf den Preflight verweisen:
  `bash <PLAYBOOK_DIR>/scripts/install-security-tools.sh` nennt den passenden
  Installationsbefehl.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook/reviews/results/iac-container/YYYY-MM-DD/`

Dateien:

- `assessment.md` - kuratierte Gesamtbewertung mit priorisierter Liste.
- `findings.md` - vollstaendiges, statusfaehiges Arbeitsregister.
- `raw/trivy-fs.json` - Filesystem-/Dependency-/Secret-/Misconfig-Rohdaten, falls genutzt.
- `raw/trivy-config.json` - IaC-/Config-Rohdaten, falls separat genutzt.
- `raw/trivy-image-<name>.json` - Image-Rohdaten, falls Images gescannt wurden.
- `raw/syft-<target>.json` - SBOM-Rohdaten, falls ausgefuehrt.
- `raw/grype-<target>.json` - Grype-Rohdaten, falls ausgefuehrt.
- `run-metadata.json` - Befehle, Exit-Codes, Zeitpunkt, Scope, Tool-Versionen.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und duerfen nach dem Schreiben nicht gekuerzt oder ueberschrieben werden.

## Ausfuehrungsentscheidung

Frage vor Tool-Ausfuehrung, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Keine neuen Scans.
- **IaC/Container-Scan ausfuehren**: Nur nach Bestaetigung. Zeige vorher alle Befehle.
- **Nur Preflight**: Pfade, Tools, erkannte Docker-/IaC-Dateien, Images und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestaetigung:

```bash
trivy fs --format json --output <result>/raw/trivy-fs.json <PROJECT_REPO_ROOT_DIR>
trivy config --format json --output <result>/raw/trivy-config.json <PROJECT_REPO_ROOT_DIR>
trivy image --format json --output <result>/raw/trivy-image-<name>.json <image-ref>
syft <target> -o json > <result>/raw/syft-<target>.json
grype <target> -o json > <result>/raw/grype-<target>.json
```

Image-Scans nur fuer explizit erkannte oder vom User bestaetigte Image-Refs ausfuehren. Keine Images bauen, pullen oder pushen, ausser der User bestaetigt genau diesen Schritt.

## Bewertungskriterien

Prioritaet:

- P1: kritische Runtime-/Base-Image-CVEs mit produktionsnaher Exposition, Secrets in Image/Config, privilegierte Container, hostPath-/Docker-Socket-Mounts, Public-Exposure mit schwacher Auth.
- P2: hohe CVEs in Runtime-Layern, root User ohne Grund, fehlende Read-only-/Capability-Reduktion, unsichere IaC-Defaults.
- P3: Dev-only Images, Build-Stage-only CVEs, niedrigere Misconfig-Findings, fehlende Labels/Metadata ohne Sicherheitswirkung.

Beruecksichtige:

- Runtime vs. Build-only.
- Produktivpfad vs. lokales Dev-Setup.
- Base-Image und Update-Pfad.
- Exponierte Ports, Volumes, Privilegien, Capabilities und Netzwerkmodus.
- Bekannte Entscheidungen aus `known-decisions.md`.

Review-Status in `findings.md`:

- `open` - neu oder noch nicht geprueft.
- `confirmed` - relevanter IaC-/Container-Befund.
- `context-needed` - Deployment-Kontext oder Image-Nutzung unklar.
- `likely-false-positive` - Tool-Mapping oder Zielkontext wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings deduplizieren, wenn Tool-Regel/CVE, Target, Layer/Datei und betroffene Komponente gleich sind.

## Assessment-Format

`assessment.md` enthaelt mindestens:

```markdown
# IaC and Container Assessment - YYYY-MM-DD

## Quellen

- Trivy FS/Config/Image: `raw/trivy-*.json`
- Syft: `raw/syft-*.json`
- Grype: `raw/grype-*.json`
- Finding-Register: `findings.md`

## Kurzfazit

- Rohmeldungen: <n>
- Deduplizierte Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Container-/IaC-Punkt: <kurz>

## Bewertete Liste

| Prio | Finding-ID | Status | Typ | Target | Ort/Layer | Bewertung | Naechster Schritt |
|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook/reviews/results/iac-container/YYYY-MM-DD/assessment.md`
```

## Finding-Register-Format

`findings.md` enthaelt pro dedupliziertem Befund:

```markdown
### iaccont-001

- Status: `open`
- Prioritaet: `P1|P2|P3`
- Typ: `cve|misconfig|secret|license|sbom`
- Tool(s): `trivy`, `syft`, `grype`
- Target: ...
- Ort/Layer: ...
- Regel/CVE: ...
- Raw-Quelle: `raw/...`
- Review-Bewertung: _offen_
- Deployment-Kontext: _offen_
- Remediation: _offen_
```

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation k-playbook/reviews/results/iac-container/YYYY-MM-DD/assessment.md
```

Remediation und Infrastruktur-Aenderungen sind ausdruecklich nicht Teil dieses Reviews.
