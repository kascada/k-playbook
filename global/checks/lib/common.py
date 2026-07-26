from __future__ import annotations

import os
import sys
from pathlib import Path


SKIP_PARTS = {
    ".git",
    ".mypy_cache",
    ".pytest_cache",
    ".ruff_cache",
    ".venv",
    "__pycache__",
    "build",
    "dist",
    "node_modules",
    "staticfiles",
    "venv",
}


def root() -> Path:
    return Path(os.environ.get("K_CHECK_TARGET_ROOT") or os.environ.get("K_CHECK_PROJECT_ROOT", ".")).resolve()


def mode() -> str:
    return os.environ.get("K_CHECK_MODE", "changed")


def listed_files() -> list[Path]:
    raw_path = os.environ.get("K_CHECK_FILES_FROM", "")
    if not raw_path:
        status("skip", "no file list provided")
        raise SystemExit(0)
    path = Path(raw_path)
    if not path.exists():
        status("skip", "file list not found")
        raise SystemExit(0)

    base = root()
    files: list[Path] = []
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        candidate = Path(line.strip())
        if not candidate.is_absolute():
            candidate = base / candidate
        try:
            resolved = candidate.resolve()
            resolved.relative_to(base)
        except (OSError, ValueError):
            continue
        if resolved.is_file() and not any(part in SKIP_PARTS for part in resolved.parts):
            files.append(resolved)
    return files


def status(value: str, reason: str = "") -> None:
    print(f"K_CHECK_STATUS={value}")
    if reason:
        print(f"K_CHECK_REASON={reason}")


def finish_findings(findings: list[str], ok_reason: str = "") -> int:
    if findings:
        for finding in findings:
            print(finding)
        status("fail", f"{len(findings)} finding(s)")
        return 1
    status("ok", ok_reason)
    return 0


def fail_technical(message: str) -> int:
    print(message, file=sys.stderr)
    return 2
