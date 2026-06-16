# === CONFIGURATION ===
APP_NAME := cardsheet
LIB ?= fpdf            # fpdf или pdfcpu
ARCHIVER ?= 7z
OUTPUT_DIR := dist
GO := go
LDFLAGS := -ldflags="-s -w"

# Supported platforms
PLATFORMS := windows/amd64 linux/amd64 darwin/amd64 linux/arm64 darwin/arm64

# Detect current platform
OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)
CURRENT := $(OS)_$(ARCH)

# Determine extension for current OS
EXT :=
ifeq ($(OS),windows)
	EXT := .exe
endif

# === TARGETS ===

all: build-current

build-current:
	@echo "Building for $(CURRENT) with tag $(LIB)..."
	@mkdir -p $(OUTPUT_DIR)
	@$(GO) build $(LDFLAGS) -tags $(LIB) -o $(OUTPUT_DIR)/$(APP_NAME)$(EXT)

build:
ifndef PLATFORM
	$(error PLATFORM not specified. Usage: make build PLATFORM=linux/arm64 LIB=pdfcpu)
endif
	@echo "Building for $(PLATFORM) with tag $(LIB)..."
	@mkdir -p $(OUTPUT_DIR)
	@OS=$${PLATFORM%/*}; ARCH=$${PLATFORM#*/}; \
	EXT2=""; \
	if [ "$$OS" = "windows" ]; then EXT2=".exe"; fi; \
	GOOS=$$OS GOARCH=$$ARCH $(GO) build $(LDFLAGS) -tags $(LIB) -o $(OUTPUT_DIR)/$(APP_NAME)$$EXT2

build-all:
	@echo "Building for all platforms with tag $(LIB)..."
	@mkdir -p $(OUTPUT_DIR)
	@for platform in $(PLATFORMS); do \
	    OS=$${platform%/*}; ARCH=$${platform#*/}; \
	    EXT2=""; \
	    if [ "$$OS" = "windows" ]; then EXT2=".exe"; fi; \
	    echo "→ $$OS/$$ARCH"; \
	    GOOS=$$OS GOARCH=$$ARCH $(GO) build $(LDFLAGS) -tags $(LIB) -o $(OUTPUT_DIR)/$(APP_NAME)_$$OS_$$ARCH$$EXT2; \
	done

pack: build-current
	@echo "Packing $(APP_NAME) for $(CURRENT) with tag $(LIB)..."
	@cd $(OUTPUT_DIR) && $(ARCHIVER) a $(APP_NAME)_$(CURRENT)_$(LIB).7z $(APP_NAME)$(EXT)

pack-platform:
ifndef PLATFORM
	$(error PLATFORM not specified. Usage: make pack-platform PLATFORM=linux/arm64 LIB=pdfcpu)
endif
	@echo "Packing for $(PLATFORM) with tag $(LIB)..."
	@mkdir -p $(OUTPUT_DIR)
	@OS=$${PLATFORM%/*}; ARCH=$${PLATFORM#*/}; \
	EXT2=""; \
	if [ "$$OS" = "windows" ]; then EXT2=".exe"; fi; \
	GOOS=$$OS GOARCH=$$ARCH $(GO) build $(LDFLAGS) -tags $(LIB) -o $(OUTPUT_DIR)/$(APP_NAME)$$EXT2; \
	cd $(OUTPUT_DIR) && $(ARCHIVER) a $(APP_NAME)_$$OS_$$ARCH_$(LIB).7z $(APP_NAME)$$EXT2

pack-all:
	@echo "Building and packing all platforms with tag $(LIB)..."
	@mkdir -p $(OUTPUT_DIR)
	@for platform in $(PLATFORMS); do \
	    OS=$${platform%/*}; ARCH=$${platform#*/}; \
	    EXT2=""; \
	    if [ "$$OS" = "windows" ]; then EXT2=".exe"; fi; \
	    echo "→ $$OS/$$ARCH"; \
	    GOOS=$$OS GOARCH=$$ARCH $(GO) build $(LDFLAGS) -tags $(LIB) -o $(OUTPUT_DIR)/$(APP_NAME)_$$OS_$$ARCH$$EXT2; \
	    cd $(OUTPUT_DIR) && $(ARCHIVER) a $(APP_NAME)_$$OS_$$ARCH_$(LIB).7z $(APP_NAME)_$$OS_$$ARCH$$EXT2; \
	    cd ..; \
	done

clean:
	@echo "Cleaning..."
	@rm -rf $(OUTPUT_DIR)

help:
	@echo "Usage:"
	@echo "  make LIB=fpdf                - build for current platform"
	@echo "  make build PLATFORM=os/arch LIB=pdfcpu"
	@echo "  make build-all LIB=fpdf"
	@echo "  make pack LIB=pdfcpu"
	@echo "  make pack-platform PLATFORM=os/arch LIB=fpdf"
	@echo "  make pack-all LIB=pdfcpu"
