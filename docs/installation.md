# Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt trägt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git
make -C k-playbook install
k-playbook
```

Ohne `make` geht derselbe Bootstrap direkt:

```bash
k-playbook/bin/install
```

Das Zielverzeichnis muss `k-playbook` heißen — Commands und Skills sprechen es so an.
Ohne Zielargument ergibt sich der Name aus dem Repo-Namen und stimmt damit von selbst;
ein eigenes Argument brauchst du nur, wenn du aus einem Fork oder Mirror unter
abweichendem Namen klonst. Dann lautet es `k-playbook`.

**Go wird nicht gebraucht.** `bin/install` lädt das zur Plattform passende Release-Binary
und installiert es nach `~/.local/bin/k-playbook`. Auf macOS und im DevContainer läuft
der Installer jeweils in seiner eigenen Umgebung und installiert deshalb das passende
macOS- beziehungsweise Linux-Binary.

**Die Installation braucht Netz.** Die Binaries liegen nicht im Clone, sondern als Assets
am Release. `bin/install` lädt genau das passende Asset und prüft es gegen das
mitgelieferte `SHA256SUMS`.

**`~/.local/bin` muss im PATH liegen** — das ist Voraussetzung, kein Hinweis am Rand.
Aufgerufen wird k-playbook ausschließlich unter seinem Namen; ohne den PATH-Eintrag wäre
es installiert, aber für niemanden auffindbar. Fehlt er, bricht der Bootstrap ab,
bevor er etwas lädt, und nennt die Zeile fürs Shell-Profil:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Auf Linux ist der Eintrag meist schon da. Auf macOS **nicht** — `/etc/paths` kennt ihn
nicht und `path_helper` ergänzt ihn nicht. Die Zeile gehört in `~/.zprofile` (zsh) oder
`~/.bashrc` (bash); danach eine neue Shell öffnen und den Bootstrap erneut aufrufen.

Ein Sonderfall beim allerersten Lauf auf einem frischen Host oder in einem DevContainer:
Debian und Ubuntu nehmen `~/.local/bin` in `~/.profile` nur auf, wenn das Verzeichnis
beim Anmelden bereits existiert. Der Bootstrap legt es deshalb an, **bevor** er den PATH
prüft, und sagt im Abbruch, dass am Profil nichts zu ändern ist — es genügt, sich neu
anzumelden (oder `. ~/.profile`) und den Bootstrap erneut aufzurufen.

**Host und DevContainer mit geteiltem Home.** `~/.local/bin/k-playbook` ist eine echte
Datei je Plattform. Teilen sich beide Umgebungen dasselbe `$HOME`, überschreiben sie
einander dort. `bin/install` erkennt ein plattformfremdes Binary und meldet beim
Ersetzen, dass die andere Umgebung denselben Bootstrap noch einmal braucht. Ruft man das
Binary dagegen direkt auf, während es der falschen Plattform gehört, fängt das nichts ab:
dann meldet die Shell `cannot execute binary file`. Getrennte HOMEs sind der saubere
Zustand.

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

### Konfiguration im Terminal anlegen

Wenn ein Anker eines übergeordneten Projekts die Ersteinrichtung der Oberfläche
überdeckt, lässt sich der Anker ohne Suche direkt anlegen:

```bash
k-playbook config create
```

Der Befehl schreibt in das aktuelle Verzeichnis und meldet danach den genauen Pfad,
das erkannte Repository und die Versionskontrolle. Optional kann ein anderer
Projektordner angegeben werden; liegt das Repository darin nicht im Hauptverzeichnis,
setzt `--repo-root` dessen relativen Pfad:

```bash
k-playbook config create --repo-root app /pfad/zum/projekt
```

Eine vorhandene `K-PLAYBOOK.yaml` bleibt auch auf diesem Weg unverändert.

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
├── TODO.md
└── version-sources.yaml   Versionsquellen des Versionsinventars, handgepflegt
```

Die erzeugten Docs-Herkünfte `docs/code/`, `docs/libs/`, `docs/extracted/` und
`docs/versions/` stehen nicht darin: sie entstehen beim ersten Lauf ihres Erzeugers.

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

