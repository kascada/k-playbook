# Regel: Tool-Installation und Projekt-venvs trennen

## Zweck

Host-lokale k-playbook Tools duerfen nicht versehentlich in Projekt-venvs installiert oder daraus als globale Tools erkannt werden.

## Grundsatz

- `/k-install*` installiert oder prueft host-/user-lokales Tooling.
- `/k-setup*` konfiguriert einzelne Projekte.
- Projekt-venvs gehoeren dem jeweiligen Projekt und enthalten nur dessen Runtime-, Test- und Entwicklungsabhaengigkeiten.

## Erlaubte Installationsziele

Host-lokale CLI-Tools duerfen verwendet werden ueber:

- native/user-lokale Binaries, z. B. `~/.opencode/bin` oder `~/.local/bin`.
- `pipx` fuer Python-CLI-Tools.
- dedizierte k-playbook Tool-venvs unter `~/.local/share/k-playbook/`, wenn `pipx` nicht verfuegbar oder nicht passend ist.
- Docker-Images als explizite Fallbacks.

## Verboten

- Kein `/k-install*` darf in ein Projekt-venv wie `.venv/`, `venv/` oder `env/` installieren.
- Kein `/k-install*` darf ein aktives `VIRTUAL_ENV` als Installationskontext nutzen.
- Preflights duerfen Tools aus einem aktiven Projekt-venv nicht als host-global vorhanden werten.
- Projekt-Dependencies duerfen nicht ueber `/k-install*` in Projekt-venvs nachinstalliert werden.

## Konsequenz fuer Erweiterungen

Neue Installer oder neue Tools muessen vor Installation und Preflight pruefen:

- ob `VIRTUAL_ENV` gesetzt ist und dann abbrechen oder den User zum `deactivate` auffordern.
- ob Zielpfade in typischen Projekt-venvs liegen und dann abbrechen.
- ob Python-CLI-Tools bevorzugt via `pipx` oder dediziertem Tool-venv installiert werden.

Wenn ein Projekt zusaetzliche Bibliotheken braucht, gehoert das in Projekt-Dokumentation, Projekt-Setup oder das jeweilige Dependency-Management, nicht in `/k-install*`.
