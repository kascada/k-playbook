#!/usr/bin/env python3
from __future__ import annotations

import re

from common import finish_findings, listed_files, status


SECRET_PATTERNS = [
    re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |)PRIVATE KEY-----"),
    re.compile(r"(?i)\b(?:api[_-]?key|secret[_-]?key|client[_-]?secret|refresh[_-]?token)\b\s*[:=]\s*['\"][^'\"]{12,}['\"]"),
    re.compile(r"(?i)\bbearer\s+[a-z0-9._~+/=-]{20,}"),
    re.compile(r"(?i)\b(?:postgres|redis|amqp|mongodb)://[^\s'\"]+:[^\s'\"]+@"),
    re.compile(r"(?i)\bDefaultEndpointsProtocol=https;AccountName=[^;]+;AccountKey=[^;]+;"),
]

EXTENSIONS = {".cfg", ".env", ".ini", ".json", ".py", ".sh", ".toml", ".yaml", ".yml"}
ALLOWLIST_MARKERS = {"k-check-local-placeholder", "k-playbook-local-check-secret"}


def main() -> int:
    files = [path for path in listed_files() if path.suffix in EXTENSIONS]
    if not files:
        status("skip", "no relevant files")
        return 0

    findings: list[str] = []
    for path in files:
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except UnicodeDecodeError:
            continue
        for line_number, line in enumerate(lines, start=1):
            if any(marker in line for marker in ALLOWLIST_MARKERS):
                continue
            if any(pattern.search(line) for pattern in SECRET_PATTERNS):
                findings.append(f"{path}:{line_number}: possible committed secret")
    return finish_findings(findings)


if __name__ == "__main__":
    raise SystemExit(main())
