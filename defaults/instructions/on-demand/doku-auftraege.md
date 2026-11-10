---
title: Doku-Aufträge
description: Wie nach einer Codeänderung der Nachzug der erzeugten Doku eingereiht wird — ein Dokument je Sitzung und Erzeuger in queue/docs/, Kopf mit erzeuger, pfade, anlass, sitzung und entstanden, wer schreibt und wer abschließt, die Prüfung auf einen vorhandenen Auftrag, wann nichts eingereiht wird und was nicht hineingehört.
---

## Doku-Aufträge

Ändert eine Sitzung Code, veraltet die erzeugte Doku unter `keph:/docs/generated/code/` und
`keph:/docs/generated/libs/`. Sie neu zu erzeugen ist ein ganzer Lauf von `/k-docs-code` bzw.
`/k-docs-tools` über das ganze Projekt. Statt diesen Lauf nach jeder Änderung zu starten, reiht
die Sitzung einen **Doku-Auftrag** ein; der Erzeuger arbeitet die Aufträge gebündelt ab. Das ist
der Normalfall nach einer Codeänderung. Den Erzeuger sofort laufen zu lassen bleibt erlaubt;
dann schließt er den Auftrag gleich mit ab.

Ein Auftrag ist ein Merkzettel, kein Rohmaterial: Er sagt, dass und warum nachzuziehen ist,
nicht was in der Doku stehen soll.

### Wohin

- **Ein Dokument je Auftrag** in `keph:/queue/docs/`. Anlegen mit `k_playbook_doc_create`,
  Sorte `docs`, Kurzname: Erzeuger und Anlass in wenigen Worten, etwa `code-doc-operationen`.
  Die Antwort nennt den Verweis. Den Inhalt schreibst du gleich danach mit Kephalaion `write`,
  `collection` gleich `kephalaion.address`, `name` gleich dem Pfad ohne `keph:/`, Frontmatter
  eingeschlossen.
- **Ein Auftrag je Sitzung und Erzeuger.** Ändert eine Sitzung mehrmals Code, bleibt es bei
  einem Auftrag für `code`; berührt sie auch die Abhängigkeiten, kommt höchstens einer für
  `tools` dazu.
- **Erledigt** liegt ein Auftrag unter `keph:/queue/docs/done/`; dorthin verschiebt ihn
  `k_playbook_doc_done`. Die Sorte hat keinen Userbereich: Jeder Erzeuger-Lauf schließt jeden
  Auftrag ab, gleich wer ihn angelegt hat.

### Kopf

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

- `title` zeigen `k_playbook_doc_list` und `/k-docs` an; der Wert steht wie jeder Wert in freiem
  Text (auch `anlass`) in doppelten Anführungszeichen, ein `"` im Wert wird `\"`, damit jeder
  YAML-Leser genau diesen Text liest, auch bei „: “ oder „ #“. `erzeuger`, `pfade`, `sitzung`
  und `entstanden` bleiben, wie die Vorlage sie zeigt.
- `erzeuger` ist `code` für `keph:/docs/generated/code/` (`/k-docs-code`) und `tools` für
  `keph:/docs/generated/libs/` (`/k-docs-tools`). Ein Wert, kein zweiter: Wer beides braucht,
  legt zwei Aufträge an. Die Zeile steht genau so im Kopf, `erzeuger: code` bzw.
  `erzeuger: tools`, weil der Erzeuger mit `k_playbook_doc_grep` danach sucht.
- `pfade` sind die geänderten Dateien oder Verzeichnisse, relativ zu `project.repoRoot`, je
  einer pro Zeile. Sie begründen den Lauf und erscheinen in seinem Preflight; sie engen den
  Scope nicht ein. Der Erzeuger läuft immer über das ganze Projekt.
- `anlass` ist ein Satz: was sich geändert hat, nicht was die Doku sagen soll.
- `sitzung` ist die Kennung deiner Sitzung, das Feld `session` des Briefings (im Klartext die
  Zeile `Sitzung:`). Hast du das Briefing neu geladen, nimm die neue Kennung.
- `entstanden` ist `context.now.date`. `/k-docs` nennt den ältesten offenen Auftrag danach.

Der Rumpf darf leer bleiben. Mehr als ein kurzer Absatz gehört nicht hinein.

### Wer ihn schreibt

