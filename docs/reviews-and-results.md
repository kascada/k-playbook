# Reviews und Results

Diese Seite beschreibt das Artefaktmodell: welche Dateien ein Review erzeugt, wo sie
liegen und welchen Status sie tragen. Der Ablauf der Commands steht in
[`code-review.md`](./code-review.md).

## Grundmodell

k-playbook trennt vier Schritte:

1. **Review ausfuehren** — `/k-review <name>` erzeugt oder bewertet Ergebnisse einer Familie.
2. **Ergebnisse ablegen** — je Familie und Datum unter `k-playbook-local/results/`.
3. **Projektweit priorisieren** — `/k-results` fasst mehrere Familien zusammen und dedupliziert.
4. **Abarbeiten** — `/k-remediation` arbeitet priorisierte, statusfaehige Findings ab.

`/k-remediation` aggregiert nicht selbst. Es startet mit einem bereits bewerteten
Assessment oder einer priorisierten Summary, gruppiert die Findings darin aber vor der
Umsetzung zu Buendeln — nach Risiko, Aufwand, Quick-Win-Potential und gemeinsamer
Verifikation.

## Verzeichnisse

Rezepte und Ergebnisse sind strikt getrennt:

```text
k-playbook/reviews/                    mitgelieferte Rezepte
k-playbook-local/reviews/              projekteigene Rezepte, Overlay
k-playbook-local/results/              alles, was Reviews erzeugen
```

`reviews/` enthaelt ausschliesslich `review-<name>.md`. Damit bleibt es ein reines
Overlay-Verzeichnis, in dem jede Datei nach derselben Regel behandelt wird: gleicher
Dateiname, lokale Datei gewinnt vollstaendig.

Alles Erzeugte liegt daneben:

```text
k-playbook-local/results/
├── log.md                        wann welches Review lief
├── known-decisions.md            bewusst getroffene Entscheidungen
├── summary-YYYY-MM-DD.md         projektweite Priorisierung aus /k-results
└── <familie>/YYYY-MM-DD/
    ├── assessment.md
    ├── findings.md
    ├── run-metadata.json
    └── raw/
```

Beispiel:

```text
k-playbook-local/results/k-check/2026-07-24/
├── assessment.md
├── findings.md
├── run-metadata.json
└── raw/
    └── k-check-baseline.txt
```

`k-playbook-local/checks/` bleibt fuer ausfuehrbare Checks reserviert. Ergebnisse gehoeren
nie dorthin.

## Artefakte pro Familie

Jede Report-/Scan-Familie erzeugt diese Dateien:

- `assessment.md` — kuratierte Gesamtbewertung, Kurzfazit, Priorisierung, Handoff.
- `findings.md` — mutable, statusfaehige Arbeitsliste aller Findings.
- `raw/` — auditierbare Originalausgaben, z. B. SARIF, JSON oder Tool-Logs.
- `run-metadata.json` oder aequivalent — auditierbare Laufmetadaten.

Raw-Artefakte und Run-Metadaten sind auditierbar. Sie duerfen nach dem Schreiben nicht
gekuerzt, ueberschrieben oder inhaltlich korrigiert werden. Korrekturen erfolgen ueber
neue Raw-Dateien plus aktualisierte Bewertung.

`findings.md` ist das mutable Arbeitsregister fuer Status, Owner, Triage-Notizen,
Remediation-Verweise und Akzeptierungen.

`assessment.md` ist kuratiert. Es darf nachvollziehbar aktualisiert werden, z. B. um einen
Abschnitt `## Remediation-Status`, aber die urspruenglichen Raw-Belege bleiben unveraendert.

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

Finding-IDs muessen stabil bleiben. Einmal vergebene IDs duerfen bei Re-Runs,
Statusaenderungen oder Remediation nicht umbenannt werden.

Schema fuer k-check:

```text
kcheck-<area>-NNN
```

Beispiele: `kcheck-logging-003`, `kcheck-secrets-001`, `kcheck-user-scope-014`.

CodeQL darf native Tool-Praefixe behalten, z. B. `py/full-ssrf-001`.

## Scanner-Tools vs. k-check

Security-Scanner wie `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft` und `grype`
werden **nicht** als Checks unter `checks/*.sh` modelliert.

Grund:

- Sie erzeugen eigene strukturierte Rohdaten wie JSON, SARIF-aehnliche Reports oder SBOMs.
- Non-zero Exit-Codes bedeuten oft fachliche Findings, nicht technische Fehler.
- Ergebnisse muessen dedupliziert, priorisiert und bewertet werden.
- Raw-Artefakte muessen dauerhaft unter `k-playbook-local/results/<familie>/YYYY-MM-DD/raw/` landen.
- Remediation braucht stabile Finding-IDs, Statuswerte und Quellenbelege.

Darum laufen diese Tools ueber Report-Mode-Reviews:

