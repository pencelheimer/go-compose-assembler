# Build the assembler binary
build:
    go build -o bin/assembler ./cmd/assembler

# Run the assembler
run *args:
    go run ./cmd/assembler {{args}}

# Run all tests with race detector
test:
    go test -race ./...

# Check code with go vet
vet:
    go vet ./...
alias lint := vet

# Format Go source files
fmt:
    gofmt -s -w .

# Clean backend build artifacts
clean:
    rm -rf bin/
