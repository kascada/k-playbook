# Umbau: projektlokale Installation

Arbeitsdatei für die Dauer der Umstellung. Sie hält fest, was besprochen und festgelegt
ist — nicht, was angedacht wurde. Was umgesetzt ist, steht nicht mehr hier, sondern in der
regulären Doku; wenn nichts mehr offen ist, wird diese Datei gelöscht.

Stand: 2026-08-20, Branch `main`.

## Was schon umgestellt ist

Der Umbau auf das projektlokale Modell ist durch und in der Doku eingearbeitet:

| Thema | Wo es steht |
|---|---|
| Modell, Anker, Verzeichnisaufteilung, Overlay-Regeln, Konfiguration | [`k-playbook-format.md`](./k-playbook-format.md) |
| Clone, die drei Einrichtungsschritte, Aktualisieren, host-weiter Aufruf | [`installation.md`](./installation.md) |
| Grundmodell und Standardabläufe | [`handbuch.md`](./handbuch.md) |
| Commands, `context` einmal je Sitzung | [`commands.md`](./commands.md) |
| Rezepte, Ergebnisse, Remediation-Policy | [`reviews-and-results.md`](./reviews-and-results.md) |
| Entfallenes: `/k-install-security-tools`, `paths.*` | [`faq.md`](./faq.md) |
| Werkzeug: Anker finden, Verlinkung, Update, Spiegelung, Altlasten, Web-API | [`../installer/docs/architecture.md`](../installer/docs/architecture.md) |
| Command-Namen und Review-Handoff: `/k-audit`, `/k-review`, `/k-task-refine`, einheitliches `review-triage.md` | [`commands.md`](./commands.md), [`code-review.md`](./code-review.md), [`review-runs.md`](./review-runs.md) |

## Arbeitsteilung: Entwicklungsrepo vs. Installation

**`~/dev/k-playbook` ist das Entwicklungsrepo — keine Installation.** Hier entstehen und
werden per git bereitgestellt: die Skills, Commands, Checks, Reviews und Regeln, der
Installer und die Doku. Gearbeitet wird am Repo-Stand; die Installation daneben unter
`k-playbook/` ist ein eigener Clone und kann eine andere Fassung tragen.

**Die tatsächliche Installation sieht anders aus.** Referenzprojekt zum Testen und
Anpassen ist `/home/kleist/dev/Aiva/kascada/`. Dort wird jede Umstellung gegen eine
echte, gewachsene Installation geprüft, nicht gegen ein frisch angelegtes
Beispielprojekt.

## Reviews auf Tools und SARIF

Besprochen, noch nicht umgesetzt. Der Abschnitt wächst mit den einzelnen Schritten; was
hier steht, ist festgelegt, alles andere steht am Ende unter „Offen".

**Das Problem.** Jedes Scan-Review startet heute seine Tools selbst — beschrieben in Prosa
im Rezept, ausgeführt vom Assistenten. `review-dependency-cve`, `review-iac-container` und
`review-secret-scanning` tragen je einen eigenen Preflight, eigene Aufrufzeilen und ein
eigenes Findings-Format. Dieselbe Orchestrierung steht mehrfach da, und die Formate laufen
auseinander: die Feldnamen wechseln zwischen `Quelle`, `Tool(s)`, `Package` und `Target`,
sodass `/k-results` beim Lesen die Vereinigung aller Varianten raten muss.

**Das Zielbild: Tools startet das Werkzeug, bewerten tut der Assistent.** Alle Tools
erzeugen SARIF und schreiben es in ein gemeinsames Ergebnisverzeichnis. Ein Merge-Schritt
führt die Dateien zusammen und konsolidiert Dubletten. Erst danach kommt der Assistent —
einmal über das konsolidierte Ergebnis, nicht einmal je Tool. Er bewertet False Positives,
priorisiert, erklärt den Kontext und schlägt Fixes vor.

Die Rohdaten bleiben dabei unangetastet, wie schon heute unter `raw/`. Nur so bleibt
nachvollziehbar, was ein Tool tatsächlich gemeldet hat, und eine spätere Neubewertung ist
möglich.

Der Merge bringt mehr als weniger Doppelarbeit: ein Befund, den zwei Scanner unabhängig
melden, wiegt schwerer als einer aus einer einzelnen Quelle. Diese Information entsteht
überhaupt erst durch das Zusammenführen — solange jedes Review für sich läuft, sieht sie
niemand.

