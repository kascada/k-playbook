---
description: Install or refresh k-playbook for the current host. Registers OpenCode slash-command symlinks, checks skill-path configuration, and runs a host-local security-tool preflight. Run once per server, and again after adding new command files.
# model: github-copilot/gpt-5.4-mini
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-install

Installiert oder aktualisiert die globale k-playbook-Registrierung auf dem aktuellen Server/Host.

Dieser Command ist **host-lokal**: Er richtet OpenCode so ein, dass die globalen Slash-Commands aus diesem Repo verfügbar sind. Er verändert keine Projektdateien und kein `K-PLAYBOOK.MD`.

Typische Nutzung:
- Einmal pro Server nach dem Klonen von `k-playbook`.
- Erneut nach dem Hinzufügen neuer Dateien unter `commands/k-*.md`, damit neue Symlinks angelegt werden.
- Nach dem Einrichten eines Hosts, um den Status der global genutzten Security-Review-Tools zu sehen.

Aufrufort:
- Bevorzugt direkt im k-playbook-Repo nach Clone/Pull.
- Aus einem Zielprojekt ist erlaubt; `/k-install` nutzt trotzdem den festen Pfadvertrag `~/dev/k-playbook`.
- Der Effekt bleibt immer host-global: OpenCode-Symlinks und Skill-Pfad werden fuer diesen Host aktualisiert, nicht das aktuelle Projekt.

Pfadvertrag:
- Der feste logische Repo-Pfad ist `~/dev/k-playbook`.
- `K-PLAYBOOK.MD` darf diesen Pfad sichtbar als `repo: ~/dev/k-playbook` enthalten, aber `/k-install` fragt ihn nicht ab und behandelt ihn nicht als frei waehlbar.
- Wenn der echte Klon woanders liegt, soll `/k-install` vorschlagen, ihn nach `~/dev/k-playbook` zu verschieben. Wenn der User das nicht will, darf `/k-install` nach Bestaetigung einen Symlink von `~/dev/k-playbook` auf den echten Klon anlegen.
- In Devcontainern gilt derselbe Vertrag: `~/dev/k-playbook` muss existieren; typischerweise ist `/home/vscode/dev/k-playbook` ein Symlink auf `/workspaces/k-playbook`. Dafuer muss das Host-Repo zuerst als Bind-Mount nach `/workspaces/k-playbook` im Container verfuegbar sein.

Hinweis zum Bootstrap:
Wenn `/k-install` auf einem frischen Server oder in einem Devcontainer noch nicht im Slash-Command-Menü sichtbar ist, einmal die manuelle Installation aus `docs/installation.md` ausführen oder zumindest diesen Command direkt symlinken:

```bash
mkdir -p ~/.config/opencode/command
ln -sf ~/dev/k-playbook/commands/k-install.md ~/.config/opencode/command/k-install.md
```

Danach OpenCode neu starten und `/k-install` ausführen.

In Devcontainern muss vor diesem Bootstrap zusaetzlich das Repo nach `/workspaces/k-playbook` gemountet und `~/dev/k-playbook` darauf verlinkt sein. `/k-install` ist kein Shell-Executable, sondern eine OpenCode-Slash-Command-Datei; Devcontainer-Lifecycle-Hooks koennen nur die Mount-/Symlink-/Bootstrap-Schritte vorbereiten.

Empfohlener Weg fuer Zielprojekte mit Devcontainer:

```bash
~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh /pfad/zum/zielprojekt
```

Das Script kopiert `scripts/templates/devcontainer-setup-k-playbook.sh` ins Zielprojekt als `.devcontainer/setup-k-playbook.sh` und registriert es in `devcontainer.json`.

---

## Schritt 1 — Pfadvertrag sicherstellen

Setze immer:

```text
PLAYBOOK_REPO=~/dev/k-playbook
```

Expand `~` gegen das Home des aktuellen Users (`/home/vscode` im Devcontainer). Danach pruefen:

1. Wenn `~/dev/k-playbook/commands/k-install.md` existiert: Pfadvertrag ist erfuellt.
2. Wenn der aktuelle Arbeitsordner selbst ein k-playbook-Repo ist (`commands/` existiert und enthält `k-*.md`), aber nicht `~/dev/k-playbook`:
   - Dem User sagen: `k-playbook` soll unter `~/dev/k-playbook` erreichbar sein.
   - Vorschlag 1: Repo nach `~/dev/k-playbook` verschieben/klonen.
   - Wenn der User nicht verschieben will: nach Bestaetigung `mkdir -p ~/dev && ln -sfn "<aktueller-repo-pfad>" ~/dev/k-playbook` ausfuehren.
3. Wenn `/workspaces/k-playbook/commands/k-install.md` existiert und `~/dev/k-playbook` fehlt, ist dies der Devcontainer-Fall:
   - Nach Bestaetigung oder wenn dies aus einem Devcontainer-Setup-Skript laeuft: `mkdir -p ~/dev && ln -sfn /workspaces/k-playbook ~/dev/k-playbook` ausfuehren.
4. Wenn `~/dev/k-playbook` fehlt und kein plausibler Klon aus aktuellem Arbeitsordner oder `/workspaces/k-playbook` erkennbar ist:
   - Abbrechen und dem User vorschlagen, das Repo dorthin zu klonen:

```bash
gh repo clone kascada/k-playbook ~/dev/k-playbook
```

Nicht nach einem alternativen dauerhaften Repo-Pfad fragen. Der Vertrag bleibt `~/dev/k-playbook`; Abweichungen werden ueber Symlinks geloest.

Anschließend prüfen:

- `<PLAYBOOK_REPO>/commands/` existiert.
- Es gibt mindestens eine Datei `<PLAYBOOK_REPO>/commands/k-*.md`.

Wenn nicht: abbrechen mit klarer Fehlermeldung.

---

## Schritt 2 — Zielpfade bestimmen

Für OpenCode:

- `OPENCODE_CONFIG_DIR` = `~/.config/opencode`
- `OPENCODE_COMMAND_DIR` = `~/.config/opencode/command`
- `OPENCODE_CONFIG_FILE` = bevorzugt `~/.config/opencode/opencode.jsonc`, falls vorhanden; sonst `~/.config/opencode/opencode.json`, falls vorhanden; sonst `~/.config/opencode/opencode.jsonc` neu anlegen (nach Rückfrage, siehe Schritt 4).

Für Claude Code nur prüfen/hinweisen, nicht automatisch ändern:

- `CLAUDE_COMMANDS` = `~/.claude/commands`

---

## Schritt 3 — Slash-Command-Symlinks installieren

1. `OPENCODE_COMMAND_DIR` anlegen, falls es fehlt.
2. Alle `<PLAYBOOK_REPO>/commands/k-*.md` sammeln.
3. Für jede Datei einen Symlink mit gleichem Dateinamen in `OPENCODE_COMMAND_DIR` setzen:
   - Ziel: `<PLAYBOOK_REPO>/commands/k-<name>.md`
   - Link: `<OPENCODE_COMMAND_DIR>/k-<name>.md`
   - Befehl: `ln -sf <target> <link>`

Nicht löschen:
- Keine fremden Dateien in `OPENCODE_COMMAND_DIR` entfernen.
- Keine alten `k-*.md`-Links entfernen, die nicht mehr im Repo existieren. Stattdessen am Ende als „verwaist" melden und fragen, ob sie entfernt werden sollen.

**Verwaiste Links:**

Wenn ein Link in `OPENCODE_COMMAND_DIR` auf eine nicht mehr existierende Datei unter `<PLAYBOOK_REPO>/commands/` zeigt:

1. Liste zeigen.
2. Fragen:
   > "Diese k-playbook-Command-Links zeigen ins Leere. Soll ich sie entfernen?"
3. Nur bei Bestätigung löschen.

---

## Schritt 4 — Skill-Pfad prüfen

OpenCode-Skills werden über `skills.paths` registriert. Prüfen, ob `~/dev/k-playbook` in der OpenCode-Konfig enthalten ist.

Vorgehen:

1. Wenn `OPENCODE_CONFIG_FILE` existiert: lesen.
2. Wenn kein `skills.paths` existiert oder `~/dev/k-playbook` nicht enthalten ist: Änderung vorschlagen.
3. Wenn die Datei nicht existiert: fragen, ob sie angelegt werden soll.

