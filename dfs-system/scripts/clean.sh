#!/bin/bash

echo "Cleaning up..."
# Kill any running go processes (Windows compatible)
taskkill //F //IM go.exe //T 2>/dev/null || true
taskkill //F //IM main.exe //T 2>/dev/null || true

# Remove binaries
rm -f *.exe
rm -rf bin/

echo "Clean complete."
