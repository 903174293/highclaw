#!/bin/bash
# Verify that default port has been changed from 18789 to 18790

set -e

echo "🔍 Verifying Port Change (18789 → 18790)"
echo "=========================================="
echo ""

HIGHCLAW="./dist/highclaw"
PASS=0
FAIL=0

# Test 1: Check if binary exists
echo "✅ Test 1: Binary exists"
if [ -f "$HIGHCLAW" ]; then
    echo "   ✓ Binary found"
    ((PASS++))
else
    echo "   ✗ Binary not found"
    ((FAIL++))
fi
echo ""

# Test 2: Check code files for 18790
echo "✅ Test 2: Code files use 18790"
CODE_FILES="internal/config/config.go internal/cli/onboard.go internal/cli/onboard_v3.go internal/cli/onboard_helpers.go"
if grep -q "18790" $CODE_FILES; then
    echo "   ✓ Found 18790 in code files"
    ((PASS++))
else
    echo "   ✗ 18790 not found in code files"
    ((FAIL++))
fi
echo ""

# Test 3: Check that 18789 is NOT in code files
echo "✅ Test 3: Code files do NOT use 18789"
if ! grep -q "18789" $CODE_FILES 2>/dev/null; then
    echo "   ✓ No 18789 found in code files"
    ((PASS++))
else
    echo "   ⚠️  Found 18789 in code files (should be 18790)"
    grep -n "18789" $CODE_FILES || true
    ((FAIL++))
fi
echo ""

# Test 4: Check README
echo "✅ Test 4: README uses 18790"
if grep -q "18790" README.md; then
    echo "   ✓ README updated"
    ((PASS++))
else
    echo "   ✗ README not updated"
    ((FAIL++))
fi
echo ""

# Test 5: Check documentation files
echo "✅ Test 5: Documentation files use 18790"
DOC_FILES="IMPLEMENTATION_STATUS.md ONBOARD_COMPARISON.md QUICKSTART.md DEMO.md"
if grep -q "18790" $DOC_FILES; then
    echo "   ✓ Documentation updated"
    ((PASS++))
else
    echo "   ✗ Documentation not updated"
    ((FAIL++))
fi
echo ""

# Test 6: Count occurrences
echo "✅ Test 6: Port occurrence count"
COUNT_18790=$(grep -r "18790" internal/ README.md *.md 2>/dev/null | wc -l | tr -d ' ')
COUNT_18789=$(grep -r "18789" internal/ README.md *.md 2>/dev/null | wc -l | tr -d ' ')
echo "   18790 occurrences: $COUNT_18790"
echo "   18789 occurrences: $COUNT_18789"
if [ "$COUNT_18790" -gt 0 ] && [ "$COUNT_18789" -eq 0 ]; then
    echo "   ✓ Port change complete"
    ((PASS++))
else
    echo "   ⚠️  Port change may be incomplete"
    ((FAIL++))
fi
echo ""

# Summary
echo "=========================================="
echo "Test Summary:"
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
    echo "✅ All tests passed! Port change verified."
    echo ""
    echo "Default port is now: 18790"
    echo "  - OpenClaw: 18789"
    echo "  - HighClaw: 18790"
    echo ""
    echo "You can now run both simultaneously!"
    exit 0
else
    echo "❌ Some tests failed. Please review."
    exit 1
fi

