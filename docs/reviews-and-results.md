# Reviews und Results

Diese Datei dokumentiert den aktuellen k-playbook-Flow fuer Review-Rezepte, Scan-/Check-Results, priorisierte Zusammenfassungen und Remediation. Sie beschreibt das globale Zielbild; einzelne Review-Rezepte liegen unter `global/reviews/`.

## Grundmodell

k-playbook trennt vier Arbeitsschritte:

1. **Review-Familie ausfuehren**: `/k-review <review-name>` erzeugt oder bewertet Ergebnisse einer Scan-/Review-Familie.
2. **Result-Familien speichern**: Ergebnisse landen projektlokal unter `k-playbook/reviews/results/<family>/YYYY-MM-DD/`.
3. **Projektweit priorisieren**: `/k-results` fasst mehrere Result-Familien zusammen und dedupliziert sie.
4. **Remediation ausfuehren**: `/k-remediation <result-or-summary>` arbeitet priorisierte, statusfaehige Findings ab.

`/k-remediation` soll nicht erst alle Scannergebnisse aggregieren. Es soll mit einem bereits bewerteten Assessment oder einer priorisierten Summary starten. Innerhalb dieser Eingabe muss es aber vor der Umsetzung Findings zu sinnvollen Remediation-Buendeln gruppieren und nach Risiko, Aufwand, Quick-Win-Potential und gemeinsamer Verifikation sortieren.

## Verzeichnisse

Globale Review-Rezepte:

```text
~/dev/k-playbook/global/reviews/review-<name>.md
```

Projektlokale Review-Ergebnisse:

```text
<project>/k-playbook/reviews/results/<family>/YYYY-MM-DD/
```

Beispiel:

```text
k-playbook/reviews/results/k-check/2026-07-24/
├── assessment.md
├── findings.md
├── run-metadata.json
└── raw/
    └── k-check-baseline.txt
```

`k-playbook/checks/` bleibt fuer ausfuehrbare Checks und Check-Definitionen reserviert. Review-Ergebnisse gehoeren unter `k-playbook/reviews/`.

## Artefakte pro Result-Familie

Jede Report-/Scan-Familie soll diese Dateien erzeugen:

- `assessment.md` — kuratierte Gesamtbewertung, Kurzfazit, Priorisierung, Handoff.
- `findings.md` — mutable, statusfaehige Arbeitsliste aller Findings oder bewusst gruppierter Baseline-Findings.
- `raw/` — auditierbare Originalausgaben, z. B. SARIF, JSON oder Tool-Logs.
- `run-metadata.json` oder aequivalent — auditierbare Laufmetadaten.

Raw-Artefakte und Run-Metadaten sind append-only/auditierbar. Sie duerfen nach dem Schreiben nicht gekuerzt, ueberschrieben oder inhaltlich korrigiert werden. Korrekturen erfolgen ueber neue Raw-Dateien plus aktualisierte Bewertung.

`findings.md` ist das mutable Arbeitsregister fuer Status, Owner, Triage-Notizen, Remediation-Verweise und Akzeptierungen.

`assessment.md` ist kuratiert. Es darf spaeter nachvollziehbar aktualisiert werden, z. B. fuer `## Remediation-Status`, aber die urspruenglichen Raw-Belege bleiben unveraendert.

## Statusmodell

Standard-Statuswerte in `findings.md`:

| Status | Bedeutung | Remediation-Relevanz |
|---|---|---|
| `open` | neu oder noch nicht geprueft | ja |
| `confirmed` | validierter echter Befund | ja |
| `context-needed` | weitere Kontextpruefung noetig | ja |
| `likely-false-positive` | plausibler Fehlalarm | nur nach expliziter Auswahl |
| `accepted` | bewusste Entscheidung oder akzeptiertes Restrisiko | nein |
| `fixed` | behoben und verifiziert | nein |

