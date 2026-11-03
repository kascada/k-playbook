"""Tests für checks/lib/command_tool_guards.py: effective_commands.

Läuft mit `python3 -m unittest discover -s checks/tests` (Ziel `test` im Makefile).
"""
from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

LIB = Path(__file__).resolve().parent.parent / "lib"
# Die Module unter checks/lib importieren einander ohne Paket.
sys.path.insert(0, str(LIB))

from command_tool_guards import effective_commands  # noqa: E402


class EffectiveCommandsTest(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        self.playbook = self.root / "k-playbook"
        self.local = self.root / "k-playbook-local"

    def put(self, path: Path, text: str = "# Command\n\nText.\n") -> Path:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def test_wrapper_and_text_of_same_name_are_both_checked(self) -> None:
        wrapper = self.put(self.playbook / "share" / "commands" / "k-x.md")
        text = self.put(self.playbook / "defaults" / "commands" / "k-x.md")
        module = self.put(self.playbook / "defaults" / "commands" / "_mod" / "a.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {
            "k-x.md": wrapper,
            "defaults/commands/k-x.md": text,
            "defaults/commands/_mod/a.md": module,
        })

    def test_local_entry_overrides_wrapper_and_text(self) -> None:
        self.put(self.playbook / "share" / "commands" / "k-x.md")
        self.put(self.playbook / "defaults" / "commands" / "k-x.md")
        local = self.put(self.local / "commands" / "k-x.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {"k-x.md": local})

    def test_local_text_only_overrides_text(self) -> None:
        self.put(self.playbook / "defaults" / "commands" / "k-y.md")
        local = self.put(self.local / "commands" / "k-y.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {"k-y.md": local})

    def test_empty_local_entry_disables_wrapper_and_text(self) -> None:
        self.put(self.playbook / "share" / "commands" / "k-x.md")
        self.put(self.playbook / "defaults" / "commands" / "k-x.md")
        self.put(self.local / "commands" / "k-x.md", "# nur Kommentar\n\n")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {})

    def test_readme_and_dotfiles_are_no_entries(self) -> None:
        self.put(self.playbook / "defaults" / "commands" / "README.md")
        self.put(self.playbook / "defaults" / "commands" / ".hidden.md")
        self.put(self.playbook / "share" / "commands" / "README.md")
        self.assertEqual(effective_commands(self.playbook, self.local), {})


if __name__ == "__main__":
    unittest.main()
