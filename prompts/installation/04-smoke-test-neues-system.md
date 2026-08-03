# Prompt: Smoke-Test Neues System

Teste die Multi-Project-Installation in einer Wegwerf-Umgebung so, als waere k-playbook auf einem neuen System noch nicht eingerichtet.

Ziel ist nicht ein vollstaendiger Produkt-DevContainer, sondern ein belastbarer Installations-Smoke-Test fuer:

- Host-OpenCode-Registrierung nach frischem Clone.
- Normales Projekt-Setup ohne DevContainer kann mit `03A-projekt-ohne-devcontainer-setup.md` getestet werden; dieser Smoke-Test fokussiert die Infrastruktur- und DevContainer-Integration.
- Vorbereitung eines Dummy-Zielprojekts mit `.devcontainer/devcontainer.json`.
- Simulation der DevContainer-Seite mit `/workspaces/k-playbook`, `/home/vscode/dev/k-playbook`, OpenCode-Command-Links und `skills.paths`.

## Grenzen

- Veraendere nicht die echte User-OpenCode-Konfiguration des Hosts.
- Nutze eine Wegwerf-Umgebung mit eigenem `HOME` oder einen Container.
- Security-Tools gehoeren zur normalen Installation. Fuer diesen Smoke-Test duerfen Tool-Downloads aber uebersprungen werden, wenn der User nur die Registrierung und DevContainer-Integration pruefen will.
- Wenn Docker oder `opencode` fehlt, stoppe nicht sofort. Dokumentiere den fehlenden Teil und fuehre alle lokal pruefbaren Schritte aus.

## Schritt 1 - Testumgebung anlegen

Lege unter `/tmp/opencode/k-playbook-install-smoke` eine saubere Testumgebung an.

Verwende diese Pfade:

```text
TEST_ROOT=/tmp/opencode/k-playbook-install-smoke
HOST_HOME=${TEST_ROOT}/host-home
DEV_HOME=${TEST_ROOT}/dev-home
HOST_WORK=${HOST_HOME}/dev
DUMMY_PROJECT=${HOST_WORK}/example-python-project
```

Erzeuge ein Dummy-Projekt mit minimalem DevContainer:

```text
${DUMMY_PROJECT}/.devcontainer/devcontainer.json
```

Inhalt:

```json
{
  "name": "example-python-project",
  "image": "mcr.microsoft.com/devcontainers/python:3.12"
}
```

Stelle sicher, dass `k-playbook` in der Testumgebung unter `${HOST_HOME}/dev/k-playbook` erreichbar ist. Bevorzugt ist ein Symlink auf das aktuelle Repo, damit keine Kopie noetig ist.

## Schritt 2 - Prompt 1 simulieren

Fuehre die Host-Registrierung mit isoliertem `HOME=${HOST_HOME}` aus.

Wenn `opencode run` sinnvoll verfuegbar ist, nutze:

```bash
HOME="${HOST_HOME}" opencode run --dir "${HOST_HOME}/dev/k-playbook" --file "${HOST_HOME}/dev/k-playbook/prompts/installation/01-host-opencode-registrieren.md" "Fuehre den angehaengten Installations-Prompt als Smoke-Test aus. Security-Tool-Downloads duerfen fuer diesen Smoke-Test uebersprungen werden."
```

Wenn `opencode run` nicht verfuegbar oder fuer den Test zu interaktiv ist, lies `commands/k-install.md` und fuehre die dort beschriebenen Bootstrap-Schritte deterministisch mit `HOME=${HOST_HOME}` aus.

Pruefe danach mindestens:

- `${HOST_HOME}/dev/k-playbook/commands/k-install.md` existiert.
- `${HOST_HOME}/.config/opencode/command/k-install.md` existiert und zeigt auf `${HOST_HOME}/dev/k-playbook/commands/k-install.md`.
- `${HOST_HOME}/.config/opencode/opencode.jsonc` oder `.json` enthaelt `skills.paths` mit `~/dev/k-playbook` oder dem Testpfad.
- Wenn Security-Tools uebersprungen wurden, ist das im Ergebnisbericht ausdruecklich dokumentiert.

## Schritt 3 - Prompt 2 gegen Dummy-Projekt ausfuehren

Fuehre die DevContainer-Vorbereitung fuer das Dummy-Projekt aus:

```bash
HOME="${HOST_HOME}" opencode run --dir "${HOST_HOME}/dev/k-playbook" --file "${HOST_HOME}/dev/k-playbook/prompts/installation/02-devcontainer-k-playbook-installieren.md" "Fuehre den angehaengten Installations-Prompt als Smoke-Test fuer dieses Zielprojekt aus: ${DUMMY_PROJECT}"
```

Wenn `opencode run` nicht genutzt wird, fuehre direkt aus:

```bash
HOME="${HOST_HOME}" "${HOST_HOME}/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh" "${DUMMY_PROJECT}"
```

Pruefe danach:

- `${DUMMY_PROJECT}/.devcontainer/setup-k-playbook.sh` existiert und ist ausfuehrbar.
- `${DUMMY_PROJECT}/.devcontainer/devcontainer.json` enthaelt den Mount nach `/workspaces/k-playbook`.
- `postCreateCommand` enthaelt `.devcontainer/setup-k-playbook.sh --install-security-tools`.
- `postStartCommand` enthaelt `.devcontainer/setup-k-playbook.sh`.

