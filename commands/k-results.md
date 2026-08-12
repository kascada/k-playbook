---
description: Build a project-wide prioritized security results summary from existing review result families. Reads k-playbook-local/results/*/*/{assessment,findings}.md, deduplicates and ranks findings, then writes k-playbook-local/results/summary-YYYY-MM-DD.md. Does not run scanners or remediation.
argument-hint: [YYYY-MM-DD|latest]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-results

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Erzeuge eine projektweite, priorisierte Ergebnis-Zusammenfassung aus vorhandenen Review-Result-Familien.

`/k-results` ist der Zwischenschritt zwischen `/k-review <family>` und `/k-remediation`. Der Command startet keine Scanner, führt keine Remediation aus und verändert keine Raw-Artefakte. Er liest vorhandene `assessment.md`/`findings.md`-Dateien unter `k-playbook-local/results/` und schreibt eine einzelne priorisierte Summary.

## Zielartefakt

```text
k-playbook-local/results/summary-YYYY-MM-DD.md
```

Dieses Artefakt ist der bevorzugte Handoff für Remediation:

```text
/k-remediation k-playbook-local/results/summary-YYYY-MM-DD.md
```

## Schritt 1 — Pfade auflösen

Aus der Context-Ausgabe:

- `RESULTS_DIR = <local.dir>/results`
- `KNOWN_DECISIONS = <RESULTS_DIR>/known-decisions.md`
- `LOG_FILE = <RESULTS_DIR>/log.md`
- `TASKS_DIR = <local.dir>/tasks`; optional aber hilfreich für existierende Remediation-Tasks.

Command-specific policy:

- Wenn `RESULTS_DIR` nicht existiert: fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen.
- Wenn `TASKS_DIR` nicht existiert: warnen und auf `/k-gui` hinweisen, aber fortfahren; dann können existierende Tasks nicht abgeglichen werden.

## Schritt 2 — Zieldatum bestimmen

Wenn `$ARGUMENTS` leer oder `latest` ist:

- Verwende `now.date` als Datum der Summary-Datei.
- Lies alle vorhandenen Result-Familien unter `RESULTS_DIR`, nicht nur heutige, aber bevorzuge pro Familie das neueste `assessment.md`.

Wenn `$ARGUMENTS` wie `YYYY-MM-DD` aussieht:

- Nutze dieses Datum für die Summary-Datei.
- Lies weiterhin pro Familie das neueste Assessment, außer der User explizit einen anderen Scope angibt.

Wenn `$ARGUMENTS` eine Datei ist:

- Abbrechen. `/k-results` akzeptiert keine Einzeldatei als Primärinput; dafür `/k-remediation` verwenden.

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

Ignoriere bestehende `summary-*.md` als Input, außer der User fordert eine Vergleichszusammenfassung explizit an.

Wenn mehrere Dates pro Family existieren:

- Standard: neuestes Date pro Family verwenden.
- Wenn ein älteres Date benutzt wird, in der Summary klar nennen.

Bekannte Familien und Standardreihenfolge:

1. `k-check`
2. `secret-scanning`
3. `dependency-cve`
4. `iac-container`
5. weitere Familien alphabetisch

## Schritt 4 — Kontext laden

Lade, falls vorhanden:

- `KNOWN_DECISIONS`: Findings, die klar gedeckt sind, als `accepted` markieren oder in P3 verschieben.
- `TASKS_DIR/*.md` und `TASKS_DIR/done/*.md`: bestehende Tasks erkennen, um doppelte Task-Erzeugung zu vermeiden.
- Alle `assessment.md` und `findings.md` der ausgewählten Familien.

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
- `Priorität`.
- `Quelle` / `Tool(s)` / `Package` / `Typ`.
- `Ort` / `Target` / `CVE/GHSA` / `Message`.
- `Raw-Quelle`.
- `Review-Bewertung`, `Triage-Notiz`, `Remediation`.

Statusmodell:

- Remediation-relevant: `open`, `confirmed`, `context-needed`.
- Nur nach expliziter Auswahl remediation-relevant: `likely-false-positive`.
- Nicht remediation-relevant: `accepted`, `fixed`.

## Schritt 6 — Deduplizieren und clustern

Cluster Findings über Familien hinweg nach Thema, nicht nur nach identischer ID.

Typische Dedupe-Regeln:

- Secret-Funde aus `secret-scanning` und `k-check` zusammenführen.
- CVE-Funde aus `dependency-cve` und `iac-container` nicht doppelt remediieren; `dependency-cve` ist primäre Quelle, `iac-container` liefert SBOM/Image-Kontext.
- Dockerfile-/Image-Funde aus `iac-container` separat halten, auch wenn Secret-/Config-Themen in anderen Familien auftauchen.
- False-positive-/Fixture-/Tooling-Artefakte als P3-Gruppe zusammenfassen.

## Schritt 7 — Priorisieren

Priorität projektweit vergeben, nicht blind Tool-Severity übernehmen.

P1:

- echte produktive Secrets oder unklare Rotation produktiver Secrets.
- bestätigbare SSRF/RCE/Auth/JWT/SQLi/critical Runtime CVEs.
- Secrets in Image-Layern oder Deployment-Konfiguration.
- direkte Provider-/Exception-Leaks mit Secret-/Config-Risiko.

P1/P2:

- hochrelevante Runtime-CVEs ohne final bestätigte Reachability.
- user-facing Authz-/Ownership-Risiken.
- sensitive Logging-Funde mit Provider-/User-Content-Risiko.

P2:

- High CVEs in zentralen Runtime-/SDK-Komponenten.
- IaC-/Helm-Abdeckungslücken im Production-Target.
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

- Nicht blind überschreiben.
- Entweder nach Bestätigung aktualisieren oder eindeutigen Namen vorschlagen, z. B. `summary-YYYY-MM-DD-2.md`.

Pflichtstruktur:

```markdown
# Security Results Summary - YYYY-MM-DD

Projekt: `<name>`

## Quellen

- <family>: `k-playbook-local/results/<family>/<date>/assessment.md`
- Known Decisions: `k-playbook-local/results/known-decisions.md` (<Status>)

## Priorisierte Übersicht

| Prio | Thema | Quelle(n) | Finding-ID(s) | Empfehlung | Status |
|---|---|---|---|---|---|

## P1-01 <Titel>

Kurzbeschreibung.

Empfehlung: konkreter nächster Schritt.

Quellen:
- `k-playbook-local/results/<family>/<date>/assessment.md`
- `k-playbook-local/results/<family>/<date>/findings.md#<finding-id>`

Was man zum Lösen braucht:
- betroffene Datei/Zeile
- relevante Tests/Smokes
- Akzeptanzkriterium

## Empfohlene Remediation-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook-local/results/summary-YYYY-MM-DD.md`
```

Schreibe knapp, aber lösungsfähig. Jede Top-Gruppe braucht:

- Beschreibung in einem Absatz.
- Empfehlung in einem Absatz.
- Quellenliste.
- Was man zum Lösen braucht.

## Schritt 9 — Review-Log aktualisieren

Wenn `LOG_FILE` gesetzt ist:

- Datei anlegen, falls sie noch nicht existiert (Skelett wie in `/k-review`).
- Sektion `## Security Results Summary` sicherstellen.
- `Letzter Lauf` auf heute setzen.
- Eine Protokollzeile anhängen:

```markdown
| YYYY-MM-DD | results-summary | alle Result-Familien | <N> priorisierte Themen -> `k-playbook-local/results/summary-YYYY-MM-DD.md`. Handoff: `/k-remediation ...` |
```

Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` empfehlen. Keine Ersatzdatei außerhalb von `RESULTS_DIR` anlegen.

## Schritt 10 — Abschluss

Berichte:

- Summary-Pfad.
- Welche Result-Familien verwendet wurden.
- Anzahl priorisierter Themen.
- Top 5 Themen.
- Handoff-Befehl für `/k-remediation`.

Keine Remediation starten.
