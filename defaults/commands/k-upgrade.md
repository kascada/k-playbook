# k-upgrade

Zieht Programm und Projekt auf den aktuellen Stand von k-playbook. Zuerst das, was das
Programm allein kann: das Binary, wenn das laufende älter ist als die Installation, danach
`AGENTS.md` (Anstoß, abgelöste Blöcke), `CLAUDE.md`, die Verlinkung der Commands und Skills
und die MCP-Einträge. Das tut der Unterbefehl `k-playbook upgrade`, und den führt die Hülle
dieses Commands schon beim Aufruf aus — vor dem ersten Schritt, damit das Briefing danach
den neuen Stand sieht und nicht das alte Programm die Verlinkung nach seinen Regeln
nachzieht. Danach die Umstellungen, die eine Beurteilung brauchen und deshalb die Sitzung
macht, nicht das Programm, zuletzt das Aufräumen der Verzeichnisse, die das Einrichten nicht
mehr anlegt. Der Command nimmt kein Argument.

## Schritt 1 — Briefing

include: _shared/erster-schritt.md

## Schritt 2 — Ausgabe des Nachziehens

Die Ausgabe von `k-playbook upgrade` steht in der Hülle dieses Commands, unter „Ausgabe von
`k-playbook upgrade`“. Gib sie unverändert wieder — jede Zeile, auch Hinweise und
Fehlermeldungen wie „Download/Install nicht möglich“. Fasse nichts zusammen und lass nichts
weg. In diesem Schritt führst du nichts anderes aus und änderst keine Datei. Was der Lauf
nicht von selbst nachziehen konnte, steht in der Ausgabe als Hinweis; nenne ihn, behebe ihn
nicht.

Steht dort keine Ausgabe von `k-playbook upgrade` — die Zeile ist leer, nennt nur den Befehl
selbst oder meldet, dass er nicht gefunden oder nicht ausgeführt wurde —, dann hat der
Assistent die Einbettung nicht ausgeführt. Führe dann in der Shell aus:

    k-playbook upgrade

und gib diese Ausgabe nach derselben Regel wieder. Nenne dazu in einer Zeile, dass die Hülle
den Aufruf nicht ausgeführt hat.

Steht in der Ausgabe, dass das Programm gewechselt hat und die Sitzung neu zu starten ist,
endet der Command hier mit genau diesem Hinweis: Der laufende MCP-Server behält das alte
Binary, bis der Assistent ihn neu startet. Füge hinzu, dass die Umstellungen aus Schritt 3
danach mit einem zweiten Lauf von `/k-upgrade` folgen; sie brauchen die Werkzeuge des neuen
Programms.

## Schritt 3 — Umstellungen

Jede Umstellung prüft zuerst ihren Anlass. Liegt er nicht vor, meldest du eine Zeile
„nichts zu tun“ und gehst zur nächsten; eine erledigte Umstellung wird so beim nächsten Lauf
von selbst übersprungen.

Für alle Umstellungen gilt:

- Gelöscht, verschoben oder überschrieben wird erst nach Bestätigung. Lege je Umstellung
  gebündelt vor, was du vorhast, und warte auf die Antwort.
- Bist du bei einem Eintrag unsicher, sag das in der Vorlage ausdrücklich, statt still zu
  entscheiden.
- Braucht eine Umstellung die Projekt-Collection und steht `context.kephalaion.ok` nicht auf
  `true`, wird sie übersprungen; sag das und nenne den Grund aus `context.kephalaion.message`.
- Eine Verweissuche im Projekt geht über alle Dateien, die unter `context.project.dir` auf der
  Platte liegen, ob Git sie erfasst oder nicht. Aufgeräumt wird, was auf der Platte liegt, und
  ein Verweis in einer ignorierten Datei zeigt nach dem Löschen genauso ins Leere. Deshalb
  kein `git grep` und keine Suche, die Ignore-Dateien beachtet (`rg`, `ugrep`). In Claude
  Code ersetzt die Shell schon `grep` durch eine solche Suche; nimm darum `command grep`:

  ```sh
  command grep -rnI --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=.venv \
    --exclude-dir=venv --exclude-dir=__pycache__ "<zeichenfolge>" <context.project.dir>
  ```

  Danach streichst du aus dem Ergebnis die Installation unter `context.playbook.dir` und
  alles, was die Umstellung sonst ausnimmt. Eine Stelle in einem anderen Git-Repository
  unterhalb des Projekts — einem Verzeichnis mit eigenem `.git`, etwa einem eingebetteten
  Clone — nennst du in der Vorlage mit `[fremd]`: Sie zählt als Verweis, wird gemeldet und
  nicht geändert.

### 3.1 — Befunde aus `material/befunde/`

Befunde lagen früher als Thema-Dateien unter `<context.local.dir>/material/befunde/`. Heute
ist ein Befund ein eigenes Dokument in `keph:/queue/findings/`, und auf ihn wird nicht
verwiesen; was als Beleg eines laufenden Vorhabens gebraucht wird, steht in der Workbench.
Diese Umstellung leert das alte Verzeichnis.

**Anlass:** In `material/befunde/` liegt mindestens eine `.md`-Datei außer `README.md`.
Braucht die Collection.

