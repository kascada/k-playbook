# Versionsinventar

Der Vertrag für das Versionsinventar: Datenmodell, Pin-Taxonomie, Quellenarten,
Normalisierung, Abweichungen, Vertrauensgrenze, Quellenkonfiguration und die Regel zu
Zeitstempel und Byte-Stabilität.

Diese Seite ist die **eine** Festlegung. Sammler, Subkommando, Command, Oberfläche und
Tests berufen sich auf sie und formulieren nichts davon neu. Was hier nicht steht, ist
nicht festgelegt.

## Was das Inventar ist — und was nicht

Das Inventar ist eine vollständige, reproduzierbare Übersicht der **deklarierten**
Versionen eines Projekts: Pakete, Tools, Runtimes, Container-Images, Helm-Charts und
Helm-Abhängigkeiten, erhoben aus deklarativen Quellen.

Es liest ausschließlich Dateien. Es fragt kein Netz, installiert nichts und führt kein
gefundenes Werkzeug aus. Es bewertet nicht, welche Version zur Laufzeit tatsächlich
aktiv ist, und es fasst Umgebungen nicht zusammen — es dokumentiert jede gefundene
Aussage mit Kontext und Herkunft.

Abgrenzung zu `docs/libs/`: `/k-docs-tools` erzeugt **kuratierte, pitfall-orientierte**
Steckbriefe für ausgewählte Tools. Das Inventar ist **vollständig und
quellenorientiert** und fragt nicht nach Auswahl. Beide teilen sich die Pin-Taxonomie,
damit nicht zwei Sprachen für dieselbe Sache entstehen. Bei abweichenden
Versionsangaben gibt das Inventar die Auskunft; `docs/libs/` wird deswegen nicht
automatisch umgeschrieben, sondern der Unterschied wird als Hinweis gemeldet.

## Namen

Diese Namen stehen fest und werden überall unverändert benutzt.

| Was | Name |
|---|---|
| Subkommando | `k-playbook inventory` |
| Command | `/k-doc-inventory` (Datei `commands/k-doc-inventory.md`) |
| Docs-Modul | `commands/_docs/inventory.md` |
| Inventardatei | `k-playbook-local/docs/versions/inventory.md` |
| Quellenkonfiguration | `k-playbook-local/version-sources.yaml` |
| Erzeuger im Frontmatter | `generated.by: k-doc-inventory` |

Der Command trägt bewusst den Singular `doc`, obwohl die übrige Familie `k-docs-`
schreibt. Das ist eine ausdrückliche Entscheidung und kein Tippfehler; wer sie
angleichen will, ändert sie an allen Stellen zugleich, nicht still an einer.

`generated.by` ist auf **beiden** Aufrufwegen `k-doc-inventory` — auch wenn das
Subkommando direkt aufgerufen wurde. Die Herkunft benennt den Erzeuger, nicht den
Aufrufweg. `docs/versions/` ist die einzige Herkunft, in die dieser Erzeuger schreibt,
und er schreibt in keine andere.

## Datenmodell einer Inventarzeile

Eine Inventarzeile ist genau **eine Aussage aus genau einer Quelle**. Zwei Quellen, die
dasselbe sagen, sind zwei Zeilen — zusammengeführt wird erst in der Darstellung.

