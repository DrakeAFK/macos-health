#!/bin/sh
# GoReleaser post-build hook. No uploads or signing without configured credentials.
set -eu
binary=${1:?binary path required}
if [ -z "${MACOS_SIGN_IDENTITY:-}" ]; then
  if [ -n "${MACOS_NOTARY_PROFILE:-}" ]; then
    echo 'Notarization requires MACOS_SIGN_IDENTITY.' >&2
    exit 1
  fi
  echo 'Developer ID not configured; producing an unsigned development artifact.'
  exit 0
fi
if [ -n "${MACOS_SIGN_KEYCHAIN:-}" ]; then
  /usr/bin/codesign --force --options runtime --timestamp --keychain "$MACOS_SIGN_KEYCHAIN" --sign "$MACOS_SIGN_IDENTITY" "$binary"
else
  /usr/bin/codesign --force --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" "$binary"
fi
/usr/bin/codesign --verify --strict --verbose=2 "$binary"
if [ -n "${MACOS_NOTARY_PROFILE:-}" ]; then
  signing_tmp=$(mktemp -d "${TMPDIR:-/tmp}/macos-health-notary.XXXXXX")
  trap 'rm -rf "$signing_tmp"' EXIT HUP INT TERM
  /usr/bin/ditto -c -k --keepParent "$binary" "$signing_tmp/binary.zip"
  if [ -n "${MACOS_SIGN_KEYCHAIN:-}" ]; then
    /usr/bin/xcrun notarytool submit "$signing_tmp/binary.zip" --keychain-profile "$MACOS_NOTARY_PROFILE" --keychain "$MACOS_SIGN_KEYCHAIN" --wait --timeout 15m --output-format json > "$signing_tmp/result.json"
  else
    /usr/bin/xcrun notarytool submit "$signing_tmp/binary.zip" --keychain-profile "$MACOS_NOTARY_PROFILE" --wait --timeout 15m --output-format json > "$signing_tmp/result.json"
  fi
  /usr/bin/python3 - "$signing_tmp/result.json" <<'PY'
import json, sys
with open(sys.argv[1]) as source:
    result = json.load(source)
if result.get('status') != 'Accepted':
    raise SystemExit('Apple notarization was not accepted; refusing to release.')
print('Apple notarization accepted.')
PY
fi
