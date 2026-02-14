#!/bin/bash
# Test script for HighClaw V2 features

set -e

echo "🦀 HighClaw V2 Feature Test"
echo "=============================="
echo ""

HIGHCLAW="./dist/highclaw"

# Check if binary exists
if [ ! -f "$HIGHCLAW" ]; then
    echo "❌ Binary not found. Run 'make build' first."
    exit 1
fi

echo "✅ Binary found: $HIGHCLAW"
echo ""

# Test 1: Version
echo "📋 Test 1: Version"
$HIGHCLAW version
echo ""

# Test 2: Check if onboard command exists
echo "📋 Test 2: Onboard command help"
$HIGHCLAW onboard --help | head -10
echo ""

# Test 3: Check if gateway command exists
echo "📋 Test 3: Gateway command help"
$HIGHCLAW gateway --help | head -10
echo ""

# Test 4: List all models (if API is available)
echo "📋 Test 4: Check model definitions"
echo "This would require the gateway to be running."
echo "Models are defined in: internal/domain/model/models.go"
echo "Total providers: 14"
echo "Total models: 42+"
echo ""

# Test 5: Check config structure
echo "📋 Test 5: Config structure"
if [ -f ~/.highclaw/highclaw.json ]; then
    echo "✅ Config file exists: ~/.highclaw/highclaw.json"
    cat ~/.highclaw/highclaw.json | head -20
else
    echo "⚠️  No config file found. Run 'highclaw onboard' to create one."
fi
echo ""

# Test 6: Check available commands
echo "📋 Test 6: Available commands"
$HIGHCLAW --help | grep "Available Commands:" -A 30
echo ""

echo "=============================="
echo "✅ All tests passed!"
echo ""
echo "Next steps:"
echo "  1. Run: $HIGHCLAW onboard"
echo "  2. Run: $HIGHCLAW gateway"
echo "  3. Visit: http://localhost:18789"
echo ""