Lies vorher `k-playbook:/instructions/on-demand/befunde.md` und den Abschnitt „Die
Workbench“ der Anweisungen. Für jede Befund-Datei gilt: sie wird ganz gelesen, bevor du sie
beurteilst.

**a) Verweise suchen.** Gesucht wird die Zeichenfolge `material/befunde/` in:

- allen Dateien des Projekts auf der Platte, nach der Verweissuche oben;
- deinen offenen Tasks und aktiven Workbench-Themen aus dem Briefing (`work.tasks`,
  `work.workbench`), gelesen mit `k_playbook_doc_read`.

Nicht gesucht wird in:

- `material/` selbst, weil Befunde einander zitieren;
- der Installation unter `context.playbook.dir`;
- erledigten Tasks (`tasks/done/`, `keph:/user/*/tasks/done/`), weil sie Geschichte sind und
  ihre Verweise stehen bleiben.

Ergebnis ist je Befund-Datei die Liste der Stellen (`pfad:zeile` oder `keph:`-Verweis), die
auf sie zeigen.

**b) Verwiesene Befunde beurteilen.** Lies zu jeder verwiesenen Datei auch die Stellen, die
auf sie zeigen. Die Frage ist, ob die Stelle den Befund heute noch braucht. Lege gebündelt
vor, eine Zeile je Datei mit Grund:

```text
Verwiesene Befunde
  markdown-rendering.md  ← installer/internal/markdown/markdown.go:11
      [workbench]  der Kommentar begründet die Wahl der Goldmark-Version mit der Messung
  release.md             ← tasks/097-todos.md:40
      [entfällt]   der Task ist umgesetzt, der Verweis erklärt nichts mehr
  briefing-health.md     ← docs/mcp.md:812
      [unklar]     gemessen an v0.16.9; ob die Doku die Messung noch braucht, weiß ich nicht
```

- **workbench** — die Stelle braucht den Befund. Er wird ein Workbench-Thema, und der
  Verweis zeigt danach dorthin.
- **entfällt** — die Stelle braucht ihn nicht mehr. Der Verweis wird entfernt; trägt der Satz
  ohne ihn nicht, formulierst du ihn so um, dass er für sich steht.
- **unklar** — du kannst es nicht entscheiden. Der Nutzer entscheidet.

**c) Ausführen, was bestätigt ist.** Für jede Datei mit `workbench`:

1. Ein Thema anlegen mit `k_playbook_doc_create`, Sorte `workbench`, Kurzname nach dem
   Dateinamen.
2. Es schreiben mit Kephalaion `write`, Form nach „Die Workbench“: `title` aus `thema:` der
   Datei. Übernommen wird, was die verweisenden Stellen brauchen und was noch gilt; ein
   widerlegter Abschnitt nur, wenn er vor einer Sackgasse warnt.
3. Jeden Verweis auf die Datei durch den `keph:`-Verweis des Themas ersetzen. Einen Anker
   (`#…`) dahinter lässt du weg.

Für jede Datei mit `entfällt` entfernst du die Verweise wie beschrieben. Stellen mit
`[fremd]` bleiben in beiden Fällen unverändert.

**d) Die übrigen Befund-Dateien beurteilen.** Jede Datei ohne Verweis prüfst du darauf, ob sie
noch Wert hat. Lege gebündelt vor, eine Zeile je Datei mit Grund:

- **workbench** — es ist noch etwas zu tun: offene Punkte, eine unbestätigte These, die zu
  prüfen bleibt, ein Vorhaben, das nicht fertig ist.
- **findings** — nur nützlich: belegtes Wissen ohne offene Arbeit, das in die
  Wissensablage gehört.
- **verwerfen** — überholt, erledigt oder vom Code schon gesagt.
- **unklar** — der Nutzer entscheidet.

Eine Datei kann beides enthalten. Dann nenne beides, und die Datei wird geteilt.

**e) Ausführen, was bestätigt ist.**

- **workbench** wie in c), nur ohne Verweise.
- **findings:** Aus jedem Abschnitt, der noch Wert hat, wird ein eigener Befund nach
  `k-playbook:/instructions/on-demand/befunde.md`. Dabei gilt:
  - `thema` kommt aus `thema:` der Datei, `sicherheit` aus dem Abschnitt.
  - `sitzung` kommt aus dem `<!-- sitzung: … -->` des Abschnitts, sonst bleibt es leer.
  - `entstanden: migration`, `ziel` leer.
  - Ein widerlegter Abschnitt wird nur übernommen, wenn er vor einer Sackgasse warnt.
  - Ein späterer Abschnitt, der einen früheren richtigstellt, ersetzt ihn; beide zu
    übernehmen hieße, einen Irrtum in die Queue zu stellen.
- **verwerfen:** nichts.

**f) Aufräumen.** Lösche jede Befund-Datei, die vollständig verarbeitet ist — erst wenn alles,
was aus ihr werden sollte, geschrieben steht. Ihr Inhalt bleibt in der Git-Geschichte.
`README.md` und die Verzeichnisse fasst diese Umstellung nicht an; über `material/` samt
`befunde/` entscheidet 3.4. Eine Datei, bei der etwas fehlgeschlagen ist, bleibt liegen, und
der nächste Lauf nimmt sie wieder auf.

**g) Melden.** Je Datei eine Zeile: was aus ihr wurde (Workbench-Verweis, Zahl der Befunde,
verworfen, liegen geblieben), dazu jede geänderte Verweisstelle als `pfad:zeile` und jede
Stelle mit `[fremd]`, die jetzt ins Leere zeigt.