**Bedienung: geführt in der Oberfläche.** Der Ablauf ist eine Abfolge von Schritten, von
denen jeder erst freischaltet, wenn der vorige steht:

1. Die Sprachen des Projekts wählen.
2. Die Tools wählen, nach den Sprachen vorgefiltert.
3. Starten. Die Tools laufen parallel, jedes schreibt seine SARIF-Datei.
4. Die Reviews des Assistenten anstoßen. Die Oberfläche kann das nicht selbst — sie startet
   keine Assistenzsitzung. Sie zeigt deshalb die Commands zum Kopieren; ihre Einstellungen
   holen sich die Reviews aus der `context`-Ausgabe, nicht aus einer eigenen Quelle.
5. Zusammenfassen. Der Schritt ist wiederholbar: wer die Reviews erst danach laufen lässt,
   ruft ihn erneut auf, und ihre Ergebnisse kommen mit hinein.

**Die Tool-Liste.** Zu den vorhandenen `gitleaks`, `trufflehog`, `pip-audit`, `trivy`,
`syft` und `grype` kommen für Python und Go:

| Tool | Sprache | Zweck |
|---|---|---|
| `ruff` | Python | Qualität, dazu das `S`-Regelwerk (flake8-bandit); ersetzt bandit |
| `semgrep` | Python, Go | generische Security-Regeln |
| `gosec` | Go | Go-Security |
| `golangci-lint` | Go | Go-Qualität, bündelt staticcheck und errcheck |
| `govulncheck` | Go | Go-CVEs mit Reachability |
| `osv-scanner` | Python, Go | Dependency-CVEs |

Alle sieben können SARIF von sich aus. Von den vorhandenen können es `gitleaks`, `trivy`
und `grype`; `trufflehog` und `pip-audit` nicht. `syft` erzeugt eine SBOM und damit
überhaupt keine Befunde — es ist der Zulieferer für `grype` und bleibt außerhalb des
Merge.

**Erledigt.**

- Die Tool-Matrix trägt `languages`, `install_method`, `install_ref` und `asset_pattern`.
  `go install` bleibt den Werkzeugen vorbehalten, die Go ohnehin brauchen — ein reines
  Python-Projekt braucht deshalb kein Go.
- Die Sprachen stehen unter `project.languages`, in der `context`-Ausgabe und als Auswahl
  im Security-Tools-Block. Vorauswahl ist `python`.
- Das Laufmodell steht in [`review-runs.md`](./review-runs.md): ein Lauf je Tag,
  `run.json` plus `entries/`, Werkzeuge und Reviews als Einträge desselben Laufs. Die
  Oberfläche legt Läufe an, startet aber nichts.
- Die Ausführung durch das Werkzeug steht ebenfalls dort: `scripts/scanners.tsv` hält je
  Job den Aufruf samt Ausschlüssen, `k-playbook scan <lauf> [eintrag …]` führt die
  Werkzeug-Einträge parallel aus, schreibt SARIF nach `raw/` und den Fortschritt nach
  `entries/<name>.json`. Der Eintrag bleibt dabei das Werkzeug; dass eines aus mehreren
  Jobs besteht, muss beim Zusammenstellen niemand wissen.
- Modulgebundene Jobs lösen ihr Verzeichnis selbst auf: `workdir: module` in
  `scanners.tsv` lässt den Ausführer die Manifeste unter dem Ziel suchen und den Job je
  Modul einmal starten. Damit laufen `govulncheck`, `golangci-lint` und `gosec` auch in
  Projekten, deren `go.mod` nicht in der Wurzel liegt. `gosec` stand dabei zunächst auf
  `target` — nachgemessen prüfte es so nichts (0 statt 154 Befunde) und steht seither
  ebenfalls auf `module`; seine Ausschlüsse für `k-playbook/` entfielen dabei ersatzlos,
  weil die Modulsuche diese Verzeichnisse ohnehin übergeht.

**Gemessen, gehört in die Scan-Jobs.** An `~/dev/Aiva/kascada` (351 Dateien, 59.000 Zeilen)
verglichen:

- **95,3 % aller Python-Befunde sind `S101`/`B101`** — schlichtes `assert`. Ein Job, der
  das nicht ausschließt, verschüttet die übrigen 170 Funde unter 3500. Testverzeichnisse
  ebenso: dort steckt der Großteil der `S105`-Treffer (`{"secret": "read-secret"}`).
