#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "pillow>=10.0",
#   "pypdf>=4.0",
#   "reportlab>=4.0",
# ]
# ///
"""Python prototype for arranging card images on A4 PDF pages."""

from __future__ import annotations

import argparse
import glob
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
    gap_values: tuple[float, ...] = ()


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
    source_name: str
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
    xs = column_positions(opts, cols)

    page = 0
    col = 0
    row = 0
    placements: list[Placement] = []

    for path in files:
        x = xs[col]
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
    xs = fitting_column_positions(opts)
    max_cols = len(xs)

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

        placements.append(Placement(path, page, xs[col], y, CARD_W_MM, CARD_H_MM))

        placed += 1
        y += CARD_H_MM + effective_vgap

    return placements


def gap_values(opts: LayoutOptions) -> tuple[float, ...]:
    return opts.gap_values or (opts.gap,)


def gap_at(values: tuple[float, ...], index: int) -> float:
    return values[index % len(values)]


def fitting_column_positions(opts: LayoutOptions) -> list[float]:
    values = gap_values(opts)
    cols = 1
    total_w = CARD_W_MM
    while total_w + gap_at(values, cols - 1) + CARD_W_MM <= PAGE_W_MM:
        total_w += gap_at(values, cols - 1) + CARD_W_MM
        cols += 1
    return column_positions(opts, cols)


def column_positions(opts: LayoutOptions, cols: int) -> list[float]:
    values = gap_values(opts)
    total_w = cols * CARD_W_MM
    for i in range(cols - 1):
        total_w += gap_at(values, i)
    x = (PAGE_W_MM - total_w) / 2
    xs: list[float] = []
    for i in range(cols):
        xs.append(x)
        if i < cols - 1:
            x += CARD_W_MM + gap_at(values, i)
    return xs


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
                source_name=path.name,
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
                            source_name=path.name,
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
                            source_name=path.name,
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


def generate_pdf(
    out_file: Path,
    placements: list[Placement],
    source_names: list[str],
    preserve_aspect: bool,
    attach: bool = False,
    original_paths: list[Path] | None = None,
) -> None:
    try:
        from reportlab.lib.pagesizes import A4
        from reportlab.pdfgen import canvas
        import reportlab.pdfbase.pdfdoc as rldoc
    except ModuleNotFoundError as err:
        raise RuntimeError("reportlab is not installed; run this script with: uv run cardsheet.py") from err

    # Patch PDFImageXObject so we can inject /CardsheetSourceFilename before save.
    _patch_image_xobject(rldoc)

    pdf = canvas.Canvas(str(out_file), pagesize=A4)
    current_page = 0
    source_index = 0

    for placement in placements:
        while current_page < placement.page:
            pdf.showPage()
            current_page += 1

        x = mm(placement.x)
        y = mm(PAGE_H_MM - placement.y - placement.h)
        w = mm(placement.w)
        h = mm(placement.h)

        # Snapshot keys before drawing so we can detect the newly registered XObject.
        keys_before = set(pdf._doc.idToObject)
        pdf.drawImage(
            str(placement.path),
            x,
            y,
            width=w,
            height=h,
            preserveAspectRatio=preserve_aspect,
            anchor="c",
        )
        # Tag any newly created image XObject with its source name.
        if source_index < len(source_names):
            new_keys = [k for k in pdf._doc.idToObject if k not in keys_before]
            for key in new_keys:
                xobj = pdf._doc.idToObject[key]
                if isinstance(xobj, rldoc.PDFImageXObject):
                    xobj._cardsheet_extra = {
                        "CardsheetSourceFilename": rldoc.PDFString(Path(source_names[source_index]).name)
                    }
                    source_index += 1
                    break

    if attach and original_paths:
        _embed_attachments(pdf, original_paths)

    pdf.save()


def _patch_image_xobject(rldoc: object) -> None:
    """Ensure rldoc.PDFImageXObject is our tagged subclass (idempotent)."""
    if getattr(rldoc.PDFImageXObject, "_cardsheet_patched", False):
        return

    base = rldoc.PDFImageXObject

    class TaggedImageXObject(base):
        _cardsheet_patched = True
        _cardsheet_extra: dict = {}

        def format(self, document: object) -> object:
            extra = self._cardsheet_extra
            if not extra:
                return super().format(document)
            OrigStream = rldoc.PDFStream
            _extra = extra

            class _InjectedStream(OrigStream):
                def __init__(self, *args: object, **kwargs: object) -> None:
                    super().__init__(*args, **kwargs)
                    for k, v in _extra.items():
                        self.dictionary[k] = v

            rldoc.PDFStream = _InjectedStream
            try:
                return super().format(document)
            finally:
                rldoc.PDFStream = OrigStream

    rldoc.PDFImageXObject = TaggedImageXObject