### 3.2 — Doku aus `k-playbook-local/docs/`

Die Projektdoku liegt heute in der Collection des Projekts unter `keph:/docs/`; das Briefing
nennt diesen Einstieg unter `Projektdoku`, und `k-playbook:/commands/_shared/docs.md` sagt,
was darunter gilt. Früher lag sie unter `<context.local.dir>/docs/`, mit einem Index
`docs/README.md` von `/k-docs-index` und Verweisen darauf in `AGENTS.md` und
`opencode.json`. Diese Umstellung bringt die Doku je Bereich nach Kephalaion oder räumt
sie ab — nichts verschwindet still, und ein Bereich, dessen Erzeuger noch auf die Platte
schreibt (heute nur `libs/`), bleibt liegen. Der Doku-Index steht nur noch in `docs/` der
Collection; `AGENTS.md` und `opencode.json[c]` tragen keinen Doku-Index und keine
Doku-Verweise mehr.

**Anlass:** Unter `<context.local.dir>/docs/` liegt mindestens eine Datei aus den Bereichen
der Tabelle unten, oder `AGENTS.md` bzw. `opencode.json[c]` nennen das Präfix
`k-playbook-local/docs/` (dieselbe Suche wie in b). Braucht die Collection.

Die Bereiche und ihr Ziel. Wird ein weiterer Erzeuger auf Kephalaion umgestellt, ändert
dessen Task seine Zeile:

| Bereich auf der Platte | Ziel | Vorgehen |
|---|---|---|
| `versions/` | `keph:/docs/generated/versions/` | Erzeuger (`/k-doc-inventory`, `k-playbook inventory`) schreibt schon nach Kephalaion: Plattendatei löschen; fehlt das Ziel, Erzeuger neu laufen lassen |
| `code/` | `keph:/docs/generated/code/` | Erzeuger `/k-docs-code` schreibt nach Kephalaion: Plattendateien löschen; neu erzeugen mit `/k-docs-code`. Kein Verschieben — der alte Stand bleibt in der Git-Geschichte |
| `libs/` | `keph:/docs/generated/libs/` | Erzeuger `/k-docs-tools` schreibt noch auf die Platte: nicht anfassen |
| `manual/` | `keph:/docs/manual/`, Pfad darunter gleich | von Hand geschrieben: verschieben; `manual/README.md` nicht verschieben (`README.md` schreibt kein Command, sie entsteht beim Indizieren), sondern löschen |
| `extracted/` und lose `.md` direkt unter `docs/` außer `README.md` | Vorschlag `keph:/docs/manual/<datei>` | je Datei verschieben oder verwerfen, der Nutzer entscheidet |
| `README.md` | entfällt | der alte Index von `/k-docs-index`: löschen, sobald seine Einträge zu `manual/`, `versions/`, `code/` und losen Dateien abgeräumt sind — auch wenn `libs/` noch liegt; sonst bliebe ein veralteter Index als stiller Rückfall |

`docs/` selbst und seine README fasst diese Umstellung nicht an; darüber entscheidet 3.4.

**a) Bestand erheben.** Lies, was unter `<context.local.dir>/docs/` liegt, je Bereich der
Tabelle. Lies dazu `keph:/docs/` mit Kephalaion `list` (`recursive`, `frontmatter`),
Collection = `context.kephalaion.address`. Für jede Datei, die verschoben werden soll,
prüfst du vorher:

- Liegt unter dem Zielnamen schon ein Dokument, vergleiche beide. Gleich → nur die
  Plattendatei löschen. Verschieden → beide zeigen; der Nutzer entscheidet, welche gilt.
- Ist die Datei kein Text oder größer als 1 MiB, bleibt sie liegen und wird gemeldet.
- Zeigt ein Link in der Datei aus `manual/` hinaus (etwa auf `../versions/…` oder in den
  Code), merkst du ihn für die Meldung vor; umgeschrieben wird er nicht.

Für einen umgestellten Erzeuger (heute `versions/` und `code/`): fehlt sein Ziel in
Kephalaion, erinnerst du daran, ihn neu laufen zu lassen. Beim Inventar bietest du an,
`k-playbook inventory` gleich auszuführen; der Lauf ist deterministisch. `/k-docs-code`
fragt nach Scope und Struktur und läuft deshalb nicht nebenbei — nenne ihn als nächsten
Schritt. Jede Datei unter `code/` wird einzeln zum Löschen vorgeschlagen, auch wenn
`keph:/docs/generated/code/` noch leer ist: Ihr Inhalt bleibt in der Git-Geschichte, und der
Erzeuger schreibt das Verzeichnis neu.

`docs/README.md` ist der alte Index, wenn sein Frontmatter oder sein Inhalt ihn als Werk
von `/k-docs-index` ausweist — die Übersicht je Herkunft mit Tabellen, der
Stichwort-Index, „Häufige Fragen → direkter Sprung“. Trägt die Datei stattdessen nur den
Text der Struktur-README, die das Einrichten früher anlegte, ist nichts zu tun. Ist sie von
Hand geschrieben und beides nicht, behandelst du sie wie eine lose Datei.

