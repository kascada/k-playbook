# k-playbook Commands

Diese Datei beschreibt die aktuell vorhandenen Setup-, Install-, Workflow- und Hilfs-Commands und grenzt ihre Zustaendigkeiten ab.

Globale Regeln, Review-Rezepte und Checks liegen in diesem Repo unter `global/`. Projektlokale Regeln, Reviews, Checks, Tasks und Docs liegen im jeweiligen Projekt und werden dort ueber `K-PLAYBOOK.MD` registriert.

Der Review-/Results-/Remediation-Flow ist in [`reviews-and-results.md`](./reviews-and-results.md) zusammengefasst.

## Grundregel: Install vs. Setup

`/k-install*` und `/k-setup*` haben absichtlich unterschiedliche Zustaendigkeiten:

- `/k-install` und `/k-install-security-tools` sind **host-global**. Sie machen Commands, Skills und Security-Tools auf diesem Server fuer alle Projekte verfuegbar.
- `/k-install-codeql` ist Tooling-/Artefakt-orientiert. Es installiert oder prueft lokale CodeQL-Artefakte, schreibt aber keine Projektkonfiguration.
- `/k-setup` und `/k-setup-codeql` sind **projektlokal**. Sie schreiben oder aktualisieren Projektkonfigurationen wie `K-PLAYBOOK.MD` und projektlokale Playbook-Pfade.
- Der globale k-playbook-Pfad ist fest `~/dev/k-playbook`. Wenn der physische Klon woanders liegt, wird ein Symlink nach `~/dev/k-playbook` angelegt; Projektkonfigurationen waehlen keinen eigenen Basis-Repo-Pfad.
- Host-Installation schreibt keine Projektdateien.
- Projekt-Setup installiert keine host-globalen Tools.
- `/k-install*` darf nicht in einem aktiven Projekt-venv laufen. Falls `VIRTUAL_ENV` gesetzt ist: zuerst `deactivate`.

Neuer Host:

```text
/k-install
/k-install-security-tools --install missing
```

Neues oder noch nicht registriertes Projekt:

```text
/k-setup
```

Projekt mit CodeQL-Entscheidung:

```text
/k-setup-codeql
```

## Kurzuebersicht

Aktueller Slash-Command-Bestand unter `commands/`: 20 Dateien (`k-*.md`). Neue Dateien werden auf dem Host erst sichtbar, nachdem `/k-install` die OpenCode-Symlinks aktualisiert hat.