| Feld | Pflicht | Bedeutung |
|---|---|---|
| `ecosystem` | ja | Welche Welt die Aussage trifft: `python`, `go`, `node`, `rust`, `ruby`, `php`, `java`, `elixir`, `container`, `helm`, `ci`, `runtime`. `runtime` trägt die Sprach- und Werkzeuglaufzeiten selbst — `runtime/python`, `runtime/node`, `runtime/go` —, damit dieselbe Laufzeit aus `.python-version`, aus einem Manifest und aus einer Setup-Action in **einer** Gruppe landet. |
| `name` | ja | Der Gegenstand, kanonisch normalisiert (siehe „Normalisierung"). |
| `kindOfThing` | ja | Was der Gegenstand ist: `package`, `tool`, `runtime`, `image`, `chart`, `chart-dependency`, `action`. |
| `version` | ja | Die Versionsangabe **wortgleich wie in der Quelle**, ohne Umschreibung. Fehlt sie ganz, steht hier der leere String. |
| `versionNormalized` | nein | Die vergleichbare Fassung, wenn eine gebildet werden konnte (siehe „Normalisierung"). Sonst leer. |
| `pin` | ja | Die Pin-Art aus der Taxonomie unten. |
| `digest` | nein | Der Inhalts-Digest, wenn die Quelle einen nennt: `sha256:…` oder ein voller Commit-SHA. |
| `context` | ja | Das Umgebungslabel: `lokal`, `dev`, `devcontainer`, `ci`, `deployment`. |
| `contextOrigin` | ja | Woher das Label stammt: `default` (aus der Quellart abgeleitet) oder `configured` (aus der Quellenkonfiguration). |
| `scope` | nein | Der deklarierte Einsatzbereich innerhalb der Quelle, soweit sie einen kennt: `main`, `dev`, `optional`, `test`, `build`. |
| `sourceFile` | ja | Die Quelldatei, projektrelativ. Liegt sie außerhalb der Projektwurzel, steht hier der absolute Pfad. |
| `sourceKey` | ja | Abschnitt oder Schlüssel innerhalb der Datei, als Pfad: `dependencies.fastapi`, `jobs.test.steps[2].uses`, `services.db.image`. |
| `sourceLine` | nein | Die Zeilennummer, wenn die Aussage genau einer Zeile zuzuordnen ist. Sonst leer — dann trägt `sourceKey` die Auffindbarkeit. |
| `group` | ja | Der Gruppenschlüssel für die Abweichungsbildung: `<ecosystem>/<name>`. |
| `deviation` | nein | Verweis auf die Abweichung, zu der diese Zeile gehört; leer, wenn der Gegenstand nur eine Aussage hat oder alle Aussagen gleich sind. |
| `note` | nein | Ein sichtbarer Hinweis zu genau dieser Zeile, etwa „Wert aus Variable, nicht auflösbar". |

`sourceFile`, `sourceKey` und — wo vorhanden — `sourceLine` zusammen müssen ausreichen,
um die Aussage **ohne Suche** wiederzufinden. Das ist der Prüfstein für jeden neuen
Parser.

Eine Folge davon ist beabsichtigt: verschiebt sich eine Zeile in einer Quelldatei, ändert
sich das Inventar. Das ist keine Instabilität, sondern eine geänderte Herkunft.

## Pin-Taxonomie

Die Taxonomie ist die aus `commands/_docs/tools.md`, **erweitert**. Die drei vorhandenen
Werte behalten dort wörtlich ihre Bedeutung; sie werden hier zitiert, nicht umformuliert:

> `exact` (`==1.2.3`, `1.2.3`) / `range` (`^1`, `>=1,<2`, `~1.2`) / `floating` (`*`, keine
> Angabe).

Dazu kommen drei Werte:

| Wert | Bedeutung |
|---|---|
| `digest` | Die Quelle bindet an einen unveränderlichen Inhalt statt an eine Version: `image@sha256:…`, eine Action auf vollem Commit-SHA, ein Helm-Chart mit `digest`. Steht daneben noch ein Tag, bleibt er in `version` — die Bindung ist trotzdem der Digest. |
| `local` | Es gibt keine externe Version: lokal gebaut oder aus dem Arbeitsbaum genommen. `build:` statt `image:` in Compose, `replace … => ../x` in `go.mod`, ein Editable- oder Pfad-Dependency, `FROM <stage>` auf eine Stage derselben Datei. |
| `unknown` | Eine Angabe ist da, lässt sich aber nicht klassifizieren: unauflösbare Variable (`${IMAGE_TAG}`), nicht lesbarer Ausdruck, defekte Syntax im sonst lesbaren Umfeld. |

`floating` und `unknown` werden nicht verwechselt: `floating` heißt „bewusst nicht
gepinnt", `unknown` heißt „nicht ermittelbar". Wo `unknown` steht, gehört ein `note`
dazu, der sagt warum.

Ein hingeschriebener Container-Tag ist `exact`, auch wenn er keine Versionsnummer ist:
`ubuntu-22.04` legt so fest wie `1.2.3`. Das ist dieselbe Lesart, die dieser Vertrag für
CI-Actions ohnehin trifft („ein Tag `exact`"). `floating` bleibt den Tags vorbehalten,
die ausdrücklich nicht festlegen sollen: `latest`, `main`, `master`, `edge`, `stable`,
`nightly`, `dev`, `devel` — und dem fehlenden Tag.

Es entsteht keine zweite Taxonomie. Braucht ein Parser einen weiteren Wert, wird die
Taxonomie hier erweitert — nicht lokal ein siebter Wert erfunden.

## Kontext und Umgebungslabels

Die Labels sind eine **geschlossene** Menge:

`lokal`, `dev`, `devcontainer`, `ci`, `deployment`

Ohne Konfiguration ergibt sich das Label deterministisch aus der Quellart
(`contextOrigin: default`):

| Quelle | Default-Label |
|---|---|
| Paketmanifeste und Lockfiles aller Ökosysteme | `lokal` |
| `.tool-versions`, `.python-version`, `.nvmrc`, `.ruby-version` | `lokal` |
| `.devcontainer/**` | `devcontainer` |
| `Dockerfile`, `Dockerfile.*`, `*/Dockerfile` | `deployment` |
| `docker-compose*.y{a,}ml`, `compose*.y{a,}ml` | `dev` |
| Helm: `Chart.yaml`, `Chart.lock`, `values*.yaml` | `deployment` |
| CI: `.github/workflows/*.y{a,}ml`, `.gitlab-ci.yml` | `ci` |

Ein Eintrag in der Quellenkonfiguration überschreibt das Label für seine Quelle
(`contextOrigin: configured`). Ein anderes Label als die fünf ist ein Fehler und wird
sichtbar abgelehnt — siehe „Fehlerfälle".

## Standardquellen und ihre Semantik

Ohne Konfiguration sucht der Sammler diese Quellen unterhalb der Projektwurzel. Was er
findet, liest er vollständig; was er nicht kennt, ignoriert er stillschweigend — nur eine
**gefundene, aber nicht lesbare** Quelle wird gemeldet.

Aus einem Lockfile werden in jedem Ökosystem ausschließlich die **direkten**
Abhängigkeiten seines zugehörigen Manifests geführt. Bei Workspaces ist die
Referenzmenge die Vereinigung des Wurzelmanifests und aller deklarierten
Mitgliedsmanifeste; sie gilt nur, wenn jedes Mitglied aufgelöst, nicht ausgeschlossen und
lesbar ist. Transitive Lockfile-Einträge treten niemals an die Stelle einer fehlenden
Referenzmenge.

Nicht betreten werden dabei die Verzeichnisse, die ein Werkzeug befüllt hat: `.git`,
`.hg`, `.svn`, `node_modules`, `vendor`, `bower_components`, `.venv`, `venv`,
`__pycache__`, `.tox`, `.nox`, `.mypy_cache`, `.pytest_cache`, `.ruff_cache`, `target`,
`.gradle`, `.terraform`, `.next`, `.cache`, `_build`, `deps`. Ohne diese Grenze hieße
„unterhalb der Projektwurzel" auch: jedes Manifest jeder heruntergeladenen
Abhängigkeit — und aus Lockfiles führt dieser Vertrag ausdrücklich nur direkte
Abhängigkeiten. Ein Verzeichnis mit gepflegtem Inhalt steht nicht auf dieser Liste.

### Wo die Standarderkennung nicht sucht

Zwei Bereiche liegen im Projekt und werden trotzdem nicht von selbst durchsucht. Das ist
kein Leseverbot — die Vertrauensgrenze erlaubt sie weiterhin —, sondern eine Aussage
darüber, was ohne Zutun als Quelle **des Projekts** gilt.

1. **Die Installation `<projekt>/k-playbook/`.** Sie ist ein Clone des Werkzeugs, liegt in
   jedem Zielprojekt an derselben Stelle, trägt dort dieselben Manifeste und wird bei jedem
   Update vollständig ersetzt. Ihre `go.mod` beschreibt die Abhängigkeiten von k-playbook,
   nicht die des Projekts; die Abweichung zwischen ihr und einem gleichnamigen Paket des
   Projekts wäre in jedem Projekt dieselbe und ließe sich in keinem beheben — in
   `k-playbook/` wird nie geschrieben. Eine Aussage, die überall gleich ist und nirgends
   behebbar, ist keine Erkenntnis.
2. **Jedes Muster aus `exclude:`** der Quellenkonfiguration. Damit nimmt ein Projekt seine
   Testfixtures und Beispielprojekte aus: gepflegter Inhalt, dessen Versionen absichtlich
   alt oder widersprüchlich sind und über das Projekt nichts aussagen. Wo solches Material
   liegt und wie es heißt — `testdata/`, `tests/fixtures/`, `spec/fixtures/` —, weiß nur
   das Projekt. Deshalb steht es in der Konfiguration und nicht in einer Rateliste im
   Werkzeug.

Beides wirkt **nur** auf die Standarderkennung. Wer eine Quelle daraus im Inventar haben
will, schreibt sie in `sources:`; ein ausdrücklicher Eintrag schlägt jeden Ausschluss und
trägt dann sein eigenes Umgebungslabel.

**Kein Ausschluss ist still.** Jede Regel steht im Abschnitt „Nicht durchsuchte Bereiche"
der Inventardatei — mit Muster, Herkunft (`installation` oder `configured`), Grund und der
Zahl der dadurch übergangenen Quellen. Die Summe steht zusätzlich im Frontmatter unter
`inventory.sources-excluded` und in der Übersicht. Auch eine Regel ohne Treffer wird
geführt: „hier wurde nichts gefunden" und „hier wurde nicht gesucht" sind zwei verschiedene
Aussagen, und der Unterschied gehört in die Datei.

Der Unterschied zur `skippedDirs`-Liste darüber ist der Grund, nicht die Wirkung: dort
steht, was ein Werkzeug befüllt hat und deshalb niemandem gehört; hier steht, was jemandem
gehört — nur nicht diesem Projekt.

### Python

`pyproject.toml`, `requirements*.txt`, `constraints*.txt`, `setup.py`, `setup.cfg`,
`Pipfile`, `Pipfile.lock`, `poetry.lock`, `uv.lock`, `.python-version`.

- Manifest ist die Absicht, Lockfile der aufgelöste Stand. Beide werden gelesen, beide
  als eigene Zeilen geführt; widersprechen sie sich, ist das eine Abweichung.
- Extras und Marker bleiben in `version` erhalten, gehen aber nicht in den Namen ein.
- `-e .`, Pfad- und VCS-Dependencies sind `local`. Die eine Ausnahme ist `-e .` selbst:
  es meint das Projekt und hat weder Gegenstandsnamen noch Version, wird also nicht als
  Zeile geführt.
- `.python-version` ist ein `runtime`-Eintrag, kein Paket.

### Go

`go.mod`, `go.sum`, `tools.go`, `.go-version`.

- `require` liefert `package`-Zeilen mit `exact`-Pin — Go-Module sind immer exakt.
- `replace` auf einen Pfad ist `local`; `replace` auf eine andere Version ist eine eigene
  Zeile mit `note`.
- Die `go`-Direktive und `toolchain` sind `runtime`-Einträge.
- `go.sum` wird nicht als Versionsquelle geführt: es wiederholt `go.mod` und trüge nur
  Rauschen bei.
- `tools.go` nennt Werkzeuge, aber keine Versionen — die stehen in `go.mod`. Es liefert
  deshalb ebenfalls keine eigenen Zeilen.

### Node und JavaScript

`package.json`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `.nvmrc`,
`.node-version`.

- `dependencies`, `devDependencies`, `optionalDependencies` und `peerDependencies` gehen
  in `scope`.
- `engines.node` und `packageManager` sind `runtime`-Einträge.
- Lockfiles folgen der allgemeinen Direkt-Regel dieses Abschnitts.

### Weitere Manifesttypen

Dieselben, die `/k-docs-tools` bereits erkennt: `Cargo.toml`/`Cargo.lock`,
`Gemfile`/`Gemfile.lock`, `composer.json`/`composer.lock`, `pom.xml`,
`build.gradle`/`build.gradle.kts`, `mix.exs`/`mix.lock`. Semantik wie oben: Manifest und
Lockfile beide; Lockfiles folgen der allgemeinen Direkt-Regel dieses Abschnitts, Scope
 aus dem Abschnitt.

### Container

`Dockerfile`, `Dockerfile.*`, `docker-compose*.y{a,}ml`, `compose*.y{a,}ml`.

- `FROM [--flag=wert …] <image>:<tag> [AS <stage>]` ist ein `image`-Eintrag;
  `@sha256:…` macht daraus `digest`. Flags gehören nicht zur Image-Referenz, und ein
  Stage-Alias registriert die lokale Stage.
- `FROM [--flag=wert …] <stage> [AS <stage>]` auf eine Stage derselben Datei ist `local`
  und wird als solche geführt,
  nicht weggelassen.
- `ARG`-Werte werden nur aufgelöst, wenn ihr Default in derselben Datei steht. Sonst
  `unknown` mit `note`.
- In Compose: `services.<name>.image` ist ein `image`-Eintrag; `services.<name>.build`
  ohne `image` ist `local`.
- Explizite Tool-Versionen aus `RUN`-Zeilen werden **nicht** geraten. Ein `apt-get install
  curl` ist keine Versionsaussage; ein `RUN pip install x==1.2.3` schon — geführt wird nur,
  was eine Version wörtlich nennt.

### DevContainer

`.devcontainer/devcontainer.json`, `.devcontainer/**/devcontainer.json`, dazu ein
`Dockerfile` oder eine Compose-Datei, auf die sie verweist.

- `image` ist ein `image`-Eintrag.
- `features` sind `tool`-Einträge; der Feature-Name ist der Gegenstand, die Version steht
  hinter dem `:` oder in `version`.
- Verweist die Datei auf ein `Dockerfile` oder eine Compose-Datei, bekommen deren Funde
  das Label `devcontainer`, nicht ihr eigenes Default.

### Helm

`Chart.yaml`, `Chart.lock`, `values*.yaml`, dazu Image-Referenzen in beliebigen
`values*.yaml`.

- `Chart.yaml`: `version` ist ein `chart`-Eintrag, `appVersion` ein `runtime`-Eintrag,
  jeder Eintrag aus `dependencies` ein `chart-dependency`. Der `appVersion`-Eintrag steht
  im Ökosystem `runtime`, nicht in `helm`: sonst lägen Chart-Version und Anwendungsversion
  in derselben Gruppe und erzeugten eine Abweichung, die keine ist.
- `Chart.lock` liefert den aufgelösten Stand derselben Abhängigkeiten — eigene Zeilen,
  Widerspruch zu `Chart.yaml` ist eine Abweichung.
- In `values*.yaml` gelten die Schlüsselpaare `image.repository` + `image.tag` und ein
  einzelner `image`-String als Image-Referenz. `image.digest` macht daraus `digest`.
  Andere Schlüssel werden nicht geraten.
- Mehrere `values-*.yaml` sind mehrere Aussagen desselben Gegenstands und damit der
  Normalfall einer Abweichung.

### CI

`.github/workflows/*.y{a,}ml`, `.gitlab-ci.yml`, `.gitlab/**/*.yml`.

- `uses: owner/repo@ref` ist ein `action`-Eintrag. Ein voller Commit-SHA ist `digest`,
  ein Tag `exact`, ein Branch `floating`.
- `container:`, `services.*.image` und `image:` sind `image`-Einträge.
- Setup-Actions mit Versionseingabe (`actions/setup-go` mit `go-version`,
  `setup-node` mit `node-version`, `setup-python` mit `python-version`) liefern
  zusätzlich einen `runtime`-Eintrag.

## Normalisierung

Normalisiert wird nur, was Vergleichbarkeit herstellt. Die Rohangabe bleibt immer
erhalten.

**Namen.** Kleinschreibung; im Python-Ökosystem zusätzlich `_` und `.` zu `-` (PEP 503).
Node-Scopes bleiben erhalten (`@scope/paket`). Go-Modulpfade bleiben vollständig, ohne
`/v2`-Kürzung. Container-Images werden auf `registry/namespace/name` normalisiert; ein
fehlendes `docker.io/library` wird **nicht** ergänzt — sonst sähen zwei gleiche Aussagen
verschieden aus, je nachdem wer sie geschrieben hat. Der Gruppenschlüssel ist
`<ecosystem>/<name>` und damit ökosystemlokal: ein Python-`redis` und ein Image `redis`
sind zwei Gegenstände.

**Versionen.** `version` ist wortgleich die Quelle. `versionNormalized` entsteht nur bei
`pin: exact`: führendes `v`, `==` und Leerzeichen fallen weg, der Rest bleibt. Bei
`range`, `floating`, `digest`, `local` und `unknown` bleibt `versionNormalized` leer —
ein normalisierter Bereich wäre eine Interpretation, und das Inventar interpretiert nicht.

**Digests.** In der Form `sha256:<64 hex>` beziehungsweise als voller 40-stelliger
Commit-SHA. Kurz-SHAs werden nicht verlängert und gelten als `unknown`.

**Lokale Builds.** `pin: local`, `version` leer, `note` sagt woher: Stage, Pfad,
Build-Kontext.

**Quellen- und Zeilenangabe.** `sourceFile` ist projektrelativ mit `/` als Trenner, auch
unter Windows. `sourceKey` ist ein Punktpfad; Listenindizes stehen in eckigen Klammern und
zählen ab `0`. `sourceLine` zählt ab `1` und wird nur gesetzt, wenn der Parser die Aussage
genau einer Zeile zuordnen kann.

## Abweichungen

Gruppiert wird über `group` (`<ecosystem>/<name>`), nicht über den Anzeigenamen.

Innerhalb einer Gruppe gilt:

1. Haben alle Zeilen dieselbe `version` **und** denselben `pin`, gibt es keine Abweichung.
   Die Zeilen werden in der Kontexttabelle geführt, ihre Herkünfte nebeneinander genannt.
2. Unterscheiden sie sich, entsteht eine Abweichung mit einer der beiden Arten:
   - `umgebungsbedingt` — die abweichenden Zeilen tragen **verschiedene** `context`-Werte.
     Das ist der Normalfall und meist Absicht: lokal eine neuere Version als im Deployment.
   - `widersprüchlich` — die abweichenden Zeilen tragen **denselben** `context`. Das ist
     der Fall, der eine Frage aufwirft: Manifest gegen Lockfile, zwei Compose-Dateien
     derselben Umgebung, `Chart.yaml` gegen `Chart.lock`.
3. Eine Abweichung wird **nie** aufgelöst, zusammengefasst oder auf einen „richtigen" Wert
   reduziert. Sie wird ausgewiesen, mit allen beteiligten Zeilen und deren Herkunft.

Beide Arten stehen im Abschnitt „Abweichungen" der Inventardatei, `widersprüchlich`
zuerst. Die Zahl der Abweichungen ist die Zahl der Gruppen mit Abweichung, nicht die Zahl
der beteiligten Zeilen.

Ein Unterschied zwischen `docs/libs/` und dem Inventar ist **keine** Abweichung in diesem
Sinn: er entsteht nicht aus zwei Quellen, sondern aus zwei Dokumenten. `/k-docs` meldet
ihn als eigenen, nicht reparierenden Befund; das Inventar gilt.

Verglichen wird das Frontmatter-Paar `version`/`version-pin` einer Datei aus `docs/libs/`
mit dem Gegenstand gleichen Namens in den Kontexttabellen des Inventars — zugeordnet über
den Namen hinter dem Gruppenschlüssel `<ecosystem>/<name>`. Ein Tool, das im Inventar
nicht vorkommt, ist kein Befund, und eine Lib-Datei ohne `version` ebenso wenig.
`docs/libs/` wird dabei **nicht** umgeschrieben, auch nicht nach Bestätigung: diese Dateien
gehören `/k-docs-tools`.

## Vertrauensgrenze

Über die Quellenkonfiguration lassen sich Pfade außerhalb des Projekts freigeben:
Host-Dateien, fremde Deployment-Repos. Gelesen werden sie von einem Binary, das im selben
Prozessmodell auch einen lokalen Webserver betreibt. Symlinks, `..` und Glob-Ausbrüche
sind deshalb kein Randfall.

Die Grenze ist **einmal** definiert und gilt für jeden Aufrufweg gleich — Subkommando,
Command und die Web-API aus Task 043. Sie wird in Etappe 3 an **genau einer Stelle** in
`internal/` implementiert: eine Funktion, die einen angefragten Pfad prüft und entweder
den geprüften, absoluten Pfad oder einen Fehler liefert. Kein Leser öffnet eine Datei an
dieser Funktion vorbei.

### Erlaubte Wurzeln

Erlaubt sind genau zwei Sorten Wurzel:

1. Die **Projektwurzel** — `project.dir` aus `k-playbook context`, also das Verzeichnis
   der `K-PLAYBOOK.yaml`. Sie ist immer erlaubt und steht nicht in der Konfiguration.
2. Jede Wurzel, die in `version-sources.yaml` unter `roots:` ausdrücklich freigegeben ist.

Nichts sonst. Es gibt keine impliziten Wurzeln: ein absoluter Pfad in `sources:` gibt
seine Wurzel **nicht** selbst frei. Wer außerhalb lesen will, schreibt die Wurzel hin —
damit die Frage „was darf dieses Binary lesen" an einer Stelle und ohne Glob-Auswertung
zu beantworten ist.

`playbook.dir` ist keine Sonderwurzel. Es liegt unterhalb der Projektwurzel und **darf**
gelesen werden wie jedes andere Verzeichnis; geschrieben wird dort ohnehin nie. Dass die
Standarderkennung es trotzdem nicht durchsucht, ist eine andere Frage — sie steht unter
„Wo die Standarderkennung nicht sucht" und ändert an dieser Grenze nichts: ein Eintrag in
`sources:`, der dorthin zeigt, wird gelesen.

### Pfadnormalisierung

Vor **jeder** Prüfung, in dieser Reihenfolge:

1. **Keine Expansion.** `~`, `$VAR`, `%VAR%` und Shell-Metazeichen werden nicht
   ausgewertet, sondern sind Bestandteil des Pfads. Ein Pfad, dessen Bedeutung von der
   Umgebung des Aufrufers abhängt, bedeutet auf dem CLI-Weg etwas anderes als im
   Webserver-Prozess — genau das darf eine Vertrauensgrenze nicht.
2. **Absolut machen.** Ein relativer Pfad wird gegen die Projektwurzel aufgelöst, nie
   gegen das Arbeitsverzeichnis des Prozesses.
3. **Lexikalisch säubern.** `.` und `..` werden aufgelöst, Mehrfachtrenner
   zusammengezogen. Ein `..`, das über eine Wurzel hinausführt, wird dadurch sichtbar und
   fällt in der Prüfung durch.
4. **Symlinks auflösen.** Der vollständige Pfad wird aufgelöst, einschließlich aller
   Elternsegmente. Geprüft wird das **aufgelöste** Ergebnis, nicht der angefragte Pfad.
   Ein Symlink innerhalb des Projekts, der nach außen zeigt, ist damit ein Ausbruch und
   wird abgelehnt, solange sein Ziel nicht unter einer freigegebenen Wurzel liegt. Die
   Wurzeln selbst werden ebenso aufgelöst, bevor verglichen wird.
5. **Prüfen.** Der aufgelöste Pfad muss gleich einer Wurzel sein oder unterhalb einer
   Wurzel liegen — segmentweise verglichen, nicht als Zeichenketten-Präfix. `/srv/deploy`
   erlaubt `/srv/deploy/x`, aber nicht `/srv/deploy-alt/x`.

Globs werden erst **nach** Schritt 1 bis 3 auf ihrem statischen Anteil expandiert, und
zwar ausschließlich innerhalb einer erlaubten Wurzel. **Jedes** Ergebnis der Expansion
durchläuft anschließend die Schritte 4 und 5 einzeln. Ein Glob ist damit nie ein Weg an
der Prüfung vorbei.

Gelesen werden nur reguläre Dateien. Verzeichnisse, Gerätedateien, FIFOs und Sockets
werden abgelehnt. Die Obergrenze für die Dateigröße ist 8 MiB; wird sie überschritten,
ist das eine sichtbare Ablehnung und kein Teil-Lesen.

Implementiert ist die Grenze als `inventory.Boundary` in
`installer/internal/inventory/trust.go`. Sie ist die einzige Stelle, die eine Quelle
öffnet: `Check` prüft einen Pfad, `Expand` löst ein Glob auf, `ReadFile` liest. Nicht
benutzt wird dafür `internal/pathnorm` — dessen `Normalize` beantwortet die Frage, ob
zwei SARIF-Pfade auf dieselbe Stelle zeigen, schreibt dafür klein und wirft ein führendes
`/` weg. Für eine Sicherheitsprüfung wäre genau das falsch: `/etc/passwd` und
`etc/passwd` dürfen hier nicht dasselbe sein.

### Ablehnung ist sichtbar

Jede Ablehnung erzeugt eine Meldung, die im Ergebnis steht — im Abschnitt „Abgelehnte
Quellen und Hinweise" der Inventardatei, in der Ausgabe des Subkommandos und in der
Antwort der API. Sie nennt den angefragten Pfad, den aufgelösten Pfad und den Grund.

**Ein stilles Überspringen gibt es nicht.** Eine Quelle, die konfiguriert ist und nicht
gelesen werden konnte, ist eine Lücke im Inventar; eine Lücke, die niemand sieht, ist
schlimmer als ein Fehler.

Der Lauf bricht deswegen **nicht** ab: die übrigen Quellen werden erhoben, das Inventar
wird geschrieben, und die Ablehnungen stehen darin. Nur eine unlesbare
Quellenkonfiguration bricht ab — siehe „Fehlerfälle".

## Quellenkonfiguration

`k-playbook-local/version-sources.yaml`. YAML, weil `K-PLAYBOOK.yaml` es auch ist und ein
zweites Format nichts brächte.

**Schreibregel.** Die Datei ist handgepflegt. Command, Subkommando und jede spätere
Oberfläche dürfen nur nach ausdrücklicher Bestätigung des Nutzers in sie schreiben, und
dann ausschließlich ergänzend: bestehende Einträge, Kommentare und Reihenfolge bleiben
unangetastet. Ohne Bestätigung wird nicht geschrieben.

**Zustand über `context`.** Commands lesen Konfiguration ausschließlich aus
`k-playbook context`. Der Zustand dieser Datei — vorhanden oder fehlend, die freigegebenen
Wurzeln, die konfigurierten Quellen — gehört deshalb in die Kontextausgabe. Schema und
Feldnamen stehen unten unter „Zustand in der Kontextausgabe"; den Leser baut Etappe 3 als
**einzige** Implementierung, die sowohl der Sammler als auch
`installer/internal/project/context.go` benutzen.

### Felder

| Schlüssel | Pflicht | Bedeutung |
|---|---|---|
| `schema_version` | ja | Aktuell `1`. Ein anderer Wert bricht den Lauf ab, statt Felder zu deuten, die etwas anderes bedeuten könnten. |
| `roots` | nein | Liste absoluter Pfade, die zusätzlich zur Projektwurzel gelesen werden dürfen. Leer oder fehlend heißt: nur die Projektwurzel. |
| `sources` | nein | Liste zusätzlicher Quellen. Leer oder fehlend heißt: nur die Standardquellen unterhalb der Projektwurzel. |
| `exclude` | nein | Liste von Mustern, in denen die Standarderkennung nicht sucht. Je Muster ein Pfad relativ zur Projektwurzel; `*` steht für beliebig viele Zeichen innerhalb eines Segments, `**` für beliebig viele Segmente. Ein Muster ohne Wildcard trifft den Pfad selbst und alles darunter. Ein absolutes Muster wird sichtbar abgelehnt: es hinge vom Rechner ab und träfe auf einem anderen nichts. |

Je Eintrag in `sources`:

| Schlüssel | Pflicht | Bedeutung |
|---|---|---|
| `path` | ja | Datei oder Glob. Relativ zur Projektwurzel oder absolut; absolut nur innerhalb einer Wurzel aus `roots`. |
| `kind` | ja | Quellart: `auto`, `python`, `go`, `node`, `rust`, `ruby`, `php`, `java`, `elixir`, `dockerfile`, `compose`, `devcontainer`, `helm`, `ci`, `tool-versions`. `auto` bestimmt die Art am Dateinamen wie bei den Standardquellen. |
| `env` | ja | Umgebungslabel: `lokal`, `dev`, `devcontainer`, `ci`, `deployment`. |
| `note` | nein | Anzeigetext für die Quellenliste des Inventars. |
| `optional` | nein | `true` heißt: fehlt die Datei, ist das kein Hinweis. Ohne den Schlüssel ist eine konfigurierte, aber fehlende Quelle ein sichtbarer Hinweis. |

Die Standardquellen werden durch `sources` **ergänzt**, nicht ersetzt. Es gibt keinen
Schalter, der die Standarderkennung abschaltet: ein Inventar, das die Projektquellen
nicht führt, wäre keines. `exclude` schaltet sie ebenfalls nicht ab — es nimmt benannte
Bereiche aus, sichtbar und einzeln, und ein Eintrag in `sources` holt jeden davon wieder
herein.

### Vorlage

Der folgende Inhalt ist zugleich die **Beispieldatei und die Vorlage**, die
`LocalStructure()` in Etappe 2 anlegt: eine gültige, leere Konfiguration mit erklärendem
Kommentar. Etappe 2 übernimmt ihn wortgleich in den `fileTemplate()`-Zweig von
`installer/internal/project/local.go` und entwirft ihn nicht neu.

```yaml
# Versionsquellen für `k-playbook inventory`
#
# Diese Datei ist handgepflegt. k-playbook schreibt nur nach ausdrücklicher
# Bestätigung in sie, und dann ausschließlich ergänzend: bestehende Einträge,
# Kommentare und Reihenfolge bleiben erhalten.
#
# Ohne Einträge erhebt `k-playbook inventory` die Standardquellen unterhalb der
# Projektwurzel — Manifeste, Lockfiles, Dockerfiles, Compose, DevContainer,
# Helm und CI. Hier stehen nur zusätzliche Quellen und die Wurzeln außerhalb
# des Projekts, die dafür gelesen werden dürfen.
#
# Vollständige Beschreibung: k-playbook/docs/versionsinventar.md

schema_version: 1

# Zusätzlich lesbare Wurzeln, je ein absoluter Pfad. Die Projektwurzel ist
# immer erlaubt und gehört nicht hierher. Was nicht unter einer dieser Wurzeln
# liegt, wird abgelehnt — sichtbar gemeldet, nicht stillschweigend übergangen.
#
#   roots:
#     - /srv/deploy
roots: []

# Zusätzliche Quellen. Je Eintrag:
#   path: Datei oder Glob, relativ zur Projektwurzel oder absolut
#   kind: auto, python, go, node, rust, ruby, php, java, elixir, dockerfile,
#         compose, devcontainer, helm, ci, tool-versions
#   env:  lokal, dev, devcontainer, ci, deployment
#   note: optionaler Anzeigetext
#
#   sources:
#     - path: /srv/deploy/values-prod.yaml
#       kind: helm
#       env: deployment
#       note: Produktionswerte aus dem Deployment-Repo
sources: []

# Bereiche, in denen die Standarderkennung nicht suchen soll — je ein Muster
# relativ zur Projektwurzel, `*` für ein Segment, `**` für beliebig viele.
# Gedacht für Testfixtures und Beispielprojekte: gepflegter Inhalt, dessen
# Versionen nichts über dieses Projekt aussagen.
#
# Gesperrt ist damit nichts. Jeder Ausschluss steht mit der Zahl der
# übergangenen Quellen im Inventar, und eine Quelle daraus kommt wieder hinein,
# sobald sie unter `sources:` steht.
#
# Die Installation `k-playbook/` ist immer ausgenommen und gehört nicht hierher:
# sie ist ein Clone des Werkzeugs und sagt nichts über dieses Projekt.
#
#   exclude:
#     - tests/fixtures/**
exclude: []
```

### Zustand in der Kontextausgabe

`k-playbook context` führt den Zustand dieser Datei unter dem Schlüssel
`versionSources`. Damit beantwortet die Kontextausgabe die Frage vollständig, und kein
Command muss `version-sources.yaml` selbst öffnen — dieselbe Regel, die schon für
`K-PLAYBOOK.yaml` gilt.

```json
"versionSources": {
  "path": "/pfad/zum/projekt/k-playbook-local/version-sources.yaml",
  "present": true,
  "schemaVersion": 1,
  "roots": ["/srv/deploy"],
  "sources": [
    {
      "path": "/srv/deploy/values-prod.yaml",
      "kind": "helm",
      "env": "deployment",
      "note": "Produktionswerte aus dem Deployment-Repo",
      "optional": false
    }
  ],
  "exclude": ["tests/fixtures/**"],
  "error": ""
}
```

| Feld | Immer da | Bedeutung |
|---|---|---|
| `path` | ja | Absoluter Pfad der Datei — auch wenn sie fehlt, damit klar ist, wo sie hingehört. |
| `present` | ja | Ob die Datei da ist. |
| `schemaVersion` | nein | Die von der Datei deklarierte Fassung. Fehlt, solange nichts gelesen wurde. |
| `roots` | nein | Die freigegebenen Wurzeln, wortgleich wie in der Datei. Die Projektwurzel steht **nicht** darin — sie ist immer erlaubt. |
| `sources` | nein | Die konfigurierten Zusatzquellen, in der Reihenfolge der Datei. Je Eintrag `path`, `kind`, `env`, `note`, `optional` — dieselben Namen wie die YAML-Schlüssel. Ein Eintrag mit unbekanntem `kind` oder `env` steht **mit** darin: die Kontextausgabe zeigt die Datei so, wie sie dasteht, und ihn wegzulassen hieße, sie anders darzustellen. Abgelehnt wird er erst im Erhebungslauf, und dort sichtbar. |
| `exclude` | nein | Die Muster, in denen die Standarderkennung nicht sucht, wortgleich wie in der Datei. Die feste Regel für die Installation steht **nicht** darin — sie kommt nicht aus der Datei. Ein absolutes und deshalb abgelehntes Muster steht ebenfalls nicht darin: anders als bei `sources` ist ein Muster kein Eintrag, den man ansehen könnte, sondern eine Regel, die entweder gilt oder nicht. |
| `error` | nein | Gesetzt, wenn die Datei da, aber nicht lesbar oder von unbekannter Fassung ist. |

Die Feldnamen sind die YAML-Schlüssel der Datei, in camelCase wie überall sonst in der
Kontextausgabe; nur `schema_version` wird dabei zu `schemaVersion`. Es gibt keine
Umbenennung und keine zweite Begriffswelt.

Drei Zustände, und mehr nicht:

- **vorhanden und gültig** — `present: true`, `error` leer. `roots`, `sources` und
  `exclude` tragen den Inhalt; leere Listen heißen „nichts konfiguriert", nicht „nicht
  gelesen".
- **fehlt** — `present: false`, `error` leer, `roots`, `sources` und `exclude` leer. Kein
  Fehler: es gelten die Standardquellen unterhalb der Projektwurzel.
- **defekt** — `present: true`, `error` gefüllt, `roots`, `sources` und `exclude` leer.

Der Kontextaufruf bricht bei einer defekten Datei **nicht** ab. Er steht am Anfang jedes
Commands; eine defekte Zusatzkonfiguration darf nicht jeden Command lahmlegen. Sichtbar
bleibt der Zustand trotzdem — dafür ist `error` da. Der **Erhebungslauf** des Inventars
bricht sehr wohl ab, so wie es unter „Fehlerfälle" steht; das ist kein Widerspruch,
sondern die Trennung zwischen Auskunft geben und Erheben.

Fehlt das Feld ganz, ist die Installation älter als es. Das ist der einzige Fall, in dem
ein Command es nicht auswerten kann.

## Die Inventardatei

`k-playbook-local/docs/versions/inventory.md`. Das Verzeichnis `docs/versions/` ist die
fünfte Docs-Herkunft; es wird nicht vom Einrichten angelegt, sondern beim ersten Lauf
seines Erzeugers — wie `docs/code/`, `docs/libs/` und `docs/extracted/`.

### Frontmatter

Vollständig, sonst melden `/k-docs` Schritt 3 und `/k-docs-index` Schritt 4 einen Befund.
`generated.by` allein genügt nicht.

```yaml
---
type: Version Inventory
title: Versionsinventar
description: Vollständige Übersicht der deklarierten Versionen dieses Projekts, nach Umgebung getrennt und mit Herkunft je Zeile.
tags: [versions, inventory, dependencies]
status: stable
generated: { by: k-doc-inventory, at: <RFC 3339> }
inventory:
  sources-configured: <N>
  sources-read: <N>
  entries: <N>
  deviations: <N>
  rejected: <N>
  sources-excluded: <N>
---
```

`generated.at` ist der Erhebungszeitpunkt — es gibt nur diesen einen Zeitstempel.

### Aufbau

Der Rumpf ist deterministisch aufgebaut; jeder Abschnitt steht immer, auch wenn er leer
ist, damit ein Diff nicht zwischen „Abschnitt fehlt" und „Abschnitt leer" unterscheiden
muss:

1. `# Versionsinventar` und ein Absatz mit Erzeuger, Erhebungszeitpunkt und dem Hinweis,
   dass die Datei erzeugt wird.
2. `## Übersicht` — Zahl der Einträge, Quellen, Abweichungen und Ablehnungen.
3. `## <Kontext>` je Umgebungslabel in der festen Reihenfolge `lokal`, `dev`,
   `devcontainer`, `ci`, `deployment`; Label ohne Einträge werden weggelassen. Darin eine
   Tabelle mit Gegenstand (`<ecosystem>/<name>`, der Gruppenschlüssel), Art
   (`kindOfThing`), Version, Pin-Art, Scope und Herkunft. Die Herkunftszelle trägt Datei
   und Zeile, den `sourceKey` und, wo vorhanden, Digest und `note`.
4. `## Abweichungen` — je Gruppe ein Block mit Art (`widersprüchlich` vor
   `umgebungsbedingt`), den beteiligten Zeilen und deren Herkunft.
5. `## Ausgewertete Quellen` — jede gelesene Datei mit Quellart, Label, Zahl der
   Einträge und, falls für die Quelle konfiguriert, ihrer `note`. Eine Quelle aus der
   Quellenkonfiguration trägt den Zusatz `(konfiguriert)` an ihrer Quellart; sonst wäre
   nicht zu sehen, welche Zeile woher kommt.
6. `## Nicht durchsuchte Bereiche` — je Ausschlussregel Muster, Herkunft
   (`installation` oder `configured`), Zahl der übergangenen Quellen und Grund. Der
   Abschnitt steht immer, auch wenn keine Regel etwas getroffen hat.
7. `## Abgelehnte Quellen und Hinweise` — jede Ablehnung mit angefragtem Pfad,
   aufgelöstem Pfad und Grund; dazu die Hinweise aus unlesbaren oder unbekannten Quellen.

**Sortierung.** Kontexte in der festen Reihenfolge oben. Innerhalb eines Kontexts nach
`ecosystem`, dann `name`, dann `sourceFile`, dann `sourceLine`, zuletzt nach `sourceKey`
und `version` — alles aufsteigend, mit byteweisem Vergleich, nicht
gebietsschema-abhängig. Die beiden letzten Schlüssel entscheiden die Fälle, in denen zwei
Aussagen aus derselben Zeile stammen — zwei gepinnte Werkzeuge in einer `RUN`-Zeile etwa;
ohne sie entschiede dort die Reihenfolge des Parsers. Abweichungen nach Art, dann nach
`group`. Quellen nach `sourceFile`. Zwei Läufe über denselben Bestand erzeugen damit
dieselbe Datei, unabhängig von der Reihenfolge, in der das Dateisystem Einträge liefert.

## Byte-Stabilität und Zeitstempel

Der Vertrag, auf den Sammler und Tests sich gleichermaßen berufen:

1. Ein Lauf erhebt und rendert das vollständige Ergebnis **im Speicher**.
2. Existiert die Inventardatei, wird das Ergebnis mit ihrem Bestand verglichen —
   **ausgenommen `generated.at`**, und ausgenommen nichts anderes.
3. Sind sie gleich, wird **gar nicht geschrieben**. Die Datei bleibt byte-identisch,
   einschließlich ihres alten Zeitstempels und ihrer Änderungszeit im Dateisystem.
4. Unterscheiden sie sich, wird die Datei vollständig neu geschrieben, mit
   `generated.at` = Zeitpunkt dieses Laufs.
5. Existiert sie nicht, wird sie geschrieben.

Daraus folgt: `generated.at` ist der Zeitpunkt der letzten **inhaltlichen Änderung**, nicht
der des letzten Laufs. Wer wissen will, wann zuletzt erhoben wurde, erhebt neu — das
Ergebnis ist dieselbe Datei. Diese Auslegung ist die Auflösung des Widerspruchs zwischen
„Erhebungszeitpunkt in der Datei" und „ein wiederholter Lauf lässt die Datei
unangetastet"; sie wird nicht an anderer Stelle abweichend formuliert.

Die Ablehnungen aus der Vertrauensgrenze sind Inhalt: ändert sich die Menge der
abgelehnten Quellen, ändert sich die Datei. Für die Ausschlüsse gilt dasselbe — ein
hinzugefügtes Muster in `exclude:` ändert das Inventar, und zwar sichtbar an zwei Stellen:
die übergangenen Quellen fehlen, und die Regel steht mit ihrer Zahl im eigenen Abschnitt.

## Woher 043 den Status nimmt

**Entschieden: aus dem Frontmatter der Inventardatei. Es entsteht kein
Zwischenartefakt.**

Die Oberfläche aus Task 043 braucht vier Angaben maschinell: Stand, Zeitpunkt der letzten
Erhebung, Zahl der Quellen und Zahl der Abweichungen. Alle vier stehen im Frontmatter
(`generated.at`, `inventory.sources-*`, `inventory.deviations`), und der Stand ergibt sich
daraus, ob die Datei da ist.

Gelesen wird ausschließlich der YAML-Block zwischen den beiden `---`-Zeilen am
Dateianfang. Der Markdown-Rumpf wird **nie** geparst — keine Tabelle wird rückwärts
gelesen, keine Überschrift ausgewertet. Genau das war die Anforderung: der Status kommt
nicht aus dem Fließtext.

Das verbietet nicht, die Datei zu **lesen**. Sie ist Dokumentation und wird gelesen wie
jede andere: `/k-docs` sieht für seinen Abgleich mit `docs/libs/` in die Kontexttabellen
und nimmt von dort Gegenstand, Version und Pin-Art. Die Grenze verläuft zwischen Status
und Inhalt — der Status kommt aus dem Frontmatter, nirgends sonst, und keine Maschine
rechnet ihn aus dem Rumpf zurück.

Gegen ein eigenes JSON-Artefakt sprechen drei Dinge:

- Der Sammler braucht den Frontmatter-Leser **ohnehin**: der Vergleich „ohne Zeitstempel"
  aus der Byte-Stabilitätsregel setzt voraus, dass er den Bestand einliest und
  `generated.at` darin findet. Ein zweiter Speicherort wäre zusätzlicher Code, kein
  gesparter.
- Zwei Dateien, die dasselbe sagen, können auseinanderlaufen — beim Löschen von Hand, beim
  Zurücksetzen aus Git, bei einem abgebrochenen Lauf. Dann gäbe es zwei Antworten auf
  dieselbe Frage, und die Oberfläche zeigte womöglich die falsche.
- Ein Maschinenartefakt bräuchte einen Ort außerhalb der Docs-Herkünfte, eine Entscheidung
  über Versionierung und einen Eintrag in der Struktur — Aufwand für vier Zahlen, die
  bereits geschrieben werden.

**Folge für Etappe 3:** Der Frontmatter-Leser ist **eine** Funktion in `internal/`, die
einen Statuswert liefert — `inventory.ReadStatus`, sie liefert `inventory.Status`. Sammler, `/k-docs`, das Subkommando und die API aus 043 benutzen
dieselbe; 043 baut keinen zweiten Leser. Fehlt die Datei, ist das ein definierter Zustand
(„fehlt"), kein Fehler. Ist sie da, aber ihr Frontmatter unvollständig oder defekt, ist
das ein sichtbarer Befund und kein stilles Nullergebnis.

## Fehlerfälle

Jeder Fall hat ein definiertes, sichtbares Verhalten. Ein stilles Leerergebnis gibt es
nirgends.

| Fall | Verhalten |
|---|---|
| `version-sources.yaml` fehlt | Kein Fehler. Es gelten die Standardquellen unterhalb der Projektwurzel; die Kontextausgabe meldet die Datei als fehlend. |
| `version-sources.yaml` ist nicht lesbares YAML | **Abbruch** vor jeder Erhebung, mit Datei, Zeile und Meldung des Parsers. Es wird nichts geschrieben. Eine defekte Konfiguration halb zu deuten hieße, eine andere Vertrauensgrenze anzuwenden als die aufgeschriebene. |
| `schema_version` fehlt oder ist nicht `1` | **Abbruch**, wie bei `K-PLAYBOOK.yaml`. |
| Eine Wurzel in `roots:` ist nicht absolut | **Abbruch**. Eine relative Wurzel hinge vom Arbeitsverzeichnis des Aufrufers ab und bedeutete im Webserver-Prozess etwas anderes als auf dem CLI-Weg; eine so erklärte Vertrauensgrenze halb zu deuten wäre schlimmer als sie abzulehnen. |
| Unbekanntes `env`-Label in einem Eintrag | Der **Eintrag** wird abgelehnt und unter „Abgelehnte Quellen und Hinweise" geführt, mit dem gefundenen Wert und den fünf gültigen. Der Lauf geht weiter. |
| Unbekanntes `kind` in einem Eintrag | Wie beim Label: Eintrag abgelehnt, sichtbar, Lauf geht weiter. |
| Absolutes oder leeres Muster in `exclude:` | Das **Muster** wird abgelehnt und unter „Abgelehnte Quellen und Hinweise" geführt, mit Zeile und Grund. Es gilt dann nicht; der Lauf geht weiter. |
| Konfigurierte Quelle fehlt auf der Platte | Sichtbarer Hinweis, es sei denn der Eintrag trägt `optional: true`. |
| Pfad außerhalb der erlaubten Wurzeln | Sichtbare Ablehnung mit angefragtem und aufgelöstem Pfad. Der Lauf geht weiter. |
| Symlink zeigt aus jeder Wurzel heraus | Wie oben; gemeldet wird das aufgelöste Ziel, damit erkennbar ist, was tatsächlich gelesen worden wäre. |
| Bekannte Quelldatei ist defekt | Sichtbarer Hinweis mit Datei und Fehler; die übrigen Quellen werden erhoben. Keine erfundenen Einträge, kein Teilergebnis ohne Kennzeichnung. |
| Lockfile lesbar, zugehöriges Manifest fehlt oder ist nicht lesbar | Das Lockfile trägt keine Einträge bei. Ein sichtbarer Hinweis nennt Lockfile, erwartetes Manifest und den Grund; transitive Lockfile-Pakete ersetzen die direkten nicht. |
| Lockfile lesbar, zugehöriges Manifest ist durch `exclude:` ausgenommen | Das Lockfile trägt keine Einträge bei. Der Ausschluss bleibt zusätzlich im Abschnitt „Nicht durchsuchte Bereiche“ sichtbar; der Hinweis nennt die greifende Ausschlussregel. Ein ausdrücklich unter `sources:` genanntes Lockfile hebt den Ausschluss nur für sich auf, nicht für sein Manifest. |
| Workspace-Lockfile mit fehlendem, nicht auflösbarem, ausgeschlossenem oder nicht lesbarem Mitgliedsmanifest | Das Lockfile trägt insgesamt keine Einträge bei. Der Hinweis nennt Wurzelmanifest, Mitgliedsdeklaration beziehungsweise aufgelösten Pfad und Grund; eine teilweise Vereinigung der übrigen Mitglieder ist unzulässig. |
| Unbekannter Dateityp unterhalb der Projektwurzel | Stillschweigend übergangen. Nur was gesucht wird, kann fehlen. |
| Inventardatei da, Frontmatter defekt | Sichtbarer Befund. Der Lauf erhebt neu und schreibt die Datei, weil ein Vergleich nicht möglich ist. |

## Wo das hier gilt

Diese Seite ist der Vertrag für:

- den Sammler in `installer/internal/inventory/`, den Leser der Quellenkonfiguration in
  `installer/internal/versionsources/` und den YAML-Leser beider in
  `installer/internal/yamllite/` (Task 042, Etappe 3),
- das Subkommando `k-playbook inventory` und den Command `/k-doc-inventory` samt Modul
  `commands/_docs/inventory.md` (Task 042, Etappe 4),
- den Inventarbereich der Oberfläche und seine API (Task 043),
- die Fixtures und Tests beider Tasks.

Ändert sich etwas daran, ändert es sich hier — und die genannten Stellen ziehen nach.
