# Reviews und Results

Diese Seite beschreibt das Artefaktmodell: welche Dateien ein Review erzeugt, wo sie
liegen und welchen Status sie tragen. Der Ablauf der Commands steht in
[`code-review.md`](./code-review.md).

## Grundmodell

k-playbook trennt drei Schritte:

1. **Review oder Audit ausführen** — `/k-review <name>` bewertet gezielt eine Familie,
   `/k-audit` führt einen vollständigen Sweep aus.
2. **Ergebnisse ablegen** — je Lauf beziehungsweise je Familie und Datum unter
   `k-playbook-local/results/`.
3. **Abarbeiten** — `/k-remediation` arbeitet die bewerteten Bündel aus genau einer
   Ergebnisdatei ab.

Zusammengeführt wird an **einer** Stelle: im Audit-Lauf. Der Merge dedupliziert über
Werkzeuge hinweg, schreibt die Deckung aus `known-decisions.md` mit, und die Triage
vergibt Priorität und Kategorie. Einen nachgelagerten Schritt, der dasselbe eine Ebene
höher noch einmal täte, gibt es nicht.

`/k-remediation` aggregiert deshalb nicht selbst und nimmt genau eine Ergebnisdatei. Es
gruppiert die Findings darin vor der Umsetzung zu Bündeln — nach Risiko, Aufwand,
Quick-Win-Potential und gemeinsamer Verifikation — und gleicht sie vor der Task-Erzeugung
gegen bestehende Tasks ab.

## Verzeichnisse

Rezepte und Ergebnisse sind strikt getrennt:

```text
k-playbook/reviews/                    mitgelieferte Rezepte
k-playbook-local/reviews/              projekteigene Rezepte, Overlay
k-playbook-local/known-decisions.md    bewusst getroffene Entscheidungen, von Hand gepflegt
k-playbook-local/results/              alles, was Reviews erzeugen
```

`reviews/` enthält ausschließlich `review-<name>.md`. Damit bleibt es ein reines
Overlay-Verzeichnis, in dem jede Datei nach derselben Regel behandelt wird: gleicher
Dateiname, lokale Datei gewinnt vollständig.

`known-decisions.md` steht bewusst eine Ebene höher, neben `rules/` und `guidelines/`:
Sie wird von Hand gepflegt und von keinem Review erzeugt — sie ist Eingabe, keine
Ausgabe. Alles Erzeugte liegt daneben:

```text
k-playbook-local/results/
├── log.md                        wann welches Review lief
├── YYYY-MM-DD/                   ein Audit-Lauf
└── <familie>/YYYY-MM-DD/
    ├── review-input.json
    ├── review-triage.md
    ├── run-metadata.json
    └── raw/
```

Beispiel:

```text
k-playbook-local/results/k-check/2026-07-24/
├── review-input.json
├── review-triage.md
├── run-metadata.json
└── raw/
    └── k-check-baseline.txt
```

`k-playbook-local/checks/` bleibt für ausführbare Checks reserviert. Ergebnisse gehören
nie dorthin.

### `results/` wird nicht versioniert

Der **gesamte** Inhalt von `results/` gilt als lokal — nicht nur, was Scanner roh
ausgeben. `raw/` und `entries/`, die erzeugten Dokumente `review-input.md`,
`review-input.json`, `run.json` und `review-triage.md`, die Review-Dokumente je Familie,
dazu `log.md` und, wo noch vorhanden, alte `summary-YYYY-MM-DD.md`.

Der Grund ist derselbe für alle: Ein Review ist aus dem Code wiederholbar. Sein Ergebnis
ist ein Stand von einem Rechner zu einem Zeitpunkt, kein Projektwissen. Bei `log.md`
kommt hinzu, dass es persönlich ist — wer wann auf seinem Rechner gescannt hat, geht das
Projekt nichts an. Und die Rohausgaben eines Secret-Scanners enthalten gefundene Secrets
im Klartext; die gehören unter keinen Umständen ins Repository.

