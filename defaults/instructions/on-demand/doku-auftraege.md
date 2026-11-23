---
title: Doku-Aufträge
description: Wie der Nachzug der erzeugten Doku eingereiht und abgearbeitet wird — Dokumente in queue/docs/, für code in zwei Reifegraden (roh von einer Sitzung oder von /k-docs, zerlegt je Dokument vom Verarbeiter /k-docs-code), Kopf mit erzeuger, pfade, anlass, sitzung und entstanden, zerlegt dazu dokument und aktion, die Feldregeln, wer schreibt, die Prüfung auf einen vorhandenen Auftrag samt dem Ergänzen seiner Pfade, die Wartezeit für rohe Aufträge, wann nichts eingereiht wird, wer abschließt und was nicht hineingehört; für tools bleibt es bei einem Auftrag je Sitzung, der den ganzen Lauf begründet.
---

## Doku-Aufträge

Ändert eine Sitzung Code, veraltet die erzeugte Doku unter `keph:/docs/generated/code/` und
`keph:/docs/generated/libs/`. Statt den Erzeuger nach jeder Änderung zu starten, reiht die
Sitzung einen **Doku-Auftrag** ein; der Erzeuger arbeitet die Aufträge gebündelt ab. Das ist
der Normalfall nach einer Codeänderung.

Die beiden Erzeuger gehen mit ihren Aufträgen verschieden um:

- **`code` — die Aufträge steuern den Lauf.** `/k-docs-code` ist ihr **Verarbeiter**: Er liest
  die offenen Aufträge, legt abhängige zusammen, zerlegt sie in Aufträge je Dokument, schreibt
  diese in die Queue zurück und arbeitet sie der Reihe nach ab. Die `pfade` bestimmen, welche
  Dokumente drankommen. Der volle Lauf ist ein Auftrag über das ganze Projekt (`pfade: .`), den
  `/k-docs code` einreiht; ein Teil-Lauf ist ein Auftrag über einzelne Pfade. Ohne offene
  Aufträge läuft `/k-docs-code` nicht.
- **`tools` — die Aufträge begründen den Lauf.** `/k-docs-tools` läuft immer über das ganze
  Projekt, listet die Aufträge in seinem Preflight und schließt sie nach einem erfolgreichen
  `publish` ab. `/k-docs-tools` sofort laufen zu lassen bleibt erlaubt; dann schließt er den
  Auftrag gleich mit ab.

Ein Auftrag ist ein Merkzettel, kein Rohmaterial: Er sagt, dass und warum nachzuziehen ist,
nicht was in der Doku stehen soll.

### Wohin

- **Ein Dokument je Auftrag** in `keph:/queue/docs/`. Eingereiht wird ein roher Auftrag mit
  `k_playbook_docs_order_add` (`k-playbook docs order add <code|tools> --pfad <pfad>…
  --anlass <satz> --sitzung <kennung>`): Das Programm legt das Dokument an, baut Kopf und
  Rumpf nach dieser Anweisung und ergänzt den offenen Auftrag der Sitzung, statt einen zweiten
  anzulegen. Die zerlegten Aufträge schreibt `k_playbook_docs_orders_split` für
  `/k-docs-code`. Von Hand legt keiner einen Auftrag an.
- **Ein Auftrag je Sitzung und Erzeuger.** Das gilt für den Auftrag nach einer Codeänderung:
  Ändert eine Sitzung mehrmals Code, bleibt es bei einem Auftrag für `code`, dessen `pfade` sie
  ergänzt („Kein Doppel“); berührt sie auch die Abhängigkeiten, kommt höchstens einer für
  `tools` dazu. Der Auftrag von `/k-docs` über das ganze Projekt und die zerlegten Aufträge
  des Verarbeiters fallen nicht darunter.
- **Erledigt** liegt ein Auftrag unter `keph:/queue/docs/done/`; dorthin verschiebt ihn
  `k_playbook_doc_done`. Die Sorte hat keinen Userbereich: Ein Erzeuger-Lauf schließt einen
  Auftrag ab, gleich wer ihn angelegt hat.

### Kopf

Eine Sorte, zwei Reifegrade. Ein Auftrag ist **zerlegt**, wenn sein Kopf `dokument` trägt,
sonst **roh**. Zerlegte Aufträge gibt es nur für `code`.

**Roh** — geschrieben von einer Sitzung nach einer Codeänderung, von `/k-enforcement` oder von
`/k-docs`:

```markdown
---
title: "<was nachzuziehen ist, in einem Satz>"
erzeuger: code | tools
pfade:
  - <pfad relativ zu project.repoRoot>
anlass: "<warum, in einem Satz>"
sitzung: <session aus dem Briefing>
entstanden: <context.now.date>
---
```

