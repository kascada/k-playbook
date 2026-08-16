# Der MCP-Server

k-playbook bringt einen MCP-Server mit. Er gibt einem Assistenten dieselbe Auskunft, die
`k-playbook context` auf der Kommandozeile liefert: die aufgelösten Pfade, die
Instruktionsdateien in Lesereihenfolge, die Remediation-Policy, die Guidelines und die
effektiven Kataloge für Regeln, Reviews und Checks — mitgeliefert und projekteigen bereits
zusammengeführt.

Ein Werkzeug, `k_playbook_context`, mit einem optionalen Parameter `dir`. Mehr tut der
Server nicht: er führt keine Reviews aus, legt nichts an und verändert nichts am Projekt.
Das machen Commands und Skills im Assistenten.

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
