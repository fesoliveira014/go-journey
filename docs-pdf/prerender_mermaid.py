#!/usr/bin/env python3
"""Pre-render Mermaid diagrams for the LaTeX/pandoc PDF build.

mdbook renders ```mermaid blocks client-side with JavaScript. A LaTeX pipeline
cannot run JS, so this script walks the mdbook `src` tree, extracts every
```mermaid fenced block to a `.mmd` file under `src/_diagrams/`, renders each to
a vector PDF with mermaid-cli (`mmdc`), and rewrites the fenced block into an
image reference the LaTeX writer can `\\includegraphics`.

Usage:
    python3 prerender_mermaid.py <mdbook-src-dir>

Environment:
    MMDC_BIN                path to mmdc (default: "mmdc" on PATH)
    PUPPETEER_CONFIG_FILE   optional puppeteer config passed to mmdc via -p
                            (use to set {"args":["--no-sandbox"]} in CI)
    PUPPETEER_EXECUTABLE_PATH   Chrome/Chromium binary mmdc should drive
"""
import hashlib
import os
import re
import subprocess
import sys

fence = re.compile(r"```mermaid[ \t]*\n(.*?)\n```", re.S)


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: prerender_mermaid.py <mdbook-src-dir>", file=sys.stderr)
        return 2
    src = sys.argv[1]
    img_dir = os.path.join(src, "_diagrams")
    os.makedirs(img_dir, exist_ok=True)

    hashes: list[str] = []

    for root, _, files in os.walk(src):
        for fn in files:
            if not fn.endswith(".md"):
                continue
            path = os.path.join(root, fn)
            text = open(path, encoding="utf-8").read()
            if "```mermaid" not in text:
                continue
            rel = os.path.relpath(img_dir, os.path.dirname(path))

            def repl(m: "re.Match[str]") -> str:
                code = m.group(1)
                h = hashlib.sha1(code.encode()).hexdigest()[:10]
                open(os.path.join(img_dir, f"{h}.mmd"), "w", encoding="utf-8").write(code)
                hashes.append(h)
                return f"![]({rel}/{h}.pdf)"

            open(path, "w", encoding="utf-8").write(fence.sub(repl, text))

    if not hashes:
        print("no mermaid blocks found")
        return 0

    mmdc = os.environ.get("MMDC_BIN", "mmdc")
    puppeteer_cfg = os.environ.get("PUPPETEER_CONFIG_FILE")
    for h in hashes:
        cmd = [mmdc, "-i", os.path.join(img_dir, f"{h}.mmd"),
               "-o", os.path.join(img_dir, f"{h}.pdf"), "--pdfFit"]
        if puppeteer_cfg:
            cmd[1:1] = ["-p", puppeteer_cfg]
        subprocess.run(cmd, check=True)

    print(f"pre-rendered {len(hashes)} mermaid diagrams -> {img_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
