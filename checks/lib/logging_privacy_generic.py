#!/usr/bin/env python3
from __future__ import annotations

import re

from common import finish_findings, listed_files, status


RISK_PATTERNS = [
    (
        re.compile(r"logger\.(?:warning|error|exception|critical)\s*\([^\n]*(?:response\.text|response\.content|resp\.text|resp\.content)"),
        "raw upstream response in log",
    ),
    (re.compile(r"logger\.(?:warning|error|exception|critical)\s*\(\s*f[\"']"), "f-string log call; verify no user content or secrets"),
    (re.compile(r"logger\.(?:error|exception|critical)\s*\([^\n]*exc_info\s*=\s*True"), "exc_info=True; verify traceback cannot expose sensitive data"),
    (re.compile(r"logger\.(?:info|warning|error|exception|critical)\s*\([^\n]*(?:request\.body|request\.data|request\.POST)"), "request payload in log"),
]


def main() -> int:
    files = [path for path in listed_files() if path.suffix == ".py"]
    if not files:
        status("skip", "no python files")
        return 0

    findings: list[str] = []
    for path in files:
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except UnicodeDecodeError:
            continue
        for line_number, line in enumerate(lines, start=1):
            for pattern, message in RISK_PATTERNS:
                if pattern.search(line):
                    findings.append(f"{path}:{line_number}: {message}")
    return finish_findings(findings)


if __name__ == "__main__":
    raise SystemExit(main())
