# Installation

k-playbook wird in das Projekt geklont, das es begleiten soll. Es gibt keine zentrale
Installation und keinen festen Hostpfad; jedes Projekt traegt seine eigene.

```bash
cd /pfad/zum/projekt
git clone git@github.com:kascada/k-playbook.git k-playbook
k-playbook/bin/k-playbook
```

Das Argument hinter der URL bestimmt den Verzeichnisnamen. Es muss `k-playbook` lauten —
Commands und Skills sprechen das Verzeichnis so an.

**Go wird nicht gebraucht.** `bin/k-playbook` ist ein Wrapper, der das zur Plattform
passende Binary aus `dist/` startet; die Binaries liegen fertig im Repo. Fuer macOS und
Linux gleichermassen, was auch den Fall abdeckt, dass Host und Container unterschiedliche
Plattformen sind.

## Die drei Schritte

Der letzte Aufruf startet die Oberflaeche im Browser. Sie fuehrt durch drei Schritte und
schreibt jeden erst nach Bestaetigung.

### 1. Konfiguration anlegen

Beim ersten Mal findet die Oberflaeche noch keine `K-PLAYBOOK.yaml` — nach einem frischen
Clone kann es sie nicht geben. Statt zu raten, schlaegt sie einen Ort vor und laesst ihn
bestaetigen. Kandidaten in dieser Reihenfolge:

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

Eine vorhandene `K-PLAYBOOK.yaml` wird nie ueberschrieben. Das Format steht in
[`k-playbook-format.md`](./k-playbook-format.md).

### 2. Projekteigene Struktur anlegen

Daneben entsteht `k-playbook-local/` mit allem, was dem Projekt gehoert:

```text
k-playbook-local/
├── rules/         Overlay zu k-playbook/rules/
├── reviews/       Overlay zu k-playbook/reviews/
├── checks/        Overlay zu k-playbook/checks/
├── commands/      Overlay zu k-playbook/commands/
├── skills/        Overlay zu k-playbook/skills/
├── results/       alles, was Reviews erzeugen
├── docs/          Projektwissen fuer AI-Sessions
├── guidelines/
├── tasks/done/
├── priv/          Inhalt gitignored, Verzeichnis versioniert
├── k-playbook.md  projekteigene Instruktionsebene
└── TODO.md
```

Jedes Verzeichnis traegt eine `README.md` mit seinem Zweck — auch weil Git leere
Verzeichnisse nicht speichert und sie sonst nach einem Clone des Projekts fehlen wuerden.
Vorhandene Dateien bleiben unberuehrt, auch READMEs mit eigenem Text.

`k-playbook-local/` gehoert ins Repository des Projekts und wird committet.

### 3. Assistenten verlinken

Verlinkt wird fuer Claude Code, OpenCode und Cursor:

```text
projekt/
├── AGENTS.md             Instruktionen, eine Quelle fuer alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md
├── .claude/
│   ├── commands/         je ein Symlink pro Command
│   └── skills/           je ein Symlink pro Skill; OpenCode liest hier mit
├── .opencode/
│   └── commands/
└── .cursor/
    └── commands/
```

Die vier Ziele sind **echte Verzeichnisse mit Einzel-Symlinks**, kein Verzeichnis-Symlink.
Ein Verzeichnis-Symlink zeigt auf genau eine Quelle; damit kaeme entweder nur die
Installation oder nur `k-playbook-local/` an. Jeder Link zeigt auf die Fassung, die nach
der Overlay-Regel gilt:

```text
.claude/commands/
  k-todo.md    -> ../../k-playbook/commands/k-todo.md          mitgeliefert
  k-review.md  -> ../../k-playbook-local/commands/k-review.md  projekteigen, ersetzt
  k-eigen.md   -> ../../k-playbook-local/commands/k-eigen.md   nur projekteigen
```

Die Oberflaeche vergleicht diesen Soll-Stand mit dem, was tatsaechlich registriert ist,
und meldet Abweichungen mit Namen: was fehlt, was auf die falsche Quelle zeigt, was
verwaist ist, und was dem Projekt gehoert und deshalb liegen bleibt. Auf Knopfdruck wird
es angeglichen. Eine echte Datei, die jemand selbst dort abgelegt hat, gewinnt immer und
wird nie ersetzt.

