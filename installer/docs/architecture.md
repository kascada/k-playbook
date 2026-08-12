# Architektur

Session-Memory fuer die Arbeit am Werkzeug unter `installer/`. Diese Datei zuerst lesen,
bevor der Code erneut analysiert wird.

## Ziel

`k-playbook` hat zwei Aufgaben. Es **richtet ein**: Anker finden, Konfiguration und
projekteigene Struktur anlegen, Assistenten verlinken. Und es **beantwortet, was gilt**:
`context` loest Verzeichnisse, Instruktionen und Kataloge auf, damit kein Command das
selbst tun muss.

Alles Weitere — Reviews, Tasks, Checks — machen Commands und Skills im Assistenten, nicht
dieses Programm.

Das Werkzeug ist ein eigenstaendiges Go-Modul unter `installer/`, wird aber als
`bin/k-playbook` aus dem Repo-Root heraus aufgerufen.

## Zwei Einstiege

```go
if len(args) == 0 {
    cleanUpLegacy()
    return webui.Run()
}
```

Ohne Argument die Oberflaeche, davor das Aufraeumen der Altlasten. Mit `context` die
JSON-Ausgabe — und dort **ohne** `cleanUpLegacy()`: dessen Meldungen wuerden das JSON
stoeren.

Mehr Subkommandos gibt es nicht. `init`, `update`, `restore`, `migrate`, `status`,
`smoke` und `projects …` des alten Stands sind entfallen, samt der lokalen Projektliste
unter `.k-playbook-local/projects.json`.

## Aufbau

```text
installer/
├── cmd/k-playbook/main.go       raeumt Altlasten weg, startet webui.Run()
├── internal/legacy/
│   └── global.go                host-globale Registrierung des alten Modells entfernen
├── internal/hostinstall/
│   └── mirror.go                host-weite Kopie spiegeln, verlinken, PATH pruefen
├── internal/project/
│   ├── discover.go              Anker finden
│   ├── environment.go           was liegt hier vor
│   ├── config.go                Config lesen, Ort vorschlagen, anlegen
│   ├── local.go                 projekteigene Struktur pruefen und anlegen
│   ├── registry.go              Commands und Skills aus beiden Quellen aufloesen
│   ├── links.go                 Assistenten-Verlinkung pruefen und herstellen
│   ├── remediation.go           remediation:-Block lesen und setzen
│   ├── context.go               Arbeitsstand aufloesen: Pfade, Kataloge, Instruktionen
│   ├── instructions.go          AGENTS.md im Hauptverzeichnis pruefen und ergaenzen
│   ├── gh.go                    tools.gh lesen und setzen, gh-Befund dieses Rechners
│   ├── update.go                Remote-Stand pruefen, Sauberkeit, Fast-Forward
│   ├── docs.go                  mitgelieferte Doku auflisten und lesen
│   └── tools.go                 Security-Tool-Preflight ueber das Skript
├── internal/webui/
│   ├── server.go                Routen, Lebenszyklus
│   ├── browser.go               Browser oeffnen, Container erkennen
│   ├── docs.go                  Doku-Endpunkte, Markdown nach HTML
│   ├── hostpath.go              PATH-Zustand melden, read-only
│   ├── config.go local.go assistant.go tools.go remediation.go context.go
│   ├── gh.go update.go
│   └── static/                  index.html, app.js, styles.css
├── go.mod
└── README.md
```

`internal/project` kennt kein HTTP, `internal/webui` keine Dateisystem-Details. Die
Trennung haelt die Fachlogik testbar.

## Anker finden

Der zentrale Mechanismus, in `project/discover.go`:

1. Ab `startDir` aufwaerts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
2. Fund: `ProjectDir = <dir>`, `PlaybookDir = <dir>/k-playbook`.
3. Grenze sind `$HOME` und `/`, jeweils einschliesslich.
4. Nichts gefunden: `ErrNotFound`. Nicht raten, nichts anlegen.

**Die Suche darf nicht am Git-Worktree-Root abbrechen.** `<projekt>/k-playbook/` ist ein
eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, kaeme sonst nie an
die Konfiguration eine Ebene darueber. Das ist der haeufigste Fall, nicht der Sonderfall.

`homeDir()` loest Symlinks auf, damit der Vergleich auch bei verlinktem `$HOME` greift.

Es gibt **keine Fallunterscheidung** zwischen Entwicklungsrepo und Zielprojekt. Beide
tragen ihre eigene `K-PLAYBOOK.yaml` und werden gleich behandelt. Der einzige Unterschied
ist installiert oder nicht — siehe `Environment` in `project/environment.go`.

Fuer das Entwicklungsrepo heisst das: der Anker ist `<repo>/K-PLAYBOOK.yaml`, die
Installation liegt darunter in `<repo>/k-playbook/` und ist ein eigener Clone. Es gibt das
Playbook dort also zweimal — den Arbeitsstand im Repo und die installierte Fassung
daneben. Dass beide auseinanderlaufen koennen, wird bewusst in Kauf genommen; gearbeitet
wird am Repo-Stand, aufgeloest wird gegen die Installation.

## Ort vorschlagen, wenn nichts gefunden wurde

Nach einem `git clone` existiert noch keine Config, die Suche schlaegt also fehl. `Suggest()`
in `project/config.go` **darf** raten, anders als `Discover` — geschrieben wird ohnehin
erst nach Bestaetigung. Kandidaten in dieser Reihenfolge:

1. **Das Git-Repository, in dem der Aufruf stattfindet.** Wer das Werkzeug startet, steht
   in aller Regel in dem Projekt, das er meint. Der staerkste Hinweis.
