# Upstream PR Plan for fpdf XObject Metadata

## Goal

Turn the local cardsheet patch into a small, safe, generally useful upstream
`fpdf` feature: let callers attach custom string metadata to registered image
XObjects before the PDF is written.

The PR should not be framed as a cardsheet-specific change. The general problem
is that `fpdf` can create image XObjects but cannot annotate their dictionaries
for downstream PDF workflows.

## Proposed Upstream API

Prefer a narrow string-only API rather than raw PDF dictionary injection.

Candidate API:

```go
func (f *Fpdf) SetImageXObjectString(imageName, key, value string)
```

or, if maintainers prefer metadata to hang from the registered image:

```go
func (info *ImageInfoType) SetXObjectString(key, value string)
```

The `Fpdf` method is closer to the current local patch and avoids exposing more
of `ImageInfoType` internals. The `ImageInfoType` method is ergonomically nice
because `RegisterImage...` already returns `*ImageInfoType`.

## Required Hardening Before PR

- Validate dictionary keys:
  - reject empty keys;
  - reject leading `/`;
  - reject whitespace, delimiters, and control characters;
  - either allow only conservative PDF name characters or implement proper PDF
    name escaping.
- Keep values string-only:
  - escape `\`, `(`, `)`, newline, carriage return, and tab;
  - do not expose a raw-PDF value API in the first PR.
- Fix image deduplication semantics:
  - `fpdf` deduplicates images by internal image hash;
  - if two identical image streams have different XObject metadata, they must
    not silently collapse into one object with one metadata value;
  - preferred fix: include sorted extra dictionary string entries in image
    identity/hash, or disable deduplication for images with distinct metadata.
- Keep deterministic output:
  - sort extra dictionary keys before writing them.

## Upstream Test Cases

- A registered image emits a custom XObject string entry.
- PDF string values are escaped correctly for parentheses, backslash, and common
  control characters.
- Invalid dictionary keys fail through normal `fpdf` error handling.
- Multiple metadata keys are emitted in deterministic sorted order.
- Identical image bytes with different metadata produce correct metadata per
  logical image, not silent metadata loss through deduplication.

## PR Pitch

Suggested framing:

> This adds a narrow extension point for image XObject string metadata. It lets
> applications attach safe string entries to registered image dictionaries before
> output. This is useful for source tracking, asset identifiers, extraction
> roundtrips, and interoperability with downstream PDF tooling. The API is
> intentionally string-only to avoid raw PDF injection.

Mention cardsheet only as an example consumer, not as the reason for the API.

## Local Patch Status

The current local patch is good enough for the cardsheet experiment, but it is
not upstream-ready until key validation and deduplication semantics are fixed.
