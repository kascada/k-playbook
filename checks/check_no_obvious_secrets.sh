#!/usr/bin/env bash
set -u

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v python3 >/dev/null 2>&1; then
  printf 'K_CHECK_STATUS=skip\n'
  printf 'K_CHECK_REASON=python3 not found\n'
  exit 0
fi

python3 "$script_dir/lib/no_obvious_secrets.py"
