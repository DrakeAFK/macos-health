#!/bin/sh
# Run only on the ephemeral macOS release runner, with secrets supplied via env.
set -eu
if [ -z "${MACOS_SIGN_IDENTITY:-}" ]; then
  echo 'Developer ID signing is not configured.'
  exit 0
fi
: "${RUNNER_TEMP:?CI runner required}"
: "${GITHUB_ENV:?CI environment required}"
: "${MACOS_CERTIFICATE_P12:?signing certificate required}"
: "${MACOS_CERTIFICATE_PASSWORD:?certificate password required}"
umask 077
cert_path="$RUNNER_TEMP/macos-health-signing.p12"
keychain_path="$RUNNER_TEMP/macos-health-signing.keychain-db"
keychain_password=$(/usr/bin/openssl rand -hex 32)
trap 'rm -f "$cert_path"' EXIT HUP INT TERM
printf '%s' "$MACOS_CERTIFICATE_P12" | /usr/bin/base64 --decode > "$cert_path"
/usr/bin/security create-keychain -p "$keychain_password" "$keychain_path"
/usr/bin/security set-keychain-settings -lut 21600 "$keychain_path"
/usr/bin/security unlock-keychain -p "$keychain_password" "$keychain_path"
/usr/bin/security import "$cert_path" -k "$keychain_path" -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign >/dev/null
/usr/bin/security set-key-partition-list -S apple-tool:,apple: -s -k "$keychain_password" "$keychain_path" >/dev/null
printf 'MACOS_SIGN_KEYCHAIN=%s\n' "$keychain_path" >> "$GITHUB_ENV"
if [ -n "${MACOS_NOTARY_APPLE_ID:-}" ]; then
  : "${MACOS_NOTARY_PASSWORD:?notarization password required}"
  : "${MACOS_TEAM_ID:?Apple team required}"
  /usr/bin/xcrun notarytool store-credentials macos-health-notary --apple-id "$MACOS_NOTARY_APPLE_ID" --password "$MACOS_NOTARY_PASSWORD" --team-id "$MACOS_TEAM_ID" --keychain "$keychain_path" >/dev/null
  printf 'MACOS_NOTARY_PROFILE=macos-health-notary\n' >> "$GITHUB_ENV"
fi
