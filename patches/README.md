# fpdf XObject Metadata Patch

## Problem

The default `pdfcpu` backend stores source filenames directly on each image
XObject using:

```pdf
/CardsheetSourceFilename (front.png)
```

That location is useful because it travels with the image object and is visible
to both extraction and PDF-input code. The upstream `fpdf` API can register and
place images, but it does not expose a hook for adding custom entries to the
image XObject dictionary before the PDF is written.

The previous fpdf experiment used a document-level trailing manifest. That kept
the PDF write path simple, but it created a backend-specific metadata format.
The default build could not read those names from fpdf-generated PDFs because
they were not stored on image XObjects.

## Patch Shape

`fpdf-xobject-metadata.patch` adds a small extension to fpdf image handling:

- `ImageInfoType.extraDict map[string]string`
  - stores additional image XObject dictionary string entries on the registered
    image;
  - is internal to fpdf, matching the existing private fields on
    `ImageInfoType`.
- `(*Fpdf).SetImageDictionaryString(imageStr, key, value string)`
  - public setter for a registered image;
  - `imageStr` is the same key already used by `RegisterImage`,
    `RegisterImageOptions`, and `Image`;
  - `key` is passed without a leading slash;
  - `value` is emitted as an escaped PDF literal string.
- `keySortStringMap`
  - gives deterministic output for map-backed extra dictionary entries.
- `pdfLiteralString`
  - escapes backslash, parentheses, and common control characters for PDF string
    syntax.
- `putimage`
  - emits sorted `extraDict` entries before `/Length`.

For cardsheet, the call site is:

```go
pdf.RegisterImage(path, "")
pdf.SetImageDictionaryString(path, "CardsheetSourceFilename", filepath.Base(sourceName))
pdf.Image(path, x, y, w, h, false, "", 0, "")
```

The PDF is still written once by fpdf through `OutputFileAndClose`. There is no
post-write PDF rewriting step.

## API Fit

The patch follows fpdf's existing image lifecycle:

1. Register an image and store its parsed image state in `f.images`.
2. Mutate registered image metadata before output.
3. Let `putimage` serialize the final image XObject dictionary.

This keeps custom dictionary metadata attached to the same registered image
object that fpdf already deduplicates and serializes. It does not change image
placement APIs, stream bytes, object numbering, or page content generation.

The setter is intentionally narrow. It only supports PDF string dictionary
entries because cardsheet only needs a basename string. It does not attempt to
be a generic raw-PDF injection API.

## Applying

Run from the repository root:

```powershell
pwsh scripts/apply-fpdf-patch.ps1
```

The script copies `codeberg.org/go-pdf/fpdf@v0.12.0` from the local Go module
cache into the ignored `internal/fpdfpatch/` directory and applies the patch by
context. `go.mod` then uses:

```go
replace codeberg.org/go-pdf/fpdf => ./internal/fpdfpatch
```

The generated `internal/fpdfpatch/` directory is intentionally ignored so the
repository stores only the small patch and script, not a full vendored fpdf
copy.
