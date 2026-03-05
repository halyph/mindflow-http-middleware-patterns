# Running the Demo

## Quick Start (One Command)

```bash
make demo
```

That's it! The `demo` target automatically:
1. Builds binaries
2. Starts Jaeger
3. Starts mock API
4. Runs demo
5. Cleans up API when done

Open Jaeger UI: http://localhost:16686

## Makefile Targets

```bash
make build    # Compile binaries to bin/
make demo     # Build, start services, run demo (recommended)
make up       # Start observability stack only
make down     # Stop observability stack
make test     # Run tests
make clean    # Remove binaries and containers
```

## Manual Steps (Advanced)

If you prefer to run components separately:

### 1. Start Observability Stack
```bash
make up
```

### 2. Start Mock API
```bash
./bin/api
```

### 3. Run Demo
```bash
./bin/demo
```

### 4. View Traces
Open http://localhost:16686
- Service: `http-client-demo-with-middleware`
- Click "Find Traces"

## Quick Test
```bash
./TEST.sh
```

## Stopping

```bash
make down     # Stop observability stack
# Ctrl+C      # Stop API in Terminal 2
make clean    # Clean everything
```

## Troubleshooting

**Port 8081 in use:**
```bash
lsof -ti:8081 | xargs kill -9
```

**Services not starting:**
```bash
make clean
make up
```
