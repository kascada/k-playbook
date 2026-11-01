# k-upgrade

Zieht Programm und Projekt auf den aktuellen Stand von k-playbook. Zuerst das, was das
Programm allein kann: das Binary, wenn das laufende älter ist als die Installation, danach
`AGENTS.md` (Anstoß, abgelöste Blöcke), `CLAUDE.md`, die Verlinkung der Commands und Skills
und die MCP-Einträge. Das tut der Unterbefehl `k-playbook upgrade`, und den führt die Hülle
dieses Commands schon beim Aufruf aus — vor dem ersten Schritt, damit das Briefing danach
den neuen Stand sieht und nicht das alte Programm die Verlinkung nach seinen Regeln
nachzieht. Danach die Umstellungen, die eine Beurteilung brauchen und deshalb die Sitzung
macht, nicht das Programm. Der Command nimmt kein Argument.

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

- allen von Git erfassten Dateien des Projekts (`git grep -n "material/befunde/"` in
  `context.project.dir`); ohne Git mit `grep -rn` über `context.project.dir`, ohne `.git/`;
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

Für jede Datei mit `entfällt` entfernst du die Verweise wie beschrieben.

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
`README.md` und das Verzeichnis selbst bleiben. Eine Datei, bei der etwas fehlgeschlagen ist,
bleibt liegen, und der nächste Lauf nimmt sie wieder auf.

Steht in `<context.local.dir>/material/README.md` noch der Absatz, der mit „Geschrieben wird
hier nur an einer einzigen Stelle: nach befunde/“ beginnt, ersetze ihn — nur ihn — durch
diesen:

```text
Hierher schreibt kein Command. Befunde aus Analyse und Fehlersuche lagen früher
unter befunde/; heute ist jeder Befund ein eigenes Dokument in der
Projekt-Collection unter queue/findings/, die Form steht in
k-playbook:/instructions/on-demand/befunde.md. Ein vorhandenes befunde/ leert
/k-upgrade: was noch zu tun ist, wird ein Workbench-Thema, was nur nützlich ist,
ein Befund in der Collection, der Rest entfällt. Alles andere in diesem
Verzeichnis bleibt unangetastet.
```

Das Einrichten schreibt diese README nur, wenn sie fehlt; ohne diesen Schritt bliebe in
einem bestehenden Projekt der alte Text stehen.

**g) Melden.** Je Datei eine Zeile: was aus ihr wurde (Workbench-Verweis, Zahl der Befunde,
verworfen, liegen geblieben), dazu jede geänderte Verweisstelle als `pfad:zeile`.

### 3.2 — Doku aus `k-playbook-local/docs/`

Die Projektdoku liegt heute in der Collection des Projekts unter `keph:/docs/`; das Briefing
nennt diesen Einstieg unter `Projektdoku`, und `k-playbook:/commands/_shared/docs.md` sagt,
was darunter gilt. Früher lag sie unter `<context.local.dir>/docs/`, mit einem Index
`docs/README.md` von `/k-docs-index` und Verweisen darauf in `AGENTS.md` und
`opencode.json`. Diese Umstellung bringt die Doku je Bereich nach Kephalaion oder räumt
sie ab — nichts verschwindet still, und ein Bereich, dessen Erzeuger noch auf die Platte
schreibt, bleibt liegen.

**Anlass:** Unter `<context.local.dir>/docs/` liegt mindestens eine Datei aus den Bereichen
der Tabelle unten, oder `AGENTS.md` bzw. `opencode.json[c]` nennen
`k-playbook-local/docs/README.md`. Braucht die Collection.

Die Bereiche und ihr Ziel. Wird ein weiterer Erzeuger auf Kephalaion umgestellt, ändert
dessen Task seine Zeile:

| Bereich auf der Platte | Ziel | Vorgehen |
|---|---|---|
| `versions/` | `keph:/docs/generated/versions/` | Erzeuger (`/k-doc-inventory`, `k-playbook inventory`) schreibt schon nach Kephalaion: Plattendatei löschen; fehlt das Ziel, Erzeuger neu laufen lassen |
| `code/` | `keph:/docs/generated/code/` | Erzeuger `/k-docs-code` schreibt noch auf die Platte: nicht anfassen |
| `libs/` | `keph:/docs/generated/libs/` | Erzeuger `/k-docs-tools` schreibt noch auf die Platte: nicht anfassen |
| `manual/` | `keph:/docs/manual/`, Pfad darunter gleich | von Hand geschrieben: verschieben; `manual/README.md` nicht verschieben (`README.md` erzeugt Kephalaion), sondern löschen |
| `extracted/` und lose `.md` direkt unter `docs/` außer `README.md` | Vorschlag `keph:/docs/manual/<datei>` | je Datei verschieben oder verwerfen, der Nutzer entscheidet |
| `README.md` | entfällt | der alte Index von `/k-docs-index`: löschen, sobald seine Einträge zu `manual/`, `versions/` und losen Dateien abgeräumt sind — auch wenn `code/` oder `libs/` noch liegen; sonst bliebe ein veralteter Index als stiller Rückfall |