**b) Doku-Verweise suchen.** Gesucht wird das Präfix `k-playbook-local/docs/` in
`AGENTS.md` sowie `opencode.json` und `opencode.jsonc` in `context.project.dir`, dazu in
`opencode.json[c]` ein Eintrag `references.docs`, dessen `path` auf `./k-playbook-local/docs`
zeigt. Der Einstieg kommt aus dem Briefing, der Doku-Index steht nur noch in `docs/` der
Collection; das Programm schreibt keinen dieser Verweise mehr neu. Der Vorschlag ist immer
**entfernen**, nie umschreiben — auch nicht in einen `keph:`-Verweis:

- eine Tabelle „Häufige Fragen → Datei“ oder eine andere Kurzverweis-Tabelle ganz;
- einen Absatz, dessen Kern ein solcher Verweis ist — etwa „Docs zuerst … unter
  `k-playbook-local/docs/`“ oder ein Verzeichnisbaum der alten Ablage — ganz;
- in einem sonst eigenständigen Absatz nur den Verweis; der Rest bleibt stehen;
- `references.docs` in `opencode.json[c]`.

Das gilt für alle Verweise mit dem Präfix, auch für solche auf `libs/`: Die Dateien dort
bleiben liegen, bis `/k-docs-tools` umgestellt ist, sind bis dahin aber nicht mehr verwiesen.
Je Fundstelle merkst du `pfad:zeile` vor, bei einer Tabelle oder einem Absatz über mehrere
Zeilen `pfad:von-bis`.

**c) Vorlegen.** Lege gebündelt vor, eine Zeile je Datei bzw. Bereich mit Vorschlag und
Grund, in dieser Reihenfolge: umgestellte Erzeuger, `manual/`, lose Dateien und
`extracted/`, der alte Index, die Verweise. Ein Beispiel aus einem Projekt mit alter
Code-Doku und einer Index-Tabelle in `AGENTS.md`:

```text
Doku aus k-playbook-local/docs/
  versions/inventory.md        [löschen]      liegt als keph:/docs/generated/versions/inventory.md, Stand 2026-10-25
  code/00-overview.md          [löschen]      Erzeuger /k-docs-code schreibt nach keph:/docs/generated/code/; neu erzeugen mit /k-docs-code
  code/13-rag-pipeline.md      [löschen]      dito; keph:/docs/generated/code/ trägt schon 17 Dokumente
  manual/release.md            [verschieben]  → keph:/docs/manual/release.md, Ziel frei
  manual/installation.md       [verschieben]  → keph:/docs/manual/installation.md, Ziel vorhanden und gleich: nur löschen
  manual/README.md             [löschen]      README.md schreibt kein Command, sie entsteht beim Indizieren
  notizen.md                   [unklar]       lose Datei ohne Erzeuger; Vorschlag keph:/docs/manual/notizen.md
  README.md                    [löschen]      alter Index von /k-docs-index, Einträge abgeräumt
  libs/                        [bleibt]       nicht Teil dieser Umstellung, /k-docs-tools schreibt noch auf die Platte
  AGENTS.md:54                 [entfernen]    Absatz „Docs zuerst …“, sein Kern ist der Verweis auf k-playbook-local/docs/
  AGENTS.md:82-83              [entfernen]    Verzeichnisbaum der alten Ablage
  AGENTS.md:224                [entfernen]    nur der Verweis auf k-playbook-local/docs/libs/…, der Absatz bleibt
  AGENTS.md:254-264            [entfernen]    Tabelle „Häufige Fragen → Datei“
  opencode.json:8              [entfernen]    references.docs mit dem alten Einstieg
```

- **löschen** — die Plattendatei entfällt, ihr Inhalt liegt in Kephalaion oder wird vom
  Erzeuger neu geschrieben.
- **verschieben** — Kephalaion `create` unter dem Zielnamen, danach die Plattendatei löschen.
- **entfernen** — eine Tabelle, ein Absatz oder ein einzelner Verweis in `AGENTS.md`, oder
  `references.docs` in `opencode.json[c]`; nie umgeschrieben.
- **bleibt** — nicht Teil dieser Umstellung.
- **unklar** — du kannst es nicht entscheiden; der Nutzer entscheidet, je Zeile.

Warte auf die Antwort. Der Nutzer bestätigt die Vorlage ganz oder nimmt einzelne Zeilen aus.

**d) Ausführen, was bestätigt ist.** In der Reihenfolge der Vorlage:

1. **verschieben:** Kephalaion `create` mit dem ganzen Inhalt der Datei unter dem
   Zielnamen (ohne führenden `/`); bei einem gleichen Dokument am Ziel entfällt das
   Schreiben, bei einem verschiedenen gilt die Entscheidung des Nutzers (`write` mit
   `base_revision` aus dem `read`). Erst nach einem bestätigten Schreiben — die Antwort
   nennt Revision und Größe — löschst du die Plattendatei.
2. **löschen:** die Plattendatei entfernen. Der alte Index zuletzt, und nur, wenn alle Zeilen
   zu `manual/`, `versions/`, `code/` und losen Dateien ausgeführt sind; blieb eine davon
   liegen, bleibt auch er und wird beim nächsten Lauf erneut vorgeschlagen.
3. **entfernen:** in `opencode.json[c]` den Eintrag `references.docs` (ein leeres
   `references` dazu); in `AGENTS.md` die bestätigte Tabelle, den Absatz bzw. den einzelnen
   Verweis. Die übrigen Einträge und die Formatierung der Datei bleiben, wie sie sind.

