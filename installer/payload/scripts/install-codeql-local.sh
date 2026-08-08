#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: install-codeql-local.sh --project <dir> --parent <dir> --languages <list> [--queries <suite>] [--force]
       install-codeql-local.sh --parent <dir> --cli-only

Installs the CodeQL CLI locally for the current user if needed, creates CodeQL
databases below the given parent directory, and runs a SARIF-producing query.

Arguments:
  --project <dir>      Project root to analyze.
  --parent <dir>       Parent directory for CodeQL artifacts.
  --languages <list>   Comma-separated CodeQL languages, e.g. python,javascript-typescript.
  --queries <suite>    Query suite or pack. Default: security-extended.
  --force              Recreate existing language databases.
  --cli-only           Only install or locate the CodeQL CLI, then print its version.

Artifacts in full mode:
  <parent>/codeql-cli/        Downloaded CodeQL CLI, if not already on PATH.
  <parent>/databases/<lang>/  CodeQL database per language.
  <parent>/results/<lang>.sarif

Artifacts in --cli-only mode:
  <parent>/codeql-cli/        Downloaded CodeQL CLI, if not already on PATH.
  PATH shim                  Symlink named `codeql` in ~/.opencode/bin or ~/.local/bin when available.
USAGE
}

PROJECT_DIR=""
PARENT_DIR=""
LANGUAGES=""
QUERIES="security-extended"
FORCE=0
CLI_ONLY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project)
      PROJECT_DIR="${2:-}"
      shift 2
      ;;
    --parent)
      PARENT_DIR="${2:-}"
      shift 2
      ;;
    --languages)
      LANGUAGES="${2:-}"
      shift 2
      ;;
    --queries)
      QUERIES="${2:-}"
      shift 2
      ;;
    --force)
      FORCE=1
      shift
      ;;
    --cli-only)
      CLI_ONLY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$PARENT_DIR" ]]; then
  usage >&2
  exit 2
fi

if [[ "$CLI_ONLY" -eq 0 && ( -z "$PROJECT_DIR" || -z "$LANGUAGES" ) ]]; then
  usage >&2
  exit 2
fi

if [[ "$CLI_ONLY" -eq 0 && ! -d "$PROJECT_DIR" ]]; then
  echo "Project directory does not exist: $PROJECT_DIR" >&2
  exit 1
fi

if [[ "$CLI_ONLY" -eq 0 ]]; then
  PROJECT_DIR="$(cd "$PROJECT_DIR" && pwd -P)"
fi
mkdir -p "$PARENT_DIR"
PARENT_DIR="$(cd "$PARENT_DIR" && pwd -P)"

detect_platform() {
  local uname_s uname_m os arch
  uname_s="$(uname -s)"
  uname_m="$(uname -m)"

  case "$uname_s" in
    Linux) os="linux" ;;
    Darwin) os="osx" ;;
    *)
      echo "Unsupported OS for automatic CodeQL install: $uname_s" >&2
      exit 1
      ;;
  esac

  case "$uname_m" in
    x86_64|amd64) arch="64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      echo "Unsupported architecture for automatic CodeQL install: $uname_m" >&2
      exit 1
      ;;
  esac

  printf '%s%s' "$os" "$arch"
}

ensure_codeql() {
  if command -v codeql >/dev/null 2>&1; then
    command -v codeql
    return
  fi

  local platform install_dir zip_path url
  platform="$(detect_platform)"
  install_dir="$PARENT_DIR/codeql-cli"
  zip_path="$PARENT_DIR/codeql-${platform}.zip"
  url="https://github.com/github/codeql-cli-binaries/releases/latest/download/codeql-${platform}.zip"

  if [[ -x "$install_dir/codeql/codeql" ]]; then
    printf '%s\n' "$install_dir/codeql/codeql"
    return
  fi

  mkdir -p "$install_dir"
  echo "Downloading CodeQL CLI: $url" >&2

  if command -v curl >/dev/null 2>&1; then
    curl -L --fail --show-error --output "$zip_path" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$zip_path" "$url"
  else
    echo "Neither curl nor wget is available; cannot download CodeQL CLI." >&2
    exit 1
  fi

  if ! command -v unzip >/dev/null 2>&1; then
    echo "unzip is required to extract the CodeQL CLI." >&2
    exit 1
  fi

  rm -rf "$install_dir/codeql"
  unzip -q "$zip_path" -d "$install_dir"

  if [[ ! -x "$install_dir/codeql/codeql" ]]; then
    echo "Downloaded archive did not contain an executable CodeQL CLI." >&2
    exit 1
  fi

  rm -f "$zip_path"

  printf '%s\n' "$install_dir/codeql/codeql"
}

