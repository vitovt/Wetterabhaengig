APP_NAME ?= wetterabhaengig
APP_ID ?= com.vitovt.$(APP_NAME)
MAIN_PKG ?= ./cmd/$(APP_NAME)

BUILD_DIR ?= build
DIST_DIR ?= dist

GOGIO ?= gogio
GOGIO_RUNNER ?= $(CURDIR)/scripts/gogio-android.sh
ANDROID_ICON_PATH ?= cmd/$(APP_NAME)/appicon.png
ANDROID_NOTIFICATION_ICON_NAME ?= ic_stat_$(APP_NAME)
ANDROID_MIN_SDK ?= 30
ANDROID_TARGET_SDK ?= 34
ANDROID_HOME_FALLBACK := $(or $(ANDROID_HOME),$(ANDROID_SDK_ROOT),$(HOME)/Android/Sdk)

LINUX_GOARCH ?= amd64
WINDOWS_GOARCH ?= amd64
MAC_GOARCH ?= amd64

LINUX_CGO_ENABLED ?= 1
WINDOWS_CGO_ENABLED ?= 0
MAC_CGO_ENABLED ?= 1

ARTIFACT_TARGETS ?= linux windows android

PROJECT_TUNE_HINT ?= Update APP_NAME first; APP_ID, MAIN_PKG and ANDROID_ICON_PATH already derive from it unless your project layout differs.
LINUX_HOST_DEPS_HINT ?= Add Linux host dependencies here if the project needs GUI or CGO system packages.
WINDOWS_HOST_DEPS_HINT ?= Add cross-build toolchain notes here if the project cannot use plain GOOS=windows builds.
ANDROID_HOST_DEPS_HINT ?= This repo uses Gio via scripts/gogio-android.sh; Fyne or other stacks will need a different Android runner and dependency notes.

EXACT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || true)
MAIN_PKG_ABS := $(abspath $(MAIN_PKG))
VERSION ?= $(shell \
	if git rev-parse --git-dir >/dev/null 2>&1; then \
		exact=$$(git describe --tags --exact-match 2>/dev/null || true); \
		last=$$(git describe --tags --abbrev=0 2>/dev/null || true); \
		sha=$$(git rev-parse --short HEAD 2>/dev/null || echo nogit); \
		dirty=$$(if [ -n "$$(git status --porcelain 2>/dev/null)" ]; then echo "-dirty"; fi); \
		if [ -n "$$exact" ]; then \
			printf "%s%s" "$$exact" "$$dirty"; \
		elif [ -n "$$last" ]; then \
			printf "%s-%s-dev%s" "$$last" "$$sha" "$$dirty"; \
		else \
			printf "0.0.0-%s-dev%s" "$$sha" "$$dirty"; \
		fi; \
	else \
		echo "0.0.0-local"; \
	fi)

