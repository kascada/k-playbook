# k-docs

Zeigt den Stand der Projektdoku unter `keph:/docs/` in der Collection des Projekts, meldet
Lücken und Widersprüche und bietet die Erzeuger an: das Versionsinventar, die Doku aus dem
Code und die Steckbriefe der Libraries und Tools. Inventar und Steckbriefe startet er selbst.
Die Doku aus dem Code startet er nicht: Für `code` reiht er den vollen Lauf als Doku-Auftrag
ein, den `/k-docs-code` abarbeitet. Mit `alle` laufen Inventar und Steckbriefe nacheinander,
die Code-Doku wird eingereiht, danach schreibt das Programm die `README.md` der Verzeichnisse
aus ihren Vorlagen. Der Ablauf eines Erzeugers steht in seinem eigenen Text; dieser
Command wiederholt ihn nicht.

Produces:

- Im Statuslauf nichts. Ohne Auswahl ist `/k-docs` read-only.
- Bei `inventory` und `tools` schreibt nur der gewählte Erzeuger, nach den Regeln seines
  Textes.
- Bei `code` und `alle` höchstens ein roher Doku-Auftrag für `code` mit `pfade: .` in
  `keph:/queue/docs/`. Die Code-Doku selbst schreibt erst `/k-docs-code`.
- Mit `alle` zuletzt die `README.md` unter `keph:/docs/`, die fehlen oder von ihrer Vorlage
  abweichen — geschrieben vom Programm (`k_playbook_docs_readme`), nicht von Hand.

## Schritt 1 — Briefing und Ablage

include: _shared/erster-schritt.md

include: _shared/docs.md

Aus dem Briefing brauchst du `context.kephalaion` (`ok`, `address`, `message`),
`context.project.dir`, `context.docs` (`exclude`, `problems`), `session` und `context.now`
(`date`, `timestamp`). Ohne
`context.kephalaion.ok: true` endet der Command hier, wie die Ablage oben sagt.

Die Erzeuger und ihre Verzeichnisse:

| Bereich | Verzeichnis | Schreiber (`generated.by`) | Command |
|---|---|---|---|
| Inventar | `keph:/docs/generated/versions/` | `k-doc-inventory` | `/k-doc-inventory` |
| Code | `keph:/docs/generated/code/` | `k-docs-code`, Altwert `k-code2docs`; `ks-overlay-repo-analyse` | `/k-docs-code` |
| Libs | `keph:/docs/generated/libs/` | `k-docs-tools`, Altwert `k-tools-scan` | `/k-docs-tools` |
| Handdoku | `keph:/docs/manual/`, darin `handbook/` als Handbuch des Projekts, das die Oberfläche zeigt | keiner — von Hand | — |
| Quellen | `keph:/docs/sources/` | keiner — Verweise auf Quellen außerhalb | — |

## Schritt 2 — Bestand

Ein Aufruf: Kephalaion `list` mit `collection` = `context.kephalaion.address`,
`path` = `docs`, `recursive: true`, `frontmatter: true` (bei `more` mit `cursor`
weiterblättern). Daraus, ohne ein Dokument einzeln zu lesen:

- Je Bereich der Tabelle: Zahl der Dokumente ohne `README.md`, ob die `README.md` des
  Erzeuger-Verzeichnisses da ist, der jüngste `generated.at` und die Verteilung von
  `generated.by`.
- Das Inventar `docs/generated/versions/inventory.md`: `generated.at` und die Zahlen aus
  `generated.stats` (`entries`, `deviations`, `sources-read`, `sources-configured`,
  `rejected`). Ein Inventar im alten Kopf trägt sie noch unter `inventory.*`; dann gilt
  das, und Schritt 3 meldet den alten Kopf. Der Stand kommt nur aus dem Kopf, nie aus dem
  Rumpf.
- Dokumente unter `docs/` außerhalb der Bereiche der Tabelle: unsortiert.