| Command | Scope | Projekt-Konfig | Artefakte / Host |
|---------|-------|----------------|------------------|
| `/k-install` | k-playbook auf diesem Host fuer OpenCode registrieren und Security-Tool-Preflight zeigen | keine Aenderung | OpenCode-Symlinks, ggf. Skill-Pfad, nur Tool-Status |
| `/k-install-security-tools` | host-lokale Security-Review-Tools installieren/pruefen | keine Aenderung | `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype` oder Docker-Images |
| `/k-install-codeql` | lokale CodeQL CLI installieren/pruefen, optional lokale DBs analysieren | keine Aenderung an `K-PLAYBOOK.MD` | optional `codeql-cli/`, `databases/`, `results/` |
| `/k-setup` | k-playbook in einem Projekt konfigurieren | schreibt `K-PLAYBOOK.MD` und gewaehlte Playbook-Pfade | keine Host-Aenderung |
| `/k-setup-codeql` | CodeQL-Entscheidung im Projekt registrieren | schreibt CodeQL-Block in `K-PLAYBOOK.MD` | optional CLI-only Artefakt unter `codeql-cli/` |
| `/k-status` | read-only Health-Check fuer Projekt und host-lokale OpenCode-Registrierung | keine Aenderung | prueft u. a. Command-Symlinks und `skills.paths` |
| `/k-task-create` | strukturierte Task-Datei aus Gespraechskontext erzeugen | liest `tasks:` | schreibt `<tasks>/<NNN>-<slug>.md` nach Bestaetigung |
| `/k-run` | Task-Dateien sequenziell ausfuehren | liest `tasks:` und `K-PLAYBOOK.MD`-Kontext | delegiert an Subagenten, schreibt Ausfuehrungsnotiz, verschiebt erfolgreiche Tasks nach `done/` |
| `/k-review-loop` | Task-/Instruktionsdateien vor Ausfuehrung per Critic/Editor-Dialog pruefen | liest optional `tasks:` | Moderator schreibt akzeptierte Task-Edits und Review-Log |
| `/k-todo` | Projekt-TODO anzeigen oder Eintrag ergaenzen | liest `todo:` | schreibt/ergaenzt `TODO.md` bzw. den registrierten Todo-Pfad |
| `/k-review` | globale oder projektlokale Review-Rezepte ausfuehren | liest `reviews:` und `known-decisions.md` | interaktive Aenderungen oder Report-Artefakte unter `<reviews>/results/<family>/YYYY-MM-DD/` |
| `/k-results` | vorhandene Review-Results projektweit priorisieren | liest `reviews:` und optional `tasks:` | schreibt `<reviews>/results/summary-YYYY-MM-DD.md` |
| `/k-remediation` | Review-Findings planen, gruppieren und abarbeiten | liest `reviews:`, `tasks:` und Remediation-Policy | erzeugt Tasks, aktualisiert Findings/Assessment oder macht freigegebene direkte Fixes |
| `/k-code2docs` | semantische Projekt-Doku erzeugen und fuer AI-Sessions registrieren | liest `docs:` | schreibt `<docs>/*.md`, `<docs>/README.md`, `AGENTS.md`, `opencode.json` |
| `/k-tools-scan` | Library-/Tool-Doku nach `/k-code2docs` ergaenzen | liest `docs:` | schreibt `<docs>/libs/*.md`, `libs/README.md`, aktualisiert Hauptindex |
| `/k-enforcement` | expliziter Check gegen globale und projektlokale Regeln | liest `enforcement:` und `docs:` | read-only Bericht; Fixes nur nach expliziter User-Freigabe |
| `/k-test-check` | Tests ausfuehren und Fehlerursachen diagnostizieren | keine eigene Pfad-Konfig | startet Tests, macht Diagnose, fragt vor Fixes |
| `/k-verlauf` | alte AI-Verlaeufe durchsuchen | keine Projektdatei noetig | liest Claude-JSONL bzw. OpenCode-Logs read-only |
| `/k-logmcp` | LogMCP-Zugriff fuer Claude-Code-Projekte einrichten | schreibt ggf. `.claude/CLAUDE.md` und `.claude/settings.local.json` | prueft MCP-Zugriff und merkt Server/Permissions |
| `/k-vscode-project-color` | VS-Code-Fensterfarbe/-Titel pro Projekt setzen | keine `K-PLAYBOOK.MD`-Pflicht | schreibt/merged `.vscode/settings.json` |
| `global/bin/k-check` | globale und projektlokale Checks ausfuehren | liest `checks:` aus `K-PLAYBOOK.MD` | stdout/stderr; optional Raw-Output + Run-Metadaten fuer Review-Artefakte |

## `/k-install`

`/k-install` installiert oder aktualisiert die globale k-playbook-Registrierung auf dem aktuellen Server.

Der Command:

- legt Symlinks von `commands/k-*.md` nach `~/.config/opencode/command/` an
- prueft, ob das k-playbook-Repo in `skills.paths` der OpenCode-Konfig registriert ist
- fuehrt am Ende einen lesenden Security-Tool-Preflight aus
- veraendert keine Projektdateien
- schreibt kein `K-PLAYBOOK.MD`

Typische Nutzung:

- einmal pro Server nach dem Klonen von `k-playbook`
- erneut nach neuen oder umbenannten Dateien unter `commands/k-*.md`

Aufrufort:

- Bevorzugt im k-playbook-Repo nach Clone oder Pull.
- Aus einem Zielprojekt ist erlaubt; der feste Pfadvertrag `~/dev/k-playbook` gilt trotzdem.
- Der Effekt ist trotzdem immer host-global; das Zielprojekt wird nicht geaendert.
- Wenn der Klon woanders liegt, soll `/k-install` vorschlagen, ihn nach `~/dev/k-playbook` zu legen oder nach Bestaetigung einen Symlink dorthin anzulegen.

Wenn Pflicht-Tools fuer Security-Reviews fehlen, installiert `/k-install` sie nicht selbst, sondern nennt den Folge-Command:

```text
/k-install-security-tools --install missing
```

## `/k-install-security-tools`

`/k-install-security-tools` ist der host-lokale Installer/Preflight fuer Security-Review-Tools, die alle Projekte ueber das globale k-playbook verwenden.

Pflicht-Tools:

- `gitleaks` und `trufflehog` fuer Secret-Scanning.
- `pip-audit` fuer Python Dependency-CVEs.
- `trivy` fuer Filesystem-, Container-, IaC- und CVE-Scans.
- `syft` fuer SBOMs.
- `grype` fuer SBOM-/Dependency-CVE-Auswertung.

