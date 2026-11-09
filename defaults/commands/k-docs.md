# k-docs

Zeigt den Stand der Projektdoku unter `keph:/docs/` in der Collection des Projekts, meldet
Lücken und Widersprüche und bietet die Erzeuger an: das Versionsinventar, die Doku aus dem
Code und die Steckbriefe der Libraries und Tools. Mit `alle` laufen sie in dieser
Reihenfolge nacheinander. Der Ablauf eines Erzeugers steht in seinem eigenen Text; dieser
Command wiederholt ihn nicht.

Produces:

- Im Statuslauf nichts. Ohne Auswahl ist `/k-docs` read-only.
- Bei Auswahl schreibt nur der gewählte Erzeuger, nach den Regeln seines Textes.

## Schritt 1 — Briefing und Ablage

include: _shared/erster-schritt.md

include: _shared/docs.md

Aus dem Briefing brauchst du `context.kephalaion` (`ok`, `address`, `message`),
`context.project.dir`, `context.local.dir` und `context.docs` (`exclude`, `problems`). Ohne
`context.kephalaion.ok: true` endet der Command hier, wie die Ablage oben sagt.

Die Erzeuger und ihre Verzeichnisse:

| Bereich | Verzeichnis | Schreiber (`generated.by`) | Command |
|---|---|---|---|
| Inventar | `keph:/docs/generated/versions/` | `k-doc-inventory` | `/k-doc-inventory` |
| Code | `keph:/docs/generated/code/` | `k-docs-code`, Altwert `k-code2docs`; `ks-overlay-repo-analyse` | `/k-docs-code` |
| Libs | `keph:/docs/generated/libs/` | `k-docs-tools`, Altwert `k-tools-scan` | `/k-docs-tools` |
| Handdoku | `keph:/docs/manual/` | keiner — von Hand | — |
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

Dazu die offenen Doku-Aufträge in `keph:/queue/docs/`
(`k-playbook:/instructions/on-demand/doku-auftraege.md`): ein `k_playbook_doc_list` mit
`kind: docs`, ohne `done` — die erledigten unter `done/` zählen nicht. Für die Aufteilung
nach Erzeuger und den ältesten Auftrag ein `k_playbook_doc_grep` mit `kind: docs` und dem
Muster `^(erzeuger|entstanden): `; je Auftrag gelten seine Zeilen `erzeuger:` und
`entstanden:`. Gelesen wird kein Auftrag einzeln. `invalid_input`, weil die Sorte `docs`
unbekannt ist, heißt: Die Installation ist älter als dieser Command — die Zeile `Aufträge`
in Schritt 4 nennt dann `/k-upgrade`.

Dazu auf der Platte, nur nachsehen:

- `<context.local.dir>/docs/` — liegen dort noch Dateien außer `README.md`, ist die
  Umstellung 3.2 von `/k-upgrade` offen. Gezählt wird je Unterverzeichnis; gelesen und als
  gültig genommen wird davon nichts.
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
- **Schreiber.** `generated.by` passt nicht zum Verzeichnis laut Tabelle in Schritt 1; ein
  Dokument unter `manual/` oder `sources/` trägt `generated`; ein Dokument in einem
  Erzeuger-Verzeichnis trägt keins.
- **README.** Ein Erzeuger-Verzeichnis mit Dokumenten, aber ohne `README.md` — sie entsteht
  beim nächsten `publish`.
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
- **Reste.** Dateien unter `<context.local.dir>/docs/` → `/k-upgrade`, Umstellung 3.2. Ein
  liegengebliebenes Arbeitsverzeichnis → der Erzeuger fragt beim nächsten Lauf, ob
  weitergemacht oder verworfen wird.

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
Befunde     <N> (<Kategorien>) | keine
Offen       Platte: <N> Dateien unter k-playbook-local/docs/ → /k-upgrade 3.2 | —
            Arbeitsverzeichnis: <erzeuger> seit <alter> | —

Mögliche Aktionen:
  1. inventory  Versionsinventar erheben            → /k-doc-inventory
  2. code       Doku aus dem Code                   → /k-docs-code    <N> Aufträge offen
  3. tools      Steckbriefe der Libraries und Tools → /k-docs-tools   <N> Aufträge offen
  4. alle       1, 2 und 3 nacheinander
  5. status     nur diesen Bericht