- **`ruff` braucht `--select S --isolated`.** Seine Standardauswahl ist `E`/`F`; ohne die
  Angabe findet es keinen einzigen Security-Fund, und mit der Projektkonfiguration fände es
  etwas anderes als beabsichtigt.
- **`bandit` ist entfallen**, ruff deckt es ab: 97,7 % identische Funde, und die Differenz
  waren überwiegend Falschpositive und bereits per `# noqa` abgehakte Fälle, die bandit
  nicht sieht.
- **`semgrep --config auto` ist ausgeschlossen, nicht bloß unerwünscht.** Die Kombination
  mit abgeschalteten Metriken verweigert das Werkzeug selbst: „Cannot create auto config
  when metrics are off. Please allow metrics or run with a specific config." Entweder
  Nutzungsdaten senden oder ein benannter Regelsatz — dazwischen gibt es nichts, und
  Ersteres kommt für ein Werkzeug, das in fremden Projekten läuft, nicht in Frage.
  (`~/.semgrep/settings.yml` trägt dafür eine `anonymous_user_id`.)

  Gemessen mit Semgrep OSS 1.172.0 an je einer Python- und einer Go-Datei:
  `--config p/security-audit --metrics=off` läuft sauber, **ein Job deckt beide Sprachen
  ab** — 79 Regeln bei nur Python, 107 bei Python und Go, weil semgrep die Auswahl nach
  den gefundenen Dateitypen trifft. Getrennte Sprach-Jobs braucht es nicht.

  Die Regeln kommen dabei bei **jedem** Lauf vom Server; `~/.semgrep/` hält keinen
  Regel-Cache. Der Job braucht also Netz, und der Regelsatz kann sich zwischen zwei Läufen
  ändern — träger als bei `auto`, aber nicht ausgeschlossen. Ein eigener Regelsatz aus
  lokalen YAML-Dateien wäre der Ausweg, hieße aber, ihn selbst zu pflegen; dagegen steht,
  dass das SARIF die gelaufenen Regeln vollständig dokumentiert: 225 Einträge unter
  `tool.driver.rules` mit ID, Beschreibung und `helpUri`, dazu die Werkzeugversion. Was in
  einem Lauf galt, bleibt damit aus `raw/` ablesbar — wofür das Verzeichnis auditierbar ist.