2. **Aus dem Ort des Binaries.** Es liegt in `<X>/dist/`, also ist `X` die Installation.
   Ob `X` selbst das Hauptverzeichnis ist oder eine Ebene darunter liegt, haengt daran, ob
   die Installation geklont wurde oder das Repo selbst ist — beides kommt vor, deshalb
   stehen beide Orte zur Auswahl.
3. Das Arbeitsverzeichnis.

Ein frueherer Ansatz leitete das Hauptverzeichnis allein aus dem Binary-Pfad ab und
pruefte, ob das Zwischenverzeichnis `k-playbook` heisst. Das ging im Entwicklungsrepo
schief: dort heisst das Hauptverzeichnis selbst so, und der Vorschlag landete eine Ebene
zu hoch.

`suggestRepoRoot()` beantwortet eine andere Frage — nicht wo das Hauptverzeichnis `A`
liegt, sondern was in `project.repo_root` gehoert:

| Situation | Hauptverzeichnis | `repo_root` |
|---|---|---|
| `A/.git` vorhanden | `A` | `.` |
| `A/G/.git`, `A` selbst ohne `.git` | `A` | `G` |
| mehrere Kandidaten unter `A` | `A` | leer, der Nutzer waehlt |

## Config lesen ohne YAML-Parser

`ReadConfig()` liest zeilenweise statt mit einem YAML-Parser. Gelesen werden nur wenige
Skalare — `schema_version`, `project.repo_root`, `project.vcs` — und der zeilenweise
Zugriff laesst Kommentare, Reihenfolge und unbekannte Bloecke unangetastet, wenn spaeter
zurueckgeschrieben wird. Die Datei gehoert dem Projekt und kann Werte tragen, die das
Werkzeug nicht kennt.

`CreateConfig()` schreibt nie ueber eine vorhandene Datei. `schema_version` ist `3`.

## Projekteigene Struktur

`LocalStructure()` in `project/local.go` ist die einzige Quelle fuer das, was unter
`k-playbook-local/` liegt:

```text
rules/  reviews/  checks/  commands/  skills/  results/  docs/  guidelines/
tasks/  tasks/done/  priv/  k-playbook.md  TODO.md
```

Jedes Verzeichnis bekommt eine `README.md` mit seinem Zweck — **auch weil Git leere
Verzeichnisse nicht speichert** und sie sonst nach einem Clone des Projekts fehlen
wuerden. `priv/` bekommt zusaetzlich eine eigene `.gitignore`, die den Inhalt ausschliesst
und das Verzeichnis selbst versioniert laesst; so muss die Projekt-`.gitignore` nicht
angefasst werden.

`writeIfMissing()` schreibt nur, wenn nichts da ist. Vorhandene READMEs mit eigenem Text
bleiben unberuehrt.

`commands/` und `skills/` sind darin die zwei Sorten, die ein Assistent direkt liest.
Wie sie mit den mitgelieferten verrechnet werden, steht im naechsten Abschnitt.

## Commands und Skills aufloesen

`project/registry.go` fuehrt zusammen, was in `k-playbook/` mitgeliefert wird und was
unter `k-playbook-local/` dazukommt. Es ist dieselbe Overlay-Regel wie bei rules, reviews
und checks: **gleicher Name gewinnt projekteigen, ein leerer Eintrag schaltet ab.**

| Sorte | Einheit | Schluessel | Abschalten durch |
|---|---|---|---|
| `commands` | eine `*.md`-Datei | Pfad ab `commands/`, z. B. `_shared/context.md` | leere Datei |
| `skills` | ein Verzeichnis mit `SKILL.md` | Verzeichnisname | leere `SKILL.md` |

Commands werden **rekursiv** aufgeloest. Namensraum-Verzeichnisse wie `_shared/` und
`_details/` sind damit bis auf die einzelne Datei ueberlagerbar: ein Projekt ersetzt
`_shared/context.md` und behaelt den Rest des Namensraums aus der Installation.

Skills werden als Ganzes ueberlagert, nicht Datei fuer Datei. `SKILL.md`, `PLAYBOOK.md`
und `vorlagen/` muessen zueinander passen; ein halb ersetzter Skill waere nicht sinnvoll
zusammensetzbar.

`ResolveRegistry()` liefert alle Eintraege mit ihrer Herkunft (`dist`, `local`,
`override`) einschliesslich der abgeschalteten — damit die Oberflaeche sie zeigen kann.
`ActiveRegistry()` liefert nur die, die tatsaechlich registriert gehoeren.

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
`.claude/skills/`, ein zweiter Ort waere Dopplung. Cursor kennt kein Skill-Konzept.

`CLAUDE.md` zeigt auf `AGENTS.md`, weil Claude Code ausschliesslich `CLAUDE.md` liest und
OpenCode `AGENTS.md` bevorzugt. Ein Symlink statt eines Imports, damit eine Aenderung
immer in beiden ankommt — wer in `CLAUDE.md` schreibt, schreibt durch den Link hindurch.
Der Link ist `Optional`, weil seine Quelle dem Projekt gehoert und nicht der Installation.

Deshalb laeuft `ApplyRootInstructions()` **vor** `ApplyLinks()`: der Symlink braucht
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

Ein Verzeichnis-Symlink zeigt auf genau eine Quelle; damit kaeme entweder nur die
Installation oder nur das Projekt an. Bis Fassung 0.4 war das so. `checkRegistryLink()`
erkennt einen solchen Link und meldet ihn als `stale`; `ApplyLinks()` baut ihn um.

### Soll gegen Ist

`checkRegistryLink()` vergleicht den aufgeloesten Katalog mit dem, was im Zielverzeichnis
tatsaechlich registriert ist, und nennt die Abweichungen **beim Namen**:

