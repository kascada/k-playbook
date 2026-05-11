---
description: Zugriff auf Basisakte-Daten und -Dienste über den akte MCP-Server. Trigger bei allen Fragen zu Basisakte: Anrufstatistiken, Kunden, Rufnummern, Umsätze, Agenten, Abrechnungen, Google Groups, Benutzerverwaltung, Systemkonfiguration. Wenn dieser Skill erwähnt wird, bezieht sich die nächste Frage auf den MCP-Server mcp.akte.de.
allowed-tools: [mcp__akte-sql__mysql_query, mcp__akte-api__get_groups, mcp__akte-api__get_groups_members, mcp__akte-api__add_member, mcp__akte-api__remove_member, mcp__akte-api__create_group, mcp__akte-api__set_description, Read]
---

# akte MCP-Server

Zugriff auf Basisakte-Geschäftsdaten und -Dienste über `mcp.akte.de`.

## Was ist Basisakte?

Über 30 Jahre gewachsenes C++-System (`basisakte/src`). Läuft als Webserver + Daemon + CLI
in einem Binary auf `grey.akte.de` / `phoenix.akte.de`. Kerngeschäft: Telefonie-Dienstleister —
Kunden mieten Servicenummern (0900er, 0800er etc.) und schalten eigene Berater dahinter.
Pro Gespräch zahlt der Anrufer einen Minutenpreis, der Kunde bekommt seinen Anteil gutgeschrieben.

---

## Dienst 1: Datenbankabfragen (`akte-sql`)

Endpunkt: `https://mcp.akte.de/mysql/mcp` — nur SELECT, kein Schreiben.  
Tool: `mcp__akte-sql__mysql_query`

| Datenbank | Inhalt |
|-----------|--------|
| `voice` | Telefonie: Anrufe, Rufnummern, Routing, Agenten, Abrechnung |
| `kosten` | Buchhaltung: Konten, Rechnungen, Posten, Tarife |
| `adr` | Adressbuch: Kunden, Rechnungsempfänger, Kontakte |
| `akte` | Benutzer- und Gruppenverwaltung des Systems |
| `plus` | CMS, Produktdefinitionen, Konfiguration |
| `dienste` | Zugriffssteuerung nach Service-ID (Legacy) |
| `Relations` | Generische Graph-DB: typisierte Verknüpfungen (Legacy) |

### Schlüssel-Entitäten

- **KNr** — Kundennummer (7-stellig), zentrales Bindeglied überall
- **SNr** — Servicenummer (z.B. `0900123456`), in `voice.TelNr` definiert
- **LineName** — Leitungskennung (z.B. `a01`), verbindet SNr ↔ Kunde via `voice.TelLine.KNr`
- **VGespr** — Wichtigste Tabelle, ein Eintrag pro Anruf. **Immer `Monat` (`YYYY-MM`) filtern!**
- **MitID** — Agent/Berater-ID in `voice.mitarbeiter`

### Schlüssel-Joins

```sql
-- Gespräch → Kundennummer
FROM voice.VGespr v JOIN voice.TelLine tl ON v.LHaupt = tl.LineName
-- tl.KNr = Kundennummer

-- Gespräch → Agent
FROM voice.VGespr v LEFT JOIN voice.mitarbeiter m ON v.MitID = m.MitID

-- Kostenstelle → Kundenname
FROM kosten.Kostenstelle k JOIN adr.Elemente e ON k.RechAID = e.AID AND e.Typ = 'zusammen'
-- e.Info = Anzeigename

-- Rechnungsposten → Kundenname
FROM kosten.Posten p
JOIN kosten.Kostenstelle k ON p.KNr = k.KNr
JOIN adr.Elemente e ON k.RechAID = e.AID AND e.Typ = 'zusammen'
```

### Typische Abfragen

```sql
-- Anrufe einer Rufnummer über die Zeit
SELECT Monat, COUNT(*) AS Anrufe, ROUND(SUM(SecTerm)/60.0,1) AS Minuten
FROM voice.VGespr WHERE SNr = '0900XXXXXXX' AND Monat BETWEEN '2026-01' AND '2026-05'
GROUP BY Monat ORDER BY Monat;

-- Neukunden
SELECT Monat, COUNT(*) AS Erstanrufe FROM voice.VGespr
WHERE SNr = '0900XXXXXXX' AND ErsterAnruf = 'erster' AND Monat BETWEEN '2026-01' AND '2026-05'
GROUP BY Monat ORDER BY Monat;

-- Umsatz pro Kunde
SELECT p.KNr, e.Info AS Kunde, ROUND(SUM(p.Euro),2) AS Euro
FROM kosten.Posten p
JOIN kosten.Kostenstelle k ON p.KNr = k.KNr
JOIN adr.Elemente e ON k.RechAID = e.AID AND e.Typ = 'zusammen'
WHERE p.Zeitraum = '2026-04' AND p.Status != 'del'
GROUP BY p.KNr, e.Info ORDER BY Euro DESC;

-- Live-Gespräche
SELECT GesprID, ZielNr, Agent, StartTime, TIMESTAMPDIFF(SECOND, StartTime, NOW()) AS Sek
FROM voice.Sprechend ORDER BY StartTime;
```

### Abfrageregeln

- **Immer `Monat` filtern** bei `VGespr` — sehr große Tabelle, `Monat` ist der Index
- `SHOW TABLES` zeigt Tabellen der Default-DB; `SHOW TABLES FROM <db>` geht nicht (Parser) → für andere DBs: `SELECT table_name FROM information_schema.tables WHERE table_schema = 'voice'`
- `Hide = 0` für datenschutzkonforme Auswertungen
- `Abgleich = 'ok'` für finale Abrechnungsdaten
- `kosten.Posten`: immer `Status != 'del'`
- `SecTerm` = Verbindungszeit mit Agent · `SecGes` = Gesamtzeit inkl. Wartezeit
- `SysTime` ist Unix-Timestamp → `FROM_UNIXTIME(SysTime)`

Vollständige Schema-Referenz: `/home/kleist/dev/MCP/akte/akte-db/references/schema.md`

---

## Dienst 2: Akte REST-API (`akte-api`)

Endpunkt: `https://mcp.akte.de/akte/mcp` — Lesend und schreibend (je nach Token).

Alles was nicht direkt aus der Datenbank kommt: Systemlogik, Konfiguration, Benutzerverwaltung,
Google Groups, Systemdokumentation — kurz: die gesamte Geschäftslogik des akte-Servers.
Die Tools werden automatisch aus der OpenAPI-Spec geladen; neue Endpunkte erscheinen ohne
Codeänderung.

Aktuell verfügbare Tools:
- `mcp__akte-api__get_groups` → `GET /groups` — Google Groups auflisten
- `mcp__akte-api__get_groups_members` → `GET /groups/members` — Groups mit Mitgliedern

Schreibende Endpunkte (`write: true` im Token erforderlich):
- `POST /groups/add-member` — Mitglied/Owner hinzufügen
- `POST /groups/remove-member` — Mitglied entfernen
- `POST /groups/create` — Neue Gruppe erstellen
- `POST /groups/set-description` — Beschreibung setzen
- `GET /docs/{path}` — Systemdokumentation abrufen