Skills stehen nur einmal unter `.claude/skills`: OpenCode durchsucht dieses Verzeichnis
mit, Cursor kennt kein Skill-Konzept. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, weil
Claude Code ausschliesslich `CLAUDE.md` liest und OpenCode `AGENTS.md` bevorzugt — so
landet jede Aenderung in beiden.

`AGENTS.md` bekommt dabei einen kurzen **Anstoss**: einen Block, der auf
`k-playbook context` verweist. Fehlt die Datei, wird sie angelegt; ist sie da, wird der
Block angehaengt und vorhandener Inhalt nicht angetastet. Ein Marker
`<!-- k-playbook:anstoss -->` verhindert, dass ein zweiter Lauf ihn erneut anhaengt.

Was ein Assistent darueber hinaus lesen soll, steht nicht in `AGENTS.md`, sondern in
`k-playbook.md` — je einmal pro Ebene:

| Datei | Gilt fuer | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge; die projekteigene Ebene ergaenzt die mitgelieferte
oder ueberstimmt sie.

Die Verlinkung ist projektlokal. Es wird nichts in `~/.config/opencode/` oder
`~/.claude/` geschrieben. Dadurch kann ein Rechner mehrere Projekte mit
unterschiedlichen k-playbook-Staenden tragen, ohne dass sie sich gegenseitig
ueberschreiben.

**Altlasten werden entfernt.** Auf Rechnern mit einer Installation nach dem alten Modell
liegen noch host-globale Symlinks unter `~/.claude/commands`, `~/.claude/skills` und
`~/.config/opencode/command`, dazu ein `skills.paths`-Eintrag in der
OpenCode-User-Config. Die wirken in jedes Projekt hinein — ein Assistent saehe dort
zusaetzlich die Commands eines fremden Standes. `k-playbook` entfernt sie bei jedem Start,
aber nur, was nachweislich zu einem k-playbook gehoert. Faellt etwas weg, meldet es das im
Terminal; sonst bleibt es still.

Nach Aenderungen an Commands oder Skills muss der jeweilige Assistent neu gestartet
werden — beide erfassen sie beim Start.

## Doku lesen

Ueber dem Kontext-Block steht **Dokumentation**. Die Karte listet alle Markdown-Dateien
aus `k-playbook/docs` — dieselbe Doku, die du gerade liest, in dem Stand, der im Projekt
installiert ist. Ein Klick oeffnet die Datei in einem Fenster ueber der Seite; Verweise
darin fuehren zur naechsten Datei, `Escape` oder ein Klick daneben schliesst.

Mermaid-Diagramme werden gezeichnet, sofern der Rechner ins Netz kommt: die Library wird
bei Bedarf geladen. Ohne Netz bleibt der Diagramm-Quelltext stehen, der Text ist
weiterhin vollstaendig lesbar.

## Nachsehen, was gilt

Ganz unten steht der Block **Aufgelöster Kontext**. Aufgeklappt zeigt er, was ein
Command sieht: die aufgeloesten Pfade, die Instruktionsdateien in Lesereihenfolge, die
effektiven Kataloge fuer Regeln, Reviews und Checks samt Herkunft — mitgeliefert,
projekteigen oder ersetzt — und die Guidelines. Abgeschaltete Eintraege stehen mit, damit
sichtbar bleibt, dass es sie gibt.

Es ist dieselbe Auskunft wie `k-playbook/bin/k-playbook context`, nur lesbar aufbereitet.
Der Block laedt erst beim Aufklappen und veraendert nichts.

## Aktualisieren

Der bequeme Weg ist die Oberflaeche. Sie prueft nach dem Start per `git ls-remote`, ob
die Installation hinter dem Remote liegt, und zieht auf Knopfdruck per
`git pull --ff-only` nach. Bewusst `ls-remote` statt `fetch`: die Pruefung laeuft
ungefragt und darf den Zustand des Repositorys nicht anfassen. Bewusst `--ff-only`: ein
Merge im Clone erzeugte eine lokale Historie, die niemand pflegt.

Von Hand geht es genauso:

```bash
cd /pfad/zum/projekt/k-playbook
git pull --ff-only
```

`k-playbook/` enthaelt nichts Projekteigenes und ist dadurch vollstaendig ersetzbar —
auch per `rm -rf` und neuem Clone. `K-PLAYBOOK.yaml` und `k-playbook-local/` liegen
daneben und bleiben unberuehrt.

