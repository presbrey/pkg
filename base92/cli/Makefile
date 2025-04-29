.PHONY: all build install clean test

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
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete."

test:
	@echo "Testing encode/decode functionality..."
	@echo "Hello, World!" > /tmp/test_base92.txt
	@$(BUILD_DIR)/$(BINARY_NAME) encode /tmp/test_base92.txt > /tmp/test_base92_encoded.txt
	@$(BUILD_DIR)/$(BINARY_NAME) decode /tmp/test_base92_encoded.txt > /tmp/test_base92_decoded.txt
	@diff /tmp/test_base92.txt /tmp/test_base92_decoded.txt && echo "Test passed!" || echo "Test failed!"
	@rm /tmp/test_base92.txt /tmp/test_base92_encoded.txt /tmp/test_base92_decoded.txt
