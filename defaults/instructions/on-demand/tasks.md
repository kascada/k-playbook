---
title: Tasks
description: Wie ein Task in der Projekt-Collection angelegt, aufgebaut, gelesen und geschrieben wird — Ablage unter user/~/tasks/, Name und Nummer, fremde und erledigte Verweise, Kopf mit title, Aufbau samt Verweisform, Lebenslauf bis done/, die Kennzeilen und Fortschrittszeilen von /k-task-run, der Schreibweg über die doc-Operationen mit since_revision samt Replica-Nachlauf und outcome_unknown, und was ohne Kephalaion gilt.
---

## Tasks

Ein Task ist ein Auftrag, den eine Sitzung ausführt — ein Dokument in der Projekt-Collection.
Diese Anweisung gilt für jeden Command und jede Sitzung, die einen Task anlegt, prüft, ausführt
oder ändert. `/k-task-create`, `/k-task-refine`, `/k-task-run` und `/k-remediation` verweisen
hierher, statt die Regeln zu wiederholen; die Geschichte eines Tasks steht in den
Kephalaion-Revisionen seines Dokuments, nicht in Git.

### Wohin und Name

- **Anlegen** mit `k_playbook_doc_create`, Sorte `tasks`, Kurzname in 2–4 Worten (klein,
  Bindestriche; k-playbook normalisiert ihn). Die Antwort nennt den `keph:`-Verweis mit dem
  echten User, etwa `keph:/user/kamran/tasks/007-audiosocket-server.md`. Den Inhalt schreibst
  du gleich danach mit Kephalaion `write`, `collection` gleich `kephalaion.address`, Frontmatter
  eingeschlossen.
- **Die Nummer vergibt k-playbook**, nicht die Sitzung: die höchste Nummer über alle
  `user/*/tasks/` samt `done/` plus eins, dreistellig; die erste ist `001`. Diese Regel steht
  nur hier. Niemand zählt selbst, und niemand gibt eine Nummer vor.
- **Eindeutig ist nur der Dateiname** `<NNN>-<kurzname>.md`, nicht die Nummer. Zwei Tasks mit
  derselben Nummer und verschiedenen Namen sind zulässig, etwa nach dem Umzug von der Platte.
  Eine Nummer als Argument eines Commands wird deshalb so aufgelöst: zuerst unter den eigenen
  offenen Tasks (`k_playbook_doc_list`, Sorte `tasks`), dann unter den eigenen erledigten
  (`done: true`). Passt mehr als ein Dokument, zeige alle Treffer und lass den Nutzer wählen.
  Ein `keph:`-Verweis als Argument wird unverändert genommen, nach den beiden Prüfungen unter
  „Fremde und erledigte Verweise“.

### Fremde und erledigte Verweise

Ein Command, der einen bestehenden Task ändert, an ihn anhängt, ihn verschiebt oder auf ihn
gestützt Code ändert, prüft den Verweis, bevor er etwas schreibt oder Code ändert:

- **Fremd.** Er nimmt nur Verweise, deren User-Pfadteil `~` oder `kephalaion.user` ist. Einen
  anderen weist er vorher ab; die Meldung nennt `foreign_user`, den Fehler, den die
  Dokument-Operationen dafür ohnehin melden würden. Ausgenommen sind reines Lesen und Suchen,
  etwa der Abgleich in `/k-remediation` mit `user: "*"`, und das Anlegen beim Umzug in
  `/k-upgrade` 3.3.
- **Erledigt.** Ein Verweis unter `done/` führt zur selben Rückfrage wie eine Nummer, die nur
  unter den eigenen erledigten Tasks trifft. Welche Rückfrage das ist, sagt der Command:
  `/k-task-run` bietet an, den Task wieder zu öffnen; `/k-task-refine` fragt, ob er trotzdem
  geprüft oder abgebrochen wird.

### Kopf

