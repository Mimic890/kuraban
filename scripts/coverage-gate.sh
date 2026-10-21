#!/usr/bin/env bash
set -euo pipefail

# Usage: scripts/coverage-gate.sh <coverage.out>
#
# Enforces:
#   - Total backend coverage >= 70% (excluding backend/main.go and backend/docs/).
#   - Per-package floor >= 70% for each of:
#       internal/auth, internal/config, internal/handlers, internal/services.

if [ "$#" -ne 1 ]; then
    echo "usage: $0 <coverage.out>" >&2
    exit 2
fi

COVERAGE_IN="$1"
TOTAL_FLOOR="70.0"
PER_PKG_FLOOR="70.0"
PER_PKG_GATES=(
    "internal/auth"
    "internal/config"
    "internal/handlers"
    "internal/services"
)

if [ ! -f "$COVERAGE_IN" ]; then
    echo "error: coverage file not found: $COVERAGE_IN" >&2
    exit 2
fi

FILTERED="coverage-filtered.out"
trap 'rm -f "$FILTERED"' EXIT

# Keep the mode: line, drop excluded paths.
# Excluded from coverage totals:
#   - main.go: entry point, covered transitively by smoke.
#   - backend/docs/: generated swagger.
#   - mocks/ and *_mock.go: test infrastructure, not production code.
#   - services/s3.go: network methods deferred to smoke per parent spec
#     (docs/superpowers/specs/2026-04-17-backend-test-suite-design.md §internal/services).
grep -E '^(mode:|.*/)' "$COVERAGE_IN" \
    | grep -v '/main\.go' \
    | grep -v '/docs/' \
    | grep -v '/mocks/' \
    | grep -v '_mock\.go' \
    | grep -v '/services/s3\.go' \
    > "$FILTERED"

FUNC_OUT=$(cd backend && go tool cover -func="../$FILTERED")
TOTAL_PCT=$(echo "$FUNC_OUT" | awk '/^total:/ {gsub("%",""); print $NF}')

if [ -z "$TOTAL_PCT" ]; then
    echo "error: could not parse total coverage from go tool cover output" >&2
    echo "$FUNC_OUT" >&2
    exit 2
fi

echo "== Coverage summary =="
printf "  %-30s %s%%\n" "total" "$TOTAL_PCT"

FAIL=0
if awk -v a="$TOTAL_PCT" -v b="$TOTAL_FLOOR" 'BEGIN { exit !(a+0 < b+0) }'; then
    echo "FAIL total: ${TOTAL_PCT}% < ${TOTAL_FLOOR}%"
    FAIL=1
fi

for pkg in "${PER_PKG_GATES[@]}"; do
    pkg_pct=$(echo "$FUNC_OUT" \
        | awk -v p="/$pkg/" '$1 ~ p { sub("%","",$NF); sum+=$NF; n++ } END { if (n>0) printf "%.1f", sum/n; else print "n/a" }')
    printf "  %-30s %s%%\n" "$pkg" "$pkg_pct"
    if [ "$pkg_pct" = "n/a" ]; then
        echo "FAIL $pkg: no coverage data"
        FAIL=1
        continue
    fi
    if awk -v a="$pkg_pct" -v b="$PER_PKG_FLOOR" 'BEGIN { exit !(a+0 < b+0) }'; then
        echo "FAIL $pkg: ${pkg_pct}% < ${PER_PKG_FLOOR}%"
        FAIL=1
    fi
done

if [ "$FAIL" -ne 0 ]; then
    exit 1
fi

echo "PASS"
