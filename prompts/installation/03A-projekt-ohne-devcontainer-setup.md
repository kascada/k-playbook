# Prompt: Projekt Ohne DevContainer Setup

Du arbeitest im Zielprojekt auf dem Host, nicht in einem DevContainer.

Lies zuerst `~/dev/k-playbook/docs/installation.md`, insbesondere Abschnitt `Projekt-Onboarding`.

Fuehre danach aus:

1. Pruefe, ob du im richtigen Projektroot bist. Wenn unklar ist, welches Zielprojekt eingerichtet werden soll, frage den User nach dem Pfad.
2. Pruefe, ob `~/dev/k-playbook/bin/k-playbook-installer` und `~/dev/k-playbook/commands/k-gui.md` existieren.
3. Pruefe, ob OpenCode `k-gui` registriert hat und ob `skills.paths` `~/dev/k-playbook` enthaelt. Wenn nicht, fuehre zuerst Prompt `01-host-opencode-registrieren.md` aus.
4. Stelle sicher, dass das Projekt ueber die Installer-GUI eingebunden ist; dort werden `K-PLAYBOOK.yaml` und die projektlokale Struktur angelegt bzw. vervollstaendigt.
5. Frage, ob CodeQL lokal oder fuer GitHub registriert werden soll. Nur bei Zustimmung `/k-setup-codeql` und danach bei Bedarf `/k-install-codeql` nutzen. `/k-install-codeql` nicht aus einem aktiven Projekt-venv starten.
6. Frage, ob Dependabot eingerichtet oder geprueft werden soll. Nur bei Zustimmung entsprechende projektlokale Schritte ausfuehren; nichts automatisch an GitHub-/Dependabot-Konfiguration aendern.
7. Fuehre am Ende `/k-status` aus oder starte `k-playbook-installer status`, wenn der Slash-Command nicht nutzbar ist.

Berichte am Ende knapp:

- ob das Zielprojekt ohne DevContainer eingerichtet wurde.
- ob `K-PLAYBOOK.yaml` existiert oder aktualisiert wurde.
- ob OpenCode-Command-Links und `skills.paths` plausibel sind.
- welche optionalen GitHub-/Projektintegrationen wie CodeQL oder Dependabot uebersprungen oder ausgefuehrt wurden.
