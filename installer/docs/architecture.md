# Architektur

Session-Memory für die Arbeit am Werkzeug unter `installer/`. Diese Datei zuerst lesen,
bevor der Code erneut analysiert wird.

## Ziel

`k-playbook` hat zwei Aufgaben. Es **richtet ein**: Anker finden, Konfiguration und
projekteigene Struktur anlegen, den MCP-Server registrieren, Assistenten verlinken. Und es
**beantwortet, was gilt**:
`context` löst Verzeichnisse, Instruktionen und Kataloge auf, damit kein Command das
selbst tun muss. Dieselbe Antwort gibt `mcp` einem Assistenten als Werkzeug — siehe
„Der MCP-Server".

Alles Weitere — Reviews, Tasks, Checks — machen Commands und Skills im Assistenten, nicht
dieses Programm.

Das Werkzeug ist ein eigenständiges Go-Modul unter `installer/`, wird aber als
`bin/k-playbook` aus dem Repo-Root heraus aufgerufen.

## Vier Einstiege

```go
if len(args) == 0 {
    cleanUpLegacy()
    return webui.Run()
}
```

Ohne Argument die Oberfläche, davor das Aufräumen der Altlasten. Mit `context` die
JSON-Ausgabe, mit `mcp` der Server für einen Assistenten, mit `scan` die Ausführung
eines Review-Laufs — und alle drei **ohne** `cleanUpLegacy()` und `mirrorHostInstall()`:
deren Meldungen gehen nach stdout und würden die Ausgabe stören. Bei `mcp` wiegt das
schwerer als bei `context`: dort trägt stdout einen JSON-RPC-Strom, der über die ganze
Sitzung offen bleibt, und eine einzige fremde Zeile macht ihn unbrauchbar. Bei `scan`
zählt ein anderer Grund: ein Scan liest nur und soll den Host nicht nebenbei anfassen.

`scan <lauf> [eintrag …]` führt die Werkzeug-Einträge eines Laufs aus und blockiert, bis
sie durch sind. Das Kommando sammelt nur zusammen, was der Lauf braucht — Installation,
Preflight, Konfiguration — und reicht es an `internal/review` weiter; dort steht die
Ausführung selbst, ohne eigene Suche nach Pfaden oder Binaries. Wie der Lauf aussieht,
den es ausführt, steht in [`../../docs/review-runs.md`](../../docs/review-runs.md).

Mehr Subkommandos gibt es nicht. `init`, `update`, `restore`, `migrate`, `status`,
`smoke` und `projects …` des alten Stands sind entfallen, samt der lokalen Projektliste
unter `.k-playbook-local/projects.json`.

## Aufbau

