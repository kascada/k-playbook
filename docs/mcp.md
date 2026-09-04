# Der MCP-Server

k-playbook bringt einen MCP-Server mit. Er gibt einem Assistenten dieselbe Auskunft, die
`k-playbook context` auf der Kommandozeile liefert: die aufgelösten Pfade, die
Instruktionsdateien in Lesereihenfolge, die Remediation-Policy, die Guidelines und die
effektiven Kataloge für Regeln, Reviews und Checks — mitgeliefert und projekteigen bereits
zusammengeführt.

`k_playbook_context` ist dabei das einzige Werkzeug, das nebenbei schreibt: Es zieht die
Assistenten-Verlinkung des Projekts auf den Katalog nach, so wie das Subkommando es auch
tut. Geschrieben wird nur, wenn Schreiben etwas ändert, und nur an den Symlinks, die
k-playbook selbst angelegt hat. Was dabei passiert ist, steht unter `links` in der
Antwort; fehlt das Feld, stand alles bereits.

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
| OpenCode | `opencode.json`, oder `opencode.jsonc`, wenn nur die existiert | `mcp` → `k-playbook` |

Registriert wird immer dasselbe Kommando, nur in zwei Schreibweisen — der beim
Schreiben aufgelöste absolute Pfad des installierten `k-playbook`:

```json
{
  "mcpServers": {
    "k-playbook": {
      "command": "/home/wer/.local/bin/k-playbook",
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
      "command": ["/home/wer/.local/bin/k-playbook", "mcp"],
      "enabled": true
    }
  }
}
```

Bei OpenCode ist `command` ein **Array** aus Kommando und Argumenten, nicht zwei Felder.

Die drei Dateien gehören dem Projekt und können fremde Einträge tragen; die
OpenCode-Konfiguration trägt daneben ganz andere Einstellungen. Angefasst wird deshalb
genau der Schlüssel `k-playbook`, alles andere bleibt unberührt — **Kommentare und
Trailing Commas eingeschlossen**. Gelesen wird im JWCC-Format (JSON mit Kommas und
Kommentaren), und in eine vorhandene Datei wird nicht ihr Inhalt zurückgeschrieben,
sondern nur dieser eine Schlüssel gepatcht. Fremde Einträge, ihre Reihenfolge und die
Kommentare stehen danach unverändert da.

Eine Nebenwirkung bleibt sichtbar: nach dem Patch wird die Datei einmal **neu
eingerückt**, mit Tabs. Bei einer eingecheckten Konfiguration mit Leerzeichen-Einrückung
ist das ein Diff über die ganze Datei. Das passiert einmal, beim ersten Einrichten —
steht der Eintrag danach richtig, wird gar nicht mehr geschrieben.

**Zwei Endungen, eine Datei.** OpenCode liest `opencode.json` und `opencode.jsonc` und
führt beide tief zusammen, wenn sie nebeneinander liegen. Deshalb legt das Einrichten nie
eine zweite an: `opencode.jsonc` ist das Ziel, wenn sie existiert und `opencode.json`
fehlt — sonst `opencode.json`. Liegen wirklich beide vor, wird nur `opencode.json`
gepflegt, und die Oberfläche meldet „zwei Konfigurationen" statt „eingetragen": was beim
Zusammenführen gewinnt, ist von außen nicht zu sehen, und eine der beiden gehört
aufgelöst.

Entsteht die OpenCode-Konfiguration dabei **neu**, wird der Schema-Verweis
`"$schema": "https://opencode.ai/config.json"` gleich mitgeschrieben. OpenCode trägt ihn
sonst beim nächsten Start selbst nach und schreibt die Datei dafür zurück; mit dem
Eintrag bleibt sie so liegen, wie das Einrichten sie hinterlassen hat. In eine bereits
vorhandene Datei wird er **nicht** nachgetragen — die gehört dem Projekt.

