#!/bin/bash
# Fix port configuration and kill old processes

set -e

echo "🔧 HighClaw Port Fix Script"
echo "============================"
echo ""

CONFIG_FILE="$HOME/.highclaw/highclaw.json"

# Step 1: Check if config file exists
if [ -f "$CONFIG_FILE" ]; then
    echo "✅ Found config file: $CONFIG_FILE"
    
    # Check current port
    CURRENT_PORT=$(cat "$CONFIG_FILE" | jq -r '.gateway.port' 2>/dev/null || echo "unknown")
    echo "   Current port: $CURRENT_PORT"
    
    if [ "$CURRENT_PORT" != "18790" ]; then
        echo "   ⚠️  Port is not 18790, updating..."
        
        # Backup
        cp "$CONFIG_FILE" "$CONFIG_FILE.backup"
        echo "   ✓ Backup created: $CONFIG_FILE.backup"
        
        # Update port
        cat "$CONFIG_FILE" | jq '.gateway.port = 18790' > /tmp/highclaw.json
        mv /tmp/highclaw.json "$CONFIG_FILE"
        echo "   ✓ Port updated to 18790"
    else
        echo "   ✓ Port is already 18790"
    fi
else
    echo "ℹ️  No config file found (will use default 18790)"
fi

echo ""

# Step 2: Kill old highclaw processes
echo "🔍 Checking for old highclaw processes..."
OLD_PIDS=$(ps aux | grep "[h]ighclaw gateway" | awk '{print $2}')

if [ -n "$OLD_PIDS" ]; then
    echo "   Found old processes: $OLD_PIDS"
    for PID in $OLD_PIDS; do
        echo "   Killing PID $PID..."
        kill $PID 2>/dev/null || true
    done
    sleep 1
    echo "   ✓ Old processes killed"
else
    echo "   ✓ No old processes found"
fi

echo ""

# Step 3: Check port availability
echo "🔍 Checking port availability..."
if lsof -i :18790 > /dev/null 2>&1; then
    echo "   ⚠️  Port 18790 is in use:"
    lsof -i :18790
    echo ""
    echo "   You may need to kill the process manually:"
    echo "   kill <PID>"
else
    echo "   ✓ Port 18790 is available"
fi

echo ""

# Step 4: Show OpenClaw status
echo "🔍 Checking OpenClaw (port 18789)..."
if lsof -i :18789 > /dev/null 2>&1; then
    echo "   ✓ OpenClaw is running on 18789"
else
    echo "   ℹ️  OpenClaw is not running"
fi

echo ""
echo "============================"
echo "✅ Port fix complete!"
echo ""
echo "Next steps:"
echo "  1. Start HighClaw:  cd highclaw && ./dist/highclaw gateway"
echo "  2. Verify:          lsof -i :18790"
echo "  3. Open Web UI:     open http://localhost:18790"
echo ""