Frontmatter mit `title`: der Titel ohne „Task <n> —“. Das Briefing (`work.tasks`) und
`k_playbook_doc_list` zeigen ihn. `title` und andere Werte in freiem Text (etwa `description`,
`thema`, `anlass`) stehen in doppelten Anführungszeichen, ein `"` im Wert wird `\"`; so liest
jeder YAML-Leser genau diesen Text, auch bei „: “ oder „ #“. Unquotiert macht „: “ das
Frontmatter unlesbar, und „ #“ beginnt einen Kommentar, der den Rest des Titels abschneidet;
`k_playbook_doc_list` und das Briefing melden das unlesbare Frontmatter unter `problem`, den
abgeschnittenen Titel nur, soweit er erkennbar ist. Aufzählungs- und Strukturwerte (etwa
`sicherheit`, `erzeuger`, `generated`, `pfade`) bleiben, wie die Vorlage sie zeigt. Sonst ist
nichts Pflicht und nichts vorgesehen. Der Zustand eines Tasks steht nicht im Kopf: offen oder
erledigt sagt der Ort (`done/`), alles Weitere sagen die Kennzeilen im Rumpf.

```markdown
---
title: "<Titel>"
---
```

### Aufbau

```markdown
---
title: "<Titel>"
---

# Task <NNN> — <Titel>

<Ein Satz, was dieser Task erreicht.>

## Intent

<1–2 Sätze, die den Rahmen setzen — was gelten muss, wenn alles fertig ist.>
- <entscheidende Bedingung oder Erfolgskriterium>
- <entscheidende Bedingung oder Erfolgskriterium>

## Referenzen

- `<verweis>` — <warum er gebraucht wird>   (nur wenn vorhanden)

## Tools

- `mcp__<server>__<werkzeug>` — <wofür>   (nur über Read, Write, Edit, Bash, Glob, Grep hinaus)

## Ziel

<Was gebaut oder getan werden soll — so genau, dass ein Agent es ohne das Gespräch ausführen kann.>

## Kontext

<Festlegungen, Architekturentscheidungen und Hintergrund aus dem Gespräch, die der Ausführende braucht.>

## Zu bauen

<Konkrete Dateien, Module, Konfigurationen oder Schritte.>

### Etappe 1 — <Titel>

<Schritte dieser Etappe.>

### Etappe 2 — <Titel>

<Schritte dieser Etappe.>
```

Ein leerer oder nicht zutreffender Abschnitt entfällt. `## Intent` steht vor Referenzen und
Tools, damit er sofort sichtbar ist. Unter `## Tools` steht nur, was über Read, Write, Edit,
Bash, Glob und Grep hinausgeht — die sind in `/k-task-run` immer erlaubt.

**Verweise.** Jeder Verweis in einem Task hat die Form aus
`k-playbook:/instructions/always/010-verweise.md`: ein Pfad mit Präfix, Dateien im Projekt als
`project:/…`. Ein relativer Link oder ein nackter Pfad an einer Stelle, die ein Verweis sein
soll, wird gemeldet, nicht geraten. Was als Verweis gilt, welches Präfix ein nackter Pfad
bekommt und wie nachgezogen wird, steht in
`k-playbook:/instructions/on-demand/verweise-nachziehen.md`. Das gilt für alles, was ein Command
in einen Task schreibt, und ebenso für Instruction-Dateien auf der Platte, die
`/k-task-refine` prüft.

**Intent.** Der Intent sagt, welchen Rahmen oder welches Ergebnis der Task als Ganzes liefern
muss: 1–2 Sätze, dann 2–5 Punkte mit den entscheidenden Bedingungen oder Erfolgskriterien,
keine Umsetzungsdetails. Ein einzelner Task trägt immer einen `## Intent`. Bei einer Reihe
von Tasks trägt ihn nur der letzte; ist unklar, ob ein Task der letzte ist, wird vor dem
Entwurf gefragt. Lässt sich aus dem Gespräch kein Intent ableiten — etwa bei rein
mechanischer Arbeit ohne Erfolgskriterium —, entfällt der Abschnitt. Der Intent ist Text und
darf Verweise in der Form oben enthalten. Wer den Intent liest oder prüft, liest die Verweise
mit, auf den Lesewegen aus `k-playbook:/instructions/always/010-verweise.md`:
`k_playbook_doc_read` für `k-playbook:` und `keph:`, die Platte für `project:`.

