#!/usr/bin/env python3
from __future__ import annotations

import re

from common import finish_findings, listed_files, status


LOOKUP_PATTERN = re.compile(r"\.objects\.(?:get|filter)\s*\([^\n]*(?:pk|id|uuid|file)[_a-z]*\s*=")
SAFE_CONTEXT_PATTERN = re.compile(
    r"\b(?:request\.user|user\s*=|owner\s*=|created_by\s*=|tenant\s*=|organization\s*=|workspace\s*=|account\s*=|permission|authorize|allowed|scope)\b"
)
INTERESTING_PATH_PARTS = {"api", "file_proxy", "services", "tasks", "views"}


def has_safe_context(lines: list[str], index: int) -> bool:
    start = max(0, index - 4)
    end = min(len(lines), index + 5)
    return any(SAFE_CONTEXT_PATTERN.search(lines[pos]) for pos in range(start, end))


def main() -> int:
    files = [
        path
        for path in listed_files()
        if path.suffix == ".py" and any(part in INTERESTING_PATH_PARTS for part in path.parts)
    ]
    if not files:
        status("skip", "no relevant python files")
        return 0

    findings: list[str] = []
    for path in files:
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except UnicodeDecodeError:
            continue
        for index, line in enumerate(lines):
            if LOOKUP_PATTERN.search(line) and not has_safe_context(lines, index):
                findings.append(f"{path}:{index + 1}: direct object lookup without nearby user or permission scoping")
    return finish_findings(findings)


if __name__ == "__main__":
    raise SystemExit(main())
