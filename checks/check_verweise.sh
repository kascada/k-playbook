#!/usr/bin/env bash
set -u

# Meldet Verweise ohne Präfix in mitgelieferten und projekteigenen Commands,
# Instructions, Rules, Guidelines und Skills: relative Markdown-Links und nackte
# Pfade, wo ein Verweis gemeint ist. Maßgeblich ist
# k-playbook:/instructions/on-demand/verweise-nachziehen.md; der Check setzt den
# Teil davon um, der am Text erkennbar ist, und korrigiert nichts. Die Ausnahmen
# liegen ausschließlich in verweis-exceptions.tsv neben diesem Skript; das
# Verzeichnis wird deshalb an die Fachlogik übergeben.
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v python3 >/dev/null 2>&1; then
  printf 'K_CHECK_STATUS=skip\n'
  printf 'K_CHECK_REASON=python3 not found\n'
  exit 0
fi

python3 "$script_dir/lib/verweise.py" "$script_dir"
