# k-befunde-einordnen

Ordnet die Befunde in `keph:/queue/findings/` ein, einen nach dem anderen, ältester zuerst:
Jeder Befund wird geprüft, an seinen Ort gebracht — Handdoku, Todo, Nachtrag in einem Task
oder Workbench-Thema, Doku-Auftrag, Datei im Projekt — oder gelöscht. Kein Befund bleibt
still liegen, und keiner geht ohne Prüfung verloren. Angefangen wird nur, wenn die
Vorprüfung im Programm alle Befunde aller Nutzer als lesbar meldet.

Das Argument ist die Obergrenze dieses Laufs: höchstens so viele Befunde, Standard 25. Der
Rest bleibt in der Queue für den nächsten Lauf; die Queue selbst ist der Wiederaufsetzpunkt.

Produces:

- `keph:/queue/einordnung/<NNN>-<datum>-<sitzung>.md` — das Protokoll dieses Laufs, eine
  Zeile je behandeltem oder übersprungenem Befund.
- Je Befund sein Ziel: Handdoku unter `keph:/docs/manual/`, ein Todo, ein Nachtrag in einem
  eigenen Task oder Workbench-Thema, ein Doku-Auftrag in `keph:/queue/docs/` samt Todo, eine
  Änderung an einer Datei im Projekt (nur im Arbeitsbaum) samt Todo, oder
  `keph:/queue/unclear/<dateiname>`.
- Gelöschte Befunde in `keph:/queue/findings/`.

Nichts sonst. Kein Commit, kein Push, nichts in einem fremden Repo.

## Schritt 1 — Briefing, Obergrenze, Projektregeln

include: _shared/erster-schritt.md

Aus dem Briefing brauchst du `context.kephalaion` (`ok`, `address`, `message`, `user`),
`context.project.dir`, `context.now` (`date`, `timestamp`), die Sitzung (`session`, im
Klartext die Zeile `Sitzung:`) und die GitHub-Zeile (`context.github.projectServices`).
Steht `context.kephalaion.ok` nicht auf `true`, endet der Command hier mit
`context.kephalaion.message`: Ohne Kephalaion gibt es keine Befunde und kein Protokoll.

**Obergrenze.** Ist `$ARGUMENTS` eine ganze Zahl ab 1, ist sie die Obergrenze `LIMIT`; leer
heißt 25. Alles andere: melden und enden. Gezählt wird jeder Befund, den Schritt 4 ganz
liest, auch wenn er danach übersprungen wird; die Befunde, die schon die Vorprüfung
übersprungen hat, zählen nicht.

**Projektregeln.** Lies `k-playbook:/instructions/on-demand/befunde-einordnen.md` mit
`k_playbook_doc_read`. k-playbook liefert unter diesem Namen selbst nichts; ein Projekt legt
die Datei lokal an (in `keph:/k-playbook/instructions/on-demand/`), um diesen Command zu
erweitern, und sagt in ihrer `description`, dass sie nur für `/k-befunde-einordnen` gilt.

- `not_found` heißt hier: Das Projekt hat keine Projektregeln. **Das ist kein Fehler.** Es
  ist die eine, ausdrückliche Ausnahme von der Regel „Ein Verweis, den k-playbook nicht
  kennt, ist ein Fehler“ aus `k-playbook:/instructions/always/010-verweise.md`, nur für
  diesen optionalen Erweiterungspunkt. Melde nichts und such die Datei nicht auf der Platte.
- Gibt es sie, ergänzt sie diesen Text und hat bei Widerspruch Vorrang — **außer** bei:
  der Vorprüfung (Schritt 2), dem Löschen und Verschieben nur mit `base_revision`, der
  Reihenfolge „erst schreiben und prüfen, dann löschen“, dem Löschen nur mit vollem Namen,
  nie per Muster oder Verzeichnis, und dem Protokoll. Diese fünf gelten immer.
