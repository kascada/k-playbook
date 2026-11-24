---
title: Die Felder des Briefings
description: Jedes Feld des Briefings und wozu es dient — die Abschnitte jeder Sitzung und der Teilabruf environment für die Umgebungsangaben einzelner Commands —, dazu die vollständige Liste der Bedingungen, unter denen eine Sitzung das Briefing neu lädt.
---

## Die Felder des Briefings

Das Briefing ist die Antwort von `k_playbook_briefing`, ersatzweise in der Shell der Klartext
von `k-playbook briefing`: Er zeigt jedes Feld, das ein Text aus dem Briefing liest, die
Abschnitte `now`, `project`, `kephalaion` und `links` gleich nach der Zeile `Sitzung:` unter
ihren Namen. Was jede Sitzung daraus **tun** muss, steht in den immer geltenden Anweisungen; hier
steht, was jedes Feld bedeutet. Der Vertrag für Entwickler ist `k-playbook:/docs/handbook/mcp.md`, „Briefing
Contract“ — dieses Blatt hält mit ihm Schritt, ein Test prüft die Vollständigkeit.

### Die Abschnitte des Briefings

| Feld | Bedeutung |
|---|---|
| `schemaVersion` | Die Fassung der Briefing-Ausgabe, heute `"9"`. |
| `session` | Die Kennung dieser Sitzung, mit jedem Briefing neu vergeben, etwa `20261024-183012-3fa9c1`. Sie steht in allem, was die Sitzung später wiederfinden muss — im Kopf eines Befunds unter `sitzung`, nach dem `/k-danke` sucht. Lädt eine Sitzung das Briefing neu, gilt die neue Kennung, und die frühere gehört weiter zu ihr. |
| `now.date`, `now.timestamp` | Der Moment des Aufrufs in der Zeitzone des Rechners. `date` ist `YYYY-MM-DD`, `timestamp` RFC 3339. |
| `project.dir` | Das Hauptverzeichnis — wo `.k-playbook/K-PLAYBOOK.yaml` liegt. Steht immer da. |
| `project.repoRoot`, `project.vcs` | Das Code-Repository für Git-Operationen, und ob es unter Versionskontrolle steht. |
| `project.languages` | Die Sprachen, für die das Projekt Werkzeuge braucht. Immer gefüllt — ein fehlender Eintrag fällt auf den Standard zurück, es gibt nichts zu deuten. |
| `project.available`, `project.message` | Ob die Konfiguration gelesen werden konnte. Wenn nicht, sagt `message` warum; `dir` steht trotzdem da, die übrigen Felder von `project` und `links` fehlen, das Briefing sonst gilt. |
| `kephalaion.ok`, `kephalaion.message` | Ob die Collection des Projekts erreichbar ist, und der Satz dazu, bei einem Fehler samt Abhilfe. `keph:`-Verweise sind nur erreichbar, wenn `ok` wahr ist. |
| `kephalaion.address` | `<hub>:<collection>` aus dem Block `kephalaion:` von `.k-playbook/K-PLAYBOOK.yaml` — der Wert für `collection` in den Kephalaion-Werkzeugen. |
| `kephalaion.user` | Der eigene Kephalaion-User: das, wofür `~` in einem `keph:`-Verweis steht. Nur bei `ok`. |
| `documents.always` | Das zusammengebaute Verzeichnis `k-playbook:/instructions/always/`: `ref`, `content` (der Text selbst, Includes eingesetzt — eine Sitzung muss ihn nicht eigens lesen), `parts` in der Reihenfolge des Texts, `problems` (übergangene Teile). |
| `documents.onDemand`, `documents.reviews`, `documents.guidelines` | Die Einträge der Sorte, je ein Dokument für sich: `ref`, `title`, `description`, `origin` (`dist`, `local`, `override` oder `disabled` für einen abgeschalteten Eintrag). Bei Anweisungen und Reviews sind mitgeliefert und projekteigen schon gegeneinander aufgelöst; Guidelines kommen nur mitgeliefert, ihre Herkunft ist immer `dist`. Gelesen wird über `ref` mit `k_playbook_doc_read`. Der Klartext zeigt je Eintrag Verweis und Titel, die Herkunft nur, wenn sie nicht `dist` ist; die Beschreibung steht nur im JSON. |
| `work.available`, `work.message`, `work.user` | Ob die Collection für die eigene Arbeit lesbar war und sonst warum nicht; `user` ist der eigene Kephalaion-User. |
| `work.tasks`, `work.workbench` | Die eigenen offenen Tasks, nach Nummer, und die eigenen aktiven Arbeitsstände, nach letzter Änderung, die neueste zuerst — **je höchstens 10**; offen heißt: nicht in `done/`. Je Eintrag `ref`, `title`, `updated` und `problem`, wenn Kephalaion das Frontmatter nicht lesen konnte oder der Titel vermutlich an „ #“ abgeschnitten ist — gemeldet, nicht repariert. Der Klartext nennt je Liste in einer Kopfzeile die Zahl aller und den Verzeichnis-Verweis, darunter je Eintrag Dateiname und Titel; der Verweis eines Eintrags ist Verzeichnis plus Dateiname. |
| `work.total` | Die Zahl aller eigenen offenen Tasks und aktiven Arbeitsstände, `tasks` und `workbench` — auch derer, die der Deckel aus den Listen hält. Ist sie größer als die Liste, liefert `k_playbook_doc_list` (kind `tasks` bzw. `workbench`) alle; der Klartext sagt dann „+ <n> weitere“. |
| `work.openFindings` | Die Zahl der offenen Befunde in `queue/findings/`. |
| `projectDocs` | Der Einstieg der Projektdokumentation, als Verweis. |
| `github.projectServices` | Die Dienste der persönlichen GitHub-Zugangsdatei, deren `repo` das Repo des Arbeitsverzeichnisses ist, in Dateireihenfolge, meist einer; leer ohne Datei, ohne Remote oder ohne passenden Dienst. Der Wert für `k-playbook github run <dienst>`. Im Klartext die Zeile `GitHub:` nach `Projektdoku:`, die dazu nennt, was `run` wählt. Das volle Objekt liegt im Teilabruf unter `environment.github`. |
| `health` | Die Umgebungsprüfung: `ok` (kein Eintrag der Stufe `error`; Warnungen lassen es wahr) und `problems`, je Eintrag `level` (`error` oder `warning`), `area` (`kephalaion`, `baseTools`, `versionSources`, `github`, `share`, `anchor`, `program`), `message` (ein Satz, nie ein Token oder Wert) und `hint` (die Abhilfe), sortiert nach Bereich, dann Stufe. Ohne Problem ist `problems` leer. Dieselbe Antwort wie `k_playbook_health` und `k-playbook health --json`; Einzelheiten unten. |
| `links` | Die Selbstheilung der Assistenten-Verknüpfungen, die das Briefing nebenbei anstößt: `links.healed` (was dazukam, wegfiel oder die Quelle wechselte: `added`, `removed`, `repointed`), `links.open` (Ziele, an denen noch etwas zu tun ist, je `path`, `state`, `detail`) und `links.note` (was das für diese Sitzung heißt). Nur da, wenn es etwas zu melden gab. |

