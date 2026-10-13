#!/usr/bin/env bash
# Build, sign, notarize and upload a Moniflip release from this Mac.
#
# Produces the notarized macOS DMG (Apple Silicon), the Windows installer and
# the signed update files of both, and uploads them to a draft GitHub release
# whose notes are the version's section of CHANGELOG.md. Publishing the draft
# stays a manual step.
#
# One-time setup on the release Mac:
#   - a Developer ID Application identity in the keychain: the first one, or
#     the one named by MONIFLIP_SIGN_IDENTITY
#   - notary credentials under the profile in macos.notarize.keychainProfile:
#       xcrun notarytool store-credentials Charker-Notary
#   - the update signing key (mygo keygen) in the login keychain; paste the
#     contents of mygo-update.key at the prompt:
#       security add-generic-password -a moniflip -s moniflip-updater -w
#   - gh auth login; brew install makensis
#   - let the terminal control Finder (asked on the first run of make-dmg.sh)
#
# Per release: set "version" in mygo.json, add its "## <version>" section to
# CHANGELOG.md, commit, tag v<version>, push the commit and the tag, then run
# scripts/release.sh. UPLOAD=0 builds and verifies without uploading, a dry
# run that needs no commit or tag.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

UPLOAD="${UPLOAD:-1}"
KEY_SERVICE="moniflip-updater"
PLATFORMS="darwin/arm64,windows/amd64"

die() {
	echo "release: $*" >&2
	exit 1
}

# cfg KEYPATH prints a value of mygo.json, nothing when it is missing
# (plutil prints its errors to stdout).
cfg() {
	local value
	if value="$(plutil -extract "$1" raw -o - mygo.json 2>/dev/null)"; then
		printf '%s' "$value"
	fi
}

# notes VERSION prints the CHANGELOG.md section of VERSION, as mygo reads it:
# from "## VERSION" (or "## [VERSION]", "## vVERSION") to the next "## ".
notes() {
	awk -v v="$1" '
		/^## / {
			if (found) exit
			split(substr($0, 4), f, " ")
			h = f[1]; sub(/^\[/, "", h); sub(/\].*$/, "", h); sub(/^v/, "", h)
			found = (h == v)
			next
		}
		found
	' CHANGELOG.md
}

for tool in go gh git makensis plutil security xcrun spctl codesign shasum; do
	command -v "$tool" >/dev/null || die "$tool not found (makensis: brew install makensis)"
done

NAME="$(cfg name)"
VERSION="$(cfg version)"
OUT="$(cfg out)"
OUT="${OUT:-build}"
REPO="$(cfg updates.github)"
PUBLIC_KEY="$(cfg updates.publicKey)"
# The first valid Developer ID Application identity of the keychain, by
# name, unless MONIFLIP_SIGN_IDENTITY names one.
IDENTITIES="$(security find-identity -v -p codesigning)"
IDENTITY="${MONIFLIP_SIGN_IDENTITY:-$(awk -F '"' '/"Developer ID Application:/ {print $2; exit}' <<<"$IDENTITIES")}"
PROFILE="$(cfg macos.notarize.keychainProfile)"
TAG="v$VERSION"

[ -n "$NAME" ] && [ -n "$VERSION" ] || die "mygo.json needs name and version"
[ -n "$REPO" ] || die "mygo.json needs updates.github (owner/repo)"
[ -n "$PUBLIC_KEY" ] || die "mygo.json needs updates.publicKey (mygo-update.pub)"
case "$IDENTITY" in
"Developer ID Application:"*) ;;
*) die "not a Developer ID Application identity: \"$IDENTITY\" (set MONIFLIP_SIGN_IDENTITY, or add one to the keychain)" ;;
esac
[ -n "$PROFILE" ] || die "mygo.json needs macos.notarize.keychainProfile"
# make-dmg.sh lays out the volume it mounts by name, which an open copy shares.
[ ! -e "/Volumes/$NAME $VERSION" ] || die "eject \"/Volumes/$NAME $VERSION\" first"

