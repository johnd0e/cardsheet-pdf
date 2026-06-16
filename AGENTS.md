# Agent Entry Point

Do not keep primary project documentation in this file. It is only a map for where to read next.

## Documentation Split

- `README.md` is user-facing documentation: purpose, layout modes, CLI usage, and build commands.
- `DEVNOTES.md` is developer-facing documentation: architecture, technical decisions, backend differences, validation, version reporting, platforms, test commands, smoke tests, and follow-ups.

## Start Here

1. Read `README.md` to understand what the tool does from a user's perspective.
2. Read `DEVNOTES.md` before changing code; it records project conventions and implementation tradeoffs.
3. For code changes, prefer starting in:
   - `internal/layout` for placement behavior.
   - `pdfgen` for backend behavior.
   - `internal/version` for version output.
   - `main.go` for CLI orchestration.
