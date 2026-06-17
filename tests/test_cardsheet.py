import contextlib
import io
import tempfile
import unittest
from pathlib import Path

import cardsheet


def base_options() -> cardsheet.LayoutOptions:
    return cardsheet.LayoutOptions(
        mode=cardsheet.MODE_NORMAL,
        gap=10.0,
        vgap=5.0,
        alternate_normal_gap=False,
    )


class LayoutTests(unittest.TestCase):
    def assert_close(self, got: float, want: float) -> None:
        self.assertAlmostEqual(got, want, places=6)

    def test_normal_layout_keeps_grid_centering(self) -> None:
        got = cardsheet.plan([Path("a.jpg")], base_options())

        self.assertEqual(len(got), 1)
        self.assert_close(got[0].x, 14.4)
        self.assert_close(got[0].y, 20.0)

    def test_normal_layout_uses_explicit_vgap(self) -> None:
        opts = cardsheet.LayoutOptions(
            mode=cardsheet.MODE_NORMAL,
            gap=10.0,
            vgap=12.0,
            alternate_normal_gap=False,
        )

        got = cardsheet.plan([Path("a.jpg"), Path("b.jpg")], opts)

        self.assertEqual(len(got), 2)
        self.assert_close(got[1].y, 20.0 + cardsheet.CARD_H_MM + 12.0)

    def test_normal_layout_alternates_default_vgap(self) -> None:
        opts = cardsheet.LayoutOptions(
            mode=cardsheet.MODE_NORMAL,
            gap=10.0,
            vgap=5.0,
            alternate_normal_gap=True,
        )

        got = cardsheet.plan([Path("a.jpg"), Path("b.jpg"), Path("c.jpg")], opts)

        self.assertEqual(len(got), 3)
        want = 20.0 + cardsheet.CARD_H_MM + 5.0 + cardsheet.CARD_H_MM + 10.0
        self.assert_close(got[2].y, want)

    def test_normal_layout_moves_to_next_column(self) -> None:
        opts = cardsheet.LayoutOptions(
            mode=cardsheet.MODE_NORMAL,
            gap=10.0,
            vgap=5.0,
            alternate_normal_gap=True,
        )

        got = cardsheet.plan(
            [Path(name) for name in ["a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"]],
            opts,
        )

        self.assertEqual(len(got), 5)
        self.assertEqual(got[4].page, 0)
        self.assert_close(got[4].x, 110.0)
        self.assert_close(got[4].y, 20.0)

    def test_side_layout_starts_new_page(self) -> None:
        opts = cardsheet.LayoutOptions(
            mode=cardsheet.MODE_SIDE,
            gap=5.0,
            vgap=10.0,
            alternate_normal_gap=False,
        )

        got = cardsheet.plan([Path(f"{i}.jpg") for i in range(9)], opts)

        self.assertEqual(len(got), 9)
        self.assertEqual(got[8].page, 1)
        self.assert_close(got[8].y, 20.0)


class CliTests(unittest.TestCase):
    def test_parse_args_defaults(self) -> None:
        args = cardsheet.parse_args([Path("a.jpg").as_posix()])

        self.assertEqual(args.files, [Path("a.jpg")])
        self.assertIsNone(args.gap)
        self.assertIsNone(args.vgap)
        self.assertEqual(args.dpi, 0)
        self.assertEqual(args.out, Path("output.pdf"))
        self.assertFalse(args.side_by_side)
        self.assertFalse(args.stretch)

    def test_parse_args_collects_repeated_gap(self) -> None:
        args = cardsheet.parse_args(["-gap", "5", "-gap", "10", "a.jpg"])

        self.assertEqual(args.gap, [5.0, 10.0])

    def test_no_files_returns_usage_error(self) -> None:
        stdout = io.StringIO()

        with contextlib.redirect_stdout(stdout):
            code = cardsheet.main([])

        self.assertEqual(code, 1)
        self.assertIn("Usage: cardsheet.py", stdout.getvalue())

    def test_negative_dpi_returns_input_error(self) -> None:
        with tempfile.NamedTemporaryFile(suffix=".jpg") as tmp:
            stderr = io.StringIO()

            with contextlib.redirect_stderr(stderr):
                code = cardsheet.main(["-dpi", "-1", tmp.name])

        self.assertEqual(code, 1)
        self.assertIn("-dpi must be greater than or equal to 0", stderr.getvalue())

    def test_expand_wildcards_sorts_matches(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "b.png").write_text("x")
            (root / "a.png").write_text("x")

            got = cardsheet.expand_wildcards([root / "*.png"])

        self.assertEqual([p.name for p in got], ["a.png", "b.png"])


class ImagePreprocessTests(unittest.TestCase):
    def test_prepare_images_downsamples_png_when_pillow_is_available(self) -> None:
        try:
            from PIL import Image
        except ModuleNotFoundError:
            self.skipTest("Pillow is not installed")

        temp_files: list[Path] = []
        with tempfile.TemporaryDirectory() as tmpdir:
            source = Path(tmpdir) / "source.png"
            Image.new("RGB", (1200, 800), "white").save(source)

            results, temp_files = cardsheet.prepare_images([source], max_dpi=100)

            self.assertEqual(len(results), 1)
            self.assertEqual(results[0].original_path, source)
            if results[0].resized:
                self.assertEqual(results[0].path.suffix, ".png")
                self.assertTrue(results[0].path.exists())
                self.assertLessEqual(results[0].output_width, 337)
                self.assertLessEqual(results[0].output_height, 213)
            else:
                self.assertTrue(results[0].kept_original)
        for path in temp_files:
            path.unlink(missing_ok=True)


class PythonRoundtripTests(unittest.TestCase):
    def test_generate_extract_preserves_source_names(self) -> None:
        try:
            from PIL import Image
            import pypdf  # noqa: F401
            import reportlab  # noqa: F401
        except ModuleNotFoundError as err:
            self.skipTest(f"{err.name} is not installed")

        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            first = root / "front.png"
            second = root / "back.png"
            Image.new("RGB", (32, 20), "red").save(first)
            Image.new("RGB", (32, 20), "blue").save(second)

            pdf = root / "cards.pdf"
            code = cardsheet.main(["-out", str(pdf), str(first), str(second)])
            self.assertEqual(code, 0)

            out_dir = root / "extract"
            code = cardsheet.main(["extract", "--out-dir", str(out_dir), "--rename", str(pdf)])
            self.assertEqual(code, 0)
            self.assertTrue((out_dir / "front.png").exists())
            self.assertTrue((out_dir / "back.png").exists())

    def test_pdf_input_rejects_pdf_without_source_names(self) -> None:
        try:
            from PIL import Image
            from reportlab.lib.pagesizes import A4
            from reportlab.pdfgen import canvas
            import pypdf  # noqa: F401
        except ModuleNotFoundError as err:
            self.skipTest(f"{err.name} is not installed")

        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            image = root / "foreign.png"
            Image.new("RGB", (32, 20), "green").save(image)

            pdf = root / "foreign.pdf"
            doc = canvas.Canvas(str(pdf), pagesize=A4)
            doc.drawImage(str(image), 20, 700, width=120, height=80)
            doc.save()

            with self.assertRaises(ValueError):
                cardsheet.expand_pdf_inputs([pdf], [])

            out_dir = root / "extract"
            written = cardsheet.extract_pdf_images(pdf, out_dir, "rename")
            self.assertEqual(len(written), 1)
            self.assertEqual(written[0].name, "foreign1.png")


if __name__ == "__main__":
    unittest.main()