**Zerlegt** — geschrieben nur vom Verarbeiter, ein Auftrag je Dokument:

```markdown
---
title: "<dokument ohne .md> <aktion>"
erzeuger: code
dokument: <NN>-<slug>.md
aktion: neu | umschreiben | entfaellt
pfade:
  - <pfad relativ zu project.repoRoot>
anlass: "<warum, in einem Satz>"
sitzung: <session des Verarbeiters>
entstanden: <ältestes Datum der aufgegangenen Aufträge>
---

<datum> — <Anlass eines aufgegangenen Auftrags>
<datum> — <Anlass eines weiteren>
```

- `title` zeigen `k_playbook_doc_list` und `/k-docs` an; der Wert steht wie jeder Wert in freiem
  Text (auch `anlass`) in doppelten Anführungszeichen, ein `"` im Wert wird `\"`, damit jeder
  YAML-Leser genau diesen Text liest, auch bei „: “ oder „ #“. Die übrigen Felder bleiben, wie
  die Vorlage sie zeigt. Ein zerlegter Auftrag heißt nach Dokument und Aktion, etwa
  `"11-installation-und-update umschreiben"`.
- `erzeuger` ist `code` für `keph:/docs/generated/code/` (`/k-docs-code`) und `tools` für
  `keph:/docs/generated/libs/` (`/k-docs-tools`). Ein Wert, kein zweiter: Wer beides braucht,
  legt zwei Aufträge an. Die Zeile steht genau so im Kopf, `erzeuger: code` bzw.
  `erzeuger: tools`, weil der Erzeuger mit `k_playbook_doc_grep` danach sucht.
- `pfade` sind die geänderten Dateien oder Verzeichnisse, relativ zu `project.repoRoot`, je
  einer pro Zeile.
  - Für `code` bestimmen sie, welche Dokumente drankommen; sie begrenzen nicht, was der
    Verarbeiter dafür liest. Deshalb so fein wie möglich: ein Verzeichnis nur, wenn wirklich
    alles darin betroffen ist.
  - `.` heißt ganzes Projekt und steht dann als einzige Zeile, genau als `  - .`. So schreibt
    `/k-docs` den vollen Lauf; im Text heißt das `pfade: .`.
  - Die Ausschlüsse von `k-playbook docs scope` gelten immer, auch für Pfade eines Auftrags.
  - Ein Pfad darf in mehreren zerlegten Aufträgen stehen, wenn er mehrere Dokumente betrifft.
    Doppelt ist nur derselbe Pfad für dasselbe Dokument.
  - Für `tools` begründen die Pfade den Lauf und erscheinen in seinem Preflight; sie engen den
    Scope nicht ein. `/k-docs-tools` läuft immer über das ganze Projekt.
- `dokument` trägt nur ein zerlegter Auftrag: der Dateiname seines Dokuments unter
  `keph:/docs/generated/code/`, ohne Verzeichnis. Die Zeile steht genau so im Kopf, weil Suche
  und Bericht den zerlegten Auftrag an `^dokument: ` erkennen.
- `aktion` trägt nur ein zerlegter Auftrag: `neu`, `umschreiben` oder `entfaellt`.
- `anlass` ist ein Satz: was sich geändert hat, nicht was die Doku sagen soll. Bei einem
  zerlegten Auftrag fasst er zusammen; die Einzelheiten stehen im Rumpf.
- `sitzung` ist die Kennung deiner Sitzung, das Feld `session` des Briefings (im Klartext die
  Zeile `Sitzung:`). Hast du das Briefing neu geladen, nimm die neue Kennung. Ein zerlegter
  Auftrag trägt die Sitzung des Verarbeiters.
- `entstanden` ist `context.now.date`. Gehen mehrere Aufträge in einem auf, bleibt das älteste
  Datum. `/k-docs` nennt den ältesten offenen Auftrag danach.

**Rumpf.** Der Rumpf eines rohen Auftrags darf leer bleiben; mehr als ein kurzer Absatz gehört
nicht hinein. Ergänzt eine Sitzung ihren Auftrag („Kein Doppel“), kommt je weiterem Anlass eine
Zeile dazu: Datum und ein Satz. Der Rumpf eines zerlegten Auftrags nennt je aufgegangenem
Auftrag eine Zeile mit Datum und Anlass — als Text und ohne Verweis, denn die Einträge der
Queue wandern nach `done/`.

### Wer ihn schreibt

- **Jede Sitzung nach einer Codeänderung**, bevor sie die Änderung abschließt — als Teil des
  Doku-Nachzugs aus `k-playbook:/instructions/always/220-regeln-anwenden.md`. Bei
  `/k-task-run` ist das der Hauptkontext im
  Abschluss, nicht der Ausführungs-Sub-Agent.
