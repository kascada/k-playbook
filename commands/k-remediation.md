---
description: Arbeitet Audit-Befunde strukturiert ab. Verifiziert jeden Befund gegen den echten Code, kategorisiert ihn, behebt sichere Fälle direkt und erstellt Tasks für komplexe. Nur saubere Lösungen — keine Quick-and-Dirty-Fixes.
argument-hint: <audit-result.md> [weitere Dateien...]
allowed-tools: [Read, Write, Edit, Bash, Glob, Agent, LSP, TodoWrite]
---

# k-remediation

Arbeitet Audit-Befunde aus `$ARGUMENTS` strukturiert ab.

---

## Schritt 1 — Datei bestimmen

Wenn `$ARGUMENTS` angegeben: diese Datei(en) einlesen.

Wenn nicht: fragen:
> "Welche Audit-Befund-Datei soll abgearbeitet werden?"

Die Datei enthält eine Befundliste. Offene Punkte sind mit `☐` markiert (oder haben keine Statusspalte). Alle anderen (✓, ~, ✗) überspringen.

---

## Schritt 2 — Kategorien und Autonomie klären

**Kategorien:**

| Kürzel | Name | Bedeutung |
|--------|------|-----------|
| **S** | Sofort | Klarer Fehler, kleiner gezielter Fix (< 30 min, kein Architektur-Impact, kein Verhaltens-Einfluss für Enduser). Beispiele: falsche Konstante, stilles Ignorieren von Fehlern, veraltete Konfigurationswerte. |
| **T** | Task | Klarer Fehler, aber größerer Eingriff — eigenständige Umsetzung braucht eine Task-Datei. |
| **K** | Klärung | Unklar ob wirklich ein Problem, oder Architekturentscheidung nötig. Erst besprechen. |
| **F** | Feature | Neue Funktionalität, kein Bugfix. Immer erst nachfragen — nie autonom umsetzen. |
| **A** | Akzeptiert | Bekanntes Design, bewusste Entscheidung oder externe Behandlung. Mit Grund dokumentieren und schließen. |
| **X** | Falsch | Befund ist nicht korrekt oder nicht relevant. Dokumentieren und schließen. |

Frage den User:
> "Welche Kategorien darf ich autonom abarbeiten?
> - **S (Sofort)** — direkt fixen
> - **T (Task)** — Task-Datei anlegen
> - **A/X** — dokumentieren und schließen
>
> Oder soll ich jeden Punkt erst vorstellen und du entscheidest?"

Warte auf Antwort. Merke welche Kategorien autonom behandelt werden dürfen (`AUTO_CATEGORIES`).

---

## Schritt 3 — Befunde einlesen und sortieren

Alle offenen Befunde aus der Datei sammeln. Falls die Datei eine Prioritätsspalte enthält: nach Priorität absteigend sortieren (höchste zuerst).

Übersicht ausgeben:
```
Befunde geladen: <N> offen
Autonom: <liste der freigegebenen Kategorien>
```

---

## Schritt 4 — Jeden Befund abarbeiten

Für jeden offenen Befund der Reihe nach:

### 4a — Code lesen und verifizieren

**Immer zuerst den echten Code lesen** — nie auf Basis der Befundbeschreibung allein handeln.

- Datei und Zeile aus dem Befund aufsuchen (LSP wenn verfügbar, sonst rg/Read)
- Prüfen: Ist das Problem real? Ist die Beschreibung korrekt?
- Prüfen: Hat sich der Code seit dem Audit geändert (Problem vielleicht schon behoben)?

Wenn das Problem nicht reproduzierbar oder bereits behoben ist → Kategorie **X**, weiter.

### 4b — Kategorisieren

Kategorie anhand der Definitionen (Schritt 2) bestimmen. Im Zweifel konservativer einordnen (K statt S, T statt S).

**Qualitätsleitlinien für Sofort-Fixes:**
- Kein Quick-and-Dirty. Wenn es keine saubere Lösung gibt, wird aus **S** ein **T**.
- Bei mehreren Lösungsoptionen: die elegantere und sicherere wählen. Beispiel: eine etablierte Library einem selbst geschriebenen Workaround vorziehen.
- Fix muss build- und testbar sein (`go build ./...` bzw. entsprechendes).

### 4c — Handeln

**Kategorie S (Sofort) — in `AUTO_CATEGORIES`:**
1. Fix direkt anwenden
2. Build/Tests prüfen
3. Status in Ergebnisdatei auf `✓ behoben` setzen
4. Im Änderungslog (Schritt 5) eintragen

