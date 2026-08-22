---
name: review-dependabot-alerts
title: GitHub Dependabot Alerts Assessment
interval-weeks: 1
scope-hint: Offene GitHub Dependabot Security Alerts für das aktuelle Repository; keine Dependency-Upgrades, keine Dependabot-PRs aus diesem Review heraus
handoff: /k-remediation
result-family: dependabot-alerts
audit:
  enabled: false
  title: GitHub Dependabot Alerts Assessment
  resultRequired: true
  defaultResult: review-dependabot-alerts.md
review:
  enabled: true
---

# Review: GitHub Dependabot Alerts Assessment

Erzeuge eine kuratierte, bewertete Liste aus GitHub Dependabot Security Alerts. Dieses Review ist für Projekte gedacht, bei denen GitHub/Dependabot die relevante Quelle für Dependency-Warnungen ist oder lokale Dependency-Scanner bewusst nicht genutzt werden.

## Zweck

- GitHub Dependabot Alerts dauerhaft als Review-Artefakte sichern.
- GitHub-/Advisory-Severity von projektspezifischer Review-Priorität trennen.
- Viele Alerts zu handhabbaren Remediation-Clustern gruppieren, ohne einzelne Alert-IDs zu verlieren.
- `review-input.json` als Belegvertrag und `review-triage.md` als Handoff erzeugen.
- Keine Dependency-Upgrades, keine Lockfile-Änderungen und keine Dependabot-PR-Erzeugung durch dieses Review.

## Abgrenzung

- Dieses Review konsumiert GitHub Dependabot Alerts über `gh api`.
- Es ersetzt lokale Scans mit `pip-audit`, `trivy` oder `grype` nicht; diese gehören zu `/k-review dependency-cve`.
- Wenn für ein Projekt GitHub Dependabot als Quelle der Wahrheit reicht, darf dieses Review statt lokaler Dependency-CVE-Scans genutzt werden.
- `open-pull-requests-limit: 0` in `.github/dependabot.yml` ist kein Fehler, wenn das Projekt Alerts zuerst manuell triagieren will. Dieses Review darf das nicht als Finding melden.

## Voraussetzungen

- Pfade kommen aus der Context-Ausgabe, die `/k-review` bereits geladen hat: `RESULTS_DIR` = `<local.dir>/results`.
- Wenn der Context-Aufruf fehlschlägt: abbrechen und `/k-gui` empfehlen.
- Wenn `RESULTS_DIR` fehlt: abbrechen und `/k-gui` nennen; dieses Review braucht ein Ergebnisverzeichnis.
- Lies optional `tools.dependabot` aus `K-PLAYBOOK.yaml`, falls vorhanden.
- Wenn vorhanden, nutze `tools.dependabot.target` als Git-/App-Root und `tools.dependabot.repo` als GitHub `owner/repo`. Wenn `target` fehlt, gilt `.`. Wenn `repo` fehlt, leite den Repo-Slug aus dem GitHub-Remote des Targets ab oder frage den User.
- Wenn `tools.dependabot.config` gesetzt ist, prüfe, ob die Dependabot-Konfig existiert. `pull_requests: false` oder `open-pull-requests-limit: 0` ist kein Fehler, wenn Alerts manuell triagiert werden sollen.
- Prüfe `gh` aus der Context-Ausgabe, bevor der erste `gh api`-Aufruf ansteht: bei
  `gh.status: disabled` abbrechen mit dem Hinweis, dass dieses Projekt gh nicht nutzt;
  bei `gh.status: unknown` abbrechen und `/k-gui` nennen, wo die Entscheidung fällt;
  bei `gh.ready: false` abbrechen und benennen, was fehlt — Installation oder
  `gh auth login --hostname github.com`.
- Prüfe, dass das Ziel ein Git-Repo mit GitHub-Remote ist oder der User `owner/repo` explizit vorgibt.
- Wenn die Dependabot-Alerts-API `404` liefert: nicht als "keine Alerts" werten. Mögliche Ursachen dokumentieren: fehlende Berechtigung, Dependabot Alerts nicht aktiviert, falsches Repository oder private-Repo-Policy.