- **`/k-enforcement`** nach Freigabe durch den Nutzer, wenn sein Bericht für die erzeugte Doku
  „nachzuziehen“ ergibt.
- **`/k-docs`** bei der Wahl `code` oder `alle`: einen rohen Auftrag für `code` mit `pfade: .`,
  den vollen Lauf. Liegt schon ein offener roher Auftrag mit `pfade: .` vor, legt es keinen
  zweiten an. Für diesen Auftrag gelten „Ein Auftrag je Sitzung und Erzeuger“, „Kein Doppel“
  und „Noch keine erzeugte Doku“ nicht: `/k-docs` reiht ihn auch ein, wenn es noch keine
  erzeugte Doku gibt — so entsteht sie zum ersten Mal.
- **Der Verarbeiter `/k-docs-code`**: die zerlegten Aufträge, einen je Dokument. Nur er
  schreibt `dokument` und `aktion`. Den Ablauf trägt `k-playbook:/commands/k-docs-code.md`.

`code` ist einzureihen, wenn sich Code des Projekts geändert hat, den `/k-docs-code` erfasst:
Quelltext unter `project.repoRoot` außerhalb von `context.docs.exclude`. `tools` ist
einzureihen, wenn sich die Bibliotheken oder Werkzeuge des Projekts geändert haben: eine neue
oder entfernte Abhängigkeit, ein Versionssprung, ein anderes Werkzeug im Build.

### Kein Doppel

Die Regel gilt für den Auftrag nach einer Codeänderung und zählt nur rohe Aufträge; ein
zerlegter Auftrag (Zeile `dokument:`) zählt nicht. Das Programm rechnet sie, wenn die Sitzung
`k_playbook_docs_order_add` aufruft: Es sucht den offenen rohen Auftrag derselben Sitzung für
denselben Erzeuger.

- **`tools`.** Gibt es ihn, kommt kein neuer dazu; der Lauf ist ohnehin ganz, die Pfade des
  vorhandenen Auftrags genügen (`covered`).
- **`code`.** Gibt es ihn, kommt kein zweiter dazu, sondern der vorhandene wird ergänzt. Die
  Pfade des vorhandenen Auftrags genügen nur, wenn jeder neue Pfad gleich einem vorhandenen
  ist oder darin liegt (`covered`): Sie bestimmen, welche Dokumente drankommen, und ein
  fehlender Pfad wäre ein Dokument, das nicht nachgezogen wird. Sonst vereinigt das Programm
  die `pfade` und hängt den weiteren Anlass als Zeile an den Rumpf (`extended`). Hat der
  Verarbeiter den Auftrag inzwischen zerlegt, entsteht ein neuer.

`/k-danke` und `/k-enforcement` zählen einen vorhandenen Auftrag als „eingereiht“ — für `code`
nur, wenn seine `pfade` die Änderung decken oder er nach dieser Regel ergänzt wurde.

### Wartezeit

Ein Schreiber soll seinen Auftrag weiter ergänzen können, bevor der Verarbeiter ihn liest.

- Der Verarbeiter nimmt einen rohen Auftrag erst, wenn seine letzte Änderung eine Weile
  zurückliegt. Bis dahin **wartet** der Auftrag, danach ist er **reif**. Den Wert und die
  Rechnung trägt allein das Programm: `k-playbook docs orders code` nennt ihn als
  `waitMinutes` und je wartendem Auftrag die Restzeit; diese Anweisung nennt ihn nicht.
- Maßgeblich ist die letzte Änderung des Auftrags, nicht `entstanden` im Kopf, das nur das
  Datum trägt. Ein Ergänzen beginnt die Frist neu. Ein Kopffeld für die Frist gibt es nicht.
- Die Frist gilt nur für rohe Aufträge. Zerlegte haben keine: Ein unterbrochener Lauf setzt
  sofort wieder an.
- Wartende Aufträge nehmen am Zusammenlegen und Zerlegen nicht teil und bleiben unverändert
  offen, auch wenn ein reifer Auftrag einen ihrer Pfade teilt.
- **Der volle Lauf wartet nicht.** Ein roher Auftrag mit `pfade: .` hat keine Frist und nimmt
  alle offenen rohen Aufträge auf, auch die wartenden. So geht `/k-docs code` und gleich danach
  `/k-docs-code` ohne Warten. Ein Argument oder Kopffeld dafür gibt es nicht.
- Für `tools` gibt es keine Wartezeit.

### Wann nicht eingereiht wird

`k_playbook_docs_order_add` meldet es als `skipped` mit Grund; nichts wird geschrieben.

