---
description: Review new Stundenzettel entries for spelling, style, and duplicate billing. Fetches entries via SSH CLI, reads Aiva project docs for context, asks about issues, and applies corrections directly.
allowed-tools: [Bash, Read, Glob]
---

# k-zeit-review

Prüft neue Stundenzettel-Einträge auf Rechtschreibung, Stil und Dopplungen gegenüber bereits abgerechneten Perioden.

## Server-Zugriff

CLI auf dem Server: `ssh kleist@vpn.pluschat.akte.de "/opt/kamran/zeit/zeit <befehl>"`  
DB-Pfad ist bereits als Default im Binary hinterlegt — kein `-db` nötig.

## Step 1 — Einträge holen

```bash
ssh kleist@vpn.pluschat.akte.de "/opt/kamran/zeit/zeit since-lock"
```

Liefert JSON mit:
- `locked_until` — Datum der letzten Abrechnung
- `locked` — alle bereits abgerechneten Einträge (Kontext für Duplikat-Check)
- `new` — neue Einträge seit der letzten Abrechnung (werden geprüft)

Wenn `new` leer ist: Meldung ausgeben und abbrechen.

## Step 2 — Projektkontext lesen

Lies die Dokumentation um zu verstehen, worum es bei den Einträgen inhaltlich geht:

```
~/dev/Aiva/outer-doc/
~/dev/Aiva/realtime-asterisk/docs/
```

Glob nach `.md`-Dateien in beiden Verzeichnissen und lies sie.
Ziel: verstehen welche Komponenten, Begriffe und Arbeiten zum Projekt gehören.

## Step 3 — Einträge prüfen

Gehe jeden Eintrag in `new` durch und prüfe:

### Rechtschreibung & Stil
- Deutsche Rechtschreibung (Einträge sind auf Deutsch)
- Klare, präzise Formulierung
- Konsistente Terminologie mit den Projektdocs

### Duplikate
- Vergleiche jeden neuen Eintrag mit allen `locked`-Einträgen
- Achte auf inhaltliche Ähnlichkeit, nicht nur wortgleiche Übereinstimmung
- Auch innerhalb der `new`-Einträge auf Dopplungen prüfen

## Step 4 — Probleme besprechen und korrigieren

Für jedes gefundene Problem:

1. Zeige den betroffenen Eintrag **immer** mit **Datum und Position** — niemals mit der internen ID. Format: `<position>. Eintrag am <tag>.<monat>.` — Position = 1-basierte Reihenfolge der Einträge des Tages nach `position`-Feld. Auch in Übersichten und Zusammenfassungen ausschließlich dieses Format verwenden, keine IDs.
2. Beschreibe das Problem klar (Rechtschreibfehler / Stilproblem / mögliches Duplikat)
3. Mache einen konkreten Korrekturvorschlag
4. **Warte auf Bestätigung** des Benutzers

Bei Bestätigung: Korrektur direkt einpflegen:

```bash
ssh kleist@vpn.pluschat.akte.de "/opt/kamran/zeit/zeit edit <id> --desc '<neue beschreibung>'"
```

Bei Stundenkorrektur:
```bash
ssh kleist@vpn.pluschat.akte.de "/opt/kamran/zeit/zeit edit <id> --hours <N>"
```

**Achtung Quoting:** Beschreibungen mit Sonderzeichen oder Anführungszeichen über eine temporäre Datei oder mit sorgfältigem Shell-Escaping übergeben.

**Neuen Eintrag anlegen** (solange `create`-CLI fehlt): über die REST-API direkt auf dem Server:
```bash
ssh kleist@vpn.pluschat.akte.de "curl -s -X POST http://localhost:8765/zeit/api/entries \
  -H 'Content-Type: application/json' \
  -d '{\"date\":\"YYYY-MM-DD\",\"description\":\"...\",\"hours\":N}'"
```

## Step 5 — Zusammenfassung

Am Ende ausgeben:
- Anzahl geprüfter Einträge
- Anzahl Korrekturen vorgenommen
- Eventuelle offene Punkte die der Benutzer selbst entscheiden soll
