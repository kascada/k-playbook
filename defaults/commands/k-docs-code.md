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
  Schritt 4.
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

Aus dem Briefing brauchst du `context.kephalaion` — `ok`, `address`, `message` —, dazu
`session` und `context.now` (`date`, `timestamp`). Ohne `ok: true` endet der Command hier
(siehe die Ablage oben).

**Kein Argument.** `/k-docs-code` nimmt kein Argument. Ist `$ARGUMENTS` nicht leer, endet der
Command hier mit dieser Meldung — bevor etwas gesucht, geholt oder geschrieben wird:

```text
/k-docs-code nimmt kein Argument: „<$ARGUMENTS>“ wird nicht ausgewertet, nichts ist gelaufen.
Welche Dokumente drankommen, bestimmen die Doku-Aufträge (pfade im Kopf).
Voller Lauf: /k-docs code reiht ihn ein, danach /k-docs-code.
```

**Scope.** Es gilt der Abschnitt „Scope“ der Docs-Ablage oben — `SCAN_ROOT`, Stand, die
Ausschlüsse und `NEW_EXCLUDES` —, mit vier Abweichungen, die nur dieser Command hat. Sonst
wird er hier nicht wiederholt.

- **Kein `EXTRA_PATH`.** Was der Abschnitt „Scope“ über `$ARGUMENTS` sagt, gilt hier nicht.
  Ein versteckter Pfad kommt nur über einen `hint` in den Scan. Ein Pfad, den `docs.exclude`
  ausnimmt, ist nicht erreichbar: Die Ausschlüsse gelten auch für Zusatzquellen aus `hints`.
- **Die Dokumente kommen aus den Aufträgen.** Die `pfade` der Aufträge bestimmen, welche
  Dokumente drankommen, nicht der Scan.
- **Gelesen wird innerhalb von `SCAN_ROOT`.** Für das Dokument eines Auftrags liest du die
  bisherige Fassung und was sein Thema braucht, auch außerhalb der Pfade des Auftrags. Der
  Rest des Projekts wird nur im vollen Lauf gescannt.
- **Die Ausschlüsse gelten** auch für die Pfade eines Auftrags: Ein ausgenommener Pfad führt
  zu keinem Dokument.

**Aufträge lesen.** Kopf, Reifegrade und Wartezeit stehen in
`k-playbook:/instructions/on-demand/doku-auftraege.md`; lies sie mit `k_playbook_doc_read`,
wenn sie dir nicht vorliegt.

1. Suche die offenen Aufträge mit `erzeuger: code` wie unter „Suche“ im Abschnitt
   „Doku-Aufträge“ der Ablage oben und lies jeden. Dazu ein `k_playbook_doc_list` mit
   `kind: docs` (ohne `done`): Es nennt je Auftrag `updated`, den Zeitpunkt seiner letzten
   Änderung.
2. Ein Auftrag mit `dokument` im Kopf ist **zerlegt**, sonst **roh**. Offene zerlegte
   Aufträge heißen: Ein früherer Lauf ist unterbrochen. Dieser Lauf setzt dort wieder an;
   eine Frist haben sie nicht.
3. Trägt ein roher Auftrag `pfade: .`, ist das der **volle Lauf**: Er nimmt alle offenen
   rohen Aufträge auf, auch die wartenden. Sonst ist es ein **Teil-Lauf**: Du nimmst nur die
   reifen rohen Aufträge. Ein roher Auftrag **wartet**, solange seine letzte Änderung
   (`updated`) jünger ist als die Wartezeit, gemessen an `context.now.timestamp`; Wert und
   Rechnung stehen im Abschnitt „Wartezeit“ von `doku-auftraege.md` und nur dort. Wartende
   Aufträge bleiben unverändert offen und nehmen an nichts teil, auch wenn ein reifer Auftrag
   einen ihrer Pfade teilt.
4. **Zusammenlegen.** Zwei rohe Aufträge sind abhängig, wenn ein Pfad des einen gleich einem
   Pfad des anderen ist oder darin liegt. Abhängige gelten als ein Auftrag mit vereinigten
   Pfaden und gesammelten Anlässen, auch über mehrere hinweg; sein `entstanden` ist das
   älteste Datum. `.` nimmt alle auf: Im vollen Lauf bleibt ein einziger Auftrag. Zusammengelegt
   wird nur für diesen Lauf; in der Queue ändert das nichts.