- Typisch stehen dort: ein anderer Hauptbranch, weitere Stellen, gegen die geprüft wird,
  Regeln, die nur dieses Projekt kennt.

Lies dazu `k-playbook:/instructions/on-demand/befunde.md` (Aufbau eines Befunds, Feld
`ziel`). `k-playbook:/instructions/on-demand/tasks.md` und
`k-playbook:/instructions/on-demand/doku-auftraege.md` liest du, sobald ein Befund in einen
Task bzw. einen Doku-Auftrag geht.

## Schritt 2 — Vorprüfung

Rufe `k_playbook_findings_check` mit `projectDir` = `context.project.dir`, ersatzweise
`k-playbook findings check --json` in der Shell. Das Programm gleicht die Replica ab, prüft
die Rechte `read`, `write` und `supersede`, jeden Eintrag unter `queue/findings/` und liest
jeden Befund. Ist das Werkzeug unbekannt oder meldet die Shell `unbekanntes Kommando:
findings`, ist die Installation älter als dieser Command: melden, `/k-upgrade` nennen, enden.

Steht `check.ready` nicht auf `true`: melde jeden Eintrag aus `check.problems` (Art, Name,
Satz samt Abhilfe), dazu `check.total` und die Zahl je Nutzer aus `check.byUser`, und ende.
Es wird nichts geschrieben, kein Befund angefasst, kein Protokoll angelegt.

Sonst merkst du dir:

- `check.findings` — die Befunde in der Reihenfolge, in der Schritt 4 sie nimmt (ältester
  zuerst), je mit `name` (dem vollen Namen in der Collection), `revision`, `user`, `ziel` und
  `thema`;
- `check.skipped` — die Befunde, die dieser Lauf überspringt, je mit `reason`;
- `check.total` und `check.byUser`.

## Schritt 3 — Hauptbranch, Bestand, Protokoll

**Hauptbranch.** Nennen die Projektregeln einen, gilt er. Sonst nennt ihn
`git symbolic-ref --short refs/remotes/origin/HEAD` im Projekt (`origin/<name>` →
`<name>`). Fehlt `origin/HEAD`, fragst du das Remote: `k-playbook github run <dienst> -- git
ls-remote --symref origin HEAD` (die Zeile `ref: refs/heads/<name>	HEAD`). Den Dienst nennt
die GitHub-Zeile des Briefings; lies vor dem ersten Aufruf
`k-playbook:/instructions/on-demand/github.md`. Bleibt der Hauptbranch unbekannt: melden und
enden, nicht raten.

**Holen.** `k-playbook github run <dienst> -- git fetch origin <hauptbranch>`. Scheitert der
Fetch: melden und enden. Ohne frischen Stand würden Befunde fälschlich als `erledigt`
gelöscht. Danach bestimmst du den Stand `STAND`, gegen den der ganze Lauf prüft:

- Deckt `remote.origin.fetch` (`git config --get-all remote.origin.fetch`) den Hauptbranch
  ab, ist `STAND` der Commit von `origin/<hauptbranch>` (`git rev-parse`).
- Sonst — ein Checkout mit eingeschränkter Refspec, dessen `origin/<hauptbranch>` alt sein
  kann — ist `STAND` der Commit von `FETCH_HEAD`, gleich nach dem Fetch mit
  `git rev-parse FETCH_HEAD` festgehalten.

Geprüft wird nur gegen `STAND` — `git show <STAND>:<pfad>`, `git grep <muster> <STAND> --
<pfad>`, `git log <STAND> -- <pfad>` —, nie gegen den lokalen Checkout.

**Bestand melden.** Eine kurze Meldung: Befunde gesamt und je Nutzer, wie viele die
Vorprüfung übersprungen hat, die Obergrenze `LIMIT` dieses Laufs, Hauptbranch und `STAND`.