**Zuschnitt: im Zweifel eine Datei.** Umfang allein ist kein Grund zu teilen. Ein Task, der
in Etappen zerfällt, wird ein Dokument mit `### Etappe N — Titel` unter `## Zu bauen`;
`/k-task-run` verfolgt den Fortschritt etappenweise und setzt nach einem Abbruch dort wieder
an. Geteilt wird nur, wenn die Teile **sachlich** auseinanderfallen: Sie brauchen
unterschiedlichen Kontext (müsste derselbe Kontextblock in beide, gehören sie zusammen), sie
stehen einzeln für sich (ein Teil, der ohne den anderen sinnlos ist, ist eine Etappe), und sie
lassen sich unabhängig verifizieren. Trifft das nicht klar zu: ein Dokument. Viele kleine
Tasks zersplittern den Intent — nur der letzte trägt ihn, die ersten laufen ohne
Erfolgskriterium.

**Etappen.** Nur gliedern, wenn die Arbeit wirklich in Abschnitte zerfällt, die nacheinander
abgeschlossen werden können; vier bis acht Etappen sind ein sinnvoller Rahmen. Jede Etappe
muss für sich fertig werden können; eine, die nur zusammen mit der nächsten Sinn ergibt,
gehört mit dieser zusammengelegt. Ohne `### Etappe`-Überschriften läuft der Task als Ganzes,
ohne Fortschrittsverfolgung.

### Lebenslauf

1. **Offen** — angelegt in `keph:/user/~/tasks/`. Das Briefing nennt ihn unter `work.tasks`.
2. **Geprüft** — `/k-task-refine` hat ein `## Review-Log (<datum>)` angehängt. Ein Task ohne
   diesen Block wurde nie gegengelesen; `/k-task-run` fragt dann nach.
3. **Ausgeführt** — `/k-task-run` hat `## Ausführung` mit der Kennzeile „ausgeführt“, Diff
   und Review angehängt. Solange das Dokument nicht in `done/` liegt, ist der Task nicht
   abgeschlossen (Review, Befunde oder eine Stopp-Abfrage offen).
4. **Erledigt** — `k_playbook_doc_done` verschiebt das Dokument nach `done/`; der Verweis
   ändert sich auf `keph:/user/<user>/tasks/done/<datei>`. Zurück nach offen geht es mit
   `k_playbook_doc_reopen`.

Jeder Schritt ist eine neue Revision des Dokuments; die Antwort von `k_playbook_doc_done`
und `k_playbook_doc_reopen` nennt die Revision nach dem Verschieben.

### Kennzeilen

`/k-task-run`, andere Commands und die spätere Oberfläche erkennen den Zustand eines Tasks an
festen Zeilen, je eine eigene Zeile am Zeilenanfang. `<datei>` ist der Dateiname des
Dokuments, der letzte Pfadteil des Verweises, etwa `014-setup-tts.md`; `<datum>` ist
`context.now.date`.

| Zeile | Schreibt | Bedeutung |
|---|---|---|
| `## Review-Log (<datum>)` | `/k-task-refine` | der Task wurde gegengelesen |
| `<!-- k-task-run: snapshot <datei> <kennung> -->` | `/k-task-run`, Hauptkontext | Snapshot des Arbeitsbaums vor dem Task; `<kennung>` ist ein Git-Tree in der Ausführungswurzel |
| `<!-- k-task-run: etappe <n> erledigt <datum> --> <beleg>` | Ausführungs-Sub-Agent von `/k-task-run`; `/k-upgrade` 3.3 beim Umzug aus einer alten Tabelle `## Fortschritt` | Etappe `<n>` ist erledigt (Fortschrittszeile, unten) |
| `<!-- k-task-run: fortschritt zurückgesetzt <datum> -->` | `/k-task-run`, Hauptkontext | alle Etappen gelten wieder als offen (Fortschrittszeile, unten) |
| `<!-- k-task-run: ausgeführt <datei> -->` | `/k-task-run`, Hauptkontext, mit dem Kopf von `## Ausführung` | ausgeführt; solange das Dokument nicht in `done/` liegt, nicht abgeschlossen. Zählt nur nach der letzten Kennzeile „wieder geöffnet“ |
| `<!-- k-task-run: abgebrochen <datei> -->` | `/k-task-run`, Hauptkontext, mit dem Kopf von `## Ausführung` eines abgebrochenen Laufs | der Lauf wurde abgebrochen; der Task bleibt offen |
| `<!-- k-task-run: wieder geöffnet <datei> <datum> -->` | `/k-task-run`, Hauptkontext, nach `k_playbook_doc_reopen` | der erledigte Task ist wieder offen; eine frühere Kennzeile „ausgeführt“ gilt nicht mehr |
| `### Diff: Geänderte Dateien` | Review-Sub-Agent (`k-playbook:/commands/_task-run/diff-review.md`) | Diff-Abschnitte eines Laufs |
| `### Review: Befunde` | Review-Sub-Agent | Review-Abschnitt eines Laufs |

