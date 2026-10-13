#!/usr/bin/env bash
# Build a Finder-styled drag-to-Applications disk image around an existing app
# (adapted from Charker's make-dmg.sh). Finder lays the window out through
# AppleScript, so the first run asks to let the terminal control Finder.
#
#   scripts/make-dmg.sh <Moniflip.app> <out.dmg> [version] [background.tiff]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:?usage: make-dmg.sh <app> <out.dmg> [version] [background.tiff]}"
OUTPUT="${2:?usage: make-dmg.sh <app> <out.dmg> [version] [background.tiff]}"
VERSION="${3:-$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP/Contents/Info.plist")}"
BACKGROUND="${4:-$ROOT/assets/dmg/background.tiff}"
BACKGROUND_NAME="background.${BACKGROUND##*.}"
APP_NAME="$(basename "$APP")"
VOLUME_NAME="${APP_NAME%.app} $VERSION"

[ -d "$APP" ] || { echo "missing app bundle: $APP" >&2; exit 1; }
[ -f "$BACKGROUND" ] || { echo "missing DMG background: $BACKGROUND" >&2; exit 1; }
# Finder finds the disk to lay out by its name, which an open copy shares.
[ ! -e "/Volumes/$VOLUME_NAME" ] || { echo "eject \"/Volumes/$VOLUME_NAME\" first" >&2; exit 1; }

WORK="$(mktemp -d -t moniflip-dmg)"
MOUNT_POINT=""
MOUNTED=0
cleanup() {
	if [ "$MOUNTED" = "1" ]; then
		hdiutil detach "$MOUNT_POINT" -quiet -force || true
	fi
	rm -rf "$WORK"
}
trap cleanup EXIT

mkdir -p "$WORK/root/.background"
ditto --noclone "$APP" "$WORK/root/$APP_NAME"
ln -s /Applications "$WORK/root/Applications"
cp "$BACKGROUND" "$WORK/root/.background/$BACKGROUND_NAME"

hdiutil create -volname "$VOLUME_NAME" -srcfolder "$WORK/root" -fs HFS+ -format UDRW -ov "$WORK/rw.dmg" >/dev/null
# Finder lays out only disks mounted under /Volumes, so attach normally and
# read where the image went.
ATTACH_PLIST="$(hdiutil attach "$WORK/rw.dmg" -readwrite -noverify -noautoopen -plist)"
MOUNT_POINT="$(printf '%s' "$ATTACH_PLIST" | plutil -p - | awk -F ' => ' '/"mount-point"/ {gsub(/"/, "", $2); print $2; exit}')"
[ -d "$MOUNT_POINT" ] || { echo "could not resolve the mounted DMG" >&2; exit 1; }
MOUNTED=1
DISK_NAME="$(basename "$MOUNT_POINT")"

osascript <<APPLESCRIPT
tell application "Finder"
    open (POSIX file "$MOUNT_POINT" as alias)
    delay 1
    tell disk "$DISK_NAME"
        open
        set current view of container window to icon view
        set toolbar visible of container window to false
        set statusbar visible of container window to false
        set pathbar visible of container window to false
        set the bounds of container window to {100, 100, 760, 500}
        set viewOptions to the icon view options of container window
        set arrangement of viewOptions to not arranged
        set icon size of viewOptions to 112
        set text size of viewOptions to 13
        set label position of viewOptions to bottom
        set shows item info of viewOptions to false
        set shows icon preview of viewOptions to false
        set background picture of viewOptions to file ".background:$BACKGROUND_NAME"
        set position of item "$APP_NAME" to {178, 226}
        set position of item "Applications" to {482, 226}
        update without registering applications
        delay 2
        close
    end tell
end tell
APPLESCRIPT

rm -rf "$MOUNT_POINT/.fseventsd" "$MOUNT_POINT/.Trashes"
sync
# Spotlight or Finder, just done with the window, can keep the volume busy
# for a while: try again after 2, 4, 8 and 16 seconds, as mygo does.
for wait in 2 4 8 16 0; do
	if DETACH="$(hdiutil detach "$MOUNT_POINT" 2>&1)"; then
		MOUNTED=0
		break
	fi
	case "$DETACH" in
	*"Resource busy"*) [ "$wait" = 0 ] || sleep "$wait" ;;
	*) break ;;
	esac
done
[ "$MOUNTED" = 0 ] || { echo "$DETACH" >&2; exit 1; }

hdiutil convert "$WORK/rw.dmg" -format UDZO -imagekey zlib-level=9 -ov -o "$OUTPUT" >/dev/null
hdiutil verify "$OUTPUT" >/dev/null
echo "created styled DMG: $OUTPUT"
