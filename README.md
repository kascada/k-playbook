# k-playbook

`k-playbook` ist eine kuratierte Sammlung wiederverwendbarer AI-Assistant-Bausteine: Slash-Commands, Skills/Playbooks, globale Regeln, Review-Rezepte und Checks. Das Repo bildet den globalen Werkzeugkasten; projektlokale Pfade, Regeln, Tasks, Reviews und Docs werden im jeweiligen Projekt ueber `K-PLAYBOOK.MD` registriert.

## Schnellstart

Pfadvertrag: `k-playbook` muss in jeder Umgebung unter `~/dev/k-playbook` erreichbar sein. Wenn der echte Klon woanders liegt, lege einen Symlink nach `~/dev/k-playbook` an.

```bash
gh repo clone kascada/k-playbook ~/dev/k-playbook
```

Ohne GitHub CLI:

```bash
git clone https://github.com/kascada/k-playbook.git ~/dev/k-playbook
```

Danach in OpenCode:

```text
/k-install
/k-install-security-tools --install missing
```

Wichtig: `/k-install*` nicht aus einem aktiven Projekt-venv starten. Falls `VIRTUAL_ENV` gesetzt ist, zuerst `deactivate` ausfuehren.

In jedem Zielprojekt einmal:

```text
/k-setup
```

Merksatz: `/k-install*` ist host-global; `/k-setup*` ist projektlokal.

## Dokumentation

Der Einstieg ist das Handbuch:

- [`docs/handbuch.md`](./docs/handbuch.md) - Zweck, Konzepte, Standardablaeufe und Betriebsregeln.
- [`docs/installation.md`](./docs/installation.md) - Installation fuer OpenCode, Security-Tools und optional Claude Code.
- [`docs/faq.md`](./docs/faq.md) - Kurze Antworten zu `/k-install`, Aufrufort und Projekt-venvs.
- [`docs/commands.md`](./docs/commands.md) - Zuständigkeiten und Details der wichtigsten Commands.
- [`docs/reviews-and-results.md`](./docs/reviews-and-results.md) - Review-, Results- und Remediation-Flow.
- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.

## Bausteine

| Bereich | Zweck | Nutzung |
|---|---|---|
| `commands/` | Manuelle Slash-Commands | `/k-<name>` |
| `ks-<name>/` | Skills/Playbooks mit Anleitung, Checkliste und Templates | automatisch durch OpenCode oder manuell |
| `global/rules/` | Projektuebergreifende Enforcement-Regeln | Skill `ks-enforcement` oder `/k-enforcement` |
| `global/reviews/` | Wiederverwendbare Review-Rezepte | `/k-review <name>` |
| `global/checks/` | Schnelle generische Checks | `global/bin/k-check` |

## Symlink-Struktur

`/k-install` registriert dieses Repo host-lokal fuer OpenCode. Die Command-Dateien bleiben im Repo; OpenCode bekommt Symlinks:

```text
~/.config/opencode/command/
├── k-install.md          -> ~/dev/k-playbook/commands/k-install.md
├── k-status.md           -> ~/dev/k-playbook/commands/k-status.md
├── k-setup.md            -> ~/dev/k-playbook/commands/k-setup.md
└── k-*.md                -> ~/dev/k-playbook/commands/k-*.md
```

Skills werden nicht einzeln verlinkt. Stattdessen muss das Repo in der OpenCode-Konfiguration stehen:

```jsonc
{
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
```

Security-Tools liegen separat host-/user-lokal, typischerweise unter `~/.opencode/bin` oder `~/.local/bin`. Python-CLI-Tools wie `pip-audit` nutzen `pipx` oder ein dediziertes k-playbook Tool-venv unter `~/.local/share/k-playbook/`, nie ein Projekt-venv.

`/k-status` prueft read-only, ob die OpenCode-Symlinks auf die erwarteten Dateien in diesem Repo zeigen und ob `skills.paths` plausibel gesetzt ist. Repariert wird mit `/k-install`.

## Grundprinzipien

- Schlank und nachvollziehbar: nur Bausteine, die im Alltag genutzt werden.
- Globales Repo, lokale Projektkonfiguration: keine Pfade raten, sondern `K-PLAYBOOK.MD` lesen.
- Docs zuerst: Projektwissen soll in `docs/` liegen und per `AGENTS.md`/OpenCode-Konfig in Sessions wirken.
- Review-Ergebnisse auditierbar speichern: `assessment.md`, `findings.md`, `raw/` und Metadaten.
- Security-Tools host-lokal installieren, nie in Projekt-venvs; Review-Artefakte projektlokal ablegen.

## Repository-Struktur

```text
~/dev/k-playbook/
├── README.md
├── docs/                         Handbuch und Detaildokumentation
├── commands/                     Slash-Commands
├── global/                       globale Regeln, Reviews, Checks
├── ks-ai-session-memory/         Playbook: Docs fuer AI-Sessions verankern
├── ks-enforcement/               Playbook: globale + lokale Regeln anwenden
├── ks-overlay-repo-analyse/      Playbook: Docker-Overlay-Projekte analysieren
├── scripts/                      host-lokale Hilfsskripte
└── Makefile
```

## Versionierung

Remote:

```text
git@github.com:kascada/k-playbook.git
```

Nach Änderungen:

```bash
cd ~/dev/k-playbook
make push
```