Vorgeschlagene Minimal-Konfig, falls neu:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

Wenn die Datei existiert:
- Vor dem Editieren kurz zeigen, welche Änderung geplant ist.
- Bestehende Inhalte erhalten.
- Nur `skills.paths` ergänzen.
- Wenn die JSON/JSONC-Struktur nicht sicher editierbar ist: nicht raten, sondern dem User den einzufügenden Block zeigen.

---

## Schritt 5 — Optionaler Claude-Hinweis

Wenn `~/.claude/` existiert: Status anzeigen.

Empfohlener Symlink für Claude Code:

```bash
ln -sfn ~/dev/k-playbook/commands ~/.claude/commands
```

Nicht automatisch ausführen, außer der User fragt ausdrücklich danach. Dieser Command ist primär für OpenCode.

---

## Schritt 6 — Security-Tool-Preflight

Am Ende host-lokal pruefen, ob die Security-Review-Tools vorhanden sind. Dieser Schritt installiert nichts automatisch und schreibt keine Projektdateien.

Wenn ein Python-venv aktiv ist (`VIRTUAL_ENV` gesetzt) oder ein typischer Projekt-venv-Pfad wie `.venv/bin`, `venv/bin` oder `env/bin` in `PATH` steht, den Security-Tool-Preflight nicht ausfuehren und nicht als host-global werten. User auffordern, `deactivate` auszufuehren bzw. `PATH` zu bereinigen und `/k-install-security-tools` danach separat zu starten.

Wenn `<PLAYBOOK_REPO>/scripts/install-security-tools.sh` existiert:

```bash
bash "<PLAYBOOK_REPO>/scripts/install-security-tools.sh" --preflight
```

Pruefe und berichte kompakt:

- Pflicht-Tools: `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype`.
- Docker-Verfuegbarkeit fuer Fallbacks.

Wenn Pflicht-Tools fehlen, **nicht** aus `/k-install` heraus installieren. Stattdessen den Folge-Command nennen:

```text
/k-install-security-tools --install missing
```

Wenn der User waehrend `/k-install` ausdruecklich installieren will, `/k-install-security-tools` separat aufrufen lassen oder nach expliziter Bestaetigung dessen Script-Befehl ausfuehren. Keine stille Installation.

Wenn das Script fehlt, nur warnen und mit der OpenCode-Verifikation fortfahren; die Command-/Skill-Registrierung darf dadurch nicht fehlschlagen.

---

## Schritt 7 — Verifikation

Nach der Installation prüfen:

- `OPENCODE_COMMAND_DIR` existiert.
- Für jede Datei `<PLAYBOOK_REPO>/commands/k-*.md` existiert ein Symlink in `OPENCODE_COMMAND_DIR`.
- Die Symlinks zeigen auf existierende Dateien.
- `skills.paths` enthält `PLAYBOOK_REPO` oder es wurde erklärt, warum nicht automatisch geändert wurde.
- Security-Tool-Preflight wurde ausgefuehrt oder als nicht verfuegbar gemeldet.

Ausgabe:

```text
k-install abgeschlossen
─────────────────────────────────────
Repo:        ~/dev/k-playbook
Commands:    <n> Symlinks ok / <m> neu oder aktualisiert
Skills:      ok | fehlt | manuell nötig
SecTools:    ok | <n> Pflicht-Tools fehlen | Preflight fehlt
Verwaist:    <n> alte Links

Wichtig: OpenCode neu starten, damit neue Commands und Skills sichtbar werden.
```

---

## Fehlerfälle

- **Repo nicht gefunden**: nicht nach einem alternativen Dauerpfad fragen; Clone nach `~/dev/k-playbook` oder Symlink dorthin vorschlagen.
- **Keine Commands gefunden**: abbrechen — `~/dev/k-playbook` zeigt nicht auf ein k-playbook-Repo.
- **OpenCode-Konfig nicht sicher editierbar**: nicht überschreiben; stattdessen konkrete manuelle Änderung ausgeben.
- **Symlink-Ziel existiert nicht**: Link nicht anlegen; Fehler in Zusammenfassung melden.
