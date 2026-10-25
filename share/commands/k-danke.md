---
description: Schließt eine Arbeitssitzung ab - legt die Befunde der Sitzung vor, trägt Fehlendes nach, lässt den Nutzer entscheiden, was aus ihnen werden soll, und prüft, ob die Doku nachzuziehen ist. Nimmt kein Argument.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_create, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-danke

## Erster Schritt

Wende `.claude/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Das Sitzungsende ist eine Tatsache über den Nutzer, nicht über das System — deshalb sagt
er es, statt dass geraten wird. Dieser Command ist der Abschluss: er sammelt ein, was die
Sitzung erarbeitet hat, und sorgt dafür, dass nichts davon nur im Kontext bleibt.

Er ist bewusst als **ein** Command angelegt, der wächst: welche Abschlussschritte greifen,
richtet sich nach dem, was die Sitzung tatsächlich getan hat.

Produces:
- Nachgetragene Befunde in `keph:/queue/findings/`, mit `entstanden: k-danke`.
- Nach Bestätigung: das Feld `ziel` in den Befunden dieser Sitzung.

Sonst schreibt dieser Command nichts. Was aus einem Befund mit gesetztem `ziel` wird —
Wissen, Betriebsfalle, Regel —, erledigt die spätere Übernahme aus der Queue, nicht dieser
Command.

## Schritt 1 — Voraussetzungen

- **Verbindlich ist `k-playbook:/instructions/on-demand/befunde.md`.** Lies sie mit
  `k_playbook_doc_read`, bevor du einen Befund liest oder schreibst. Sie enthält Ablage, Kopf,
  Rumpf und das Widerlegen; dieser Command wiederholt sie nicht.
- `COLLECTION = kephalaion.address` aus der Context-Ausgabe.
- Steht `kephalaion.ok` nicht auf `true`, sind die Befunde weder zu lesen noch zu schreiben.
  Sag das ausdrücklich, führe nur Schritt 6 aus und gib in Schritt 7 die Befunde, die
  festzuhalten wären, vollständig im Text wieder.

## Schritt 2 — Bestimmen, was diese Sitzung getan hat

Ordne die Sitzung in einem Satz ein, bevor du etwas vorlegst. Danach richtet sich, welche
der folgenden Schritte überhaupt greifen:

- **Analysiert** — es wurde herausgefunden, wie oder warum etwas funktioniert.
  → Schritte 3 bis 5.
- **Geändert** — es wurde Code, Konfiguration oder Doku geschrieben. → Schritt 6.
- **Beides** — der Regelfall bei einem Fix. → alle Schritte.
- **Weder noch** — nur Auskunft, Formatierung, Werkzeugbedienung. Dann sage das, führe
  keinen weiteren Schritt aus und beende den Command.

Dieser Schritt ist die Stelle, an der später weitere Abschlussarten andocken.

## Schritt 3 — Befunde vorlegen

Die Kennung dieser Sitzung ist das Feld `session` des Briefings (im Klartext die Zeile
`Sitzung:`). Wurde das Briefing in dieser Sitzung mehrmals geladen, gehören alle seine
Kennungen dazu. Hole mit `k_playbook_doc_list`, Sorte `findings`, die offenen Befunde. Lies
die, die du selbst angelegt hast, mit `k_playbook_doc_read` und behalte die, deren `sitzung`
eine dieser Kennungen trägt. Zeige sie verdichtet — eine Zeile je Befund, nicht den Volltext:

```text
/k-danke — in dieser Sitzung festgehalten
─────────────────────────────────────
  Importlauf bricht ab
    012  [bestaetigt]   Abbruch kommt aus dem Timeout des Vorlaufs, nicht aus dem Parser
    013  [widerlegt]    Es liegt nicht an der Zeichenkodierung
  Rechteproblem im Cache
    014  [unbestaetigt] Der Cache wird vermutlich zweimal angelegt
```

Gruppiert wird nach `thema`, die Zahl ist die Nummer des Dokuments.

Gibt es keine Befunde, obwohl die Sitzung analysiert hat, sage das ausdrücklich — es ist
der häufigste Fehlerfall, nicht ein leeres Ergebnis.

## Schritt 4 — Nachtragen, was fehlt

Gehe die Sitzung durch und prüfe, was sie ergeben hat, das noch nicht geschrieben ist.
Achte besonders auf das, was am leichtesten verlorengeht:

- Wege, die geprüft und **ausgeschlossen** wurden.
- Thesen, die sich als **falsch** erwiesen haben.
- Der Grund, **warum** etwas so gebaut ist, wenn er erkennbar wurde.

Trage Fehlendes als neue Befunde nach, mit `entstanden: k-danke`. Ohne Rückfrage, aber
melde je Befund eine Zeile. Vorhandene Befunde werden dabei nicht geändert.

Ist der Kontext dieser Sitzung bereits komprimiert worden, sage das: dann ist das
Nachtragen unvollständig, und das ist ein Befund für den Nutzer.

## Schritt 5 — Ziel erfragen

Frage **gebündelt**, in einem Zug, und schreibe nichts vorher:

```text
Was soll aus diesen Befunden werden?

  012  Abbruch kommt aus dem Timeout des Vorlaufs        [wissen | falle | regel | verwerfen | offen]
  013  Es liegt nicht an der Zeichenkodierung            [wissen | falle | regel | verwerfen | offen]
  014  Der Cache wird vermutlich zweimal angelegt        [wissen | falle | regel | verwerfen | offen]
