#!/usr/bin/env bash
# Memory profiling helpers for geckty (gg glyph spike + leak checks).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== unit mem tests ==="
go test ./internal/ui/raster/ ./internal/ui/termview/ -run 'TestGGGlyph|TestLoadFontBundleHeap|TestMemLeak|TestFontSource' -count=1 -v

echo ""
echo "=== heap profile (termview paint loop) ==="
prof=$(mktemp)
go test ./internal/ui/termview/ -run TestMemLeakGGGlyphPaintLoop -memprofile="$prof" -count=1
echo "top alloc sites:"
go tool pprof -top -nodecount=20 "$prof" 2>/dev/null | head -25

echo ""
echo "=== live app profiling ==="
echo "Terminal 1: GECKTY_PPROF=localhost:6060 GECKTY_GG_GLYPHMASK=1 go run ./cmd/geckty"
echo "Terminal 2: go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap"
echo "           go tool pprof -http=:8081 http://localhost:6060/debug/pprof/allocs"
