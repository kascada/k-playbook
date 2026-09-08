---
description: Schließt eine Arbeitssitzung ab - legt die festgehaltenen Befunde vor, befördert Bestätigtes in die Dokumentation, sichert Betriebswissen und prüft, ob die Doku nachzuziehen ist. Nimmt kein Argument.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep]
---

# k-danke

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
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
- Ergänzungen in `k-playbook-local/material/befunde/<slug>.md` — was noch nicht
  festgehalten war.
- Nach Bestätigung: einen Eintrag in `k-playbook-local/guidelines/fallen.md` oder eine
  projekteigene Regel unter `k-playbook-local/rules/`.

Nach `k-playbook-local/docs/` schreibt dieser Command **nicht** selbst; dafür ruft er
`/k-docs-extract` auf.

## Schritt 1 — Pfade auflösen

Aus der Context-Ausgabe:

- `BEFUNDE_DIR = <local.dir>/material/befunde`
- `BEFUNDE_DISPLAY_PATH = k-playbook-local/material/befunde`
- `GUIDELINES_DIR = <local.dir>/guidelines`
- `FALLEN_PATH = <GUIDELINES_DIR>/fallen.md`
- `LOCAL_RULES_DIR = <local.dir>/rules`
- `RESOLVED_DOCS_DIR = <local.dir>/docs` — nur zum Lesen, für den Abgleich.

Command-specific policy:

- `BEFUNDE_DIR` ist das Erzeuger-Verzeichnis der Regel `befunde.md`. Fehlt es, lege es
  ohne Rückfrage an — vor dem ersten Befund ist das der Normalzustand.
- Fehlt `GUIDELINES_DIR` oder `LOCAL_RULES_DIR`, frage, ob genau dieses Verzeichnis
  angelegt werden soll oder ob `/k-gui` laufen soll. Beide gehören zur Struktur, die das
  Einrichten anlegt. Kein Ersatzpfad, kein harter Abbruch.
- **Verbindlich ist die Regel `befunde.md`** aus `catalogs.rules`. Sie enthält Anlässe,
  Dateiformat, Anhänge- und Konfliktregel; dieser Command wiederholt sie nicht.
- Geschrieben wird ausschließlich in `BEFUNDE_DIR`, `FALLEN_PATH` und `LOCAL_RULES_DIR`.
  `docs/` gehört seinen Erzeugern, `docs/README.md` gehört `/k-docs-index`.

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

Lies die Dateien in `BEFUNDE_DIR`, deren Abschnitte die Kennung dieser Sitzung tragen, und
zeige sie verdichtet — eine Zeile je Befund, nicht den Volltext:

```text
/k-danke — in dieser Sitzung festgehalten
─────────────────────────────────────
  importlauf-bricht-ab.md
    [bestaetigt]   Abbruch kommt aus dem Timeout des Vorlaufs, nicht aus dem Parser
    [widerlegt]    Es liegt nicht an der Zeichenkodierung
  rechteproblem-cache.md
    [unbestaetigt] Der Cache wird vermutlich zweimal angelegt
```

Gibt es keine Einträge, obwohl die Sitzung analysiert hat, sage das ausdrücklich — es ist
der häufigste Fehlerfall, nicht ein leeres Ergebnis.

## Schritt 4 — Nachtragen, was fehlt

Gehe die Sitzung durch und prüfe, was sie ergeben hat, das noch nicht geschrieben ist.
Achte besonders auf das, was am leichtesten verlorengeht:

- Wege, die geprüft und **ausgeschlossen** wurden.
- Thesen, die sich als **falsch** erwiesen haben.
- Der Grund, **warum** etwas so gebaut ist, wenn er erkennbar wurde.

Trage Fehlendes nach — Format und Anhängeregel stehen in `befunde.md`. Ohne Rückfrage,
aber melde je Datei eine Zeile.

Ist der Kontext dieser Sitzung bereits komprimiert worden, sage das: dann ist das
Nachtragen unvollständig, und das ist ein Befund für den Nutzer.

## Schritt 5 — Gelerntes einordnen

Frage **gebündelt**, in einem Zug, und schreibe nichts vorher:

```text
Was davon soll dauerhaft festgehalten werden?

  Projektwissen → Dokumentation
    - Abbruch kommt aus dem Timeout des Vorlaufs         [ja | nein]

  Betriebsfalle → guidelines/fallen.md
    - Der Integrationstest läuft ohne Netz durch und sagt nichts aus   [ja | nein]

  Prüfbare Regel → rules/
    - Vor jedem Lauf die Rechte des Zielverzeichnisses prüfen          [ja | nein]
```

