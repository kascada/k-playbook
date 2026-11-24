---
title: Regeln eines schreibenden Docs-Laufs
description: "Modul für /k-docs-code und /k-docs-tools und ihre Sub-Agenten — was gilt, wenn ein Lauf erzeugte Dokumente selbst schreibt (nicht das Programm, wie beim Inventar): GENERATED_AT einmal je Lauf, Unklarheiten sammeln und vor publish vorlegen, keine Befunde, was nicht in die Doku gehört, und was ein Sub-Agent darf."
---

Diese Regeln gelten für jeden Lauf, der erzeugte Dokumente selbst schreibt, und für jeden
Sub-Agenten, der dabei hilft.

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
