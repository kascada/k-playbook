# Multi-Project-Installation mit k-playbook und optionalem DevContainer

Viele aehnliche Frameworks werden einmalig in ein Projekt kopiert, dort angepasst und danach nur schwer aktualisiert. Das erschwert Updates und Wartung, besonders wenn dieselben Workflows in mehreren Projekten genutzt werden.

Dieses Setup trennt deshalb eine zentrale, user-lokale k-playbook-Installation, die per Git aktualisiert werden kann, von projektlokalen Anpassungen. Jedes Zielprojekt bekommt seine eigene lokale Konfiguration; die Beispiele sind fuer Python-lastige Projekte gedacht, die mit oder ohne Projekt-venv laufen und optional einen DevContainer nutzen.

Diese Reihenfolge richtet k-playbook fuer den Host, ein Zielprojekt und optional den DevContainer ein.

## Vereinfachte Prompt-Reihenfolge

Die Prompts liegen unter [`../prompts/installation/`](../prompts/installation/). Zuerst kommt immer die gemeinsame Host-Registrierung, danach genau ein Projektpfad: `3A` ohne DevContainer oder `3B` mit DevContainer.

### Gemeinsame Basis

Zuerst k-playbook klonen:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
```

Danach auf dem Host ausfuehren:

```bash
opencode run --dir ~/dev/k-playbook --file ~/dev/k-playbook/prompts/installation/01-host-opencode-registrieren.md "Fuehre den angehaengten Installations-Prompt aus."
```

Prompt: [`01-host-opencode-registrieren.md`](../prompts/installation/01-host-opencode-registrieren.md)

### Pfad A: Projekt ohne DevContainer

Im Zielprojekt auf dem Host ausfuehren:

```bash
opencode run --dir ~/dev/example-python-project --file ~/dev/k-playbook/prompts/installation/03A-projekt-ohne-devcontainer-setup.md "Fuehre den angehaengten Installations-Prompt aus."
```

Prompt: [`03A-projekt-ohne-devcontainer-setup.md`](../prompts/installation/03A-projekt-ohne-devcontainer-setup.md)

### Pfad B: Projekt mit DevContainer

Zuerst auf dem Host ausfuehren und das Zielprojekt abfragen lassen:

```bash
opencode run --dir ~/dev/k-playbook --file ~/dev/k-playbook/prompts/installation/02-devcontainer-k-playbook-installieren.md "Fuehre den angehaengten Installations-Prompt aus."
```

Prompt: [`02-devcontainer-k-playbook-installieren.md`](../prompts/installation/02-devcontainer-k-playbook-installieren.md)

Danach den DevContainer neu bauen oder neu starten. Anschliessend im DevContainer aus dem Zielprojekt heraus ausfuehren, z. B.:

```bash
opencode run --dir /workspaces/example-python-project --file ~/dev/k-playbook/prompts/installation/03B-devcontainer-projekt-setup.md "Fuehre den angehaengten Installations-Prompt aus."
```

Prompt: [`03B-devcontainer-projekt-setup.md`](../prompts/installation/03B-devcontainer-projekt-setup.md)

### Tests

Zum Testen auf einem simulierten neuen System gibt es [`04-smoke-test-neues-system.md`](../prompts/installation/04-smoke-test-neues-system.md). Der reproduzierbare Durchlauf steht in [`RUNBOOK-smoke-test-neues-system.md`](../prompts/installation/RUNBOOK-smoke-test-neues-system.md).

Die Prompts stehen auch im Index [`../prompts/README.md`](../prompts/README.md).

## Halbmanuelle Installation

## 1. k-playbook auf dem Host Klonen

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
```

## 2. Host-OpenCode Registrieren

OpenCode auf dem Host starten und den AI-Assistenten im k-playbook-Repo anweisen:

```text
Lies ~/dev/k-playbook/commands/k-install.md und fuehre die Installationsanleitung darin aus.
```

Dieser Bootstrap registriert den Command; danach reicht fuer zukuenftige Aktualisierungen normal `/k-install`.

Danach Security-Tools host-lokal installieren:

```text
/k-install-security-tools --install missing
```

