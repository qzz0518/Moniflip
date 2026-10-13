# Contributing to Moniflip

Thanks for helping. Bug reports, monitor compatibility reports and focused
pull requests are all welcome. Issues and PRs can be written in English or
Chinese.

## Build

You need Go 1.27.1 or later. The MyGo CLI is pinned in `go.mod` as a tool, so
there is nothing else to install.

```sh
go tool mygo dev      # run "Moniflip Dev" with live reload
go test ./...
```

The dev app has its own name, identifier and settings, so it does not touch
an installed Moniflip.

To build release-style apps without the maintainer's Developer ID (ad hoc
signature, no disk image, no notarization):

```sh
go tool mygo build -platform darwin/arm64,windows/amd64 -sign - -skip-dmg -skip-notarize
```

- `build/darwin-arm64/Moniflip.app` runs on the Mac that built it.
- `build/windows-amd64/` holds `Moniflip.exe` and, if NSIS is installed
  (`brew install makensis` on macOS), the installer
  `Moniflip Setup <version> amd64.exe`.

This build is the same app as an installed Moniflip: it shares its
settings, Apple Home pairing and single-instance lock, and offers updates
from the official releases.

Everything cross-compiles from one Mac; no cgo is involved. Official
releases are built with `scripts/release.sh`, which finds the maintainer's
Developer ID identity in the keychain and needs the maintainer's
notarization and update keys.

## Before you open a pull request

- `gofmt -l .` prints nothing.
- `GOOS=darwin GOARCH=arm64 go vet ./...` and
  `GOOS=windows GOARCH=amd64 go vet ./...` pass. CI runs the same checks.
- `go mod tidy -diff` prints nothing, and every new dependency is credited in
  THIRD-PARTY-NOTICES.md (a table row plus its license text and copyright
  line); CI checks both.
- `go test ./...` passes. Tests must never talk to real monitors: keep DDC/CI
  calls behind the platform layer and test the logic around them with fakes.
  CI runs on machines without displays.
- **Test on real hardware.** Say in the PR which monitors, ports or docks,
  OS and chip you tested, and on which computer the app was running.
- UI text exists in Chinese and English; update both.
- Keep pull requests small and about one thing.

## DDC/CI writes

Moniflip writes exactly one VCP code: `0x60`, input source. Other codes
(power, factory reset, OSD lock, brightness and so on) behave differently on
every monitor and can leave one in a state that needs the OSD or a power
cycle to recover. Pull requests that write any other VCP code will not be
merged without an issue discussing it first. Reading VCP codes is fine.

## Reporting monitor compatibility

DDC/CI support varies by monitor, firmware, cable, port and dock. Use the
[Monitor report](https://github.com/qzz0518/Moniflip/issues/new?template=monitor-report.yml) form, both for
monitors that work and for ones that don't, and paste the diagnostics:
Settings → General → Diagnostics → Copy (设置 → 通用 → 诊断信息 → 复制),
or `moniflip diagnose`. They list the detected monitors and the input values
they report, which is what is needed to add support; monitor serial numbers
are masked.

## License

By contributing you agree that your contributions are licensed under the
[MIT License](LICENSE).
