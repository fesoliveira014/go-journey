#!/usr/bin/env python3
"""Compile the mdBook sources in docs/src into a single, print-ready PDF.

Pipeline
--------
1. Parse ``docs/src/SUMMARY.md`` to get the canonical chapter / section order.
2. For every Markdown file:
     * pre-render ```mermaid blocks to SVG with mermaid-cli (mmdc) + Chromium,
     * convert Markdown to HTML (footnotes scoped per file),
     * strip the duplicate leading <h1> and demote section headings,
     * inline the rendered diagrams.
3. Render the **body** (Introduction, chapters, Appendix) as its own document so
   its folios start at 1, and capture an anchor -> page-number map.
4. Render the **front matter** (O'Reilly-style cover, copyright page and an
   auto-numbered table of contents) as a second, unnumbered document, using the
   map for the TOC page numbers.
5. Merge the two with pypdf. WeasyPrint emits a chapter/section bookmark tree on
   the body, which the merge preserves as a navigable PDF outline.

WeasyPrint cannot restart the CSS ``page`` counter mid-document, which is why the
body is rendered separately rather than as one continuous file.

The mermaid step is best-effort: if a diagram fails to render, its source is
emitted as a labelled code block instead of aborting the whole build.

Usage
-----
    python build_pdf.py [--src DIR] [--out FILE] [--no-mermaid]

Environment
-----------
    MMDC                       path to the mermaid-cli binary (default: the
                               node_modules copy next to this script)
    PUPPETEER_EXECUTABLE_PATH  Chromium binary mermaid-cli should drive
"""
from __future__ import annotations

import argparse
import hashlib
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

import markdown

HERE = Path(__file__).resolve().parent
REPO_ROOT = HERE.parent.parent

BOOK_TITLE = "Building Microservices in Go"
BOOK_SUBTITLE = ("A Hands-On Guide to Cloud-Native Go with "
                 "gRPC, Kafka, Kubernetes, and AWS")
AUTHOR = "Felipe Santos"

MERMAID_CONFIG = HERE / "mermaid-config.json"
PUPPETEER_CONFIG = HERE / "puppeteer-config.json"
STYLESHEET = HERE / "styles.css"
COVER_ANIMAL = HERE / "cover-animal.svg"


# --------------------------------------------------------------------------- #
# SUMMARY parsing
# --------------------------------------------------------------------------- #

SUMMARY_LINE = re.compile(r"^(?P<indent>\s*)-\s*\[(?P<title>[^\]]+)\]\((?P<path>[^)]+)\)")


class Entry:
    __slots__ = ("title", "path", "level", "kind", "anchor", "number")

    def __init__(self, title, path, level):
        self.title = title.strip()
        self.path = path
        self.level = level
        self.anchor = ""
        self.number = None
        if level == 0 and re.match(r"^Chapter\s+\d+", self.title, re.I):
            self.kind = "chapter"
        elif level == 0:
            self.kind = "frontish"   # Introduction, Appendix, ...
        else:
            self.kind = "section"


def parse_summary(summary_path: Path) -> list[Entry]:
    entries: list[Entry] = []
    chapter_no = 0
    for raw in summary_path.read_text(encoding="utf-8").splitlines():
        m = SUMMARY_LINE.match(raw)
        if not m:
            continue
        indent = m.group("indent").replace("\t", "    ")
        level = len(indent) // 2
        rel = m.group("path").lstrip("./")
        entry = Entry(m.group("title"), rel, level)
        if entry.kind == "chapter":
            chapter_no += 1
            entry.number = chapter_no
        entries.append(entry)
    for i, e in enumerate(entries):
        e.anchor = f"entry-{i}"
    return entries


# --------------------------------------------------------------------------- #
# Mermaid -> SVG
# --------------------------------------------------------------------------- #

MERMAID_BLOCK = re.compile(r"^```mermaid[^\n]*\n(.*?)^```[ \t]*$", re.S | re.M)


def find_mmdc() -> str | None:
    env = os.environ.get("MMDC")
    if env and Path(env).exists():
        return env
    candidate = HERE / "node_modules" / ".bin" / "mmdc"
    if candidate.exists():
        return str(candidate)
    return shutil.which("mmdc")


