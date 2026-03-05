.PHONY: build test clean demo up down

# Directories and binaries
BIN_DIR := bin
DEMO_BIN := $(BIN_DIR)/demo
API_BIN := $(BIN_DIR)/api
API_PID := /tmp/http-middleware-api.pid

# Build binaries
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(DEMO_BIN) cmd/demo/main.go
	go build -o $(API_BIN) cmd/external-api/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts and services
clean:
	rm -rf $(BIN_DIR)
	docker-compose down -v
	@rm -f $(API_PID)

# Start observability stack (Jaeger/Grafana/etc)
up:
	docker-compose up -d

# Stop observability stack
down:
	docker-compose down

# Run demo (starts API automatically, then runs demo)
demo: build up
	@echo "Starting API..."
	# Run API in background, discard output, save PID to file
	# $(API_BIN) > /dev/null 2>&1 & - runs API in background, discards stdout/stderr
	# echo $$! > $(API_PID) - saves PID of last background process to file
	@$(API_BIN) > /dev/null 2>&1 & echo $$! > $(API_PID)
	@sleep 2

	@echo "Running demo..."
	@$(DEMO_BIN)
	
	@echo "Stopping API..."
	@kill $$(cat $(API_PID)) 2>/dev/null || true
	@rm -f $(API_PID)
