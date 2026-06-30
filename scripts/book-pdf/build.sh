#!/usr/bin/env bash
#
# Build the book PDF (cover + front matter + TOC + chapters) from docs/src.
#
# It provisions an isolated Python virtualenv and the mermaid-cli toolchain on
# first run, then renders the PDF. Re-runs reuse both, and diagrams are cached.
#
# Usage:
#   scripts/book-pdf/build.sh                       # -> dist/building-microservices-in-go.pdf
#   scripts/book-pdf/build.sh --out /tmp/book.pdf   # custom output path
#   scripts/book-pdf/build.sh --no-mermaid          # skip diagram rendering
#
# Requirements:
#   - python3 (with venv) and pip
#   - node + npm (for mermaid-cli)
#   - a Chromium/Chrome for mermaid-cli. If you already have one, point
#     PUPPETEER_EXECUTABLE_PATH at it; otherwise mermaid-cli downloads its own.
#   - WeasyPrint's native deps: libpango-1.0, libpangocairo, libgdk-pixbuf,
#     libffi, libcairo (preinstalled on most desktops; see the workflow for the
#     Debian/Ubuntu package list).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

VENV_DIR="${BOOK_PDF_VENV:-$SCRIPT_DIR/.venv}"

# --- Python venv ---------------------------------------------------------- #
if [[ ! -x "$VENV_DIR/bin/python" ]]; then
    echo "==> Creating virtualenv at $VENV_DIR"
    python3 -m venv "$VENV_DIR"
fi
echo "==> Installing Python dependencies"
"$VENV_DIR/bin/pip" install --quiet --upgrade pip
"$VENV_DIR/bin/pip" install --quiet -r requirements.txt pypdf

# --- mermaid-cli ---------------------------------------------------------- #
if [[ ! -x "$SCRIPT_DIR/node_modules/.bin/mmdc" ]]; then
    echo "==> Installing mermaid-cli (npm)"
    npm install --no-fund --no-audit
fi

# --- Locate a Chromium for mermaid-cli ------------------------------------ #
if [[ -z "${PUPPETEER_EXECUTABLE_PATH:-}" ]]; then
    for c in \
        /opt/pw-browsers/chromium-*/chrome-linux/chrome \
        "$(command -v chromium 2>/dev/null || true)" \
        "$(command -v chromium-browser 2>/dev/null || true)" \
        "$(command -v google-chrome 2>/dev/null || true)"; do
        if [[ -n "$c" && -x "$c" ]]; then
            export PUPPETEER_EXECUTABLE_PATH="$c"
            break
        fi
    done
fi
if [[ -n "${PUPPETEER_EXECUTABLE_PATH:-}" ]]; then
    echo "==> Using Chromium at $PUPPETEER_EXECUTABLE_PATH"
fi

# --- Build ---------------------------------------------------------------- #
echo "==> Building PDF"
exec "$VENV_DIR/bin/python" build_pdf.py "$@"