`docs/` selbst bleibt: `code/` und `libs/` schreiben noch dorthin. Legt das Einrichten die
README von `docs/` neu an, ist das die Struktur-README ohne Index, kein Rückfall.

**a) Bestand erheben.** Lies, was unter `<context.local.dir>/docs/` liegt, je Bereich der
Tabelle. Lies dazu `keph:/docs/` mit Kephalaion `list` (`recursive`, `frontmatter`),
Collection = `context.kephalaion.address`. Für jede Datei, die verschoben werden soll,
prüfst du vorher:

- Liegt unter dem Zielnamen schon ein Dokument, vergleiche beide. Gleich → nur die
  Plattendatei löschen. Verschieden → beide zeigen; der Nutzer entscheidet, welche gilt.
- Ist die Datei kein Text oder größer als 1 MiB, bleibt sie liegen und wird gemeldet.
- Zeigt ein Link in der Datei aus `manual/` hinaus (etwa auf `../versions/…` oder in den
  Code), merkst du ihn für die Meldung vor; umgeschrieben wird er nicht.

Für einen umgestellten Erzeuger (heute `versions/`): fehlt sein Ziel in Kephalaion, erinnerst
du daran, ihn neu laufen zu lassen. Beim Inventar bietest du an, `k-playbook inventory`
gleich auszuführen; der Lauf ist deterministisch.

`docs/README.md` ist der alte Index, wenn sein Frontmatter oder sein Inhalt ihn als Werk
von `/k-docs-index` ausweist — die Übersicht je Herkunft mit Tabellen, der
Stichwort-Index, „Häufige Fragen → direkter Sprung“. Trägt die Datei stattdessen nur den
Text der Struktur-README aus dem Einrichten, ist nichts zu tun. Ist sie von Hand geschrieben
und beides nicht, behandelst du sie wie eine lose Datei.

**b) Alte Einstiegsverweise suchen.** Gesucht wird die Zeichenfolge
`k-playbook-local/docs/README.md` in `AGENTS.md` sowie `opencode.json` und
`opencode.jsonc` in `context.project.dir`, dazu in `opencode.json[c]` ein Eintrag
`references.docs`, dessen `path` auf `./k-playbook-local/docs` zeigt. Der Einstieg kommt
aus dem Briefing; das Programm schreibt diesen Eintrag nicht mehr neu. Vorschlag ist, den
Eintrag bzw. den Absatz zu entfernen. Steht im Absatz in `AGENTS.md` mehr als der Verweis,
schlägst du vor, nur den Verweis zu entfernen und den Rest stehen zu lassen.

**c) Vorlegen.** Lege gebündelt vor, eine Zeile je Datei bzw. Bereich mit Vorschlag und
Grund, in dieser Reihenfolge: umgestellte Erzeuger, `manual/`, lose Dateien und
`extracted/`, der alte Index, die Verweise:

```text
Doku aus k-playbook-local/docs/
  versions/inventory.md        [löschen]      liegt als keph:/docs/generated/versions/inventory.md, Stand 2026-10-25
  manual/release.md            [verschieben]  → keph:/docs/manual/release.md, Ziel frei
  manual/installation.md       [verschieben]  → keph:/docs/manual/installation.md, Ziel vorhanden und gleich: nur löschen
  manual/README.md             [löschen]      README.md erzeugt Kephalaion
  notizen.md                   [unklar]       lose Datei ohne Erzeuger; Vorschlag keph:/docs/manual/notizen.md
  README.md                    [löschen]      alter Index von /k-docs-index, Einträge abgeräumt
  code/, libs/                 [bleibt]       Erzeuger schreiben noch auf die Platte
  opencode.json:19             [entfernen]    references.docs mit dem alten Einstieg
  AGENTS.md:12                 [entfernen]    Verweis auf k-playbook-local/docs/README.md
```

- **löschen** — die Plattendatei entfällt, ihr Inhalt liegt in Kephalaion oder ist erzeugt.
- **verschieben** — Kephalaion `create` unter dem Zielnamen, danach die Plattendatei löschen.
- **entfernen** — ein Verweis in `AGENTS.md` oder `opencode.json[c]`.
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
   zu `manual/`, `versions/` und losen Dateien ausgeführt sind; blieb eine davon liegen,
   bleibt auch er und wird beim nächsten Lauf erneut vorgeschlagen.
