---
name: k-ai-session-memory
description: Use when the user wants existing project documentation (docs/, README, analysis notes) to be automatically loaded and prioritized by AI sessions instead of re-analyzed. Sets up AGENTS.md, opencode.json with instructions + references, and a keyword-indexed docs/README.md so future sessions consult docs first before reading code. Trigger keywords - "in memory", "damit du das nächste Mal weißt", "session context", "docs zuerst", "AGENTS.md", "OpenCode Konfig".
---

# Skill: AI Session Memory

**Kurzfassung.** Sorgt dafür, dass in einem Projekt vorhandene Dokumentation
in jeder OpenCode-Session **automatisch als autoritative Quelle** genutzt
wird, statt dass der AI-Assistent jedes Mal wieder von vorn Code analysiert.

## Wann anwenden

Immer wenn:

1. Recherche/Analyse-Arbeit gemacht wurde und in `docs/` abgelegt ist.
2. Der Nutzer merkt: „Warum durchsucht die AI jedes Mal wieder alles?"
3. Ein neues Projekt eingerichtet wird, in dem später Docs entstehen werden.
4. Der Nutzer explizit sagt: „das soll in dein Memory", „damit du das
   beim nächsten Mal weißt", „lass uns das aufschreiben".

## Was einrichten

Drei zusammenspielende Bausteine:

1. **`AGENTS.md`** im Projekt-Root – wird von OpenCode in jede Session
   injiziert. Sagt der AI: „Docs sind autoritativ, konsultiere sie
   ZUERST."
2. **`opencode.json`** im Projekt-Root – bindet `AGENTS.md` als
   `instructions` ein und registriert `docs/` als beschriebene
   `references`.
3. **`docs/README.md`** mit **Stichwort-Index** (A–Z) und
   **„Häufige Fragen → Datei"**-Tabelle – damit die AI beim Nachschlagen
   in den Docs gezielt findet, was drin ist, ohne alle Files zu lesen.

## Ausführung

Detaillierte Schritt-für-Schritt-Anleitung mit Prosa + abhakbarer
Checkliste steht in:

→ **`~/dev/k-playbook/k-ai-session-memory/PLAYBOOK.md`**

Vorlagen zum Kopieren:

→ **`~/dev/k-playbook/k-ai-session-memory/vorlagen/`**

## Wichtigste Regel

OpenCode lädt Konfig **einmal beim Start**. Nach dem Anlegen/Ändern von
`AGENTS.md` oder `opencode.json` muss OpenCode beendet und **neu
gestartet** werden, sonst greift die neue Session-Memory-Kette nicht.

## Verifikation

Nach Neustart eine Frage stellen, deren Antwort in den Docs steht und
vorher nur durch Code-Recherche zu beantworten war. Die AI sollte:

- den Stichwort-Index aus `docs/README.md` konsultieren,
- direkt in die passende Doc-Datei springen,
- ohne Grep über Repos zu antworten.

Wenn sie stattdessen im Code sucht: eine der drei Konfig-Ebenen ist
nicht wirksam.