```text
installer/
├── cmd/k-playbook/
│   ├── main.go                  räumt Altlasten weg, startet webui.Run()
│   └── scan.go                  Subkommando scan: Lauf lesen, Auswahl, Ausführung anstoßen
├── internal/legacy/
│   └── global.go                host-globale Registrierung des alten Modells entfernen
├── internal/hostinstall/
│   └── mirror.go                host-weite Kopie spiegeln, verlinken, PATH prüfen
├── internal/project/
│   ├── discover.go              Anker finden
│   ├── environment.go           was liegt hier vor
│   ├── config.go                Config lesen, Ort vorschlagen, anlegen
│   ├── local.go                 projekteigene Struktur prüfen und anlegen
│   ├── local_private.go         messen und umschalten, ob priv/ und material/ privat sind
│   ├── registry.go              Commands und Skills aus beiden Quellen auflösen
│   ├── links.go                 Assistenten-Verlinkung prüfen, herstellen, selbst heilen
│   ├── mcp.go                   MCP-Registrierung in den drei Assistenten-Dateien
│   ├── setup.go                 ein Ablauf für alle Einstiege: einordnen, Anstoß, verlinken
│   ├── instructions_layout.go   CLAUDE.md/AGENTS.md als Paar einordnen und auflösen
│   ├── remediation.go           remediation:-Block lesen und setzen
│   ├── context.go               Arbeitsstand auflösen: Pfade, Kataloge, Instruktionen
│   ├── instructions.go          AGENTS.md im Hauptverzeichnis prüfen und ergänzen
│   ├── gh.go                    tools.gh lesen und setzen, gh-Befund dieses Rechners
│   ├── update.go                Remote-Stand prüfen, Sauberkeit, Fast-Forward
│   ├── docs.go                  mitgelieferte Doku auflisten und lesen
│   ├── tasks.go                 offene und erledigte Tasks auflisten und lesen
│   ├── todos.go                 TODO.md parsen: offene und abgehakte Einträge
│   └── tools.go                 Security-Tool-Preflight über das Skript
├── internal/webui/
│   ├── server.go                Routen, Lebenszyklus
│   ├── browser.go               Browser öffnen, Container erkennen
│   ├── docs.go                  Doku-Endpunkte, Markdown nach HTML
│   ├── tasks.go                 Task-Endpunkte, Liste und einzelne Datei
│   ├── todos.go                 Todo-Endpunkte, offen und erledigt getrennt
│   ├── hostpath.go              PATH-Zustand melden, read-only
│   ├── mcp.go                   Registrierung messen und herstellen, Werkzeug-Selbsttest
│   ├── config.go local.go local_private.go assistant.go tools.go
│   ├── remediation.go context.go
│   ├── gh.go update.go reviews.go
│   └── static/                  index.html, workflows.html, docs.html, mcp.html,
│                                sidebar.html (Fragment der linken Spalte),
│                                session.js, nav.js, app.js, workflows.js,
│                                docs.js, mcp.js, styles.css
├── internal/mcpserver/
│   └── server.go                MCP-Server über stdio, Werkzeug k_playbook_context
├── internal/review/
│   ├── run.go                   Läufe anlegen und auflisten, run.json
│   ├── scanners.go              scanners.tsv lesen und prüfen: ein Aufruf je Job
│   ├── modules.go               Modulverzeichnisse suchen, Job-Namen daraus ableiten
│   ├── candidates.go            zählen, was ein Job hätte prüfen können — je
│   │                            Bezugspunkt und Sorte einmal im Lauf
│   ├── entries.go               entries/<name>.json, Zustandsableitung, atomares Schreiben
│   └── execute.go               Jobs starten, je Modul auffächern, SARIF zählen,
│                                Fortschritt fortschreiben
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details. Die
Trennung hält die Fachlogik testbar. `internal/mcpserver` steht neben `webui`: beide
sind Fassaden auf `project`, die eine über HTTP, die andere über JSON-RPC.

`internal/review` bekommt seine Vorgaben ebenfalls von außen — Laufverzeichnis, Ziel,
Sprachen, Katalog und die aufgelösten Werkzeuge stehen in `review.Options`. Deshalb
lässt sich ein Lauf mit Attrappen prüfen, ohne dass eine Installation, ein Preflight
oder ein echter Scanner vorhanden sein müsste.

Die eine Ausnahme ist `modules.go`: es sieht selbst auf die Platte. Eine Katalogzeile mit
`workdir: module` nennt kein Verzeichnis, sondern verlangt eins, und wo die Module eines
Projekts liegen, steht in keiner Konfiguration — `go.mod` ist die Tatsache im
Dateisystem. `FindModules()` nimmt das gesuchte Manifest als Parameter, damit `pip-audit`
denselben Mechanismus benutzen kann; angewandt wird er bisher nur auf Go. Aus dem
Ergebnis macht `execute.go` die Jobs: einer je Modul, bei genau einem Modul mit
unverändertem Namen ([`docs/review-runs.md`](../../docs/review-runs.md)).

## Anker finden

Der zentrale Mechanismus, in `project/discover.go`:

1. Ab `startDir` aufwärts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
2. Fund: `ProjectDir = <dir>`, `PlaybookDir = <dir>/k-playbook`.
3. Grenze sind `$HOME` und `/`, jeweils einschließlich.
4. Nichts gefunden: `ErrNotFound`. Nicht raten, nichts anlegen.

**Die Suche darf nicht am Git-Worktree-Root abbrechen.** `<projekt>/k-playbook/` ist ein
eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, käme sonst nie an
die Konfiguration eine Ebene darüber. Das ist der häufigste Fall, nicht der Sonderfall.

`homeDir()` löst Symlinks auf, damit der Vergleich auch bei verlinktem `$HOME` greift.

Es gibt **keine Fallunterscheidung** zwischen Entwicklungsrepo und Zielprojekt. Beide
tragen ihre eigene `K-PLAYBOOK.yaml` und werden gleich behandelt. Der einzige Unterschied
ist installiert oder nicht — siehe `Environment` in `project/environment.go`.

Für das Entwicklungsrepo heißt das: der Anker ist `<repo>/K-PLAYBOOK.yaml`, die
Installation liegt darunter in `<repo>/k-playbook/` und ist ein eigener Clone. Es gibt das
Playbook dort also zweimal — den Arbeitsstand im Repo und die installierte Fassung
daneben. Dass beide auseinanderlaufen können, wird bewusst in Kauf genommen; gearbeitet
wird am Repo-Stand, aufgelöst wird gegen die Installation.

## Ort vorschlagen, wenn nichts gefunden wurde

Nach einem `git clone` existiert noch keine Config, die Suche schlägt also fehl. `Suggest()`
in `project/config.go` **darf** raten, anders als `Discover` — geschrieben wird ohnehin
erst nach Bestätigung. Kandidaten in dieser Reihenfolge:

1. **Das Git-Repository, in dem der Aufruf stattfindet.** Wer das Werkzeug startet, steht
   in aller Regel in dem Projekt, das er meint. Der stärkste Hinweis.
2. **Aus `InstallDir()`.** Vorrangig sagt der Wrapper über
   `K_PLAYBOOK_INSTALL_DIR`, wo die Installation liegt; ersatzweise wird sie aus dem Ort
   des Binaries abgeleitet (`<X>/dist/` → `X`). Ob `X` selbst das Hauptverzeichnis ist
   oder eine Ebene darunter liegt, hängt daran, ob die Installation geklont wurde oder
   das Repo selbst ist — beides kommt vor, deshalb stehen beide Orte zur Auswahl.
3. Das Arbeitsverzeichnis.

Ein früherer Ansatz leitete das Hauptverzeichnis allein aus dem Binary-Pfad ab und
prüfte, ob das Zwischenverzeichnis `k-playbook` heißt. Das ging im Entwicklungsrepo
schief: dort heißt das Hauptverzeichnis selbst so, und der Vorschlag landete eine Ebene
zu hoch.

`suggestRepoRoot()` beantwortet eine andere Frage — nicht wo das Hauptverzeichnis `A`
liegt, sondern was in `project.repo_root` gehört:

| Situation | Hauptverzeichnis | `repo_root` |
|---|---|---|
| `A/.git` vorhanden | `A` | `.` |
| `A/G/.git`, `A` selbst ohne `.git` | `A` | `G` |
| mehrere Kandidaten unter `A` | `A` | leer, der Nutzer wählt |

## Config lesen ohne YAML-Parser

`ReadConfig()` liest zeilenweise statt mit einem YAML-Parser. Gelesen werden nur wenige
Skalare — `schema_version`, `project.repo_root`, `project.vcs` — und der zeilenweise
Zugriff lässt Kommentare, Reihenfolge und unbekannte Blöcke unangetastet, wenn später
zurückgeschrieben wird. Die Datei gehört dem Projekt und kann Werte tragen, die das
Werkzeug nicht kennt.

`CreateConfig()` schreibt nie über eine vorhandene Datei. `schema_version` ist `3`.

`SchemaState()` ordnet die gefundene Fassung ein — `ok`, `missing`, `outdated`, `newer`.
Die Oberfläche braucht den Fall und nicht nur den Fehlertext von `CheckSchema()`: nur
bei `outdated`/`missing` steht das Zurücksetzen zur Wahl. Bei `newer` wäre es genau
falsch, dort ist die Installation hinterher.

`ResetConfig()` in `project/reset.go` ist der einzige Weg, der eine vorhandene
Konfiguration ersetzt. Der Ablauf: Fassung prüfen, `LegacyContent()` prüfen, alte Datei
umbenennen, `CreateConfig()`. Scheitert der letzte Schritt, wird die Sicherung
zurückbenannt — sonst stünde das Projekt ohne Konfiguration da, und die Oberfläche böte
das Anlegen an, als wäre nie eine dagewesen.

`LegacyContent()` sucht Projektinhalte im Installationsverzeichnis, weil Modell 1 sie
dort ablegte und dieses Verzeichnis heute ersetzbar ist. Zwei Wege, weil keiner allein
reicht: der `paths.`-Block der alten Datei nennt die Orte genau, und der Zustand des
Verzeichnisses fängt den Rest ab — ohne `.git` gehört alles darin dem Projekt, mit
`.git` ist es das Untracked. Gemeldet wird die Vereinigung; verschoben wird nichts.

## Projekteigene Struktur

`LocalStructure()` in `project/local.go` ist die einzige Quelle für das, was unter
`k-playbook-local/` liegt:

```text
rules/  reviews/  checks/  commands/  skills/  results/  docs/  docs/manual/
guidelines/  tasks/  tasks/done/  priv/  material/  k-playbook.md  TODO.md
```

Jedes Verzeichnis bekommt eine `README.md` mit seinem Zweck — **auch weil Git leere
Verzeichnisse nicht speichert** und sie sonst nach einem Clone des Projekts fehlen
würden. Mehr schreibt `CreateLocal()` nicht — mit **einer** Ausnahme: Einträge mit
`PrivateByDefault` bekommen die verwaltete `.gitignore`, und zwar nur, wenn das
Verzeichnis in genau diesem Lauf entsteht. Deshalb wird vor `os.MkdirAll` geprüft, ob es
schon da ist; `MkdirAll` selbst meldet das nicht. Die Bedingung löst zwei Fälle auf
einmal: `makePublic()` entfernt die Datei bewusst, ein späterer `CreateLocal()`-Lauf —
jeder `/k-gui`-Start, jedes „Struktur anlegen" — dürfte sie nicht still zurückbringen;
und Bestandsprojekte mit getrackten Dateien unter `results/` landeten sonst im Zustand
`PrivacyPartial`. Ansonsten gilt weiter: was ein Projekt versioniert, entscheidet das
Projekt.

Das Feld `Private` an einem `LocalEntry` markiert, für welche Verzeichnisse diese Wahl
überhaupt ansteht — `results/`, `priv/` **und** `material/`. Bei `priv/` ist der Grund
offensichtlich: dort liegen eigene Notizen und Zwischenstände. Bei `material/` ist er
derselbe und wird leicht übersehen: Rohmaterial sind Chat-Mitschnitte, Notizen und
Zulieferungen, und die enthalten typischerweise Tokens, Pfade und Namen. Alle bekommen
über dasselbe `Private: true` denselben Weg zu einer eigenen `.gitignore`, die den Inhalt
ausschließt und das Verzeichnis selbst versioniert lässt. Ihre README beschreibt den Weg.
Das Feld geht als JSON an die Oberfläche und ist dort die Whitelist des Blocks
[Lokale Einstellungen](#lokale-einstellungen).

`results/` unterscheidet sich in einem Punkt: es trägt zusätzlich `PrivateByDefault` und
ist damit das einzige Verzeichnis, das bei der Installation schon privat angelegt wird.
Bei `priv/` und `material/` geht es um Geschmack, dort ist Zurückhaltung richtig. Bei
`results/` nicht: ein Werkzeug, das gefundene Secrets im Klartext ins Repository des
Nutzers schreibt, ist ein Fehler von k-playbook und keine Projektentscheidung — und die
Rohausgaben sind nur der schärfste Fall. Ein Review ist aus dem Code wiederholbar, sein
Ergebnis ist ein Stand von einem Rechner. Umschaltbar bleibt es trotzdem, in beide
Richtungen, und einmal umgeschaltet bleibt es dabei.

`writeIfMissing()` schreibt nur, wenn nichts da ist. Vorhandene READMEs mit eigenem Text
bleiben unberührt.

`commands/` und `skills/` sind darin die zwei Sorten, die ein Assistent direkt liest.
Wie sie mit den mitgelieferten verrechnet werden, steht im nächsten Abschnitt.

## Lokale Einstellungen

Der Block zeigt, was für dieses Projekt lokal entschieden ist, statt dass k-playbook es
stillschweigend erzwingt. Bisher steht dort eine einzige Frage, für drei Verzeichnisse:
ob der **Inhalt** von `results/`, `priv/` und `material/` aus der Versionskontrolle
bleibt. `project/local_private.go` misst das und schaltet es um,
`webui/local_private.go` reicht es an die Oberfläche.

„Statt dass k-playbook es stillschweigend erzwingt" trägt auch für `results/`: dort ist
die Antwort bei einer Neuinstallation zwar vorbelegt, aber sichtbar im selben Block und
in beide Richtungen umschaltbar — und `CreateLocal()` nimmt eine Umschaltung nie zurück.
Vorbelegt ist nicht erzwungen.

**Gemessen, nicht geraten.** Gefragt wird `git check-ignore -v --no-index` auf einen Pfad
*innerhalb* des Verzeichnisses; das eigene Parsen von `.gitignore`-Dateien fiele über
alle Ebenen, die globale Konfiguration und `.git/info/exclude` her, die git ohnehin schon
kennt. Der Pfad muss innen liegen, weil eine `.gitignore` **im** Verzeichnis mit `*`
dessen Inhalt ignoriert und nicht den Verzeichniseintrag — auf `priv` selbst gefragt
meldete check-ignore „nicht ignoriert". Ist das Verzeichnis von einer höheren Ebene
ignoriert, ist sein Inhalt es ebenfalls; dieser gleichwertige Weg fällt damit
automatisch mit ab.

`--no-index` ist Pflicht: per Default zieht git den Index heran und meldet eine getrackte
Datei als nicht ignoriert. Für genau die Zustände, die privat aussehen und keiner sind,
gäbe es dann gar keine Aussage.

Gemessen wird am nächstgelegenen Repository des jeweiligen Verzeichnisses, nicht zwingend
an `project.repo_root`: `project.repo_root` ist der Code-Zielbaum, während
`k-playbook-local/` auch in einem eigenen oder übergeordneten Repository liegen kann. Gibt
es dort kein Repository, meldet der Block das gezielt statt den rohen
`fatal: not a git repository`-Text von Git anzuzeigen.

Vier Zustände beschreiben ein Repository, nicht zwei:

| Zustand | Erkennung | Anzeige |
|---|---|---|
| `private` | Regel greift, weder Index noch HEAD tragen erfasste Dateien | privat |
| `public` | keine Regel | wird versioniert |
| `partial` | Regel greift, Dateien stehen im Index | teilweise privat: N Dateien stehen weiterhin im Repository |
| `pending-commit` | Regel greift, Index leer, HEAD trägt noch Dateien | privat erst nach dem nächsten Commit |

Die letzten beiden sind der Grund für den Block: beide sehen privat aus und sind es
nicht. Nach einem `git rm --cached` ist die Löschung nur gestaget; ohne Commit trägt jeder
Clone die Dateien weiter. Deshalb wird getrennt gegen den Index (`ls-files`) **und** gegen
`HEAD` (`ls-tree`) gehalten. Und was schon gepusht ist, bleibt in der Historie — kein
Zustand dieser Oberfläche macht das rückgängig; die Abfrage vor dem Umschalten sagt es.

Beide Listen laufen durch `check-ignore --stdin`, bevor sie zählen: der verwaltete Inhalt
lässt `README.md` und die `.gitignore` selbst ausdrücklich drin, ungefiltert stünde jedes
verwaltete Verzeichnis dauerhaft als `partial` da.

Der **Repo-Root** (`rev-parse --show-toplevel`) gehört zum Zustand und in die Anzeige:
`git -C <verzeichnis>` findet das nächstgelegene Repository, und liegt in
`k-playbook-local/` ein eigenes, gilt die Aussage für dieses. Ohne die Angabe wäre sie
mehrdeutig.

Drei Zustände sagen, dass es nichts zu messen gab, und sind trotzdem kein Fehler:
`no-vcs` (`project.vcs` ist nicht `git`), `missing` (Verzeichnis noch nicht angelegt) und
`unknown` mit Grund (kein git, kein Repository, Timeout, Exit 128). Vorbedingungen und
Timeout folgen `agentsIgnored`, mit einer bewussten Abweichung: **Exit 1 wird nicht wie
jeder andere Nicht-Null-Ausgang behandelt.** Er ist die reguläre Antwort „keine Regel",
während 128 heißt, dass die Frage gar nicht beantwortet wurde — sonst stünde ein kaputter
git-Aufruf als „nicht privat" da. Ein Repository ohne Commits wird vorher abgefangen:
`ls-tree HEAD` scheitert dort mit Exit 128, und ein frisch initialisiertes Repository
stünde ohne diesen Vorabtest als `unknown` statt als `private` da.

### Gemessen wird über alle Ebenen, geschrieben nur eine Datei

Umschalten wird **nur angeboten, wenn die von `check-ignore -v` gemeldete Quelle genau die
verwaltete `.gitignore` im Verzeichnis mit genau ihrem Inhalt ist** (`*`, `!.gitignore`,
`!README.md`). Stammt die Regel von woanders — Projekt-Root-`.gitignore`,
`.git/info/exclude`, globale Konfiguration — oder trägt die Datei eigenen Inhalt, wird
nichts geschrieben: der Zustand wird angezeigt und die fremde Quelle benannt. Sonst wäre
ein Ausschalten entweder wirkungslos oder es löschte eine Datei, die dem Projekt gehört.

`SetPrivate()` schreibt die Datei und nimmt die davon erfassten Dateien mit
`git rm --cached` aus dem Index — als Teil derselben Operation, sonst bliebe genau der
Zwischenzustand stehen, den der Block sichtbar machen soll. Welche Dateien das sind,
beantwortet nach dem Schreiben eine erneute Messung, keine eigene Ableitung aus dem
Dateiinhalt. Zurück kommt der neue Zustand plus die Liste der genommenen Dateien und der
Hinweis auf den ausstehenden Commit. Der Aufruf ist idempotent.

### Kontrakt des POST

Eingabe ist ein **Eintrag plus Zielzustand**, kein freier Pfad — der Handler führt
schreibende git-Operationen aus. Zulässig ist nur, was in `LocalStructure()` steht und
dort `Private` trägt; alles andere wird mit 400 abgelehnt. Kein Projekt konfiguriert:
dieselbe Behandlung wie in `createLocalHandler`.

### Abgrenzung zum Block „Projekteigene Struktur"

Der zeigt weiterhin, welche Verzeichnisse es gibt und wozu sie da sind, `private`-Flag als
Eigenschaft des Eintrags eingeschlossen. „Lokale Einstellungen" zeigt ausschließlich den
gemessenen Ist-Zustand dieser Entscheidung im Repository. Der Zustand hat nur eine Quelle:
`PrivacyStatuses()`. Die Struktur-Liste bleibt unverändert.

Die Handanleitung selbst steht in den `Purpose`-Texten in `project/local.go`, aus denen
die READMEs entstehen; sie verweisen auf den Block. `writeIfMissing()` überschreibt eine
vorhandene README nie — in bestehenden Projekten bleibt der alte Text stehen. Das bleibt
so und wird nicht umgangen.

## Commands und Skills auflösen

`project/registry.go` führt zusammen, was in `k-playbook/` mitgeliefert wird und was
unter `k-playbook-local/` dazukommt. Es ist dieselbe Overlay-Regel wie bei rules, reviews
und checks: **gleicher Name gewinnt projekteigen, ein leerer Eintrag schaltet ab.**

| Sorte | Einheit | Schlüssel | Abschalten durch |
|---|---|---|---|
| `commands` | eine `*.md`-Datei | Pfad ab `commands/`, z. B. `_shared/context.md` | leere Datei |
| `skills` | ein Verzeichnis mit `SKILL.md` | Verzeichnisname | leere `SKILL.md` |

Commands werden **rekursiv** aufgelöst. Namensraum-Verzeichnisse wie `_shared/` sind
damit bis auf die einzelne Datei überlagerbar: ein Projekt ersetzt `_shared/context.md`
und behält den Rest des Namensraums aus der Installation.

Der Preis der Rekursion: Jede Datei in einem solchen Verzeichnis wird auch **registriert**
und erscheint beim Assistenten als `_shared:context`. Das Frontmatter ändert daran
nichts — `collectCommands()` überspringt nur Dotfiles, Symlinks und `README.md`. Wer ein
reines Include will, das nicht als Command auftaucht, muss den Ausschluss dort ergänzen.

Skills werden als Ganzes überlagert, nicht Datei für Datei. `SKILL.md`, `PLAYBOOK.md`
und `vorlagen/` müssen zueinander passen; ein halb ersetzter Skill wäre nicht sinnvoll
zusammensetzbar.

`ResolveRegistry()` liefert alle Einträge mit ihrer Herkunft (`dist`, `local`,
`override`) einschließlich der abgeschalteten — damit die Oberfläche sie zeigen kann.
`ActiveRegistry()` liefert nur die, die tatsächlich registriert gehören.

## Assistenten-Verlinkung

`Links()` in `project/links.go`:

| Link | Inhalt | Wer liest |
|---|---|---|
| `.claude/commands` | Katalog `commands` | Claude Code |
| `.claude/skills` | Katalog `skills` | Claude Code, OpenCode |
| `.opencode/commands` | Katalog `commands` | OpenCode |
| `.cursor/commands` | Katalog `commands` | Cursor |
| `CLAUDE.md` | Datei-Link auf `AGENTS.md` | Claude Code |

Skills stehen nur einmal: OpenCode durchsucht neben `.opencode/skills/` auch
`.claude/skills/`, ein zweiter Ort wäre Dopplung. Cursor kennt kein Skill-Konzept.

`CLAUDE.md` zeigt auf `AGENTS.md`, weil Claude Code ausschließlich `CLAUDE.md` liest und
OpenCode `AGENTS.md` bevorzugt. Ein Symlink statt eines Imports, damit eine Änderung
immer in beiden ankommt — wer in `CLAUDE.md` schreibt, schreibt durch den Link hindurch.
Der Link ist `Optional`, weil seine Quelle dem Projekt gehört und nicht der Installation.

Die Richtung ist eine Konstante, keine projektabhängige Variable: ein umgedrehter Link
müsste in Prüfung, Oberfläche und Doku dauerhaft mitgedacht werden. Deshalb wird eine
mitgebrachte echte `CLAUDE.md` einmalig **umbenannt** statt der Link umgedreht. Siehe
[Instruktionsdateien einordnen](#instruktionsdateien-einordnen).

Die Reihenfolge — einordnen, `ApplyRootInstructions()`, `ApplyLinks()` — steckt in
`ApplyAssistantSetup()` (`project/setup.go`) und nicht mehr im Handler: der Symlink
braucht `AGENTS.md` als Ziel, und die Umbenennung muss vor dem Anlegen aus der Vorlage
laufen. Siehe [Instruktionen](#instruktionen).

### Selbstheilung auf dem Lesepfad

`HealLinks()` in `project/links.go` prüft, richtet ein, was sich einrichten lässt, und
meldet als `LinkRepair`, was danach offen bleibt. Gerufen wird es von zwei Stellen, die
beide **lesen** wollen: `assistantHandler()` (`GET /api/assistant`) und `ContextForDir()`
— und damit vom Subkommando `context` wie vom MCP-Werkzeug.

Der Grund ist eine Zuordnung, die lange falsch herum gedacht war. Welche Links gelten,
folgt dem **Katalog dieses Projekts**, nicht dem Weg, auf dem die Installation zu ihrem
Stand kam. Hing das Nachziehen an einem Weg — dem Update-Handler —, blieb die
Registrierung bei jedem anderen stehen: `make installer-update`, `make installer-sync`,
ein `git pull` von Hand. Und zwar unbemerkt; gesehen hat es nur, wer die
Assistenten-Karte öffnete. Der Lesepfad ist die richtige Stelle, weil er ohnehin bei
jedem Sitzungsstart betreten wird und weil er dasselbe Projekt meint.

Zwei Eigenschaften halten das billig und harmlos:

- **Geschrieben wird nur, wenn Schreiben etwas ändert.** `LinksFixable()` fragt vorher,
  ob Einrichten überhaupt etwas bewirkt. Ohne die Unterscheidung schriebe jeder
  Lesezugriff in einem blockierten Projekt dieselben Links neu — `applyRegistryLink()`
  setzt jeden Link unbedingt neu.
- **`blocked` und `conflict` lösen kein Anwenden aus.** Beide sind für `ApplyLinks()`
  unauflösbar und stehen stattdessen in `LinkRepair.Open`.

Die Bilanz stammt aus dem Zustand **vor** dem Anwenden — danach ist sie per Definition
leer, und genau sie ist das, was gelesen werden soll. Bei einem Ziel, das es vorher gar
nicht gab, bleibt sie leer und `Applied` trägt die Aussage: dort hat sich keine
Registrierung verändert, dort ist eine entstanden.

In der Kontextausgabe steht das Ergebnis als `links` und **fehlt im Normalfall**
(`contextLinks()` in `project/context.go`): eine Meldung, die bei jedem Aufruf dasselbe
sagt, liest niemand mehr. `note` sagt dazu, was das für die laufende Sitzung heißt —
nämlich nichts, weil Assistenten ihre Command-Liste beim Start lesen.

`ContextForDir()` baut **erst** den Kontext und heilt **dann**. Bricht der Aufbau an
einer unlesbaren oder zu neuen Konfiguration ab, wird auch nichts eingerichtet: ein
Werkzeug, das die Fassung nicht versteht, soll die Registrierung nicht nach seinen Regeln
umschreiben.

Der Knopf bleibt trotzdem. `ApplyAssistantSetup()` tut mehr als die Heilung: Es ordnet
das Paar `CLAUDE.md`/`AGENTS.md` ein und legt den Anstoß an — Inhalt, der auf einem
Lesepfad nichts zu suchen hat.

### Instruktionsdateien einordnen

`project/instructions_layout.go` ordnet das **Paar** (`CLAUDE.md`, `AGENTS.md`) ein,
bevor irgendetwas geschrieben wird. Abgelesen wird mit `Lstat` und `Readlink`, nie mit
`os.Stat`: das folgt Symlinks, und die verdrehte Richtung — `AGENTS.md` als Link auf
`CLAUDE.md` — bliebe damit unsichtbar. Verglichen wird das aufgelöste Ziel, nicht der
Rohstring aus `Readlink`.

Jede der beiden Dateien steht in genau einem von acht Zuständen (fehlt, echte Datei,
Verzeichnis, Link auf die andere, Link auf ein fremdes vorhandenes Ziel, Rest-Link,
unlesbar, sonstiges). Die Fallmatrix daraus wird von oben nach unten geprüft, die erste
passende Zeile gewinnt, und die letzte fängt alles Übrige auf — kein Zustand fällt durch.
Aufgelöst werden nur die Zeilen, bei denen nichts verloren geht: Umbenennen und das
Entfernen eines irreführenden Symlinks an `AGENTS.md`.

Alles andere wird als `StateConflict` gemeldet und **nicht angefasst** — auch nicht
angelegt. Der Detailtext nennt die Ursache, den Ausweg und die Folge: bis zur Handarbeit
sieht Claude Code den Anstoß nicht. Prüfung und Einrichten benutzen dieselbe
Klassifikation; zwei Implementierungen liefen auseinander. `checkFileLink()` ordnet
deshalb in dieser Reihenfolge ein: Konflikt, `blocked`, `no-source`, Zielzustand.

Ein einziger Vorbehalt blockiert eine sonst mögliche Umbenennung: steht in der
`K-PLAYBOOK.yaml` `project.vcs: git` und meldet `git check-ignore --quiet AGENTS.md`
(bewusst **ohne** `--no-index`, damit eine getrackte Datei als nicht ignoriert gilt)
Exit 0, nähme die Umbenennung versionierten Inhalt still aus der Versionskontrolle. Ein
Gate auf einen sauberen git-Zustand gibt es dagegen nicht — ein Arbeitsverzeichnis mit
Änderungen ist der Normalfall.

### Einzel-Links statt Verzeichnis-Symlink

Die vier Katalog-Ziele sind **echte Verzeichnisse** mit je einem Symlink pro Eintrag:

```text
.claude/commands/
  k-todo.md    -> ../../k-playbook/commands/k-todo.md          dist
  k-review.md  -> ../../k-playbook-local/commands/k-review.md  override
  k-eigen.md   -> ../../k-playbook-local/commands/k-eigen.md   local
  _shared/
    context.md   -> ../../../k-playbook/commands/_shared/context.md
