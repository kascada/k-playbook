# Regel: Tool-Installation und Projekt-venvs trennen

## Zweck

Host-lokale k-playbook Tools dürfen nicht versehentlich in Projekt-venvs installiert oder daraus als globale Tools erkannt werden.

## Grundsatz

- `/k-install*` installiert oder prüft host-/user-lokales Tooling.
- `/k-setup*` konfiguriert einzelne Projekte.
- Projekt-venvs gehören dem jeweiligen Projekt und enthalten nur dessen Runtime-, Test- und Entwicklungsabhängigkeiten.

## Erlaubte Installationsziele

Host-lokale CLI-Tools dürfen verwendet werden über:

- native/user-lokale Binaries, z. B. `~/.opencode/bin` oder `~/.local/bin`.
- `pipx` für Python-CLI-Tools.
- dedizierte k-playbook Tool-venvs unter `~/.local/share/k-playbook/`, wenn `pipx` nicht verfügbar oder nicht passend ist.
- Docker-Images als explizite Fallbacks.

## Verboten

- Kein `/k-install*` darf in ein Projekt-venv wie `.venv/`, `venv/` oder `env/` installieren.
- Kein `/k-install*` darf ein aktives `VIRTUAL_ENV` als Installationskontext nutzen.
- Preflights dürfen Tools aus einem aktiven Projekt-venv nicht als host-global vorhanden werten.
- Projekt-Dependencies dürfen nicht über `/k-install*` in Projekt-venvs nachinstalliert werden.

## Konsequenz für Erweiterungen

Neue Installer oder neue Tools müssen vor Installation und Preflight prüfen:

- ob `VIRTUAL_ENV` gesetzt ist und dann abbrechen oder den User zum `deactivate` auffordern.
- ob Zielpfade in typischen Projekt-venvs liegen und dann abbrechen.
- ob Python-CLI-Tools bevorzugt via `pipx` oder dediziertem Tool-venv installiert werden.

Wenn ein Projekt zusätzliche Bibliotheken braucht, gehört das in Projekt-Dokumentation, Projekt-Setup oder das jeweilige Dependency-Management, nicht in `/k-install*`.