## Ergebnisverzeichnis

Dieses Review schreibt in:

`k-playbook-local/results/dependabot-alerts/YYYY-MM-DD/`

Dateien:

- `review-input.json` - strukturierter Belegvertrag mit Scope, Gruppen, Evidence und Known-Decision-Coverage.
- `review-triage.md` - kuratierte Gesamtbewertung mit Bündeln, nicht gebündelten Findings und Handoff.
- `raw/dependabot-alerts-open.jsonl` - auditierbare Original-Alerts, ein GitHub-Alert pro JSON-Zeile.
- `raw/dependabot-alerts-summary.tsv` - optionale tabellarische Arbeitskopie für schnelle Sichtung.
- `run-metadata.json` - Repo, Branch, API-Endpoint, Befehle, Exit-Codes, Zeitpunkt, `gh`-Account und Filter.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar und dürfen nach dem Schreiben nicht gekürzt oder überschrieben werden. Bei erneutem Lauf am selben Tag eindeutige Dateinamen verwenden, z. B. `dependabot-alerts-open-2.jsonl` und `run-metadata-2.json`.

## Ausführungsentscheidung

Frage vor GitHub-API-Aufrufen, was passieren soll:

- **Vorhandene Raw-Ausgaben auswerten (Default)**: Nutzt `raw/dependabot-alerts-*.jsonl` oder explizit angegebene JSON/JSONL-Dateien. Keine neue GitHub-Abfrage.
- **GitHub Dependabot Alerts importieren**: Nur nach Bestätigung. Zeige vorher Repo-Slug, State-Filter und Befehle.
- **Nur Preflight**: Pfade, GitHub-Repo, `gh`-Auth und geplante Artefakte zeigen.
- **Abbrechen**.

Typische Befehle nach Bestätigung:

```bash
gh repo view <owner>/<repo> --json owner,name,url,defaultBranchRef
gh api --paginate --jq '.[]' \
  '/repos/<owner>/<repo>/dependabot/alerts?state=open&per_page=100' \
  > <result>/raw/dependabot-alerts-open.jsonl
gh api --paginate --jq '.[] | [.number, .state, .security_vulnerability.severity, .dependency.package.ecosystem, .dependency.package.name, .dependency.manifest_path, .dependency.scope, .dependency.relationship, .security_vulnerability.vulnerable_version_range, (.security_vulnerability.first_patched_version.identifier // "none"), .security_advisory.ghsa_id, .security_advisory.cve_id, .html_url] | @tsv' \
  '/repos/<owner>/<repo>/dependabot/alerts?state=open&per_page=100' \
  > <result>/raw/dependabot-alerts-summary.tsv
```

Wenn der User explizit dismissed/fixed Alerts einbeziehen will, `state=all` verwenden und den Filter im Assessment deutlich nennen. Default ist `state=open`.

## Bewertungskriterien

GitHub-Severity allein ist nicht die Review-Priorität. Priorisiere nach:

- P1: `critical` oder remote ausnutzbare `high` Alerts in Runtime-/Produktionsdependencies, Auth/Crypto/Parser/Network-Pfaden oder direkt genutzten Paketen.
- P2: hohe Alerts in direkten Dependencies, produktionsnahen transitiven Dependencies oder zentralen Build-/Runtime-Komponenten mit klarem Fix.
- P3: Dev-only, Build-only oder transitive Alerts ohne sichtbaren Produktpfad, niedrige Severity, aggregierte Altlasten ohne akute Exposition.

Berücksichtige:

- Manifest-/Lockfile-Quelle.
- Ecosystem (`pip`, `npm`, `github-actions`, ...).
- `scope` (`runtime`, `development`) und `relationship` (`direct`, `transitive`).
- Fix-Version und ob mehrere Alerts durch ein einziges Paket-Upgrade geschlossen werden.
- Ob das betroffene Paket im Produktpfad genutzt wird oder nur Tooling/Build betrifft.
- Ob Dependabot-PRs absichtlich deaktiviert sind, um die Alert-Menge manuell abzuarbeiten.
- Bekannte Entscheidungen aus `known-decisions.md`.