**Wurde dort trotzdem lokal gearbeitet, sagt die Oberflaeche es und aktualisiert nicht.**
Der Block `Installation` erscheint nur in diesem Fall, nennt die betroffenen Dateien und
gibt den Befehl zum Zuruecksetzen aus; ausgefuehrt wird er nicht von selbst. Der Grund
fuer die Pruefung ist, dass der Fehler sich sonst versteckt: aendert sich eine lokal
veraenderte Datei upstream nicht mit, laeuft `git pull` sauber durch und laesst sie
stehen — die Aenderung ueberlebt dann jedes Update, ohne je aufzufallen. Denselben Befund
meldet `/k-status` in der Zeile `Installation:`, auch ohne dass jemand die Oberflaeche
oeffnet.

Haben sich dabei die Binaries unter `dist/` geaendert, verlangt die Oberflaeche einen
Neustart: unter Linux behaelt ein laufender Prozess seinen Inode und arbeitet mit dem
alten Code weiter, auch wenn die Datei ersetzt wurde. Sind nur Commands, Regeln oder
Rezepte neu, genuegt ein Neustart des Assistenten.

**Die Verlinkung zieht die Oberflaeche dabei selbst nach.** Weil Commands und Skills
einzeln verlinkt sind, kommt ein neu mitgelieferter Command nicht von allein an — nach
dem Pull richtet sie die Registrierung neu aus und meldet, was sich geaendert hat
(`Verlinkung nachgezogen: 3 dazugekommen, 1 entfernt.`). Wer von Hand `git pull` macht,
startet die Oberflaeche danach einmal oder drueckt im Assistenten-Block auf Einrichten.

Damit die neuen Commands im Assistenten ankommen, muss dieser danach neu gestartet
werden — Claude Code, OpenCode und Cursor erfassen sie beim Start.

### Bestehende Projekte: zwei Dinge kommen dazu

Wer ein Projekt aus einer Fassung bis 0.4 aktualisiert, findet nach dem Update zweierlei
vor. Beides erledigt ein Klick, gelöscht oder überschrieben wird nichts:

| Wo | Was die Oberflaeche meldet | Was zu tun ist |
|---|---|---|
| Projekteigene Struktur | `Fehlende Eintraege: commands, skills` | **Anlegen** — die beiden Overlay-Verzeichnisse entstehen mit ihrer README |
| Assistenten-Verlinkung | `Verzeichnis-Symlink aus einer aelteren Fassung` | **Einrichten** — der Symlink wird durch Einzel-Links ersetzt |

Der zweite Punkt ist die eigentliche Umstellung: aus `.claude/commands -> ../k-playbook/commands`
wird ein echtes Verzeichnis mit einem Link je Command. Die Quelle in `k-playbook/` bleibt
dabei unangetastet.

Die Einzel-Links gehören ins Repository des Projekts und werden committet — dann hat ein
frischer Clone die Commands sofort registriert.

## Host-weit aufrufbar

Der tiefe Pfad `k-playbook/bin/k-playbook` ist nur beim ersten Mal noetig. Jeder Start der
Oberflaeche legt eine host-weite Kopie an und verlinkt sie:

```text
~/.local/
├── bin/k-playbook -> ../share/k-playbook/installation/bin/k-playbook
└── share/k-playbook/
    ├── installation/{bin,dist}   die gespiegelte Installation
    └── security-tools/           Tool-venvs, davon unberuehrt
```

Danach genuegt ueberall:

```bash
cd /pfad/zum/projekt
k-playbook
```

Es ist dasselbe Werkzeug fuer alle Projekte. Welches Projekt gemeint ist, ergibt sich aus
dem Verzeichnis, in dem der Aufruf stattfindet — nicht aus dem Ort des Programms.

Gespiegelt wird nur die eigene Plattform und nur, wenn der Clone einen neueren Stand
mitbringt als die Kopie. Wer in einem Projekt `git pull` macht und dort startet, hebt die
host-weite Kopie damit an. Umgekehrt ueberschreibt ein aelterer Clone sie nicht.

Ein DevContainer bekommt seine eigene Kopie unter seinem eigenen Home; nach einem Rebuild
stellt der naechste Start sie wieder her. Auf einem Mac mit Container liegen beide
Plattformen nebeneinander, falls `~/.local` geteilt ist.