Eingetragen wird `k-playbook mcp` — beim Schreiben aufgelöst zum **absoluten Pfad** des
installierten Binaries. Aus Dock oder Finder gestartete Clients erben die Shell-PATH
nicht; ein bloßer Kommandoname wäre dort tot. Wer den Eintrag einchecken will, trägt von
Hand den bloßen Namen `k-playbook` ein: die automatische Korrektur fasst ihn nicht an.
Beides steht in [`mcp.md`](./mcp.md#warum-der-eintrag-ein-absoluter-pfad-ist). Eine
Bedingung gilt in jedem Fall: der Eintrag wirkt nur, wenn der Assistent im
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
├── CLAUDE.md             Include-Datei mit der Zeile @AGENTS.md; die Richtung ist fest
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
mit, Cursor kennt kein Skill-Konzept. `CLAUDE.md` ist eine Include-Datei mit der
Import-Zeile `@AGENTS.md`, weil Claude Code ausschließlich `CLAUDE.md` liest und OpenCode
wie Cursor `AGENTS.md` bevorzugen — so gibt es genau eine echte Instruktionsdatei, und
Claude Code lädt sie beim Start über den Import. Das Einrichten schreibt dazu einen
kurzen Stub: über der Import-Zeile ein Hinweis, dass Projektregeln nach `AGENTS.md`
gehören, dann die Zeile `@AGENTS.md` allein auf einer Zeile.

`AGENTS.md` bekommt dabei einen kurzen **Anstoß**: einen Block, der auf
`k-playbook context` verweist. Fehlt die Datei, wird sie angelegt; ist sie da, wird der
Block angehängt und vorhandener Inhalt nicht angetastet. Ein Marker
`<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf ihn erneut anhängt.

### Eine vorhandene CLAUDE.md

Die Richtung ist fest: `CLAUDE.md` bindet `AGENTS.md` ein, nie umgekehrt. Damit daneben
keine zweite, abweichende Instruktionsdatei entsteht, ordnet das Einrichten das Paar
`CLAUDE.md`/`AGENTS.md` zuerst ein und löst auf, was sich auflösen lässt:

| Ausgangslage | Was geschieht |
|---|---|
| `CLAUDE.md` trägt die Zeile `@AGENTS.md`, `AGENTS.md` ist eine echte Datei | der **Sollzustand** — nichts zu tun. Was neben der Import-Zeile steht, gehört dem Projekt und bleibt unangetastet |
| `CLAUDE.md` ist noch ein Symlink auf `AGENTS.md` | die ältere Bauform: der Symlink wird **verlustfrei ersetzt**, der Inhalt steht ohnehin in `AGENTS.md` — auch auf dem Lesepfad, siehe unten |
| nur eine echte `CLAUDE.md` ohne Import-Zeile | sie wird nach `AGENTS.md` **umbenannt**, der Anstoß an den erhaltenen Inhalt angehängt, `CLAUDE.md` neu als Include-Datei angelegt |
| nur die Include-Datei, `AGENTS.md` fehlt | sie bleibt liegen, `AGENTS.md` entsteht aus der Vorlage. Umbenannt ergäbe sie ein `AGENTS.md`, das sich selbst importiert |
| `AGENTS.md` ist ein Symlink auf `CLAUDE.md` | die verdrehte Richtung wird aufgelöst: Symlink weg; eine echte `CLAUDE.md` wird umbenannt, eine Include-Datei bleibt liegen; dann Include neu anlegen bzw. `AGENTS.md` aus der Vorlage |
| `AGENTS.md` ist ein toter Symlink | er wird entfernt, damit die Datei nicht an seinem Ziel landet |
| beide sind echte Dateien, `CLAUDE.md` ohne wirksame Import-Zeile | **Konflikt** — ob der Inhalt für alle Assistenten gilt oder nur für Claude Code, entscheidet das Projekt: entweder den Inhalt nach `AGENTS.md` übernehmen und `CLAUDE.md` auf die Zeile `@AGENTS.md` reduzieren, oder die Zeile `@AGENTS.md` vor den vorhandenen Inhalt setzen und ihn dort stehen lassen. Automatisch geschieht keines von beiden |
| `CLAUDE.md` zeigt bewusst auf ein anderes Ziel | **Konflikt** — der Link des Projekts bleibt stehen, sonst wären die dort gelesenen Instruktionen ab sofort unwirksam |
| `AGENTS.md` zeigt bewusst auf ein anderes Ziel | eine Entscheidung des Projekts, kein Fehler: die Include-Datei wirkt durch den Link hindurch, dort kommt auch der Anstoß an. Trägt `CLAUDE.md` daneben eigenen Inhalt ohne Import-Zeile, ist das ein **Konflikt** |
| `AGENTS.md` ist in git ignoriert | **Konflikt** — sonst fiele der Inhalt still aus der Versionskontrolle; Ignore-Regel entfernen und neu einrichten |

**Wirksam** ist die Import-Zeile nur außerhalb von Backticks und Code-Blöcken — dort
überliest Claude Code sie beim Import-Parsing. Eine `CLAUDE.md`, die `@AGENTS.md` nur
in Backticks oder in einem Code-Block nennt, gilt deshalb als Datei ohne Import und
landet im Konflikt, nicht im Sollzustand.

Bei einem Konflikt wird nichts verschoben, nichts gelöscht, nichts gesichert und auch
kein `AGENTS.md` angelegt. Das ist kein Schönheitsfehler: solange er steht, sieht Claude
Code vom Einrichten nichts, weil er ausschließlich `CLAUDE.md` liest. Die
Assistenten-Karte meldet den Zustand als `Konflikt` und nennt den Ausweg im Detailtext.

Derselbe Ablauf läuft beim **Aktualisieren**. Ein Projekt, das nur eine echte
`CLAUDE.md` hat, wird also auch darüber eingerichtet, und ein Projekt ganz ohne
`AGENTS.md` bekommt sie dabei erstmals.

**Bestandsprojekte** migrieren sich von selbst. Der erste `k-playbook context` — oder
der Assistenten-Block der Oberfläche — ersetzt den alten Symlink durch die Include-Datei
und sagt das in `links.note`. Danach zeigt `git status` einmalig eine geänderte
`CLAUDE.md` mit gewechseltem Modus (Symlink → reguläre Datei, `120000` → `100644`); die
Änderung gehört committet. Es ist die eine Stelle, an der ein reiner Lesepfad eine
versionierte Datei im Hauptverzeichnis ändert, und genau so wird sie benannt. Kein
Include ins Leere: solange `AGENTS.md` fehlt, wartet ein alter Symlink auf sein Ziel und
wird erst ersetzt, wenn es da ist. Steht dagegen die Include-Datei und `AGENTS.md` fehlt,
nennt der Detailtext den Import ins Leere — Claude Code lädt daraus nichts, bis
**Einrichten** `AGENTS.md` anlegt.

**Der Preis der Trennung.** Ohne Symlink sind es zwei Dateien, die auseinanderlaufen
können: was Claude Code über `/memory` oder `#` nach `CLAUDE.md` schreibt, erreicht
OpenCode und Cursor nicht, und die Prüfung meldet Projektinhalt neben dem Include nicht
als Konflikt. Das ist bewusst so — die Gegenrichtung hieße, jeden Inhalt neben dem
Include zum Konflikt zu erklären. Als Gegengewicht steht im Stub über der Import-Zeile,
dass Projektregeln nach `AGENTS.md` gehören.

**Was Claude Code dafür können muss.** `@`-Importe in `CLAUDE.md` gibt es seit Claude
Code **0.2.107** (CHANGELOG, Eintrag „CLAUDE.md files can now import other files"); eine
ältere Fassung lädt aus dem Stub still gar nichts. Die Importtiefe ist begrenzt: laut
Dokumentation folgt Claude Code Importen bis zu **vier Ebenen** tief („Imported files
can recursively import other files, with a maximum depth of four hops", code.claude.com,
Seite „How Claude remembers your project", Stand 2026-09-05). Der Stub verbraucht davon
eine Ebene — ein `AGENTS.md`, das selbst mit `@` importiert, hat gegenüber dem früheren
Symlink eine Ebene weniger.

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

Beim Start der Oberfläche steht die URL im Terminal, und der Browser wird geöffnet. Der
Server dahinter läuft als Hintergrunddienst je Projekt weiter: der Aufruf kehrt zurück,
sobald der Browser offen ist, und ein zweiter Aufruf im selben Projekt öffnet nur ein
weiteres Fenster auf denselben Server. Beendet wird er über `Dienst beenden` in der
Oberfläche oder `k-playbook stop`; ohne jede Anfrage beendet er sich nach 60 Minuten von
selbst. Welches Programm den Browser öffnet, entscheidet sich in dieser Reihenfolge:

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

Der Bereich **Workflows** führt die Arbeitsvorräte zusammen: Review-Läufe aus
`k-playbook-local/results/`, Tasks aus `k-playbook-local/tasks/` und Todos aus
`k-playbook-local/TODO.md`. Jede Liste trägt ihre eigene Zahl.

Ein Beschreibungsblock steht voran und sagt, was die drei Sorten sind und wann man
welche nimmt. Darunter stehen die bisherigen Läufe, dann die offenen Tasks nach ihrer
Nummer; ein Klick zeigt den Task als Markdown unter der Liste. Die erledigten aus
`tasks/done/` stehen in einem zugeklappten Block — die jüngste Nummer oben — und lassen
sich genauso lesen. Den Schluss machen die Todos, offene und abgehakte getrennt.

Rechts an jeder Task-Zeile steht, ob sie schon durch `/k-task-refine` gegangen ist —
mit Datum, sofern das Review-Log eines nennt. „ohne Task-Refine" ist kein Fehler, aber
der Grund, warum `/k-task-run` vor der Ausführung nachfragt.

Gelesen wird nur. Angelegt und ausgeführt werden Tasks über `/k-task-create` und
`/k-task-run` im Assistenten.

## Doku lesen

Der Bereich **Docs** zeigt alle Markdown-Dateien aus `k-playbook/docs` — dieselbe Doku,
die du gerade liest, in dem Stand, der im Projekt installiert ist. Der Index steht links
im Menü, die geöffnete Datei rechts; ohne Auswahl steht dort die `README.md`. Verweise im
Text führen zur nächsten Datei, Anker springen innerhalb der offenen.

Mermaid-Diagramme werden gezeichnet, sofern der Rechner ins Netz kommt: die Library wird
bei Bedarf geladen. Ohne Netz bleibt der Diagramm-Quelltext stehen, der Text ist
weiterhin vollständig lesbar.

## Nachsehen, was gilt

Ganz unten steht der Block **Aufgelöster Kontext**. Aufgeklappt zeigt er, was ein
Command sieht: die aufgelösten Pfade, die Instruktionsdateien in Lesereihenfolge, die
effektiven Kataloge für Regeln, Reviews und Checks samt Herkunft — mitgeliefert,
projekteigen oder ersetzt — und die Guidelines. Abgeschaltete Einträge stehen mit, damit
sichtbar bleibt, dass es sie gibt.

Es ist dieselbe Auskunft wie `k-playbook context`, nur lesbar aufbereitet.
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

Das Make-Target entspricht `chmod -R u+w k-playbook && git -C k-playbook fetch origin && git -C k-playbook reset --hard origin/main && git -C k-playbook clean -fd && chmod -R a-w k-playbook` und sperrt die Installation auch dann wieder, wenn der Pull fehlschlägt. Der harte Reset ist bewusst: `k-playbook/` trägt per Vertrag keine lokalen Änderungen; alles darin darf nur durch Pull entstehen. Im Entwicklungsrepo funktioniert zusätzlich `make installer-update`, weil dort der Installations-Clone unter `./k-playbook/` liegt.

**Der Make-Weg aktualisiert den Clone, nicht das Projekt daneben.** Er ist eine reine
Git-Kette und läuft absichtlich ohne Go und ohne das installierte Binary — das ist sein
Zweck: er muss auch dann noch funktionieren, wenn im Projekt gar nichts anderes läuft.
Was im Hauptverzeichnis liegt und nicht im Clone — die MCP-Registrierung, der
Anstoßblock in `AGENTS.md` — erreicht er deshalb nicht.

Nachgezogen wird das beim **nächsten Aufruf von `k-playbook`**. Der Start ist der zweite,
allgemeine Auffangweg: er korrigiert eine veraltete MCP-Registrierung von selbst und
meldet, was er getan hat. Das gilt für jeden Weg, der am Update-Knopf der Oberfläche
vorbeigeht — auch für ein `git pull` von Hand. Nach einem Update von Hand also einmal:

```bash
k-playbook
```

Das ist kein zusätzlicher Handgriff im Alltag, sondern derselbe Aufruf, mit dem eine
Sitzung ohnehin beginnt.

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

Hat dabei `VERSION` gewechselt, gehört zu dem neuen Stand ein anderes Binary. Der Dienst
beendet sich dann nach der Antwort selbst. Installiert wird das neue Binary ausdrücklich
über den Bootstrap — `make -C k-playbook install`, ohne make `k-playbook/bin/install` —;
der Update-Pfad lädt oder ersetzt von sich aus kein Host-Binary. Danach startet
`k-playbook` die neue Fassung. Nach einem `git pull` von Hand oder
`make -C k-playbook installer-update` läuft ein alter Dienst zunächst weiter; erkannt und
ersetzt wird er erst, wenn der nächste Aufruf von `k-playbook` aus einem neu installierten
Binary kommt. Verglichen wird dabei die Binärdatei selbst und nicht nur ihre Version — ein
neu gebautes Binary derselben `VERSION` löst den alten Dienst also ebenso ab. Sind nur Commands, Regeln oder Rezepte neu, ändert sich
`VERSION` nicht: der Dienst läuft weiter, `Neu einlesen` in der Oberfläche holt den Stand,
und ein Neustart des Assistenten genügt.

**Das Übergangsfenster beim Wechsel auf die direkte Installation.** Ein Projekt, das noch
nach dem abgelösten Wrapper-Modell eingerichtet ist, braucht die Schritte in dieser
Reihenfolge — und dazwischen liegt ein Fenster, in dem nichts läuft:

1. **Zuerst den Clone aktualisieren.** Vorher gibt es `k-playbook/bin/install` in diesem
   Projekt gar nicht; der Bootstrap ist erst nach dem Update vorhanden.
2. **Danach der Bootstrap**, einmal je Host und einmal je DevContainer:
   `make -C k-playbook install`, ohne make `k-playbook/bin/install`.

Zwischen beiden Schritten zeigen die Commands, der Anstoßblock in `AGENTS.md` und die
MCP-Registrierung noch auf `k-playbook/bin/k-playbook` — die Datei, die das Update
entfernt hat. In diesem Fenster gibt es kein funktionierendes Kommando, auch keinen
Ersatzaufruf: die beiden selbsttätigen Korrekturwege laufen im installierten Binary und
setzen den Bootstrap deshalb voraus. Er ist der eine verbleibende Handgriff; danach zieht
der nächste Aufruf von `k-playbook` Registrierung und Anstoßblock von selbst nach.

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

## Ein Werkzeug für alle Projekte

Nach dem Bootstrap genügt überall:

```bash
cd /pfad/zum/projekt
k-playbook
```

Es ist dasselbe Werkzeug für alle Projekte. Welches Projekt gemeint ist, ergibt sich aus
dem Verzeichnis, in dem der Aufruf stattfindet — nicht aus dem Ort des Programms. Der
Clone unter `k-playbook/` ist reine Inhaltsquelle: Commands, Skills, Regeln, Reviews,
Checks und Doku. Ein zweiter Einstiegspunkt im Projekt selbst existiert nicht mehr;
aufgerufen wird k-playbook ausschließlich unter seinem Namen, und die Commands tun
dasselbe.

`~/.local/bin/k-playbook` ist eine echte Datei, kein Symlink und keine Auflösung zur
Laufzeit. Jede Arbeitsumgebung installiert die Fassung ihrer eigenen Plattform: der
macOS-Host ein Darwin-Binary, der DevContainer ein Linux-Binary. Ein DevContainer
bootstrappt deshalb einmal für sich selbst, nach einem Rebuild erneut.

**Eine eigene DevContainer-Integration gibt es nicht mehr** — keinen Bind-Mount nach
`/workspaces/k-playbook`, keinen Symlink im Container und kein Setup-Skript in
`.devcontainer/`. Die Installation liegt im Projektverzeichnis und kommt mit ihm in den
Container, wie jede andere Projektdatei auch.

**Ein Stand, ein Binary.** `VERSION` im Wurzelverzeichnis des Clones nennt das Release,
dessen Assets zu diesem Clone-Stand gehören; `SHA256SUMS` daneben trägt deren Prüfsummen.
Commits an Regeln, Reviews, Commands oder Docs ändern beide Dateien nicht. Wechselt
`VERSION` beim Update, gehört zum neuen Stand ein anderes Binary — installiert wird es
ausdrücklich über den Bootstrap, nie nebenbei.

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
oder user-lokal installiert, nie in ein Projekt-venv. Sie sind eine von zwei bewussten
Ausnahmen von der Projektlokalität: ein Scanner gehört zur Arbeitsumgebung, nicht zum
Projekt. Die zweite Ausnahme sind die Basis-Werkzeuge, siehe unten.

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
| `ruff` | Python | Python-Qualität und flake8-bandit-Regeln |
| `semgrep` | Python, Go, JS/TS | generische Security-Regeln |
| `osv-scanner` | Python, Go, JS/TS | Dependency-CVEs mit SARIF |
| `gosec` | Go | Go-Security |
| `govulncheck` | Go | Go-CVEs mit Reachability |
| `njsscan` | JS/TS | Node-/JS-Security |

Optional, weil sie sich mit anderen überschneiden: `golangci-lint` (Go-Qualität, bündelt
staticcheck und errcheck). `docker` ist ebenfalls optional und wird als Fallback-Kontext
angezeigt, aber nicht durch k-playbook installiert.

`bandit` steht bewusst in keiner der beiden Listen: `ruff` deckt es ab, sein
`S`-Regelwerk *ist* flake8-bandit. Ein zweites Werkzeug für dieselben Regeln brächte nur
doppelte Befunde.

**JavaScript und TypeScript sind zwei getrennte Sprachen** in der Matrix, keine
gemeinsame. `AppliesTo` kostet das nichts, aber die Kandidatenzählung leitet aus
derselben Angabe ab, welche Endungen zählen — mit nur `javascript` bekäme ein reines
TypeScript-Projekt eine 0 auf seinen `.ts`-Dateien. Ein Projekt, das beides hat, nennt
beide.

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

## Basis-Werkzeuge

Die zweite bewusste Ausnahme von der Projektlokalität, und eine andere Sorte als die
Security-Tools: Basis-Werkzeuge sind keine Scanner, sondern der Boden, auf dem die
Commands stehen — `bash`, `git`, `curl` oder `wget`, `tar`, `python3` und `rg`. Von
`curl` und `wget` genügt eines.

Die Matrix liegt in [`../scripts/base-tools.tsv`](../scripts/base-tools.tsv), getrennt von
der Security-Matrix: `scanners.tsv` referenziert jene über die Spalte `tool`, und ein `rg`
darin erschiene in jedem Review-Lauf als übersprungener Eintrag.

**Ein fehlendes Basis-Werkzeug warnt, es blockiert nicht.** `k-playbook context` meldet
den Zustand unter `baseTools`; ein Command benennt die Lücke, nimmt einen Rückfall, wo es
einen gibt, und läuft weiter. Das unterscheidet sie von `gh`, dessen Fehlen ein PR-Review
hart beendet.

Zustand ansehen und installieren:

```bash
k-playbook/scripts/install-base-tools.sh --preflight
k-playbook/scripts/install-base-tools.sh --install
```

Das Skript entscheidet je Werkzeug: Als root mit `apt-get` installiert es systemweit über
den Paketmanager. Sonst geht es den user-lokalen Weg aus einem GitHub-Release — heute
trifft das allein `rg`, und dieser Weg braucht keinen root. Für `git`, `curl`, `wget`,
`tar` und `python3` gibt es keinen sinnvollen user-lokalen Weg; dort gibt das Skript den
`sudo apt-get`-Befehl aus und endet mit dem Rückgabewert `3`, der „für dieses Werkzeug
gibt es hier keinen Weg" vom Fehlschlag trennt.

**k-playbook eskaliert nie selbst zu root.** Der `sudo`-Befehl wird gezeigt, nie
ausgeführt, und das Skript startet sich nicht per `sudo` neu.

Das Ziel des user-lokalen Wegs lässt sich mit `--prefix` und `--bin-dir` verlegen, dazu
über `K_BASE_TOOLS_PREFIX` und `K_BASE_TOOLS_BIN_DIR`. Ein schreibender Aufruf, dessen
aufgelöstes Ziel nicht dem ausführenden Benutzer gehört, wird abgewiesen — das fängt den
`sudo`-Tippfehler ab, der sonst Binaries mit falschem Eigentümer hinterließe.

### Für ein Dockerfile oder einen DevContainer

`--yes` schaltet jede Rückfrage ab, damit eine einzelne RUN-Zeile unbeaufsichtigt
durchläuft. Als root mit `apt-get` — der Normalfall im Image-Build — installiert sie alles
systemweit:

```dockerfile
RUN bash /opt/projekt/k-playbook/scripts/install-base-tools.sh --install --yes
```

Hat ein Werkzeug auf diesem Host keinen Weg, endet der Lauf mit `3`. Das ist kein
Fehlschlag, aber `docker build` bricht darauf ab. Wer das nicht will, hängt
`|| test $? -eq 3` an; wer die Lücke beim Bauen sehen will, lässt es stehen.

## Selbst bauen

Für den normalen Betrieb genügt das Release-Asset, das `bin/install` lädt. Wer am
Werkzeug arbeitet oder lieber selbst baut, braucht Go:

```bash
make -C k-playbook dist         # alle Plattformen nach dist/
make -C k-playbook dist-host    # nur die Plattform dieses Rechners
make -C k-playbook dev-install  # baut diese Plattform und installiert sie
```

Alle Build-Targets verwenden dieselben Flags wie CI beim Bauen der Release-Assets, damit
jeder Weg bitgleiche Binaries liefert. `dist-host` spart die drei fremden Plattformen und
genügt, wenn nur dieser Rechner den Stand starten soll. Gestartet wird ein selbst gebautes
Binary nicht von allein: `dev-install` legt es nach `~/.local/bin/k-playbook`, und erst
danach nimmt der Aufruf `k-playbook` es auf. Das ist zugleich der Weg, ganz ohne
Netzzugriff zu arbeiten — gebaut statt geladen.

Die Installation ist schreibgeschützt; zum Bauen gibt `make -C k-playbook
installer-writable` sie frei, `installer-readonly` sperrt sie wieder. Ein Update setzt
sie ohnehin auf den Clone-Stand zurück.

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
- [ ] `CLAUDE.md` ist eine reguläre Datei mit der Zeile `@AGENTS.md` außerhalb von
      Backticks und Code-Blöcken, und `AGENTS.md` trägt den Anstoß. Eine mitgebrachte
      echte `CLAUDE.md` wurde dabei nach `AGENTS.md` umbenannt, ein Symlink aus einer
      älteren Fassung durch die Include-Datei ersetzt; steht stattdessen ein `Konflikt`,
      ist er von Hand aufzulösen — bis dahin sieht Claude Code den Anstoß nicht.
- [ ] `k-playbook context` läuft durch und nennt die erwarteten Kataloge.

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

**`k-playbook: command not found`.** Entweder ist der Bootstrap in dieser Umgebung noch
nicht gelaufen — dann `make -C k-playbook install` (ohne make: `k-playbook/bin/install`)
—, oder `~/.local/bin` fehlt im PATH. Der Bootstrap prüft den PATH selbst und bricht in
diesem Fall ab, bevor er etwas lädt.

**`cannot execute binary file`.** Unter `~/.local/bin/k-playbook` liegt das Binary einer
anderen Plattform — typisch bei Host und DevContainer mit geteiltem `$HOME`. Den
Bootstrap in dieser Umgebung erneut ausführen; er erkennt den Fall und meldet, dass die
andere Umgebung ihn danach ebenfalls noch einmal braucht.

**Der Bootstrap findet kein Asset.** Fehlt `VERSION` im Clone, gehört zu diesem Stand
kein Release — dann hilft `git pull` in `k-playbook/` oder ein eigener Build, siehe
[Selbst bauen](#selbst-bauen).
