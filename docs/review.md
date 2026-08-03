# /k-review

`/k-review` fuehrt globale oder projektlokale Review-Rezepte gegen das aktuelle Zielprojekt aus. Der Command ist der Einstieg fuer strukturierte Reviews ausserhalb eines konkreten GitHub-PRs.

## Zweck

`/k-review` trennt den generischen Review-Ablauf von den konkreten Review-Inhalten:

- Der Command loest Projektpfade aus `K-PLAYBOOK.yaml` auf.
- Review-Rezepte beschreiben nur Kriterien, Scope, Beispiele und Anti-Patterns.
- Projektlokale Rezepte koennen globale Rezepte mit gleichem Namen ueberlagern.
- `known-decisions.md` verhindert, dass bewusste Entscheidungen immer wieder als neue Findings auftauchen.

## Aufrufe

```text
/k-review
/k-review tech
/k-review codeql-security
/k-review secret-scanning
```

Ohne Argument zeigt der Command die verfuegbaren Review-Rezepte aus dem globalen Katalog und aus dem projektlokalen Reviews-Verzeichnis.

## Review-Arten

Interaktive Reviews moderieren Stelle fuer Stelle:

- Kandidaten suchen.
- Kompakte Fundliste zeigen.
- Bei unklaren Punkten Rueckfragen gesammelt stellen.
- Pro Stelle Vorschlag zeigen und auf Freigabe warten.
- Nur bestaetigte Aenderungen ausfuehren.

Report-Mode-Reviews erzeugen Ergebnisartefakte:

```text
<paths.reviews>/results/<family>/<YYYY-MM-DD>/assessment.md
<paths.reviews>/results/<family>/<YYYY-MM-DD>/findings.md
<paths.reviews>/results/<family>/<YYYY-MM-DD>/raw/
```

Report-Mode-Reviews ohne eigene `result-family`, z. B. `tech`, schreiben direkt eine Summary:

```text
<paths.reviews>/results/summary-YYYY-MM-DD.md
```

Diese Artefakte sind der Input fuer `/k-results` und `/k-remediation`.

## Typische Review-Familien

- `/k-review codeql-security`
- `/k-review k-check-security`
- `/k-review secret-scanning`
- `/k-review dependency-cve`
- `/k-review dependabot-alerts`
- `/k-review iac-container`
- `/k-review tech`
- `/k-review python-comment-hardspots`

## Handoff

Nach einem Report-Mode-Review nennt der Command den naechsten Handoff, typischerweise:

```text
/k-results
/k-remediation <paths.reviews>/results/summary-YYYY-MM-DD.md
/k-remediation <paths.reviews>/results/<family>/<YYYY-MM-DD>/assessment.md
```

Wenn aus Review-Ergebnissen Tasks entstehen, laufen sie danach durch den normalen Task-Flow:

```text
/k-review-loop
/k-run
```

## Abgrenzung

- `/k-review` bewertet oder erzeugt Findings, setzt groessere Remediation aber nicht direkt um.
- `/k-pr-review` ist fuer konkrete GitHub-PRs zustaendig.
- `/k-results` priorisiert mehrere vorhandene Result-Familien projektweit.
- `/k-remediation` plant die Abarbeitung und erzeugt je nach Policy Tasks oder kleine freigegebene Fixes.
