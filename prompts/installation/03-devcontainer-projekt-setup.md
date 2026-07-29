# Prompt: DevContainer-Projekt Setup

Du arbeitest im DevContainer des Zielprojekts.

Lies zuerst `~/dev/k-playbook/docs/multi-project-installation.md`, insbesondere Abschnitt `6. DevContainer Neu Erzeugen` bis `10. Status Pruefen`.

Fuehre danach aus:

1. Pruefe, ob `/workspaces/k-playbook/commands/k-install.md` existiert.
2. Pruefe, ob `~/dev/k-playbook/commands/k-install.md` existiert.
3. Pruefe, ob `~/.config/opencode/command/k-install.md` und `~/.config/opencode/commands/k-install.md` existieren oder plausibel repariert werden koennen.
4. Wenn `/k-install` in OpenCode sichtbar ist, fuehre `/k-install` aus. Wenn du Slash-Commands in deiner Umgebung nicht selbst ausloesen kannst oder `/k-install` nicht sichtbar ist, lies `~/dev/k-playbook/commands/k-install.md` direkt und fuehre die dort beschriebenen Bootstrap-/Installationsschritte fuer den Container aus.
5. Starte `/k-install*` nicht aus einem aktiven Projekt-venv. Wenn `VIRTUAL_ENV` gesetzt ist oder ein Projekt-venv im `PATH` liegt, fordere zuerst `deactivate` bzw. eine bereinigte Shell an.
6. Fuehre im Projektroot `/k-setup` aus, um `K-PLAYBOOK.MD` und die projektlokalen Pfade einzurichten oder zu aktualisieren. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-setup.md` direkt und fuehre die dort beschriebenen Schritte aus.
7. Frage, ob CodeQL lokal registriert werden soll. Nur bei Zustimmung `/k-setup-codeql` und danach bei Bedarf `/k-install-codeql` nutzen.
8. Frage, ob fehlende Security-Tools im Container installiert werden sollen. Nur bei Zustimmung `/k-install-security-tools --install missing --yes` nutzen.
9. Fuehre am Ende `/k-status` aus oder, falls der Slash-Command nicht nutzbar ist, lies `~/dev/k-playbook/commands/k-status.md` direkt und pruefe die dort beschriebenen Statuspunkte.

Berichte am Ende knapp:

- ob `/workspaces/k-playbook` und `~/dev/k-playbook` korrekt verbunden sind.
- ob OpenCode-Command-Links und `skills.paths` plausibel sind.
- ob `K-PLAYBOOK.MD` im Zielprojekt eingerichtet wurde.
- welche optionalen Schritte uebersprungen oder ausgefuehrt wurden.
