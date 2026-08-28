#!/usr/bin/env bash
# CPU + heap profiling for geckty hot paths (terminal paint / glyph raster).
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${1:-/tmp/geckty-profile}"
mkdir -p "$OUT"

echo "=== benchmarks (throughput) ==="
go test ./internal/ui/termview/ -bench='BenchmarkPaintGrid' -benchmem -count=1 -run=^$ -parallel=1

echo ""
echo "=== CPU profile (80x24 paint, 3s) ==="
CPU="$OUT/cpu-paint.prof"
go test ./internal/ui/termview/ -bench=BenchmarkPaintGrid80x24 -cpuprofile="$CPU" -benchtime=3s -count=1 -run=^$
echo "--- top flat ---"
go tool pprof -top -nodecount=25 "$CPU" 2>/dev/null | head -30
echo "--- top cumulative ---"
go tool pprof -top -cum -nodecount=15 "$CPU" 2>/dev/null | head -20

echo ""
echo "=== heap profile (paint loop) ==="
MEM="$OUT/mem-paint.prof"
go test ./internal/ui/termview/ -bench=BenchmarkPaintGrid80x24 -memprofile="$MEM" -benchtime=1s -count=1 -run=^$
go tool pprof -top -nodecount=20 "$MEM" 2>/dev/null | head -25

echo ""
echo "Profiles written to $OUT/"
echo "Interactive: go tool pprof -http=:8080 $CPU"
