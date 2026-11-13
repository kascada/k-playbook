"""Tests für checks/lib/verweise.py: scan, read_exceptions, ExceptionEntry.covers.

Läuft mit `python3 -m unittest discover -s checks/tests` (Ziel `test` im Makefile).
"""
from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

LIB = Path(__file__).resolve().parent.parent / "lib"
# verweise.py importiert common und command_tool_guards ohne Paket.
sys.path.insert(0, str(LIB))

from verweise import ExceptionEntry, read_exceptions, scan, source_files  # noqa: E402


def lines(text: str) -> list[str]:
    return textwrap.dedent(text).strip("\n").splitlines()


def kinds(found: list[tuple[int, str, str]]) -> list[tuple[int, str]]:
    return [(number, art) for number, art, _ in found]


class ScanTest(unittest.TestCase):
    def test_comment_in_shell_block_does_not_end_references(self) -> None:
        text = lines("""
            ## Referenzen

            ```bash
            # Kommentar, keine Überschrift
            k-playbook read x
            ```

            - `defaults/commands/x.md` — ohne Präfix
            """)
        self.assertEqual(kinds(scan(text)), [(8, "pfad")])

    def test_references_heading_in_code_block_opens_no_area(self) -> None:
        text = lines("""
            ## Vorlage

            ```markdown
            ## Referenzen

            - `docs/a.md`
            ```

            - `docs/b.md` — Fließtext ohne Lese-Wort
            """)
        self.assertEqual(scan(text), [])

    def test_references_area_ends_at_same_level_heading(self) -> None:
        text = lines("""
            ## Referenzen

            - `docs/a.md`

            ## Ziel

            - `docs/b.md`
            """)
        self.assertEqual(kinds(scan(text)), [(3, "pfad")])

    def test_read_word_outside_area(self) -> None:
        text = lines("""
            ## Ziel

            Lies zuerst `docs/a.md`.
            Dann `docs/b.md` anpassen.
            """)
        self.assertEqual(kinds(scan(text)), [(3, "pfad")])

    def test_relative_link_reported_inside_and_outside_code_block(self) -> None:
        text = lines("""
            Siehe [a](../a.md).

            ```markdown
            [b](./b.md)
            ```
            """)
        self.assertEqual(kinds(scan(text)), [(1, "link"), (4, "link")])

    def test_path_in_code_block_under_references_not_reported(self) -> None:
        text = lines("""
            ## Referenzen

            ```text
            lies `docs/a.md`
            ```

            - `docs/b.md`
            """)
        self.assertEqual(kinds(scan(text)), [(7, "pfad")])

    def test_indented_tilde_fence_and_longer_closing(self) -> None:
        text = lines("""
            ## Referenzen

            1. Schritt:

               ~~~~sh
               # Kommentar
               ```
               lies `docs/a.md`
               ~~~~~

            - `docs/b.md`
            """)
        self.assertEqual(kinds(scan(text)), [(11, "pfad")])

    def test_unclosed_block_runs_to_end(self) -> None:
        text = lines("""
            ## Referenzen

            ```
            - `docs/a.md`
            """)
        self.assertEqual(scan(text), [])

    def test_backtick_in_info_opens_no_block(self) -> None:
        text = lines("""
            ## Referenzen

            ```k-playbook read``` liest die Datei

            - `docs/a.md`
            """)
        self.assertEqual(kinds(scan(text)), [(5, "pfad")])


