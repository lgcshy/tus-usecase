#!/bin/bash

echo "=== Testing TUS Resumable Upload ==="
echo "File: very_large_test.bin (100MB)"
echo "Chunk size: 1MB (to make it slower)"
echo ""

# Clean up any existing state files
echo "Cleaning up any existing state files..."
rm -f .tusc_*.json

# Start the upload and capture its PID
echo "Starting upload..."
./tusc -t http://localhost:9508/files --chunk-size 1 --verbose upload very_large_test.bin &
UPLOAD_PID=$!

echo "Upload PID: $UPLOAD_PID"
echo "Waiting 8 seconds before interrupting..."
sleep 8

echo "Interrupting upload..."
kill $UPLOAD_PID
wait $UPLOAD_PID 2>/dev/null

echo ""
echo "Checking for state file..."
ls -la .tusc_*.json 2>/dev/null || echo "No state file found"

echo ""
echo "=== Attempting to resume upload ==="
echo "Starting the same upload again (should resume from where it left off)..."
./tusc -t http://localhost:9508/files --chunk-size 1 --verbose upload very_large_test.bin

echo ""
echo "Checking if state file was cleaned up..."
ls -la .tusc_*.json 2>/dev/null || echo "State file successfully cleaned up"