def normalise_svg(svg: str) -> str:
    """Give the mermaid SVG concrete pixel dimensions so WeasyPrint can size it,
    and drop the inline max-width that would otherwise win over our CSS."""
    svg = re.sub(r'\sstyle="[^"]*max-width[^"]*"', "", svg, count=1)
    head = svg.split(">", 1)[0]
    if "height=" not in head:
        vb = re.search(r'viewBox="[\d.]+ [\d.]+ ([\d.]+) ([\d.]+)"', svg)
        if vb:
            w, h = float(vb.group(1)), float(vb.group(2))
            svg = re.sub(r'\swidth="[^"]*"', "", svg, count=1)
            svg = svg.replace("<svg", f'<svg width="{w:.0f}" height="{h:.0f}"', 1)
    return svg[svg.find("<svg"):].strip()


class MermaidRenderer:
    def __init__(self, mmdc, cache_dir: Path, enabled: bool = True):
        self.mmdc = mmdc
        self.cache_dir = cache_dir
        self.enabled = enabled and bool(mmdc)
        self.rendered = 0
        self.failed = 0
        cache_dir.mkdir(parents=True, exist_ok=True)

    def render(self, code: str) -> str | None:
        if not self.enabled:
            return None
        digest = hashlib.sha1(code.encode("utf-8")).hexdigest()[:16]
        out = self.cache_dir / f"{digest}.svg"
        if not out.exists():
            src = self.cache_dir / f"{digest}.mmd"
            src.write_text(code, encoding="utf-8")
            cmd = [
                self.mmdc, "-i", str(src), "-o", str(out),
                "-c", str(MERMAID_CONFIG),
                "-p", str(PUPPETEER_CONFIG),
                "-b", "transparent",
            ]
            try:
                subprocess.run(cmd, check=True, capture_output=True,
                               text=True, timeout=180)
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
                detail = (getattr(exc, "stderr", "") or "").strip()
                msg = detail.splitlines()[-1] if detail else str(exc)
                print(f"  ! mermaid render failed ({digest}): {msg}", file=sys.stderr)
                self.failed += 1
                return None
        self.rendered += 1
        return normalise_svg(out.read_text(encoding="utf-8"))


# --------------------------------------------------------------------------- #
# Markdown -> HTML
# --------------------------------------------------------------------------- #

MD_EXTENSIONS = [
    "fenced_code", "tables", "footnotes", "attr_list",
    "def_list", "sane_lists", "md_in_html", "admonition",
    "pymdownx.tilde", "pymdownx.caret", "codehilite",
]
MD_EXTENSION_CONFIGS = {
    "codehilite": {"guess_lang": False, "noclasses": False, "linenums": False},
    "footnotes": {"BACKLINK_TEXT": "&#8617;"},
}

LEADING_H1 = re.compile(r"<h1[^>]*>.*?</h1>", re.S)


def demote_headings(html: str, by: int) -> str:
    if by <= 0:
        return html
    for level in range(5, 0, -1):
        new = min(level + by, 6)
        html = re.sub(rf"<(/?)h{level}([ >])", rf"<\1h{new}\2", html)
    return html


def scope_footnotes(html: str, slug: str) -> str:
    html = html.replace("fnref:", f"fnref:{slug}-")
    html = html.replace("fn:", f"fn:{slug}-")
    return html


def render_markdown_file(path: Path, slug: str, mermaid: MermaidRenderer) -> str:
    text = path.read_text(encoding="utf-8")

    diagrams: list[str] = []

    def stash(match: re.Match) -> str:
        idx = len(diagrams)
        diagrams.append(match.group(1))
        return f"\n\nMERMAIDTOKEN{idx}ENDTOKEN\n\n"

    text = MERMAID_BLOCK.sub(stash, text)

    md = markdown.Markdown(extensions=MD_EXTENSIONS,
                           extension_configs=MD_EXTENSION_CONFIGS)
    html = md.convert(text)

    def restore(match: re.Match) -> str:
        idx = int(match.group(1))
        code = diagrams[idx]
        svg = mermaid.render(code)
        if svg:
            return f'<figure class="diagram">{svg}</figure>'
        escaped = code.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
        return (f'<figure class="diagram"><pre class="mermaid-src">'
                f'<code>{escaped}</code></pre></figure>')

    html = re.sub(r"<p>MERMAIDTOKEN(\d+)ENDTOKEN</p>", restore, html)
    return scope_footnotes(html, slug)


