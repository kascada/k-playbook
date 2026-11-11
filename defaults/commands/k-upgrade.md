# k-upgrade

Zieht Kephalaion, Programm und Projekt auf den aktuellen Stand von k-playbook. Zuerst das, was
das Programm allein kann, in dieser Reihenfolge: Kephalaion — `kephalaion upgrade --auto`
aktualisiert es und startet seinen Node neu, soweit Kephalaion das selbst für gefahrlos hält,
sonst steht in der Ausgabe, was zu tun ist —, dann das Binary, wenn das laufende älter ist als
die Installation, dann `AGENTS.md` (Anstoß, abgelöste Blöcke), `CLAUDE.md`, die Verlinkung der
Commands und Skills und die MCP-Einträge. Das tut der Unterbefehl `k-playbook upgrade`, und den führt die Hülle
dieses Commands schon beim Aufruf aus — vor dem ersten Schritt, damit das Briefing danach
den neuen Stand sieht und nicht das alte Programm die Verlinkung nach seinen Regeln
nachzieht. Danach, falls das Projekt eines mitbringt, sein eigenes Umbauskript
`scripts/k-upgrade-umbau.sh`. Danach die Umstellungen, die eine Beurteilung brauchen und deshalb die Sitzung
macht, nicht das Programm, zuletzt das Aufräumen der Verzeichnisse, die das Einrichten nicht
mehr anlegt, und der alten `K-PLAYBOOK.local.yaml`. Der Command nimmt kein Argument.

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

    K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade

und gib diese Ausgabe nach derselben Regel wieder. Nenne dazu in einer Zeile, dass die Hülle
den Aufruf nicht ausgeführt hat. Hülle und Ersatzaufruf setzen `K_PLAYBOOK_VIA_COMMAND=1`,
damit der Hinweis entfällt, den `k-playbook upgrade` im Terminal nach einer neuen Version
gibt — `/k-upgrade` aufzurufen —; hier läuft `/k-upgrade` schon.

**Projekteigener Umbau.** Liegt im Projekt die Datei
`<context.project.dir>/scripts/k-upgrade-umbau.sh`, führst du sie direkt danach aus, mit
`context.project.dir` als Arbeitsverzeichnis:

    bash scripts/k-upgrade-umbau.sh

Das Skript gehört dem Projekt und räumt vor den Umstellungen ab, was sie sonst einzeln prüfen
müssten, etwa ganze Verzeichnisse. Es läuft deshalb vor Schritt 3 und ohne Rückfrage; die
Bestätigungsregel aus Schritt 3 gilt für das Skript nicht. Gib seine Ausgabe nach derselben
Regel wie oben unverändert wieder. Endet es mit einem Exit-Code ungleich 0, nenne den Code
und frag, ob Schritt 3 trotzdem laufen soll; die Umstellungen auf einem halb umgebauten
Projekt laufen zu lassen, entscheidet der Nutzer. Fehlt die Datei, meldest du nichts dazu.
Schritt 3 stellt seinen Bestand erst danach fest, also nach dem, was das Skript gelöscht
oder verschoben hat.

Steht in der Ausgabe, dass die Sitzung neu zu starten ist — weil das Programm gewechselt hat
oder weil Kephalaion aktualisiert bzw. sein Node neu gestartet wurde —, endet der Command hier
mit genau diesem Hinweis: Der laufende MCP-Server behält das alte Binary, bis der Assistent ihn
neu startet, und die Verbindung zu einem neu gestarteten Node ist danach nicht verlässlich.
Füge hinzu, dass die Umstellungen aus Schritt 3 danach mit einem zweiten Lauf von `/k-upgrade`
folgen; sie brauchen die Werkzeuge des neuen Programms und einen erreichbaren Node.

## Schritt 3 — Umstellungen

Jede Umstellung prüft zuerst ihren Anlass. Liegt er nicht vor, gehst du ohne Meldung zur
nächsten; eine erledigte Umstellung wird so beim nächsten Lauf von selbst übersprungen.
`/k-upgrade` läuft oft, und eine Zeile „nichts zu tun“ je Umstellung sagt dann nichts mehr.
Gemeldet wird nur, was zu tun ist: was eine Umstellung vorlegt, ausführt oder liegen lässt,
dazu jede Warnung, etwa dass sie auf den Binärwechsel wartet. Liegt bei keiner Umstellung ein
Anlass vor, steht für Schritt 3 genau eine Zeile: `Umstellungen: alles erledigt.`

Für alle Umstellungen gilt:

- Gelöscht, verschoben oder überschrieben wird erst nach Bestätigung. Lege je Umstellung
  gebündelt vor, was du vorhast, und warte auf die Antwort. Ausgenommen sind zwei Fälle, und
  nur sie: `cache/`, `inbox/` und `queue/` aus 3.0 a), die 3.0 ohne Inhaltsprüfung löscht, und
  die leeren Verzeichnisse aus 3.4. Beide löschen ohne Vorlage und ohne auf eine Antwort zu
  warten.
