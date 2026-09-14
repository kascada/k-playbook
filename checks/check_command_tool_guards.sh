#!/usr/bin/env bash
set -u

# Prüft, ob Commands externe Werkzeuge aus scripts/base-tools.tsv nur mit
# lokalem Guard oder Rückfall aufrufen. Die Ausnahmen liegen ausschließlich in
# command-tool-guard-exceptions.tsv neben diesem Skript; das Verzeichnis wird
# deshalb an die Fachlogik übergeben.
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v python3 >/dev/null 2>&1; then
  printf 'K_CHECK_STATUS=skip\n'
  printf 'K_CHECK_REASON=python3 not found\n'
  exit 0
fi

python3 "$script_dir/lib/command_tool_guards.py" "$script_dir"
