---
description: "Durchsucht alte AI-Verläufe. Claude: volle Chat-JSONL-Suche. OpenCode: Log-/Session-Suche in opencode.log. Unterstützt Provider claude|opencode|all, Suchbegriff und optionalen Zeitraum."
argument-hint: "[claude|opencode|all] <Suchbegriff> [heute | gestern | YYYY-MM-DD | YYYY-MM-DD..YYYY-MM-DD] [-all]"
# model: github-copilot/gpt-5.5
allowed-tools: [Bash, Read, Glob]
---

# k-verlauf

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Durchsucht alte AI-Verläufe nach einem Begriff.

Provider:

- `claude` — sucht echte Claude-Chatverläufe in `~/.claude/projects/**/*.jsonl`.
- `opencode` — sucht OpenCode-Session-/Log-Metadaten in `~/.local/share/opencode/log/opencode.log`.
- `all` — sucht in allen verfügbaren Providern.

Wichtig: OpenCode speichert auf diesem System nicht dieselben leicht lesbaren Chat-JSONL-Dateien wie Claude. Der OpenCode-Modus ist deshalb eine Log-/Session-Suche, keine vollständige Chattext-Suche.

## Schritt 1 — Argumente parsen

Aus den Argumenten extrahieren:

- `PROVIDER` = optional erstes Token `claude`, `opencode` oder `all`.
- `SEARCH` = der Suchbegriff (alles außer Provider, Scope-Flags und optionalem Datum).
- `DATE_FILTER` = optionaler Zeitraum.
- `SCOPE_ALL` = true, wenn `-all` oder „alle" übergeben wurde.

Wenn kein Provider angegeben wurde:

- Wenn nur Claude-Daten vorhanden sind: `PROVIDER=claude`.
- Wenn nur OpenCode-Logs vorhanden sind: `PROVIDER=opencode`.
- Wenn beide vorhanden sind: fragen: "Wo soll ich suchen? Claude, OpenCode oder beide?"

Erlaubte Zeitraum-Formate:

- `heute` → nur Einträge von heute
- `gestern` → nur Einträge von gestern
- `YYYY-MM-DD` → nur Einträge dieses Datums
- `YYYY-MM-DD..YYYY-MM-DD` → Einträge in diesem Bereich

Wenn `SEARCH` leer ist: nach dem Suchbegriff fragen.

## Schritt 2 — Verfügbare Provider erkennen

Claude ist verfügbar, wenn `~/.claude/projects/` existiert.

OpenCode ist verfügbar, wenn mindestens eine dieser Dateien existiert:

- `~/.local/share/opencode/log/opencode.log`
- `$XDG_DATA_HOME/opencode/log/opencode.log`

Für OpenCode niemals `auth.json` ausgeben oder durchsuchen. Die SQLite-DB nur verwenden, wenn später eine Message-/Session-Tabelle eindeutig identifiziert wurde; aktuell ist der robuste Modus die Logdatei.

## Schritt 3 — Claude-Scope bestimmen

Nur wenn `PROVIDER` `claude` oder `all` enthält.

Wenn `SCOPE_ALL=true`: in allen Claude-Projekten suchen.

Sonst frage den Nutzer:

> Soll ich nur im aktuellen Projekt suchen (`<aktuelles Projekt>`) oder in allen Claude-Projekten?

Zeige dabei den Namen des aktuellen Projekts (abgeleitet aus `$CWD`: Pfad → alle `/` durch `-` ersetzen, führendes `-` behalten). Beispiel: CWD `/home/user/dev/Aiva/kascada` → Projektordner `-home-user-dev-Aiva-kascada`.

## Schritt 4 — Claude-Dateien finden

Nur wenn `PROVIDER` `claude` oder `all` enthält.

```bash
# Projektpfad bestimmen
PROJECT_DIR="$HOME/.claude/projects/<abgeleiteter-name>"
# oder bei -all:
PROJECT_DIR="$HOME/.claude/projects"

# Dateien auflisten, optional nach Datum filtern
find "$PROJECT_DIR" -name "*.jsonl" -newer <datum-start> ! -newer <datum-end>
```

Ohne Zeitraum: alle JSONL-Dateien im Scope.