```

Ein Verzeichnis-Symlink zeigt auf genau eine Quelle; damit käme entweder nur die
Installation oder nur das Projekt an. Bis Fassung 0.4 war das so. `checkRegistryLink()`
erkennt einen solchen Link und meldet ihn als `stale`; `ApplyLinks()` baut ihn um.

### Soll gegen Ist

`checkRegistryLink()` vergleicht den aufgelösten Katalog mit dem, was im Zielverzeichnis
tatsächlich registriert ist, und nennt die Abweichungen **beim Namen**:

| Feld | Bedeutung |
|---|---|
| `missing` | im Katalog, aber nicht registriert |
| `wrong` | registriert, zeigt aber auf die falsche Quelle — typisch, wenn das Projekt einen mitgelieferten Eintrag neuerdings überschreibt |
| `stale` | registriert, steht aber nicht mehr im Katalog — entfernt oder abgeschaltet |
| `blocked` | eine echte Datei des Projekts steht an der Stelle; sie gewinnt und bleibt liegen |

`blocked` zählt **nicht** als offener Punkt. Der Eintrag bleibt in der Liste sichtbar —
sonst wundert man sich, warum ein Override nicht greift —, aber der Gesamtzustand bleibt
`ok`: `ApplyLinks()` könnte daran nichts ändern, und ein Knopf, der etwas verspricht,
das er nicht einlöst, ist schlechter als ein Hinweis.

Die Gegenrichtung — `stale` — braucht ein Kriterium dafür, welche Links von uns stammen.
`ownedLinks()` löst jeden Symlink auf und nimmt nur die, die nach `k-playbook/` oder
`k-playbook-local/` zeigen. Alles andere im Verzeichnis gehört dem Projekt und wird
weder bewertet noch angefasst.

`ApplyLinks()` räumt erst die verwaisten Links weg und setzt dann die des Katalogs. Die
Reihenfolge zählt: ein abgeschalteter Eintrag muss verschwinden, bevor ein gleichnamiger
aus der anderen Quelle nachrückt. Danach entfernt `removeEmptyDirs()` Namensraum-
Verzeichnisse, die dabei leer geworden sind.

Zustände in `LinkState`:

| Zustand | Bedeutung |
|---|---|
| `ok` | alles steht so, wie es stehen soll |
| `missing` | nichts vorhanden |
| `stale` | Datei-Link auf ein falsches Ziel, oder Verzeichnis-Symlink aus einer älteren Fassung |
| `incomplete` | das Verzeichnis steht, sein Inhalt weicht vom Katalog ab |
| `blocked` | etwas Echtes steht im Weg; wird nicht angefasst |
| `no-source` | es gibt nichts zu verlinken |
| `conflict` | `CLAUDE.md` und `AGENTS.md` lassen sich nicht auflösen, ohne Inhalt zu verlieren oder zu verdoppeln; der Detailtext nennt die Auflösungen |

## Instruktionen

Zwei Ebenen, in `project/context.go` und `project/instructions.go`:

| Datei | Gilt für | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

`instructionFiles()` sammelt sie in dieser Reihenfolge und nimmt nur auf, was existiert —
ein Pfad ins Leere wäre schlechter als keiner.

Die Datei heißt bewusst **nicht** `AGENTS.md`. Diesen Namen lesen die Assistenten von
sich aus, und er ist dem Hauptverzeichnis vorbehalten.

`ApplyRootInstructions()` legt `AGENTS.md` an, falls sie fehlt, und hängt sonst einen
kurzen Anstoß an, der auf `k-playbook context` verweist. Vorhandener Inhalt wird nie
überschrieben. Der Marker `<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf
den Block erneut anhängt; `CheckRootInstructions()` prüft darauf.

Intern nimmt die Funktion einen Schalter: `applyRootInstructions(projectDir, mayCreate)`.
Im Konfliktfall ruft `ApplyAssistantSetup()` sie **ohne** Anlegen. Zu verhindern ist
ausschließlich das Anlegen — es erzeugte neben dem echten Inhalt eine zweite, fast leere
Instruktionsquelle und folgte bei einem Rest-Link sogar dessen totem Ziel. Das Anhängen
bleibt richtig, wo bereits eine Instruktionsdatei steht.

Der Anstoß nennt **keine Verzeichnisebene**. Dieselbe Datei liegt im Projekt, in der
Installation und im Entwicklungsrepo — ein Verweis auf eine Ebene wäre an zwei dieser
Orte falsch. Wo die Instruktionen liegen, beantwortet der Aufruf.

## Kataloge auflösen

`BuildContext()` und `resolveCatalog()` in `project/context.go` sind der Kern. Hier liegt
die Overlay-Regel als Code statt als Prosa, die jeder Command selbst befolgen müsste.

Drei Sorten, definiert in `catalogKinds()`:

| Sorte | Verzeichnis | Muster |
|---|---|---|
| `rules` | `rules/` | `*.md` |
| `reviews` | `reviews/` | `review-*.md` |
| `checks` | `checks/` | `*.sh` |

Die Vergleichseinheit ist der **Dateiname** — beide Seiten benutzen dieselbe
Namenskonvention, ein abgeleiteter Schlüssel wäre unnötiger Zwischenschritt. `key` gibt
es trotzdem: den Aufrufnamen ohne Endung und Sortenpräfix, damit `/k-review secret-scanning`
funktioniert.

Die Vereinigung beider Seiten, mit `origin` je Eintrag:

| `origin` | Bedeutung |
|---|---|
| `dist` | nur mitgeliefert |
| `local` | nur projekteigen |
| `override` | projekteigen, ersetzt einen gleichnamigen mitgelieferten |

**Abgeschaltet wird über eine leere Datei**, nicht über eine Liste in der
Konfiguration. `isEmptyFile()` gilt als leer, was außer Leerzeilen und Kommentaren nichts
enthält — so kann die Datei ihren Grund tragen. Der Eintrag bleibt im Katalog und trägt
`disabled: true`.

Warum keine Liste: die lokale Datei ersetzt die mitgelieferte ohnehin vollständig. Bei
einer leeren bleibt nichts übrig — das Abschalten fällt aus der bestehenden Regel heraus,
statt einen zweiten Mechanismus zu brauchen. Es steht außerdem im Repo, versioniert und
mit Begründung, statt in einer Konfigurationszeile.

`isCatalogEntry()` filtert, was nie ein Eintrag ist: `README.md`, Dotfiles und alles, was
nicht zum Muster passt. Unterverzeichnisse wie `checks/lib/` ebenso — dort liegt
Hilfscode.

