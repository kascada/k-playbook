#!/usr/bin/env bash
set -euo pipefail

install_security_tools=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-security-tools)
      install_security_tools=1
      shift
      ;;
    -h|--help)
      cat <<'USAGE'
Usage: setup-k-playbook.sh [--install-security-tools]

Prepares k-playbook inside a DevContainer. With --install-security-tools it
also installs missing required security tools into the vscode user's home.
USAGE
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

playbook_repo="/workspaces/k-playbook"
home_playbook="/home/vscode/dev/k-playbook"
opencode_config_dir="/home/vscode/.config/opencode"
opencode_command_dir="${opencode_config_dir}/command"
opencode_commands_dir="${opencode_config_dir}/commands"
opencode_config_file="${opencode_config_dir}/opencode.jsonc"
security_tools_script="${home_playbook}/scripts/install-security-tools.sh"

ensure_user_tool_path() {
  local user_home shell_file marker path_line
  user_home="$1"
  marker="# k-playbook security tools PATH"
  path_line='export PATH="$HOME/.local/bin:$HOME/.opencode/bin:$PATH"'

  for shell_file in "${user_home}/.profile" "${user_home}/.bashrc" "${user_home}/.zshrc"; do
    touch "${shell_file}"
    if ! grep -qF "${marker}" "${shell_file}"; then
      {
        printf '\n%s\n' "${marker}"
        printf '%s\n' "${path_line}"
      } >>"${shell_file}"
    fi
  done
}

if [[ ! -d "${playbook_repo}/commands" ]]; then
  echo "k-playbook bind mount missing: ${playbook_repo}/commands" >&2
  exit 1
fi

mkdir -p "/home/vscode/dev" "${opencode_command_dir}" "${opencode_commands_dir}"
ln -sfn "${playbook_repo}" "${home_playbook}"

for command_file in "${playbook_repo}"/commands/k-*.md; do
  [[ -e "${command_file}" ]] || continue
  ln -sf "${command_file}" "${opencode_command_dir}/$(basename "${command_file}")"
  ln -sf "${command_file}" "${opencode_commands_dir}/$(basename "${command_file}")"
done

if [[ ! -e "${opencode_config_file}" ]]; then
  cat >"${opencode_config_file}" <<'JSON'
{
  "$schema": "https://opencode.ai/config.json",
  "skills": {
    "paths": ["~/dev/k-playbook"]
  }
}
JSON
elif ! grep -q 'k-playbook' "${opencode_config_file}"; then
  cat >&2 <<'WARN'
k-playbook skills path is not configured in /home/vscode/.config/opencode/opencode.jsonc.
Run /k-install inside the devcontainer or add:
  "skills": { "paths": ["~/dev/k-playbook"] }
WARN
fi

chown -h vscode:vscode "${home_playbook}" || true
chown -R vscode:vscode "${opencode_config_dir}" || true

if [[ "${install_security_tools}" -eq 1 ]]; then
  if [[ ! -f "${security_tools_script}" ]]; then
    echo "k-playbook security tool installer missing: ${security_tools_script}" >&2
    exit 1
  fi

  if [[ "$(id -u)" -eq 0 ]]; then
    mkdir -p /home/vscode/.local/bin /home/vscode/.local/share/k-playbook/security-tools
    ensure_user_tool_path /home/vscode
    chown -R vscode:vscode /home/vscode/.local
    chown vscode:vscode /home/vscode/.profile /home/vscode/.bashrc /home/vscode/.zshrc 2>/dev/null || true
    sudo -H -u vscode env \
      HOME=/home/vscode \
      PATH="/home/vscode/.opencode/bin:/home/vscode/.local/bin:${PATH}" \
      K_SECURITY_TOOLS_PREFIX=/home/vscode/.local \
      K_SECURITY_TOOLS_BIN_DIR=/home/vscode/.local/bin \
      K_SECURITY_TOOLS_VENV=/home/vscode/.local/share/k-playbook/security-tools/pip-audit-venv \
      bash "${security_tools_script}" --install missing --method auto --yes
  else
    mkdir -p "${HOME}/.local/bin" "${HOME}/.local/share/k-playbook/security-tools"
    ensure_user_tool_path "${HOME}"
    env \
      PATH="${HOME}/.opencode/bin:${HOME}/.local/bin:${PATH}" \
      K_SECURITY_TOOLS_PREFIX="${HOME}/.local" \
      K_SECURITY_TOOLS_BIN_DIR="${HOME}/.local/bin" \
      K_SECURITY_TOOLS_VENV="${HOME}/.local/share/k-playbook/security-tools/pip-audit-venv" \
      bash "${security_tools_script}" --install missing --method auto --yes
  fi
fi

echo "k-playbook devcontainer setup complete"
