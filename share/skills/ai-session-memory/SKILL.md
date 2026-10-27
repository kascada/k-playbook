---
name: ks-ai-session-memory
description: Vorübergehend nicht verfügbar, während die Projektdoku nach keph:/docs/ umzieht. Nur verwenden, wenn der Nutzer diesen Skill ausdrücklich beim Namen nennt; dann meldet er, dass er nicht verfügbar ist.
---

# Skill: AI Session Memory

## Vorübergehend nicht verfügbar

Dieser Skill wird auf die Projektdoku in Kephalaion (`keph:/docs/`) umgebaut und steht bis
dahin nicht zur Verfügung. Führe nichts von dem aus, was darunter steht — nichts lesen,
nichts schreiben, nichts anbieten. Insbesondere schreibst du weder `AGENTS.md` noch `opencode.json`: Den Einstieg in die Projektdoku liefert seit dem Umbau das Briefing.
Sag dem Nutzer in einem Satz, dass `ks-ai-session-memory`
vorübergehend nicht verfügbar ist, und nenne, was es schon gibt:

- das Versionsinventar: `/k-doc-inventory`, Ergebnis unter
  `keph:/docs/generated/versions/inventory.md`;
- die Projektdoku: `keph:/docs/`, Einstieg im Briefing;
- den Umzug alter Doku von der Platte: `/k-upgrade`, Umstellung 3.2.

Der Text darunter bleibt als Vorlage für den Umbau stehen.

## Erster Schritt

Wende `.claude/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Skills stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


**Kurzfassung.** Sorgt dafür, dass in einem Projekt vorhandene Dokumentation
in jeder OpenCode-Session **automatisch als autoritative Quelle** genutzt
wird, statt dass der AI-Assistent jedes Mal wieder von vorn Code analysiert.

> **Ausführung:** Die Docs gliedern sich in fünf Herkünfte, und die operative
> Umsetzung machen die Werkzeuge der jeweiligen Herkunft: **`/k-docs-code`**
> schreibt nach `docs/code/`, **`/k-docs-tools`** nach `docs/libs/`,
> **`/k-docs-extract`** nach `docs/extracted/`, **`/k-doc-inventory`** nach
> `docs/versions/`, `docs/manual/` ist
> handgepflegt. Nach `docs/code/` schreibt außerdem der Skill
> `ks-overlay-repo-analyse` — welches Werkzeug eine einzelne Datei geschrieben
> hat, steht in ihrem Frontmatter unter `generated.by`. Den Index
> `docs/README.md` und die Registrierung
> (`AGENTS.md`, `opencode.json`) macht allein **`/k-docs-index`** — er ist der
> letzte Schritt und der einzige, der die Session-Memory-Kette schließt.
> Dieser Skill liefert das *Konzept* und die zugrundeliegenden Vorlagen.

## Wann anwenden

Immer wenn:

1. Recherche/Analyse-Arbeit gemacht wurde und in `k-playbook-local/docs/` abgelegt ist.
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
   `instructions` ein und registriert `k-playbook-local/docs/` als beschriebene
   `references`.
3. **`k-playbook-local/docs/README.md`** mit **Stichwort-Index** (A–Z) und
   **„Häufige Fragen → Datei"**-Tabelle – damit die AI beim Nachschlagen
   in den Docs gezielt findet, was drin ist, ohne alle Files zu lesen. Es ist
   der einzige Index, und er deckt alle Herkünfte ab: `code/`, `libs/`,
   `extracted/`, `versions/` und `manual/`.

## Ausführung

Detaillierte Schritt-für-Schritt-Anleitung mit Prosa + abhakbarer
Checkliste steht in:

→ **`.claude/skills/ai-session-memory/PLAYBOOK.md`**

Vorlagen zum Kopieren:

→ **`.claude/skills/ai-session-memory/vorlagen/`**

## Wichtigste Regel

OpenCode lädt Konfig **einmal beim Start**. Nach dem Anlegen/Ändern von
`AGENTS.md` oder `opencode.json` muss OpenCode beendet und **neu
gestartet** werden, sonst greift die neue Session-Memory-Kette nicht.

## Verifikation

Nach Neustart eine Frage stellen, deren Antwort in den Docs steht und
vorher nur durch Code-Recherche zu beantworten war. Die AI sollte:

- den Stichwort-Index aus `k-playbook-local/docs/README.md` konsultieren,
- direkt in die passende Doc-Datei springen,
- ohne Grep über Repos zu antworten.

Wenn sie stattdessen im Code sucht: eine der drei Konfig-Ebenen ist
nicht wirksam.
