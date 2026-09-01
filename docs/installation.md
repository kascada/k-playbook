# Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt trägt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git
k-playbook/bin/k-playbook
```

Das Zielverzeichnis muss `k-playbook` heißen — Commands und Skills sprechen es so an.
Ohne Zielargument ergibt sich der Name aus dem Repo-Namen und stimmt damit von selbst;
ein eigenes Argument brauchst du nur, wenn du aus einem Fork oder Mirror unter
abweichendem Namen klonst. Dann lautet es `k-playbook`.

**Go wird nicht gebraucht.** `bin/k-playbook` ist ein Wrapper, der das zur Plattform
passende Binary startet — für macOS und Linux gleichermaßen, was auch den Fall abdeckt,
dass Host und Container unterschiedliche Plattformen sind.

**Der erste Start braucht Netz.** Die Binaries liegen nicht mehr im Clone, sondern als
Assets am Release. Der Wrapper lädt genau das eine, das er braucht, und prüft es gegen
das mitgelieferte `SHA256SUMS`. Ohne Netzzugriff: [Das Binary und der
Cache](#das-binary-und-der-cache).

## Die vier Schritte

Der letzte Aufruf startet die Oberfläche im Browser. Sie führt durch vier Schritte und
schreibt jeden erst nach Bestätigung.

### 1. Konfiguration anlegen

Beim ersten Mal findet die Oberfläche noch keine `K-PLAYBOOK.yaml` — nach einem frischen
Clone kann es sie nicht geben. Statt zu raten, schlägt sie einen Ort vor und lässt ihn
bestätigen. Kandidaten in dieser Reihenfolge:

1. das Git-Repository, in dem der Aufruf stattfindet,
2. der Ort abgeleitet aus der Lage des Binaries,
3. das Arbeitsverzeichnis.

Mitvorgeschlagen wird, wo das Projekt-Repository liegt — entweder das Hauptverzeichnis
selbst oder ein Unterverzeichnis daneben, etwa wenn der Code parallel zum Playbook
ausgecheckt ist. Das Ergebnis:

```text
projekt/
├── K-PLAYBOOK.yaml     der Anker; sein Ort bestimmt das Hauptverzeichnis
└── k-playbook/         die Installation
```

Eine vorhandene `K-PLAYBOOK.yaml` wird nie überschrieben. Das Format steht in
[`k-playbook-format.md`](./k-playbook-format.md).

### 2. Projekteigene Struktur anlegen

Daneben entsteht `k-playbook-local/` mit allem, was dem Projekt gehört:

```text
k-playbook-local/
├── rules/         Overlay zu k-playbook/rules/
├── reviews/       Overlay zu k-playbook/reviews/
├── checks/        Overlay zu k-playbook/checks/
├── commands/      Overlay zu k-playbook/commands/
├── skills/        Overlay zu k-playbook/skills/
├── results/       alles, was Reviews erzeugen; siehe unten
├── docs/          Projektwissen für AI-Sessions, nach Herkunft getrennt
│   └── manual/    handgepflegte Doku; kein Command schreibt hier hinein
├── guidelines/
├── tasks/done/
├── priv/          eigene Notizen, siehe unten
├── material/      Rohmaterial als Quelle für Docs, siehe unten
├── k-playbook.md  projekteigene Instruktionsebene
└── TODO.md
```

Jedes Verzeichnis trägt eine `README.md` mit seinem Zweck — auch weil Git leere
Verzeichnisse nicht speichert und sie sonst nach einem Clone des Projekts fehlen würden.
Vorhandene Dateien bleiben unberührt, auch READMEs mit eigenem Text.

`k-playbook-local/` gehört ins Repository des Projekts und wird committet — bis auf den
**Inhalt** von drei Verzeichnissen, für die diese Wahl ansteht: `results/`, `priv/` und
`material/`. Für `priv/` und `material/` gilt weiterhin, dass k-playbook von sich aus
keine `.gitignore` schreibt und nicht entscheidet, was ein Projekt versioniert.

`results/` ist die Ausnahme: es wird beim erstmaligen Anlegen schon privat angelegt.
Ein Review ist aus dem Code wiederholbar, sein Ergebnis ist ein Stand von diesem Rechner
— und die Rohausgaben eines Secret-Scanners gehören ohnehin nicht ins Repository. Auch
das bleibt umschaltbar, und einmal umgeschaltet nimmt k-playbook es nicht zurück: die
verwaltete `.gitignore` entsteht nur beim erstmaligen Anlegen des Verzeichnisses, nicht
bei jedem Lauf. Bestandsprojekte, die `results/` bisher versioniert haben, merken vom
Update also nichts.

Sichtbar und umschaltbar ist diese Wahl im Block **Lokale Einstellungen** der
Oberfläche. Er misst je Verzeichnis mit `git check-ignore`, ob der Inhalt
wirklich draußen ist, und nennt das Repository, auf das sich die Aussage
bezieht — liegt in `k-playbook-local/` ein eigenes, gilt sie für dieses.
Angezeigt wird einer von vier Zuständen:

| Zustand | Bedeutung |
|---|---|
| privat | der Inhalt bleibt draußen |
| wird versioniert | keine Regel, der Inhalt kommt mit ins Repository |
| teilweise privat | eine Regel greift, es stehen aber Dateien im Repository — die Regel wirkt nur für neue |
| privat erst nach dem nächsten Commit | die Dateien sind aus dem Index genommen, aber noch nicht committet |

Die beiden letzten sehen privat aus und sind es nicht; sie stehen deshalb als
Warnung da und nennen die betroffenen Dateien. Das Umschalten auf privat legt
die `.gitignore` an und nimmt bereits versionierte Dateien mit
`git rm --cached` aus dem Index — wirksam wird das erst mit dem nächsten
Commit, und was schon gepusht wurde, bleibt in der Historie.

Stammt die Ignore-Regel von woanders — der `.gitignore` im Projekt-Root,
`.git/info/exclude`, der globalen Konfiguration — oder trägt die Datei im
Verzeichnis eigenen Inhalt, wird nichts geschrieben: der Block zeigt dann den
Zustand und benennt die Quelle. Von Hand geht der Weg weiterhin über eine
`.gitignore` im betreffenden Verzeichnis; die jeweilige `README.md` nennt den
Inhalt.

### 3. MCP-Server registrieren

Der Block **k-playbook-MCP** trägt den mitgelieferten MCP-Server bei den drei Assistenten
ein. Damit bekommt ein Assistent den aufgelösten Arbeitsstand als Werkzeug, statt ihn über
die Kommandozeile zu holen:

```text
projekt/
├── .mcp.json          Claude Code:  mcpServers -> k-playbook
├── .cursor/mcp.json   Cursor:       dasselbe Schema
└── opencode.json      OpenCode:     mcp -> k-playbook
```

Eingetragen wird `k-playbook/bin/k-playbook mcp` — der projekteigene Wrapper, **relativ**
zum Hauptverzeichnis. Nur so lässt sich der Eintrag einchecken und gilt im DevContainer
genauso. Der Preis ist eine Bedingung: der Eintrag wirkt nur, wenn der Assistent im
Hauptverzeichnis geöffnet ist — dort, wo `K-PLAYBOOK.yaml` liegt.

Die drei Dateien gehören dem Projekt. Angefasst wird genau der Schlüssel `k-playbook`,
fremde Einträge bleiben stehen. Fertig ist die Registrierung erst nach einem Neustart des
Assistenten; Claude Code fragt dabei einmal nach der Freigabe.

Alles Weitere — die beiden Schemata, der Umgang mit fremden Werten, das Entfernen von
Hand und die Seite `/mcp` mit den tatsächlich angebotenen Werkzeugen — steht in
[`mcp.md`](./mcp.md).

### 4. Assistenten verlinken

Verlinkt wird für Claude Code, OpenCode und Cursor:

```text
projekt/
├── AGENTS.md             Instruktionen, eine Quelle für alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md; die Richtung ist fest
├── .claude/
│   ├── commands/         je ein Symlink pro Command
│   └── skills/           je ein Symlink pro Skill; OpenCode liest hier mit
├── .opencode/
│   └── commands/
└── .cursor/
    └── commands/
