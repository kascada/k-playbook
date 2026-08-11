# Umbau: projektlokale Installation

Arbeitsdatei für die Dauer der Umstellung. Sie hält fest, was besprochen und festgelegt
ist — nicht, was angedacht wurde. Wenn alles umgestellt ist, wird der bleibende Teil in
die reguläre Doku eingearbeitet und diese Datei gelöscht.

Stand: 2026-08-10, Branch `main`.

## Arbeitsteilung: Entwicklungsrepo vs. Installation

**`~/dev/k-playbook` ist das Entwicklungsrepo — keine Installation.** Hier entstehen und
werden per git bereitgestellt: die Skills, Commands, Checks, Reviews und Regeln, der
Installer und die Doku.

**Die tatsächliche Installation sieht anders aus.** Referenzprojekt zum Testen und
Anpassen ist `/home/kleist/dev/Aiva/kascada/`. Dort wird jede Umstellung gegen eine
echte, gewachsene Installation geprüft, nicht gegen ein frisch angelegtes
Beispielprojekt.

## Vorgaben

- Je Projekt eine eigene Installation.
- Die lokalen Einstellungen überschreiben die Vorgaben.
- Die Installation muss ohne Go möglich sein. Ein eigener Build muss trotzdem möglich
  sein und dasselbe Ergebnis liefern.
- `dist/` muss die Binaries mitliefern.
- Mehrere Zielplattformen gleichzeitig: macOS für den Host und Linux für den
  DevContainer. Ein Apple-Nutzer braucht beide.

## Festgelegtes Modell

- `<projekt>/k-playbook/` entsteht per `git clone` des Entwicklungsrepos. Der Clone
  bringt auch Quellcode und Docs mit; das Entwicklungsrepo wird passend dafür
  strukturiert.
- Das Werkzeug heißt `k-playbook`, nicht mehr `k-playbook-installer`. Es ist nicht nur
  der Installer, sondern soll künftig weitere Aufgaben übernehmen; die Aufgabe steckt im
  Subkommando.
- `bin/` enthält ausschließlich den Wrapper, versioniert im Repo, damit direkt nach dem
  Clone ein Einstiegspunkt vorhanden ist. `dist/` enthält ausschließlich die
  Plattform-Binaries. Es gibt nur ein Build-Target, damit ein eigener Build und die
  Auslieferung dasselbe Ergebnis liefern.
- Der Wrapper `bin/k-playbook` ruft die zur Plattform passende Version aus `dist/` auf.
- Nach dem Clone wird `k-playbook/bin/k-playbook` aufgerufen. Es startet die Oberfläche,
  die durch die Einrichtung führt; ein `install`-Unterkommando gibt es nicht.
- Geschrieben wird ausschließlich auf Bestätigung, Schritt für Schritt.

## Verzeichnisaufteilung und Anker

Die `K-PLAYBOOK.yaml` liegt im **Hauptverzeichnis**, nicht in der Installation:

```text
<projekt>/                 beliebig benannt
├── K-PLAYBOOK.yaml        der Anker
├── k-playbook/            Installation, vollstaendig ersetzbar
└── k-playbook-local/      projekteigen
```

Weil `k-playbook/` damit nichts Projekteigenes mehr enthält, ist es komplett updatebar —
auch per `rm -rf` und neuem Clone.

**Das Entwicklungsrepo wird wie jede andere Installation behandelt.** Es gibt keinen
Sonderfall und keine Erkennungsheuristik. Für `~/dev/k-playbook` heißt das:
`~/dev/k-playbook/K-PLAYBOOK.yaml` ist der Anker, die Installation liegt unter
`~/dev/k-playbook/k-playbook/`. Dass die dort installierte Version eine andere ist als der
Arbeitsstand daneben, wird bewusst in Kauf genommen.