class ExceptionsTest(unittest.TestCase):
    def write(self, body: str) -> Path:
        handle = tempfile.NamedTemporaryFile("w", suffix=".tsv", delete=False, encoding="utf-8")
        handle.write("datei\tzeile\tart\tbegründung\n" + body)
        handle.close()
        self.addCleanup(os.unlink, handle.name)
        return Path(handle.name)

    def test_area_entry_covers_everything_below(self) -> None:
        entries, problems = read_exceptions(self.write("share/skills/*/vorlagen/\t*\t*\tInhalt für Leser\n"))
        self.assertEqual(problems, [])
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertIsNone(entry.zeile)
        self.assertTrue(entry.covers("share/skills/x/vorlagen/a.md", 3, "link"))
        self.assertTrue(entry.covers("share/skills/x/vorlagen/sub/b.md", 9, "pfad"))
        self.assertFalse(entry.covers("share/skills/x/SKILL.md", 3, "link"))

    def test_line_entry_covers_only_its_line_and_kind(self) -> None:
        entries, problems = read_exceptions(self.write("defaults/guidelines/a.md\t4\tpfad\tOrtsangabe\n"))
        self.assertEqual(problems, [])
        entry = entries[0]
        self.assertTrue(entry.covers("defaults/guidelines/a.md", 4, "pfad"))
        self.assertFalse(entry.covers("defaults/guidelines/a.md", 5, "pfad"))
        self.assertFalse(entry.covers("defaults/guidelines/a.md", 4, "link"))

    def test_area_in_commands_is_rejected(self) -> None:
        entries, problems = read_exceptions(self.write("defaults/commands/\t*\t*\tzu breit\n"))
        self.assertEqual(entries, [])
        self.assertEqual(len(problems), 1)
        self.assertIn("list each finding", problems[0])

    def test_invalid_and_duplicate_entries(self) -> None:
        entries, problems = read_exceptions(self.write(
            "defaults/guidelines/a.md\t0\tpfad\tGrund\n"
            "defaults/guidelines/a.md\t4\twort\tGrund\n"
            "defaults/guidelines/a.md\t4\tpfad\t\n"
            "defaults/guidelines/a.md\t5\tlink\tGrund\n"
            "defaults/guidelines/a.md\t5\tlink\tGrund\n"
        ))
        self.assertEqual([(e.datei, e.zeile, e.art) for e in entries], [("defaults/guidelines/a.md", 5, "link")])
        self.assertEqual(len(problems), 4)

    def test_line_entry_below_project_commands_and_skills_is_valid(self) -> None:
        entries, problems = read_exceptions(self.write(
            ".k-playbook/commands/k-eigen.md\t3\tpfad\tOrtsangabe\n"
            ".k-playbook/skills/forelle/SKILL.md\t4\tlink\tVorlage\n"
        ))
        self.assertEqual(problems, [])
        self.assertEqual(len(entries), 2)
        self.assertTrue(entries[0].covers(".k-playbook/commands/k-eigen.md", 3, "pfad"))


class SourceFilesTest(unittest.TestCase):
    """Projekteigene Commands und Skills kommen aus .k-playbook/, der alte Ort zählt nicht.

    k-playbook-local/instructions/ zählt seit Task 143 ebenso nicht mehr.
    """

    def test_project_sources_from_dot_k_playbook_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            def put(rel: str) -> Path:
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("# x\n", encoding="utf-8")
                return path
            command = put(".k-playbook/commands/_x/eins.md")
            skill = put(".k-playbook/skills/forelle/SKILL.md")
            put("k-playbook-local/guidelines/g.md")
            put("k-playbook-local/rules/r.md")
            put("k-playbook-local/commands/alt.md")
            put("k-playbook-local/skills/alt/SKILL.md")
            put("k-playbook-local/instructions/on-demand/alt.md")
            files = source_files(root / "k-playbook", root / ".k-playbook")
        self.assertEqual(files, {
            ".k-playbook/commands/_x/eins.md": command,
            ".k-playbook/skills/forelle/SKILL.md": skill,
        })

    def test_dot_k_playbook_is_a_known_root(self) -> None:
        text = lines("""
            ## Referenzen

            - `.k-playbook/commands/k-eigen.md`
            """)
        self.assertEqual(kinds(scan(text)), [(3, "pfad")])


class RunTest(unittest.TestCase):
    """Ganzer Lauf über ein kleines Playbook: Treffer, Ausnahme, veraltete Ausnahme."""

    def test_stale_exception_is_reported(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            playbook = root / "playbook"
            guidelines = playbook / "defaults" / "guidelines"
            guidelines.mkdir(parents=True)
            (guidelines / "a.md").write_text("## Referenzen\n\n- `docs/a.md`\n", encoding="utf-8")
            script_dir = root / "checks"
            script_dir.mkdir()
            (script_dir / "verweis-exceptions.tsv").write_text(
                "datei\tzeile\tart\tbegründung\n"
                "defaults/guidelines/a.md\t3\tpfad\tOrtsangabe\n"
                "defaults/guidelines/a.md\t9\tpfad\tOrtsangabe\n",
                encoding="utf-8",
            )
            env = dict(os.environ, K_CHECK_PLAYBOOK_DIR=str(playbook), K_CHECK_CONFIG_ROOT=str(root))
            result = subprocess.run(
                [sys.executable, str(LIB / "verweise.py"), str(script_dir)],
                env=env, capture_output=True, text=True, check=False,
            )
        self.assertIn("K_CHECK_STATUS=fail", result.stdout)
        self.assertIn("stale exception: no pfad finding at defaults/guidelines/a.md:9", result.stdout)
        self.assertNotIn("datei=defaults/guidelines/a.md zeile=3", result.stdout)


if __name__ == "__main__":
    unittest.main()