path_contains_dir() {
  local dir
  dir="$1"
  case ":$PATH:" in
    *":$dir:"*) return 0 ;;
    *) return 1 ;;
  esac
}

ensure_codeql_path_shim() {
  local codeql_bin current candidate shim_dir shim_path
  codeql_bin="$1"

  current="$(command -v codeql || true)"
  if [[ -n "$current" && "$(realpath "$current")" == "$(realpath "$codeql_bin")" ]]; then
    return
  fi

  for candidate in "$HOME/.opencode/bin" "$HOME/.local/bin"; do
    if path_contains_dir "$candidate"; then
      shim_dir="$candidate"
      break
    fi
  done

  if [[ -z "${shim_dir:-}" ]]; then
    echo "CodeQL CLI is installed, but no supported user bin directory is on PATH." >&2
    echo "Add this directory to PATH or symlink manually: $codeql_bin" >&2
    return
  fi

  mkdir -p "$shim_dir"
  shim_path="$shim_dir/codeql"

  if [[ -e "$shim_path" && ! -L "$shim_path" ]]; then
    echo "CodeQL PATH shim already exists and is not a symlink: $shim_path" >&2
    echo "Leaving it unchanged. CLI remains available at: $codeql_bin" >&2
    return
  fi

  ln -sfn "$codeql_bin" "$shim_path"
  echo "CodeQL PATH shim: $shim_path -> $codeql_bin"
}

CODEQL_BIN="$(ensure_codeql)"
ensure_codeql_path_shim "$CODEQL_BIN"

echo "CodeQL CLI: $CODEQL_BIN"
"$CODEQL_BIN" version

if [[ "$CLI_ONLY" -eq 1 ]]; then
  echo
  echo "CodeQL CLI install complete"
  echo "Parent: $PARENT_DIR"
  exit 0
fi

DATABASE_ROOT="$PARENT_DIR/databases"
RESULTS_ROOT="$PARENT_DIR/results"
mkdir -p "$DATABASE_ROOT" "$RESULTS_ROOT"

IFS=',' read -r -a LANGUAGE_ARRAY <<< "$LANGUAGES"

query_arg_for_language() {
  local language suite_language
  language="$1"

  case "$QUERIES" in
    security-extended|security-and-quality)
      case "$language" in
        javascript-typescript) suite_language="javascript" ;;
        java-kotlin) suite_language="java" ;;
        c-cpp) suite_language="cpp" ;;
        *) suite_language="$language" ;;
      esac
      printf 'codeql-suites/%s-%s.qls\n' "$suite_language" "$QUERIES"
      ;;
    *)
      printf '%s\n' "$QUERIES"
      ;;
  esac
}

for raw_language in "${LANGUAGE_ARRAY[@]}"; do
  language="$(printf '%s' "$raw_language" | tr -d '[:space:]')"
  if [[ -z "$language" ]]; then
    continue
  fi

  db_dir="$DATABASE_ROOT/$language"
  sarif_path="$RESULTS_ROOT/$language.sarif"

  if [[ -e "$db_dir" && "$FORCE" -eq 1 ]]; then
    rm -rf "$db_dir"
  fi

  if [[ ! -e "$db_dir" ]]; then
    echo "Creating CodeQL database for $language: $db_dir"
    "$CODEQL_BIN" database create "$db_dir" --language="$language" --source-root="$PROJECT_DIR"
  else
    echo "Using existing CodeQL database for $language: $db_dir"
  fi

  query_arg="$(query_arg_for_language "$language")"
  echo "Analyzing $language database with $query_arg: $sarif_path"
  "$CODEQL_BIN" database analyze "$db_dir" "$query_arg" --format=sarif-latest --output="$sarif_path"
done

echo
echo "CodeQL local setup complete"
echo "Project:   $PROJECT_DIR"
echo "Parent:    $PARENT_DIR"
echo "Databases: $DATABASE_ROOT"
echo "Results:   $RESULTS_ROOT"