Was vom Ergebnis wirklich Projektwissen ist, wandert ohnehin heraus: in
`k-playbook-local/known-decisions.md` — die genau deshalb eine Ebene höher liegt — und in
die Tasks, die aus einer Remediation entstehen. Der Preis ist bewusst in Kauf genommen:
AI-Bewertungen sind nicht mehr im Repository nachlesbar.

Weil der Zuschnitt damit homogen ist, reicht der übliche verwaltete Ignore-Inhalt (`*`,
`!.gitignore`, `!README.md`) unverändert. `results/` ist deshalb das einzige Verzeichnis,
das k-playbook bei der Installation schon privat anlegt; umschaltbar bleibt es wie
`priv/` und `material/` über den Block **Lokale Einstellungen** der Oberfläche. Für
Bestandsprojekte ändert sich nichts von selbst — die verwaltete `.gitignore` entsteht nur
beim erstmaligen Anlegen des Verzeichnisses.

## Artefakte pro Familie

Jede neue Report-/Scan-Familie erzeugt diese Dateien:

- `review-input.json` — der Belegvertrag. Sein Schema steht an genau einer Stelle:
  `commands/_review-run/review-input-contract.md`. Dort ist auch beschrieben, welche
  Felder nur der Merge füllt und was gilt, wenn sie fehlen.
- `review-triage.md` — einheitliches Endartefakt mit Kopf, Bündel-Tabelle,
  Bündel-Details, Nicht gebündelt und Deckung aus known-decisions.
- `raw/` — auditierbare Originalausgaben, z. B. SARIF, JSON oder Tool-Logs.
- `run-metadata.json` oder äquivalent — auditierbare Laufmetadaten.

Raw-Artefakte und Run-Metadaten sind auditierbar. Sie dürfen nach dem Schreiben nicht
gekürzt, überschrieben oder inhaltlich korrigiert werden. Korrekturen erfolgen über
neue Raw-Dateien plus aktualisierte Bewertung.

`review-triage.md` ist kuratiert. Es darf nachvollziehbar aktualisiert werden, z. B. um
einen Abschnitt `## Remediation-Status`, aber die ursprünglichen Raw-Belege bleiben
unverändert. `assessment.md` und `findings.md` sind Legacy-Artefakte älterer
Ergebnisfamilien und werden nur gelesen, wenn kein `review-triage.md` vorhanden ist.

