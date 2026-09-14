.PHONY: all build install uninstall clean test

# Go executable
GO ?= go
BINARY_NAME = base92
INSTALL_PATH ?= /usr/local/bin

# Build settings
BUILD_DIR = .
LDFLAGS = -ldflags "-s -w"

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete! You can now use '$(BINARY_NAME)' command."

uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstallation complete."

clean:
	@echo "Cleaning up..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Clean complete."

test: build
	@set -eu; \
	work_dir=$$(mktemp -d); \
	trap 'rm -rf "$$work_dir"' EXIT; \
	printf 'Hello, World!\n' > "$$work_dir/input"; \
	$(BUILD_DIR)/$(BINARY_NAME) encode "$$work_dir/input" > "$$work_dir/encoded"; \
	$(BUILD_DIR)/$(BINARY_NAME) decode "$$work_dir/encoded" > "$$work_dir/decoded"; \
	diff "$$work_dir/input" "$$work_dir/decoded"; \
	echo "Test passed!"
