---
title: Scope der Docs-Erzeuger, die den Code durchsuchen
description: Modul für /k-docs-code und /k-docs-tools, vom Hauptkontext gelesen im ersten Schritt — den Scope aus k-playbook docs scope holen (SCAN_ROOT, Stand, Zusatzpfad, Ausschlüsse nach Herkunft), die Scope-Zeilen des Preflights, was die Sitzung selbst entscheidet (anderer Stand, ergänzte Ausschlüsse), und wie ergänzte Ausschlüsse nach Bestätigung mit k_playbook_docs_exclude_add gespeichert werden.
---

Der Scope gilt nur für die Erzeuger, die den Code durchsuchen: `/k-docs-code` und
`/k-docs-tools`. `/k-doc-inventory` liest seine Quellen nach seinem eigenen Vertrag,
`ks-overlay-repo-analyse` die Repos, die er analysiert; beide wenden ihn nicht an.

**Rechnen lässt ihn das Programm.** Rufe `k_playbook_docs_scope` mit `projectDir` =
`project.dir` auf (ersatzweise `k-playbook docs scope [<pfad>] [--check <pfad>…]
--json`). Die Ausschlüsse — immer, versteckte Pfade, weitere, `.gitignore`, `docs.exclude`
samt verworfenen Einträgen —, ihre Wirkung und die Prüfung eines Zusatzpfads stehen nur im
Programm; dieser Text wiederholt sie nicht, und du rechnest sie nicht nach.

- `pfad` ist `EXTRA_PATH`, der **eine** Zusatzpfad aus `$ARGUMENTS`; ohne Argument oder wo der
  Command keinen nimmt, bleibt es weg. Die Antwort `invalid_input` heißt: der Pfad fehlt auf
  der Platte, liegt außerhalb von `project.dir` oder in einem immer ausgeschlossenen
  Verzeichnis — abbrechen und den Pfad nennen; ein anderes Projekt wird dort dokumentiert,
  nicht von hier.
- `check` nimmt Pfade (etwa die eines Auftrags) und meldet je Pfad `excluded` mit `reason`
  und `by`: Ein ausgenommener Pfad führt zu keinem Dokument und ist nicht erreichbar. Dazu
  `exists` (ob es den Pfad gibt) und `suggest` (der berichtigte Pfad, wenn das
  Repo-Verzeichnis doppelt vorangestellt ist).
- Aus der Antwort: `scanRoot` ist `SCAN_ROOT` (immer `project.repoRoot`, nicht das
  aktuelle Verzeichnis), `stand` Branch, Commit und `clean`, `extra` den geprüften
  Zusatzpfad, `exclude` die Ausschlüsse nach Herkunft (`always`, `hidden`, `further`,
  `gitignore`, `config`, `rejected`).
- Ein versteckter Pfad kommt nur in den Scan, wenn `$ARGUMENTS` oder ein `hint` ihn
  ausdrücklich nennt; eine Zusatzquelle aus einem `hint` gehört für dieses Dokument zum Scan,
  mit denselben Ausschlüssen — auch denen aus `NEW_EXCLUDES`, nie den immer ausgeschlossenen
  Verzeichnissen. Immer ausgeschlossen sind `.k-playbook/` nur direkt unter `project.dir`
  (in der Antwort mit Anker: `/.k-playbook/`) — ein tieferes Verzeichnis dieses Namens gehört
  zum Scan wie jedes andere — sowie `keph-backup/` und `.git/` auf jeder Ebene.
- Ist `gitignoreTruncated` gesetzt, hat die Grenze von 200 gelesenen `.gitignore` gegriffen: so
  viele Dateien blieben ungelesen; der Scope bleibt gültig, nenne den Hinweis im Preflight.

**Stand.** Beschrieben wird der Arbeitsbaum von `SCAN_ROOT`, wie er ausgecheckt ist. Ist
`stand.git` wahr und der Branch nicht der Standard-Branch, oder ist `clean` falsch, fragst du
im Preflight, welcher Stand beschrieben wird. Ein anderer Stand wird mit `git show <ref>:<pfad>`
und `git grep <muster> <ref>` gelesen; der Arbeitsbaum wird nicht umgeschaltet.

**Preflight.** Der Command zeigt den Scope mit diesen Zeilen, aus der Antwort gefüllt, und
lässt den Preflight **einmal** bestätigen:

```text
Scope         <scanRoot> (project.repoRoot)
Stand         <branch> @ <commit>, sauber | geändert | beschrieben wird <ref>
Zusätzlich    <extra.path> | —
Immer außen   /.k-playbook/, keph-backup/, .git/
Versteckt     ausgeschlossen (.*) | ausgenommen: <aus Argument oder hint>
Aus K-PLAYBOOK.yaml  <exclude.config, durch Komma getrennt> | —
              verworfen: <pattern> — <reason>   (je Eintrag aus exclude.rejected)
Ausschlüsse   <kompakte Liste der übrigen wirksamen Ausschlüsse>
```

Der Nutzer darf dabei Ausschlüsse ergänzen. Ergänzte Ausschlüsse gelten für diesen Lauf; du
merkst sie dir als `NEW_EXCLUDES`, Muster wie in einer `.gitignore`, relativ zu `SCAN_ROOT`.
Ob sie gespeichert werden, fragt erst der Abschluss.

**Ausschlüsse speichern.** Hat der Nutzer im Preflight Ausschlüsse ergänzt, fragst du im
Abschluss einmal, ob sie dauerhaft in `K-PLAYBOOK.yaml` gespeichert werden sollen, und nennst
sie dabei. Nach Ja rufst du `k_playbook_docs_exclude_add` mit `projectDir` =
`project.dir` und `patterns` = `NEW_EXCLUDES` auf (ersatzweise
`k-playbook docs exclude add <muster>… --json`) und nennst `exclude` und `rejected` aus der
Antwort. Danach lädst du das Briefing neu (`k-playbook:/instructions/on-demand/briefing-felder.md`,
„Neu laden“). Bei Nein, oder wenn nichts ergänzt wurde, entfällt dieser Teil. Die Datei liest
und bearbeitest du nie selbst.

- `rejected` nennt Einträge → mit `reason` nennen; sie stehen nicht in der Datei, die
  übrigen sind trotzdem eingetragen.
- `unsupported` → `docs.exclude` steht in einer Form, die das Programm nicht sicher ergänzt
  (etwa `exclude: [docs/]`); nichts ist eingetragen. Die Meldung mit Zeile und Grund nennen;
  der Nutzer stellt die Liste selbst um, du bearbeitest `K-PLAYBOOK.yaml` nicht.

**Ältere Installation.** Ist `k_playbook_docs_scope` unbekannt (`invalid_input` für das
Werkzeug), meldet `k-playbook docs` `unbekannter Unterbefehl` oder `unbekanntes Kommando`
(auch bei `exclude add`), ist die Installation älter als der Command: melden, `/k-upgrade`
nennen, nichts von Hand rechnen oder in `K-PLAYBOOK.yaml` schreiben.
