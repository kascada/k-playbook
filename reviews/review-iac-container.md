---
name: review-iac-container
title: IaC and Container Assessment
interval-weeks: 4
scope-hint: Trivy-/Syft-/Grype-Evidence für Containerfiles, IaC, Images und Filesystem; keine Remediation aus diesem Review heraus
handoff: /k-remediation
result-family: iac-container
audit:
  enabled: true
  title: IaC and Container Assessment
  resultRequired: true
  defaultResult: review-iac-container.md
  scope:
    tools: [trivy, syft, grype]
review:
  enabled: true
---

# Review: IaC and Container Assessment

Bewerte IaC-, Container- und Filesystem-Security-Belege als fokussierte Perspektive auf
`review-input.json`. Dieses Rezept führt keine eigenen Scans aus und schreibt genau eine
Ergebnisdatei im aktuellen Lauf- oder Family-Ordner.

## Zweck

- Container-, IaC- und OS-/Image-Risiken priorisiert sichtbar machen.
- Tool-Meldungen von projektspezifischer Review-Bewertung trennen.
- Fehlkonfigurationen, kritische Base-Image-CVEs und riskante Dockerfile-/Compose-Muster
  bewerten.
- Stabile Gruppen-IDs aus `review-input.json` erhalten.
- Keine Infrastruktur- oder Container-Änderungen durch dieses Review.

## Eingaben

Lies `review-input.json` aus dem vom aufrufenden Command genannten Ordner. Im
Audit-Laufmodell gilt der im `run.json`-Eintrag gespeicherte Scope:

```yaml
scope:
  tools: [trivy, syft, grype]
```

Filtere auf Evidence-Ebene:

- Eine Gruppe gehört zur Perspektive, wenn mindestens eine Evidence dieser Gruppe ein
  `evidence.tool` aus `scope.tools` trägt.
- Die Gruppen-ID bleibt unverändert; nicht neu deduplizieren, splitten oder umnummerieren.
- Bewerte nur scoped Evidence als primären IaC-/Container-Befund.
- Evidence anderer Tools bleibt als Kontext sichtbar und wird eindeutig als „außerhalb des
  Scopes" markiert.
- Leere Scope-Ergebnisse sind gültig; schreibe dann einen Report mit Status „keine scoped
  Findings".

## Bewertungskriterien

Priorität:

- P1: kritische Runtime-/Base-Image-CVEs mit produktionsnaher Exposition, Secrets in
  Image/Config, privilegierte Container, hostPath-/Docker-Socket-Mounts, Public-Exposure
  mit schwacher Auth.
- P2: hohe CVEs in Runtime-Layern, root User ohne Grund, fehlende
  Read-only-/Capability-Reduktion, unsichere IaC-Defaults.
- P3: Dev-only Images, Build-Stage-only CVEs, niedrigere Misconfig-Findings, fehlende
  Labels/Metadata ohne Sicherheitswirkung.

Berücksichtige:

- Runtime vs. Build-only.
- Produktivpfad vs. lokales Dev-Setup.
- Base-Image und Update-Pfad.
- Exponierte Ports, Volumes, Privilegien, Capabilities und Netzwerkmodus.
- Bekannte Entscheidungen aus `known-decisions.md` im Merge-Beleg.

Review-Status im Perspektiven-Report:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - relevanter IaC-/Container-Befund.
- `context-needed` - Deployment-Kontext oder Image-Nutzung unklar.
- `likely-false-positive` - Tool-Mapping oder Zielkontext wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings inhaltlich zusammen bewerten, wenn Tool-Regel/CVE, Target, Layer/Datei und
betroffene Komponente gleich sind. Die Dedupe-Entscheidung aus `review-input.json` bleibt
maßgeblich.

## Perspektiven-Report-Format

Schreibe die im `run.json`-Eintrag genannte Datei, standardmäßig
`review-iac-container.md`, direkt in den aktuellen Ordner:

```markdown
# IaC and Container Assessment - <lauf-oder-family-date>

Erzeugt: <RFC3339-Zeitstempel>
Quelle: `review-input.json`
Scope-Tools: `trivy`, `syft`, `grype`
Status: <bewertet | keine scoped Findings | technisch nicht bewertbar>

## Kurzfazit

- Scoped Gruppen: <n>
- Scoped Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Container-/IaC-Punkt: <kurz oder keiner>

## Bewertete IaC-/Container-Gruppen

| Prio | Gruppen-ID | Status | Typ | Target | Ort/Layer | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|---|

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
klar erkennbar sein, welche Belege den IaC-/Container-Scope tragen und welche nur Kontext
sind.

## Handoff

Nach Abschluss verweist das Review auf `review-triage.md` im selben Ordner. Remediation
und Infrastruktur-Änderungen sind ausdrücklich nicht Teil dieses Reviews.

**Eigenständiger `/k-review`-Lauf nach dem Umbau.** Dieses Rezept läuft im Audit mit; dort
steckt sein Beleg schon im gemeinsamen Merge und eine Aggregation danach wäre doppelt.
Über `review.enabled: true` bleibt es daneben einzeln aufrufbar, und ein solcher Lauf legt
einen eigenen Family-Ordner **außerhalb** jedes Laufordners an:

```text
k-playbook-local/results/iac-container/<datum>/
```

Sein `review-triage.md` geht direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/iac-container/<datum>/review-triage.md
```

Es gibt dabei **keine Zusammenführung mit dem Audit-Lauf und keine Dedupe gegen dessen
Befunde**. `/k-remediation` nimmt genau eine Ergebnisdatei; ein Befund, den derselbe Tag
auch im Audit-Lauf trägt, steht dann in beiden Ergebnissen einmal. Wer beide Seiten
zusammen sehen will, nimmt den Audit-Lauf — dort und nur dort sitzt die Zusammenführung.
