---
name: k-overlay-repo-analyse
description: Use when the user wants to systematically understand a project that uses the Docker-Overlay-Pattern (Base-Image + slim Overlay-Repo that only contains delta files). Extracts or clones the base, diffs against the overlay, quantifies actual changes, writes structured docs that separate "base behavior" from "overlay additions". Trigger keywords - "Overlay", "Base-Image analysieren", "was ist wirklich neu", "FROM ghcr.io", "Custom-Image", "Delta zwischen Repos", "was macht der Overlay wirklich".
---

# Skill: Overlay-Repo-Analyse

**Kurzfassung.** Systematisches Vorgehen, um Projekte mit
Docker-Overlay-Pattern (schlankes Overlay-Repo baut per `FROM base-image` +
`COPY . .` auf ein größeres Base-Image auf) vollständig zu verstehen –
inklusive der Frage „was ist wirklich Custom-Code vs. was kommt aus der
Base".

## Wann anwenden

- Projekt hat ein Repo dessen `Dockerfile` mit `FROM ghcr.io/…` oder
  ähnlichem beginnt und dann nur `COPY . .` macht.
- Overlay-Repo enthält verdächtig wenige Dateien (keine `package.json`
  oder `requirements.txt`, nur ein paar Configs).
- Der Nutzer fragt „was macht dieser Overlay eigentlich neu?", „ist das RAG
  handgeschrieben?", „was ändert sich gegenüber dem Base-Image?" oder
  ähnliches.
- Vor jeder Doku-Erstellung für Overlay-Projekte, damit die Doku nicht
  Overlay-Inhalte als „Eigenentwicklung" fehlinterpretiert.

## Was passiert

Fünf Phasen:

1. **Bestandsaufnahme** – Overlay-Repos oberflächlich erfassen.
2. **Overlay-Analyse** – was steht wirklich im Repo.
3. **Base beschaffen** – per Git-Clone oder Docker-Extract.
4. **Diff Base ↔ Overlay** – quantifizieren welche Zeilen wirklich Custom sind.
5. **Docs schreiben** – klar trennen zwischen Base-Verhalten und Overlay-Delta.

Details, Kommandos, Templates und Checkliste in:

→ **`~/dev/k-playbook/k-overlay-repo-analyse/PLAYBOOK.md`**

## Wichtigste Regel

**Nie ohne Base-Analyse behaupten, dass Overlay-Code neu ist.** Das führt zu
falschen Docs. Erst diffen, dann schreiben. Wenn Base nicht verfügbar ist,
in der Doku explizit vermerken „konnte nicht mit Base verglichen werden".

## Verwandte Playbooks

- `k-ai-session-memory/` – die im letzten Schritt geschriebenen Docs
  dauerhaft für AI-Sessions verankern.