Preflight ohne Installation:

```text
/k-install-security-tools
```

Fehlende Pflicht-Tools installieren, nach expliziter Bestaetigung:

```text
/k-install-security-tools --install missing
```

Installationswege:

- `--method auto`: native GitHub-Release-Binaries fuer CLI-Tools; `pip-audit` via `pipx`, sonst dediziertes k-playbook Tool-venv.
- `--method docker`: pullt offizielle Docker-Fallback-Images, soweit definiert; `pip-audit` wird dabei per dediziertem Tool-venv bereitgestellt.
- `--method pipx` oder `--method venv`: gezielt fuer `pip-audit`; `venv` meint immer ein dediziertes Tool-venv, nie ein Projekt-venv.

Der Command bricht ab, wenn ein Python-venv aktiv ist, und schreibt keine Projektdateien, installiert nichts in `.venv/`, `venv/` oder `env/` und startet keine Scans. Review-Familien wie Secret-Scanning, Dependency-CVE und IaC/Container konsumieren spaeter nur diese host-lokal verfuegbaren Tools. GitHub Dependabot Alerts werden separat ueber `/k-review dependabot-alerts` und `gh api` importiert; dafuer ist kein lokaler Scanner noetig.

Die zugehoerigen Review-Rezepte sind globale Report-Mode-Reviews:

- `/k-review secret-scanning`
- `/k-review dependency-cve`
- `/k-review dependabot-alerts`
- `/k-review iac-container`

Jede dieser Familien erzeugt ein `assessment.md` mit bewerteter Liste und ein `findings.md` als statusfaehiges Arbeitsregister unter `<reviews>/results/<family>/YYYY-MM-DD/`.

Die Scanner selbst werden nicht als `global/checks/*.sh` aufgerufen. `k-check` bleibt fuer leichte generische Checks und Heuristiken; `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype` und GitHub Dependabot Alerts laufen ueber die passenden `/k-review`-Report-Familien, damit Raw-Artefakte, Run-Metadaten, stabile Finding-IDs und Priorisierung erhalten bleiben.

## `/k-setup`

`/k-setup` installiert oder aktualisiert die k-playbook-Konfiguration in einem konkreten Projekt.

Der Command:

- legt oder aktualisiert `K-PLAYBOOK.MD` im Projekt-Root
- registriert projektlokale Pfade wie `tasks`, `todo`, `checks`, `reviews`, `guidelines`, `enforcement`, `docs`
- erstellt bestaetigte Verzeichnisse oder Initialdateien
- fuehrt keine Tasks, Reviews, Checks oder CodeQL-Analysen aus
- veraendert keine globale OpenCode-Registrierung

`K-PLAYBOOK.MD` ist dabei eine Pointer-/Config-Datei. Spaetere Commands lesen daraus, wo die projektlokalen Bausteine liegen.

`/k-setup` schreibt `repo: ~/dev/k-playbook` in den Managed Block. Dieser Wert ist nicht interaktiv waehlbar; Abweichungen werden ueber Symlinks geloest.

## `/k-setup-codeql`

`/k-setup-codeql` gehoert zur Projektkonfiguration und besitzt den CodeQL-Block in `K-PLAYBOOK.MD`.

Der Command fragt getrennt ab:

- ob GitHub CodeQL aktiv, inaktiv oder geplant ist
- ob eine lokale CodeQL-Datenbank aktiv, inaktiv oder geplant ist
- welches Projekt-/Git-Verzeichnis CodeQL analysieren soll (`target:`), z. B. `./app` bei Wrapper-Repos
- welche Sprachen und Queries registriert werden sollen

Bei GitHub CodeQL darf der Command nur eine lokale CLI-only Installation fuer Preflight- und Statuschecks anbieten. Der erlaubte Script-Aufruf ist:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" --parent "<PLAYBOOK_BASE_DIR>" --cli-only
```

Dieser Pfad darf nur `codeql-cli/` anlegen oder wiederverwenden. Er darf keine lokalen Datenbanken erzeugen, keine SARIF-Dateien schreiben und keine Analyse ausfuehren.

## `/k-install-codeql`

`/k-install-codeql` ist der lokale CodeQL-Tooling-Command. Er aendert `K-PLAYBOOK.MD` nicht und konfiguriert GitHub CodeQL nicht.

Mit `--cli-only` installiert oder prueft er nur die CLI:

```bash
/k-install-codeql --cli-only
```

Intern nutzt er:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" --parent "<PARENT_DIR>" --cli-only
```

