GO      ?= go
VERSION := 1.1.0
DIST    := dist
APP     := $(DIST)/Veil.app
DMG     := $(DIST)/Veil-$(VERSION).dmg
PORTABLE := $(DIST)/Veil-$(VERSION)-portable.zip
PORTDIR := $(DIST)/Veil-$(VERSION)-portable
ICONSET := $(DIST)/Veil.iconset
LDFLAGS := -s -w

WINDIR  := $(DIST)/windows

.PHONY: all server docker app app-universal icns macos-app dmg portable macos-release windows test clean

all: server app

server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/server/veil-server-linux-amd64 ./cmd/veil-server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/server/veil-server-linux-arm64 ./cmd/veil-server
	# userspace WireGuard и AmneziaWG для контейнеров (образы собираются на сервере из этих бинарников)
	for a in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$a $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/server/wireguard-go-linux-$$a golang.zx2c4.com/wireguard && \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$a $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/server/amneziawg-go-linux-$$a github.com/amnezia-vpn/amneziawg-go || exit 1; \
	done

# Docker: образы протоколов из исходников (docker compose в корне репозитория)
docker:
	docker compose build

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
	cp $(DIST)/server/*-linux-* $(APP)/Contents/Resources/server/
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

portable: macos-app
	rm -rf $(PORTDIR) $(PORTABLE)
	mkdir -p $(PORTDIR)
	cp -R $(APP) $(PORTDIR)/
	cp packaging/macos/PORTABLE.txt "$(PORTDIR)/Прочтите.txt"
	ditto -c -k --sequesterRsrc --keepParent $(PORTDIR) $(PORTABLE)
	rm -rf $(PORTDIR)

macos-release: dmg portable

test:
	$(GO) test ./...

clean:
	rm -rf $(DIST)

# Windows: GUI (нужен MinGW-w64 в PATH), консольный клиент и серверные бинарники для Linux рядом.
windows: server
	mkdir -p $(WINDIR)/server
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS) -H=windowsgui' -o $(WINDIR)/veil.exe ./cmd/veil
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(WINDIR)/veil-cli.exe ./cmd/veil-cli
	cp $(DIST)/server/*-linux-* $(WINDIR)/server/
