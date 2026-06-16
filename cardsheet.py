#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "pillow>=10.0",
#   "reportlab>=4.0",
# ]
# ///
"""Python prototype for arranging card images on A4 PDF pages."""

from __future__ import annotations

import argparse
import importlib.metadata
import tempfile
import sys
from dataclasses import dataclass
from pathlib import Path


MODE_NORMAL = "normal"
MODE_SIDE = "side"

CARD_W_MM = 85.6
CARD_H_MM = 53.98
PAGE_W_MM = 210.0
PAGE_H_MM = 297.0
TOP_MARGIN_MM = 20.0
BOTTOM_MARGIN_MM = 20.0


@dataclass(frozen=True)
class LayoutOptions:
    mode: str
    gap: float
    vgap: float
    alternate_normal_gap: bool


@dataclass(frozen=True)
class Placement:
    path: Path
    page: int
    x: float
    y: float
    w: float
    h: float


@dataclass(frozen=True)
class ImageInfo:
    original_path: Path
    path: Path
    width: int
    height: int
    effective_x: float
    effective_y: float
    output_width: int
    output_height: int
    output_x: float
    output_y: float
    original_size: int
    output_size: int
    resized: bool
    kept_original: bool
    temp: bool


def mm(value: float) -> float:
    return value * 72.0 / 25.4


def effective_dpi(px: int, mm_value: float) -> float:
    return px / (mm_value / 25.4)


def plan(files: list[Path], opts: LayoutOptions) -> list[Placement]:
    if opts.mode == MODE_SIDE:
        return plan_side(files, opts)
    return plan_normal(files, opts)


def plan_side(files: list[Path], opts: LayoutOptions) -> list[Placement]:
    cols = 2
    bottom_limit = PAGE_H_MM - BOTTOM_MARGIN_MM
    x_start = (PAGE_W_MM - cols * (CARD_W_MM + opts.gap) + opts.gap) / 2

    page = 0
    col = 0
    row = 0
    placements: list[Placement] = []

    for path in files:
        x = x_start + col * (CARD_W_MM + opts.gap)
        y = TOP_MARGIN_MM + row * (CARD_H_MM + opts.vgap)

        if y + CARD_H_MM > bottom_limit:
            page += 1
            row = 0
            y = TOP_MARGIN_MM

        placements.append(Placement(path, page, x, y, CARD_W_MM, CARD_H_MM))

        col += 1
        if col == cols:
            col = 0
            row += 1

    return placements


def plan_normal(files: list[Path], opts: LayoutOptions) -> list[Placement]:
    bottom_limit = PAGE_H_MM - BOTTOM_MARGIN_MM
    max_cols = int((PAGE_W_MM + opts.gap) / (CARD_W_MM + opts.gap))
    max_cols = max(max_cols, 1)
    x_start = (PAGE_W_MM - max_cols * (CARD_W_MM + opts.gap) + opts.gap) / 2

    page = 0
    col = 0
    y = TOP_MARGIN_MM
    placed = 0
    placements: list[Placement] = []

    for path in files:
        effective_vgap = opts.vgap
        if opts.alternate_normal_gap:
            effective_vgap = 5.0 if placed % 2 == 0 else 10.0

        if y + CARD_H_MM > bottom_limit:
            if col + 1 < max_cols:
                col += 1
                y = TOP_MARGIN_MM
            else:
                page += 1
                col = 0
                y = TOP_MARGIN_MM

        x = x_start + col * (CARD_W_MM + opts.gap)
        placements.append(Placement(path, page, x, y, CARD_W_MM, CARD_H_MM))

        placed += 1
        y += CARD_H_MM + effective_vgap

    return placements


def reportlab_version() -> str:
    try:
        return importlib.metadata.version("reportlab")
    except importlib.metadata.PackageNotFoundError:
        return "not installed"


def validate_inputs(files: list[Path]) -> None:
    for path in files:
        if not path.is_file():
            raise ValueError(f"{path}: file does not exist")