**Listen einmal holen.** Die offenen Tasks (`k_playbook_doc_list` mit `kind: tasks`,
`user: *`, `compact: true`), die Todos (`k_playbook_todo_list`) und die Workbench-Themen
(`k_playbook_doc_list` mit `kind: workbench`, `user: *`, `compact: true`) holst du einmal
hier, nicht je Befund; ist eine Antwort `truncated`, holst du mit `next.after` als `after`
die nächste Seite, bis alle da sind. Was du im Lauf selbst schreibst — ein neues oder ergänztes Todo, ein Nachtrag —,
trägst du in deine Fassung der Listen nach. Neu geholt wird nur, wenn ein Schreiben
`foreign_change` meldet: Dann hat jemand anderes daran geschrieben.

**Protokoll anlegen.** `k_playbook_doc_create` mit `kind: einordnung` und `name` =
`<context.now.date>-<sitzung>`. Den Inhalt schreibst du gleich danach mit Kephalaion `write`
(`collection` = `context.kephalaion.address`, `name` = der Pfad aus `ref` ohne `keph:/`):

```markdown
---
title: "Einordnung <datum>, Sitzung <sitzung>"
sitzung: <sitzung>
stand: "<hauptbranch> <STAND>"
---

# Einordnung <datum>

Obergrenze <LIMIT>. Vor dem Lauf: <total> Befunde (<user> <zahl>, …).
```

Die Revision aus der Antwort ist dein erstes `since_revision` für das Protokoll. Jede Zeile
hängst du danach mit `k_playbook_doc_append` und `since_revision` an, die Revision aus jeder
Antwort ist die nächste. `foreign_change` am eigenen Protokoll heißt: Ein zweiter Lauf
schreibt hinein. Halte an, fasse keinen weiteren Befund an und melde es im Abschluss.

Jede Protokollzeile ist ein Listenpunkt: `- <voller Name> | <Ziel oder Grund>`.
`k_playbook_doc_append` setzt vor jeden Eintrag eine Leerzeile; als Liste sieht das Protokoll
gerendert trotzdem gleich aus, ob mehrere Zeilen in einem Anhängen kamen oder einzeln.

Die Befunde aus `check.skipped` bekommen jetzt ihre Zeilen, in einem Anhängen:
`- <voller Name> | übersprungen (<reason>)`, je Befund eine.

**Ausnahme vom Verweisverbot.** Das Protokoll nennt jeden Befund mit vollem Namen. Das ist
eine bewusste, vorübergehende Ausnahme von der Regel, dass auf nichts unter `keph:/queue/`
verwiesen wird: Das Protokoll liegt selbst in der Queue und entfällt mit ihr. Auf das
Protokoll verweist nur deine Antwort an den Nutzer am Ende des Laufs — nie ein Task, ein
Workbench-Thema, ein Todo oder eine Doku.

## Schritt 4 — Je Befund

Nimm die Befunde aus `check.findings` einzeln, bis `LIMIT` erreicht ist: den ältesten noch
nicht behandelten, und gleich nach ihm die übrigen mit demselben `thema`, auch wenn ältere
dazwischen liegen. Sie fragen meist nach denselben Code- und Doku-Stellen, die du dann noch
vor dir hast. Jeder Befund zählt einzeln gegen `LIMIT`; erreicht sie mitten in einem Thema,
bleibt der Rest für den nächsten Lauf. Ohne `thema` steht ein Befund für sich. Gedacht ist ein Lauf je Projekt zur Zeit.

### 4.1 Lesen

Lies den Befund ganz mit Kephalaion `read` (`collection` = `context.kephalaion.address`,
`name` = sein voller Name) und merke dir die Revision aus dieser Antwort — sie ist die
`base_revision` für Löschen und Verschieben.

- Leer, oder in den letzten sechs Stunden geändert (`updated` gegen
  `context.now.timestamp`): Zeile `- <voller Name> | übersprungen (leer)` bzw.
  `übersprungen (jünger als 6 h)`, weiter mit dem nächsten. `k_playbook_doc_create` legt
  einen Befund zuerst leer an, der Inhalt kommt im zweiten Aufruf, und eine Sitzung kann ihn
  noch nachtragen; die Wartezeit verhindert, dass gelesen wird, woran noch geschrieben wird.