Finding-IDs muessen stabil bleiben. Einmal vergebene IDs duerfen bei Re-Runs, Statusaenderungen oder Remediation nicht umbenannt werden.

Schema fuer k-check:

```text
kcheck-<area>-NNN
```

Beispiele:

- `kcheck-logging-003`
- `kcheck-secrets-001`
- `kcheck-user-scope-014`

CodeQL darf native Tool-Praefixe behalten, z. B. `py/full-ssrf-001`.

## Bestehende Review-Familien

## Scanner-Tools vs. k-check

Security-Scanner wie `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft` und `grype` werden **nicht** als `global/checks/*.sh` modelliert.

Grund:

- Sie erzeugen eigene strukturierte Rohdaten wie JSON, SARIF-aehnliche Reports oder SBOMs.
- Non-zero Exit-Codes bedeuten oft fachliche Findings, nicht technische Fehler.
- Ergebnisse muessen dedupliziert, priorisiert und bewertet werden.
- Raw-Artefakte muessen dauerhaft unter `k-playbook/reviews/results/<family>/YYYY-MM-DD/raw/` landen.
- Remediation braucht stabile Finding-IDs, Statuswerte und Quellenbelege.

Darum laufen diese Tools ueber Report-Mode Review-Familien:

| Tool | Review-Familie | Result-Familie |
|---|---|---|
| `gitleaks` | `/k-review secret-scanning` | `secret-scanning` |
| `trufflehog` | `/k-review secret-scanning` | `secret-scanning` |
| `pip-audit` | `/k-review dependency-cve` | `dependency-cve` |
| `trivy` | `/k-review dependency-cve` und `/k-review iac-container` | `dependency-cve` / `iac-container` |
| `syft` | `/k-review iac-container` | `iac-container` |
| `grype` | `/k-review dependency-cve` oder `/k-review iac-container` | `dependency-cve` / `iac-container` |
| GitHub Dependabot Alerts | `/k-review dependabot-alerts` | `dependabot-alerts` |

`global/checks/*.sh` bleibt fuer schnelle, generische k-check-Heuristiken und Preflight-artige Checks reserviert. Kleine Tool-Verfuegbarkeitschecks duerfen dort liegen, aber nicht der eigentliche Scannerlauf mit dauerhafter Bewertung.

### CodeQL

Rezept:

```text
global/reviews/review-codeql-security.md
```

Result-Familie:

```text
k-playbook/reviews/results/codeql/YYYY-MM-DD/
```

Typische Artefakte:

- `assessment.md`
- `findings.md`
- `raw/codeql-python.sarif`
- `raw/codeql-javascript-typescript.sarif`

CodeQL bewertet CWE-/Code-Findings, nicht Dependency-CVEs. Dependency-CVEs gehoeren in eine eigene Result-Familie.

Bei Wrapper-Repos soll der CodeQL-Block in `K-PLAYBOOK.MD` ein `target:` enthalten. Dieses Feld benennt den tatsaechlichen Analyse-/Git-Root, z. B. `./app`, waehrend Result-Artefakte weiterhin unter `k-playbook/reviews/` liegen.

### k-check

Rezept:

```text
global/reviews/review-k-check-security.md
```

Runner:

```text
global/bin/k-check
```

Result-Familie:

```text
k-playbook/reviews/results/k-check/YYYY-MM-DD/
```

Typischer auditierbarer Lauf:

```bash
~/dev/k-playbook/global/bin/k-check \
  --config-root <project-root> \
  --target-root <target-root> \
  --mode baseline \
  --output k-playbook/reviews/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt \
  --metadata-output k-playbook/reviews/results/k-check/YYYY-MM-DD/run-metadata.json
```

`--output` erhaelt stdout/stderr und schreibt zusaetzlich den vollstaendigen Raw-Stream. `--metadata-output` schreibt Kommando, Exit-Code, Zeitstempel, Roots, Modus, Check-Konfiguration und k-playbook-Version/Git-Commit soweit verfuegbar.

