# k-docs-tools

Erzeugt kuratierte, **pitfall-orientierte** Steckbriefe zu den nicht-trivialen Libraries und
Tools des Projekts — Pitfalls, Auth- und Nebenläufigkeitsfallen, Hinweise zur Migration,
empfohlene Idiome; kein Tutorial, dafür gibt es die offizielle Doku. Welche Libraries und
Tools es gibt, sagt das Versionsinventar; dieser Command scannt keine Manifeste selbst.
Geschrieben wird nur in die Collection des Projekts, über ein lokales Arbeitsverzeichnis:
`k_playbook_docs_checkout` holt den Bestand, die Steckbriefe entstehen dort mit den
Datei-Werkzeugen, `k_playbook_docs_publish` gleicht zurück ab.

Produces:

- `keph:/docs/generated/libs/<name>.md` — ein Steckbrief je ausgewähltem Gegenstand des
  Inventars, geschrieben von `k_playbook_docs_publish`.

Dazu verschiebt der Lauf nach einem erfolgreichen `publish` die Doku-Aufträge aus dem
Preflight nach `keph:/queue/docs/done/`.

Nichts sonst. Die `README.md` des Verzeichnisses schreibt `publish` aus ihrer Vorlage; das
Arbeitsverzeichnis unter `.k-playbook/work/docs/libs/` entfernt `publish` nach einem
vollständigen Abgleich. Ausnahmen: Bestätigt der Nutzer im Abschluss, die im Preflight
ergänzten Ausschlüsse zu speichern, trägt `k_playbook_docs_exclude_add` sie in
`docs.exclude` von `K-PLAYBOOK.yaml` ein; bestätigt er in Schritt 1, das fehlende Inventar
zu erheben, schreibt `k-playbook inventory` nach `keph:/docs/generated/versions/inventory.md`.

**Abgrenzung zum Versionsinventar.** Das Inventar ist **vollständig und quellenorientiert**:
jede deklarierte Version aus jeder Quelle, nach Umgebung getrennt, ohne Auswahl. Die
Steckbriefe sind **kuratiert**: eine Auswahl, je Gegenstand ein Dokument, gelesen gegen die
Versionen, die `sources` nennt. Weicht das Inventar später ab, gibt das **Inventar** die
Auskunft; ein Steckbrief wird deswegen nicht aus dem Inventar umgeschrieben, sondern über
diesen Command nachgezogen, wenn sein Inhalt — nicht nur seine Versionszahl — veraltet ist.
Beide sprechen dieselbe Pin-Taxonomie: die aus dem Vertrag des Inventars,
`k-playbook:/docs/handbook/version-inventory.md`, Abschnitt „Pin taxonomy“. Dieser Text
definiert keine eigene.

## Schritt 1 — Briefing, Inventar und Scope

include: _shared/erster-schritt.md

include: _shared/docs.md

Dieser Command ist der Schreiber des Erzeugers `libs`: sein Verzeichnis ist
`keph:/docs/generated/libs/`, seine Dokumente tragen `generated.by: k-docs-tools`.

Aus dem Briefing brauchst du `context.kephalaion` — `ok`, `address`, `message` — und
`context.project.dir`. Ohne `ok: true` endet der Command hier (siehe die Ablage oben).

**Das Inventar.** Lies `keph:/docs/generated/versions/inventory.md` mit `k_playbook_doc_read`
(ersatzweise `k-playbook read`). Aus dem Kopf nimmst du `generated.at` und die Zahl der
Einträge, `generated.stats.entries` — ein Inventar im alten Kopf trägt sie noch unter
`inventory.entries`.

- `not_found` — das Inventar fehlt. Biete an, `k-playbook inventory` jetzt laufen zu lassen
  (der Lauf ist deterministisch; Näheres in `k-playbook:/commands/k-doc-inventory.md`), und
  lies das Inventar danach neu. Lehnt der Nutzer ab oder scheitert der Lauf, endet der
  Command hier: ohne Inventar gibt es keinen Lauf und keinen eigenen Scan der Manifeste.
