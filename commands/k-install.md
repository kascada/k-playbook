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

Hinweis zum Bootstrap:
Wenn `/k-install` auf einem frischen Server noch nicht im Slash-Command-Menü sichtbar ist, einmal die manuelle Installation aus `install.md` ausführen oder zumindest diesen Command direkt symlinken:

```bash
mkdir -p ~/.config/opencode/command
ln -sf ~/dev/k-playbook/commands/k-install.md ~/.config/opencode/command/k-install.md
```

Danach OpenCode neu starten und `/k-install` ausführen.

---

## Schritt 1 — Playbook-Repo bestimmen

Bestimme `PLAYBOOK_REPO`:

1. Wenn der aktuelle Arbeitsordner selbst das k-playbook-Repo ist (`commands/` existiert und enthält `k-*.md`): diesen Pfad verwenden.
2. Sonst, wenn `K-PLAYBOOK.MD` im aktuellen Projekt existiert: aus `## Playbook-Quelle` den Wert `repo:` lesen.
3. Sonst Default prüfen: `~/dev/k-playbook`.
4. Wenn unklar oder nicht vorhanden: User fragen:
   > "Wo liegt das k-playbook-Repo auf diesem Server?"

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

OpenCode-Skills werden über `skills.paths` registriert. Prüfen, ob `PLAYBOOK_REPO` in der OpenCode-Konfig enthalten ist.

Vorgehen:

1. Wenn `OPENCODE_CONFIG_FILE` existiert: lesen.
2. Wenn kein `skills.paths` existiert oder `PLAYBOOK_REPO` nicht enthalten ist: Änderung vorschlagen.
3. Wenn die Datei nicht existiert: fragen, ob sie angelegt werden soll.

Vorgeschlagene Minimal-Konfig, falls neu:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["<PLAYBOOK_REPO>"]
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
ln -sfn <PLAYBOOK_REPO>/commands ~/.claude/commands
```

Nicht automatisch ausführen, außer der User fragt ausdrücklich danach. Dieser Command ist primär für OpenCode.

---

## Schritt 6 — Security-Tool-Preflight

Am Ende host-lokal pruefen, ob die Security-Review-Tools vorhanden sind. Dieser Schritt installiert nichts automatisch und schreibt keine Projektdateien.

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
Repo:        <PLAYBOOK_REPO>
Commands:    <n> Symlinks ok / <m> neu oder aktualisiert
Skills:      ok | fehlt | manuell nötig
SecTools:    ok | <n> Pflicht-Tools fehlen | Preflight fehlt
Verwaist:    <n> alte Links

Wichtig: OpenCode neu starten, damit neue Commands und Skills sichtbar werden.
```

---

## Fehlerfälle

- **Repo nicht gefunden**: Pfad erfragen oder abbrechen.
- **Keine Commands gefunden**: abbrechen — wahrscheinlich falscher Repo-Pfad.
- **OpenCode-Konfig nicht sicher editierbar**: nicht überschreiben; stattdessen konkrete manuelle Änderung ausgeben.
- **Symlink-Ziel existiert nicht**: Link nicht anlegen; Fehler in Zusammenfassung melden.
