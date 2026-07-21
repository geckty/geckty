#!/usr/bin/env bash
# Local pre-flight mirroring CI, run before tagging a release.
# Usage: scripts/pre-release-check.sh [--quick]
#   --quick  skip lint and coverage (fast local iteration)

set -euo pipefail

QUICK=false
if [[ "${1:-}" == "--quick" ]]; then
  QUICK=true
fi

GO_PACKAGES="./cmd/... ./internal/..."

step() { printf '\n==> %s\n' "$1"; }

step "Go version"
go version

step "Git status is clean"
if [[ -n "$(git status --porcelain)" ]]; then
  echo "warning: working tree has uncommitted changes — not blocking, but make sure this is intentional before tagging:" >&2
  git status --short
fi

step "gofmt"
unformatted=$(gofmt -l .)
if [[ -n "$unformatted" ]]; then
  echo "error: the following files are not gofmt-formatted:" >&2
  echo "$unformatted" >&2
  exit 1
fi

step "go vet"
go vet $GO_PACKAGES

step "go build"
go build -trimpath -ldflags="-w -s" -o /dev/null ./cmd/geckty

step "go mod verify / tidy check"
go mod verify
cp go.mod /tmp/geckty-go.mod.bak
cp go.sum /tmp/geckty-go.sum.bak
go mod tidy
if ! diff -q go.mod /tmp/geckty-go.mod.bak >/dev/null || ! diff -q go.sum /tmp/geckty-go.sum.bak >/dev/null; then
  echo "error: go.mod/go.sum are not tidy — run 'go mod tidy' and commit the result" >&2
  exit 1
fi

if [[ "$QUICK" == true ]]; then
  step "Race tests (quick mode: no coverage)"
  go test -race $GO_PACKAGES
else
  step "Race tests with coverage"
  go test -race -parallel 4 -coverprofile=coverage.txt -covermode=atomic $GO_PACKAGES

  step "golangci-lint"
  golangci-lint run --timeout=5m
fi

step "All checks passed"
echo "Ready to tag a release: git tag vX.Y.Z && git push origin vX.Y.Z"