def _embed_attachments(pdf: "canvas.Canvas", paths: list[Path]) -> None:
    """Embed files as PDF-level attachments using ReportLab's internal PDF objects."""
    import reportlab.pdfbase.pdfdoc as rldoc

    doc = pdf._doc
    seen: set[str] = set()
    names_array: list[object] = []

    for path in sorted(paths, key=lambda p: p.name):
        name = path.name
        if name in seen:
            base, ext = path.stem, path.suffix
            i = 1
            while f"{base}-{i}{ext}" in seen:
                i += 1
            name = f"{base}-{i}{ext}"
        seen.add(name)

        ef_stream = rldoc.PDFStream(
            dictionary=rldoc.PDFDictionary({"Type": rldoc.PDFName("EmbeddedFile")}),
            content=path.read_bytes(),
        )
        ef_ref = doc.Reference(ef_stream)
        fs_dict = rldoc.PDFDictionary({
            "Type": rldoc.PDFName("Filespec"),
            "F": rldoc.PDFString(name),
            "EF": rldoc.PDFDictionary({"F": ef_ref}),
        })
        fs_ref = doc.Reference(fs_dict)
        names_array.append(rldoc.PDFString(name))
        names_array.append(fs_ref)

    doc.Catalog.Names = rldoc.PDFDictionary({
        "EmbeddedFiles": rldoc.PDFDictionary({
            "Names": rldoc.PDFArray(names_array),
        }),
    })


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="cardsheet.py",
        description="Arrange fixed-size card images on A4 PDF pages.",
    )
    parser.add_argument("files", nargs="*", type=Path)
    parser.add_argument("-gap", type=float, action="append", default=None, help="Horizontal gap in mm; repeat to alternate")
    parser.add_argument("-vgap", type=float, default=None, help="Vertical gap in mm")
    parser.add_argument("-dpi", type=int, default=0, help="Limit embedded image resolution to this effective DPI")
    parser.add_argument("-verbose", action="store_true", help="Show image DPI and resize information")
    parser.add_argument("-side-by-side", action="store_true", help="Force side-by-side layout")
    parser.add_argument("-out", type=Path, default=Path("output.pdf"), help="Output PDF file")
    parser.add_argument("-stretch", action="store_true", help="Stretch images to fill each card rectangle")
    parser.add_argument("-attach", action="store_true", help="Also embed each source image as a file attachment in the PDF")
    parser.add_argument("-version", action="store_true", help="Show version and exit")
    return parser.parse_args(argv)


def parse_extract_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="cardsheet.py extract")
    parser.add_argument("pdf", type=Path)
    parser.add_argument("--out-dir", type=Path, default=Path("."))
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--overwrite", action="store_true")
    group.add_argument("--rename", action="store_true")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    if argv and argv[0] == "extract":
        args = parse_extract_args(argv[1:])
        mode = "overwrite" if args.overwrite else "rename" if args.rename else "ask"
        try:
            extract_pdf_images(args.pdf, args.out_dir, mode)
        except (RuntimeError, OSError, ValueError) as err:
            print(f"extract error: {err}", file=sys.stderr)
            return 1
        return 0

    args = parse_args(argv)

    if args.version:
        print("cardsheet local")
        print(f"backend: reportlab {reportlab_version()}")
        return 0

    try:
        args.files = expand_wildcards(args.files)
    except ValueError as err:
        print(f"input error: {err}", file=sys.stderr)
        return 1

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
    temp_dirs: list[tempfile.TemporaryDirectory[str]] = []
    try:
        input_files = expand_pdf_inputs(args.files, temp_dirs)
        images, temp_files = prepare_images(input_files, args.dpi)
    except (RuntimeError, OSError, ValueError) as err:
        print(f"input error: {err}", file=sys.stderr)
        return 1

    if args.verbose:
        print_image_info(images, args.dpi)

    mode = MODE_SIDE if args.side_by_side else MODE_NORMAL
    gaps = args.gap
    if gaps is None:
        gaps = [5.0 if mode == MODE_SIDE else 10.0]

    vgap_passed = args.vgap is not None
    vgap = args.vgap
    if vgap is None:
        vgap = 10.0 if mode == MODE_SIDE else 5.0

    prepared_files = [img.path for img in images]
    placements = plan(
        prepared_files,
        LayoutOptions(
            mode=mode,
            gap=gaps[0],
            vgap=vgap,
            alternate_normal_gap=mode == MODE_NORMAL and not vgap_passed,
            gap_values=tuple(gaps),
        ),
    )

    try:
        generate_pdf(
            args.out,
            placements,
            [img.source_name for img in images],
            preserve_aspect=not args.stretch,
            attach=args.attach,
            original_paths=[img.original_path for img in images] if args.attach else None,
        )
    except RuntimeError as err:
        print(f"save error: {err}", file=sys.stderr)
        return 2
    finally:
        for path in temp_files:
            path.unlink(missing_ok=True)
        for tmp in temp_dirs:
            tmp.cleanup()

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