Dazu der Stand der `README.md` gegen ihre Vorlagen: `k_playbook_docs_readme` mit
`projectDir` = `context.project.dir` und `check: true` (ersatzweise
`k-playbook docs readme --check --json`). Das schreibt nichts. Die Antwort nennt je Vorlage
unter `k-playbook:/docs/` das Verzeichnis (`dir`), die README (`ref`) und `status`:
`unchanged`, `missing` (fehlt), `differs` (weicht ab — von Hand geändert, älter als die
Vorlage oder Kephalaions Mahnung) oder `failed` mit `code`; `hint`, wenn es keine Vorlagen
gibt. Verzeichnisse ohne Vorlage kommen darin nicht vor. Ist das Werkzeug unbekannt oder
meldet `k-playbook docs` `unbekannter Unterbefehl`, ist die Installation älter als dieser
Command — die Zeile `README` in Schritt 4 nennt dann `/k-upgrade`.

Dazu die offenen Doku-Aufträge in `keph:/queue/docs/`
(`k-playbook:/instructions/on-demand/doku-auftraege.md`): ein `k_playbook_doc_list` mit
`kind: docs`, ohne `done` — die erledigten unter `done/` zählen nicht; es nennt je Auftrag
`updated`, den Zeitpunkt seiner letzten Änderung. Die Antwort ist eine Seite (Standard
50 Einträge): Ist sie `truncated`, holst du mit `next.after` als `after` die nächste, bis
alle da sind; `total` ist die Zahl aller offenen Aufträge. Für die Aufteilung ein
`k_playbook_doc_grep` mit `kind: docs` und dem Muster
`^(erzeuger|entstanden|dokument): |^\s*- \.$`; je Auftrag gelten seine Zeilen:

- `erzeuger:` teilt nach Erzeuger, `entstanden:` nennt den ältesten Auftrag.
- Ein Auftrag mit einer Zeile `dokument:` ist **zerlegt**, sonst **roh**. Zerlegte gibt es
  nur für `code`; liegen welche offen, ist ein Lauf von `/k-docs-code` unterbrochen.
- Ein roher Auftrag für `code` mit der Zeile `- .` trägt `pfade: .`: Der volle Lauf ist
  schon eingereiht.
- Ein roher Auftrag für `code` **wartet**, solange sein `updated` jünger ist als die
  Wartezeit; ob er wartet, sagt `k_playbook_docs_orders` mit `erzeuger: code` (read-only, `waiting`, `remainingMinutes`), nicht eine eigene Rechnung. Liegt ein roher Auftrag mit `pfade: .` offen, wartet
  keiner: Er nimmt alle auf.

Gelesen wird kein Auftrag einzeln. `invalid_input`, weil die Sorte `docs` unbekannt ist,
heißt: Die Installation ist älter als dieser Command — die Zeile `Aufträge` in Schritt 4
nennt dann `/k-upgrade`.

Dazu auf der Platte, nur nachsehen:

- `<context.project.dir>/.k-playbook/work/docs/<erzeuger>/` — ein Arbeitsverzeichnis, das
  noch liegt, ist ein Lauf, der nicht veröffentlicht hat. Nenne Erzeuger und Alter der
  Merkdatei `<erzeuger>.checkout.json`.
- `context.docs.problems` — verworfene Einträge in `docs.exclude`.

`not_found` für `docs` heißt: Es gibt noch keine Projektdoku; das ist ein Stand, kein
Fehler. `kephalaion_unavailable` heißt: nicht erreichbar; dann endet der Command wie in der
Ablage oben.

## Schritt 3 — Befunde melden

Melden, nicht beheben. Grundlage ist das Frontmatter aus Schritt 2; ein Dokument wird nur
gelesen, wo eine Prüfung unten es verlangt.

