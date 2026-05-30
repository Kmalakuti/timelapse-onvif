#!/bin/sh
# Ensure Go dependencies are resolved before running any command
# This handles the case where go.sum is empty on the mounted volume
if [ ! -s go.sum ]; then
    echo "Resolving Go dependencies..."
    go mod tidy 2>/dev/null
fi
exec "$@"
