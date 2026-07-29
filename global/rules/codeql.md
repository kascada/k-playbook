# Regel: CodeQL

## Zweck

CodeQL-Konfiguration, lokale CLI-Installation und lokale Analysen muessen getrennt bleiben, damit Setup-Commands keine schweren oder unerwarteten Analysen starten.

## Zustaendigkeiten

- `/k-setup-codeql` besitzt den CodeQL-Entscheidungsblock in `K-PLAYBOOK.MD`.
- `/k-install-codeql` installiert oder prueft lokale CodeQL-Artefakte, ohne `K-PLAYBOOK.MD` zu aendern.
- `/k-status` prueft CodeQL nur lesend.
- `/k-setup` weist nur auf CodeQL hin und schreibt keine CodeQL-Konfiguration.

## GitHub CodeQL

Wenn GitHub CodeQL aktiv oder geplant ist:

- Es darf hoechstens eine lokale CLI-only Installation angeboten werden.
- Erlaubter Script-Aufruf: `scripts/install-codeql-local.sh --parent "<PLAYBOOK_BASE_DIR>" --cli-only`.
- Es duerfen keine lokalen Datenbanken erzeugt werden.
- Es duerfen keine SARIF-Dateien erzeugt werden.
- Es darf keine Analyse gestartet werden.

## Lokale CodeQL-Datenbanken

Lokale Datenbanken sind eine separate Entscheidung.

- Der Datenbankpfad wird nur registriert, wenn `local-database:` `true` oder `planned` ist.
- Datenbanken werden nicht von `/k-setup-codeql` erzeugt.
- Analysen laufen nur nach expliziter User-Freigabe ueber `/k-install-codeql` ohne `--cli-only` oder einen spaeteren Analyse-Command.

## Analyse-Target

- Der CodeQL-Block soll `target:` enthalten, wenn der Projektroot nicht identisch mit dem zu analysierenden Git-/App-Root ist.
- `target:` ist projektrelativ zu `TARGET_DIR`; `.` bedeutet Projektroot, `./app` bedeutet verschachteltes Produkt-Repo.
- `/k-setup-codeql` besitzt dieses Feld. Review-, Status- und Install-Commands muessen es lesen und duerfen bei fehlendem Feld nur legacy-kompatibel `.` annehmen.

## Checks

Ein CodeQL-Check darf als Preflight pruefen:

- ob der CodeQL-Block vorhanden und konsistent ist.
- ob `codeql version` funktioniert.
- ob registrierte Workflow- oder Datenbankpfade existieren.
- ob ein registriertes `target:` existiert und plausibel ist.

Ein CodeQL-Check darf nicht implizit:

- GitHub Actions konfigurieren.
- Datenbanken erzeugen.
- SARIF hochladen.
- langlaufende Analysen starten.