**Das Playbook-Verzeichnis heißt immer `k-playbook`.** Skills und Commands verwenden
durchgängig `<projekt>/k-playbook/`. Wie das Projektverzeichnis heißt, spielt keine Rolle;
dass es hier ebenfalls `k-playbook` heißt, ist Zufall und darf kein Kriterium sein.

**Das Projekt-Repo steht in der Config**, nicht in einer Ableitung aus dem Dateisystem. Es
kann das übergeordnete Verzeichnis sein oder — etwa im DevContainer — ein paralleles.

## Mechanismus: Anker finden

Gilt gleichermaßen für die LLM und für das Go-Programm.

1. Wurde ein Verzeichnis übergeben, gilt dieses; geprüft wird `<arg>/K-PLAYBOOK.yaml`.
2. Sonst ab `realpath(CWD)` aufwärts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
3. Fund: `PROJECT_DIR = <dir>`, `PLAYBOOK_DIR = <dir>/k-playbook`.
4. Grenze der Aufwärtssuche sind `$HOME` und `/`, jeweils einschließlich.
5. Nichts gefunden: melden, dass keine Installation vorliegt. Nicht raten, nichts anlegen.

Die Aufwärtssuche darf **nicht** am Git-Worktree-Root abbrechen. `<projekt>/k-playbook/`
ist ein eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, käme sonst
nie an die Config eine Ebene darüber.

## Anlegen, wenn nichts gefunden wird

Nach `git clone` existiert noch keine Config — die Suche schlägt also fehl. Statt zu raten,
schlägt das Werkzeug den Ort vor und lässt ihn bestätigen.

Der Vorschlag darf raten — anders als die Suche, denn geschrieben wird erst nach
Bestätigung. Kandidaten in dieser Reihenfolge:

1. **Das Git-Repository, in dem der Aufruf stattfindet.** Wer das Werkzeug startet, steht
   in aller Regel in dem Projekt, das er meint. Das ist der stärkste Hinweis.
2. **Aus dem Ort des Binaries.** Es liegt in `<X>/dist/`, also ist `X` die Installation.
   Ob `X` selbst das Hauptverzeichnis ist oder eine Ebene darunter liegt, hängt daran, ob
   die Installation geklont wurde oder das Repo selbst ist — beides kommt vor, deshalb
   stehen beide Orte zur Auswahl.
3. Das Arbeitsverzeichnis.

Ein früherer Ansatz leitete das Hauptverzeichnis allein aus dem Binary-Pfad ab und prüfte,
ob das Zwischenverzeichnis `k-playbook` heißt. Das ging im Entwicklungsrepo schief: dort
heißt das Hauptverzeichnis selbst so, und der Vorschlag landete eine Ebene zu hoch.

Die `.git`-Suche beantwortet nicht, wo `A` liegt, sondern was in `project.repo_root`
gehört:

| Situation | Hauptverzeichnis | `repo_root` |
|---|---|---|
| `A/.git` vorhanden | `A` | `.` |
| `A/G/.git`, `A` selbst ohne `.git` | `A` | `G` |
| mehrere Kandidaten unter `A` | `A` | leer, der Nutzer wählt |

Geschrieben wird ausschließlich auf Bestätigung. Eine vorhandene `K-PLAYBOOK.yaml` wird
nie überschrieben — sie gehört dem Projekt und kann Werte tragen, die das Werkzeug nicht
kennt.

## Verzeichnisstruktur eines Projekts

