---
name: review-dependency-cve
title: Dependency-CVE Assessment
interval-weeks: 4
scope-hint: Dependency-CVE-Ergebnisse aus Manifesten und Lockfiles; keine Upgrades oder Remediation aus diesem Review heraus
handoff: /k-remediation
result-family: dependency-cve
---

# Review: Dependency-CVE Assessment

Erzeuge eine kuratierte, bewertete Liste aus Dependency-CVE-Scans. Dieses Review nutzt host-lokal installierte Tools aus `/k-install-security-tools` und schreibt projektlokale Review-Artefakte unter `reviews:`.

## Zweck

- Bekannte CVEs in direkten und relevanten transitiven Dependencies sichtbar machen.
- Tool-Severity von projektspezifischer Review-Prioritaet trennen.
- Exploitability, Reachability-Hinweise und Upgrade-Aufwand fuer die Triage erfassen.
- Ein `assessment.md` mit priorisierter Liste und ein statusfaehiges `findings.md` erzeugen.
- Keine Dependency-Upgrades durch dieses Review.

## Voraussetzungen

- Lies und verwende `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.
- Wenn `K-PLAYBOOK.MD` fehlt: abbrechen und `/k-setup` nennen.
- Wenn `base:` fehlt: abbrechen und `/k-setup` nennen.
- Wenn `reviews:` fehlt oder inaktiv ist: abbrechen; dieses Review braucht ein lokales `reviews:`-Ziel.
- Pruefe `pip-audit --version`, `trivy --version` und `grype --version`.
- Wenn Pflicht-Tools fehlen: abbrechen und `/k-install-security-tools --install missing` nennen.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`<reviews>/results/dependency-cve/YYYY-MM-DD/`

Dateien:

- `assessment.md` - kuratierte Gesamtbewertung mit priorisierter Liste.
- `findings.md` - vollstaendiges, statusfaehiges Arbeitsregister.
- `raw/pip-audit.json` - Python-CVE-Rohdaten, falls anwendbar.
- `raw/trivy-fs.json` - Trivy-Filesystem-/Dependency-Rohdaten.
- `raw/grype.json` - Grype-Rohdaten, falls ausgefuehrt.
- `run-metadata.json` - Befehle, Exit-Codes, Zeitpunkt, Scope, Tool-Versionen.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und duerfen nach dem Schreiben nicht gekuerzt oder ueberschrieben werden.

## Ausfuehrungsentscheidung

Frage vor Tool-Ausfuehrung, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Keine neuen Scans.
- **Dependency-CVE-Scan ausfuehren**: Nur nach Bestaetigung. Zeige vorher alle Befehle.
- **Nur Preflight**: Pfade, Tools, erkannte Manifest-/Lockfiles und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestaetigung:

```bash
pip-audit --format json --output <result>/raw/pip-audit.json --path <TARGET_DIR>
trivy fs --format json --output <result>/raw/trivy-fs.json <TARGET_DIR>
grype dir:<TARGET_DIR> -o json > <result>/raw/grype.json
```

`pip-audit` nur ausfuehren, wenn Python-Manifeste oder Python-Lockfiles gefunden wurden. `grype` gehoert zur Pflicht-Toolchain; ob der konkrete Grype-Scan in diesem Review sinnvoll ist, haengt vom Scope und den vorhandenen Manifests/SBOMs ab.

## Bewertungskriterien

Tool-Severity allein ist nicht die Review-Prioritaet. Priorisiere nach:

- P1: kritisch/hoch, remote ausnutzbar, produktionsnah, direkt erreichbar oder in Auth/Parsing/Network-Pfad.
- P2: hohe oder mittlere CVEs in direkt genutzten Dependencies oder in zentralen Runtime-Komponenten.
- P3: transitive CVEs ohne sichtbare Nutzung, Dev-/Test-only Dependencies, unklare oder niedrige Ausnutzbarkeit.

Beruecksichtige:

- Direkt vs. transitiv.
- Manifest-/Lockfile-Quelle.
- Fix-Version und Upgrade-Pfad.
- Ob die betroffene Komponente im Produktpfad genutzt wird.
- Bekannte Entscheidungen aus `known-decisions.md`.

Review-Status in `findings.md`:

- `open` - neu oder noch nicht geprueft.
- `confirmed` - CVE betrifft eine relevante Dependency im Produktkontext.
- `context-needed` - Reachability oder Laufzeitpfad unklar.
- `likely-false-positive` - Tool-Mapping oder Umgebung wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings deduplizieren, wenn CVE-ID, Package, Version und Manifest-/Lockfile-Quelle gleich sind.

## Assessment-Format

`assessment.md` enthaelt mindestens:

```markdown
# Dependency-CVE Assessment - YYYY-MM-DD

## Quellen

- pip-audit: `raw/pip-audit.json`
- Trivy: `raw/trivy-fs.json`
- Grype: `raw/grype.json`
- Finding-Register: `findings.md`

## Kurzfazit

- Rohmeldungen: <n>
- Deduplizierte CVE-Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Upgrade-/Patch-Punkt: <kurz>

## Bewertete Liste

| Prio | Finding-ID | Status | Package | CVE | Betroffene Version | Fix-Version | Bewertung | Naechster Schritt |
|---|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation <reviews>/results/dependency-cve/YYYY-MM-DD/assessment.md`
```

## Finding-Register-Format

`findings.md` enthaelt pro dedupliziertem Befund:

```markdown
### depcve-001

- Status: `open`
- Prioritaet: `P1|P2|P3`
- Package: ...
- Version: ...
- CVE/GHSA: ...
- Severity: ...
- Fix-Version: ...
- Quelle: `pyproject.toml`, `requirements.txt`, `package-lock.json`, ...
- Raw-Quelle: `raw/...`
- Review-Bewertung: _offen_
- Upgrade-Hinweis: _offen_
- Remediation: _offen_
```

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation <reviews>/results/dependency-cve/YYYY-MM-DD/assessment.md
```

Remediation und Dependency-Upgrades sind ausdruecklich nicht Teil dieses Reviews.
