BIN     := gogeoip
CMD     := ./cmd/gogeoip
IMAGE   := gogeoip
TAG     := latest

.PHONY: all build run test lint fmt vet clean \
        docker-build docker-run docker-stop

# ── Local ────────────────────────────────────────────────────────────────────

all: build

## build: compile the binary
build:
	go build -o $(BIN) $(CMD)

## run: build then run
run: build
	./$(BIN)

## test: run all tests
test:
	go test ./... --cover

## fmt: format source files
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (requires golangci-lint in PATH)
lint:
	golangci-lint run ./...

## clean: remove build artefacts
clean:
	rm -f $(BIN)

# ── Docker ───────────────────────────────────────────────────────────────────

## docker-build: build the Docker image
docker-build:
	docker build -t $(IMAGE):$(TAG) .

## docker-run: run the container
docker-run:
	docker run --rm -p 8080:8080 \
		--name $(BIN) \
		$(IMAGE):$(TAG)

## docker-stop: stop the running container
docker-stop:
	docker stop $(BIN) 2>/dev/null || true

# ── Help ─────────────────────────────────────────────────────────────────────

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
