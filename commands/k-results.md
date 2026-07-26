---
description: Build a project-wide prioritized security results summary from existing review result families. Reads reviews/results/*/*/{assessment,findings}.md, deduplicates and ranks findings, then writes reviews/results/summary-YYYY-MM-DD.md. Does not run scanners or remediation.
argument-hint: [YYYY-MM-DD|latest]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-results

Erzeuge eine projektweite, priorisierte Ergebnis-Zusammenfassung aus vorhandenen Review-Result-Familien.

`/k-results` ist der Zwischenschritt zwischen `/k-review <family>` und `/k-remediation`. Der Command startet keine Scanner, fuehrt keine Remediation aus und veraendert keine Raw-Artefakte. Er liest vorhandene `assessment.md`/`findings.md`-Dateien unter `reviews/results/` und schreibt eine einzelne priorisierte Summary.

## Zielartefakt

```text
<PROJECT_REVIEWS_DIR>/results/summary-YYYY-MM-DD.md
```

Dieses Artefakt ist der bevorzugte Handoff fuer Remediation:

```text
/k-remediation <reviews>/results/summary-YYYY-MM-DD.md
```

## Schritt 1 — Pfade aufloesen

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

For this command, resolve:

- `reviews:` -> `PROJECT_REVIEWS_DIR`.
- `tasks:` -> `TASKS_DIR`, optional aber hilfreich fuer existierende Remediation-Tasks.

Also require `base:` from `K-PLAYBOOK.MD`; use it only as validation metadata, not to infer `reviews:` or `tasks:`.

Command-specific policy:

- Wenn `K-PLAYBOOK.MD` fehlt: abbrechen und `/k-setup` nennen.
- Wenn `base:` fehlt: abbrechen und `/k-setup` nennen.
- Wenn `reviews:` fehlt, inaktiv ist oder das Verzeichnis nicht existiert: abbrechen; Results brauchen ein projektlokales `reviews:`-Ziel.
- Wenn `tasks:` fehlt oder nicht existiert: warnen, aber fortfahren; dann koennen existierende Tasks nicht abgeglichen werden.

Abgeleitete Pfade:

- `RESULTS_DIR = <PROJECT_REVIEWS_DIR>/results`
- `KNOWN_DECISIONS = <PROJECT_REVIEWS_DIR>/known-decisions.md`
- `LOG_FILE = <PROJECT_REVIEWS_DIR>/log.md`

## Schritt 2 — Zieldatum bestimmen

Wenn `$ARGUMENTS` leer oder `latest` ist:

- Verwende das heutige Datum `YYYY-MM-DD` fuer die Summary-Datei.
- Lies alle vorhandenen Result-Familien unter `RESULTS_DIR`, nicht nur heutige, aber bevorzuge pro Familie das neueste `assessment.md`.

Wenn `$ARGUMENTS` wie `YYYY-MM-DD` aussieht:

- Nutze dieses Datum fuer die Summary-Datei.
- Lies weiterhin pro Familie das neueste Assessment, ausser der User explizit einen anderen Scope angibt.

Wenn `$ARGUMENTS` eine Datei ist:

- Abbrechen. `/k-results` akzeptiert keine Einzeldatei als Primaerinput; dafuer `/k-remediation` verwenden.

## Schritt 3 — Result-Familien finden

Suche unter:

```text
<RESULTS_DIR>/*/*/assessment.md
```

Erkenne Family und Date aus dem Pfad:

```text
<RESULTS_DIR>/<family>/<date>/assessment.md
```

Erwarte optional im selben Verzeichnis:

- `findings.md`
- `raw/`
- `run-metadata.json` oder andere `run-metadata*.json`

Ignoriere bestehende `summary-*.md` als Input, ausser der User fordert eine Vergleichszusammenfassung explizit an.

Wenn mehrere Dates pro Family existieren:

- Standard: neuestes Date pro Family verwenden.
- Wenn ein aelteres Date benutzt wird, in der Summary klar nennen.

Bekannte Familien und Standardreihenfolge:

1. `codeql`
2. `k-check`
3. `secret-scanning`
4. `dependency-cve`
5. `iac-container`
6. weitere Familien alphabetisch

## Schritt 4 — Kontext laden

Lade, falls vorhanden:

- `KNOWN_DECISIONS`: Findings, die klar gedeckt sind, als `accepted` markieren oder in P3 verschieben.
- `TASKS_DIR/*.md` und `TASKS_DIR/done/*.md`: bestehende Tasks erkennen, um doppelte Task-Erzeugung zu vermeiden.
- Alle `assessment.md` und `findings.md` der ausgewaehlten Familien.

Wenn `known-decisions.md` leer ist, das in der Summary nennen.

## Schritt 5 — Findings extrahieren

Extrahiere aus `assessment.md`:

- Kurzfazit / wichtigste Befunde.
- Bewertete Tabellen.
- Handoff-Pfade.
- Sofortige Triage-Reihenfolge.

Extrahiere aus `findings.md`:

- Heading-ID, z. B. `### secret-001`.
- `Status`.
- `Prioritaet`.
- `Quelle` / `Tool(s)` / `Package` / `Typ`.
- `Ort` / `Target` / `CVE/GHSA` / `Message`.
- `Raw-Quelle`.
- `Review-Bewertung`, `Triage-Notiz`, `Remediation`.

Statusmodell:

- Remediation-relevant: `open`, `confirmed`, `context-needed`.
- Nur nach expliziter Auswahl remediation-relevant: `likely-false-positive`.
- Nicht remediation-relevant: `accepted`, `fixed`.

## Schritt 6 — Deduplizieren und clustern

Cluster Findings ueber Familien hinweg nach Thema, nicht nur nach identischer ID.

Typische Dedupe-Regeln:

- Secret-Funde aus `secret-scanning` und `k-check` zusammenfuehren.
- CVE-Funde aus `dependency-cve` und `iac-container` nicht doppelt remediieren; `dependency-cve` ist primaere Quelle, `iac-container` liefert SBOM/Image-Kontext.
- Logging-Funde aus CodeQL und k-check zusammenfuehren, wenn gleicher Pfad oder gleicher Logging-Kontrakt betroffen ist.
- Dockerfile-/Image-Funde aus `iac-container` separat halten, auch wenn Secret-/Config-Themen in anderen Familien auftauchen.
- False-positive-/Fixture-/Tooling-Artefakte als P3-Gruppe zusammenfassen.

## Schritt 7 — Priorisieren

Prioritaet projektweit vergeben, nicht blind Tool-Severity uebernehmen.

P1:

- echte produktive Secrets oder unklare Rotation produktiver Secrets.
- bestaetigbare SSRF/RCE/Auth/JWT/SQLi/critical Runtime CVEs.
- Secrets in Image-Layern oder Deployment-Konfiguration.
- direkte Provider-/Exception-Leaks mit Secret-/Config-Risiko.

P1/P2:

- hochrelevante Runtime-CVEs ohne final bestaetigte Reachability.
- user-facing Authz-/Ownership-Risiken.
- sensitive Logging-Funde mit Provider-/User-Content-Risiko.

P2:

- High CVEs in zentralen Runtime-/SDK-Komponenten.
- IaC-/Helm-Abdeckungsluecken im Production-Target.
- Operational-Logging-/Monitoring-Kontraktverletzungen.

P2/P3:

- Build-/Dev-Toolchain-CVEs.
- Defense-in-depth-Funde mit begrenztem Blast Radius.

P3:

- Scanner-Noise, Test-Fixtures, vendored Artefakte, Tooling-Kontext.
- Hardening ohne akute Exploitability.

## Schritt 8 — Summary schreiben

Schreibe oder aktualisiere:

```text
<RESULTS_DIR>/summary-YYYY-MM-DD.md
```

Wenn die Datei existiert:

- Nicht blind ueberschreiben.
- Entweder nach Bestaetigung aktualisieren oder eindeutigen Namen vorschlagen, z. B. `summary-YYYY-MM-DD-2.md`.

Pflichtstruktur:

```markdown
# Security Results Summary - YYYY-MM-DD

Projekt: `<name>`

## Quellen

- <family>: `reviews/results/<family>/<date>/assessment.md`
- Known Decisions: `reviews/known-decisions.md` (<Status>)

## Priorisierte Uebersicht

| Prio | Thema | Quelle(n) | Finding-ID(s) | Empfehlung | Status |
|---|---|---|---|---|---|

## P1-01 <Titel>

Kurzbeschreibung.

Empfehlung: konkreter naechster Schritt.

Quellen:
- `reviews/results/<family>/<date>/assessment.md`
- `reviews/results/<family>/<date>/findings.md#<finding-id>`

Was man zum Loesen braucht:
- betroffene Datei/Zeile
- relevante Tests/Smokes
- Akzeptanzkriterium

## Empfohlene Remediation-Reihenfolge

1. ...

## Handoff

`/k-remediation <reviews>/results/summary-YYYY-MM-DD.md`
```

Schreibe knapp, aber loesungsfaehig. Jede Top-Gruppe braucht:

- Beschreibung in einem Absatz.
- Empfehlung in einem Absatz.
- Quellenliste.
- Was man zum Loesen braucht.

## Schritt 9 — Review-Log aktualisieren

Wenn `LOG_FILE` existiert:

- Sektion `## Security Results Summary` sicherstellen.
- `Letzter Lauf` auf heute setzen.
- Eine Protokollzeile anhaengen:

```markdown
| YYYY-MM-DD | results-summary | alle Result-Familien | <N> priorisierte Themen -> `k-playbook/reviews/results/summary-YYYY-MM-DD.md`. Handoff: `/k-remediation ...` |
```

Wenn `LOG_FILE` fehlt, zeige die Log-Zeile im Abschluss, aber lege keine Ersatzdatei ausserhalb des registrierten `reviews:`-Pfads an.

## Schritt 10 — Abschluss

Berichte:

- Summary-Pfad.
- Welche Result-Familien verwendet wurden.
- Anzahl priorisierter Themen.
- Top 5 Themen.
- Handoff-Befehl fuer `/k-remediation`.

Keine Remediation starten.
