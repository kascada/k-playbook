# Der MCP-Server

k-playbook bringt einen MCP-Server mit. Er gibt einem Assistenten dieselbe Auskunft, die
`k-playbook context` auf der Kommandozeile liefert: die aufgelösten Pfade, die
Instruktionsdateien in Lesereihenfolge, die Remediation-Policy, die Guidelines und die
effektiven Kataloge für Regeln, Reviews und Checks — mitgeliefert und projekteigen bereits
zusammengeführt.

Der Server bietet den Arbeitsstand und die Review-Lauf-Werkzeuge an. `k_playbook_context`
hat den optionalen Parameter `dir`; alle Review-Werkzeuge verlangen `projectDir`, weil ein
stdio-Server nicht annehmen darf, dass sein Prozess-Arbeitsverzeichnis das Zielprojekt ist.

Die Innensicht — Protokollfassungen, stdout-Regel, Sitzungsende — steht in
[`../installer/docs/architecture.md`](../installer/docs/architecture.md#der-mcp-server).

## Registrieren

In der Oberfläche steht dafür der Block **k-playbook-MCP**. Ein Klick auf *Einrichten*
trägt den Server in allen drei Dateien ein:

| Assistent | Datei | Eintrag |
|---|---|---|
| Claude Code | `.mcp.json` im Hauptverzeichnis | `mcpServers` → `k-playbook` |
| Cursor | `.cursor/mcp.json` | dasselbe Schema, derselbe Schlüssel |
| OpenCode | `opencode.json` | `mcp` → `k-playbook` |

Registriert wird immer dasselbe Kommando, nur in zwei Schreibweisen:

```json
{
  "mcpServers": {
    "k-playbook": {
      "command": "k-playbook/bin/k-playbook",
      "args": ["mcp"]
    }
  }
}
```

```json
{
  "mcp": {
    "k-playbook": {
      "type": "local",
      "command": ["k-playbook/bin/k-playbook", "mcp"],
      "enabled": true
    }
  }
}
```

Bei OpenCode ist `command` ein **Array** aus Kommando und Argumenten, nicht zwei Felder.

Die drei Dateien gehören dem Projekt und können fremde Einträge tragen; `opencode.json`
trägt daneben ganz andere Einstellungen. Angefasst wird deshalb genau der Schlüssel
`k-playbook`, alles andere bleibt inhaltlich unberührt. Nicht erhalten bleibt die
**Formatierung**: gelesen und geschrieben wird als JSON, und ein solcher Round-Trip
sortiert die Schlüssel alphabetisch und setzt die Einrückung auf zwei Leerzeichen. Wer
eine handformatierte Datei hat, sieht sie danach normalisiert.

Der Schlüssel `k-playbook` gehört k-playbook. Steht dort etwas anderes — ein absoluter
Pfad, ein fremdes Kommando —, ist das kein Konflikt, sondern ein falscher Stand: die
Oberfläche meldet „zeigt woandershin" und überschreibt ihn beim Einrichten. Nur eine
Datei, die sich nicht als JSON lesen lässt, wird gemeldet und **nicht** angefasst.

In einem Zielprojekt gehören alle drei Dateien ins Repository, damit das Team denselben
Server bekommt.

## Freigabe und Neustart

Geschrieben ist nicht dasselbe wie verfügbar. Der Assistent liest seine Konfiguration
beim Start — nach dem Einrichten ist er also einmal neu zu starten.

Claude Code verlangt zusätzlich eine **Freigabe**: projektbezogene Server aus einer
`.mcp.json` gelten erst nach ausdrücklicher Zustimmung in einer interaktiven Sitzung. Die
Frage kommt einmal beim nächsten Start. Seit v2.1.196 kommt Workspace-Trust dazu; ein
frisch geklontes Projekt kann seine eigenen Server nicht selbst freigeben.

## Warum der Eintrag relativ ist

Eingetragen wird der projekteigene Wrapper `k-playbook/bin/k-playbook`, relativ zum
Hauptverzeichnis — nicht die host-weite Kopie unter `~/.local/bin`. Drei Gründe:

- **DevContainer.** Dort ist `$HOME` ein anderes; `~/.local/bin/k-playbook` gibt es
  nicht. Das Projekt dagegen ist gemountet.
- **Teilbarkeit.** Ein absoluter Pfad wäre auf jedem Rechner ein anderer. Nur ein
  relativer Eintrag lässt sich einchecken und gilt für das ganze Team.
- **Plattformen.** Der Wrapper wählt die Binary selbst über `uname`. Aus einem einzigen
  Eintrag funktionieren damit macOS auf dem Host und Linux im Container.

Der Server selbst ist ortsunabhängig: er löst das Projekt zur Laufzeit über die
Aufwärtssuche nach `K-PLAYBOOK.yaml` auf, nicht über seinen eigenen Ort.

## Werkzeuge

| Werkzeug | Zweck |
|---|---|
| `k_playbook_context` | aufgelösten Arbeitsstand wie `k-playbook context` zurückgeben |
| `k_playbook_review_status` | Auswahlbasis für neue Läufe oder Status eines bestehenden Laufs lesen |
| `k_playbook_review_create` | Review-Lauf anlegen oder im Dry-Run die validierte `run.json` zurückgeben |
| `k_playbook_review_scan` | Tool-Einträge eines Laufs über `review.Execute` ausführen |
| `k_playbook_review_merge` | Lauf über `merge.Run` zu `review-input.json` und `review-input.md` zusammenführen |
| `k_playbook_review_write_ai_entry` | Status und Ergebnis eines AI-Review-Eintrags schreiben |

Ein `k_playbook_review_next_steps`-Werkzeug gibt es bewusst noch nicht. Der orchestrierende
Command liest den Status und entscheidet daraus selbst.

### Review-Response-Vertrag

Alle Review-Werkzeuge liefern dieselbe Hülle. Erfolgreich:

```json
{
  "ok": true,
  "tool": "k_playbook_review_status",
  "project": {
    "inputDir": "/uebergebener/pfad",
    "root": "/projekt",
    "playbookDir": "/projekt/k-playbook",
    "localDir": "/projekt/k-playbook-local",
    "reviewRunsDir": "/projekt/k-playbook-local/results",
    "languages": ["go"]
  },
  "data": {},
  "warnings": []
}
```

Fachlicher Fehler:

```json
{
  "ok": false,
  "tool": "k_playbook_review_status",
  "project": { "inputDir": "/uebergebener/pfad" },
  "error": {
    "code": "project_not_found",
    "message": "Kein k-playbook-Projekt gefunden.",
    "details": {}
  },
  "warnings": []
}
```

Fachliche Fehler bleiben Werkzeugergebnisse mit `ok: false`; der MCP-Server bleibt für den
nächsten Aufruf ansprechbar. MCP-Protokollfehler sind kaputten JSON-RPC-/MCP-Nachrichten
vorbehalten. Fehlercodes sind stabil und in `snake_case`, unter anderem
`project_not_found`, `run_not_found`, `run_exists`, `invalid_mode`, `invalid_selection`,
`selection_unknown`, `selection_unavailable`, `entry_not_found`, `entry_kind_invalid`,
`entry_state_invalid`, `result_required`, `result_path_invalid`, `read_failed`,
`write_failed`, `preflight_failed`, `execution_failed` und `merge_failed`.

### Auswahlvalidierung

`k_playbook_review_status` im Modus `available` und `k_playbook_review_create` benutzen
dieselbe Auswahlbasis. Kandidaten enthalten mindestens `name`, `kind`, `title`,
`selectable`, `defaultSelected` und `unavailableReason`.

Tool-Kandidaten kommen aus der Tool-Matrix und dem Preflight, gefiltert nach
`project.languages`. Nicht installierte oder sprachlich unpassende Tools bleiben sichtbar,
sind aber nicht `defaultSelected`. AI-Kandidaten kommen aus dem effektiven Review-Katalog;
abgeschaltete lokale Review-Dateien und Rezepte mit `audit.enabled: false` fehlen.

Ohne `entries` wählt `k_playbook_review_create` alle Kandidaten mit
`defaultSelected: true`. Unbekannte Namen ergeben `selection_unknown`, ausdrücklich
angeforderte nicht auswählbare Kandidaten `selection_unavailable`, doppelte oder falsch
typisierte Einträge `invalid_selection`.

### Scan-Semantik

`k_playbook_review_scan` shellt nicht die `k-playbook`-CLI. Es ruft die Go-Fachlogik
`review.Execute` direkt auf; nur diese darf die konfigurierten externen Scanner-Binaries
starten. Der MCP-Input enthält keine Shell-Kommandos und kann Scanner-Befehlszeilen nicht
überschreiben.

Ein Scanner-Fehlschlag ist normalerweise ein Entry-Status in `data.entries[]`; der
Werkzeugaufruf bleibt `ok: true`, wenn Lauf, Auswahl und Statusdateien konsistent gelesen
und geschrieben wurden. `ok: false` steht für Orchestrierungsfehler wie fehlende Läufe,
ungültige Auswahl, nicht lesbare Laufdateien, nicht schreibbare Entry-Dateien oder einen
globalen Preflight-Abbruch.

### AI-Rezeptmetadaten

Review-Rezepte können am Dateianfang getrenntes `audit`-/`review`-Frontmatter tragen:

```yaml
---
audit:
  enabled: true
  title: "Technischer Review"
  resultRequired: true
  defaultResult: "review-tech.md"
review:
  enabled: true
---
```

Die MCP-Werkzeuge heißen weiter `k_playbook_review_*`, weil sie die technische
Review-Katalog-/Run-API bilden. Die sichtbare Nutzerrolle des vollständigen Sweeps ist
trotzdem `/k-audit`.

Beim Anlegen eines Laufs werden `recipeKey`, `recipePath`, `recipeOrigin`, `title`,
`resultRequired` und `defaultResult` in `run.json` kopiert. Das Schreibwerkzeug für
AI-Einträge validiert später gegen diese Kopie, nicht gegen den eventuell geänderten
Rezepttext.

### Die Bedingung, die daraus folgt

Ein relativer Eintrag wird vom Assistenten gegen dessen **Arbeitsverzeichnis** aufgelöst,
nicht gegen den Ort der Konfigurationsdatei. Der Eintrag gilt deshalb nur, wenn der
Assistent im Hauptverzeichnis geöffnet ist — dort, wo `K-PLAYBOOK.yaml` liegt.

Das ist eine bekannte Grenze, keine Nachlässigkeit. Wer den Assistenten in einem
Unterverzeichnis öffnet — etwa im Code-Repository, das nach
[`installation.md`](./installation.md#1-konfiguration-anlegen) neben dem Playbook liegen
darf —, bekommt einen Pfad, der ins Leere zeigt. Einen absoluten Fallback gibt es
bewusst nicht: er gäbe genau die Teilbarkeit auf, wegen der der Eintrag relativ ist.

Die Oberfläche sagt die Bedingung im Block und weist deutlich darauf hin, wenn schon sie
selbst nicht im Hauptverzeichnis gestartet wurde.

## Nachsehen, was der Server anbietet

Die Seite **/mcp** — im Block über *Zustand und Werkzeuge* erreichbar — zeigt zweierlei:

- den Registrierungszustand je Assistent, ausführlicher als im Block,
- die tatsächlich angebotenen Werkzeuge. Sie stehen nicht in einer Liste im Code: die
  Oberfläche startet den registrierten Wrapper als eigenen Prozess, schickt ihm
  `initialize` und `tools/list` und zeigt, was zurückkommt. Damit beantwortet dieselbe
  Anzeige auch die Frage, ob der Server überhaupt läuft.

Gestartet wird dabei genau das, was auch der Assistent startet — Wrapper und
Binary-Auswahl inbegriffen. Antwortet er nicht, fehlt der Wrapper oder kommt kein
verwertbares JSON zurück, ist das ein Ergebnis der Seite und keine Störung: sie meldet
„Server antwortet nicht" samt Grund und bleibt bedienbar.

## Von Hand

Der Server lässt sich auch ohne Oberfläche starten:

```bash
k-playbook/bin/k-playbook mcp
```

Er spricht dann JSON-RPC über stdin und stdout und wartet auf einen Client.

**Entfernen geht nur von Hand.** Die Oberfläche richtet ein, sie räumt nicht ab. Wer den
Server loswerden will, löscht den Eintrag `k-playbook` aus `.mcp.json`,
`.cursor/mcp.json` und `opencode.json` — und startet den Assistenten neu.

## In diesem Repo

Das Quell-Repo von k-playbook ist zugleich sein eigenes Zielprojekt, aber die drei
Dateien werden hier **nicht** eingecheckt:

- `.mcp.json` und `.cursor/mcp.json` stehen in `.gitignore`. `/k-playbook/` ist hier
  selbst ignoriert; der eingetragene Wrapper existiert nach einem frischen Clone also gar
  nicht, ein eingecheckter Eintrag zeigte ins Leere.
- `opencode.json` ist getrackt und lässt sich nicht teilweise ignorieren. Der
  `mcp`-Block, den das Einrichten hinzufügt, bleibt hier eine lokale Änderung und wird
  nicht committet.

In einem Zielprojekt gilt das Gegenteil: dort gehören alle drei Dateien ins Repository.
