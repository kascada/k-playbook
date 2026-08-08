# k-playbook

`k-playbook` ist ein Werkzeugkasten aus Slash-Commands, Skills, Review-Rezepten, Regeln und Checks. Er wird in ein Unterverzeichnis des Zielprojekts installiert, konventionell `<projekt>/k-playbook/`. Dort liegt die mitgelieferte Installation unter `_dist/`, daneben die projekteigenen Artefakte: `K-PLAYBOOK.yaml`, Tasks, Reviews, Ergebnisse und Docs. Ein Projekt ist damit selbstgenuegsam.

> **Umstellung laeuft.** Das Zielmodell ist die projektlokale Installation; der Kontrakt dafuer steht in [`docs/k-playbook-format.md`](./docs/k-playbook-format.md) (`schema_version: 2`).
> Der Installer setzt derzeit noch das alte Modell mit zentraler Basisinstallation unter `~/dev/k-playbook` um. Bis er umgebaut ist, gelten die Installationsanweisungen unten unveraendert.
> Bestehende Projekte mit `schema_version: 1` werden spaeter per `k-playbook-installer migrate` umgestellt.

## Installation (aktueller Stand)

Repo an den Standardpfad klonen:

```bash
git clone git@github.com:kascada/k-playbook.git ~/dev/k-playbook
```

Oder mit GitHub CLI:

```bash
gh repo clone kascada/k-playbook ~/dev/k-playbook
```

Der Pfad `~/dev/k-playbook` ist bewusst fest. Projektkonfigurationen, Command-Symlinks und Skills verweisen auf diesen logischen Pfad. Wenn das Repo physisch anders liegt, soll nicht jedes Projekt angepasst werden; der Installer kann den Pfadvertrag per Symlink reparieren.

Danach installieren und die GUI starten:

```bash
cd ~/dev/k-playbook
make install
~/dev/k-playbook/bin/k-playbook-installer
```

`make install` installiert die plattformspezifischen Installer-Binaries nach `bin/`, legt den Wrapper `bin/k-playbook-installer` an, verlinkt `~/.local/bin/k-playbook-installer` als Komfort-Symlink auf diesen Wrapper und ergaenzt das Shell-Profil um `~/dev/k-playbook/bin`. Quelle sind vorhandene Release-Artefakte unter `dist/` oder die GitHub Releases. Dafuer ist kein lokal installiertes Go noetig.

Nach neu geladener Shell funktioniert der globale Aufruf `k-playbook-installer`. Vorher kann die GUI direkt gestartet werden:

```bash
~/dev/k-playbook/bin/k-playbook-installer
```

Die GUI prueft den Pfadvertrag, registriert OpenCode-/Claude-Commands und Skills, verwaltet Zielprojekte, erzeugt projektlokale `K-PLAYBOOK.yaml`-Dateien und kann Security-Tools per Preflight pruefen.

## Projekt-Onboarding

Neue oder bestehende Zielprojekte werden ueber die GUI eingebunden. Am schnellsten und sichersten startest du sie direkt in der Shell:

```bash
k-playbook-installer
```

Falls der Installer noch nicht im `PATH` liegt, nutze den direkten Pfad:

```bash
~/.local/bin/k-playbook-installer
```

Alternativ kann die GUI im Assistant per Slash-Command gestartet werden:

```text
/k-gui
```

Der direkte Shell-Aufruf ist zu bevorzugen, weil er schneller und robuster ist. Wenn `/k-gui` nach einer frischen Installation noch nicht im Assistant sichtbar ist, starte zuerst `k-playbook-installer` direkt. Nach der Registrierung OpenCode oder Claude neu starten.

Die GUI legt im Zielprojekt die konfigurierten lokalen Strukturen an, konventionell unter `k-playbook/`, und schreibt `K-PLAYBOOK.yaml`. Spaetere Commands lesen daraus alle projektlokalen Pfade.

## DevContainer

DevContainer verwenden denselben logischen Pfad. Das Host-Repo wird nach `/workspaces/k-playbook` gemountet; im Container zeigt `~/dev/k-playbook` per Symlink darauf. Dadurch muessen Host und Container nicht getrennt aktualisiert werden.

Die Integration richtet die GUI pro Zielprojekt ein. Details stehen in [`docs/installation.md`](./docs/installation.md#devcontainer-pfadvertrag).

## Dokumentation

- [`docs/README.md`](./docs/README.md) - kompletter Dokumentationsindex.
- [`docs/installation.md`](./docs/installation.md) - Installation, Pfadvertrag, DevContainer und Installer-Binary.
- [`docs/commands.md`](./docs/commands.md) - kompakter Command-Index.
- [`docs/pr-review.md`](./docs/pr-review.md) - PR-Review-Flow.
- [`docs/code-review.md`](./docs/code-review.md) - Review-Rezepte, Results-Summary, Remediation und Handoffs.
- [`docs/task-flow.md`](./docs/task-flow.md) - Task-Erzeugung, Review-Loop und Ausfuehrung.
- [`docs/reviews-and-results.md`](./docs/reviews-and-results.md) - Artefaktmodell fuer Reviews, Findings und Remediation.

## Bausteine

| Bereich | Zweck | Nutzung |
|---|---|---|
| `commands/` | Slash-Commands | `/k-<name>` |
| `ks-<name>/` | Skills und Playbooks | automatisch durch OpenCode oder manuell |
| `global/rules/` | projektuebergreifende Enforcement-Regeln | Skill `ks-enforcement` oder `/k-enforcement` |
| `global/reviews/` | wiederverwendbare Review-Rezepte | `/k-review <name>` |
| `global/checks/` | schnelle generische Checks | `global/bin/k-check` |
| `prompts/` | kopierbare AI-Assistenten-Auftraege | gefuehrte Setup- oder Analyseablaeufe |

## Grundprinzipien

- Jedes Projekt traegt seine eigene Installation in einem Unterverzeichnis. Kein fester Hostpfad, kein globaler Symlink.
- Installation und Projekt-Eigentum sind strikt getrennt: `_dist/` wird bei jedem Update vollstaendig ersetzt, alles daneben nie angefasst.
- Mitgelieferte Regeln, Reviews und Checks werden nicht editiert. Ein Projekt weicht per Overlay ab: gleichnamige lokale Datei ersetzt, `overlay.<kind>.disabled` schaltet ab.
- Pfade werden aus `K-PLAYBOOK.yaml` gelesen und nicht geraten.
- Docs, Tasks, Reviews und Results bleiben projektlokale Artefakte.
- Security-Tools werden host- oder user-lokal installiert, nie in Projekt-venvs. Sie sind die eine bewusste Ausnahme von der Projektlokalitaet.