TEST_FILES := $(shell find . -type f -name '*_test.go' -not -path './vendor/*' -not -path './build/*' -not -path './dist/*' 2>/dev/null)
GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*' -not -path './build/*' -not -path './dist/*' 2>/dev/null)

LINUX_BIN := $(BUILD_DIR)/linux/$(APP_NAME)
WINDOWS_BIN := $(BUILD_DIR)/windows/$(APP_NAME).exe
MAC_BIN := $(BUILD_DIR)/mac/$(APP_NAME)
ANDROID_APK := $(BUILD_DIR)/android/$(APP_NAME).apk

ARTIFACT_DIR := $(DIST_DIR)/$(VERSION)
LINUX_ARTIFACT := $(ARTIFACT_DIR)/$(APP_NAME)_$(VERSION)_linux_$(LINUX_GOARCH).tar.gz
WINDOWS_ARTIFACT := $(ARTIFACT_DIR)/$(APP_NAME)_$(VERSION)_windows_$(WINDOWS_GOARCH).zip
ANDROID_ARTIFACT := $(ARTIFACT_DIR)/$(APP_NAME)_$(VERSION)_android.apk
CHECKSUM_FILE := $(ARTIFACT_DIR)/SHA256SUMS.txt

ARTIFACT_FILES :=
ifneq ($(filter linux,$(ARTIFACT_TARGETS)),)
ARTIFACT_FILES += $(LINUX_ARTIFACT)
endif
ifneq ($(filter windows,$(ARTIFACT_TARGETS)),)
ARTIFACT_FILES += $(WINDOWS_ARTIFACT)
endif
ifneq ($(filter android,$(ARTIFACT_TARGETS)),)
ARTIFACT_FILES += $(ANDROID_ARTIFACT)
endif
RELEASE_ASSETS := $(ARTIFACT_FILES) $(CHECKSUM_FILE)

.DEFAULT_GOAL := help

.PHONY: help prepare deps mod-tidy fmt fmt-check test check build linux windows mac android artifacts snapshot release release-check clean

help:
	@echo "UNIVERSAL GO BUILD / TEST / PACKAGE / RELEASE ENTRYPOINT"
	@echo "Drop this Makefile into a Go project, then tune only the small project-specific variables at the top."
	@echo "Start by updating APP_NAME, then override APP_ID, MAIN_PKG, icon paths, targets, or platform commands only if that project differs."
	@echo "Linux/Windows builds are expected to be common. Android support is optional and depends on the UI stack."
	@echo "This repo uses Gio + gogio. A Fyne-based Android project will usually replace GOGIO_RUNNER and Android hints."
	@echo ""
	@echo "Project tuning checklist:"
	@echo "  1. Set APP_NAME to the shipped binary name."
	@echo "  2. Override APP_ID only if the Android application id should not be com.vitovt.<APP_NAME>."
	@echo "  3. Override MAIN_PKG only if your main package is not ./cmd/<APP_NAME>."
	@echo "  4. Set ARTIFACT_TARGETS to the platforms this repo actually releases."
	@echo "  5. Review Linux/Windows/macOS CGO flags and platform commands for the current UI/toolchain."
	@echo "  6. If Android exists, review ANDROID_ICON_PATH, ANDROID_NOTIFICATION_ICON_NAME, ANDROID_* values, and the Android runner if the stack is not Gio."
	@echo "  7. Replace the host dependency hints below with project-specific package/toolchain notes."
	@echo ""
	@echo "Configured project hints:"
	@echo "  Tune hint: $(PROJECT_TUNE_HINT)"
	@echo "  Linux deps hint: $(LINUX_HOST_DEPS_HINT)"
	@echo "  Windows deps hint: $(WINDOWS_HOST_DEPS_HINT)"
	@echo "  Android deps hint: $(ANDROID_HOST_DEPS_HINT)"
	@echo ""
	@echo "Targets:"
	@echo "  make help         - Show this help output (default)."
	@echo "  make prepare      - Create local build and dist folders."
	@echo "  make deps         - Download Go modules without mutating go.mod/go.sum."
	@echo "  make mod-tidy     - Run go mod tidy explicitly."
	@echo "  make fmt          - Format Go sources in-place."
	@echo "  make fmt-check    - Fail if Go sources are not gofmt formatted."
	@echo "  make test         - Run automated tests, or print a clear skip message if there are none."
	@echo "  make check        - Run deps + fmt-check + test."
	@echo "  make build        - Build the default desktop target (currently linux)."
	@echo "  make linux        - Build Linux binary (currently amd64 only)."
	@echo "  make windows      - Build Windows binary (currently amd64 only)."
	@echo "  make mac          - Build macOS binary (currently amd64 only, macOS host expected)."
	@echo "  make android      - Build Android APK with the configured Android runner."
	@echo "  make artifacts    - Package versioned release artifacts into $(DIST_DIR)/<version>/."
	@echo "  make snapshot     - Build versioned local artifacts without publishing a GitHub release."
	@echo "  make release-check - Validate git/gh release prerequisites without publishing."
	@echo "  make release      - Verify release context, package assets, and publish a GitHub release."
	@echo "  make clean        - Remove build and dist artifacts."
	@echo ""
	@echo "Release behavior:"
	@echo "  release requires an exact git tag on HEAD, a clean git tree, gh installed, and gh auth ready."
	@echo "  snapshot works from any commit and produces a -dev/-dirty version when HEAD is not an exact tag."
	@echo "  Artifact coverage is intentionally limited to $(ARTIFACT_TARGETS); extend it per project when needed."
	@echo ""
	@echo "Current values:"
	@echo "  APP_NAME:         $(APP_NAME)"
	@echo "  APP_ID:           $(APP_ID)"
	@echo "  MAIN_PKG:         $(MAIN_PKG)"
	@echo "  VERSION:          $(VERSION)"
	@echo "  ARTIFACT_TARGETS: $(ARTIFACT_TARGETS)"
	@echo "  BUILD_DIR:        $(BUILD_DIR)"
	@echo "  DIST_DIR:         $(DIST_DIR)"
	@echo "  ANDROID_ICON_PATH: $(ANDROID_ICON_PATH)"
	@echo "  ANDROID_NOTIFICATION_ICON_NAME: $(ANDROID_NOTIFICATION_ICON_NAME)"
	@echo "  GOGIO_RUNNER:     $(GOGIO_RUNNER)"

prepare:
	@mkdir -p $(BUILD_DIR)/linux $(BUILD_DIR)/windows $(BUILD_DIR)/mac $(BUILD_DIR)/android $(DIST_DIR)

deps:
	@echo "Downloading Go modules..."
	@go mod download
	@go mod verify

mod-tidy:
	@echo "Running go mod tidy..."
	@go mod tidy

fmt:
	@if [ -z "$(strip $(GO_FILES))" ]; then \
		echo "No Go files found, skipping format."; \
	else \
		echo "Formatting Go files..."; \
		gofmt -s -w $(GO_FILES); \
	fi

fmt-check:
	@if [ -z "$(strip $(GO_FILES))" ]; then \
		echo "No Go files found, skipping format check."; \
	else \
		unformatted=$$(gofmt -l $(GO_FILES)); \
		if [ -n "$$unformatted" ]; then \
			echo "Go files are not formatted. Run 'make fmt'."; \
			printf '%s\n' "$$unformatted"; \
			exit 1; \
		fi; \
		echo "Formatting check passed."; \
	fi

test:
	@if [ -z "$(strip $(TEST_FILES))" ]; then \
		echo "No tests found, skipping."; \
	else \
		echo "Running Go tests..."; \
		go test ./...; \
	fi

check: deps fmt-check test
	@echo "Checks completed."

build: linux

linux: check prepare
	@echo "Building Linux binary..."
	@GOOS=linux GOARCH=$(LINUX_GOARCH) CGO_ENABLED=$(LINUX_CGO_ENABLED) go build -o $(LINUX_BIN) $(MAIN_PKG)
	@echo "Linux build completed: $(LINUX_BIN)"

windows: check prepare
	@echo "Building Windows binary..."
	@GOOS=windows GOARCH=$(WINDOWS_GOARCH) CGO_ENABLED=$(WINDOWS_CGO_ENABLED) go build -o $(WINDOWS_BIN) $(MAIN_PKG)
	@echo "Windows build completed: $(WINDOWS_BIN)"

mac: check prepare
	@if [ "$$(go env GOHOSTOS)" = "darwin" ]; then \
		echo "Building macOS binary..."; \
		GOOS=darwin GOARCH=$(MAC_GOARCH) CGO_ENABLED=$(MAC_CGO_ENABLED) go build -o $(MAC_BIN) $(MAIN_PKG); \
		echo "macOS build completed: $(MAC_BIN)"; \
	else \
		echo "mac target requires a macOS host (or a separately configured osxcross toolchain)."; \
		echo "This repo currently treats non-macOS hosts as unsupported for mac builds."; \
		exit 1; \
	fi

android: check prepare
	@test -x $(GOGIO_RUNNER) || (echo "Missing Android build runner: $(GOGIO_RUNNER)" && exit 1)
	@test -f $(ANDROID_ICON_PATH) || (echo "Missing Android icon: $(ANDROID_ICON_PATH)" && exit 1)
	@ANDROID_HOME="$(ANDROID_HOME_FALLBACK)"; \
	if [ ! -d "$$ANDROID_HOME" ]; then \
		echo "Android SDK path not found. Set ANDROID_HOME or ANDROID_SDK_ROOT."; \
		exit 1; \
	fi; \
	export ANDROID_HOME; \
	echo "Building Android APK with Gio runner..."; \
	cd $(BUILD_DIR)/android && APP_NAME="$(APP_NAME)" ANDROID_NOTIFICATION_ICON_NAME="$(ANDROID_NOTIFICATION_ICON_NAME)" $(GOGIO_RUNNER) "$(GOGIO)" "$(ANDROID_MIN_SDK)" "$(ANDROID_TARGET_SDK)" "$(APP_ID)" "$(MAIN_PKG_ABS)" "$(APP_NAME).apk"

$(LINUX_ARTIFACT): linux
	@mkdir -p $(ARTIFACT_DIR)
	@echo "Packaging Linux artifact: $@"
	@tar -C $(BUILD_DIR)/linux -czf $@ $(APP_NAME)

$(WINDOWS_ARTIFACT): windows
	@mkdir -p $(ARTIFACT_DIR)
	@if ! command -v zip >/dev/null 2>&1; then \
		echo "zip is required to package Windows artifacts."; \
		exit 1; \
	fi
	@echo "Packaging Windows artifact: $@"
	@rm -f $@
	@zip -jq $@ $(WINDOWS_BIN)

$(ANDROID_ARTIFACT): android
	@mkdir -p $(ARTIFACT_DIR)
	@echo "Packaging Android artifact: $@"
	@cp $(ANDROID_APK) $@

$(CHECKSUM_FILE): $(ARTIFACT_FILES)
	@mkdir -p $(ARTIFACT_DIR)
	@echo "Writing checksums: $@"
	@cd $(ARTIFACT_DIR) && \
	if command -v sha256sum >/dev/null 2>&1; then \
		sha256sum $(notdir $(ARTIFACT_FILES)) > $(notdir $(CHECKSUM_FILE)); \
	elif command -v shasum >/dev/null 2>&1; then \
		shasum -a 256 $(notdir $(ARTIFACT_FILES)) > $(notdir $(CHECKSUM_FILE)); \
	else \
		echo "sha256sum or shasum is required to create checksums."; \
		exit 1; \
	fi

artifacts: $(RELEASE_ASSETS)
	@echo "Artifacts are ready in $(ARTIFACT_DIR)"

snapshot: artifacts
	@echo "Snapshot build completed for version $(VERSION)"

release-check:
	@if [ -z "$(EXACT_TAG)" ]; then \
		last_tag=$$(git describe --tags --abbrev=0 2>/dev/null || echo "<none>"); \
		echo "Release requires an exact git tag on HEAD."; \
		echo "Current derived version is $(VERSION). Use 'make snapshot' for untagged builds."; \
		echo "Last git tag was: $$last_tag"; \
		echo "Recent commits (git log --oneline -n 10):"; \
		git log --oneline -n 10; \
		exit 1; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Release requires a clean git working tree."; \
		exit 1; \
	fi
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "GitHub CLI (gh) is required for make release."; \
		exit 1; \
	fi
	@if ! gh auth status >/dev/null 2>&1; then \
		echo "GitHub CLI is installed but not authenticated. Run 'gh auth login'."; \
		exit 1; \
	fi
	@if gh release view "$(EXACT_TAG)" >/dev/null 2>&1; then \
		echo "GitHub release $(EXACT_TAG) already exists."; \
		exit 1; \
	fi
	@echo "Release context looks good for tag $(EXACT_TAG)."

release: release-check artifacts
	@echo "Publishing GitHub release $(EXACT_TAG)..."
	@gh release create "$(EXACT_TAG)" $(RELEASE_ASSETS) --verify-tag --title "Release $(EXACT_TAG)" --generate-notes
	@echo "GitHub release $(EXACT_TAG) published."

clean:
	@echo "Removing build and dist artifacts..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