- Das Inventar ist da, aber leer (`0` Einträge) → das sagen, `/k-doc-inventory` nennen, und
  der Command endet hier.
- Der Kopf ist unvollständig oder nicht lesbar → das als Hinweis im Preflight nennen und
  anbieten, `k-playbook inventory` neu laufen zu lassen; der Rumpf bleibt lesbar.

**Der Scope.** Lies `k-playbook:/commands/_docs/scope.md` und rufe `k_playbook_docs_scope`
auf, mit `pfad` = `EXTRA_PATH`, dem einen Pfad aus `$ARGUMENTS` (leer, wenn keiner da ist).
Der Scope begrenzt hier nur das Zählen der Nutzung im Code (Schritt 4), nicht die Liste der
Kandidaten.

**Aufträge.** Lies `k-playbook:/commands/_docs/auftraege.md` und rufe `k_playbook_docs_orders`
mit `erzeuger: tools` auf. Die offenen Aufträge in `orders` merkst du dir als `ORDERS`; `problems` nennst du im Preflight. Sie
begründen den Lauf; sie engen den Scope nicht ein, der Lauf bleibt ganz. Eine Wartezeit gibt
es hier nicht.

Zeige den Preflight und lass ihn **einmal** bestätigen; der Nutzer darf dabei Ausschlüsse
ergänzen (`NEW_EXCLUDES`, wie im Modul zum Scope).

```text
/k-docs-tools — Preflight
─────────────────────────────────────
Collection    <context.kephalaion.address>
Ziel          keph:/docs/generated/libs/
Inventar      erhoben <generated.at>, <N> Einträge | Hinweis: <Problem im Kopf>
<die Scope-Zeilen aus dem Modul zum Scope>
Aufträge      <N> offen | keine
              <datei> (<entstanden>) — <anlass>
                <pfad>, <pfad>, …
```

Bei „nein“ abbrechen; nichts wurde geholt oder geschrieben.

## Schritt 2 — checkout

Rufe `k_playbook_docs_checkout` mit `projectDir` = `context.project.dir` und
`erzeuger: libs` (ersatzweise `k-playbook docs checkout libs --json`).

- `state: checked_out` — das Arbeitsverzeichnis ist geholt. `WORK_DIR` ist `dir` aus der
  Antwort, `WORK_DISPLAY_PATH` derselbe Pfad ab `context.project.dir`
  (`.k-playbook/work/docs/libs`).
- `state: existing` — ein Arbeitsverzeichnis liegt schon da, etwa von einem
  abgebrochenen Lauf. Nichts wurde geholt oder überschrieben. Nenne `existing.since`,
  `existing.ageMinutes` und `existing.files` und frage: **weitermachen** (mit dem, was dort
  liegt) oder **verwerfen** (`k_playbook_docs_checkout` mit `discard: true`, holt neu)?
  Ohne Antwort geht es nicht weiter.
- `kephalaion_unavailable` — nichts geschrieben; der Command endet wie in der Ablage oben.

Geschrieben wird ab hier nur in `WORK_DIR`. Einzige Ausnahme ist der bestätigte Eintrag
über `k_playbook_docs_exclude_add` im Abschluss.

## Schritt 3 — Lesen

Lies **alle** Dokumente in `WORK_DIR`, bevor du die Kandidaten bildest. Je Dokument merkst
du dir Dateiname, `generated.by`, `generated.at`, `hints`, `questions` und die Einträge in
`sources`, deren `resource` das Inventar ist (`item`, `version`).

- `hints` befolgst du (Kopf in der Ablage oben) — etwa Schwerpunkte oder eine Zusatzquelle.
  Eine Zusatzquelle aus einem `hint` gehört zum Zählen wie im Modul zum Scope.
- **Zuständig** ist dieser Lauf nur für Dokumente mit `generated.by: k-docs-tools` oder dem
  Altwert `k-tools-scan`. Alle anderen — ohne `generated.by` oder mit einem anderen Wert —
  bleiben im Arbeitsverzeichnis **unverändert liegen**, samt `hints` und `questions`.