- **Kopf.** Fehlt `title` oder `description`. Trägt ein Dokument Kopffelder außerhalb des
  Kopfs aus der Ablage oben (`type`, `tags`, `status`, `lib`, `version`, `version-pin`,
  `severity`, `last-reviewed`, ein eigener Block wie `inventory:` oder ein anderes Feld),
  ist das ein **alter Kopf**: je Dokument eine Zeile mit den Feldern. Der nächste Lauf
  seines Erzeugers schreibt ihn neu; ein Handdokument zieht der Nutzer nach.
- **Sprache.** Die Doku unter `keph:/docs/` ist englisch. Sind `title` oder `description`
  eines Dokuments erkennbar nicht englisch, etwa deutsch, ist das eine Lücke: je Dokument
  eine Zeile. Ein erzeugtes Dokument wird erst englisch, wenn ein Lauf seines Erzeugers es
  umschreibt — bei `code` braucht das einen Auftrag (`code` einreihen); ein Handdokument
  übersetzt der Nutzer.
- **Schreiber.** `generated.by` passt nicht zum Verzeichnis laut Tabelle in Schritt 1; ein
  Dokument unter `manual/` oder `sources/` trägt `generated`; ein Dokument in einem
  Erzeuger-Verzeichnis trägt keins.
- **README.** Je Vorlage eine `README.md`, die laut Schritt 2 fehlt (`missing`) oder abweicht
  (`differs`) — eine Zeile je README mit Verweis; `failed` mit Code (`template_missing`: Der
  Lieferumfang auf dem Hub ist älter als das Programm). Verzeichnisse ohne Vorlage — etwa
  Unterordner von `manual/` — meldest du nicht; dort mahnt Kephalaion selbst. Nachgezogen
  wird mit `alle` oder `k-playbook docs readme`; eine von Hand geänderte README wird dabei
  überschrieben.
- **`questions`.** Ein erzeugtes Dokument ohne `questions`.
- **`sources`.** Ein Eintrag ohne `resource`; ein `keph:`-Verweis darin, der auf kein
  Dokument der Liste aus Schritt 2 zeigt.
- **Steckbriefe gegen das Inventar.** Nur, wenn das Inventar da ist. Je Dokument unter
  `generated/libs/` nimmst du die `sources`-Einträge mit
  `resource: keph:/docs/generated/versions/inventory.md` (`item`, `version`) und suchst den
  Gegenstand `item` im Inventar. Dafür liest du das Inventar einmal mit `k_playbook_doc_read`
  und nimmst nur die Spalten `Gegenstand`, `Version` und `Pin` seiner Tabellen. Jede
  Abweichung ist eine eigene Zeile:

  ```text
  libs/<name>.md: gelesen gegen <item> <version> — Inventar: <version> (<pin>), <herkunft>
  ```

  Ein Gegenstand, der im Inventar fehlt, ist eine Zeile „nicht mehr im Inventar“. Das
  Inventar gibt die Auskunft; der Steckbrief wird nicht umgeschrieben. Nachgezogen wird er
  mit `/k-docs-tools`, wenn sein Inhalt veraltet ist, nicht nur seine Zahl. Diese Zeilen
  sind keine Abweichungen des Inventars: Die entstehen aus zwei Quellen, diese aus zwei
  Dokumenten. Kommt in einem Manifest, Lockfile, einer Container-, Compose-, Helm- oder
  CI-Datei eine Versionsangabe hinzu, fällt sie weg oder wechselt sie, wird das Inventar im
  selben Arbeitsgang über den Erzeuger nachgezogen (`/k-doc-inventory`); die Steckbriefe
  werden danach eingereiht (`erzeuger: tools`). Eine fremde, schon vorhandene Abweichung ist
  ein Hinweis an den Nutzer, kein stilles Nachziehen.
- **Reste.** Ein liegengebliebenes Arbeitsverzeichnis → der Erzeuger fragt beim nächsten
  Lauf, ob weitergemacht oder verworfen wird.

Gibt es keine Befunde, sag das in einer Zeile.