```

Die vier Ziele sind **echte Verzeichnisse mit Einzel-Symlinks**, kein Verzeichnis-Symlink.
Ein Verzeichnis-Symlink zeigt auf genau eine Quelle; damit käme entweder nur die
Installation oder nur `k-playbook-local/` an. Jeder Link zeigt auf die Fassung, die nach
der Overlay-Regel gilt:

```text
.claude/commands/
  k-todo.md    -> ../../k-playbook/commands/k-todo.md          mitgeliefert
  k-review.md  -> ../../k-playbook-local/commands/k-review.md  projekteigen, ersetzt
  k-eigen.md   -> ../../k-playbook-local/commands/k-eigen.md   nur projekteigen
```

Die Oberfläche vergleicht diesen Soll-Stand mit dem, was tatsächlich registriert ist,
und meldet Abweichungen mit Namen: was fehlt, was auf die falsche Quelle zeigt, was
verwaist ist, und was dem Projekt gehört und deshalb liegen bleibt. Auf Knopfdruck wird
es angeglichen. Eine echte Datei, die jemand selbst dort abgelegt hat, gewinnt immer und
wird nie ersetzt.

Skills stehen nur einmal unter `.claude/skills`: OpenCode durchsucht dieses Verzeichnis
mit, Cursor kennt kein Skill-Konzept. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, weil
Claude Code ausschließlich `CLAUDE.md` liest und OpenCode `AGENTS.md` bevorzugt — so
landet jede Änderung in beiden.

`AGENTS.md` bekommt dabei einen kurzen **Anstoß**: einen Block, der auf
`k-playbook context` verweist. Fehlt die Datei, wird sie angelegt; ist sie da, wird der
Block angehängt und vorhandener Inhalt nicht angetastet. Ein Marker
`<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf ihn erneut anhängt.

### Eine vorhandene CLAUDE.md

Die Richtung des Symlinks ist fest. Damit daneben keine zweite, abweichende
Instruktionsdatei entsteht, ordnet das Einrichten das Paar `CLAUDE.md`/`AGENTS.md`
zuerst ein und löst auf, was sich auflösen lässt:

| Ausgangslage | Was geschieht |
|---|---|
| nur eine echte `CLAUDE.md` | sie wird nach `AGENTS.md` **umbenannt**, der Anstoß an den erhaltenen Inhalt angehängt, `CLAUDE.md` neu als Symlink gesetzt |
| `AGENTS.md` ist ein Symlink auf `CLAUDE.md` | die verdrehte Richtung wird aufgelöst: Symlink weg, umbenennen, neu verlinken |
| `AGENTS.md` ist ein toter Symlink | er wird entfernt, damit die Datei nicht an seinem Ziel landet |
| beide sind echte Dateien | **Konflikt** — von Hand zusammenführen und `CLAUDE.md` löschen; oder, wenn ein Editor beim Speichern den Symlink ersetzt hat, `CLAUDE.md` löschen und neu einrichten |
| `CLAUDE.md` oder `AGENTS.md` zeigt bewusst auf ein anderes Ziel | **Konflikt** — der Link des Projekts bleibt stehen |
| `AGENTS.md` ist in git ignoriert | **Konflikt** — sonst fiele der Inhalt still aus der Versionskontrolle; Ignore-Regel entfernen und neu einrichten |

