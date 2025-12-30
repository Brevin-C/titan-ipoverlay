# Titan IP Overlay Root Makefile

.PHONY: all client ippop manager benchmark clean help

# Default target
all: build-mac

# --- Mac Build Targets ---
build-mac: client-mac ippop-mac manager-mac benchmark-mac

client-mac:
	@echo "Building Client (Mac)..."
	cd client && go build -o ../bin/client-mac main.go
	@echo "✓ Client built: bin/client-mac"

ippop-mac:
	@echo "Building IP POP (Mac)..."
	cd ippop && go build -o ../bin/ippop-mac main.go
	@echo "✓ IP POP built: bin/ippop-mac"

manager-mac:
	@echo "Building Manager (Mac)..."
	cd manager && go build -o ../bin/manager-mac server.go
	@echo "✓ Manager built: bin/manager-mac"

benchmark-mac:
	@echo "Building Benchmark (Mac)..."
	cd benchmark && make build-mac
	@cp benchmark/bin/benchmark-mac bin/benchmark-mac
	@echo "✓ Benchmark built: bin/benchmark-mac"

# --- Linux Build Targets ---
build-linux: client-linux ippop-linux manager-linux benchmark-linux

client-linux:
	@echo "Building Client (Linux)..."
	cd client && GOOS=linux GOARCH=amd64 go build -o ../bin/client-linux main.go
	@echo "✓ Client built: bin/client-linux"

ippop-linux:
	@echo "Building IP POP (Linux)..."
	cd ippop && GOOS=linux GOARCH=amd64 go build -o ../bin/ippop-linux main.go
	@echo "✓ IP POP built: bin/ippop-linux"

manager-linux:
	@echo "Building Manager (Linux)..."
	cd manager && GOOS=linux GOARCH=amd64 go build -o ../bin/manager-linux server.go
	@echo "✓ Manager built: bin/manager-linux"

benchmark-linux:
	@echo "Building Benchmark (Linux)..."
	cd benchmark && make build-linux
	@cp benchmark/bin/benchmark-linux bin/benchmark-linux
	@echo "✓ Benchmark built: bin/benchmark-linux"

# --- Common Targets ---
clean:
	@echo "Cleaning binaries..."
	rm -rf bin/
	cd benchmark && make clean
	@echo "✓ Cleaned"

deps:
	@echo "Installing root dependencies..."
	go mod download
	go mod tidy
	@echo "Installing benchmark dependencies..."
	cd benchmark && make deps

work:
	@echo "Setting up Go workspace..."
	go work init .
	go work use ./benchmark
	@echo "✓ go.work created"

help:
	@echo "Available commands:"
	@echo "  make help           - Show this help"
	@echo "  make build-mac      - Build everything for Mac"
	@echo "  make build-linux    - Build everything for Linux"
	@echo "  make client-mac     - Build only client for Mac"
	@echo "  make benchmark-mac  - Build only benchmark for Mac"
	@echo "  make clean          - Remove all binaries"
	@echo "  make work           - Initialize Go workspace"