| Feld | Bedeutung |
|---|---|
| `missing` | im Katalog, aber nicht registriert |
| `wrong` | registriert, zeigt aber auf die falsche Quelle — typisch, wenn das Projekt einen mitgelieferten Eintrag neuerdings ueberschreibt |
| `stale` | registriert, steht aber nicht mehr im Katalog — entfernt oder abgeschaltet |
| `blocked` | eine echte Datei des Projekts steht an der Stelle; sie gewinnt und bleibt liegen |

`blocked` zaehlt **nicht** als offener Punkt. Der Eintrag bleibt in der Liste sichtbar —
sonst wundert man sich, warum ein Override nicht greift —, aber der Gesamtzustand bleibt
`ok`: `ApplyLinks()` koennte daran nichts aendern, und ein Knopf, der etwas verspricht,
das er nicht einloest, ist schlechter als ein Hinweis.

Die Gegenrichtung — `stale` — braucht ein Kriterium dafuer, welche Links von uns stammen.
`ownedLinks()` loest jeden Symlink auf und nimmt nur die, die nach `k-playbook/` oder
`k-playbook-local/` zeigen. Alles andere im Verzeichnis gehoert dem Projekt und wird
weder bewertet noch angefasst.

`ApplyLinks()` raeumt erst die verwaisten Links weg und setzt dann die des Katalogs. Die
Reihenfolge zaehlt: ein abgeschalteter Eintrag muss verschwinden, bevor ein gleichnamiger
aus der anderen Quelle nachrueckt. Danach entfernt `removeEmptyDirs()` Namensraum-
Verzeichnisse, die dabei leer geworden sind.

Zustaende in `LinkState`:

| Zustand | Bedeutung |
|---|---|
| `ok` | alles steht so, wie es stehen soll |
| `missing` | nichts vorhanden |
| `stale` | Datei-Link auf ein falsches Ziel, oder Verzeichnis-Symlink aus einer aelteren Fassung |
| `incomplete` | das Verzeichnis steht, sein Inhalt weicht vom Katalog ab |
| `blocked` | etwas Echtes steht im Weg; wird nicht angefasst |
| `no-source` | es gibt nichts zu verlinken |

## Instruktionen

Zwei Ebenen, in `project/context.go` und `project/instructions.go`:

| Datei | Gilt fuer | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

`instructionFiles()` sammelt sie in dieser Reihenfolge und nimmt nur auf, was existiert —
ein Pfad ins Leere waere schlechter als keiner.

Die Datei heisst bewusst **nicht** `AGENTS.md`. Diesen Namen lesen die Assistenten von
sich aus, und er ist dem Hauptverzeichnis vorbehalten.

