#!/usr/bin/env bash
# Memory profiling helpers for geckty (heap from go test -memprofile).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== unit mem tests ==="
go test ./internal/ui/raster/ ./internal/ui/termview/ -run 'TestLoadFontBundleHeap|TestMemLeak|TestFontSource' -count=1 -v

echo ""
echo "=== heap profile (paint loop) ==="
prof=$(mktemp)
go test ./internal/ui/termview/ -bench=BenchmarkPaintGrid80x24 -memprofile="$prof" -benchtime=1s -count=1 -run=^$
echo "top alloc sites:"
go tool pprof -top -nodecount=20 "$prof" 2>/dev/null | head -25