- **Projektwissen** wird nicht hier geschrieben, sondern in Schritt 7 über
  `/k-docs-extract` befördert.
- **Betriebsfalle** heißt: was man wissen muss, **bevor** man handelt. Anhängen an
  `FALLEN_PATH`; existiert die Datei nicht, lege sie mit einer H1 und einem Einleitungssatz an.
- **Prüfbare Regel** heißt: was jedes Mal zu geschehen hat. Eine Datei unter
  `LOCAL_RULES_DIR`, aufgebaut wie die mitgelieferten Regeln.

Beide wirken dauerhaft und kosten künftige Sitzungen Aufmerksamkeit — deshalb nur nach
ausdrücklicher Bestätigung, und im Zweifel lieber nicht.

## Schritt 6 — Doku nachziehen

Wurde in dieser Sitzung Code geändert, greift die Regel `docs-sync.md`. Sie verlangt den
Nachzug **im selben Arbeitsgang**; dieser Schritt ist das Auffangnetz, nicht der Hauptweg.

Führe dafür **`/k-enforcement`** aus. Baue die Prüfung nicht nach — sie gehört dorthin und
kennt den vollständigen Regelkatalog.

Bleibt danach etwas offen, nenne es beim Abschluss, statt es still zu lassen.

## Schritt 7 — Abschluss

Kompakte Zusammenfassung:

- Geschriebene und ergänzte Dateien unter `BEFUNDE_DISPLAY_PATH`, mit Dateinamen.
- Was nach `fallen.md` oder `rules/` ging — oder ausdrücklich: nichts.
- Ergebnis der Docs-Prüfung, mit offenen Punkten.
- Verteilung `bestaetigt` gegen `unbestaetigt` gegen `widerlegt`.

Wurde in Schritt 5 Projektwissen zur Beförderung bestätigt, nenne als Folge-Command
**`/k-docs-extract befunde`** — er verdichtet das Rohmaterial nach
`k-playbook-local/docs/extracted/` und hat dafür sein eigenes Bestätigungs-Gate. Danach
nimmt **`/k-docs-index`** die neuen Dateien in den Index auf; ohne diesen Lauf sind sie
für Folge-Sessions nicht auffindbar.

Wurde nichts befördert, sage ausdrücklich, dass kein Folge-Command ansteht und die
Sitzung abgeschlossen ist.

## Fehlerfälle

- `BEFUNDE_DIR` fehlt und die Sitzung hat nichts analysiert → kein Fehler. Sagen und
  ohne Schritt 3 bis 5 weitermachen.
- `BEFUNDE_DIR` ist leer, obwohl analysiert wurde → ausdrücklich melden und in Schritt 4
  vollständig nachtragen. Nicht stillschweigend übergehen.
- Der Sitzungskontext wurde komprimiert → melden, dass das Nachtragen unvollständig sein
  kann, und trotzdem festhalten, was noch da ist.
- `GUIDELINES_DIR` oder `LOCAL_RULES_DIR` fehlt → fragen, ob genau dieses Verzeichnis
  angelegt werden soll, oder `/k-gui` nennen.
- Eine Befund-Datei wurde seit dem Lesen von außen verändert → Konfliktregel aus
  `befunde.md` anwenden, Zweitdatei anlegen und das ausdrücklich melden.
- Der Nutzer bestätigt in Schritt 5 nichts → nichts schreiben und das im Abschluss sagen.

## Anti-Muster (nicht tun)

- **Die Prüfung aus `/k-enforcement` nachbauen.** Sie kennt den zusammengeführten
  Regelkatalog; eine zweite Fassung läuft davon weg.
- **Nach `docs/` schreiben.** Auch wenn ein Befund dort thematisch hinpasst: der Weg führt
  über `/k-docs-extract`, das die Herkunft und die Konfidenz mitschreibt.
- **Vor dem Gate in Schritt 5 schreiben.** Ein halb angelegter Eintrag aus einem
  abgebrochenen Lauf sieht später aus wie bestätigtes Wissen.
- **Alles nach `fallen.md` kippen.** Diese Datei wird künftig vor dem Handeln gelesen;
  was dort ohne Not steht, kostet jede weitere Sitzung Aufmerksamkeit.
- **Die Sitzung zusammenfassen.** Dieser Command sammelt Befunde ein, er schreibt kein
  Protokoll. Was nichts erklärt, gehört nicht hinein.
- **Bei leerem Ergebnis so tun, als sei alles gut.** Eine Analyse-Sitzung ohne einen
  einzigen Befund ist ein Hinweis darauf, dass die Stufen 1 und 2 nicht gegriffen haben.
