# PLAYBOOK: AI Session Memory

**Ziel:** Vorhandene Projekt-Dokumentation (`docs/`) für OpenCode-Sessions
als **autoritative Quelle** verankern, sodass die AI in jeder Session
zuerst dort nachschlägt, statt Code neu zu analysieren.

**Aufwand:** ~15–30 Minuten für ein bestehendes Projekt mit vorhandenen
Docs. Bei neuen Projekten wächst der Aufwand mit den Docs mit.

**Nachprüfbar:** Ja – siehe Checkliste am Ende und Verifikations-Test.

---

## Wann anwenden

- **Auslöser 1:** Analyse-Sitzung ergab Docs → soll dauerhaft nutzbar sein.
- **Auslöser 2:** AI-Sessions durchsuchen Code, obwohl die Antwort in
  vorhandenen Docs steht.
- **Auslöser 3:** Neues Projekt aufsetzen, Doku-first-Kultur etablieren.

## Voraussetzungen

- OpenCode installiert und lauffähig.
- Projekt-Verzeichnis mit Doku-Verzeichnis (Standardname `docs/`).
- Docs sollten mindestens einen Einstiegspunkt haben (`docs/README.md`
  oder `docs/index.md`).

## Konzept

Drei Bausteine wirken zusammen:

```
Projekt-Root/
├── AGENTS.md            (1) meta-Instruktion für jede Session
├── opencode.json        (2) sorgt dafür, dass (1) und (3) wirksam sind
└── docs/
    └── README.md        (3) Stichwort-Index in den Docs selbst
```

- **`AGENTS.md`** = „Regel": Docs sind autoritativ.
- **`opencode.json`** = Mechanik: `AGENTS.md` wird jede Session injiziert,
  `docs/` als Referenz mit Beschreibung registriert.
- **`docs/README.md`** = Wegweiser: A–Z-Index + Frage→Datei-Mapping, damit
  die AI innerhalb der Docs gezielt navigiert.

Ohne alle drei Bausteine funktioniert der Mechanismus nicht:

- Fehlt `AGENTS.md` → Session weiß nicht, dass Docs autoritativ sind.
- Fehlt `opencode.json`-Konfig → `AGENTS.md` wird nicht automatisch geladen.
- Fehlt der Index in `docs/README.md` → AI kennt die Docs, findet aber die
  Inhalte nicht gezielt und fällt evtl. doch auf Grep zurück.

---

## Ausführung – Schritt für Schritt

### Schritt 1: Docs-Bestand aufnehmen

```bash
cd <projekt-root>
ls docs/
```

Identifiziere:

- Welche Doc-Dateien existieren?
- Ist es eine flache Struktur oder gibt es Präfixe/Nummern?
- Gibt es bereits einen Index/TOC (in `README.md` oder `INDEX.md`)?

Wenn keine Docs existieren: dieses Playbook ist noch nicht anwendbar –
zuerst Docs schreiben (siehe `k-overlay-repo-analyse/` oder eigene
Recherche).

### Schritt 2: `docs/README.md` mit Stichwort-Index erweitern

Grundstruktur:

```markdown
# <Projektname> – Dokumentation

<Ein-Absatz-Beschreibung des Projekts>

> **Für AI-Sessions:** Diese Docs sind **autoritativ**. Nutze sie zuerst,
> bevor du Code liest. Siehe [`AGENTS.md`](../AGENTS.md) im Workspace-Root.

## Übersicht der Dokumente

| Datei | Inhalt |
|-------|--------|
| ... | ... |

## Stichwort-Index

Alphabetisch. Format: Stichwort → Datei-Nr. + Abschnitt.

### A
- **<Begriff>** → `<datei>` §<abschnitt>
...

## Häufige Fragen → Direkter Sprung

| Frage | Datei |
|-------|-------|
| ... | ... |
```

**Kriterien für einen guten Index:**

- Deckt alle in den Docs behandelten Konzepte/Begriffe ab
- Nutzt konkrete Begriffe (auch Fachwörter, Bug-Namen, Env-Variablen)
- Verweist präzise auf Datei UND Abschnitt (nicht nur Datei)
- Wächst mit den Docs mit (Regel: neue Doc-Datei → Index-Einträge nachziehen)

Vorlage: `vorlagen/docs-README-header.md.template`

### Schritt 3: `AGENTS.md` im Projekt-Root anlegen

Zweck: Meta-Instruktion, die in jede Session injiziert wird.

Muss enthalten:

- **Was ist dieses Projekt** (1–3 Sätze)
- **Die kritische Regel:** „Docs zuerst konsultieren, bevor Code gelesen wird"
- **Verweis auf `docs/README.md`** als Einstieg
- **Wann DOCH in den Code:** klare Ausnahmen (Docs veraltet, konkreter Fix)
- **Sprache** (falls nicht Englisch)
- Optional: Kurzverweise auf die wichtigsten Doc-Dateien

Vorlage: `vorlagen/AGENTS.md.template`

### Schritt 4: `opencode.json` im Projekt-Root anlegen

Muss zwei Dinge tun:

1. `AGENTS.md` als `instructions` einbinden → wird jede Session
   auto-geladen
2. `docs/` als `references` registrieren, MIT `description` – nur dann
   wird sie AI-seitig sichtbar

