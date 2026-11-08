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

from command_tool_guards import effective_commands, project_commands_dir  # noqa: E402


class EffectiveCommandsTest(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        self.playbook = self.root / "k-playbook"
        self.old_local = self.root / "k-playbook-local" / "commands"
        self.local = self.root / ".k-playbook" / "commands"

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
        local = self.put(self.local / "k-x.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {"k-x.md": local})

    def test_local_text_only_overrides_text(self) -> None:
        self.put(self.playbook / "defaults" / "commands" / "k-y.md")
        local = self.put(self.local / "k-y.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {"k-y.md": local})

    def test_empty_local_entry_disables_wrapper_and_text(self) -> None:
        self.put(self.playbook / "share" / "commands" / "k-x.md")
        self.put(self.playbook / "defaults" / "commands" / "k-x.md")
        self.put(self.local / "k-x.md", "# nur Kommentar\n\n")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {})

    def test_readme_and_dotfiles_are_no_entries(self) -> None:
        self.put(self.playbook / "defaults" / "commands" / "README.md")
        self.put(self.playbook / "defaults" / "commands" / ".hidden.md")
        self.put(self.playbook / "share" / "commands" / "README.md")
        self.assertEqual(effective_commands(self.playbook, self.local), {})

    def test_namespace_entry_overrides_file_by_file(self) -> None:
        self.put(self.playbook / "share" / "commands" / "_x" / "eins.md")
        zwei = self.put(self.playbook / "share" / "commands" / "_x" / "zwei.md")
        eins = self.put(self.local / "_x" / "eins.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {"_x/eins.md": eins, "_x/zwei.md": zwei})

    def test_old_location_is_not_read(self) -> None:
        wrapper = self.put(self.playbook / "share" / "commands" / "k-x.md")
        self.put(self.old_local / "k-x.md", "")
        self.put(self.old_local / "k-alt.md")
        result = effective_commands(self.playbook, self.local)
        self.assertEqual(result, {"k-x.md": wrapper})

    def test_project_commands_dir_is_below_dot_k_playbook(self) -> None:
        self.assertEqual(project_commands_dir(self.root), self.local)


if __name__ == "__main__":
    unittest.main()
