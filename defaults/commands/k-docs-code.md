# k-docs-code

Erzeugt aus dem Code eine kuratierte Projektdoku, die **Bedeutung** beschreibt, nicht
Oberfläche — ein Dokument je Thema, kein Ersatz für grep. Welche Dokumente ein Lauf schreibt,
bestimmen die Doku-Aufträge in `keph:/queue/docs/`: Dieser Command ist ihr **Verarbeiter**
für den Erzeuger `code`. Er liest die offenen Aufträge, legt abhängige zusammen, zerlegt sie
in Aufträge je Dokument, schreibt diese in die Queue zurück und arbeitet sie der Reihe nach
ab. Die Doku selbst wird nur in die Collection des Projekts geschrieben, über ein lokales
Arbeitsverzeichnis: `k_playbook_docs_checkout` holt den Bestand, die Dokumente entstehen dort
mit den Datei-Werkzeugen, `k_playbook_docs_publish` gleicht zurück ab.

Produces:

- `keph:/docs/generated/code/<NN>-<slug>.md` — ein Dokument je zusammenhängendem Thema,
  geschrieben von `k_playbook_docs_publish`.
- Zerlegte Doku-Aufträge in `keph:/queue/docs/`, einer je Dokument, angelegt oder ergänzt in
  Schritt 4 (`k_playbook_docs_orders_split`).
- Abgearbeitete Aufträge unter `keph:/queue/docs/done/`: die rohen nach dem Zerlegen
  (Schritt 4), die zerlegten nach „Abschluss eines Auftrags“ (Schritt 6).

Nichts sonst. Die `README.md` des Verzeichnisses schreibt `publish` aus ihrer Vorlage; das
Arbeitsverzeichnis unter `.k-playbook/work/docs/code/` entfernt `publish` nach einem
vollständigen Abgleich. Einzige Ausnahme: Bestätigt der Nutzer in Schritt 7, die im
Preflight ergänzten Ausschlüsse zu speichern, trägt `k_playbook_docs_exclude_add` sie in
`docs.exclude` von `K-PLAYBOOK.yaml` ein.

## Schritt 1 — Briefing, Aufträge und Scope

include: _shared/erster-schritt.md

include: _shared/docs.md

Dieser Command ist ein Schreiber des Erzeugers `code`: sein Verzeichnis ist
`keph:/docs/generated/code/`. Den zweiten Schreiber dort erkennst du an
`generated.by: ks-overlay-repo-analyse`.

Aus dem Briefing brauchst du `kephalaion` — `ok`, `address`, `message` —, dazu
`session` und `now`. Ohne `ok: true` endet der Command hier (siehe die Ablage oben).

**Kein Argument.** `/k-docs-code` nimmt kein Argument. Ist `$ARGUMENTS` nicht leer, endet der
Command hier mit dieser Meldung — bevor etwas gesucht, geholt oder geschrieben wird:

```text
/k-docs-code nimmt kein Argument: „<$ARGUMENTS>“ wird nicht ausgewertet, nichts ist gelaufen.
Welche Dokumente drankommen, bestimmen die Doku-Aufträge (pfade im Kopf).
Voller Lauf: /k-docs code reiht ihn ein, danach /k-docs-code.
```

**Aufträge.** Lies `k-playbook:/commands/_docs/auftraege.md` mit `k_playbook_doc_read`
(`projectDir` = `project.dir`) und rufe `k_playbook_docs_orders` mit `erzeuger: code`
auf. Du merkst dir `RAW_ORDERS` (`rawOrders`), `SPLIT_ORDERS` (`splitOrders`) und `WAITING`
(`waiting`); meldet `problems` unlesbare Aufträge, nennst du sie im Preflight. Ein Auftrag
ist **zerlegt**, wenn er `dokument` trägt, sonst **roh**; offene zerlegte heißen, ein früherer Lauf ist unterbrochen, und dieser setzt dort wieder an.

**Nichts zu tun.** Meldet die Antwort `nothingToDo`, endet der Command hier: Gibt es keinen
offenen Auftrag mit `erzeuger: code`, sag das und nenne `/k-docs code` als Weg zum vollen
Lauf. Sind nur wartende offen, nenne sie mit ihrer Restzeit und `/k-docs code` als Weg,
sofort zu laufen. Nichts wurde geholt oder geschrieben.

