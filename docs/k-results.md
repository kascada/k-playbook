# /k-results

`/k-results` erzeugt aus vorhandenen Review-Result-Familien eine projektweite, priorisierte Summary. Der Command ist der Zwischenschritt zwischen Report-Reviews und Remediation.

## Zweck

`/k-results` beantwortet die Frage: Was ist ueber alle Review-Familien hinweg wirklich wichtig und was sollte zuerst abgearbeitet werden?

Der Command:

- liest vorhandene `assessment.md`- und `findings.md`-Dateien unter `<paths.reviews>/results/`.
- startet keine Scanner.
- veraendert keine Raw-Artefakte.
- dedupliziert Findings ueber Familien hinweg.
- beruecksichtigt `known-decisions.md` und vorhandene Tasks, soweit vorhanden.
- schreibt eine Summary unter `<paths.reviews>/results/summary-YYYY-MM-DD.md`.

## Aufrufe

```text
/k-results
/k-results latest
/k-results 2026-08-03
```

## Input

Erwartete Struktur:

```text
<paths.reviews>/results/<family>/<date>/assessment.md
<paths.reviews>/results/<family>/<date>/findings.md
```

Bekannte Familien sind zum Beispiel:

- `codeql`
- `k-check`
- `secret-scanning`
- `dependency-cve`
- `dependabot-alerts`
- `iac-container`

## Output

Zielartefakt:

```text
<paths.reviews>/results/summary-YYYY-MM-DD.md
```

Die Summary enthaelt:

- verwendete Quellen.
- priorisierte Uebersicht.
- Dedupe-Entscheidungen.
- konkrete Empfehlungen.
- Handoff fuer Remediation.

## Typischer Ablauf

```text
/k-review codeql-security
/k-review k-check-security
/k-review secret-scanning
/k-review dependency-cve
/k-review dependabot-alerts
/k-review iac-container
/k-results
/k-remediation k-playbook/reviews/results/summary-YYYY-MM-DD.md
```

Wenn `/k-remediation` aus der Summary Tasks erzeugt, folgen danach `/k-review-loop` und `/k-run`.

## Abgrenzung

- `/k-results` ist read-mostly und priorisiert vorhandene Ergebnisse.
- `/k-results` erzeugt keine Tasks.
- `/k-results` fuehrt keine Remediation aus.
- Einzelne Result-Dateien werden direkt an `/k-remediation` uebergeben, nicht an `/k-results`.
