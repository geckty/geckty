#!/usr/bin/env bash
# Pre-release validation for geckty — mirrors CI plus a few local extras.
#
# Usage:
#   bash scripts/pre-release-check.sh          # Full check before tagging
#   bash scripts/pre-release-check.sh --quick  # Skip lint + coverage (fast iteration)
#
# On Windows with multiple Go installs, set GOROOT:
#   GOROOT="/c/Program Files/Go" bash scripts/pre-release-check.sh

set -euo pipefail

if [[ -n "${GOROOT:-}" ]]; then
  export PATH="$GOROOT/bin:$PATH"
fi

QUICK=false
if [[ "${1:-}" == "--quick" ]]; then
  QUICK=true
fi

GO_PACKAGES="./cmd/... ./internal/..."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

ERRORS=0
WARNINGS=0

echo ""
echo "================================================"
echo "  geckty — Pre-Release Check"
echo "================================================"
echo ""

# 1. Go version (must match go.mod)
log_info "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}')
REQUIRED_VERSION="go1.26"
if [[ "$GO_VERSION" < "$REQUIRED_VERSION" ]]; then
  log_error "Go $REQUIRED_VERSION+ required, found $GO_VERSION"
  ERRORS=$((ERRORS + 1))
else
  log_success "Go version: $GO_VERSION"
fi
echo ""

# 2. Git status
log_info "Checking git status..."
if git diff-index --quiet HEAD -- 2>/dev/null; then
  log_success "Working directory is clean"
else
  log_warning "Uncommitted changes detected"
  git status --short
  WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 3. gofmt (same as CI)
log_info "Checking code formatting (gofmt -l .)..."
UNFORMATTED=$(gofmt -l .)
if [[ -n "$UNFORMATTED" ]]; then
  log_error "The following files need formatting:"
  echo "$UNFORMATTED"
  log_info "Run: gofmt -w ."
  ERRORS=$((ERRORS + 1))
else
  log_success "All files are properly formatted"
fi
echo ""

# 4. go vet
log_info "Running go vet..."
if go vet $GO_PACKAGES; then
  log_success "go vet passed"
else
  log_error "go vet failed"
  ERRORS=$((ERRORS + 1))
fi
echo ""

# 5. Build
log_info "Building ./cmd/geckty..."
if go build -trimpath -ldflags="-w -s" -o /dev/null ./cmd/geckty; then
  log_success "Build successful"
else
  log_error "Build failed"
  ERRORS=$((ERRORS + 1))
fi
echo ""

# 6. go.mod verify + tidy
log_info "Validating go.mod..."
if go mod verify; then
  log_success "go.mod verified"
else
  log_error "go.mod verification failed"
  ERRORS=$((ERRORS + 1))
fi

MOD_BACKUP=$(mktemp)
SUM_BACKUP=$(mktemp)
cp go.mod "$MOD_BACKUP"
cp go.sum "$SUM_BACKUP"
go mod tidy
if diff -q go.mod "$MOD_BACKUP" >/dev/null && diff -q go.sum "$SUM_BACKUP" >/dev/null; then
  log_success "go.mod is tidy"
else
  log_error "go.mod/go.sum are not tidy — run 'go mod tidy' and commit"
  diff -u "$MOD_BACKUP" go.mod || true
  # Restore so the working tree is not left dirty by the check itself.
  cp "$MOD_BACKUP" go.mod
  cp "$SUM_BACKUP" go.sum
  ERRORS=$((ERRORS + 1))
fi
rm -f "$MOD_BACKUP" "$SUM_BACKUP"
echo ""

# 7. golangci-lint config verify
log_info "Verifying golangci-lint configuration..."
if command -v golangci-lint >/dev/null 2>&1; then
  if golangci-lint config verify; then
    log_success "golangci-lint config is valid"
  else
    log_error "golangci-lint config is invalid"
    ERRORS=$((ERRORS + 1))
  fi
else
  log_warning "golangci-lint not installed (required for a full release check)"
  log_info "Install: https://golangci-lint.run/welcome/install/"
  WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 8. Tests (+ coverage / race outside --quick)