Der Security-Tool-Preflight fehlt im Kontext bewusst: er startet je Tool ein `--version`
und dauert spürbar. `context` soll billig genug sein, um am Anfang jedes Commands zu
stehen.

`CheckSchema()` läuft vor allem anderen. Bei einer anderen Fassung als `3` bricht
`BuildContext()` ab, statt zu raten — die Werte ließen sich lesen, bedeuteten aber etwas
anderes.

## Aktualisieren

`project/update.go`, zwei Schritte:

`CheckUpdate()` fragt den Remote-Stand ab. Bewusst `git ls-remote` statt `git fetch`: die
Prüfung läuft ungefragt nach dem Start und darf den Zustand des Repositorys nicht
anfassen. Upstream wird aus `branch.<name>.remote` und `branch.<name>.merge` gelesen; ohne
Upstream, ohne Branch oder bei nicht erreichbarem Remote gibt es eine Meldung statt eines
Fehlers. Ein Timeout von 15 Sekunden verhindert, dass ein hängender Remote die
Oberfläche blockiert.

`Update()` holt den Stand per `git pull --ff-only`. Nur Fast-Forward: ein Merge im Clone
erzeugte eine lokale Historie, die niemand pflegt. Wer dort committet hat, soll das selbst
auflösen. Die Installation ist normalerweise read-only; `Update()` macht sie unmittelbar
vor dem Pull für den Eigentümer beschreibbar und setzt sie per `defer` danach wieder
read-only.

Beim Start der Oberfläche ruft `protectProjectInstallation()` denselben Schutz auf. Das
ist keine Sicherheitsgrenze gegen den Besitzer des Verzeichnisses, sondern eine wirksame
Barriere gegen versehentliche Schreibwerkzeuge: in `k-playbook/` wird nur noch in
gezielten Wartungswegen geschrieben.

### Die Installation muss sauber sein

`CheckCleanliness()` liest bei jeder Prüfung den lokalen Zustand des Clones mit — rein
lokal, ohne Netz, deshalb billig genug für den ungefragten Lauf nach dem Start. Die
Git-Aufrufe laufen mit `GIT_OPTIONAL_LOCKS=0`, damit die rein lesende Prüfung auch in
einem read-only gesetzten Clone nicht am Index-Lock scheitert.

Der Grund ist ein stiller Fehlerfall. Das Modell verlangt, dass in `k-playbook/` nie
geschrieben wird, aber die Regel erzwingt sich nicht. Ändert sich eine lokal veränderte
Datei upstream nicht mit, läuft `git pull` sauber durch und lässt sie stehen: die
Änderung überlebt dann jedes Update, ohne je gemeldet zu werden. Ändert sie sich doch
mit, bricht git ab — mit einer Meldung, die im `output` verschwindet.

Drei Zustände, zwei Schweregrade:

| Zustand | `Blocking()` | Warum |
|---|---|---|
| verfolgte Datei geändert/gelöscht | ja | geht beim Update verloren oder verhindert es |
| zusätzliche Datei | nein | steht einem Fast-Forward nicht im Weg, gehört aber nach `k-playbook-local/` |
| lokale Commits (`@{u}..HEAD`) | ja | blockieren `--ff-only`, nur von Hand auflösbar |

`Update()` prüft **vor** dem Pull und bricht bei `Blocking()` ab, statt hinterher zu
stolpern. Das ist der Unterschied zwischen „irgendwas ging schief" und „`bin/k-playbook`
ist verändert".

Die Oberfläche zeigt den Befund in einer eigenen Karte, weil dort Dateinamen hinmüssen.
Ist ein Remote-Update verfügbar und der Zustand blockierend, heißt der Kopfknopf
„Update blockiert" statt „Update verfügbar"; ein Klick prüft erneut, startet aber keinen
Pull. Bewusst **ohne** Knopf zum Zurücksetzen: das wäre `git checkout -- .` in einem
fremden Verzeichnis, und die Oberfläche kann nicht wissen, ob dort jemand absichtlich
entwickelt. Der Befehl steht zum Kopieren da.

Ist ausgerechnet `bin/k-playbook` die veränderte Datei, ist die Oberfläche über den
Wrapper nicht mehr erreichbar. Dann führt der host-weite `k-playbook` aus dem `PATH` zum
selben Ergebnis.

Vor und nach dem Pull wird `VERSION` gelesen. `BinaryChanged` meldet, ob sie gewechselt
hat — **nur dann** gehört zum neuen Stand ein anderes Binary, und nur dann bringt ein
Neustart eine andere Programmversion. Unter Linux behält der laufende Prozess ohnehin
seinen Inode und arbeitet mit dem alten Code weiter.

Früher wurden dafür die Dateien unter `dist/` gehasht. Seit die Binaries Release-Assets
sind, liegt im Clone keins mehr, das sich vergleichen ließe; `VERSION` ist an diese
Stelle getreten und trennt zugleich sauber: Commits an Regeln, Reviews, Commands oder
Docs ändern sie nicht.

Hat `VERSION` gewechselt, ruft der Update-Pfad
`filepath.Join(PlaybookDir(projectDir), "bin", "k-playbook")` mit `--prefetch` auf und
legt das neue Binary gleich in den Cache — sonst wartet der Nutzer beim Neustart auf den
Download. Festgenagelt genau dieser Wrapper und nicht `InstallDir()`: das laufende Binary
kann eine fremde Installation aktualisieren (die WebUI ruft
`project.Update(environment.ProjectDir)`), und nur der frisch gezogene Clone trägt nach
dem Pull die neue `VERSION` samt passender Prüfsummen.

Der Prefetch ist **best effort** — eigener Timeout statt des `pullTimeout`-Kontexts,
Fehler nur als Hinweis in `Message` und `Output`, nie als Rückgabefehler. Sonst ließe der
benannte Rückgabefehler mit `defer keepInstallationReadOnly(projectDir, &err)` ein bereits
erfolgreiches `git pull` als gescheitertes Update erscheinen — getroffen wird das offline,
hinter Proxys und genau im Fenster zwischen Tag und Asset-Upload.

### Die Verlinkung wird mitgezogen

`relinkAfterUpdate()` in `webui/update.go` ruft nach erfolgreichem Pull
`ApplyAssistantSetup()` auf. Das ist kein Komfort, sondern nötig: seit Commands und
Skills **einzeln** verlinkt werden, kommt ein neu mitgelieferter Command nicht mehr von
selbst an. Ein Verzeichnis-Symlink hatte das automatisch getan; ein Update, das den
Katalog ändert, ihn aber nicht registriert, wäre halb erledigt — und zwar unsichtbar.

Aufgerufen wird derselbe Ablauf wie beim Einrichten, nicht bloß `ApplyLinks()`. Zwei
Änderungen an diesem Einstieg sind gewollt und stehen deshalb im Antworttext: das
Aktualisieren bringt jetzt den Anstoß mit — der Marker macht das idempotent —, und in
einem Projekt ohne `AGENTS.md` legt es die Datei erstmals aus der Vorlage an. Sonst
bliebe ein Projekt mit nur echter `CLAUDE.md` über „Aktualisieren" für immer unverändert.
Umbenennung und Konflikt gehören ebenfalls in den Antworttext.

`PendingLinkChanges()` liest vorher die Bilanz und meldet sie: dazugekommen, entfernt,
auf eine andere Quelle umgesetzt. Die Namen werden **über alle Ziele zusammengefasst**,
sonst zählte ein einzelner neuer Command dreifach — er steht in `.claude/`, `.opencode/`
und `.cursor/`.

Schlägt das Nachziehen fehl, bleibt das Update gültig: der Pull ist durch, und die
Verlinkung lässt sich über die Assistenten-Karte nachholen.

