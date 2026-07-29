# Runbook: Smoke-Test Neues System

Dieses Runbook beschreibt einen reproduzierbaren Durchlauf fuer `04-smoke-test-neues-system.md`. Es initialisiert die Testumgebung jedes Mal neu und prueft danach deterministisch, ob die Installation auf einem frischen System funktionieren wuerde.

Der Test veraendert nicht die echte OpenCode-Konfiguration des Users. Alle Host-Dateien liegen unter `/tmp/opencode/k-playbook-install-smoke`.

## Voraussetzungen

- `docker` ist verfuegbar, wenn die DevContainer-Seite realistisch simuliert werden soll.
- `opencode` ist optional. Fuer den deterministischen Smoke-Test werden die erwarteten Datei- und Symlink-Effekte direkt geprueft.
- Das aktuelle Repo liegt irgendwo lokal und kann in die Testumgebung verlinkt werden.

## Pfade

```text
TEST_ROOT=/tmp/opencode/k-playbook-install-smoke
HOST_HOME=${TEST_ROOT}/host-home
HOST_WORK=${HOST_HOME}/dev
DUMMY_PROJECT=${HOST_WORK}/example-python-project
PLAYBOOK_UNDER_TEST=${HOST_WORK}/k-playbook
```

## 1. Testumgebung neu initialisieren

Jeder neue Durchlauf beginnt mit einer leeren Testumgebung:

```bash
rm -rf "/tmp/opencode/k-playbook-install-smoke"
mkdir -p "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project/.devcontainer"
ln -sfn "$(pwd)" "/tmp/opencode/k-playbook-install-smoke/host-home/dev/k-playbook"
```

Danach im Dummy-Projekt eine minimale DevContainer-Konfiguration anlegen:

```json
{
  "name": "example-python-project",
  "image": "mcr.microsoft.com/devcontainers/python:3.12"
}
```

Pfad:

```text
/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project/.devcontainer/devcontainer.json
```

## 2. Host-Registrierung pruefen

Fuer den deterministischen Test die erwarteten Bootstrap-Effekte aus `01-host-opencode-registrieren.md` mit isoliertem `HOME` erzeugen:

```bash
mkdir -p \
  "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/command" \
  "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/commands"

for f in "/tmp/opencode/k-playbook-install-smoke/host-home/dev/k-playbook"/commands/k-*.md; do
  ln -sf "$f" "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/command/$(basename "$f")"
  ln -sf "$f" "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/commands/$(basename "$f")"
done
```

OpenCode-Konfiguration im isolierten Home:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

Pfad:

```text
/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/opencode.jsonc
```

Pruefen:

```bash
test -f "/tmp/opencode/k-playbook-install-smoke/host-home/dev/k-playbook/commands/k-install.md"
test -L "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/command/k-install.md"
test -L "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/commands/k-install.md"
grep -q "~/dev/k-playbook" "/tmp/opencode/k-playbook-install-smoke/host-home/.config/opencode/opencode.jsonc"
```

Security-Tools gehoeren in der echten Installation zu Prompt 1. In diesem Smoke-Test duerfen Downloads uebersprungen werden; das Ergebnis muss dann ausdruecklich sagen, dass nur Registrierung und DevContainer-Integration getestet wurden.

## 3. Dummy-Projekt fuer DevContainer vorbereiten

Installer mit isoliertem `HOME` ausfuehren:

```bash
HOME="/tmp/opencode/k-playbook-install-smoke/host-home" \
  "/tmp/opencode/k-playbook-install-smoke/host-home/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh" \
  "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project"
```

Pruefen:

```bash
test -x "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project/.devcontainer/setup-k-playbook.sh"
grep -q "/workspaces/k-playbook" "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project/.devcontainer/devcontainer.json"
grep -q "postCreateCommand" "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project/.devcontainer/devcontainer.json"
grep -q "postStartCommand" "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project/.devcontainer/devcontainer.json"
```

## 4. DevContainer-Seite per Docker pruefen

Nicht-interaktiver Check:

```bash
docker run --rm \
  -v "/tmp/opencode/k-playbook-install-smoke/host-home/dev/k-playbook:/workspaces/k-playbook" \
  -v "/tmp/opencode/k-playbook-install-smoke/host-home/dev/example-python-project:/workspaces/example-python-project" \
  mcr.microsoft.com/devcontainers/python:3.12 \
  bash -lc 'bash /workspaces/example-python-project/.devcontainer/setup-k-playbook.sh && test -f /workspaces/k-playbook/commands/k-install.md && test -L /home/vscode/dev/k-playbook && test "$(readlink /home/vscode/dev/k-playbook)" = "/workspaces/k-playbook" && test -L /home/vscode/.config/opencode/command/k-install.md && test -L /home/vscode/.config/opencode/command/k-status.md && test -L /home/vscode/.config/opencode/commands/k-install.md && test -L /home/vscode/.config/opencode/commands/k-status.md && test -f /home/vscode/.config/opencode/opencode.jsonc && grep -q "~/dev/k-playbook" /home/vscode/.config/opencode/opencode.jsonc'
```

Erwartung: Der Befehl beendet sich mit Exit-Code `0`.

## 5. Ergebnis dokumentieren

## Optional: Security-Tool-Downloads testen

Dieser Extended-Test prueft, ob die Security-Tools in einer Wegwerf-Umgebung wirklich heruntergeladen und user-lokal installiert werden koennen. Er ist absichtlich nicht Teil des schnellen Standard-Smoke-Tests, weil er Netzwerk, GitHub-Releases, PyPI und ggf. Docker-Image-Downloads braucht.

Der Test nutzt einen Container und schreibt nur in dessen `/home/vscode`:

```bash
docker run --rm \
  -v "/tmp/opencode/k-playbook-install-smoke/host-home/dev/k-playbook:/workspaces/k-playbook" \
  mcr.microsoft.com/devcontainers/python:3.12 \
  bash -lc 'set -euo pipefail
    mkdir -p /home/vscode/dev /home/vscode/.local/bin /home/vscode/.local/share/k-playbook/security-tools
    ln -sfn /workspaces/k-playbook /home/vscode/dev/k-playbook
    chown -R vscode:vscode /home/vscode/dev /home/vscode/.local
    sudo -H -u vscode env \
      HOME=/home/vscode \
      PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin \
      K_SECURITY_TOOLS_PREFIX=/home/vscode/.local \
      K_SECURITY_TOOLS_BIN_DIR=/home/vscode/.local/bin \
      K_SECURITY_TOOLS_VENV=/home/vscode/.local/share/k-playbook/security-tools/pip-audit-venv \
      bash /home/vscode/dev/k-playbook/scripts/install-security-tools.sh --install missing --method auto --yes
    sudo -H -u vscode env HOME=/home/vscode PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin gitleaks version
    sudo -H -u vscode env HOME=/home/vscode PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin trufflehog --version
    sudo -H -u vscode env HOME=/home/vscode PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin pip-audit --version
    sudo -H -u vscode env HOME=/home/vscode PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin trivy --version
    sudo -H -u vscode env HOME=/home/vscode PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin syft version
    sudo -H -u vscode env HOME=/home/vscode PATH=/home/vscode/.local/bin:/usr/local/bin:/usr/bin:/bin grype version'
```

Erwartung: Alle sechs Tools melden eine Version. Wenn der Test fehlschlaegt, zuerst unterscheiden:

- Netzwerk-/Rate-Limit-/Registry-Fehler: Installation wahrscheinlich korrekt, externer Download war nicht verfuegbar.
- Fehlender Systembaustein im Container, z. B. `python3-venv`, `curl`, `tar`: DevContainer-Basisimage oder Installer-Voraussetzungen pruefen.
- Projekt-venv-Fehler: Testumgebung ist falsch, weil `VIRTUAL_ENV` oder ein Projekt-venv im `PATH` aktiv ist.

## 6. Ergebnis dokumentieren

Der Abschlussbericht soll enthalten:

- Ob die Testumgebung neu initialisiert wurde.
- Ob Host-Registrierung im isolierten Home funktioniert.
- Ob das Dummy-Projekt korrekt fuer DevContainer vorbereitet wurde.
- Ob der Docker-Check erfolgreich war.
- Ob `/k-status` im simulierten DevContainer ueber `command/` und `commands/` auffindbar ist.
- Ob Security-Tool-Downloads bewusst uebersprungen wurden.
- Ob der optionale Security-Tool-Download-Test ausgefuehrt wurde und welche Tools Versionen gemeldet haben.
- Welche Punkte weiterhin nur in einem echten interaktiven neuen System pruefbar sind.