Minimalfassung:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "instructions": ["AGENTS.md"],
  "references": {
    "docs": {
      "path": "./docs",
      "description": "Autoritative Projektdokumentation. IMMER zuerst konsultieren (docs/README.md als Index), bevor Code gelesen wird. Enthält <hier Themen listen>."
    }
  }
}
```

Wichtig: `description` möglichst konkret formulieren – der Text ist das,
womit die AI entscheidet, wann die Referenz relevant ist.

Vorlage: `vorlagen/opencode.json.template`

### Schritt 5: OpenCode neu starten

**Nicht überspringen.** OpenCode liest Konfig einmal beim Start. Ohne
Neustart passiert nichts.

```bash
# OpenCode beenden (Ctrl+C oder /exit)
# Dann neu starten:
opencode
```

Oder Tab/Terminal schließen und neu öffnen.

### Schritt 6: Verifikation

**Test 1 – Kontext-Check.** Neue Session starten und die AI fragen:

> „Welche Regeln gelten für diese Session laut AGENTS.md?"

Die AI sollte die Regel „Docs zuerst" zitieren können. Wenn nicht: Konfig
ist nicht aktiv.

**Test 2 – Effekt-Check.** Eine Frage stellen, deren Antwort in den Docs
steht und die vorher nur durch Code-Grep zu beantworten war, z.B.:

> „Was macht [spezifisches Feature/Bug/Begriff]?"

Erwartetes Verhalten:

- AI konsultiert `docs/README.md`
- Springt in die im Index verwiesene Datei
- Antwortet ohne Grep über Repos

Wenn stattdessen Code-Suche startet: Setup ist unvollständig – siehe
Troubleshooting.

---

## Troubleshooting

### AI ignoriert die Docs

- ✗ OpenCode wurde nicht neu gestartet → beenden und neu starten
- ✗ `opencode.json` liegt am falschen Ort → muss im Projekt-Root sein
- ✗ `AGENTS.md`-Pfad in `instructions` falsch → prüfen ob Datei existiert
- ✗ `AGENTS.md` selbst enthält keine klare Regel → präziser formulieren

### AI kennt die Docs, findet Inhalt aber nicht

- ✗ `docs/README.md` hat keinen Stichwort-Index → Schritt 2 nachholen
- ✗ Index deckt den gefragten Begriff nicht ab → Index erweitern
- ✗ `references.docs.description` ist zu generisch → konkretere Themen
  auflisten

### AI sagt „ich weiß nicht" trotz Docs

- ✗ Doc-Dateien selbst sind lückenhaft → Docs verbessern (nicht das
  Memory-Setup)
- ✗ Frage betrifft Bereich, den Docs nicht abdecken → Docs erweitern und
  Index nachziehen

---

## Checkliste zur Verifikation

Zum Abhaken – ideal für Reviews oder späteres Nachprüfen ob das Setup
korrekt angewendet wurde.

- [ ] Projekt-Root enthält `AGENTS.md`
- [ ] `AGENTS.md` beschreibt Projekt kurz (was ist es)
- [ ] `AGENTS.md` enthält explizit die Regel „Docs zuerst konsultieren"
- [ ] `AGENTS.md` verweist auf `docs/README.md` als Einstieg
- [ ] `AGENTS.md` beschreibt Ausnahmen (wann doch in Code)
- [ ] Projekt-Root enthält `opencode.json` (oder `.jsonc`)
- [ ] `opencode.json` hat `"$schema": "https://opencode.ai/config.json"`
- [ ] `opencode.json` hat `"instructions": ["AGENTS.md"]`
- [ ] `opencode.json` hat `"references"` mit Key `"docs"`
- [ ] Die Referenz hat einen konkreten `"description"`-Text
- [ ] `docs/README.md` existiert
- [ ] `docs/README.md` hat eine Datei-Übersicht (TOC)
- [ ] `docs/README.md` hat einen alphabetischen Stichwort-Index
- [ ] `docs/README.md` hat eine „Häufige Fragen → Datei"-Tabelle
- [ ] `docs/README.md` verweist zurück auf `AGENTS.md`
- [ ] OpenCode wurde nach dem Einrichten neu gestartet
- [ ] Verifikations-Test 1 (Kontext-Check) bestanden
- [ ] Verifikations-Test 2 (Effekt-Check) bestanden

---

## Wartung im laufenden Betrieb

Der Wert dieses Setups hängt daran, dass Docs und Index gepflegt werden.

**Regeln:**

- Wenn eine neue Doc-Datei entsteht → in Datei-Übersicht UND Stichwort-Index
  eintragen.
- Wenn im Code etwas gefunden wird, das der Doku widerspricht → Doku
  updaten (nicht nur die Antwort geben).
- Wenn ein neues Konzept/Begriff im Projekt auftaucht → neuer Stichwort-
  Eintrag.
- Bei größeren Änderungen: kurz die Checkliste durchgehen.

## Verwandte Playbooks

- `k-overlay-repo-analyse/` – erzeugt Docs für Base+Overlay-Projekte,
  auf denen dieses Playbook aufsetzt.

## Referenz-Implementierung

Ein vollständig durchgeführtes Beispiel dieses Playbooks:

- `/home/kleist/dev/clara/` (CLARA-Projekt)
  - `AGENTS.md` im Root
  - `opencode.json` im Root
  - `docs/README.md` mit ~120 Stichwort-Einträgen und Frage→Datei-Tabelle