Ohne `--cli-only` kann der Command lokale CodeQL-Datenbanken und SARIF-Ergebnisse erzeugen:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-codeql-local.sh" \
  --project "<CODEQL_TARGET_DIR>" \
  --parent "<PARENT_DIR>" \
  --languages "<LANGUAGES>" \
  --queries "<QUERIES>"
```

Dieser Full-Local-Modus ist fuer Projekte gedacht, die bewusst lokale CodeQL-Analysen ausfuehren wollen.

## Abgrenzung

Die Trennung ist absichtlich:

- `/k-install` ist host-global und macht Commands/Skills fuer OpenCode sichtbar.
- `/k-install-security-tools` ist host-global und macht Security-Review-Tools verfuegbar.
- `/k-setup` ist projektlokal und schreibt die zentrale Projekt-Konfig.
- `/k-setup-codeql` trifft und dokumentiert die CodeQL-Entscheidung.
- `/k-install-codeql` installiert oder betreibt lokale CodeQL-Artefakte, ohne Projekt-Konfig zu schreiben.

Dadurch startet ein Setup-Command nicht versehentlich langlaufende Analysen oder erzeugt grosse lokale Artefakte.

## `/k-status`

`/k-status` ist read-only. Neben Projektstatus aus `K-PLAYBOOK.MD` prueft der Command auch die host-lokale OpenCode-Registrierung:

- ob `~/.config/opencode/command/k-*.md` auf die erwarteten Dateien unter `<PLAYBOOK_REPO>/commands/` zeigt.
- ob verwaiste k-playbook-Symlinks existieren.
- ob `skills.paths` das k-playbook-Repo plausibel enthaelt.

`/k-status` repariert nichts. Wenn Symlinks oder Skill-Pfad unvollstaendig sind, ist `/k-install` die naechste Aktion.

## Task-Commands: `/k-task-create`, `/k-run`, `/k-review-loop`, `/k-todo`

Diese Commands bilden die Task-Pipeline. Sie raten keine Projektpfade, sondern lesen `K-PLAYBOOK.MD`.

`/k-task-create [short-name]` erzeugt aus dem Gespraechskontext eine neue Task-Datei im registrierten `tasks:`-Pfad. Der Command bestimmt die naechste freie Nummer ueber offene Tasks und `done/`, zeigt den Entwurf zuerst und speichert erst nach Bestaetigung.

`/k-run [file-or-directory]` fuehrt Task-Dateien strikt sequenziell aus. Ohne Argument nutzt er den registrierten `tasks:`-Pfad; mit explizitem Datei- oder Directory-Argument ist ein One-off-Lauf moeglich. Erfolgreiche Tasks bekommen eine Ausfuehrungsnotiz und werden nach `done/` verschoben. Bei Abbruch bleibt die Task-Datei offen.

`/k-run` respektiert `## Ausfuehrungskontext` in Tasks. Wenn dort `Target repo`, `Base branch`, `Work branch` oder `PR required` stehen, fuehrt der Command vor Delegation einen Branch-/Dirty-Worktree-Preflight aus. Bei `PR required: true` gehoert PR-Handoff nach erfolgreichem Task zum Flow.

`/k-review-loop [path]` prueft Task- oder Instruktionsdateien vor der Ausfuehrung. Critic und Editor laufen read-only; nur der Moderator schreibt akzeptierte minimale Edits und haengt ein Review-Log an. Ohne Argument nutzt der Command den registrierten `tasks:`-Pfad.

`/k-todo [todo text]` liest oder ergaenzt die projektlokale Todo-Datei aus `todo:`. Ohne Argument wird der Inhalt angezeigt; mit Text wird ein neuer `- [ ] ...`-Eintrag angehaengt.

## `/k-review`

`/k-review [review-name]` orchestriert globale und projektlokale Review-Rezepte.

Der Command:

- liest globale Rezepte aus `<PLAYBOOK_REPO>/global/reviews/`.
- liest projektlokale Rezepte aus `reviews:`, falls aktiv.
- laesst projektlokale Rezepte globale mit gleichem Namen ueberlagern.
- laedt `known-decisions.md`, falls vorhanden, damit bewusste Entscheidungen nicht erneut als Findings gemeldet werden.
- schreibt im Projekt-Review-Pfad `log.md`, wenn `reviews:` aktiv ist.

