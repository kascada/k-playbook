# Multi-Project-Installation mit k-playbook und DevContainer

Viele aehnliche Frameworks werden einmalig in ein Projekt kopiert, dort angepasst und danach nur schwer aktualisiert. Das erschwert Updates und Wartung, besonders wenn dieselben Workflows in mehreren Projekten genutzt werden.

Dieses Setup trennt deshalb eine zentrale, user-lokale k-playbook-Installation, die per Git aktualisiert werden kann, von projektlokalen Anpassungen. Jedes Zielprojekt bekommt seine eigene lokale Konfiguration; die Beispiele sind fuer Python-lastige Projekte gedacht, die mit oder ohne Projekt-venv laufen und optional einen DevContainer nutzen.

Diese Reihenfolge richtet k-playbook fuer den Host, ein Zielprojekt und optional den DevContainer ein.

## Vereinfachte Prompt-Reihenfolge

Wenn du die Installation nicht manuell Schritt fuer Schritt ausfuehren willst, kopiere nacheinander diese Prompts in den AI-Assistenten:

1. `git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook`
2. [`../prompts/installation/01-host-opencode-registrieren.md`](../prompts/installation/01-host-opencode-registrieren.md) - auf dem Host nach dem Clone ausfuehren.
3. [`../prompts/installation/02-devcontainer-k-playbook-installieren.md`](../prompts/installation/02-devcontainer-k-playbook-installieren.md) - auf dem Host ausfuehren und Zielprojekt abfragen lassen.
4. [`../prompts/installation/03-devcontainer-projekt-setup.md`](../prompts/installation/03-devcontainer-projekt-setup.md) - im neu gebauten oder neu gestarteten DevContainer ausfuehren.

Mit OpenCode geht das direkt so:

```bash
opencode run --dir ~/dev/k-playbook --file ~/dev/k-playbook/prompts/installation/01-host-opencode-registrieren.md "Fuehre den angehaengten Installations-Prompt aus."
```

```bash
opencode run --dir ~/dev/k-playbook --file ~/dev/k-playbook/prompts/installation/02-devcontainer-k-playbook-installieren.md "Fuehre den angehaengten Installations-Prompt aus."
```

Im DevContainer dann aus dem Zielprojekt heraus, z. B.:

```bash
opencode run --dir /workspaces/example-python-project --file ~/dev/k-playbook/prompts/installation/03-devcontainer-projekt-setup.md "Fuehre den angehaengten Installations-Prompt aus."
```

Fuer andere Prompts ist der Aufruf analog: `--dir` setzt den Arbeitskontext, `--file` uebergibt die Prompt-Datei.

Die Prompts stehen auch im Index [`../prompts/README.md`](../prompts/README.md).

Zum Testen auf einem simulierten neuen System gibt es zusaetzlich [`../prompts/installation/04-smoke-test-neues-system.md`](../prompts/installation/04-smoke-test-neues-system.md). Der reproduzierbare Durchlauf steht in [`../prompts/installation/RUNBOOK-smoke-test-neues-system.md`](../prompts/installation/RUNBOOK-smoke-test-neues-system.md).

## Halbmanuelle Installation

