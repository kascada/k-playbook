---
title: Die Docs-Ablage in der Projekt-Collection
description: Was für jeden Docs-Command gilt — die Ablage unter keph:/docs/ in der Collection des Projekts, ein Schreiber ändert nur seine Dokumente in generated/<erzeuger>/, Erzeuger mit mehreren Dokumenten schreiben über k-playbook docs checkout/publish, der eine Kopf jedes Dokuments (title, description, generated samt stats, hints, questions, sources, verified), Regeln für einen schreibenden Lauf (Zeitpunkt, Unklarheiten, keine Befunde, Sub-Agenten), der Scope der Erzeuger, die den Code durchsuchen (/k-docs-code und /k-docs-tools), samt dauerhaften Ausschlüssen in docs.exclude, die Doku-Aufträge aus queue/docs/, die diese beiden im Preflight listen und nach einem erfolgreichen publish abschließen, README.md und index.md schreibt kein Command-Text, und ohne Kephalaion gibt es keinen Rückfall auf die Platte.
---

Die Projektdokumentation liegt in der Collection des Projekts unter `keph:/docs/`; das ist
der Teil der Collection, den Kephalaion indiziert. Darunter gilt:

- `docs/generated/<erzeuger>/` ist das Verzeichnis eines Erzeugers. Es kann mehrere
  Schreiber haben — in `code/` etwa `/k-docs-code` und `ks-overlay-repo-analyse`, erkennbar
  an `generated.by` im Frontmatter. Ein Lauf ändert nur die Dokumente, für die er zuständig
  ist; alle anderen bleiben unverändert. Ein Docs-Command schreibt nur in das Verzeichnis
  seines Erzeugers und nirgends sonst unter `docs/`.
- Ein Erzeuger mit mehreren Dokumenten schreibt über `k_playbook_docs_checkout` und
  `k_playbook_docs_publish` (`k-playbook docs checkout|publish <erzeuger>`): `checkout`
  holt das Verzeichnis in ein Arbeitsverzeichnis unter `.k-playbook/work/docs/<erzeuger>/`,
  dort wird mit den Datei-Werkzeugen geschrieben, `publish` gleicht ab und meldet, was
  inzwischen in Kephalaion geändert wurde, statt es zu überschreiben. Was im
  Arbeitsverzeichnis fehlt, löscht `publish` — ein Dokument, das der Lauf nicht behandelt,
  bleibt deshalb dort liegen.
- Jedes Dokument trägt den Kopf aus dem Abschnitt „Der Kopf“ unten; `hints` sind darin der
  einzige von Hand gepflegte Teil eines erzeugten Dokuments.
- Schreibst du in einem Lauf selbst erzeugte Dokumente (nicht das Programm, wie beim
  Inventar), gilt außerdem:
  - **Zeitpunkt:** `generated.at` ist `GENERATED_AT`, einmal je Lauf aus der Shell gesetzt
    (`date -Iseconds`) und für alle Dokumente des Laufs derselbe — nie geschätzt, gerundet
    oder erfunden.
  - **Unklarheiten** sind etwas anderes als `questions`: `questions` beantwortet das Dokument,
    eine Unklarheit kann es nicht beantworten. Fragen statt raten — Unklarheiten je Dokument
    sammeln und **vor `publish`** gebündelt vorlegen, höchstens drei je Dokument, die
    wichtigsten zuerst. Beantwortetes einarbeiten; was offen bleibt, nennt die
    Zusammenfassung am Ende.
  - **Keine Befunde.** Was der Lauf findet — Abweichungen zur bisherigen Fassung, mögliche
    Fehler, Altlasten —, gehört ins passende Dokument und wird nicht zusätzlich nach
    `keph:/queue/findings/` geschrieben: Die Doku ist schon die Ablage, in die ein Befund
    übernommen würde.
  - **Nicht in die Doku:** Ticket-, Testlauf- und Reviewprotokolle und der Stand laufender
    Vorhaben; der gehört in die Workbench.
  - **Sub-Agenten** bekommen `GENERATED_AT`, diese Regeln und den Kopf mit. Sie schreiben
    höchstens ihre eigenen Dateien im Arbeitsverzeichnis — nicht nach Kephalaion, kein
    checkout oder publish, keine Befunde, keine Rückfragen an den Nutzer — und melden ihre
    Unklarheiten und die Stellen zurück, an denen die bisherige Fassung nicht mehr stimmt.
    Sammeln, vorlegen und veröffentlichen tut nur der Lauf selbst.