- Bist du bei einem Eintrag unsicher, sag das in der Vorlage ausdrücklich, statt still zu
  entscheiden; in 3.4, das nichts vorlegt, steht es in der Meldung, ohne neue Rückfrage.
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

### 3.0 — Wissensablage auf der Platte

Die Wissensablage lag früher auf der Platte unter `<context.local.dir>/`: verdichtetes Wissen
in `knowledge/`, Rohmaterial im Eingang `inbox/`, die Warteschlange `queue/` und der Suchindex
unter `cache/`, geschrieben über das Subkommando `k-playbook knowledge` und die MCP-Werkzeuge
`k_playbook_knowledge_*`. Beides gibt es nicht mehr: Projektwissen liegt in `keph:/docs/`,
Befunde liegen in `keph:/queue/findings/`, und Rohmaterial hat keinen Nachfolger. Diese
Umstellung räumt die vier Verzeichnisse ab; sie läuft als erste.

**Anlass:** Unter `<context.local.dir>/` liegt `cache/`, `inbox/`, `queue/` oder `knowledge/`.
Die Collection braucht nur b).

**Vorbedingung:** wie in 3.4 — Schritt 2 hat das Programm als aktuell gemeldet, in seiner
Ausgabe steht die Zeile `Programm-Stand: nichts zu tun — …`. Fehlt sie, nennst du in einer
Zeile, dass 3.0 auf den Binärwechsel wartet: Ein älteres Programm schreibt mit seinen
Wissenswerkzeugen wieder in diese Verzeichnisse. Dann bleibt alles liegen, und du gehst zu 3.1.

**a) `cache/`, `inbox/` und `queue/` löschen.** Ohne Inhaltsprüfung und ohne Rückfrage — die
erste Ausnahme von der Bestätigungsregel oben: `cache/` ist abgeleitet, `inbox/` ist
Rohmaterial ohne Leser, und die Einträge in `queue/` zeigen auf `inbox/`. Zähle je Verzeichnis,
das liegt, die Dateien darin, rekursiv und mit versteckten Dateien; einem Symlink folgst du
nicht. Dann löschst du das Verzeichnis samt Inhalt im Dateisystem. Committet wird nicht. Je
Verzeichnis eine Zeile:

```text
Wissensablage auf der Platte
  cache/       gelöscht: 3 Dateien
  inbox/       gelöscht: 45 Dateien
  queue/       gelöscht: 2 Dateien
```

**b) `knowledge/` vorlegen.** Braucht die Collection. Jede Datei unter `knowledge/`, rekursiv,
außer `README.md` und einer `.gitignore` mit dem verwalteten Inhalt (Definition „Leer“ in
3.4), geht durch das Verfahren, das 3.2 für `extracted/` und lose Dateien hat: die Prüfungen
aus 3.2 a) für Dateien, die verschoben werden sollen, dann c) Vorlegen bis f) Melden. Ein
zweites Verfahren gibt es nicht. Abweichend davon gilt:

- Je Datei steht `[verschieben] → keph:/docs/manual/<name>.md` oder `[löschen]`. `<name>` ist
  ein Vorschlag aus Pfad oder Thema der Datei; der Nutzer kann ihn in der Vorlage ändern.
- Ergeben zwei Dateien denselben Vorschlag, stehen beide auf `[unklar]`.
- Beim Verschieben baust du das Frontmatter auf `title`, `description` und `sources` um:
  `title` bleibt, `description` sagt in einem Satz, was die Datei enthält, `sources` nennt
  ihre Quellen. Verweise auf `inbox/…` entfernst du, im Frontmatter wie im Text; die übrigen
  Felder der alten Ablage entfallen.

```text
Wissensablage auf der Platte: knowledge/
  extracted/01-release.md          [verschieben]  → keph:/docs/manual/release.md, Ziel frei
  external/opencode/server-api.md  [verschieben]  → keph:/docs/manual/opencode-server-api.md, Ziel frei
  findings/alt.md                  [löschen]      Befund von damals; Befunde liegen heute in keph:/queue/findings/
```

Nach dem Ausführen löschst du `knowledge/`, wenn darin nur noch `README.md`, die verwaltete
`.gitignore` und Unterverzeichnisse liegen, die in diesem Sinn leer sind. Sonst bleibt es, und
die Meldung nennt, was noch darin liegt. Die Umstellung ist wiederholbar: Was ein Lauf liegen
ließ, schlägt der nächste wieder vor.

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
sie ab — nichts verschwindet still. Alle Erzeuger schreiben inzwischen nach Kephalaion;
was sie auf der Platte hinterlassen haben, wird gelöscht und neu erzeugt, nicht verschoben.
Der Doku-Index steht nur noch in `docs/` der Collection; `AGENTS.md` und
`opencode.json[c]` tragen keinen Doku-Index und keine Doku-Verweise mehr.

