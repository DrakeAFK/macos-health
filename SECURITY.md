# Security

Report vulnerabilities through
[GitHub's private reporting form](https://github.com/DrakeAFK/macos-health/security/advisories/new)
if it is enabled. Otherwise, contact the owner through the contact details on
[their profile](https://github.com/DrakeAFK). Do not post exploit details or
private captures in a public issue.

Include the affected commit or version, macOS version, reproduction steps,
and impact. Remove personal data that is not needed to reproduce the problem.
Fixes target the current development branch; older versions may need an upgrade.

## What the tool accesses

Collection reads Darwin system interfaces and Apple tools. It does not invoke
`sudo`, install a daemon, request Full Disk Access, or make outbound network
connections. Native sensors are enabled by default and use private IOReport
and AppleSMC APIs. Use `--sensors=false` to disable them.

Snapshots retain process executable names, PIDs, parent PIDs, and start times,
not full argument lists or environments. Host identity and local addresses can
also appear. `--redact` removes host identity, addresses, and listener details;
it does not remove process names or PIDs.

Recording and preference saving write local files. Recordings use mode 0600,
refuse to replace existing files, and stop at 256 MiB. `--serve` opens read-only
HTTP endpoints on a literal loopback address. These endpoints have no
authentication; other local processes can read them. Use `--redact` when needed.

Terminal escape injection, command execution, parser crashes, unsafe file
handling, and release tampering are relevant security reports.

## Release integrity

Build from source until packaged releases are available. Local GoReleaser
packaging creates architecture-specific archives and SHA-256 checksums;
maintainers upload them manually. There is no automated release workflow.

When downloading a release archive, calculate its digest:

```sh
shasum -a 256 macos-health_VERSION_darwin_ARCH.tar.gz
```

Compare the full digest and filename with `checksums.txt` from that release.
Checksums detect a mismatch; they do not authenticate an archive if the archive
and checksum file were both replaced. Archives are not Developer ID signed or
Apple-notarized. Gatekeeper checks are separate from the permissions used for
metric collection.
