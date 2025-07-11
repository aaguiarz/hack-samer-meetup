#!/bin/bash

# Auth0 to OpenFGA Event Processor CLI Demo
# This script demonstrates the CLI capabilities

set -e

echo "🚀 Auth0 to OpenFGA Event Processor CLI Demo"
echo "============================================="
echo ""

# Build the CLI if it doesn't exist
if [ ! -f "bin/event-processor" ]; then
    echo "📦 Building event processor CLI..."
    go build -o bin/event-processor cmd/event-processor/main.go
    echo "✅ Build complete!"
    echo ""
fi

echo "📊 Available sample event files:"
ls -la examples/*.json | awk '{print "   " $9 " (" $5 " bytes)"}'
echo ""

echo "🔍 Demo 1: Basic event processing with dry-run mode"
echo "---------------------------------------------------"
echo "Processing simple events with verbose output..."
echo ""
./bin/event-processor -events examples/sample-events.json -dry-run -verbose
echo ""

echo "🔍 Demo 2: Complex event processing with summary output"
echo "------------------------------------------------------"
echo "Processing complex events with summary output..."
echo ""
./bin/event-processor -events examples/complex-events.json -dry-run
echo ""

echo "📋 Demo 3: Show CLI help information"
echo "------------------------------------"
./bin/event-processor -help
echo ""

echo "🎉 Demo complete!"
echo ""
echo "💡 Next steps:"
echo "   1. Create your own events JSON file"
echo "   2. Set up a real OpenFGA store with -store-id"
echo "   3. Remove -dry-run flag to make real changes"
echo "   4. Use -verbose flag for detailed tuple operations"
echo ""
echo "📚 Documentation:"
echo "   - CLI Documentation: README-cli.md"
echo "   - Webhook Service: README-webhook.md"
echo "   - Library Examples: examples/complete_example.go"
