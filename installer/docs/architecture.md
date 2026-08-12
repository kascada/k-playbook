# Architektur

Session-Memory für die Arbeit am Werkzeug unter `installer/`. Diese Datei zuerst lesen,
bevor der Code erneut analysiert wird.

## Ziel

`k-playbook` hat zwei Aufgaben. Es **richtet ein**: Anker finden, Konfiguration und
projekteigene Struktur anlegen, Assistenten verlinken. Und es **beantwortet, was gilt**:
`context` löst Verzeichnisse, Instruktionen und Kataloge auf, damit kein Command das
selbst tun muss.

Alles Weitere — Reviews, Tasks, Checks — machen Commands und Skills im Assistenten, nicht
dieses Programm.

Das Werkzeug ist ein eigenständiges Go-Modul unter `installer/`, wird aber als
`bin/k-playbook` aus dem Repo-Root heraus aufgerufen.

## Zwei Einstiege

```go
if len(args) == 0 {
    cleanUpLegacy()
    return webui.Run()
}
```

Ohne Argument die Oberfläche, davor das Aufräumen der Altlasten. Mit `context` die
JSON-Ausgabe — und dort **ohne** `cleanUpLegacy()`: dessen Meldungen würden das JSON
stören.

Mehr Subkommandos gibt es nicht. `init`, `update`, `restore`, `migrate`, `status`,
`smoke` und `projects …` des alten Stands sind entfallen, samt der lokalen Projektliste
unter `.k-playbook-local/projects.json`.

## Aufbau