Bei einem Konflikt wird nichts verschoben, nichts gelöscht, nichts gesichert und auch
kein `AGENTS.md` angelegt. Das ist kein Schönheitsfehler: solange er steht, sieht Claude
Code vom Einrichten nichts, weil er ausschließlich `CLAUDE.md` liest. Die
Assistenten-Karte meldet den Zustand als `Konflikt` und nennt den Ausweg im Detailtext.

Derselbe Ablauf läuft beim **Aktualisieren**. Ein Projekt, das nur eine echte
`CLAUDE.md` hat, wird also auch darüber eingerichtet, und ein Projekt ganz ohne
`AGENTS.md` bekommt sie dabei erstmals.

Was ein Assistent darüber hinaus lesen soll, steht nicht in `AGENTS.md`, sondern in
`k-playbook.md` — je einmal pro Ebene:

| Datei | Gilt für | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge; die projekteigene Ebene ergänzt die mitgelieferte
oder überstimmt sie.

Die Verlinkung ist projektlokal. Es wird nichts in `~/.config/opencode/` oder
`~/.claude/` geschrieben. Dadurch kann ein Rechner mehrere Projekte mit
unterschiedlichen k-playbook-Ständen tragen, ohne dass sie sich gegenseitig
überschreiben.

**Altlasten werden entfernt.** Auf Rechnern mit einer Installation nach dem alten Modell
liegen noch host-globale Symlinks unter `~/.claude/commands`, `~/.claude/skills` und
`~/.config/opencode/command`, dazu ein `skills.paths`-Eintrag in der
OpenCode-User-Config. Die wirken in jedes Projekt hinein — ein Assistent sähe dort
zusätzlich die Commands eines fremden Standes. `k-playbook` entfernt sie bei jedem Start,
aber nur, was nachweislich zu einem k-playbook gehört. Fällt etwas weg, meldet es das im
Terminal; sonst bleibt es still.

Nach Änderungen an Commands oder Skills muss der jeweilige Assistent neu gestartet
werden — beide erfassen sie beim Start.

## Browser beim Start

Beim Start der Oberfläche steht die URL im Terminal, und der Browser wird geöffnet.
Welches Programm das tut, entscheidet sich in dieser Reihenfolge:

1. **`$BROWSER`**, sofern gesetzt. Die freedesktop-Konvention: eine mit `:` getrennte
   Liste von Kommandos, in denen `%s` für die URL steht. Fehlt der Platzhalter, wird die
   URL angehängt.
2. Andernfalls die üblichen Verdächtigen der Plattform — `open` auf macOS, sonst
   `wslview`, `xdg-open`, `gio open` und weitere, bis eines startet.

**Im Container zählt allein `$BROWSER`.** Dort liefe jeder geratene Kandidat im Container
statt auf dem Rechner vor dem Nutzer; schlimmer noch, `x-www-browser` und
`sensible-browser` zeigen in schlanken Images gern auf einen Terminal-Browser, der dann
das Terminal übernimmt. Ein ausdrücklich gesetzter `$BROWSER` weiß es dagegen besser: Er
zeigt auf einen Helfer, der die URL an den Host durchreicht.

Der DevContainer von VS Code richtet genau das von selbst ein — die Variable zeigt dort
auf ein Skript, das `code --openExternal` aufruft und damit den Browser auf dem Host
öffnet. Der weitergeleitete Port kommt ebenfalls von VS Code. Es ist derselbe Weg, über
den auch `gh auth login` seinen Browser öffnet.

Ist `$BROWSER` im Container nicht gesetzt, bleibt es beim bisherigen Verhalten: das
Terminal nennt den erkannten Container-Marker und die URL zum Selbsteintragen. Wer den
Helfer nachrüsten will, setzt die Variable selbst:

```bash
export BROWSER=/pfad/zum/helfer.sh   # bekommt die URL als Argument
```

## Reviews und Tasks

Der Block **Workflows** führt zu den beiden Arbeitsvorräten. Auf jedem Knopf steht, wie
viel dort liegt: die Zahl der Review-Läufe unter `k-playbook-local/results/` und die der
offenen Tasks unter `k-playbook-local/tasks/`.

Aufgelistet wird auf den Seiten selbst. `/reviews` zeigt die bisherigen Läufe und stellt
einen neuen zusammen. `/tasks` listet die offenen Tasks nach ihrer Nummer; ein Klick
zeigt den Task als Markdown unter der Liste. Die erledigten aus `tasks/done/` stehen
darunter in einem zugeklappten Block — die jüngste Nummer oben — und lassen sich genauso
lesen.

Rechts an jeder Task-Zeile steht, ob sie schon durch `/k-task-refine` gegangen ist —
mit Datum, sofern das Review-Log eines nennt. „ohne Task-Refine" ist kein Fehler, aber
der Grund, warum `/k-task-run` vor der Ausführung nachfragt.

Gelesen wird nur. Angelegt und ausgeführt werden Tasks über `/k-task-create` und
`/k-task-run` im Assistenten.

## Doku lesen