# Source of an upload: clean, tagged v<version>, and that tag on GitHub at
# this commit. mygo creates the release with gh without --verify-tag, which
# would make a missing tag at the default branch's head.
if [ "$UPLOAD" = "1" ]; then
	[ -z "$(git status --porcelain)" ] || die "the working tree is not clean"
	HEAD_SHA="$(git rev-parse HEAD)"
	[ "$(git rev-parse -q --verify "refs/tags/$TAG^{commit}" || true)" = "$HEAD_SHA" ] ||
		die "HEAD must be tagged $TAG (mygo.json version $VERSION)"
	gh auth status --active >/dev/null 2>&1 || die "gh is not signed in: gh auth login"
	REMOTE_REF="$(gh api "repos/$REPO/git/ref/tags/$TAG" --jq '.object.type + " " + .object.sha' 2>/dev/null)" ||
		die "$TAG is not on github.com/$REPO: git push origin $TAG"
	read -r REF_TYPE REMOTE_SHA <<<"$REMOTE_REF"
	if [ "$REF_TYPE" = "tag" ]; then
		REMOTE_SHA="$(gh api "repos/$REPO/git/tags/$REMOTE_SHA" --jq .object.sha)"
	fi
	[ "$REMOTE_SHA" = "$HEAD_SHA" ] || die "$TAG on github.com/$REPO is not HEAD"
	[ "$(gh release view "$TAG" --repo "$REPO" --json isDraft --jq .isDraft 2>/dev/null || echo none)" != "false" ] ||
		die "release $TAG is already published; uploading would replace its files"
fi

NOTES="$(mktemp -t moniflip-notes)"
trap 'rm -f "$NOTES"' EXIT
notes "$VERSION" >"$NOTES"
grep -q '[^[:space:]]' "$NOTES" || die "CHANGELOG.md has no notes under \"## $VERSION\""

# Every module linked into either app is credited in THIRD-PARTY-NOTICES.md.
UNCREDITED="$(for p in ${PLATFORMS//,/ }; do
	CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="${p%/*}" GOARCH="${p#*/}" \
		go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}' .
done | sort -u | while read -r module; do
	[ -z "$module" ] || grep -Fq "$module" THIRD-PARTY-NOTICES.md || echo "$module"
done)"
[ -z "$UNCREDITED" ] || die "THIRD-PARTY-NOTICES.md does not credit $(paste -sd ' ' - <<<"$UNCREDITED")"

# Signing and notarization credentials, checked before the build because
# mygo only uses them after compiling, and the update key after notarizing.
grep -Fq "\"$IDENTITY\"" <<<"$IDENTITIES" || die "the keychain has no valid identity \"$IDENTITY\""
xcrun notarytool history --keychain-profile "$PROFILE" >/dev/null ||
	die "notary profile $PROFILE does not work: xcrun notarytool store-credentials $PROFILE"
UPDATE_KEY="$(security find-generic-password -s "$KEY_SERVICE" -w 2>/dev/null)" ||
	die "no update key in the keychain: security add-generic-password -a moniflip -s $KEY_SERVICE -w"
# An Ed25519 private key ends with its public key.
[ "$(printf '%s' "$UPDATE_KEY" | base64 -d | tail -c 32 | base64)" = "$PUBLIC_KEY" ] ||
	die "the keychain's update key does not match updates.publicKey"

MAC_DIR="$OUT/darwin-arm64"
WIN_DIR="$OUT/windows-amd64"
APP="$MAC_DIR/$NAME.app"
DMG="$MAC_DIR/$NAME $VERSION arm64.dmg"
SETUP="$WIN_DIR/$NAME Setup $VERSION amd64.exe"
rm -rf "$MAC_DIR" "$WIN_DIR"

