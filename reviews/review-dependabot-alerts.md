---
name: review-dependabot-alerts
title: GitHub Dependabot Alerts Assessment
interval-weeks: 1
scope-hint: Offene GitHub Dependabot Security Alerts für das aktuelle Repository; keine Dependency-Upgrades, keine Dependabot-PRs aus diesem Review heraus
handoff: /k-remediation
result-family: dependabot-alerts
audit:
  enabled: false
review:
  enabled: true
---

# Review: GitHub Dependabot Alerts Assessment

Erzeuge eine kuratierte, bewertete Liste aus GitHub Dependabot Security Alerts. Dieses
Review ist für Projekte gedacht, bei denen GitHub/Dependabot die relevante Quelle für
Dependency-Warnungen ist oder lokale Dependency-Scanner bewusst nicht genutzt werden.

Dieses Rezept bleibt im Audit-Laufmodell deaktiviert, weil der Input extern über
`gh api` kommt und noch kein Tool-Eintrag im Lauf-Merge ist. Über
`/k-review dependabot-alerts` bleibt es gezielt auswählbar.

## Zweck

- GitHub Dependabot Alerts dauerhaft als Review-Artefakte sichern.
- GitHub-/Advisory-Severity von projektspezifischer Review-Priorität trennen.
- Viele Alerts zu handhabbaren Remediation-Clustern gruppieren, ohne einzelne Alert-IDs
  zu verlieren.
- Keine Dependency-Upgrades, keine Lockfile-Änderungen und keine Dependabot-PR-Erzeugung
  durch dieses Review.

## Abgrenzung

- Dieses Review konsumiert GitHub Dependabot Alerts über `gh api`.
- Es ersetzt lokale Scans mit `pip-audit`, `trivy` oder `grype` nicht; diese gehören zu
  `/k-review dependency-cve`.
- Wenn für ein Projekt GitHub Dependabot als Quelle der Wahrheit reicht, darf dieses
  Review statt lokaler Dependency-CVE-Scans genutzt werden.
- `open-pull-requests-limit: 0` in `.github/dependabot.yml` ist kein Fehler, wenn das
  Projekt Alerts zuerst manuell triagieren will. Dieses Review darf das nicht als Finding
  melden.

## Bewertungskriterien

GitHub-Severity allein ist nicht die Review-Priorität. Priorisiere nach:

- P1: `critical` oder remote ausnutzbare `high` Alerts in Runtime-/Produktionsdependencies,
  Auth-/Crypto-/Parser-/Network-Pfaden oder direkt genutzten Paketen.
- P2: hohe Alerts in direkten Dependencies, produktionsnahen transitiven Dependencies oder
  zentralen Build-/Runtime-Komponenten mit klarem Fix.
- P3: Dev-only, Build-only oder transitive Alerts ohne sichtbaren Produktpfad, niedrige
  Severity, aggregierte Altlasten ohne akute Exposition.

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

- Finding-ID ist stabil aus der GitHub-Alert-Nummer: `depbot-<alert-number>`, z. B.
  `depbot-81`.
- Einzelne GitHub-Alerts werden in `review-input.json` nicht still zusammengelegt, weil
  Alert-Nummern, GHSA/CVE und GitHub-URLs für Audit und Status wichtig sind.
- `review-triage.md` darf Alerts zu Remediation-Clustern gruppieren, z. B. `Django in
  requirements.txt auf 5.2.15+`, `Pillow auf 12.3.0+`, `Vite/PostCSS im Frontend-Lockfile`.
- Wenn mehrere Alerts dasselbe Paket, Manifest und dieselbe Fix-Version betreffen, im
  Assessment als ein Upgrade-/Triage-Punkt behandeln und alle `depbot-*` IDs referenzieren.
- Alerts ohne Fix-Version bleiben eigene Cluster mit `context-needed`, sofern kein bekannter
  Workaround oder akzeptiertes Restrisiko dokumentiert ist.

## Handoff

Nach Abschluss verweist `/k-review` auf `review-triage.md` im Family-Ordner. Remediation,
Dependency-Upgrades und Dependabot-PR-Erzeugung sind ausdrücklich nicht Teil dieses
Reviews.
