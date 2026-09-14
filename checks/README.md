# Checks

`<playbook.dir>/bin/k-check` führt die effektive Check-Menge aus: mitgelieferte Checks aus
`<playbook.dir>/checks/` kombiniert mit projekteigenen aus `<local.dir>/checks/`.
Es gilt dieselbe Overlay-Regel wie für Regeln und Reviews:
ein gleichnamiger lokaler Check ersetzt den mitgelieferten, eine leere lokale Datei
schaltet mitgelieferte Checks ab.

Jeder Check wird mit seiner Herkunft ausgewiesen: `dist`, `local` oder `override`.

Der Runner trennt zwei Roots:

- `--config-root`: das k-playbook-Verzeichnis mit `K-PLAYBOOK.yaml`, also `<projekt>/k-playbook`.
- `--target-root`: Codebaum, der gescannt wird. Default ist `project.repo_root` aus der Config, normalerweise das Projektverzeichnis über dem k-playbook-Verzeichnis.

## Aufruf

```bash
<playbook.dir>/bin/k-check --mode changed
<playbook.dir>/bin/k-check --mode baseline
<playbook.dir>/bin/k-check --config-root /path/to/project --mode changed
<playbook.dir>/bin/k-check --config-root /path/to/project --target-root app --mode baseline
<playbook.dir>/bin/k-check --config-root /path/to/project --target-root app --mode baseline --output /path/to/k-playbook-local/results/k-check/YYYY-MM-DD/raw/k-check-baseline.txt --metadata-output /path/to/k-playbook-local/results/k-check/YYYY-MM-DD/run-metadata.json
```

Der Config-Root ist standardmäßig das aktuelle Arbeitsverzeichnis; dort liegt die `K-PLAYBOOK.yaml`. Lokale Checks liegen fest unter `k-playbook-local/checks/`. Der Target-Root ergibt sich aus `project.repo_root`. Nested Repos werden nicht automatisch als neuer Config-Root interpretiert.

`--output <file>` schreibt die vollständige Runner-Ausgabe zusätzlich zu stdout/stderr in eine Raw-Datei. `--metadata-output <file>` schreibt Run-Metadaten als JSON, inklusive Kommando, Exit-Code, Arbeitsverzeichnis, Datum/Zeit, Roots, Modus, Check-Konfiguration und k-check-Version/Git-Commit soweit verfügbar. Beide Optionen verweigern vorhandene Ziel-Dateien, damit auditierbare Artefakte nicht still überschrieben werden. Für Review-Läufe gehören diese Artefakte unter `<reviews>/results/k-check/YYYY-MM-DD/`.

## Check-Schnittstelle

Die stabile Runner-Schnittstelle ist eine `.sh`-Datei direkt im Check-Verzeichnis. Python, Ruby oder andere Implementierungen dürfen dahinter liegen, bleiben aber Implementierungsdetail des jeweiligen `.sh`-Wrappers.

Jeder Check muss genau eine Statuszeile auf stdout schreiben:

```text
K_CHECK_STATUS=ok
K_CHECK_STATUS=skip
K_CHECK_STATUS=fail
```

Optional darf eine Begründung folgen:

```text
K_CHECK_REASON=<text>
```

Exit-Konvention:

- `0` plus `K_CHECK_STATUS=ok`: bestanden.
- `0` plus `K_CHECK_STATUS=skip`: bewusst nicht anwendbar.
- `1` plus `K_CHECK_STATUS=fail`: fachliche Findings.
- `2`: technischer Check-Fehler.
- Fehlende, doppelte oder widersprüchliche Statuszeilen wertet der Runner als technischen Fehler.

Checks prüfen ihre Anwendbarkeit selbst: Sprache vorhanden, Tool installiert, relevante Dateien vorhanden, Framework erkennbar. Nicht anwendbare Checks skippen sauber und blockieren nicht.

Checks bekommen diese Umgebungsvariablen:

- `K_CHECK_CONFIG_ROOT` — Root mit `K-PLAYBOOK.yaml`.
- `K_CHECK_PLAYBOOK_DIR` — das Playbook-Verzeichnis, aus dem der Runner stammt
  (`<playbook.dir>`). Ein Check unter `k-playbook-local/checks/` kann es aus seinem
  eigenen Ort nicht ableiten.
- `K_CHECK_TARGET_ROOT` — gescannter Codebaum.
- `K_CHECK_PROJECT_ROOT` — Kompatibilitäts-Alias für `K_CHECK_TARGET_ROOT`.
- `K_CHECK_MODE` — `changed` oder `baseline`.
- `K_CHECK_FILES_FROM` — newline-separierte Dateiliste im Target-Root.

Das ist die vollstaendige Schnittstelle. Ein Check bekommt seine Pfade aus dieser
Umgebung und ruft `k-playbook context` **nicht** auf: er laeuft als Unterprozess des
Runners, der die Pfade bereits aufgeloest hat.

## Modi

`changed` ermittelt die Dateimenge deterministisch in dieser Priorität:

1. `--files-from <file>`.
2. `--base-ref <ref>`.
3. PR-/CI-Base-Ref aus der Umgebung.
4. Git-Merge-Base gegen den Upstream der aktuellen Branch.
5. Uncommitted und staged Änderungen im Target-Root.

Ist der Target-Root kein Git-Repo, scannt der globale Runner keine nested Repos automatisch; die Dateimenge ist dann leer, außer `--files-from` wird explizit gesetzt.

`baseline` inventarisiert den Projektbaum ab Target-Root mit Standard-Excludes wie `.git`, virtuelle Environments, `node_modules`, Build-Artefakte und weiteren üblichen Cache-Verzeichnissen. Nested Git-Worktrees werden dabei nicht automatisch als eigene Projekte betreten.

## Werkzeug-Guards in Commands

`check_command_tool_guards.sh` setzt den Abschnitt „Externe Werkzeuge" aus
`rules/command-authoring.md` durch: Ein Command ruft ein externes Werkzeug nur mit Guard
oder Rückfall auf. Der Check meldet Fundstellen, er repariert nichts.

- **Werkzeugliste** ist allein die Spalte `name` aus `<playbook.dir>/scripts/base-tools.tsv`.
  Eine neue Zeile dort wirkt ohne Änderung am Check. Fehlt die Matrix, endet der Check mit
  `skip` und nennt den Pfad.
- **Umfang** ist immer der ganze effektive Command-Katalog: `<playbook.dir>/commands/`
  zusammen mit `<config-root>/k-playbook-local/commands/`, verglichen über den Pfad ab
  `commands/`; ein gleichnamiger lokaler Command ersetzt den mitgelieferten, ein leerer
  schaltet ihn ab. `K_CHECK_MODE` und `K_CHECK_FILES_FROM` spielen keine Rolle — nur über
  den ganzen Katalog lässt sich eine veraltete Ausnahme erkennen. Den Katalog liest der
  Check selbst, ohne `k-playbook context`.
- **Erfasst** werden umzäunte Codeblöcke mit der Sprachkennung `bash`, `sh`, `shell` oder
  `zsh`, dazu `console` mit `$ `-Prompt. Blöcke ohne oder mit anderer Kennung, Fließtext
  und Backticks zählen nicht. Ein Aufruf ist der Programmname am Befehlsanfang: am
  Zeilenanfang, nach `;`, `&&`, `||`, `&` oder `(` und nach `if`, `elif`, `then`, `else`,
  `do`, `while`, `until`, `!` oder `{`. Aufrufe nach einer Pipe, in Befehlsersetzungen,
  in Heredoc-Inhalten und hinter `env`, `sudo`, `xargs` oder `command` liegen bewusst
  außerhalb.