Ein Schritt, der fehlschlägt, lässt seine Datei liegen; der nächste Lauf nimmt sie wieder
auf. Ihr Inhalt bleibt außerdem in der Git-Geschichte.

**e) Aufräumen.** Ein Verzeichnis unter `docs/`, das danach leer ist (`manual/`, `versions/`,
`code/`, `extracted/`), löschst du mit. `docs/` selbst und seine README fasst diese
Umstellung nicht an.

**f) Melden.** Je Datei eine Zeile: was aus ihr wurde (`keph:`-Verweis, gelöscht, liegen
geblieben und warum), dazu je vorgemerktem Link aus `manual/` hinaus `datei:zeile` und je
entferntem Verweis `pfad:zeile`. Prüfe das Ergebnis mit Kephalaion `list` auf `docs/` und
nenne, was dort jetzt liegt.

Die Umstellung ist wiederholbar: `libs/` bleibt liegen, solange `/k-docs-tools` noch auf
die Platte schreibt. Ist es umgestellt, ändert sein Task die Zeile `libs/`, und der nächste
Lauf schlägt auch dort das Abräumen vor.

### 3.3 — Tasks von der Platte

Tasks lagen früher als Dateien unter `<context.local.dir>/tasks/`, erledigte unter
`tasks/done/`, ausgemusterte unter `tasks/old/`. Heute ist ein Task ein Dokument in der
Projekt-Collection unter `keph:/user/<user>/tasks/`, erledigt unter `done/`; die Form steht
in `k-playbook:/instructions/on-demand/tasks.md`, und `/k-task-create`, `/k-task-refine`,
`/k-task-run` und `/k-remediation` lesen und schreiben nur noch dort. Diese Umstellung
verschiebt die Dateien: gleicher Name, gleiche Nummer, beim richtigen User.

**Anlass:** In `<context.local.dir>/tasks/`, `tasks/done/` oder `tasks/old/` liegt eine
`.md`-Datei außer `README.md`. Braucht die Collection.

Lies vorher `k-playbook:/instructions/on-demand/tasks.md` und
`k-playbook:/instructions/on-demand/verweise-nachziehen.md`.

**a) Bestand erheben.** Je Datei das Ziel: aus `tasks/` nach `user/<owner>/tasks/<gleicher
Name>`, aus `tasks/done/` nach `user/<owner>/tasks/done/<gleicher Name>`. `<owner>` ist
vorgeschlagen `context.kephalaion.user`; der Nutzer kann je Zeile einen anderen User nennen.
Dateien aus `tasks/old/` bekommen keinen Vorschlag — `[unklar]`: nach `done/` oder verwerfen,
der Nutzer entscheidet.

Für jede Datei prüfst du vorher:

- **Kollision.** Liegt der Dateiname schon irgendwo unter `user/*/tasks/` — auch bei einem
  anderen User oder unter `done/` —, zeigt es Kephalaion `list` auf `user/` mit `recursive`
  (`collection` = `context.kephalaion.address`). Dann vergleichen: Ist der Inhalt gleich
  (nach dem Ergänzen des Frontmatters, siehe unten), wird nur die Plattendatei gelöscht. Ist
  er verschieden, zeigst du beide; der Nutzer entscheidet, welcher gilt.
- **Kein Text oder größer als 1 MiB:** liegen lassen und melden.

**Frontmatter.** Ein Task trägt `title` im Frontmatter
(`k-playbook:/instructions/on-demand/tasks.md`, „Kopf“). Fehlt es der Datei, bildest du es
aus der ersten Überschrift `# Task <n> — <Titel>`: `title` ist `<Titel>`, ohne „Task <n> —“.
Fehlt auch die, ist `title` der Dateiname ohne Nummer und `.md`, mit Leerzeichen statt
Bindestrichen. Der übrige Inhalt bleibt Byte für Byte; das Frontmatter kommt davor. Trägt die
Datei schon ein Frontmatter ohne `title`, kommt die Zeile `title: <Titel>` als erste Zeile in
diesen Block — kein zweiter Block. Trägt sie ein Frontmatter mit `title`, bleibt sie ganz
unverändert.

**Fortschritt.** Ein Task mit einer alten Tabelle `## Fortschritt` und ohne Kennzeile
`<!-- k-task-run: ausgeführt <datei> -->` bekommt seine erledigten Etappen als
Fortschrittszeilen (`k-playbook:/instructions/on-demand/tasks.md`, „Fortschritt anhängend“,
„Alte Tabelle `## Fortschritt`“), damit
`/k-task-run` keine erledigte Etappe noch einmal ausführt. Je Zeile der Tabelle mit Status
`erledigt` entsteht genau eine Zeile:

```text
<!-- k-task-run: etappe <n> erledigt <datum> --> <notiz>
```

- `<n>` ist die Nummer der Etappe aus der ersten Spalte (`<n> — <Titel>`). Sie muss zu einer
  Überschrift `### Etappe <n> — <Titel>` desselben Tasks passen, Nummer und Titel.
- `<datum>` ist das Datum aus der Tabelle, nicht `context.now.date`.
- `<notiz>` ist die Notiz aus der Tabelle, einzeilig und auf etwa 200 Zeichen gekürzt
  (Ende mit „…“); Durchgestrichenes (`~~…~~`) entfällt.
