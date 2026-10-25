---
title: Die Felder des Briefings
description: Jedes Feld des Briefings und wozu es dient — die eigenen Abschnitte und der Übergangsabschnitt context —, dazu die vollständige Liste der Bedingungen, unter denen eine Sitzung das Briefing neu lädt.
---

## Die Felder des Briefings

Das Briefing ist die Antwort von `k_playbook_briefing` (ersatzweise `k-playbook briefing
--json`). Was jede Sitzung daraus **tun** muss, steht in den immer geltenden Anweisungen; hier
steht, was jedes Feld bedeutet. Der Vertrag für Entwickler ist `docs/mcp.md`, „Briefing
Contract“ — dieses Blatt hält mit ihm Schritt, ein Test prüft die Vollständigkeit.

### Die eigenen Abschnitte

| Feld | Bedeutung |
|---|---|
| `schemaVersion` | Die Fassung der Briefing-Ausgabe, heute `"5"`. |
| `session` | Die Kennung dieser Sitzung, mit jedem Briefing neu vergeben, etwa `20261024-183012-3fa9c1`. Sie steht in allem, was die Sitzung später wiederfinden muss — im Kopf eines Befunds unter `sitzung`, nach dem `/k-danke` sucht. Lädt eine Sitzung das Briefing neu, gilt die neue Kennung, und die frühere gehört weiter zu ihr. |
| `documents.always` | Das zusammengebaute Verzeichnis `k-playbook:/instructions/always/`: `ref`, `content` (der Text selbst, Includes eingesetzt — eine Sitzung muss ihn nicht eigens lesen), `parts` in der Reihenfolge des Texts, `problems` (übergangene Teile). |
| `documents.onDemand`, `documents.rules`, `documents.reviews`, `documents.guidelines` | Die Einträge der Sorte, je ein Dokument für sich: `ref`, `title`, `description`, `origin` (`dist`, `local`, `override` oder `disabled` für einen abgeschalteten Eintrag). Mitgeliefert und projektlokal sind schon gegeneinander aufgelöst. Gelesen wird über `ref` mit `k_playbook_doc_read`. |
| `work` | Die eigene offene Arbeit aus der Projekt-Collection: `available` und `message` (ob die Collection lesbar war und sonst warum nicht), `user` (der eigene Kephalaion-User), `tasks` und `workbench` (je `ref`, `title`, `updated`; offen heißt: nicht in `done/`), `openFindings` (Zahl der offenen Befunde in `queue/findings/`). |
| `projectDocs` | Der Einstieg der Projektdokumentation, als Verweis. |
| `health` | Die Umgebungsprüfung: `ok` (kein Eintrag der Stufe `error`; Warnungen lassen es wahr) und `problems`, je Eintrag `level` (`error` oder `warning`), `area` (`kephalaion`, `gh`, `baseTools`, `versionSources`), `message` (ein Satz, nie ein Token oder Wert) und `hint` (die Abhilfe), sortiert nach Bereich, dann Stufe. Ohne Problem ist `problems` leer. Dieselbe Antwort wie `k_playbook_health` und `k-playbook health --json`; Einzelheiten unten. |
| `context` | Der Rest von `k-playbook context`: jedes Feld der Kontextausgabe unter seinem Namen, das das Briefing nicht in eigenen Abschnitten liefert — `catalogs.rules`, `catalogs.reviews` und `guidelines` fehlen darum. Ein Übergang: Jedes Feld bekommt noch einen eigenen Platz oder entfällt. |

### Der Abschnitt `context`

| Feld | Verwendung |
|---|---|
| `context.available`, `context.message` | Ob die Kontextausgabe gebaut werden konnte. Wenn nicht, sagt `message` warum, und die übrigen Felder fehlen; das Briefing davor gilt trotzdem. |
| `context.schemaVersion` | Die Fassung der Konfiguration `K-PLAYBOOK.yaml`. Liest kein Command. |
| `context.now.date`, `context.now.timestamp` | Der Moment des Aufrufs in der Zeitzone des Rechners. `date` ist `YYYY-MM-DD`, `timestamp` RFC 3339. |
| `context.instructions` | Dateien, die zuerst zu lesen sind, in dieser Reihenfolge: die mitgelieferte Ebene, dann die des Projekts. |
| `context.project.dir` | Das Hauptverzeichnis — wo `K-PLAYBOOK.yaml` liegt. |
| `context.project.repoRoot`, `context.project.vcs` | Das Code-Repository für Git-Operationen, und ob es unter Versionskontrolle steht. |
| `context.project.config` | Der Pfad der Konfiguration `K-PLAYBOOK.yaml`. Nur zur Auskunft; gelesen wird sie nie selbst. |
| `context.project.languages` | Die Sprachen, für die das Projekt Werkzeuge braucht. Immer gefüllt — ein fehlender Eintrag fällt auf den Standard zurück, es gibt nichts zu deuten. |
| `context.playbook.dir` | Die Installation. Nur lesen; sie wird bei jedem Update ersetzt. |
| `context.local.dir` | Alles, was das Projekt besitzt. |
| `context.remediation` | Wie Befunde abgearbeitet werden: `mode`, `target`, `grouping`, `quickWins`, `branchPrefix`, `prRequired`, `directFixes`, `configured`. |
| `context.gh` | Ob das Projekt die GitHub CLI nutzt und ob sie auf diesem Rechner nutzbar ist; Einzelheiten unten. |
| `context.mcp` | Die Pflichtliste der MCP-Server aus der Konfiguration (`required`, `configured`). Liest kein Command; Sache der Oberfläche. |
| `context.git` | Ob die Oberfläche das Umschalten des Branches anbietet (`switch`, `allow`, `environments`, `configured`). Liest kein Command. |
| `context.baseTools` | Die Werkzeuge, die k-playbook selbst aufruft, und was davon fehlt: `matrix`, `present`, `missing` (je `name`, `role`, `methods`), `ok`, `installCommand`. |
| `context.catalogs.checks` | Die effektiven Checks — mitgeliefert und projektlokal schon zusammengeführt, `origin` vermerkt, abgeschaltete als `disabled`. Regeln und Reviews stehen nicht hier, sondern unter `documents.rules` und `documents.reviews`. |
| `context.versionSources` | Zustand der Quellenkonfiguration des Versionsinventars `<local.dir>/version-sources.yaml`: ob sie da ist, welche Wurzeln außerhalb des Projekts sie öffnet, welche Zusatzquellen sie nennt, welche Bereiche sie ausschließt und welche Helm-Werte sie ausdrücklich nennt. Von hier lesen — nie die Datei selbst. Einzelheiten unten. |
| `context.kephalaion` | Die Anbindung des Projekts an seine Collection: `address`, `ok`, `user`, `message` u. a. `keph:`-Verweise sind nur erreichbar, wenn `ok` wahr ist. |
| `context.links` | Die Selbstheilung der Assistenten-Verknüpfungen: `healed`, `open`, `note`. Nur da, wenn es etwas zu melden gab; `note` sagt, was das für diese Sitzung heißt. |