## 1. k-playbook auf dem Host Klonen

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
```

Falls das Repo schon existiert:

```bash
cd ~/dev/k-playbook
git pull
```

## 2. Host-OpenCode Registrieren

OpenCode auf dem Host starten und den AI-Assistenten im k-playbook-Repo anweisen:

```text
Lies ~/dev/k-playbook/commands/k-install.md und fuehre die Installationsanleitung darin aus.
```

Dieser Bootstrap registriert den Command; danach reicht fuer zukuenftige Aktualisierungen normal `/k-install`.

Optional danach Security-Tools host-lokal installieren:

```text
/k-install-security-tools --install missing
```

Wichtig: `/k-install*` nicht aus einem aktiven Projekt-venv starten. Falls `VIRTUAL_ENV` gesetzt ist, zuerst `deactivate` ausfuehren.

## 3. DevContainer Fuer k-playbook Vorbereiten

Auf dem Host ausfuehren:

```bash
~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh ~/dev/example-python-project
```

Das Script schreibt im Zielprojekt `.devcontainer/setup-k-playbook.sh` und passt `.devcontainer/devcontainer.json` an.

Im DevContainer wird dadurch vorbereitet:

- `/workspaces/k-playbook` ist der Bind-Mount des Host-Repos `~/dev/k-playbook`.
- `/home/vscode/dev/k-playbook` zeigt per Symlink auf `/workspaces/k-playbook`.
- k-playbook-Commands werden in die Container-lokale OpenCode-Konfiguration verlinkt.
- `skills.paths: ["~/dev/k-playbook"]` wird bei Bedarf in der Container-OpenCode-Konfig angelegt.
- fehlende Security-Pflicht-Tools werden beim DevContainer-Rebuild fuer den Container-User `vscode` installiert.

## 4. DevContainer Neu Erzeugen

Den DevContainer fuer `~/dev/example-python-project` neu bauen oder neu starten.

Nach dem Start im Container pruefen:

```bash
ls -l /workspaces/k-playbook/commands/k-install.md
ls -l ~/dev/k-playbook/commands/k-install.md
ls -l ~/.config/opencode/command/k-install.md
ls -l ~/.config/opencode/commands/k-install.md
```

Alle Pfade muessen existieren.

## 5. Projektlokales k-playbook Setup

OpenCode im Container neu starten, im Projektroot `/workspaces/example-python-project` oeffnen und ausfuehren:

```text
/k-setup
```

`/k-setup` legt oder aktualisiert `K-PLAYBOOK.MD` und die projektlokalen k-playbook-Pfade.

## 6. Optional CodeQL Einrichten

Wenn das Zielprojekt lokal mit CodeQL vorbereitet werden soll, direkt danach im Container ausfuehren:

```text
/k-setup-codeql
```

Falls die lokale CodeQL-CLI noch fehlt, danach den Installationscommand nutzen:

```text
/k-install-codeql
```

`/k-setup-codeql` dokumentiert die CodeQL-Entscheidung und Pfade in `K-PLAYBOOK.MD`; `/k-install-codeql` installiert die lokale CodeQL-CLI ausserhalb von Projekt-venvs.

## 7. Container-OpenCode Nochmal Registrieren

Nach `/k-setup` im Container einmal ausfuehren:

```text
/k-install
```

Grund: `/k-setup` richtet das Projekt ein, aber die Container-lokale OpenCode-Registrierung muss danach noch einmal sicherstellen, dass `skills.paths` und die Command-Symlinks im Container-Home korrekt sind.

Wenn `/k-install` danach noch fehlende Security-Tools meldet, ist der DevContainer vermutlich noch nicht mit der aktuellen `.devcontainer/setup-k-playbook.sh` neu erzeugt worden oder der automatische Installationslauf ist fehlgeschlagen. Dann im Container einmal manuell ausfuehren:

```text
/k-install-security-tools --install missing --yes
```

Danach OpenCode im Container neu starten.

## 8. Status Pruefen

Im Projektroot im Container:

```text
/k-status
```

Erwartung:

- `playbook` zeigt auf `~/dev/k-playbook`.
- `opencode` meldet Command-Links und `skills.paths` plausibel.
- `devcontainer` erkennt `/workspaces/k-playbook` und den Symlink `~/dev/k-playbook`.

## 9. VS-Code-Erweiterungen Empfohlen

VS Code sollte beim Oeffnen des Projekts die empfohlenen Erweiterungen aus `.vscode/extensions.json` anzeigen. Fuer Python-lastige Projekte sind insbesondere sinnvoll:

- SARIF Viewer (`MS-SarifVSCode.sarif-viewer`) fuer CodeQL-/Security-Ergebnisse.
- GitHub CodeQL (`GitHub.vscode-codeql`) fuer lokale CodeQL-Queries und Datenbanken.
- Dev Containers (`ms-vscode-remote.remote-containers`) fuer den Container-Workflow.
- Python (`ms-python.python`) und Pylance (`ms-python.vscode-pylance`) fuer Python-Code.
- Docker (`ms-azuretools.vscode-docker`) fuer Container-Kontext.
- YAML (`redhat.vscode-yaml`) fuer GitHub Actions, Dependabot und Configs.
- ShellCheck (`timonwong.shellcheck`) fuer DevContainer-/Setup-Shellscripts.

## Kurzreihenfolge

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
```

```text
/k-install
```

```bash
~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh ~/dev/example-python-project
```

DevContainer neu bauen, OpenCode im Container neu starten, dann:

```text
/k-setup
/k-setup-codeql
/k-install-codeql
/k-install-security-tools --install missing --yes
/k-install
/k-status
```