| Tool | Review | Ergebnisfamilie |
|---|---|---|
| `gitleaks` | `/k-review secret-scanning` | `secret-scanning` |
| `trufflehog` | `/k-review secret-scanning` | `secret-scanning` |
| `pip-audit` | `/k-review dependency-cve` | `dependency-cve` |
| `trivy` | `/k-review dependency-cve` und `/k-review iac-container` | `dependency-cve` / `iac-container` |
| `syft` | `/k-review iac-container` | `iac-container` |
| `grype` | `/k-review dependency-cve` oder `/k-review iac-container` | `dependency-cve` / `iac-container` |
| GitHub Dependabot Alerts | `/k-review dependabot-alerts` | `dependabot-alerts` |

`checks/*.sh` bleibt fuer schnelle, generische k-check-Heuristiken und Preflight-artige
Checks reserviert. Kleine Tool-Verfuegbarkeitschecks duerfen dort liegen, aber nicht der
eigentliche Scannerlauf mit dauerhafter Bewertung.

## Die Familien im Einzelnen

### CodeQL

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-codeql-security.md` |
| Ergebnisse | `k-playbook-local/results/codeql/YYYY-MM-DD/` |

Typische Artefakte: `assessment.md`, `findings.md`, `raw/codeql-python.sarif`,
`raw/codeql-javascript-typescript.sarif`.

CodeQL bewertet CWE-/Code-Findings, nicht Dependency-CVEs. Die gehoeren in eine eigene
Familie.

Bei Projekten, deren Code nicht im Hauptverzeichnis liegt, benennt `tools.codeql.target`
in `K-PLAYBOOK.yaml` den tatsaechlichen Analyse-Root. Die Ergebnisse liegen trotzdem unter
`k-playbook-local/results/`.

### k-check

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-k-check-security.md` |
| Runner | `k-playbook/bin/k-check` |
| Ergebnisse | `k-playbook-local/results/k-check/YYYY-MM-DD/` |

Typischer auditierbarer Lauf:

```bash
k-playbook/bin/k-check \
  --mode baseline \
  --output k-playbook-local/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt \
  --metadata-output k-playbook-local/results/k-check/YYYY-MM-DD/run-metadata.json
```

`--output` erhaelt stdout/stderr und schreibt zusaetzlich den vollstaendigen Raw-Stream.
`--metadata-output` schreibt Kommando, Exit-Code, Zeitstempel, Roots, Modus,
Check-Konfiguration und Version bzw. Git-Commit, soweit verfuegbar.

Vorhandene Ziel-Dateien werden nicht ueberschrieben. Fuer erneute Laeufe am selben Tag
eindeutige Namen verwenden, z. B. `k-check-baseline-e2e.txt` und `run-metadata-e2e.json`.

### Secret-Scanning

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-secret-scanning.md` |
| Ergebnisse | `k-playbook-local/results/secret-scanning/YYYY-MM-DD/` |

Typische Artefakte: `assessment.md` mit bewerteter Liste und Triage-Reihenfolge,
`findings.md` mit statusfaehigen deduplizierten Findings, `raw/gitleaks-*.json`,
`raw/trufflehog.json`.

Die Tools kommen host-lokal aus
`k-playbook/scripts/install-security-tools.sh`. Fehlende Tools werden nicht im Projekt
installiert.

### Dependency-CVE

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-dependency-cve.md` |
| Ergebnisse | `k-playbook-local/results/dependency-cve/YYYY-MM-DD/` |

Typische Artefakte: `assessment.md` mit Bewertung nach Projektprioritaet statt nur
Tool-Severity, `findings.md` mit statusfaehigen CVE-/GHSA-Findings, `raw/pip-audit.json`,
`raw/trivy-fs.json`, bei Bedarf `raw/grype.json`.

### GitHub Dependabot Alerts

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-dependabot-alerts.md` |
| Ergebnisse | `k-playbook-local/results/dependabot-alerts/YYYY-MM-DD/` |

Typische Artefakte: `assessment.md` mit Remediation-Clustern und Einzelalert-Uebersicht,
`findings.md` mit einem Register je GitHub-Alert (`depbot-<alert-number>`),
`raw/dependabot-alerts-open.jsonl` als auditierbarer Import,
`raw/dependabot-alerts-summary.tsv` fuer schnelle Triage.

Diese Familie nutzt GitHub als Quelle. Sie passt besonders, wenn ein Projekt lokale
Dependency-Scanner nicht nutzt oder zuerst die in GitHub vorhandene Alert-Menge bewerten
will. Absichtlich deaktivierte Dependabot-PRs, z. B. `open-pull-requests-limit: 0`, sind
kein Finding.

### IaC/Container

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-iac-container.md` |
| Ergebnisse | `k-playbook-local/results/iac-container/YYYY-MM-DD/` |

Typische Artefakte: `assessment.md` fuer Container-, Image-, IaC- und
Filesystem-Findings, `findings.md` mit statusfaehigen deduplizierten Findings,
`raw/trivy-*.json`, bei Bedarf `raw/syft-*.json` und `raw/grype-*.json`.