`ApplyRootInstructions()` legt `AGENTS.md` an, falls sie fehlt, und haengt sonst einen
kurzen Anstoss an, der auf `k-playbook context` verweist. Vorhandener Inhalt wird nie
ueberschrieben. Der Marker `<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf
den Block erneut anhaengt; `CheckRootInstructions()` prueft darauf.

Der Anstoss nennt **keine Verzeichnisebene**. Dieselbe Datei liegt im Projekt, in der
Installation und im Entwicklungsrepo — ein Verweis auf eine Ebene waere an zwei dieser
Orte falsch. Wo die Instruktionen liegen, beantwortet der Aufruf.

## Kataloge aufloesen

`BuildContext()` und `resolveCatalog()` in `project/context.go` sind der Kern. Hier liegt
die Overlay-Regel als Code statt als Prosa, die jeder Command selbst befolgen muesste.

Drei Sorten, definiert in `catalogKinds()`:

| Sorte | Verzeichnis | Muster |
|---|---|---|
| `rules` | `rules/` | `*.md` |
| `reviews` | `reviews/` | `review-*.md` |
| `checks` | `checks/` | `*.sh` |

Die Vergleichseinheit ist der **Dateiname** — beide Seiten benutzen dieselbe
Namenskonvention, ein abgeleiteter Schluessel waere unnoetiger Zwischenschritt. `key` gibt
es trotzdem: den Aufrufnamen ohne Endung und Sortenpraefix, damit `/k-review secret-scanning`
funktioniert.

Die Vereinigung beider Seiten, mit `origin` je Eintrag:

| `origin` | Bedeutung |
|---|---|
| `dist` | nur mitgeliefert |
| `local` | nur projekteigen |
| `override` | projekteigen, ersetzt einen gleichnamigen mitgelieferten |

**Abgeschaltet wird ueber eine leere Datei**, nicht ueber eine Liste in der
Konfiguration. `isEmptyFile()` gilt als leer, was ausser Leerzeilen und Kommentaren nichts
enthaelt — so kann die Datei ihren Grund tragen. Der Eintrag bleibt im Katalog und traegt
`disabled: true`.

Warum keine Liste: die lokale Datei ersetzt die mitgelieferte ohnehin vollstaendig. Bei
einer leeren bleibt nichts uebrig — das Abschalten faellt aus der bestehenden Regel heraus,
statt einen zweiten Mechanismus zu brauchen. Es steht ausserdem im Repo, versioniert und
mit Begruendung, statt in einer Konfigurationszeile.

`isCatalogEntry()` filtert, was nie ein Eintrag ist: `README.md`, Dotfiles und alles, was
nicht zum Muster passt. Unterverzeichnisse wie `checks/lib/` ebenso — dort liegt
Hilfscode.

Der Security-Tool-Preflight fehlt im Kontext bewusst: er startet je Tool ein `--version`
und dauert spuerbar. `context` soll billig genug sein, um am Anfang jedes Commands zu
stehen.

`CheckSchema()` laeuft vor allem anderen. Bei einer anderen Fassung als `3` bricht
`BuildContext()` ab, statt zu raten — die Werte liessen sich lesen, bedeuteten aber etwas
anderes.

## Aktualisieren

`project/update.go`, zwei Schritte:

`CheckUpdate()` fragt den Remote-Stand ab. Bewusst `git ls-remote` statt `git fetch`: die
Pruefung laeuft ungefragt nach dem Start und darf den Zustand des Repositorys nicht
anfassen. Upstream wird aus `branch.<name>.remote` und `branch.<name>.merge` gelesen; ohne
Upstream, ohne Branch oder bei nicht erreichbarem Remote gibt es eine Meldung statt eines
Fehlers. Ein Timeout von 15 Sekunden verhindert, dass ein haengender Remote die
Oberflaeche blockiert.

`Update()` holt den Stand per `git pull --ff-only`. Nur Fast-Forward: ein Merge im Clone
erzeugte eine lokale Historie, die niemand pflegt. Wer dort committet hat, soll das selbst
aufloesen.

### Die Installation muss sauber sein

`CheckCleanliness()` liest bei jeder Pruefung den lokalen Zustand des Clones mit — rein
lokal, ohne Netz, deshalb billig genug fuer den ungefragten Lauf nach dem Start.

Der Grund ist ein stiller Fehlerfall. Das Modell verlangt, dass in `k-playbook/` nie
geschrieben wird, aber die Regel erzwingt sich nicht. Aendert sich eine lokal veraenderte
Datei upstream nicht mit, laeuft `git pull` sauber durch und laesst sie stehen: die
Aenderung ueberlebt dann jedes Update, ohne je gemeldet zu werden. Aendert sie sich doch
mit, bricht git ab — mit einer Meldung, die im `output` verschwindet.

Drei Zustaende, zwei Schweregrade:

| Zustand | `Blocking()` | Warum |
|---|---|---|
| verfolgte Datei geaendert/geloescht | ja | geht beim Update verloren oder verhindert es |
| zusaetzliche Datei | nein | steht einem Fast-Forward nicht im Weg, gehoert aber nach `k-playbook-local/` |
| lokale Commits (`@{u}..HEAD`) | ja | blockieren `--ff-only`, nur von Hand aufloesbar |

`Update()` prueft **vor** dem Pull und bricht bei `Blocking()` ab, statt hinterher zu
stolpern. Das ist der Unterschied zwischen „irgendwas ging schief" und „`bin/k-playbook`
ist veraendert".

Die Oberflaeche zeigt den Befund in einer eigenen Karte, weil dort Dateinamen hinmuessen
— der Update-Button hat nur Platz fuer einen Zustand. Bewusst **ohne** Knopf zum
Zuruecksetzen: das waere `git checkout -- .` in einem fremden Verzeichnis, und die
Oberflaeche kann nicht wissen, ob dort jemand absichtlich entwickelt. Der Befehl steht
zum Kopieren da.

Denselben Befund meldet `/k-status` in der Zeile `Installation:`. Das faengt den Fall ab,
in dem ausgerechnet `bin/k-playbook` die veraenderte Datei ist: dann ist die Oberflaeche
ueber den Wrapper gar nicht erreichbar.

Vor und nach dem Pull werden die Dateien unter `dist/` per SHA-256 gehasht.
`BinaryChanged` meldet, ob sich etwas geaendert hat — **nur dann** bringt ein Neustart
eine andere Programmversion. Unter Linux behaelt der laufende Prozess seinen Inode und
arbeitet mit dem alten Code weiter, auch wenn die Datei ersetzt wurde.

### Die Verlinkung wird mitgezogen

`relinkAfterUpdate()` in `webui/update.go` ruft nach erfolgreichem Pull `ApplyLinks()`
auf. Das ist kein Komfort, sondern noetig: seit Commands und Skills **einzeln** verlinkt
werden, kommt ein neu mitgelieferter Command nicht mehr von selbst an. Ein
Verzeichnis-Symlink hatte das automatisch getan; ein Update, das den Katalog aendert, ihn
aber nicht registriert, waere halb erledigt — und zwar unsichtbar.

`PendingLinkChanges()` liest vorher die Bilanz und meldet sie: dazugekommen, entfernt,
auf eine andere Quelle umgesetzt. Die Namen werden **ueber alle Ziele zusammengefasst**,
sonst zaehlte ein einzelner neuer Command dreifach — er steht in `.claude/`, `.opencode/`
und `.cursor/`.

Schlaegt das Nachziehen fehl, bleibt das Update gueltig: der Pull ist durch, und die
Verlinkung laesst sich ueber die Assistenten-Karte nachholen.

## Host-weite Spiegelung

`Mirror()` in `hostinstall/mirror.go` laeuft bei jedem Start der Oberflaeche, direkt nach
dem Aufraeumschritt. Sie loest das Problem, dass die Oberflaeche haeufig gebraucht wird,
aber nur ueber `<projekt>/k-playbook/bin/k-playbook` erreichbar ist.

Der globale Aufruf ist ueberhaupt moeglich, weil das Programm sein Projekt aus dem
**Arbeitsverzeichnis** ableitet (`Detect()` ueber `os.Getwd()`) und nicht aus seinem
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
    └── security-tools/                    Tool-venvs, davon unberuehrt
```

Die Ebene `installation/` trennt die Spiegelung von den Tool-venvs, die unter
`~/.local/share/k-playbook/` ebenfalls zuhause sind (`rules/tool-install-scope.md`). Ein
venv bringt ein eigenes `bin/` mit; ohne diese Ebene kollidierten beide.