- Dauerhafte Ausschlüsse für den Code-Scan eines Projekts stehen in `docs.exclude` von
  `K-PLAYBOOK.yaml`: gelesen aus `context.docs.exclude`, eingetragen nur über
  `k_playbook_docs_exclude_add` (`k-playbook docs exclude add`) nach Bestätigung durch den
  Nutzer. Die Datei selbst liest und bearbeitet kein Docs-Command. Wie sie wirken, steht im
  Abschnitt „Scope“ unten.
- `docs/manual/` ist von Hand gepflegt; `docs/sources/` verlinkt Quellen außerhalb von
  Kephalaion. Beides fasst ein Erzeuger nicht an.
- `README.md` und `index.md` schreibt kein Command-Text: Die `README.md` von
  `generated/<erzeuger>/` schreibt `publish` aus der Erzeugertabelle im Programm, `index.md`
  entsteht später beim Indizieren. Auch kein Eintrag „schon einmal“ — er wäre beim nächsten
  Schreiben weg.
- Gelesen und geschrieben wird über die Verweisregel aus dem Briefing: `keph:`-Verweise mit
  `k_playbook_doc_read` bzw. Kephalaion `read`, Collection = `context.kephalaion.address`.
- Steht `context.kephalaion.ok` nicht auf `true`, ist die Ablage nicht erreichbar. Dann gibt es
  kein Ergebnis und keinen Rückfall auf die Platte: nichts nach `k-playbook-local/docs/`
  schreiben, nichts von dort als gültig lesen. Sag, dass Kephalaion nicht bereit ist, nenne
  `context.kephalaion.message`, und verweise auf die Projekt-Collection auf der Setup-Seite
  (`/k-gui`).

### Der Kopf

Jedes Dokument unter `keph:/docs/` hat denselben Kopf, erzeugt oder von Hand geschrieben:
Kephalaion indiziert alles zusammen. Die Felder in dieser Reihenfolge:

```yaml
---
title: <Titel>
description: <Ein Satz: was dieses Dokument erklärt.>
generated: { by: <erzeuger>, at: <GENERATED_AT>, stats: { … } }
hints: [ "<worauf der nächste Lauf achten soll>", … ]
questions: [ "<Frage, die genau dieses Dokument beantwortet>", … ]
sources:
  - resource: <keph:-Verweis oder URL>
    item: <Eintrag darin>
    version: <Stand, gegen den gelesen wurde>
verified: <nur ein Mensch>
---
```

- `title` — Pflicht.
- `description` — Pflicht, ein Satz. Ohne `title` oder `description` lehnt `publish` ab und
  schreibt nichts.
- `generated` — nur bei erzeugten Dokumenten. `by` ist der Schreiber (`k-docs-code`,
  `ks-overlay-repo-analyse`, `k-docs-tools`, `k-doc-inventory`), `at` der Zeitpunkt des
  Laufs. `stats` ist optional und nimmt eigene Zahlen eines Erzeugers auf, etwa die Zählungen
  des Inventars; es ist kein Suchinhalt, das Indizieren übergeht es.
- `hints` — optional, der einzige von Hand gepflegte Teil eines erzeugten Dokuments: kurze
  Sätze, worauf der nächste Lauf achten soll. Der Erzeuger befolgt sie und übernimmt sie
  unverändert. Neue schlägt er nur vor und trägt sie erst nach Bestätigung ein; dasselbe gilt,
  wenn ein entfallendes Dokument `hints` hat. `publish` lässt geänderte `hints` nur mit
  `allow_hints` durch, nach Bestätigung durch den Nutzer.