Was soll ich tun?
```

`tools` setzt das Inventar voraus. Fehlt es, steht bei 3 „braucht das Inventar (1)“; `alle`
erhebt es zuerst. Bei 2 und 3 steht die Zahl der offenen Aufträge des Erzeugers
(`erzeuger: code` bzw. `erzeuger: tools`); ohne offene Aufträge entfällt der Zusatz. Der
gewählte Erzeuger listet sie in seinem Preflight und schließt sie nach einem erfolgreichen
`publish` ab.

## Schritt 5 — Weitergeben

Ohne `$ARGUMENTS` zeigst du den Bericht und wartest auf die Wahl. Kein Erzeuger startet von
selbst.

Mit Argument entfällt die Frage nach dem Bericht, nicht der Bericht: Das erste Wort wählt die
Aktion, der Rest geht an den Erzeuger.

- `status` → Bericht, dann Ende.
- `inventory` → lies `k-playbook:/commands/k-doc-inventory.md` mit `k_playbook_doc_read`
  (ersatzweise `k-playbook read`) und befolge ihn. Er nimmt kein Argument.
- `code` → lies `k-playbook:/commands/k-docs-code.md` und befolge ihn mit dem Rest der
  Argumente.
- `tools` → lies `k-playbook:/commands/k-docs-tools.md` und befolge ihn mit dem Rest der
  Argumente.
- `alle` → nacheinander `inventory`, `code`, `tools`, in genau dieser Reihenfolge: Die
  Steckbriefe bauen auf dem Inventar auf. Der Rest der Argumente geht an `code` und
  `tools`. Jeder Erzeuger läuft mit seinem eigenen Preflight und seinen eigenen
  Bestätigungen; dieser Command fasst sie nicht zusammen. Endet einer ohne Ergebnis —
  abgelehnt, abgebrochen, Konflikt offen —, fragst du, ob die übrigen trotzdem laufen
  sollen. `tools` läuft nicht ohne Inventar.

Das Briefing aus Schritt 1 gilt für den Erzeuger weiter; er lädt es nicht neu, außer sein
Text verlangt es.

## Schritt 6 — Abschluss

Im Statuslauf:

- Dokumente je Bereich, offene Aufträge und Befunde je Kategorie, gezählt.
- Ausdrücklich: Es wurde nichts geschrieben.
- Folge-Command: einer der Erzeuger aus Schritt 4, sobald der Nutzer ihn wählt; ohne Wahl
  keiner. Sind Reste auf der Platte gemeldet: `/k-upgrade`.

Nach einer Weitergabe gilt der Abschluss des Erzeugers. Bei `alle` danach eine Zeile je
Erzeuger: gelaufen, übersprungen oder abgebrochen, mit Grund.

## Fehlerfälle

- `context.kephalaion.ok` ist nicht `true` oder `list` meldet `kephalaion_unavailable` →
  kein Bericht; `context.kephalaion.message` nennen und auf die Projekt-Collection auf der
  Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf `k-playbook-local/docs/`.
- Unbekanntes Argument → die gültigen nennen: `status`, `inventory`, `code`, `tools`,
  `alle`.
- Der Text eines Erzeugers ist nicht lesbar (`not_found`) → die Installation ist
  unvollständig oder älter als dieser Command: melden, `/k-upgrade` nennen; den Ablauf nicht
  im Gespräch nachbauen.

## Anti-Muster (nicht tun)

- **Erzeuger von selbst starten.** Der Bericht ist read-only; geschrieben wird erst nach
  einer Wahl.
- **Abläufe nachbauen.** Was ein Erzeuger tut, steht in seinem Text.
- **Befunde nebenbei beheben.** Kein Dokument wird im Statuslauf geändert, auch kein alter
  Kopf und kein Steckbrief mit abweichender Version.
- **Den Stand des Inventars aus dem Rumpf lesen.** Er kommt nur aus dem Kopf.
- **Die Platte als Doku nehmen.** `k-playbook-local/docs/` ist ein Rest für `/k-upgrade`,
  keine Quelle; `index.md` und `README.md` schreibt kein Command.
