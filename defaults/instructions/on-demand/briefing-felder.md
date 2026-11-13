---
title: Die Felder des Briefings
description: Jedes Feld des Briefings und wozu es dient — die eigenen Abschnitte und der Übergangsabschnitt context —, dazu die vollständige Liste der Bedingungen, unter denen eine Sitzung das Briefing neu lädt.
---

## Die Felder des Briefings

Das Briefing ist die Antwort von `k_playbook_briefing` (ersatzweise `k-playbook briefing
--json`). Was jede Sitzung daraus **tun** muss, steht in den immer geltenden Anweisungen; hier
steht, was jedes Feld bedeutet. Der Vertrag für Entwickler ist `docs/mcp.md`, „Briefing
Contract“ — dieses Blatt hält mit ihm Schritt, ein Test prüft die Vollständigkeit.

### Die eigenen Abschnitte

| Feld | Bedeutung |
|---|---|
| `schemaVersion` | Die Fassung der Briefing-Ausgabe, heute `"6"`. |
| `session` | Die Kennung dieser Sitzung, mit jedem Briefing neu vergeben, etwa `20261024-183012-3fa9c1`. Sie steht in allem, was die Sitzung später wiederfinden muss — im Kopf eines Befunds unter `sitzung`, nach dem `/k-danke` sucht. Lädt eine Sitzung das Briefing neu, gilt die neue Kennung, und die frühere gehört weiter zu ihr. |
| `documents.always` | Das zusammengebaute Verzeichnis `k-playbook:/instructions/always/`: `ref`, `content` (der Text selbst, Includes eingesetzt — eine Sitzung muss ihn nicht eigens lesen), `parts` in der Reihenfolge des Texts, `problems` (übergangene Teile). |
| `documents.onDemand`, `documents.reviews`, `documents.guidelines` | Die Einträge der Sorte, je ein Dokument für sich: `ref`, `title`, `description`, `origin` (`dist`, `local`, `override` oder `disabled` für einen abgeschalteten Eintrag). Bei Anweisungen und Reviews sind mitgeliefert und projekteigen schon gegeneinander aufgelöst; Guidelines kommen nur mitgeliefert, ihre Herkunft ist immer `dist`. Gelesen wird über `ref` mit `k_playbook_doc_read`. |
| `work` | Die eigene offene Arbeit aus der Projekt-Collection: `available` und `message` (ob die Collection lesbar war und sonst warum nicht), `user` (der eigene Kephalaion-User), `tasks` und `workbench` (je `ref`, `title`, `updated` und `problem`, wenn Kephalaion das Frontmatter nicht lesen konnte oder der Titel vermutlich an „ #“ abgeschnitten ist — gemeldet, nicht repariert; offen heißt: nicht in `done/`), `openFindings` (Zahl der offenen Befunde in `queue/findings/`). |
| `projectDocs` | Der Einstieg der Projektdokumentation, als Verweis. |
| `health` | Die Umgebungsprüfung: `ok` (kein Eintrag der Stufe `error`; Warnungen lassen es wahr) und `problems`, je Eintrag `level` (`error` oder `warning`), `area` (`kephalaion`, `baseTools`, `versionSources`, …), `message` (ein Satz, nie ein Token oder Wert) und `hint` (die Abhilfe), sortiert nach Bereich, dann Stufe. Ohne Problem ist `problems` leer. Dieselbe Antwort wie `k_playbook_health` und `k-playbook health --json`; Einzelheiten unten. |
| `context` | Der Rest von `k-playbook context`: jedes Feld der Kontextausgabe unter seinem Namen, das das Briefing nicht in eigenen Abschnitten liefert — `catalogs.reviews` fehlt darum. Ein Übergang: Jedes Feld bekommt noch einen eigenen Platz oder entfällt. |

### Der Abschnitt `context`

| Feld | Verwendung |
|---|---|
| `context.available`, `context.message` | Ob die Kontextausgabe gebaut werden konnte. Wenn nicht, sagt `message` warum, und die übrigen Felder fehlen; das Briefing davor gilt trotzdem. |
| `context.schemaVersion` | Die Fassung der Konfiguration `K-PLAYBOOK.yaml`. Liest kein Command. |
| `context.now.date`, `context.now.timestamp` | Der Moment des Aufrufs in der Zeitzone des Rechners. `date` ist `YYYY-MM-DD`, `timestamp` RFC 3339. |
| `context.project.dir` | Das Hauptverzeichnis — wo `K-PLAYBOOK.yaml` liegt. |
| `context.project.repoRoot`, `context.project.vcs` | Das Code-Repository für Git-Operationen, und ob es unter Versionskontrolle steht. |
| `context.project.config` | Der Pfad der Konfiguration `K-PLAYBOOK.yaml`. Nur zur Auskunft; gelesen wird sie nie selbst. |
| `context.project.languages` | Die Sprachen, für die das Projekt Werkzeuge braucht. Immer gefüllt — ein fehlender Eintrag fällt auf den Standard zurück, es gibt nichts zu deuten. |
| `context.playbook.dir` | Die Installation. Nur lesen; sie wird bei jedem Update ersetzt. |
| `context.local.dir` | Alles, was das Projekt besitzt. |
| `context.remediation` | Wie Befunde abgearbeitet werden: `mode`, `target`, `grouping`, `quickWins`, `branchPrefix`, `prRequired`, `directFixes`, `configured`. |
| `context.docs` | Was die Docs-Erzeuger beim Code-Scan zusätzlich zu ihren festen Ausschlüssen auslassen, aus dem Block `docs` in `K-PLAYBOOK.yaml`: `exclude` (Muster wie in `.gitignore`, relativ zu `project.repoRoot`; leer ohne Block) und `problems` (verworfene Einträge, je `pattern` und `reason`). Von hier lesen — nie die Datei selbst; eingetragen wird über `k_playbook_docs_exclude_add`. |
| `context.gh` | Der Befund zur GitHub CLI auf diesem Rechner: installiert, angemeldet, aktives Konto, bereit; Einzelheiten unten. |
| `context.mcp` | Die Pflichtliste der MCP-Server aus der Konfiguration (`required`, `configured`). Liest kein Command; Sache der Oberfläche. |
| `context.git` | Ob die Oberfläche das Umschalten des Branches anbietet (`switch`, `allow`, `environments`, `configured`). Liest kein Command. |
| `context.baseTools` | Die Werkzeuge, die k-playbook selbst aufruft, und was davon fehlt: `matrix`, `present`, `missing` (je `name`, `role`, `methods`), `ok`, `installCommand`. |
| `context.catalogs.checks` | Die Checks — nur die mitgelieferten, `origin` immer `dist`; projekteigene Checks gibt es nicht. Reviews stehen nicht hier, sondern unter `documents.reviews`. |
| `context.versionSources` | Zustand der Quellenkonfiguration des Versionsinventars `keph:/k-playbook/inventory.md`: ob sie da und lesbar ist, welche Zusatzquellen sie nennt, welche Bereiche sie ausschließt und welche Helm-Werte sie ausdrücklich nennt. Von hier lesen — nie das Dokument selbst. Einzelheiten unten. |
| `context.kephalaion` | Die Anbindung des Projekts an seine Collection: `address`, `ok`, `user`, `message` u. a.; Hub und Collection aus dem Block `kephalaion:` von `.k-playbook/K-PLAYBOOK.yaml`, `account` und seine Herkunft `choice` (`override`, `default`, `only`) aus der Auskunft von Kephalaion. `keph:`-Verweise sind nur erreichbar, wenn `ok` wahr ist. |
| `context.links` | Die Selbstheilung der Assistenten-Verknüpfungen: `healed`, `open`, `note`. Nur da, wenn es etwas zu melden gab; `note` sagt, was das für diese Sitzung heißt. |
| `context.github` | Welcher Dienst der persönlichen GitHub-Zugangsdatei zum Repo des Arbeitsverzeichnisses gehört und welche Zugänge `k-playbook github run` dafür wählt: `file`, `present`, `problems`, `remote`, `repo`, `projectServices`, `services`. Im Klartext eine eigene Zeile `GitHub:` nach `Projektdoku:`. Einzelheiten unten. |

### `versionSources`

`context.versionSources` trägt die ganze Antwort, kein Command muss das Dokument öffnen: `ref`
sagt, wo es liegt — immer `keph:/k-playbook/inventory.md`, auch wenn es fehlt —, `state`, in
welchem Zustand es ist, `present`, ob es da ist, `schemaVersion`, welche Vertragsfassung es
erklärt (gelesen wird nur `3`), `sources` die konfigurierten Zusatzquellen (`path`, `kind`,
`env`, `note`, `optional` je Eintrag), `exclude` die Muster, die der Standardlauf übergeht, und
`helmValues` die konfigurierten Helm-Werte (`path`, `key`, `item` je Eintrag). Jeder Pfad steht
relativ zur Projektwurzel; gelesen wird nur unterhalb von ihr. Die Installation `<playbook.dir>`
durchsucht der Standardlauf nicht, und sie steht nicht in `exclude` — diese Regel kommt aus dem
Vertrag, nicht aus dem Dokument.

`state` ist einer von vier Werten:

- `present` — das Dokument liegt und ist lesbar.
- `missing` — es fehlt; es gelten die Standardquellen. Kein Fehler.
- `invalid` — es liegt, ist aber nicht lesbar, von anderer Fassung als `3` oder trägt den
  Schlüssel `roots`. `error` nennt den Grund mit Zeile; `sources`, `exclude` und `helmValues`
  sind leer, und ein Lauf von `k-playbook inventory` bräche ab.
- `unavailable` — Kephalaion ist nicht erreichbar; über das Dokument ist nichts bekannt.
  `error` nennt den Grund. Es gibt keinen Rückfall auf die Platte und keinen auf die
  Standardquellen.

Die alte Datei `<local.dir>/version-sources.yaml` liest kein Programmteil mehr. Liegt sie noch,
meldet die Umgebungsprüfung sie als Altbestand; `/k-upgrade` zieht ihren Inhalt um. Auf einer
Installation, die älter ist als das Feld, fehlt es. Der vollständige Vertrag steht in
`<playbook.dir>/docs/version-inventory.md`.

### `health`

`health` bewertet, was `context` an Umgebungsangaben trägt, und ersetzt nichts davon: Die
Felder `context.kephalaion`, `context.baseTools` und `context.versionSources`
stehen weiter unverändert da. Die Einträge heute: Die Anbindung an Kephalaion ist eingetragen,
aber nicht `ok` — `error`, denn `keph:`-Verweise und `work` sind dann nicht erreichbar; sie
ist nicht eingetragen — ebenfalls `error`. Steht im Block `kephalaion:` ein `account:` oder
liegt noch die alte `k-playbook-local/K-PLAYBOOK.local.yaml` mit Block `kephalaion:` —
`warning`. Ohne Anbindung gibt es auch kein Briefing: Es
scheitert mit `kephalaion_unavailable`, weil die mitgelieferten Texte aus der Collection
kommen. Je fehlendem Basis-Werkzeug
(`context.baseTools.missing`) — `warning`. Die Quellenkonfiguration des Versionsinventars
`keph:/k-playbook/inventory.md` ist da, aber nicht lesbar, von anderer Fassung oder trägt
`roots` (`state` `invalid`) — `error`; ebenso, wenn sie nicht erreichbar ist (`unavailable`),
außer Kephalaion selbst ist nicht bereit: Dann trägt den Fehler schon der Bereich
`kephalaion`. Die Einzelheiten stehen in `context.versionSources.error`. Liegt noch die alte
`<local.dir>/version-sources.yaml` — `warning`, Abhilfe `/k-upgrade`.
Was eine Sitzung mit den Einträgen tut, steht in den immer geltenden Anweisungen: in der
ersten Antwort nennen, Fehler vor der Arbeit, nichts still beheben, die Abhilfe aus `hint`
anbieten. Nach einer Abhilfe liefert `k_playbook_health` den Stand neu, ohne das Briefing neu
zu laden. Weitere Quellen hängen sich hier ein; die Liste der Bereiche wächst dann.

### `gh`

`context.gh` ist ein Befund über diesen Rechner: `gh.installed`, `gh.loggedIn`, `gh.account`,
`gh.accounts` und `gh.ready`. Eine Entscheidung des Projekts über gh gibt es nicht. `gh.ready`
heißt beides: Die CLI ist da, und ein Konto ist hinterlegt. Gelesen wird das aus der
Konfiguration von gh, nicht gegen den Server geprüft — ein Token kann also abgelaufen sein.

Abgrenzung zu `context.github`: Ob gh im Spiel ist, entscheidet allein der Weg zu GitHub — mit
Dienst dessen gewählter Zugang, ohne Dienst `gh.ready`. `gh.ready` und `gh.account` gelten nur
ohne Dienst: `gh.account` ist nur das gerade aktive Konto von gh und zählt allein dort, wo kein
Dienst für das Ziel-Repo passt; passt einer, sagt dessen `selected`, welcher Zugang gilt.
Welcher Zweig gilt und was ein Command dann tut, entscheidet
`k-playbook:/commands/_shared/gh.md`.

### `github`

`context.github` kommt aus derselben Quelle wie `k-playbook github show`: der persönlichen
Zugangsdatei `~/.config/k-playbook/github.yaml` (Ort nach `K_PLAYBOOK_GITHUB_CONFIG` bzw.
`XDG_CONFIG_HOME`). `file` ist ihr Pfad, `present`, ob sie dort liegt. Die Datei ist
freiwillig, und das Briefing legt nichts an — weder das Feld noch die Umgebungsprüfung, die
vorher läuft. Fehlt sie, ist `present` falsch, die Umgebung nennt nichts dazu, und die Zeile
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
„die Zugangsdatei hat Fehler“; den Rest hinter der Trennzeile zeigt `github` ohne
`services`. Das Feld zeigt den Stand beim Laden des Briefings, `k-playbook github show` den
aktuellen. Was eine Sitzung mit der Zeile tut — GitHub über `k-playbook github run <dienst>`
erreichen —, steht in der immer geltenden Anweisung „GitHub über run“; die Einzelheiten in
`k-playbook:/instructions/on-demand/github.md`.

### Neu laden

Das Briefing gilt für die ganze Sitzung; der aufgelöste Stand ändert sich während der Arbeit
nicht. Neu geladen wird nur, wenn eines davon zutrifft:

- `K-PLAYBOOK.yaml` wurde seit dem Laden geschrieben.
- Ein Katalogeintrag — Regel, Review, Check, Guideline oder Anweisung — ist seit dem Laden
  hinzugekommen, weggefallen oder geleert worden.
- k-playbook wurde aktualisiert.
- Die Arbeit ist in ein anderes Projekt gewechselt. Das Briefing sucht `K-PLAYBOOK.yaml` vom
  Arbeitsverzeichnis aufwärts, ein anderes Arbeitsverzeichnis kann also eine andere Installation
  treffen — `context.project.dir` mit dem geladenen vergleichen.
- `context.now` ist das eine Feld, das altert: Es ist der Moment des Aufrufs. Eine Sitzung, die
  über Mitternacht läuft, trägt noch das Datum von gestern. Neu laden, bevor ein falsches Datum
  in eine Datei käme.

Trifft eines zu, wird das Briefing neu geholt, und ab dann gilt nur die neue Ausgabe. Felder aus
zwei Ladungen werden nie gemischt.