- `questions` — die Fragen, die ein Neuer in der ersten Woche stellt und die genau dieses
  Dokument beantwortet. Eine Frage, eine Datei: Verteilt sich eine Antwort über mehrere
  Dokumente, ist der Zuschnitt falsch.
- `sources` — die Quellen außer dem Code, gegen die gelesen wurde; fehlt, wenn es keine gab.
  Je Eintrag ist `resource` Pflicht, ein `keph:`-Verweis oder eine URL; `item` nennt einen
  Eintrag darin, `version` den Stand, gegen den gelesen wurde.
- `verified` — trägt nur ein Mensch ein. Ein Erzeuger setzt es nie und übernimmt es nicht
  von sich aus.

Darüber hinaus schreibt kein Erzeuger ein Kopffeld. Die Felder älterer Fassungen — `type`,
`tags`, `status`, `lib`, `version`, `version-pin`, `severity`, `last-reviewed`, ein eigener
Block wie `inventory:` — entfallen oder gehen in diese Felder auf. Die Vorlage eines
Erzeugers zeigt, wie er diesen Kopf belegt; sie erweitert ihn nicht.

### Scope

Dieser Abschnitt gilt nur für die Erzeuger, die den Code durchsuchen: `/k-docs-code` und
`/k-docs-tools`. `/k-doc-inventory` und `ks-overlay-repo-analyse` binden dieses Modul auch
ein, wenden den Scope aber nicht an — das Inventar liest seine Quellen nach seinem eigenen
Vertrag, der Overlay-Skill die Repos, die er analysiert.

Aus dem Briefing brauchst du dafür `context.project.dir`, `context.project.repoRoot`,
`context.local.dir`, `context.playbook.dir`, dazu `context.docs.exclude` und
`context.docs.problems` — die dauerhaften Ausschlüsse des Projekts aus `K-PLAYBOOK.yaml` und
die Einträge, die dort verworfen wurden. Der Scope ergibt sich aus Briefing und Argument,
nicht geraten:

- `SCAN_ROOT = context.project.repoRoot` — immer, nicht das aktuelle Verzeichnis.
- **Stand** — beschrieben wird der Arbeitsbaum von `SCAN_ROOT`, wie er ausgecheckt ist.
  Branch und Commit stehen im Preflight (`git -C <SCAN_ROOT> rev-parse --abbrev-ref HEAD`,
  `git -C <SCAN_ROOT> log -1 --format=%h`, dazu ob `git status --porcelain` leer ist). Ist es
  nicht der Standard-Branch oder ist der Arbeitsbaum nicht sauber, fragst du im Preflight,
  welcher Stand beschrieben wird. Ein anderer Stand wird mit `git show <ref>:<pfad>` und
  `git grep <muster> <ref>` gelesen; der Arbeitsbaum wird nicht umgeschaltet.
- `$ARGUMENTS` ergänzt den Scope um **genau einen** Pfad, statt ihn zu ersetzen. Er muss
  existieren und innerhalb von `context.project.dir` liegen; relativ gilt er ab
  `context.project.dir`. Fehlt er oder liegt er außerhalb, abbrechen und den Pfad nennen —
  ein anderes Projekt wird dort dokumentiert, nicht von hier. `EXTRA_PATH` ist dieser Pfad,
  sonst leer.
- **Immer ausgeschlossen**, auch wenn das Argument hineinzeigt: `context.playbook.dir`
  (`k-playbook/`), `context.local.dir` (`k-playbook-local/`) und
  `<context.project.dir>/.k-playbook/`. Zeigt `$ARGUMENTS` in einen davon, abbrechen und
  sagen, dass diese Verzeichnisse nie zum Scan gehören.
