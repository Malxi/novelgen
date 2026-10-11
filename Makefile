BINARY_NAME := novelgen.exe
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(BINARY_NAME)

.PHONY: build test test-all vet webui-build webui-test
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY)

# Module tests (the webui lives in its own Go module, see go.work).
test:
	go test ./...

webui-build:
	cd webui && go build ./...

webui-test:
	cd webui && go test ./...

# Run every module's tests in one shot (root module + webui module).
test-all: test webui-test

vet:
	go vet ./...
	cd webui && go vet ./...