**Zum PATH:** Auf Linux ist `~/.local/bin` meist schon drin. Auf macOS **nicht** —
`/etc/paths` kennt es nicht und `path_helper` ergaenzt es nicht.

Fehlt es, zeigt die Oberflaeche ganz oben die Karte **Aufruf von ueberall** mit der Zeile
zum Kopieren:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Die Zeile gehoert ins Shell-Profil — `~/.zprofile` bei zsh (Standard seit Catalina),
`~/.bashrc` bei bash. Danach eine neue Shell oeffnen; geprueft wird der `PATH` des
laufenden Prozesses, eine gerade eingetragene Zeile sieht er noch nicht.

**Geschrieben wird dort nichts von selbst.** Das Profil gehoert dir. Steht der Aufruf,
verschwindet die Karte wieder — dieselbe Zeile steht ausserdem beim Start im Terminal.

Der Aufruf ueber `k-playbook/bin/k-playbook` im Projekt bleibt jederzeit moeglich und
gleichwertig. Die Commands nutzen ausschliesslich ihn, nie den `PATH`.

## GitHub CLI

`/k-pr-review` und das Dependabot-Review arbeiten ueber `gh`. Die Karte **GitHub CLI**
haelt zwei Dinge auseinander, die leicht durcheinandergeraten.

Das eine ist die **Entscheidung des Projekts**: nutzt es `gh` oder nicht. Sie wird hier
gesetzt und landet in `K-PLAYBOOK.yaml` unter `tools.gh.status`. Bis sie faellt, steht sie
auf `unknown`, und die Karte zeigt das rot — nicht als Schoenheitsfehler, sondern weil ein
Command sonst nicht weiss, ob ein fehlendes `gh` ein Problem oder gewollt ist. Commands,
die `gh` brauchen, brechen bei `unknown` ab.

Das andere ist der **Befund fuer diesen Rechner**: liegt `gh` im PATH, und ist ein Account
hinterlegt. Der steht nur in der Karte und in der Kontextausgabe, nie in der
Konfiguration — auf dem naechsten Rechner ist er ein anderer.

Installiert und angemeldet wird im Terminal, wie bei den Security-Tools: beides veraendert
den Host, und `gh auth login` will einen Browser. Die Karte zeigt dafuer den passenden
Befehl.

```bash
gh auth login --hostname github.com   # anmelden
gh auth status                        # Token beim Server pruefen
```

Der Befund ist aus `~/.config/gh/hosts.yml` gelesen und **nicht beim Server geprueft**:
ein hinterlegter Token kann abgelaufen oder zurueckgezogen sein. Wer Gewissheit braucht,
ruft `gh auth status` auf.

Sind mehrere Accounts hinterlegt, nennt die Karte sie und zeigt den Umschaltbefehl:

```bash
gh auth switch --hostname github.com --user <account>
```

Bewusst als Befehl und nicht als Knopf. Der Wechsel gilt fuer jedes Terminal und jedes
Projekt auf diesem Rechner, nicht nur fuer dieses — und ein Approve oder Merge laeuft
danach unter dem neuen Namen. `/k-pr-review` nennt den aktiven Account deshalb vor jeder
Schreibaktion.

Nur `github.com`. Enterprise-Instanzen haetten eigene Accounts je Host und eine eigene
Entscheidung je Projekt; das waere etwas anderes als das hier.

## Security-Tools

Security-Tools werden host- oder user-lokal installiert, nie in ein Projekt-venv. Sie
sind die eine bewusste Ausnahme von der Projektlokalitaet: ein Scanner gehoert zur
Arbeitsumgebung, nicht zum Projekt.

Die kanonische Matrix liegt in [`../scripts/security-tools.tsv`](../scripts/security-tools.tsv).
Sie wird vom Installationsskript und von der Oberflaeche gelesen; die Liste steht nicht
zusaetzlich im Go-Code.

Pflicht-Tools:

| Tool | Rolle |
|---|---|
| `gitleaks` | Secret-Scanning |
| `trufflehog` | tiefes Secret-Scanning |
| `pip-audit` | Python Dependency-CVEs |
| `trivy` | Filesystem-, Container- und IaC-CVEs |
| `syft` | SBOM-Erzeugung |
| `grype` | SBOM-/Dependency-CVE-Auswertung |

`docker` ist optional und wird als Fallback-Kontext angezeigt, aber nicht durch
k-playbook installiert.

