---
description: Durchsucht alte Claude-Chat-Verläufe (JSONL-Dateien in ~/.claude/projects/). Übergib einen Suchbegriff und optional einen Zeitraum. Sucht im aktuellen Projekt oder auf Nachfrage in allen Projekten.
argument-hint: <Suchbegriff> [Zeitraum z.B. heute | gestern | 2026-05-20 | 2026-05-20..2026-05-21]
allowed-tools: [Bash, Read]
---

# k-verlauf

Durchsucht alte Claude-Konversationen nach einem Begriff.

## Schritt 1 — Argumente parsen

Aus den Argumenten extrahieren:
- `SEARCH` = der Suchbegriff (alles vor einem optionalen Datum)
- `DATE_FILTER` = optionaler Zeitraum, falls vorhanden

Erlaubte Zeitraum-Formate:
- `heute` → nur Dateien von heute
- `gestern` → nur Dateien von gestern
- `YYYY-MM-DD` → nur Dateien dieses Datums
- `YYYY-MM-DD..YYYY-MM-DD` → Dateien in diesem Bereich

## Schritt 2 — Scope bestimmen

Frage den Nutzer:

> Soll ich nur im aktuellen Projekt suchen (`<aktuelles Projekt>`) oder in allen Projekten?

Zeige dabei den Namen des aktuellen Projekts (abgeleitet aus `$CWD`: Pfad → alle `/` durch `-` ersetzen, führendes `-` behalten). Beispiel: CWD `/home/kleist/dev/Aiva/kascada` → Projektordner `-home-kleist-dev-Aiva-kascada`.

Wenn der Nutzer schon `-all` oder „alle" als Argument übergeben hat, überspring die Frage und such in allen Projekten.

## Schritt 3 — Dateien finden

```bash
# Projektpfad bestimmen
PROJECT_DIR="$HOME/.claude/projects/<abgeleiteter-name>"
# oder bei -all:
PROJECT_DIR="$HOME/.claude/projects"

# Dateien auflisten, optional nach Datum filtern
find "$PROJECT_DIR" -name "*.jsonl" -newer <datum-start> ! -newer <datum-end>
```

Ohne Zeitraum: alle JSONL-Dateien im Scope.

## Schritt 4 — Suchen

Benutze `rg` (ripgrep) um den Suchbegriff in den gefundenen Dateien zu suchen:

```bash
rg -l "SEARCH" <dateien>
```

Wenn keine Treffer: „Keine Treffer für `SEARCH`." — fertig.

## Schritt 5 — Ergebnisse aufbereiten

Für jede Datei mit Treffern:

1. **Datum** aus dem ersten `queue-operation`-Eintrag lesen:
   ```bash
   head -c 300 <datei> | python3 -c "import json,sys; d=json.loads(sys.stdin.readline()); print(d.get('timestamp','?')[:10])"
   ```

2. **Projekt** aus dem Verzeichnisnamen ableiten (den Ordnernamen unter `~/.claude/projects/` zurückumwandeln: `-` → `/`, führendes `/home/kleist` ergänzen).

3. **Snippets mit Zeitstempel** — Zeilen mit dem Suchbegriff, die `"text":` enthalten, per Python aufbereiten:
   ```python
   import sys, json, re
   for line in sys.stdin:
       # Zeitstempel direkt aus dem JSON-Eintrag lesen
       try:
           entry = json.loads(line)
           ts = entry.get('timestamp', '')[:19].replace('T', ' ')
       except Exception:
           ts = ''
       m = re.search(r'"text"\s*:\s*"(.*?)"', line)
       if m:
           txt = m.group(1)[:120].replace('\\n', ' ').replace('\\t', ' ')
           print(f"  [{ts}] » {txt}")
   ```
   Aufruf:
   ```bash
   rg -i "SEARCH" <datei> | python3 -c "<obiges Skript>" | head -5
   ```

4. **Ausgabe-Format** pro Treffer:

```
📄 <session-id>  (Session-Start: <datum>)
   Projekt: <projekt-pfad>
   ──
   [2026-05-20 09:53:13] » "<snippet 1>"
   [2026-05-20 09:54:01] » "<snippet 2>"
   ...
```

## Schritt 6 — Zusammenfassung

Abschließend ausgeben:
```
<N> Konversation(en) gefunden für "<SEARCH>"
```
