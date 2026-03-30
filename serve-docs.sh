#!/usr/bin/env bash
set -euo pipefail

# Build and serve the mdBook documentation site.
# Usage:
#   ./serve-docs.sh          # build and serve on localhost:3000
#   ./serve-docs.sh build    # build only (output to site/)
#   ./serve-docs.sh --port 4000  # serve on a custom port

PORT=3000
BUILD_ONLY=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        build)
            BUILD_ONLY=true
            shift
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        *)
            echo "Usage: $0 [build] [--port PORT]"
            exit 1
            ;;
    esac
done

# Ensure mdbook is installed
if ! command -v mdbook &>/dev/null; then
    echo "mdbook not found. Installing via cargo..."
    if ! command -v cargo &>/dev/null; then
        echo "Error: cargo is required to install mdbook. Install Rust first: https://rustup.rs"
        exit 1
    fi
    cargo install mdbook
fi

cd "$(dirname "$0")/docs"

if $BUILD_ONLY; then
    mdbook build
    echo "Site built to $(cd .. && pwd)/site/"
else
    echo "Serving docs at http://localhost:${PORT}"
    mdbook serve --port "$PORT"
fi
