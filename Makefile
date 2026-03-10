# bmcinv - BMC Inventory Tool
# Installation targets for Linux systems

BINARY_NAME=bmcinv
INSTALL_PATH=/usr/local/bin
MAN_PATH=/usr/local/share/man/man1
GO=go

.PHONY: all build install uninstall clean test install-user install-completion

all: build

# Build the binary
build:
	$(GO) build -o $(BINARY_NAME) .

# Install globally (requires sudo)
install: build
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	sudo chmod 755 $(INSTALL_PATH)/$(BINARY_NAME)
	sudo mkdir -p $(MAN_PATH)
	sudo cp man/bmcinv.1 $(MAN_PATH)/bmcinv.1
	sudo gzip -f $(MAN_PATH)/bmcinv.1
	@echo "✓ Installed $(BINARY_NAME) to $(INSTALL_PATH)"
	@echo "✓ Installed man page (run 'man bmcinv')"
	@echo "  Run 'bmcinv --help' to get started"

# Remove from system
uninstall:
	sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	sudo rm -f $(MAN_PATH)/bmcinv.1.gz
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
	mkdir -p $(HOME)/.local/share/man/man1
	cp $(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	cp man/bmcinv.1 $(HOME)/.local/share/man/man1/bmcinv.1
	chmod 755 $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "✓ Installed to ~/.local/bin/$(BINARY_NAME)"
	@echo "✓ Installed man page to ~/.local/share/man/man1/"
	@echo "  Make sure ~/.local/bin is in your PATH"

# Install shell completion
install-completion:
	@echo "Installing bash completion..."
	sudo mkdir -p /etc/bash_completion.d
	$(INSTALL_PATH)/$(BINARY_NAME) completion bash | sudo tee /etc/bash_completion.d/bmcinv > /dev/null
	@echo "✓ Bash completion installed. Restart your shell or run:"
	@echo "  source /etc/bash_completion.d/bmcinv"