Vorhandene Ziel-Dateien werden nicht ueberschrieben. Fuer erneute Laeufe am selben Tag eindeutige Dateinamen verwenden, z. B. `k-check-baseline-e2e.txt` und `run-metadata-e2e.json`.

### Secret-Scanning

Rezept:

```text
global/reviews/review-secret-scanning.md
```

Result-Familie:

```text
k-playbook/reviews/results/secret-scanning/YYYY-MM-DD/
```

Typische Artefakte:

- `assessment.md` mit bewerteter Liste und Triage-Reihenfolge.
- `findings.md` mit statusfaehigen deduplizierten Secret-Findings.
- `raw/gitleaks-*.json`.
- `raw/trufflehog.json`.

Tools kommen host-lokal aus `/k-install-security-tools`. Fehlende Tools werden nicht im Projekt installiert.

### Dependency-CVE

Rezept:

```text
global/reviews/review-dependency-cve.md
```

Result-Familie:

```text
k-playbook/reviews/results/dependency-cve/YYYY-MM-DD/
```

Typische Artefakte:

- `assessment.md` mit bewerteter Liste nach Projektprioritaet, nicht nur Tool-Severity.
- `findings.md` mit statusfaehigen CVE-/GHSA-Findings.
- `raw/pip-audit.json`.
- `raw/trivy-fs.json`.
- `raw/grype.json`, falls Grype fuer den Scope ausgefuehrt wurde.

### GitHub Dependabot Alerts

Rezept:

```text
global/reviews/review-dependabot-alerts.md
```

Result-Familie:

```text
k-playbook/reviews/results/dependabot-alerts/YYYY-MM-DD/
```

Typische Artefakte:

- `assessment.md` mit bewerteten Remediation-Clustern und Einzelalert-Uebersicht.
- `findings.md` mit statusfaehigem Register pro GitHub-Alert (`depbot-<alert-number>`).
- `raw/dependabot-alerts-open.jsonl` als auditierbarer Import der offenen GitHub Alerts.
- `raw/dependabot-alerts-summary.tsv` fuer schnelle Triage.

Diese Familie nutzt GitHub/Dependabot als Quelle fuer Dependency-Warnungen. Sie ist besonders passend, wenn ein Projekt lokale Dependency-Scanner nicht nutzt oder zuerst die in GitHub bereits vorhandene Alert-Menge bewerten will. Absichtlich deaktivierte Dependabot-PRs, z. B. `open-pull-requests-limit: 0`, sind dabei kein Finding.

### IaC/Container

Rezept:

```text
global/reviews/review-iac-container.md
```

Result-Familie:

```text
k-playbook/reviews/results/iac-container/YYYY-MM-DD/
```

Typische Artefakte:

- `assessment.md` mit bewerteter Liste fuer Container-, Image-, IaC- und Filesystem-Findings.
- `findings.md` mit statusfaehigen deduplizierten Findings.
- `raw/trivy-*.json`.
- `raw/syft-*.json` und `raw/grype-*.json`, falls SBOM-/Grype-Scans fuer den Scope ausgefuehrt wurden.

Jede dieser Familien muss am Ende eine bewertete Liste in `assessment.md` erzeugen. `findings.md` bleibt das vollstaendige Arbeitsregister.

## Review-Log

`/k-review` pflegt projektlokal:

```text
k-playbook/reviews/log.md
```

Das Log enthaelt pro Review-Familie:

- letzter Lauf
- faellig ab
- Modus/Fokus
- Protokollzeile mit Scope, Output und Handoff

Beispiel-Handoff:

```text
/k-remediation k-playbook/reviews/results/k-check/2026-07-24/assessment.md
```

## Remediation

`/k-remediation` soll zwei Eingabeformen verstehen:

- Legacy-Ergebnisdateien wie `k-playbook/reviews/result-*.md`.
- Result-Familien wie `k-playbook/reviews/results/<family>/<date>/assessment.md` mit zugehoerigem `findings.md`.