Nicht mehr im Briefing: der Ort der Konfiguration, ihre Fassung, die Pflichtliste der
MCP-Server, die Git-Einstellungen der Oberfläche und von der Anbindung an Kephalaion Hub,
Collection, Konto, Rechte, Node und Stand des Abgleichs — kein Text las sie.

### Der Teilabruf `environment`

Umgebungsangaben, die nur einzelne Commands brauchen, stehen nicht im Briefing. Sie kommen über
den Teilabruf: `k_playbook_briefing` mit `part: "environment"`, ersatzweise `k-playbook briefing
--part environment`. Ein Command holt ihn an der Stelle, an der er eines der Felder braucht.

- Der Teilabruf ist kein Briefing: Er vergibt keine Sitzungskennung, liefert weder Anweisungen
  noch Arbeitsliste, schreibt nichts und gilt nicht als Neuladen.
- Er wird bei Bedarf frisch geholt, auch mehrmals je Sitzung. Seine Felder stehen neben denen
  des geladenen Briefings; das ist kein Mischen zweier Ladungen.
- Abgrenzung zu `health` (im Klartext „Umgebung:“): `health` bewertet und nennt Abhilfen,
  `environment` liefert die Rohwerte.

| Feld | Verwendung |
|---|---|
| `environment.available`, `environment.message` | Ob der Teil gebaut werden konnte. Wenn nicht — die Konfiguration ist nicht lesbar —, sagt `message` warum, und die übrigen Felder fehlen. |
| `environment.gh` | Der Befund zur GitHub CLI auf diesem Rechner: `ready` und `account`; Einzelheiten unten. |
| `environment.github` | Welcher Dienst der persönlichen GitHub-Zugangsdatei zum Repo des Arbeitsverzeichnisses gehört und welche Zugänge `k-playbook github run` dafür wählt: `file`, `present`, `problems`, `remote`, `repo`, `projectServices`, `services`. Einzelheiten unten. |
| `environment.baseTools` | Die Werkzeuge, die k-playbook selbst aufruft, und was davon fehlt: `present`, `error`, `missing` (je `name`, `role`, `methods`), `ok`, `installCommand`. |
| `environment.versionSources` | Zustand der Quellenkonfiguration des Versionsinventars `keph:/k-playbook/inventory.md`: ob sie da und lesbar ist, welche Zusatzquellen sie nennt, welche Bereiche sie ausschließt und welche Helm-Werte sie ausdrücklich nennt. Von hier lesen — nie das Dokument selbst. Einzelheiten unten. |
| `environment.docs` | Was die Docs-Erzeuger beim Code-Scan zusätzlich zu ihren festen Ausschlüssen auslassen, aus dem Block `docs` in `K-PLAYBOOK.yaml`: `exclude` (Muster wie in `.gitignore`, relativ zu `project.repoRoot`; leer ohne Block) und `problems` (verworfene Einträge, je `pattern` und `reason`). Von hier lesen — nie die Datei selbst; eingetragen wird über `k_playbook_docs_exclude_add`. |
| `environment.remediation` | Wie Befunde abgearbeitet werden: `mode`, `target`, `grouping`, `quickWins`, `branchPrefix`, `prRequired`, `directFixes`, `configured`. |