### Kopie statt Symlink ins Repo

Der abgeloeste Weg war ein Symlink von `~/.local/bin/k-playbook` auf den Wrapper **eines
bestimmten Clones**. Das bindet die host-weite Installation an ein einzelnes Projekt: wird
dort nicht gepullt, veraltet sie, und verschwindet der Clone, zeigt der Link ins Leere. Im
DevContainer kam hinzu, dass er nach jedem Rebuild von Hand neu zu setzen war.

Stattdessen kopiert nun jeder Start seine eigenen Dateien dorthin, sofern er einen neueren
Stand mitbringt. Wer aus einem aktuelleren Clone startet, hebt die host-weite Kopie damit
von selbst an — und ein geloeschter Clone stoert nicht.

### Wrapper und Binary, nicht das Binary allein

Nahe liegt, nur das fertige Binary zu kopieren: die Plattform ist beim Kopieren ja bereits
entschieden, ein Wrapper waere dann ueberfluessig. Das greift zu kurz. Auf einem Mac mit
DevContainer ist `~/.local/bin` derselbe Pfad, aber Host und Container brauchen
verschiedene Plattformen. Ist `~/.local` per `mounts` geteilt, treffen beide auf dieselbe
Datei.

Deshalb wird der Wrapper mitkopiert und die Struktur `<X>/bin/` neben `<X>/dist/`
eingehalten — die Plattformwahl bleibt damit zur Laufzeit, wo sie hingehoert. Der Wrapper
loest seine Symlink-Kette selbst auf und leitet `dist/` aus seinem **aufgeloesten** Ort
ab; er braucht fuer den Symlink in `~/.local/bin` keine Anpassung.

### Commit-Stand statt mtime

Verglichen wird der Zeitpunkt des letzten Commits, der `dist/` angefasst hat:
`git log -1 --format=%ct -- dist`. Der Wert landet als `<plattform>.stamp` neben dem
gespiegelten Binary.

Die mtime der Dateien waere das naheliegende Kriterium und ist trotzdem falsch: Git setzt
sie beim Auschecken auf den Zeitpunkt des Clones, nicht des Commits. Ein frisch geklonter
alter Stand saehe damit neuer aus als eine korrekte Installation und wuerde sie
ueberschreiben — bei mehreren Projekten mit je eigenem Clone staendig.

Ist die Quelle kein Git-Repository, bleibt der Stempel leer und es wird nur gespiegelt,
wenn im Ziel etwas fehlt. Ein unbekannter Stand darf einen bekannten nicht verdraengen.

### Stempel pro Plattform

Der Stempel gilt bewusst je Plattformdatei, nicht fuer die Installation als Ganzes.
Spiegelt der Mac zuerst, steht dort sein Stand und nur `darwin-arm64`. Startet danach der
Container aus demselben Clone, waere ein gemeinsamer Stempel gleich — und `linux-arm64`
fehlte dauerhaft. Kopiert wird deshalb auch, wenn das **eigene** Binary im Ziel fehlt,
unabhaengig vom Stand.

So wachsen genau die Plattformen zusammen, von denen aus tatsaechlich gestartet wurde,
statt alle vier Artefakte mit ihren rund 42 MB zu kopieren.

### Kein Sonderfall DevContainer

Im Container ist `~/.local/bin` ein **anderes** Verzeichnis: der Benutzer ist `vscode`
oder `root`, gemountet wird standardmaessig nur der Workspace nach `/workspaces/<name>`,
nicht das Home. Die Spiegelung laeuft dort deshalb ganz normal und erzeugt eine
container-eigene Kopie. Nach einem Rebuild ist sie weg und wird vom naechsten Start
wiederhergestellt — genau der Vorzug gegenueber dem alten Symlink.

`containerMarker()` in `webui/browser.go` bleibt davon unberuehrt und dient weiterhin
allein dazu, den Browserstart zu unterdruecken.

### Nur beim Start der Oberflaeche

Aufgerufen wird ausschliesslich im Zweig ohne Argumente, nicht bei `context`. Dessen JSON
auf stdout vertraegt keine Beigaben — dieselbe Begruendung, aus der auch der
Aufraeumschritt dort ausgelassen wird. Zugleich spart es das Git-Kommando bei den
haeufigen Kontextaufrufen der Commands, denen die Spiegelung ohnehin nichts bringt: sie
rufen `k-playbook/bin/k-playbook context` projektlokal auf und beruehren den `PATH` nie.

### Schreiben ohne Kollision

Jede Datei wird nach `<ziel>.tmp` geschrieben und dann umbenannt. Das Umbenennen ist
atomar und umgeht `ETXTBSY`: eine parallel laufende Instanz haelt die alte Datei offen,
waehrend der Name schon auf die neue zeigt.

Der Symlink wird angelegt oder neu ausgerichtet, wenn er fehlt oder woandershin zeigt.
Liegt dort eine **echte Datei**, gewinnt sie und bleibt unberuehrt — dieselbe Regel wie
bei der Assistenten-Verlinkung.

Wie beim Aufraeumschritt meldet sich die Spiegelung nur, wenn etwas passiert ist, und ein
Fehler haelt den Start nicht auf: die Oberflaeche laeuft auch ohne host-weite Kopie.

### Der PATH wird geprueft, nicht geschrieben

`CheckPath()` ist die rein lesende Gegenstueck-Funktion: Liegt der Symlink? Steht
`~/.local/bin` im `PATH`? Sie haengt **nicht** an `Mirror()`. Das war vorher so und war
ein Fehler: der Hinweis stand unter `!result.Empty()` und erschien damit nur beim ersten
Start. Beim zweiten war nichts zu spiegeln, `Result` blieb leer — und der Hinweis fiel
weg, obwohl der `PATH` weiterhin nicht stimmte.

