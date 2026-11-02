BINARY_NAME := novelgen.exe
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(BINARY_NAME)

.PHONY: build test test-all test-race vet fmt-check webui-build webui-test lint
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY)

# Module tests (the webui lives in its own Go module, see go.work).
test:
	go test ./...

webui-build:
	cd webui && go build ./...

webui-test:
	cd webui && go list ./... | grep -v /node_modules/ | xargs go test

# Run every module's tests in one shot (root module + webui module).
test-all: test webui-test

# Concurrency-sensitive packages get the race detector in CI; this target is
# the local equivalent.
test-race:
	go test -race ./...

vet:
	go vet ./...
	cd webui && go vet ./...

# Same gate as the CI gofmt job.
fmt-check:
	@unformatted="$$(git ls-files '*.go' | xargs gofmt -l)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-formatted:"; echo "$$unformatted"; exit 1; \
	fi

lint: vet fmt-check
