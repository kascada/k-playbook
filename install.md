# k-playbook – Installation

Setup-Anleitung für das Einbinden des `k-playbook`-Verzeichnisses in
verschiedene AI-Assistant-Umgebungen.

**Primär:** OpenCode. **Optional:** Claude Code.

---

## Voraussetzungen

- Repo geclont nach `~/dev/k-playbook/`
  ```bash
  gh repo clone kascada/k-playbook ~/dev/k-playbook
  # oder: git clone git@github.com:kascada/k-playbook.git ~/dev/k-playbook
  ```

---

## Setup für OpenCode

### Empfohlen: `/k-install`

Wenn `/k-install` bereits verfügbar ist, diesen Command ausführen:

```text
/k-install
```

Der Command erledigt den OpenCode-Teil host-lokal:

- `commands/k-*.md` nach `~/.config/opencode/command/` symlinken
- `skills.paths` in `~/.config/opencode/opencode.jsonc` prüfen bzw. ergänzen
- verwaiste alte Command-Links melden
- daran erinnern, OpenCode neu zu starten

Nach dem Hinzufügen neuer Dateien unter `commands/k-*.md` auf jedem Server erneut `/k-install` ausführen, damit die neuen Symlinks entstehen.

Die folgenden Schritte zeigen die manuelle Variante und dienen als Referenz.

### 1. Skills registrieren (Playbook-Verzeichnisse)

Die `ks-<name>/`-Ordner mit `SKILL.md` sollen automatisch geladen werden.

Datei `~/.config/opencode/opencode.jsonc` um `skills.paths` erweitern:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

OpenCode scannt beim Start rekursiv nach `**/SKILL.md` in diesem Pfad.

### 2. Slash-Commands registrieren

Damit die `commands/k-*.md` als Slash-Commands (`/k-akte`,
`/k-review-loop`, …) in OpenCode verfügbar sind, alle Dateien in das
OpenCode-Command-Verzeichnis symlinken:

```bash
mkdir -p ~/.config/opencode/command
for f in ~/dev/k-playbook/commands/k-*.md; do
  ln -sf "$f" ~/.config/opencode/command/
done
```

Vorteil Symlinks: Änderungen am Repo wirken sofort, kein erneutes Kopieren.

### 3. OpenCode neu starten

Konfig wird nur beim Start gelesen:

```bash
# Session beenden (Ctrl+C oder /exit) und neu:
opencode
```

### 4. Verifikation

- Nach Neustart: neue Session → `/` eintippen → Autocomplete sollte
  `/k-akte`, `/k-review-loop`, … zeigen
- Zum Testen der Skills: eine Nachricht schreiben, deren Thema zu einem
  Playbook passt (z.B. „Ich habe hier ein Overlay-Projekt, das soll
  analysiert werden") → OpenCode sollte den Skill `ks-overlay-repo-analyse`
  laden

---

## Setup für Claude Code

Claude Code sucht Slash-Commands unter `~/.claude/commands/`. Symlink
darauf:

```bash
ln -sf ~/dev/k-playbook/commands ~/.claude/commands
```

Danach sind die Commands per `/k-<name>` in Claude Code verfügbar.

**Hinweis:** Claude Code kennt aktuell kein direktes Äquivalent zu
OpenCode-Skills (auto-getriggerte SKILL.md). Die Playbook-Ordner
`ks-<name>/` sind in Claude Code manuell zu konsultieren (z.B. per
`Read`-Tool auf die `PLAYBOOK.md`).

---

## Verzeichnisstruktur (Konvention)

```
~/dev/k-playbook/
├── README.md
├── install.md                                  diese Datei
├── Makefile
├── commands/                                    Slash-Commands
│   └── k-<name>.md
└── ks-<name>/                                   Playbook (Skill)
    ├── SKILL.md                                 Frontmatter + Kurzfassung
    ├── PLAYBOOK.md                              Prosa + Checkliste
    └── vorlagen/, scripts/ (optional)
```

## Aufbau einer SKILL.md

```markdown
---
name: ks-<name>
description: Use when ...  (konkrete Trigger-Keywords front-loaded)
---

# Skill: <Name>

(Body in Markdown: Anleitung, Verweis auf PLAYBOOK.md)
```

Regeln:

- `name` = Ordnername = lowercase + hyphens, ≤64 Zeichen
- `description` ist Pflicht (OpenCode entscheidet danach, wann Skill
  relevant ist)

## Verifikations-Checkliste

- [ ] `~/dev/k-playbook/` existiert und ist Git-Repo
- [ ] `~/.config/opencode/opencode.jsonc` hat `skills.paths` mit dem Pfad
- [ ] Symlinks unter `~/.config/opencode/command/` zeigen auf `commands/*.md`
- [ ] (Claude Code) `~/.claude/commands -> ~/dev/k-playbook/commands`
- [ ] OpenCode neu gestartet
- [ ] Autocomplete zeigt `/k-*`-Commands
- [ ] Ein Test-Skill wurde erfolgreich getriggert

---

## Fehlersuche

### Slash-Commands tauchen nicht auf

- Symlinks im richtigen Verzeichnis? (`ls -la ~/.config/opencode/command/`)
- OpenCode wirklich neu gestartet? (Konfig ist nicht hot-reloaded)
- Frontmatter der Command-Datei valide?

### Skills werden nicht automatisch getriggert

- `SKILL.md` im richtigen Ordnernamen? (`name`-Frontmatter muss zu Ordner
  passen)
- `description` konkret genug? Trigger-Keywords passen nicht zum User-Text?
- Skill-Loader-Log prüfen (in OpenCode-Logs)

### Änderungen am Repo greifen nicht

- Bei Symlinks: greifen sofort, kein Neu-Start nötig
- Bei kopierten Dateien: hier arbeiten wir immer mit Symlinks, um genau
  das zu vermeiden
