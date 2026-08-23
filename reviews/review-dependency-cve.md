---
name: review-dependency-cve
title: Dependency-CVE Assessment
interval-weeks: 4
scope-hint: Dependency-CVE-Evidence aus Manifesten, Lockfiles und Modulen; keine Upgrades oder Remediation aus diesem Review heraus
handoff: /k-remediation
result-family: dependency-cve
audit:
  enabled: true
  title: Dependency-CVE Assessment
  resultRequired: true
  defaultResult: review-dependency-cve.md
  scope:
    tools: [pip-audit, trivy, grype, osv-scanner, govulncheck]
review:
  enabled: true
---

# Review: Dependency-CVE Assessment

Bewerte Dependency-CVE-Belege als fokussierte Perspektive auf `review-input.json`.
Dieses Rezept führt keine eigenen Scanner aus und schreibt genau eine Ergebnisdatei im
aktuellen Lauf- oder Family-Ordner.

## Zweck

- Bekannte CVEs in direkten und relevanten transitiven Dependencies sichtbar machen.
- Tool-Severity von projektspezifischer Review-Priorität trennen.
- Exploitability, Reachability-Hinweise und Upgrade-Aufwand für die Triage erfassen.
- Stabile Gruppen-IDs aus `review-input.json` erhalten.
- Keine Dependency-Upgrades durch dieses Review.

## Eingaben

Lies `review-input.json` aus dem vom aufrufenden Command genannten Ordner. Im
Audit-Laufmodell gilt der im `run.json`-Eintrag gespeicherte Scope:

```yaml
scope:
  tools: [pip-audit, trivy, grype, osv-scanner, govulncheck]
```

Filtere auf Evidence-Ebene:

- Eine Gruppe gehört zur Perspektive, wenn mindestens eine Evidence dieser Gruppe ein
  `evidence.tool` aus `scope.tools` trägt.
- Die Gruppen-ID bleibt unverändert; nicht neu deduplizieren, splitten oder umnummerieren.
- Bewerte nur scoped Evidence als primären Dependency-Befund.
- Evidence anderer Tools bleibt als Kontext sichtbar und wird eindeutig als „außerhalb des
  Scopes" markiert.
- Leere Scope-Ergebnisse sind gültig; schreibe dann einen Report mit Status „keine scoped
  Findings".

## Bewertungskriterien

Tool-Severity allein ist nicht die Review-Priorität. Priorisiere nach:

- P1: kritisch/hoch, remote ausnutzbar, produktionsnah, direkt erreichbar oder in
  Auth-/Parsing-/Network-Pfaden.
- P2: hohe oder mittlere CVEs in direkt genutzten Dependencies oder in zentralen
  Runtime-Komponenten.
- P3: transitive CVEs ohne sichtbare Nutzung, Dev-/Test-only Dependencies, unklare oder
  niedrige Ausnutzbarkeit.

Berücksichtige:

- Direkt vs. transitiv.
- Manifest-/Lockfile-Quelle.
- Fix-Version und Upgrade-Pfad.
- Ob die betroffene Komponente im Produktpfad genutzt wird.
- Bekannte Entscheidungen aus `known-decisions.md` im Merge-Beleg.

Review-Status im Perspektiven-Report:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - CVE betrifft eine relevante Dependency im Produktkontext.
- `context-needed` - Reachability oder Laufzeitpfad unklar.
- `likely-false-positive` - Tool-Mapping oder Umgebung wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings inhaltlich zusammen bewerten, wenn CVE-ID, Package, Version und
Manifest-/Lockfile-Quelle gleich sind. Die Dedupe-Entscheidung aus `review-input.json`
bleibt maßgeblich.

## Perspektiven-Report-Format

Schreibe die im `run.json`-Eintrag genannte Datei, standardmäßig
`review-dependency-cve.md`, direkt in den aktuellen Ordner:

```markdown
# Dependency-CVE Assessment - <lauf-oder-family-date>

Erzeugt: <RFC3339-Zeitstempel>
Quelle: `review-input.json`
Scope-Tools: `pip-audit`, `trivy`, `grype`, `osv-scanner`, `govulncheck`
Status: <bewertet | keine scoped Findings | technisch nicht bewertbar>

## Kurzfazit

- Scoped Gruppen: <n>
- Scoped Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Upgrade-/Patch-Punkt: <kurz oder keiner>

## Bewertete Dependency-Gruppen

| Prio | Gruppen-ID | Status | Package | CVE/GHSA/OSV | Betroffene Version | Fix-Version | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Evidence außerhalb des Scopes

| Gruppen-ID | Tool | Grund Für Kontext |
|---|---|---|

## Deckung aus known-decisions

| Decision-ID | Betroffene Gruppen | Wirkung |
|---|---|---|

## Handoff

`/k-remediation <aktueller-ordner>/review-triage.md`
```

Der Report nennt alle betrachteten Gruppen-IDs. Bei Gruppen mit gemischter Evidence muss
klar erkennbar sein, welche Belege den Dependency-Scope tragen und welche nur Kontext
sind.

## Handoff

Nach Abschluss verweist das Review auf `review-triage.md` im selben Ordner. Remediation
und Dependency-Upgrades sind ausdrücklich nicht Teil dieses Reviews.
