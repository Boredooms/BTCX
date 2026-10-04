# BCTX Makefile — Linux-native build/test/offline verification.

BINARY      := bctx
CMD         := ./apps/bctx
BIN_DIR     := bin
VERSION     ?= 0.1.0-dev
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# CGO is required for the SQLite driver.
export CGO_ENABLED := 1

.PHONY: all build run test vet fmt tidy clean test-offline doctor

all: vet test build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)
	@echo "built $(BIN_DIR)/$(BINARY)"

run: build
	$(BIN_DIR)/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR) dist

# Offline proof: run the analysis-adjacent commands with networking disabled.
# `unshare -n` drops the network namespace so any hidden external call fails.
test-offline: build
	@echo "== Offline verification (no network namespace) =="
	BCTX_HOME=$$(mktemp -d) ; \
	unshare -rn $(BIN_DIR)/$(BINARY) --airgap init ; \
	unshare -rn $(BIN_DIR)/$(BINARY) --airgap case create offline-check ; \
	unshare -rn $(BIN_DIR)/$(BINARY) --airgap doctor ; \
	echo "offline checks completed"

doctor: build
	$(BIN_DIR)/$(BINARY) doctor
