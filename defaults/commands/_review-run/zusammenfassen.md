---
title: Baustein Zusammenfassen
description: Baustein der Review-Familie, eingebunden als include — die Fundlisten eines Arbeitsverzeichnisses mit k_playbook_review_merge_dir zu review-input.json zusammenführen (Dedupe, stabile Gruppen-IDs, Schwere) und danach mit dem Triage-Modul die Ergebnisdatei review-triage.md als Abhakliste schreiben; prüft, dass jeder Fund in einem Bündel oder im Restabschnitt steht.
---

Zusammenfassen macht aus den Fundlisten eines Arbeitsverzeichnisses die Ergebnisdatei. Es
hat zwei Teile: den Go-Merge, der deterministisch gruppiert, und die Triage, die bewertet.
Dieselbe Ergebnisform liefert ein eigenständiges Review mit einer Fundliste wie ein Audit mit
vielen.

Der aufrufende Command nennt:

- `WORK_DIR` — das Arbeitsverzeichnis mit den Fundlisten `<eintrag>.sarif`, absolut,
- den Kopf der Ergebnisdatei: Rezept, Scope in Worten, Datum (`now.date`) und den Weg
  (`/k-review` oder Audit), von dem im Triage-Modul der Kopf abhängt.

**1. Merge.** Rufe `k_playbook_review_merge_dir` mit `projectDir` = `project.dir` und
`dir` = `WORK_DIR`, ersatzweise in der Shell `k-playbook review merge <WORK_DIR>`. Er prüft
jede Fundliste noch einmal gegen ihr Rezept und schreibt `WORK_DIR/review-input.json` und
`WORK_DIR/review-input.md`, mit den Rechten `0600`. Bekannte Entscheidungen gleicht er
nicht ab; das steht in `warnings`.

- Antwortet er mit einem Fehler (`sarif_invalid`, `sarif_missing`, `read_failed`,
  `merge_failed`), ist nichts geschrieben. Brich ab und nenne Code und Grund; eine
  Ergebnisdatei ohne Beleg entsteht nicht.
- Merke dir `findings` und `groups` aus der Antwort.

**2. Triage.** Lies `k-playbook:/commands/_audit/review-scan-triage.md` mit
`k_playbook_doc_read` und wende es wortlaut-treu an: `RUN_DIR` ist `WORK_DIR`, der Kopf
kommt aus dem Aufruf. Es schreibt `WORK_DIR/review-triage.md`. Bündelung, Prioritäten
`P1`–`P3`, Kategorien `S`/`T`/`K`/`F`/`A`/`X`, die Gewichtung von KI-Evidence und das Format
der Abhakliste stehen dort und nirgends sonst.

**3. Vollständigkeit.** Die Summe der Spalte `Funde` über alle Zeilen der Tabelle in
`review-triage.md` ist `findings` aus dem Merge, und jede stabile Gruppen-ID aus
`review-input.json` steht in genau einem Bündel oder im Restabschnitt. Stimmt das nicht,
ergänze die fehlenden Gruppen im Restabschnitt, bevor du abschließt.

Zusammenfassen schreibt eine Fundliste `<eintrag>.sarif` bereinigt zurück, wenn die Prüfung im
Merge Funde außerhalb von `audit.scope.paths` verworfen hat; sonst ändert es keine Fundliste.
Daneben schreibt es nur den Beleg `review-input.json` und `review-input.md` und die
Ergebnisdatei `review-triage.md`.