Über dem Kontext-Block steht **Dokumentation**. Die Karte listet alle Markdown-Dateien
aus `k-playbook/docs` — dieselbe Doku, die du gerade liest, in dem Stand, der im Projekt
installiert ist. Ein Klick öffnet die Datei in einem Fenster über der Seite; Verweise
darin führen zur nächsten Datei, `Escape` oder ein Klick daneben schließt.

Mermaid-Diagramme werden gezeichnet, sofern der Rechner ins Netz kommt: die Library wird
bei Bedarf geladen. Ohne Netz bleibt der Diagramm-Quelltext stehen, der Text ist
weiterhin vollständig lesbar.

## Nachsehen, was gilt

Ganz unten steht der Block **Aufgelöster Kontext**. Aufgeklappt zeigt er, was ein
Command sieht: die aufgelösten Pfade, die Instruktionsdateien in Lesereihenfolge, die
effektiven Kataloge für Regeln, Reviews und Checks samt Herkunft — mitgeliefert,
projekteigen oder ersetzt — und die Guidelines. Abgeschaltete Einträge stehen mit, damit
sichtbar bleibt, dass es sie gibt.

Es ist dieselbe Auskunft wie `k-playbook/bin/k-playbook context`, nur lesbar aufbereitet.
Der Block lädt erst beim Aufklappen und verändert nichts.

Ein Assistent kann dieselbe Auskunft als Werkzeug bekommen, statt sie über die
Kommandozeile zu holen. Dafür ist der MCP-Server da; eingerichtet wird er im Block
**k-playbook-MCP**, und die Seite `/mcp` zeigt den Registrierungszustand samt den
Werkzeugen, die der Server tatsächlich anbietet. Alles dazu steht in
[`mcp.md`](./mcp.md).

## Aktualisieren

Der bequeme Weg ist die Oberfläche. Sie prüft nach dem Start per `git ls-remote`, ob
die Installation hinter dem Remote liegt, und zieht auf Knopfdruck per
`git pull --ff-only` nach. Dafür macht sie `k-playbook/` kurz beschreibbar und setzt es
danach wieder read-only. Bewusst `ls-remote` statt `fetch`: die Prüfung läuft
ungefragt und darf den Zustand des Repositorys nicht anfassen. Bewusst `--ff-only`: ein
Merge im Clone erzeugte eine lokale Historie, die niemand pflegt.

Von Hand geht es genauso:

```bash
cd /pfad/zum/projekt
make -C k-playbook installer-update
```

Das Make-Target entspricht `chmod -R u+w k-playbook && git -C k-playbook fetch origin && git -C k-playbook reset --hard origin/main && git -C k-playbook clean -fd && chmod -R a-w k-playbook` und sperrt die Installation auch dann wieder, wenn der Pull fehlschlägt. Der harte Reset ist bewusst: `k-playbook/` trägt per Vertrag keine lokalen Änderungen; alles darin darf nur durch Pull entstehen. Wurde zwischenzeitlich per `installer-sync` ein Arbeitsstand eingespielt, wird er beim nächsten Update automatisch aufgeräumt. Im Entwicklungsrepo funktioniert zusätzlich `make installer-update`, weil dort der Installations-Clone unter `./k-playbook/` liegt.

`k-playbook/` enthält nichts Projekteigenes und ist dadurch vollständig ersetzbar —
auch per `rm -rf` und neuem Clone. `K-PLAYBOOK.yaml` und `k-playbook-local/` liegen
daneben und bleiben unberührt.

Bei jedem Start der Oberfläche wird eine vorhandene Installation ebenfalls read-only
gesetzt. Das ist nur eine lokale Schutzschicht gegen versehentliche Schreibzugriffe; das
Update hebt sie gezielt und temporär auf.

**Wurde dort trotzdem lokal gearbeitet, sagt die Oberfläche es und aktualisiert nicht.**
Der Block `Installation` erscheint nur in diesem Fall, nennt die betroffenen Dateien und
gibt den Befehl zum Zurücksetzen aus; ausgeführt wird er nicht von selbst. Der Grund
für die Prüfung ist, dass der Fehler sich sonst versteckt: ändert sich eine lokal
veränderte Datei upstream nicht mit, läuft `git pull` sauber durch und lässt sie
stehen — die Änderung überlebt dann jedes Update, ohne je aufzufallen.

Hat dabei `VERSION` gewechselt, gehört zu dem neuen Stand ein anderes Binary: die
Oberfläche verlangt einen Neustart und lädt das neue Binary gleich in den Cache, damit
der Neustart nicht darauf warten muss. Schlägt das Laden fehl — offline, hinter einem
Proxy —, bleibt es bei einem Hinweis; das Update selbst gilt trotzdem als gelungen, und
der nächste Start holt es nach. Sind nur Commands, Regeln oder Rezepte neu, ändert sich
`VERSION` nicht und ein Neustart des Assistenten genügt.

**Die Verlinkung zieht sich selbst nach.** Weil Commands und Skills einzeln verlinkt
sind, kommt ein neu mitgelieferter Command nicht von allein an. Nachgezogen wird deshalb
auf dem Lesepfad, nicht erst auf Knopfdruck: Der Assistenten-Block richtet die
Registrierung beim Anzeigen aus und meldet, was sich geändert hat (`Verlinkung
nachgezogen: 3 dazugekommen, 1 entfernt.`), und `k-playbook context` tut dasselbe — der
Aufruf, der ohnehin am Anfang jeder Assistenten-Sitzung steht. Wie die Installation zu
ihrem Stand kam, spielt dabei keine Rolle: über die Oberfläche, mit `git pull` von Hand
oder über ein Ziel im Makefile. Was sich nicht von selbst auflösen lässt — eine echte
Projektdatei im Weg, ein Konflikt an `CLAUDE.md` — bleibt liegen und steht weiter im
Assistenten-Block.