**Scope.** Lies `k-playbook:/commands/_docs/scope.md` und rufe `k_playbook_docs_scope` auf,
mit `check` = den Pfaden der Aufträge (ohne `.`). Fünf Abweichungen hat dieser Command:

- **Kein `EXTRA_PATH`:** `pfad` bleibt weg, die Zeile `Zusätzlich` zeigt immer `—`. Ein
  versteckter Pfad kommt nur über einen `hint` in den Scan; ein Pfad, den `docs.exclude`
  ausnimmt, ist auch für Zusatzquellen aus `hints` nicht erreichbar.
- **Die Dokumente kommen aus den Aufträgen.** Die `pfade` bestimmen, welche Dokumente
  drankommen, nicht der Scan.
- **Gelesen wird innerhalb von `SCAN_ROOT`:** Für das Dokument eines Auftrags liest du die
  bisherige Fassung und was sein Thema braucht, auch außerhalb der Pfade. Der Rest des
  Projekts wird nur im vollen Lauf gescannt.
- **Die Ausschlüsse gelten** auch für die Pfade eines Auftrags (`checks`): Ein ausgenommener
  Pfad führt zu keinem Dokument.
- Der Stand wird nur beschrieben, nie umgeschaltet; was der Preflight fragt, steht im Modul.

Zeige den Preflight und lass ihn **einmal** bestätigen; der Nutzer darf dabei Ausschlüsse
ergänzen (`NEW_EXCLUDES`, wie im Modul zum Scope).

```text
/k-docs-code — Preflight
─────────────────────────────────────
Collection    <kephalaion.address>
Ziel          keph:/docs/generated/code/
<die Scope-Zeilen aus dem Modul zum Scope>
Lauf          voll (Auftrag über .) | Teil-Lauf | Wiederaufnahme (nur zerlegte Aufträge)
Aufträge      <N> roh, zusammengelegt zu <M> | <N> zerlegt | <N> warten
  roh         <datei>[, <datei>, …] (<entstanden>) — <anlass>[; <anlass>; …]
                <pfad>, <pfad>, …
  zerlegt     <datei> (<entstanden>) — <dokument>: <aktion>
  wartet      <datei> — noch <restzeit>
                <pfad>, <pfad>, …
```

Bei „nein“ abbrechen; nichts wurde geholt oder geschrieben, kein Auftrag angefasst.

## Schritt 2 — checkout

Rufe `k_playbook_docs_checkout` mit `projectDir` = `project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs checkout code --json`).

- `state: checked_out` — das Arbeitsverzeichnis ist geholt. `WORK_DIR` ist `dir` aus der
  Antwort, `WORK_DISPLAY_PATH` derselbe Pfad ab `project.dir`
  (`.k-playbook/work/docs/code`).
- `state: existing` — ein Arbeitsverzeichnis liegt schon da, etwa von einem
  abgebrochenen Lauf. Nichts wurde geholt oder überschrieben. Nenne `existing.since`,
  `existing.ageMinutes` und `existing.files` und frage: **weitermachen** (mit dem, was dort
  liegt) oder **verwerfen** (`k_playbook_docs_checkout` mit `discard: true`, holt neu)?
  Ohne Antwort geht es nicht weiter.
- `kephalaion_unavailable` — nichts geschrieben; der Command endet wie in der Ablage oben.

Die Doku wird ab hier nur in `WORK_DIR` geschrieben. Daneben schreibt der Lauf nur die
Aufträge in der Queue (Schritt 4 und 6) und den bestätigten Eintrag über
`k_playbook_docs_exclude_add` in Schritt 7.

## Schritt 3 — Lesen

Lies **alle** Dokumente in `WORK_DIR`, bevor du den Code analysierst — auch im Teil-Lauf. Je
Dokument merkst du dir `generated.by`, `hints` und `questions` aus dem Frontmatter, dazu sein
Thema und seine Stellen im Code.

- `hints` befolgst du (Kopf in der Ablage oben) — etwa Zusatzquellen oder Schwerpunkte.
  Eine Zusatzquelle aus einem `hint` gehört zum Scan wie im Modul zum Scope.
- Den bisherigen Text nimmst du als Anregung, nicht als Vorlage zum Abschreiben. Er sagt
  aber, welche Stellen zum Thema gehören.
