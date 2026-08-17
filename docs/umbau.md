# Umbau: projektlokale Installation

Arbeitsdatei für die Dauer der Umstellung. Sie hält fest, was besprochen und festgelegt
ist — nicht, was angedacht wurde. Was umgesetzt ist, steht nicht mehr hier, sondern in der
regulären Doku; wenn nichts mehr offen ist, wird diese Datei gelöscht.

Stand: 2026-08-12, Branch `main`.

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

**Offen.** Wird einzeln besprochen, bevor daran gearbeitet wird:

- Das Merge-Werkzeug. Der naheliegende Kandidat, der Microsoft SARIF Multitool, gibt es nur
  als .NET-Tool oder npm-Paket und zöge damit eine Laufzeitumgebung nach, die die
  Installation bisher nicht braucht.
- Was mit `trufflehog` und `pip-audit` geschieht, die kein SARIF können: umwandeln oder
  ersetzen.
- Eine Tabelle Regel → Schwere. `ruff` stuft im SARIF alles als `level: error` ein; bandit
  hätte Severity und Confidence geliefert. Statt das von einem einzelnen Scanner abhängig
  zu machen, soll die Zuordnung einmal in k-playbook stehen und für alle Werkzeuge gelten.
- Der Umbau der Rezepte auf reine Bewertung, und wo die Bewertung des Assistenten landet.
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