# --------------------------------------------------------------------------- #
# HTML fragments
# --------------------------------------------------------------------------- #

def esc(text: str) -> str:
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def page_shell(body_html: str, pygments_css: str) -> str:
    base_css = STYLESHEET.read_text(encoding="utf-8")
    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{esc(BOOK_TITLE)}</title>
<style>
{base_css}
{pygments_css}
figure.diagram svg {{ max-width: 100%; height: auto; }}
</style>
</head>
<body>
{body_html}
</body>
</html>
"""


def build_cover() -> str:
    animal = COVER_ANIMAL.read_text(encoding="utf-8")
    animal = animal[animal.find("<svg"):]
    return f"""
<section class="cover">
  <div class="band-top"></div>
  <div class="series">Cloud-Native Engineering &middot; The Field Guide</div>
  <div class="animal">
    {animal}
    <div class="caption">The Pocket Gopher</div>
  </div>
  <div class="title-block">
    <h1 class="title">{esc(BOOK_TITLE)}</h1>
    <p class="subtitle">{esc(BOOK_SUBTITLE)}</p>
  </div>
  <div class="author">{esc(AUTHOR)}</div>
  <div class="band-bottom"></div>
</section>
"""


def build_frontmatter(year: str) -> str:
    return f"""
<section class="frontmatter">
  <div class="copyright">
    <h1>{esc(BOOK_TITLE)}</h1>
    <p class="meta"><em>{esc(BOOK_SUBTITLE)}</em></p>
    <p class="meta">by {esc(AUTHOR)}</p>
    <p>&nbsp;</p>
    <p>Copyright &copy; {year} {esc(AUTHOR)}. All rights reserved.</p>
    <p>This book is a learning project: a complete, microservices-based library
       management system built in Go, taken from first principles through to a
       production deployment on AWS. It is written as a hands-on field guide for
       experienced engineers who are new to Go and cloud-native tooling.</p>
    <p>The cover illustration is an original engraving-style drawing of a pocket
       gopher, a nod to Go's mascot. This title is an independent work and is not
       affiliated with or endorsed by O'Reilly Media.</p>
    <p>Generated from the Markdown sources in <code>docs/src</code>. The latest
       version is available at
       <a href="https://github.com/fesoliveira014/go-journey">github.com/fesoliveira014/go-journey</a>.</p>
    <p>&nbsp;</p>
    <p class="meta">Revision: {year} edition &middot; Built with WeasyPrint.</p>
  </div>