Du merkst dir `RAW_ORDERS` — die rohen Aufträge nach dem Zusammenlegen, je mit den Verweisen
der Aufträge, die darin aufgegangen sind —, `SPLIT_ORDERS` — die offenen zerlegten — und
`WAITING` — die wartenden mit ihrer Restzeit.

**Nichts zu tun.** Gibt es keinen offenen Auftrag mit `erzeuger: code`, endet der Command
hier: Sag das und nenne `/k-docs code` als Weg zum vollen Lauf. Sind nur wartende offen
(`RAW_ORDERS` und `SPLIT_ORDERS` leer), endet er ebenso: Nenne die wartenden mit ihrer
Restzeit und `/k-docs code` als Weg, sofort zu laufen. Nichts wurde geholt oder geschrieben.

Zeige den Preflight und lass ihn **einmal** bestätigen; der Nutzer darf dabei Ausschlüsse
ergänzen (`NEW_EXCLUDES`, wie im Abschnitt „Scope“). Die Zeile `Zusätzlich` der Scope-Zeilen
zeigt hier immer `—`.

```text
/k-docs-code — Preflight
─────────────────────────────────────
Collection    <context.kephalaion.address>
Ziel          keph:/docs/generated/code/
<die Scope-Zeilen aus dem Abschnitt „Scope“>
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

Rufe `k_playbook_docs_checkout` mit `projectDir` = `context.project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs checkout code --json`).

- `state: checked_out` — das Arbeitsverzeichnis ist geholt. `WORK_DIR` ist `dir` aus der
  Antwort, `WORK_DISPLAY_PATH` derselbe Pfad ab `context.project.dir`
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
  Eine Zusatzquelle aus einem `hint` gehört zum Scan wie im Abschnitt „Scope“.
- Den bisherigen Text nimmst du als Anregung, nicht als Vorlage zum Abschreiben. Er sagt
  aber, welche Stellen zum Thema gehören.
- **Zuordnung Pfad → Dokument.** Welches Dokument ein Pfad eines Auftrags betrifft, kommt aus
  den Dokumenten: aus ihren Stellen im Code (`pfad:zeile`, vor allem unter
  `## Key places in the code`), ihrem Thema und den Zusatzquellen ihrer `hints`. Ein Pfad kann
  mehrere Dokumente betreffen. Ein Pfad, den kein Dokument abdeckt, führt in Schritt 4 zu
  `neu` oder zu einem bestehenden Thema.
- **Zuständig** ist dieser Lauf nur für eigene Dokumente — `generated.by: k-docs-code` oder
  der Altwert `k-code2docs`. Im vollen Lauf sind das die, deren Thema im Scope aus Schritt 1
  liegt. Im Teil-Lauf und in der Wiederaufnahme sind es die, für die ein Auftrag vorliegt:
  Der Auftrag tritt an die Stelle von „im Scope“. Alle anderen — mit
  `generated.by: ks-overlay-repo-analyse`, ohne `generated.by`, mit einem Thema außerhalb
  des Scans oder ohne Auftrag — bleiben im Arbeitsverzeichnis **unverändert liegen**, samt
  `hints` und `questions`.
- **Außerhalb des Scans.** Im vollen Lauf bleibt ein eigenes Dokument, dessen Thema
  außerhalb des Scans liegt, ebenso liegen — und veraltet, solange kein Lauf es erfasst.
  Merke dir, welcher Pfad es abdecken würde; Schritt 4 schlägt dafür einen `hint` mit dieser
  Zusatzquelle vor. Im Teil-Lauf liegt ein Thema nur außerhalb des Scans, wenn die
  Ausschlüsse den Pfad eines Auftrags ausnehmen. Den `hint` schlägt Schritt 4 dann nur vor,
  soweit ein `hint` den Pfad nach dem Abschnitt „Scope“ erreicht; ein Dokument ohne Auftrag
  bekommt keinen Vorschlag.

## Schritt 4 — Analysieren und zerlegen

**Keine Datei-für-Datei-Beschreibung.** Ergebnis dieses Schritts ist ein Verständnis nach
Bedeutung, nicht nach Verzeichnissen. Suche nach:

- **Fachlichen Subsystemen** — eigenständige Verantwortungsbereiche, gebündelt über
  Datenmodelle, Aufrufwege und gemeinsames Vokabular.