## Schritt 4 — Stand und Möglichkeiten

```text
/k-docs — Stand
─────────────────────────────────────
Collection  <context.kephalaion.address>
Inventar    erhoben <generated.at>, <entries> Einträge, <sources-read> Quellen gelesen,
            <deviations> Abweichungen | fehlt
Code        <N> Dokumente (<N> k-docs-code, <N> ks-overlay-repo-analyse), zuletzt <generated.at> | leer
Libs        <N> Steckbriefe, zuletzt <generated.at> | leer
Handdoku    <N> Dokumente | leer
Quellen     <N> Verweise | leer
Unsortiert  <N> Dokumente | —
Aufträge    <N> (<N> code, <N> tools), ältester <entstanden> | keine
            code: <N> roh (<N> warten noch), <N> zerlegt — ein Lauf ist unterbrochen
README      <N> Vorlagen, <N> fehlen, <N> weichen ab | alle aktuell | keine Vorlagen
Befunde     <N> (<Kategorien>) | keine
Offen       Arbeitsverzeichnis: <erzeuger> seit <alter> | —

Mögliche Aktionen:
  1. inventory  Versionsinventar erheben            → /k-doc-inventory
  2. code       Doku aus dem Code: vollen Lauf einreihen
                <N> Aufträge offen → /k-docs-code
  3. tools      Steckbriefe der Libraries und Tools → /k-docs-tools   <N> Aufträge offen
  4. alle       1 und 3 nacheinander, 2 einreihen, danach die README aus den Vorlagen
  5. status     nur diesen Bericht

Was soll ich tun?
```

Die zweite Zeile zu `Aufträge` steht nur, wenn Aufträge für `code` offen sind: rohe und
zerlegte getrennt, bei den rohen, wie viele noch warten. Offene zerlegte heißen: Ein Lauf ist
unterbrochen; `/k-docs-code` setzt ihn fort. Ohne zerlegte entfällt dieser Teil der Zeile.

`tools` setzt das Inventar voraus. Fehlt es, steht bei 3 „braucht das Inventar (1)“; `alle`
erhebt es zuerst. Bei 2 und 3 steht die Zahl der offenen Aufträge des Erzeugers
(`erzeuger: code` bzw. `erzeuger: tools`); ohne offene Aufträge entfällt der Zusatz.

- **`code`** startet keinen Erzeuger, sondern reiht den vollen Lauf über das ganze Projekt
  ein (Schritt 5). Offene Aufträge für `code` arbeitet `/k-docs-code` ab, ohne dass hier
  etwas gewählt wird — dann nur ihre Dokumente. Liegt der volle Lauf schon als Auftrag offen,
  steht bei 2 „voller Lauf eingereiht → /k-docs-code“.
- **`tools`** listet seine Aufträge im Preflight und schließt sie nach einem erfolgreichen
  `publish` ab.

## Schritt 5 — Weitergeben

Ohne `$ARGUMENTS` zeigst du den Bericht und wartest auf die Wahl. Kein Erzeuger startet von
selbst.

Mit Argument entfällt die Frage nach dem Bericht, nicht der Bericht: Das erste Wort wählt die
Aktion. Einen Rest nimmt nur `tools`, auch innerhalb von `alle`.

- `status` → Bericht, dann Ende.
- `inventory` → lies `k-playbook:/commands/k-doc-inventory.md` mit `k_playbook_doc_read`
  (ersatzweise `k-playbook read`) und befolge ihn. Er nimmt kein Argument.
- `code` → **„Vollen Lauf einreihen“** unten. `/k-docs-code` startest du nicht; seinen Text
  `k-playbook:/commands/k-docs-code.md` liest und befolgst du hier nicht. Steht nach `code`
  ein Rest, weist du ihn mit dieser Meldung ab und reihst nichts ein:

  ```text
  /k-docs code nimmt kein Argument: „<rest>“ wird nicht ausgewertet, nichts ist eingereiht.
  Der volle Lauf geht über das ganze Projekt; einzelne Pfade kommen über Doku-Aufträge.
  ```

