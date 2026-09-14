# Contributing

Keep collection overhead low and readings traceable to their source. A failed
metric should be unavailable or stale, never a successful-looking zero.

## Local development

Use macOS with Go 1.25 or newer and Xcode Command Line Tools. Native telemetry
requires CGO and the macOS SDK; Linux cross-builds are not supported.

```sh
go mod download
make check
```

`make check` checks formatting, runs `go vet` and race-enabled tests, and builds
`dist/macos-health`.

Other targets:

```sh
make fmt            # format Go code
make test           # run tests once
make build-all      # build Darwin arm64 and amd64 locally
make bench          # UI benchmarks
make vuln           # download and run the pinned Go vulnerability scanner
make release-check  # validate GoReleaser config; requires GoReleaser v2
make snapshot       # create local archives/checksums; replaces dist/
```

Do not commit binaries, `dist/`, or diagnostic recordings. Dependency updates
are reviewed manually. Run `make vuln` when checking dependencies and test any
updates with `make check`.

This project does not use GitHub Actions, scheduled Dependabot updates, or
hosted build/release jobs. Keep checks local and releases manual. Repository
owners should also disable Actions in GitHub's repository settings; deleting
workflow files does not change that setting.

## Changes

For a new metric, document its source, units, sampling interval, and limits in
[docs/metrics.md](docs/metrics.md). Test missing or malformed output, the first
sample, counter resets, cancellation, and stale readings. Cache slow-changing
data instead of running expensive commands every second.

External commands need an absolute Apple system path, fixed locale, deadline,
and sanitized output. Keep collection unprivileged and local. Private Apple
APIs need availability checks and a documented fallback.

Check UI changes at 80×24, wide, narrow, and very short terminal sizes. Test
Unicode and terminal control characters when changing text rendering.

Before submitting:

```sh
make check
git diff --check
```

Describe the behavior changed, how you tested it, and any hardware-specific
limits. Update documentation when flags, keys, output, or thresholds change.
Use [SECURITY.md](SECURITY.md) for sensitive reports.

## Manual releases

GoReleaser v2 packages archives on your Mac. Its release publisher is disabled,
and there are no signing or notarization hooks. No GitHub token is needed to
build archives.

1. Update the changelog and commit the intended release changes.
2. From that clean commit, run `make check` and `make release-check`.
3. Create an annotated version tag, for example `git tag -a v0.3.0 -m 'v0.3.0'`.
4. Run `goreleaser release --clean --skip=publish` locally. This replaces `dist/`
   with Darwin arm64/x86_64 archives and `checksums.txt`.
5. Inspect the archives and test the binaries on the target Macs. Include the
   root license and third-party notices.
6. When ready to publish, push that specific tag and manually attach the two
   archives and `checksums.txt` to a GitHub Release for it.

Pushing a tag does not build or publish anything. Keep published assets tied
to their tagged source; make a new release for corrections.
