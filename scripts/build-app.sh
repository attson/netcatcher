#!/usr/bin/env bash
# Build NetCatcher.app bundle locally (macOS).
# Usage: scripts/build-app.sh
set -euo pipefail

cd "$(dirname "$0")/.."

APP="build/NetCatcher.app"

echo "==> Building frontend"
(
  cd frontend
  if [ ! -x node_modules/.bin/vite ]; then
    npm ci
  fi
  ./node_modules/.bin/vite build
)

echo "==> Building Go binary"
mkdir -p build/bin
go build -o build/bin/netcatcher .

echo "==> Packaging $APP"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp build/bin/netcatcher "$APP/Contents/MacOS/NetCatcher"

ICONSET="build/appicon.iconset"
rm -rf "$ICONSET"
mkdir -p "$ICONSET"
sips -z 16 16     build/appicon.png --out "$ICONSET/icon_16x16.png"     >/dev/null
sips -z 32 32     build/appicon.png --out "$ICONSET/icon_16x16@2x.png"  >/dev/null
sips -z 32 32     build/appicon.png --out "$ICONSET/icon_32x32.png"     >/dev/null
sips -z 64 64     build/appicon.png --out "$ICONSET/icon_32x32@2x.png"  >/dev/null
sips -z 128 128   build/appicon.png --out "$ICONSET/icon_128x128.png"   >/dev/null
sips -z 256 256   build/appicon.png --out "$ICONSET/icon_128x128@2x.png">/dev/null
sips -z 256 256   build/appicon.png --out "$ICONSET/icon_256x256.png"   >/dev/null
sips -z 512 512   build/appicon.png --out "$ICONSET/icon_256x256@2x.png">/dev/null
sips -z 512 512   build/appicon.png --out "$ICONSET/icon_512x512.png"   >/dev/null
cp build/appicon.png "$ICONSET/icon_512x512@2x.png"
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/appicon.icns"
rm -rf "$ICONSET"

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>NetCatcher</string>
  <key>CFBundleIconFile</key>
  <string>appicon</string>
  <key>CFBundleIdentifier</key>
  <string>com.attson.netcatcher</string>
  <key>CFBundleName</key>
  <string>NetCatcher</string>
  <key>CFBundleVersion</key>
  <string>1.0.0</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0.0</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

# Ad-hoc sign so the bundle identifier is honored (unsigned bundles can
# have notification delivery quirks on macOS).
codesign --force --deep --sign - "$APP" >/dev/null 2>&1 || true

echo "==> Done: $APP"
echo "Run with: open $APP"
