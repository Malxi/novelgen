BINARY_NAME := nolvegen.exe
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(BINARY_NAME)

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY)