- **Querschnittsthemen** — Auth, Konfiguration und Secrets, Logging, Fehlerbehandlung,
  Persistenz, HTTP-Oberfläche, Hintergrundjobs, Feature-Flags, Wiederholungen und
  Zeitgrenzen.
- **Einstiegspunkten** — was startet was.
- **Externen Kopplungen** — Datenbanken, Queues, fremde APIs, Auth-Anbieter, Cloud-Dienste:
  wo sie im Code sitzen und wo sie konfiguriert sind.
- **Datenmodell** — zentrale Entitäten, wo definiert, wo umgeformt.
- **Kernlogik gegen Framework-Standard und Boilerplate** — nur Kernlogik verdient tiefe Doku.
- **Altlasten** — sichtbare Deprecations, TODOs, alte Muster, getrennt notiert.

Nicht aufnehmen: Framework-Standard ohne Eigenanteil, Kommentare, die den Code
nacherzählen, Signaturen einzelner Funktionen — dazu, was die Ablage oben ausschließt.
Was der Scan an möglichen Fehlern und Altlasten findet, steht unter `## Edge cases` oder in
einem Dokument für bekannte Macken, nicht in Befunden.

**Was analysiert wird.** Im vollen Lauf der ganze Scope aus Schritt 1. Im Teil-Lauf nur, was
die Dokumente der Aufträge brauchen: die Pfade der Aufträge und, je betroffenem Dokument,
die Stellen, die zu seinem Thema gehören. Der Rest des Projekts wird nicht gescannt. In der
Wiederaufnahme ohne rohe Aufträge entfällt die Analyse hier; Schritt 5 liest je Auftrag.

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
  (`/k-docs code`) erfasst. Ein Dokument, dessen Thema einen ganzen Bereich umfasst (etwa
  alle Pakete), liest dafür seinen ganzen Bereich, nicht nur die Pfade der Aufträge.
- Zugeordnet wird nach dem heutigen Stand der Pfade, nicht nach dem `anlass`: Hat eine
  spätere Änderung die eines Auftrags überholt, zählt, was die Pfade jetzt enthalten. Ein
  gelöschter Pfad führt zu `umschreiben` des Dokuments, das ihn beschreibt (seine Stellen
  fallen weg), oder zu `entfällt`, wenn vom Thema nichts bleibt.
- Ein Pfad, den die Ausschlüsse ausnehmen, führt zu keinem Dokument. Folgt aus einem Auftrag
  deshalb kein Dokument, steht er unter „Ohne Dokument“, und Schritt 7 nennt den Nachzug als
  offen.
- Liegt für ein Dokument schon ein zerlegter Auftrag offen (`SPLIT_ORDERS`), gilt seine
  Zeile mit; er wird unten ergänzt statt verdoppelt.

**Sub-Agenten.** Bei sehr großen Repos teilst du Analyse und Schreiben auf, einen
Sub-Agenten je Thema, nach den Regeln der Ablage oben. Jeder bekommt zusätzlich
`SCAN_ROOT`, die Ausschlüsse, den Stand aus Schritt 1, sein Thema, die Pfade und Anlässe
seines Auftrags, `WORK_DIR`, die bisherige Fassung seines Dokuments samt `hints`, die Vorlage
aus Schritt 5, die Regel „nur Bedeutung, keine Zeilenlisten“ und den Abschnitt „Nur was der
Code jetzt zeigt“ aus Schritt 5; er meldet außerdem Dateiname und Zeilenzahl. Ein Sub-Agent schreibt keine Aufträge und schließt keine ab.

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
  was ein Auftrag berührt hätte; die übrigen Dokumente bleiben ohne Zeile liegen.
- Ein Dokument, das umgeschrieben wird, behält seinen Dateinamen. Ein neues Thema bekommt
  eine freie Nummer im Arbeitsverzeichnis: Einer-Schritte mit bewussten Lücken (`00`, `01`,
  …, `90`), umsortiert wird nicht.
- **Nichts schreiben, bevor `neu` und `entfällt` entschieden sind** — weder in die Queue
  noch ins Arbeitsverzeichnis.

**In die Queue schreiben.** Aus der Liste wird je Dokument ein zerlegter Auftrag, mit Kopf
und Rumpf aus `doku-auftraege.md`. In der Liste steht `entfällt`, im Kopf der Wert
`entfaellt`.

