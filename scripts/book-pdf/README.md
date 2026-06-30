# Book PDF builder

Compiles the mdBook sources in [`docs/src`](../../docs/src) into a single,
print-ready PDF — an O'Reilly-style cover, a copyright/front-matter page, a
table of contents with real page numbers, and every chapter with its mermaid
diagrams rendered as vector art.

The PDF is a **build artifact**: it is written to `dist/` and is never
committed. Only the scripts here are tracked.

## Quick start

```bash
scripts/book-pdf/build.sh
# -> dist/building-microservices-in-go.pdf
```

The wrapper creates a Python virtualenv, installs the toolchain on first run,
then renders the PDF. Re-runs reuse the venv and a per-diagram SVG cache, so
they are much faster.

Options are forwarded to the Python script:

```bash
scripts/book-pdf/build.sh --out /tmp/book.pdf   # custom output path
scripts/book-pdf/build.sh --no-mermaid          # skip diagram rendering
scripts/book-pdf/build.sh --year 2027           # copyright year
```

## How it works

WeasyPrint cannot restart the CSS `page` counter mid-document, so the build is
two passes that are merged with `pypdf`:

1. **Body** (Introduction → chapters → Appendix) is rendered on its own, so its
   folios naturally start at `1`. The renderer records an *anchor → page* map.
2. **Front matter** (cover, copyright, TOC) is rendered second, unnumbered. The
   TOC page numbers come from the map captured in pass 1.
3. The two PDFs are concatenated. WeasyPrint emits a chapter/section bookmark
   tree on the body, which survives the merge as a navigable PDF outline.

```
SUMMARY.md ─┐
docs/src/** ─┼─▶ build_pdf.py ─┬─▶ body.pdf  (folios 1..N, outline)
mermaid ─────┘   (mmdc+Chromium)└─▶ front.pdf (cover, ©, TOC) ─▶ merge ─▶ dist/*.pdf
```

Mermaid blocks are pre-rendered to SVG with
[`@mermaid-js/mermaid-cli`](https://github.com/mermaid-js/mermaid-cli) driving a
headless Chromium (`htmlLabels: false`, so the SVG uses `<text>` that WeasyPrint
can paint). Rendering is best-effort: a diagram that fails to render falls back
to its source as a code block rather than aborting the build.

## Files

| File | Purpose |
|------|---------|
| `build.sh` | Provisions deps and runs the build. Start here. |
| `build_pdf.py` | The pipeline: SUMMARY parsing, Markdown→HTML, two-pass render, merge. |
| `styles.css` | Print stylesheet (cover, paged-media layout, TOC, code, tables). |
| `cover-animal.svg` | The engraving-style gopher on the cover. Edit to taste. |
| `mermaid-config.json` | Mermaid theme + `htmlLabels:false` for WeasyPrint compatibility. |
| `puppeteer-config.json` | Chromium flags (`--no-sandbox`) for mermaid-cli. |
| `requirements.txt` / `package.json` | Python and Node dependencies. |

## Requirements

- **Python 3.10+** with `venv`
- **Node 18+** with `npm`
- A **Chromium/Chrome** for mermaid-cli. Set `PUPPETEER_EXECUTABLE_PATH` to use
  an existing one; otherwise mermaid-cli downloads its own on `npm install`.
- **WeasyPrint native libraries** — Pango, cairo, gdk-pixbuf, libffi. On
  Debian/Ubuntu:

  ```bash
  sudo apt-get install -y libpango-1.0-0 libpangoft2-1.0-0 libpangocairo-1.0-0 \
       libgdk-pixbuf-2.0-0 libcairo2 libffi8 shared-mime-info \
       fonts-dejavu fonts-liberation2
  ```

## CI

[`.github/workflows/book-pdf.yml`](../../.github/workflows/book-pdf.yml) builds
the PDF on changes to `docs/**` or this directory, uploads it as a workflow
artifact, and attaches it to the GitHub Release on `v*` tags.