Damit die neuen Commands im Assistenten ankommen, muss dieser danach neu gestartet
werden — Claude Code, OpenCode und Cursor erfassen sie beim Start.

### Bestehende Projekte: zwei Dinge kommen dazu

Wer ein Projekt aus einer Fassung bis 0.4 aktualisiert, findet nach dem Update zweierlei
vor. Beides erledigt ein Klick, gelöscht oder überschrieben wird nichts:

| Wo | Was die Oberfläche meldet | Was zu tun ist |
|---|---|---|
| Projekteigene Struktur | `Fehlende Einträge: commands, skills` | **Anlegen** — die beiden Overlay-Verzeichnisse entstehen mit ihrer README |
| Assistenten-Verlinkung | `Verzeichnis-Symlink aus einer älteren Fassung` | **Einrichten** — der Symlink wird durch Einzel-Links ersetzt |

Der zweite Punkt ist die eigentliche Umstellung: aus `.claude/commands -> ../k-playbook/commands`
wird ein echtes Verzeichnis mit einem Link je Command. Die Quelle in `k-playbook/` bleibt
dabei unangetastet.

Die Einzel-Links gehören ins Repository des Projekts und werden committet — dann hat ein
frischer Clone die Commands sofort registriert.

### Eine Konfiguration aus einem abgelösten Modell

Wer ein Projekt aus einer der ersten Fassungen weiterträgt, stößt irgendwann auf:

```text
K-PLAYBOOK.yaml hat schema_version 1 und beschreibt ein abgelöstes Modell …
```