## Review-Log

`/k-review` pflegt das Log neben den Ergebnissen:

```text
k-playbook-local/results/log.md
```

Es enthaelt pro Familie den letzten Lauf, ab wann der naechste faellig ist, Modus und
Fokus sowie eine Protokollzeile mit Scope, Output und Handoff.

Beispiel-Handoff:

```text
/k-remediation k-playbook-local/results/k-check/2026-07-24/assessment.md
```

`known-decisions.md` liegt daneben und haelt fest, was bewusst so entschieden wurde. Jedes
Review liest die Datei, damit dieselbe Stelle nicht bei jedem Lauf erneut als Finding
auftaucht.

## Remediation

`/k-remediation` versteht zwei Eingaben:

- eine Summary, `k-playbook-local/results/summary-YYYY-MM-DD.md`,
- eine Familie, `k-playbook-local/results/<familie>/<datum>/assessment.md` mit
  zugehoerigem `findings.md`.

Bei einer Familie ist `findings.md` die primaere Arbeitsdatei; `assessment.md` liefert
Kontext und Kurzbewertung. `raw/` und `run-metadata.*` sind read-only.

Ein erzeugter Remediation-Task muss enthalten:

- die Quelle, `k-playbook-local/results/<familie>/<datum>/assessment.md`,
- die Finding-IDs aus `findings.md`,
- das Arbeitsregister `findings.md`,
- die Raw-Quelle, falls vorhanden,
- die urspruengliche Ort-/Message-Angabe,
- alle Findings, die zusammen geloest werden sollen, wenn es einen gemeinsamen
  Fix-/Verifikationspfad gibt,
- den Remediation-Modus aus `K-PLAYBOOK.yaml`,
- konkrete Verifikationsschritte.

Projektweite Policy:

```yaml
remediation:
  mode: task-branch-pr
  target: app
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false
```

Modi, vom striktesten zum offensten:

- `task-branch-pr` — keine direkten Fixes. Jedes bestaetigte Buendel wird eine Task mit
  Branch- und PR-Hinweis; umgesetzt wird spaeter ueber `/k-run`.
- `task-first` — Tasks sind der Standard. Direkte Fixes nur nach ausdruecklicher Freigabe
  fuer einzelne kleine Buendel. **Das ist der Default.**
- `direct-allowed` — kleine, sichere Befunde duerfen nach Code-Sichtung sofort behoben
  werden, wenn die Kategorien freigegeben sind.

`pr_required` und `direct_fixes` werden aus `mode` abgeleitet und mitgeschrieben.

## Priorisierte Gesamtzusammenfassung

`/k-results` erzeugt:

```text
k-playbook-local/results/summary-YYYY-MM-DD.md
```

Die Summary fasst mehrere Familien zusammen — CodeQL, k-check, Secret-Scanning,
Dependency-CVE, IaC/Container und spaetere.

Aufgaben von `/k-results`:

- `assessment.md` und `findings.md` aus allen Familien lesen,
- Befunde ueber Familien hinweg deduplizieren,
- `known-decisions.md` beruecksichtigen,
- existierende Tasks beruecksichtigen,
- eine priorisierte Tabelle der wichtigsten Punkte schreiben,
- je Top-Punkt Beschreibung, Empfehlung, Quellen und Loesungskontext ergaenzen.

Format:

```markdown
# Security Results Summary - YYYY-MM-DD

## Priorisierte Uebersicht

| Prio | Thema | Quelle(n) | Finding-ID(s) | Empfehlung | Status |
|---|---|---|---|---|---|

## P1-01 <Titel>

Kurzbeschreibung.

Empfehlung: konkreter naechster Schritt.

Quellen:
- `k-playbook-local/results/<familie>/<datum>/assessment.md`
- `k-playbook-local/results/<familie>/<datum>/findings.md#<finding-id>`

Was man zum Loesen braucht:
- betroffene Datei/Zeile
- relevante Tests
- Akzeptanzkriterium
```

`/k-remediation` kann gegen diese Summary laufen, damit die Reihenfolge der Bearbeitung
projektweit priorisiert ist.

## Security-Tools

Tool-Installation und Docker-Fallbacks sind host- oder user-lokal, nie projektlokal. Vor
der Installation darf kein Projekt-venv aktiv sein; Python-CLI-Tools gehoeren in `pipx`
oder ein dediziertes k-playbook-Tool-venv.

```bash
k-playbook/scripts/install-security-tools.sh                    # Status
k-playbook/scripts/install-security-tools.sh --install missing  # fragt vor der Installation
```

Die Pflicht-Tools stehen kanonisch in
[`../scripts/security-tools.tsv`](../scripts/security-tools.tsv). Skript, Oberflaeche und
Review-Rezepte lesen dieselbe Matrix.