1. Je Zeile `neu`, `umschreiben` oder `entfällt`, für deren Dokument noch kein zerlegter
   Auftrag offen ist: `k_playbook_doc_create` mit `kind: docs` und dem Kurznamen
   `code-<dokument ohne .md>`, gleich danach Kephalaion `write` mit `collection` =
   `context.kephalaion.address` und `name` = Verweis ohne `keph:/`. `dokument` und `aktion`
   kommen aus der Zeile; `pfade` sind die Pfade der aufgegangenen Aufträge, die dieses
   Dokument betreffen, im vollen Lauf nur `.`; `sitzung` ist `session`; `entstanden` ist das
   älteste Datum der aufgegangenen Aufträge. Der Rumpf nennt je aufgegangenem Auftrag, der
   dieses Dokument betrifft, eine Zeile mit Datum und Anlass — als Text, ohne Verweis.
2. Ist für das Dokument schon ein zerlegter Auftrag offen, ergänzt du ihn, statt einen
   zweiten anzulegen: mit `k_playbook_doc_read` lesen, `pfade` vereinigen, die neuen Zeilen
   an den Rumpf, `aktion` nach dieser Liste, `entstanden` bleibt das älteste Datum; dann
   Kephalaion `write` mit `base_revision` gleich der gelesenen Revision. Bei
   `stale_revision` liest du neu und ergänzt auf dem neuen Stand. Ist nichts zu ergänzen,
   schreibst du nichts.
3. Auch ein `neu` oder `entfällt`, das der Nutzer abgelehnt hat, steht als Auftrag in der
   Queue — neu geschrieben nach 1 oder schon offen aus einem früheren Lauf. Diesen Auftrag
   schließt du gleich mit `k_playbook_doc_done`, ohne `publish`; in `WORK_ORDERS` kommt er
   nicht. Schritt 7 nennt ihn.
4. Erst wenn alle zerlegten Aufträge in der Queue stehen, wandern die rohen Aufträge, die in
   `RAW_ORDERS` aufgegangen sind, nach `done/`: je einer mit `k_playbook_doc_done` — auch
   die, aus denen kein Dokument folgt. Scheitert das Schreiben eines zerlegten Auftrags,
   bleiben die rohen offen; der nächste Lauf zerlegt sie erneut und ergänzt, was schon
   dasteht.

Du merkst dir `WORK_ORDERS`: die offenen zerlegten Aufträge nach diesem Schritt, alte wie
neue, je mit Verweis. Rohe Aufträge, die seit Schritt 1 dazugekommen sind, bleiben für den
nächsten Lauf offen; gesucht wird nicht noch einmal.

## Schritt 5 — Schreiben, je Auftrag

Setze zuerst `GENERATED_AT` wie in der Ablage oben (`date -Iseconds`), falls Sub-Agenten
ihn nicht schon in Schritt 4 brauchten.

Arbeite `WORK_ORDERS` der Reihe nach ab: das älteste `entstanden` zuerst, bei gleichem Datum
nach `dokument`. Je Auftrag:

- `neu` oder `umschreiben` — lies die bisherige Fassung, falls es eine gibt, und was das
  Thema des Dokuments braucht, auch außerhalb der Pfade des Auftrags; dann schreibst du
  `<WORK_DIR>/<dokument>` mit den Datei-Werkzeugen.
- `entfaellt` — lösche `<WORK_DIR>/<dokument>`; `publish` löscht, was im Arbeitsverzeichnis
  fehlt.
- Danach ist der Auftrag **abgearbeitet**; du merkst ihn dir in `WORKED`. Ein Dokument ohne
  Auftrag fasst du nicht an.

**Nur was der Code jetzt zeigt.** Aufträge sammeln sich über Tage. Was ihr `anlass` oder ihr
Rumpf beschreibt, kann eine spätere Änderung überholt haben: Eingeführtes wurde wieder
entfernt, ein Name hat sich geändert, ein Pfad ist gelöscht. Dasselbe gilt für die bisherige
Fassung des Dokuments. Beide sagen nur, wo du hinschaust, nie, was in der Doku steht.

- Quelle jeder Aussage ist der Stand aus Schritt 1. Was dort nicht mehr vorkommt, kommt nicht
  in die Doku, auch wenn ein Auftrag oder die bisherige Fassung es nennt.
