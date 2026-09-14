# Contributing to macos-health

Thanks for helping improve `macos-health`. The project is intentionally focused
on a fast, trustworthy Darwin dashboard, especially for Apple Silicon
MacBooks. Changes should keep metric provenance, failure behavior, privacy, and
self-overhead easy to understand.

## Development requirements

- macOS with Xcode Command Line Tools installed;
- Go 1.25 or newer;
- Apple Clang and CGO enabled;
- GoReleaser v2 only when testing release configuration.

Release builds target Darwin arm64 and amd64 with `CGO_ENABLED=1`. A Linux
cross-build is not a supported development path because the release depends on
the macOS SDK and Darwin CGO telemetry.

## Set up the project

```sh
git clone https://github.com/DrakeAFK/macos-health.git
cd macos-health
go mod download
make check
```

`make check` verifies formatting, runs `go vet`, executes race-enabled tests,
and builds a native Darwin binary in `dist/`.

Useful targets:

```sh
make fmt            # format Go packages
make fmt-check      # report formatting drift without modifying files
make test           # run tests once
make test-race      # run the race detector
make vuln           # scan for reachable Go vulnerabilities
make build          # build the native architecture
make build-all      # build Darwin arm64 and amd64 with Apple Clang
make release-check  # validate .goreleaser.yaml
make snapshot       # create an unpublished local release in dist/
make clean
```

Do not commit binaries or `dist/` output. Release assets are produced from tags
by GitHub Actions.

## Making a change

Keep collectors best-effort and bounded. A failed metric must be unavailable or
stale, never a successful-looking zero. New external commands must use an
absolute Apple system path, a fixed locale, a context deadline, and sanitized
output. Do not introduce root requirements, automatic permission prompts,
network calls or telemetry. Optional private Apple APIs are acceptable for
read-only, capability-tested native telemetry when the user-facing fallback,
privacy implications, and version limitations are documented and tested.

When adding or changing a metric:

1. Document its source, unit, cadence, and limitations in `docs/metrics.md`.
2. Separate command execution from parsing so parsers can use fixtures.
3. Add table tests for normal, missing, malformed, and changed macOS output.
4. Fuzz text parsers when malformed input could panic or allocate excessively.
5. Test the first-sample, counter-reset, cancellation, and stale-value paths.
6. Measure collector cost and avoid running slow-changing commands every second.

UI changes should be tested at wide, standard 80×24, narrow, and very short
terminal sizes. Include Unicode, combining characters, and control-character
inputs in rendering tests. The final view must remain within the reported
terminal width and height.

## Pull requests

Before opening a pull request:

```sh
make check
git diff --check
```

Describe the user-visible result, macOS and hardware used for manual testing,
and any unavailable metrics or permission differences you observed. Keep
unrelated cleanup out of focused fixes. Update documentation when behavior,
keys, output fields, thresholds, or installation steps change.

Security-sensitive reports should follow `SECURITY.md` instead of using a
public issue.

## Releases

Maintainers release from a clean, reviewed commit:

1. Update release notes and verify `make check`.
2. Run `make release-check` and, when useful, `make snapshot`.
3. Create an annotated semantic-version tag such as `v0.3.0`.
4. Push the tag.

The release workflow runs tests and GoReleaser, then publishes separate Darwin
arm64 and x86_64 archives plus `checksums.txt`. Never upload a locally built
replacement asset to an existing tag.