Für Läufe im neuen Laufmodell (`k-playbook-local/results/YYYY-MM-DD/`) tritt ein zweites
Artefaktpaar daneben: `review-input.json` und `review-input.md` aus `k-playbook merge`.
Sie fassen `raw/` und `entries/` zusammen und dienen als Eingabe für die Bewertung durch
den Assistenten. Details in
[`review-runs.md`](./review-runs.md#zusammenfassen-mit-k-playbook-merge).

Das kuratierbare Endprodukt beider Bewertungswege ist `review-triage.md`: beim Audit
direkt unter `k-playbook-local/results/YYYY-MM-DD/`, beim gezielten Report-Review unter
`k-playbook-local/results/<familie>/YYYY-MM-DD/`. Nur der Scope unterscheidet sich.
Aktive Audit-Katalog-Rezepte tragen je nach `audit.mode` unterschiedlich bei. Eine
**Perspektive** (`mode: perspective`) schreibt zusätzlich je eine Perspektiven-Datei direkt
im Laufordner, etwa `review-secret-scanning.md`; sie liest denselben Merge-Beleg, filtert
über ihren gespeicherten `scope.tools` und dient `scan-triage` nur als Kontext. Eine
**Evidence-Quelle** (`mode: evidence`) schreibt gar kein Markdown, sondern läuft vor dem
Merge und legt `raw/<entry>.sarif` ab; ihre Funde stehen danach als Gruppen mit dem Präfix
`ai-<entry>-` in `review-input.json` und gehen damit denselben Weg wie Scanner-Funde.
`/k-remediation` arbeitet gegen `review-triage.md`;
Legacy-`assessment.md`/`findings.md` bleiben nur Fallback für Family-Ordner ohne
`review-triage.md`.

## Statusmodell

Statuswerte in Legacy-`findings.md`:

| Status | Bedeutung | Remediation-Relevanz |
|---|---|---|
| `open` | neu oder noch nicht geprüft | ja |
| `confirmed` | validierter echter Befund | ja |
| `context-needed` | weitere Kontextprüfung nötig | ja |
| `likely-false-positive` | plausibler Fehlalarm | nur nach expliziter Auswahl |
| `accepted` | bewusste Entscheidung oder akzeptiertes Restrisiko | nein |
| `fixed` | behoben und verifiziert | nein |

Finding-IDs müssen stabil bleiben. Einmal vergebene IDs dürfen bei Re-Runs,
Statusänderungen oder Remediation nicht umbenannt werden.

Bewusste projektweite Entscheidungen stehen in `k-playbook-local/known-decisions.md` und
werden von `k-playbook merge` als Deckung an Findings und Gruppen geschrieben. Das Format,
der Ort und die Ablaufregel stehen in
[`review-runs.md`](./review-runs.md#wirkung-von-known-decisionsmd). Eine Decision ersetzt
keinen Statuswert und filtert nichts aus den Rohdaten; sie macht nur sichtbar, dass ein
Befund durch eine dokumentierte Entscheidung gedeckt ist.

Schema für k-check:

```text
kcheck-<area>-NNN
```

Beispiele: `kcheck-logging-003`, `kcheck-secrets-001`, `kcheck-user-scope-014`.

Scanner-Familien dürfen die nativen Regel-Präfixe ihres Tools behalten, damit ein
Finding zu seiner Rohmeldung zurückverfolgbar bleibt.

## Scanner-Tools vs. k-check

Security-Scanner wie `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft` und `grype`
werden **nicht** als Checks unter `checks/*.sh` modelliert.

Grund:

- Sie erzeugen eigene strukturierte Rohdaten wie JSON, SARIF-ähnliche Reports oder SBOMs.
- Non-zero Exit-Codes bedeuten oft fachliche Findings, nicht technische Fehler.
- Ergebnisse müssen dedupliziert, priorisiert und bewertet werden.
- Raw-Artefakte müssen dauerhaft unter `k-playbook-local/results/<familie>/YYYY-MM-DD/raw/` landen.
- Remediation braucht stabile Finding-IDs, Statuswerte und Quellenbelege.

Darum laufen diese Tools über Report-Mode-Reviews:

| Tool | Review | Ergebnisfamilie |
|---|---|---|
| `gitleaks` | `/k-review secret-scanning` | `secret-scanning` |
| `trufflehog` | `/k-review secret-scanning` | `secret-scanning` |
| `pip-audit` | `/k-review dependency-cve` | `dependency-cve` |
| `trivy` | `/k-review dependency-cve` und `/k-review iac-container` | `dependency-cve` / `iac-container` |
| `syft` | `/k-review iac-container` | `iac-container` |
| `grype` | `/k-review dependency-cve` oder `/k-review iac-container` | `dependency-cve` / `iac-container` |
| GitHub Dependabot Alerts | `/k-review dependabot-alerts` | `dependabot-alerts` |

`checks/*.sh` bleibt für schnelle, generische k-check-Heuristiken und Preflight-artige
Checks reserviert. Kleine Tool-Verfügbarkeitschecks dürfen dort liegen, aber nicht der
eigentliche Scannerlauf mit dauerhafter Bewertung.

## Die Familien im Einzelnen

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

`--output` erhält stdout/stderr und schreibt zusätzlich den vollständigen Raw-Stream.
`--metadata-output` schreibt Kommando, Exit-Code, Zeitstempel, Roots, Modus,
Check-Konfiguration und Version bzw. Git-Commit, soweit verfügbar.

Vorhandene Ziel-Dateien werden nicht überschrieben. Für erneute Läufe am selben Tag
eindeutige Namen verwenden, z. B. `k-check-baseline-e2e.txt` und `run-metadata-e2e.json`.

### Secret-Scanning

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-secret-scanning.md` |
| Ergebnisse | `k-playbook-local/results/secret-scanning/YYYY-MM-DD/` |
| Audit-Perspektive | `k-playbook-local/results/YYYY-MM-DD/review-secret-scanning.md` |
| Audit-Scope | `gitleaks`, `trufflehog` |

Typische Artefakte: `review-input.json`, `review-triage.md`, `raw/gitleaks-*.json`,
`raw/trufflehog.json`.

Die Tools kommen host-lokal aus
`k-playbook/scripts/install-security-tools.sh`. Fehlende Tools werden nicht im Projekt
installiert.

### Dependency-CVE

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-dependency-cve.md` |
| Ergebnisse | `k-playbook-local/results/dependency-cve/YYYY-MM-DD/` |
| Audit-Perspektive | `k-playbook-local/results/YYYY-MM-DD/review-dependency-cve.md` |
| Audit-Scope | `pip-audit`, `trivy`, `grype`, `osv-scanner`, `govulncheck` |

Typische Artefakte: `review-input.json`, `review-triage.md`, `raw/pip-audit.json`,
`raw/trivy-fs.json`, bei Bedarf `raw/grype.json`.

Im Audit-Laufmodell ist dieses Rezept aktiv. Es bewertet die Gruppen aus
`review-input.json`, die mindestens eine Evidence mit einem der Audit-Scope-Tools tragen.

### GitHub Dependabot Alerts

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-dependabot-alerts.md` |
| Ergebnisse | `k-playbook-local/results/dependabot-alerts/YYYY-MM-DD/` |
| Audit-Laufmodell | deaktiviert; Begründung und geprüfte Alternativen im Rezept |

Typische Artefakte: `review-input.json`, `review-triage.md`,
`raw/dependabot-alerts-open.jsonl` als auditierbarer Import,
`raw/dependabot-alerts-summary.tsv` für schnelle Triage.

Diese Familie nutzt GitHub als Quelle. Sie passt besonders, wenn ein Projekt lokale
Dependency-Scanner nicht nutzt oder zuerst die in GitHub vorhandene Alert-Menge bewerten
will. Absichtlich deaktivierte Dependabot-PRs, z. B. `open-pull-requests-limit: 0`, sind
kein Finding.

### IaC/Container

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-iac-container.md` |
| Ergebnisse | `k-playbook-local/results/iac-container/YYYY-MM-DD/` |
| Audit-Perspektive | `k-playbook-local/results/YYYY-MM-DD/review-iac-container.md` |
| Audit-Scope | `trivy`, `syft`, `grype` |

Typische Artefakte: `review-input.json`, `review-triage.md` für Container-, Image-, IaC- und
Filesystem-Findings,
`raw/trivy-*.json`, bei Bedarf `raw/syft-*.json` und `raw/grype-*.json`.

Im Audit-Laufmodell ist dieses Rezept aktiv. Es bewertet die Gruppen aus
`review-input.json`, die mindestens eine Evidence mit einem der Audit-Scope-Tools tragen.

### Tech-Debt

| | |
|---|---|
| Rezept | `k-playbook/reviews/review-tech.md` |
| Ergebnisse | `k-playbook-local/results/tech/YYYY-MM-DD/` |
| Audit-Eintrag | Evidence-Quelle `tech`, SARIF unter `k-playbook-local/results/YYYY-MM-DD/raw/tech.sarif` |

Typische Artefakte im Report-Modus: `review-input.json` und `review-triage.md`. Dieses
Rezept arbeitet in zwei Betriebsarten mit denselben Prüfkriterien; unterschiedlich ist
allein die Ergebnisform. Siehe auch **Rezepte als Evidence-Quelle** weiter unten.

### Rezepte als Evidence-Quelle

Zwei Katalog-Rezepte liefern im Lauf eigene Belege aus dem Code, statt vorhandene zu
filtern. Sie laufen vor dem Merge, lesen ausschließlich in ihrem eingefrorenen
`scope.paths` und schreiben SARIF nach `raw/<entry>.sarif`:

| Rezept | Eintrag | Was es liest |
|---|---|---|
| `review-tech.md` | `tech` | Quell- und Infrastrukturdateien; Tech-Debt-Kandidaten mit `tech-*`-Rule-IDs. |
| `review-python-comment-hardspots.md` | `python-comment-hardspots` | Python-Quellen; Stellen ohne rekonstruierbare Begründung, mit `hardspot-*`-Rule-IDs. |

Beide bleiben daneben über `/k-review` auswählbar — `review-tech` im Report-Modus mit der
Ergebnisfamilie `tech`, `review-python-comment-hardspots` interaktiv. Die Prüfkriterien
sind in beiden Betriebsarten dieselben; unterschiedlich ist nur die Ergebnisform.

### Family-only-Rezepte im Audit-Laufmodell

Einige Katalog-Rezepte bleiben über `/k-review` auswählbar, sind aber für `/k-audit`
deaktiviert, bis ihre Eingaben als Evidence in `review-input.json` vorliegen oder ein
separater Scope-Vertrag existiert:

| Rezept | Grund |
|---|---|
| `review-k-check-security.md` | `k-check`-Ergebnisse sind noch nicht als Evidence im Merge modelliert. |
| `review-dependabot-alerts.md` | Der Input kommt extern über `gh api` und ist noch kein Tool-Eintrag im Lauf-Merge. |

Beide Rezepte tragen die ausführliche Prüfung selbst — welcher Teil des Evidence-Vertrags
sie hindert und was eine Umstellung bräuchte, steht in ihrem Abschnitt **Stellung im
Audit-Laufmodell**. Für den Weg dieser beiden gilt bis dahin: Ihr `review-triage.md` geht
direkt an `/k-remediation`, **ohne** familienübergreifende Zusammenführung und **ohne**
Dedupe gegen andere Quellen. Dasselbe gilt für jeden eigenständigen `/k-review`-Lauf eines
Familienrezepts: Der Family-Ordner liegt außerhalb jedes Laufordners und wird mit dem
Audit-Lauf nicht verrechnet.

## Review-Log

`/k-review` pflegt das Log neben den Ergebnissen:

```text
k-playbook-local/results/log.md
```

Es enthält pro Familie den letzten Lauf, ab wann der nächste fällig ist, Modus und
Fokus sowie eine Protokollzeile mit Scope, Output und Handoff.

Beispiel-Handoff:

```text
/k-remediation k-playbook-local/results/k-check/2026-07-24/review-triage.md
```

`k-playbook-local/known-decisions.md` hält daneben fest, was bewusst so entschieden wurde.
Der Merge-Schritt liest diese eine Datei und schreibt ihre Wirkung sichtbar in
`review-input.json` und `review-input.md`; die Bewertung übernimmt diese Information
anschließend aus dem JSON.

## Remediation

`/k-remediation` versteht diese Eingaben:

- einen Audit-Lauf, `k-playbook-local/results/<datum>/review-triage.md` — der Hauptweg,
- eine Familie, `k-playbook-local/results/<familie>/<datum>/review-triage.md`,
- Legacy: eine vorhandene Summary, `k-playbook-local/results/summary-YYYY-MM-DD.md`,
- Legacy: eine Familie mit `assessment.md` und `findings.md`, wenn dort kein
  `review-triage.md` liegt.

`review-triage.md` ist überall die primäre Arbeitsdatei. `raw/` und `run-metadata.*` sind
read-only.

**Summaries werden nicht mehr erzeugt.** Mit dem nachgelagerten Priorisierungsschritt ist
auch ihr Erzeuger entfallen; weder `/k-audit` noch `/k-review` schreiben sie. Was in einem bereits eingerichteten
Projekt noch liegt, bleibt für `/k-remediation` lesbar — als Eingabe aus der Vergangenheit,
nicht als Ausgabe eines Laufs.

Vor der Task-Erzeugung gleicht `/k-remediation` jeden Befund gegen `tasks/` und
`tasks/done/` ab. Ein bestehender Task deckt einen Befund ab, wenn **Quelle und
Bündel-/Gruppen-ID** übereinstimmen — Titelähnlichkeit allein reicht nicht. Ein Treffer in
`tasks/` verhindert den zweiten Task; ein Treffer in `tasks/done/` wird gemeldet, schließt
den Befund aber nicht: dass er erneut im Ergebnis steht, heißt, dass er wieder da ist.

Ein erzeugter Remediation-Task muss enthalten:

- die Quelle, `k-playbook-local/results/<familie>/<datum>/review-triage.md`,
- die Bündel- oder Gruppen-IDs aus `review-triage.md`,
- das Arbeitsregister `review-triage.md`,
- die Raw-Quelle, falls vorhanden,
- die ursprüngliche Ort-/Message-Angabe,
- alle Findings, die zusammen gelöst werden sollen, wenn es einen gemeinsamen
  Fix-/Verifikationspfad gibt,
- den Remediation-Modus aus `K-PLAYBOOK.yaml`,
- konkrete Verifikationsschritte.

Der Result-Pfad in einem committeten Task ist dabei eine **Herkunftsangabe, keine
auflösbare Referenz**: `results/` wird nicht versioniert, Tasks werden es. Wer den Task
aus dem Repository liest, hat die Ergebnisdatei nicht — und selbst auf dem Rechner, der
sie erzeugt hat, ist sie nach dem nächsten Lauf überschrieben. Deshalb bleibt die
Inline-Evidence Pflicht: Gruppen-IDs, Ort und Message gehören in den Task selbst, nicht
nur als Verweis.

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

- `task-branch-pr` — keine direkten Fixes. Jedes bestätigte Bündel wird eine Task mit
  Branch- und PR-Hinweis; umgesetzt wird später über `/k-task-run`.
- `task-first` — Tasks sind der Standard. Direkte Fixes nur nach ausdrücklicher Freigabe
  für einzelne kleine Bündel. **Das ist der Default.**
- `direct-allowed` — kleine, sichere Befunde dürfen nach Code-Sichtung sofort behoben
  werden, wenn die Kategorien freigegeben sind.

`pr_required` und `direct_fixes` werden aus `mode` abgeleitet und mitgeschrieben.

## Security-Tools

Projekt-venvs sind für Projekt-Abhängigkeiten normal. Der read-only Status darf ein
aktives Projekt-venv messen und kennzeichnet diesen Messkontext. Tool-Installation und
Docker-Fallbacks bleiben davon getrennt host-/user-lokal; vor Installation darf kein
Projekt-venv aktiv sein. Python-CLI-Tools kommen empfohlen über `pipx` oder, mit `--method
venv`, in dedizierte k-playbook-Tool-venvs.

```bash
k-playbook/scripts/install-security-tools.sh                    # Status
k-playbook/scripts/install-security-tools.sh --install missing  # fragt vor der Installation
k-playbook/scripts/install-security-tools.sh --install missing --method venv  # dedizierte Tool-venvs
```

Die Pflicht-Tools stehen kanonisch in
[`../scripts/security-tools.tsv`](../scripts/security-tools.tsv). Skript, Oberfläche und
Review-Rezepte lesen dieselbe Matrix.