### `versionSources`

`environment.versionSources` trägt die ganze Antwort, kein Command muss das Dokument öffnen: `ref`
sagt, wo es liegt — immer `keph:/k-playbook/inventory.md`, auch wenn es fehlt —, `state`, in
welchem Zustand es ist, `present`, ob es da ist, `schemaVersion`, welche Vertragsfassung es
erklärt (gelesen wird nur `3`), `sources` die konfigurierten Zusatzquellen (`path`, `kind`,
`env`, `note`, `optional` je Eintrag), `exclude` die Muster, die der Standardlauf übergeht, und
`helmValues` die konfigurierten Helm-Werte (`path`, `key`, `item` je Eintrag). Jeder Pfad steht
relativ zur Projektwurzel; gelesen wird nur unterhalb von ihr.

`state` ist einer von vier Werten:

- `present` — das Dokument liegt und ist lesbar.
- `missing` — es fehlt; es gelten die Standardquellen. Kein Fehler.
- `invalid` — es liegt, ist aber nicht lesbar, von anderer Fassung als `3` oder trägt den
  Schlüssel `roots`. `error` nennt den Grund mit Zeile; `sources`, `exclude` und `helmValues`
  sind leer, und ein Lauf von `k-playbook inventory` bräche ab.
- `unavailable` — Kephalaion ist nicht erreichbar; über das Dokument ist nichts bekannt.
  `error` nennt den Grund. Es gibt keinen Rückfall auf die Platte und keinen auf die
  Standardquellen.