- Entferntes wird aus dem Dokument gestrichen, nicht als Geschichte beschrieben („wurde mit
  Task … entfernt“, „früher …“). Die Doku beschreibt den Stand, nicht den Weg dorthin.
- **Vor dem Einfügen prüfen.** Bevor eine Aussage ins Dokument kommt, prüfst du sie erneut
  am Code: Jede Stelle `pfad:zeile` existiert und zeigt auf das Beschriebene; jeder
  genannte Bezeichner, jedes Feld, jeder Befehl, jede Option und jede Meldung kommt im Code
  so vor; kein Satz stützt sich allein auf einen Auftrag oder die bisherige Fassung. Was sich
  nicht bestätigen lässt, kommt nicht hinein; ist es wichtig, wird es eine Unklarheit.
- Das gilt auch für Sub-Agenten. Was sie schreiben, prüft der Lauf vor `publish` noch einmal
  nach denselben Punkten — mindestens alle Stellen `pfad:zeile` und alle Bezeichner — und
  berichtigt, was nicht stimmt.

Dateinamen `<NN>-<slug>.md`, klein, nur `a–z`, `0–9`, `.`, `_`, `-`. Keine `README.md`,
keine anderen Dateien als `.md`. Der Kopf ist der aus dem Abschnitt „Der Kopf“ der Ablage
oben; die Vorlage zeigt, wie dieser Command ihn belegt — `sources` nur, wenn Quellen außer
dem Code genutzt wurden.

```markdown
---
title: "<Titel>"
description: "<Ein Satz: was dieses Dokument erklärt.>"
generated: { by: k-docs-code, at: <GENERATED_AT> }
hints: [ <unverändert übernommen; das Feld fehlt, wenn das Dokument keine hatte> ]
questions: [ "<Frage, die ein Neuer in der ersten Woche stellt>", … ]
---

# <Titel>

<1–2 Sätze: worum es geht, aus welcher Sicht.>

## Purpose

<Der fachliche oder technische Zweck.>

## How it works

<Die Kernidee: Datenfluss, Abhängigkeiten, Aufrufweg. Stellen im Code als `pfad:zeile`.>

## Key places in the code

- `pfad/zur/datei.go:45` — <was dort steht, in einem Satz>

## Edge cases

<Nicht Offensichtliches, versteckte Verträge, bekannte Fehler.>

## Related topics

- `<NN>-<slug>.md` — <in welchem Zusammenhang>
```

Regeln:

- Neue `hints` schlägt Schritt 4 vor; eingetragen werden sie erst nach Bestätigung, wie im
  Kopf beschrieben.
- Stellen im Code immer als `pfad:zeile`, mit dem Pfad ab `SCAN_ROOT` und an jeder Stelle
  ganz — nie nur der Dateiname, auch nicht, wenn der volle Pfad im selben Abschnitt schon
  stand: Namen wie `server.go` gibt es in mehreren Paketen, und ein Leser springt über die
  Suche mitten in einen Abschnitt. Verwandte Themen im selben Verzeichnis mit ihrem
  Dateinamen, andere Teile der Projektdoku als `keph:`-Verweis — ein isoliertes Dokument ist
  ein Fehler. Auf einen Auftrag verweist kein Dokument.
- Keine erfundenen Erklärungen: Unklarheiten — auch die der Sub-Agenten — legst du wie in
  der Ablage oben vor `publish` vor; was offen bleibt, nennt Schritt 7.
- **Englisch schreiben.** Kopf (`title`, `description`, `questions`) und Text sind englisch,
  auch wenn Code, Kommentare oder der bisherige Text deutsch sind; die Überschriften der
  Vorlage stehen wörtlich so. Bezeichner, Pfade, Command-Namen und Meldungen aus dem Code
  bleiben unverändert in Code-Schrift. Bestehende `hints` übersetzt du nicht — sie zu
  ändern braucht die Bestätigung wie im Kopf beschrieben —, neue schlägst du englisch vor.
- Keine Fließtextwände.

Nach jedem Auftrag kurz melden: Auftrag, Dateiname und Zeilenzahl, nicht der Inhalt.

## Schritt 6 — publish und Aufträge schließen

Wann `publish` läuft, ist frei: nach jedem Auftrag oder nach mehreren.

Rufe `k_playbook_docs_publish` mit `projectDir` = `context.project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs publish code --json`). `allow_hints: true`
nur, wenn der Nutzer in Schritt 4 geänderte, neue oder mit einem Dokument entfallende
`hints` bestätigt hat.