def expand_wildcards(files: list[Path]) -> list[Path]:
    expanded: list[Path] = []
    for path in files:
        text = path.as_posix()
        if not any(ch in text for ch in "*?["):
            expanded.append(path)
            continue
        matches = sorted(glob.glob(text))
        if not matches:
            raise ValueError(f"{path}: wildcard matched no files")
        expanded.extend(Path(match) for match in matches)
    return expanded


def expand_pdf_inputs(files: list[Path], temp_dirs: list[tempfile.TemporaryDirectory[str]]) -> list[Path]:
    expanded: list[Path] = []
    for path in files:
        if path.suffix.lower() != ".pdf":
            expanded.append(path)
            continue
        tmp = tempfile.TemporaryDirectory(prefix="cardsheet-pdf-input-")
        temp_dirs.append(tmp)
        expanded.extend(extract_pdf_images(path, Path(tmp.name), "rename", require_source_names=True))
    return expanded


def extract_pdf_images(
    pdf_path: Path,
    out_dir: Path,
    conflict_mode: str,
    require_source_names: bool = False,
) -> list[Path]:
    try:
        from pypdf import PdfReader
    except ModuleNotFoundError as err:
        raise RuntimeError("pypdf is not installed; run this script with: uv run cardsheet.py") from err

    out_dir.mkdir(parents=True, exist_ok=True)
    reader = PdfReader(str(pdf_path))
    written: list[Path] = []
    fallback = 1
    for page in reader.pages:
        xobjects = page.get("/Resources", {}).get("/XObject", {})
        for image_file in getattr(page, "images", []):
            source_name = ""
            raw = xobjects.get(f"/{image_file.name}")
            if raw is not None:
                source_name = str(raw.get_object().get("/CardsheetSourceFilename", ""))
            if source_name:
                name = Path(source_name).name
            else:
                if require_source_names:
                    raise ValueError(f"{pdf_path}: PDF input must be a PDF previously created by cardsheet")
                suffix = Path(image_file.name).suffix or ".jpg"
                name = f"{pdf_path.stem}{fallback}{suffix}"
            fallback += 1
            target, should_write = resolve_output_path(out_dir / name, conflict_mode)
            if not should_write:
                continue
            target.write_bytes(image_file.data)
            written.append(target)
    return written


def resolve_output_path(path: Path, mode: str) -> tuple[Path, bool]:
    if not path.exists():
        return path, True
    if mode == "overwrite":
        return path, True
    if mode == "rename":
        return renamed_path(path), True
    if not sys.stdin.isatty():
        raise ValueError(f"{path} exists; use --overwrite or --rename")
    stat = path.stat()
    print(
        f"{path} exists ({format_bytes(stat.st_size)}, modified "
        f"{stat.st_mtime:.0f}). overwrite/rename/skip? [o/r/s]: ",
        end="",
    )
    answer = sys.stdin.readline().strip().lower()
    if answer in {"o", "overwrite"}:
        return path, True
    if answer in {"r", "rename"}:
        return renamed_path(path), True
    return path, False


def renamed_path(path: Path) -> Path:
    for i in range(1, 1000000):
        candidate = path.with_name(f"{path.stem}-{i}{path.suffix}")
        if not candidate.exists():
            return candidate
    raise ValueError(f"could not find available name for {path}")


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