Das ist kein Fehler im Projekt, sondern die Folge einer bewussten Aufteilung: die
Installation aktualisiert sich per `git pull`, die `K-PLAYBOOK.yaml` liegt daneben und
wird nie überschrieben — sie gehört dem Projekt. Irgendwann ist das Werkzeug drei
Modelle weiter und die Datei noch beim ersten. Umgerechnet wird nicht
([`k-playbook-format.md`](./k-playbook-format.md#schema_version) sagt, warum);
zurückgesetzt schon.

Die Oberfläche starten: der Block **Projektkonfiguration** steht dann wieder da, nennt
die gefundene Fassung und das Modell, das sie beschreibt, und bietet **Zurücksetzen und
neu anlegen** an. Dabei wird

- die alte Datei als `K-PLAYBOOK.yaml.v1-alt` daneben gelegt — nicht gelöscht, denn
  `remediation`, `tools` und `project.repo_root` stehen nur dort,
- eine frische `K-PLAYBOOK.yaml` mit `schema_version: 3` geschrieben, mit dem alten
  `project.repo_root` vorbelegt,
- eine bereits vorhandene Sicherung nie überschrieben; sie bekommt `-2`, `-3` angehängt.

Danach die restlichen Blöcke wie bei einer neuen Installation durchgehen und die eigenen
Werte aus der Sicherung zurückholen.

**Vorher zieht das Projekteigene um.** Unter Modell 1 lagen Tasks, Checks, Reviews,
Guidelines, Docs und die `TODO.md` **innerhalb** von `k-playbook/` — genau dem
Verzeichnis, das heute der ersetzbare Clone ist. Nur die Konfiguration zu erneuern
hinterließe eine stille Falle: alles sähe gesund aus, und das nächste Update nähme die
Inhalte mit. Findet die Oberfläche dort Projekteigenes, schreibt sie deshalb nichts,
nennt die Pfade und bleibt bei „Veraltet" stehen:

```bash
cd /pfad/zum/projekt
git mv k-playbook/tasks     k-playbook-local/tasks
git mv k-playbook/reviews   k-playbook-local/reviews
git mv k-playbook/TODO.md   k-playbook-local/TODO.md
```

Welche Pfade es sind, steht im `paths.`-Block der alten Datei. Sind sie umgezogen, wird
der Knopf frei.

## Host-weit aufrufbar

Der tiefe Pfad `k-playbook/bin/k-playbook` ist nur beim ersten Mal nötig. Jeder Start der
Oberfläche legt eine host-weite Kopie an und verlinkt sie:

```text
~/.local/
├── bin/k-playbook -> ../share/k-playbook/installation/bin/k-playbook
└── share/k-playbook/
    ├── installation/bin          der gespiegelte Wrapper
    └── security-tools/           Tool-venvs, davon unberührt
```

Danach genügt überall:

```bash
cd /pfad/zum/projekt
k-playbook
```

Es ist dasselbe Werkzeug für alle Projekte. Welches Projekt gemeint ist, ergibt sich aus
dem Verzeichnis, in dem der Aufruf stattfindet — nicht aus dem Ort des Programms.

Gespiegelt werden genau drei Dateien — `bin/k-playbook`, `VERSION` und `SHA256SUMS` —
und nur, wenn der Clone einen neueren Stand mitbringt als die Kopie. Kein Binary: das
löst der Wrapper über den Cache auf. Wer in einem Projekt `git pull` macht und dort
startet, hebt die host-weite Kopie damit an. Umgekehrt überschreibt ein älterer Clone
sie nicht.

Maßgeblich für „neuer" ist der HEAD-Commit des Clones. Die Kopie wird dadurch öfter
erneuert als früher, als nur Commits an den Binaries zählten — sie ist dafür klein, und
der Wrapper ist genau die Datei, die aktuell sein muss.

Ein DevContainer bekommt seine eigene Kopie unter seinem eigenen Home; nach einem Rebuild
stellt der nächste Start sie wieder her. Auf einem Mac mit Container teilen sich beide
denselben Wrapper, falls `~/.local` geteilt ist — die Plattformen trennt erst der Cache.

**Eine eigene DevContainer-Integration gibt es nicht mehr** — keinen Bind-Mount nach
`/workspaces/k-playbook`, keinen Symlink im Container und kein Setup-Skript in
`.devcontainer/`. Die Installation liegt im Projektverzeichnis und kommt mit ihm in den
Container, wie jede andere Projektdatei auch.

**Zum PATH:** Auf Linux ist `~/.local/bin` meist schon drin. Auf macOS **nicht** —
`/etc/paths` kennt es nicht und `path_helper` ergänzt es nicht.

Fehlt es, zeigt die Oberfläche ganz oben die Karte **Aufruf von überall** mit der Zeile
zum Kopieren:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Die Zeile gehört ins Shell-Profil — `~/.zprofile` bei zsh (Standard seit Catalina),
`~/.bashrc` bei bash. Danach eine neue Shell öffnen; geprüft wird der `PATH` des
laufenden Prozesses, eine gerade eingetragene Zeile sieht er noch nicht.

**Geschrieben wird dort nichts von selbst.** Das Profil gehört dir. Steht der Aufruf,
verschwindet die Karte wieder — dieselbe Zeile steht außerdem beim Start im Terminal.

Der Aufruf über `k-playbook/bin/k-playbook` im Projekt bleibt jederzeit möglich und
gleichwertig. Die Commands nutzen ausschließlich ihn, nie den `PATH`.

## Das Binary und der Cache

Der Wrapper sucht das Binary in dieser Reihenfolge und nimmt das erste, das er findet:

1. `$K_PLAYBOOK_BINARY` — ausdrücklich gesetzt, gewinnt immer
2. `<installation>/dist/` — lokal gebaut; im Repo-Checkout hat das Vorrang, damit der
   Entwicklungs-Loop netzfrei bleibt
3. der Cache
4. Download des Release-Assets zu der Version aus `VERSION`, geprüft gegen `SHA256SUMS`

Der Cache liegt **außerhalb** der Installation — die wird nach jedem Update per
`chmod -R a-w` gesperrt, ein Cache darunter wäre nicht beschreibbar. Der Ort ergibt sich
aus `$K_PLAYBOOK_CACHE`, sonst `$XDG_CACHE_HOME/k-playbook`, sonst
`$HOME/.cache/k-playbook`; darunter liegt `bin/<version>/k-playbook-<os>-<arch>`. Alle
Projekte desselben Rechners teilen ihn, und Host und Container kollidieren nicht, weil
die Dateinamen die Plattform tragen.

**Ohne Netzzugriff.** `bin/k-playbook --prefetch` lädt das Binary der eigenen Plattform
vorab, `--prefetch --all` alle vier auf einmal — das deckt den Mac mit Linux-Container in
einem Aufruf ab. Wo `objects.githubusercontent.com` nicht erreichbar ist, wird der Cache
anderswo befüllt und über `K_PLAYBOOK_CACHE` eingebunden; alternativ baut `make dist-host`
das Binary selbst (dafür braucht es Go).

**Im DevContainer.** Ein Cache unter `$HOME` überlebt den Rebuild nicht: der Container
hat sein eigenes Home. Deshalb zeigt man ihn in den Workspace:

```json
"containerEnv": { "K_PLAYBOOK_CACHE": "${containerWorkspaceFolder}/k-playbook-local/.cache" }
```

`k-playbook-local/.cache/` gehört in die ignorierten Pfade.

**Ein Stand, ein Binary.** `VERSION` im Wurzelverzeichnis nennt das Release, dessen
Assets zu diesem Clone-Stand gehören. Commits an Regeln, Reviews, Commands oder Docs
ändern sie nicht und lösen deshalb keinen Download aus. Wechselt sie beim Update, lädt
der Update-Pfad das neue Binary gleich in den Cache und meldet, dass ein Neustart eine
andere Programmversion bringt.

## GitHub CLI

`/k-pr-review` und das Dependabot-Review arbeiten über `gh`. Die Karte **GitHub CLI**
hält zwei Dinge auseinander, die leicht durcheinandergeraten.

Das eine ist die **Entscheidung des Projekts**: nutzt es `gh` oder nicht. Sie wird hier
gesetzt und landet in `K-PLAYBOOK.yaml` unter `tools.gh.status`. Bis sie fällt, steht sie
auf `unknown`, und die Karte zeigt das rot — nicht als Schönheitsfehler, sondern weil ein
Command sonst nicht weiß, ob ein fehlendes `gh` ein Problem oder gewollt ist. Commands,
die `gh` brauchen, brechen bei `unknown` ab.

Das andere ist der **Befund für diesen Rechner**: liegt `gh` im PATH, und ist ein Account
hinterlegt. Der steht nur in der Karte und in der Kontextausgabe, nie in der
Konfiguration — auf dem nächsten Rechner ist er ein anderer.

Installiert und angemeldet wird im Terminal, wie bei den Security-Tools: beides verändert
den Host, und `gh auth login` will einen Browser. Die Karte zeigt dafür den passenden
Befehl.

```bash
gh auth login --hostname github.com   # anmelden
gh auth status                        # Token beim Server prüfen
```

Der Befund ist aus `~/.config/gh/hosts.yml` gelesen und **nicht beim Server geprüft**:
ein hinterlegter Token kann abgelaufen oder zurückgezogen sein. Wer Gewissheit braucht,
ruft `gh auth status` auf.

Sind mehrere Accounts hinterlegt, nennt die Karte sie und zeigt den Umschaltbefehl:

```bash
gh auth switch --hostname github.com --user <account>
```

Bewusst als Befehl und nicht als Knopf. Der Wechsel gilt für jedes Terminal und jedes
Projekt auf diesem Rechner, nicht nur für dieses — und ein Approve oder Merge läuft
danach unter dem neuen Namen. `/k-pr-review` nennt den aktiven Account deshalb vor jeder
Schreibaktion.

Nur `github.com`. Enterprise-Instanzen hätten eigene Accounts je Host und eine eigene
Entscheidung je Projekt; das wäre etwas anderes als das hier.

## Security-Tools

Projekte dürfen mit eigenem `.venv` arbeiten. Security-Tools werden davon getrennt host-
oder user-lokal installiert, nie in ein Projekt-venv. Sie sind die eine bewusste Ausnahme
von der Projektlokalität: ein Scanner gehört zur Arbeitsumgebung, nicht zum Projekt.

Die kanonische Matrix liegt in [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv).
Sie wird vom Installationsskript und von der Oberfläche gelesen; die Liste steht nicht
zusätzlich im Go-Code.

Pflicht-Tools:

| Tool | Sprachen | Rolle |
|---|---|---|
| `gitleaks` | alle | Secret-Scanning |
| `trufflehog` | alle | tiefes Secret-Scanning |
| `trivy` | alle | Filesystem-, Container- und IaC-CVEs |
| `syft` | alle | SBOM-Erzeugung |
| `grype` | alle | SBOM-/Dependency-CVE-Auswertung |
| `pip-audit` | Python | Python Dependency-CVEs |
| `semgrep` | Python, Go | generische Security-Regeln |
| `osv-scanner` | Python, Go | Dependency-CVEs mit SARIF |
| `gosec` | Go | Go-Security |
| `govulncheck` | Go | Go-CVEs mit Reachability |

Optional, weil sie sich mit anderen überschneiden oder eine projekteigene Konfiguration
brauchen: `ruff` (Python-Qualität; sein `S`-Regelwerk *ist* flake8-bandit), `bandit`
(Python-Security) und `golangci-lint` (Go-Qualität). `docker` ist ebenfalls optional und
wird als Fallback-Kontext angezeigt, aber nicht durch k-playbook installiert.

**Pflicht gilt je Sprache.** Ein sprachgebundenes Tool zählt nur dann als fehlende
Pflicht, wenn seine Sprache gefragt war — und ohne Angabe gilt gar keine Sprachbindung als
Pflicht, weil sich ohne diese Information nicht verlangen lässt, was vielleicht nicht
gebraucht wird:

```bash
k-playbook/scripts/install-security-tools.sh --languages python,go --preflight
```

Die Oberfläche zeigt den Status read-only und installiert nichts. Alles Weitere macht
das Skript selbst:

```bash
k-playbook/scripts/install-security-tools.sh                       # Status, das ist der Default
k-playbook/scripts/install-security-tools.sh --install missing     # fragt vor der Installation
k-playbook/scripts/install-security-tools.sh --help                # erklärt die Methoden
```

`--method` wählt zwischen `auto`, `native`, `docker`, `pipx` und `venv`. Ohne `--yes`
zeigt das Skript den Plan und fragt.

Woher ein Tool kommt, steht in der Matrix und nicht im Skript: die Spalte
`install_method` nennt `github` (Release-Asset), `go` (`go install`), `pipx` (pipx oder
ein dediziertes Tool-venv) oder `none`, die Spalte `install_ref` die passende Referenz und
`asset_pattern` bei GitHub-Releases das Namensmuster des Assets. Ein neues Tool ist damit
eine Zeile in der TSV.

**`go install` bleibt den Tools vorbehalten, die Go ohnehin brauchen.** Sonst müsste ein
reines Python-Projekt Go installieren, nur um an einen Scanner zu kommen. Betroffen ist
allein `govulncheck`: es analysiert Go-Quellen und braucht die Toolchain zur Laufzeit, hat
aber keine Release-Binaries. `gosec`, `golangci-lint` und `osv-scanner` kommen deshalb aus
GitHub-Releases — `osv-scanner` als blanke Binary ohne Archiv, was das Skript am
Asset-Namen erkennt.

Ein Projekt darf selbstverständlich mit `.venv` arbeiten. Der read-only Preflight misst
dann genau dieses aktive venv und kennzeichnet den Messkontext in der Oberfläche. **Nur vor
der Installation der Security-Tools darf kein Projekt-venv aktiv sein**, damit nichts ins
Projekt-venv geschrieben wird. Falls `VIRTUAL_ENV` gesetzt ist und installiert werden soll:

```bash
deactivate
```

Empfohlen ist `--method auto`: native Binaries, Go-Tools und Python-CLI-Tools über `pipx`
oder dedizierte Tool-venvs. Wer Python-CLI-Tools grundsätzlich in venvs kapseln will,
nutzt explizit:

```bash
k-playbook/scripts/install-security-tools.sh --install missing --method venv
```

Auch das installiert nicht in `<projekt>/.venv`, sondern in dedizierte k-playbook-Tool-venvs
unter `~/.local/share/k-playbook/security-tools/<tool>-venv`. `--method venv` betrifft nur
Python-CLI-Tools; GitHub-Release- und Go-Tools nutzen weiterhin ihren nativen
Installationsweg. Je Python-Tool gibt es ein eigenes venv, damit sich ihre Abhängigkeiten
nicht in die Quere kommen; die Wurzel lässt sich mit `--venv-root` verlegen.

## Selbst bauen

Für den normalen Betrieb genügt das Release-Asset, das der Wrapper selbst lädt. Wer am
Werkzeug arbeitet oder lieber selbst baut, braucht Go:

```bash
make -C k-playbook dist        # alle Plattformen nach dist/
make -C k-playbook dist-host   # nur die Plattform dieses Rechners
k-playbook/bin/k-playbook      # startet den gebauten Stand
```

Beide Build-Targets verwenden dieselben Flags wie CI beim Bauen der Release-Assets,
damit jeder Weg bitgleiche Binaries liefert. `dist-host` spart die drei fremden
Plattformen und genügt, wenn nur dieser Rechner den Stand starten soll. Ein selbst
gebautes `dist/` hat Vorrang vor Cache und Download — und ist damit zugleich der Weg,
ganz ohne Netzzugriff zu arbeiten.

Die Installation ist schreibgeschützt; zum Bauen gibt `make -C k-playbook
installer-writable` sie frei, `installer-readonly` sperrt sie wieder. Ein Update setzt
sie ohnehin auf den Clone-Stand zurück.

`make gui` erscheint zwar in der Hilfe, gilt aber nur im Entwicklungsrepo von
k-playbook: es spielt vor dem Start einen Arbeitsstand in die Installation ein, den es
in einem Zielprojekt nicht gibt.

## Verifikation

Checkliste für ein Projekt:

- [ ] `K-PLAYBOOK.yaml` liegt im Hauptverzeichnis, nicht in `k-playbook/`.
- [ ] `schema_version: 3` ist gesetzt.
- [ ] `project.repo_root` zeigt auf das Projekt-Repository, `project.vcs` ist `git` oder `none`.
- [ ] `k-playbook/` ist ein eigener Clone und enthält nichts Projekteigenes.
- [ ] `k-playbook-local/` existiert vollständig und ist im Projekt-Repository committet —
      der Inhalt von `results/` ausgenommen, der bei einer Neuinstallation von vornherein
      draußen bleibt.
- [ ] `.claude/commands`, `.claude/skills`, `.opencode/commands` und `.cursor/commands`
      sind Verzeichnisse mit Einzel-Symlinks nach `k-playbook/` bzw. `k-playbook-local/`;
      die Oberfläche meldet sie als eingerichtet.
- [ ] `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, und `AGENTS.md` trägt den Anstoß.
      Eine mitgebrachte echte `CLAUDE.md` wurde dabei nach `AGENTS.md` umbenannt; steht
      stattdessen ein `Konflikt`, ist er von Hand aufzulösen — bis dahin sieht Claude
      Code den Anstoß nicht.
- [ ] `k-playbook/bin/k-playbook context` läuft durch und nennt die erwarteten Kataloge.

Der letzte Punkt prüft alles Vorherige auf einmal: das Kommando bricht ab, wenn die
Konfiguration fehlt oder eine andere `schema_version` trägt.

## Fehlersuche

**Slash-Commands tauchen nicht auf.** Die Oberfläche starten: sie vergleicht den
Katalog mit dem, was registriert ist, und nennt die fehlenden Commands beim Namen.
Nach dem Einrichten den Assistenten neu starten.

**Ein neuer Command aus `k-playbook-local/commands/` fehlt.** Er wird nicht automatisch
registriert — die Oberfläche meldet ihn als fehlend und legt den Link auf Knopfdruck an.
Dasselbe gilt, wenn eine projekteigene Datei einen mitgelieferten Command neuerdings
ersetzt: dann zeigt der bestehende Link noch auf die alte Quelle.

**Skills werden nicht getriggert.** Unter jedem Skill-Ordner muss `SKILL.md` liegen —
ohne sie gilt das Verzeichnis nicht als Skill und wird nicht verlinkt. Danach den
Assistenten neu starten.

**`schema_version` passt nicht.** Ist die Zahl kleiner als `3` oder fehlt sie, ist die
Konfiguration älter als das Werkzeug — die Oberfläche setzt sie zurück, siehe
[Eine Konfiguration aus einem abgelösten Modell](#eine-konfiguration-aus-einem-abgelösten-modell).
Ist sie größer, liegt es umgekehrt: die Installation ist hinterher. Dann hilft `git pull`
in `k-playbook/`, kein Zurücksetzen — das würde die neuere Datei wegwerfen.

**Das Werkzeug findet kein Projekt.** Dann fehlt die `K-PLAYBOOK.yaml` oberhalb des
Aufrufortes. Die Suche läuft ab dem Arbeitsverzeichnis aufwärts bis `$HOME` bzw. `/`
und rät bewusst nicht. Die Oberfläche schlägt dann einen Ort vor.

**Ein Assistent sieht fremde Commands.** Typisch nach einer Installation nach dem alten
Modell: die host-globalen Symlinks wirken in jedes Projekt hinein. Die Oberfläche einmal
starten, sie räumt sie weg und meldet, was entfernt wurde.

**Das Binary fehlt.** `bin/k-playbook` nennt jede Stelle, an der es gesucht hat, und die
Wege weiter: `bin/k-playbook --prefetch` lädt es in den Cache, `K_PLAYBOOK_CACHE` zeigt
auf einen vorbefüllten Cache, `make dist-host` baut es selbst. Fehlt `VERSION`, gehört zu
diesem Stand kein Release — dann hilft nur `git pull` oder ein eigener Build.
