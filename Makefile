PREFIX ?= /usr/local
DESTDIR ?=

DIST_DIR := dist
BIN_DIR := $(DESTDIR)$(PREFIX)/bin
APP_DIR := $(DESTDIR)$(PREFIX)/share/applications
ICON_DIR := $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps

GUI_BINARY := $(DIST_DIR)/eqdps-gui
WINDOWS_GUI_BINARY := $(DIST_DIR)/eqdps-gui-windows-amd64.exe

.PHONY: all gui windows install uninstall clean test

all: gui

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

gui: | $(DIST_DIR)
	go build -o $(GUI_BINARY) .

windows: | $(DIST_DIR)
	env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w -H=windowsgui" -o $(WINDOWS_GUI_BINARY) .

install: all
	install -d $(BIN_DIR) $(APP_DIR) $(ICON_DIR)
	install -m 0755 $(GUI_BINARY) $(BIN_DIR)/eqdps-gui
	install -m 0644 packaging/eqdps.desktop $(APP_DIR)/eqdps.desktop
	install -m 0644 img/eqdps-icon.svg $(ICON_DIR)/eqdps.svg

uninstall:
	rm -f $(BIN_DIR)/eqdps-gui
	rm -f $(APP_DIR)/eqdps.desktop
	rm -f $(ICON_DIR)/eqdps.svg

clean:
	rm -rf $(DIST_DIR)

test:
	go test ./...
	go vet ./...
	go test -race ./...
