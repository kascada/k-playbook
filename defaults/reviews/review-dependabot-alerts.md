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

## Stellung im Audit-Laufmodell

Dieses Rezept bleibt im Audit-Laufmodell deaktiviert (`audit.enabled: false`). Über
`/k-review dependabot-alerts` bleibt es gezielt auswählbar.

**Geprüft und verworfen: Umstellung auf `audit.mode: evidence`.** Der Gedanke lag nahe —
das Rezept schriebe seine Alerts als SARIF nach `raw/dependabot-alerts.sarif`, liefe damit
im selben Merge wie die Scanner und bräuchte keinen Go-Konverter, weil ein
Evidence-Eintrag sein SARIF selbst schreibt. Die Dedupe gegen `pip-audit`, `trivy` und
`grype` käme gratis dazu: der Merge liest GHSA-/CVE-Kennungen aus `properties`
(`ghsa`, `cve`, `id`, `aliases`) und aus dem Meldungstext, nicht aus der Rule-ID. Drei
Punkte stehen dem entgegen, und der zweite ist nicht aus dem Rezept heraus lösbar:

1. **`audit.ruleIds` ist eine abgeschlossene Liste, die Advisory-Menge ist offen.** Jede
   Rule-ID eines Evidence-SARIF muss in `audit.ruleIds` stehen; eine unbekannte macht
   nicht den einzelnen Fund, sondern das ganze Artefakt ungültig. GHSA-IDs lassen sich
   nicht vorab aufzählen. Ein Ersatz aus synthetischen IDs — etwa nach Severity oder nach
   `runtime`/`development` — wäre möglich, verschöbe die Advisory-Kennung aber vollständig
   in `properties`.
2. **Der `github-actions`-Anteil fiele stillschweigend heraus.** Ein Evidence-Fund braucht
   einen Ort innerhalb von `audit.scope.paths`, und über jedem Pfad-Scope liegen die
   zentralen Ausschlüsse: Punkt-Verzeichnisse fallen immer heraus. Das Manifest der
   `github-actions`-Alerts ist `.github/workflows/…` — genau dieses Ökosystem nennen die
   Bewertungskriterien unten ausdrücklich. Die Alerts würden als „außerhalb des Scopes"
   verworfen, der Eintrag bliebe gültig und der Verlust nur eine Zahl im Bericht. Aus dem
   Rezept heraus ist das nicht zu heilen: `scope.paths` ist der weitere Rahmen, die
   Ausschlüsse sind der engere.
3. **Ein Evidence-Rezept liest Code, dieses liest eine fremde API.** Der Lauf führt
   Evidence-Rezepte auf dem Code im Pfad-Scope aus. Dieses Rezept braucht Netz und eine
   `gh`-Anmeldung, und sein Pfad-Scope wäre eine Behauptung: Es liest die Manifeste nicht,
   es beschriftet seine Funde nur damit. Für „dieser Eintrag braucht Netz" gibt es im
   Laufmodell bisher keinen Ausdruck.

Die Umstellung braucht deshalb einen eigenen Task — entweder für eine Ausnahme von der
Punkt-Verzeichnis-Regel im Pfad-Scope oder für einen Werkzeug-Eintrag mit SARIF-Konverter
nach dem Muster von `trufflehog` und `pip-audit`. Beides ist Arbeit am Go-Werkzeug und
gehört nicht in dieses Rezept.

**Was das für den Weg dieses Rezepts bedeutet.** Ein `/k-review dependabot-alerts`-Lauf
legt einen Family-Ordner außerhalb jedes Laufordners an; sein `review-triage.md` geht
direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/dependabot-alerts/<datum>/review-triage.md
```

Es gibt dabei **keine familienübergreifende Zusammenführung und keine Dedupe gegen andere
Quellen** mehr. `/k-remediation` nimmt genau eine Ergebnisdatei; ein Alert, den daneben
auch `pip-audit` oder `trivy` im Audit-Lauf gemeldet hat, steht in beiden Ergebnissen
einmal. Das ist die bewusste Folge des Umbaus, kein Fehler: Wer eine Zusammenführung
braucht, bringt seine Belege in den Lauf, statt sie danach zusammenzurechnen.

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
