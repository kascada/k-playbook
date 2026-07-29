# Prompt: Host-OpenCode Registrieren

Du arbeitest auf dem Host, nicht in einem DevContainer.

Lies zuerst `~/dev/k-playbook/docs/multi-project-installation.md` und fuehre Abschnitt `2. Host-OpenCode Registrieren` aus.

Beachte dabei:

- Wenn `/k-install` in OpenCode bereits sichtbar ist, fuehre `/k-install` aus.
- Wenn du Slash-Commands in deiner Umgebung nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-install.md` direkt und fuehre die dort beschriebenen Schritte aus.
- Wenn `/k-install` nach frischem Clone noch nicht sichtbar ist, lies `~/dev/k-playbook/commands/k-install.md` direkt und fuehre die dort beschriebenen Bootstrap-/Installationsschritte aus.
- Starte `/k-install*` nicht aus einem aktiven Projekt-venv. Wenn `VIRTUAL_ENV` gesetzt ist oder ein Projekt-venv im `PATH` liegt, fordere zuerst `deactivate` bzw. eine bereinigte Shell an.
- Installiere fehlende Security-Tools als Teil der Host-Installation mit `/k-install-security-tools --install missing`. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-install-security-tools.md` direkt und fuehre die dort beschriebenen Schritte aus.
- Frage nur bei GitHub-nahen Projektintegrationen wie Dependabot oder CodeQL nach. Dieser Host-Prompt richtet sie nicht automatisch ein.

Pruefe am Ende:

- `~/dev/k-playbook/commands/k-install.md` existiert.
- `~/.config/opencode/command/k-install.md` zeigt auf `~/dev/k-playbook/commands/k-install.md`.
- Falls vorhanden, `~/.config/opencode/commands/k-install.md` ist ebenfalls plausibel oder erklaere, warum nur `command/` genutzt wird.
- Die OpenCode-Konfiguration enthaelt `skills.paths` mit `~/dev/k-playbook` oder es ist dokumentiert, warum das nicht automatisch angepasst wurde.
- Fehlende Security-Tools wurden installiert oder es ist klar dokumentiert, warum die Installation nicht moeglich war.

Berichte knapp, ob die Host-Registrierung vollstaendig ist und ob OpenCode neu gestartet werden muss.