# mygo signs with IDENTITY (-sign, hardened runtime), makes and notarizes
# its plain DMG and staples the app, so the update archives carry a stapled
# app; it also signs the updates. The DMG people download is then rebuilt with
# the Finder background (scripts/make-dmg.sh), signed, notarized and stapled.
MYGO_UPDATER_PRIVATE_KEY="$UPDATE_KEY" go tool mygo build -platform "$PLATFORMS" -sign "$IDENTITY"
unset UPDATE_KEY

for f in "$APP" "$DMG" "$SETUP" "$MAC_DIR/update-darwin-arm64.json" "$WIN_DIR/update-windows-amd64.json"; do
	[ -e "$f" ] || die "the build did not produce $f"
done
for target in darwin-arm64 windows-amd64; do
	manifest="$OUT/$target/update-$target.json"
	[ "$(plutil -extract version raw -o - "$manifest")" = "$VERSION" ] ||
		die "$manifest is not version $VERSION"
done

scripts/make-dmg.sh "$APP" "$DMG" "$VERSION" assets/dmg/background.tiff
codesign --force --timestamp --sign "$IDENTITY" "$DMG"
SUBMISSION="$(xcrun notarytool submit "$DMG" --keychain-profile "$PROFILE" --wait --output-format json)"
STATUS="$(plutil -extract status raw -o - - <<<"$SUBMISSION" 2>/dev/null || true)"
[ "$STATUS" = "Accepted" ] || die "notarization of $DMG: ${STATUS:-no status}; see xcrun notarytool log $(plutil -extract id raw -o - - <<<"$SUBMISSION" 2>/dev/null) --keychain-profile $PROFILE"
xcrun stapler staple "$DMG"

# gatekeeper ARGS... runs spctl and requires a notarized Developer ID verdict.
gatekeeper() {
	local verdict
	verdict="$(spctl "$@" 2>&1)" || die "Gatekeeper rejected ${*: -1}: $verdict"
	echo "$verdict"
	grep -q "source=Notarized Developer ID" <<<"$verdict" || die "${*: -1} is not notarized"
}
codesign --verify --deep --strict --verbose=2 "$APP"
gatekeeper -a -vv -t exec "$APP"
gatekeeper -a -vv -t open --context context:primary-signature "$DMG"
xcrun stapler validate "$APP"
xcrun stapler validate "$DMG"

if [ "$UPLOAD" = "1" ]; then
	gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1 ||
		gh release create "$TAG" --repo "$REPO" --draft --verify-tag --title "$TAG" --notes-file "$NOTES" >/dev/null
	gh release edit "$TAG" --repo "$REPO" --title "$TAG" --notes-file "$NOTES" >/dev/null
	# Installers and update archives first, the update manifests last (as mygo
	# does), so a manifest never points at an archive that is not there yet.
	# Delta updates exist once an earlier version is published.
	shopt -s nullglob
	DELTAS=("$MAC_DIR"/*.delta "$WIN_DIR"/*.delta)
	shopt -u nullglob
	gh release upload "$TAG" --repo "$REPO" --clobber "$DMG" "$SETUP" "$MAC_DIR"/*.tar.gz "$WIN_DIR"/*.tar.gz ${DELTAS[@]+"${DELTAS[@]}"}
	gh release upload "$TAG" --repo "$REPO" --clobber "$MAC_DIR/update-darwin-arm64.json" "$WIN_DIR/update-windows-amd64.json"
fi

echo
echo "Moniflip $VERSION is built, signed and notarized:"
shasum -a 256 "$DMG" "$SETUP"
echo
if [ "$UPLOAD" = "1" ]; then
	echo "Next: review the draft, then publish it, which makes installed apps update:"
	echo "  gh release view $TAG --repo $REPO --web"
	echo "  gh release edit $TAG --repo $REPO --draft=false"
else
	echo "UPLOAD=0: nothing was uploaded. Run again without it to upload to a draft release."
fi
