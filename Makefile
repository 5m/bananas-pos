BINARY_NAME := bananas-pos
APP_NAME := Bananas POS
CMD_DIR := ./cmd/bananas-pos
DIST_DIR := dist
LOCAL_BIN_DIR := .bin
ICON_PNG := internal/trayicon/assets/icon.png
MACOS_APP_ICON_PNG := internal/appicon/assets/icon-macos.png
MACOS_APP_ICON_ICNS := internal/appicon/assets/icon-macos.icns
ICON_ICO := internal/trayicon/assets/icon.ico
WINDOWS_SYSO := cmd/bananas-pos/bananas-pos_windows_amd64.syso
VERSION := $(shell sed -n 's/^var Version = "\(.*\)"$$/\1/p' internal/meta/meta.go)
FYNE_CLI_VERSION := v1.7.0
FYNE_BIN := $(LOCAL_BIN_DIR)/fyne
MACOSX_DEPLOYMENT_TARGET := 10.14
MACOS_CC_WRAPPER := $(PWD)/scripts/clang-macos14.sh
MACOS_CXX_WRAPPER := $(PWD)/scripts/clang++-macos14.sh

LINUX_ARCHIVE := $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz
MACOS_AMD64_ARCHIVE := $(BINARY_NAME)-$(VERSION)-macos-amd64.tar.gz
MACOS_ARM64_ARCHIVE := $(BINARY_NAME)-$(VERSION)-macos-arm64.tar.gz
WINDOWS_ARCHIVE := $(BINARY_NAME)-$(VERSION)-windows-amd64.zip

.PHONY: version clean build package-linux package-macos-app package-macos-amd64 package-macos-arm64 package-windows

version:
	@printf '%s\n' "$(VERSION)"

clean:
	rm -rf "$(DIST_DIR)" "cmd/bananas-pos/$(APP_NAME).app" "$(WINDOWS_SYSO)"

$(FYNE_BIN):
	mkdir -p "$(LOCAL_BIN_DIR)"
	GOBIN="$(PWD)/$(LOCAL_BIN_DIR)" go install fyne.io/tools/cmd/fyne@$(FYNE_CLI_VERSION)

build:
	mkdir -p "$(DIST_DIR)"
	go build -o "$(DIST_DIR)/$(BINARY_NAME)" $(CMD_DIR)

package-linux:
	mkdir -p "$(DIST_DIR)"
	go build -o "$(DIST_DIR)/$(BINARY_NAME)" $(CMD_DIR)
	tar -C "$(DIST_DIR)" -czf "$(LINUX_ARCHIVE)" "$(BINARY_NAME)"

package-macos-app: $(FYNE_BIN) $(MACOS_APP_ICON_ICNS)
	mkdir -p "$(DIST_DIR)"
	rm -rf "cmd/bananas-pos/$(APP_NAME).app" "$(DIST_DIR)/$(APP_NAME).app"
	cd cmd/bananas-pos && \
		MACOSX_DEPLOYMENT_TARGET="$(MACOSX_DEPLOYMENT_TARGET)" \
		CC="$(MACOS_CC_WRAPPER)" \
		CXX="$(MACOS_CXX_WRAPPER)" \
		CGO_CFLAGS="-mmacosx-version-min=$(MACOSX_DEPLOYMENT_TARGET)" \
		CGO_CXXFLAGS="-mmacosx-version-min=$(MACOSX_DEPLOYMENT_TARGET)" \
		CGO_LDFLAGS="-mmacosx-version-min=$(MACOSX_DEPLOYMENT_TARGET)" \
		../../$(FYNE_BIN) package -os darwin -release -icon ../../$(MACOS_APP_ICON_PNG) -name "$(APP_NAME)"
	cp "$(MACOS_APP_ICON_ICNS)" "cmd/bananas-pos/$(APP_NAME).app/Contents/Resources/icon.icns"
	mv "cmd/bananas-pos/$(APP_NAME).app" "$(DIST_DIR)/"

package-macos-amd64: package-macos-app
	tar -C "$(DIST_DIR)" -czf "$(MACOS_AMD64_ARCHIVE)" "$(APP_NAME).app"

package-macos-arm64: package-macos-app
	tar -C "$(DIST_DIR)" -czf "$(MACOS_ARM64_ARCHIVE)" "$(APP_NAME).app"

package-windows:
	pwsh -NoProfile -Command "$$ErrorActionPreference = 'Stop'; New-Item -ItemType Directory -Force -Path '$(DIST_DIR)' | Out-Null; $$runnerMingw = Join-Path $$env:RUNNER_TEMP 'msys64\mingw64\bin'; if ($$env:RUNNER_TEMP -and (Test-Path $$runnerMingw)) { $$env:Path = $$runnerMingw + ';' + $$env:Path }; $$fallbackMingw = 'C:\msys64\mingw64\bin'; if (Test-Path $$fallbackMingw) { $$env:Path = $$fallbackMingw + ';' + $$env:Path }; $$env:CC = 'gcc'; $$env:CXX = 'g++'; rsrc -ico '$(ICON_ICO)' -o '$(WINDOWS_SYSO)'; go build -o '$(DIST_DIR)/$(BINARY_NAME).exe' $(CMD_DIR); Compress-Archive -Path '$(DIST_DIR)/$(BINARY_NAME).exe' -DestinationPath '$(WINDOWS_ARCHIVE)' -Force; Remove-Item '$(WINDOWS_SYSO)' -Force"
