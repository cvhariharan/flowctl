#!/bin/bash
# Test script for remote options functionality

set -e

echo "Running remote options tests..."
echo ""

# Run Go unit tests for the options package
echo "1. Running Go unit tests for options utility..."
go test -v ./internal/utils -run TestInterpolateVariables -timeout 10s
echo "✓ Variable interpolation tests passed"
echo ""

go test -v ./internal/utils -run TestFetchRemoteOptions -timeout 10s
echo "✓ Remote options fetching tests passed"
echo ""

go test -v ./internal/utils -run TestFetchRemoteOptionsWithVariables -timeout 10s
echo "✓ Variable interpolation with remote fetch tests passed"
echo ""

# Run all options tests
echo "2. Running all options tests..."
go test -v ./internal/utils -timeout 30s
echo "✓ All options tests passed"
echo ""

echo "================================"
echo "All tests passed successfully! ✓"
echo "================================"
echo ""
echo "To test the integration with a live flow:"
echo "1. Start the flowctl server"
echo "2. Create a flow with select inputs using options_url"
echo "3. Set up a mock API serve using 'node test-api-server.js'"
echo "4. Trigger the flow and observe the remote options being fetched"
