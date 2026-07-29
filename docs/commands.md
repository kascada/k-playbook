# k-playbook Commands

Diese Datei beschreibt die wichtigsten Setup- und Install-Commands und grenzt ihre Zuständigkeiten ab.

Globale Regeln, Review-Rezepte und Checks liegen in diesem Repo unter `global/`. Projektlokale Regeln, Reviews, Checks, Tasks und Docs liegen im jeweiligen Projekt und werden dort ueber `K-PLAYBOOK.MD` registriert.

Der Review-/Results-/Remediation-Flow ist in [`reviews-and-results.md`](./reviews-and-results.md) zusammengefasst.

## Grundregel: Install vs. Setup

`/k-install*` und `/k-setup*` haben absichtlich unterschiedliche Zustaendigkeiten:

- `/k-install` und `/k-install-security-tools` sind **host-global**. Sie machen Commands, Skills und Security-Tools auf diesem Server fuer alle Projekte verfuegbar.
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

| Command | Scope | Projekt-Konfig | Artefakte / Host |
|---------|-------|----------------|------------------|
| `/k-install` | k-playbook auf diesem Host fuer OpenCode registrieren und Security-Tool-Preflight zeigen | keine Aenderung | OpenCode-Symlinks, ggf. Skill-Pfad, nur Tool-Status |
| `/k-install-security-tools` | host-lokale Security-Review-Tools installieren/pruefen | keine Aenderung | `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype` oder Docker-Images |
| `/k-setup` | k-playbook in einem Projekt konfigurieren | schreibt `K-PLAYBOOK.MD` und gewaehlte Playbook-Pfade | keine Host-Aenderung |
| `/k-setup-codeql` | CodeQL-Entscheidung im Projekt registrieren | schreibt CodeQL-Block in `K-PLAYBOOK.MD` | optional CLI-only Artefakt unter `codeql-cli/` |
| `/k-install-codeql` | lokale CodeQL CLI installieren/pruefen, optional lokale DBs analysieren | keine Aenderung an `K-PLAYBOOK.MD` | optional `codeql-cli/`, `databases/`, `results/` |
| `/k-status` | read-only Health-Check fuer Projekt und host-lokale OpenCode-Registrierung | keine Aenderung | prueft u. a. Command-Symlinks und `skills.paths` |
| `/k-results` | vorhandene Review-Results projektweit priorisieren | liest `reviews:` und optional `tasks:` | schreibt `<reviews>/results/summary-YYYY-MM-DD.md` |
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
- registriert projektlokale Pfade wie `tasks`, `checks`, `reviews`, `docs`, `enforcement`
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
/k-review iac-container
/k-results
/k-remediation k-playbook/reviews/results/summary-YYYY-MM-DD.md
```

`/k-result` ist ein Alias fuer `/k-results`.

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