Wichtig: `/k-install*` nicht aus einem aktiven Projekt-venv starten. Falls `VIRTUAL_ENV` gesetzt ist, zuerst `deactivate` ausfuehren.

Danach pro Zielprojekt genau einen Pfad waehlen: `3A` fuer ein normales Projekt ohne DevContainer oder `3B` fuer ein Projekt mit DevContainer.

## 3A. Projekt ohne DevContainer einrichten

Im vorhandenen Zielprojekt OpenCode starten und ausfuehren:

```text
/k-setup
```

`/k-setup` legt oder aktualisiert `K-PLAYBOOK.MD` und die projektlokalen k-playbook-Pfade. Das gilt fuer normale Projekte genauso wie fuer Projekte mit aktivem Projekt-venv; der Command installiert keine Tools in das Projekt-venv.

Optional, wenn das Zielprojekt lokal mit CodeQL vorbereitet werden soll, direkt danach ausfuehren:

```text
/k-setup-codeql
```

Falls die lokale CodeQL-CLI noch fehlt, danach den Installationscommand ohne aktives Projekt-venv nutzen:

```text
/k-install-codeql
```

Danach im Projektroot pruefen:

```text
/k-status
```

Erwartung:

- `playbook` zeigt auf `~/dev/k-playbook`.
- `opencode` meldet Command-Links und `skills.paths` plausibel.
- `K-PLAYBOOK.MD` existiert im Projektroot.

## 3B. Projekt mit DevContainer einrichten

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

Den DevContainer fuer `~/dev/example-python-project` neu bauen oder neu starten.

Nach dem Start im Container pruefen:

```bash
ls -l /workspaces/k-playbook/commands/k-install.md
ls -l ~/dev/k-playbook/commands/k-install.md
ls -l ~/.config/opencode/command/k-install.md
ls -l ~/.config/opencode/commands/k-install.md
```

Alle Pfade muessen existieren.

OpenCode im Container neu starten, im Projektroot `/workspaces/example-python-project` oeffnen und ausfuehren:

```text
/k-setup
```

Optional, wenn das Zielprojekt lokal mit CodeQL vorbereitet werden soll, direkt danach im Container ausfuehren:

```text
/k-setup-codeql
```

Falls die lokale CodeQL-CLI noch fehlt, danach den Installationscommand nutzen:

```text
/k-install-codeql
```

Nach `/k-setup` im Container einmal ausfuehren:

```text
/k-install
```

Grund: `/k-setup` richtet das Projekt ein, aber die Container-lokale OpenCode-Registrierung muss danach noch einmal sicherstellen, dass `skills.paths` und die Command-Symlinks im Container-Home korrekt sind.

Wenn `/k-install` danach noch fehlende Security-Tools meldet, ist der DevContainer vermutlich noch nicht mit der aktuellen `.devcontainer/setup-k-playbook.sh` neu erzeugt worden oder der automatische Installationslauf ist fehlgeschlagen. Dann im Container einmal manuell ausfuehren:

```text
/k-install-security-tools --install missing --yes
```

Danach OpenCode im Container neu starten und im Projektroot pruefen:

```text
/k-status
```

Erwartung:

- `playbook` zeigt auf `~/dev/k-playbook`.
- `opencode` meldet Command-Links und `skills.paths` plausibel.
- `devcontainer` erkennt `/workspaces/k-playbook` und den Symlink `~/dev/k-playbook`.

## 4. VS-Code-Erweiterungen Empfohlen

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
Lies ~/dev/k-playbook/commands/k-install.md und fuehre die Installationsanleitung darin aus.
/k-install-security-tools --install missing
```

Ohne DevContainer im Zielprojekt:

```text
/k-setup
# optional:
/k-setup-codeql
/k-install-codeql
/k-status
```

Mit DevContainer auf dem Host:

```bash
~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh ~/dev/example-python-project
```

DevContainer neu bauen, OpenCode im Container neu starten, dann:

```text
/k-setup
# optional:
/k-setup-codeql
/k-install-codeql
/k-install-security-tools --install missing --yes
/k-install
/k-status
```