```text
installer/
├── cmd/k-playbook/main.go       räumt Altlasten weg, startet webui.Run()
├── internal/legacy/
│   └── global.go                host-globale Registrierung des alten Modells entfernen
├── internal/hostinstall/
│   └── mirror.go                host-weite Kopie spiegeln, verlinken, PATH prüfen
├── internal/project/
│   ├── discover.go              Anker finden
│   ├── environment.go           was liegt hier vor
│   ├── config.go                Config lesen, Ort vorschlagen, anlegen
│   ├── local.go                 projekteigene Struktur prüfen und anlegen
│   ├── registry.go              Commands und Skills aus beiden Quellen auflösen
│   ├── links.go                 Assistenten-Verlinkung prüfen und herstellen
│   ├── remediation.go           remediation:-Block lesen und setzen
│   ├── context.go               Arbeitsstand auflösen: Pfade, Kataloge, Instruktionen
│   ├── instructions.go          AGENTS.md im Hauptverzeichnis prüfen und ergänzen
│   ├── gh.go                    tools.gh lesen und setzen, gh-Befund dieses Rechners
│   ├── update.go                Remote-Stand prüfen, Sauberkeit, Fast-Forward
│   ├── docs.go                  mitgelieferte Doku auflisten und lesen
│   └── tools.go                 Security-Tool-Preflight über das Skript
├── internal/webui/
│   ├── server.go                Routen, Lebenszyklus
│   ├── browser.go               Browser öffnen, Container erkennen
│   ├── docs.go                  Doku-Endpunkte, Markdown nach HTML
│   ├── hostpath.go              PATH-Zustand melden, read-only
│   ├── config.go local.go assistant.go tools.go remediation.go context.go
│   ├── gh.go update.go
│   └── static/                  index.html, app.js, styles.css
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details. Die
Trennung hält die Fachlogik testbar.

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
2. **Aus dem Ort des Binaries.** Es liegt in `<X>/dist/`, also ist `X` die Installation.
   Ob `X` selbst das Hauptverzeichnis ist oder eine Ebene darunter liegt, hängt daran, ob
   die Installation geklont wurde oder das Repo selbst ist — beides kommt vor, deshalb
   stehen beide Orte zur Auswahl.
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

## Projekteigene Struktur

`LocalStructure()` in `project/local.go` ist die einzige Quelle für das, was unter
`k-playbook-local/` liegt:

```text
rules/  reviews/  checks/  commands/  skills/  results/  docs/  guidelines/
tasks/  tasks/done/  priv/  k-playbook.md  TODO.md
```

Jedes Verzeichnis bekommt eine `README.md` mit seinem Zweck — **auch weil Git leere
Verzeichnisse nicht speichert** und sie sonst nach einem Clone des Projekts fehlen
würden. `priv/` bekommt zusätzlich eine eigene `.gitignore`, die den Inhalt ausschließt
und das Verzeichnis selbst versioniert lässt; so muss die Projekt-`.gitignore` nicht
angefasst werden.

`writeIfMissing()` schreibt nur, wenn nichts da ist. Vorhandene READMEs mit eigenem Text
bleiben unberührt.

`commands/` und `skills/` sind darin die zwei Sorten, die ein Assistent direkt liest.
Wie sie mit den mitgelieferten verrechnet werden, steht im nächsten Abschnitt.

## Commands und Skills auflösen

`project/registry.go` führt zusammen, was in `k-playbook/` mitgeliefert wird und was
unter `k-playbook-local/` dazukommt. Es ist dieselbe Overlay-Regel wie bei rules, reviews
und checks: **gleicher Name gewinnt projekteigen, ein leerer Eintrag schaltet ab.**

| Sorte | Einheit | Schlüssel | Abschalten durch |
|---|---|---|---|
| `commands` | eine `*.md`-Datei | Pfad ab `commands/`, z. B. `_shared/context.md` | leere Datei |
| `skills` | ein Verzeichnis mit `SKILL.md` | Verzeichnisname | leere `SKILL.md` |

Commands werden **rekursiv** aufgelöst. Namensraum-Verzeichnisse wie `_shared/` und
`_details/` sind damit bis auf die einzelne Datei überlagerbar: ein Projekt ersetzt
`_shared/context.md` und behält den Rest des Namensraums aus der Installation.

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

Deshalb läuft `ApplyRootInstructions()` **vor** `ApplyLinks()`: der Symlink braucht
`AGENTS.md` als Ziel. Siehe [Instruktionen](#instruktionen).

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
auflösen.

### Die Installation muss sauber sein

`CheckCleanliness()` liest bei jeder Prüfung den lokalen Zustand des Clones mit — rein
lokal, ohne Netz, deshalb billig genug für den ungefragten Lauf nach dem Start.

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

Die Oberfläche zeigt den Befund in einer eigenen Karte, weil dort Dateinamen hinmüssen
— der Update-Button hat nur Platz für einen Zustand. Bewusst **ohne** Knopf zum
Zurücksetzen: das wäre `git checkout -- .` in einem fremden Verzeichnis, und die
Oberfläche kann nicht wissen, ob dort jemand absichtlich entwickelt. Der Befehl steht
zum Kopieren da.

Denselben Befund meldet `/k-status` in der Zeile `Installation:`. Das fängt den Fall ab,
in dem ausgerechnet `bin/k-playbook` die veränderte Datei ist: dann ist die Oberfläche
über den Wrapper gar nicht erreichbar.

Vor und nach dem Pull werden die Dateien unter `dist/` per SHA-256 gehasht.
`BinaryChanged` meldet, ob sich etwas geändert hat — **nur dann** bringt ein Neustart
eine andere Programmversion. Unter Linux behält der laufende Prozess seinen Inode und
arbeitet mit dem alten Code weiter, auch wenn die Datei ersetzt wurde.

### Die Verlinkung wird mitgezogen

`relinkAfterUpdate()` in `webui/update.go` ruft nach erfolgreichem Pull `ApplyLinks()`
auf. Das ist kein Komfort, sondern nötig: seit Commands und Skills **einzeln** verlinkt
werden, kommt ein neu mitgelieferter Command nicht mehr von selbst an. Ein
Verzeichnis-Symlink hatte das automatisch getan; ein Update, das den Katalog ändert, ihn
aber nicht registriert, wäre halb erledigt — und zwar unsichtbar.

`PendingLinkChanges()` liest vorher die Bilanz und meldet sie: dazugekommen, entfernt,
auf eine andere Quelle umgesetzt. Die Namen werden **über alle Ziele zusammengefasst**,
sonst zählte ein einzelner neuer Command dreifach — er steht in `.claude/`, `.opencode/`
und `.cursor/`.

Schlägt das Nachziehen fehl, bleibt das Update gültig: der Pull ist durch, und die
Verlinkung lässt sich über die Assistenten-Karte nachholen.

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
    │   └── dist/
    │       ├── k-playbook-darwin-arm64    vom Mac-Host gespiegelt
    │       ├── k-playbook-darwin-arm64.stamp
    │       ├── k-playbook-linux-arm64     vom Container gespiegelt
    │       └── k-playbook-linux-arm64.stamp
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

### Wrapper und Binary, nicht das Binary allein

Nahe liegt, nur das fertige Binary zu kopieren: die Plattform ist beim Kopieren ja bereits
entschieden, ein Wrapper wäre dann überflüssig. Das greift zu kurz. Auf einem Mac mit
DevContainer ist `~/.local/bin` derselbe Pfad, aber Host und Container brauchen
verschiedene Plattformen. Ist `~/.local` per `mounts` geteilt, treffen beide auf dieselbe
Datei.

Deshalb wird der Wrapper mitkopiert und die Struktur `<X>/bin/` neben `<X>/dist/`
eingehalten — die Plattformwahl bleibt damit zur Laufzeit, wo sie hingehört. Der Wrapper
löst seine Symlink-Kette selbst auf und leitet `dist/` aus seinem **aufgelösten** Ort
ab; er braucht für den Symlink in `~/.local/bin` keine Anpassung.

### Commit-Stand statt mtime

Verglichen wird der Zeitpunkt des letzten Commits, der `dist/` angefasst hat:
`git log -1 --format=%ct -- dist`. Der Wert landet als `<plattform>.stamp` neben dem
gespiegelten Binary.

Die mtime der Dateien wäre das naheliegende Kriterium und ist trotzdem falsch: Git setzt
sie beim Auschecken auf den Zeitpunkt des Clones, nicht des Commits. Ein frisch geklonter
alter Stand sähe damit neuer aus als eine korrekte Installation und würde sie
überschreiben — bei mehreren Projekten mit je eigenem Clone ständig.

Ist die Quelle kein Git-Repository, bleibt der Stempel leer und es wird nur gespiegelt,
wenn im Ziel etwas fehlt. Ein unbekannter Stand darf einen bekannten nicht verdrängen.

### Stempel pro Plattform

Der Stempel gilt bewusst je Plattformdatei, nicht für die Installation als Ganzes.
Spiegelt der Mac zuerst, steht dort sein Stand und nur `darwin-arm64`. Startet danach der
Container aus demselben Clone, wäre ein gemeinsamer Stempel gleich — und `linux-arm64`
fehlte dauerhaft. Kopiert wird deshalb auch, wenn das **eigene** Binary im Ziel fehlt,
unabhängig vom Stand.

So wachsen genau die Plattformen zusammen, von denen aus tatsächlich gestartet wurde,
statt alle vier Artefakte mit ihren rund 42 MB zu kopieren.

### Kein Sonderfall DevContainer

Im Container ist `~/.local/bin` ein **anderes** Verzeichnis: der Benutzer ist `vscode`
oder `root`, gemountet wird standardmäßig nur der Workspace nach `/workspaces/<name>`,
nicht das Home. Die Spiegelung läuft dort deshalb ganz normal und erzeugt eine
container-eigene Kopie. Nach einem Rebuild ist sie weg und wird vom nächsten Start
wiederhergestellt — genau der Vorzug gegenüber dem alten Symlink.

`containerMarker()` in `webui/browser.go` bleibt davon unberührt und dient weiterhin
allein dazu, den Browserstart zu unterdrücken.

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

Das ist Absicht. In einem Zielprojekt liegt das Binary in `<projekt>/k-playbook/dist/`,
also *innerhalb* der Installation — „neben dem Binary" und „die Installation" fallen dort
zusammen. Für die host-weite Kopie wäre „neben dem Binary" sogar falsch: dort liegen nur
Wrapper und Binary, kein `scripts/`, keine `rules/`.

| Ort | Was | Wird aktualisiert durch |
|---|---|---|
| `<projekt>/k-playbook/bin|dist/` | Installation | `git pull` im Clone, also „Update prüfen" |
| `~/.local/share/k-playbook/installation/bin|dist/` | host-weite Kopie | `Mirror()` bei jedem Start — mit Einschränkung, siehe unten |
| `~/.local/bin/k-playbook` | Symlink auf den Wrapper der Kopie | `Mirror()` |
| `<entwicklungsrepo>/dist/` | Build des Arbeitsstands | `make dist` |

**Die host-weite Kopie erneuert sich nicht beim lokalen Bauen.** `needsCopy()` vergleicht
den Commit-Zeitpunkt des letzten Commits, der `dist/` angefasst hat — bewusst nicht die
Dateizeit, weil Git die beim Auschecken auf den Zeitpunkt des Clones setzt. `make dist`
ändert diesen Stempel nicht, die Kopie bleibt also stehen.

Daraus folgt für den Aufruf:

| Start | Binary aus | Dateien aus | kann driften |
|---|---|---|---|
| `<projekt>/k-playbook/bin/k-playbook` | dem Clone | dem Clone | nein |
| `k-playbook` aus dem `PATH` | der host-weiten Kopie | der Installation des aktuellen Projekts | ja |
| `make installer-run` im Entwicklungsrepo | dem Arbeitsstand | siehe „Entwicklungsstand" | ohne Sync ja |

### Entwicklungsstand

Im Entwicklungsrepo fallen Quelle und Installation auseinander: `~/dev/k-playbook/` ist
der Arbeitsstand, `~/dev/k-playbook/k-playbook/` ein eigener Clone auf dem zuletzt
gepushten Commit. Ein frisch gebautes Binary läse also weiterhin alte Dateien.

`make installer-sync` spielt deshalb den verfolgten Dateisatz — `git ls-files`, per
Definition das, was ein Clone enthält — in die Installation ein und legt dort
`.k-playbook-devsync` ab. `make installer-run` tut das mit, `make installer-reset` stellt
den unberührten Clone wieder her.

Die Markierung ist nötig, weil Git die eingespielten Dateien zwangsläufig als Änderungen
sieht. Verbergen lässt sich das nicht: `.git/info/exclude` wirkt nur auf Unverfolgtes,
`--assume-unchanged` ist unverbindlich, und `--skip-worktree` bricht den Checkout. Also
wird der Zustand benannt statt versteckt — `CheckCleanliness()` prüft die Markierung vor
dem `git status` und meldet `DevSync` statt einer Liste einzelner Dateien. `Blocking()`
bleibt trotzdem wahr: ein Pull in einen eingespielten Stand wäre falsch.

Ohne diese Unterscheidung stünde in der Installations-Karte dauerhaft „lokal gearbeitet" —
also genau der Alarm, der echte Handarbeit im Clone melden soll, und der damit wertlos
würde.

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
sind. Installiert wird bewusst im Terminal, weil das den Host verändert und nicht das
Projekt. Ein Timeout von 30 Sekunden begrenzt den Aufruf, weil der Preflight je Tool ein
`--version` startet und eines davon hängen kann.

Bricht das Skript ab — etwa bei aktivem Projekt-venv — landet die erste stderr-Zeile in
der Fehlermeldung.

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

Über dem Kontext-Block steht die mitgelieferte Doku aus `k-playbook/docs`. Die Karte
listet alle Markdown-Dateien, auch die aus Unterverzeichnissen wie `libs/`; ein Klick
öffnet die Datei in einem Fenster über der Seite.

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
neuen Tab.

Mermaid-Blöcke rendert der Browser nach. Die Library kommt bei Bedarf vom CDN — sie ist
zu groß, um sie mitzuliefern. Ohne Netz bleibt der Quelltext des Diagramms als
Codeblock stehen, die Datei ist also weiterhin lesbar.

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
| `GET` | `/api/assistant` | Verlinkung prüfen |
| `POST` | `/api/assistant` | Verlinkung herstellen |
| `GET` | `/api/tools` | Security-Tool-Preflight, read-only |
| `POST` | `/api/languages` | `project.languages` setzen; antwortet mit dem neuen Tool-Zustand |
| `GET` | `/api/reviews` | Läufe auflisten, dazu die wählbaren Werkzeuge und Rezepte |
| `POST` | `/api/reviews` | Lauf anlegen; startet nichts |
| `GET` | `/api/gh` | `tools.gh` lesen, dazu den gh-Befund dieses Rechners |
| `POST` | `/api/gh` | `tools.gh.status` setzen; installiert und meldet nichts an |
| `GET` | `/api/remediation` | `remediation:`-Block lesen |
| `POST` | `/api/remediation` | `remediation:`-Block setzen |
| `GET` | `/api/update` | per `git ls-remote` prüfen, ob die Installation zurückliegt; liefert den lokalen Sauberkeitszustand mit |
| `POST` | `/api/update` | `git pull --ff-only` ausführen; bricht bei lokal veränderter Installation vorher ab |
| `GET` | `/api/context` | aufgelösten Arbeitsstand lesen, read-only |
| `GET` | `/api/docs` | mitgelieferte Doku auflisten, read-only |
| `GET` | `/api/docs/file` | eine Datei daraus als HTML lesen, read-only |

Statische Assets liegen unter `/static/`, die Startseite unter `/`. `indexHandler`
rendert `static/index.html` als Template und liefert vorab mit, ob eine Installation
gefunden wurde.

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

Der Browser wird nicht automatisch geschlossen, wenn der Server endet. Browser blockieren
das in vielen Fällen, und `open`/`xdg-open` liefern keinen verlässlichen Tab-Handle.

In einem Container wird der Browser gar nicht erst geöffnet; `containerMarker()` erkennt
das und weist auf die Port-Weiterleitung hin.

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
- `dist/` wird mitversioniert, damit die Installation ohne Go auskommt.

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
  entfallen ist. Daran hängt, was von `/k-status` ins Binary zurückwandert: der Bericht
  steht derzeit auf der `context`-Ausgabe plus billigen Existenzprüfungen, weil das alte
  Subkommando `status` entfallen ist.
- Keine automatisierten Tests für die HTTP-Handler; getestet ist bisher nur
  `internal/project`.
- Release-Artefakte gibt es für macOS und Linux. Windows ist offen.
