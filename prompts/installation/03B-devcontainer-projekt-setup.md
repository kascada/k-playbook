# Prompt: DevContainer-Projekt Setup

Du arbeitest im DevContainer des Zielprojekts.

Lies zuerst `~/dev/k-playbook/docs/multi-project-installation.md`, insbesondere Abschnitt `3B. Projekt mit DevContainer einrichten`.

Fuehre danach aus:

1. Pruefe, ob `/workspaces/k-playbook/commands/k-install.md` existiert.
2. Pruefe, ob `~/dev/k-playbook/commands/k-install.md` existiert.
3. Pruefe, ob `~/.config/opencode/command/k-install.md` und `~/.config/opencode/commands/k-install.md` existieren oder plausibel repariert werden koennen.
4. Wenn `/k-install` in OpenCode sichtbar ist, fuehre `/k-install` aus. Wenn du Slash-Commands in deiner Umgebung nicht selbst ausloesen kannst oder `/k-install` nicht sichtbar ist, lies `~/dev/k-playbook/commands/k-install.md` direkt und fuehre die dort beschriebenen Bootstrap-/Installationsschritte fuer den Container aus.
5. Starte `/k-install*` nicht aus einem aktiven Projekt-venv. Wenn `VIRTUAL_ENV` gesetzt ist oder ein Projekt-venv im `PATH` liegt, fordere zuerst `deactivate` bzw. eine bereinigte Shell an.
6. Fuehre im Projektroot `/k-setup` aus, um `K-PLAYBOOK.yaml` und die projektlokale Struktur einzurichten oder zu aktualisieren. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-setup.md` direkt und fuehre die dort beschriebenen Schritte aus.
7. Installiere fehlende Security-Tools im Container als Teil der Standardinstallation mit `/k-install-security-tools --install missing --yes`. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-install-security-tools.md` direkt und fuehre die dort beschriebenen Schritte aus.
8. Frage, ob CodeQL lokal oder fuer GitHub registriert werden soll. Nur bei Zustimmung `/k-setup-codeql` und danach bei Bedarf `/k-install-codeql` nutzen.
9. Frage, ob Dependabot eingerichtet oder geprueft werden soll. Nur bei Zustimmung entsprechende projektlokale Schritte ausfuehren; nichts automatisch an GitHub-/Dependabot-Konfiguration aendern.
10. Fuehre am Ende `/k-status` aus oder, falls der Slash-Command nicht nutzbar ist, lies `~/dev/k-playbook/commands/k-status.md` direkt und pruefe die dort beschriebenen Statuspunkte.

Berichte am Ende knapp:

- ob `/workspaces/k-playbook` und `~/dev/k-playbook` korrekt verbunden sind.
- ob OpenCode-Command-Links und `skills.paths` plausibel sind.
- ob `K-PLAYBOOK.yaml` im Zielprojekt eingerichtet wurde.
- ob Security-Tools installiert wurden oder warum das nicht moeglich war.
- welche optionalen GitHub-/Projektintegrationen wie CodeQL oder Dependabot uebersprungen oder ausgefuehrt wurden.