- **Versteckte Pfade** sind ausgeschlossen, ohne Rückfrage: jede Datei und jedes
  Verzeichnis, dessen Name mit `.` beginnt, auf jeder Ebene — etwa `.git/`, `.github/`,
  `.claude/`, `.opencode/`, `.devcontainer/`, `.venv/`, `.env`. Ein versteckter Pfad kommt
  nur in den Scan, wenn `$ARGUMENTS` oder ein `hint` ihn ausdrücklich nennt; `.git/` und
  `.k-playbook/` nie.
- **Weitere Ausschlüsse**, ohne Rückfrage: `venv/`, `env/`, `node_modules/`, `dist/`,
  `build/`, `target/`, `_bases/`, `__pycache__/`, `coverage/`, `htmlcov/`,
  `tests/fixtures/`, `**/*.min.js`, `**/*.lock`, `**/*.bundle.*` — dazu alles, was eine
  `.gitignore` unter `SCAN_ROOT` ausnimmt (Einträge als Glob-Muster, verschachtelte
  `.gitignore` nach bestem Vermögen).
- **Aus `K-PLAYBOOK.yaml`**, ohne Rückfrage: die Muster aus `context.docs.exclude`. Sie
  gelten wie in einer `.gitignore` unter `SCAN_ROOT`. Ein Pfad, den `$ARGUMENTS`
  ausdrücklich nennt, wird trotzdem gescannt, auch wenn ein Eintrag ihn ausschließt. Was
  unter `context.docs.problems` steht, wurde verworfen und gilt nicht; der Preflight nennt
  es.
- **Zusatzquelle aus einem `hint`.** Nennt ein `hint` eine Zusatzquelle innerhalb von
  `context.project.dir`, gehört sie für dieses Dokument zum Scan — mit denselben
  Ausschlüssen, auch denen aus `context.docs.exclude` und `NEW_EXCLUDES`, nie die immer
  ausgeschlossenen Verzeichnisse.

**Preflight.** Der Command zeigt den Scope in seinem Preflight mit diesen Zeilen und lässt
den Preflight **einmal** bestätigen:

```text
Scope         <SCAN_ROOT> (project.repoRoot)
Stand         <branch> @ <commit>, sauber | geändert | beschrieben wird <ref>
Zusätzlich    <EXTRA_PATH> | —
Immer außen   k-playbook/, k-playbook-local/, .k-playbook/, .git/
Versteckt     ausgeschlossen (.*) | ausgenommen: <aus Argument oder hint>
Aus K-PLAYBOOK.yaml  <context.docs.exclude, durch Komma getrennt> | —
              verworfen: <pattern> — <reason>   (je Eintrag aus context.docs.problems)
Ausschlüsse   <kompakte Liste der übrigen wirksamen Ausschlüsse>
```

Der Nutzer darf dabei Ausschlüsse ergänzen. Ergänzte Ausschlüsse gelten für diesen Lauf; du
merkst sie dir als `NEW_EXCLUDES`, Muster wie in einer `.gitignore`, relativ zu `SCAN_ROOT`.
Ob sie gespeichert werden, fragt erst der Abschluss.

**Ausschlüsse speichern.** Hat der Nutzer im Preflight Ausschlüsse ergänzt
(`NEW_EXCLUDES`), fragst du im Abschluss einmal, ob sie dauerhaft in `K-PLAYBOOK.yaml`
gespeichert werden sollen, und nennst sie dabei. Nach Ja rufst du
`k_playbook_docs_exclude_add` mit `projectDir` = `context.project.dir` und `patterns` =
`NEW_EXCLUDES` auf (ersatzweise `k-playbook docs exclude add <muster>… --json`) und nennst
`exclude` und `rejected` aus der Antwort. Danach lädst du das Briefing neu
(`k-playbook:/instructions/on-demand/briefing-felder.md`, „Neu laden“). Bei Nein, oder wenn
nichts ergänzt wurde, entfällt dieser Teil. Die Datei liest und bearbeitest du nie selbst.

- `rejected` nennt Einträge → mit `reason` nennen; sie stehen nicht in der Datei, die
  übrigen sind trotzdem eingetragen.