`ExportLine()` baut die Zeile fuers Profil und setzt `$HOME` ein, wenn das Verzeichnis
darunter liegt. Dasselbe Profil wird auf Host und im Container gelesen; ein absoluter
Pfad waere dort falsch.

**Geschrieben wird in kein Shell-Profil.** Es gibt zu `/api/path` kein `POST`. Das Profil
gehoert dem Nutzer, und ein Programm, das ungefragt darin schreibt, waere schwerer zu
durchschauen als eine Zeile zum Kopieren. Die Oberflaeche zeigt den Zustand — als Karte,
die nur erscheint, solange etwas fehlt —, gehandelt wird im Terminal.

Geprueft wird der `PATH` **dieses** Prozesses. Wer die Zeile gerade eingetragen hat, sieht
die Aenderung erst in einer neuen Shell; die Meldung sagt das dazu.

## Altlasten des globalen Modells

`RemoveGlobalLinks()` in `legacy/global.go` laeuft bei jedem Programmstart, vor der
Oberflaeche. Sie raeumt weg, was das abgeloeste host-globale Modell hinterlassen hat:

| Ort | Alte Form |
|---|---|
| `~/.claude/commands` | Verzeichnis-Symlink auf `<repo>/commands`, sonst Einzel-Symlinks darin |
| `~/.claude/skills` | Einzel-Symlinks auf die Skill-Verzeichnisse des Repos |
| `~/.config/opencode/command` | Einzel-Symlinks auf `<repo>/commands/*.md` |
| `~/.config/opencode/opencode.jsonc`, `.json` | `skills.paths` auf die Basisinstallation |

Bleiben diese Links liegen, sieht ein Assistent in **jedem** Projekt zusaetzlich die
Commands eines fremden Standes — genau das, was die projektlokale Installation aufloesen
soll. Deshalb der Aufraeumschritt beim Start und nicht in der Oberflaeche.

Erkannt wird an einem Pfadsegment `k-playbook` im Symlink-Ziel, unabhaengig davon, ob das
Ziel noch existiert: der Repo-Pfad war frei waehlbar, das Verzeichnis hiess aber immer so,
und ein toter Link ist genauso eine Altlast wie ein lebender. Echte Dateien und Symlinks,
die woandershin zeigen, bleiben unberuehrt; ein Verzeichnis faellt nur weg, wenn danach
nichts mehr darin steht. Aus der OpenCode-Config wird ausschliesslich der Top-Level-Key
`skills` geschnitten, wenn sein Wert ein `k-playbook` nennt — der Rest der Datei bleibt
Zeichen fuer Zeichen erhalten, Kommentare eingeschlossen. Deshalb ein eigener
JSONC-Scanner statt Parsen und Neuschreiben.

Gemeldet wird nur, wenn tatsaechlich etwas wegfaellt; auf einem sauberen Rechner ist der
Schritt still. Ein Fehler dabei haelt den Start nicht auf.

## Security-Tool-Preflight

`CheckTools()` in `project/tools.go` ruft
`k-playbook/scripts/install-security-tools.sh --json` auf und liest dessen JSON.

Die Tool-Liste steht **nicht** im Go-Code. Sie kommt aus `scripts/security-tools.tsv`,
derselben Matrix, die auch das Skript und die Review-Rezepte lesen. Dort steht auch, wie
ein Tool auf den Host kommt (`install_method`, `install_ref`) und fuer welche
Projektsprachen es zustaendig ist (`languages`) — ein neues Tool ist eine Zeile in der
TSV, keine Aenderung an Skript oder Go-Code.

`Tool.Required` traegt die Sprachregel bereits: ein sprachgebundenes Tool ist nur Pflicht,
wenn seine Sprache im Aufruf stand. Ohne `--languages` gilt nur Sprachunabhaengiges als
Pflicht — was nicht gefragt wurde, kann auch nicht fehlen. Go reicht die Antwort
unveraendert durch und rechnet nichts nach.

Der Aufruf ist ausschliesslich lesend: `--json` prueft nur, ob die Binaries vorhanden
sind. Installiert wird bewusst im Terminal, weil das den Host veraendert und nicht das
Projekt. Ein Timeout von 30 Sekunden begrenzt den Aufruf, weil der Preflight je Tool ein
`--version` startet und eines davon haengen kann.

Bricht das Skript ab — etwa bei aktivem Projekt-venv — landet die erste stderr-Zeile in
der Fehlermeldung.

## GitHub CLI

`project/gh.go` haelt zwei Dinge auseinander, die in einer Karte zusammen erscheinen.

Die Projektentscheidung steht in `K-PLAYBOOK.yaml` unter `tools.gh.status` und wird
zeilenweise gelesen und geschrieben, wie der Rest der Konfiguration. `SetGHStatus()`
ersetzt nur den `gh:`-Unterblock; `replaceNestedBlock()` laesst einen danebenliegenden
Block eines anderen Tools stehen. Der Default ist `unknown` und
kein `disabled`: ohne Entscheidung weiss ein Command nicht, ob ein fehlendes `gh` ein
Problem oder gewollt ist, und die Oberflaeche zeigt den offenen Punkt rot. Ein unbekannter
Wert laesst `BuildContext()` abbrechen, damit ein Tippfehler nicht wie eine Entscheidung
aussieht.