**Kategorie S — NICHT in `AUTO_CATEGORIES`:**
Befund kurz vorstellen (was genau ist falsch, was wäre der Fix), dann fragen:
> "Soll ich das beheben?"

Antwort abwarten, dann entsprechend handeln.

**Kategorie T (Task) — in `AUTO_CATEGORIES`:**
1. Task-Datei anlegen (in `priv/tasks/` oder dem projektüblichen Tasks-Verzeichnis)
   - Nächste freie Nummer bestimmen (tasks/ + tasks/done/ durchsuchen)
   - Datei: `NNN-<kurzname>.md`
   - Inhalt: Intent, Kontext, Zu bauen (kein Quick-and-Dirty, saubere Lösung beschreiben)
2. Status in Ergebnisdatei auf `✓ Task NNN` setzen
3. Im Änderungslog eintragen

**Kategorie T — NICHT in `AUTO_CATEGORIES`:**
Befund vorstellen und fragen ob Task anlegen.

**Kategorie K (Klärung):**
Befund vorstellen:
- Was genau ist das Problem (nach Code-Lektüre)
- Warum unklar / welche Architekturentscheidung steht dahinter
- Mögliche Optionen (ohne eine zu empfehlen)

Auf User-Entscheidung warten. Je nach Entscheidung als S/T/A/X weiterbehandeln.

**Kategorie F (Feature):**
Immer vorstellen und fragen — auch wenn `AUTO_CATEGORIES` alles enthält.
> "Das ist eine Funktionserweiterung. Soll ich dafür einen Task anlegen?"

**Kategorie A (Akzeptiert) — in `AUTO_CATEGORIES`:**
Status auf `~ akzeptiert` setzen. Kurzen Grund in den Änderungslog schreiben.

**Kategorie X (Falsch) — in `AUTO_CATEGORIES`:**
Status auf `✗ falsch` setzen. Kurze Begründung notieren.

---

## Schritt 5 — Ergebnisdatei aktualisieren

Nach jedem bearbeiteten Befund:

**Statusspalte:** Falls die Tabelle noch keine `**Status**`-Spalte hat, diese hinzufügen.

Statuswerte:
| Symbol | Bedeutung |
|--------|-----------|
| `✓ behoben` | Direkt gefixt |
| `✓ Task NNN` | Task-Datei angelegt |
| `~ akzeptiert` | Bekannt/bewusst, kein Handlungsbedarf |
| `✗ falsch` | Befund nicht korrekt |
| `☐` | Noch offen |

**Änderungslog:** Am Ende der Datei einen Abschnitt pflegen (anlegen wenn nicht vorhanden):

```markdown
---

## Änderungslog

| Datum | # | Kategorie | Aktion | Notiz |
|-------|---|-----------|--------|-------|
| YYYY-MM-DD | 12 | Sofort | ✓ behoben | TLS MinVersion 1.2 → 1.3 |
| YYYY-MM-DD | 3  | Task   | ✓ Task 002 | Log-Streaming, priv/tasks/002-log-streaming.md |
| YYYY-MM-DD | 13 | Akzeptiert | ~ | Rate-Limiting extern behandelt (SecurityConfig) |
```

---

## Schritt 6 — Ergebnisdatei archivieren

Wenn alle Befunde abgearbeitet sind (keine ☐ mehr offen):

1. `done/`-Verzeichnis bestimmen:
   - Naheliegend: `done/` im selben Verzeichnis wie die Ergebnisdatei
   - Falls unklar (kein `done/` vorhanden, kein Projektkontext erkennbar): fragen
     > "Wohin soll die abgeschlossene Datei verschoben werden? (Vorschlag: `done/`)"

2. Datei verschieben:
   - Neuer Name: `YYYY-MM-DD-<originalname>` (heutiges Datum voranstellen)
   - Beispiel: `result-tech.md` → `done/2026-05-25-result-tech.md`

3. Kurz bestätigen: "Archiviert: `done/YYYY-MM-DD-<name>`"

Wenn noch offene Befunde (☐) vorhanden: Datei **nicht** verschieben — sie bleibt offen.

---

## Schritt 7 — Abschlusszusammenfassung

Nach allen Befunden ausgeben:

```
Remediation abgeschlossen
─────────────────────────────────────
Bearbeitet:   <N>
✓ behoben:    <n>
✓ Tasks:      <n>  (<Task-Nummern>)
~ akzeptiert: <n>
✗ falsch:     <n>
☐ offen:      <n>  (K/F — warten auf Klärung)
Archiviert:   done/YYYY-MM-DD-<name>  (oder: — offen, nicht archiviert)
```

Wenn noch offene K- oder F-Punkte vorhanden: diese auflisten mit kurzer Begründung warum sie offen blieben.