- Ein Dokument gehört zu dem Gegenstand, den sein Inventar-Eintrag in `sources` als `item`
  nennt, sonst zu dem Gegenstand, aus dem sein Dateiname nach der Regel im Modul zu Schritt 5
  (`_docs-tools/schreiben.md`) entsteht. Den bisherigen Text nimmst du als Anregung, nicht als Vorlage zum Abschreiben.

## Schritt 4 — Kandidaten und Auswahl

Lies `k-playbook:/commands/_docs-tools/kandidaten.md` und befolge es: Kandidaten sind alle
Gegenstände des Inventars; du zählst ihre Nutzung im Code im Scope aus Schritt 1, vergibst
Score und Klasse (A empfohlen, B optional, C übersprungen) und zeigst die kompakte Liste, aus
der der Nutzer in **einer** Nachricht wählt. **Nichts schreiben, bevor die Auswahl
bestätigt ist.** Fällt alles in C und wählt der Nutzer nichts, sag das und ende mit dem
Abschluss.

## Schritt 5 — Recherche und Schreiben

Setze `GENERATED_AT` einmal je Lauf (`date -Iseconds`). Lies
`k-playbook:/commands/_docs-tools/schreiben.md`: Recherche je ausgewähltem Kandidaten (Basis,
bei Klasse A gezielt im Web, höchstens drei Abrufe), Dateiname, Kopf und Vorlage des
Steckbriefs. Recherchiere in Gruppen von etwa drei Kandidaten je Zwischenmeldung, damit der Nutzer den Fortschritt sieht. Bei vielen Kandidaten darfst du Recherche und Schreiben auf Sub-Agenten
verteilen. Ihr Auftrag beginnt mit: Führe zuerst `k-playbook briefing` aus und lies die
Ausgabe; er liest `k-playbook:/commands/_docs-tools/schreiben.md` mit `k_playbook_doc_read`
(`projectDir` = `context.project.dir`; in Claude Code lädt er die Werkzeuge
`mcp__k-playbook__*` vorher mit der Werkzeugsuche) und bekommt von dir seinen Kandidaten mit
den Zeilen aus Schritt 4, den Scope, `WORK_DIR`, `GENERATED_AT` und die bisherige Fassung
seines Dokuments samt `hints`.

Was sich nicht klären lässt, ist eine Unklarheit; sie wird vor `publish` vorgelegt. Nach
jeder geschriebenen Datei kurz melden: Dateiname und Zeilenzahl, nicht der Inhalt.

## Schritt 6 — publish

Lies `k-playbook:/commands/_docs/publish.md` und rufe `publish` mit `erzeuger: libs` auf.
Ist `open` leer und `cleaned` wahr, schließt du genau die Aufträge aus `ORDERS` ab, je einen
mit `k_playbook_doc_done`; Aufträge, die nach dem Preflight angelegt wurden, stehen nicht in
`ORDERS` und bleiben offen. Bricht der Lauf ab, meldet `publish` einen Konflikt, eine
Ablehnung oder `failed`, oder bleibt etwas offen, schließt du keinen Auftrag ab; ein
`publish`, das nach einem behobenen Konflikt doch noch erfolgreich durchläuft, schließt
`ORDERS` ab. Die Rückmeldungen von `k_playbook_doc_done` stehen im Modul zu den Aufträgen.

## Schritt 7 — Abschluss

**Ausschlüsse speichern** wie im Modul zum Scope, wenn der Nutzer im Preflight `NEW_EXCLUDES`
ergänzt hat. Kompakte Zusammenfassung:

- Kandidaten aus dem Inventar: N Gegenstände, davon A = x, B = y, C = z; ausgewählt: k.
- Je Dokument der Zustand aus `publish`, gezählt nach neu, umgeschrieben, entfallen,
  unverändert; dazu die unberührten Dokumente anderer Schreiber.
- Geänderte `hints` (nur mit Bestätigung) und die offen gebliebenen Unklarheiten.
- Der Stand des Inventars (`generated.at`) und der beschriebene Stand des Codes aus
  Schritt 1 (Branch bzw. `<ref>` und Commit).