Der Host-Befund kommt aus `DetectGH()`: ein `exec.LookPath("gh")` und ein Blick in
`hosts.yml` im gh-Konfigurationsverzeichnis, dazu `GH_TOKEN`/`GITHUB_TOKEN`, die die
Datei stechen. Gelesen werden dort nur `user` und die Namen unter `users`; die
Token-Zeilen daneben werden uebergangen und tauchen in keiner Antwort auf.

**Kein Aufruf von `gh auth status`.** Der prueft den Token beim Server und kostet einen
Netzzugriff. Deshalb steht der Befund anders als der Security-Preflight im Kontext: er
kostet nichts. Der Preis ist, dass ein abgelaufener Token als Anmeldung gilt — das sagen
Karte und Doku ausdruecklich.

Geschrieben wird nur die Entscheidung. Installation und Anmeldung bleiben im Terminal,
aus demselben Grund wie bei den Security-Tools; `gh auth login` will ohnehin einen
Browser. Auch das Umschalten zwischen Accounts steht nur als Befehl da: es gilt
maschinenweit fuer jedes Terminal und jedes Projekt, und ein Approve laeuft danach unter
dem neuen Namen. Ein Knopf in einer Projektoberflaeche wuerde diese Reichweite verdecken.

## Aufgelöster Kontext in der Oberflaeche

Der unterste Block der Startseite zeigt, was `BuildContext()` liefert — dasselbe, was
das Unterkommando `context` ausgibt: Pfade, Instruktionsdateien, die effektiven Kataloge
und die Guidelines.

Er ist ein `<details>` und der einzige Block, der **nicht** beim Seitenaufbau laedt.
`/api/context` wird erst beim ersten Aufklappen gerufen und danach nicht wieder: die
Ausgabe ist lang und wird nur gebraucht, wenn jemand nachsieht. Schlaegt der Aufruf fehl,
faellt die Sperre zurueck, ein erneutes Aufklappen versucht es noch einmal.

`contextResponse` reicht den Kontext unveraendert durch, damit die Antwort dasselbe
bedeutet wie die des Unterkommandos. Daneben steht `display` mit denselben Pfaden in
Anzeigeform — die Kuerzung auf `~` braucht das Home-Verzeichnis und kann nur serverseitig
passieren. Die uebrigen Pfade kuerzt die Oberflaeche gegen das Projektverzeichnis.

## Doku in der Oberflaeche

Ueber dem Kontext-Block steht die mitgelieferte Doku aus `k-playbook/docs`. Die Karte
listet alle Markdown-Dateien, auch die aus Unterverzeichnissen wie `libs/`; ein Klick
oeffnet die Datei in einem Fenster ueber der Seite.

`project.ListDocs()` sammelt die Dateien und nimmt als Titel die erste Ueberschrift,
ersatzweise den Dateinamen. Die `README.md` steht vorn, sie ist der Einstieg. Fehlt das
Verzeichnis, ist das ein Befund und keine leere Liste: die Antwort traegt dann eine
Meldung mit dem erwarteten Pfad.

`project.ReadDoc()` liefert den Rohtext, gerendert wird erst in `webui/docs.go` mit
Goldmark (GFM, `WithAutoHeadingID`). Rohes HTML aus der Quelle bleibt abgeschaltet — das
ist die Voreinstellung von Goldmark und genau richtig fuer Text, der ungeprueft im
Browser landet.

Der angefragte Pfad kommt aus dem Browser und wird in `docFilePath()` geprueft: relativ,
innerhalb des Doku-Verzeichnisses, Endung `.md`. Ohne diese Pruefung waere der Endpunkt
ein Weg, beliebige Dateien des Rechners zu lesen.

Verweise innerhalb der Doku faengt die Oberflaeche ab, weil ein Klick sonst die Seite
verlassen und damit den Server hinter ihr beenden wuerde: `.md`-Ziele oeffnet sie im
selben Fenster, Anker springen innerhalb der Datei, Ziele mit Schema gehen in einen
neuen Tab.

Mermaid-Bloecke rendert der Browser nach. Die Library kommt bei Bedarf vom CDN — sie ist
zu gross, um sie mitzuliefern. Ohne Netz bleibt der Quelltext des Diagramms als
Codeblock stehen, die Datei ist also weiterhin lesbar.

## Web-API

| Methode | Pfad | Zweck |
|---|---|---|
| `GET` | `/api/health` | Heartbeat in beide Richtungen |
| `POST` | `/api/client-gone` | Browser meldet Tab-/Fenster-Schliessen |
| `POST` | `/api/shutdown` | Server beenden |
| `GET` | `/api/path` | host-weite Aufrufbarkeit pruefen, read-only; kein `POST` |
| `GET` | `/api/config` | Anker suchen bzw. Ort vorschlagen |
| `POST` | `/api/config` | `K-PLAYBOOK.yaml` anlegen |
| `GET` | `/api/local` | projekteigene Struktur pruefen |
| `POST` | `/api/local` | fehlende Teile anlegen |
| `GET` | `/api/assistant` | Verlinkung pruefen |
| `POST` | `/api/assistant` | Verlinkung herstellen |
| `GET` | `/api/tools` | Security-Tool-Preflight, read-only |
| `GET` | `/api/gh` | `tools.gh` lesen, dazu den gh-Befund dieses Rechners |
| `POST` | `/api/gh` | `tools.gh.status` setzen; installiert und meldet nichts an |
| `GET` | `/api/remediation` | `remediation:`-Block lesen |
| `POST` | `/api/remediation` | `remediation:`-Block setzen |
| `GET` | `/api/update` | per `git ls-remote` pruefen, ob die Installation zurueckliegt; liefert den lokalen Sauberkeitszustand mit |
| `POST` | `/api/update` | `git pull --ff-only` ausfuehren; bricht bei lokal veraenderter Installation vorher ab |
| `GET` | `/api/context` | aufgeloesten Arbeitsstand lesen, read-only |
| `GET` | `/api/docs` | mitgelieferte Doku auflisten, read-only |
| `GET` | `/api/docs/file` | eine Datei daraus als HTML lesen, read-only |