if [[ "$QUICK" == true ]]; then
  log_info "Running tests (quick mode: no race, no coverage)..."
  if go test $GO_PACKAGES; then
    log_success "All tests passed"
  else
    log_error "Tests failed"
    ERRORS=$((ERRORS + 1))
  fi
else
  log_info "Running race tests with coverage..."
  if go test -race -parallel 4 -coverprofile=coverage.txt -covermode=atomic $GO_PACKAGES; then
    log_success "All tests passed with race detector"
  else
    log_error "Tests failed"
    ERRORS=$((ERRORS + 1))
  fi
  echo ""

  log_info "Running golangci-lint..."
  if command -v golangci-lint >/dev/null 2>&1; then
    LINT_OUTPUT=$(golangci-lint run --timeout=5m ./... 2>&1 || true)
    if [[ -z "$LINT_OUTPUT" ]] || echo "$LINT_OUTPUT" | grep -qE '(^0 issues|no issues)'; then
      log_success "golangci-lint passed with 0 issues"
    elif echo "$LINT_OUTPUT" | grep -q 'issues:'; then
      log_error "Linter found issues"
      echo "$LINT_OUTPUT"
      ERRORS=$((ERRORS + 1))
    else
      # Some versions print only file findings without a trailing "issues:" line.
      if echo "$LINT_OUTPUT" | grep -qE ':[0-9]+:[0-9]+:'; then
        log_error "Linter found issues"
        echo "$LINT_OUTPUT"
        ERRORS=$((ERRORS + 1))
      else
        log_success "golangci-lint passed"
      fi
    fi
  else
    log_error "golangci-lint not installed — cannot complete full release check"
    ERRORS=$((ERRORS + 1))
  fi
fi
echo ""

# 9. UI stack deps from GitHub (no local replace)
log_info "Checking UI stack module sources..."
check_github_mod() {
  local path=$1
  local line
  line=$(go list -m -f '{{.Path}} {{.Version}}{{if .Replace}} => {{.Replace.Path}} {{.Replace.Version}}{{end}}' "$path" 2>/dev/null || true)
  if [[ -z "$line" ]]; then
    log_warning "$path not in the module graph"
    WARNINGS=$((WARNINGS + 1))
    return
  fi
  if echo "$line" | grep -q ' => '; then
    log_error "$path uses a replace directive: $line"
    ERRORS=$((ERRORS + 1))
  else
    log_success "$line"
  fi
}
check_github_mod github.com/gogpu/gogpu
check_github_mod github.com/gogpu/gpucontext
echo ""

# 10. Critical docs
log_info "Checking documentation..."
DOCS_OK=1
for doc in README.md LICENSE CONTRIBUTING.md; do
  if [[ ! -f "$doc" ]]; then
    log_error "Missing: $doc"
    DOCS_OK=0
    ERRORS=$((ERRORS + 1))
  fi
done
if [[ $DOCS_OK -eq 1 ]]; then
  log_success "Required documentation present"
fi
echo ""

# Summary
echo "========================================"
echo "  Summary"
echo "========================================"
echo ""

if [[ $ERRORS -eq 0 && $WARNINGS -eq 0 ]]; then
  log_success "All checks passed! Ready for release."
  echo ""
  log_info "Next steps:"
  echo "  1. Commit any prep changes"
  echo "  2. Wait for CI on the release branch"
  echo "  3. git tag -a vX.Y.Z -m \"Release vX.Y.Z\""
  echo "  4. git push origin vX.Y.Z"
  echo ""
  exit 0
elif [[ $ERRORS -eq 0 ]]; then
  log_warning "Checks completed with $WARNINGS warning(s)"
  echo ""
  log_info "Review warnings above before tagging"
  echo ""
  exit 0
else
  log_error "Checks failed with $ERRORS error(s) and $WARNINGS warning(s)"
  echo ""
  log_error "Fix errors before creating a release"
  echo ""
  exit 1
fi
