# bmcinv - BMC Inventory Tool
# Installation targets for Linux systems

BINARY_NAME=bmcinv
INSTALL_PATH=/usr/local/bin
GO=go

.PHONY: all build install uninstall clean test

all: build

# Build the binary
build:
	$(GO) build -o $(BINARY_NAME) .

# Install globally (requires sudo)
install: build
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	sudo chmod 755 $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Installed $(BINARY_NAME) to $(INSTALL_PATH)"
	@echo "  Run 'bmcinv --help' to get started"

# Remove from system
uninstall:
	sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Uninstalled $(BINARY_NAME)"

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	$(GO) clean

# Run tests
test:
	$(GO) test -v ./...

# Install for current user only (no sudo)
install-user: build
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	chmod 755 $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "✓ Installed to ~/.local/bin/$(BINARY_NAME)"
	@echo "  Make sure ~/.local/bin is in your PATH"