- Offene Dateien mit Grund, falls `publish` nicht aufgeräumt hat.
- Die Doku-Aufträge: abgeschlossen mit Verweis, offen geblieben mit Grund, oder keine.
- Ausdrücklich: geschrieben wurde nur nach `keph:/docs/generated/libs/`, ins
  Arbeitsverzeichnis `WORK_DISPLAY_PATH` und — beim Abschließen der Aufträge — nach
  `keph:/queue/docs/done/` — und, nur wenn der Nutzer es bestätigt hat, über
  `k-playbook inventory` nach `keph:/docs/generated/versions/inventory.md` und über
  `k_playbook_docs_exclude_add` nach `docs.exclude` in `K-PLAYBOOK.yaml`.
- Hinweis: Nach einem Versionssprung zieht zuerst das Inventar nach (`/k-doc-inventory`),
  danach dieser Command die Steckbriefe, deren Inhalt veraltet ist.
- Folge-Command: keiner. Den Index über `keph:/docs/` erzeugt das Indizieren, nicht ein
  Command.

## Fehlerfälle

- `context.kephalaion.ok` ist nicht `true` oder ein Werkzeug meldet
  `kephalaion_unavailable` → kein Lauf; `context.kephalaion.message` nennen und auf die
  Projekt-Collection auf der Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf die Platte.
- Das Inventar fehlt, und der Nutzer lässt es nicht erheben, oder `k-playbook inventory`
  scheitert → kein Lauf; die Meldung nennen und `/k-doc-inventory` als nächsten Schritt.
  Keine Manifeste selbst lesen.
- `$ARGUMENTS` fehlt auf der Platte, liegt außerhalb von `context.project.dir` oder in
  einem der immer ausgeschlossenen Verzeichnisse (`invalid_input` von `k_playbook_docs_scope`)
  → abbrechen, den Pfad nennen.
- `k_playbook_docs_checkout` meldet `existing` → fragen: weitermachen oder verwerfen.
- Ein Werkzeug ist unbekannt (`k_playbook_docs_checkout`, `_scope`, `_orders`), `k-playbook
  docs` meldet `unbekanntes Kommando` oder `unbekannter Erzeuger "libs"` → die Installation
  ist älter als dieser Command: melden, `/k-upgrade` nennen, nichts von Hand rechnen oder
  nach Kephalaion schreiben.
- `WebFetch` scheitert oder liefert nichts Brauchbares → den Steckbrief aus eigenem Wissen
  schreiben, die Lücke als Unklarheit vorlegen, die URL nicht in `sources` eintragen.
- `publish` meldet `conflict`, `hints_changed` oder `failed` → wie im Modul zu `publish`.

## Anti-Muster (nicht tun)

- **Manifeste selbst scannen** oder **Kandidaten nach dem Scope filtern.** Die Kandidaten
  kommen allein aus dem Inventar; fehlt es, wird es erhoben, nicht nachgebaut; der Scope
  begrenzt nur das Zählen im Code.
- **Abschreiben aus der offiziellen Doku** oder **„Kann kaputtgehen, wenn X“.** Der Wert
  entsteht aus konkretem Pitfall und Projektkontext.
- **Web-Recherche für alle.** Nur für A, gezielt, höchstens drei Abrufe je Gegenstand.
- **Eigene Kopffelder.** Kein `lib`, `version`, `severity`, `last-reviewed`, `type`, `tags`
  oder `status` — Versionen stehen in `sources`, das Gewicht im Text.
- **Fremde Dokumente löschen oder umschreiben.** Was ein anderer Schreiber angelegt hat,
  bleibt unverändert im Arbeitsverzeichnis — `publish` löscht, was dort fehlt.
- **`hints` oder Konflikte still ändern.** Neue `hints` nur nach Bestätigung und mit
  `allow_hints`; kein `discard` ohne Entscheidung des Nutzers.
- **Direkt in die Collection schreiben, `README.md` oder `index.md` schreiben, auf die
  Platte ausweichen.** Die README schreibt `publish`, den Index das Indizieren; ohne
  Kephalaion gibt es keinen Lauf.
