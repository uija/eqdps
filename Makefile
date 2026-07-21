PREFIX ?= /usr/local
DESTDIR ?=

DIST_DIR := dist
BIN_DIR := $(DESTDIR)$(PREFIX)/bin
APP_DIR := $(DESTDIR)$(PREFIX)/share/applications
ICON_DIR := $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps

GUI_BINARY := $(DIST_DIR)/eqdps-gui
TUI_BINARY := $(DIST_DIR)/eqdps
WINDOWS_GUI_BINARY := $(DIST_DIR)/eqdps-gui-windows-amd64.exe
WINDOWS_TUI_BINARY := $(DIST_DIR)/eqdps-tui-windows-amd64.exe

.PHONY: all gui tui windows install uninstall clean test

all: gui tui

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

gui: | $(DIST_DIR)
	go build -o $(GUI_BINARY) ./gui

tui: | $(DIST_DIR)
	go build -o $(TUI_BINARY) ./tui

windows: | $(DIST_DIR)
	env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w -H=windowsgui" -o $(WINDOWS_GUI_BINARY) ./gui
	env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" -o $(WINDOWS_TUI_BINARY) ./tui

install: all
	install -d $(BIN_DIR) $(APP_DIR) $(ICON_DIR)
	install -m 0755 $(GUI_BINARY) $(BIN_DIR)/eqdps-gui
	install -m 0755 $(TUI_BINARY) $(BIN_DIR)/eqdps
	install -m 0644 packaging/eqdps.desktop $(APP_DIR)/eqdps.desktop
	install -m 0644 img/eqdps-icon.svg $(ICON_DIR)/eqdps.svg

uninstall:
	rm -f $(BIN_DIR)/eqdps-gui
	rm -f $(BIN_DIR)/eqdps
	rm -f $(APP_DIR)/eqdps.desktop
	rm -f $(ICON_DIR)/eqdps.svg

clean:
	rm -rf $(DIST_DIR)

test:
	go test ./...
	go test ./tui/...
	go test ./gui/...
	go vet ./...
	go vet ./tui/...
	go vet ./gui/...
	cd gui && go test -race ./...