Interaktive Reviews moderieren Stelle fuer Stelle: Vorschlag zeigen, User-Freigabe abwarten, dann erst aendern. Report-Mode-Reviews mit `handoff` schreiben ein Ergebnisartefakt. Bei `result-family` landet es unter `<reviews>/results/<family>/YYYY-MM-DD/` mit typischer Struktur `assessment.md`, `findings.md`, `raw/` und Run-Metadaten.

Aktuelle globale Review-Rezepte:

- `/k-review codeql-security`
- `/k-review k-check-security`
- `/k-review secret-scanning`
- `/k-review dependency-cve`
- `/k-review dependabot-alerts`
- `/k-review iac-container`
- `/k-review tech`
- `/k-review python-comment-hardspots`

## `/k-remediation`

`/k-remediation [result-datei.md]` arbeitet Findings aus Review-Ergebnissen oder priorisierten Summaries ab.

Unterstuetzte Eingaben:

- Legacy-Dateien wie `<reviews>/result-*.md`.
- Result-Familien wie `<reviews>/results/<family>/<date>/assessment.md` mit zugehoerigem `findings.md`.
- Von `/k-results` erzeugte Summaries unter `<reviews>/results/summary-YYYY-MM-DD.md`.

Der Command liest `reviews:`, `tasks:` und die Remediation-Policy aus `K-PLAYBOOK.MD`. Er gruppiert Findings zuerst zu sinnvollen Buendeln nach Risiko, Aufwand, Kopplung und gemeinsamer Verifikation. Je nach Policy entstehen Tasks mit Branch-/PR-Hinweis, oder kleine direkte Fixes sind nur nach expliziter Freigabe erlaubt.

Wichtig: `raw/` und Run-Metadaten von Result-Familien bleiben read-only. Status, Triage-Notizen, Task-Verweise und Remediation-Logs werden in `findings.md` bzw. nachvollziehbar in `assessment.md` gepflegt.

## `/k-code2docs` und `/k-tools-scan`

`/k-code2docs` erzeugt die initiale Docs-First-Dokumentation im in `K-PLAYBOOK.MD` registrierten `docs:`-Pfad und registriert sie fuer spaetere AI-Sessions ueber `AGENTS.md` und `opencode.json`.

`/k-tools-scan` ist der zweite Docs-Schritt. Er ergaenzt unter `<docs>/libs/` pitfall-fokussierte Referenzen zu nicht-trivialen Libraries und verlinkt sie im Hauptindex.

Neue von diesen Commands erzeugte Doc-Dateien bleiben normale Markdown-Dateien, enthalten aber leichtgewichtig OKF-kompatibles YAML-Frontmatter. Pflichtanker fuer Menschen und OpenCode bleibt `<docs>/README.md`; ein OKF-`index.md` wird nicht als Ersatz eingefuehrt.

## `/k-results`

`/k-results` ist der Zwischenschritt zwischen Report-Reviews und Remediation.

Der Command:

- liest vorhandene Result-Familien unter `<reviews>/results/<family>/<date>/`.
- nutzt pro Familie standardmaessig das neueste `assessment.md` plus `findings.md`.
- dedupliziert Themen ueber Familien hinweg.
- beruecksichtigt `known-decisions.md` und vorhandene Tasks, soweit registriert.
- schreibt `<reviews>/results/summary-YYYY-MM-DD.md`.
- startet keine Scanner und keine Remediation.

Typischer Ablauf:

```text
/k-review codeql-security
/k-review k-check-security
/k-review secret-scanning
/k-review dependency-cve
/k-review dependabot-alerts
/k-review iac-container
/k-results
/k-remediation k-playbook/reviews/results/summary-YYYY-MM-DD.md
```

Es gibt aktuell keine eigene `commands/k-result.md`; der dokumentierte Command ist `/k-results`.

## `/k-enforcement`

`/k-enforcement [target-dir]` ist der explizite Check gegen k-playbook-Regeln. Der gleichnamige Skill `ks-enforcement` wendet diese Regeln waehrend Implementierungsarbeit laufend an; der Slash-Command ist fuer Mid-work- oder Abschlusschecks gedacht.

Der Command:

- laedt globale Regeln aus `<PLAYBOOK_REPO>/global/rules/`.
- laedt projektlokale Regeln aus `enforcement:`, falls aktiv.
- prueft bei Code-Aenderungen immer die Docs-Sync-Regel gegen `docs:`, `README.md`, `AGENTS.md` und offensichtliche Architektur-/Setup-/API-Doku.
- bleibt read-only, solange der User nicht explizit einen Fix verlangt.

