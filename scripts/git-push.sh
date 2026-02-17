#!/bin/bash
# Push to remote repository

cd "$(dirname "$0")/.."

echo "📤 Pushing to remote repository..."
echo ""

git push -u origin main

echo ""
echo "✅ Push complete!"
echo "🔗 Repository: https://github.com/903174293/highclaw"

