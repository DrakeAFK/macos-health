# Third-party acknowledgments

The root [LICENSE](LICENSE) covers macos-health. The license under
`docs/licenses/` belongs to the upstream project credited below; it is not a
second copy of this project's license. Keep both notices in source and release
archives.

The read-only IOReport/AppleSMC bridge in `internal/stats/silicon_native_darwin.c`
uses ABI layouts and channel/sensor conventions documented by
[vladkens/macmon](https://github.com/vladkens/macmon), MIT licensed.
See [the retained license](docs/licenses/macmon-MIT.txt).
The bridge is an independent C implementation; macmon is not a runtime dependency.

Go module dependencies retain their respective licenses in their source modules.