Melde die Antwort je Dokument: `created`, `written`, `deleted`, `unchanged`, `conflict`
oder `failed`, mit Revision; dazu `hintsChanged`, `readme` und ob aufgeräumt wurde
(`cleaned`).

- `invalid_input` — eine Ablehnung im Arbeitsverzeichnis (fehlendes Frontmatter, falscher
  Name, `README.md`, andere Datei als `.md`). Nichts wurde geschrieben. Die Stellen
  beheben und `publish` erneut aufrufen.
- `hints_changed` — `hints` weichen vom checkout ab, oder ein Dokument mit `hints` wird
  gelöscht oder neu angelegt. Nichts wurde geschrieben. Dem Nutzer die genannten Dokumente
  vorlegen; erst nach Bestätigung mit `allow_hints: true` erneut aufrufen, sonst die
  `hints` zurücksetzen.
- `conflict` — das Dokument wurde seit dem checkout in Kephalaion geändert, angelegt oder
  gelöscht; die eigene Fassung wurde nicht geschrieben. **Vorlegen, nicht still
  nachziehen.** Der Nutzer entscheidet: eigene Änderung verwerfen, oder die eigene Fassung
  sichern, mit `k_playbook_docs_checkout` und `discard: true` neu holen — was schon
  veröffentlicht ist, kommt dabei mit —, die Änderung auf den neuen Stand übertragen und
  erneut veröffentlichen.
- `failed` mit `kephalaion_unavailable` — beim Hub ist nichts angekommen. Später erneut
  `publish`; es schreibt dann nur noch den Rest.
- `failed` mit `outcome_unknown` — der Vorgang kann angekommen sein. **Nicht** noch einmal
  veröffentlichen, sondern mit Kephalaion `read` nachsehen und dem Nutzer berichten.

**Abschluss eines Auftrags.** Nach jedem `publish` schließt du Aufträge einzeln, je einen
mit `k_playbook_doc_done`:

- Geschlossen wird ein Auftrag aus `WORKED`, dessen Dokument dieses `publish` mit `created`,
  `written`, `deleted` oder `unchanged` meldet.
- `publish` meldet auch unberührte Dokumente als `unchanged`. Ein Auftrag, der nicht in
  `WORKED` steht, bleibt deshalb offen, was immer `publish` über sein Dokument sagt.
- Offen bleibt ein Auftrag bei `conflict`, bei `failed` und bei jeder Ablehnung des ganzen
  Abgleichs (`invalid_input`, `hints_changed`) — bis ein späteres `publish` sein Dokument mit
  einem der vier Zustände meldet.
- Die Rückmeldungen von `k_playbook_doc_done` stehen im Abschnitt „Doku-Aufträge“ der Ablage
  oben.

**Neu auschecken.** Ein vollständiger Abgleich räumt das Arbeitsverzeichnis (`cleaned` ist
wahr). Sind danach noch Aufträge aus `WORK_ORDERS` offen, rufst du vor dem nächsten Auftrag
`k_playbook_docs_checkout` erneut auf und arbeitest im neuen `WORK_DIR` weiter. Solange etwas
offen ist (`open`), bleiben Arbeitsverzeichnis und Merkdatei liegen.

Bricht der Lauf ab, bleiben die nicht geschlossenen zerlegten Aufträge offen; der nächste
Lauf setzt an ihnen wieder an.

## Schritt 7 — Abschluss

**Ausschlüsse speichern** wie im Abschnitt „Scope“ der Ablage, wenn der Nutzer im
Preflight `NEW_EXCLUDES` ergänzt hat.

Kompakte Zusammenfassung:

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

- `context.kephalaion.ok` ist nicht `true` oder ein Werkzeug meldet
  `kephalaion_unavailable` → kein Lauf; `context.kephalaion.message` nennen und auf die
  Projekt-Collection auf der Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf die Platte.
- `$ARGUMENTS` ist nicht leer → kein Lauf; die Meldung aus Schritt 1. Nichts wird gesucht,
  geholt oder geschrieben, kein Auftrag angelegt.
- Kein offener Auftrag mit `erzeuger: code`, oder nur wartende → kein Lauf; wie „Nichts zu
  tun“ in Schritt 1.
