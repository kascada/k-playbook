---
name: review-secret-scanning
title: Secret-Scanning Assessment
interval-weeks: 4
scope-hint: gitleaks/trufflehog-Evidence aus `review-input.json`; keine Remediation, keine Secret-Rotation aus diesem Review heraus
handoff: /k-remediation
result-family: secret-scanning
audit:
  enabled: true
  title: Secret-Scanning Assessment
  resultRequired: true
  defaultResult: review-secret-scanning.md
  scope:
    tools: [gitleaks, trufflehog]
review:
  enabled: true
---

# Review: Secret-Scanning Assessment

Bewerte Secret-Scanning-Belege als fokussierte Perspektive auf `review-input.json`.
Dieses Rezept führt keine eigenen Scans aus und schreibt genau eine Ergebnisdatei im
aktuellen Lauf- oder Family-Ordner.

## Zweck

- Echte Secrets, produktive Credentials und Token-Leaks priorisiert sichtbar machen.
- Tool-Meldungen von projektspezifischer Review-Bewertung trennen.
- False Positives, Test-Fixtures und bekannte Entscheidungen nachvollziehbar markieren.
- Stabile Gruppen-IDs aus `review-input.json` erhalten.
- Keine Produktcode-Änderungen und keine Secret-Rotation durch dieses Review.

## Eingaben

Lies `review-input.json` aus dem vom aufrufenden Command genannten Ordner. Im
Audit-Laufmodell gilt der im `run.json`-Eintrag gespeicherte Scope:

```yaml
scope:
  tools: [gitleaks, trufflehog]
```

Filtere auf Evidence-Ebene:

- Eine Gruppe gehört zur Perspektive, wenn mindestens eine Evidence dieser Gruppe ein
  `evidence.tool` aus `scope.tools` trägt.
- Die Gruppen-ID bleibt unverändert; nicht neu deduplizieren, splitten oder umnummerieren.
- Bewerte nur Evidence aus `gitleaks` und `trufflehog` als primären Secret-Befund.
- Evidence anderer Tools bleibt als Kontext sichtbar und wird eindeutig als „außerhalb des
  Scopes" markiert.
- Leere Scope-Ergebnisse sind gültig; schreibe dann einen Report mit Status „keine scoped
  Findings".

## Bewertungskriterien

Priorität:

- P1: produktive Secrets, private Keys, Cloud-/Payment-/Database-Credentials, CI/CD-Tokens
  mit Schreibrechten.
- P2: plausibel aktive Tokens mit begrenztem Scope, interne Service-Credentials, Secrets in
  Git-Historie ohne sichtbare Rotation.
- P3: Test-Fixtures, Beispielwerte, low-confidence Findings, bereits rotierte oder
  offensichtlich deaktivierte Werte.

Review-Status im Perspektiven-Report:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - echter Secret-Fund.
- `context-needed` - Aktivität, Scope oder Rotation unklar.
- `likely-false-positive` - plausibler Fehlalarm.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings aus mehreren Tools inhaltlich zusammen bewerten, wenn Datei, Zeile,
Secret-Fingerprint oder semantischer Credential-Typ gleich sind. Die Dedupe-Entscheidung
aus `review-input.json` bleibt maßgeblich.

## Perspektiven-Report-Format

Schreibe die im `run.json`-Eintrag genannte Datei, standardmäßig
`review-secret-scanning.md`, direkt in den aktuellen Ordner:

```markdown
# Secret-Scanning Assessment - <lauf-oder-family-date>

Erzeugt: <RFC3339-Zeitstempel>
Quelle: `review-input.json`
Scope-Tools: `gitleaks`, `trufflehog`
Status: <bewertet | keine scoped Findings | technisch nicht bewertbar>

## Kurzfazit

- Scoped Gruppen: <n>
- Scoped Findings: <n>
- P1/P2/P3: <counts>
- Wichtigster Secret-Punkt: <kurz oder keiner>

## Bewertete Secret-Gruppen

| Prio | Gruppen-ID | Status | Typ | Ort | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|

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
klar erkennbar sein, welche Belege den Secret-Scope tragen und welche nur Kontext sind.

## Handoff

Nach Abschluss verweist das Review auf `review-triage.md` im selben Ordner. Remediation
und Secret-Rotation sind ausdrücklich nicht Teil dieses Reviews.

**Eigenständiger `/k-review`-Lauf nach dem Umbau.** Dieses Rezept läuft im Audit mit; dort
steckt sein Beleg schon im gemeinsamen Merge und eine Aggregation danach wäre doppelt.
Über `review.enabled: true` bleibt es daneben einzeln aufrufbar, und ein solcher Lauf legt
einen eigenen Family-Ordner **außerhalb** jedes Laufordners an:

```text
k-playbook-local/results/secret-scanning/<datum>/
```

Sein `review-triage.md` geht direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/secret-scanning/<datum>/review-triage.md
```

Es gibt dabei **keine Zusammenführung mit dem Audit-Lauf und keine Dedupe gegen dessen
Befunde**. `/k-remediation` nimmt genau eine Ergebnisdatei; ein Befund, den derselbe Tag
auch im Audit-Lauf trägt, steht dann in beiden Ergebnissen einmal. Wer beide Seiten
zusammen sehen will, nimmt den Audit-Lauf — dort und nur dort sitzt die Zusammenführung.
