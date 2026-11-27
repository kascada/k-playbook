# k-feedback

Zeigt und pflegt die Meldungen des Projekts: Fehler und Wünsche, die jemand anderes erledigen
soll — an k-playbook, an Kephalaion oder am Projekt selbst. Ohne Argument listet er die
eigenen offenen Meldungen und zählt die offenen in jeder anderen Collection, die du lesen
darfst; mit einem Text legt er eine Meldung an, mit `done`, `ablehnen` oder `annehmen` und
einer Nummer ändert er ihren Status.

Eine Meldung ist kein Befund. Ein Befund ist belegtes Wissen über das Projekt und geht in die
Wissensablage; eine Meldung wartet darauf, dass jemand anderes sie erledigt, und bleibt bis
dahin liegen. Behebt die Sitzung einen Fehler selbst, gehört der Stand in die Workbench, nicht
hierher.

Produces:

- `keph:/feedback/<NNN>-<slug>.md` — eine neue Meldung, nur mit einem Text als Argument und
  erst nach Bestätigung, angelegt über `k_playbook_feedback_create`.
- Der geänderte Status einer Meldung; erledigt und abgelehnt liegt sie danach unter
  `keph:/feedback/done/`.

Nichts sonst. Auf die Platte schreibt dieser Command nichts.

## Schritt 1 — Briefing und Zugriffsweg

include: _shared/erster-schritt.md

Die Meldungen liegen als Dokumente unter `feedback/` in der Projekt-Collection, die erledigten
und abgelehnten unter `feedback/done/` (`k-playbook:/guidelines/betrieb.md`, „Was wohin“). Ihr
Format gehört Go. **Sie werden in diesem Command nie direkt gelesen und nie direkt
geschrieben** — weder über Kephalaion noch mit Read, Write oder Edit. Jede Auskunft und jede
Änderung läuft über den Zugriffsweg unten.

Steht `kephalaion.ok` nicht auf `true`, endet der Command hier mit `kephalaion.message`: Ohne
Kephalaion gibt es keine Meldungen und keinen Rückfall auf die Platte.

Zugriffsweg, in dieser Reihenfolge, der erste verfügbare gewinnt:

1. **MCP-Werkzeuge.** Bietet der Client `k_playbook_feedback_list`,
   `k_playbook_feedback_create` und `k_playbook_feedback_update` an, werden sie benutzt.
   `projectDir` ist `project.dir` aus dem Briefing.
2. **Subkommando.** Sonst `k-playbook feedback list|create|update` über die Shell; die
   Ausgabe ist JSON auf stdout. Das ist kein Notbehelf: Der MCP-Server kann fehlen, das Binary
   ist mit der Installation da. `k-playbook feedback --help` nennt die Flags.

Beide Wege rufen dieselbe Fachlogik und geben dieselben Felder zurück: je Meldung `ref`,
`number`, `revision`, `title`, `art`, `dringlichkeit`, `betrifft`, `status`, `body` und die
Felder zur Herkunft.

## Schritt 2 — Argument deuten

| Eingabe | Aktion |
|---|---|
| kein Argument | eigene Meldungen zeigen, darunter die Zählung (Schritt 3) |
| `alle` | wie ohne Argument, dazu die erledigten und abgelehnten |
| `neu` oder ein Text | Meldung anlegen (Schritt 4) |
| `done <nr>` | Status `erledigt` (Schritt 5) |
| `ablehnen <nr>` | Status `abgelehnt` (Schritt 5) |
| `annehmen <nr>` | Status `angenommen` (Schritt 5) |
| `offen <nr>` | Status `offen`, holt eine Meldung aus `feedback/done/` zurück (Schritt 5) |

## Schritt 3 — Zeigen

Rufe `k_playbook_feedback_list` mit `allCollections: true` auf, bei `alle` dazu
`includeDone: true` — bzw. `k-playbook feedback list --all-collections` (`--all`).

Zeige zuerst die eigenen Meldungen, nach Dringlichkeit geordnet (`hoch`, `mittel`, `niedrig`),
innerhalb derselben nach Nummer. Je Meldung eine Zeile: Nummer, Art, Betreff, Status, Titel,
etwa `#3 fehler · kephalaion · offen — Sync hängt nach Rename`. Den Text (`body`) zeigst du
nur, wenn der Nutzer nach einer Meldung fragt. Gibt es keine, sag das in einem Satz.