Der vollständige Vertrag steht in `k-playbook:/docs/handbook/version-inventory.md`.

### `health`

`health` bewertet die Umgebung und nennt Abhilfen; die Rohwerte dazu stehen unter `kephalaion`
im Briefing und unter `environment.baseTools` und `environment.versionSources` im Teilabruf.
Die Einträge heute: Die Anbindung an Kephalaion ist eingetragen,
aber nicht `ok` — `error`, denn `keph:`-Verweise und `work` sind dann nicht erreichbar; sie
ist nicht eingetragen — ebenfalls `error`. Steht im Block `kephalaion:` ein `account:` —
`warning`. Ohne Anbindung gibt es auch kein Briefing: Es
scheitert mit `kephalaion_unavailable`, weil die mitgelieferten Texte aus der Collection
kommen. Je fehlendem Basis-Werkzeug
(`environment.baseTools.missing`) — `warning`. Die Quellenkonfiguration des Versionsinventars
`keph:/k-playbook/inventory.md` ist da, aber nicht lesbar, von anderer Fassung oder trägt
`roots` (`state` `invalid`) — `error`; ebenso, wenn sie nicht erreichbar ist (`unavailable`),
außer Kephalaion selbst ist nicht bereit: Dann trägt den Fehler schon der Bereich
`kephalaion`. Die Einzelheiten stehen in `environment.versionSources.error`. Das Programm ist älter
als das neueste veröffentlichte Release (Bereich `program`) — `warning`, Abhilfe
`k-playbook upgrade`; das neueste Release liest die Prüfung aus einem Zwischenspeicher, der
einige Stunden alt sein kann, und ohne Netz schweigt sie.
Was eine Sitzung mit den Einträgen tut, steht in den immer geltenden Anweisungen: in der
ersten Antwort nennen, Fehler vor der Arbeit, nichts still beheben, die Abhilfe aus `hint`
anbieten. Nach einer Abhilfe liefert `k_playbook_health` den Stand neu, ohne das Briefing neu
zu laden. Weitere Quellen hängen sich hier ein; die Liste der Bereiche wächst dann.

### `gh`

`environment.gh` ist ein Befund über diesen Rechner: `gh.ready` und `gh.account`. Eine
Entscheidung des Projekts über gh gibt es nicht. `gh.ready` heißt beides: Die CLI ist da, und
ein Konto ist hinterlegt. Gelesen wird das aus der
Konfiguration von gh, nicht gegen den Server geprüft — ein Token kann also abgelaufen sein.

Abgrenzung zu `environment.github`: Ob gh im Spiel ist, entscheidet allein der Weg zu GitHub — mit
Dienst dessen gewählter Zugang, ohne Dienst `gh.ready`. `gh.ready` und `gh.account` gelten nur
ohne Dienst: `gh.account` ist nur das gerade aktive Konto von gh und zählt allein dort, wo kein
Dienst für das Ziel-Repo passt; passt einer, sagt dessen `selected`, welcher Zugang gilt.
Welcher Zweig gilt und was ein Command dann tut, entscheidet
`k-playbook:/commands/_shared/gh.md`.

### `github`