3. **entfernen:** in `opencode.json[c]` den Eintrag `references.docs` (ein leeres
   `references` dazu); in `AGENTS.md` den Verweis bzw. Absatz. Die übrigen Einträge und die
   Formatierung der Datei bleiben, wie sie sind.

Ein Schritt, der fehlschlägt, lässt seine Datei liegen; der nächste Lauf nimmt sie wieder
auf. Ihr Inhalt bleibt außerdem in der Git-Geschichte.

**e) Aufräumen.** Ein Verzeichnis unter `docs/`, das danach leer ist (`manual/`, `versions/`,
`extracted/`), löschst du mit. `docs/` selbst und seine README bleiben.

**f) Melden.** Je Datei eine Zeile: was aus ihr wurde (`keph:`-Verweis, gelöscht, liegen
geblieben und warum), dazu je vorgemerktem Link aus `manual/` hinaus `datei:zeile` und je
entferntem Verweis `pfad:zeile`. Prüfe das Ergebnis mit Kephalaion `list` auf `docs/` und
nenne, was dort jetzt liegt.

Die Umstellung ist wiederholbar: `/k-docs-index` (auch über `/k-docs` → `index`) und der
Skill `ai-session-memory` schreiben bis zu ihrer eigenen Umstellung den alten Index und
die Verweise in `AGENTS.md`/`opencode.json` neu; der nächste Lauf schlägt das Abräumen dann
erneut vor.

### 3.3 — Tasks von der Platte

Tasks lagen früher als Dateien unter `<context.local.dir>/tasks/`, erledigte unter
`tasks/done/`, ausgemusterte unter `tasks/old/`. Heute ist ein Task ein Dokument in der
Projekt-Collection unter `keph:/user/<user>/tasks/`, erledigt unter `done/`; die Form steht
in `k-playbook:/instructions/on-demand/tasks.md`, und `/k-task-create`, `/k-task-refine`,
`/k-task-run` und `/k-remediation` lesen und schreiben nur noch dort. Diese Umstellung
verschiebt die Dateien: gleicher Name, gleiche Nummer, beim richtigen User.

**Anlass:** In `<context.local.dir>/tasks/`, `tasks/done/` oder `tasks/old/` liegt eine
`.md`-Datei außer `README.md`. Braucht die Collection.

Lies vorher `k-playbook:/instructions/on-demand/tasks.md`.

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

**Frontmatter.** Ein Task trägt `title` im Frontmatter (`on-demand/tasks.md`, „Kopf“). Fehlt
es der Datei, bildest du es aus der ersten Überschrift `# Task <n> — <Titel>`: `title` ist
`<Titel>`, ohne „Task <n> —“. Fehlt auch die, ist `title` der Dateiname ohne Nummer und
`.md`, mit Leerzeichen statt Bindestrichen. Der übrige Inhalt bleibt Byte für Byte; das
Frontmatter kommt davor. Trägt die Datei schon ein Frontmatter mit `title`, bleibt sie ganz
unverändert.

**b) Verweise suchen.** Gesucht wird die Zeichenfolge `k-playbook-local/tasks/` in:

- allen von Git erfassten Dateien des Projekts (`git grep -n "k-playbook-local/tasks/"` in
  `context.project.dir`; ohne Git mit `grep -rn` über `context.project.dir`, ohne `.git/`),
  ohne `k-playbook-local/tasks/` selbst und ohne die Installation unter
  `context.playbook.dir`;
- deinen offenen Tasks und aktiven Workbench-Themen aus dem Briefing (`work.tasks`,
  `work.workbench`), gelesen mit `k_playbook_doc_read`.

Vorschlag je Stelle: den Pfad durch den `keph:`-Verweis des Ziels ersetzen, das die Datei
nach a) bekommt. Der Nutzer kann Zeilen ausnehmen — etwa Geschichte in `docs/migration.md`,
die den alten Ort beschreibt und ihn weiter nennen darf.

**c) Vorlegen.** Lege gebündelt vor, eine Zeile je Datei bzw. Stelle, in dieser Reihenfolge:
offene Tasks, erledigte, `old/`, Verweise, Aufräumen:

```text
Tasks von der Platte
  tasks/077-skills-sprechend-benennen.md   [verschieben]  → keph:/user/kamran/tasks/077-skills-sprechend-benennen.md, Ziel frei
  tasks/082-status-mcp-server.md           [verschieben]  → keph:/user/anna/tasks/082-status-mcp-server.md (User anna); gehört danach kamran, anna braucht supersede
  tasks/done/104-doc-operationen.md        [verschieben]  → keph:/user/kamran/tasks/done/104-doc-operationen.md, Ziel vorhanden und gleich: nur löschen
  tasks/done/103-health.md                 [unklar]       Ziel vorhanden und verschieden; beide zeigen
  tasks/old/012-alt.md                     [unklar]       nach done/ oder verwerfen
  tasks/old/013-binaer.pdf                 [bleibt]       kein Text
  docs/migration.md:41                     [ersetzen]     k-playbook-local/tasks/097-… → keph:/user/kamran/tasks/done/097-…
  keph:/user/kamran/workbench/005-….md     [ersetzen]     zwei Stellen
  tasks/README.md                          [ersetzen]     neuer Text (Tasks liegen in der Collection)
  tasks/old/                               [löschen]      wenn danach leer
```

- **verschieben** — Kephalaion `create` unter dem Zielnamen, danach die Plattendatei löschen.
- **ersetzen** — ein Verweis in einer Datei oder einem Dokument; bei `tasks/README.md` der
  ganze Text.
- **löschen** — die Plattendatei bzw. das leere Verzeichnis entfällt.
- **bleibt** — nicht Teil dieser Umstellung.
- **unklar** — du kannst es nicht entscheiden; der Nutzer entscheidet, je Zeile.

Eine Zeile mit einem anderen User als `context.kephalaion.user` trägt den Hinweis: Das
Dokument legst du an, also gehört es danach dir (`created_by`), nicht dem Ziel-User; der kann
es nur anhängen oder nach `done/` verschieben, wenn er in der Collection das Recht
`supersede` hat. Alternative: die Datei liegen lassen, und der Ziel-User zieht sie mit seinem
eigenen `/k-upgrade` um. Beides steht in der Zeile; der Nutzer wählt.

Warte auf die Antwort. Der Nutzer bestätigt die Vorlage ganz oder nimmt einzelne Zeilen aus.

**d) Ausführen, was bestätigt ist.** In der Reihenfolge der Vorlage:

1. **verschieben:** Kephalaion `create` mit dem ganzen Inhalt der Datei — Frontmatter
   ergänzt, sonst Byte für Byte — unter dem Zielnamen (ohne führenden `/`),
   `collection` = `context.kephalaion.address`. Bei einem gleichen Dokument am Ziel entfällt
   das Schreiben, bei einem verschiedenen gilt die Entscheidung des Nutzers (`write` mit
   `base_revision` aus dem `read`). Erst nach einem bestätigten Schreiben — die Antwort nennt
   Revision und Größe — löschst du die Plattendatei.
2. **ersetzen:** jeden bestätigten Verweis durch den `keph:`-Verweis des Ziels; in einer
   Plattendatei mit dem Edit-Werkzeug, in einem Task oder Workbench-Thema mit
   `k_playbook_doc_replace` (Task) bzw. Kephalaion `write` mit `base_revision` (Workbench).
   Einen Anker (`#…`) hinter dem Pfad lässt du weg.

Ein Schritt, der fehlschlägt, lässt seine Datei liegen; der nächste Lauf nimmt sie wieder
auf. Ihr Inhalt bleibt außerdem in der Git-Geschichte.

**e) Aufräumen** — nur, wenn die Zeilen in c) bestätigt sind:

- `tasks/README.md` bekommt den Text, den das Einrichten heute schreibt
  (`LocalStructure()` in `installer/internal/project/local.go`): Tasks liegen in der
  Projekt-Collection, kein Command liest oder schreibt hier noch Tasks, das Verzeichnis
  bleibt bis zur Umstellung der Oberfläche. Das Einrichten schreibt diese README nur, wenn
  sie fehlt; ohne diesen Schritt bliebe in einem bestehenden Projekt der alte Text stehen.
- `tasks/` und `tasks/done/` bleiben: Die Seite Tasks der Oberfläche zeigt bis zu ihrer
  Umstellung noch dieses Verzeichnis, das danach leer ist. Ein leeres `tasks/old/` wird
  gelöscht.

**f) Melden.** Je Datei eine Zeile: der `keph:`-Verweis, unter dem sie jetzt liegt, oder der
Grund, warum sie liegen blieb; je geänderte Stelle `pfad:zeile` bzw. der Verweis des
Dokuments. Prüfe das Ergebnis mit `k_playbook_doc_list` (Sorte `tasks`, `user: "*"`,
`done: true`) und nenne, was dort jetzt liegt.

## Schritt 4 — Schluss

Nenne in einer Zeile je Umstellung, ob sie etwas getan hat, und was liegen geblieben ist.
