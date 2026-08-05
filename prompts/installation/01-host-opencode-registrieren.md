# Prompt: Host-OpenCode Registrieren

Du arbeitest auf dem Host, nicht in einem DevContainer.

Lies zuerst `~/dev/k-playbook/docs/installation.md`, besonders `Pfadvertrag`, `Installer-Binary und Launcher` und `OpenCode-Registrierung`.

Beachte dabei:

- Stelle sicher, dass `~/dev/k-playbook` existiert und den Pfadvertrag erfuellt.
- Fuehre im k-playbook-Repo `make install` aus, falls der Installer noch nicht installiert ist.
- Starte danach `~/dev/k-playbook/bin/k-playbook-installer` oder `k-playbook-installer`, wenn der `PATH` bereits aktualisiert ist.
- Nutze die GUI fuer Host-Registrierung von OpenCode-/Claude-Commands und Skills.
- Starte `/k-install*` nicht aus einem aktiven Projekt-venv. Wenn `VIRTUAL_ENV` gesetzt ist oder ein Projekt-venv im `PATH` liegt, fordere zuerst `deactivate` bzw. eine bereinigte Shell an.
- Installiere fehlende Security-Tools als Teil der Host-Installation mit `/k-install-security-tools --install missing`. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-install-security-tools.md` direkt und fuehre die dort beschriebenen Schritte aus.
- Frage nur bei GitHub-nahen Projektintegrationen wie Dependabot oder CodeQL nach. Dieser Host-Prompt richtet sie nicht automatisch ein.

Pruefe am Ende:

- `~/dev/k-playbook/bin/k-playbook-installer` existiert und ist ausfuehrbar.
- `~/dev/k-playbook/commands/k-gui.md` existiert.
- `~/.config/opencode/command/k-gui.md` zeigt auf `~/dev/k-playbook/commands/k-gui.md` oder einen plausibel aufgeloesten Symlink.
- Falls vorhanden, `~/.config/opencode/commands/k-gui.md` ist ebenfalls plausibel oder erklaere, warum nur `command/` genutzt wird.
- Die OpenCode-Konfiguration enthaelt `skills.paths` mit `~/dev/k-playbook` oder es ist dokumentiert, warum das nicht automatisch angepasst wurde.
- Fehlende Security-Tools wurden installiert oder es ist klar dokumentiert, warum die Installation nicht moeglich war.

Berichte knapp, ob die Host-Registrierung vollstaendig ist und ob OpenCode neu gestartet werden muss.