**Anlass:** Unter `<context.local.dir>/docs/` liegt mindestens eine Datei aus den Bereichen
der Tabelle unten, oder `AGENTS.md` bzw. `opencode.json[c]` nennen das Präfix
`k-playbook-local/docs/` (dieselbe Suche wie in b). Braucht die Collection.

Die Bereiche und ihr Ziel:

| Bereich auf der Platte | Ziel | Vorgehen |
|---|---|---|
| `versions/` | `keph:/docs/generated/versions/` | Erzeuger (`/k-doc-inventory`, `k-playbook inventory`) schreibt schon nach Kephalaion: Plattendatei löschen; fehlt das Ziel, Erzeuger neu laufen lassen |
| `code/` | `keph:/docs/generated/code/` | Erzeuger `/k-docs-code` schreibt nach Kephalaion: Plattendateien löschen; neu erzeugen mit `/k-docs-code`. Kein Verschieben — der alte Stand bleibt in der Git-Geschichte |
| `libs/` | `keph:/docs/generated/libs/` | Erzeuger `/k-docs-tools` schreibt nach Kephalaion: Plattendateien löschen; neu erzeugen mit `/k-docs-tools`. Kein Verschieben — der alte Stand bleibt in der Git-Geschichte |
| `manual/` | `keph:/docs/manual/`, Pfad darunter gleich | von Hand geschrieben: verschieben; `manual/README.md` nicht verschieben (`README.md` schreibt kein Command, sie entsteht beim Indizieren), sondern löschen |
| `extracted/` und lose `.md` direkt unter `docs/` außer `README.md` | Vorschlag `keph:/docs/manual/<datei>` | je Datei verschieben oder verwerfen, der Nutzer entscheidet |
| `README.md` | entfällt | der alte Index von `/k-docs-index`: löschen, sobald seine Einträge zu `manual/`, `versions/`, `code/`, `libs/` und losen Dateien abgeräumt sind; sonst bliebe ein veralteter Index als stiller Rückfall |

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

Für die Erzeuger (`versions/`, `code/` und `libs/`): fehlt ihr Ziel in Kephalaion,
erinnerst du daran, sie neu laufen zu lassen. Beim Inventar bietest du an,
`k-playbook inventory` gleich auszuführen; der Lauf ist deterministisch. `/k-docs-code`
fragt nach Scope und Struktur, `/k-docs-tools` nach der Auswahl der Kandidaten; beide laufen
deshalb nicht nebenbei — nenne sie als nächsten Schritt, `/k-docs-tools` nach dem Inventar,
auf dem es aufbaut. Jede Datei unter `code/` und `libs/` wird einzeln zum Löschen
vorgeschlagen, auch wenn `keph:/docs/generated/code/` bzw. `keph:/docs/generated/libs/` noch
leer ist: Ihr Inhalt bleibt in der Git-Geschichte, und der Erzeuger schreibt das Verzeichnis
neu.

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

Das gilt für alle Verweise mit dem Präfix, auch für solche auf `libs/`. Je Fundstelle
merkst du `pfad:zeile` vor, bei einer Tabelle oder einem Absatz über mehrere Zeilen
`pfad:von-bis`.

**c) Vorlegen.** Lege gebündelt vor, eine Zeile je Datei bzw. Bereich mit Vorschlag und
Grund, in dieser Reihenfolge: die Erzeuger, `manual/`, lose Dateien und
`extracted/`, der alte Index, die Verweise. Ein Beispiel aus einem Projekt mit alter
Code-Doku, alten Steckbriefen und einer Index-Tabelle in `AGENTS.md`:

```text
Doku aus k-playbook-local/docs/
  versions/inventory.md        [löschen]      liegt als keph:/docs/generated/versions/inventory.md, Stand 2026-10-25
  code/00-overview.md          [löschen]      Erzeuger /k-docs-code schreibt nach keph:/docs/generated/code/; neu erzeugen mit /k-docs-code
  code/13-rag-pipeline.md      [löschen]      dito; keph:/docs/generated/code/ trägt schon 17 Dokumente
  libs/celery.md               [löschen]      Erzeuger /k-docs-tools schreibt nach keph:/docs/generated/libs/; neu erzeugen mit /k-docs-tools
  manual/release.md            [verschieben]  → keph:/docs/manual/release.md, Ziel frei
  manual/installation.md       [verschieben]  → keph:/docs/manual/installation.md, Ziel vorhanden und gleich: nur löschen
  manual/README.md             [löschen]      README.md schreibt kein Command, sie entsteht beim Indizieren
  notizen.md                   [unklar]       lose Datei ohne Erzeuger; Vorschlag keph:/docs/manual/notizen.md
  README.md                    [löschen]      alter Index von /k-docs-index, Einträge abgeräumt
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
- **bleibt** — nicht Teil dieser Umstellung, etwa eine Datei, die kein Text oder größer als
  1 MiB ist.
- **unklar** — du kannst es nicht entscheiden; der Nutzer entscheidet, je Zeile.

Warte auf die Antwort. Der Nutzer bestätigt die Vorlage ganz oder nimmt einzelne Zeilen aus.

**d) Ausführen, was bestätigt ist.** In der Reihenfolge der Vorlage:

1. **verschieben:** Kephalaion `create` mit dem ganzen Inhalt der Datei unter dem
   Zielnamen (ohne führenden `/`); bei einem gleichen Dokument am Ziel entfällt das
   Schreiben, bei einem verschiedenen gilt die Entscheidung des Nutzers (`write` mit
   `base_revision` aus dem `read`). Erst nach einem bestätigten Schreiben — die Antwort
   nennt Revision und Größe — löschst du die Plattendatei.
2. **löschen:** die Plattendatei entfernen. Der alte Index zuletzt, und nur, wenn alle Zeilen
   zu `manual/`, `versions/`, `code/`, `libs/` und losen Dateien ausgeführt sind; blieb eine davon
   liegen, bleibt auch er und wird beim nächsten Lauf erneut vorgeschlagen.
3. **entfernen:** in `opencode.json[c]` den Eintrag `references.docs` (ein leeres
   `references` dazu); in `AGENTS.md` die bestätigte Tabelle, den Absatz bzw. den einzelnen
   Verweis. Die übrigen Einträge und die Formatierung der Datei bleiben, wie sie sind.

Ein Schritt, der fehlschlägt, lässt seine Datei liegen; der nächste Lauf nimmt sie wieder
auf. Ihr Inhalt bleibt außerdem in der Git-Geschichte.

**e) Aufräumen.** Ein Verzeichnis unter `docs/`, das danach leer ist (`manual/`, `versions/`,
`code/`, `libs/`, `extracted/`), löschst du mit. `docs/` selbst und seine README fasst diese
Umstellung nicht an.

**f) Melden.** Je Datei eine Zeile: was aus ihr wurde (`keph:`-Verweis, gelöscht, liegen
geblieben und warum), dazu je vorgemerktem Link aus `manual/` hinaus `datei:zeile` und je
entferntem Verweis `pfad:zeile`. Prüfe das Ergebnis mit Kephalaion `list` auf `docs/` und
nenne, was dort jetzt liegt.

Die Umstellung ist wiederholbar: Was ein Lauf liegen ließ — eine ausgenommene Zeile, ein
fehlgeschlagener Schritt, der alte Index —, schlägt der nächste Lauf wieder vor.

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
Bindestrichen. Der eingesetzte Titel steht in doppelten Anführungszeichen, ein inneres `"` als
`\"` (Regel in „Kopf“). Der übrige Inhalt bleibt Byte für Byte; das Frontmatter kommt davor.
Trägt die Datei schon ein Frontmatter ohne `title`, kommt die Zeile `title: "<Titel>"` als
erste Zeile in diesen Block — kein zweiter Block. Trägt sie ein Frontmatter mit `title`, bleibt sie ganz
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

Das Einrichten legt elf Verzeichnisse unter `<context.local.dir>/` nicht mehr an und zählt
sie nicht mehr als fehlend: Der Inhalt der ersten fünf liegt in der Projekt-Collection, der von
`commands/` und `skills/` in `<context.project.dir>/.k-playbook/commands/` und
`<context.project.dir>/.k-playbook/skills/`. Projekteigene Instructions, Includes und
Review-Rezepte liegen in der Projekt-Collection unter `keph:/k-playbook/instructions/`,
`keph:/k-playbook/includes/` und `keph:/k-playbook/reviews/`, mit demselben Baum wie unter
`k-playbook:/`; `instructions/`, `includes/` und `reviews/` auf der Platte wertet k-playbook
nicht mehr aus. `reviews/` hat das Einrichten angelegt, `instructions/` und `includes/` legten
Projekte von Hand an. Der Inhalt von `rules/` hat keinen Nachfolger in
der Collection: Die Sorte gibt es nicht mehr, k-playbook liest dort nichts. Auch der Inhalt von
`checks/` hat keinen Nachfolger: `k-check` führt nur noch die mitgelieferten Checks aus,
projekteigene Checks gibt es nicht, bis sie eine neue Form bekommen. Projekteigene Guidelines
liegen in der Projekt-Collection unter `keph:/docs/guidelines/`; `guidelines/` auf der Platte
wird nicht umgezogen, sondern gelöscht (Ausnahme unten). Dazu kommt `tmp/`, das
das Einrichten nie angelegt hat: Sitzungen legten es zur Zeit der Wissensablage von Hand als
Schmierzettel an, mit der verwalteten `.gitignore` und einer README, die einen Aufräumer
versprach, den es nie gab. Nichts liest dort, nichts räumt dort auf; Notizen und Zwischenstände
gehören nach `priv/` (`k-playbook:/guidelines/betrieb.md`). In einem bestehenden Projekt
liegen die Verzeichnisse noch, oft nur mit ihrer README.
Diese Umstellung löscht jedes davon, das leer ist, `data/` mit der Ausnahme unten für die
Übernahmereste der Todos, ohne Rückfrage; jedes andere bleibt liegen und wird nur gemeldet.
`guidelines/` löscht sie immer, auch mit Inhalt.
Sie läuft nach 3.0 bis 3.3, damit zählt, was dort umgezogen ist. Die Liste gehört zu dieser
Fassung des Commands; das Programm nennt sie nicht:

- `docs/`
- `tasks/`
- `tasks/done/`
- `data/`
- `material/`
- `commands/`
- `skills/`
- `instructions/`
- `includes/`
- `reviews/`
- `rules/`
- `checks/`
- `guidelines/`
- `tmp/`

**Anlass:** Mindestens eines dieser Verzeichnisse liegt unter `<context.local.dir>/`. Braucht
die Collection nicht; nur die Prüfung auf `Ziel belegt` in d) und der Umzug in e) für
`instructions/`, `includes/` und `reviews/` brauchen sie. Steht `context.kephalaion.ok` nicht auf
`true`, nennt d) die Ziele ohne diese Prüfung und sagt das, und e) zieht nicht um.

**Vorbedingung:** Schritt 2 hat das Programm als aktuell gemeldet — in seiner Ausgabe steht
die Zeile `Programm-Stand: nichts zu tun — …`. Fehlt sie, nennst du in einer Zeile, dass das
Aufräumen auf den Binärwechsel wartet: Ein älteres Programm legt die Verzeichnisse beim Knopf
„Anlegen“ der Oberfläche wieder an. Für `commands/` und `skills/` trifft das genau: Erst das
neue Programm nimmt Einträge aus `.k-playbook/<sorte>/`; mit einem älteren verschwände eine
umgezogene Datei aus der Verlinkung. Für `instructions/`, `includes/` und `reviews/` ebenso:
Ein älteres Programm liest sie noch von der Platte und nicht aus der Collection. Dann bleibt
alles liegen, und du gehst zu Schritt 4.

**Leer** heißt: Im Verzeichnis liegt, auch in Unterverzeichnissen, nichts außer

- Dateien mit dem Namen `README.md`,
- Dateien mit dem Namen `.gitignore`, deren Inhalt genau der ist, den k-playbook verwaltet:
  die drei Zeilen `*`, `!.gitignore` und `!README.md` in dieser Reihenfolge — Leerzeilen und
  Leerraum am Zeilenrand zählen nicht mit,
- Unterverzeichnissen, die ihrerseits in diesem Sinn leer sind, etwa `tasks/done/` mit seiner
  README oder ein `material/befunde/` ohne Inhalt.

**Ausnahme für `data/`.** Direkt unter `data/`, nicht in einem Unterverzeichnis, zählen auch
die Übernahmereste von `k-playbook todo migrate` wie eine `README.md`: die Datei
`todos.json.migriert` und jede Datei `todos.json.nachzuegler-<datum>` oder
`todos.json.nachzuegler-<datum>-<n>` (`<datum>` als `JJJJ-MM-TT`). Jede zählt für sich, auch
ohne die andere; ein `data/` mit `README.md` und nur Nachzügler-Dateien ist also ebenfalls
leer. Diese Dateien liest niemand mehr, und `k-playbook todo migrate` erkennt einen Nachzügler
auch ohne sie. Liegt dagegen `data/todos.json` selbst noch da, ist sie noch nicht übernommen:
`data/` bleibt, und seine Zeile nennt `k-playbook todo migrate`.

**Ausnahme für `guidelines/`.** Es wird gelöscht, ob leer oder nicht, ohne Rückfrage und ohne
Umzug: Projekteigene Guidelines gehören nach `keph:/docs/guidelines/`, und wer welche hatte,
zieht sie vorher von Hand dorthin um. Eine versionierte Datei bleibt in der Git-Geschichte.
Weil hier Inhalt verschwinden kann, nennt die Meldung jede gelöschte Datei, ohne die Grenze
von fünf Pfaden.

Alles andere heißt „liegt anderes darin“: jede weitere Datei, eine `.gitignore` mit anderem
Inhalt, ein anderes Dotfile, ein Symlink.

**a) Bestand erheben.** Lies je Verzeichnis, das liegt, alles darin, rekursiv und mit
versteckten Dateien — ein Verzeichnislisting ohne Dotfiles reicht nicht, und einem Symlink
folgst du nicht. Danach ist jedes Verzeichnis leer oder nicht. Liegt `tasks/` leer, gilt das
auch für `tasks/done/`; beide stehen dann in einer Zeile `tasks/`.

**b) Einteilen.** Jedes Verzeichnis gehört danach in eine von zwei Gruppen: die leeren, die c)
löscht, und die übrigen, die liegen bleiben. `guidelines/` gehört immer zu den ersten. Den Inhalt einer `README.md` prüfst und bewertest
du nicht; ob ein Verzeichnis leer ist, entscheidet allein die Definition oben. Vorgelegt wird
nichts, und 3.4 stellt keine Frage: Auch wenn es nur Zeilen `bleibt: …` gibt, sind sie eine
Meldung.

