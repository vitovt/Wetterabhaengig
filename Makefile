APP_NAME := wetterabhaengig
APP_ID := com.vitovt.wetterabhaengig
MAIN_PKG := ./cmd/wetterabhaengig
BUILD_DIR := build
GOGIO ?= gogio
GOGIO_RUNNER := $(CURDIR)/scripts/gogio-android.sh
ANDROID_MIN_SDK := 30
ANDROID_TARGET_SDK := 34
ANDROID_HOME_FALLBACK := $(or $(ANDROID_HOME),$(ANDROID_SDK_ROOT),$(HOME)/Android/Sdk)

.DEFAULT_GOAL := help

.PHONY: help prepare deps linux windows mac android clean

help:
	@echo "Available targets:"
	@echo "  make help     - Show this help output (default)."
	@echo "  make prepare  - Create local build folders."
	@echo "  make deps     - Sync Go dependencies."
	@echo "  make linux    - Build Linux binary."
	@echo "  make windows  - Build Windows binary."
	@echo "  make mac      - Build macOS binary (run on macOS host)."
	@echo "  make android  - Build Android package with gogio."
	@echo "  make clean    - Remove build artifacts."

prepare:
	@mkdir -p $(BUILD_DIR)/linux
	@mkdir -p $(BUILD_DIR)/windows
	@mkdir -p $(BUILD_DIR)/mac
	@mkdir -p $(BUILD_DIR)/android

deps:
	go mod tidy

linux: prepare
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o $(BUILD_DIR)/linux/$(APP_NAME) $(MAIN_PKG)

windows: prepare
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/windows/$(APP_NAME).exe $(MAIN_PKG)

mac: prepare
	@if [ "$$(go env GOHOSTOS)" = "darwin" ]; then \
		GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o $(BUILD_DIR)/mac/$(APP_NAME) $(MAIN_PKG); \
	else \
		echo "mac target requires a macOS host (or configured osxcross toolchain)."; \
		echo "Run this target on macOS for reliable Gio desktop builds."; \
		exit 1; \
	fi

android: prepare
	@test -x $(GOGIO_RUNNER) || (echo "Missing Android gogio wrapper: $(GOGIO_RUNNER)" && exit 1)
	@test -f cmd/wetterabhaengig/appicon.png || (echo "Missing icon: cmd/wetterabhaengig/appicon.png" && exit 1)
	@ANDROID_HOME="$(ANDROID_HOME_FALLBACK)"; \
	if [ ! -d "$$ANDROID_HOME" ]; then \
		echo "Android SDK path not found. Set ANDROID_HOME or ANDROID_SDK_ROOT."; \
		exit 1; \
	fi; \
	export ANDROID_HOME; \
	cd $(BUILD_DIR)/android && $(GOGIO_RUNNER) "$(GOGIO)" "$(ANDROID_MIN_SDK)" "$(ANDROID_TARGET_SDK)" "$(APP_ID)" ../../cmd/wetterabhaengig

clean:
	rm -rf $(BUILD_DIR)