def prepare_images(files: list[Path], max_dpi: int) -> tuple[list[ImageInfo], list[Path]]:
    try:
        from PIL import Image
    except ModuleNotFoundError as err:
        raise RuntimeError("pillow is not installed; run this script with: uv run cardsheet.py") from err

    results: list[ImageInfo] = []
    temp_files: list[Path] = []

    for path in files:
        with Image.open(path) as img:
            img.load()
            width, height = img.size
            original_size = path.stat().st_size
            info = ImageInfo(
                original_path=path,
                path=path,
                width=width,
                height=height,
                effective_x=effective_dpi(width, CARD_W_MM),
                effective_y=effective_dpi(height, CARD_H_MM),
                output_width=width,
                output_height=height,
                output_x=effective_dpi(width, CARD_W_MM),
                output_y=effective_dpi(height, CARD_H_MM),
                original_size=original_size,
                output_size=original_size,
                resized=False,
                kept_original=False,
                temp=False,
            )

            if max_dpi > 0:
                target_w = round((CARD_W_MM / 25.4) * max_dpi)
                target_h = round((CARD_H_MM / 25.4) * max_dpi)
                if width > target_w or height > target_h:
                    scale = min(target_w / width, target_h / height)
                    new_w = max(1, round(width * scale))
                    new_h = max(1, round(height * scale))

                    resized = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
                    suffix = ".png" if img.format == "PNG" else ".jpg"
                    tmp = tempfile.NamedTemporaryFile(prefix="cardsheet-", suffix=suffix, delete=False)
                    tmp_path = Path(tmp.name)
                    tmp.close()

                    if suffix == ".png":
                        resized.save(tmp_path, format="PNG")
                    else:
                        resized.convert("RGB").save(tmp_path, format="JPEG", quality=90)

                    output_size = tmp_path.stat().st_size
                    if output_size >= original_size:
                        tmp_path.unlink(missing_ok=True)
                        info = ImageInfo(
                            original_path=path,
                            path=path,
                            width=width,
                            height=height,
                            effective_x=effective_dpi(width, CARD_W_MM),
                            effective_y=effective_dpi(height, CARD_H_MM),
                            output_width=width,
                            output_height=height,
                            output_x=effective_dpi(width, CARD_W_MM),
                            output_y=effective_dpi(height, CARD_H_MM),
                            original_size=original_size,
                            output_size=original_size,
                            resized=False,
                            kept_original=True,
                            temp=False,
                        )
                    else:
                        temp_files.append(tmp_path)
                        info = ImageInfo(
                            original_path=path,
                            path=tmp_path,
                            width=width,
                            height=height,
                            effective_x=effective_dpi(width, CARD_W_MM),
                            effective_y=effective_dpi(height, CARD_H_MM),
                            output_width=new_w,
                            output_height=new_h,
                            output_x=effective_dpi(new_w, CARD_W_MM),
                            output_y=effective_dpi(new_h, CARD_H_MM),
                            original_size=original_size,
                            output_size=output_size,
                            resized=True,
                            kept_original=False,
                            temp=True,
                        )

            results.append(info)

    return results, temp_files