**c) Leere löschen.** Prüfe je leerem Verzeichnis unmittelbar vor dem Löschen noch einmal, ob
es leer ist, für `data/` mit derselben Ausnahme; zwischen Erhebung und Löschen kann etwas
hinzugekommen sein, etwa eine neue `data/todos.json`. Ist es leer, löschst du es samt Inhalt
im Dateisystem. Ist es das nicht mehr, bleibt es und wird mit dem Grund gemeldet. `guidelines/`
prüfst du nicht noch einmal auf leer, sondern listest es unmittelbar vor dem Löschen erneut,
damit die Meldung jede Datei nennt, die tatsächlich gelöscht wurde. Committet wird nicht:
Versionierte Dateien stehen danach als gelöscht in `git status`.

**d) Melden.** Je Verzeichnis eine Zeile in der Reihenfolge der Liste: `gelöscht: <was darin
lag>`, oder `bleibt: <was noch liegt>` mit dem Grund — liegt anderes darin oder beim
zweiten Prüfen nicht mehr leer. Genannt werden Pfade relativ zum Verzeichnis, eine README
eingeschlossen, höchstens fünf; liegt mehr darin, dazu die Gesamtzahl der Dateien. Bleibt
`data/`, weil `todos.json` darin liegt, endet seine Zeile mit `— erst k-playbook todo migrate`.

Für `commands/` und `skills/` weicht die Zeile `bleibt:` ab, wenn anderes darin liegt. Statt
höchstens fünf Pfaden nennt sie als Grund, dass diese Dateien mit diesem Programm nicht mehr
wirken, und darunter steht je Eintrag eine eigene Zeile mit seinem Ziel
`.k-playbook/<sorte>/<gleicher pfad>`:

- unter `commands/` je Datei, auch in Namensräumen wie `_audit/`, mit gleichem Pfad ab
  `commands/`;
- unter `skills/` je Eintrag direkt darin, ein Skill-Verzeichnis als Ganzes.

Dateien mit dem Namen `README.md` und die verwaltete `.gitignore` ziehen nicht um und
erscheinen dort nicht. Liegt am Ziel schon etwas, steht `Ziel belegt` hinter der Zeile.

Für `instructions/`, `includes/` und `reviews/` gilt dasselbe: Liegt anderes darin, nennt die
Zeile `bleibt:` als Grund, dass diese Dateien mit diesem Programm nicht mehr gelesen werden,
und darunter steht je Datei, auch in Unterverzeichnissen wie `instructions/always/`, eine
eigene Zeile mit ihrem Ziel `keph:/k-playbook/<sorte>/<gleicher pfad>` in der
Projekt-Collection. Eine leere Datei zieht mit um und nennt dazu `schaltet ab`: Auch in der
Collection schaltet sie die gleichnamige mitgelieferte ab. Ob am Ziel schon etwas liegt,
prüfst du mit Kephalaion `read` (`collection` gleich `context.kephalaion.address`, `name` gleich
dem Ziel ohne `keph:/`); liegt dort ein Dokument, steht `Ziel belegt` hinter der Zeile. Das
genannte Ziel ist ein Vorschlag: Der Nutzer kann beim Umzug ein anderes nennen.

Für `rules/` nennt die Zeile `bleibt:` wie sonst höchstens fünf Pfade und dazu als Warnung,
dass diese Regeln nicht mehr gelesen werden: Ihr Inhalt gehört von Hand in Guidelines
(`keph:/docs/`) oder Instructions. 3.4 zieht ihn nicht um.

Für `checks/` nennt die Zeile `bleibt:` ebenso höchstens fünf Pfade und dazu als Hinweis, dass
diese Checks nicht mehr laufen: `k-check` führt nur noch die mitgelieferten aus. 3.4 zieht
nichts um und fragt nicht.

Für `guidelines/` gibt es nur die Zeile `gelöscht:`. Sie nennt jede Datei, auch über fünf, und
dazu, dass projekteigene Guidelines unter `keph:/docs/guidelines/` liegen.

Für `tmp/` nennt die Zeile `bleibt:` ebenso höchstens fünf Pfade und dazu als Hinweis, dass
dort niemand liest und niemand aufräumt: Was bleiben soll, gehört nach `priv/`, das Übrige
löscht der Nutzer von Hand. 3.4 zieht nichts um und fragt nicht.