Darunter die Zählung, wenn `collections` Einträge trägt: je Collection eine Zeile mit
`address`, der Zahl offener Meldungen und der Aufteilung nach `betrifft`, etwa
`vm:omni — 2 offen (kephalaion 1, k-playbook 1)`. Eine Collection mit `error` erscheint mit
diesem Fehler statt einer Zahl; `collectionsError` nennst du in einem Satz. Welche Collections
dort stehen, entscheiden allein die Leserechte in Kephalaion; du filterst nichts weg und
fügst nichts hinzu. Die Meldungen fremder Collections liest dieser Command nicht.

## Schritt 4 — Anlegen

Der Text ist das, was gemeldet werden soll; bei `neu` ohne Text fragst du danach. Schlage aus
dem Text vor:

- `title` — eine Zeile, die die Meldung ohne den Text erkennbar macht,
- `art` — `fehler` oder `wunsch`,
- `dringlichkeit` — `hoch`, `mittel` oder `niedrig`; `hoch` beim Fehler heißt, er blockiert
  die Arbeit,
- `betrifft` — `k-playbook`, `kephalaion` oder `projekt`,
- `body` — der Text in Markdown: was geschah, was erwartet war, wie es sich nachstellen lässt.
  Der Text des Nutzers geht dabei nicht verloren; ergänze ihn, schreib ihn nicht um.

Zeige den Vorschlag und frage: „Passt das so, oder soll ich etwas anpassen?“ Erst nach der
Bestätigung genau ein Aufruf:

- **MCP:** `k_playbook_feedback_create` mit `title`, `art`, `dringlichkeit`, `betrifft`,
  `body`, `sitzung` (die Sitzungs-ID aus der Zeile `Sitzung:` des Briefings) und `modell`
  (deine Modell-ID); `client` nur, wenn du ihn kennst — sonst nimmt das Werkzeug den Namen aus
  der Initialisierung. Über MCP gilt die Meldung als von der KI.
- **Subkommando:** `k-playbook feedback create --title … --art … --dringlichkeit …
  --betrifft … --von ki --client <client> --modell <modell> --sitzung <sitzung>`, den Text auf
  stdin. Schreibt der Mensch die Meldung selbst in der Shell, nimmt er `--von mensch` ohne
  `--client` und `--modell`.

User, Account, Version und Status `offen` setzt das Werkzeug; du schreibst sie nicht in den
Text. Bestätige in einer Zeile: „Meldung angelegt: `<ref>`“.

## Schritt 5 — Status ändern

`k_playbook_feedback_update` mit `number` und `status` — bzw. `k-playbook feedback update
<nr> --status <status>`. `erledigt` und `abgelehnt` verschieben die Meldung nach
`feedback/done/`, `offen` und `angenommen` holen sie zurück. Hast du die Meldung in dieser
Sitzung gelistet, gib ihre `revision` als `since_revision` (`--since`) mit.

Den Status ändert, wer in der Collection der Meldung arbeitet: der Meldende oder ein
Empfänger mit einer Sitzung in diesem Projekt. Meldungen in fremden Collections ändert dieser
Command nicht.

Bestätige in einer Zeile mit Nummer, neuem Status und `ref`.

## Fehlerfälle

- `invalid_input`: ein ungültiger Wert oder ein fehlendes Feld; die Meldung nennt alle.
  Korrigieren und erneut aufrufen, nichts wurde geschrieben.
- `not_found` oder eine mehrdeutige Nummer: die Liste zeigen und wählen lassen.
- `foreign_change`: Die Meldung hat inzwischen jemand anderes geändert; nichts wurde
  geschrieben. Nicht still nachziehen — den Nutzer mit `revision` und `updatedAt` fragen.
- `outcome_unknown`: nicht wiederholen, sondern mit `k_playbook_feedback_list`
  (`includeDone`) nachsehen, ob der Vorgang geschehen ist.
- `kephalaion_unavailable`: wie in Schritt 1 — Meldung mit `kephalaion.message`, nichts auf die
  Platte.
- `k-playbook feedback` meldet `unbekanntes Kommando`: Die Installation ist älter als dieser
  Command. Melden, `/k-upgrade` nennen, die Ablage **nicht** von Hand nachbauen.

Folge-Command: keiner.

## Anti-Muster (nicht tun)

- **Dokumente unter `feedback/` selbst lesen oder schreiben**, auch nicht „nur kurz
  nachsehen“ und auch nicht in einer fremden Collection: Es gäbe eine zweite Auslegung des
  Formats, die in keine Prüfung eingeht.
- **Eine Meldung als Befund schreiben** oder einen Befund als Meldung. Ein Befund ist Wissen
  über das Projekt, eine Meldung ein Auftrag an jemand anderen.
- **Den Kopf in den Text schreiben** — wer meldet, welcher Client, welche Version. Das setzt
  das Werkzeug.
- **Ohne Bestätigung anlegen.** Erst den Vorschlag zeigen, dann schreiben.