- Zeilen mit einem anderen Status (`offen`, leer) ergeben keine Zeile; die Etappe bleibt
  offen.
- Lässt sich eine erledigte Zeile nicht eindeutig einer `### Etappe <n>` zuordnen — Nummer
  fehlt, Titel weicht ab, Datum fehlt —, steht der ganze Task mit `[unklar]` in der Vorlage,
  mit der Zeile, die nicht passt; der Nutzer entscheidet.
- Die Tabelle bleibt als Beleg stehen.

**b) Verweise suchen.** Gesucht wird die Zeichenfolge `k-playbook-local/tasks/` in:

- allen Dateien des Projekts auf der Platte, nach der Verweissuche oben, ohne
  `k-playbook-local/tasks/` selbst;
- deinen offenen Tasks und aktiven Workbench-Themen aus dem Briefing (`work.tasks`,
  `work.workbench`), gelesen mit `k_playbook_doc_read`;
- den Task-Dateien, die nach a) umziehen — ihre Treffer werden nach dem Anlegen in den neuen
  Dokumenten ersetzt (d).

Vorschlag je Stelle: den Pfad durch den `keph:`-Verweis des Ziels ersetzen, das die Datei
nach a) bekommt. Was ein Verweis ist, was als Beleg unberührt bleibt — Review-Logs,
`## Ausführung` und was danach folgt, Diffs, erledigte Tasks — und wie nachgezogen wird,
steht in `k-playbook:/instructions/on-demand/verweise-nachziehen.md`; 3.3 wiederholt es
nicht. Command-eigen ist nur der Suchbegriff `k-playbook-local/tasks/` und sein Ersatz durch
den `keph:`-Verweis. Der Nutzer kann Zeilen ausnehmen — etwa Geschichte in
`project:/docs/migration.md`, die den alten Ort beschreibt und ihn weiter nennen darf.

**c) Vorlegen.** Lege gebündelt vor, eine Zeile je Datei bzw. Stelle, in dieser Reihenfolge:
offene Tasks, erledigte, `old/`, Verweise:

```text
Tasks von der Platte
  tasks/077-skills-sprechend-benennen.md   [verschieben]  → keph:/user/kamran/tasks/077-skills-sprechend-benennen.md, Ziel frei
  tasks/078-wissensablage-append.md        [verschieben]  → keph:/user/kamran/tasks/078-wissensablage-append.md, Ziel frei; Fortschritt: Etappen 1–3, 5 erledigt (4 Zeilen), 4 offen
  tasks/079-beispiel.md                    [unklar]       Fortschritt: Zeile „3 — Doku“ passt zu keiner ### Etappe 3
  tasks/082-status-mcp-server.md           [verschieben]  → keph:/user/anna/tasks/082-status-mcp-server.md (User anna); gehört danach kamran (created_by), anna braucht supersede
  tasks/done/104-doc-operationen.md        [verschieben]  → keph:/user/kamran/tasks/done/104-doc-operationen.md, Ziel vorhanden und gleich: nur löschen
  tasks/done/103-health.md                 [unklar]       Ziel vorhanden und verschieden; beide zeigen
  tasks/old/012-alt.md                     [unklar]       nach done/ oder verwerfen
  tasks/old/013-binaer.pdf                 [bleibt]       kein Text
  docs/migration.md:41                     [ersetzen]     k-playbook-local/tasks/097-… → keph:/user/kamran/tasks/done/097-…
  keph:/user/kamran/workbench/005-….md     [ersetzen]     zwei Stellen
```

- **verschieben** — Kephalaion `create` unter dem Zielnamen, danach die Plattendatei löschen.
- **ersetzen** — ein Verweis in einer Datei oder einem Dokument.
- **löschen** — die Plattendatei entfällt.
- **bleibt** — nicht Teil dieser Umstellung.
- **unklar** — du kannst es nicht entscheiden; der Nutzer entscheidet, je Zeile.

Eine Zeile mit einem anderen User als `context.kephalaion.user` trägt den Hinweis, dass danach
zwei Prüfungen gegeneinander stehen:

- **Kephalaion** prüft den Urheber: Das Dokument legst du an, also ist `created_by` dein User.
  Ändern, anhängen, umbenennen (nach `done/` verschieben) darf mit dem Recht `write` nur der
  Urheber; jeder andere, auch der Ziel-User, braucht in der Collection das Recht `supersede`.
- **k-playbook** prüft den Pfad: Die Dokument-Werkzeuge (`k_playbook_doc_append`,
  `_replace`, `_done` …) ändern nur Dokumente unter dem eigenen `user/<user>/`, gleich wer sie
  angelegt hat; für dich ist das Dokument danach `foreign_user`.

Der Ziel-User kann den Task deshalb nur weiterführen, wenn er `supersede` hat; du kannst es
über k-playbook gar nicht. Was 3.3 an einem solchen Dokument nach dem Anlegen noch tun würde —
Fortschrittszeilen, ersetzte Verweise —, steht deshalb schon im Inhalt, mit dem du es anlegst
(d). Alternative: die Datei liegen lassen, und der Ziel-User zieht sie mit seinem eigenen
`/k-upgrade` um. Beides steht in der Zeile; der Nutzer wählt.