## Schritt 4 - DevContainer-Seite simulieren

Simuliere die wichtigsten DevContainer-Pfade ohne VS-Code-DevContainer-Rebuild:

```text
${TEST_ROOT}/workspaces/k-playbook
${TEST_ROOT}/workspaces/example-python-project
```

Nutze Symlinks oder Bind-Mounts, je nachdem was lokal verfuegbar ist:

- `${TEST_ROOT}/workspaces/k-playbook` zeigt auf `${HOST_HOME}/dev/k-playbook`.
- `${TEST_ROOT}/workspaces/example-python-project` zeigt auf `${DUMMY_PROJECT}`.

Wenn Docker verfuegbar ist, ist ein Container-Test besser. Fuer einen schnellen nicht-interaktiven Smoke-Test nutze:

```bash
docker run --rm \
  -v "${HOST_HOME}/dev/k-playbook:/workspaces/k-playbook" \
  -v "${DUMMY_PROJECT}:/workspaces/example-python-project" \
  mcr.microsoft.com/devcontainers/python:3.12 \
  bash -lc 'bash /workspaces/example-python-project/.devcontainer/setup-k-playbook.sh && test -f /workspaces/k-playbook/commands/k-install.md && test -L /home/vscode/dev/k-playbook && test "$(readlink /home/vscode/dev/k-playbook)" = "/workspaces/k-playbook" && test -L /home/vscode/.config/opencode/command/k-install.md && test -L /home/vscode/.config/opencode/command/k-status.md && test -L /home/vscode/.config/opencode/commands/k-install.md && test -L /home/vscode/.config/opencode/commands/k-status.md && test -f /home/vscode/.config/opencode/opencode.jsonc && grep -q "~/dev/k-playbook" /home/vscode/.config/opencode/opencode.jsonc'
```

Fuer manuelles Debugging kannst du alternativ einen interaktiven Container starten mit:

- `${HOST_HOME}/dev/k-playbook` gemountet nach `/workspaces/k-playbook`.
- `${DUMMY_PROJECT}` gemountet nach `/workspaces/example-python-project`.
- User `root`, damit `.devcontainer/setup-k-playbook.sh` die erwarteten `/home/vscode`-Pfade vorbereiten kann.

Beispiel:

```bash
docker run --rm -it \
  -v "${HOST_HOME}/dev/k-playbook:/workspaces/k-playbook" \
  -v "${DUMMY_PROJECT}:/workspaces/example-python-project" \
  mcr.microsoft.com/devcontainers/python:3.12 \
  bash
```

Im Container dann:

```bash
bash /workspaces/example-python-project/.devcontainer/setup-k-playbook.sh
```

Pruefe im Container:

- `/workspaces/k-playbook/commands/k-install.md` existiert.
- `/home/vscode/dev/k-playbook` zeigt auf `/workspaces/k-playbook`.
- `/home/vscode/.config/opencode/command/k-install.md` und `/home/vscode/.config/opencode/command/k-status.md` existieren.
- `/home/vscode/.config/opencode/commands/k-install.md` und `/home/vscode/.config/opencode/commands/k-status.md` existieren.
- `/home/vscode/.config/opencode/opencode.jsonc` enthaelt `skills.paths` mit `~/dev/k-playbook`.

## Schritt 5 - Prompt 3 pruefen

Wenn der Docker-Container aus Schritt 4 laeuft und `opencode` darin verfuegbar ist, fuehre aus:

```bash
opencode run --dir /workspaces/example-python-project --file /home/vscode/dev/k-playbook/prompts/installation/03B-devcontainer-projekt-setup.md "Fuehre den angehaengten Installations-Prompt als Smoke-Test aus. Security-Tool-Downloads duerfen fuer diesen Smoke-Test uebersprungen werden. Frage vor CodeQL oder Dependabot."
```

Wenn `opencode` im Container nicht verfuegbar ist, pruefe die deterministischen Effekte aus Prompt 3 direkt:

- Lies `/home/vscode/dev/k-playbook/commands/k-install.md` und pruefe, ob die Container-Registrierung bereits erfuellt ist.
- Lies `/home/vscode/dev/k-playbook/commands/k-setup.md` und dokumentiere, ob ein echtes `/k-setup` ohne interaktive Entscheidungen sinnvoll ist. Fuer den Smoke-Test reicht es, wenn die Voraussetzungen fuer `/k-setup` stimmen.
- Lies `/home/vscode/dev/k-playbook/commands/_details/k-status.md` und pruefe die dort relevanten Statuspunkte fuer `playbook`, `opencode` und `devcontainer`.

## Ergebnisbericht

Berichte am Ende knapp:

- Welche Teile mit `opencode run` ausgefuehrt wurden.
- Welche Teile deterministisch ohne LLM ausgefuehrt wurden.
- Ob ein frisches Home die Host-Registrierung erhaelt.
- Ob das Dummy-Projekt korrekt fuer DevContainer vorbereitet wurde.
- Ob die DevContainer-Simulation die erwarteten Pfade und OpenCode-Links erzeugt.
- Ob `/k-status` im simulierten DevContainer ueber die OpenCode-Command-Links auffindbar ist.
- Ob Security-Tools installiert oder fuer den Smoke-Test bewusst uebersprungen wurden.
- Welche manuellen oder interaktiven Punkte fuer einen echten neuen Host bleiben.
