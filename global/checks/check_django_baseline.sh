#!/usr/bin/env bash
set -u

root="${K_CHECK_PROJECT_ROOT:-$PWD}"
manage_py="$root/manage.py"

if [[ ! -f "$manage_py" ]]; then
  printf 'K_CHECK_STATUS=skip\n'
  printf 'K_CHECK_REASON=manage.py not found at target root\n'
  exit 0
fi

python_bin="python3"
if [[ -x "$root/.venv/bin/python" ]]; then
  python_bin="$root/.venv/bin/python"
elif ! command -v python3 >/dev/null 2>&1; then
  printf 'K_CHECK_STATUS=skip\n'
  printf 'K_CHECK_REASON=python3 not found\n'
  exit 0
fi

if ! "$python_bin" - <<'PY' >/dev/null 2>&1
import django
PY
then
  printf 'K_CHECK_STATUS=skip\n'
  printf 'K_CHECK_REASON=django not importable\n'
  exit 0
fi

: "${DEBUG:=False}"
: "${SECRET_KEY:=k-check-local-placeholder}"
export DEBUG SECRET_KEY

printf '$ %s manage.py check\n' "$python_bin"
if (cd "$root" && "$python_bin" manage.py check); then
  printf 'K_CHECK_STATUS=ok\n'
  exit 0
fi

printf 'K_CHECK_STATUS=fail\n'
printf 'K_CHECK_REASON=django system check failed\n'
exit 1