```text
projekt/
├── K-PLAYBOOK.yaml       der Anker; sein Ort bestimmt das Hauptverzeichnis
├── AGENTS.md             Instruktionen, eine Quelle für alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md
├── .claude/
│   ├── commands  ──┐     Symlink
│   └── skills      │     Symlink; OpenCode liest hier mit
├── .opencode/      │
│   └── commands  ──┤     Symlink
├── .cursor/        │
│   └── commands  ──┤     Symlink
├── k-playbook/   ←─┘     die Installation, vollständig ersetzbar
│   ├── commands/ skills/ rules/ reviews/ checks/
│   ├── bin/ dist/ scripts/
│   ├── k-playbook.md     mitgelieferte Instruktionsebene
│   └── installer/ docs/
└── k-playbook-local/     projekteigen, committed
    ├── rules/            Overlay zu k-playbook/rules/
    ├── reviews/          Overlay zu k-playbook/reviews/
    ├── checks/           Overlay zu k-playbook/checks/
    ├── results/          Review-Ergebnisse
    ├── docs/             Projektwissen für AI-Sessions
    ├── guidelines/
    ├── tasks/done/
    ├── priv/             Inhalt gitignored, Verzeichnis versioniert
    ├── k-playbook.md     projekteigene Instruktionsebene
    └── TODO.md
```

Jedes Verzeichnis unter `k-playbook-local/` trägt eine `README.md` mit seinem Zweck —
auch weil Git leere Verzeichnisse nicht speichert und sie sonst nach einem Clone des
Projekts fehlen würden.

**Assistenten.** Verlinkt wird für Claude Code, OpenCode und Cursor. Skills stehen nur
einmal unter `.claude/skills`: OpenCode durchsucht dieses Verzeichnis mit, Cursor kennt
kein Skill-Konzept. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, weil Claude Code
ausschließlich `CLAUDE.md` liest und OpenCode `AGENTS.md` bevorzugt — so landet jede
Änderung in beiden. `AGENTS.md` wird angelegt, falls sie fehlt, und sonst um einen kurzen
Anstoß ergänzt; Näheres unter „Instruktionen".

**Commands und Skills gibt es nur mitgeliefert.** Es gibt kein
`k-playbook-local/commands/` und kein `k-playbook-local/skills/`. Das ist auch der Grund,
warum die Verlinkung so einfach bleiben kann: `.claude/commands` zeigt als einzelner
Symlink auf `k-playbook/commands`, und ein Symlink kann nur auf eine Quelle zeigen. Gäbe
es beide Verzeichnisse, müsste die Verlinkung pro Datei erfolgen und nach jedem Update
nachgezogen werden.

## Zusammenfassen: mitgeliefert und projekteigen

Drei Verzeichnisse existieren doppelt. Was gilt, ist die Vereinigung beider Seiten:

| Sorte | mitgeliefert | projekteigen | Dateimuster |
|---|---|---|---|
| Regeln | `k-playbook/rules/` | `k-playbook-local/rules/` | `*.md` |
| Review-Rezepte | `k-playbook/reviews/` | `k-playbook-local/reviews/` | `review-*.md` |
| Checks | `k-playbook/checks/` | `k-playbook-local/checks/` | `*.sh`, nur oberste Ebene |

Die Vergleichseinheit ist der **Dateiname**. Beide Seiten benutzen dieselbe
Namenskonvention, deshalb braucht es keinen abgeleiteten Schlüssel.

**Bei gleichem Dateinamen gewinnt die projekteigene Datei, und zwar vollständig.** Die
mitgelieferte wird dann gar nicht erst gelesen; es werden auch keine einzelnen Abschnitte
daraus übernommen. Wer eine mitgelieferte Regel ändern will, kopiert sie und ändert die
Kopie — mit dem bekannten Preis, dass spätere Verbesserungen am Original diese Kopie nicht
mehr erreichen. Der Vorteil wiegt schwerer: was gilt, steht in genau einer Datei.

**Abgeschaltet wird über eine leere Datei**, nicht über eine Liste in der Konfiguration.
Da eine gleichnamige lokale Datei die mitgelieferte vollständig ersetzt, bleibt bei einer
leeren nichts übrig. „Leer" heißt: nichts außer Leerzeilen und Kommentaren — so trägt die
Datei ihren eigenen Grund.