- `unsupported` → `docs.exclude` steht in einer Form, die das Programm nicht sicher ergänzt
  (etwa `exclude: [docs/]`); nichts ist eingetragen. Die Meldung mit Zeile und Grund nennen;
  der Nutzer stellt die Liste selbst um, du bearbeitest `K-PLAYBOOK.yaml` nicht.
- Das Werkzeug ist unbekannt, oder `k-playbook docs` meldet `unbekannter Unterbefehl` → die
  Installation ist älter als der Command: melden, `/k-upgrade` nennen, nichts von Hand in
  `K-PLAYBOOK.yaml` schreiben.

### Doku-Aufträge

Dieser Abschnitt gilt wie „Scope“ nur für `/k-docs-code` und `/k-docs-tools`. Nach einer
Codeänderung reiht eine Sitzung den Nachzug als Auftrag in `keph:/queue/docs/` ein, statt den
Erzeuger sofort laufen zu lassen; Kopf und Lebenslauf stehen in
`k-playbook:/instructions/on-demand/doku-auftraege.md`. Ein Erzeuger arbeitet die Aufträge
seines `erzeuger` ab: `/k-docs-code` die mit `erzeuger: code`, `/k-docs-tools` die mit
`erzeuger: tools` — dessen Verzeichnis heißt `libs`, der Auftrag `tools`.

- **Preflight.** Vor dem Preflight suchst du die offenen Aufträge deines Erzeugers:
  `k_playbook_doc_grep` mit `kind: docs` (ohne `done`) und dem Muster `^erzeuger: code$`
  bzw. `^erzeuger: tools$` (ersatzweise `k-playbook doc grep --kind docs '<muster>' --json`).
  Jeder Treffer ist ein Auftrag; lies ihn mit `k_playbook_doc_read` und nimm aus dem Kopf
  `anlass`, `pfade` und `entstanden`. Der Preflight listet sie:

  ```text
  Aufträge      <N> offen | keine
                <datei> (<entstanden>) — <anlass>
                  <pfad>, <pfad>, …
  ```

  Diese Liste merkst du dir als `ORDERS`. Die Aufträge begründen den Lauf; sie engen den
  Scope nicht ein, der Lauf bleibt ganz.
- **Abschließen.** Nach einem erfolgreichen `publish` schließt du genau die Aufträge aus
  `ORDERS` ab, je einen mit `k_playbook_doc_done` (ersatzweise `k-playbook doc done <ref>
  --json`); sie liegen danach unter `keph:/queue/docs/done/`. Erfolgreich heißt: Die Antwort
  von `publish` meldet keine offenen Dokumente (`open` leer), und `cleaned` ist wahr.
  Aufträge, die nach dem Preflight angelegt wurden, stehen nicht in `ORDERS` und bleiben
  offen; sie gehören zum nächsten Lauf.
- **Offen lassen.** Bricht der Lauf ab, meldet `publish` einen Konflikt, eine Ablehnung oder
  `failed`, oder bleibt etwas offen, schließt du keinen Auftrag ab. Ein `publish`, der nach
  einem behobenen Konflikt doch noch erfolgreich durchläuft, schließt `ORDERS` ab.
- **Rückmeldungen.** `wrong_dir` oder `not_found` bei `k_playbook_doc_done` heißt, ein
  anderer Lauf hat den Auftrag schon abgeschlossen — nennen, nicht wiederholen.
  `outcome_unknown` heißt, das Verschieben kann angekommen sein: nicht wiederholen, sondern mit
  `k_playbook_doc_list` (`kind: docs`, `done: true`) nachsehen. Kennt das Werkzeug die Sorte
  `docs` nicht (`invalid_input`), ist die Installation älter als der Command: melden,
  `/k-upgrade` nennen; der Lauf selbst bleibt gültig.
- **Abschluss.** Die Zusammenfassung nennt die abgeschlossenen Aufträge mit Verweis und die,
  die offen geblieben sind, mit Grund.