</section>
"""


def build_toc(entries: list[Entry], page_map: dict[str, int]) -> str:
    items = []
    for e in entries:
        cls = "toc-chapter" if e.kind in ("chapter", "frontish") else "toc-section"
        page = page_map.get(e.anchor, "")
        items.append(
            f'<li class="{cls}"><a href="#{e.anchor}" data-page="{page}">'
            f'{esc(e.title)}</a></li>'
        )
    return ('<section class="toc"><h1>Table of Contents</h1><ul>'
            + "\n".join(items) + "</ul></section>")


def build_body(entries: list[Entry], src_dir: Path, mermaid: MermaidRenderer) -> str:
    parts = [f'<span class="title-anchor">{esc(BOOK_TITLE)}</span>',
             '<div class="main-matter">']
    for e in entries:
        md_path = src_dir / e.path
        if not md_path.exists():
            print(f"  ! missing source: {e.path}", file=sys.stderr)
            continue
        slug = re.sub(r"[^a-z0-9]+", "-", e.path.lower())
        html = render_markdown_file(md_path, slug, mermaid)
        html = LEADING_H1.sub("", html, count=1)   # drop the duplicate title

        if e.kind == "chapter":
            html = demote_headings(html, 1)
            number = f"Chapter {e.number}"
            title = re.sub(r"^Chapter\s+\d+\s*[:.—-]\s*", "", e.title)
            head = (f'<div class="chapter-head">'
                    f'<div class="chapter-number">{esc(number)}</div>'
                    f'<h1 class="chapter-title" id="{e.anchor}">{esc(title)}</h1>'
                    f'</div>')
            parts.append(f'<section class="chapter">{head}{html}</section>')
        elif e.kind == "frontish":
            head = f'<h1 class="chapter-title" id="{e.anchor}">{esc(e.title)}</h1>'
            parts.append(f'<section class="frontish">{head}{html}</section>')
        else:  # section
            html = demote_headings(html, 1)
            head = f'<h2 id="{e.anchor}">{esc(e.title)}</h2>'
            parts.append(f'<section class="section">{head}{html}</section>')
    parts.append("</div>")
    return "\n".join(parts)


# --------------------------------------------------------------------------- #
# Rendering / merge
# --------------------------------------------------------------------------- #

def anchors_to_pages(document) -> dict[str, int]:
    page_map: dict[str, int] = {}
    for i, page in enumerate(document.pages, start=1):
        for name in page.anchors:
            page_map.setdefault(name, i)
    return page_map


def main() -> int:
    ap = argparse.ArgumentParser(description="Build the book PDF.")
    ap.add_argument("--src", default=str(REPO_ROOT / "docs" / "src"))
    ap.add_argument("--out", default=str(REPO_ROOT / "dist" /
                    "building-microservices-in-go.pdf"))
    ap.add_argument("--year", default=os.environ.get("BOOK_YEAR", "2026"))
    ap.add_argument("--no-mermaid", action="store_true",
                    help="Skip diagram rendering (emit mermaid source as code).")
    ap.add_argument("--cache-dir", default=None,
                    help="Where to cache rendered diagram SVGs.")
    args = ap.parse_args()

    src_dir = Path(args.src).resolve()
    summary = src_dir / "SUMMARY.md"
    if not summary.exists():
        print(f"error: {summary} not found", file=sys.stderr)
        return 1

    import logging
    logging.getLogger("pypdf").setLevel(logging.ERROR)  # mute benign merge notes
    logging.getLogger("fontTools").setLevel(logging.ERROR)

    from weasyprint import HTML
    from pygments.formatters import HtmlFormatter
    from pypdf import PdfWriter

    entries = parse_summary(summary)
    n_chapters = sum(e.kind == "chapter" for e in entries)
    print(f"Parsed {len(entries)} entries from SUMMARY.md ({n_chapters} chapters).")

    mmdc = find_mmdc()
    if not args.no_mermaid and not mmdc:
        print("  ! mermaid-cli (mmdc) not found; diagrams will be rendered as "
              "source. Run `npm install` in scripts/book-pdf to enable them.",
              file=sys.stderr)

    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    cache = Path(args.cache_dir) if args.cache_dir else out.parent / ".diagram-cache"
    mermaid = MermaidRenderer(mmdc, cache, enabled=not args.no_mermaid)
    pygments_css = HtmlFormatter().get_style_defs(".codehilite")

    # --- Pass 1: body (folios start at 1) -------------------------------- #
    print("Rendering diagrams and converting Markdown...")
    body_html = build_body(entries, src_dir, mermaid)
    if mermaid.enabled:
        print(f"Diagrams: {mermaid.rendered} rendered, {mermaid.failed} failed.")
    print("Rendering body PDF with WeasyPrint...")
    body_doc = HTML(string=page_shell(body_html, pygments_css),
                    base_url=str(src_dir)).render()
    page_map = anchors_to_pages(body_doc)
    body_pdf = body_doc.write_pdf()

    # --- Pass 2: front matter (cover + copyright + TOC) ------------------ #
    print("Rendering front matter and table of contents...")
    front_html = (build_cover() + build_frontmatter(args.year)
                  + build_toc(entries, page_map))
    front_pdf = HTML(string=page_shell(front_html, pygments_css),
                     base_url=str(src_dir)).write_pdf()

    # --- Merge ----------------------------------------------------------- #
    print("Merging front matter and body...")
    writer = PdfWriter()
    writer.append(stream_from_bytes(front_pdf))
    writer.append(stream_from_bytes(body_pdf))
    writer.add_metadata({"/Title": BOOK_TITLE, "/Author": AUTHOR})
    with open(out, "wb") as fh:
        writer.write(fh)

    from pypdf import PdfReader
    pages = len(PdfReader(out).pages)
    size_mb = out.stat().st_size / (1024 * 1024)
    print(f"✓ Wrote {out} ({pages} pages, {size_mb:.1f} MB)")
    return 0


def stream_from_bytes(data: bytes):
    import io
    return io.BytesIO(data)


if __name__ == "__main__":
    sys.exit(main())
