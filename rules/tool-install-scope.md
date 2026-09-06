# Regel: Tool-Installation und Projekt-venvs trennen

## Zweck

Host-lokale k-playbook Tools dürfen nicht versehentlich in Projekt-venvs installiert oder daraus als globale Tools erkannt werden.

## Grundsatz

- `/k-install*` installiert oder prüft host-/user-lokales Tooling.
- `/k-setup*` konfiguriert einzelne Projekte.
- Projekt-venvs gehören dem jeweiligen Projekt und enthalten nur dessen Runtime-, Test- und Entwicklungsabhängigkeiten.
- Projekte dürfen mit aktivem venv arbeiten. Read-only-Preflights dürfen dieses venv als Messkontext nutzen, müssen es aber als solchen kennzeichnen.

## Erlaubte Installationsziele

Host-lokale CLI-Tools dürfen verwendet werden über:

- native/user-lokale Binaries, z. B. `~/.opencode/bin` oder `~/.local/bin`.
- `pipx` für Python-CLI-Tools.
- dedizierte k-playbook Tool-venvs unter `~/.local/share/k-playbook/`, wenn `pipx` nicht verfügbar, nicht passend oder explizit unerwünscht ist.
- Docker-Images als explizite Fallbacks.
- eine **systemweite Paketinstallation über den Paketmanager der Distribution** (`apt-get` und Verwandte), unter drei Bedingungen: Sie läuft als root, sie eskaliert sich nicht selbst dorthin, und sie berührt kein Projekt-venv. Das ist der Weg des Image-Builds und der Basis-Werkzeuge; für Werkzeuge wie `git`, `curl`, `tar` oder `python3` gibt es keinen sinnvollen user-lokalen Weg.

„Eskaliert sich nicht selbst" heißt wörtlich: Kein Installer startet sich per `sudo` neu. Ist root nötig und nicht vorhanden, wird der Befehl **ausgegeben** und der Lauf endet dort — mit einem Rückgabewert, der „kein user-lokaler Weg" vom Fehlschlag trennt.

Der venv-Guard gilt unverändert für **alle** Wege, den systemweiten eingeschlossen.

Zusätzlich gilt für jeden schreibenden Aufruf: Gehört das aufgelöste Installationsziel — ersatzweise sein nächstes vorhandenes Elternverzeichnis — nicht der effektiven UID, wird der Aufruf abgewiesen. Das trifft user-lokale Ziele, die an `$HOME` hängen (der `sudo`-Tippfehler), nicht den systemweiten Weg, dessen Ziel root gehört. Lesende Läufe sind ausgenommen.

## Verboten

- Kein `/k-install*` darf in ein Projekt-venv wie `.venv/`, `venv/` oder `env/` installieren.
- Kein `/k-install*` darf ein aktives `VIRTUAL_ENV` als Installationskontext nutzen.
- Preflights dürfen Tools aus einem aktiven Projekt-venv nicht als host-global vorhanden werten; wenn sie sie messen, muss der Kontext sichtbar sein.
- Projekt-Dependencies dürfen nicht über `/k-install*` in Projekt-venvs nachinstalliert werden.
- Kein Installer darf sich selbst mit erweiterten Rechten neu starten. Ein `sudo`-Befehl wird gezeigt, nie ausgeführt.

## Konsequenz für Erweiterungen

Neue Installer oder neue Tools müssen vor Installation und Preflight prüfen:

- ob `VIRTUAL_ENV` gesetzt ist und dann Installationen abbrechen oder den User zum `deactivate` auffordern.
- ob Zielpfade in typischen Projekt-venvs liegen und dann abbrechen.
- ob Python-CLI-Tools bevorzugt via `pipx` oder dediziertem Tool-venv installiert werden.
- ob das aufgelöste Installationsziel — ersatzweise sein nächstes vorhandenes Elternverzeichnis — der effektiven UID gehört, und sonst abbrechen. Geprüft wird das tatsächliche Ziel, nachdem Optionen und Umgebungs-Overrides ausgewertet sind, nicht das Default-Ziel.
- ob der systemweite Weg gemeint ist: dann muss er ausdrücklich benannt sein (ein Zielpfad außerhalb von `$HOME` oder der Paketmanager) und als root laufen, statt sich dorthin zu eskalieren.

Wenn ein Projekt zusätzliche Bibliotheken braucht, gehört das in Projekt-Dokumentation, Projekt-Setup oder das jeweilige Dependency-Management, nicht in `/k-install*`.
