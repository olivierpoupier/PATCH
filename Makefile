BINARY_NAME := patch
HARALTD_VERSION := v0.0.2
HARALTD_APP := /Applications/Haraltd.app

# Detect OS and architecture
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

.PHONY: build run clean deps setup check-haraltd start-haraltd stop-haraltd

APP_BUNDLE := $(BINARY_NAME).app

## build: Compile the binary (macOS: .app bundle for Location Services)
build:
ifeq ($(UNAME_S),Darwin)
	go build -o $(BINARY_NAME) .
	@mkdir -p $(APP_BUNDLE)/Contents/MacOS
	@cp $(BINARY_NAME) $(APP_BUNDLE)/Contents/MacOS/$(BINARY_NAME)
	@cp Info.plist $(APP_BUNDLE)/Contents/Info.plist
	codesign -f -s - $(APP_BUNDLE)
else
	go build -o $(BINARY_NAME) .
endif

## run: Build and run the app (handles platform setup automatically)
run: build
ifeq ($(UNAME_S),Linux)
	@echo "Running on Linux (BlueZ)..."
	./$(BINARY_NAME)
else ifeq ($(UNAME_S),Darwin)
	@$(MAKE) --no-print-directory ensure-haraltd
	./$(APP_BUNDLE)/Contents/MacOS/$(BINARY_NAME)
else
	@echo "Unsupported platform: $(UNAME_S)"
	@exit 1
endif

## deps: Install Go dependencies
deps:
	go mod tidy

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(APP_BUNDLE)

# --- macOS haraltd management ---

## setup: Install haraltd daemon (macOS only, required once)
setup:
ifeq ($(UNAME_S),Darwin)
ifeq ($(UNAME_M),arm64)
	$(eval HARALTD_ASSET := haraltd-$(HARALTD_VERSION)-osx-arm64.zip)
else
	$(eval HARALTD_ASSET := haraltd-$(HARALTD_VERSION)-osx-x64.zip)
endif
	@echo "Downloading haraltd $(HARALTD_VERSION)..."
	@gh release download $(HARALTD_VERSION) --repo bluetuith-org/haraltd --pattern '$(HARALTD_ASSET)' --output /tmp/haraltd.zip --clobber
	@echo "Installing to /Applications..."
	@unzip -o /tmp/haraltd.zip -d /Applications/
	@xattr -dr com.apple.quarantine $(HARALTD_APP)
	@rm /tmp/haraltd.zip
	@echo "Installed. On first launch, grant Bluetooth and Full Disk Access when prompted."
else ifeq ($(UNAME_S),Linux)
	@echo "No extra setup needed on Linux. Make sure BlueZ is installed:"
	@echo "  sudo apt install bluez     # Debian/Ubuntu"
	@echo "  sudo pacman -S bluez       # Arch"
	@echo "  sudo dnf install bluez     # Fedora"
endif

## check-haraltd: Check if haraltd is installed (macOS)
check-haraltd:
ifeq ($(UNAME_S),Darwin)
	@test -d $(HARALTD_APP) || (echo "haraltd not installed. Run 'make setup' first." && exit 1)
endif

## start-haraltd: Start the haraltd daemon (macOS)
start-haraltd: check-haraltd
ifeq ($(UNAME_S),Darwin)
	@if ! pgrep -f haraltd > /dev/null 2>&1; then \
		echo "Starting haraltd..."; \
		open $(HARALTD_APP); \
		sleep 2; \
	else \
		echo "haraltd already running."; \
	fi
endif

## stop-haraltd: Stop the haraltd daemon (macOS)
stop-haraltd:
ifeq ($(UNAME_S),Darwin)
	@if pgrep -f haraltd > /dev/null 2>&1; then \
		echo "Stopping haraltd..."; \
		pkill -f haraltd || true; \
	else \
		echo "haraltd not running."; \
	fi
endif

# Internal: ensure haraltd is installed and running before launching on macOS
ensure-haraltd: check-haraltd start-haraltd

## help: Show this help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