- **Jede Sitzung nach einer Codeänderung**, bevor sie die Änderung abschließt — als Teil des
  Doku-Nachzugs aus `k-playbook:/instructions/always/220-regeln-anwenden.md`. Bei
  `/k-task-run` ist das der Hauptkontext im
  Abschluss, nicht der Ausführungs-Sub-Agent.
- **`/k-enforcement`** nach Freigabe durch den Nutzer, wenn sein Bericht für die erzeugte Doku
  „nachzuziehen“ ergibt.

`code` ist einzureihen, wenn sich Code des Projekts geändert hat, den `/k-docs-code` erfasst:
Quelltext unter `project.repoRoot` außerhalb von `context.docs.exclude`. `tools` ist
einzureihen, wenn sich die Bibliotheken oder Werkzeuge des Projekts geändert haben: eine neue
oder entfernte Abhängigkeit, ein Versionssprung, ein anderes Werkzeug im Build.

### Kein Doppel

Vor dem Anlegen prüft der Schreiber, ob schon ein offener Auftrag derselben Sitzung für
denselben Erzeuger besteht: `k_playbook_doc_grep` über die Sorte `docs` (ohne `done`) mit dem
Muster `^sitzung: <session>$`, dann für die Treffer `^erzeuger: code$` bzw.
`^erzeuger: tools$`. Gibt es ihn, legt er keinen neuen an; der Lauf ist ohnehin ganz, die Pfade
des vorhandenen Auftrags genügen. `/k-danke` und `/k-enforcement` zählen einen vorhandenen
Auftrag als „eingereiht“.

### Wann nicht eingereiht wird

- **Noch keine erzeugte Doku.** Liegt unter `keph:/docs/generated/code/` bzw.
  `keph:/docs/generated/libs/` noch kein Dokument (Kephalaion `list` auf `docs/generated/code`
  bzw. `docs/generated/libs`), wird für diesen Erzeuger nichts eingereiht: Es gibt nichts
  nachzuziehen. Der Abschluss erwähnt es in einem Satz.
- **Ohne Kephalaion** (`kephalaion.ok` nicht `true`, oder `kephalaion_unavailable`) wird nicht
  eingereiht, auch nicht auf die Platte. Der Abschluss nennt den fehlenden Nachzug als offen.
- **Keine Codeänderung.** Reine Änderungen an Handdoku, Tasks, Befunden oder der Konfiguration
  von k-playbook reihen nichts ein.

### Wer ihn abschließt

Der Erzeuger, nach einem erfolgreichen `publish`. Sein Preflight listet die offenen Aufträge
seines `erzeuger` mit Anlass und Pfaden; nach dem `publish` schließt er genau diese mit
`k_playbook_doc_done` ab. Erfolgreich heißt: Die Antwort von `publish` meldet keine offenen
Dokumente (`open` leer), und `cleaned` ist wahr. Aufträge, die nach dem Preflight dazugekommen
sind, bleiben offen. Bricht der Lauf ab oder meldet `publish` einen Konflikt, bleiben alle
offen. Die Einzelheiten stehen in `k-playbook:/commands/_shared/docs.md`.

### Was nicht hineingehört

- **Das Versionsinventar.** `keph:/docs/generated/versions/inventory.md` wird sofort nachgezogen
  (`/k-doc-inventory`); der Lauf ist billig und lässt das Dokument ohne inhaltliche Änderung
  byte-gleich stehen.
- **Handdoku, `README.md`, `AGENTS.md`.** Sie werden im selben Arbeitsgang angepasst wie der
  Code, nicht eingereiht. Eine Handbuchseite unter `keph:/docs/manual/` ändert
  `k_playbook_doc_replace` stellenweise, `k_playbook_doc_append` hängt an — mit
  `since_revision`, der Revision aus `k_playbook_doc_grep` mit `count_only` oder aus
  `k_playbook_doc_read`; `foreign_change` heißt, jemand anderes hat inzwischen geschrieben,
  dann nicht still nachziehen. Nicht als Ganzes mit `write`; in der Shell ersatzweise
  `k-playbook doc replace|append <keph:-verweis> [--since N]`.
- **Ein Teil-Scope.** Ein Auftrag engt den Lauf nicht auf seine Pfade ein.
- **Inhalt für die Doku.** Was ein Erzeuger schreiben soll, findet er im Code. Erkenntnisse
  sind Befunde (`k-playbook:/instructions/on-demand/befunde.md`), Arbeitsstände gehören in die
  Workbench.
