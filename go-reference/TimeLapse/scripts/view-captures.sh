#!/bin/bash
# Script to view captured images

CAPTURE_DIR="/data/captures"

echo "╔═══════════════════════════════════════════════════════╗"
echo "║           TimeLapse - View Captures                  ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

if [ ! -d "$CAPTURE_DIR" ]; then
    echo "❌ Capture directory not found: $CAPTURE_DIR"
    exit 1
fi

echo "📁 Capture Directory: $CAPTURE_DIR"
echo ""

# Count total files
total=$(find "$CAPTURE_DIR" -name "*.jpg" 2>/dev/null | wc -l)
echo "📊 Total captures: $total"
echo ""

if [ "$total" -eq 0 ]; then
    echo "ℹ️  No captures found yet. Wait for the server to capture some images."
    exit 0
fi

# Show recent captures
echo "📸 Recent captures (last 10):"
echo "----------------------------------------"
find "$CAPTURE_DIR" -name "*.jpg" -type f | sort | tail -10 | while read file; do
    filename=$(basename "$file")
    size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)
    echo "  $filename ($size bytes)"
done

echo ""
echo "✓ To view all captures: ls -lh $CAPTURE_DIR"