Wenn der Claude-Scope nicht existiert: diesen Provider mit Hinweis überspringen.

## Schritt 5 — Claude suchen und aufbereiten

Nur wenn Claude-Dateien gefunden wurden.

Benutze `rg` (ripgrep), um den Suchbegriff in den gefundenen Dateien zu suchen:

```bash
rg -il "SEARCH" <dateien>
```

Für jede Datei mit Treffern:

1. **Datum** aus dem ersten JSONL-Eintrag lesen.
2. **Projekt** aus dem Verzeichnisnamen ableiten.
3. **Snippets mit Zeitstempel** aus JSONL-Zeilen mit `text` extrahieren.

Ausgabe-Format pro Claude-Treffer:

```text
Claude: <session-id>  (Session-Start: <datum>)
Projekt: <projekt-pfad>
--
[2026-05-20 09:53:13] » "<snippet 1>"
[2026-05-20 09:54:01] » "<snippet 2>"
```

## Schritt 6 — OpenCode-Logs bestimmen

Nur wenn `PROVIDER` `opencode` oder `all` enthält.

Bestimme `OPENCODE_LOG`:

1. Wenn `$XDG_DATA_HOME/opencode/log/opencode.log` existiert: verwenden.
2. Sonst `~/.local/share/opencode/log/opencode.log` verwenden, falls vorhanden.
3. Sonst OpenCode-Provider mit Hinweis überspringen.

OpenCode-Scope:

- Wenn `SCOPE_ALL=true`: alle Logeinträge durchsuchen.
- Sonst auf das aktuelle Projekt filtern, wenn möglich:
  - `CURRENT_DIR = realpath($CWD)`
  - Suche bevorzugt Logzeilen mit `directory=<CURRENT_DIR>`.
  - Zusätzlich Treffer mit derselben `session.id` aufnehmen, wenn die Session vorher für dieses Directory erstellt wurde.
- Wenn kein Directory-Bezug gefunden wird: frage, ob in allen OpenCode-Logs gesucht werden soll.

## Schritt 7 — OpenCode suchen und aufbereiten

Nur wenn `OPENCODE_LOG` existiert.

OpenCode durchsucht Logzeilen, nicht komplette Chattexte. Sinnvolle Suchbegriffe sind z. B.:

- Projektpfade oder Verzeichnisnamen
- `session.id`, `messageID`, `run`
- Fehlermeldungen
- Tool-/MCP-Namen
- Model- oder Provider-Namen

Suche case-insensitive:

```bash
rg -i "SEARCH" "$OPENCODE_LOG"
```

Wenn `DATE_FILTER` gesetzt ist, filtere zusätzlich über den Prefix `timestamp=YYYY-MM-DD...`.

Für die Ausgabe pro Treffergruppe:

- `timestamp`
- `run`
- `session.id`, falls vorhanden
- `directory`, falls vorhanden
- `message` oder relevante Log-Zeile, gekürzt auf ca. 220 Zeichen

Gruppiere, wenn möglich, nach `session.id`; sonst nach `run`.

Ausgabe-Format:

```text
OpenCode: <session.id oder run>
Directory: <directory oder ?>
Zeitraum: <erstes timestamp> .. <letztes timestamp>
--
[2026-07-06 09:43:07] created session ... title="..."
[2026-07-06 09:43:51] asking id=... questions=1
[2026-07-06 09:44:14] replied requestID=...
```

Begrenze die Ausgabe auf die ersten 10 Gruppen und maximal 5 Zeilen pro Gruppe. Wenn mehr Treffer vorhanden sind, am Ende sagen, wie viele weggelassen wurden und wie man enger suchen kann.

## Schritt 8 — Zusammenfassung

Abschließend ausgeben:

```text
Gefunden für "<SEARCH>":
- Claude: <N> Konversation(en)
- OpenCode: <M> Loggruppe(n)
```

Wenn nichts gefunden wurde:

```text
Keine Treffer für "<SEARCH>".
```

Wenn OpenCode durchsucht wurde, immer den Hinweis ergänzen:

```text
Hinweis: OpenCode-Modus durchsucht Log-/Session-Metadaten, nicht zwingend vollständige Chattexte.
```
