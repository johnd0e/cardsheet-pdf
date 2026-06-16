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


if __name__ == "__main__":
    unittest.main()
