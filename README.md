# cardsheet-pdf - Printable A4 PDF Sheets for Card Images

`cardsheet` is a command-line tool designed specifically for preparing printable PDF sheets from images of standard-sized cards.
It is intended for workflows such as:

- ID cards
- membership cards
- access badges
- collectible cards
- any other card‑like images with consistent dimensions

The tool arranges card images onto an **A4 portrait** page using a clean, predictable layout.

---

## Purpose

The project solves a simple but common problem:  
**take a set of card images (typically JPEGs) and place them onto an A4 sheet in a clean, predictable layout suitable for printing.**

The tool assumes:

- Input images are **JPEG or PNG files**
- Images are already in the **correct orientation**
- The output page format is **A4 portrait** (210 × 297 mm)
- Cards are placed in a **grid layout** with configurable spacing

Card dimensions are fixed to real‑world [ID‑1](https://en.wikipedia.org/wiki/ISO/IEC_7810) card size:

- **Width:** 85.6 mm  
- **Height:** 53.98 mm  

Images are scaled to these physical dimensions regardless of pixel resolution.

---

## Layout Modes

### Default layout (stacked vertically)

- Cards are placed **top → bottom**
- When vertical space runs out, a **new column** begins
- Default gaps: vertical alternates **5/10 mm**, horizontal **10 mm**

### Side‑by‑Side Mode

- Always uses **two columns**
- Cards are placed **left → right**, then next row
- Default gaps: horizontal **5 mm**, vertical **10 mm**

---

## Usage

Basic usage:

```
cardsheet -out output.pdf image1.jpg image2.jpg
```


### Options

| Option | Description |
|--------|-------------|
| `-out <file>` | Output PDF file name |
| `-gap <mm>` | Horizontal spacing |
| `-vgap <mm>` | Vertical spacing |
| `-dpi <dpi>` | Limit embedded image resolution to this effective DPI |
| `-verbose` | Show image DPI and resize information |
| `-side-by-side` | Force side-by-side layout |
| `-version` | Show version and backend |

### Example

```
cardsheet -out cards.pdf -side-by-side card1.jpg card2.jpg
```


This produces an A4 PDF with two cards per row.

---

## Python Version

A Python implementation is also available for quick runs without building the Go binary:

```sh
uv run cardsheet.py -out cards.pdf card1.jpg card2.jpg
```

It supports the same core layout, DPI, verbose, side-by-side, and version options as the Go CLI.

---

## Build

Build the default Go CLI:

```sh
go build
```

The Go implementation supports multiple PDF backends. Backend-specific build tags and behavior notes are documented in [DEVNOTES.md](DEVNOTES.md).


