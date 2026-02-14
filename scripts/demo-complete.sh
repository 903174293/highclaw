#!/bin/bash
# Complete demo of HighClaw with all phases

set -e

echo "🦀 HighClaw Complete Demo"
echo "=========================="
echo ""

HIGHCLAW="./dist/highclaw"

# Check if binary exists
if [ ! -f "$HIGHCLAW" ]; then
    echo "Building HighClaw..."
    make build
fi

echo "✅ HighClaw built successfully"
echo ""

# Show version
echo "📋 Version:"
$HIGHCLAW version
echo ""

# Show all phases completed
echo "📊 All Phases Completed:"
echo "  ✅ Phase 1: Provider System (20+ providers, 100+ models)"
echo "  ✅ Phase 2: Channels System (19+ channels, 4 implementations)"
echo "  ✅ Phase 3: Skills System (8+ skills, dependency management)"
echo ""

# Show onboard help
echo "📋 Onboard V3 Features:"
echo "  ✅ Provider filtering (All providers or specific)"
echo "  ✅ Model selection (100+ models or manual entry)"
echo "  ✅ Keep current model option"
echo "  ✅ 19+ channel selection"
echo "  ✅ Skills configuration"
echo "  ✅ Dependency installation (npm/pnpm/bun)"
echo "  ✅ API key configuration"
echo ""

# Show available commands
echo "📋 Available Commands:"
$HIGHCLAW --help | grep "Available Commands:" -A 30 | head -20
echo ""

echo "=========================="
echo "✅ Demo Complete!"
echo ""
echo "Next steps:"
echo "  1. Run onboarding:  $HIGHCLAW onboard-v3"
echo "  2. Start gateway:   $HIGHCLAW gateway"
echo "  3. Open Web UI:     http://localhost:18789"
echo ""
echo "Documentation:"
echo "  - ALL_PHASES_COMPLETE.md - Complete summary"
echo "  - PHASE1_COMPLETE.md - Provider system"
echo "  - PHASE2_COMPLETE.md - Channels system"
echo "  - PHASE3_COMPLETE.md - Skills system"
echo ""