Seit die Heilung auf dem Lesepfad sitzt (siehe
[Selbstheilung auf dem Lesepfad](#selbstheilung-auf-dem-lesepfad)), ist dieser Aufruf
nicht mehr die einzige Absicherung, sondern die frühestmögliche: Er meldet die Bilanz
schon im Antworttext des Updates, statt sie dem nächsten Lesezugriff zu überlassen. Und
er tut mehr — `ApplyAssistantSetup()` statt bloß `ApplyLinks()`.

## Host-weite Spiegelung

`Mirror()` in `hostinstall/mirror.go` läuft bei jedem Start der Oberfläche, direkt nach
dem Aufräumschritt. Sie löst das Problem, dass die Oberfläche häufig gebraucht wird,
aber nur über `<projekt>/k-playbook/bin/k-playbook` erreichbar ist.

Der globale Aufruf ist überhaupt möglich, weil das Programm sein Projekt aus dem
**Arbeitsverzeichnis** ableitet (`Detect()` über `os.Getwd()`) und nicht aus seinem
eigenen Ort. Ein einziges Binary bedient damit alle Projekte; projektspezifisch ist nur
der Kontext, nicht das Werkzeug.

Gespiegelt wird nach:

```text
~/.local/
├── bin/
│   └── k-playbook -> ../share/k-playbook/installation/bin/k-playbook
└── share/k-playbook/
    ├── installation/
    │   ├── bin/k-playbook                 Kopie des Wrappers
    │   ├── VERSION                        welches Release dazugehört
    │   ├── SHA256SUMS                     Prüfsummen der Assets
    │   └── .stamp                         Commit-Stand der Quelle
    └── security-tools/                    Tool-venvs, davon unberührt
```

Die Ebene `installation/` trennt die Spiegelung von den Tool-venvs, die unter
`~/.local/share/k-playbook/` ebenfalls zuhause sind (`rules/tool-install-scope.md`). Ein
venv bringt ein eigenes `bin/` mit; ohne diese Ebene kollidierten beide.

### Kopie statt Symlink ins Repo

Der abgelöste Weg war ein Symlink von `~/.local/bin/k-playbook` auf den Wrapper **eines
bestimmten Clones**. Das bindet die host-weite Installation an ein einzelnes Projekt: wird
dort nicht gepullt, veraltet sie, und verschwindet der Clone, zeigt der Link ins Leere. Im
DevContainer kam hinzu, dass er nach jedem Rebuild von Hand neu zu setzen war.

Stattdessen kopiert nun jeder Start seine eigenen Dateien dorthin, sofern er einen neueren
Stand mitbringt. Wer aus einem aktuelleren Clone startet, hebt die host-weite Kopie damit
von selbst an — und ein gelöschter Clone stört nicht.

### Der Wrapper, kein Binary

Gespiegelt werden genau drei Dateien: `bin/k-playbook`, `VERSION` und `SHA256SUMS`. Kein
Binary — das löst der Wrapper über den Cache auf, und dort liegt es ohnehin schon.

Der Wrapper allein trägt auch den Fall Mac mit DevContainer: `~/.local/bin` ist derselbe
Pfad, Host und Container brauchen aber verschiedene Plattformen. Früher musste dafür je
Plattform ein eigenes Binary neben dem Wrapper liegen; heute trennt sie der Cache über
den Dateinamen `k-playbook-<os>-<arch>`, und die Kopie muss davon nichts wissen. Die
Plattformwahl bleibt zur Laufzeit, wo sie hingehört. Der Wrapper löst seine
Symlink-Kette selbst auf und leitet seine Installation aus dem **aufgelösten** Ort ab; er
braucht für den Symlink in `~/.local/bin` keine Anpassung.

**Ein zurückgebliebenes `dist/` wird entfernt.** Kopien aus der Zeit vor den
Release-Assets tragen dort noch ein Binary, und der Wrapper zieht ein vorhandenes `dist/`
dem Cache vor — es bliebe sonst für immer der Startpunkt der host-weiten Kopie. Sein
Vorhandensein löst die Spiegelung auch dann aus, wenn der Stempel gleich geblieben ist.

### Commit-Stand statt mtime

Verglichen wird der Zeitpunkt des HEAD-Commits: `git log -1 --format=%ct`. Der Wert
landet als `.stamp` im Wurzelverzeichnis der Kopie.

Früher zählte der letzte Commit an `dist/`. Seit dort nichts mehr liegt, wäre das ein
eingefrorener Stempel. Der HEAD wechselt dafür bei jedem Content-Commit: die Kopie wird
öfter erneuert als früher — sie ist dafür klein, und der Wrapper ist genau die Datei, die
aktuell sein muss.

Die mtime der Dateien wäre das naheliegende Kriterium und ist trotzdem falsch: Git setzt
sie beim Auschecken auf den Zeitpunkt des Clones, nicht des Commits. Ein frisch geklonter
alter Stand sähe damit neuer aus als eine korrekte Installation und würde sie
überschreiben — bei mehreren Projekten mit je eigenem Clone ständig.

Ist die Quelle kein Git-Repository, bleibt der Stempel leer und es wird nur gespiegelt,
wenn im Ziel etwas fehlt. Ein unbekannter Stand darf einen bekannten nicht verdrängen.

### Fehlende Datei schlägt den Stempel

Der Stempel allein entscheidet nicht: fehlt im Ziel eine der drei Dateien, wird kopiert,
auch wenn die Stände gleich sind. Früher trug das den Fall, dass die Kopie nur die
Plattformen enthielt, von denen aus sie schon einmal aufgerufen wurde — mit dem Wegfall
des Binaries ist daraus die schlichte Vollständigkeitsprüfung geworden.

### Kein Sonderfall DevContainer

Im Container ist `~/.local/bin` ein **anderes** Verzeichnis: der Benutzer ist `vscode`
oder `root`, gemountet wird standardmäßig nur der Workspace nach `/workspaces/<name>`,
nicht das Home. Die Spiegelung läuft dort deshalb ganz normal und erzeugt eine
container-eigene Kopie. Nach einem Rebuild ist sie weg und wird vom nächsten Start
wiederhergestellt — genau der Vorzug gegenüber dem alten Symlink.

`containerMarker()` in `webui/browser.go` bleibt davon unberührt und dient weiterhin
allein dazu, die geratenen Browser-Kandidaten auszusortieren — siehe
[Browser öffnen](#browser-öffnen).

### Nur beim Start der Oberfläche

Aufgerufen wird ausschließlich im Zweig ohne Argumente, nicht bei `context`. Dessen JSON
auf stdout verträgt keine Beigaben — dieselbe Begründung, aus der auch der
Aufräumschritt dort ausgelassen wird. Zugleich spart es das Git-Kommando bei den
häufigen Kontextaufrufen der Commands, denen die Spiegelung ohnehin nichts bringt: sie
rufen `k-playbook/bin/k-playbook context` projektlokal auf und berühren den `PATH` nie.

### Schreiben ohne Kollision

Jede Datei wird nach `<ziel>.tmp` geschrieben und dann umbenannt. Das Umbenennen ist
atomar und umgeht `ETXTBSY`: eine parallel laufende Instanz hält die alte Datei offen,
während der Name schon auf die neue zeigt.

Der Symlink wird angelegt oder neu ausgerichtet, wenn er fehlt oder woandershin zeigt.
Liegt dort eine **echte Datei**, gewinnt sie und bleibt unberührt — dieselbe Regel wie
bei der Assistenten-Verlinkung.

Wie beim Aufräumschritt meldet sich die Spiegelung nur, wenn etwas passiert ist, und ein
Fehler hält den Start nicht auf: die Oberfläche läuft auch ohne host-weite Kopie.

### Der PATH wird geprüft, nicht geschrieben

`CheckPath()` ist die rein lesende Gegenstück-Funktion: Liegt der Symlink? Steht
`~/.local/bin` im `PATH`? Sie hängt **nicht** an `Mirror()`. Das war vorher so und war
ein Fehler: der Hinweis stand unter `!result.Empty()` und erschien damit nur beim ersten
Start. Beim zweiten war nichts zu spiegeln, `Result` blieb leer — und der Hinweis fiel
weg, obwohl der `PATH` weiterhin nicht stimmte.

`ExportLine()` baut die Zeile fürs Profil und setzt `$HOME` ein, wenn das Verzeichnis
darunter liegt. Dasselbe Profil wird auf Host und im Container gelesen; ein absoluter
Pfad wäre dort falsch.

**Geschrieben wird in kein Shell-Profil.** Es gibt zu `/api/path` kein `POST`. Das Profil
gehört dem Nutzer, und ein Programm, das ungefragt darin schreibt, wäre schwerer zu
durchschauen als eine Zeile zum Kopieren. Die Oberfläche zeigt den Zustand — als Karte,
die nur erscheint, solange etwas fehlt —, gehandelt wird im Terminal.

Geprüft wird der `PATH` **dieses** Prozesses. Wer die Zeile gerade eingetragen hat, sieht
die Änderung erst in einer neuen Shell; die Meldung sagt das dazu.

## Woher Binary und Dateien kommen

Zwei Dinge, die getrennt driften können — und deren Verwechslung teuer ist.

**Das Binary** gibt es an mehreren Orten. **Die Dateien** — Skripte, Tool-Matrix, Regeln,
Reviews, Checks — kommen dagegen **immer** aus `PlaybookDir(projektDir)`, also aus
`<projekt>/k-playbook/`. Das Binary liest nie neben sich.

Das ist Absicht — und seit die Binaries Release-Assets sind, unumgänglich: das Binary
liegt im Regelfall im Cache unter `$HOME`, weit außerhalb jeder Installation. „Neben dem
Binary" gäbe es dort nichts zu lesen. Genau deshalb sagt der Wrapper über
`K_PLAYBOOK_INSTALL_DIR`, wo die Installation liegt, statt sie aus dem eigenen Ort raten
zu lassen.

| Ort | Was | Wird aktualisiert durch |
|---|---|---|
| `$K_PLAYBOOK_CACHE`, sonst `$XDG_CACHE_HOME/k-playbook`, sonst `$HOME/.cache/k-playbook` | Cache, geteilt über alle Projekte | Download beim Start oder `bin/k-playbook --prefetch` |
| `<projekt>/k-playbook/bin/` | Wrapper der Installation | `git pull` im Clone, also „Update prüfen" |
| `~/.local/share/k-playbook/installation/bin/` | Wrapper der host-weiten Kopie | `Mirror()` bei jedem Start — mit Einschränkung, siehe unten |
| `~/.local/bin/k-playbook` | Symlink auf den Wrapper der Kopie | `Mirror()` |
| `<entwicklungsrepo>/dist/` | Build des Arbeitsstands, schlägt Cache und Download | `make dist` / `make dist-host` |

**Die host-weite Kopie erneuert sich nicht beim lokalen Bauen.** `needsCopy()` vergleicht
den Commit-Zeitpunkt des HEAD — bewusst nicht die Dateizeit, weil Git die beim Auschecken
auf den Zeitpunkt des Clones setzt. `make dist` ändert diesen Stempel nicht, die Kopie
bleibt also stehen.

### Die Auflösungskette des Wrappers

`bin/k-playbook` nimmt das erste Binary, das er findet:

1. **`$K_PLAYBOOK_BINARY`** — ausdrücklich gesetzt, gewinnt immer. Zeigt es auf nichts
   Ausführbares, bricht der Wrapper ab, statt weiterzusuchen: eine gesetzte Variable ist
   eine Ansage, kein Vorschlag.
2. **`<installation>/dist/`** — im Repo-Checkout hat der lokale Build Vorrang vor Cache
   und Download. Nur so bleibt `make gui` netzfrei.
3. **Der Cache** — `$K_PLAYBOOK_CACHE`, sonst `$XDG_CACHE_HOME/k-playbook`, sonst
   `$HOME/.cache/k-playbook`, darunter `bin/<version>/k-playbook-<os>-<arch>`.
4. **Download** des Release-Assets zu der Version aus `<installation>/VERSION`, geprüft
   gegen `<installation>/SHA256SUMS`.

Geladen wird in eine Temp-Datei im Zielverzeichnis und dann umbenannt — parallele Starts
sehen so nie eine halbe Datei. Stimmt die Prüfsumme nicht, wird die Datei verworfen.

**Warum der Cache außerhalb der Installation liegt.** Die Installation wird nach jedem
Update per `chmod -R a-w` gesperrt; in ein `dist/` darunter könnte der Wrapper nicht
schreiben. Der Cache löst das und wird zugleich von allen Projekten desselben Rechners
geteilt. Host und Container kollidieren nicht, weil der Dateiname die Plattform trägt.

**Der Wrapper exportiert `K_PLAYBOOK_INSTALL_DIR`** mit seinem eigenen Elternverzeichnis.
Das Binary kann aus dem Cache kommen und wüsste sonst nicht, zu welcher Installation es
gehört. `InstallDir()` liest die Variable vorrangig; die Ableitung aus dem Binary-Pfad
bleibt Rückfall für direkt gestartete Binaries und gilt nur, wenn unter dem abgeleiteten
Verzeichnis auch wirklich `bin/k-playbook` oder `K-PLAYBOOK.yaml` liegt. Ohne diese
Plausibilisierung lieferte ein Cache-Binary `<cache>/bin` mit `ok = true` — kein Fehler,
sondern ein falsches Ergebnis.

**Kein Versions-Fallback.** Findet der Wrapper zur `VERSION` kein Asset, bricht er mit
einer Meldung ab, die jede geprüfte Stelle und die Wege weiter nennt — `--prefetch`,
`K_PLAYBOOK_CACHE`, `make dist-host`. Ein stiller Rückfall auf eine ältere Version
widerspräche der Zusage, dass ein Clone-Stand eindeutig einem Binary zugeordnet ist.

Daraus folgt für den Aufruf:

| Start | Binary aus | Dateien aus | kann driften |
|---|---|---|---|
| `<projekt>/k-playbook/bin/k-playbook` | Cache oder Download zur `VERSION` des Clones | dem Clone | nein |
| `k-playbook` aus dem `PATH` | Cache oder Download zur `VERSION` der Kopie | der Installation des aktuellen Projekts | ja |
| `make installer-run` im Entwicklungsrepo | dem Arbeitsstand | siehe „Entwicklungsstand" | ohne Sync ja |

### Entwicklungsstand

Im Entwicklungsrepo fallen Quelle und Installation auseinander: `~/dev/k-playbook/` ist
der Arbeitsstand, `~/dev/k-playbook/k-playbook/` ein eigener Clone auf dem zuletzt
gepushten Commit. Ein frisch gebautes Binary läse also weiterhin alte Dateien.

`make installer-sync` spielt deshalb den verfolgten Dateisatz — `git ls-files`, per
Definition das, was ein Clone enthält — in die Installation ein und legt dort
`.k-playbook-devsync` ab. `make installer-run` tut das mit. Zurück geht es über die
Oberfläche, siehe unten — ein Make-Target dafür gibt es bewusst nicht: der nächste
`installer-sync` spielt ohnehin wieder ein, ein zweiter Weg im Terminal führte nur zum
selben Ergebnis.

Die Markierung ist nötig, weil Git die eingespielten Dateien zwangsläufig als Änderungen
sieht. Verbergen lässt sich das nicht: `.git/info/exclude` wirkt nur auf Unverfolgtes,
`--assume-unchanged` ist unverbindlich, und `--skip-worktree` bricht den Checkout. Also
wird der Zustand benannt statt versteckt — `CheckCleanliness()` prüft die Markierung vor
dem `git status` und meldet `DevSync` statt einer Liste einzelner Dateien. `Blocking()`
bleibt trotzdem wahr: ein Pull in einen eingespielten Stand wäre falsch.

Ohne diese Unterscheidung stünde in der Installations-Karte dauerhaft „lokal gearbeitet" —
also genau der Alarm, der echte Handarbeit im Clone melden soll, und der damit wertlos
würde.

**Verworfen wird aus der Oberfläche.** Der Update-Knopf heißt im Entwicklungsstand
„Arbeitsstand verwerfen" und ruft `/api/update/discard` auf; dahinter steht
`DiscardDevSync()` mit `git checkout -- .` und `git clean -fd`. Danach zeigt die Antwort
den neuen Stand, aus dem wie gewohnt „Update verfügbar" werden kann — **zwei Klicks, nicht
einer.** Der Zustand, den man ansieht, verschwindet beim Verwerfen; das gehört angesagt und
nicht in dieselbe Aktion wie das Aktualisieren gepackt.

Dass die Oberfläche hier von sich aus `git checkout -- .` und `git clean -fd` ausführt, ist
die einzige Ausnahme von der Regel, dass sie fremde Verzeichnisse nicht zurücksetzt. Sie
trägt: die Markierung sagt, woher der Inhalt kommt, nämlich aus dem Arbeitsstand. Verworfen
wird eine Kopie, keine Arbeit. **Ohne Markierung lehnt `DiscardDevSync()` ab** — dann lässt
sich nicht wissen, ob dort jemand absichtlich entwickelt hat, und es bleibt beim Befehl zum
Kopieren.

## Altlasten des globalen Modells

`RemoveGlobalLinks()` in `legacy/global.go` läuft bei jedem Programmstart, vor der
Oberfläche. Sie räumt weg, was das abgelöste host-globale Modell hinterlassen hat:

| Ort | Alte Form |
|---|---|
| `~/.claude/commands` | Verzeichnis-Symlink auf `<repo>/commands`, sonst Einzel-Symlinks darin |
| `~/.claude/skills` | Einzel-Symlinks auf die Skill-Verzeichnisse des Repos |
| `~/.config/opencode/command` | Einzel-Symlinks auf `<repo>/commands/*.md` |
| `~/.config/opencode/opencode.jsonc`, `.json` | `skills.paths` auf die Basisinstallation |

Bleiben diese Links liegen, sieht ein Assistent in **jedem** Projekt zusätzlich die
Commands eines fremden Standes — genau das, was die projektlokale Installation auflösen
soll. Deshalb der Aufräumschritt beim Start und nicht in der Oberfläche.

Erkannt wird an einem Pfadsegment `k-playbook` im Symlink-Ziel, unabhängig davon, ob das
Ziel noch existiert: der Repo-Pfad war frei wählbar, das Verzeichnis hieß aber immer so,
und ein toter Link ist genauso eine Altlast wie ein lebender. Echte Dateien und Symlinks,
die woandershin zeigen, bleiben unberührt; ein Verzeichnis fällt nur weg, wenn danach
nichts mehr darin steht. Aus der OpenCode-Config wird ausschließlich der Top-Level-Key
`skills` geschnitten, wenn sein Wert ein `k-playbook` nennt — der Rest der Datei bleibt
Zeichen für Zeichen erhalten, Kommentare eingeschlossen. Deshalb ein eigener
JSONC-Scanner statt Parsen und Neuschreiben.

Gemeldet wird nur, wenn tatsächlich etwas wegfällt; auf einem sauberen Rechner ist der
Schritt still. Ein Fehler dabei hält den Start nicht auf.

## Security-Tool-Preflight

`CheckTools()` in `project/tools.go` ruft
`k-playbook/scripts/install-security-tools.sh --json` auf und liest dessen JSON.

Die Tool-Liste steht **nicht** im Go-Code. Sie kommt aus `scripts/security-tools.tsv`,
derselben Matrix, die auch das Skript und die Review-Rezepte lesen. Dort steht auch, wie
ein Tool auf den Host kommt (`install_method`, `install_ref`) und für welche
Projektsprachen es zuständig ist (`languages`) — ein neues Tool ist eine Zeile in der
TSV, keine Änderung an Skript oder Go-Code.

`Tool.Required` trägt die Sprachregel bereits: ein sprachgebundenes Tool ist nur Pflicht,
wenn seine Sprache im Aufruf stand. Ohne `--languages` gilt nur Sprachunabhängiges als
Pflicht — was nicht gefragt wurde, kann auch nicht fehlen. Go reicht die Antwort
unverändert durch und rechnet nichts nach.

Der Aufruf ist ausschließlich lesend: `--json` prüft nur, ob die Binaries vorhanden
sind. Installiert wird bewusst im Terminal, weil das die Arbeitsumgebung verändert und
nicht die Projekt-Abhängigkeiten. Ein Timeout von 30 Sekunden begrenzt den Aufruf, weil
der Preflight je Tool ein `--version` startet und eines davon hängen kann.

Ein aktives Projekt-venv ist für diesen read-only Aufruf erlaubt. Das JSON meldet dann
`toolScope: "project-venv"`, damit die Oberfläche den Status nicht als host-/user-lokale
Tool-Installation ausgibt. Bricht das Skript aus anderen Gründen ab, landet die erste
stderr-Zeile in der Fehlermeldung.

## GitHub CLI

`project/gh.go` hält zwei Dinge auseinander, die in einer Karte zusammen erscheinen.

Die Projektentscheidung steht in `K-PLAYBOOK.yaml` unter `tools.gh.status` und wird
zeilenweise gelesen und geschrieben, wie der Rest der Konfiguration. `SetGHStatus()`
ersetzt nur den `gh:`-Unterblock; `replaceNestedBlock()` lässt einen danebenliegenden
Block eines anderen Tools stehen. Der Default ist `unknown` und
kein `disabled`: ohne Entscheidung weiß ein Command nicht, ob ein fehlendes `gh` ein
Problem oder gewollt ist, und die Oberfläche zeigt den offenen Punkt rot. Ein unbekannter
Wert lässt `BuildContext()` abbrechen, damit ein Tippfehler nicht wie eine Entscheidung
aussieht.

Der Host-Befund kommt aus `DetectGH()`: ein `exec.LookPath("gh")` und ein Blick in
`hosts.yml` im gh-Konfigurationsverzeichnis, dazu `GH_TOKEN`/`GITHUB_TOKEN`, die die
Datei stechen. Gelesen werden dort nur `user` und die Namen unter `users`; die
Token-Zeilen daneben werden übergangen und tauchen in keiner Antwort auf.

**Kein Aufruf von `gh auth status`.** Der prüft den Token beim Server und kostet einen
Netzzugriff. Deshalb steht der Befund anders als der Security-Preflight im Kontext: er
kostet nichts. Der Preis ist, dass ein abgelaufener Token als Anmeldung gilt — das sagen
Karte und Doku ausdrücklich.

Geschrieben wird nur die Entscheidung. Installation und Anmeldung bleiben im Terminal,
aus demselben Grund wie bei den Security-Tools; `gh auth login` will ohnehin einen
Browser. Auch das Umschalten zwischen Accounts steht nur als Befehl da: es gilt
maschinenweit für jedes Terminal und jedes Projekt, und ein Approve läuft danach unter
dem neuen Namen. Ein Knopf in einer Projektoberfläche würde diese Reichweite verdecken.

## Bereiche und die linke Spalte

Die Oberfläche hat drei Bereiche: **Setup** unter `/`, **Workflows** unter `/workflows`
und **Docs** unter `/docs`. `/mcp` ist keine vierte Sorte, sondern die Detailseite des
Setup-Blocks und trägt dessen Bereich.

Jede dieser Seiten hat links dieselbe Spalte, und die beantwortet zwei Fragen: In welchem
Bereich bin ich, und was steht in diesem Bereich? Oben der **Umschalter**, eine Liste von
`<a>` mit dem aktiven Bereich markiert; darunter das **Blockmenü** der Seite.

Beides steht in einem einzigen Template-Fragment, `static/sidebar.html` mit
`{{define "sidebar"}}`. `pageTemplate()` parst es mit jeder Seite zusammen — die
Seitendatei zuerst, denn `ParseFS` benennt das Ergebnis nach der ersten Datei, und
`Execute` führt damit die Seite aus und nicht das Fragment. Dreimal dasselbe Markup zu
kopieren wäre die Variante, die beim nächsten Bereich wieder auseinanderläuft.

Welcher Eintrag aktiv ist, kommt aus den Vorlagendaten: `renderPage()` bekommt den
Bereich vom Handler. Ob es Workflows und Docs überhaupt gibt, entscheidet `.Installed` —
vor der Einrichtung führt der Umschalter nur nach Setup, weil die beiden anderen Bereiche
dort nichts zu zeigen hätten.

Das Blockmenü ist **generiert** und steht in `static/nav.js`: `buildBlockNav()` läuft über
`.blocks > .card`, nimmt Id und `<h2>` jeder Karte und hängt je einen Eintrag an
`#block-nav`. Der Statuspunkt spiegelt die Pill der Karte, ein `MutationObserver` zieht
das nach — eine neue Karte braucht deshalb nichts weiter als Id und Überschrift. Der
Aufruf muss **vor** den Ladefunktionen einer Seite stehen: die blenden Karten ein, und
das Menü zieht das nur mit, wenn es sie schon beobachtet.

`nav.js` gehört keiner Seite und holt sich `#block-nav` selbst, statt in ein
seitenspezifisches `elements` zu greifen. Eine Ausnahme kennt der Mechanismus: `/docs`
füllt dieselbe Liste aus seinen Dateien statt aus Karten und nutzt davon nur
`markBlockNavItem()`.

Unter 1080px entfällt das Blockmenü — für die Karten bliebe daneben zu wenig übrig. Der
Umschalter bleibt und legt sich waagerecht über die Karten: ohne ihn wären die anderen
Bereiche von dort aus nicht mehr erreichbar.

## Befehle zum Kopieren

Mehrere Karten zeigen einen Befehl, der ins Terminal gehört — `PATH`-Zeile, Aufräumen der
Installation, `gh`-Installation und -Anmeldung, Security-Tools. Jeder dieser Blöcke trägt
einen Knopf, der den Befehl in die Zwischenablage legt.

Der Knopf steht **unter** dem Befehl, nicht darin: der Block scrollt waagerecht, ein Knopf
darüber würde Text verdecken. Sein Ziel nennt er über `data-copy` als Element-Id, und ein
einziger Listener am Dokument bedient alle — die Blöcke werden erst später sichtbar, und
einzeln gebundene Listener müssten nachgezogen werden.

`navigator.clipboard` gibt es nur im sicheren Kontext. `127.0.0.1` zählt dazu, aber nicht
jeder Browser hält sich daran, deshalb liegt `document.execCommand("copy")` als
Rückfallebene darunter. Beide Wege melden ihr Ergebnis am Knopf zurück.

## Aufgelöster Kontext in der Oberfläche

Der unterste Block der Startseite zeigt, was `BuildContext()` liefert — dasselbe, was
das Unterkommando `context` ausgibt: Pfade, Instruktionsdateien, die effektiven Kataloge
und die Guidelines.

Er ist ein `<details>` und der einzige Block, der **nicht** beim Seitenaufbau lädt.
`/api/context` wird erst beim ersten Aufklappen gerufen und danach nicht wieder: die
Ausgabe ist lang und wird nur gebraucht, wenn jemand nachsieht. Schlägt der Aufruf fehl,
fällt die Sperre zurück, ein erneutes Aufklappen versucht es noch einmal.

`contextResponse` reicht den Kontext unverändert durch, damit die Antwort dasselbe
bedeutet wie die des Unterkommandos. Daneben steht `display` mit denselben Pfaden in
Anzeigeform — die Kürzung auf `~` braucht das Home-Verzeichnis und kann nur serverseitig
passieren. Die übrigen Pfade kürzt die Oberfläche gegen das Projektverzeichnis.

## Doku in der Oberfläche

Der Bereich **Docs** zeigt die mitgelieferte Doku aus `k-playbook/docs`. Das Menü links
listet alle Markdown-Dateien, auch die aus Unterverzeichnissen wie `libs/`; ein Klick
zeigt die Datei in der Karte daneben. Ohne Auswahl steht dort die `README.md`.

`project.ListDocs()` sammelt die Dateien und nimmt als Titel die erste Überschrift,
ersatzweise den Dateinamen. Die `README.md` steht vorn, sie ist der Einstieg. Fehlt das
Verzeichnis, ist das ein Befund und keine leere Liste: die Antwort trägt dann eine
Meldung mit dem erwarteten Pfad.

`project.ReadDoc()` liefert den Rohtext, gerendert wird erst in `webui/docs.go` mit
Goldmark (GFM, `WithAutoHeadingID`). Rohes HTML aus der Quelle bleibt abgeschaltet — das
ist die Voreinstellung von Goldmark und genau richtig für Text, der ungeprüft im
Browser landet.

Der angefragte Pfad kommt aus dem Browser und wird in `docFilePath()` geprüft: relativ,
innerhalb des Doku-Verzeichnisses, Endung `.md`. Ohne diese Prüfung wäre der Endpunkt
ein Weg, beliebige Dateien des Rechners zu lesen.

Verweise innerhalb der Doku fängt die Oberfläche ab, weil ein Klick sonst die Seite
verlassen und damit den Server hinter ihr beenden würde: `.md`-Ziele öffnet sie im
selben Fenster, Anker springen innerhalb der Datei, Ziele mit Schema gehen in einen
neuen Tab. Führt ein Verweis in eine andere Datei, zieht das Menü mit.

Der Text steht in einer Karte und ist kein eigener Scroll-Container. Ohne Anker scrollt
deshalb das **Fenster** nach oben, nicht das Element — sonst bliebe die Ansicht dort
stehen, wo die vorige Datei endete.

Mermaid-Blöcke rendert der Browser nach. Die Library kommt bei Bedarf vom CDN — sie ist
zu groß, um sie mitzuliefern. Ohne Netz bleibt der Quelltext des Diagramms als
Codeblock stehen, die Datei ist also weiterhin lesbar.

## Workflows: Reviews, Tasks und Todos

Drei Arbeitsvorräte, ein Bereich. `/workflows` stellt sie untereinander: die bisherigen
Läufe, die offenen und erledigten Tasks samt gelesenem Inhalt, die offenen und
abgehakten Todos. Ein Beschreibungsblock steht voran und sagt, was die drei Sorten sind
und wann man welche nimmt — die Listen darunter beantworten das nicht von selbst.

Jede Liste trägt ihre eigene Pill mit ihrer Zahl. Einen Aggregat-Endpunkt daneben gibt
es nicht: er wäre die Doppelung dieser drei Zahlen.

Offen heißt bei Tasks: Markdown unmittelbar in `k-playbook-local/tasks/`. Erledigte
liegen in `done/`, und die `README.md` beschreibt das Verzeichnis — beides ist keine
Aufgabe und steht nicht in der Liste der offenen.

Die drei Herkünfte hatten je ein eigenes Seitenskript, mit gleichnamigen Namen auf
Top-Level: dreimal `elements`, dreimal `load` und `render`, dazu `doneRequested` und
`doneOpenKey` in zweien. Aneinandergehängt wäre das ein SyntaxError, und dann liefe keine
Zeile. In `workflows.js` steht deshalb jede Herkunft in einer eigenen Kapsel und behält
ihre Namen bei sich. Geteilt wird nur, was zur Seite gehört und nicht zu einer ihrer
Listen: das Blockmenü, der Merkspeicher der Aufklapp-Blöcke und **ein** `startSession()`
— drei Aufrufe wären drei Intervalle und drei Klick-Listener, von denen nur der zuletzt
gesetzte Handler zählt.

`project.ListTasks()` sortiert nach Dateinamen; die Nummer steht vorn und ordnet damit
bereits richtig. Als Titel dient die erste Überschrift, ersatzweise der Dateiname —
dieselbe Regel wie bei der Doku.

Jede Zeile sagt außerdem, ob der Task schon gegengelesen wurde. Erkannt wird das an der
`## Review-Log`-Sektion, die `/k-task-refine` an **jede** geprüfte Datei anhängt, auch an
die unveränderte — dieselbe Spur, an der `/k-task-run` Step 1.2 das prüft. Nennt die
Überschrift ein Datum, steht es dabei; mehrere Runden hängen mehrere Logs an, gezeigt
wird das jüngste. Codeblöcke bleiben außen vor, sonst gälte eine zitierte Vorlage als
Nachweis. Ein Task ohne Log ist kein Fehler, aber der Grund, warum `/k-task-run` vor der
Ausführung nachfragt — deshalb steht er in Warnfarbe. Ein fehlendes Task-Verzeichnis ist anders als bei der
Doku **kein** Befund: die projekteigene Struktur wird erst angelegt, und "noch keine
Tasks" ist dieselbe Auskunft wie ein leeres Verzeichnis.

Die Liste stellt die offenen Tasks untereinander — die Zeile trägt Dateinamen
und Titel, das liest sich nebeneinander schlecht. Ein Klick zeigt den Task als Markdown
in einer Karte **unter** der Liste, nicht in einem Fenster darüber: die Liste bleibt
sichtbar, der nächste Task ist einen Klick entfernt. Gerendert wird wie bei der Doku mit
Goldmark, rohes HTML aus der Quelle bleibt abgeschaltet.

Darunter steht ein zugeklappter Block mit den **erledigten** Tasks aus `done/`. Sie
werden nie weniger, deshalb liegen sie nicht in der Hauptliste; geholt werden sie erst
beim ersten Aufklappen über `/api/tasks/done`, denn für die Liste wird jede Datei einmal
gelesen. Ob der Block offen war, merkt sich die Seite im `localStorage` — die Erledigten
sucht man meist mehrmals hintereinander. `project.ListDoneTasks()` dreht die Ordnung um:
bei Abgearbeitetem zählt der letzte Stand, die jüngste Nummer steht oben. Den
Review-Stand trägt die Zeile dort nicht — nach der Ausführung sagt er nichts mehr aus.

`taskFilePath()` prüft den angefragten Namen: eine Datei unmittelbar im Task-Verzeichnis
oder als `done/<datei>.md` eine erledigte, Endung `.md`. Erledigte sind der einzige
Grund, den flachen Vergleich mit `filepath.Base()` zu verlassen; genau dieses eine
Verzeichnis, genau eine Ebene tiefer — alles andere fällt weiterhin weg. Die Namen aus
beiden Listen sind damit dieselben, unter denen die Datei wieder angefragt wird.

## Web-API

| Methode | Pfad | Zweck |
|---|---|---|
| `GET` | `/api/health` | Heartbeat in beide Richtungen |
| `POST` | `/api/client-gone` | Browser meldet Tab-/Fenster-Schließen |
| `POST` | `/api/shutdown` | Server beenden |
| `GET` | `/api/path` | host-weite Aufrufbarkeit prüfen, read-only; kein `POST` |
| `GET` | `/api/config` | Anker suchen bzw. Ort vorschlagen |
| `POST` | `/api/config` | `K-PLAYBOOK.yaml` anlegen |
| `GET` | `/api/local` | projekteigene Struktur prüfen |
| `POST` | `/api/local` | fehlende Teile anlegen |
| `GET` | `/api/local/private` | messen, ob der Inhalt von `priv/` und `material/` privat ist |
| `POST` | `/api/local/private` | einen dieser Einträge umschalten; nur Einträge mit `Private` |
| `GET` | `/api/assistant` | Verlinkung prüfen und dabei nachziehen, was sich nachziehen lässt |
| `POST` | `/api/assistant` | Verlinkung herstellen |
| `GET` | `/api/mcp` | MCP-Registrierung der drei Assistenten prüfen |
| `POST` | `/api/mcp` | Registrierung herstellen; fremde Einträge bleiben unberührt |
| `GET` | `/api/mcp/tools` | Werkzeug-Selbsttest: startet den registrierten Wrapper als Subprozess |
| `GET` | `/api/tools` | Security-Tool-Preflight, read-only |
| `POST` | `/api/languages` | `project.languages` setzen; antwortet mit dem neuen Tool-Zustand |
| `GET` | `/api/reviews` | bisherige Läufe auflisten, read-only; angelegt wird über die Commands |
| `GET` | `/api/gh` | `tools.gh` lesen, dazu den gh-Befund dieses Rechners |
| `POST` | `/api/gh` | `tools.gh.status` setzen; installiert und meldet nichts an |
| `GET` | `/api/remediation` | `remediation:`-Block lesen |
| `POST` | `/api/remediation` | `remediation:`-Block setzen |
| `GET` | `/api/update` | per `git ls-remote` prüfen, ob die Installation zurückliegt; liefert den lokalen Sauberkeitszustand mit |
| `POST` | `/api/update` | `git pull --ff-only` ausführen; bricht bei lokal veränderter Installation vorher ab |
| `POST` | `/api/update/discard` | eingespielten Arbeitsstand verwerfen; nur bei vorhandener Markierung |
| `GET` | `/api/context` | aufgelösten Arbeitsstand lesen, read-only |
| `GET` | `/api/docs` | mitgelieferte Doku auflisten, read-only |
| `GET` | `/api/docs/file` | eine Datei daraus als HTML lesen, read-only |
| `GET` | `/api/tasks` | offene Tasks auflisten, read-only |
| `GET` | `/api/tasks/done` | erledigte Tasks aus `done/` auflisten, read-only |
| `GET` | `/api/tasks/file` | einen Task als HTML lesen, read-only |
| `GET` | `/api/todos` | offene Todos aus `TODO.md` auflisten, read-only |
| `GET` | `/api/todos/done` | abgehakte Todos auflisten, read-only |

Statische Assets liegen unter `/static/`. Die Seiten sind `/` (Setup), `/workflows`,
`/docs` und `/mcp`; alle vier rendert `renderPage()` aus derselben Vorlage für den Kopf
und die linke Spalte. Mitgeliefert werden der aktive Bereich und die Auskunft, ob eine
Installation gefunden wurde.

## Der MCP-Server

`k-playbook mcp` bietet den aufgelösten Arbeitsstand einem Assistenten als Werkzeug an,
über stdin und stdout. Es ist die dritte Fassade auf `project.BuildContext`, neben dem
Subkommando `context` und `GET /api/context`. Wer eine vierte Quelle aufmacht, bekommt
zwangsläufig eine abweichende Antwort.

Ein Werkzeug, `k_playbook_context`. Es gibt dasselbe JSON zurück wie das Subkommando —
dieselbe Serialisierung, damit sich beide Seiten überhaupt vergleichen lassen.

**Maßgeblich ist die Spec-Fassung [`2026-07-28`](https://modelcontextprotocol.io/docs/2026-07-28/learn/architecture).**
Die Clients sprechen sie allerdings noch nicht: Claude Code trägt den Pfad zwar im
Programm, aber hinter abgeschalteten Schaltern, und benutzt voreingestellt den älteren
`initialize`-Handshake mit `2025-11-25`; OpenCode kann über sein SDK ohnehin nicht mehr.
Das ist der Grund für das offizielle Go-SDK: es bedient beide Fassungen über dieselbe
API, sonst müsste hier jeder Dialekt selbst gebaut werden.

Drei Regeln gelten:

- **stdout gehört dem Protokoll.** Dort läuft der JSON-RPC-Strom, und er bleibt über die
  ganze Sitzung offen. Diagnose geht nach stderr — die Spec hat ihr Logging-Primitiv
  abgekündigt und empfiehlt für stdio genau das. Deshalb laufen im `mcp`-Zweig weder
  `cleanUpLegacy()` noch `mirrorHostInstall()`.
- **Zustand läuft über einen expliziten Bezeichner, nie über die Verbindung.** Die Spec
  verlangt das ausdrücklich, und für einen stdio-Server ist es keine Formalie: er wird
  einmal gestartet und behält das Arbeitsverzeichnis des Clients über die ganze Sitzung.
  Deshalb hat das Werkzeug den optionalen Parameter `dir` — ohne ihn könnte es nicht
  sagen, welches Projekt es überhaupt beschreibt.
- **Keine Tasks-Extension**, solange die Clients sie nicht deklarieren. Sie wäre das
  passende Muster für lange Läufe, aber Claude Code schaltet sie ab, OpenCode hat sie
  auskommentiert, und das Go-SDK hat sie nicht. Die Spec verbietet, einem Client ohne
  Deklaration einen Task zu schicken. Ein späterer Scan-Lauf bekommt deshalb gewöhnliche
  Werkzeuge nach dem Muster start/status/collect.

Das Ende einer Sitzung meldet das SDK als Fehler, obwohl es der Normalfall ist: der
Client schließt stdin, und `Run` liefert „server is closing: EOF". Ein Sentinel dafür
fehlt — der Typ liegt im `internal`-Paket des SDK, und weder `io.EOF` noch
`mcp.ErrConnectionClosed` greifen über `errors.Is`. `isSessionEnd()` prüft deshalb den
Text. Ohne diese Unterscheidung endete der Prozess bei jedem normalen Sitzungsende mit
Exit 1, was für den Client wie ein Absturz aussieht.

Geprüft wird das in `cmd/k-playbook/mcp_test.go`, und zwar gegen einen echten Prozess:
das Test-Binary ruft sich mit einem Umgebungsmarker selbst auf. In-process ließe sich die
stdout-Reinheit nicht abgreifen, und genau sie ist die Invariante — ein `fmt.Print` in
einem transitiv genutzten Paket würde die Verbindung unbrauchbar machen, ohne dass
irgendwo ein Fehler auftaucht.

### Registrierung

`project/mcp.go` ist der vierte Einrichtungspunkt neben der Verlinkung, nach demselben
Muster: `CheckMCP()` misst, `ApplyMCP()` stellt her, `MCPStatus` trägt Zustand und
Detailtext. Drei Ziele, zwei Schemata:

| Assistent | Datei | Schlüssel | Form |
|---|---|---|---|
| Claude Code | `.mcp.json` | `mcpServers` | `{"command": …, "args": ["mcp"]}` |
| Cursor | `.cursor/mcp.json` | `mcpServers` | dasselbe |
| OpenCode | `opencode.json` oder `opencode.jsonc` | `mcp` | `{"type": "local", "command": […, "mcp"], "enabled": true}` |

Bei OpenCode ist `command` ein **Array** aus Kommando und Argumenten, nicht zwei Felder.

**Zielwahl statt fester Liste.** Deshalb bekommt `MCPTargets()` den `projectRoot`: das
OpenCode-Ziel steht nicht fest, sondern folgt einer Regel. `opencode.jsonc` gilt, wenn sie
existiert und `opencode.json` fehlt — sonst `opencode.json`. Angelegt wird nie eine
zweite: OpenCode führt beide Endungen im Projekt-Root tief zusammen, und bei
kollidierenden Schlüsseln entscheidet die Merge-Reihenfolge, ein Implementierungsdetail.
Liegen trotzdem beide vor, wird nur `opencode.json` geschrieben, und der Zustand ist nie
`MCPStateOK`, sondern `MCPStateAmbiguousTarget`: was am Ende wirkt, ist von außen nicht
mehr zu sehen, und ein `ok` wäre eine Behauptung ohne Deckung.

**Hineinschreiben statt anlegen.** Anders als bei der Verlinkung, wo `ownedLinks()` das
eigene Werk wiedererkennt, gehören diese Dateien vollständig dem Projekt: sie können
fremde MCP-Server tragen, und die OpenCode-Konfiguration daneben ganz andere
Einstellungen. Gesetzt wird deshalb genau der Schlüssel `k-playbook`; alles andere bleibt
unberührt. Gelesen und geschrieben wird über `github.com/tailscale/hujson`: eine
vorhandene Datei wird als JWCC geparst und per JSON-Patch (RFC 6902) auf
`/<schema>/k-playbook` verändert, statt über `map[string]any` neu serialisiert zu werden.
Kommentare, Trailing Commas und die Schlüsselreihenfolge überleben das. Nur eine neu
entstehende Datei geht weiter über `json.MarshalIndent` — dort gibt es nichts zu erhalten.

Sichtbar bleibt eine Nebenwirkung: nach dem Patch läuft `Value.Format()` und rückt die
Datei einheitlich mit Tabs ein. Bei einer eingecheckten Konfiguration mit
Leerzeichen-Einrückung ist das ein Diff über die ganze Datei. Es passiert einmal:
`applyMCPTarget()` schreibt gar nicht, wenn der Eintrag schon gleich ist — sonst
formatierte jeder Lauf eine fremde Datei erneut um.

Der Schlüssel gehört k-playbook. Ein abweichender Wert darunter ist kein Konflikt,
sondern ein falscher Stand: `MCPStateStale` meldet ihn, `ApplyMCP()` überschreibt ihn. Als
echter, unangetasteter Fall bleibt nur `MCPStateUnreadable` — und der ist seit dem
JWCC-Parser eng: Kommentare und Trailing Commas sind lesbar, gemeldet und nicht angefasst
wird nur noch, was auch als JWCC kein JSON-Objekt ergibt (kaputte Syntax, ein Array, ein
`null`). So geht keine Handarbeit eines Projekts verloren.

**Registriert wird der projekteigene Wrapper**, `project.WrapperPath()` — relativ zum
Hauptverzeichnis, nicht die host-weite Kopie. Die wäre bequemer und trüge sogar immer den
neuesten Stand, scheitert aber am Container: dort ist `$HOME` ein anderes,
`~/.local/bin/k-playbook` existiert nicht, während das Projekt gemountet ist. Dazu kommt,
dass nur ein relativer Eintrag teilbar ist und dass der Wrapper die Plattform selbst über
`uname` wählt. `WrapperName` und `BinDirName` stehen deshalb in `internal/project`;
`hostinstall` benutzt sie von dort, die umgekehrte Richtung wäre ein Import-Zyklus.

Der Preis ist eine **Bedingung**: ein relativer Eintrag wird gegen das Arbeitsverzeichnis
des Assistenten aufgelöst, nicht gegen den Ort der Konfigurationsdatei. Er gilt nur, wenn
der Assistent im Hauptverzeichnis geöffnet ist. Das wird nicht umgangen, sondern gesagt —
im Block und in [`docs/mcp.md`](../../docs/mcp.md). Weicht `Environment.SearchedFrom` von
`Environment.ProjectDir` ab, wurde schon die Oberfläche nicht dort gestartet; dann zeigt
`GET /api/mcp` `workdirMismatch` und der Hinweis wird deutlich statt beiläufig.

**Fehlt der Wrapper**, meldet jedes Ziel `MCPStateNoWrapper` und `ApplyMCP()` schreibt
nichts. Sonst meldete ein frischer Clone ohne `k-playbook/` eine Registrierung als „steht
richtig", die auf nichts zeigt. Ein **Entfernen** gibt es nicht: die Oberfläche richtet
ein, sie räumt nicht ab.

### Die Seite /mcp

`webui/mcp.go` bedient neben `GET`/`POST /api/mcp` noch `GET /api/mcp/tools` — den
Werkzeug-Selbsttest. Er ist ein eigener Endpunkt, weil dahinter ein Subprozess steht: als
Teil von `GET /api/mcp` bremste er die Startseite aus. Aufgerufen wird er nur von
`mcp.js`, also erst beim Öffnen der Seite.

Gestartet wird der **registrierte Wrapper** mit dem Hauptverzeichnis als
Arbeitsverzeichnis, nicht der laufende Prozess: nur so misst die Seite das, was der
Assistent später bekommt, Binary-Auswahl inbegriffen. Die Mechanik ist dieselbe wie in
`cmd/k-playbook/mcp_test.go` — `initialize`, `notifications/initialized`, `tools/list`,
und stdin bleibt offen, bis die Antworten da sind.

Fehlender Wrapper, keine Antwort, kein verwertbares JSON: alles davon ist ein **Ergebnis**
der Seite, keine Störung — sie zeigt „Server antwortet nicht" samt Grund. Der Handler darf
unter keinen Umständen hängen bleiben und die Seite mitnehmen.

Genau das ist die Stelle, an der es einmal nicht gereicht hat. `exec.CommandContext` mit 10
Sekunden und ein `defer`, das den Prozess beendet, sehen nach der vollständigen Antwort aus
— sie sind es nicht. Gemessen an einem Wrapper, der ein Kindeskind hinterlässt: das Enkelkind
erbt die Rohre und lebt weiter, der abgelaufene Kontext beendet nur den Wrapper selbst, und
der Handler hing über 30 Sekunden in einem `Read`, das niemand mehr bedienen wollte. Ein Lesen
auf einem offenen Rohr lässt sich durch kein Zeitlimit unterbrechen.

Drei Dinge zusammen lösen es, und keines davon ist entbehrlich:

- Der Dialog läuft in einer eigenen Goroutine und meldet sein Ergebnis über einen gepufferten
  Kanal. Der Handler wartet mit `select` auf das Ergebnis **oder** auf `ctx.Done()` — dadurch
  greift das Zeitlimit auch dann, wenn niemand antwortet.
- `cmd.WaitDelay` begrenzt, wie lange `Wait()` danach noch auf die Rohre wartet. Ohne das
  hinge das Aufräumen an genau dem Enkelkind, dem der Handler eben entkommen ist.
- Das `defer` schließt stdin, cancelt, schließt stdout und wartet erst dann. Das Schließen ist
  das, was den Leser aus seiner Blockade löst; sich darauf zu verlassen, dass der Prozess von
  selbst geht, wäre die Wette, die schon einmal verloren ging.

Wer hier vereinfacht, bekommt einen Handler zurück, der im Normalfall funktioniert und im
Fehlerfall die Seite mitnimmt — also in dem Fall, für den er gebaut ist.

## Lebenszyklus

Der Server bindet auf `127.0.0.1:0`, nimmt also einen freien Port, und gibt die URL im
Terminal aus.

Beenden funktioniert in beide Richtungen:

- Der Client ruft alle paar Sekunden `/api/health`. Bleibt er länger als 5 Sekunden aus,
  ist das Browserfenster weg und der Server beendet sich.
- Beim Schließen meldet der Browser `/api/client-gone` per `sendBeacon`. Danach wartet
  der Server 3 Sekunden, damit ein Reload ihn nicht abräumt.
- Solange sich **nie** ein Client gemeldet hat, bleibt der Server stehen: der Browser kann
  noch unterwegs sein, oder die URL wird von Hand eingetragen.
- `Ctrl+C` und der `Schließen`-Button gehen ebenfalls.

Weil die Oberfläche mehrere Seiten hat, gilt das für **jede** von ihnen: `session.js`
trägt Heartbeat und Abmeldung und wird von `index.html`, `workflows.html`, `docs.html`
und `mcp.html` vor `nav.js` und der jeweiligen Seitenlogik geladen. Eine Seite ohne
Lebenszeichen beendete den Server wenige Sekunden nach dem Wechsel zu ihr — der Weg
zurück führte dann auf eine tote Seite. Ein Bereichswechsel ist genau so ein Wechsel.

Ein Klick auf einen Verweis dieser Oberfläche meldet gar nicht erst ab: `session.js`
merkt sich, dass das Fenster zur nächsten eigenen Seite geht. Der Zurück-Knopf löst
keinen Klick aus und meldet deshalb weiterhin ab — die nächste Seite ist innerhalb der
3 Sekunden da und hebt die Abmeldung mit ihrem ersten Lebenszeichen wieder auf.

Der Browser wird nicht automatisch geschlossen, wenn der Server endet. Browser blockieren
das in vielen Fällen, und `open`/`xdg-open` liefern keinen verlässlichen Tab-Handle.

### Browser öffnen

`announce()` in `webui/server.go` gibt die URL aus und öffnet den Browser. Die Kandidaten
liefert `browserOpeners()` in `webui/browser.go`, in dieser Reihenfolge:

1. **`$BROWSER`**, zerlegt von `envOpeners()` nach der freedesktop-Konvention: `:`-getrennte
   Liste, `%s` als Platzhalter für die URL, sonst wird sie angehängt.
2. Die geratenen Kandidaten der Plattform — `open` auf macOS, sonst `wslview`, `xdg-open`,
   `gio open` und weitere bis hin zu `powershell.exe Start-Process` als letztem Ausweg in
   WSL ohne `wslu`.

Probiert wird der Reihe nach; wer startet, ohne binnen `openerGrace` mit Fehler
zurückzukommen, gilt als Erfolg.

**Im Container fallen die geratenen Kandidaten weg.** Meldet `containerMarker()` einen
Container, bleiben nur die aus `$BROWSER`. Der Grund ist nicht nur, dass ein Browser im
Container am Nutzer vorbeiliefe: `x-www-browser` und `sensible-browser` zeigen in
schlanken Images gern auf einen Terminal-Browser. Der startet erfolgreich, überlebt die
Wartezeit und übernimmt danach das Terminal.

Ein ausdrücklich gesetzter `$BROWSER` ist dagegen kein Ratespiel — im DevContainer von VS
Code zeigt er auf einen Helfer, der `code --openExternal` aufruft und die URL damit an den
Host durchreicht. Denselben Weg nimmt `gh auth login`. Ist die Variable im Container nicht
gesetzt, bleibt keine Kandidatenliste übrig, und das Terminal nennt Marker, URL und den
Hinweis auf die Port-Weiterleitung.

## Designentscheidungen

- Go ist die einzige Runtime. Keine Node-Toolchain, kein Build-Schritt für das Frontend.
- Die Oberfläche ist eine lokale Web-UI, keine native App und kein Electron/Wails-Setup.
- Assets sind per `embed` im Binary. Ein Binary, keine Begleitdateien.
- Keine Shell-Pipelines. Fachlogik ruft Go-Funktionen; Fremdprozesse sind nur das
  Security-Tool-Skript und `git`.
- Kein YAML-Parser. Zeilenweises Lesen erhält Kommentare und unbekannte Blöcke.
- Die Overlay-Regel liegt im Code, nicht in der Prosa. `context` gibt eine Antwort, und
  alle Commands bekommen dieselbe.
- Abgeschaltet wird über eine leere Datei, nicht über eine Liste in der Konfiguration.
  Ein Mechanismus statt zweier.
- `git status --porcelain` und `git rev-list --count @{u}..HEAD` für den lokalen Zustand.
- `git ls-remote` zum Prüfen, `git pull --ff-only` zum Holen. Nichts, was den Zustand des
  Clones ungefragt verändert.
- Kein Projekt-Store. Es gibt keine Liste bekannter Projekte mehr — das Werkzeug arbeitet
  auf dem Projekt, in dem es liegt.
- Geschrieben wird ausschließlich nach Bestätigung, Schritt für Schritt.
- Die Binaries sind Release-Assets, nicht versionierte Dateien. „Go wird nicht
  gebraucht" gilt weiter, „ohne Netz" nicht mehr: der erste Start lädt genau ein Binary.
- `SHA256SUMS` liegt versioniert im Repo. Die erwartete Prüfsumme kommt damit über den
  Git-Remote und nicht über dieselbe HTTPS-Quelle wie das Binary.

## Abhängigkeiten

Eine: `github.com/yuin/goldmark` für die Doku-Anzeige. Cobra und die
Charmbracelet-Pakete des alten Stands sind mit ihm entfallen; alles übrige kommt aus
der Standardbibliothek.

Goldmark ist reines Go und wird statisch mit einkompiliert — das Binary wächst um rund
1,6 MB, zur Laufzeit wird nichts nachgeladen. Ein Build ohne Netz braucht das Modul im
lokalen Cache; `go mod vendor` wäre die Alternative, ist aber bewusst nicht gesetzt.

## Debuggen

Wenn ein laufendes Werkzeug eine URL wie `http://127.0.0.1:34531/` ausgibt:

```bash
curl -sS http://127.0.0.1:34531/api/config
curl -sS http://127.0.0.1:34531/api/local
curl -sS http://127.0.0.1:34531/api/assistant
curl -sS http://127.0.0.1:34531/api/docs
curl -sS -X POST http://127.0.0.1:34531/api/shutdown
```

`/api/config` ist der erste Check: er sagt, ob ein Anker gefunden wurde oder ob die
Oberfläche im Vorschlagsmodus läuft.

Ein laufender Server nutzt weiter den Code im Speicher. Nach Backend- oder
Asset-Änderungen neu starten.

## Der alte Stand

`installer/_old/` enthält rund 7800 Zeilen Go vor dem Umbau, per `git mv` verschoben und
damit in der Historie nachvollziehbar. Verzeichnisse mit `_`-Präfix ignoriert die
Go-Toolchain vollständig: kein Build, keine Tests, keine Imports. Der Code ist dort
**nicht baubar** — seine Imports zeigen auf Pfade, die es nicht mehr gibt. Er ist zum
Lesen und Herüberkopieren da.

| Ort | Inhalt |
|---|---|
| `_old/internal/install/config.go` | vollständiger Config-Vertrag: `PathKeys`, `ValidatePath`, `RenderConfig` |
| `_old/internal/install/migrate.go` | zeilenweise Migration, die Kommentare und unbekannte Blöcke erhält |
| `_old/internal/projects/status.go` | die elf Statusprüfungen der alten Oberfläche |
| `_old/internal/webui/server.go` | die alte GUI mit allen Bedienabläufen |
| `_old/payload/payload.go` | `go:embed` samt Extract-Logik des abgelösten Modells |

## Offene Punkte

- Was der Projektstatus in der Oberfläche zeigen soll, nachdem der Projekt-Store
  entfallen ist. Der Slash-Command `/k-status` ist gelöscht: sein Kern — die Prüfung, ob
  die Installation unverändert ist — war eine Nachbildung von `CheckCleanliness()` in
  Prosa, der Rest billige Existenzprüfungen auf der `context`-Ausgabe. Der Zustand kommt
  jetzt aus der Oberfläche.
- Kaum automatisierte Tests für die HTTP-Handler; getestet ist im Wesentlichen
  `internal/project`. Ausnahme sind die schreibenden Endpunkte unter
  `/api/local/private` — dort hängt an der Whitelist eine git-Operation, deshalb steht
  sie in `webui/local_private_test.go` unter Test.
- Release-Artefakte gibt es für macOS und Linux. Windows ist offen.