```text
Aufräumen in k-playbook-local/
  docs/        gelöscht: README.md
  tasks/       gelöscht: README.md, done/README.md
  data/        gelöscht: README.md, todos.json.migriert
  material/    bleibt: README.md, notizen.md, chat/2026-09-12.md, chat/2026-09-13.md, scan/seite.pdf (45 Dateien)
  commands/    bleibt: wirkt mit diesem Programm nicht mehr, Ziel je Datei
    k-eigen.md       → .k-playbook/commands/k-eigen.md
    _audit/notiz.md  → .k-playbook/commands/_audit/notiz.md
    k-gui.md         → .k-playbook/commands/k-gui.md  Ziel belegt
  skills/      bleibt: wirkt mit diesem Programm nicht mehr, Ziel je Skill
    forelle/         → .k-playbook/skills/forelle/
  instructions/  bleibt: wird mit diesem Programm nicht mehr gelesen, Ziel je Datei
    on-demand/release.md  → keph:/k-playbook/instructions/on-demand/release.md
    on-demand/muster.md   → keph:/k-playbook/instructions/on-demand/muster.md  schaltet ab
    always/500-eigen.md   → keph:/k-playbook/instructions/always/500-eigen.md  Ziel belegt
  includes/    gelöscht: README.md
  reviews/     gelöscht: README.md
  rules/       gelöscht: README.md
  checks/      gelöscht: README.md
  guidelines/  gelöscht: README.md, stil.md — projekteigene Guidelines liegen unter keph:/docs/guidelines/
  tmp/         gelöscht: README.md
```

Liegt in `rules/` mehr als die README, lautet seine Zeile etwa:

```text
  rules/       bleibt: README.md, eigene-regel.md — wird nicht mehr gelesen; Inhalt von Hand in Guidelines (keph:/docs/) oder Instructions
```

Liegt in `checks/` mehr als die README, lautet seine Zeile etwa:

```text
  checks/      bleibt: README.md, check_eigen.sh — läuft nicht mehr; k-check führt nur noch die mitgelieferten Checks aus
```

Liegt in `tmp/` mehr als README und verwaltete `.gitignore`, lautet seine Zeile etwa:

```text
  tmp/         bleibt: README.md, notizen.md — liest und räumt niemand; Bleibendes nach priv/, Übriges von Hand löschen
```

Liegt in `data/` noch eine `todos.json`, lautet seine Zeile etwa:

```text
  data/        bleibt: README.md, todos.json — erst k-playbook todo migrate
```

Ein Verzeichnis, das nicht liegt, erscheint nicht. Auf die Meldung folgt keine Frage.

Bleibt `commands/`, `skills/`, `instructions/`, `includes/` oder `reviews/`, folgt auf die
Meldung ein Hinweis, ebenfalls ohne Frage: Umgebaut wird von Hand oder auf ausdrücklichen
Zuruf des Nutzers in derselben Sitzung; 3.4 zieht nicht von selbst um. Das Löschen am alten Ort und die Dateien am neuen gehören in den
nächsten Commit des Projekts; eine versionierte Datei verschoben mit `git mv` behält ihre
Geschichte.

**e) Umziehen auf Zuruf.** Sagt der Nutzer in derselben Sitzung ausdrücklich, dass umgezogen
wird, verschiebst du genau die Einträge, die d) mit Ziel genannt hat, Pfad und Namensraum
gleich; fehlende Verzeichnisse unter `.k-playbook/` legst du dabei an. Einen versionierten
Eintrag verschiebst du mit `git mv`, wenn `git` da ist (`baseTools`) und
`git ls-files --error-unmatch <pfad>` ihn kennt, sonst mit `mv`. Ein Eintrag mit
`Ziel belegt` bleibt liegen und wird genannt; zusammengeführt wird nicht. Danach lädst du das
Briefing neu (`k-playbook briefing`; der Katalog hat sich geändert), das die Verlinkung
nachzieht, prüfst, dass die Links in `.claude/commands/`,
`.opencode/commands/` und `.claude/skills/` für die umgezogenen Einträge auf
`.k-playbook/<sorte>/` zeigen, und führst c) und d) für `commands/` und `skills/` noch
einmal aus.

Für `instructions/`, `includes/` und `reviews/` heißt Umziehen: Du legst je genannter Datei
mit Kephalaion `create` ein Dokument am Ziel an (`collection` gleich
`context.kephalaion.address`, `name` gleich dem Ziel ohne `keph:/`, `content` gleich dem
Inhalt der Datei, unverändert, auch leer). Nennt der Nutzer für eine Datei ein anderes Ziel,
nimmst du seines; es muss unter `keph:/k-playbook/<sorte>/` liegen, sonst wirkt die Datei
nicht, und das sagst du ihm. Erst wenn `create` das Dokument bestätigt hat, löschst du die
Datei auf der Platte — `git rm`, wenn `git` da ist und sie versioniert ist, sonst `rm`.
Meldet `create` `name_taken`, bleibt die Datei liegen und wird mit `Ziel belegt` genannt;
zusammengeführt wird nicht. Bei `outcome_unknown` liest du das Ziel mit Kephalaion `read` nach,
statt noch einmal anzulegen, und löschst erst, wenn es dort mit dem Inhalt der Datei liegt.
Danach lädst du das Briefing neu und prüfst, dass es jede umgezogene Datei mit ihrem
`keph:`-Ort neben der Herkunft nennt (eine Anweisung auf Abruf oder ein Rezept als Eintrag,
ein Teil von `always/` im Text), und führst c) und d) für diese drei Verzeichnisse noch einmal
aus.