def generate_pdf(out_file: Path, placements: list[Placement], preserve_aspect: bool) -> None:
    try:
        from reportlab.lib.pagesizes import A4
        from reportlab.pdfgen import canvas
    except ModuleNotFoundError as err:
        raise RuntimeError("reportlab is not installed; run this script with: uv run cardsheet.py") from err

    pdf = canvas.Canvas(str(out_file), pagesize=A4)
    current_page = 0

    for placement in placements:
        while current_page < placement.page:
            pdf.showPage()
            current_page += 1

        x = mm(placement.x)
        y = mm(PAGE_H_MM - placement.y - placement.h)
        w = mm(placement.w)
        h = mm(placement.h)

        pdf.drawImage(
            str(placement.path),
            x,
            y,
            width=w,
            height=h,
            preserveAspectRatio=preserve_aspect,
            anchor="c",
        )

    pdf.save()


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="cardsheet.py",
        description="Arrange fixed-size card images on A4 PDF pages.",
    )
    parser.add_argument("files", nargs="*", type=Path)
    parser.add_argument("-gap", type=float, default=None, help="Horizontal gap in mm")
    parser.add_argument("-vgap", type=float, default=None, help="Vertical gap in mm")
    parser.add_argument("-dpi", type=int, default=0, help="Limit embedded image resolution to this effective DPI")
    parser.add_argument("-verbose", action="store_true", help="Show image DPI and resize information")
    parser.add_argument("-side-by-side", action="store_true", help="Force side-by-side layout")
    parser.add_argument("-out", type=Path, default=Path("output.pdf"), help="Output PDF file")
    parser.add_argument("-stretch", action="store_true", help="Stretch images to fill each card rectangle")
    parser.add_argument("-version", action="store_true", help="Show version and exit")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)

    if args.version:
        print("cardsheet local")
        print(f"backend: reportlab {reportlab_version()}")
        return 0

    if not args.files:
        print("Usage: cardsheet.py [options] img1 img2 ...")
        return 1

    try:
        validate_inputs(args.files)
    except ValueError as err:
        print(f"input error: {err}", file=sys.stderr)
        return 1
    if args.dpi < 0:
        print("input error: -dpi must be greater than or equal to 0", file=sys.stderr)
        return 1

    temp_files: list[Path] = []
    try:
        images, temp_files = prepare_images(args.files, args.dpi)
    except (RuntimeError, OSError) as err:
        print(f"input error: {err}", file=sys.stderr)
        return 1

    if args.verbose:
        print_image_info(images, args.dpi)

    mode = MODE_SIDE if args.side_by_side else MODE_NORMAL
    gap = args.gap
    if gap is None:
        gap = 5.0 if mode == MODE_SIDE else 10.0

    vgap_passed = args.vgap is not None
    vgap = args.vgap
    if vgap is None:
        vgap = 10.0 if mode == MODE_SIDE else 5.0

    prepared_files = [img.path for img in images]
    placements = plan(
        prepared_files,
        LayoutOptions(
            mode=mode,
            gap=gap,
            vgap=vgap,
            alternate_normal_gap=mode == MODE_NORMAL and not vgap_passed,
        ),
    )

    try:
        generate_pdf(args.out, placements, preserve_aspect=not args.stretch)
    except RuntimeError as err:
        print(f"save error: {err}", file=sys.stderr)
        return 2
    finally:
        for path in temp_files:
            path.unlink(missing_ok=True)

    print(f"Saved: {args.out}")
    return 0


def print_image_info(images: list[ImageInfo], max_dpi: int) -> None:
    for img in images:
        min_dpi = min(img.effective_x, img.effective_y)
        status = "ok"
        if max_dpi <= 0 and min_dpi < 300:
            status = "below 300 dpi"

        message = (
            f"{img.original_path}: {img.width}x{img.height}, "
            f"effective {img.effective_x:.0f}x{img.effective_y:.0f} dpi"
        )

        if max_dpi > 0:
            if img.kept_original:
                message += ", kept original; resized candidate was larger"
                print(message)
                continue

            saved = img.original_size - img.output_size
            saved_pct = 0.0
            if img.original_size:
                saved_pct = saved * 100 / img.original_size
            sign = "-" if saved_pct > 0 else "+"
            message += (
                f", output {img.output_width}x{img.output_height}, "
                f"{img.output_x:.0f}x{img.output_y:.0f} dpi, "
                f"size {format_bytes(img.original_size)} -> {format_bytes(img.output_size)} "
                f"({sign}{abs(saved_pct):.1f}%)"
            )
            if not img.resized:
                message += ", unchanged"
        else:
            message += f", {status}"

        print(message)


def format_bytes(value: int) -> str:
    if value < 1024:
        return f"{value} B"
    labels = ["KiB", "MiB", "GiB"]
    amount = float(value)
    for label in labels:
        amount /= 1024
        if amount < 1024:
            return f"{amount:.1f} {label}"
    return f"{amount / 1024:.1f} TiB"


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
