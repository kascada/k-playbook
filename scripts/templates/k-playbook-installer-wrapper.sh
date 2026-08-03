#!/usr/bin/env bash
set -euo pipefail

source_path="${BASH_SOURCE[0]}"
while [[ -L "${source_path}" ]]; do
  source_dir="$(cd -P "$(dirname "${source_path}")" && pwd)"
  link_target="$(readlink "${source_path}")"
  if [[ "${link_target}" == /* ]]; then
    source_path="${link_target}"
  else
    source_path="${source_dir}/${link_target}"
  fi
done
script_dir="$(cd -P "$(dirname "${source_path}")" && pwd)"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    printf 'Unsupported OS for k-playbook-installer: %s\n' "$(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    printf 'Unsupported architecture for k-playbook-installer: %s\n' "$(uname -m)" >&2
    exit 1
    ;;
esac

target="${script_dir}/k-playbook-installer-${os}-${arch}"
if [[ ! -x "${target}" ]]; then
  printf 'Missing k-playbook-installer binary: %s\n' "${target}" >&2
  printf 'Run make install in ~/dev/k-playbook or update k-playbook from the GUI.\n' >&2
  exit 1
fi

exec "${target}" "$@"