Die Umstellung ist wiederholbar: Ein Verzeichnis, das bleibt, löscht der nächste Lauf, sobald
es leer ist — etwa `material/` nach 3.1 oder `tasks/` nach 3.3.

### 3.5 — Alte Anbindung in `K-PLAYBOOK.local.yaml`

Bis k-playbook die Anbindung aus der Auskunft von Kephalaion nahm (`kephalaion node client`),
stand sie rechnerlokal in `<context.local.dir>/K-PLAYBOOK.local.yaml`, Block `kephalaion:` mit
Hub, Collection und Account. Heute stehen Hub und Collection im Block `kephalaion:` von
`.k-playbook/K-PLAYBOOK.yaml`, und den Account wählt Kephalaion. Kein Programmteil liest die
alte Datei mehr. Umgezogen hat sie schon `k-playbook upgrade` in Schritt 2, ohne Rückfrage;
diese Umstellung löscht sie nur noch. Sie braucht die Collection nicht.

**Anlass:** Die Datei `<context.local.dir>/K-PLAYBOOK.local.yaml` liegt.

**Bedingung:** In der Ausgabe von Schritt 2 steht eine Zeile, die mit
`Projekt: Anbindung umgezogen` oder mit `Projekt: Anbindung — der Block kephalaion:` beginnt.
Nur dann steht der Block in `.k-playbook/K-PLAYBOOK.yaml`. Fehlt eine solche Zeile — Schritt 2
lief mit einem Programm, das den Umzug noch nicht kennt, oder meldet
`Projekt: Anbindung nicht umgezogen` —, bleibt die Datei liegen: Nenne die Zeile aus Schritt 2
oder, wenn es keine gibt, dass der Umzug auf das neue Programm wartet, und geh weiter.
`context.kephalaion` taugt hier nicht als Beleg: Der MCP-Server einer laufenden Sitzung behält
nach dem Binärwechsel das alte Programm, liest die alte Datei und liefert diesen Text auch ohne
Block.

**Vorlage.** Lies die Datei und lege gebündelt vor:

- die Datei mit ihrem Pfad und dass sie gelöscht wird;
- was Schritt 2 zum Block gemeldet hat: umgezogen, steht, oder steht und weicht ab, dann mit
  beiden Werten aus der Zeile;
- trägt die Datei neben dem Block `kephalaion:` und Kommentaren weitere Schlüssel auf oberster
  Ebene, deren Namen, ohne Werte — gelöscht wird sie trotzdem nur als Ganzes;
- hat Schritt 2 eine Zeile `Projekt: Account — …` gemeldet, diesen Account samt dem Befehl
  `kephalaion node account default <hub> <account>` aus derselben Zeile: So bleibt er der
  Account dieses Rechners. Ist das Token-Verzeichnis nur lesend eingebunden, wie im
  DevContainer, wird der Standard auf dem Host gesetzt.

```text
Alte Anbindung
  k-playbook-local/K-PLAYBOOK.local.yaml löschen — der Block kephalaion: steht in .k-playbook/K-PLAYBOOK.yaml (hub vm, collection omni)
  Account der alten Datei: kamran-wsl — Kephalaion wählt für vm keinen; setzen mit kephalaion node account default vm kamran-wsl
```

**Löschen.** Gelöscht wird erst nach Bestätigung, die Datei allein. Den Befehl für den Account
führst du nicht aus; er gehört dem Nutzer, auch weil er im DevContainer auf dem Host laufen
muss. Die Zeile `/K-PLAYBOOK.local.yaml` in `<context.local.dir>/.gitignore` bleibt stehen;
sie schadet nicht. Committet wird nicht: Die Änderung an `.k-playbook/K-PLAYBOOK.yaml` aus
Schritt 2 gehört in den nächsten Commit des Projekts — sag das.

## Schritt 4 — Schluss

Nenne nur die Umstellungen, die etwas getan haben, bei denen etwas liegen geblieben ist oder die
eine Warnung tragen, je eine Zeile; eine Umstellung ohne Anlass erscheint nicht. Trifft das auf
keine zu, steht hier nur `Umstellungen: alles erledigt.`, dazu Hinweise aus Schritt 2. Für
3.0 je gelöschtem Verzeichnis die Zahl seiner Dateien und je Datei aus `knowledge/` ihr Ziel
(`keph:`-Verweis, gelöscht oder liegen geblieben), für das Aufräumen die gelöschten
Verzeichnisse und jede Zeile `bleibt: …`, bei `commands/` und `skills/` mit den Zeilen je
Eintrag und dem, was e) umgezogen hat — oder bei 3.0 und beim Aufräumen, dass sie auf den
Binärwechsel warten. Für 3.5: ob die alte `K-PLAYBOOK.local.yaml` gelöscht ist oder warum sie
liegt, ein genannter Account mit seinem Befehl und dass `.k-playbook/K-PLAYBOOK.yaml` committet
werden will.
