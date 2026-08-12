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
| Entfallenes: CodeQL, `/k-install-security-tools`, `paths.*` | [`faq.md`](./faq.md) |
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
| `ruff` | Python | Qualität, dazu das `S`-Regelwerk (flake8-bandit) |
| `bandit` | Python | Python-Security, tiefer als ruffs `S` |
| `semgrep` | Python, Go | generische Security-Regeln |
| `gosec` | Go | Go-Security |
| `golangci-lint` | Go | Go-Qualität, bündelt staticcheck und errcheck |
| `govulncheck` | Go | Go-CVEs mit Reachability |
| `osv-scanner` | Python, Go | Dependency-CVEs |

Alle sieben können SARIF von sich aus. Von den vorhandenen können es `gitleaks`, `trivy`
und `grype`; `trufflehog` und `pip-audit` nicht. `syft` erzeugt eine SBOM und damit
überhaupt keine Befunde — es ist der Zulieferer für `grype` und bleibt außerhalb des
Merge.

**Offen.** Wird einzeln besprochen, bevor daran gearbeitet wird:

- Wie die Sprachen in der `K-PLAYBOOK.yaml`, in der `context`-Ausgabe und in der Oberfläche
  stehen.
- Das Ergebnisverzeichnis: ein Laufverzeichnis mit Tagesdatum unter
  `k-playbook-local/results/`, und was daraufhin aus `<familie>/YYYY-MM-DD/` wird.
- Die Ausführung durch das Werkzeug: Scan-Jobs, Parallelität, ein neues Subkommando.
- Das Merge-Werkzeug. Der naheliegende Kandidat, der Microsoft SARIF Multitool, gibt es nur
  als .NET-Tool oder npm-Paket und zöge damit eine Laufzeitumgebung nach, die die
  Installation bisher nicht braucht.
- Was mit `trufflehog` und `pip-audit` geschieht, die kein SARIF können: umwandeln oder
  ersetzen.
- Der Umbau der Rezepte auf reine Bewertung, und wo die Bewertung des Assistenten landet.