Gesucht wird mit `k_playbook_doc_grep`, am Zeilenanfang und am Zeilenende verankert, Punkte im
Dateinamen maskiert. Die Kommentarzeilen mit `<datei>` tragen den eigenen Dateinamen: Ein
Task, der diese Vorlagen zitiert, trägt dort `<datei>` oder einen fremden Namen und gilt
deshalb nicht als ausgeführt. Die beiden Überschriftenzeilen `### Diff: …` und
`### Review: …` sind der Vertrag mit dem Modul `diff-review.md`; die Fortschrittszeilen der
Vertrag mit dem Ausführungs-Sub-Agenten.

Die Kennzeilen „ausgeführt“ und „abgebrochen“ stehen als erste Zeile nach `## Ausführung` und
der Leerzeile, vor den Statuszeilen.

**Rumpf und Lauf-Abschnitte.** Wer nur den Task selbst lesen will, ohne die Abschnitte früherer
Läufe, schneidet an der letzten Zeile `^## Ausführung$` vor der ersten Kennzeile
`^<!-- k-task-run: (ausgeführt|abgebrochen) <datei> -->$`. Gibt es keine solche Kennzeile, ist
der Schnitt an der ersten Zeile `^## Ausführung$`; fehlt auch sie, ist der ganze Task Rumpf.
Beide Muster sucht `k_playbook_doc_grep` mit Zeilennummern; gelesen wird dann
`k_playbook_doc_read` mit `lines: "1-<zeile vor dem Schnitt>"`, ohne Schnitt `lines: "1-"`.

### Fortschritt anhängend

Der Fortschritt eines Tasks mit Etappen wird anhängend geführt: eine Zeile je Ereignis, am
Ende des Dokuments, wo immer es gerade endet. Es gibt keine Tabelle `## Fortschritt` mehr und
keine Zeile, die später geändert wird.

- **Etappe erledigt.** Der Ausführungs-Sub-Agent hängt unmittelbar nach Abschluss jeder
  Etappe — vor der nächsten, nicht am Ende des Laufs — genau eine Zeile an:

  ```text
  <!-- k-task-run: etappe <n> erledigt <datum> --> <beleg>
  ```

  `<n>` ist die Nummer aus `### Etappe <n> — …`, `<datum>` `context.now.date`, `<beleg>` ein
  kurzer Satz oder Halbsatz auf derselben Zeile (Dateien, Tests, Messung), ohne Zeilenumbruch.
  Nur so überlebt der Stand einen harten Abbruch: Bricht die Sitzung mitten in Etappe 4 ab,
  stehen die Zeilen für 1 bis 3 schon im Dokument.
- **Zurücksetzen.** Soll ein Task von vorn laufen, hängt der Hauptkontext von `/k-task-run`
  eine Zeile an:

  ```text
  <!-- k-task-run: fortschritt zurückgesetzt <datum> -->
  ```

- **Lesen.** Der Stand ergibt sich allein aus diesen Zeilen, nicht aus dem Code und nicht aus
  `git log`: `k_playbook_doc_grep` mit dem Muster
  `^<!-- k-task-run: (etappe [0-9]+ erledigt|fortschritt zurückgesetzt) ` liefert sie mit
  Zeilennummern. Es gilt je Etappe die letzte Zeile nach dem letzten Zurücksetzen; eine
  Etappe ohne solche Zeile ist offen. Eine offene Etappe gilt als nicht ausgeführt, auch wenn
  Teile davon im Code sichtbar sind — der Sub-Agent prüft zu Beginn der Etappe selbst, was
  schon da ist.
- **Alte Tabelle `## Fortschritt`.** Eine solche Tabelle aus der Zeit auf der Platte liest
  allein `/k-upgrade` 3.3: Beim Umzug übersetzt es ihre erledigten Zeilen in
  Fortschrittszeilen, die Tabelle bleibt als Beleg stehen. `/k-task-run` wertet sie nicht
  aus. Ohne Fortschrittszeilen gelten ihre Etappen als offen.

### Lesen und schreiben

Ein Task wird nach dem Anlegen nicht mehr als Ganzes geschrieben. Es gelten die
Dokument-Operationen von k-playbook; jede Antwort nennt die Revision des Dokuments:

| Was | Werkzeug |
|---|---|
| ganzen Inhalt schreiben | nur, wer den Task anlegt: Kephalaion `write` direkt nach `k_playbook_doc_create` |
| lesen | `k_playbook_doc_read` mit `section` (etwa `## Intent`) oder `lines`; die Antwort trägt `revision` und `totalLines` |
| suchen | `k_playbook_doc_grep` mit `ref` und `pattern`; mit `count_only` nur die Anzahl und die Revision |
| anhängen | `k_playbook_doc_append` mit `content` und `since_revision` |
| eine Stelle ändern | `k_playbook_doc_replace` mit `old` (genau ein Vorkommen) und `new`, `since_revision` |
| Frontmatter ändern | `k_playbook_doc_set_field` mit `key`, `value` oder `unset`, `since_revision` |
| verschieben | `k_playbook_doc_done`, `k_playbook_doc_reopen`; die Antwort nennt die neue Revision |

Ersatzweise ohne MCP in der Shell: `k-playbook doc append <ref> [--since N] <<'EOF'` mit der
Endmarke in Anführungszeichen (Backticks und `$(…)` kommen so wörtlich an), `k-playbook doc
replace <ref> [--since N]` mit `{"old": …, "new": …}` auf stdin, `k-playbook doc set <ref>
<key> <value>`, `k-playbook doc grep <ref> <muster> [--count]`, `k-playbook read <ref>
--section '<überschrift>'` bzw. `--lines <von>-<bis>`, `k-playbook doc done <ref> --json` und
`k-playbook doc reopen <ref> --json`.

**Ein Schreiber.** Ein Task hat zu einer Zeit einen Schreiber. Jedes Anhängen und Ändern trägt
`since_revision`: die Revision des eigenen letzten Schreibens — aus der Antwort von `append`,
`replace` oder `set_field`, nach dem Verschieben aus der Antwort von `k_playbook_doc_done`
bzw. `k_playbook_doc_reopen`. Vor dem ersten Schreiben einer Sitzung kommt die Revision aus
`k_playbook_doc_read` oder aus `k_playbook_doc_grep` mit `count_only`.

- **`foreign_change`** heißt: Seit dem eigenen letzten Schreiben hat jemand anderes
  geschrieben. Nichts wurde geschrieben. Halte an, schreibe nichts weiter, sende die
  Fertig-Meldung und frage den Nutzer, wer weitermacht — mit `revision`, `updatedBy` und
  `updatedAt` aus der Antwort. Nie still nachziehen.
- **`outcome_unknown`** heißt: Ob das Schreiben angekommen ist, ist offen. Nicht blind
  wiederholen, sondern nachlesen — der feste Weg je Operation steht unten unter
  „`outcome_unknown` je Operation“.
- **Wiedereinstieg nach `foreign_change`.** Sagt der Nutzer, dass weitergemacht wird, gilt
  die Revision aus der `foreign_change`-Antwort als nächstes `since_revision` — auch dann,
  wenn ein Sub-Agent sie gemeldet hat (Zeile `Foreign-Change:` unten).

**Replica-Nachlauf.** Gelesen wird aus der Replica des Nodes, und die läuft dem Hub nach. Die
Schreib-Operationen (`append`, `replace`, `set_field`) warten darauf: Liegt die gelesene
Revision unter `since_revision`, lesen sie nach kurzer Pause einige Male neu, bevor sie
`foreign_change` melden. `k_playbook_doc_grep` und `k_playbook_doc_read` warten nicht.

- Liefert ein Lesen oder Zählen direkt nach dem eigenen Schreiben eine Revision **unter** der
  eigenen letzten, läuft die Replica nach: kurz warten (etwa eine Sekunde), neu lesen oder
  zählen, bis zu dreimal.
- Nur eine **höhere** Revision, oder eine, die nach diesen Versuchen weiter abweicht, gilt wie
  `foreign_change`.
- Dasselbe gilt für `not_found` direkt nach dem eigenen `k_playbook_doc_done` bzw.
  `k_playbook_doc_reopen`, wenn unter dem neuen Verweis gelesen oder gezählt wird.

Jeder Command, der nach dem eigenen Schreiben zählt oder liest, verweist auf diese Regel,
statt eine eigene Fassung zu schreiben.

