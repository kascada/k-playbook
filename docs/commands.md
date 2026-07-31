# k-playbook Commands

Diese Datei beschreibt die aktuell vorhandenen Setup-, Install-, Workflow- und Hilfs-Commands und grenzt ihre Zustaendigkeiten ab.

Globale Regeln, Review-Rezepte und Checks liegen in diesem Repo unter `global/`. Projektlokale Regeln, Reviews, Checks, Tasks und Docs liegen im jeweiligen Projekt unter dem festen `k-playbook/`-Layout; Projektentscheidungen werden ueber `K-PLAYBOOK.yaml` registriert.

Der Review-/Results-/Remediation-Flow ist in [`reviews-and-results.md`](./reviews-and-results.md) zusammengefasst.

Die empfohlene Reihenfolge fuer Host, mehrere Zielprojekte und DevContainer steht in [`multi-project-installation.md`](./multi-project-installation.md). Das Handbuch enthaelt die kompakte Command-Uebersicht in [`handbuch.md`](./handbuch.md#commands).

## Grundregel: Install vs. Setup

Installer-GUI, host-lokale Tool-Commands und projektlokale Spezialcommands haben absichtlich unterschiedliche Zustaendigkeiten:

- Der k-playbook Installer ist **host-global** fuer Registrierung und Projekt-Onboarding: Pfadvertrag, OpenCode-/Claude-Registrierung, Projektliste, `K-PLAYBOOK.yaml`, feste Projektstruktur und Remediation-Default.
- `/k-install-security-tools` ist **host-global** und installiert/prueft Security-Tools fuer alle Projekte.
- `/k-install-codeql` ist Tooling-/Artefakt-orientiert. Es installiert oder prueft lokale CodeQL-Artefakte, schreibt aber keine Projektkonfiguration.
- `/k-setup-codeql` ist **projektlokal** und schreibt die CodeQL-Entscheidung.
- Der globale k-playbook-Pfad ist fest `~/dev/k-playbook`. Wenn der physische Klon woanders liegt, wird ein Symlink nach `~/dev/k-playbook` angelegt; Projektkonfigurationen waehlen keinen eigenen Basis-Repo-Pfad.
- Host-Registrierung schreibt keine Projektdateien ausser der lokalen Installer-Projektliste; Projekt-Onboarding schreibt bewusst ins Zielprojekt.
- Projekt-Setup installiert keine host-globalen Tools.
- `/k-install-security-tools` darf nicht in einem aktiven Projekt-venv laufen. Falls `VIRTUAL_ENV` gesetzt ist: zuerst `deactivate`.

Neuer Host:

```text
k-playbook-installer
/k-install-security-tools --install missing  # optional, wenn Pflicht-Tools fehlen
```

Neues oder noch nicht registriertes Projekt:

```text
k-playbook Installer starten und Projekt hinzufuegen
```

Projekt mit CodeQL-Entscheidung:

```text
/k-setup-codeql
```

## Kurzuebersicht

Aktueller Slash-Command-Bestand unter `commands/`: neue Dateien werden auf dem Host erst sichtbar, nachdem die Installer-GUI die OpenCode-/Claude-Registrierung aktualisiert hat.

| Command | Scope | Projekt-Konfig | Artefakte / Host |
|---------|-------|----------------|------------------|
| **Install** | | | |
| `/k-install-security-tools` | host-lokale Security-Review-Tools aus `global/security-tools.tsv` installieren/pruefen | keine Aenderung | Pflicht-Scanner oder Docker-Images laut Tool-Matrix |
| `/k-install-codeql` | lokale CodeQL CLI installieren/pruefen, optional lokale DBs analysieren | keine Aenderung an `K-PLAYBOOK.yaml` | optional `codeql-cli/`, `databases/`, `results/` |
| `/k-setup-codeql` | CodeQL-Entscheidung im Projekt registrieren | schreibt `tools.codeql` in `K-PLAYBOOK.yaml` | optional CLI-only Artefakt unter `codeql-cli/` |
| `/k-code2docs` | semantische Projekt-Doku erzeugen und fuer AI-Sessions registrieren | nutzt `k-playbook/docs` | schreibt `k-playbook/docs/*.md`, `k-playbook/docs/README.md`, `AGENTS.md`, `opencode.json` |
| `/k-tools-scan` | Library-/Tool-Doku nach `/k-code2docs` ergaenzen | nutzt `k-playbook/docs` | schreibt `k-playbook/docs/libs/*.md`, `libs/README.md`, aktualisiert Hauptindex |
| `/k-status` | read-only Health-Check fuer Projekt und host-lokale OpenCode-Registrierung | keine Aenderung | prueft u. a. Command-Symlinks und `skills.paths` |
| `/k-gui` | lokale k-playbook Installer-GUI starten | keine Aenderung | startet `~/.local/bin/k-playbook-installer` im Vordergrund |
| **Code-Review** | | | |
| `/k-review` | globale oder projektlokale Review-Rezepte ausfuehren | nutzt `k-playbook/reviews` und `known-decisions.md` | interaktive Aenderungen oder Report-Artefakte unter `k-playbook/reviews/results/<family>/YYYY-MM-DD/` |
| `/k-results` | vorhandene Review-Results projektweit priorisieren | nutzt `k-playbook/reviews` und `k-playbook/tasks` | schreibt `k-playbook/reviews/results/summary-YYYY-MM-DD.md` |
| `/k-remediation` | Review-Findings planen, gruppieren und abarbeiten | nutzt `k-playbook/reviews`, `k-playbook/tasks` und Remediation-Policy | erzeugt Tasks, aktualisiert Findings/Assessment oder macht freigegebene direkte Fixes |
| **Task-Flow** | | | |
| `/k-task-create` | strukturierte Task-Datei aus Gespraechskontext erzeugen | nutzt `k-playbook/tasks` | schreibt `k-playbook/tasks/<NNN>-<slug>.md` nach Bestaetigung |
| `/k-review-loop` | Task-/Instruktionsdateien vor Ausfuehrung per Critic/Editor-Dialog pruefen | nutzt optional `tasks` | Moderator schreibt akzeptierte Task-Edits und Review-Log |
| `/k-run` | Task-Dateien sequenziell ausfuehren | nutzt `k-playbook/tasks` und `K-PLAYBOOK.yaml`-Kontext | delegiert an Subagenten, schreibt Ausfuehrungsnotiz, verschiebt erfolgreiche Tasks nach `done/` |
| **Nuetzliches** | | | |
| `/k-verlauf` | alte AI-Verlaeufe durchsuchen | keine Projektdatei noetig | liest Claude-JSONL bzw. OpenCode-Logs read-only |
| `/k-vscode-project-color` | VS-Code-Fensterfarbe/-Titel pro Projekt setzen | keine `K-PLAYBOOK.yaml`-Pflicht | schreibt/merged `.vscode/settings.json` |
| **Weitere** | | | |
| `/k-todo` | Projekt-TODO anzeigen oder Eintrag ergaenzen | nutzt `k-playbook/TODO.md` | schreibt/ergaenzt `k-playbook/TODO.md` |
| `/k-enforcement` | expliziter Check gegen globale und projektlokale Regeln | nutzt `enforcement` und `docs`, falls aktiv | read-only Bericht; Fixes nur nach expliziter User-Freigabe |
| `/k-test-check` | Tests ausfuehren und Fehlerursachen diagnostizieren | keine eigene Pfad-Konfig | startet Tests, macht Diagnose, fragt vor Fixes |

## Installer-GUI

Die Installer-GUI ist der normale Weg fuer Host-Registrierung und Projekt-Onboarding.

Sie wird ueber das installierte Binary gestartet:

```text
k-playbook-installer
```

Oder aus OpenCode heraus:

```text
/k-gui
```

Die GUI:

- prueft und repariert den Pfadvertrag `~/dev/k-playbook`,
- registriert OpenCode- und Claude-Commands/Skills,
- prueft Security-Tools read-only,
- verwaltet die lokale Projektliste,
- erzeugt `K-PLAYBOOK.yaml` und die feste Projektstruktur im Zielprojekt,
- zeigt und aktualisiert Remediation-Policy pro Projekt,
- verwaltet DevContainer-Integration pro Projekt.

## `/k-install-security-tools`

`/k-install-security-tools` ist der host-lokale Installer/Preflight fuer Security-Review-Tools, die alle Projekte ueber das globale k-playbook verwenden. Die kanonische Liste liegt in `global/security-tools.tsv` und wird auch von der Installer-GUI gelesen.

Aktuelle Pflicht-Tools laut Matrix:

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

Jede dieser Familien erzeugt ein `assessment.md` mit bewerteter Liste und ein `findings.md` als statusfaehiges Arbeitsregister unter `k-playbook/reviews/results/<family>/YYYY-MM-DD/`.

Die Scanner selbst werden nicht als `global/checks/*.sh` aufgerufen. `k-check` bleibt fuer leichte generische Checks und Heuristiken; die Security-Scanner aus `global/security-tools.tsv` und GitHub Dependabot Alerts laufen ueber die passenden `/k-review`-Report-Familien, damit Raw-Artefakte, Run-Metadaten, stabile Finding-IDs und Priorisierung erhalten bleiben.

## `/k-setup-codeql`

`/k-setup-codeql` gehoert zur Projektkonfiguration und besitzt `tools.codeql` in `K-PLAYBOOK.yaml`.

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

`/k-install-codeql` ist der lokale CodeQL-Tooling-Command. Er aendert `K-PLAYBOOK.yaml` nicht und konfiguriert GitHub CodeQL nicht.

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

- Die Installer-GUI ist host-global und macht Commands/Skills fuer OpenCode und Claude sichtbar.
- `/k-install-security-tools` ist host-global und macht Security-Review-Tools verfuegbar.
- Der k-playbook Installer schreibt die zentrale Projekt-Konfig und feste Projektstruktur.
- `/k-setup-codeql` trifft und dokumentiert die CodeQL-Entscheidung.
- `/k-install-codeql` installiert oder betreibt lokale CodeQL-Artefakte, ohne Projekt-Konfig zu schreiben.

Dadurch startet ein Setup-Command nicht versehentlich langlaufende Analysen oder erzeugt grosse lokale Artefakte.

## `/k-status`

`/k-status` ist read-only. Neben Projektstatus aus `K-PLAYBOOK.yaml` prueft der Command auch die host-lokale OpenCode-Registrierung:

- ob `~/.config/opencode/command/k-*.md` auf die erwarteten Dateien unter `<PLAYBOOK_REPO>/commands/` zeigt.
- ob verwaiste k-playbook-Symlinks existieren.
- ob `skills.paths` das k-playbook-Repo plausibel enthaelt.

`/k-status` repariert nichts. Wenn Symlinks oder Skill-Pfad unvollstaendig sind, ist die Installer-GUI die naechste Aktion.

Fuer maschinenlesbaren Projektstatus startet `/k-status json` bevorzugt das Installer-Binary im aktuellen Projekt:

```bash
k-playbook-installer status
```

Die Ausgabe ist JSON und enthaelt die in der Installer-GUI genutzten Projektfelder plus leichte Statusbereiche wie `playbook`, `tasks`, `todo`, `reviews`, `enforcement`, `git` und `recommendations`. `/k-status` startet keine Tests, Builds, Smoke-Tests, Scanner oder Analysen; solche Pruefungen bleiben separaten Commands vorbehalten.

## Task-Commands: `/k-task-create`, `/k-run`, `/k-review-loop`, `/k-todo`

Diese Commands bilden die Task-Pipeline. Sie raten keine Projektpfade, sondern lesen `K-PLAYBOOK.yaml` als Projekt-Kontext und nutzen die festen Unterverzeichnisse.

`/k-task-create [short-name]` erzeugt aus dem Gespraechskontext eine neue Task-Datei unter `k-playbook/tasks/`. Der Command bestimmt die naechste freie Nummer ueber offene Tasks und `done/`, zeigt den Entwurf zuerst und speichert erst nach Bestaetigung.

`/k-run [file-or-directory]` fuehrt Task-Dateien strikt sequenziell aus. Ohne Argument nutzt er `k-playbook/tasks/`; mit explizitem Datei- oder Directory-Argument ist ein One-off-Lauf moeglich. Erfolgreiche Tasks bekommen eine Ausfuehrungsnotiz und werden nach `done/` verschoben. Bei Abbruch bleibt die Task-Datei offen.

`/k-run` respektiert `## Ausfuehrungskontext` in Tasks. Wenn dort `Target repo`, `Base branch`, `Work branch` oder `PR required` stehen, fuehrt der Command vor Delegation einen Branch-/Dirty-Worktree-Preflight aus. Bei `PR required: true` gehoert PR-Handoff nach erfolgreichem Task zum Flow.

`/k-review-loop [path]` prueft Task- oder Instruktionsdateien vor der Ausfuehrung. Critic und Editor laufen read-only; nur der Moderator schreibt akzeptierte minimale Edits und haengt ein Review-Log an. Ohne Argument nutzt der Command `k-playbook/tasks/`.

`/k-todo [todo text]` liest oder ergaenzt `k-playbook/TODO.md`. Ohne Argument wird der Inhalt angezeigt; mit Text wird ein neuer `- [ ] ...`-Eintrag angehaengt.

## `/k-review`

`/k-review [review-name]` orchestriert globale und projektlokale Review-Rezepte.

Der Command:

- liest globale Rezepte aus `<PLAYBOOK_REPO>/global/reviews/`.
- liest projektlokale Rezepte aus `k-playbook/reviews/`.
- laesst projektlokale Rezepte globale mit gleichem Namen ueberlagern.
- laedt `known-decisions.md`, falls vorhanden, damit bewusste Entscheidungen nicht erneut als Findings gemeldet werden.
- schreibt `k-playbook/reviews/log.md`.

Interaktive Reviews moderieren Stelle fuer Stelle: Vorschlag zeigen, User-Freigabe abwarten, dann erst aendern. Report-Mode-Reviews mit `handoff` schreiben ein Ergebnisartefakt. Bei `result-family` landet es unter `k-playbook/reviews/results/<family>/YYYY-MM-DD/` mit typischer Struktur `assessment.md`, `findings.md`, `raw/` und Run-Metadaten.

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

- Legacy-Dateien wie `k-playbook/reviews/result-*.md`.
- Result-Familien wie `k-playbook/reviews/results/<family>/<date>/assessment.md` mit zugehoerigem `findings.md`.
- Von `/k-results` erzeugte Summaries unter `k-playbook/reviews/results/summary-YYYY-MM-DD.md`.

Der Command nutzt `k-playbook/reviews`, `k-playbook/tasks` und die Remediation-Policy aus `K-PLAYBOOK.yaml`. Er gruppiert Findings zuerst zu sinnvollen Buendeln nach Risiko, Aufwand, Kopplung und gemeinsamer Verifikation. Je nach Policy entstehen Tasks mit Branch-/PR-Hinweis, oder kleine direkte Fixes sind nur nach expliziter Freigabe erlaubt.

Wichtig: `raw/` und Run-Metadaten von Result-Familien bleiben read-only. Status, Triage-Notizen, Task-Verweise und Remediation-Logs werden in `findings.md` bzw. nachvollziehbar in `assessment.md` gepflegt.

## `/k-code2docs` und `/k-tools-scan`

`/k-code2docs` erzeugt die initiale Docs-First-Dokumentation unter `k-playbook/docs/` und registriert sie fuer spaetere AI-Sessions ueber `AGENTS.md` und `opencode.json`.

`/k-tools-scan` ist der zweite Docs-Schritt. Er ergaenzt unter `k-playbook/docs/libs/` pitfall-fokussierte Referenzen zu nicht-trivialen Libraries und verlinkt sie im Hauptindex.

Neue von diesen Commands erzeugte Doc-Dateien bleiben normale Markdown-Dateien, enthalten aber leichtgewichtig OKF-kompatibles YAML-Frontmatter. Pflichtanker fuer Menschen und OpenCode bleibt `k-playbook/docs/README.md`; ein OKF-`index.md` wird nicht als Ersatz eingefuehrt.

## `/k-results`

`/k-results` ist der Zwischenschritt zwischen Report-Reviews und Remediation.

Der Command:

- liest vorhandene Result-Familien unter `k-playbook/reviews/results/<family>/<date>/`.
- nutzt pro Familie standardmaessig das neueste `assessment.md` plus `findings.md`.
- dedupliziert Themen ueber Familien hinweg.
- beruecksichtigt `known-decisions.md` und vorhandene Tasks, soweit registriert.
- schreibt `k-playbook/reviews/results/summary-YYYY-MM-DD.md`.
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
- laedt projektlokale Regeln aus `k-playbook/enforcement/`, sofern dort Regeln liegen.
- prueft bei Code-Aenderungen immer die Docs-Sync-Regel gegen `k-playbook/docs/`, `README.md`, `AGENTS.md` und offensichtliche Architektur-/Setup-/API-Doku.
- bleibt read-only, solange der User nicht explizit einen Fix verlangt.

Wenn lokale Enforcement- oder Docs-Dateien fehlen, werden keine alternativen Pfade erfunden; globale Regeln gelten trotzdem.

## `/k-test-check`

`/k-test-check [path-or-command]` fuehrt Tests aus, diagnostiziert Fehlschlaege und fragt erst danach, ob Fixes gemacht werden sollen.

Ohne Argument erkennt der Command typische Test-Frameworks, z. B. `pytest`, Jest/Vitest, `make test`, Go oder Rust. Mit Argument nutzt er einen expliziten Pfad oder Testbefehl. Bei roten Tests liest er relevante Test- und Source-Dateien, ordnet die Ursache ein und meldet knapp, ob Test, Code, Umgebung oder ein gebrochener Vertrag betroffen ist.

## Hilfs-Commands: `/k-verlauf`, `/k-vscode-project-color`

`/k-verlauf [claude|opencode|all] <Suchbegriff> [zeitraum] [-all]` durchsucht alte AI-Verlaeufe. Claude wird ueber `~/.claude/projects/**/*.jsonl` durchsucht; OpenCode ueber Log-/Session-Metadaten in `opencode.log`, nicht zwingend ueber vollstaendige Chattexte. Der Command ist read-only und gibt Treffergruppen mit Zeit, Projekt/Directory und Snippets aus.

`/k-vscode-project-color [project-name]` schreibt oder merged eine projektlokale `.vscode/settings.json`, damit VS-Code-Fenster anhand Titel und Farbe unterscheidbar sind. Der Command ist nicht an `K-PLAYBOOK.yaml` gebunden und erhaelt bestehende unrelated Settings.

## `global/bin/k-check`

`global/bin/k-check` ist kein Slash-Command, sondern ein wiederverwendbarer CLI-Entry-Point fuer globale und projektlokale Checks.

Typische Nutzung aus einem Projekt-Root:

```bash
~/dev/k-playbook/global/bin/k-check --mode changed
~/dev/k-playbook/global/bin/k-check --mode baseline
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --mode changed
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --target-root app --mode baseline --output /path/to/project/k-playbook/reviews/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt --metadata-output /path/to/project/k-playbook/reviews/results/k-check/YYYY-MM-DD/run-metadata.json
```

Der Runner liest `K-PLAYBOOK.yaml` am Projekt-Root, fuehrt `.sh`-Checks aus `global/checks/` und projektlokale `.sh`-Checks aus `k-playbook/checks/` aus. `K-PLAYBOOK.yaml` bleibt dabei Config-Datei; Runner-Logik liegt im globalen Repo.

`--output <file>` erhaelt das normale Terminalverhalten und schreibt die vollstaendige Raw-Ausgabe zusaetzlich in eine Datei. `--metadata-output <file>` schreibt JSON-Metadaten zum Lauf, darunter Kommando, Exit-Code, Arbeitsverzeichnis, Datum/Zeit, Roots, Modus, Check-Konfiguration und k-check-Version/Git-Commit soweit verfuegbar. Beide Optionen verweigern vorhandene Ziel-Dateien. Fuer `/k-review k-check-security` gehoeren diese Dateien nach `k-playbook/reviews/results/k-check/YYYY-MM-DD/`.

Die stabile Check-Schnittstelle ist `.sh`. Einzelne Checks duerfen Python oder andere Tools intern verwenden, muessen aber am Ende genau eine Statuszeile `K_CHECK_STATUS=ok|skip|fail` und optional `K_CHECK_REASON=<text>` schreiben.

`changed` nutzt der Reihe nach `--files-from`, `--base-ref`, CI-/PR-Base-Refs, Upstream-Merge-Base und zuletzt uncommitted/staged Git-Aenderungen. Ist der Projekt-Root kein Git-Repo, werden nested Repos nicht automatisch gescannt. `baseline` scannt den Projektbaum mit Standard-Excludes wie `.git`, virtuelle Environments, `node_modules` und Build-Artefakte.

Globale Checks muessen domain-neutral bleiben. Projekt- oder produktbezogene Regeln, Modellnamen und Runtime-Dateien gehoeren in projektlokale Checks.