- `kind: none`: Ein anderer Lauf hat ihn schon behandelt — `übersprungen (anderer Lauf)`.
- Ein Fehler: `übersprungen (Fehler: <code>)`.

### 4.2 Prüfen

- **Relevant?** Prüfe die Aussage gegen den Code auf `STAND`, gegen die Doku
  (`keph:/docs/`: Kephalaion `search` mit englischen Wörtern, dann `read` des Abschnitts),
  gegen die offenen Tasks, Todos und Workbench-Themen aus den Listen von Schritt 3, dazu gegen das, was die Projektregeln nennen. Einen Befund
  mit `sicherheit: unbestaetigt` bestätigst oder widerlegst du, wenn das mit vertretbarem
  Aufwand geht; was dabei herauskommt, geht ins Ziel.
- **Nachhaltig?** Zwei getrennte Fragen: Ist der Anlass erledigt? Ist das Wissen noch
  wertvoll? Gelöscht wird nur, wenn das Wissen nicht (mehr) wertvoll ist.
  - Nicht nachhaltig: Momentaufnahmen („PR X hatte im Review Problem Y“), PR- und
    Ticketstände, Validierungsprotokolle, Untersuchungsschritte zu behobenen Fehlern, alles,
    was Code, Konfiguration oder erzeugte Doku ohnehin sagen, Review-Befunde zu
    zurückgenommenen Features.
  - Nachhaltig: bleibende Eigenschaften des Systems, Abläufe (Deploy, Rollback,
    Validierung), wiederkehrende Bewertungen (Sicherheitsmeldungen), Fallen, ausgeschlossene
    Wege.
- **`abgedeckt`** gilt nur mit einer Fundstelle, deren Inhalt du gelesen hast — nicht nach
  einem Stichworttreffer.

### 4.3 Ziel wählen

Das Feld `ziel` des Befunds (`wissen`, `falle`, `regel`, `verwerfen`) ist ein Hinweis, keine
Entscheidung. Es gilt die erste passende Zeile:

| Lage | Ziel |
|---|---|
| Anlass erledigt und Wissen nicht nachhaltig, oder schon abgedeckt | löschen: `erledigt`, `abgedeckt (<wo>)` oder `nicht nachhaltig` |
| Wissen betrifft ein laufendes Vorhaben (offener Task, Workbench-Thema) | dort als Nachtrag (Task nach `k-playbook:/instructions/on-demand/tasks.md`) |
| Nachhaltiges Projektwissen | Handdoku `keph:/docs/manual/` ergänzen, sonst neues Dokument (Schritt 5) |
| Wissen gehört an eine Datei im Projekt (README eines Werkzeugs, Skriptkopf) | dort ergänzen, im Arbeitsbaum, kein Commit; dazu ein Todo „Änderung an `<datei>` committen“, das den Inhalt in eigenen Worten trägt, damit er einen verworfenen Arbeitsbaum überlebt |
| Es ist etwas zu tun | Todo (zusätzlich zur Doku oder allein) |
| Änderung an Instructions, Skills, Commands, `AGENTS.md` | nur nach Rückfrage: im Lauf nichts schreiben, Zeile `übersprungen (Rückfrage)`, der Befund bleibt liegen, die Frage geht in den Abschluss |
| Änderung an einem fremden Repo | nur als Todo, nichts dort schreiben |
| Befund betrifft ein Werkzeug statt das Projekt (k-playbook, Kephalaion) | sollte nicht vorkommen; sonst `unclear/` |
| Erzeugte Doku (`docs/generated/`) ist falsch oder unvollständig | nicht von Hand: Doku-Auftrag nach `k-playbook:/instructions/on-demand/doku-auftraege.md` und dazu ein Todo, das den Fehler in eigenen Worten trägt — ein Auftrag ist nur ein Merkzettel und allein kein Löschgrund |
| Ziel oder Bewertung unklar, oder nur der Verfasser kann es beurteilen | `unclear/` |

