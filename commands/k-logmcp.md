---
description: "Stellt den Zugriff auf Logdateien eines Servers über LogMCP her. Fragt nach dem Server wenn nicht angegeben, prüft Zugriff, lädt verfügbare Logs und merkt sich Server und Freigaben im Projekt."
argument-hint: "[<server-name-oder-hostname>] — z.B. \"switchbox-dev\" oder \"logmcp-switchbox-dev\""
# model: github-copilot/gpt-5.4-mini
allowed-tools: [Bash, Read, Write, Edit]
---

# k-logmcp

Richtet den Zugriff auf einen LogMCP-Server ein und stellt die verfügbaren Logs bereit.

## Konzept

LogMCP läuft auf verschiedenen Linux-Servern. Jede Instanz ist in Claude Code als MCP-Server
unter dem Namen `logmcp-<hostname>` registriert. Die zugehörigen Tools heißen:
`mcp__logmcp-<hostname>__list_logs`, `__read_log`, `__search_log`, `__log_info`.

---

## Schritt 1 — Server bestimmen

Prüfe der Reihe nach:

**a) Aus `.claude/CLAUDE.md` des Projekts**
Lies `.claude/CLAUDE.md` (falls vorhanden). Steht dort ein Abschnitt `## LogMCP` mit einem
`Server:`-Eintrag? → Diesen Servernamen verwenden, direkt zu Schritt 3.

**b) Aus dem Argument**
Wurde ein Argument übergeben (z.B. `switchbox-dev` oder `logmcp-switchbox-dev`)?
→ Vollständigen Namen ableiten: wenn nicht mit `logmcp-` beginnend, `logmcp-` voranstellen.

**c) Interaktiv ermitteln**
```bash
claude mcp list 2>/dev/null | grep -i "logmcp-"
```
- Genau ein Treffer und Status `Connected`: direkt verwenden.
- Mehrere Treffer: User fragen, welchen er meint.
- Kein Treffer: Fehlermeldung ausgeben:
  > Kein LogMCP-Server registriert. Auf dem Zielserver ausführen:
  > `logmcp client-config claude-code`

---

## Schritt 2 — Permissions einrichten (einmalig)

Prüfe `.claude/settings.local.json` (Read). Enthält `permissions.allow` bereits einen Eintrag
für diesen Server oder das Muster `mcp__logmcp-*`?

Falls nein: füge das Wildcard-Muster in `.claude/settings.local.json` ein (mit Read + Write,
JSON korrekt zusammenführen — `jq` per Bash nutzen falls verfügbar):

```json
"mcp__logmcp-*"
```

Dieses Muster deckt alle Tools aller LogMCP-Server ab — immer dieses Muster verwenden,
keine serverspezifischen Einzeleinträge.

---

## Schritt 3 — Zugriff testen und Logs laden

Rufe `mcp__<server>__list_logs` auf.

**Fehlschlag:**
> Zugriff auf `<server>` fehlgeschlagen.
> Falls du gerade Permissions eingetragen hast: **Claude Code neu starten**, damit die
> Freigaben wirksam werden. Danach `/k-logmcp` erneut aufrufen.

**Erfolg:**
Die Liste der verfügbaren Log-Dateien ausgeben. Diese Liste im weiteren Verlauf der Session
als Kontext behalten, damit du weißt, was auf dem Server liegt — ohne bei jeder Frage
erneut `list_logs` aufzurufen.

---

## Schritt 4 — Im Projekt merken

Falls `.claude/CLAUDE.md` noch keinen `## LogMCP`-Abschnitt enthält, füge ihn an:

```markdown
## LogMCP

Server: `logmcp-<hostname>`
Tools: `mcp__logmcp-<hostname>__list_logs` / `read_log` / `search_log` / `log_info`
```

---

## Schritt 5 — Abschlussmeldung

Ausgeben:
- Welcher Server verwendet wird
- Anzahl verfügbarer Log-Dateien (aus `list_logs`)
- Falls Permissions neu eingetragen: "Freigaben in `.claude/settings.local.json` gespeichert."
- Falls Permissions neu und `list_logs` direkt erfolgreich: kein Neustart nötig.
- Falls `list_logs` fehlgeschlagen: Neustart-Hinweis (siehe Schritt 3).
