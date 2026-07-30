# Prompt: Projekt Ohne DevContainer Setup

Du arbeitest im Zielprojekt auf dem Host, nicht in einem DevContainer.

Lies zuerst `~/dev/k-playbook/docs/multi-project-installation.md`, insbesondere Abschnitt `3A. Projekt ohne DevContainer einrichten`.

Fuehre danach aus:

1. Pruefe, ob du im richtigen Projektroot bist. Wenn unklar ist, welches Zielprojekt eingerichtet werden soll, frage den User nach dem Pfad.
2. Pruefe, ob `~/dev/k-playbook/commands/k-install.md` existiert.
3. Pruefe, ob `~/.config/opencode/command/k-install.md` existiert und ob `skills.paths` `~/dev/k-playbook` enthaelt. Wenn nicht, fuehre zuerst Prompt `01-host-opencode-registrieren.md` bzw. die dort beschriebenen Bootstrap-Schritte aus.
4. Starte `/k-setup` nicht aus der Annahme heraus, dass Projekt-Dependencies installiert werden. `/k-setup` registriert nur projektlokale k-playbook-Pfade.
5. Stelle sicher, dass das Projekt im k-playbook Installer eingebunden ist; dort werden `K-PLAYBOOK.yaml` und die projektlokale Struktur angelegt bzw. vervollstaendigt. Fuehre danach im Projektroot `/k-setup` aus, um die Kernkonfiguration zu pruefen oder zu aktualisieren. Wenn du Slash-Commands nicht selbst ausloesen kannst, lies `~/dev/k-playbook/commands/k-setup.md` direkt und fuehre die dort beschriebenen Schritte aus.
6. Frage, ob CodeQL lokal oder fuer GitHub registriert werden soll. Nur bei Zustimmung `/k-setup-codeql` und danach bei Bedarf `/k-install-codeql` nutzen. `/k-install-codeql` nicht aus einem aktiven Projekt-venv starten.
7. Frage, ob Dependabot eingerichtet oder geprueft werden soll. Nur bei Zustimmung entsprechende projektlokale Schritte ausfuehren; nichts automatisch an GitHub-/Dependabot-Konfiguration aendern.
8. Fuehre am Ende `/k-status` aus oder, falls der Slash-Command nicht nutzbar ist, lies `~/dev/k-playbook/commands/k-status.md` direkt und pruefe die dort beschriebenen Statuspunkte.

Berichte am Ende knapp:

- ob das Zielprojekt ohne DevContainer eingerichtet wurde.
- ob `K-PLAYBOOK.yaml` existiert oder aktualisiert wurde.
- ob OpenCode-Command-Links und `skills.paths` plausibel sind.
- welche optionalen GitHub-/Projektintegrationen wie CodeQL oder Dependabot uebersprungen oder ausgefuehrt wurden.