- **Vor einem Todo** siehst du die Todos, offenen Tasks und Workbench-Themen aus den Listen
  von Schritt 3 durch, bei Bedarf dazu Kephalaion `search`. Gibt es eins zum Thema, ergänzt du
  dieses (`k_playbook_todo_update`); deckt ein offener Task es ab, entfällt das Todo. Todos
  legst du ohne Rückfrage an.
- **Nachträge** gehen nur in eigene Tasks und Workbench-Themen (`k_playbook_doc_append`
  lehnt fremde mit `foreign_user` ab). Gehört das Wissen zu einem fremden Vorhaben, wird es
  ein Todo, das den Inhalt trägt und das Vorhaben nennt.
- **Ziele nennen den Befund nicht.** Kein Ziel — Todo, Nachtrag, Doku, Datei — nennt Namen
  oder Nummer des Befunds; sein Inhalt geht in eigenen Worten hinein. `origin` eines Todos
  nennt eine bleibende Quelle (etwa einen Task) oder bleibt leer, nie den Befund.

### 4.4 Schreiben, prüfen, löschen, protokollieren

Immer in dieser Reihenfolge:

- **(a) Ziel schreiben.**
- **(b) Gegenlesen:** das Geschriebene neu lesen und prüfen, dass es den Befund trägt —
  Aussage, Beleg, Bedingung, Sackgassen. Liefert das Lesen direkt nach dem eigenen
  Schreiben eine ältere Revision, läuft die Replica nach; dann gilt der Replica-Nachlauf aus
  `k-playbook:/instructions/on-demand/tasks.md`.
- **(c) Löschen bzw. verschieben,** mit `base_revision` aus 4.1 und einem eigenen
  `request_id` je Aufruf (etwa eine UUID aus `cat /proc/sys/kernel/random/uuid`):
  - Löschen: Kephalaion `delete` mit `name` = dem vollen Namen des Befunds. Nur dieser eine
    Name — nie ein Muster, nie ein Verzeichnis, nie `recursive`. Nummern kommen doppelt vor.
  - Nach `unclear/`: Kephalaion `rename` nach `queue/unclear/<dateiname>`, danach `write` mit
    der Revision aus der Antwort von `rename` als `base_revision`: Der neue Inhalt trägt im
    Frontmatter das Feld `frage` mit der offenen Frage in doppelten Anführungszeichen (ein
    `"` im Wert wird `\"`); hat der Befund kein Frontmatter, bekommt er eines nur mit
    `frage`. Der Rumpf bleibt Zeichen für Zeichen.
- **(d) Erst dann die Protokollzeile** mit dem tatsächlichen Ausgang:
  `- <voller Name> | <Ziel oder Löschgrund>`, etwa `| Handdoku keph:/docs/manual/<datei>`,
  `| Todo #<id>`, `| Nachtrag <task>`, `| Doku-Auftrag + Todo #<id>`, `| Datei <pfad> +
  Todo #<id>`, `| unclear`, `| erledigt`, `| abgedeckt (<wo>)`, `| nicht nachhaltig`.

Bei einem Löschgrund ohne neues Ziel entfallen (a) und (b); `abgedeckt` braucht trotzdem die
gelesene Fundstelle. Ein gelöschter Befund ist in der Replica nur noch eine Löschmarke ohne
Inhalt — deshalb wird nie gelöscht, bevor das Ziel steht und gegengelesen ist.

Was in (c) zurückkommt:

- **`stale_revision`:** Der Befund hat sich seit 4.1 geändert. Lies ihn neu und vergleiche
  mit dem Geschriebenen. Deckt das Ziel den neuen Stand, löschst bzw. verschiebst du mit der
  neuen Revision. Sonst ziehst du erst das Ziel nach und machst ab (b) weiter — oder der
  Befund bekommt `übersprungen (geändert)` und bleibt liegen. Nie blind löschen.