- **Zuordnung Pfad → Dokument.** Welches Dokument ein Pfad eines Auftrags betrifft, kommt aus
  den Dokumenten: aus ihren Stellen im Code (`pfad:zeile`, vor allem unter
  `## Key places in the code`), ihrem Thema und den Zusatzquellen ihrer `hints`. Ein Pfad kann
  mehrere Dokumente betreffen. Ein Pfad, den kein Dokument abdeckt, führt in Schritt 4 zu
  `neu` oder zu einem bestehenden Thema.
- **Zuständig** ist dieser Lauf nur für eigene Dokumente — `generated.by: k-docs-code` oder
  der Altwert `k-code2docs`. Im vollen Lauf sind das die, deren Thema im Scope liegt. Im
  Teil-Lauf und in der Wiederaufnahme sind es die, für die ein Auftrag vorliegt: Der Auftrag
  tritt an die Stelle von „im Scope“. Alle anderen — mit `generated.by:
  ks-overlay-repo-analyse`, ohne `generated.by`, mit einem Thema außerhalb des Scans oder
  ohne Auftrag — bleiben im Arbeitsverzeichnis **unverändert liegen**, samt `hints` und
  `questions`.
- **Außerhalb des Scans.** Im vollen Lauf bleibt ein eigenes Dokument, dessen Thema
  außerhalb des Scans liegt, ebenso liegen — und veraltet, solange kein Lauf es erfasst.
  Merke dir, welcher Pfad es abdecken würde; Schritt 4 schlägt dafür einen `hint` mit dieser
  Zusatzquelle vor, soweit ein `hint` den Pfad nach dem Scope erreicht. Im Teil-Lauf liegt ein
  Thema nur außerhalb, wenn die Ausschlüsse den Pfad eines Auftrags ausnehmen; ein Dokument
  ohne Auftrag bekommt keinen Vorschlag.

## Schritt 4 — Analysieren und zerlegen

Setze `GENERATED_AT` jetzt, einmal je Lauf (`date -Iseconds`), wenn du Sub-Agenten
einsetzt. Lies `k-playbook:/commands/_docs-code/analyse.md`: worauf die Analyse achtet, was im vollen
Lauf und im Teil-Lauf gelesen wird, und wie du bei sehr großen Repos Sub-Agenten je Thema
einsetzt. In der Wiederaufnahme ohne rohe Aufträge entfällt die Analyse.

**Zerlegen.** Jeder Auftrag aus `RAW_ORDERS` wird in Aufträge je Dokument geteilt:

- Im vollen Lauf bekommt jedes Dokument, für das der Lauf zuständig ist, und jedes neue Thema
  eine Zeile: `neu`, `umschreiben` oder `entfällt`.
- Im Teil-Lauf bekommen die Dokumente, die ein Pfad des Auftrags nach der Zuordnung aus
  Schritt 3 betrifft, `umschreiben`. Ein Pfad, den kein Dokument abdeckt, führt zu `neu` oder
  gehört zu einem bestehenden Thema, das dann `umschreiben` bekommt. Ein Dokument, dessen
  Thema mit der Änderung weggefallen ist, bekommt `entfällt`.
- Ist die Zuordnung eines Pfads unklar, fragst du den Nutzer, statt zu raten.
- Betrifft ein Teil-Lauf jedes Dokument, für das er zuständig ist, nennt die Liste unter
  „Aus dem Scan“, dass Code, den kein Dokument und kein Auftrag abdeckt, nur ein voller Lauf
  (`/k-docs code`) erfasst. Ein Dokument, dessen Thema einen ganzen Bereich umfasst, liest
  dafür seinen ganzen Bereich, nicht nur die Pfade der Aufträge.
- Zugeordnet wird nach dem heutigen Stand der Pfade, nicht nach dem `anlass`. Ein gelöschter
  Pfad führt zu `umschreiben` des Dokuments, das ihn beschreibt, oder zu `entfällt`, wenn
  vom Thema nichts bleibt.
- Ein Pfad, den die Ausschlüsse ausnehmen, führt zu keinem Dokument. Folgt aus einem Auftrag
  deshalb kein Dokument, steht er unter „Ohne Dokument“, und Schritt 7 nennt den Nachzug als
  offen.
- Liegt für ein Dokument schon ein zerlegter Auftrag offen (`SPLIT_ORDERS`), gilt seine
  Zeile mit; er wird ergänzt statt verdoppelt.

Zeige dann eine **kompakte** Liste, nicht die Doku selbst:

```text
Struktur für keph:/docs/generated/code/:

  neu          00-overview            Was das System tut, für wen, grober Ablauf
  umschreiben  02-architektur         Komponenten und wie sie zusammenspielen
  entfällt     07-altes-thema         <Grund>
  bleibt       20-overlay-delta       generated.by: ks-overlay-repo-analyse — nicht behandelt
  bleibt       13-entwicklungsumgebung  außerhalb des Scans (.devcontainer/) — hint vorgeschlagen
  …

Aus dem Scan:  <2–3 Beobachtungen, die den Vorschlag begründen>
Nicht klar:    <1–2 Punkte, die Klärung brauchen>
Neue hints:    <Vorschläge je Dokument, oder keine>
Ohne Dokument: <Auftrag und Pfad, aus dem kein Dokument folgt, mit Grund — oder keine>

neu und entfällt bestätigen — OK / zusammenlegen / trennen / streichen / umbenennen?
```

- **`neu` und `entfällt` bestätigt der Nutzer, `umschreiben` nicht.** Steht in der Liste nur
  `umschreiben`, zeigst du sie und gehst weiter. Ein offener zerlegter Auftrag aus einem
  früheren Lauf mit `neu` oder `entfaellt` wird erneut vorgelegt: Die Bestätigung steht nicht
  im Auftrag.
- **entfällt** schlägst du nur für Dokumente vor, für die dieser Lauf zuständig ist
  (Schritt 3). Jedes andere steht unter **bleibt** und wird nicht angefasst — `publish`
  deutet eine fehlende Datei als Löschung. Im Teil-Lauf nennt die Liste unter **bleibt** nur,
  was ein Auftrag berührt hätte.
- Ein Dokument, das umgeschrieben wird, behält seinen Dateinamen. Ein neues Thema bekommt
  eine freie Nummer im Arbeitsverzeichnis: Einer-Schritte mit bewussten Lücken (`00`, `01`,
  …, `90`), umsortiert wird nicht.
- **Nichts schreiben, bevor `neu` und `entfällt` entschieden sind** — weder in die Queue
  noch ins Arbeitsverzeichnis.

**In die Queue schreiben.** Aus der bestätigten Liste ruft die Sitzung
`k_playbook_docs_orders_split` auf (ersatzweise `k-playbook docs orders split --json` mit der
Eingabe auf stdin). Das Programm legt je Dokument den zerlegten Auftrag an oder ergänzt den
offenen, schließt danach die rohen Aufträge, die darin aufgegangen sind, und die abgelehnten
`neu` und `entfällt`. Die Eingabe: `sitzung` (die Kennung aus dem Briefing), `documents` und
`raw` (die Verweise der Aufträge aus `RAW_ORDERS`, `refs`). Je Dokument:
`dokument`, `aktion` (`neu`, `umschreiben`, `entfaellt`), `pfade` (die Pfade der
aufgegangenen Aufträge, die es betreffen, im vollen Lauf nur `.`), `anlaesse` (je
aufgegangenem Auftrag `datum` und `anlass`), `anlass` (ein Satz, der zusammenfasst),
`entstanden` (das älteste Datum) und `abgelehnt: true` für ein vom Nutzer abgelehntes `neu`
oder `entfällt` — auch ein offener Auftrag aus einem früheren Lauf. Die Antwort nennt je
Dokument `created`, `extended` oder `failed` (mit `code` und `message`), je rohen Auftrag
`closed`, `gone` oder `kept` und `closedRaw`. Scheitert ein Dokument, bleiben die rohen
Aufträge offen; der nächste Lauf zerlegt sie erneut.

Du merkst dir `WORK_ORDERS`: die offenen zerlegten Aufträge nach diesem Schritt, alte wie
neue, mit Verweis; abgelehnte kommen nicht hinein, Schritt 7 nennt sie. Rohe Aufträge, die
seit Schritt 1 dazugekommen sind, bleiben für den nächsten Lauf offen.

## Schritt 5 — Schreiben, je Auftrag

Setze `GENERATED_AT` wie in den Regeln des schreibenden Laufs (`date -Iseconds`), falls
noch nicht in Schritt 4 geschehen.
Arbeite `WORK_ORDERS` der Reihe nach ab: das älteste `entstanden` zuerst, bei gleichem Datum
nach `dokument`. Je Auftrag:

- `neu` oder `umschreiben` — lies `k-playbook:/commands/_docs-code/schreiben.md`, falls noch
  nicht gelesen, und schreibe `<WORK_DIR>/<dokument>` danach: Stand, Prüfung vor dem
  Einfügen, Kopf, Vorlage und Regeln stehen dort. Dasselbe Modul gibst du einem Sub-Agenten
  als Anleitung (Schritt 4).
- `entfaellt` — lösche `<WORK_DIR>/<dokument>`; `publish` löscht, was im Arbeitsverzeichnis
  fehlt.
- Danach ist der Auftrag **abgearbeitet**; du merkst ihn dir in `WORKED`. Ein Dokument ohne
  Auftrag fasst du nicht an.

Unklarheiten — auch die der Sub-Agenten — legst du vor `publish` vor; was offen bleibt, nennt
Schritt 7. Nach jedem Auftrag kurz melden: Auftrag, Dateiname und Zeilenzahl, nicht der
Inhalt.

## Schritt 6 — publish und Aufträge schließen

Lies `k-playbook:/commands/_docs/publish.md` und rufe `publish` mit `erzeuger: code` auf; wann,
ist frei: nach jedem Auftrag oder nach mehreren. `allow_hints: true` nur, wenn der Nutzer in
Schritt 4 geänderte, neue oder mit einem Dokument entfallende `hints` bestätigt hat.

**Abschluss eines Auftrags.** Nach jedem `publish` schließt du Aufträge einzeln, je einen
mit `k_playbook_doc_done`:

- Geschlossen wird ein Auftrag aus `WORKED`, dessen Dokument dieses `publish` mit `created`,
  `written`, `deleted` oder `unchanged` meldet.
- `publish` meldet auch unberührte Dokumente als `unchanged`. Ein Auftrag, der nicht in
  `WORKED` steht, bleibt deshalb offen, was immer `publish` über sein Dokument sagt.
- Offen bleibt ein Auftrag bei `conflict`, bei `failed` und bei jeder Ablehnung des ganzen
  Abgleichs (`invalid_input`, `hints_changed`) — bis ein späteres `publish` sein Dokument mit
  einem der vier Zustände meldet.

**Neu auschecken.** Sind nach einem vollständigen Abgleich (`cleaned`) noch Aufträge aus
`WORK_ORDERS` offen, rufst du vor dem nächsten Auftrag `k_playbook_docs_checkout` erneut auf
und arbeitest im neuen `WORK_DIR` weiter. Bricht der Lauf ab, bleiben die nicht
geschlossenen zerlegten Aufträge offen; der nächste Lauf setzt an ihnen wieder an.

## Schritt 7 — Abschluss

**Ausschlüsse speichern** wie im Modul zum Scope, wenn der Nutzer im Preflight `NEW_EXCLUDES`
ergänzt hat. Kompakte Zusammenfassung:

- Je Dokument der Zustand aus `publish`, gezählt nach neu, umgeschrieben, entfallen,
  unverändert; dazu die unberührten Dokumente anderer Schreiber.
- Geänderte `hints` (nur mit Bestätigung) und die offen gebliebenen Unklarheiten aus
  Schritt 5.
- Der beschriebene Stand aus Schritt 1 (Branch bzw. `<ref>` und Commit).
- Offene Dateien mit Grund, falls `publish` nicht aufgeräumt hat.
- Die Doku-Aufträge, je mit Verweis:
  - rohe, die zerlegt wurden und nach `done/` gewandert sind — getrennt die, aus denen kein
    Dokument folgte, mit Grund; liegt ihr Pfad außerhalb des Scans, ist der Nachzug offen;
  - zerlegte, die geschlossen wurden, und die, die offen geblieben sind, mit Grund;
  - abgelehnte `neu` und `entfällt`, ohne `publish` geschlossen;
  - wartende, mit Restzeit, und rohe, die während des Laufs dazugekommen sind: Sie bleiben
    für den nächsten Lauf.
- Ausdrücklich: geschrieben wurde nur nach `keph:/docs/generated/code/`, ins
  Arbeitsverzeichnis `WORK_DISPLAY_PATH`, nach `keph:/queue/docs/` (zerlegte Aufträge) und
  `keph:/queue/docs/done/` (abgeschlossene) — und, nur wenn der Nutzer es bestätigt hat, über
  `k_playbook_docs_exclude_add` nach `docs.exclude` in `K-PLAYBOOK.yaml`.
