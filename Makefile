APP_NAME := wetterabhaengig
APP_ID := com.vitovt.wetterabhaengig
MAIN_PKG := ./cmd/wetterabhaengig
BUILD_DIR := build

.DEFAULT_GOAL := help

.PHONY: help prepare deps linux windows mac android clean

help:
	@echo "Available targets:"
	@echo "  make help     - Show this help output (default)."
	@echo "  make prepare  - Create local build folders."
	@echo "  make deps     - Sync Go dependencies."
	@echo "  make linux    - Build Linux binary."
	@echo "  make windows  - Build Windows binary."
	@echo "  make mac      - Build macOS binary."
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
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/linux/$(APP_NAME) $(MAIN_PKG)

windows: prepare
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/windows/$(APP_NAME).exe $(MAIN_PKG)

mac: prepare
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/mac/$(APP_NAME) $(MAIN_PKG)

android: prepare
	@test -f cmd/wetterabhaengig/appicon.png || (echo "Missing icon: cmd/wetterabhaengig/appicon.png" && exit 1)
	@cd $(BUILD_DIR)/android && gogio -target android -appid $(APP_ID) ../../cmd/wetterabhaengig

clean:
	rm -rf $(BUILD_DIR)