Bei Result-Familien ist `findings.md` die primaere Arbeitsdatei. `assessment.md` liefert Kontext und Kurzbewertung. `raw/` und `run-metadata.*` sind read-only.

Beim Task-Anlegen muss ein Remediation-Task enthalten:

- Quelle: `k-playbook/reviews/results/<family>/<date>/assessment.md`
- Finding-ID(s) aus `findings.md`
- Arbeitsregister: `findings.md`
- Raw-Quelle falls vorhanden
- urspruengliche Ort-/Message-Angabe
- alle Findings, die zusammen geloest werden sollen, wenn ein gemeinsamer Fix-/Verifikationspfad existiert
- den projektlokalen Remediation-Modus aus `K-PLAYBOOK.MD`
- konkrete Verifikationsschritte

Projektweite Remediation-Policy:

```markdown
<!-- k-setup-remediation:managed:begin -->

## Remediation

- mode:           task-branch-pr
- target:         ./app
- grouping:       true
- quick-wins:     true
- branch-prefix:  remediation/
- pr-required:    true
- direct-fixes:   false
- setup-run:      2026-07-26

<!-- k-setup-remediation:managed:end -->
```

Modi:

- `task-branch-pr`: Remediation erzeugt Tasks/Buendel mit Branch-/PR-Hinweis; keine direkten Fixes aus `/k-remediation`.
- `task-first`: Tasks/Buendel zuerst; direkte Fixes nur nach expliziter Freigabe.
- `direct-allowed`: kleine sichere Fixes duerfen direkt umgesetzt werden, groessere werden Tasks.

## Priorisierte Gesamtzusammenfassung

Die priorisierte Gesamtzusammenfassung wird ueber `/k-results` erzeugt.

Ziel:

```text
k-playbook/reviews/results/summary-YYYY-MM-DD.md
```

Dieses Summary soll mehrere Result-Familien zusammenfassen:

- CodeQL
- k-check
- Secret-Scanning
- Dependency-CVE
- IaC/Container
- weitere spaetere Familien

Aufgaben von `/k-results`:

- `assessment.md` und `findings.md` aus allen Familien lesen.
- Befunde ueber Familien hinweg deduplizieren.
- `known-decisions.md` beruecksichtigen.
- existierende Tasks beruecksichtigen.
- eine priorisierte Tabelle der wichtigsten Punkte schreiben.
- pro Top-Punkt eine knappe Beschreibung, Empfehlung, Quellen und Loesungskontext ergaenzen.

Gewuenschtes Summary-Format:

```markdown
# Security Results Summary - YYYY-MM-DD

## Priorisierte Uebersicht

| Prio | Thema | Quelle(n) | Finding-ID(s) | Empfehlung | Status |
|---|---|---|---|---|---|

## P1-01 <Titel>

Kurzbeschreibung.

Empfehlung: konkreter naechster Schritt.

Quellen:
- `k-playbook/reviews/results/<family>/<date>/assessment.md`
- `k-playbook/reviews/results/<family>/<date>/findings.md#<finding-id>`

Was man zum Loesen braucht:
- betroffene Datei/Zeile
- relevante Tests
- Akzeptanzkriterium
```

`/k-remediation` kann gegen diese Summary laufen, damit die Reihenfolge der Bearbeitung projektweit priorisiert ist.

## Security-Tool-Installation

Tool-Installation und Docker-Fallbacks gehoeren ins globale k-playbook, nicht in einzelne Zielprojekte.

Vor Installation darf kein Projekt-venv aktiv sein. Python-CLI-Tools gehoeren in `pipx` oder ein dediziertes k-playbook Tool-venv, nicht in `<projekt>/.venv`.

Host-lokaler Preflight:

```text
/k-install-security-tools
```

Fehlende Pflicht-Tools installieren:

```text
/k-install-security-tools --install missing
```

Pflicht-Tools: `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype`.