- Folge-Command: keiner, solange keine Aufträge offen sind. Bleiben zerlegte offen oder
  warten rohe, ist es `/k-docs-code` — sofort über das ganze Projekt mit `/k-docs code`
  davor. Den Index über `keph:/docs/` erzeugt das Indizieren, nicht ein Command.

## Fehlerfälle

- `kephalaion.ok` ist nicht `true` oder ein Werkzeug meldet
  `kephalaion_unavailable` → kein Lauf; `kephalaion.message` nennen und auf die
  Projekt-Collection auf der Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf die Platte.
- `$ARGUMENTS` ist nicht leer → kein Lauf; die Meldung aus Schritt 1.
- Kein offener Auftrag mit `erzeuger: code`, oder nur wartende → kein Lauf; „Nichts zu tun“.
- `k_playbook_docs_checkout` meldet `existing` → fragen: weitermachen oder verwerfen.
- Ein Werkzeug ist unbekannt (`k_playbook_docs_checkout`, `_scope`, `_orders`,
  `_orders_split`, `k-playbook docs` meldet `unbekanntes Kommando`) oder `k_playbook_doc_done`
  meldet `invalid_input` für die Sorte `docs` → die Installation ist älter als dieser
  Command: melden, `/k-upgrade` nennen, nichts von Hand rechnen, anlegen oder nach Kephalaion
  schreiben.
- Das Schreiben eines zerlegten Auftrags scheitert → die rohen Aufträge bleiben offen; der
  Lauf endet vor Schritt 5 und nennt, was schon in der Queue steht. `outcome_unknown` →
  nicht wiederholen, mit `k_playbook_doc_list` (`kind: docs`, `number` oder `name` des
  geschriebenen Auftrags) nachsehen.
- `publish` meldet `conflict`, `hints_changed` oder `failed` → wie im Modul zu `publish`; die
  betroffenen Aufträge bleiben offen.
- Der Scan findet keine Kernlogik, nur Framework-Standard → das sagen und keine Doku
  erfinden.

## Anti-Muster (nicht tun)

- **Datei-für-Datei-Beschreibung** und **aus dem Auftrag oder der alten Fassung schreiben**:
  Die Doku beschreibt Bedeutung, und `anlass`, Rumpf und bisheriger Text zeigen nur, wo
  nachzusehen ist.
- **Themen zersplittern.** Dreißig winzige Dokumente statt zehn sinnvoller; eine Frage
  gehört in genau eine Datei.
- **Ohne Auftrag laufen.** Kein Dokument wird geschrieben oder gelöscht, für das kein
  Auftrag vorliegt; im Teil-Lauf wird nicht das ganze Projekt gescannt.
- **Einen wartenden Auftrag anfassen** oder **einen Auftrag zu früh schließen.** Nicht vor dem
  `publish`, das sein Dokument meldet, und nie einen, der in diesem Lauf nicht abgearbeitet
  wurde — `unchanged` allein sagt nichts.
- **Zerlegte Aufträge verdoppeln.** Ein offener Auftrag für dasselbe Dokument wird ergänzt.
- **Fremde Dokumente löschen oder umschreiben.** Was `ks-overlay-repo-analyse` geschrieben
  hat, außerhalb des Scans liegt oder keinen Auftrag hat, bleibt unverändert im
  Arbeitsverzeichnis — `publish` löscht, was dort fehlt.
- **`hints` oder Konflikte still ändern.** Neue `hints` nur nach Bestätigung und mit
  `allow_hints`; kein `discard` und kein erneutes Schreiben über eine fremde Änderung hinweg
  ohne Entscheidung des Nutzers.
- **Die Doku an `publish` vorbei in die Collection schreiben** oder die Aufträge von Hand
  anlegen, ergänzen oder die rohen schließen — das tut `k_playbook_docs_orders_split`.
  Geschlossen werden von Hand nur die zerlegten, mit `k_playbook_doc_done` nach „Abschluss
  eines Auftrags“ (Schritt 6). Kephalaion `write` ist hier kein Weg für Dokumente.
- **`K-PLAYBOOK.yaml` selbst lesen oder bearbeiten, `README.md` oder `index.md` schreiben,
  auf die Platte ausweichen.** Ausschlüsse kommen aus dem Programm; die README schreibt
  `publish`, den Index das Indizieren; ohne Kephalaion gibt es keinen Lauf.