Statische Assets liegen unter `/static/`, die Startseite unter `/`. `indexHandler`
rendert `static/index.html` als Template und liefert vorab mit, ob eine Installation
gefunden wurde.

## Lebenszyklus

Der Server bindet auf `127.0.0.1:0`, nimmt also einen freien Port, und gibt die URL im
Terminal aus.

Beenden funktioniert in beide Richtungen:

- Der Client ruft alle paar Sekunden `/api/health`. Bleibt er laenger als 5 Sekunden aus,
  ist das Browserfenster weg und der Server beendet sich.
- Beim Schliessen meldet der Browser `/api/client-gone` per `sendBeacon`. Danach wartet
  der Server 3 Sekunden, damit ein Reload ihn nicht abraeumt.
- Solange sich **nie** ein Client gemeldet hat, bleibt der Server stehen: der Browser kann
  noch unterwegs sein, oder die URL wird von Hand eingetragen.
- `Ctrl+C` und der `Schliessen`-Button gehen ebenfalls.

Der Browser wird nicht automatisch geschlossen, wenn der Server endet. Browser blockieren
das in vielen Faellen, und `open`/`xdg-open` liefern keinen verlaesslichen Tab-Handle.

In einem Container wird der Browser gar nicht erst geoeffnet; `containerMarker()` erkennt
das und weist auf die Port-Weiterleitung hin.

## Designentscheidungen

- Go ist die einzige Runtime. Keine Node-Toolchain, kein Build-Schritt fuer das Frontend.
- Die Oberflaeche ist eine lokale Web-UI, keine native App und kein Electron/Wails-Setup.
- Assets sind per `embed` im Binary. Ein Binary, keine Begleitdateien.
- Keine Shell-Pipelines. Fachlogik ruft Go-Funktionen; Fremdprozesse sind nur das
  Security-Tool-Skript und `git`.
- Kein YAML-Parser. Zeilenweises Lesen erhaelt Kommentare und unbekannte Bloecke.
- Die Overlay-Regel liegt im Code, nicht in der Prosa. `context` gibt eine Antwort, und
  alle Commands bekommen dieselbe.
- Abgeschaltet wird ueber eine leere Datei, nicht ueber eine Liste in der Konfiguration.
  Ein Mechanismus statt zweier.
- `git status --porcelain` und `git rev-list --count @{u}..HEAD` fuer den lokalen Zustand.
- `git ls-remote` zum Pruefen, `git pull --ff-only` zum Holen. Nichts, was den Zustand des
  Clones ungefragt veraendert.
- Kein Projekt-Store. Es gibt keine Liste bekannter Projekte mehr — das Werkzeug arbeitet
  auf dem Projekt, in dem es liegt.
- Geschrieben wird ausschliesslich nach Bestaetigung, Schritt fuer Schritt.
- `dist/` wird mitversioniert, damit die Installation ohne Go auskommt.

## Abhaengigkeiten

Eine: `github.com/yuin/goldmark` fuer die Doku-Anzeige. Cobra und die
Charmbracelet-Pakete des alten Stands sind mit ihm entfallen; alles uebrige kommt aus
der Standardbibliothek.

Goldmark ist reines Go und wird statisch mit einkompiliert — das Binary waechst um rund
1,6 MB, zur Laufzeit wird nichts nachgeladen. Ein Build ohne Netz braucht das Modul im
lokalen Cache; `go mod vendor` waere die Alternative, ist aber bewusst nicht gesetzt.

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
Oberflaeche im Vorschlagsmodus laeuft.

Ein laufender Server nutzt weiter den Code im Speicher. Nach Backend- oder
Asset-Aenderungen neu starten.

## Der alte Stand

`installer/_old/` enthaelt rund 7800 Zeilen Go vor dem Umbau, per `git mv` verschoben und
damit in der Historie nachvollziehbar. Verzeichnisse mit `_`-Praefix ignoriert die
Go-Toolchain vollstaendig: kein Build, keine Tests, keine Imports. Der Code ist dort
**nicht baubar** — seine Imports zeigen auf Pfade, die es nicht mehr gibt. Er ist zum
Lesen und Herueberkopieren da.

| Ort | Inhalt |
|---|---|
| `_old/internal/install/config.go` | vollstaendiger Config-Vertrag: `PathKeys`, `ValidatePath`, `RenderConfig` |
| `_old/internal/install/migrate.go` | zeilenweise Migration, die Kommentare und unbekannte Bloecke erhaelt |
| `_old/internal/projects/status.go` | die elf Statuspruefungen der alten Oberflaeche |
| `_old/internal/webui/server.go` | die alte GUI mit allen Bedienablaeufen |
| `_old/payload/payload.go` | `go:embed` samt Extract-Logik des abgeloesten Modells |

## Offene Punkte

- Was der Projektstatus in der Oberflaeche zeigen soll, nachdem der Projekt-Store
  entfallen ist. Daran haengt, was von `/k-status` ins Binary zurueckwandert: der Bericht
  steht derzeit auf der `context`-Ausgabe plus billigen Existenzpruefungen, weil das alte
  Subkommando `status` entfallen ist.
- Keine automatisierten Tests fuer die HTTP-Handler; getestet ist bisher nur
  `internal/project`.
- Release-Artefakte gibt es fuer macOS und Linux. Windows ist offen.