Die Oberflaeche zeigt den Status read-only und installiert nichts. Alles Weitere macht
das Skript selbst:

```bash
k-playbook/scripts/install-security-tools.sh                       # Status, das ist der Default
k-playbook/scripts/install-security-tools.sh --install missing     # fragt vor der Installation
k-playbook/scripts/install-security-tools.sh --help                # erklaert die Methoden
```

`--method` waehlt zwischen `auto`, `native`, `docker`, `pipx` und `venv`. Ohne `--yes`
zeigt das Skript den Plan und fragt.

**Vor der Installation darf kein Projekt-venv aktiv sein.** Sonst wird ein Tool aus dem
venv faelschlich als host-global vorhanden erkannt. Falls `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

Python-CLI-Tools gehoeren in `pipx` oder in ein dediziertes k-playbook-Tool-venv unter
`~/.local/share/k-playbook/`, nicht in `<projekt>/.venv`.

## Selbst bauen

Die mitgelieferten Binaries genuegen fuer den normalen Betrieb. Wer am Werkzeug selbst
arbeitet oder lieber selbst baut, braucht Go:

```bash
make dist   # baut alle Plattformen nach dist/
make gui    # baut und startet die Oberflaeche
```

`make dist` ist das einzige Build-Target und verwendet dieselben Flags wie die
ausgelieferten Artefakte, damit beide Wege dasselbe Ergebnis liefern. `make gui` ist der
Weg beim Entwickeln: es startet den frisch gebauten Stand.

## Verifikation

Checkliste fuer ein Projekt:

- [ ] `K-PLAYBOOK.yaml` liegt im Hauptverzeichnis, nicht in `k-playbook/`.
- [ ] `schema_version: 3` ist gesetzt.
- [ ] `project.repo_root` zeigt auf das Projekt-Repository, `project.vcs` ist `git` oder `none`.
- [ ] `k-playbook/` ist ein eigener Clone und enthaelt nichts Projekteigenes.
- [ ] `k-playbook-local/` existiert vollstaendig und ist im Projekt-Repository committet.
- [ ] `.claude/commands`, `.claude/skills`, `.opencode/commands` und `.cursor/commands`
      sind Verzeichnisse mit Einzel-Symlinks nach `k-playbook/` bzw. `k-playbook-local/`;
      die Oberflaeche meldet sie als eingerichtet.
- [ ] `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, und `AGENTS.md` traegt den Anstoss.
- [ ] `k-playbook/bin/k-playbook context` laeuft durch und nennt die erwarteten Kataloge.

Der letzte Punkt prueft alles Vorherige auf einmal: das Kommando bricht ab, wenn die
Konfiguration fehlt oder eine andere `schema_version` traegt.

## Fehlersuche

**Slash-Commands tauchen nicht auf.** Die Oberflaeche starten: sie vergleicht den
Katalog mit dem, was registriert ist, und nennt die fehlenden Commands beim Namen.
Nach dem Einrichten den Assistenten neu starten.

**Ein neuer Command aus `k-playbook-local/commands/` fehlt.** Er wird nicht automatisch
registriert — die Oberflaeche meldet ihn als fehlend und legt den Link auf Knopfdruck an.
Dasselbe gilt, wenn eine projekteigene Datei einen mitgelieferten Command neuerdings
ersetzt: dann zeigt der bestehende Link noch auf die alte Quelle.

**Skills werden nicht getriggert.** Unter jedem Skill-Ordner muss `SKILL.md` liegen —
ohne sie gilt das Verzeichnis nicht als Skill und wird nicht verlinkt. Danach den
Assistenten neu starten.

**Das Werkzeug findet kein Projekt.** Dann fehlt die `K-PLAYBOOK.yaml` oberhalb des
Aufrufortes. Die Suche laeuft ab dem Arbeitsverzeichnis aufwaerts bis `$HOME` bzw. `/`
und raet bewusst nicht. Die Oberflaeche schlaegt dann einen Ort vor.

**Ein Assistent sieht fremde Commands.** Typisch nach einer Installation nach dem alten
Modell: die host-globalen Symlinks wirken in jedes Projekt hinein. Die Oberflaeche einmal
starten, sie raeumt sie weg und meldet, was entfernt wurde.

**Das Binary fehlt.** `bin/k-playbook` meldet, welches Artefakt es unter `dist/` erwartet
hat. Entweder `git pull` oder `make dist`.
