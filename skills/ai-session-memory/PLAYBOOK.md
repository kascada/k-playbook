# PLAYBOOK: AI Session Memory

**Ziel:** Vorhandene Projekt-Dokumentation (`k-playbook-local/docs/`) für OpenCode-Sessions
als **autoritative Quelle** verankern, sodass die AI in jeder Session
zuerst dort nachschlägt, statt Code neu zu analysieren.

**Aufwand:** ~15–30 Minuten für ein bestehendes Projekt mit vorhandenen
Docs. Bei neuen Projekten wächst der Aufwand mit den Docs mit.

**Nachprüfbar:** Ja – siehe Checkliste am Ende und Verifikations-Test.

> **Ausführungs-Hinweis:** Die konkrete Umsetzung machen heute vier Commands,
> nacheinander und jeder in seinem eigenen Verzeichnis:
>
> 1. **`/k-code2docs`** — Doku aus dem Code nach `docs/code/`.
> 2. **`/k-tools-scan`** — Library- und Tool-Steckbriefe nach `docs/libs/`.
> 3. **`/k-docs-extract`** — Rohmaterial aus `material/` nach `docs/extracted/`.
> 4. **`/k-docs-index`** — baut `docs/README.md` über alle Herkünfte und
>    schreibt `AGENTS.md` und `opencode.json`.
>
> Nur der letzte Schritt schließt die Session-Memory-Kette; die drei davor
> erzeugen nur Inhalt. Dieses PLAYBOOK beschreibt weiterhin das *Modell* —
> nützlich, wenn jemand konzeptionell verstehen will, was passiert, oder das
> Setup manuell (ohne Command) machen möchte.

---

## Wann anwenden

- **Auslöser 1:** Analyse-Sitzung ergab Docs → soll dauerhaft nutzbar sein.
- **Auslöser 2:** AI-Sessions durchsuchen Code, obwohl die Antwort in
  vorhandenen Docs steht.
- **Auslöser 3:** Neues Projekt aufsetzen, Doku-first-Kultur etablieren.

## Voraussetzungen

- OpenCode installiert und lauffähig.
- Projekt-Verzeichnis mit Doku-Verzeichnis `k-playbook-local/docs/`.
- Docs sollten mindestens einen Einstiegspunkt haben (`k-playbook-local/docs/README.md`).

> **Pfad-Hinweis:** `/k-gui` legt `k-playbook-local/docs/` an oder vervollständigt die Struktur. Keine alternativen Docs-Pfade verwenden.

## Konzept

Drei Bausteine wirken zusammen:

```
Projekt-Root/
├── AGENTS.md                    (1) meta-Instruktion für jede Session
├── opencode.json                (2) sorgt dafür, dass (1) und (3) wirksam sind
└── k-playbook-local/
    ├── docs/
    │   ├── README.md            (3) Stichwort-Index über alle Herkünfte
    │   ├── code/                    erzeugt von /k-code2docs
    │   ├── libs/                    erzeugt von /k-tools-scan
    │   ├── extracted/               erzeugt von /k-docs-extract
    │   └── manual/                  von Hand gepflegt
    └── material/                Rohmaterial, nie indiziert
```

- **`AGENTS.md`** = „Regel": Docs sind autoritativ.
- **`opencode.json`** = Mechanik: `AGENTS.md` wird jede Session injiziert,
  `k-playbook-local/docs/` als Referenz mit Beschreibung registriert.
- **`k-playbook-local/docs/README.md`** = Wegweiser: A–Z-Index + Frage→Datei-Mapping, damit
  die AI innerhalb der Docs gezielt navigiert.

Ohne alle drei Bausteine funktioniert der Mechanismus nicht:

- Fehlt `AGENTS.md` → Session weiß nicht, dass Docs autoritativ sind.
- Fehlt `opencode.json`-Konfig → `AGENTS.md` wird nicht automatisch geladen.
- Fehlt der Index in `k-playbook-local/docs/README.md` → AI kennt die Docs, findet aber die
  Inhalte nicht gezielt und fällt evtl. doch auf Grep zurück.

---

## Ausführung – Schritt für Schritt

### Schritt 1: Docs-Bestand je Herkunft aufnehmen

```bash
cd <projekt-root>
ls k-playbook-local/docs/
ls k-playbook-local/docs/code/ k-playbook-local/docs/libs/ \
   k-playbook-local/docs/extracted/ k-playbook-local/docs/manual/
```

Identifiziere:

- Welche Doc-Dateien existieren, und **in welcher Herkunft**? Die Herkunft ist
  am Verzeichnis ablesbar, und daran hängt, wer die Datei pflegt.
- Ein fehlendes `code/`, `libs/` oder `extracted/` ist der Normalzustand: es
  entsteht erst beim ersten Lauf seines Erzeugers.
- Liegen flache `docs/*.md` direkt im Docs-Verzeichnis? Das sind Dateien aus
  der Zeit vor dieser Struktur; sie haben keinen Erzeuger. `/k-docs-index`
  bietet an, sie nach `docs/code/` zu verschieben.
- Gibt es bereits einen Index in `docs/README.md`?

Wenn keine Docs existieren: dieses Playbook ist noch nicht anwendbar –
zuerst Docs schreiben (siehe `ks-overlay-repo-analyse/` oder eigene
Recherche).

### Schritt 2: `k-playbook-local/docs/README.md` als einzigen Index aufbauen

Es gibt genau einen Index, und er deckt alle Herkünfte ab. Ausgeführt schreibt
ihn `/k-docs-index`; von Hand sieht die Grundstruktur so aus:

```markdown
# <Projektname> – Dokumentation

<Ein-Absatz-Beschreibung des Projekts>

> **Für AI-Sessions:** Diese Docs sind **autoritativ**. Nutze sie zuerst,
> bevor du Code liest. Siehe [`AGENTS.md`](../../AGENTS.md) im Projekt-Root.

## Übersicht der Dokumente

### Code (`code/`) — erzeugt von `/k-code2docs`

| Datei | Inhalt |
|-------|--------|
| ... | ... |

### Extrahiert (`extracted/`) — erzeugt von `/k-docs-extract`

| Datei | Inhalt | Konfidenz |
|-------|--------|-----------|
| ... | ... | ... |

### Handgepflegt (`manual/`)

| Datei | Inhalt |
|-------|--------|
| ... | ... |

## Libs & Stack

Erzeugt von `/k-tools-scan` unter `libs/`.

| Lib | Version | Severity | Letzter Review |
|-----|---------|----------|----------------|
| ... | ... | ... | ... |

## Stichwort-Index

Alphabetisch. Format: Stichwort → Datei + Abschnitt.

### A
- **<Begriff>** → `<herkunft>/<datei>.md` §<abschnitt>
...

## Häufige Fragen → Direkter Sprung

| Frage | Datei |
|-------|-------|
| ... | ... |
```

**Kriterien für einen guten Index:**

- Deckt alle in den Docs behandelten Konzepte/Begriffe ab, über alle Herkünfte
  hinweg — ein Index über nur eine Herkunft ist kein Index
- Jeder Link trägt das Herkunftsverzeichnis im Pfad (`code/00-overview.md`,
  nicht `00-overview.md`)
- Nutzt konkrete Begriffe (auch Fachwörter, Bug-Namen, Env-Variablen)
- Verweist präzise auf Datei UND Abschnitt (nicht nur Datei)
- Wächst mit den Docs mit (Regel: neue Doc-Datei → Index-Einträge nachziehen)
- Ein zweiter Index — etwa eine Übersichtstabelle in `libs/README.md` — ist ein
  Fehler; die Lib-Tabelle steht im Index selbst

Neue Themen- und Tool-Referenzdateien sollten normales Markdown mit leichtgewichtig OKF-kompatiblem YAML-Frontmatter sein. Minimal sinnvoll sind `type`, `title`, `description`, `tags`, `status` und `generated`; `sources` wird nur eingetragen, wenn tatsächlich externe Quellen genutzt wurden.

Vorlage: `vorlagen/docs-README.md.template`

### Schritt 3: `AGENTS.md` im Projekt-Root anlegen

Zweck: Meta-Instruktion, die in jede Session injiziert wird.

Muss enthalten:

- **Was ist dieses Projekt** (1–3 Sätze)
- **Die kritische Regel:** „Docs zuerst konsultieren, bevor Code gelesen wird"
- **Verweis auf `k-playbook-local/docs/README.md`** als Einstieg
- **Wann DOCH in den Code:** klare Ausnahmen (Docs veraltet, konkreter Fix)
- **Sprache** (falls nicht Englisch)
- Optional: Kurzverweise auf die wichtigsten Doc-Dateien

Vorlage: `vorlagen/AGENTS.md.template`

### Schritt 4: `opencode.json` im Projekt-Root anlegen

Muss zwei Dinge tun:

1. `AGENTS.md` als `instructions` einbinden → wird jede Session
   auto-geladen
2. `k-playbook-local/docs/` als `references` registrieren, MIT `description` – nur dann
   wird sie AI-seitig sichtbar

Minimalfassung:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "instructions": ["AGENTS.md"],
  "references": {
    "docs": {
      "path": "./k-playbook-local/docs",
      "description": "Autoritative Projektdokumentation. IMMER zuerst konsultieren (k-playbook-local/docs/README.md als Index), bevor Code gelesen wird. Enthält <hier Themen listen>."
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

- AI konsultiert `k-playbook-local/docs/README.md`
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

- ✗ `k-playbook-local/docs/README.md` hat keinen Stichwort-Index → Schritt 2 nachholen
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
- [ ] `AGENTS.md` verweist auf `k-playbook-local/docs/README.md` als Einstieg
- [ ] `AGENTS.md` beschreibt Ausnahmen (wann doch in Code)
- [ ] Projekt-Root enthält `opencode.json` (oder `.jsonc`)
- [ ] `opencode.json` hat `"$schema": "https://opencode.ai/config.json"`
- [ ] `opencode.json` hat `"instructions": ["AGENTS.md"]`
- [ ] `opencode.json` hat `"references"` mit Key `"docs"`
- [ ] Die Referenz hat einen konkreten `"description"`-Text
- [ ] `k-playbook-local/docs/README.md` existiert
- [ ] `k-playbook-local/docs/README.md` hat eine Datei-Übersicht (TOC)
- [ ] `k-playbook-local/docs/README.md` hat einen alphabetischen Stichwort-Index
- [ ] `k-playbook-local/docs/README.md` hat eine „Häufige Fragen → Datei"-Tabelle
- [ ] `k-playbook-local/docs/README.md` verweist zurück auf `AGENTS.md`
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

- `ks-overlay-repo-analyse/` – erzeugt Docs für Base+Overlay-Projekte,
  auf denen dieses Playbook aufsetzt.
- `/k-code2docs`, `/k-tools-scan`, `/k-docs-extract` (Commands) – erzeugen die
  Doku je Herkunft unter `docs/code/`, `docs/libs/` und `docs/extracted/`.
- `/k-docs-index` (Command) – letzter Schritt der Kette: baut `docs/README.md`
  über alle Herkünfte und schreibt `AGENTS.md` und `opencode.json`. Erst
  danach greift die hier beschriebene Session-Memory.
- `/k-gui` – legt `K-PLAYBOOK.yaml` und die feste Struktur an oder vervollständigt sie.
