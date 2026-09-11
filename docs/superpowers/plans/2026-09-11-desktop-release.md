# Desktop release implementation and evidence

Goal: package Windows x64, macOS ARM64/Intel, Linux x64/ARM64 using a new Loadout signing identity. Android/iOS excluded by user clarification.

Architecture: keep Tauri and shared CLI bridge; native runners build their own bundles. A release-only overlay requires signed updater artifacts; normal local deb builds remain possible without private keys. macOS uses an ad-hoc bundle seal, as in LambChat. This is not Apple notarization or Windows Authenticode. Keep application identifier com.loadout.desktop.

User authorized implementation in the current workspace and a new signing identity. User subsequently authorized a scoped commit/push and remote builds, with no public release. Private key stays outside the repository, with a copy in the repository's GitHub Actions secret; public key is tracked. No automatic updater UI is added.

- [x] Add behavioral release preflight tests: reject branch names, version drift, missing signing key; accept matching stable version.
- [x] Implement preflight and run RED/GREEN checks.
- [x] Generate independent signing identity and macOS icon; add release overlay, native platform matrix and artifact checks. Run native commands/UI tests before bundling; verify packaged bridge on native architecture.
- [x] Require complete build jobs before publishing the draft release; preserve CLI/container jobs.
- [x] Build local Linux artifacts; verify extracted bridge; run make check and record actual limitations.

Configuration and generated icons are verified through real Tauri builds and workflow lint. No source-string tests replace behavior tests.

## Evidence

- RED: `node --test tests/release-preflight.test.mjs`: 3 failures (missing expected exception for invalid tag, version drift and absent key), 1 pass. Stub compiled; failures were behavior assertions.
- GREEN: same command after implementation: 4 passed. REFACTOR: Biome formatted both files; same 4 tests passed again.
- `pnpm exec biome check scripts/release-preflight.mjs tests/release-preflight.test.mjs desktop/tauri.conf.json desktop/tauri.release.conf.json`: passed.
- Official Tauri icon CLI generated `desktop/icons/icon.icns` from the existing SVG (other generated platform assets stayed in ignored build output).
- `make lint-desktop test-desktop`: exit 0; UI typecheck/build, clippy/rustfmt, seven UI behavior tests, shared command tests and native bridge passed. Log: `build/desktop-release-tests.log`.
- `TAURI_SIGNING_PRIVATE_KEY=/home/yangyang/.config/loadout-signing/release.key APPIMAGE_EXTRACT_AND_RUN=1 pnpm --dir desktop/ui tauri build --ci --bundles deb,rpm,appimage --config tauri.release.conf.json -- --locked`: exit 0; all three Linux x64 installers and signatures generated. Log: `build/desktop-signed-build.log`.
- Build preparation found beforeBuildCommand runs in `desktop/ui` (observed by printing cwd), so final hook is `pnpm build`. A local interactive signing attempt required a password prompt; final command uses `--ci` and completed unattended. These are configuration corrections, not RED evidence.
- Copies in `build/desktop-release/` were signed with Tauri CLI and verified with distro Minisign 0.11 against the tracked public key: all three accepted. Appending a byte to a temporary copy was rejected; verifying against LambChat's public key was rejected. SHA256SUMS includes all three installers and signatures.
- deb extracted with `dpkg-deb --extract` into TemporaryDirectory; AppImage extracted with `--appimage-extract` into another TemporaryDirectory. For each, `LOADOUT_DESKTOP_TEST_BIN=<extracted usr/bin/loadout-desktop> cargo test --manifest-path desktop/Cargo.toml --locked --test native_bridge`: exit 0, 1 passed. No actual user client config or system install modified.
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/release.yml`: passed. Older actionlint 1.7.7 lacked the official macos-15-intel runner label; verified label against GitHub runner docs and used current stable linter.
- Independent code review found macOS Bash 3.2 does not support unused globstar, and PowerShell could swallow an early pnpm test failure. Removed globstar and made the multi-command test step explicit Bash with `set -euo pipefail`.
- `node --env-file=.env build/full-check.mjs` ran `make check` twice against configured PG/Redis: exit 2 in repository-wide Biome lint. Other concurrently modified web/server files had formatting/lint errors; retained those edits. This is NOT a full-harness pass. Scoped diff whitespace check passed.
- GitHub `gh secret list --json name` confirms `TAURI_SIGNING_PRIVATE_KEY`. Private file stored outside repository (0600, parent 0700); no private value logged. At local verification time no commit/push/tag/public release or remote workflow run had occurred; user subsequently authorized a scoped build branch and build-only workflow dispatch.

## Remaining platform acceptance

Windows/macOS/Linux ARM64 workflows are configured but have not run remotely. No Windows local Rust toolchain or macOS host is available in this session. Windows MSI extraction and macOS bundle-seal checks remain unexecuted here. RPM package generated and signature verified, but RPM installation not exercised. No actual GUI/tray interaction, Gatekeeper/SmartScreen or clean-machine installation was claimed. No Apple notarization or Windows Authenticode certificate was created. User authorized remote build-only validation. Public publishing remains unauthorized.
