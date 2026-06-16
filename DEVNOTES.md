# Development Notes

## Architecture

- `main.go` owns CLI parsing and orchestration only.
- `internal/layout` owns physical page placement and is covered by unit tests.
- `pdfgen` owns backend-specific PDF generation behind the `Generator` interface.
- `internal/version` owns build and backend dependency version reporting.

## Layout Decisions

- Horizontal centering intentionally centers the full grid block, not the currently populated partial row or partial page.
- Default normal mode keeps the historical alternating vertical gap of `5/10 mm`.
- If `-vgap` is explicitly supplied in normal mode, the supplied value is used as a fixed vertical gap.
- Side-by-side mode always uses two columns.

## Backend Differences

- The project supports two Go PDF backends behind the same `Generator` interface.
- Default backend: [`fpdf`](https://github.com/go-pdf/fpdf). Build normally with `go build`, or explicitly with `go build -tags fpdf`.
- Alternative backend: [`pdfcpu`](https://github.com/pdfcpu/pdfcpu). Build with `go build -tags pdfcpu`.
- Both backends implement the same CLI behavior.
- `fpdf` draws images directly into the requested rectangle.
- `pdfcpu` uses high-level image boxes and fits image content into the requested rectangle while preserving aspect ratio.
- `pdfcpu` coordinates are converted from millimetres to PDF points before passing data to `api.Create`.
- `pdfcpu` uses `LowerLeft` origin, so y coordinates are translated from the CLI's top-left layout model.
- Page layout coordinates are shared between backends, but image content inside each card rectangle may differ slightly because of backend rendering behavior.

## Validation

- The CLI validates that every input file can be opened and decoded before creating the generator.
- JPEG and PNG are enabled via blank imports in `main.go`.
- Unsupported or unreadable images fail early with an `input error`.

## DPI Limiting

- `-dpi` limits embedded image pixel dimensions based on the fixed card rectangle, reducing PDF size while preserving printed dimensions.
- It does not change PDF page DPI; PDF placement still uses physical dimensions.
- Effective DPI is calculated as `image pixels / printed card inches`.
- `-verbose` prints original effective DPI and, when `-dpi` is active, output dimensions and byte-size change.
- Without `-dpi`, `-verbose` only reports current effective DPI and highlights images below 300 DPI.
- Downsampled images are written to temporary files and deleted after PDF generation.
- If a downsampled candidate is not smaller than the original encoded image, the original file is kept.

## Build Tags

- Default build uses the `fpdf` backend.
- `go build -tags pdfcpu` uses the `pdfcpu` backend.
- Both backend variants should pass tests:

```sh
go test ./...
go test -tags pdfcpu ./...
```

Smoke-test generation with local sample files:

```sh
go run . -out cards.pdf card1.jpg card2.jpg
go run -tags pdfcpu . -out cards.pdf card1.jpg card2.jpg
```

## Examples

The repository includes public-domain specimen cards for trying the layout modes:

| German ID card | U.S. passport card |
|----------------|--------------------|
| <img src="examples/input/de-id-front.png" width="220" alt="German ID card front specimen"> | <img src="examples/input/us-passport-card-front.jpg" width="220" alt="U.S. passport card front specimen"> |

Generate a default top-to-bottom layout:

```sh
cardsheet -out examples/output/default.pdf examples/input/de-id-front.png examples/input/de-id-back.png examples/input/us-passport-card-front.jpg examples/input/us-passport-card-back.jpg
```

Generate a two-column side-by-side layout:

```sh
cardsheet -side-by-side -out examples/output/side-by-side.pdf examples/input/de-id-front.png examples/input/de-id-back.png examples/input/us-passport-card-front.jpg examples/input/us-passport-card-back.jpg
```

The README layout thumbnails are generated from the same example images and layout coordinates:

- `examples/output/default.png`
- `examples/output/side-by-side.png`

See [examples/SOURCES.md](examples/SOURCES.md) for image sources and licensing notes.

## Platforms

- The code should build on Windows, Linux, and macOS.
- Common target architectures are amd64 and arm64.
- Cross-compilation uses standard Go environment variables:

```sh
GOOS=linux GOARCH=amd64 go build
```

## Python Prototype

- `cardsheet.py` is an experimental Python implementation of the same layout rules.
- It uses [ReportLab](https://www.reportlab.com/dev/docs/) canvas APIs and draws images into A4 pages.
- By default it preserves image aspect ratio inside each card rectangle.
- Use `-stretch` to fill each card rectangle like the `fpdf` backend.
- It declares its Python dependency inline using script metadata.
- Run it with `uv run`; `uv` creates and manages the script environment.
- It supports the same core CLI options as the Go version: `-out`, `-gap`, `-vgap`, `-dpi`, `-verbose`, `-side-by-side`, and `-version`.
- Image validation, effective-DPI reporting, and optional downsampling are implemented with [Pillow](https://python-pillow.org/).
- Unsupported or unreadable images fail with an `input error`, matching the Go CLI behavior.

```sh
uv run cardsheet.py -out cards.pdf card1.jpg card2.jpg
```

Run Python-only tests with the standard library test runner:

```sh
python -m unittest discover -s tests
```

## Version Reporting

- `AppVersion` can be injected via `-ldflags "-X main.AppVersion=..."`.
- Without an injected version, build info is used: module version first, then short VCS revision, then `local`.
- Dirty VCS builds are marked with `+dirty`.
- Backend dependency versions are resolved by exact module path.

## Known Follow-ups

- Add golden or visual regression tests for generated PDFs if backend parity becomes important.
- Consider promoting page/card dimensions to CLI flags if non-ID-1 cards become a real use case.
- Consider a richer generator error model if more backend operations are added.

## Change Checklist

- Run `gofmt` on edited Go files.
- Run `go test ./...`.
- Run `go test -tags pdfcpu ./...` if any shared, layout, CLI, version, or pdfgen code changed.
- For output/layout changes, generate both fpdf and pdfcpu smoke-test PDFs.
