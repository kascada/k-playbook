#!/usr/bin/env python3
from __future__ import annotations

import ast
from pathlib import Path

from common import finish_findings, listed_files, mode, root, status


def migration_name(path: Path) -> str:
    return path.stem


def dependency_names(path: Path, app_label: str) -> set[str]:
    try:
        tree = ast.parse(path.read_text(encoding="utf-8"))
    except (SyntaxError, UnicodeDecodeError) as exc:
        return {f"__parse_error__:{exc}"}

    dependencies: set[str] = set()
    for node in ast.walk(tree):
        if not isinstance(node, ast.Assign):
            continue
        if not any(isinstance(target, ast.Name) and target.id == "dependencies" for target in node.targets):
            continue
        if not isinstance(node.value, (ast.List, ast.Tuple)):
            continue
        for item in node.value.elts:
            if not isinstance(item, (ast.List, ast.Tuple)) or len(item.elts) < 2:
                continue
            dep_app = item.elts[0]
            dep_name = item.elts[1]
            if (
                isinstance(dep_app, ast.Constant)
                and dep_app.value == app_label
                and isinstance(dep_name, ast.Constant)
                and isinstance(dep_name.value, str)
            ):
                dependencies.add(dep_name.value)
    return dependencies


def migrations_dirs() -> list[Path]:
    if mode() == "baseline":
        return sorted(path for path in root().glob("*/migrations") if path.is_dir())

    dirs = {
        path.parent
        for path in listed_files()
        if path.parent.name == "migrations" and path.name[:4].isdigit() and path.suffix == ".py"
    }
    return sorted(dirs)


def main() -> int:
    dirs = migrations_dirs()
    if not dirs:
        status("skip", "no django migration files")
        return 0

    findings: list[str] = []
    for migrations_dir in dirs:
        app_label = migrations_dir.parent.name
        migration_files = [
            path
            for path in migrations_dir.glob("[0-9][0-9][0-9][0-9]_*.py")
            if path.name != "__init__.py"
        ]
        names = {migration_name(path) for path in migration_files}
        depended_on: set[str] = set()
        for path in migration_files:
            dependencies = dependency_names(path, app_label)
            parse_errors = [item for item in dependencies if item.startswith("__parse_error__:")]
            for parse_error in parse_errors:
                findings.append(f"{path}: could not parse dependencies: {parse_error}")
            depended_on.update(name for name in dependencies if name in names)
        leaves = sorted(names - depended_on)
        if len(leaves) > 1:
            findings.append(f"{app_label}: unresolved migration leaf conflict: {', '.join(leaves)}")
    return finish_findings(findings)


if __name__ == "__main__":
    raise SystemExit(main())