Der Unterschied zwischen den Sorten ist beabsichtigt: `rules` und `reviews` werden von
einem Assistenten gelesen, dort spricht der Text für sich. Ein Check wird ausgeführt — ein
leeres Skript liefe mit Exit 0 durch und sähe aus wie ein bestandener Check. Deshalb fällt
er ganz aus dem Katalog, statt leer zu laufen.

Eine projekteigene Datei schaltet man ab, indem man sie löscht.

`README.md` in einem der Verzeichnisse ist nie ein Eintrag, ebensowenig irgendetwas unter
`checks/lib/`.

**Nur projekteigen, ohne Gegenstück:** `results/`, `docs/`, `guidelines/`, `tasks/`,
`priv/`, `TODO.md`. **Nur mitgeliefert:** `commands/`, `skills/`, `docs/`, `scripts/`,
`bin/`, `dist/`, `installer/`.

`docs/` steht in beiden Listen, ist aber kein Paar: `k-playbook/docs/` dokumentiert
k-playbook selbst, `k-playbook-local/docs/` das Projekt. Zwei verschiedene Gegenstände
unter demselben Namen, nichts zusammenzufassen.

## Ergebnisse liegen unter `results/`

`k-playbook-local/reviews/` enthält ausschließlich Review-**Rezepte**. Alles, was ein
Review erzeugt, liegt unter `k-playbook-local/results/`:

```text
k-playbook-local/
├── reviews/                       nur review-<name>.md
└── results/
    ├── log.md                     wann welches Review lief
    ├── known-decisions.md         bewusst getroffene Entscheidungen
    ├── summary-YYYY-MM-DD.md      projektweite Priorisierung aus /k-results
    └── <familie>/YYYY-MM-DD/
        ├── assessment.md
        ├── findings.md
        ├── run-metadata.json
        └── raw/
```

Damit bleibt `reviews/` ein reines Overlay-Verzeichnis, in dem jede Datei nach derselben
Regel behandelt wird. Vorher lagen Rezepte, Log, Entscheidungen und Ergebnisse
durcheinander unter `<paths.reviews>/`.

## Der aufgelöste Arbeitsstand: `k-playbook context`

Das einzige Subkommando. Es gibt als JSON aus, was ein Command sonst selbst aus
Konfiguration und Dateisystem zusammenrechnen müsste: die aufgelösten Verzeichnisse, die
Instruktionsdateien in Lesereihenfolge, die Remediation-Policy, die Guidelines und die
drei Kataloge — mitgeliefert und projekteigen bereits zusammengeführt, mit Herkunft je
Eintrag (`dist`, `local`, `override`) und markierten Abschaltungen.

```bash
k-playbook/bin/k-playbook context
```

Das verschiebt die Overlay-Auflösung aus der Prosa in den Code. Bisher sollte jeder
Command die Regeln selbst anwenden — beschrieben in `commands/_shared/overlay-resolution.md`,
umgesetzt je nach Tagesform. Jetzt gibt es eine Antwort, und alle bekommen dieselbe.

Der Aufruf ist bewusst billig: der Security-Tool-Preflight fehlt darin, weil er je Tool
ein `--version` startet und spürbar dauert. `context` soll am Anfang jedes Commands
stehen können.

Gesucht wird ab dem Arbeitsverzeichnis aufwärts. Findet sich keine `K-PLAYBOOK.yaml`,
bricht der Aufruf mit einer Meldung ab. Eine `schema_version`, die nicht `3` ist, führt
ebenfalls zum Abbruch: die Werte ließen sich lesen, bedeuteten aber etwas anderes.

## Instruktionen: zwei Ebenen plus Anstoß

Was ein Assistent vor der Arbeit lesen soll, steht in `k-playbook.md` — je einmal pro
Ebene:

| Datei | Gilt für | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge; die projekteigene Ebene kann die mitgelieferte
ergänzen oder überstimmen. `context` nennt beide unter `instructions`, und zwar nur die,
die es tatsächlich gibt — ein Pfad ins Leere wäre schlechter als keiner.