**Zählen vor dem Anhängen.** Wird die Ankunft eines Anhängens später an einer Überschrift oder
Kennzeile geprüft — `## Review-Log (<datum>)`, `### Diff: Geänderte Dateien`,
`### Review: Befunde`, jede Kennzeile —, zählt der Schreiber diese Zeile vorher mit
`k_playbook_doc_grep` und `count_only` und merkt sich die Anzahl. Die Revision aus dieser
Antwort muss die eigene letzte sein (Replica-Nachlauf oben).

**`outcome_unknown` je Operation.** Immer erst nachlesen, die Revision aus dieser Antwort
übernehmen und nur dann neu schreiben, wenn sicher nichts angekommen ist:

| Operation | Nachlesen | Angekommen, wenn |
|---|---|---|
| `append` | `k_playbook_doc_grep` mit `count_only` auf die gezählte Zeile (oben) | die Zahl gestiegen ist |
| `replace` | `k_playbook_doc_grep` mit `count_only` auf eine Zeile aus `new`, die in `old` nicht vorkommt | sie jetzt dasteht |
| `set_field` | `k_playbook_doc_read` mit `lines` über das Frontmatter | das Feld den neuen Wert trägt bzw. fehlt |
| `done`, `reopen` | `k_playbook_doc_list` (Sorte `tasks`, `done: true`), dann `k_playbook_doc_grep` mit `count_only` unter dem neuen Verweis für die Revision | das Dokument am neuen Ort liegt |

Ist es angekommen, gilt die Revision aus dem Nachlesen als die eigene letzte. Ist es nicht
angekommen und stimmt die Revision mit der eigenen letzten überein (nach dem
Replica-Nachlauf), wird einmal neu geschrieben, mit derselben `since_revision`: Kommt das erste
Schreiben doch noch an, weist `since_revision` das zweite als `foreign_change` ab, statt es
doppelt zu schreiben. `done` und `reopen` haben keine `since_revision`; sie werden nur
wiederholt, wenn das Dokument nach dem Nachlauf weiter am alten Ort liegt. Weicht die
Revision anders ab, gilt es wie `foreign_change`.

**Sub-Agenten.** Wer einen Sub-Agenten in den Task schreiben lässt, hält die Revisionskette
selbst:

1. Vor der Übergabe mit `k_playbook_doc_grep` und `count_only` die Kennzeile zählen, die der
   Sub-Agent schreiben soll (etwa `^### Review: Befunde$` oder
   `^<!-- k-task-run: etappe [0-9]+ erledigt `), und sich Anzahl und Revision aus dieser
   Antwort merken. Weicht die Revision von der gemerkten eigenen ab — höher, oder nach dem
   Replica-Nachlauf weiter darunter —, gilt es wie `foreign_change`.
2. Der Sub-Agent bekommt den `keph:`-Verweis und die aktuelle Revision, hängt mit
   `since_revision` an und gibt in seinem Rückgabeformat eine Zeile `Revision: <n>` zurück —
   die Revision seines letzten Schreibens. Bekommt er `foreign_change`, hält er an, schreibt
   nichts weiter und gibt statt `Revision: <n>` die Zeile
   `Foreign-Change: <revision> <updatedBy> <updatedAt>` zurück. Der Aufrufer behandelt sie
   wie ein eigenes `foreign_change`: Fertig-Meldung, Rückfrage mit diesen Angaben.
3. Fehlen beide Zeilen, wird erneut gezählt (mit dem Replica-Nachlauf oben): Ist die Anzahl
   gegenüber Schritt 1 gestiegen, gilt die Revision aus dieser Antwort; sonst gilt es wie
   `foreign_change`. Eine alte Kennzeile aus einem früheren Lauf zählt so nicht als neu.

Die Operationen lassen den Inhalt im Programm: Ein Hauptkontext, der nur anhängt, sucht und
Abschnitte liest, sieht Diff und Review nie — dafür sind sie gebaut.

### Ohne Kephalaion

Steht `context.kephalaion.ok` nicht auf `true`, oder antwortet eine Operation mit
`kephalaion_unavailable`, gibt es keinen Task: nichts auf die Platte, kein Rückfall auf ein
Verzeichnis. Der Command endet mit der Meldung aus `context.kephalaion.message` und nennt die
Setup-Seite (`/k-gui`), auf der die Projekt-Collection eingerichtet wird.