`environment.github` kommt aus derselben Quelle wie `k-playbook github show`: der persönlichen
Zugangsdatei `~/.config/k-playbook/github.yaml` (Ort nach `K_PLAYBOOK_GITHUB_CONFIG` bzw.
`XDG_CONFIG_HOME`). `file` ist ihr Pfad, `present`, ob sie dort liegt. Die Datei ist
freiwillig, und weder das Briefing noch der Teilabruf legen etwas an — auch nicht die
Umgebungsprüfung. Fehlt sie, ist `present` falsch, die Umgebung nennt nichts dazu, und die Zeile
sagt „GitHub: keine Zugangsdatei“. Angelegt wird sie nur ausdrücklich mit `k-playbook github
init` (oder dem Knopf „Anlegen“ der Oberfläche); eine Sitzung schreibt das YAML nicht selbst.
Ein belegter Pfad ohne lesbare Datei, etwa ein symbolischer Link ohne Ziel, gilt nicht als
fehlend: `present` ist wahr, `problems` trägt den `error`, und die Zeile sagt, dass die Datei
Fehler hat. `problems` sind die
Probleme der Datei in der Form von `k-playbook github check --json`; bei einem `error` sind
`services` `{}` und `projectServices` `[]`. `remote` ist der Eintrag des Remotes, das zählt —
das Upstream-Remote des aktuellen Branches, sonst `origin`, roh gelesen ohne `insteadOf` und
ohne Zugangsdaten —, `repo` daraus `<owner>/<repo>`; beide leer ohne Remote oder ohne eine Form,
die `run` umlenkt (https, `git@github.com:`, `ssh://git@github.com/`, ein SSH-Alias, den die
Datei nennt oder `ssh -G` nach github.com auflöst). `projectServices` sind die Dienste, deren
`repo` gleich `repo` ist (Groß- und Kleinschreibung zählen nicht), in Dateireihenfolge, meist
einer. `services` ist dasselbe Objekt wie bei `github show --json`, je Zugang mit `state` und
`reason` und je Dienst mit `selected`, der Wahl von `run`. Kein Netz, kein Token-Wert. Im
Klartext steht die Zeile `GitHub: dieses Repo = Dienst <name> — run: git über …, api über …`
bzw. „kein Dienst für <owner>/<repo>“, „kein GitHub-Remote erkannt“, „keine Zugangsdatei“ oder
„die Zugangsdatei hat Fehler“; das Briefing selbst trägt davon `github.projectServices`.
Die Zeile zeigt den Stand beim Laden des Briefings, der Teilabruf und `k-playbook github show`
den aktuellen. Was eine Sitzung mit der Zeile tut — GitHub über `k-playbook github run <dienst>`
erreichen —, steht in der immer geltenden Anweisung „GitHub über run“; die Einzelheiten in
`k-playbook:/instructions/on-demand/github.md`.

### Neu laden

Das Briefing gilt für die ganze Sitzung; der aufgelöste Stand ändert sich während der Arbeit
nicht. Neu geladen wird nur, wenn eines davon zutrifft:

- `K-PLAYBOOK.yaml` wurde seit dem Laden geschrieben.
- Ein Katalogeintrag — Review, Guideline oder Anweisung — ist seit dem Laden
  hinzugekommen, weggefallen oder geleert worden.
- k-playbook wurde aktualisiert.
- Die Arbeit ist in ein anderes Projekt gewechselt. Das Briefing sucht `K-PLAYBOOK.yaml` vom
  Arbeitsverzeichnis aufwärts, ein anderes Arbeitsverzeichnis kann also eine andere Installation
  treffen — `project.dir` mit dem geladenen vergleichen.
- `now` ist das eine Feld, das altert: Es ist der Moment des Aufrufs. Eine Sitzung, die
  über Mitternacht läuft, trägt noch das Datum von gestern. Neu laden, bevor ein falsches Datum
  in eine Datei käme.

Trifft eines zu, wird das Briefing neu geholt, und ab dann gilt nur die neue Ausgabe. Felder aus
zwei Ladungen werden nie gemischt.

Kein Neuladen ist der Teilabruf `environment`: Er vergibt keine Sitzungskennung, wird bei Bedarf
frisch geholt — auch mehrmals je Sitzung —, und seine Felder dürfen neben denen des geladenen
Briefings stehen. Ebenso wenig `k_playbook_health`, das nur die Umgebungsprüfung neu liefert.