- **`not_found`:** Ein anderer Lauf hat den Befund schon behandelt. Sieh nach, ob sein
  Wissen jetzt doppelt dasteht, und nimm dein eben geschriebenes Ziel dann zurück; Zeile
  `übersprungen (anderer Lauf)`.
- **`outcome_unknown`:** Schick denselben Aufruf mit demselben `request_id` noch einmal.
  `replayed` heißt: Er war schon angekommen. Bleibt der Ausgang unklar, ist die Zeile
  `übersprungen (Ausgang unklar)`; nicht ein drittes Mal.
- **Ein anderer Fehler:** `übersprungen (Fehler: <code>)`; das geschriebene Ziel bleibt
  stehen und wird im Abschluss genannt.

Die Protokollzeile folgt nur auf eine eindeutige Antwort.

## Schritt 5 — Doku-Einträge

Für jedes Ziel unter `keph:/docs/manual/`:

- Doku unter `keph:/docs/` ist englisch.
- Der Kopf eines Handdoku-Dokuments steht in `k-playbook:/commands/_shared/docs.md`,
  Abschnitt „Der Kopf“: Lies ihn mit `k_playbook_doc_read` und `section: "### Der Kopf"`
  (Handdoku: `title`, `description`, `questions`, `sources`; kein `generated`, kein
  `verified`). Ebenso steht dort, was nicht in die Doku gehört — Ticket-, Testlauf- und
  Reviewprotokolle, der Stand laufender Vorhaben.
- Bestehende Handdoku änderst du mit `k_playbook_doc_replace` bzw. `k_playbook_doc_append`,
  neue legst du mit Kephalaion `create` an.
- Am heutigen Stand belegt: Pfad und Zeile auf `STAND`, Prüfdatum (`context.now.date`),
  Befehl zum Nachprüfen.
- Bewertungen tragen eine Bedingung „gilt nicht mehr, wenn …“ und den Befehl, sie zu prüfen.
- Beim Einarbeiten prüfst du die Nachbaraussagen im selben Abschnitt mit; Falsches
  korrigierst du.
- Ein Doku-Eintrag trägt die Aussage des Befunds. Was du beim Prüfen am Code zusätzlich
  findest, nimmst du nur auf, wenn es dieselbe Frage beantwortet und auf `STAND` belegt ist.
  Alles andere legst du als neuen Befund an (`k_playbook_doc_create`, Sorte `findings`,
  Aufbau nach `k-playbook:/instructions/on-demand/befunde.md`); ein späterer Lauf prüft ihn
  wie jeden anderen. Sonst landet in der Doku, was nie auf Relevanz und Nachhaltigkeit
  geprüft wurde.
- Keine Personennamen aus Tickets.

## Schritt 6 — Abschluss

Melde dem Nutzer:

- die Zahlen je Ziel (gelöscht je Grund, Handdoku, Todo, Nachtrag, Doku-Auftrag, Datei,
  `unclear/`) und je Grund für `übersprungen`;
- **immer** — auch wenn die Obergrenze nicht erreicht wurde —, wie viele Befunde noch in
  `keph:/queue/findings/` offen sind, neu gezählt mit `k_playbook_doc_list`, `kind: findings` und `limit: 1`
  (gezählt wird `total`, die Liste selbst wird nicht gebraucht). Ein eben gelöschter Befund kann in der Replica noch kurz stehen; dann
  gilt der Replica-Nachlauf aus `k-playbook:/instructions/on-demand/tasks.md`;
- den Verweis auf das Protokoll — nur hier, in dieser Antwort;
- die geänderten Dateien im Projekt (nicht committet) und die eingereihten Doku-Aufträge;
- gesammelt die Rückfragen (`übersprungen (Rückfrage)`), je mit der Frage.

Rückfragen an den Nutzer gibt es nur für Instructions, Skills, Commands und `AGENTS.md`;
sonst läuft der Command ohne Rückfrage durch.
