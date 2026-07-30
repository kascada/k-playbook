# Global Checks

`global/bin/k-check` fuehrt wiederverwendbare Checks aus `global/checks/` und projektlokale Checks aus `k-playbook/checks/` aus, wenn das Verzeichnis existiert.

Der Runner trennt zwei Roots:

- `--config-root`: Root mit `K-PLAYBOOK.MD`; darunter wird die feste lokale k-playbook-Struktur erwartet.
- `--target-root`: Codebaum, der gescannt wird; relativ zu `--config-root`, wenn nicht absolut.

## Aufruf

```bash
~/dev/k-playbook/global/bin/k-check --mode changed
~/dev/k-playbook/global/bin/k-check --mode baseline
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --mode changed
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --target-root app --mode baseline
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --target-root app --mode baseline --output /path/to/reviews/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt --metadata-output /path/to/reviews/results/k-check/YYYY-MM-DD/run-metadata.json
```

Der Config-Root ist standardmaessig das aktuelle Arbeitsverzeichnis. Dort wird `K-PLAYBOOK.MD` gelesen; lokale Checks liegen fest unter `k-playbook/checks/`. Der Target-Root ist standardmaessig identisch mit dem Config-Root. Nested Repos werden nicht automatisch als neuer Config-Root interpretiert.

`--output <file>` schreibt die vollstaendige Runner-Ausgabe zusaetzlich zu stdout/stderr in eine Raw-Datei. `--metadata-output <file>` schreibt Run-Metadaten als JSON, inklusive Kommando, Exit-Code, Arbeitsverzeichnis, Datum/Zeit, Roots, Modus, Check-Konfiguration und k-check-Version/Git-Commit soweit verfuegbar. Beide Optionen verweigern vorhandene Ziel-Dateien, damit auditierbare Artefakte nicht still ueberschrieben werden. Fuer Review-Laeufe gehoeren diese Artefakte unter `<reviews>/results/k-check/YYYY-MM-DD/`.

## Check-Schnittstelle

Die stabile Runner-Schnittstelle ist eine `.sh`-Datei direkt im Check-Verzeichnis. Python, Ruby oder andere Implementierungen duerfen dahinter liegen, bleiben aber Implementierungsdetail des jeweiligen `.sh`-Wrappers.

Jeder Check muss genau eine Statuszeile auf stdout schreiben:

```text
K_CHECK_STATUS=ok
K_CHECK_STATUS=skip
K_CHECK_STATUS=fail
```

Optional darf eine Begruendung folgen:

```text
K_CHECK_REASON=<text>
```

Exit-Konvention:

- `0` plus `K_CHECK_STATUS=ok`: bestanden.
- `0` plus `K_CHECK_STATUS=skip`: bewusst nicht anwendbar.
- `1` plus `K_CHECK_STATUS=fail`: fachliche Findings.
- `2`: technischer Check-Fehler.
- Fehlende, doppelte oder widerspruechliche Statuszeilen wertet der Runner als technischen Fehler.

Checks pruefen ihre Anwendbarkeit selbst: Sprache vorhanden, Tool installiert, relevante Dateien vorhanden, Framework erkennbar. Nicht anwendbare Checks skippen sauber und blockieren nicht.

Checks bekommen diese Umgebungsvariablen:

- `K_CHECK_CONFIG_ROOT` — Root mit `K-PLAYBOOK.MD`.
- `K_CHECK_TARGET_ROOT` — gescannter Codebaum.
- `K_CHECK_PROJECT_ROOT` — Kompatibilitaets-Alias fuer `K_CHECK_TARGET_ROOT`.
- `K_CHECK_MODE` — `changed` oder `baseline`.
- `K_CHECK_FILES_FROM` — newline-separierte Dateiliste im Target-Root.
- `K_PLAYBOOK_REPO` — globales k-playbook-Repo.

## Modi

`changed` ermittelt die Dateimenge deterministisch in dieser Prioritaet:

1. `--files-from <file>`.
2. `--base-ref <ref>`.
3. PR-/CI-Base-Ref aus der Umgebung.
4. Git-Merge-Base gegen den Upstream der aktuellen Branch.
5. Uncommitted und staged Aenderungen im Target-Root.

Ist der Target-Root kein Git-Repo, scannt der globale Runner keine nested Repos automatisch; die Dateimenge ist dann leer, ausser `--files-from` wird explizit gesetzt.

`baseline` inventarisiert den Projektbaum ab Target-Root mit Standard-Excludes wie `.git`, virtuelle Environments, `node_modules`, Build-Artefakte und weiteren ueblichen Cache-Verzeichnissen. Nested Git-Worktrees werden dabei nicht automatisch als eigene Projekte betreten.

## Global vs. Lokal

Globale Checks muessen wiederverwendbar bleiben. Projektlokale Checks gehoeren in `<projekt>/k-playbook/checks` und werden genutzt, wenn das Verzeichnis existiert.

Domain-spezifische Begriffe, Modellnamen und Runtime-Dateien gehoeren nicht in globale Checks. Solche Regeln bleiben projektlokal; Test-Fixtures duerfen Domain-Begriffe nur als explizite Negativbeispiele markieren.

## Keine Scanner-Result-Familien

`global/checks/*.sh` ist nicht der Ort fuer schwere Security-Scanner wie `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft` oder `grype` und auch nicht fuer GitHub-Alert-Imports wie Dependabot Alerts.

Diese Tools liefern strukturierte Rohdaten und brauchen Review-Bewertung, Deduplizierung, stabile Finding-IDs und Artefakte unter `reviews/results/<family>/YYYY-MM-DD/`. Sie werden ueber globale Report-Mode Reviews gestartet:

- `/k-review secret-scanning`
- `/k-review dependency-cve`
- `/k-review dependabot-alerts`
- `/k-review iac-container`

Zulaessig in `global/checks/*.sh` sind hoechstens leichte Preflight-Heuristiken, z. B. ob relevante Manifestdateien existieren oder ob ein Tool installiert ist. Der eigentliche Scan gehoert in die passende Review-Familie.
