# k-playbook FAQ

## Wann rufe ich `/k-install` auf?

`/k-install` ist host-global. Rufe es auf:

- einmal pro Host nach dem Klonen von `k-playbook`.
- nach einem Pull/Update, wenn neue oder geaenderte Dateien unter `commands/k-*.md` fuer OpenCode sichtbar werden sollen.
- wenn OpenCode-Symlinks oder `skills.paths` auf diesem Host geprueft oder repariert werden sollen.

`/k-install` ist kein Projekt-Setup. Fuer einzelne Projekte ist `/k-setup` zustaendig.

Wenn du nur pruefen willst, ob die OpenCode-Symlinks und `skills.paths` stimmen, nutze `/k-status`. Der Command ist read-only und empfiehlt `/k-install`, wenn die host-lokale Registrierung repariert werden muss.

## Muss ich `/k-install` im k-playbook-Repo ausfuehren?

Bevorzugt ja: direkt im k-playbook-Repo, z. B. `~/dev/k-playbook`, nach Clone oder Pull. Dann ist eindeutig, welches Repo fuer OpenCode registriert wird.

Aus einem Zielprojekt heraus ist `/k-install` ebenfalls erlaubt. Der Command nutzt trotzdem den festen Pfadvertrag `~/dev/k-playbook`; `K-PLAYBOOK.MD` waehlt keinen alternativen Basis-Repo-Pfad.

Der Effekt bleibt in beiden Faellen host-global:

- OpenCode-Command-Symlinks werden aktualisiert.
- `skills.paths` wird geprueft oder ergaenzt.
- optional wird der Security-Tool-Preflight gezeigt.
- Projektdateien werden nicht geaendert.

Wenn der k-playbook-Klon woanders liegt, soll er nach `~/dev/k-playbook` verschoben/geklont werden. Wenn du das nicht willst, legt `/k-install` nach Bestaetigung einen Symlink nach `~/dev/k-playbook` an.

## Warum ist `~/dev/k-playbook` fest?

Damit Projektdateien, OpenCode-Config, Host und Devcontainer denselben logischen Pfad nutzen. Das echte Repo darf physisch woanders liegen, aber jede Umgebung muss `~/dev/k-playbook` bereitstellen.

Host-Beispiel:

```bash
mkdir -p ~/dev
ln -sfn /anderer/pfad/k-playbook ~/dev/k-playbook
```

Devcontainer-Beispiel:

```bash
mkdir -p /home/vscode/dev
ln -sfn /workspaces/k-playbook /home/vscode/dev/k-playbook
```

## Was ist der Unterschied zwischen `/k-install` und `/k-setup`?

- `/k-install*` gehoert zum Host: OpenCode-Registrierung, host-lokale Tools, Security-Tool-Preflight.
- `/k-setup*` gehoert zum Projekt: `K-PLAYBOOK.MD`, projektlokale Pfade, Projektentscheidungen.

Merksatz:

```text
/k-install*  = Host/User-Tooling
/k-setup*    = Projekt-Konfiguration
```

## Darf `/k-install*` in einem aktiven Python-venv laufen?

Nein. Vor `/k-install-security-tools` und host-globalen Tool-Preflights darf kein Projekt-venv aktiv sein.

Wenn `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

Auch `.venv/bin`, `venv/bin` oder `env/bin` im `PATH` sind fuer `/k-install-security-tools` nicht erlaubt. Sonst koennte ein Tool aus einem Projekt-venv faelschlich als host-global vorhanden erkannt werden.

Python-CLI-Tools gehoeren in `pipx` oder in ein dediziertes k-playbook Tool-venv unter `~/.local/share/k-playbook/`, nicht in `<projekt>/.venv`.

## Wann rufe ich `/k-install-security-tools` auf?

Nach `/k-install`, wenn Pflicht-Tools fehlen:

```text
/k-install-security-tools --install missing
```

Der Command installiert host-/user-lokale Review-Tools wie `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft` und `grype`. Er schreibt keine Projektdateien und startet keine Scans.

## Wann muss ich `/k-install` aus einem Projekt heraus aufrufen?

Nur wenn es praktisch ist oder wenn du gerade in einem Zielprojekt arbeitest und die globale k-playbook-Registrierung dieses Hosts aktualisieren willst.

Voraussetzung: `~/dev/k-playbook` funktioniert auf diesem Host bzw. im Container. Wenn das Projekt noch nicht eingerichtet ist, zuerst im k-playbook-Repo `/k-install` ausfuehren und danach im Projekt `/k-setup`.