Die Datei heißt bewusst **nicht** `AGENTS.md`: diesen Namen lesen die Assistenten von
sich aus, und er ist dem Hauptverzeichnis vorbehalten.

**`AGENTS.md` bekommt nur einen Anstoß.** Früher galt: fehlt die Datei, wird nichts
angelegt. Das hat nicht getragen — ohne Eintrag dort erfährt ein Assistent nie, dass es
k-playbook gibt. Jetzt wird sie angelegt, falls sie fehlt, und sonst um einen kurzen Block
ergänzt, der auf `k-playbook context` verweist. Vorhandener Inhalt bleibt unangetastet;
ein Marker `<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf den Block
erneut anhängt.

Der Anstoß nennt absichtlich keine Verzeichnisebene. Dieselbe Datei liegt im Projekt, in
der Installation und im Entwicklungsrepo — ein Verweis auf eine Ebene wäre an zwei dieser
Orte falsch. Wo die Instruktionen liegen, beantwortet der Aufruf.

## Aktualisieren aus der Oberfläche

Die Oberfläche prüft nach dem Start per `git ls-remote`, ob die Installation hinter dem
Remote liegt, und kann per `git pull --ff-only` nachziehen. Bewusst `ls-remote` statt
`fetch`: die Prüfung läuft ungefragt und darf den Zustand des Repositorys nicht anfassen.
Bewusst `--ff-only`: ein Merge im Clone erzeugte eine lokale Historie, die niemand pflegt.

Vor und nach dem Pull werden die Binaries unter `dist/` gehasht. Nur wenn sich etwas
geändert hat, wird ein Neustart verlangt — unter Linux behält der laufende Prozess seinen
Inode, läuft also mit dem alten Code weiter, auch wenn die Datei ersetzt wurde.

## Stand

Das Werkzeug führt durch drei Schritte: Konfiguration anlegen, projekteigene Struktur
anlegen, Assistenten verlinken. Dazu kommen ein rein lesender Block für die
Security-Tools, die Remediation-Policy, der aufgelöste Kontext und das Aktualisieren.

Der Go-Code liegt unter `installer/internal/`:

| Paket | Inhalt |
|---|---|
| `project/discover.go` | Anker finden, aufwärts ab einem Startverzeichnis |
| `project/environment.go` | was am Aufrufort vorliegt |
| `project/config.go` | Config lesen und anlegen, Schema prüfen, Vorschlag für Ort und `repo_root` |
| `project/local.go` | projekteigene Struktur prüfen und anlegen |
| `project/links.go` | Assistenten-Verlinkung prüfen und herstellen |
| `project/instructions.go` | `AGENTS.md` anlegen bzw. um den Anstoß ergänzen |
| `project/context.go` | Kataloge auflösen, Arbeitsstand zusammenstellen |
| `project/remediation.go` | `remediation:`-Block lesen und setzen |
| `project/update.go` | Remote-Stand prüfen, per Fast-Forward nachziehen |
| `project/tools.go` | Security-Tool-Preflight über das Skript |
| `legacy/global.go` | host-globale Verlinkung des alten Modells entfernen |
| `webui/` | Server, Endpunkte, eingebettete Oberfläche |

## Der alte Code als Nachschlagewerk

`installer/_old/` enthält den vollständigen Stand vor dem Umbau, rund 7800 Zeilen Go.
Verschoben per `git mv`, also in der Historie nachvollziehbar. Verzeichnisse mit
`_`-Präfix ignoriert die Go-Toolchain vollständig: kein Build, keine Tests, keine
Imports. Der Code ist dort **nicht baubar** — seine Imports zeigen auf Pfade, die es
nicht mehr gibt. Er ist zum Lesen und Herüberkopieren da, nicht zum Ausführen.

Was dort steht und noch gebraucht werden könnte:

| Ort | Inhalt |
|---|---|
| `_old/internal/install/config.go` | vollständiger Config-Vertrag: `PathKeys`, `ValidatePath`, `RenderConfig` |
| `_old/internal/install/migrate.go` | zeilenweise Migration, die Kommentare und unbekannte Blöcke erhält |
| `_old/internal/projects/status.go` | die elf Statusprüfungen der alten Oberfläche |
| `_old/internal/webui/server.go` | die alte GUI mit allen Bedienabläufen |
| `_old/payload/payload.go` | `go:embed` samt Extract-Logik des abgelösten Modells |

Als Referenz für eine gewachsene Konfiguration dient
`/home/kleist/dev/Aiva/kascada/k-playbook/K-PLAYBOOK.yaml`.

## Konfiguration

**`paths.*` entfällt ersatzlos.** Die neun Schlüssel waren nötig, solange die Struktur
frei wählbar war. Sie ist es nicht mehr: der Ort der `K-PLAYBOOK.yaml` bestimmt das
Hauptverzeichnis, daneben liegen `k-playbook/` und `k-playbook-local/` mit fester
Aufteilung. Ein Schlüssel, dessen Wert immer derselbe ist, wäre nur eine Fehlerquelle.
Commands und Skills leiten ihre Ziele künftig aus der Position der Datei ab statt sie zu
lesen; das ist ein eigener Umbauschritt.

**`k_playbook.dist`, `version`, `installed_at` und `layout`** beschreiben das abgelöste
Modell und entfallen ebenfalls. Eine Version ließe sich bei Bedarf aus
`git -C k-playbook describe --always` gewinnen.

**`schema_version` ist `3`.** Die `2` ist an das abgelöste Modell vergeben — Anker im
k-playbook-Verzeichnis, `_dist/`, `layout: project-local`, `paths.*`. Eine eigene Nummer
macht den Unterschied erkennbar, statt zwei unvereinbare Layouts unter derselben Zahl zu
führen.

Damit bleibt:

```yaml
schema_version: 3