Warte auf die Antwort. Der Nutzer bestätigt die Vorlage ganz oder nimmt einzelne Zeilen aus.

**d) Ausführen, was bestätigt ist.** In der Reihenfolge der Vorlage:

1. **verschieben:** Kephalaion `create` mit dem ganzen Inhalt der Datei — Frontmatter
   ergänzt, sonst Byte für Byte — unter dem Zielnamen (ohne führenden `/`),
   `collection` = `context.kephalaion.address`. Bei einem gleichen Dokument am Ziel entfällt
   das Schreiben, bei einem verschiedenen gilt die Entscheidung des Nutzers (`write` mit
   `base_revision` aus dem `read`). Erst nach einem bestätigten Schreiben — die Antwort nennt
   Revision und Größe — löschst du die Plattendatei. Bei einem Ziel unter einem anderen User
   gehören die Fortschrittszeilen und die bestätigten Ersetzungen dieses Tasks schon in den
   Inhalt des `create`: die Ersetzungen an ihrer Stelle, die Fortschrittszeilen am Ende, je
   durch eine Leerzeile getrennt, wie `k_playbook_doc_append` sie setzen würde. Schritte 2
   und 3 entfallen dann für dieses Dokument.
2. **Fortschritt:** Je Task mit Fortschrittszeilen (a), unter dem eigenen User, mit
   `k_playbook_doc_append`, eine Zeile je Aufruf, in der Reihenfolge der Etappen, nach dem
   Schreibweg in `k-playbook:/instructions/on-demand/tasks.md`, „Lesen und schreiben“: Das
   erste `since_revision` ist die Revision aus der Antwort des Schreibens in Schritt 1
   (`create` bzw. `write`), jedes weitere die aus dem letzten Anhängen; vorher die Zeilen
   `^<!-- k-task-run: etappe [0-9]+ erledigt ` zählen („Zählen vor dem Anhängen“). Stand das
   Dokument am Ziel schon gleich da, kommt die erste Revision aus diesem Zählen, und eine
   Zeile, die dort schon steht, wird nicht noch einmal angehängt.
3. **ersetzen:** jeden bestätigten Verweis durch den `keph:`-Verweis des Ziels; in einer
   Plattendatei mit dem Edit-Werkzeug, in einem Task mit `k_playbook_doc_replace` mit
   `since_revision` — aus dem vorherigen `k_playbook_doc_read` dieses Tasks, bei einem eben
   umgezogenen Task aus dem letzten Schreiben in Schritt 1 oder 2 —, in einem
   Workbench-Thema mit Kephalaion `write` und `base_revision` aus dem `read`. Einen Anker
   (`#…`) hinter dem Pfad lässt du weg. Liefert ein Lesen oder Zählen direkt nach dem eigenen
   Schreiben eine ältere Revision, ist das der Replica-Nachlauf aus
   `k-playbook:/instructions/on-demand/tasks.md`, kein fremder Schreiber.

Ein Schritt, der fehlschlägt, lässt seine Datei liegen; der nächste Lauf nimmt sie wieder
auf. Ihr Inhalt bleibt außerdem in der Git-Geschichte.

Die READMEs von `tasks/` und `tasks/done/` und die Verzeichnisse selbst fasst diese
Umstellung nicht an; darüber entscheidet 3.4.

**e) Melden.** Je Datei eine Zeile: der `keph:`-Verweis, unter dem sie jetzt liegt, oder der
Grund, warum sie liegen blieb, dazu die Zahl der Fortschrittszeilen, die der Task bekommen
hat; je geänderte Stelle `pfad:zeile` bzw. der Verweis des Dokuments. Prüfe das Ergebnis mit `k_playbook_doc_list` (Sorte `tasks`, `user: "*"`,
`done: true`) und nenne, was dort jetzt liegt.

### 3.4 — Aufräumen

Das Einrichten legt neun Verzeichnisse unter `<context.local.dir>/` nicht mehr an und zählt
sie nicht mehr als fehlend: Ihr Inhalt liegt in der Projekt-Collection oder zieht als
Nächstes dorthin. In einem bestehenden Projekt liegen sie noch, oft nur mit ihrer README.
Diese Umstellung löscht jedes davon, das leer ist, `data/` mit der Ausnahme unten für die
Übernahmereste der Todos. Sie läuft nach 3.1 bis 3.3, damit zählt,
was dort umgezogen ist. Die Liste gehört zu dieser Fassung des Commands; das Programm nennt
sie nicht:

- `docs/`
- `tasks/`
- `tasks/done/`
- `data/`
- `material/`
- `inbox/`
- `queue/`
- `knowledge/`
- `cache/`

**Anlass:** Mindestens eines dieser Verzeichnisse liegt unter `<context.local.dir>/`. Braucht
die Collection nicht.

**Vorbedingung:** Schritt 2 hat das Programm als aktuell gemeldet — in seiner Ausgabe steht
die Zeile `Programm-Stand: nichts zu tun — …`. Fehlt sie, nennst du in einer Zeile, dass das
Aufräumen auf den Binärwechsel wartet: Ein älteres Programm legt die Verzeichnisse beim Knopf
„Anlegen“ der Oberfläche wieder an. Dann bleibt alles liegen, und du gehst zu Schritt 4.

**Leer** heißt: Im Verzeichnis liegt, auch in Unterverzeichnissen, nichts außer