- `tools` → lies `k-playbook:/commands/k-docs-tools.md` und befolge ihn mit dem Rest der
  Argumente.
- `alle` → zuerst „Vollen Lauf einreihen“ für `code`, dann
  nacheinander `inventory` und `tools`, in genau dieser Reihenfolge: Die Steckbriefe bauen
  auf dem Inventar auf. `code` wird nur eingereiht, die Code-Doku läuft hier nicht. Der Rest
  der Argumente geht nur an `tools`. Jeder Erzeuger läuft mit seinem eigenen Preflight und
  seinen eigenen Bestätigungen; dieser Command fasst sie nicht zusammen. Endet einer ohne
  Ergebnis — abgelehnt, abgebrochen, Konflikt offen —, fragst du, ob die übrigen trotzdem
  laufen sollen. `tools` läuft nicht ohne Inventar. Zuletzt, auch wenn ein Erzeuger übersprungen
  wurde: `k_playbook_docs_readme` mit `projectDir` = `context.project.dir`, ohne `pfad` und
  ohne `check` (ersatzweise `k-playbook docs readme --json`) — es schreibt jede README, die
  fehlt oder von ihrer Vorlage abweicht. `publish` und das Inventar haben die ihrer
  Verzeichnisse schon geschrieben; dieser Aufruf holt die übrigen nach, etwa die von
  `manual/` und `sources/`.

Das Briefing aus Schritt 1 gilt für den Erzeuger weiter; er lädt es nicht neu, außer sein
Text verlangt es.

**Vollen Lauf einreihen.** Für `code` und `alle` reihst du höchstens einen rohen Auftrag für
`code` mit `pfade: .` ein, nach `k-playbook:/instructions/on-demand/doku-auftraege.md`. Das
Programm trägt die Regeln dazu: Für diesen Auftrag gelten „Ein Auftrag je Sitzung und
Erzeuger“, „Kein Doppel“ und „Noch keine erzeugte Doku“ nicht — er wird auch eingereiht,
wenn es unter `keph:/docs/generated/code/` noch kein Dokument gibt, und ein schon offener
roher Auftrag mit `pfade: .` wird nicht verdoppelt, gleich aus welcher Sitzung er stammt.

1. Liegen laut Schritt 2 feine rohe Aufträge für `code` offen (ohne `.`), sagst du vor dem
   Einreihen: Der Auftrag über `.` nimmt sie alle auf, auch die wartenden, und der Lauf geht
   dann über das ganze Projekt statt nur über ihre Dokumente. Ohne den vollen Lauf arbeitet
   `/k-docs-code` allein die feinen ab.
2. Rufe `k_playbook_docs_order_add` mit `projectDir` = `context.project.dir`, `erzeuger:
   code`, `pfade: ["."]`, `anlass: "Voller Lauf, eingereiht über /k-docs."`, `sitzung` =
   `session` und `title: "Code-Doku: voller Lauf über das ganze Projekt"` auf (ersatzweise
   `k-playbook docs order add code --pfad . --anlass "…" --sitzung <session> --title "…"
   --json`). `status` ist `created` (eingereiht), `covered` (ein offener Auftrag mit `.`
   liegt schon vor: nenne ihn, `ref`) oder `skipped` mit `reason` (`kephalaion_unavailable`:
   nicht eingereiht).
3. Sag ausdrücklich: Die Code-Doku ist noch nicht gelaufen. `/k-docs-code` arbeitet den
   Auftrag ab — ohne Wartezeit, ein Auftrag mit `pfade: .` hat keine.

## Schritt 6 — Abschluss

Im Statuslauf:

- Dokumente je Bereich, offene Aufträge — für `code` rohe, wartende und zerlegte getrennt —
  und Befunde je Kategorie, gezählt.
