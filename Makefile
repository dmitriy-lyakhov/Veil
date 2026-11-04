GO      ?= go
VERSION := 0.5.0
DIST    := dist
APP     := $(DIST)/Veil.app
DMG     := $(DIST)/Veil-$(VERSION).dmg
ICONSET := $(DIST)/Veil.iconset
LDFLAGS := -s -w

.PHONY: all server app app-universal icns macos-app dmg test clean

all: server app

server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/server/veil-server-linux-amd64 ./cmd/veil-server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/server/veil-server-linux-arm64 ./cmd/veil-server

app:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/veil ./cmd/veil

app-universal:
	CGO_ENABLED=1 GOARCH=amd64 CC="clang -arch x86_64" CGO_CFLAGS="-mmacosx-version-min=13.0" CGO_LDFLAGS="-mmacosx-version-min=13.0" $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/veil-darwin-amd64 ./cmd/veil
	CGO_ENABLED=1 GOARCH=arm64 CC="clang -arch arm64" CGO_CFLAGS="-mmacosx-version-min=13.0" CGO_LDFLAGS="-mmacosx-version-min=13.0" $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/veil-darwin-arm64 ./cmd/veil
	lipo -create -output $(DIST)/veil-universal $(DIST)/veil-darwin-amd64 $(DIST)/veil-darwin-arm64

icns:
	rm -rf $(ICONSET) && mkdir -p $(ICONSET)
	for s in 16 32 128 256 512; do \
		sips -z $$s $$s packaging/macos/icon.png --out $(ICONSET)/icon_$${s}x$${s}.png >/dev/null; \
		sips -z $$((s*2)) $$((s*2)) packaging/macos/icon.png --out $(ICONSET)/icon_$${s}x$${s}@2x.png >/dev/null; \
	done
	iconutil -c icns $(ICONSET) -o $(DIST)/Veil.icns

macos-app: server app-universal icns
	rm -rf $(APP)
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources/server
	cp $(DIST)/veil-universal $(APP)/Contents/MacOS/veil
	cp $(DIST)/server/veil-server-linux-* $(APP)/Contents/Resources/server/
	cp $(DIST)/Veil.icns $(APP)/Contents/Resources/Veil.icns
	cp packaging/macos/icon.png $(APP)/Contents/Resources/icon.png
	sed 's/@VERSION@/$(VERSION)/g' packaging/macos/Info.plist > $(APP)/Contents/Info.plist
	codesign --force --deep --sign - $(APP)

dmg: macos-app
	rm -rf $(DIST)/dmg $(DMG)
	mkdir -p $(DIST)/dmg
	cp -R $(APP) $(DIST)/dmg/
	ln -s /Applications $(DIST)/dmg/Applications
	cp packaging/macos/README.txt "$(DIST)/dmg/Прочтите.txt"
	hdiutil create -volname "Veil $(VERSION)" -srcfolder $(DIST)/dmg -fs HFS+ -format UDZO -imagekey zlib-level=9 -ov $(DMG)
	rm -rf $(DIST)/dmg

test:
	$(GO) test ./...

clean:
	rm -rf $(DIST)