- **Geschützt** ist ein Aufruf nur im then-Zweig eines `if command -v <werkzeug> …; then`
  im selben Codeblock (auch mehrere `command -v`, verbunden mit `&&`) oder durch einen
  Rückfall `<werkzeug> … || …` direkt dahinter. Ein Guard in einem anderen Codeblock, eine
  verneinte Bedingung, ein `else`-Zweig oder ein Verweis auf `baseTools` zählt nicht.
  Unklares wird gemeldet.
- **Nicht auswertbar** ist der Rest eines Blocks hinter einem offenen Konstrukt: einem
  nicht geschlossenen `'`, `"`, Backtick oder `$(`, einem Heredoc ohne Endmarke oder
  einem `<<`, das kein Heredoc ist (etwa `(( x << 2 ))` oder ein Platzhalter
  `<<name>>`). Der Check meldet die Stelle als eigene Fundstelle, statt die Aufrufe
  dahinter still zu übergehen. Sie lässt sich nicht als Ausnahme eintragen; der Block
  wird so umgeschrieben, dass er auswertbar ist.

Bewusst zurückgestellte Fundstellen stehen in `command-tool-guard-exceptions.tsv` neben
dem wirksamen Skript. Der Check liest nur diese eine Datei: ersetzt ein projekteigener
Check den mitgelieferten, gilt ausschließlich die TSV neben ihm unter
`k-playbook-local/checks/`. Matrix und mitgelieferte Commands kommen aus
`K_CHECK_PLAYBOOK_DIR`, das der Runner setzt. Fehlt die Variable, nimmt ein Override
`<config-root>/k-playbook/` an; liegt dort keine Matrix, endet er mit einem technischen
Fehler (Exit 2) statt mit `skip`, damit ein geratener Pfad den Check nicht unbemerkt
abschaltet. Eine projekteigene TSV allein überlagert nichts.

Die TSV hat die Kopfzeile `datei`, `zeile`, `werkzeug`, `begründung`, getrennt durch
Tabulatoren; Zeilen mit `#` sind Kommentare. `datei` ist der Pfad ab `commands/`, `zeile`
die 1-basierte Zeile in dieser Datei. Ein Eintrag nimmt genau den einen Aufruf aus, auf
den alle drei Werte passen. Ein Eintrag ohne passende Fundstelle, mit fehlender Spalte
oder mit einem Werkzeug außerhalb der Matrix ist selbst ein `fail`. Bleiben nur exakt
zugeordnete Ausnahmen, meldet der Check `ok` und nennt ihre Zahl in `K_CHECK_REASON`.

## Global vs. Lokal

Mitgelieferte Checks müssen wiederverwendbar bleiben. Projekteigene Checks gehören nach `k-playbook-local/checks/` und werden per Overlay dazugenommen.

Domain-spezifische Begriffe, Modellnamen und Runtime-Dateien gehören nicht in globale Checks. Solche Regeln bleiben projektlokal; Test-Fixtures dürfen Domain-Begriffe nur als explizite Negativbeispiele markieren.

## Keine Scanner-Result-Familien

`<playbook.dir>/checks/*.sh` ist nicht der Ort für schwere Security-Scanner wie `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft` oder `grype` und auch nicht für GitHub-Alert-Imports wie Dependabot Alerts.

Diese Tools liefern strukturierte Rohdaten und brauchen Review-Bewertung, Deduplizierung, stabile Finding-IDs und Artefakte unter `reviews/results/<family>/YYYY-MM-DD/`. Sie werden über globale Report-Mode Reviews gestartet:

- `/k-review secret-scanning`
- `/k-review dependency-cve`
- `/k-review dependabot-alerts`
- `/k-review iac-container`

Zulässig in `<playbook.dir>/checks/*.sh` sind höchstens leichte Preflight-Heuristiken, z. B. ob relevante Manifestdateien existieren oder ob ein Tool installiert ist. Der eigentliche Scan gehört in die passende Review-Familie.