- Ausdrücklich: Es wurde nichts geschrieben.
- Folge-Command: einer der Erzeuger aus Schritt 4, sobald der Nutzer ihn wählt; ohne Wahl
  keiner.

Nach `code`: der Auftrag mit Verweis — eingereiht oder schon offen gewesen —, welche feinen
Aufträge er aufnimmt, und ausdrücklich: Die Code-Doku ist noch nicht gelaufen. Folge-Command:
`/k-docs-code`.

Nach einer Weitergabe gilt der Abschluss des Erzeugers. Bei `alle` danach eine Zeile je
Erzeuger: `inventory` und `tools` gelaufen, übersprungen oder abgebrochen, mit Grund; `code`
eingereiht oder schon offen gewesen, mit Verweis und dem ausdrücklichen Satz, dass die
Code-Doku noch nicht gelaufen ist, Folge-Command `/k-docs-code` — und eine Zeile `README` aus
der Antwort von `k_playbook_docs_readme`: wie viele angelegt, überschrieben und unverändert,
und jede README mit `failed`, ihrem Code und ihrer Meldung. `outcome_unknown` heißt, das
Schreiben kann angekommen sein: nicht wiederholen, mit `check: true` nachsehen.

## Fehlerfälle

- `context.kephalaion.ok` ist nicht `true` oder `list` meldet `kephalaion_unavailable` →
  kein Bericht; `context.kephalaion.message` nennen und auf die Projekt-Collection auf der
  Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf die Platte.
- Unbekanntes Argument → die gültigen nennen: `status`, `inventory`, `code`, `tools`,
  `alle`.
- Ein Rest nach `code` → die Meldung aus Schritt 5; nichts wird eingereiht.
- `k_playbook_docs_order_add` ist unbekannt (`invalid_input` für das Werkzeug), `k-playbook
  docs` meldet `unbekanntes Kommando` oder `unbekannter Unterbefehl`, oder `k_playbook_doc_list`
  meldet `invalid_input` für die Sorte `docs` → die Installation ist älter als dieser Command:
  melden, `/k-upgrade` nennen, keinen Auftrag von Hand anlegen.
- `skipped` mit `kephalaion_unavailable` beim Einreihen → nicht eingereiht; der Abschluss
  nennt es. `outcome_unknown` → nicht wiederholen, mit `k_playbook_doc_list` (`kind: docs`,
  `name` oder `number` des Auftrags) nachsehen.
- Der Text eines Erzeugers ist nicht lesbar (`not_found`) → die Installation ist
  unvollständig oder älter als dieser Command: melden, `/k-upgrade` nennen; den Ablauf nicht
  im Gespräch nachbauen.

## Anti-Muster (nicht tun)

- **Erzeuger von selbst starten.** Der Bericht ist read-only; geschrieben wird erst nach
  einer Wahl.
- **`/k-docs-code` starten.** Für `code` wird nur eingereiht; den Lauf startet der Nutzer.
- **Einen zweiten Auftrag mit `pfade: .` anlegen** oder einen vorhandenen Auftrag ändern.
  Zusammengelegt und zerlegt wird allein in `/k-docs-code`.
- **Abläufe nachbauen.** Was ein Erzeuger tut, steht in seinem Text.
- **Befunde nebenbei beheben.** Kein Dokument wird im Statuslauf geändert, auch kein alter
  Kopf und kein Steckbrief mit abweichender Version.
- **Den Stand des Inventars aus dem Rumpf lesen.** Er kommt nur aus dem Kopf.
- **Die Platte als Doku nehmen.** Die Projektdoku liegt allein unter `keph:/docs/`; eine
  Datei auf der Platte ist keine Quelle.
- **`README.md` oder `index.md` von Hand schreiben.** Die README schreibt das Programm aus
  ihrer Vorlage (`k_playbook_docs_readme`), `index.md` erzeugt Kephalaion; kein Command
  schreibt sie mit Kephalaion `write` oder `create`.