project:
  repo_root: .                # Ort des Projekt-Repositorys, relativ zu dieser Datei
  vcs: git

remediation:                  # wie Befunde abgearbeitet werden
  mode: task-first
  target: .                   # relativ zum Hauptverzeichnis
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: false          # aus mode abgeleitet
  direct_fixes: true          # aus mode abgeleitet
```

Beim `remediation`-Block hat sich zweierlei geändert. **Der Default ist `task-first`**,
nicht mehr `direct-allowed`: Tasks als Standard sind die sichere Vorgabe, nichts wird ohne
Zutun am Code geändert, und direkte Fixes bleiben nach Freigabe trotzdem möglich. Und
**`pr_required` und `direct_fixes` werden aus `mode` abgeleitet** statt frei gepflegt. Sie
stehen trotzdem in der Datei, damit ein Command sie lesen kann, ohne den Modus deuten zu
müssen; geschrieben werden sie beim Setzen des Modus mit.

Der Block wird bei einer neuen Konfiguration gleich mit angelegt, damit nicht erst beim
ersten Review darüber entschieden werden muss.

## Entfallen

**Der Pfadvertrag `~/dev/k-playbook`** und alles, was daran hing: der Symlink, seine
Reparatur durch die Oberfläche, die Repo-Erkennung über Markerdateien und die Annahme
einer zentralen Basisinstallation, die viele Projekte bedient.

**Die host-globale Assistenten-Registrierung.** Es werden keine Symlinks mehr nach
`~/.config/opencode/command/`, `~/.claude/commands/` oder `~/.claude/skills/` gelegt und
kein `skills.paths` in der OpenCode-User-Config gepflegt. Verlinkt wird projektlokal, in
`.claude/`, `.opencode/` und `.cursor/` des Projekts. Damit kann ein Host mehrere Projekte
mit unterschiedlichen k-playbook-Ständen tragen, ohne dass sie sich gegenseitig
überschreiben.

Auf Rechnern mit einer alten Installation liegen diese globalen Links noch. Sie wirken
weiter in jedes Projekt hinein, deshalb entfernt `k-playbook` sie bei jedem Start selbst
(`installer/internal/legacy`): Symlinks unter den drei Verzeichnissen, deren Ziel ein
Pfadsegment `k-playbook` enthält, und den `skills`-Block der OpenCode-User-Config, wenn er
auf ein k-playbook zeigt. Entfernt der Schritt etwas, meldet er es im Terminal; sonst
bleibt er still.

**Die DevContainer-Integration.** Kein Bind-Mount nach `/workspaces/k-playbook`, kein
Symlink `~/dev/k-playbook` im Container, kein `.devcontainer/setup-k-playbook.sh` und kein
`install-devcontainer-k-playbook.sh`. Ein Container bekommt die Installation über das
Projektverzeichnis mit, wie jede andere Datei des Projekts auch.

**Der Command `k-install-security-tools`.** Status und Installationsbefehl kommen aus der
Oberfläche, alles Weitere kann das Skript selbst: `--preflight` ist sein Standardverhalten,
ohne `--yes` fragt es vor der Installation, und `--help` erklärt die Methoden. Der Command
hätte das nur in Prosa gedoppelt und wäre bei jeder Skriptänderung nachzuziehen gewesen.

**Die alten Subkommandos.** `init`, `update`, `restore`, `migrate`, `status`, `smoke` und
`projects …` gibt es nicht mehr, und damit auch keine lokale Projektliste unter
`.k-playbook-local/projects.json`. Ohne Argument startet `k-playbook` die Oberfläche.
Geblieben ist `context` — siehe unten.

## Nachzuziehen

Sammelstelle für alles, was der neuen Struktur noch folgen muss.

**Commands und Skills** leiten ihre Ziele noch aus `paths.*` ab, das es nicht mehr gibt.
Sie müssen künftig aus dem Ort der `K-PLAYBOOK.yaml` ableiten. Betroffen ist praktisch
jeder Command, im Kern aber `commands/_shared/path-resolution.md`, das noch `_dist/` als
Installation und die Config im k-playbook-Verzeichnis beschreibt.

**Ergebnisse liegen unter `k-playbook-local/results/`**, vorher unter
`<paths.reviews>/results/`. Umzustellen: `/k-results` sowie die Review-Rezepte, die dorthin
schreiben — `review-secret-scanning`, `review-codeql-security`, `review-k-check-security`,
`review-dependabot-alerts`, `review-dependency-cve`, `review-iac-container`, `review-tech`.
Dasselbe gilt für `log.md` und `known-decisions.md`, die `/k-review` bisher unter
`<paths.reviews>/` pflegte.

**Der projektlokale Regelordner heißt `rules/`**, nicht mehr `enforcement/`. Umzustellen:
`rules/README.md`, der Skill `enforcement` und der Command `k-enforcement`.

**`checks/README.md` und `bin/k-check`** setzen den Env-Kontrakt `K_PLAYBOOK_DIST` und
leiten das Verzeichnis aus der eigenen Lage ab.

**Die Oberfläche** deckt bisher nur Konfiguration, projekteigene Struktur und
Assistenten-Verlinkung ab. Was der Status je Projekt zeigen soll, nachdem der frühere
Projekt-Store entfallen ist, ist offen.

**Die Projekt-Dokumentation liegt fest unter `k-playbook-local/docs/`**, Tool-Steckbriefe
darunter in `libs/`. `paths.docs` war früher der eine Pfad, der das k-playbook-Verzeichnis
per `../` verlassen durfte — damit ein Projekt seine bereits vorhandene Doku
weiterverwenden konnte. Dieser Sonderfall entfällt zugunsten eines festen Ortes.
Umzustellen: `/k-code2docs`, `/k-tools-scan` und der Skill `ai-session-memory`, die alle
noch `k-playbook/docs/` fest verdrahten.