**Erledigt: Merge-Werkzeug.** Aus Task 014 (`k-playbook-local/tasks/done/014-sarif-merge-review-input.md`).
Ein eigenes Unterkommando statt einer fremden Runtime: `k-playbook merge <lauf>` liest
`run.json`, `entries/*.json` und `raw/*.sarif`, normalisiert die Findings, dedupliziert
sie und schreibt zwei Artefakte: `review-input.json` als vollständigen Audit-Beleg und
`review-input.md` als kompakte Ansicht. Details in
[`review-runs.md`](./review-runs.md#zusammenfassen-mit-k-playbook-merge). Externe
Kandidaten wurden dabei ausgeschlossen: der Microsoft SARIF Multitool zöge .NET oder
npm nach, und `sarif-tools` (PyPI) deckt Cross-Tool-Deduplizierung und `entries`-Kontext
nicht ab.

Erster Realdurchlauf (2026-08-19, dieses Repo): 347 Findings, 227 Gruppen. Rohdaten
blieben unangetastet. Zweiter Realdurchlauf am selben Tag am Projekt OMNI
(`~/dev/squad-km-dev-setup/k-playbook-local/results/2026-08-19/`): 327 Findings, 141
Gruppen — die Dedupe-Wirkung ist dort deutlich stärker, weil Lockfile-Zeilen viele
CVEs bündeln.

**Erledigt: Merge-Nachbesserungen aus dem Realdurchlauf.** Aus Task 016
(`k-playbook-local/tasks/done/016-merge-nachbesserungen.md`). Sechs Punkte, die aus den
beiden ersten Realläufen kamen, sind sauber im Merge angekommen:

- Same-Location-Bundle innerhalb desselben Entry/Job/Tools zieht mehrere Rule-IDs an
  einer Stelle in eine Gruppe, ohne Belege zu verlieren.
- Zentrale Zuordnung Regel → Schwere in `scripts/severity.tsv`; native und CVSS-Werte
  gehen vor, das Mapping deckt den Rest. Der OMNI-Lauf zeigt es klar: die 167 vorher
  `unknown` gemeldeten Findings sind jetzt einer normalen Schwere zugeordnet.
- `entries[].source` verdoppelt die Entry-Daten nicht mehr; die Rückführbarkeit läuft
  über Entry-/Job-ID, Tool-Name und Rohdatenpfad.
- `kPlaybookVersion` kommt aus `runtime/debug.ReadBuildInfo` (Release-Version,
  Dev-Version, Dirty-Suffix) statt hart `"unknown"`.
- Markdown-Zahlen-Block führt auch `done`-Tools mit 0 Findings.
- Stabile Gruppen-IDs (`stableId`/`stableKey`) mit sprechendem Präfix (`scan-<tool>-…`
  oder `scan-cve-<id>-…`) und deterministischer Kollisionsauflösung. Zwei aufeinander
  folgende Merges desselben Laufs produzieren dieselbe ID-Menge.

Realdurchlauf nach 016: k-playbook-Repo 347/173, OMNI 327/110. Fanout-Stellen aus dem
`_old/`-Baum und `requirements.txt:29` sind sichtbar gebündelt; `unknown` verschwindet.
Rohdaten in beiden Läufen `sha256`-identisch vor und nach dem Merge.

**Erledigt: Soft-Skip aus dem Katalog.** Aus Task 015
(`k-playbook-local/tasks/done/015-scanner-soft-skip.md`). Auslöser war `osv-scanner` im
OMNI-Lauf am 2026-08-19: Exit 128 plus „No package sources found", ohne SARIF — vom
Ausführer als technischer `failed` gewertet, obwohl das Werkzeug selbst gemeldet hatte,
dass es unter dem Bezugspunkt nichts zu prüfen gab. Statt für jeden Scanner einen
Sonderfall in den Ausführer zu schreiben, steht das Signal jetzt im Katalog: eine
Spalte `soft_skip` in `scripts/scanners.tsv` trägt Regeln der Form `<Exit-Code>:<Regex>`,
mehrere durch `;` getrennt. Passen Prozess-Exit-Code und Muster in stderr oder stdout,
führt der Ausführer den Job als `skipped` mit der passenden Zeile als Grund; `failed`
bleibt technischen Fehlern vorbehalten. Vorrang bleibt eindeutig: lesbares SARIF gewinnt
und bleibt `done`, kaputtes nicht leeres SARIF bleibt `failed`, Timeouts und Runner-Abbruch
ebenso. Details in
[`review-runs.md`](./review-runs.md#zustände) und der Kopfzeile von
[`scripts/scanners.tsv`](../scripts/scanners.tsv).

**Erledigt: Alternative 2, Command orchestriert über MCP.** Aus Task 018
(`k-playbook-local/tasks/done/018-review-run-und-triage.md`). `/k-audit` ist jetzt
scharfgeschaltet: Der Command legt Läufe über MCP an oder setzt sie fort, liest vor jedem
Schritt den Status, startet Scanner, führt AI-Review-Einträge, startet den Merge und ruft
danach das Bewertungsmodul `review-scan-triage` auf. Die Auswahlbasis kommt aus
`k_playbook_review_status` im Modus `available`: Werkzeuge, aktive Review-Rezepte und der
Command-Moduleintrag `scan-triage` werden vor `create` bestätigt.

**Erledigt: MCP-Werkzeuge für Review-Läufe.** Aus Task 017
(`k-playbook-local/tasks/done/017-mcp-review-werkzeuge.md`). Der MCP-Server bietet fünf
Werkzeuge mit einheitlicher Response-/Fehlerhülle und verpflichtendem `projectDir`:
`k_playbook_review_status`, `_create`, `_scan`, `_merge` und `_write_ai_entry`. Sie
rufen die bestehende Fachlogik unter `installer/internal/review/*` und
`installer/internal/review/merge/*` direkt auf; kein Shell-Out zur CLI. Details in
[`mcp.md`](./mcp.md).

**Erledigt: Bewertung als Command-Modul statt Katalog-Rezept.** Aus Task 018. Die
Bewertung eines Laufs liegt unter `commands/_audit/review-scan-triage.md`, liest
`review-input.json` und `review-input.md`, berücksichtigt `known-decisions.md` über einen
festen Suchpfad und schreibt `review-triage.md` direkt in den Laufordner. Der AI-Eintrag
`scan-triage` wird über den MCP-Vertrag geführt, obwohl er nicht in `catalogs.reviews`
steht; ein leeres lokales Overlay des Moduls schaltet ihn ab.

**Erledigt: `known-decisions.md` wirkt projektweit.** Aus Task 019
(`k-playbook-local/tasks/done/019-known-decisions.md`). Das Format ist festgelegt: ein
`##`-Eintrag je Decision, genau ein fenced `yaml`-Block mit Pflichtfeldern und danach
Begründung. `k-playbook merge` liest die Datei, markiert gedeckte Findings und Gruppen in
`review-input.json` und zeigt vollständige oder teilweise Deckung in `review-input.md`.
Das Bewertungsmodul matcht nicht mehr selbst, sondern übernimmt `knownDecisions` und
`coveredByKnownDecision` aus dem JSON. Der Realdurchlauf vom 2026-08-19 deckt die 74
`_old/`-Gruppen über `kd-old-tree`; `raw/`, `run.json` und `entries/*.json` blieben per
SHA256 unverändert.

**Nachtrag aus Task 030: die Zwei-Ebenen-Regel ist zurückgenommen.** Task 019 hatte zwei
Suchpfade festgelegt — eine laufspezifische `RUN_DIR/known-decisions.md` vor der
projektweiten Datei, bei gleicher `id` gewinnt die laufspezifische Fassung. Beides ist
entfallen. Die laufspezifische Fassung hatte keinen Erzeuger: kein Command, kein Skill und
kein Oberflächenelement legte sie an, in keinem Lauf dieses Repos hat es sie gegeben. Sie
trägt konzeptionell auch nicht — eine bewusste Entscheidung ist nicht laufgebunden, dafür
gibt es `expires` — und sie hatte einen stillen Nebeneffekt: eine Datei, die niemand
erwartet, konnte eine projektweite Entscheidung vollständig aushebeln.

Zugleich ist die projektweite Datei von `k-playbook-local/results/` eine Ebene hoch nach
`k-playbook-local/known-decisions.md` gezogen. `results/` ist „alles, was Reviews
erzeugen"; diese Datei wird von Hand gepflegt und ist Eingabe, keine Ausgabe. Sie ist
bewusst **kein** Eintrag in `LocalStructure()` geblieben: `CreateLocal()` legt
Datei-Einträge per `writeIfMissing` an, der neue Ort existierte danach immer — und die
Übergangslesung des alten Orts liefe nie mehr an. `/k-gui` legt sie deshalb nicht an, ihr
Zweck steht in [`review-runs.md`](./review-runs.md#wirkung-von-known-decisionsmd). Der alte
Ort wird befristet weiter gelesen und der Umzug sichtbar gemeldet; der Ausbau hängt an
Task 031.

**Erledigt: Namensraum-Konvention für Command-Module.** Zusammen mit Task 018 festgelegt
und in `rules/command-authoring.md` verankert: `commands/_<name>/` trägt Module (kein
Command). `_shared/` bleibt für Module, die alle Commands teilen; `_<command-name>/`
sammelt command-eigene Module (`_audit/`), `_<familie>/` sammelt Module einer
Command-Familie (z. B. `_docs/`, sobald die Docs-Commands gemeinsame Module bekommen).
Overlay funktioniert per Datei-Pfad ab `commands/`; leere Datei schaltet ab. Regel in
[`../rules/command-authoring.md`](../rules/command-authoring.md#ablage).

**Erster Realtriage-Lauf.** 2026-08-19, dieses Repo, `review-triage.md` unter
`k-playbook-local/results/2026-08-19/`. 173 Gruppen wurden zu sechs Bündeln plus einer
Restgruppe verdichtet: B1 Dependency-Upgrade Go (P1/S), B2 Pfadvalidierung (P2/S), B3
Prozessaufrufe (P2/K), B4 Dateirechte und ignorierte Cleanup-Fehler (P2/T), B5 `_old/`
aus dem Scope nehmen (P3/X), B6 Staticcheck-Aufräumen (P3/T). Der Bündel-Schnitt bildet
den Nutzen der ganzen Pipeline zum ersten Mal ab: aus 347 SARIF-Findings entstehen sechs
handhabbare Bewertungseinheiten, jede mit konkretem nächsten Schritt.

## Als Nächstes

Stand 2026-08-20. Diese Punkte sind besprochen, sitzen im aktuellen Ergebnis fest und
werden in dieser Reihenfolge angegangen:

1. **`_old/`-Cleanup.** Bündel B5 zeigt sichtbar: 74 Gruppen aus `_old/internal/*` sind
   archivierter Legacy-Code. Sobald `known-decisions.md` deren Deckung trägt, entscheiden
   wir mit ruhiger Übersicht: löschen, verschieben oder ausschließen. Danach neuer Merge
   und Triage zur Kontrolle.
2. **End-to-End-Test des `/k-audit`-Flows.** Der Command ist scharf, MCP-Werkzeuge
   liegen an, Modul und Merge sind erledigt, `known-decisions.md` wirkt. Was noch fehlt,
   ist der Durchlauf am Stück in einer Chat-Sitzung — neuer Lauf, Auswahl bestätigen,
   Scan, Merge, Triage, `review-triage.md` lesen. Erste Zielprojekte: dieses Repo und
   OMNI. Nach dem Testlauf notieren wir, was am Command, am Modul und an der Auswahl
   auffällt.
3. **Handoff nach der Triage.** Was passiert mit `review-triage.md` nach der Bewertung?
   `/k-remediation` versteht `review-triage.md` als aktuelles Format; historische
   `assessment.md`/`findings.md` bleiben Legacy-Fallback. Offen ist nur noch die konkrete
   Qualität der Task-Erzeugung aus Bündeln im End-to-End-Test.

**Kleiner Aufräumpunkt: Installations-Sync.** Die zwei letzten Merges brauchten einen
`chmod u+w` auf den Installations-Clone, damit `scripts/severity.tsv` (aus Task 016)
verfügbar war. Sobald der aktuelle Stand in den Remote gepusht und die Installationen
über den regulären `git pull`-Weg aktualisiert sind, entfällt der Workaround. Trotzdem
prüfen wir, ob es einen bequemeren Weg gibt, den Installations-Clone gegen den Dev-Stand
zu synchronisieren — heute wäre das ein Handgriff, morgen ein wiederkehrendes Detail.

## Offen, ohne konkreten Termin

Wird einzeln besprochen, bevor daran gearbeitet wird:

- **OpenCode-Usage als MCP-Werkzeug.** OpenCode speichert die Sitzungsnutzung bereits
  lokal in SQLite: `~/.local/share/opencode/opencode.db`. Die Tabelle `session` trägt
  aggregierte Werte je Sitzung: `cost`, `tokens_input`, `tokens_output`,
  `tokens_reasoning`, `tokens_cache_read`, `tokens_cache_write`, dazu `model`, `agent`,
  `directory`, `title`, `time_created` und `time_updated`. Feinere Werte je
  Assistant-Antwort liegen zusätzlich als JSON in `event.data`, vor allem bei
  `message.updated.1`; `session.updated.1` enthält fortlaufende Sitzungs-Snapshots mit
  denselben Summen.

  Für eine manuelle Abfrage reicht OpenCodes eigenes DB-Kommando, z. B.:

  ```bash
  opencode db "select id,title,directory,agent,model,cost,tokens_input,tokens_output,tokens_reasoning,tokens_cache_read,tokens_cache_write,time_updated from session order by time_updated desc limit 10" --format json
  ```

  Ein MCP-Werkzeug dafür darf nicht die ganze Datenbank freigeben. Dieselbe DB und das
  danebenliegende Datenverzeichnis enthalten auch sensible Daten wie Accounts,
  Credentials, `auth.json` und `mcp-auth.json`. Der sichere Zuschnitt wäre deshalb:
  read-only öffnen (`mode=ro`), nur Whitelist-Queries erlauben und nur `session` sowie
  optional `event` lesen. Mögliche Tools: `opencode_usage_recent_sessions(limit,
  directory?)`, `opencode_usage_session(session_id)`, `opencode_usage_daily_totals(days?)`
  und `opencode_usage_message_events(session_id)`. Antworten geben nur Usage-Metadaten
  zurück, nie Prompts, Tool-Ausgaben oder Credential-Tabellen.

  Die Kernabfrage wäre:

  ```sql
  select
    id,
    title,
    directory,
    agent,
    json_extract(model, '$.providerID') as provider,
    json_extract(model, '$.id') as model,
    json_extract(model, '$.variant') as variant,
    cost,
    tokens_input,
    tokens_output,
    tokens_reasoning,
    tokens_cache_read,
    tokens_cache_write,
    datetime(time_created / 1000, 'unixepoch', 'localtime') as created_at,
    datetime(time_updated / 1000, 'unixepoch', 'localtime') as updated_at
  from session
  order by time_updated desc
  limit ?;
  ```

  Für Claude Code gibt es ein ähnliches Nutzsignal, aber anders zugeschnitten. In
  nichtinteraktiven Läufen (`claude --print --output-format json` beziehungsweise
  `stream-json`) liefert der finale Result-Output beziehungsweise die SDK-`ResultMessage`
  aggregierte `usage` und `total_cost_usd`. Die Usage-Felder folgen dem Anthropic-Schema:
  `input_tokens`, `output_tokens`, `cache_read_input_tokens` und
  `cache_creation_input_tokens`. Hooks bekommen außerdem `session_id` und
  `transcript_path`; das Transcript ist JSONL und kann als Beleg gelesen werden. Lokal ist
  aber kein OpenCode-ähnliches, dokumentiertes SQLite-Schema mit Session-Summen zu sehen;
  auf diesem Rechner liegt `~/.claude/sessions/` leer und die CLI ist in der Shell nicht
  eingeloggt. Für Claude Code wäre deshalb eher ein Hook- oder Wrapper-Export sinnvoll:
  Result-JSON bei `--print` speichern oder über Hooks den `transcript_path` erfassen und
  nur Usage-Metadaten extrahieren. Ein MCP-Werkzeug sollte auch dort nicht pauschal
  Transcripts ausliefern, weil darin Prompts, Antworten und Tool-Ergebnisse stehen können.
- **Alternative 1: GUI startet nur die Werkzeug-Scans.** Die Oberfläche startet nach dem
  Anlegen eines Laufs die Werkzeug-Einträge im Hintergrund, fachlich also
  `k-playbook scan <lauf>`. Der Merge läuft danach nicht automatisch. Nach Abschluss der
  Werkzeug-Scans prüft die Oberfläche den Lauf: Gibt es noch `ai`-Einträge auf `start`,
  zeigt sie an, dass diese durch einen Assistenten auszuführen sind, und nennt die dafür
  vorgesehenen nächsten Schritte. Gibt es keine offenen KI-Einträge mehr, oder sollen die
  vorhandenen Tool-Ergebnisse trotzdem verdichtet werden, bietet die Oberfläche einen Knopf
  für `k-playbook merge <lauf>` an. Nach dem Merge zeigt sie die geschriebenen Artefakte
  (`review-input.json`, `review-input.md`) und den nächsten Schritt für die Bewertung durch
  den Assistenten. Während ein Scan läuft, darf der GUI-Server nicht wegen eines verlorenen
  Browserfensters beenden. Fortschritt wird aus `entries/*.json` gelesen; zusätzlich gibt
  der Server grobe Statuszeilen zu Lauf, Tool/Job-Zuständen und Abschluss auf stdout aus.
- Was mit `trufflehog` und `pip-audit` geschieht, die kein SARIF können: umwandeln oder
  ersetzen.
- **`k-check` als MCP-Werkzeug.** Der Runner gibt heute Terminaltext aus; `review-k-check-security`
  sichert ihn als `raw/k-check-<mode>.txt`, und der Assistent liest und deutet ihn. Seine
  Parameter — `--mode changed|baseline`, `--files-from`, `--base-ref`, `--exclude`,
  `--timeout` — stehen dabei als Prosa im Rezept. Ein Werkzeug könnte sie als Schema führen
  und statt der Rohausgabe zurückgeben, welche Checks liefen und welche mit Datei und Zeile
  angeschlagen haben. `--metadata-output` schreibt bereits JSON; die Struktur ist also da,
  sie erreicht den Assistenten nur nicht. Die Rohausgabe bleibt für `raw/` erhalten — sie ist
  auditierbar und wird nicht ersetzt.
- **Der geänderte Stand als MCP-Werkzeug.** Was bei einer Bewertung zählt und was nicht,
  steht heute als Prosa in `commands/k-run.md` („omit generated files, lockfiles, and binary
  files"; bei über ~100 Zeilen zusammenfassen) und fällt damit bei jedem Lauf neu aus.
  Mechanisch daran ist: Bezugspunkt über `git merge-base`, Dateien und Zeilen über
  `git diff --numstat`, binär meldet git selbst, `linguist-generated` und `-diff` stehen in
  `.gitattributes`, Lockfiles erkennt eine gepflegte Namensliste. Das gehört ins Programm —
  nach demselben Grundsatz wie bei den lokalen Einstellungen: gemessen, nicht geraten.
  Beurteilung bleibt beim Assistenten: welche Hunks zählen, was zusammengefasst wird, ob
  eine als generiert markierte Datei doch interessant ist.
  Den **Diff-Text soll das Werkzeug nicht liefern** — mit der Pfadliste holt der Assistent
  ihn gezielt selbst. Sonst bräuchte das Werkzeug Größengrenzen und Kürzungsregeln, und das
  sind wieder Urteile. Die Antwort trägt damit keinen Inhalt, bleibt auch bei 200 geänderten
  Dateien klein und kann nichts still verschlucken.
- **Ein leeres Ergebnis ist nicht von keinem Ergebnis zu unterscheiden.** Der Ausgang eines
  Jobs hängt daran, ob lesbares SARIF vorliegt — bewusst so, weil fast alle Scanner mit
  einem Code ungleich 0 enden, sobald sie etwas gefunden haben. Ein Werkzeug, das gar
  nichts prüfen konnte, schreibt aber dieselbe Datei wie eines, das nichts gefunden hat:
  Exit 0, valides SARIF, leeres `results`. Beide sind `done`.

  Zweimal aufgetreten, in zwei Sprachen: `gitleaks` hätte in einem Projekt namens
  `k-playbook` stumm 0 Befunde gemeldet, weil das Ausschlussmuster ohne Anker die ganze
  Projektwurzel traf (in Task 004 beim Verifizieren gefunden und dort behoben). `gosec`
  meldete aus der Projektwurzel 0 statt 154, weil ihm der Modulkontext fehlte (Task 010).
  Beide Male fiel es nur auf, weil jemand nachgemessen hat.

  **Der naheliegende Weg ist versperrt.** SARIF hat mit `runs[].artifacts` und
  `invocations` Felder für „was habe ich angefasst"; gemessen an den Dateien dieses Repos
  füllt sie kein Werkzeug: `gosec`, `ruff`, `gitleaks` und `golangci-lint` lassen beide
  leer, `semgrep` schreibt ein `invocations` mit nichts als `executionSuccessful: true`.
  Aus dem SARIF selbst ist es also nicht abzulesen.

  Was bleibt, steht auf der Eingangsseite: der Ausführer weiß vor dem Start, was da ist —
  er kennt das Ziel und die Sprachauswahl des Laufs. „0 Befunde bei 40 Go-Dateien" ist
  etwas anderes als „0 bei 0", und diese Unterscheidung braucht keine werkzeugspezifische
  Regel.

  **Entschieden: eine Auskunft neben dem Zustand.** Ein Job kann legitim nichts finden, ein
  `failed` dafür wäre falsch, und ein vierter Zustand verschöbe das Urteil bloß. Jeder Job
  trägt deshalb `candidates` — die Zahl der Dateien, die unter seinem Bezugspunkt als
  Gegenstand in Frage kamen. Was als Gegenstand zählt, sagt die gleichnamige Spalte in
  `scripts/scanners.tsv` (`source`, `any`, `manifest`, `none`), damit im Code kein
  Sonderfall je Werkzeugname entsteht. Die Zahl ist eine Obergrenze, keine
  Abdeckungsmessung; beurteilt wird sie im Bewertungsschritt. Einzelheiten in
  [`review-runs.md`](./review-runs.md), „`candidates` — was der Job hätte prüfen können".

  **Ergänzender Fall: Werkzeug erklärt selbst, dass es nichts zu prüfen gibt.** `candidates`
  greift, wenn der Ausführer vor dem Aufruf zählen kann, was es gäbe. Ein Werkzeug kann
  aber auch **selbst** entscheiden, dass es unter dem Bezugspunkt nichts zu prüfen gibt —
  wenn es Kandidaten sieht, sie aber nicht als Gegenstand akzeptiert. `osv-scanner` fand am
  2026-08-19 im OMNI-Lauf ein Kandidatenmanifest (`candidates: 1`), akzeptierte es aber
  selbst nicht und endete mit Exit 128 „No package sources found" und leerem SARIF. Die
  Kandidatenzählung allein reichte nicht: sie sagte, dass etwas da war; ob der Scanner es
  als Quelle nahm, wusste sie nicht. Deshalb steht die Auskunft jetzt neben `candidates`
  im Katalog: die Spalte `soft_skip` in `scripts/scanners.tsv` erlaubt je Zeile
  Kombinationen aus Exit-Code und Regex, unter denen der Job als `skipped` mit Grund gilt.
  Details oben unter „Erledigt: Soft-Skip aus dem Katalog".
