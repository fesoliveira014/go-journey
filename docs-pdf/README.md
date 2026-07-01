# docs-pdf — book PDF build

Assets for compiling the mdBook in [`../docs`](../docs) into a single,
book-quality PDF (cover, page-numbered table of contents, LaTeX typography).

The [`Docs PDF`](../.github/workflows/docs-pdf.yml) workflow drives this; the
files here let you reproduce it locally.

## Why not `mdbook-pdf`?

`mdbook-pdf` prints the HTML through headless Chrome — it cannot produce a real
title page, a page-numbered ToC, or book typography. This pipeline uses
`mdbook-pandoc` → LaTeX (Tectonic) instead. The tradeoff: LaTeX can't run
JavaScript, so Mermaid diagrams are pre-rendered to vector PDFs first.

## Files

| file | purpose |
|------|---------|
| `book.pandoc.toml`     | mdbook config with the `[output.pandoc]` PDF profile. Copied in as `book.toml` for the build; keeps the committed `docs/book.toml` HTML-only. |
| `header.tex`           | LaTeX preamble — code line-wrapping, image scaling, paragraph spacing. |
| `prerender_mermaid.py` | Extracts ```` ```mermaid ```` blocks and renders each to a vector PDF with `mmdc`. |

## Build locally

Requires `mdbook`, `mdbook-pandoc`, `pandoc` (>= 2.10), `tectonic`,
`@mermaid-js/mermaid-cli` (`mmdc`), a Chrome/Chromium, and the
`DejaVu Sans Mono` font.

```bash
BUILD=$(mktemp -d)/book
cp -r docs "$BUILD"
cp docs-pdf/book.pandoc.toml "$BUILD/book.toml"
cp docs-pdf/header.tex "$BUILD/header.tex"

# Mermaid -> vector PDFs (point mmdc at a real Chrome; --no-sandbox in CI)
export PUPPETEER_EXECUTABLE_PATH=/usr/bin/google-chrome
python3 docs-pdf/prerender_mermaid.py "$BUILD/src"

mdbook build "$BUILD"
# -> $BUILD/site/pdf/output.pdf
```