Wenn `enforcement:` oder `docs:` inaktiv ist, werden keine Default-Pfade erfunden; globale Regeln gelten trotzdem.

## `/k-test-check`

`/k-test-check [path-or-command]` fuehrt Tests aus, diagnostiziert Fehlschlaege und fragt erst danach, ob Fixes gemacht werden sollen.

Ohne Argument erkennt der Command typische Test-Frameworks, z. B. `pytest`, Jest/Vitest, `make test`, Go oder Rust. Mit Argument nutzt er einen expliziten Pfad oder Testbefehl. Bei roten Tests liest er relevante Test- und Source-Dateien, ordnet die Ursache ein und meldet knapp, ob Test, Code, Umgebung oder ein gebrochener Vertrag betroffen ist.

## Hilfs-Commands: `/k-verlauf`, `/k-logmcp`, `/k-vscode-project-color`

`/k-verlauf [claude|opencode|all] <Suchbegriff> [zeitraum] [-all]` durchsucht alte AI-Verlaeufe. Claude wird ueber `~/.claude/projects/**/*.jsonl` durchsucht; OpenCode ueber Log-/Session-Metadaten in `opencode.log`, nicht zwingend ueber vollstaendige Chattexte. Der Command ist read-only und gibt Treffergruppen mit Zeit, Projekt/Directory und Snippets aus.

`/k-logmcp [server]` richtet in Claude-Code-Projekten LogMCP-Zugriff ein. Er erkennt oder fragt den `logmcp-<hostname>`-Server, prueft MCP-Tools wie `list_logs`, ergaenzt bei Bedarf `.claude/settings.local.json` um das Wildcard-Permission-Muster `mcp__logmcp-*` und merkt den Server in `.claude/CLAUDE.md`.

`/k-vscode-project-color [project-name]` schreibt oder merged eine projektlokale `.vscode/settings.json`, damit VS-Code-Fenster anhand Titel und Farbe unterscheidbar sind. Der Command ist nicht an `K-PLAYBOOK.MD` gebunden und erhaelt bestehende unrelated Settings.

## `global/bin/k-check`

`global/bin/k-check` ist kein Slash-Command, sondern ein wiederverwendbarer CLI-Entry-Point fuer globale und projektlokale Checks.

Typische Nutzung aus einem Projekt-Root:

```bash
~/dev/k-playbook/global/bin/k-check --mode changed
~/dev/k-playbook/global/bin/k-check --mode baseline
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --mode changed
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --target-root app --mode baseline --output /path/to/reviews/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt --metadata-output /path/to/reviews/results/k-check/YYYY-MM-DD/run-metadata.json
```

Der Runner liest `K-PLAYBOOK.MD` am Projekt-Root, fuehrt `.sh`-Checks aus `global/checks/` und projektlokale `.sh`-Checks aus dem registrierten `checks:`-Pfad aus. `K-PLAYBOOK.MD` bleibt dabei Pointer-/Config-Datei; Runner-Logik liegt im globalen Repo.

`--output <file>` erhaelt das normale Terminalverhalten und schreibt die vollstaendige Raw-Ausgabe zusaetzlich in eine Datei. `--metadata-output <file>` schreibt JSON-Metadaten zum Lauf, darunter Kommando, Exit-Code, Arbeitsverzeichnis, Datum/Zeit, Roots, Modus, Check-Konfiguration und k-check-Version/Git-Commit soweit verfuegbar. Beide Optionen verweigern vorhandene Ziel-Dateien. Fuer `/k-review k-check-security` gehoeren diese Dateien nach `reviews/results/k-check/YYYY-MM-DD/`.

Die stabile Check-Schnittstelle ist `.sh`. Einzelne Checks duerfen Python oder andere Tools intern verwenden, muessen aber am Ende genau eine Statuszeile `K_CHECK_STATUS=ok|skip|fail` und optional `K_CHECK_REASON=<text>` schreiben.

`changed` nutzt der Reihe nach `--files-from`, `--base-ref`, CI-/PR-Base-Refs, Upstream-Merge-Base und zuletzt uncommitted/staged Git-Aenderungen. Ist der Projekt-Root kein Git-Repo, werden nested Repos nicht automatisch gescannt. `baseline` scannt den Projektbaum mit Standard-Excludes wie `.git`, virtuelle Environments, `node_modules` und Build-Artefakte.

Globale Checks muessen domain-neutral bleiben. Projekt- oder produktbezogene Regeln, Modellnamen und Runtime-Dateien gehoeren in projektlokale Checks.