- **Noch keine erzeugte Doku** (`no_generated_docs`). Das gilt für den Auftrag nach einer
  Codeänderung: Liegt unter `keph:/docs/generated/code/` bzw. `keph:/docs/generated/libs/`
  noch kein Dokument, wird für diesen Erzeuger nichts eingereiht: Es gibt nichts nachzuziehen.
  Der Abschluss erwähnt es in einem Satz. Den Auftrag mit `pfade: .` reiht `/k-docs` trotzdem
  ein.
- **Ohne Kephalaion** (`kephalaion_unavailable`) wird nicht eingereiht, auch nicht auf die
  Platte. Der Abschluss nennt den fehlenden Nachzug als offen.
- **Keine Codeänderung.** Das entscheidet die Sitzung, nicht das Programm: Reine Änderungen
  an Handdoku, Tasks, Befunden oder der Konfiguration von k-playbook reihen nichts ein.

### Wer ihn abschließt

Der Erzeuger, mit `k_playbook_doc_done` — `code` je Auftrag, `tools` alle auf einmal.

**`code`.** Der Verarbeiter schließt Auftrag für Auftrag:

- Ein roher Auftrag wandert beim Zerlegen nach `done/`, sobald die zerlegten Aufträge, in denen
  er aufgegangen ist, in der Queue stehen. Folgt aus ihm kein Dokument, etwa weil alle seine
  Pfade ausgenommen sind, wandert er ebenfalls nach `done/`; der Abschluss des Laufs nennt ihn.
- Ein zerlegter Auftrag wird geschlossen, wenn er in diesem Lauf abgearbeitet ist und `publish`
  sein Dokument danach mit `created`, `written`, `deleted` oder `unchanged` meldet. `publish`
  meldet auch unberührte Dokumente als `unchanged`; ein noch nicht abgearbeiteter Auftrag
  bleibt deshalb offen.
- Er bleibt offen bei `conflict`, bei `failed` und bei jeder Ablehnung des ganzen Abgleichs
  (`invalid_input`, `hints_changed`).
- Lehnt der Nutzer ein `neu` oder `entfaellt` ab, wird der Auftrag ohne `publish` geschlossen;
  der Abschluss des Laufs nennt ihn.
- Ein abgebrochener Lauf setzt an den offenen zerlegten Aufträgen wieder an. Rohe Aufträge,
  die während des Laufs dazukommen, bleiben für den nächsten Lauf offen.

**`tools`.** Der Preflight von `/k-docs-tools` listet die offenen Aufträge mit Anlass und
Pfaden; nach einem erfolgreichen `publish` schließt er genau diese ab. Erfolgreich heißt: Die
Antwort von `publish` meldet keine offenen Dokumente (`open` leer), und `cleaned` ist wahr.
Aufträge, die nach dem Preflight dazugekommen sind, bleiben offen. Bricht der Lauf ab oder
meldet `publish` einen Konflikt, bleiben alle offen.

Die Einzelheiten stehen in `k-playbook:/commands/_docs/auftraege.md`, der Ablauf des
Verarbeiters in `k-playbook:/commands/k-docs-code.md`.

### Was nicht hineingehört

- **Das Versionsinventar.** `keph:/docs/generated/versions/inventory.md` wird sofort nachgezogen
  (`/k-doc-inventory`); der Lauf ist billig und lässt das Dokument ohne inhaltliche Änderung
  byte-gleich stehen.
- **Handdoku, `README.md`, projekteigene Regeln.** Sie werden im selben Arbeitsgang angepasst wie der
  Code, nicht eingereiht. Eine Handbuchseite unter `keph:/docs/manual/` ändert
  `k_playbook_doc_replace` stellenweise, `k_playbook_doc_append` hängt an — mit
  `since_revision`, der Revision aus `k_playbook_doc_grep` mit `count_only` oder aus
  `k_playbook_doc_read`; `foreign_change` heißt, jemand anderes hat inzwischen geschrieben,
  dann nicht still nachziehen. Nicht als Ganzes mit `write`; in der Shell ersatzweise
  `k-playbook doc replace|append <keph:-verweis> [--since N]`.
- **Ein Teil-Scope für `tools`.** Ein Auftrag für `tools` engt den Lauf nicht auf seine Pfade
  ein. Für `code` wählen die Pfade die Dokumente aus, begrenzen aber nicht, was der Verarbeiter
  für sie liest.
- **Inhalt für die Doku.** Was ein Erzeuger schreiben soll, findet er im Code. Erkenntnisse
  sind Befunde (`k-playbook:/instructions/on-demand/befunde.md`), Arbeitsstände gehören in die
  Workbench.
