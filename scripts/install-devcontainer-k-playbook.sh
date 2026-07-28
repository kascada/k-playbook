#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: install-devcontainer-k-playbook.sh <project-root> [--dry-run]

Installs k-playbook DevContainer integration into a target project.

The script writes <project-root>/.devcontainer/setup-k-playbook.sh and updates
<project-root>/.devcontainer/devcontainer.json so the DevContainer:

  - bind-mounts host ~/dev/k-playbook to /workspaces/k-playbook
  - links /home/vscode/dev/k-playbook to /workspaces/k-playbook
  - links k-*.md commands into the container-local OpenCode config
  - configures skills.paths for the container-local OpenCode user config
  - installs missing required security tools during postCreateCommand

Security tools are installed container-/user-locally for the vscode user,
typically under /home/vscode/.local/bin and /home/vscode/.local/share/k-playbook.

Options:
  --dry-run   Show planned changes but do not write files.
  -h, --help  Show this help.

Example:
  ~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh ~/dev/my-project
USAGE
}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

PROJECT_ROOT=""
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -* )
      die "Unknown option: $1"
      ;;
    *)
      if [[ -n "$PROJECT_ROOT" ]]; then
        die "Only one project root is supported."
      fi
      PROJECT_ROOT="$1"
      shift
      ;;
  esac
done

[[ -n "$PROJECT_ROOT" ]] || die "Missing project root."

PROJECT_ROOT="${PROJECT_ROOT/#\~/$HOME}"
[[ -d "$PROJECT_ROOT" ]] || die "Project root does not exist: $PROJECT_ROOT"

DEVCONTAINER_DIR="$PROJECT_ROOT/.devcontainer"
DEVCONTAINER_JSON="$DEVCONTAINER_DIR/devcontainer.json"
SETUP_SCRIPT="$DEVCONTAINER_DIR/setup-k-playbook.sh"

[[ -d "$DEVCONTAINER_DIR" ]] || die "Missing .devcontainer directory: $DEVCONTAINER_DIR"
[[ -f "$DEVCONTAINER_JSON" ]] || die "Missing devcontainer.json: $DEVCONTAINER_JSON"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLAYBOOK_REPO="$(cd "$SCRIPT_DIR/.." && pwd -P)"
SETUP_TEMPLATE="$SCRIPT_DIR/templates/devcontainer-setup-k-playbook.sh"
[[ -d "$PLAYBOOK_REPO/commands" ]] || die "This script must live inside a k-playbook repo with commands/."
[[ -f "$SETUP_TEMPLATE" ]] || die "Missing setup template: $SETUP_TEMPLATE"

write_or_print_setup_script() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: would write $SETUP_SCRIPT"
    return
  fi

  cp "$SETUP_TEMPLATE" "$SETUP_SCRIPT"
  chmod 0755 "$SETUP_SCRIPT"
}

update_devcontainer_json() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    python3 - "$DEVCONTAINER_JSON" --dry-run <<'PY'
import sys
path = sys.argv[1]
print(f"DRY-RUN: would update {path}")
PY
    return
  fi

  python3 - "$DEVCONTAINER_JSON" <<'PY'
import json
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text()

mount_entry = 'source=${localEnv:HOME}/dev/k-playbook,target=/workspaces/k-playbook,type=bind'
setup_cmd = 'sudo bash .devcontainer/setup-k-playbook.sh'
setup_create_cmd = setup_cmd + ' --install-security-tools'


def find_matching(text, start, open_char, close_char):
    depth = 0
    in_string = False
    escape = False
    line_comment = False
    block_comment = False
    for i in range(start, len(text)):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ''
        if line_comment:
            if ch == '\n':
                line_comment = False
            continue
        if block_comment:
            if ch == '*' and nxt == '/':
                block_comment = False
            continue
        if in_string:
            if escape:
                escape = False
            elif ch == '\\':
                escape = True
            elif ch == '"':
                in_string = False
            continue
        if ch == '/' and nxt == '/':
            line_comment = True
            continue
        if ch == '/' and nxt == '*':
            block_comment = True
            continue
        if ch == '"':
            in_string = True
            continue
        if ch == open_char:
            depth += 1
        elif ch == close_char:
            depth -= 1
            if depth == 0:
                return i
    raise SystemExit(f"Could not find matching {close_char} in {path}")


def insert_top_level_property(text, property_text):
    root_start = text.find('{')
    if root_start == -1:
        raise SystemExit(f"{path} does not look like JSONC")
    root_end = find_matching(text, root_start, '{', '}')
    before = text[:root_end].rstrip()
    after = text[root_end:]
    comma = ',' if not before.endswith('{') and not before.endswith(',') else ''
    return f"{before}{comma}\n\t{property_text}\n{after}"


def ensure_mount(text):
    if mount_entry in text:
        return text
    m = re.search(r'"mounts"\s*:\s*\[', text)
    entry = f'\t\t"{mount_entry}"'
    if not m:
        return insert_top_level_property(text, f'"mounts": [\n{entry}\n\t]')
    array_start = text.find('[', m.start())
    array_end = find_matching(text, array_start, '[', ']')
    body = text[array_start + 1:array_end]
    if body.strip():
        new_body = body.rstrip()
        if not new_body.rstrip().endswith(','):
            new_body += ','
        new_body += f'\n{entry}\n\t'
    else:
        new_body = f'\n{entry}\n\t'
    return text[:array_start + 1] + new_body + text[array_end:]


def json_string(value):
    return json.dumps(value, ensure_ascii=True)


def ensure_command_property(text, name, desired_cmd):
    pattern = re.compile(rf'"{re.escape(name)}"\s*:\s*"((?:\\.|[^"\\])*)"')
    m = pattern.search(text)
    if m:
        raw = '"' + m.group(1) + '"'
        try:
            value = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise SystemExit(f"Could not parse {name} string in {path}: {exc}")
        parts = [part.strip() for part in value.split('&&')]
        if desired_cmd not in parts:
            value = value.rstrip() + ' && ' + desired_cmd
        return text[:m.start(1) - 1] + json_string(value) + text[m.end(1) + 1:]
    return insert_top_level_property(text, f'"{name}": {json_string(desired_cmd)}')


updated = text
updated = ensure_mount(updated)
updated = ensure_command_property(updated, 'postCreateCommand', setup_create_cmd)
updated = ensure_command_property(updated, 'postStartCommand', setup_cmd)

if updated != text:
    path.write_text(updated)
PY
}

write_or_print_setup_script
update_devcontainer_json

if [[ "$DRY_RUN" -eq 1 ]]; then
  log "k-playbook DevContainer integration dry-run complete"
else
  log "k-playbook DevContainer integration installed"
fi
log "Project:     $PROJECT_ROOT"
log "Script:      $SETUP_SCRIPT"
log "Config:      $DEVCONTAINER_JSON"
log "Mount:       ~/dev/k-playbook -> /workspaces/k-playbook"
log "Next step:   rebuild or restart the DevContainer, then restart OpenCode inside it."