Der Schlüssel `k-playbook` gehört k-playbook. Steht dort ein fremdes Kommando, ist das
kein Konflikt, sondern ein falscher Stand: die Oberfläche meldet „zeigt woandershin" und
überschreibt ihn beim Einrichten. Was als richtiger Stand gilt, ist dabei nicht ein
einzelner Wert, sondern eine Menge von Formen — siehe
[Warum der Eintrag ein absoluter Pfad ist](#warum-der-eintrag-ein-absoluter-pfad-ist).
Nur eine Datei, die sich auch als JWCC nicht lesen lässt — eine fehlende Klammer, ein
Fragment —, wird gemeldet und **nicht** angefasst. Kommentare allein sind kein solcher
Fall.

Die geschriebene Registrierung ist ein absoluter Pfad und damit an eine Umgebung
gebunden; einchecken lässt sie sich so nicht. Wer sie teilen will, trägt von Hand die
[portable Form](#die-portable-form-der-bloße-kommandoname) ein — den bloßen
Kommandonamen. Was dort steht, wird von der automatischen Korrektur nicht angefasst,
solange es zur Menge der akzeptierten Formen gehört.

## Freigabe und Neustart

Geschrieben ist nicht dasselbe wie verfügbar. Der Assistent liest seine Konfiguration
beim Start — nach dem Einrichten ist er also einmal neu zu starten.

Claude Code verlangt zusätzlich eine **Freigabe**: projektbezogene Server aus einer
`.mcp.json` gelten erst nach ausdrücklicher Zustimmung in einer interaktiven Sitzung. Die
Frage kommt einmal beim nächsten Start. Seit v2.1.196 kommt Workspace-Trust dazu; ein
frisch geklontes Projekt kann seine eigenen Server nicht selbst freigeben.

## Warum der Eintrag ein absoluter Pfad ist

Eingetragen wird der **beim Schreiben aufgelöste absolute Pfad** des installierten
Binaries, typischerweise `~/.local/bin/k-playbook`, ausgeschrieben. Nicht der bloße
Kommandoname und nicht mehr der projekteigene Wrapper.

Der Grund ist der Fall, in dem der Eintrag gebraucht wird. Ein Client, der aus Dock oder
Finder gestartet wird — Cursor, VS Code, Claude Desktop —, erbt die PATH einer
Login-Shell **nicht**; `~/.local/bin` steht dort typischerweise nicht drin. Ein bloßes
`k-playbook` wäre in genau diesen Umgebungen tot, während derselbe Eintrag aus einem
Terminal heraus funktioniert. Ein absoluter Pfad hängt von keiner geerbten Umgebung ab.

### Eine Menge akzeptierter Formen, nicht ein Sollwert

Geschrieben wird immer genau eine Form. **Geprüft** wird gegen eine Menge:

| Form | Gilt als |
|---|---|
| jeder absolute Pfad, dessen Dateiname `k-playbook` ist | aktuell |
| derselbe Pfad aus einem anderen `$HOME` | aktuell |
| der bloße Kommandoname `k-playbook` — die portable Form | aktuell |
| `k-playbook/bin/k-playbook`, `bin/k-playbook` und jeder Pfad, der darauf endet | veraltet |
| alles andere, auch `./k-playbook` | falscher Stand, wird gemeldet |

Ein Vergleich auf Gleichheit mit einem einzigen Sollwert erklärte jede andere gültige
Schreibweise für falsch und schriebe sie bei jedem Lauf um. Zwei Fälle machen das
konkret: eine eingecheckte Registrierung kann keinen absoluten Home-Pfad tragen, und
Host und DevContainer haben verschiedene HOMEs.

„Veraltet" ist deshalb **eng** definiert: der alte Wrapper-Pfad und sonst nichts. Nur er
wird von selbst überschrieben. Ein Eintrag, der weder in die Menge passt noch der alte
Wrapper ist, wird gemeldet und erst beim ausdrücklichen Klick auf *Einrichten*
überschrieben.

### Die portable Form: der bloße Kommandoname

Ein absoluter Pfad trägt ein `$HOME` und ist damit an eine Umgebung gebunden. Wer seine
Registrierung **einchecken** will, kann ihn nicht verwenden: dieselbe Datei gilt auf dem
Host und im DevContainer, und deren HOMEs sind verschieden. Dafür steht der bloße Name:

```json
{
  "mcpServers": {
    "k-playbook": {
      "command": "k-playbook",
      "args": ["mcp"]
    }
  }
}
```

Er ist die einzige Schreibweise, die gar keine Umgebung nennt. Jede Umgebung löst ihn
über ihre eigene PATH auf ihr eigenes Binary auf — genau das, was der Bootstrap
sicherstellt: `~/.local/bin` **muss** im PATH liegen, sonst bricht `bin/install` ab. Aus
demselben Grund spielen Host und Container an dieser Datei kein Ping-Pong: keiner von
beiden findet darin etwas zu korrigieren.

Geschrieben wird sie nie. *Einrichten* trägt weiterhin den aufgelösten absoluten Pfad
ein; die portable Form kommt von Hand in die Datei und bleibt dann liegen. Akzeptiert
ist genau der Name — `./k-playbook` und jeder Pfad, der auf den Namen endet, sind es
nicht.

### Drei Grenzen, ausdrücklich

**Die portable Form deckt den Dock-/Finder-Fall nicht ab.** Das ist der Preis, und er
wird hier genannt statt verschwiegen: ein Client, der aus Dock oder Finder gestartet
wird — Cursor, VS Code, Claude Desktop —, erbt die Login-Shell-PATH nicht. Unter einem
bloßen Namen findet er nichts, und der Server bleibt tot. Genau dieser Fall ist der
Grund, warum *Einrichten* den absoluten Pfad schreibt.

Wer eine eingecheckte Registrierung teilt, teilt damit eine Form, die aus dem Terminal
heraus und in einem DevContainer funktioniert, aus dem Dock heraus nicht. Der Weg dorthin
ist derselbe wie bei getrennten HOMEs: in dieser Umgebung einmal auf *Einrichten*
klicken. Das schreibt den absoluten Pfad in die Datei — in einem Repo, das sie trackt,
ist das ein Diff, der nicht committet gehört.

Auch der Selbsttest auf `/mcp` deckt die portable Form nicht ab: er startet, was
*Einrichten* schreiben würde — den aufgelösten absoluten Pfad —, nicht das, was in der
Datei steht. Er beantwortet also „antwortet das installierte Binary", nicht „findet der
Client den Eintrag".


**Ein absoluter Pfad aus einem fremden `$HOME` gilt als aktuell.** Das ist eine
Entscheidung, keine Nachlässigkeit: sonst erklärten Host und DevContainer sich in
derselben Datei wechselseitig für veraltet und schrieben sie bei jedem Wechsel um. Der
Preis: teilen sich Host und Container dasselbe Repository, haben aber getrennte HOMEs,
bleibt MCP in der jeweils anderen Umgebung tot, ohne dass die automatische Korrektur
greift. Anschlagen tut dann nur der Selbsttest auf `/mcp`. Wer in dieser Lage arbeitet,
richtet in jeder Umgebung einmal ausdrücklich ein.

**Der Server findet das Projekt über sein Arbeitsverzeichnis.** Der eingetragene Pfad
sagt, welches Binary startet — nicht, welches Projekt gemeint ist. Der Server löst das
zur Laufzeit über die Aufwärtssuche nach `K-PLAYBOOK.yaml` auf, ab seinem
Arbeitsverzeichnis. Das ist das des Assistenten. Wer den Assistenten in einem
Unterverzeichnis öffnet — etwa im Code-Repository, das nach
[`installation.md`](./installation.md#1-konfiguration-anlegen) neben dem Playbook liegen
darf —, bekommt einen Server, der sein Projekt nicht findet. Die Oberfläche sagt die
Bedingung im Block und weist deutlich darauf hin, wenn schon sie selbst nicht im
Hauptverzeichnis gestartet wurde.

## Bestandsprojekte ziehen von selbst nach

Ein Projekt, dessen Registrierung noch den abgelösten Wrapper nennt, wird ohne
Handgriff korrigiert — an zwei Stellen:

- beim **Clone-Update** über die Oberfläche. Der `git pull` allein erreicht die Dateien
  nicht: sie liegen im Hauptverzeichnis, nicht in `k-playbook/`.
- bei **jedem Start** von `k-playbook`. Das ist der Auffangweg für alles, was daran
  vorbeigeht — ein `git pull` von Hand oder `make -C k-playbook installer-update`.

Ein Klick auf *Einrichten* ist dafür nicht nötig; er bleibt daneben der ausdrückliche
Schreibweg.

Diese Korrektur ist **eng und idempotent**. Geschrieben wird nur bei einem vorhandenen
Eintrag, der im engen Sinn veraltet ist. Eine fehlende Datei wird nicht angelegt, ein
fehlender Eintrag nicht ergänzt, und keine akzeptierte Form wird angefasst — sonst
machte jeder Start die eingecheckten MCP-Dateien eines Projekts dreckig. Ersetzt wird
ein Konfigurationseintrag; gelöscht wird nichts.

Voraussetzung bleibt der einmalige Bootstrap je Host oder DevContainer —
`make -C k-playbook install`, ohne make `k-playbook/bin/install`: beide Korrekturwege
laufen im installierten Binary. Ohne installiertes `k-playbook` gibt es keinen Pfad, der
sich eintragen ließe — dann wird gar nichts geschrieben und die Oberfläche sagt es.

## Werkzeuge

| Werkzeug | Zweck |
|---|---|
| `k_playbook_context` | aufgelösten Arbeitsstand wie `k-playbook context` zurückgeben und dabei die Assistenten-Verlinkung nachziehen |
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

Dazu die Codes der Evidence-Betriebsart, alle aus `k_playbook_review_write_ai_entry`:
`entry_job_invalid` (ein Job an einer Perspektive oder an einer Meldung, die nicht
`done` ist), `entry_result_invalid` (`result` an einem Evidence-Eintrag, der keine
Ergebnisdatei hat), `sarif_required` (`done` ohne Job), `sarif_path_invalid` (Pfad
außerhalb von `raw/`, Datei fehlt oder ist leer) und `recipe_contract_invalid` (das
Rezept erfüllt den Evidence-Vertrag nicht mehr, die Rule-ID-Liste ist damit nicht
prüfbar). Sie alle sind **Fehler des Aufrufs**: das Werkzeug schreibt nichts, und der
Rezeptlauf muss nicht wiederholt werden. Ein ungültiges **Artefakt** — unlesbares SARIF,
falscher `tool.driver.name`, fremde Rule-ID — ist dagegen kein Fehlercode, sondern ein
geschriebener Entry-Status `failed` mit Grund und `stateOverridden: true` in der Antwort.

### Auswahlvalidierung

`k_playbook_review_status` im Modus `available` und `k_playbook_review_create` benutzen
dieselbe Auswahlbasis. Kandidaten enthalten mindestens `name`, `kind`, `title`,
`selectable`, `defaultSelected` und `unavailableReason`.

Tool-Kandidaten kommen aus der Tool-Matrix und dem Preflight, gefiltert nach
`project.languages`. Nicht installierte oder sprachlich unpassende Tools bleiben sichtbar,
sind aber nicht `defaultSelected`. AI-Kandidaten kommen aus dem effektiven Review-Katalog;
abgeschaltete lokale Review-Dateien und Rezepte mit `audit.enabled: false` fehlen. Ein
Rezept mit widersprüchlichem `audit`-Block — etwa `mode: evidence` ohne `ruleIds` oder mit
`scope.tools` — wird nicht zurechtgebogen: es steht unter `unavailableCandidates` mit
`selectable: false` und einem `unavailableReason`, der die verletzte Regel nennt.

AI-Kandidaten tragen zusätzlich `mode`. Die Auswahlbasis stellt daneben
`evidenceCandidates` und `perspectiveCandidates`, im bestehenden Lauf `evidenceEntries`
und `perspectiveEntries` — die Reihenfolge des Laufs soll aus der Ausgabe ablesbar sein,
ohne dass ein Command sie aus den Rezepten neu ableitet.

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
  mode: perspective
  title: "Secret-Scanning Assessment"
  resultRequired: true
  defaultResult: "review-secret-scanning.md"
  scope:
    tools: [gitleaks, trufflehog]
review:
  enabled: true
---
```

`audit.mode` entscheidet, welche der übrigen Felder gelten. Ohne Angabe ist es
`perspective`. Für `mode: evidence` treten `ruleIds` und `scope.paths` an die Stelle von
`scope.tools`, `resultRequired` und `defaultResult`:

```yaml
---
audit:
  enabled: true
  mode: evidence
  title: "Tech-Debt-Analyse"
  ruleIds: [tech-swallowed-error, tech-duplicated-logic]
  scope:
    paths: ["**/*.go", "**/*.py"]
review:
  enabled: true
---
```

Die MCP-Werkzeuge heißen weiter `k_playbook_review_*`, weil sie die technische
Review-Katalog-/Run-API bilden. Die sichtbare Nutzerrolle des vollständigen Sweeps ist
trotzdem `/k-audit`.

Beim Anlegen eines Laufs werden `recipeKey`, `recipePath`, `recipeOrigin`, `title`,
`mode`, `resultRequired`, `defaultResult` und `scope` in `run.json` kopiert. Das
Schreibwerkzeug für AI-Einträge validiert später gegen diese Kopie, nicht gegen den
eventuell geänderten Rezepttext. `scope.tools` und `scope.paths` sind damit ein Snapshot:
bestehende Läufe behalten ihren Scope, auch wenn das Rezept später geändert wird. `mode`
wird ausgeschrieben, auch für `perspective`, damit der Lauf aus sich heraus sagt, wie ein
Eintrag gemeint war; ein leeres Feld in einem Altlauf liest sich als `perspective`.

Für `mode: evidence` wird `resultRequired` hart auf `false` gesetzt — Pflichtartefakt ist
`raw/<entry>.sarif`, und mit der Vorgabe `true` meldete `k_playbook_review_status` jeden
erfolgreichen Evidence-Eintrag als `resultMissing` und `inconsistent`.

`ruleIds` ist die eine Ausnahme von der Snapshot-Regel: die Liste wird **nicht** kopiert,
sondern beim Melden frisch aus dem Rezept gelesen. Sie ist der Vertrag des Rezepts und
keine Festlegung des Laufs — wer sie ändert, ändert sie für alle Läufe, und ein Rezept,
das seinen Evidence-Vertrag inzwischen nicht mehr erfüllt, soll beim Melden auffallen
statt mit einer halben Prüfung durchzugehen.

`k_playbook_review_write_ai_entry` nimmt für einen Evidence-Eintrag zusätzlich `job` mit
`sarif` (relativ zum Laufverzeichnis, unterhalb von `raw/`) und optional `started` und
`finished`. Zustand, Fundzahl und Job-Name entstehen beim Melden. Die Antwort trägt unter
`evidence` die Zahl der übernommenen Funde, die außerhalb des Scopes verworfenen samt
ihrer ersten Pfade und ob das SARIF bereinigt zurückgeschrieben wurde.

### Timeouts und Progress-Notifications

`k_playbook_review_scan` läuft in echten Läufen mehrere Minuten. Der Server sendet
während der Ausführung MCP-Progress-Notifications an den aufrufenden Client, sofern
dessen `CallTool`-Aufruf ein `progressToken` mitbringt. Ohne Token bleibt der Server
still; die finale Antwort ist in beiden Fällen dieselbe.

Ein Progress-Event wird bei jeder Zustandsänderung eines Jobs oder Eintrags gesendet
und zusätzlich als Heartbeat spätestens alle 15 Sekunden, damit auch ein einzelner
lange laufender Scanner nachweislich noch am Leben ist. Ein Debounce von einer
Sekunde verhindert, dass ein schneller Scanner mehrere Events auslöst.

Was die MCP-Spezifikation garantiert: das Standardformat `notifications/progress` mit
`progressToken`, `progress`, `total` und optionaler `message`.

Was die MCP-Spezifikation **nicht** garantiert: dass ein Client sein Request-Timeout
beim Empfang einer solchen Notification zurücksetzt. Das ist Client-Verhalten und muss
je Ziel-Client verifiziert werden.

Für **OpenCode** ist das Verhalten am 2026-08-22 mit dem in dieser Fassung mitgelieferten
Server verifiziert: ein Scan-Lauf von 158 Sekunden lief in einem OpenCode-Client mit
`mcp.k-playbook.timeout: 90000` (90 Sekunden) vollständig durch, ohne dass der Tool-Call
abgebrochen wurde; kein Scanner persistierte `reason: "abgebrochen"`. Der Nachweis stützt
sich auf die persistierten Entry-Zustände (`k-playbook-local/results/<lauf>/entries/*.json`).

Der Wert gehört in die Client-Konfiguration und wird nicht von k-playbook geschrieben.
Die eingecheckte `opencode.json` dieses Repos setzt ihn **nicht**; sie trägt nur die
Registrierung selbst. Ein von Hand ergänztes `timeout` überlebt die automatische
Korrektur allerdings — `mcpEntryCommand()` bewertet nur Kommando und Argumente und lässt
zusätzliche Schlüssel stehen.

Für Clients mit `progressToken`-Support und geprüftem Timeout-Reset ist ein moderater
Anfrage-Timeout (60–90 Sekunden) sinnvoll: er begrenzt einen wirklich hängenden Server
auf eine spürbare Wartezeit, ohne einen normalen langen Lauf zu killen.

Für Clients **ohne** `progressToken` gibt es keine Alive-Garantie. Der Server sendet
ihnen keine Notifications, ihr Timeout schlägt ohne Vorwarnung. Empfehlungen:

- höherer Anfrage-Timeout (fünf bis zehn Minuten je nach Scanner-Zoo), oder
- CLI-Weg (`k-playbook scan <lauf>`), der von Client-Timeouts entkoppelt ist, oder
- eine spätere Background-Scan-/Polling-Lösung.

Ein expliziter Client-Cancel nach Scan-Start bricht die Scanner weiterhin hart ab;
der zuletzt laufende Scanner persistiert `reason: "abgebrochen"`. Progress-Notifications
ändern daran nichts — sie zielen ausschließlich auf den Timeout-Fehlermodus, nicht auf
Cancellation.

## Nachsehen, was der Server anbietet

Die Seite **/mcp** — im Block über *Zustand und Werkzeuge* erreichbar — zeigt zweierlei:

- den Registrierungszustand je Assistent, ausführlicher als im Block,
- die tatsächlich angebotenen Werkzeuge. Sie stehen nicht in einer Liste im Code: die
  Oberfläche startet den registrierten Befehl als eigenen Prozess, schickt ihm
  `initialize` und `tools/list` und zeigt, was zurückkommt. Damit beantwortet dieselbe
  Anzeige auch die Frage, ob der Server überhaupt läuft.

Gestartet wird dabei genau das, was auch der Assistent startet: derselbe absolute Pfad,
mit dem Hauptverzeichnis als Arbeitsverzeichnis und **ohne die geerbte Shell-PATH**. Das
letzte ist kein Detail. Liefe der Selbsttest mit der PATH der Shell, in der die
Oberfläche gestartet wurde, meldete er grün, während der aus Dock oder Finder gestartete
Client scheitert — er misst dann eine Umgebung, die der Client gar nicht hat. Gesetzt
wird stattdessen die minimale System-PATH, die ein GUI-Programm bekommt.

Antwortet der Server nicht, ist gar keines installiert oder kommt kein verwertbares JSON
zurück, ist das ein Ergebnis der Seite und keine Störung: sie meldet „Server antwortet
nicht" samt Grund und bleibt bedienbar.

## Von Hand

Der Server lässt sich auch ohne Oberfläche starten:

```bash
k-playbook mcp
```

Er spricht dann JSON-RPC über stdin und stdout und wartet auf einen Client.

**Entfernen geht nur von Hand.** Die Oberfläche richtet ein, sie räumt nicht ab. Wer den
Server loswerden will, löscht den Eintrag `k-playbook` aus `.mcp.json`,
`.cursor/mcp.json` und `opencode.json` beziehungsweise `opencode.jsonc` — und startet den
Assistenten neu.

## In diesem Repo

Das Quell-Repo von k-playbook ist zugleich sein eigenes Zielprojekt. Von den drei Dateien
ist nur eine getrackt:

- `.mcp.json` und `.cursor/mcp.json` stehen in `.gitignore`.
- `opencode.json` ist getrackt und lässt sich nicht teilweise ignorieren; ihr
  `mcp`-Block steht deshalb im Repository.

Alle drei tragen die **portable Form** — den bloßen Kommandonamen `k-playbook`. Der
geschriebene absolute Pfad ist an ein `$HOME` gebunden und hier nicht verwendbar; die
beiden ignorierten Dateien führen ihn nur mit, damit auf diesem Rechner überall dasselbe
steht. Die automatische Korrektur fasst keine davon an: die portable Form gehört zur
Menge der akzeptierten, und geschrieben wird von selbst nur der alte Wrapper-Pfad.

Damit bleibt der Arbeitsbaum bei jedem Clone-Update und jedem Start sauber. Wer hier auf
*Einrichten* klickt, bekommt dagegen den absoluten Pfad in `opencode.json` — ein Diff in
einer getrackten Datei, der zurückzunehmen ist.