```

- **wissen** — Projektwissen für die Wissensablage.
- **falle** — was man wissen muss, **bevor** man handelt.
- **regel** — was jedes Mal zu geschehen hat.
- **verwerfen** — nicht übernehmen.
- **offen** — jetzt nicht entscheiden; `ziel` bleibt leer.

Schlage je Befund ein Ziel vor. Fallen und Regeln wirken dauerhaft und kosten künftige
Sitzungen Aufmerksamkeit — schlage sie deshalb nur vor, wenn es klar ist, und im Zweifel
`wissen`.

Für jede Antwort außer `offen` setze `ziel` im Kopf des Befunds: lies ihn mit Kephalaion
`read`, ändere nur diese Zeile und schreibe ihn mit `write` und `base_revision` aus dem
`read` zurück. Antwortet `write` mit `stale_revision`, lies neu und versuche es einmal
wieder.

## Schritt 6 — Doku nachziehen

Wurde in dieser Sitzung Code geändert, greift die Regel `docs-sync.md`. Sie verlangt den
Nachzug **im selben Arbeitsgang**; dieser Schritt ist das Auffangnetz, nicht der Hauptweg.

Führe dafür **`/k-enforcement`** aus. Baue die Prüfung nicht nach — sie gehört dorthin und
kennt den vollständigen Regelkatalog.

Bleibt danach etwas offen, nenne es beim Abschluss, statt es still zu lassen.

## Schritt 7 — Abschluss

Kompakte Zusammenfassung:

- Die Befunde dieser Sitzung mit Nummer, davon die in Schritt 4 nachgetragenen.
- Verteilung `bestaetigt` gegen `unbestaetigt` gegen `widerlegt`.
- Verteilung der gesetzten `ziel` — oder ausdrücklich: keines gesetzt.
- Ergebnis der Docs-Prüfung, mit offenen Punkten.

Danach ist die Sitzung abgeschlossen. Einen Folge-Command gibt es nicht: die Befunde
warten in `keph:/queue/findings/` auf die Übernahme.

## Fehlerfälle

- Kephalaion ist nicht erreichbar → sagen, nur Schritt 6 ausführen und die festzuhaltenden
  Befunde vollständig im Abschluss wiedergeben. Nicht auf die Platte ausweichen.
- Die Sitzung hat nichts analysiert → kein Fehler. Sagen und ohne Schritt 3 bis 5
  weitermachen.
- Keine Befunde dieser Sitzung, obwohl analysiert wurde → ausdrücklich melden und in
  Schritt 4 vollständig nachtragen. Nicht stillschweigend übergehen.
- Der Sitzungskontext wurde komprimiert → melden, dass das Nachtragen unvollständig sein
  kann, und trotzdem festhalten, was noch da ist.
- `write` antwortet beim Setzen von `ziel` zweimal mit `stale_revision` → den Befund
  nennen und `ziel` dort offen lassen.
- Der Nutzer entscheidet in Schritt 5 nichts → kein `ziel` setzen und das im Abschluss
  sagen.

## Anti-Muster (nicht tun)

- **Die Prüfung aus `/k-enforcement` nachbauen.** Sie kennt den zusammengeführten
  Regelkatalog; eine zweite Fassung läuft davon weg.
- **Selbst übernehmen.** Nichts nach `docs/`, `knowledge/`, `guidelines/fallen.md` oder
  `rules/` schreiben, auch wenn das Ziel feststeht. Das tut die Übernahme aus der Queue.
- **`ziel` ohne Bestätigung setzen.** Der Vorschlag ist deiner, die Entscheidung gehört
  dem Nutzer.
- **Einen Befund umschreiben.** Außer `ziel` bleibt er, wie er geschrieben wurde; was
  fehlt, ist ein neuer Befund.
- **Die Sitzung zusammenfassen.** Dieser Command sammelt Befunde ein, er schreibt kein
  Protokoll. Was nichts erklärt, gehört nicht hinein.
- **Bei leerem Ergebnis so tun, als sei alles gut.** Eine Analyse-Sitzung ohne einen
  einzigen Befund ist ein Hinweis darauf, dass die Stufen 1 und 2 nicht gegriffen haben.