- `k_playbook_docs_checkout` meldet `existing` → fragen: weitermachen oder verwerfen.
- `k_playbook_docs_checkout` ist unbekannt, oder `k-playbook docs` meldet
  `unbekanntes Kommando` → die Installation ist älter als dieser Command: melden,
  `/k-upgrade` nennen, nichts von Hand nach Kephalaion schreiben.
- `k_playbook_doc_create` oder `k_playbook_doc_list` meldet `invalid_input` für die Sorte
  `docs` → die Installation ist älter als dieser Command: melden, `/k-upgrade` nennen, keinen
  Auftrag von Hand anlegen.
- Das Schreiben eines zerlegten Auftrags scheitert (`kephalaion_unavailable`) → die rohen
  Aufträge bleiben offen, nichts wird nach `done/` verschoben; der Lauf endet vor Schritt 5
  und nennt, was schon in der Queue steht. `outcome_unknown` → nicht wiederholen, mit
  `k_playbook_doc_list` (`kind: docs`) nachsehen.
- `k_playbook_docs_exclude_add` meldet `rejected`, `unsupported` oder ist unbekannt → wie im
  Abschnitt „Scope“ der Ablage.
- `publish` meldet `conflict`, `hints_changed` oder `failed` → wie in Schritt 6; die
  betroffenen Aufträge bleiben offen.
- Der Scan findet keine Kernlogik, nur Framework-Standard → das sagen und keine Doku
  erfinden.

## Anti-Muster (nicht tun)

- **Datei-für-Datei-Beschreibung.** „modul_x enthält Klasse Y“ steht schon im Code; die
  Doku beschreibt Bedeutung.
- **Aus dem Auftrag oder der alten Fassung schreiben.** `anlass`, Rumpf und bisheriger Text
  zeigen, wo nachzusehen ist; ungeprüft übernommen machen sie die Doku falsch, sobald eine
  spätere Änderung sie überholt hat.
- **Themen zersplittern.** Dreißig winzige Dokumente statt zehn sinnvoller; eine Frage
  gehört in genau eine Datei.
- **Ohne Auftrag laufen.** Kein Dokument wird geschrieben oder gelöscht, für das kein
  Auftrag vorliegt; den vollen Lauf reiht `/k-docs code` ein.
- **Im Teil-Lauf das ganze Projekt scannen.** Gelesen wird, was die Dokumente der Aufträge
  brauchen.
- **Einen wartenden Auftrag anfassen.** Er wird nicht zusammengelegt, nicht zerlegt und nicht
  geschlossen, solange kein Auftrag über `.` ihn aufnimmt.
- **Einen Auftrag zu früh schließen.** Nicht vor dem `publish`, das sein Dokument meldet, und
  nie einen, der in diesem Lauf nicht abgearbeitet wurde — `unchanged` allein sagt nichts.
- **Zerlegte Aufträge verdoppeln.** Ein offener Auftrag für dasselbe Dokument wird ergänzt.
- **Fremde Dokumente löschen oder umschreiben.** Was `ks-overlay-repo-analyse` geschrieben
  hat, außerhalb des Scans liegt oder keinen Auftrag hat, bleibt unverändert im
  Arbeitsverzeichnis — `publish` löscht, was dort fehlt.
- **`hints` still ändern.** Sie sind der einzige von Hand gepflegte Teil; neue nur nach
  Bestätigung, und dann mit `allow_hints`.
- **Konflikte still lösen.** Kein erneutes Schreiben über eine fremde Änderung hinweg,
  kein `discard` ohne Entscheidung des Nutzers.
- **Die Doku direkt in die Collection schreiben.** Nicht mit Kephalaion `write` oder `create`
  an `publish` vorbei — die Regeln (Frontmatter, `hints`, Konflikte) setzt nur `publish`
  durch. Kephalaion `write` dient hier allein den Aufträgen in `keph:/queue/docs/`.
- **`K-PLAYBOOK.yaml` selbst lesen oder bearbeiten.** Ausschlüsse kommen aus dem Briefing
  und werden nur nach Bestätigung über `k_playbook_docs_exclude_add` eingetragen.
- **`README.md` oder `index.md` schreiben.** Die README schreibt `publish`, den Index das
  Indizieren.
- **Auf die Platte ausweichen.** Ohne Kephalaion gibt es keinen Lauf; eine Datei auf der
  Platte ist kein gültiger Ort für die Doku.
