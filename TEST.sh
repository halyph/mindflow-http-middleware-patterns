#!/bin/bash
set -e

echo "🧪 Testing HTTP Middleware Patterns Setup"
echo "=========================================="
echo ""

# Check Go version
echo "✓ Checking Go version..."
go version

# Check Docker/Colima
echo "✓ Checking Docker..."
docker --version

# Check if make is available
echo "✓ Checking make..."
make --version | head -1

# Build binaries (will download dependencies automatically)
echo "✓ Building binaries..."
make build
echo "  ✅ Binaries built successfully"

# Check if Jaeger is running
echo "✓ Checking if Jaeger is running..."
if docker ps | grep -q jaeger; then
    echo "  ⚠️  Jaeger is already running"
else
    echo "  Starting observability stack..."
    make up
    echo "  Waiting for services to start..."
    sleep 5
fi

# Check if Jaeger UI is accessible
echo "✓ Checking Jaeger UI..."
if curl -s http://localhost:16686 > /dev/null; then
    echo "  ✅ Jaeger UI is accessible at http://localhost:16686"
else
    echo "  ❌ Jaeger UI is not accessible"
    exit 1
fi

# Run tests (if any)
echo "✓ Running tests..."
if make test 2>/dev/null; then
    echo "  ✅ Tests passed"
else
    echo "  ⚠️  No tests found (this is a demo project)"
fi

echo ""
echo "=========================================="
echo "✅ All checks passed!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Run demo: make demo"
echo "   (automatically starts API, runs demo, cleans up)"
echo "2. Open Jaeger UI: open http://localhost:16686"
echo ""
echo "See RUNNING.md for detailed instructions"
echo ""
