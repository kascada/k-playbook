---
title: Code analysieren für die Doku nach Bedeutung
description: Modul für /k-docs-code, vom Hauptkontext gelesen, wenn er den Code für die Zerlegung analysiert — worauf es ankommt (Subsysteme, Querschnittsthemen, Einstiegspunkte, Kopplungen, Datenmodell, Kernlogik, Altlasten), was nicht aufgenommen wird, was im vollen Lauf und im Teil-Lauf analysiert wird und was ein Sub-Agent je Thema bekommt.
---

**Keine Datei-für-Datei-Beschreibung.** Ergebnis der Analyse ist ein Verständnis nach
Bedeutung, nicht nach Verzeichnissen. Suche nach:

- **Fachlichen Subsystemen** — eigenständige Verantwortungsbereiche, gebündelt über
  Datenmodelle, Aufrufwege und gemeinsames Vokabular.
- **Querschnittsthemen** — Auth, Konfiguration und Secrets, Logging, Fehlerbehandlung,
  Persistenz, HTTP-Oberfläche, Hintergrundjobs, Feature-Flags, Wiederholungen und
  Zeitgrenzen.
- **Einstiegspunkten** — was startet was.
- **Externen Kopplungen** — Datenbanken, Queues, fremde APIs, Auth-Anbieter, Cloud-Dienste:
  wo sie im Code sitzen und wo sie konfiguriert sind.
- **Datenmodell** — zentrale Entitäten, wo definiert, wo umgeformt.
- **Kernlogik gegen Framework-Standard und Boilerplate** — nur Kernlogik verdient tiefe Doku.
- **Altlasten** — sichtbare Deprecations, TODOs, alte Muster, getrennt notiert.

Nicht aufnehmen: Framework-Standard ohne Eigenanteil, Kommentare, die den Code
nacherzählen, Signaturen einzelner Funktionen — dazu, was die Docs-Ablage ausschließt. Was
die Analyse an möglichen Fehlern und Altlasten findet, steht unter `## Edge cases` oder in
einem Dokument für bekannte Macken, nicht in Befunden.

**Was analysiert wird.** Im vollen Lauf der ganze Scope. Im Teil-Lauf nur, was die
Dokumente der Aufträge brauchen: die Pfade der Aufträge und, je betroffenem Dokument, die
Stellen, die zu seinem Thema gehören. Der Rest des Projekts wird nicht gescannt. In der
Wiederaufnahme ohne rohe Aufträge entfällt die Analyse; es wird je Auftrag gelesen, wenn er
drankommt.

**Sub-Agenten.** Bei sehr großen Repos teilst du Analyse und Schreiben auf, einen Sub-Agenten
je Thema. Sein Auftrag beginnt mit: Führe zuerst `k-playbook briefing` aus und lies die
Ausgabe; er liest als Anleitung `k-playbook:/commands/_docs-code/schreiben.md`
(`k_playbook_doc_read`, `projectDir` = `project.dir`; die Werkzeuge
`mcp__k-playbook__*` lädt er in Claude Code vorher mit der Werkzeugsuche). Du gibst ihm:
`SCAN_ROOT`, die Ausschlüsse und den Stand aus dem Preflight, sein Thema, die Pfade und
Anlässe seines Auftrags, `WORK_DIR`, `GENERATED_AT`, die bisherige Fassung seines Dokuments
samt `hints` und die Regel „nur Bedeutung, keine Zeilenlisten“. Gib ihm auch die Leitlinien dieses Moduls (Suche nach Bedeutung, Nicht aufnehmen) mit. Er meldet Dateiname,
Zeilenzahl, Unklarheiten und die Stellen, an denen die bisherige Fassung nicht mehr stimmt.
Ein Sub-Agent schreibt keine Aufträge und schließt keine ab.
