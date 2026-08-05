# Prompt: DevContainer-Projekt Setup

Du arbeitest im DevContainer des Zielprojekts.

Lies zuerst `~/dev/k-playbook/docs/installation.md`, insbesondere Abschnitte `DevContainer-Pfadvertrag` und `Projekt-Onboarding`.

Fuehre danach aus:

1. Pruefe, ob `/workspaces/k-playbook/commands/k-gui.md` existiert.
2. Pruefe, ob `~/dev/k-playbook/commands/k-gui.md` existiert und auf den Bind-Mount zeigt.
3. Pruefe, ob `~/.config/opencode/command/k-gui.md` und `~/.config/opencode/commands/k-gui.md` existieren oder plausibel repariert werden koennen.
4. Starte `/k-install*` nicht aus einem aktiven Projekt-venv. Wenn `VIRTUAL_ENV` gesetzt ist oder ein Projekt-venv im `PATH` liegt, fordere zuerst `deactivate` bzw. eine bereinigte Shell an.
5. Stelle sicher, dass das Projekt ueber die Installer-GUI eingebunden ist; dort werden `K-PLAYBOOK.yaml` und die projektlokale Struktur angelegt bzw. vervollstaendigt.
6. Installiere fehlende Security-Tools im Container mit `/k-install-security-tools --install missing --yes`. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-install-security-tools.md` direkt und fuehre die dort beschriebenen Schritte aus.
7. Frage, ob CodeQL lokal oder fuer GitHub registriert werden soll. Nur bei Zustimmung `/k-setup-codeql` und danach bei Bedarf `/k-install-codeql` nutzen.
8. Frage, ob Dependabot eingerichtet oder geprueft werden soll. Nur bei Zustimmung entsprechende projektlokale Schritte ausfuehren; nichts automatisch an GitHub-/Dependabot-Konfiguration aendern.
9. Fuehre am Ende `/k-status` aus oder starte `k-playbook-installer status`, wenn der Slash-Command nicht nutzbar ist.

Berichte am Ende knapp:

- ob `/workspaces/k-playbook` und `~/dev/k-playbook` korrekt verbunden sind.
- ob OpenCode-Command-Links und `skills.paths` plausibel sind.
- ob `K-PLAYBOOK.yaml` im Zielprojekt eingerichtet wurde.
- ob Security-Tools installiert wurden oder warum das nicht moeglich war.
- welche optionalen GitHub-/Projektintegrationen wie CodeQL oder Dependabot uebersprungen oder ausgefuehrt wurden.
