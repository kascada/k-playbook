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

Das Werkzeug ist ein eigenständiges Go-Modul unter `installer/`. Ausgeliefert wird es
als ein Binary je Plattform; im Betrieb liegt es unter `~/.local/bin/k-playbook` und wird
unter dem bloßen Namen `k-playbook` aufgerufen. Einen Einstiegspunkt im Projekt-Clone
gibt es nicht mehr — `bin/install` ist der Bootstrap, nicht der Aufruf.

## Acht Einstiege

Ohne Argument die Oberfläche, dazu die sieben Subkommandos `config create`, `context`,
`mcp`, `scan`, `merge`, `inventory` und `stop`. Die Aufzählung unten geht sie in dieser
Reihenfolge durch; `help` zählt nicht mit, es gibt nur diese Übersicht aus.

```go
if len(args) == 0 {
    if guiproc.ServeMode() {
        return webui.Serve()
    }
    return runGUI()
}
```

Ohne Argument die Oberfläche — genauer: der **Client-Pfad** `runGUI()` in
`cmd/k-playbook/gui.go`. Er räumt Altlasten weg, sperrt die
Installation und sieht dann in der Laufzeitdatei nach, ob für dieses Projekt schon ein
Server läuft. Läuft einer, öffnet er nur den Browser; läuft keiner, startet er das eigene
Binary mit `K_PLAYBOOK_SERVE=1` als abgekoppelten Server und endet, sobald der antwortet
(siehe „Lebenszyklus"). Diese Umgebungsmarke ist der **verdeckte Servermodus**: kein
Subkommando, weil ihn niemand von Hand aufruft — `webui.Serve()` ohne Wirt-Pflege und
ohne Browser, die Ausgaben gehen ins Log.

`stop` beendet den Server dieses Projekts: Laufzeitdatei lesen, Prozessidentität prüfen,
`POST /api/shutdown`, sonst SIGTERM. Ohne Datei eine Auskunft und kein Fehler; eine
verwaiste Datei wird dabei entfernt.

`config create` legt den Anker ohne Aufwärtssuche im ausdrücklich gewählten oder
aktuellen Verzeichnis an. Mit `context` die JSON-Ausgabe, mit `mcp` der Server für einen
Assistenten, mit `scan` die Ausführung eines Review-Laufs, mit `merge` seine
Zusammenfassung, mit `inventory` die Erhebung des Versionsinventars — und diese wie `stop`
**ohne** `cleanUpLegacy()` und die Migrationsbereinigung: deren Meldungen gehen nach stdout
und würden die Ausgabe stören. Bei `mcp` wiegt das schwerer als bei `context`: dort trägt
stdout einen JSON-RPC-Strom, der über die ganze Sitzung offen bleibt, und eine einzige
fremde Zeile macht ihn unbrauchbar. Bei `scan`, `merge` und `inventory` zählt ein anderer
Grund: sie arbeiten auf dem Projekt und sollen den Host nicht nebenbei anfassen. Bei
`stop`: wer beendet, will nichts einrichten.

`scan <lauf> [eintrag …]` führt die Werkzeug-Einträge eines Laufs aus und blockiert, bis
sie durch sind. Das Kommando sammelt nur zusammen, was der Lauf braucht — Installation,
Preflight, Konfiguration — und reicht es an `internal/review` weiter; dort steht die
Ausführung selbst, ohne eigene Suche nach Pfaden oder Binaries. Wie der Lauf aussieht,
den es ausführt, steht in [`../../docs/review-runs.md`](../../docs/review-runs.md).

`merge <lauf>` fasst einen bereits gelaufenen Review-Lauf zu den Review-Input-Artefakten
zusammen: `review-input.json` und `review-triage.md` im Laufverzeichnis. Auch hier löst
das Kommando nur Lauf, Projektumgebung und Severity-Katalog auf; die Zusammenführung
selbst steht in `internal/review/merge`. Es scannt nichts nach — was nicht im Lauf steht,
kommt nicht in den Input.

`inventory` erhebt das Versionsinventar des Projekts und schreibt es nach
`k-playbook-local/docs/versions/inventory.md`. Es liest ausschließlich deklarative
Quellen — Manifeste, Lockfiles, Container-, DevContainer-, Helm- und CI-Dateien, dazu die
in `k-playbook-local/version-sources.yaml` konfigurierten —, fragt kein Netz und führt
kein gefundenes Werkzeug aus. Nicht durchsucht werden dabei die Installation
`<projekt>/k-playbook/` — ein Clone des Werkzeugs, der in jedem Zielprojekt dieselben
Manifeste trägt — und die Muster aus `exclude:` der Quellenkonfiguration; beide Ausschlüsse
stehen mit der Zahl der übergangenen Quellen im Inventar, und ein Eintrag in `sources:`
holt jede Quelle daraus wieder herein. Das Kommando sammelt Projektwurzel,
Quellenkonfiguration und
Zielpfad zusammen und reicht sie an `internal/inventory` weiter; dort liegen Parser,
Vertrauensgrenze und Renderer. Ein Lauf ohne inhaltliche Änderung lässt die Datei
byte-identisch stehen. Der Vertrag steht in
[`../../docs/versionsinventar.md`](../../docs/versionsinventar.md); denselben Lauf stoßen
der Command `/k-doc-inventory` und der Bereich „Inventar" der Oberfläche über
`POST /api/inventory` an (siehe „Das Versionsinventar in der Oberfläche").

Entfallen sind die Einrichtungs- und Lebenszyklus-Kommandos des alten Stands: `init`,
`update`, `restore`, `migrate`, `status`, `smoke` und `projects …`, samt der lokalen
Projektliste unter `.k-playbook-local/projects.json`. `status` kommt auch mit dem
Hintergrunddienst nicht zurück: wer die URL braucht, ruft `k-playbook` ohne Argument auf —
das findet den laufenden Server und öffnet ihn. Ein neues Subkommando entsteht dort, wo
deterministische Go-Fachlogik einen eigenen Anstoß braucht, den kein Assistent nachbaut —
so ist `inventory` dazugekommen.

## Aufbau

```text
installer/
├── cmd/k-playbook/
│   ├── main.go                  Einstiege: Client-Pfad, verdeckter Servermodus, Subkommandos
│   ├── gui.go                   Client-Pfad: Wirt-Pflege, Laufzeitdatei einordnen, abkoppeln
│   ├── stop.go                  Subkommando stop
│   ├── scan.go                  Subkommando scan: Lauf lesen, Auswahl, Ausführung anstoßen
│   ├── merge.go                 Subkommando merge: Lauf als Review-Input zusammenfassen
│   └── inventory.go             Subkommando inventory: Erhebung anstoßen, Bericht ausgeben
├── internal/guiproc/
│   ├── guiproc.go               Schlüssel, Laufzeitverzeichnis, Laufzeitdatei (O_EXCL)
│   ├── classify.go              Einordnung in fünf Ergebnisse, Antwort von /api/health
│   ├── process.go               Prozessidentität: PID lebt und Startzeit passt
│   ├── control.go               Shutdown anfordern, auf das Ende warten
│   ├── spawn.go                 eigenes Binary als Server starten, auf Antwort warten
│   └── *_unix.go *_linux.go *_darwin.go
│                                Signal 0, SIGTERM, Setsid, Startzeit je Plattform
├── internal/legacy/
│   └── global.go                host-globale Registrierung des alten Modells entfernen
├── internal/project/
│   ├── discover.go              Anker finden
│   ├── environment.go           was liegt hier vor
│   ├── config.go                Config lesen, Ort vorschlagen, anlegen
│   ├── local.go                 projekteigene Struktur prüfen und anlegen
│   ├── local_private.go         messen und umschalten, ob priv/ und material/ privat sind
│   ├── registry.go              Commands und Skills aus beiden Quellen auflösen
│   ├── links.go                 Assistenten-Verlinkung prüfen, herstellen, selbst heilen
│   ├── mcp.go                   MCP-Registrierung in den drei Assistenten-Dateien
│   ├── installed.go             den absoluten Pfad des installierten k-playbook auflösen
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
│   ├── server.go                Routen, Servermodus, Leerlaufwächter, Herkunftsprüfung
│   ├── browser.go               Browser öffnen, Container erkennen
│   ├── docs.go                  Doku-Endpunkte, Markdown nach HTML
│   ├── inventory.go             Versionsinventar: Stand, Anstoß der Erhebung, Datei
│   ├── tasks.go                 Task-Endpunkte, Liste und einzelne Datei
│   ├── todos.go                 Todo-Endpunkte, offen und erledigt getrennt
│   ├── mcp.go                   Registrierung messen und herstellen, Werkzeug-Selbsttest
│   ├── config.go local.go local_private.go assistant.go tools.go
│   ├── remediation.go context.go
│   ├── gh.go update.go reviews.go
│   └── static/                  index.html, workflows.html, docs.html, inventory.html,
│                                mcp.html, sidebar.html (Fragment der linken Spalte),
│                                session.js, nav.js, app.js, workflows.js,
│                                docs.js, inventory.js, mcp.js, styles.css
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
├── internal/inventory/
│   ├── model.go                 Datenmodell, Pin-Taxonomie, Labels, Ökosysteme
│   ├── trust.go                 die Vertrauensgrenze: Check, Expand, ReadFile
│   ├── discover.go              Standardquellen unterhalb der Projektwurzel finden
│   ├── collect.go               Standard- und Zusatzquellen planen, lesen, sammeln
│   ├── parse*.go                je Ökosystem ein Leser: Python, Go, Node, Container,
│   │                            Helm, CI, weitere Manifesttypen
│   ├── normalize.go             Namen, Versionen, Digests, Pin-Art bestimmen
│   ├── deviations.go            Gruppen bilden, Abweichungen ausweisen
│   ├── render.go                die Inventardatei erzeugen, deterministisch
│   ├── write.go                 Byte-Stabilitätsregel: vergleichen, sonst nicht schreiben
│   ├── status.go                Frontmatter der Inventardatei lesen (ReadStatus)
│   ├── tomllite.go jsonc.go textutil.go
│   │                            kleine Leser für TOML, JSON mit Kommentaren, Textzeilen
│   └── testdata/projekte/       Fixture-Projekte für die Parser
├── internal/versionsources/
│   └── versionsources.go        version-sources.yaml lesen — die eine Implementierung,
│                                benutzt vom Sammler und von project/context.go
├── internal/yamllite/
│   ├── yamllite.go              der YAML-Ausschnitt, den beide brauchen, mit Zeilennummer
│   └── flow.go                  Flow-Schreibweise: [a, b] und { k: v }
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

`internal/inventory` folgt derselben Trennung: Projektwurzel, Quellenkonfiguration und
Zielpfad stehen in `inventory.Options`, gesucht wird nichts selbst. Geöffnet wird jede
Quelle ausschließlich über `Boundary.ReadFile` in `trust.go` — die Vertrauensgrenze ist
einmal implementiert und gilt für Subkommando, Command und die spätere Web-API gleich.
`internal/versionsources` liegt daneben statt darin, weil `project` es importiert: läge es
in `project`, zöge der Sammler das ganze Projektpaket herein und `project` könnte den
Sammler nie benutzen. `internal/yamllite` ist der YAML-Ausschnitt, den beide brauchen; es
merkt sich Zeilennummern, weil die Herkunft einer Inventarzeile sie trägt.

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
2. Das Arbeitsverzeichnis.

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
version-sources.yaml
```

Die erzeugten Docs-Herkünfte — `docs/code/`, `docs/libs/`, `docs/extracted/` und
`docs/versions/` — stehen bewusst **nicht** darin: sie entstehen beim ersten Lauf ihres
Erzeugers.

Datei-Einträge bekommen ihren Erstinhalt aus `fileTemplate()`. Jeder von ihnen braucht
dort einen eigenen Zweig: der Rückfall ist `todoTemplate()`, und der schriebe sonst einen
TODO-Rumpf in eine Datei, die etwas anderes ist. `version-sources.yaml` bekommt deshalb
die gültige, leere Quellenkonfiguration aus `versionSourcesTemplate()` — wortgleich die
Vorlage aus `docs/versionsinventar.md`.

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
| `CLAUDE.md` | Include-Datei mit `@AGENTS.md` (`IsInclude`) | Claude Code |

Skills stehen nur einmal: OpenCode durchsucht neben `.opencode/skills/` auch
`.claude/skills/`, ein zweiter Ort wäre Dopplung. Cursor kennt kein Skill-Konzept.

`CLAUDE.md` ist eine erzeugte reguläre Datei, die `AGENTS.md` über die Import-Zeile
`ClaudeIncludeLine` (`"@" + RootInstructionsFile`) einbindet, weil Claude Code
ausschließlich `CLAUDE.md` liest und OpenCode wie Cursor `AGENTS.md` bevorzugen. Früher
war es ein Symlink: der hielt beide Dateien zwangsläufig gleich, brach aber bei jedem
Editor, der beim Speichern atomar ersetzt, und die Prüfung meldete danach `blocked` mit
„echte Datei statt Symlink". Dieser Fall existiert nicht mehr. Der Preis: zwei Dateien,
die auseinanderlaufen können — was Claude Code nach `CLAUDE.md` schreibt, erreicht die
anderen nicht, und die Prüfung deckt das als `ok` zu. Das ist bewusst so; der Stub aus
`claudeIncludeStub()` sagt über der Import-Zeile, dass Projektregeln nach `AGENTS.md`
gehören.

Geprüft wird am **Inhalt**, nicht an der Bauform: `hasEffectiveInclude()` sucht die
Import-Zeile als eigenes Wort außerhalb eingezäunter Code-Blöcke und außerhalb von
Backtick-Spans — dort überliest Claude Code sie beim Import-Parsing. Trägt die Datei den
wirksamen Include, ist sie `ok`, gleichgültig was daneben steht. Ein Symlink auf
`AGENTS.md` ist `stale` und damit heilbar; `applyIncludeFile()` entfernt ihn und schreibt
den Stub — verlustfrei, der Inhalt steht in `AGENTS.md`. Eine echte Datei wird nie
überschrieben. Kein Include ins Leere: fehlt `AGENTS.md`, schreibt `applyIncludeFile()`
nichts, und `checkIncludeFile()` meldet `no-source` — auch über den Migrationszweig
hinweg, sonst meldete jeder Lesezugriff erneut `Applied`.

Die Include-Datei ist `Optional`, weil ihre Quelle dem Projekt gehört: der Lesepfad ruft
`HealLinks()` direkt und legt nie ein `AGENTS.md` an. Ohne das Flag meldete jedes Projekt
ohne `AGENTS.md` dauerhaft einen offenen, nicht heilbaren Punkt. Die Kehrseite: steht der
Stub und fehlt `AGENTS.md`, bleibt `LinksOK` true, während der Import ins Leere zeigt —
der Detailtext benennt genau das.

Die Richtung ist eine Konstante, keine projektabhängige Variable: eine umgedrehte
Richtung müsste in Prüfung, Oberfläche und Doku dauerhaft mitgedacht werden. Deshalb wird
eine mitgebrachte echte `CLAUDE.md` einmalig **umbenannt** statt die Richtung umgedreht.
Siehe [Instruktionsdateien einordnen](#instruktionsdateien-einordnen).

Die Reihenfolge — einordnen, `ApplyRootInstructions()`, `ApplyLinks()` — steckt in
`ApplyAssistantSetup()` (`project/setup.go`) und nicht mehr im Handler: die Include-Datei
braucht `AGENTS.md` als Ziel, und die Umbenennung muss vor dem Anlegen aus der Vorlage
laufen. Siehe [Instruktionen](#instruktionen).

Zwei Eigenschaften der Import-Syntax, belegt am Stand vom 2026-09-05: `@`-Importe in
`CLAUDE.md` gibt es seit Claude Code 0.2.107 (CHANGELOG), und die Importtiefe ist auf vier
Ebenen begrenzt (code.claude.com, „How Claude remembers your project"). Der Stub
verbraucht davon eine.

### Selbstheilung auf dem Lesepfad

`HealLinks()` in `project/links.go` prüft, richtet ein, was sich einrichten lässt, und
meldet als `LinkRepair`, was danach offen bleibt. Gerufen wird es von zwei Stellen, die
beide **lesen** wollen: `assistantHandler()` (`GET /api/assistant`) und `ContextForDir()`
— und damit vom Subkommando `context` wie vom MCP-Werkzeug.

Der Grund ist eine Zuordnung, die lange falsch herum gedacht war. Welche Links gelten,
folgt dem **Katalog dieses Projekts**, nicht dem Weg, auf dem die Installation zu ihrem
Stand kam. Hing das Nachziehen an einem Weg — dem Update-Handler —, blieb die
Registrierung bei jedem anderen stehen: `make -C k-playbook installer-update`,
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

Eine Ausnahme von „nur registrieren" ist die Migration von `CLAUDE.md`: ein Symlink auf
`AGENTS.md` ist `stale` und damit `Fixable()`, der Lesepfad ersetzt ihn durch die
Include-Datei — die eine Stelle, an der er eine versionierte Datei im Hauptverzeichnis
ändert (git zeigt den Modewechsel `120000` → `100644`). `LinkRepair.IncludeMigrated`
trägt das; `contextLinks()` und `describeRepair()` benennen es eigens, und der Hinweis auf
die laufende Sitzung bleibt den Katalog-Links vorbehalten. Sie erscheint nur einmal:
danach ist die Datei `ok`. Scheitert das Schreiben — etwa an einem nicht beschreibbaren
Projektverzeichnis —, steht der Grund in `LinkRepair.Error`, und gelesen wird trotzdem.

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

Jede der beiden Dateien steht in genau einem von neun Zuständen (fehlt, echte Datei,
Include-Datei, Verzeichnis, Link auf die andere, Link auf ein fremdes vorhandenes Ziel,
Rest-Link, unlesbar, sonstiges). „Include-Datei" (`kindInclude`) gibt es nur an
`CLAUDE.md`: eine reguläre Datei mit wirksamer Import-Zeile, gleichgültig was daneben
steht — ein `AGENTS.md` mit derselben Zeile importierte sich selbst und bleibt „echte
Datei". Die Fallmatrix daraus wird von oben nach unten geprüft, die erste passende Zeile
gewinnt, und die letzte fängt alles Übrige auf — kein Zustand fällt durch. Aufgelöst
werden nur die Zeilen, bei denen nichts verloren geht: Umbenennen einer echten `CLAUDE.md`
und das Entfernen eines irreführenden Symlinks an `AGENTS.md`. Die Include-Datei wird
**nie** umbenannt — umbenannt ergäbe sie ein `AGENTS.md`, das sich selbst importiert.
Neben fehlendem `AGENTS.md` ist sie ein Rest, der liegen bleibt, während `AGENTS.md` aus
der Vorlage entsteht; neben verdrehter Richtung weicht erst der Symlink an `AGENTS.md`.

Die Zeilen 10 („nur `CLAUDE.md`") und 11 („beide echte Dateien") tragen deshalb je zwei
Zweige unter **derselben Nummer**: die Nummer benennt die Ausgangslage nach Dateisorte,
der Zweig hängt am wirksamen Include — Zeile 10 benennt um oder lässt den Stub liegen,
Zeile 11 ist Sollzustand oder Konflikt. Eine eigene Nummer je Zweig hätte die Matrix auf
19 Zeilen gestreckt und jede Zeilenangabe in Tests und Doku verschoben; `Outcome` und
`Detail` unterscheiden die Zweige ohnehin. Zeile 14 (Symlink neben echter `AGENTS.md`) ist
nicht mehr der Sollzustand, sondern die Migration; Zeile 9 (fremd verlinktes `AGENTS.md`)
zählt die Include-Datei ausdrücklich mit, weil sie den Stub selbst schreibt und er sonst
beim nächsten Lauf in Zeile 8 fiele — ein Konflikt an der eigenen Datei.

Alles andere wird als `StateConflict` gemeldet und **nicht angefasst** — auch nicht
angelegt. Der Detailtext nennt die Ursache, den Ausweg und die Folge: bis zur Handarbeit
sieht Claude Code den Anstoß nicht. Prüfung und Einrichten benutzen dieselbe
Klassifikation; zwei Implementierungen liefen auseinander. `checkIncludeFile()` ordnet
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
| `stale` | eine ältere Bauform, die verlustfrei ersetzt wird: der Symlink `CLAUDE.md -> AGENTS.md`, der zur Include-Datei wird, oder der Verzeichnis-Symlink eines Katalog-Links, der zu Einzel-Links wird; die Oberfläche beschriftet ihn „alte Form, wird ersetzt" |
| `incomplete` | das Verzeichnis steht, sein Inhalt weicht vom Katalog ab |
| `blocked` | etwas Echtes steht im Weg; wird nicht angefasst |
| `no-source` | es gibt nichts zu verlinken; an `CLAUDE.md` nennt der Detailtext, ob ein Stub schon ins Leere zeigt |
| `conflict` | `CLAUDE.md` und `AGENTS.md` lassen sich nicht auflösen, ohne Inhalt zu verlieren oder zu verdoppeln — typisch eine echte `CLAUDE.md` ohne wirksame Import-Zeile neben `AGENTS.md`; der Detailtext nennt die Auflösungen |

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
Orte falsch. Wo die Instruktionen liegen, beantwortet der Aufruf. Aufgerufen wird das
einmal je Host oder DevContainer installierte `k-playbook` unter seinem Namen.

### Ein veralteter Anstoß wird ersetzt

Der Marker allein war zu wenig. Ein Bestandsprojekt trug den Anstoß des abgelösten
Wrapper-Modells (`k-playbook/bin/k-playbook context`); weil der Marker dastand, tat
`applyRootInstructions()` nichts — und der Git-Update-Weg erreicht die Datei nicht, denn
sie liegt im Hauptverzeichnis und nicht im Clone. Nach dem Entfernen des Wrappers zeigte
sie dauerhaft auf eine Datei, die es nicht mehr gibt.

`replaceOutdatedInstructionsBlock()` ersetzt deshalb den Block, wenn er noch
`legacyInstructionsCommand` (`/bin/k-playbook context`) enthält — dieselbe enge
Definition von „veraltet" wie bei der MCP-Registrierung, und aus demselben Grund: die
neue Fassung enthält das Muster nicht, der Lauf ist damit idempotent.

`instructionsBlockBounds()` grenzt den Block ab: von der Markerzeile bis zum nächsten
HTML-Kommentar oder zur nächsten Überschrift, abzüglich der Leerzeilen davor. Der Block
trägt genau einen Marker und genau eine Überschrift; was ein Projekt dahinter geschrieben
hat — auch der Session-Memory-Block — bleibt dadurch stehen.

Zwei Einstiege, wie bei MCP:

| Weg | Einstieg | Tut |
|---|---|---|
| ausdrücklich | `ApplyRootInstructions()`, Einrichten und Clone-Update über `ApplyAssistantSetup()` | anlegen, anhängen, veralteten Block ersetzen |
| selbsttätig | `RepairRootInstructions()`, jeder Start | ausschließlich einen veralteten Block ersetzen |

Der Auffangweg legt nichts an, ergänzt nichts und fasst einen fremden Text nicht an. Er
ist der Gegenpart zu `RepairMCP()` für die zweite Datei, die im Hauptverzeichnis liegt.

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

Der Security-Tool-Preflight fehlt im Kontext bewusst — aber nicht, weil eine
Werkzeugprüfung generell zu teuer wäre. Entscheidend ist, **was** geprüft wird:

- **Versionsprüfung, teuer.** Der Security-Preflight startet je Tool einen Unterprozess
  und liest dessen `--version`. Das dauert spürbar, und ein hängendes Tool hält den
  ganzen Aufruf auf; deshalb hat `CheckTools()` ein Timeout. So etwas gehört nicht an den
  Anfang jedes Commands.
- **Anwesenheitsprüfung, billig.** Der Befund zu den Basis-Werkzeugen schlägt je Werkzeug
  nur im PATH nach — `exec.LookPath`, kein Unterprozess, keine Shell. Gemessen auf dem
  Entwicklungshost: rund 0,3 Millisekunden für die sieben Werkzeuge, gegen rund
  46 Millisekunden für den gesamten `k-playbook context`. Das ist etwa ein halbes Prozent
  des Aufrufs. Deshalb steht `baseTools` im Kontext und `tools` nicht.

`context` soll billig genug sein, um am Anfang jedes Commands zu stehen. Die Grenze
verläuft zwischen Unterprozess und Nachschlagen, nicht zwischen Werkzeug und Nicht-Werkzeug.

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
stolpern. Das ist der Unterschied zwischen „irgendwas ging schief" und „`bin/install`
ist verändert".

Die Oberfläche zeigt den Befund in einer eigenen Karte, weil dort Dateinamen hinmüssen.
Ist ein Remote-Update verfügbar und der Zustand blockierend, heißt der Kopfknopf
„Update blockiert" statt „Update verfügbar"; ein Klick prüft erneut, startet aber keinen
Pull. Bewusst **ohne** Knopf zum Zurücksetzen: das wäre `git checkout -- .` in einem
fremden Verzeichnis, und die Oberfläche kann nicht wissen, ob dort jemand absichtlich
entwickelt. Der Befehl steht zum Kopieren da.

Vor und nach dem Pull wird `VERSION` gelesen. `BinaryChanged` meldet, ob sie gewechselt
hat — **nur dann** gehört zum neuen Stand ein anderes Binary, und nur dann bringt ein
Neustart eine andere Programmversion. Unter Linux behält der laufende Prozess ohnehin
seinen Inode und arbeitet mit dem alten Code weiter.

Früher wurden dafür die Dateien unter `dist/` gehasht. Seit die Binaries Release-Assets
sind, liegt im Clone keins mehr, das sich vergleichen ließe; `VERSION` ist an diese
Stelle getreten und trennt zugleich sauber: Commits an Regeln, Reviews, Commands oder
Docs ändern sie nicht.

Hat `VERSION` gewechselt, beendet sich der Hintergrunddienst nach der Update-Antwort. Das
neue Binary wird anschließend ausdrücklich über den Bootstrap installiert — die Antwort
nennt ihn in der kanonischen Form aus `project.BootstrapHint`, also
`make -C k-playbook install`, ohne make `k-playbook/bin/install`. Der Update-Pfad lädt
oder ersetzt keine Host-Binaries selbst.

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

## Direktinstallation

`bin/install` bleibt im Projekt-Clone und installiert einmalig das passende, gegen
`SHA256SUMS` geprüfte Release-Binary direkt nach `~/.local/bin/k-playbook`. Der normale
Aufruf ist danach immer `k-playbook`; der GUI-Start installiert oder aktualisiert kein
Binary selbst.

Der globale Aufruf ist überhaupt möglich, weil das Programm sein Projekt aus dem
**Arbeitsverzeichnis** ableitet (`Detect()` über `os.Getwd()`) und nicht aus seinem
eigenen Ort. Ein einziges Binary bedient damit alle Projekte; projektspezifisch ist nur
der Kontext, nicht das Werkzeug.

### Der PATH ist Voraussetzung, kein Hinweis

Der ganze Vertrag hängt daran, dass `~/.local/bin` im `PATH` liegt: `k-playbook` wird
ausschließlich unter seinem Namen aufgerufen — von Commands, von den Instruktionen, vom
Nutzer. Ein installiertes, aber nicht auffindbares Binary wäre deshalb ein stiller
Totalausfall.

`bin/install` bricht in diesem Fall ab und nennt die Zeile fürs Shell-Profil. Die Prüfung
liegt **vor** dem Download: sie braucht kein Release-Asset, und ein Abbruch soll nichts
geladen und nichts ersetzt haben.

### Ein Binary je Plattform, ein Ort

`~/.local/bin/k-playbook` ist eine echte Datei, kein Symlink und keine Auflösung zur
Laufzeit. Jede Arbeitsumgebung installiert die Fassung ihrer eigenen Plattform: der
macOS-Host ein Darwin-Binary, der DevContainer ein Linux-Binary.

Teilen sich beide dasselbe `$HOME`, überschreiben sie einander an dieser einen Datei.
Getrennte HOMEs sind der saubere Zustand, aber nicht erzwingbar. `bin/install` liest
deshalb vor dem Ersetzen die Magic-Bytes einer vorhandenen Datei — ELF gegen Mach-O,
ohne `file`, das in schlanken Containern fehlt — und meldet beim Ersetzen, dass die
andere Umgebung denselben Bootstrap noch einmal braucht.

**Die Grenze davon steht hier ausdrücklich:** Träger der Erkennung ist allein
`bin/install`. Wer `~/.local/bin/k-playbook` direkt aufruft, während dort ein
plattformfremdes Binary liegt, bekommt weiterhin die Meldung der Shell
(`cannot execute binary file`) — nach dem Wrapper-Ausstieg fängt das nichts mehr ab. Die
make-Targets tragen die Erkennung ebenfalls nicht: `make dev-install` und `make gui`
bauen vorher unbedingt für die eigene Plattform, dort kann nie ein fremdes Binary stehen.

### Übergang aufräumen

Der erste Start des neuen Binaries — und ebenso `bin/install` — entfernt die alte
Spiegelung unter `~/.local/share/k-playbook/installation/` sowie den früheren
Standard-Cache unter `~/.cache/k-playbook/`. Ein direkter Eintrag
`~/.local/bin/k-playbook` bleibt erhalten; nur ein Symlink auf die alte Spiegelung wird
entfernt.

Die Tool-venvs unter `~/.local/share/k-playbook/security-tools/`
(`rules/tool-install-scope.md`) bleiben ausdrücklich außerhalb dieser Bereinigung. Die
Ebene `installation/` gab es genau dafür: ein venv bringt ein eigenes `bin/` mit, ohne
diese Trennung kollidierten beide.

## Woher Binary und Dateien kommen

Zwei Dinge, die getrennt driften können — und deren Verwechslung teuer ist.

**Das Binary** gibt es an mehreren Orten. **Die Dateien** — Skripte, Tool-Matrix, Regeln,
Reviews, Checks — kommen dagegen **immer** aus `PlaybookDir(projektDir)`, also aus
`<projekt>/k-playbook/`. Das Binary liest nie neben sich.

Das ist Absicht: Das direkt installierte Binary unter `~/.local/bin` ist nicht an einen
bestimmten Projekt-Clone gebunden. Es leitet den Projektinhalt ausschließlich aus dem
Arbeitsverzeichnis und dem Anker ab.

| Ort | Was | Wird aktualisiert durch |
|---|---|---|
| `~/.local/bin/k-playbook` | direkt installiertes Binary | `make -C k-playbook install`, ohne make `k-playbook/bin/install`; im Entwicklungsrepo `make dev-install` |
| `<entwicklungsrepo>/dist/` | Build des Arbeitsstands | `make dist` / `make dist-host` |

`bin/install` ermittelt die Plattform, lädt das Release-Asset zu `VERSION`, prüft es
gegen `SHA256SUMS` und ersetzt `~/.local/bin/k-playbook` atomar. Es gibt keinen
Programm-Binary-Cache und keine Laufzeit-Auflösung mehr. Die Zuordnung eines laufenden
Servers zu dem Binary, aus dem er stammt, wird gesondert behandelt.

### Entwicklungsstand

Im Entwicklungsrepo fallen Quelle und Installation auseinander: `~/dev/k-playbook/` ist
der Arbeitsstand, `~/dev/k-playbook/k-playbook/` ein eigener Clone auf dem zuletzt
gepushten Commit.

Das bleibt so und ist gewollt. `make dev-install` baut den Arbeitsstand und ersetzt damit
das Binary unter `~/.local/bin`; der Inhalt — Regeln, Reviews, Checks, Commands, Skills —
kommt weiterhin aus dem Clone unter `k-playbook/` und damit vom zuletzt gepushten Stand.
Der Preis dafür ist bewusst bezahlt: der Clone bleibt ein read-only Vendor-Verzeichnis,
in das nichts eingespielt wird, und `CheckCleanliness()` meldet jede Abweichung dort als
das, was sie ist — Handarbeit in einem fremden Verzeichnis. Es gibt keinen Sync-Weg aus
dem Arbeitsstand in die Installation und keine Ausnahme davon in der Oberfläche.

**Der Hintergrunddienst erkennt auch einen dev-Build.** Verglichen wird nicht die
`VERSION`, sondern die Datei dahinter, siehe „Lebenszyklus": Größe und Änderungszeit von
`os.Executable()`. Ein frisch gebautes Binary bei unveränderter `VERSION` — `make
dist-host`, `make gui` — ist damit ein anderer Stand, und der nächste Aufruf in **jedem**
Projekt beendet dessen alten Server und zieht einen neuen hoch. Ein `k-playbook stop` von
Hand ist dafür nicht mehr nötig.

Was das nicht leistet: die Ablösung geschieht je Projekt und erst beim nächsten Aufruf
dort. `make gui` im Arbeitsstand beendet weiterhin nur den Server dieses Projekts — die
Server anderer Projekte laufen bis zu ihrem nächsten `k-playbook` mit altem Code weiter.

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

## Basis-Werkzeuge

Die Werkzeuge, die k-playbook **selbst** aufruft, haben eine eigene Matrix:
`scripts/base-tools.tsv`. Heute sind das `bash`, `git`, `curl` oder `wget`, `tar`,
`python3` und `rg`.

**Warum getrennt von den Security-Tools.** `scripts/scanners.tsv` referenziert
`security-tools.tsv` über die Spalte `tool`, und `resolveTools()` übersetzt jeden Eintrag
dort in einen Lauf-Zustand. Ein `rg` in der Security-Matrix erschiene in jedem Review-Lauf
als übersprungener Eintrag. Die Basis-Matrix taucht in keinem Review auf.

Die Matrix trennt, was `security-tools.tsv` in einer Spalte führt: Paketname und
Programmname fallen auseinander (`ripgrep` liefert `rg`, `fd-find` liefert `fdfind`).
Deshalb gibt es je Methode eine eigene Referenzspalte — `apt_package`, `github_repo` und
`asset_ref` — neben dem Programmnamen `name`, der der Prüfschlüssel ist. `group`
kennzeichnet Einträge, von denen einer genügt: `curl` und `wget` bilden eine Gruppe, und
sie gilt als vorhanden, sobald eines ihrer Mitglieder da ist. `guarded` hält je Eintrag
fest, ob die Aufrufstellen im Repo heute schon abgesichert sind — damit ein bestehender
Guard nicht ein zweites Mal gebaut wird.

### Befund im Kontext

`DetectBaseTools()` in `project/basetools.go` liest die Matrix und prüft je Eintrag mit
`exec.LookPath`. Kein Unterprozess, kein `--version`, keine Shell; die Kostenrechnung
steht oben bei „Der Security-Tool-Preflight fehlt im Kontext bewusst". Das Ergebnis steht
als `baseTools` neben `gh` in der Kontextausgabe.

Das Skript wird von Go aus **nie** aufgerufen — auch nicht mit `--json`. Als
Installationsbefehl meldet der Befund genau einen Aufruf:
`bash "<installation>/scripts/install-base-tools.sh" --install`. Welchen Weg das Skript je
Eintrag geht, rechnet Go nicht nach; sonst läge die Rangfolge zweimal vor, in Shell und in
Go, und liefe nach der ersten einseitigen Änderung auseinander. Gemeldet werden nur
Matrix-Daten: Programmname, Rolle und die Methodenspalte, wie sie in der TSV steht.

Fehlt die Matrix, ist das kein Abbruch: `present` wird `false`, `error` nennt den Zustand,
`missing` bleibt leer. Der Kontext steht am Anfang jedes Commands und muss auch mit einer
älteren Installation nutzbar bleiben.

**Bekannter Fehlalarm.** Gemessen wird Anwesenheit im PATH. Eine Shell-Funktion oder ein
Alias steht nicht im PATH: Claude Code setzt eine Shell-Funktion `rg` auf das im eigenen
Binary mitgelieferte ripgrep, und dort meldet der Befund `rg` dauerhaft als fehlend,
obwohl der Aufruf funktioniert. Das ist bekannt und wird hingenommen. In genau den
Umgebungen, für die der Befund existiert — OpenCode, Cursor, ein normales Terminal —, ist
er richtig, und dort fällt der Command sonst wortlos um. Die Alternative wäre, je Werkzeug
eine Shell zu starten; das kostet den Unterprozess, den der Kontext gerade nicht ausgeben
soll, und machte den Befund von der Shell des Aufrufers abhängig.

**Warnen statt blockieren.** Ein fehlendes Basis-Werkzeug beendet keinen Lauf. Das ist der
Unterschied zu `gh.ready`, das ein PR-Review hart abbricht. `commands/_shared/context.md`
schreibt vor, dass ein Command die Lücke benennt — Werkzeug, Rolle, Installationsbefehl —
und mit dem Rückfall weiterarbeitet, wo es einen gibt.

### Installationswege

`scripts/install-base-tools.sh` führt dieselben Optionen wie das Security-Skript:
`--preflight`, `--json`, `--install`, `--yes`, `--dry-run`, `--prefix` und `--bin-dir`.
Sein Installationsziel liegt in einem **eigenen** Namensraum: `K_BASE_TOOLS_MATRIX`,
`K_BASE_TOOLS_PREFIX` und `K_BASE_TOOLS_BIN_DIR`. Die `K_SECURITY_TOOLS_*`-Variablen
gehören dem anderen Skript, und ein fest verdrahtetes `~/.local/bin` ließe den gemeinsamen
Guard ins Leere zeigen, weil es dann gar kein auflösbares Ziel gäbe. Das Default-Ziel ist
PATH-sichtbar gewählt (erstes vorhandene aus `~/.opencode/bin` und `~/.local/bin`); ist
keines im PATH, wird trotzdem installiert, aber mit einem ausdrücklichen PATH-Hinweis —
sonst meldete `exec.LookPath` ein erfolgreich installiertes Werkzeug weiterhin als fehlend.

Root wird über die effektive UID erkannt, nicht über die Existenz von `sudo`: im
Image-Build ist man root, und `sudo` fehlt dort oft ganz.

Die Rangfolge wird **je Eintrag** durchlaufen. Jeder Fall endet benannt; keiner läuft ins
Leere:

1. **Root und `apt-get` vorhanden** — über apt, mit `apt-get update` vorweg und
   `DEBIAN_FRONTEND=noninteractive`. Das Ziel ist systemweit und hängt nicht an `$HOME`.
2. **Sonst, und nur für Einträge mit `github` in der Methodenspalte** — user-lokaler
   Release-Weg nach dem aufgelösten Ziel, auch dann, wenn `apt-get` vorhanden ist. Das ist
   der häufigste Entwicklerfall: Ubuntu-Host, apt da, kein root; der Weg funktioniert dort
   ohne root. Heute trifft das allein `rg`. Ist zusätzlich `apt-get` da, wird der
   vollständige `sudo apt-get`-Befehl mit ausgegeben — als Hinweis, nicht als Endstation:
   installiert wird trotzdem user-lokal.
3. **Apt-only auf einem Host mit `apt-get`, aber ohne root** — der `sudo apt-get`-Befehl
   wird ausgegeben. Er ist hier das **Ergebnis**, nicht ein Zwischenschritt: es wird nichts
   user-lokal geschrieben und kein Erfolg gemeldet.
4. **Apt-only auf einem Host ohne `apt-get`** — Alpine, RHEL, macOS, gleich ob root oder
   nicht: es gibt keinen Weg. Werkzeug und Grund werden benannt. Ohne diesen Fall bliebe
   etwa `git` als root im Alpine-Container ohne Ergebnis.

Die Fälle 3 und 4 sowie die Methode `none` enden mit dem eigenen Rückgabewert **3**. Er
trennt „für dieses Werkzeug gibt es hier keinen Weg" vom echten Fehlschlag, der wie überall
mit 1 endet.

**Apt-only ist ein zulässiger Zustand, keine Lücke.** Für `git`, `curl`, `wget`, `tar` und
`python3` gibt es keinen sinnvollen GitHub-Release, den man nach `~/.local/bin` entpacken
könnte. Einen Release-Weg für sie zu erfinden wäre schlimmer als keiner. Der Kopfkommentar
der Matrix sagt das ausdrücklich, damit niemand die Lücke „füllt".

**Warum apt bevorzugt ist** — nicht wegen automatischer Updates. Im Container wird gebaut,
nicht gepflegt, und Layer-Caching hält dieselbe Version fest. Der Grund ist: es ist die
Version, die die Distribution ohnehin testet, sie braucht keinen Netzzugriff auf die
GitHub-API, und für ein Basis-Werkzeug ist eine vorhandene Version wichtiger als eine neue.

### Für ein Dockerfile

`--yes` schaltet jede Rückfrage ab, damit eine einzelne RUN-Zeile unbeaufsichtigt
durchläuft. Als root mit `apt-get` greift für jeden Eintrag Fall 1:

```dockerfile
RUN bash /opt/projekt/k-playbook/scripts/install-base-tools.sh --install --yes
```

Ein DevContainer übernimmt dieselbe Zeile. Zwei Dinge sind dabei zu wissen:

- Der Lauf endet mit **3**, wenn ein Werkzeug auf diesem Host keinen Weg hat — etwa
  apt-only ohne apt. Das ist kein Fehlschlag, aber `docker build` bricht darauf ab. Wer das
  nicht will, hängt `|| test $? -eq 3` an; wer es will, lässt es stehen und sieht die
  Lücke beim Bauen.
- Wird als nicht-root gebaut, greift für `rg` Fall 2 und das Ziel hängt an `$HOME`; die
  übrigen Einträge fallen dann in Fall 3 und geben nur den `sudo apt-get`-Befehl aus.

### Root-Doktrin beider Skripte

Im Security-Skript ist root der Abbruch, im Basis-Skript der Installationsweg. Das sieht
gegensätzlich aus, ist aber dieselbe Regel in drei Punkten:

1. **k-playbook eskaliert nie selbst.** Kein Skript startet sich per `sudo` neu. Ein
   `sudo`-Befehl wird gezeigt, nie ausgeführt. Ein Skript, das sich selbst mit erweiterten
   Rechten neu startet und danach Binaries aus dem Netz holt, bräche die Zusage, die
   `tools.go` und die Oberfläche überall sonst einhalten.
2. **Root wird akzeptiert, wo das Ziel systemweit ist.** Der apt-Weg im Basis-Skript und
   der Image-Build sind legitim. Ein Guard auf die effektive UID 0 würde ein Dockerfile
   mithinausweisen, das als root nach `/root/.local/bin` installiert — das ist ein
   erlaubter Weg, den man nicht zunagelt, um einen Tippfehler zu fangen.
3. **Ein schreibender Aufruf, dessen aufgelöstes Ziel nicht der effektiven UID gehört,
   wird abgewiesen** — egal welches Skript. Das betrifft user-lokale Ziele, die an `$HOME`
   hängen; im Basis-Skript trifft es den user-lokalen Weg (Fall 2), nicht den apt-Weg
   (Fall 1).

Beide Skripte verweisen in ihrer Abbruchmeldung auf diese Doktrin, damit der Unterschied
am Abbruch selbst erklärt ist.

### Der Guard auf das Installationsziel

`ensure_target_owner()` in `scripts/lib/install-common.sh` ist Punkt 3 der Doktrin.

Der ursprüngliche Fehler: `install-security-tools.sh` prüfte ein aktives Python-venv, aber
nie, ob das Installationsziel zum ausführenden Benutzer passt. `default_bin_dir` und
`PREFIX` hängen an `$HOME`. Wer `sudo ./install-security-tools.sh --install missing` tippt,
bekommt je nach sudo-Konfiguration entweder Binaries in root's Home, wo sie niemand findet,
oder Dateien im eigenen `~/.local/bin`, die root gehören. Beides scheitert lautlos.

Es gibt genau **ein** Abbruchkriterium: Das aufgelöste Installationsziel gehört nicht der
effektiven UID. Daraus folgt:

- **Abgewiesen wird der gebrochene Zielpfad, nicht die UID.** Root allein ist kein
  Abbruchgrund.
- **Weder `$HOME` noch `SUDO_USER` sind eigenständige Abbruchgründe.** Sie kommen nur im
  Meldungstext als Erklärung vor. Als ODER-Liste träfe die Bedingung sonst auch
  `sudo -u builder -H …` — eine Rechteabgabe, kein Fehler — und blockierte den systemweiten
  Weg `--bin-dir /usr/local/bin`, den dieselbe Doktrin ausdrücklich erlaubt.
- **Geprüft wird das aufgelöste Ziel, nicht das Default-Ziel** — also nachdem `--prefix`,
  `--bin-dir` und die Overrides ausgewertet sind. Ein Aufruf mit
  `--bin-dir /usr/local/bin` zielt auf einen Pfad, der gar nicht an `$HOME` hängt.
- **Existiert das Ziel noch nicht, gilt das nächste vorhandene Elternverzeichnis.** Beim
  ersten Lauf ist `~/.local/bin` oft nicht da, und eine Eigentümerprüfung auf einen nicht
  existierenden Pfad wäre undefiniert. Genau das trifft den Image-Build.
- **Lesende Läufe sind ausgenommen.** `--preflight`, `--json` und `--dry-run` schreiben
  nichts; ein Abbruch nähme dort gerade die Diagnose, mit der man den Fall versteht.

Die fünf Aufrufformen trennen damit sauber: `sudo …` auf das Default-Ziel bricht ab;
`sudo … --bin-dir /usr/local/bin` läuft durch; `sudo -H …` läuft durch; `sudo -u <user> -H …`
läuft durch; ein normaler Nutzer läuft durch.

**Der Ort ist verschieden, das Kriterium nicht.** Im Security-Skript steht der Guard am
Anfang des schreibenden Laufs: das Ziel ist dort für den ganzen Lauf dasselbe, einmal aus
Optionen und Overrides aufgelöst. Im Basis-Skript steht er je Eintrag unmittelbar vor dem
Schreiben, weil erst je Eintrag und erst nach `command -v apt-get` feststeht, ob überhaupt
user-lokal geschrieben wird — ein Guard am Skriptanfang bräche
`sudo ./install-base-tools.sh --install` auf Ubuntu zu Unrecht ab, wo jeder apt-Eintrag in
Fall 1 fällt. Kein Widerspruch, sondern derselbe Guard an der jeweils frühestmöglichen
Stelle.

### Die geteilte Bibliothek

`scripts/lib/install-common.sh` ist eine gesourcete Bibliothek neben den ausführbaren
Skripten — im Muster von `checks/lib/`, das dort schon Hilfscode trägt. Ein
`lib/`-Unterverzeichnis ist kein Katalogeintrag: `isCatalogEntry()` übergeht es, und der
Assistent sieht es nicht.

Beide Installer sourcen sie relativ zum Ort des aufrufenden Skripts (`BASH_SOURCE`), nicht
zum Arbeitsverzeichnis — die Skripte laufen aus der Installation heraus und aus beliebigen
Verzeichnissen. Der Vertrag an das sourcende Skript steht im Kopf der Datei: es definiert
`die()`, `log()` und `run_or_print()` und setzt `DRY_RUN`.

Darin liegen **der Resolver und der Guard**, also alles, was beide gemeinsam haben:
`has_cmd`, `nearest_existing_dir`, `path_owner_uid`, `ensure_target_owner`, `download_file`,
`platform_key`, `resolve_release_asset`, `latest_asset` und `install_release_binary`.

Warum geteilt und nicht kopiert: Der Platzhaltersatz wird für `rg` erweitert, und die
Zusage lautet, dass die bestehenden Muster der Security-Matrix unverändert dasselbe Asset
auflösen. Eine zweite Kopie entwertete diese Zusage nach der ersten einseitigen Änderung —
sie gälte dann für den einen Resolver und stillschweigend nicht mehr für den anderen. Der
Regressionstest in `installer/internal/scripts/release_asset_test.go` prüft deshalb die
Muster, wie sie in der ausgelieferten `security-tools.tsv` stehen, und nicht eine Abschrift
davon.

### Der Platzhaltersatz für Asset-Muster

`resolve_release_asset` ersetzt im Asset-Muster Platzhalter und lässt alles Übrige als
regulären Ausdruck stehen. Bisher kannte er `{tool}`, `{version}`, `{os}` (linux|darwin),
`{arch}` (amd64|arm64), `{arch_x64}`, `{os_cap}` und `{arch_bits}`.

Das trägt `rg` nicht. `BurntSushi/ripgrep` benennt seine Assets nach Rust-Target-Triples:
`ripgrep-<version>-x86_64-unknown-linux-musl.tar.gz`, `…-aarch64-unknown-linux-gnu…`,
`…-x86_64-apple-darwin…`. Keine Kombination der bisherigen Platzhalter erzeugt
`x86_64-unknown-linux-musl` oder `apple-darwin`. Hinzugekommen sind deshalb:

- **`{arch_raw}`** — `x86_64` beziehungsweise `aarch64`, der Architekturteil des Triples.
- **`{vendor_os}`** — der Vendor-/Libc-Teil: unter Linux `unknown-linux-(?:musl|gnu)`,
  unter macOS `apple-darwin`. Der Libc-Teil ist unter Linux nicht einheitlich — x86_64
  kommt als musl, aarch64 als gnu —, deshalb steht dort eine Alternative statt eines festen
  Tokens. Der Platzhalter hängt nur am Betriebssystem; die Architektur steckt schon in
  `{arch_raw}`, und ein Muster, das beide mischte, träfe unter Linux auch das
  `apple-darwin`-Asset.

Zwei weitere Punkte gehören dazu:

- **`{tool}` bindet an die Installationsreferenz, nicht an den Programmnamen.** In der
  Security-Matrix ist beides dasselbe, deshalb fiel es nie auf. In der Basis-Matrix steht
  dafür `asset_ref`: `ripgrep`, nicht `rg`. Mit dem Programmnamen träfe das Muster kein
  einziges Asset.
- **`.sha256`-Assets werden nie getroffen.** Jedes Release-Asset hat ein
  Prüfsummen-Geschwister, und ein laxes Muster ohne Endanker griffe zuerst die Prüfsumme —
  installiert würde dann eine Textdatei.

Die Erweiterung ist **rein additiv**: kein bestehender Platzhalter ändert seine Bedeutung,
und kein Muster der Security-Matrix wurde angefasst. Der Regressionstest belegt das für
jeden `github`-Eintrag auf zwei Plattformen.

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

Die Oberfläche hat vier Bereiche: **Setup** unter `/`, **Workflows** unter `/workflows`,
**Docs** unter `/docs` und **Inventar** unter `/inventory`. `/mcp` ist keine fünfte
Sorte, sondern die Detailseite des Setup-Blocks und trägt dessen Bereich.

Das Inventar ist ein eigener Bereich und keine Karte auf der Startseite — nach demselben
Muster wie Workflows und Docs: die Startseite trägt die Einrichtungsschritte, und das
Inventar ist keiner. Eine Karte dort hätte auch nicht gereicht: die erzeugte Inventardatei
muss im Bereich selbst lesbar sein, und der Aktualisieren-Knopf braucht Verlaufsstatus
und ein Ergebnis mit Ablehnungen im Wortlaut. Beides braucht eine Seite. Eine zusätzliche
Verweis-Karte auf der Startseite gibt es deshalb ebenso wenig wie für Workflows und Docs;
der Weg ist der Umschalter. Was der Bereich zeigt und warum er vom Docs-Bereich getrennt
ist, steht unter „Das Versionsinventar in der Oberfläche".

Jede dieser Seiten hat links dieselbe Spalte, und die beantwortet zwei Fragen: In welchem
Bereich bin ich, und was steht in diesem Bereich? Oben der **Umschalter**, eine Liste von
`<a>` mit dem aktiven Bereich markiert; darunter das **Blockmenü** der Seite.

Beides steht in einem einzigen Template-Fragment, `static/sidebar.html` mit
`{{define "sidebar"}}`. `pageTemplate()` parst es mit jeder Seite zusammen — die
Seitendatei zuerst, denn `ParseFS` benennt das Ergebnis nach der ersten Datei, und
`Execute` führt damit die Seite aus und nicht das Fragment. Dreimal dasselbe Markup zu
kopieren wäre die Variante, die beim nächsten Bereich wieder auseinanderläuft.

Welcher Eintrag aktiv ist, kommt aus den Vorlagendaten: `renderPage()` bekommt Bereich
und offene Seite vom Handler. Beides ist nicht dasselbe, sobald ein Bereich mehr als eine
Seite hat — deshalb führt `aria-current="page"` nur der Verweis auf die offene Seite,
während ein aktiver Bereich mit anderem Ziel `aria-current="true"` bekommt: der Fall auf
`/mcp`, wo Setup aktiv ist, die Startseite darunter aber nicht offen. Ob es Workflows und
Docs überhaupt gibt, entscheidet `.Installed` — vor der Einrichtung führt der Umschalter
nur nach Setup, weil die beiden anderen Bereiche dort nichts zu zeigen hätten.

Die Spalte ist so hoch wie das Fenster abzüglich des sticky-Abstands, oben und unten je
einmal. Darin teilen sich ihre Kästen den Platz selbst auf: jeder behält seine Höhe,
allein das Blockmenü gibt nach und scrollt dann selbst. Geschätzt wird daran nichts, und
ein weiterer Kasten in der Spalte ändert die Rechnung nicht.

Unter dem Blockmenü steht **Neu einlesen**: ein `window.location.reload()`, nicht mehr.
Der Server hält keinen Zustand — jeder Handler ruft `project.Detect()` und liest die
Dateien bei jeder Anfrage neu —, ein vollständiges Neu-Einlesen ist deshalb genau ein
Neuladen der Seite, dieselbe Bewegung, die `app.js` nach dem Anlegen der Konfiguration
schon macht. Der Knopf steht auf jeder Seite, weil der einzige Weg zu einem frischen
Stand sonst über das Terminal führte; seit der Server im Hintergrund läuft, hieße das
„Dienst beenden und neu aufrufen". Nicht erneuert wird dabei das Binary samt
eingebetteter Assets — dafür gibt es den Update-Fall. Der Knopf behält wie jeder Kasten
seine Höhe; die Rechnung der Spalte hängt nicht an der Elementzahl.

Das Blockmenü ist **generiert** und steht in `static/nav.js`: `buildBlockNav()` läuft über
`.blocks > .card`, nimmt Id und `<h2>` jeder Karte und hängt je einen Eintrag an
`#block-nav`. Statuspunkt und Beschriftung spiegeln Pill und Überschrift der Karte, je ein
`MutationObserver` zieht das nach — eine neue Karte braucht deshalb nichts weiter als Id
und Überschrift, und ein Eintrag folgt seiner Karte auch dann noch, wenn sie ihre
Überschrift zur Laufzeit austauscht: `#task-card` trägt den Titel des geöffneten Tasks.
Der Aufruf muss **vor** den Ladefunktionen einer Seite stehen: die blenden Karten ein,
und das Menü zieht das nur mit, wenn es sie schon beobachtet.

`nav.js` gehört keiner Seite und holt sich `#block-nav` selbst, statt in ein
seitenspezifisches `elements` zu greifen. Eine Ausnahme kennt der Mechanismus: `/docs`
füllt dieselbe Liste aus seinen Dateien statt aus Karten und nutzt davon nur die
Markierung: `markBlockNavItem()` setzt sie, `clearBlockNavMarking()` räumt sie ab, ohne
eine neue zu setzen. Die zweite Funktion braucht `/docs`, weil ein Verweis in eine Datei
außerhalb des Index führen kann — dann gehört gar kein Eintrag markiert.

Unter 1080px entfällt das Blockmenü, **solange es ein Sprungmenü ist** — für die Karten
bliebe daneben zu wenig übrig, und seine Ziele stehen ohnehin als Karten gleich darunter.
Trägt es dagegen den Dateiindex von `/docs`, bleibt es stehen: dort führt sonst kein Weg
mehr zu den übrigen Dateien. Es steht dann als eigener Kasten über dem Text, nicht
mitlaufend, und begrenzt sich auf einen Anteil der Fensterhöhe — die eine Stelle, an der
eine Höhe geschätzt wird, weil es ohne sticky Spalte kein Gitter gibt, aus dem sich
rechnen ließe.

Welcher der beiden Fälle vorliegt, sagt der Modifier `file-index` am `#block-nav`.
`sidebar.html` setzt ihn serverseitig aus `.Area`, zusammen mit der Beschriftung des
Menüs — nicht `docs.js` beim Füllen: schmal bliebe der Index sonst so lange ausgeblendet,
bis `/api/docs` antwortet.

Der Umschalter bleibt in beiden Fällen und legt sich waagerecht über die Karten: ohne ihn
wären die anderen Bereiche von dort aus nicht mehr erreichbar.

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

Das bleibt so, ausdrücklich: „Doku in der Oberfläche" zeigt **weiterhin nur die
mitgelieferte** Doku der Installation. `project.DocsDir()`, `project.ListDocs()`,
`GET /api/docs`, `GET /api/docs/file` und die Seite `/docs` kennen `k-playbook-local/docs`
nicht und werden für das Versionsinventar nicht erweitert — das Inventar ist ein eigener
Bereich mit eigener API (siehe „Das Versionsinventar in der Oberfläche") und davon
getrennt. Eine zweite Docs-Wurzel in Werkzeug, API und Oberfläche wäre teurer als der
eigene Bereich gewesen und hätte eine bewusst getroffene Entscheidung rückgängig gemacht.

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

Verweise innerhalb der Doku fängt die Oberfläche ab: `.md`-Ziele öffnet sie im selben
Fenster, Anker springen innerhalb der Datei, Ziele mit Schema gehen in einen neuen Tab.
Der Grund ist nicht mehr der Server — der läuft im Hintergrund und endet mit keiner
Seite —, sondern die Ansicht selbst: ein roher Klick auf `handbuch.md` führte auf einen
Pfad, den der Server nicht kennt, statt in die gerenderte Datei, und das Menü soll
mitziehen. Führt ein Verweis in eine andere Datei, zieht es mit; steht die Datei nicht im
Index, bleibt gar kein Eintrag markiert statt der vorige.

Der Text steht in einer Karte und ist kein eigener Scroll-Container. Ohne Anker scrollt
deshalb das **Fenster** nach oben, nicht das Element — sonst bliebe die Ansicht dort
stehen, wo die vorige Datei endete.

Mermaid-Blöcke rendert der Browser nach. Die Library kommt bei Bedarf vom CDN — sie ist
zu groß, um sie mitzuliefern. Ohne Netz bleibt der Quelltext des Diagramms als
Codeblock stehen, die Datei ist also weiterhin lesbar.

## Das Versionsinventar in der Oberfläche

Der Bereich **Inventar** unter `/inventory` zeigt das Versionsinventar des Projekts und
stößt seine Erhebung an. Der Vertrag steht in
[`../../docs/versionsinventar.md`](../../docs/versionsinventar.md); die Oberfläche
formuliert nichts davon neu.

Die Seite hat vier Karten. **Stand** zeigt, ob die Inventardatei da ist, wann sie zuletzt
inhaltlich geändert wurde, den Erzeuger und die Zahlen aus ihrem Frontmatter — Quellen
gelesen und konfiguriert, Einträge, Abweichungen, abgelehnte und nicht durchsuchte
Quellen —, dazu den Pfad der Datei und den Knopf **Aktualisieren**. Solange der Lauf
steht, ist der Knopf gesperrt und ein Ring daneben sagt, dass gearbeitet wird; ein Lauf
liest jede Quelle des Projekts. **Letzter Lauf** erscheint danach: Erfolg oder Abbruch,
ob geschrieben wurde oder die Datei unverändert blieb, die Zahlen des Laufs — dieselben,
die `k-playbook inventory` ausgibt —, und jede Ablehnung, jeder greifende Ausschluss und
jeder Hinweis im Wortlaut. **Quellenkonfiguration** zeigt `version-sources.yaml`: Pfad,
vorhanden oder fehlend oder defekt, und die Zahl der Wurzeln, Zusatzquellen und
Ausschlussmuster. **Inventardatei** zeigt die erzeugte Datei gerendert, ohne ihren
Frontmatter-Block — der steht als Stand darüber.

Der Status kommt aus `inventory.ReadStatus`, also aus dem Frontmatter der Inventardatei
und nirgends sonst; ein Zwischenartefakt gibt es nicht. Das ist die eine Statusquelle, die
auch der Sammler, `/k-docs` und das Subkommando benutzen.

**Nur die Fachlogik.** Die Handler in `webui/inventory.go` rufen ausschließlich
`internal/inventory` auf: `inventory.FilePath` für den Ort der Datei, `inventory.ReadStatus`
für den Stand, `inventory.Run` für den Anstoß, `inventory.Body` zum Abtrennen des
Frontmatters vor dem Rendern. Es gibt keinen zweiten Parser und keine Sonderbehandlung von
Pfaden: `GET /api/inventory/file` nimmt keinen Pfad aus der Anfrage entgegen, sondern liest
genau die eine Datei. Die Vertrauensgrenze aus `inventory/trust.go` gilt für den Anstoß
unverändert und wird nicht zweitverwendet implementiert — ein abgelehnter Pfad steht in
der Antwort so, wie die Fachlogik ihn meldet: angefragter Pfad, aufgelöster Pfad, Grund.
Die Quellenkonfiguration liest derselbe `versionsources.Read` wie im Sammler und in
`k-playbook context`. Gerendert wird mit dem Goldmark aus `docs.go`; geteilt wird nur der
Renderer, der Docs-Bereich selbst bleibt unberührt.

**Anzeigen, nicht pflegen.** Die Quellenkonfiguration wird in der Oberfläche nur
gezeigt. Die Schreibregel des Vertrags verlangt ausdrückliche Bestätigung und rein
ergänzendes Schreiben unter Erhalt von Einträgen, Kommentaren und Reihenfolge; eine
Eingabemaske bräuchte dafür eine eigene Validierung gegen das Quellenmodell und einen
zweiten Schreibpfad neben `/k-doc-inventory`, der den Diff vor Augen hat. Es gibt deshalb
weder ein Formular noch einen Endpunkt, der in `version-sources.yaml` schreibt. Der
einzige schreibende Endpunkt des Bereichs ist `POST /api/inventory`, und der schreibt
allein die Inventardatei — und auch die nur, wenn die Erhebung sich inhaltlich vom Bestand
unterscheidet: die Byte-Stabilitätsregel steht in `inventory.Write`, nicht im Handler.
Der POST steht wie jeder andere hinter der Herkunftsprüfung.

**Eigener Bereich, eigene API.** Docs zeigt die mitgelieferte Doku der Installation; das
Inventar ist eine erzeugte Datei des Projekts unter `k-playbook-local/docs/versions/`.
Deshalb ein eigener Eintrag im Umschalter, eine eigene Seite und drei eigene Endpunkte —
und keine Erweiterung von `/api/docs`. Das Blockmenü entsteht wie auf der Startseite aus
den Karten; unter 1080px entfällt es wie dort.

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
| `GET` | `/api/health` | Lebenszeichen des Fensters; antwortet mit Schlüssel, Version, Build-Kennung und PID des Servers |
| `POST` | `/api/shutdown` | Dienst beenden, für alle Fenster |
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
| `GET` | `/api/mcp/tools` | Werkzeug-Selbsttest: startet den registrierten Befehl als Subprozess |
| `GET` | `/api/tools` | Security-Tool-Preflight, read-only |
| `POST` | `/api/languages` | `project.languages` setzen; antwortet mit dem neuen Tool-Zustand |
| `GET` | `/api/base-tools` | Befund zu den Basis-Werkzeugen aus dem Kontext, read-only; PATH-Lookup je Werkzeug, kein Skriptaufruf |
| `GET` | `/api/reviews` | bisherige Läufe auflisten, read-only; angelegt wird über die Commands |
| `GET` | `/api/gh` | `tools.gh` lesen, dazu den gh-Befund dieses Rechners |
| `POST` | `/api/gh` | `tools.gh.status` setzen; installiert und meldet nichts an |
| `GET` | `/api/remediation` | `remediation:`-Block lesen |
| `POST` | `/api/remediation` | `remediation:`-Block setzen |
| `GET` | `/api/update` | per `git ls-remote` prüfen, ob die Installation zurückliegt; liefert den lokalen Sauberkeitszustand mit |
| `POST` | `/api/update` | `git pull --ff-only` ausführen; bricht bei lokal veränderter Installation vorher ab; hat `VERSION` gewechselt, beendet sich der Dienst nach der Antwort |
| `GET` | `/api/context` | aufgelösten Arbeitsstand lesen, read-only |
| `GET` | `/api/docs` | mitgelieferte Doku auflisten, read-only |
| `GET` | `/api/docs/file` | eine Datei daraus als HTML lesen, read-only |
| `GET` | `/api/inventory` | Stand des Versionsinventars aus dem Frontmatter der Inventardatei, dazu der Zustand der Quellenkonfiguration; read-only |
| `POST` | `/api/inventory` | Erhebung anstoßen über `inventory.Run`; schreibt allein die Inventardatei, und die nur bei inhaltlicher Änderung; antwortet mit Zahlen, Ablehnungen, Ausschlüssen, Hinweisen und dem Stand danach |
| `GET` | `/api/inventory/file` | die Inventardatei als HTML lesen, ohne Frontmatter; fester Pfad, kein Parameter; read-only |
| `GET` | `/api/tasks` | offene Tasks auflisten, read-only |
| `GET` | `/api/tasks/done` | erledigte Tasks aus `done/` auflisten, read-only |
| `GET` | `/api/tasks/file` | einen Task als HTML lesen, read-only |
| `GET` | `/api/todos` | offene Todos aus `TODO.md` auflisten, read-only |
| `GET` | `/api/todos/done` | abgehakte Todos auflisten, read-only |

Statische Assets liegen unter `/static/`. Die Seiten sind `/` (Setup), `/workflows`,
`/docs`, `/inventory` und `/mcp`; alle fünf rendert `renderPage()` aus derselben Vorlage
für den Kopf und die linke Spalte. Mitgeliefert werden der aktive Bereich, die Auskunft, ob eine
Installation gefunden wurde, und die Version des Binarys: sie steht rechts oben im Kopf als
Marke, weil die Installation daneben einen anderen Stand tragen kann und ein offenes Fenster
nach einem Update sonst nicht verrät, welcher Server gerade antwortet. Ein Build ohne
`-ldflags` zeigt dort „ohne Version".

Alle `POST`-Routen stehen hinter der Herkunftsprüfung, siehe „Lebenszyklus". `POST
/api/client-gone` gibt es nicht mehr; der Server endet nicht mehr mit dem Fenster.

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
  die Wirt-Pflege.
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

### Der Serverprozess überlebt sein Binary

Ein MCP-Server läuft, solange der Client ihn offenhält — bei OpenCode und Claude Code also
über die ganze Assistenten-Sitzung, oft stundenlang. Wird `~/.local/bin/k-playbook` in
dieser Zeit ersetzt, arbeitet er mit dem Code weiter, mit dem er gestartet ist. Der
Standvergleich des GUI-Servers greift hier nicht: es gibt keine Laufzeitdatei, keinen
Port und keinen zweiten Prozess, der nachsehen könnte.

Der Schaden ist begrenzt, aber real. Die **Inhalte** sind aktuell: `contextTool` ruft bei
jedem Aufruf `project.ContextForDir()`, Regeln, Reviews, Checks und Instruktionen kommen
frisch von der Platte. Veraltet ist der **Code** der Werkzeuge — und `tools/list` geht
genau einmal beim Start, ein seither hinzugekommenes Werkzeug kann die laufende Sitzung
deshalb nie sehen.

Gemeldet wird das über eine empfangende Middleware, `staleBinaryNotice()` in
`internal/mcpserver/stale.go`. Sie hält beim Start `guiproc.OwnBuild()` fest und ruft
dieselbe Funktion bei jedem `tools/call` erneut: der Pfad aus `os.Executable()` steht für
einen laufenden Prozess fest, und Go schneidet das `" (deleted)"` weg, das Linux an
`/proc/self/exe` hängt, sobald die Datei ersetzt wurde — ein späterer Aufruf sieht also
die neue Datei. Weichen beide ab, hängt an der Antwort ein **zusätzlicher** Inhaltsblock
mit dem Hinweis. Der erste Block bleibt Zeichen für Zeichen der des Subkommandos; nur so
bleibt der Vergleich beider Fassaden prüfbar. Fehlt eine der beiden Kennungen, wird nichts
gemeldet: eine Kennung, die sich nicht erheben lässt, ist kein Nachweis eines Wechsels.

Der Text richtet sich an den **Assistenten**, nicht an den Nutzer — er ist der einzige
Leser einer Werkzeugantwort und die einzige Stelle, die es weitersagen kann. Deshalb steht
die Handlung ausdrücklich darin.

**Warum kein Beenden im Leerlauf.** Bei stdio startet der *Client* den Prozess. Ob er nach
einem Ende neu startet, steht in keiner Spec und ist je Client anders — für OpenCode ist es
am 6. September 2026 gemessen worden, an einer laufenden Sitzung, deren Server per SIGTERM
beendet wurde:

```
15:56:11  run=ea62bce9  permission=k-playbook_k_playbook_context   ← MCP-Weg, funktioniert
15:57:09  run=ea62bce9  WARN "MCP connection closed" server=k-playbook
15:57:31  run=ea62bce9  permission=bash pattern="k-playbook context"   ← Ausweichweg
```

Der Abgang wird bemerkt und protokolliert, **nachgestartet wird nichts**. Der Assistent ist
still auf das Subkommando ausgewichen; für den Nutzer sah der Aufruf aus wie ein Erfolg.
Genau darin liegt die Gefahr: `k_playbook_context` hat ein CLI-Gegenstück, die
Review-Werkzeuge nach dem Muster start/status/collect haben keins. Dort verschwände die
Fähigkeit stumm, ohne erkennbaren Zusammenhang mit dem Update.

Ein Server, der sich selbst beendet, nähme einer laufenden Sitzung damit dauerhaft seine
Werkzeuge — schlechter als ein alter Server, der arbeitet. Der Hinweis ist deshalb kein
Kompromiss, sondern die Lösung. Sollte sich ein Client anders verhalten und das je Client
unterschieden werden, bräuchte ein Abgang im Leerlauf zusätzlich einen In-flight-Zähler:
die Review-Werkzeuge laufen minutenlang.

**Warum kein `syscall.Exec`** auf das neue Binary, das die Pipes behielte und vom Client
unbemerkt bliebe: das neue Prozessabbild erwartete ein `initialize`, das der Client längst
geschickt hat und nicht wiederholt. Den Sitzungszustand zu übergeben, ginge nur mit
Eingriffen ins Go-SDK.

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
`applyMCPTarget()` schreibt gar nicht, wenn der Eintrag schon in einer akzeptierten Form
dasteht — sonst formatierte jeder Lauf eine fremde Datei erneut um.

Der Schlüssel gehört k-playbook. Ein abweichender Wert darunter ist kein Konflikt,
sondern ein falscher Stand: `MCPStateStale` meldet ihn, `ApplyMCP()` überschreibt ihn. Als
echter, unangetasteter Fall bleibt nur `MCPStateUnreadable` — und der ist seit dem
JWCC-Parser eng: Kommentare und Trailing Commas sind lesbar, gemeldet und nicht angefasst
wird nur noch, was auch als JWCC kein JSON-Objekt ergibt (kaputte Syntax, ein Array, ein
`null`). So geht keine Handarbeit eines Projekts verloren.

#### Was eingetragen wird

`MCPCommand()` liefert den **beim Schreiben aufgelösten absoluten Pfad** des installierten
k-playbook, dazu das Subkommando `mcp`. Aufgelöst wird er in `project/installed.go`:
zuerst `~/.local/bin/k-playbook` — das Ziel von `bin/install` und `make dev-install` —,
sonst der laufende Prozess, wenn er selbst `k-playbook` heißt. Über die `PATH` wird
bewusst **nicht** gesucht: ein PATH-Treffer hinge wieder an der Umgebung des Aufrufers
und könnte auf einen ganz anderen Stand zeigen als den, den `bin/install` gelegt hat.

Absolut und nicht als bloßer Kommandoname, weil aus Dock oder Finder gestartete Clients
die Shell-`PATH` nicht erben; `~/.local/bin` fehlt dort typischerweise. Lässt sich kein
Binary auflösen, meldet jedes Ziel `MCPStateNoCommand` und es wird nichts geschrieben:
eine Registrierung, die auf nichts zeigt, ist schlechter als keine.

#### Eine Menge akzeptierter Formen statt eines Sollwerts

Geschrieben wird genau eine Form, geprüft wird gegen eine Menge. `reflect.DeepEqual`
gegen `mcpEntry()` ist deshalb weg; an seiner Stelle stehen drei Prädikate in `mcp.go`:

- `acceptedMCPCommandForms()` — die Menge selbst, als Liste benannter Formen. Heute zwei:
  jeder absolute Pfad, dessen Dateiname `k-playbook` ist, **auch der eines fremden
  `$HOME`** (`isInstalledCommandForm`), und der bloße Kommandoname `k-playbook`
  (`isPortableCommandForm`) — die eincheckbare Form, die kein `$HOME` nennt. Eine
  weitere Form ist ein weiterer Listeneintrag; der Vergleich selbst bleibt unverändert.
- `isOutdatedMCPCommand()` — „veraltet" im engen Sinn: der abgelöste Wrapper. Ein
  **relativer** Eintrag genügt mit der Endung `bin/k-playbook`, ein **absoluter** muss
  auf `k-playbook/bin/k-playbook` enden. Ohne diese Trennung fiele
  `~/.local/bin/k-playbook` selbst darunter — es endet ebenfalls auf `bin/k-playbook` —
  und die Auto-Korrektur schriebe endlos.
- `mcpEntryCommand()` — liest Kommando und Argumente aus dem vorgefundenen Eintrag,
  ohne den Rest zu bewerten. Zusätzliche Schlüssel wie ein von Hand gesetztes `timeout`
  bei OpenCode überleben damit; der frühere Strukturvergleich hätte sie weggeschrieben.

`legacyWrapperCommand` und `legacyWrapperTail` in `mcp.go` sowie
`legacyInstructionsCommand` in `instructions.go` sind bewusst **eigene** Konstanten und
nicht aus einem Wrapper-Bezeichner abgeleitet. Die Datei `bin/k-playbook` gibt es im
Quell-Repo nicht mehr; genau deshalb müssen die drei Konstanten sie überleben — sie sind
das Einzige, was ein Bestandsprojekt noch von ihr weg migriert. Sie werden erst
entfernt, wenn kein Projekt mehr aus dem Wrapper-Modell aktualisiert.

Der fremde `$HOME` ist eine Entscheidung mit Preis, und der steht in
[`docs/mcp.md`](../../docs/mcp.md): teilen sich Host und Container ein Repository bei
getrennten HOMEs, bleibt MCP in der jeweils anderen Umgebung tot, ohne dass die
Auto-Korrektur greift. Die Alternative wäre schlechter — beide Umgebungen erklärten sich
in derselben Datei wechselseitig für veraltet.

Die portable Form hat ihren eigenen Preis, und auch der steht dort: der bloße Name
hängt an der `PATH`, und aus Dock oder Finder gestartete Clients erben sie nicht. Sie ist
deshalb ausdrücklich **kein** Schreibziel — `MCPCommand()` liefert weiter den absoluten
Pfad —, sondern nur eine Form, die eine eingecheckte Registrierung tragen darf, ohne von
der Auto-Korrektur überschrieben zu werden. `isPortableCommandForm` prüft bewusst ohne
`path.Clean` auf Gleichheit mit dem Namen: `./k-playbook` bliebe sonst als „aktuell"
hängen, obwohl es einen Projektpfad meint.

#### Zwei Schreibwege, eine Entscheidungsstelle

| Weg | Einstieg | Schreibt bei |
|---|---|---|
| ausdrücklich | `ApplyMCP()`, Klick auf *Einrichten* | allem, was nicht zur Menge gehört |
| selbsttätig | `RepairMCP()`, Clone-Update und jeder Start | ausschließlich `MCPStateOutdated` |

Beide gehen durch `mcpTargetNeedsWrite()`; das `onlyOutdated`-Flag ist der einzige
Unterschied. Die Enge des zweiten Weges ist die Idempotenz-Zusage: er legt keine Datei
an, ergänzt keinen fehlenden Eintrag und fasst keine akzeptierte Form an. Ohne das machte
jeder Start die getrackten MCP-Dateien eines Projekts dreckig, und ein Repo mit
eingecheckter Registrierung käme nie an einem sauberen Arbeitsbaum vorbei.

Gerufen wird `RepairMCP()` von `Update()` (`project/update.go`, Ergebnis in
`UpdateResult.MCPRepaired`) und von `runGUI()` (`cmd/k-playbook/gui.go`). Beide Stellen
sind nötig: der `git pull` erreicht die Dateien nicht, weil sie im Hauptverzeichnis
liegen und nicht im Clone, und ein `git pull` von Hand oder `make -C k-playbook
installer-update` geht am Update-Handler ganz vorbei — für den ist der Start der
Auffangweg. Ein **Entfernen** gibt es weiterhin nicht: die Oberfläche richtet ein, sie
räumt nicht ab.

#### Die Bedingung, die bleibt

Der eingetragene Pfad sagt, welches Binary startet — nicht, welches Projekt gemeint ist.
Der Server löst das zur Laufzeit über die Aufwärtssuche nach `K-PLAYBOOK.yaml` ab seinem
Arbeitsverzeichnis auf, und das ist das des Assistenten. Weicht `Environment.SearchedFrom`
von `Environment.ProjectDir` ab, wurde schon die Oberfläche nicht im Hauptverzeichnis
gestartet; dann zeigt `GET /api/mcp` `workdirMismatch` und der Hinweis wird deutlich
statt beiläufig.

### Die Seite /mcp

`webui/mcp.go` bedient neben `GET`/`POST /api/mcp` noch `GET /api/mcp/tools` — den
Werkzeug-Selbsttest. Er ist ein eigener Endpunkt, weil dahinter ein Subprozess steht: als
Teil von `GET /api/mcp` bremste er die Startseite aus. Aufgerufen wird er nur von
`mcp.js`, also erst beim Öffnen der Seite.

Gestartet wird der **registrierte Befehl** mit dem Hauptverzeichnis als
Arbeitsverzeichnis, nicht der laufende Prozess: nur so misst die Seite das, was der
Assistent später bekommt. Die Mechanik ist dieselbe wie in `cmd/k-playbook/mcp_test.go` —
`initialize`, `notifications/initialized`, `tools/list`, und stdin bleibt offen, bis die
Antworten da sind.

**Ohne die geerbte Shell-`PATH`.** `mcpProbeEnv()` streicht `PATH` aus der Umgebung und
setzt die minimale System-`PATH`, die launchd einem GUI-Programm mitgibt. Das ist der
Fall, den der Test abbilden soll: ein aus Dock oder Finder gestarteter Client erbt keine
Login-Shell. Mit der `PATH` der Shell, in der die Oberfläche gestartet wurde, meldete der
Test grün, während der Client scheitert — er misst dann eine Umgebung, die es beim Client
nicht gibt. Leer wäre falsch, der Server ruft seinerseits `git` auf.

Kein installiertes Binary, keine Antwort, kein verwertbares JSON: alles davon ist ein
**Ergebnis** der Seite, keine Störung — sie zeigt „Server antwortet nicht" samt Grund.
Der Handler darf unter keinen Umständen hängen bleiben und die Seite mitnehmen.

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

**Das gelesene Schema bleibt nachsichtig.** `tools/list` liefert je Parameter ein
JSON-Schema, und dessen `type` darf zwei Formen haben: einen Namen (`"string"`) und eine
Liste von Namen (`["null","array"]`). Die zweite ist keine Ausnahme — das Go-SDK erzeugt
sie für jeden optionalen Parameter, der aus einem Zeiger- oder Slice-Feld kommt, und die
eigenen Werkzeuge dieses Servers nutzen sie. Ein blankes `string` an dieser Stelle wäre
deshalb kein Anzeigefehler gewesen, sondern ein falscher Befund: `json.Unmarshal` bricht
die **ganze** Antwort ab, und die Seite meldete „Server antwortet nicht" für einen Server,
der einwandfrei geantwortet hat. `schemaType` in `webui/mcp.go` liest deshalb beide Formen
und macht aus der Liste `null | array`; eine dritte, unbekannte Form bleibt still leer und
kippt den Selbsttest nicht. Dieselbe Zurückhaltung gilt für jedes weitere Feld, das aus
`tools/list` gelesen wird: es beschreibt eine fremde Antwort, nicht den eigenen Zustand.

## Lebenszyklus

Der Server ist ein **Hintergrunddienst je Projekt**. Der argumentlose Aufruf ist nur der
Client: er endet, sobald der Browser offen ist, und das Terminal ist wieder frei. Der
Server hängt an keinem Fenster und an keinem Terminal — er bleibt stehen, bis ihn
`k-playbook stop` oder der Knopf `Dienst beenden` beendet, ein Update ein neues Binary
verlangt oder ihn `idleTimeout` (60 Minuten) lang niemand mehr fragt. Er bindet auf
`127.0.0.1:0`, nimmt also einen freien Port.

**Ein Server je Projekt, das Arbeitsverzeichnis trägt die Fachlogik.** Alle Handler
rufen pro Anfrage `project.Detect()`, und das leitet das Projekt aus `os.Getwd()` ab.
Der abgekoppelte Prozess behält deshalb das Arbeitsverzeichnis seines Starts; das
`chdir("/")` klassischer Daemonisierung wäre hier genau der Fehler, der alles bricht.
Mehrere Projekte aus einem Prozess zu bedienen ist bewusst nicht vorgesehen.

### Die Laufzeitdatei

`internal/guiproc` hält je Projekt eine Datei unter `$XDG_RUNTIME_DIR/k-playbook/`
(Rückfall `$XDG_STATE_HOME/k-playbook/`, sonst `~/.local/state/k-playbook/`, angelegt
mit 0700), benannt nach den ersten 16 Hex-Zeichen des SHA-256 über den **Schlüssel**.
Der Schlüssel ist das aufgelöste `ProjectDir` aus `project.Detect()` — nicht das
Arbeitsverzeichnis, das bei einem Start aus einem Unterverzeichnis davon abweicht; ohne
Installation gilt das Arbeitsverzeichnis. Client und Server berechnen ihn über dieselbe
Funktion, `guiproc.Key()`. Die Datei trägt Schlüssel, Adresse, PID, Version, Build-Kennung
und die Startzeit des Prozesses; daneben liegt `<hash>.log` mit stdout und stderr des
Servers, bei jedem Start neu.

Geschrieben wird sie nach dem Binden mit `O_CREAT|O_EXCL`, damit zwei gleichzeitige
Aufrufe nicht beide einen Server hochziehen: der Verlierer endet mit einer Meldung im
Log, sein Elternprozess liest die Datei des Gewinners und öffnet dessen Server. Legt
`POST /api/config` die Konfiguration an und ändert sich dadurch das aufgelöste
`ProjectDir`, schlüsselt der Server um — neue Datei, alte weg. Beim Beenden verschwindet
die Datei über `defer`; SIGINT und SIGTERM werden gleich behandelt, damit sie auch nach
einem `kill` weg ist.

Warum das Laufzeitverzeichnis und nicht das Projekt: die Installation ist read-only,
`k-playbook-local/` gehört dem Projekt. `$XDG_RUNTIME_DIR` trennt Host und DevContainer,
weil es je Sitzung vergeben wird. Der Rückfall ins Home — auf macOS immer — nimmt ein
geteiltes Home als bekannte Grenze in Kauf: ist dort der Projektpfad identisch gemountet,
sieht die andere Seite eine PID aus fremdem Namensraum und ein Loopback, das nicht
antwortet, ersetzt die Datei durch ihre eigene, und beide Seiten laufen mit eigenem
Server. Kein Schaden, aber die Datei flattert zwischen ihnen.

### Gefunden wird der Server, nicht der Port

Ein Port aus einer alten Datei kann einem fremden Prozess gehören, eine PID nach
unsauberem Ende neu vergeben sein. `guiproc.Inspect()` prüft deshalb in zwei Stufen und
liefert eines von fünf Ergebnissen:

1. **Prozessidentität**: die PID lebt (`Signal 0`, `EPERM` zählt als lebend) **und** ihre
   Startzeit passt zu der in der Datei — Linux aus `/proc/<pid>/stat` (Feld 22, hinter
   der letzten `)`, gegen `btime`), macOS über `ps -o lstart=`, Toleranz 3 Sekunden. Der
   Server schreibt seine Startzeit über dieselbe Funktion. Ist die PID tot oder passt die
   Startzeit nicht, ist die Datei **verwaist** — nur dann wird sie verworfen. Eine Prüfung
   des Prozessnamens gibt es nicht; `/proc/<pid>/comm` ist auf 15 Zeichen begrenzt.
2. **`/api/health`** muss mit demselben Schlüssel und derselben PID antworten. Bleibt die
   Antwort aus oder kommt sie von einem Fremden, **lebt der Server ohne Antwort**: die
   Datei bleibt liegen, der Client startet nichts und verweist auf `k-playbook stop`. Die
   Datei eines lebenden eigenen Prozesses zu löschen hieße, beim nächsten Aufruf einen
   zweiten Server für dasselbe Projekt hochzuziehen.
3. Antwortet der eigene Server, entscheidet der **Stand**: gleich → **läuft unter dieser
   URL**, anders → **läuft mit anderem Stand**. Ohne Datei: **nicht vorhanden**.

Der Stand ist `guiproc.Identity` — Version **und** Build-Kennung —, und `Matches()` legt
fest, welche von beiden entscheidet.

Die Version wird beim Build mit `-ldflags -X` in das Binary gestempelt. Der Wert kommt
aus der `VERSION` des Standes, der gebaut wird: `make dist`, `make dist-host` und
`make dev-install` lesen sie über `INSTALLER_BUILD_VERSION`, der Release-Workflow liest
dieselbe Datei und stempelt denselben Wert in die vier Assets. Der Bootstrap
`make -C k-playbook install` stempelt nichts; er lädt das fertig gestempelte Asset.

Genau daraus folgt, dass die Version allein nicht reicht: ein lokaler Entwicklungsbuild
und das Release-Asset derselben `VERSION` tragen dieselbe Kennung, und zwei
aufeinanderfolgende `make gui` ebenfalls. Der laufende Server bliebe stehen und bediente
weiter mit altem Code, samt eingebetteter Assets.

Deshalb kommt die **Build-Kennung** dazu: `guiproc.OwnBuild()`, Größe und Änderungszeit
der Datei hinter `os.Executable()`, als `"<bytes>-<unixnano>"`. Jede eingespielte Datei
unterscheidet sich darin — `make dev-install` wie `bin/install` schreiben das Binary neu.
Kein Inhalts-Hash: das wären 16 MB je Aufruf, und ein Fehlurteil kostet hier nur einen
überflüssigen Serverneustart, nie falschen Code.

`Matches()` vergleicht die Build-Kennung; die Version zählt nur, wenn die **eigene**
Kennung fehlt — `os.Executable()` nicht auflösbar —, damit ein Prozess ohne eigene
Auskunft nicht bei jedem Aufruf den laufenden Server abräumt. Ein Server, der gar keine
Kennung meldet, gilt dagegen als anderer Stand: er läuft aus einem Binary von vor dieser
Erweiterung, und das *ist* ein Wechsel. So löst der erste Aufruf mit dem neuen Binary die
alten Daemons von selbst ab.

Beide Werte hält jeder Prozess beim **Start** fest und liest sie nicht von der Platte,
sonst wäre ein Wechsel dort nie zu erkennen — bei der Kennung wiegt das doppelt: nach
einem `mv` über die Datei zeigt `os.Executable()` auf einen gelöschten Inode, und ein
Server, der erst auf Nachfrage nachsähe, meldete die Kennung des **neuen** Binaries statt
seiner eigenen. Der Schlüssel in `/api/health` wird dagegen je Anfrage berechnet: nach
dem Umschlüsseln muss schon die nächste Antwort den neuen tragen.

### Der Aufruf

`runGUI()` in `cmd/k-playbook/gui.go` pflegt zuerst den Wirt — `cleanUpLegacy()`,
die Migrationsbereinigung, `protectProjectInstallation()` — und zwar bei **jedem** Aufruf,
auch bei dem, der nur ein Fenster öffnet; im Server liefen sie nur beim allerersten Start.
Dann entscheidet `reuseOrStart()` nach dem Ergebnis der Einordnung:

| Ergebnis | Handlung |
|---|---|
| läuft unter dieser URL | URL ausgeben, Browser öffnen, Ende 0 |
| läuft mit anderem Stand | `POST /api/shutdown`, warten, bis die Datei weg oder die PID tot ist (10 s, über den 5 s `shutdownTimeout`), dann starten; läuft die Zeit ab, wie „lebt ohne Antwort". Die Meldung nennt den Unterschied: andere Version, oder anderer Build derselben Version |
| verwaist | Datei löschen, starten |
| lebt ohne Antwort | nichts starten; Meldung mit Dateipfad und Hinweis auf `k-playbook stop`, Ende ≠ 0 |
| nicht vorhanden | starten |

**Starten heißt abkoppeln.** `guiproc.Spawn()` startet das eigene Binary
(`os.Executable()`) mit der Umgebung plus `K_PLAYBOOK_SERVE=1`,
`SysProcAttr{Setsid: true}`, unverändertem Arbeitsverzeichnis, stdin aus `/dev/null`,
stdout und stderr in die Logdatei. Der Elternprozess wartet bis zu 10 Sekunden, bis die
Laufzeitdatei die PID des Kindes trägt und `/api/health` mit dem eigenen Schlüssel
antwortet, gibt dann die URL aus, öffnet den Browser und endet. Endet das Kind vorher,
hat vielleicht ein gleichzeitiger Aufruf gewonnen: dann gilt dessen Server. Antwortet es
nicht rechtzeitig, bekommt es SIGTERM, und die Meldung trägt das Log. Das
Plattformspezifische — `Setsid`, `Signal 0`, SIGTERM, Startzeit — steht in eigenen Dateien
mit Build-Tags (`_unix.go`, `_linux.go`, `_darwin.go`); eine Windows-Fassung käme nur
dazu.

Die Umgebungsmarke `K_PLAYBOOK_SERVE=1` ist der verdeckte Servermodus: `webui.Serve()`
ohne Wirt-Pflege und ohne Browser, Ausgaben nur ins Log. Der Server löscht die Marke aus
seiner Umgebung, damit kein Kindprozess sie erbt und selbst zum Server wird.

### Im Server

Der Heartbeat ist geblieben, hat aber seine Rolle gewechselt: er ist das **Lebenszeichen
des Fensters**, nicht mehr die Lebensader des Servers. Ein offenes Fenster ruft alle 10
Sekunden `/api/health` und hält den Server damit aus dem Leerlauf — solange jemand
hinsieht, läuft er. Jede Anfrage, nicht nur diese, setzt `lastRequestAt`; `watchIdle`
prüft alle 30 Sekunden und beendet nach `idleTimeout` ohne Anfrage. Der Leerlauf zählt ab
dem Start, damit auch ein nie besuchter Server nicht ewig steht. Entfallen sind
`clientHeartbeatTimeout`, `clientGoneShutdownDelay`, `POST /api/client-gone` und im
Browser `notifyClientGone` samt `leavingForOwnPage` — sie existierten nur, um den Server
über einen Seitenwechsel zu retten, und ohne Suizid gibt es nichts mehr zu retten. Ein
Mechanismus statt zweier.

Im Browser sperrt ein ausgebliebenes Lebenszeichen erst nach drei Fehlschlägen in Folge,
damit ein Aussetzer die Seite nicht totstellt. Die Sperrfläche der Startseite ist keine
Sackgasse: sie bietet „Erneut verbinden" — antwortet der Server, lädt die Seite neu —,
und erst wenn das scheitert, den Weg über `k-playbook` im Terminal. Der Knopf
`Dienst beenden` sagt, was er tut: er beendet den Server für alle Fenster.

`POST /api/update` beendet den Server nach der Antwort, wenn zum neuen Stand ein anderes
Binary gehört als das laufende — und ein alter Daemon soll nicht stehen bleiben. Die
Bedingung dafür hat zwei Teile, `binaryOutdated()` in `webui/update.go`: der Pull muss die
`VERSION` bewegt haben (`UpdateResult.VersionChanged`) **und** der laufende Prozess darf
diese Version nicht schon tragen.

Der zweite Teil ist der Entwicklungsfall: dort steht unter `~/.local/bin` längst das Binary
des neuen Standes, weil `make dev-install` es gebaut hat, während der Clone noch dem zuletzt
gepushten Commit folgt. Holt er ihn nach, wechselt dort die `VERSION` — und der Dienst
beendete sich, obwohl nichts zu tun war, und verwies auf den Bootstrap. Der lädt das
Release-Asset: im Entwicklungsrepo der falsche Weg, und vor dem Release liegt das Asset
nicht einmal.

Der schlichtere Vergleich „laufende Version ≠ `VERSION` der Installation" taugt dafür
nicht: im Entwicklungsrepo ist das Binary regelmäßig **neuer** als der Clone, und jeder
Pull ohne Versionswechsel schlüge dann in die Aufforderung um, ein älteres Binary zu
installieren. Fehlt eine der beiden Angaben, wird nichts verlangt.

Die Entscheidung liegt bewusst in `webui` und nicht in `project.Update()`: nur der Server
kennt die Version, aus der er selbst läuft. `UpdateResult` meldet deshalb die Tatsache
(`VersionChanged`, `Version`), nicht die Aufforderung. Das deckt den Weg über die Oberfläche; nach einem `git pull` von
Hand greift der Standvergleich des Clients beim nächsten Aufruf — und der erkennt über
die Build-Kennung auch ein frisch gebautes Binary bei unveränderter `VERSION`. Ein
portstabiler Neustart, bei dem der Dienst den Listener an das neue Binary vererbt, wäre
möglich und ist bewusst nicht gebaut — er lohnt erst, wenn Updates im Alltag stören.

`k-playbook stop` nutzt dieselbe Einordnung: läuft der eigene Server, `POST
/api/shutdown` und warten; antwortet er nicht, SIGTERM an die PID — die Identitätsprüfung
hat zuvor gesichert, dass es der eigene Prozess ist. Eine verwaiste Datei wird gelöscht,
eine fehlende ist eine Auskunft und kein Fehler.

### Herkunftsprüfung

Alle `POST`-Routen stehen hinter `sameOrigin`: trägt die Anfrage einen `Origin`-Header,
muss dessen Host-Anteil (Schema abgeschnitten, Rest bis zum nächsten `/`) dem
`Host`-Header gleichen, sonst 403. Fehlt `Origin` — `curl`, `stop` —, gilt die Anfrage
als eigene Herkunft. Keine Fixierung auf `127.0.0.1:<port>` und keine Loopback-Liste:
hinter einer Portweiterleitung (VS Code weicht bei belegtem Port aus, Codespaces liefert
eine Fremddomain) kommt der Host-Header vom Browser unverändert und wäre sonst gerade dort
abgewiesen. Das Schema bleibt außen vor, weil Codespaces TLS terminiert. Der Zufallsport
macht DNS-Rebinding ohnehin unpraktikabel. Der Grund für die Prüfung ist die Lebensdauer:
der Prozess lebt jetzt Stunden statt Minuten, und eine beliebige Seite im Browser könnte
sonst Endpunkte treffen, hinter denen `git pull` und Schreibvorgänge stehen.

Der Browser wird nicht automatisch geschlossen, wenn der Server endet. Browser blockieren
das in vielen Fällen, und `open`/`xdg-open` liefern keinen verlässlichen Tab-Handle.

### Browser öffnen

`Announce()` in `webui/server.go` gibt die URL aus und öffnet den Browser. Aufgerufen wird
es vom Client-Pfad in `cmd/k-playbook/gui.go`, nicht vom Server — der hat kein Terminal
mehr, in dem ein Browser sinnvoll wäre. Die Kandidaten liefert `browserOpeners()` in
`webui/browser.go`, in dieser Reihenfolge:

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
- Die Oberfläche läuft als Hintergrunddienst je Projekt, wiedergefunden über eine
  Laufzeitdatei im Laufzeitverzeichnis des Nutzers. Ein zweiter Aufruf öffnet nur den
  Browser; das Terminal ist nach jedem Aufruf frei.
- Kein fester Port, kein Lesezeichen. Ein fester Port ließe sich bei einem Server je
  Projekt nicht kollisionsfrei vergeben und wäre für fremde Seiten im Browser
  vorhersagbar. Dass die URL bei jedem Serverstart wechselt, ist in Kauf genommen — der
  Aufruf gibt sie aus und öffnet den Browser ohnehin.
- Das Laufzeitverzeichnis statt Projekt oder Installation: die eine ist read-only, das
  andere gehört dem Projekt. Ein geteiltes Home im Rückfall ist bekannte Grenze.
  Verworfen wird eine Laufzeitdatei nur, wenn ihr Prozess nachweislich nicht mehr läuft —
  PID tot oder Startzeit passt nicht.
- Same-Origin-Vergleich statt Host-Liste für schreibende Endpunkte: der Host-Anteil von
  `Origin` gegen den `Host`-Header, ohne Schema.
- Kein `status`-Kommando; neu ist allein `stop`.
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
Oberfläche im Vorschlagsmodus läuft. Ohne `Origin`-Header kommt `curl` an der
Herkunftsprüfung vorbei; ein Browser täte das nicht.

Läuft der Server schon, gibt `k-playbook` ohne Argument die URL erneut aus. Die
Laufzeitdatei liegt unter `$XDG_RUNTIME_DIR/k-playbook/<hash>.json`, das Log des
abgekoppelten Prozesses als `<hash>.log` daneben.

Ein laufender Server nutzt weiter den Code im Speicher — der nächste `k-playbook` in
diesem Projekt löst ihn aber ab, sobald ein anderes Binary unter `~/.local/bin` liegt.
Erkannt wird das an der Build-Kennung, nicht an der `VERSION`; ein `k-playbook stop` nach
Backend- oder Asset-Änderungen ist deshalb nur noch nötig, wenn dasselbe Binary neuen
Code bedienen soll, was es nicht kann.

## Der alte Stand

Der Code vor dem Umbau lag bis September 2026 als `installer/_old/` im Baum und ist
gelöscht. Wer ihn nachschlagen will, findet ihn in der Git-Historie, zuletzt in
Commit `02f78d3`, zum Beispiel mit
`git show 02f78d3:installer/_old/internal/install/migrate.go`.

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
