---
description: Erzeugt eine projektweite priorisierte Summary aus Review-Triage-Ergebnissen; liest neue review-triage.md-Dateien und Legacy assessment/findings nur als Fallback.
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

`/k-results` ist der Zwischenschritt zwischen `/k-review <family>` oder `/k-audit` und `/k-remediation`. Der Command startet keine Scanner, führt keine Remediation aus und verändert keine Raw-Artefakte. Er liest neue `review-triage.md`-Dateien unter `k-playbook-local/results/` und nutzt `assessment.md`/`findings.md` nur als Legacy-Fallback, wenn im selben Ergebnisordner kein `review-triage.md` vorhanden ist.

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
- `KNOWN_DECISIONS = <local.dir>/known-decisions.md`
- `LOG_FILE = <RESULTS_DIR>/log.md`
- `TASKS_DIR = <local.dir>/tasks`; optional aber hilfreich für existierende Remediation-Tasks.

Command-specific policy:

- Wenn `RESULTS_DIR` nicht existiert: fragen, ob genau dieses Verzeichnis angelegt werden soll, oder `/k-gui` nennen.
- Wenn `TASKS_DIR` nicht existiert: warnen und auf `/k-gui` hinweisen, aber fortfahren; dann können existierende Tasks nicht abgeglichen werden.

## Schritt 2 — Zieldatum bestimmen

Wenn `$ARGUMENTS` leer oder `latest` ist:

- Verwende `now.date` als Datum der Summary-Datei.
- Lies alle vorhandenen Audit- und Result-Family-Triages unter `RESULTS_DIR`, nicht nur heutige, aber bevorzuge pro Scope das neueste `review-triage.md`.

Wenn `$ARGUMENTS` wie `YYYY-MM-DD` aussieht:

- Nutze dieses Datum für die Summary-Datei.
- Lies weiterhin pro Scope die neueste Triage, außer der User explizit einen anderen Scope angibt.

Wenn `$ARGUMENTS` eine Datei ist:

- Abbrechen. `/k-results` akzeptiert keine Einzeldatei als Primärinput; dafür `/k-remediation` verwenden.

## Schritt 3 — Result-Familien finden

Suche unter:

```text
<RESULTS_DIR>/*/review-triage.md
<RESULTS_DIR>/*/*/review-triage.md
```

Legacy-Fallback nur, wenn im selben Ordner kein `review-triage.md` existiert:

```text
<RESULTS_DIR>/*/*/assessment.md
```

Erkenne Scope und Date aus dem Pfad:

```text
<RESULTS_DIR>/<date>/review-triage.md
<RESULTS_DIR>/<family>/<date>/review-triage.md
```

Erwarte optional im selben Verzeichnis:

- `findings.md`
- `raw/`
- `run-metadata.json` oder andere `run-metadata*.json`
- `review-input.json`

Ignoriere bestehende `summary-*.md` als Input, außer der User fordert eine Vergleichszusammenfassung explizit an.

Wenn mehrere Dates pro Scope existieren:

- Standard: neuestes Date pro Scope verwenden.
- Wenn ein älteres Date benutzt wird, in der Summary klar nennen.

Bekannte Scopes und Standardreihenfolge:

1. Audit-Läufe
2. `k-check`
3. `secret-scanning`
4. `dependency-cve`
5. `iac-container`
6. weitere Familien alphabetisch

## Schritt 4 — Kontext laden

Lade, falls vorhanden:

- `KNOWN_DECISIONS`: Findings, die klar gedeckt sind, als `accepted` markieren oder in P3 verschieben.
- `TASKS_DIR/*.md` und `TASKS_DIR/done/*.md`: bestehende Tasks erkennen, um doppelte Task-Erzeugung zu vermeiden.
- Alle `review-triage.md` der ausgewählten Scopes. Nur bei Legacy-Familien ohne
  `review-triage.md`: `assessment.md` und `findings.md` mitladen.

Wenn `known-decisions.md` leer ist, das in der Summary nennen.

## Schritt 5 — Findings extrahieren

Extrahiere aus `review-triage.md`:

- Kopf mit Scope, Quelle und Known-Decisions-Status.
- Bündel-Tabelle.
- Bündel-Details inklusive Begründung, Belege und nächstem Schritt.
- Nicht gebündelte Gruppen.
- Deckung aus known-decisions.

Extrahiere aus Legacy-`assessment.md` und `findings.md` nur, wenn kein `review-triage.md`
vorhanden ist. Aus `findings.md`:

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

- <family>: `k-playbook-local/results/<family>/<date>/review-triage.md`
- Known Decisions: `k-playbook-local/known-decisions.md` (<Status>)

## Priorisierte Übersicht

| Prio | Thema | Quelle(n) | Finding-ID(s) | Empfehlung | Status |
|---|---|---|---|---|---|

## P1-01 <Titel>

Kurzbeschreibung.

Empfehlung: konkreter nächster Schritt.

Quellen:
- `k-playbook-local/results/<family>/<date>/review-triage.md#<buendel-id>`
- Legacy-Fallback: `k-playbook-local/results/<family>/<date>/findings.md#<finding-id>`

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
