#!/usr/bin/env bash
# Build the browser bot (WebAssembly) and stage the static site.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "building docs/bot.wasm ..."
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o docs/bot.wasm ./cmd/botwasm

# wasm_exec.js ships with the Go toolchain; its location varies by version.
if [ -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ]; then
  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" docs/wasm_exec.js
else
  cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" docs/wasm_exec.js
fi
echo "done: docs/bot.wasm + docs/wasm_exec.js"