- Dateien mit dem Namen `README.md`,
- Dateien mit dem Namen `.gitignore`, deren Inhalt genau der ist, den k-playbook verwaltet:
  die drei Zeilen `*`, `!.gitignore` und `!README.md` in dieser Reihenfolge — Leerzeilen und
  Leerraum am Zeilenrand zählen nicht mit,
- Unterverzeichnissen, die ihrerseits in diesem Sinn leer sind, etwa `tasks/done/` mit seiner
  README oder ein `inbox/<quelle>/` ohne Inhalt.

**Ausnahme für `data/`.** Direkt unter `data/`, nicht in einem Unterverzeichnis, zählen auch
die Übernahmereste von `k-playbook todo migrate` wie eine `README.md`: die Datei
`todos.json.migriert` und jede Datei `todos.json.nachzuegler-<datum>` oder
`todos.json.nachzuegler-<datum>-<n>` (`<datum>` als `JJJJ-MM-TT`). Jede zählt für sich, auch
ohne die andere; ein `data/` mit `README.md` und nur Nachzügler-Dateien ist also ebenfalls
leer. Diese Dateien liest niemand mehr, und `k-playbook todo migrate` erkennt einen Nachzügler
auch ohne sie. Liegt dagegen `data/todos.json` selbst noch da, ist sie noch nicht übernommen:
`data/` bleibt, und seine Zeile nennt `k-playbook todo migrate`.

Alles andere heißt „liegt anderes darin“: jede weitere Datei, eine `.gitignore` mit anderem
Inhalt, ein anderes Dotfile, ein Symlink. Auch ein abgeleiteter Index wie
`cache/knowledge/index.json` zählt dazu, obwohl er jederzeit neu entsteht.

**a) Bestand erheben.** Lies je Verzeichnis, das liegt, alles darin, rekursiv und mit
versteckten Dateien — ein Verzeichnislisting ohne Dotfiles reicht nicht, und einem Symlink
folgst du nicht. Danach ist jedes Verzeichnis leer oder nicht. Liegt `tasks/` leer, gilt das
auch für `tasks/done/`; beide stehen dann in einer Zeile `tasks/`.

**b) Vorlegen.** Lege gebündelt vor, eine Zeile je Verzeichnis in der Reihenfolge der Liste:
für ein leeres der Vorschlag, es zu löschen, mit dem, was darin liegt; für jedes andere die
Zeile `bleibt: <was noch liegt>`. Genannt werden Pfade relativ zum Verzeichnis, eine README
eingeschlossen, höchstens fünf; liegt mehr darin, dazu die Gesamtzahl der Dateien. Bleibt
`data/`, weil `todos.json` darin liegt, endet seine Zeile mit `— erst k-playbook todo migrate`.

```text
Aufräumen in k-playbook-local/
  docs/        [löschen]  nur README.md
  tasks/       [löschen]  nur README.md, done/README.md
  data/        [löschen]  nur README.md, todos.json.migriert
  material/    bleibt: README.md, notizen.md
  inbox/       bleibt: README.md, .gitignore, chat/2026-09-12.md, chat/2026-09-13.md, scan/seite.pdf (45 Dateien)
  queue/       [löschen]  nur README.md
  cache/       bleibt: README.md, .gitignore, knowledge/index.json
```

Liegt in `data/` noch eine `todos.json`, lautet seine Zeile etwa:

```text
  data/        bleibt: README.md, todos.json — erst k-playbook todo migrate
```

Ein Verzeichnis, das nicht liegt, erscheint nicht. Warte auf die Antwort. Der Nutzer bestätigt
die Vorlage ganz oder nimmt einzelne Zeilen aus.

**c) Ausführen, was bestätigt ist.** Prüfe je bestätigter Zeile unmittelbar vor dem Löschen
noch einmal, ob das Verzeichnis leer ist, für `data/` mit derselben Ausnahme; zwischen Vorlage
und Antwort kann etwas hinzugekommen sein, etwa eine neue `data/todos.json`. Ist es leer, löschst du es samt Inhalt im Dateisystem. Ist es das nicht
mehr, bleibt es und wird mit dem Grund gemeldet. Committet wird nicht: Versionierte READMEs
stehen danach als gelöscht in `git status`.

**d) Melden.** Je Verzeichnis eine Zeile: gelöscht, oder `bleibt: …` mit dem Grund — liegt
anderes darin, vom Nutzer ausgenommen oder beim zweiten Prüfen nicht mehr leer.

Die Umstellung ist wiederholbar: Ein Verzeichnis, das bleibt, schlägt der nächste Lauf erneut
vor, sobald es leer ist — etwa `material/` nach 3.1 oder `tasks/` nach 3.3. Die
Wissenswerkzeuge (`k-playbook knowledge`, MCP-Werkzeuge `k_playbook_knowledge_*`) legen
`inbox/`, `queue/`, `knowledge/` und `cache/` bis zu ihrer Umstellung in die Collection beim
Schreiben selbst wieder an, ohne README; das ist kein Fehler.

## Schritt 4 — Schluss

Nenne in einer Zeile je Umstellung, ob sie etwas getan hat, und was liegen geblieben ist; für
das Aufräumen die gelöschten Verzeichnisse und jede Zeile `bleibt: …`, oder dass es auf den
Binärwechsel wartet.