Review-Status im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - Alert betrifft eine relevante Dependency im Projektkontext.
- `context-needed` - Reachability, Runtime-Pfad oder Upgrade-Auswirkung unklar.
- `likely-false-positive` - Alert-Mapping oder Zielkontext wahrscheinlich nicht zutreffend.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert, z. B. Alert in GitHub geschlossen.

## Deduplizierung und Cluster

- Finding-ID ist stabil aus der GitHub-Alert-Nummer: `depbot-<alert-number>`, z. B. `depbot-81`.
- Einzelne GitHub-Alerts werden in `review-input.json` nicht still zusammengelegt, weil Alert-Nummern, GHSA/CVE und GitHub-URLs für Audit und Status wichtig sind.
- `review-triage.md` darf Alerts zu Remediation-Clustern gruppieren, z. B. `Django in requirements.txt auf 5.2.15+`, `Pillow auf 12.3.0+`, `Vite/PostCSS im Frontend-Lockfile`.
- Wenn mehrere Alerts dasselbe Paket, Manifest und dieselbe Fix-Version betreffen, im Assessment als ein Upgrade-/Triage-Punkt behandeln und alle `depbot-*` IDs referenzieren.
- Alerts ohne Fix-Version bleiben eigene Cluster mit `context-needed`, sofern kein bekannter Workaround oder akzeptiertes Restrisiko dokumentiert ist.

## Review-Triage-Format

`review-triage.md` enthält mindestens die Pflichtabschnitte aus `commands/_audit/review-scan-triage.md`:

```markdown
# GitHub Dependabot Alerts Assessment - YYYY-MM-DD

## Quellen

- Repository: `<owner>/<repo>`
- Target: `<target>`
- Config: `<config>`
- API-Filter: `state=open`
- Raw: `raw/dependabot-alerts-open.jsonl`
- Summary: `raw/dependabot-alerts-summary.tsv`
- Run-Metadaten: `run-metadata.json`
- Quelle: `review-input.json`

## Kurzfazit

- Offene Alerts: <n>
- Severity: critical/high/medium/low = <counts>
- Betroffene Manifeste: <liste mit counts>
- Wichtigste Remediation-Cluster: <kurz>
- Dependabot-PRs: <aktiv/deaktiviert/unklar>; keine Bewertung als Fehler, wenn bewusst deaktiviert.

## Bewertete Remediation-Cluster

| Prio | Cluster | Alert-IDs | Package(s) | Manifest(e) | Fix-Version(en) | Bewertung | Nächster Schritt |
|---|---|---|---|---|---|---|---|

## Einzelalert-Übersicht

| Alert | Prio | Status | Severity | Package | Manifest | Scope | Direct/Transitive | GHSA/CVE | Fix | URL |
|---|---|---|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Handoff

`/k-remediation k-playbook-local/results/dependabot-alerts/YYYY-MM-DD/review-triage.md`
```

## Review-Input-Format

`review-input.json` enthält pro GitHub-Alert:

```json
{
  "scope": { "type": "review", "family": "dependabot-alerts" },
  "groups": [
    {
      "id": "depbot-81",
      "title": "<package> in <manifest>",
      "priority": "P1|P2|P3",
      "findings": ["depbot-81"],
      "evidence": [
        {
          "file": "<manifest>",
          "line": null,
          "source": "dependabot:81",
          "message": "<GHSA/CVE>, Severity, vulnerable range, fix version, URL"
        }
      ],
      "coveredByKnownDecision": false,
      "partialCoverage": false,
      "knownDecisionCoverage": []
    }
  ],
  "ungroupedFindings": [],
  "knownDecisions": { "status": "loaded|missing|empty", "coverage": [] }
}
```

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation k-playbook-local/results/dependabot-alerts/YYYY-MM-DD/review-triage.md
```

Remediation, Dependency-Upgrades und Dependabot-PR-Erzeugung sind ausdrücklich nicht Teil dieses Reviews.