### `versionSources`

`context.versionSources` trägt die ganze Antwort, kein Command muss die Datei öffnen: `present`
sagt, ob sie da ist, `path`, wo sie liegt, `schemaVersion`, welche Vertragsfassung sie erklärt,
`roots` die zusätzlich lesbaren Wurzeln und `sources` die konfigurierten Zusatzquellen (`path`,
`kind`, `env`, `note`, `optional` je Eintrag), `exclude` die Muster, die der Standardlauf
übergeht, und `helmValues` die konfigurierten Helm-Werte (`path`, `key`, `item` je Eintrag; nur
unter `schemaVersion` 2 angewendet). Die Installation `<playbook.dir>` wird immer übergangen und
steht nicht in `exclude` — diese Regel kommt aus dem Vertrag, nicht aus der Datei. `error` ist
nur gefüllt, wenn die Datei da, aber unlesbar oder von unbekannter Fassung ist — dann sind
`roots`, `sources`, `exclude` und `helmValues` leer, und der Zustand ist ein sichtbarer Befund,
kein leeres Ergebnis. Auf einer Installation, die älter ist als das Feld, fehlt es. Der
vollständige Vertrag steht in `<playbook.dir>/docs/version-inventory.md`.

### `health`

`health` bewertet, was `context` an Umgebungsangaben trägt, und ersetzt nichts davon: Die
Felder `context.kephalaion`, `context.gh`, `context.baseTools` und `context.versionSources`
stehen weiter unverändert da. Die Einträge heute: Die Anbindung an Kephalaion ist eingetragen,
aber nicht `ok` — `error`, denn `keph:`-Verweise und `work` sind dann nicht erreichbar; sie
ist nicht eingetragen — `warning`. Das Projekt setzt gh voraus (`context.gh.status` ist
`enabled`), aber `context.gh.ready` ist falsch — `warning`. Je fehlendem Basis-Werkzeug
(`context.baseTools.missing`) — `warning`. Die Quellenkonfiguration des Versionsinventars
ist da, aber nicht lesbar oder von unbekannter Fassung (`context.versionSources.error`) —
`error`; der Eintrag nennt den Pfad, die Einzelheiten stehen in `context.versionSources.error`.
Was eine Sitzung mit den Einträgen tut, steht in den immer geltenden Anweisungen: in der
ersten Antwort nennen, Fehler vor der Arbeit, nichts still beheben, die Abhilfe aus `hint`
anbieten. Nach einer Abhilfe liefert `k_playbook_health` den Stand neu, ohne das Briefing neu
zu laden. Weitere Quellen hängen sich hier ein; die Liste der Bereiche wächst dann.

### `gh`

`context.gh` trägt zwei getrennte Dinge. `gh.status` ist die Entscheidung des Projekts —
`enabled`, `disabled` oder `unknown` — und steht in `K-PLAYBOOK.yaml`. `gh.installed`,
`gh.loggedIn`, `gh.account` und `gh.ready` sind ein Befund über diesen Rechner. `gh.ready`
heißt beides: Die CLI ist da, und ein Konto ist hinterlegt. Gelesen wird das aus der
Konfiguration von gh, nicht gegen den Server geprüft — ein Token kann also abgelaufen sein.

### Neu laden

Das Briefing gilt für die ganze Sitzung; der aufgelöste Stand ändert sich während der Arbeit
nicht. Neu geladen wird nur, wenn eines davon zutrifft:

- `K-PLAYBOOK.yaml` wurde seit dem Laden geschrieben.
- Ein Katalogeintrag — Regel, Review, Check, Guideline oder Anweisung — ist seit dem Laden
  hinzugekommen, weggefallen oder geleert worden.
- k-playbook wurde aktualisiert.
- Die Arbeit ist in ein anderes Projekt gewechselt. Das Briefing sucht `K-PLAYBOOK.yaml` vom
  Arbeitsverzeichnis aufwärts, ein anderes Arbeitsverzeichnis kann also eine andere Installation
  treffen — `context.project.dir` mit dem geladenen vergleichen.
- `context.now` ist das eine Feld, das altert: Es ist der Moment des Aufrufs. Eine Sitzung, die
  über Mitternacht läuft, trägt noch das Datum von gestern. Neu laden, bevor ein falsches Datum
  in eine Datei käme.

Trifft eines zu, wird das Briefing neu geholt, und ab dann gilt nur die neue Ausgabe. Felder aus
zwei Ladungen werden nie gemischt.
