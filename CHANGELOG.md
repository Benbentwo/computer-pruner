# Changelog

All notable changes to ComputerPruner are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). ComputerPruner is pre-1.0:
until 1.0.0 the public surface — the Wails-bound service methods, the JSON shapes they return and
the on-disk formats — may change in a minor release.

There has been no tagged release yet. Everything below is the work that will make up the first
one.

## [Unreleased]

This pass takes an internal, macOS-only prototype called "diskanalyzer" and turns it into a
cross-platform, publishable open-source project: a real Windows implementation, a fixed and
tested protected-path guard, a CI and release pipeline, and documentation.

### Added

- **Windows support.** A complete platform layer: the protected-path list is resolved from the
  environment (`%SystemRoot%`, `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%ProgramData%`,
  `%APPDATA%`, `%LOCALAPPDATA%`, `%USERPROFILE%`) with literal fallbacks, so a machine with
  Windows on a drive other than `C:` is guarded correctly. Deletes go to the Recycle Bin through
  `SHFileOperationW` with `FOF_ALLOWUNDO`, bound directly against `shell32.dll` rather than
  pulling in a third-party trash library. Reveal opens Explorer with the item selected, preview
  hands the file to its default handler, and the app appearance is read from the
  `AppsUseLightTheme` registry value.
- **Windows volume enumeration.** Logical drives are listed via `GetLogicalDriveStrings` and
  sized via `GetDiskFreeSpaceEx`, with Explorer-style display names (`Local Disk (C:)` for an
  unlabelled volume). Drives that cannot be inspected — an empty removable bay, an unreadable
  mount — are skipped rather than failing the whole listing.
- **Per-OS build-tag layout.** `internal/platform` and `internal/volume` are now split into
  `*_darwin.go`, `*_windows.go` and `*_other.go` files with matching exported APIs.
  `internal/scanner` gained `paths_caseinsensitive.go` / `paths_casesensitive.go` for the
  filesystem case-sensitivity constant. The `_other` fallbacks exist so the tree compiles, vets
  and runs its tests on Linux CI; they fail closed rather than faking behaviour.
- **A test suite.** New tests across `internal/fileops`, `internal/platform`,
  `internal/preferences`, `internal/scanner` (paths, pruning, cache, scan lifecycle) and
  `internal/volume`. They are table-driven over synthesised paths and `t.TempDir()`, so the
  Windows and macOS rules are both exercised from a single Linux host.
- **Continuous integration.** `.github/workflows/ci.yml` runs five jobs: `gofmt` plus `go vet`
  for both `GOOS=darwin` and `GOOS=windows`; `go test ./internal/... -race` with a coverage
  artifact; `govulncheck`; an `npm ci` and advisory `npm audit`; and a real `wails build` on
  `macos-latest` and `windows-latest` with `svelte-check` gating it. Workflow permissions are
  least-privilege throughout.
- **Release pipeline.** `.github/workflows/release.yml` builds on native runners (macOS
  universal, Windows amd64, Windows arm64) because Wails cannot cross-compile, then stages the
  artifacts for GoReleaser, which archives, checksums and publishes to GitHub Releases.
  `.goreleaser.yaml` uses meta archives rather than the Pro-only `prebuilt` builder. Release
  notes come from Release Drafter, configured in `.github/release-drafter.yml` with a
  conventional-commit autolabeler and label-driven semantic versioning.
- **Repository furniture.** Dependabot configuration, structured bug-report and feature-request
  issue templates (both of which warn about leaking private directory names in screenshots), a
  pull request template that requires a test for any change to a destructive operation,
  `CODEOWNERS` and `.editorconfig`.
- **Scan cache hardening.** The cache is now schema-versioned: the version is part of every key,
  entries written by a different version are dropped on open, and a stale `gob` layout can never
  reach a decoder that no longer matches it. Entries are capped at 64 MiB compressed and 256 MiB
  decoded, decompression runs through a reader that errors rather than presenting the cap as a
  clean EOF, and the decode is wrapped in a `recover` because `gob` is not hardened against
  malformed input.
- **A recursion depth limit** in the filesystem walk, so a pathologically deep tree reports as an
  empty directory instead of exhausting the goroutine stack.
- **Open-source documentation.** `LICENSE` (MIT), `README.md`, `CONTRIBUTING.md`, `SECURITY.md`,
  this changelog, `docs/architecture.md` and `docs/roadmap.md`.

### Changed

- **Renamed the project to ComputerPruner.** The Go module is now
  `github.com/Benbentwo/computer-pruner`, the binary is `computer-pruner`, and the window title
  is "ComputerPruner". All internal imports use the full module path.
- **Upgraded the toolchain and dependencies.** Go 1.25 (toolchain pinned to 1.25.12), Wails
  v2.13.0, bbolt v1.5.0, and current `golang.org/x/*` modules. On the frontend: Vite 6,
  TypeScript 5.9, svelte-check 4, and a `postcss` override.
- **Unified the on-disk state directory.** Preferences and the scan cache now both go through
  `preferences.ConfigDir()`, which uses `os.UserConfigDir` — `%AppData%\computer-pruner` on
  Windows, `~/Library/Application Support/computer-pruner` on macOS — falling back to
  `$HOME/.config` and returning an error rather than writing somewhere unpredictable. The
  directory is created mode `0700` and the preferences file written mode `0600`; the settings
  record which directories you browse and exclude, which is nobody else's business.
- **Reworked the scan lifecycle.** Each scan carries a monotonically increasing epoch and owns
  its state for its whole life; a superseded goroutine's writes are dropped. `CancelScan` now
  only signals, and a subsequent `StartScan` waits for the previous goroutine to drain (bounded
  at 30 seconds) instead of racing it.
- **Symlinks are counted but never followed** during a walk, and contribute zero bytes. This
  under-reports a home directory whose large folders are links to another volume, but it
  guarantees the walk is acyclic and that no bytes are attributed to a directory that does not
  own them. For a tool whose output drives deletion decisions, double-counting is the worse
  failure.
- **Removed the default exclusion paths.** The defaults used to be `/System`, `/Library` and
  `/Applications`. Nothing read the list, so they had no effect; now that the scanner honours
  exclusions, shipping those defaults would make a whole-disk macOS scan silently omit most of
  the disk. Exclusions are an explicit opt-in and default to empty.
- **Preference values are normalised on both read and write.** The scan depth is clamped to
  `[1, 64]` (a zero or negative value means "unset" and becomes the default of 8), and the
  exclusion list is trimmed, cleaned, de-duplicated and stripped of entries the scanner cannot
  act on — blanks, relative paths, and anything starting with `~`. A tilde is dropped rather than
  guessed at, because home-directory expansion belongs to `internal/platform`.
- **Deprecated `platform.GetProtectedPaths`** in favour of `platform.ProtectedPaths`. The old
  name still works and now returns fully expanded absolute paths rather than a mix of absolute
  and `~/`-prefixed strings.
- **Documented the safety model in the code.** `internal/platform`, `internal/fileops` and
  `internal/scanner` carry package-level comments describing the guard, the fail-closed
  behaviour, and the single-writer scan invariant.

### Fixed

- **A data race between a cancelled scan and its replacement.** `CancelScan` used to clear the
  `scanning` flag itself, which let a new scan start while the old goroutine was still walking
  the filesystem. Both then wrote the shared scan tree and progress, so a cancelled scan could
  overwrite the results of the scan that replaced it. Only the scan goroutine clears the flag
  now, as it exits.
- **Double-counted directories in the "(other small items)" summary node.** Pruning added both
  the child's own `DirCount` and a separate increment for the child itself in a way that counted
  the child's subtree twice, inflating the directory count shown for grouped items.
- **Progress totals that grew faster than the disk.** Only leaf files contribute to the running
  byte total; a directory's size is the sum of its descendants, so adding directory totals as the
  recursion unwound counted the same bytes once per level. On completion the reported total now
  equals the root node's size.
- **Preferences and the scan cache disagreeing about where they live.** Preferences hard-coded
  `$HOME/.config` while the cache used `os.UserConfigDir`. On macOS those are two different
  directories, and on Windows `$HOME/.config` is simply the wrong place.
- **Scan-depth and exclusion preferences that were never read.** The scanner now takes a snapshot
  of the user's settings at the start of each scan, so a change takes effect on the next scan
  without a restart. (The running application does not yet pass the preferences service to the
  scanner — see Known issues.)
- **AppleScript injection surface in the macOS trash path.** Filenames are escaped in the correct
  order — backslashes doubled first, quotes second, because escaping quotes first introduces
  backslashes that the backslash pass then doubles — and names containing raw control characters,
  which AppleScript string literals cannot represent, are rejected with a clear error instead of
  reaching `osascript` and coming back as an opaque parse failure.
- **Unsigned wraparound in used-space arithmetic.** Free space can exceed the reported total on
  quota-backed and sparse filesystems; the subtraction is now clamped at zero instead of wrapping
  to an enormous number.
- **A corrupt scan cache taking the application down.** A schema mismatch could panic the `gob`
  decoder; a bad entry now degrades to "no cache", is deleted, and the scan proceeds.
- **Stale-toolchain build metadata.** `wails.json`, the embedded asset path and the macOS bundle
  templates all refer to `computer-pruner` consistently.

### Security

- **The protected-path guard did not protect anything it was supposed to. This is the most
  important entry in this changelog.**

  Two independent defects, both in the code path that decides whether a delete is allowed:

  1. **Tilde paths were never expanded.** The protected-path list contained literal strings such
     as `~/Documents`, `~/Desktop`, `~/Downloads` and `~/Library/*`. The delete guard compared
     the requested path against those strings as-is. A real path is always fully resolved —
     `/Users/example/Documents` — and it never equals the literal three-character-plus string
     `~/Documents`. Every home-directory entry in the list was therefore inert: the guard
     believed it was protecting them and was protecting none of them.
  2. **The comparison used exact string equality.** Even for the entries that were spelled
     absolutely, such as `/System`, the guard only fired when the requested path was character-
     for-character identical. Nothing *inside* a protected root was covered, so
     `/System/Library/...` passed a check that `/System` would have failed.

  **Were you affected?** If you ran a build of this project from before this change and used the
  delete feature, then yes — in the sense that the guard you were relying on was not doing its
  job. Concretely, a delete request for your home directory, `~/Documents`, `~/Desktop`,
  `~/Downloads`, any `~/Library` subdirectory, or for any path *inside* `/System`, `/Library`,
  `/Applications` or `/usr`, would have been accepted and carried out. Two things limited the
  damage: the user interface only ever offered "Move to Trash", so anything deleted this way was
  recoverable from the Trash until it was emptied, and macOS's own permissions still refused
  writes to system-integrity-protected locations regardless of what this application asked for.
  Nothing here required an attacker; the failure mode was an ordinary mis-click being carried out
  instead of refused.

  **The fix.** The guard is now boundary-aware, symlink-resolving and case-insensitive on the
  platforms whose filesystems are:

  - Home-relative entries are joined against the real home directory when the list is built. A
    literal `~` can no longer leave `internal/platform`, and the pure list builders take the home
    directory as a parameter so the property is unit-testable for every target OS from one host.
  - Containment is tested on whole path segments, never on raw string prefixes, with `.` and `..`
    normalised first. `/Users/bobby` is correctly not inside `/Users/bob`.
  - Symlinks are resolved before comparison, including the parent directory of a path that does
    not exist yet, so a link pointing into a protected tree is not a bypass.
  - Comparison folds case on macOS and Windows, matching how APFS and NTFS actually behave. This
    protects marginally more than the filesystem strictly requires, which is the right direction
    to err in.
  - Protection has an explicit reach per entry. OS-owned trees are protected all the way down;
    containers of user data (the home directory, `~/Documents`, `%LOCALAPPDATA%`, …) protect the
    directory itself but not its contents, which is what the application exists to clean up.
    `/Users`, `/Volumes` and `%SystemDrive%\Users` additionally protect their immediate children,
    so one user cannot delete another user's whole home directory.
  - The guard fails closed. An empty path, a relative path, an unexpanded `~` path, a filesystem
    or volume root, or an unresolvable home directory all report as protected.
  - Whole batches are validated before any deletion runs, so a rejection late in the list cannot
    be preceded by files that have already been destroyed.

  **Second pass: an adversarial review of the fix above.** The rewritten guard closed the two
  original defects, and a review that specifically tried to defeat it found nine further routes
  through. All of them are now closed, and each has a regression test that fails if the fix is
  reverted (`internal/platform/protected_bypass_test.go`):

  - **The parent of a protected directory was itself deletable.** `~/Library/Application
    Support`, `~/Library/Preferences` and `~/Library/Caches` were protected; `~/Library` was not,
    and depth does not propagate upwards. One delete of `~/Library` took out Keychains, Mail,
    Messages, Safari data and all three "protected" children. `%USERPROFILE%\AppData` had the
    identical shape. Every ancestor of every entry is now protected. `~/Library`, `~/.ssh`,
    `~/.gnupg`, `~/.aws`, `~/.kube`, `~/Library/Keychains`, `~/Movies`, `/cores` and `/Network`
    were also missing from the list outright and have been added; the credential directories are
    protected all the way down.
  - **Alternate Windows volume spellings evaded the entire list.** `\\?\C:\`, `\\.\C:\`,
    `\??\C:\`, `\\localhost\C$\` and `\\?\UNC\…` are all accepted by `filepath.IsAbs`
    and by `os.RemoveAll`, and `filepath.EvalSymlinks` preserves the prefix verbatim, so the
    volume never matched and nothing was protected. The same defect covered `subst` drives and
    mapped network drives, which needed no crafting at all. Volumes are now canonicalised, and
    anything that cannot be reduced to a drive letter or a UNC share fails closed.
  - **Unicode normalisation was not handled.** For any account whose short name contains a
    non-ASCII character, `José` from `$HOME` (NFC) and `José` from `readdir` (NFD) are
    byte-different and case folding does not reconcile them, so the home-relative protections
    evaporated. Segments are now folded to NFC before comparison, with an ASCII fast path.
  - **Windows environment variables replaced the literal defaults rather than adding to them.**
    Launching from a shortcut with `SystemRoot=D:\Decoy` removed `C:\Windows`, `C:\Program
    Files` and `C:\ProgramData` from the protected list, and `IsProtected` then returned "not
    protected" with no error, so the fail-closed branch never ran. The list is now the union.
  - **A bogus `$HOME` relocated every home-relative protection.** `os.UserHomeDir()` on darwin is
    literally `$HOME` with no validation and no error. Per-user locations are now templated
    across every account under the users root (`/Users/*/Documents`, …), so tampering with
    `$HOME` on a stock machine achieves nothing.
  - **Other users' data was unprotected**, asymmetrically with your own. The same templating
    fixes this: another account's `~/Documents` is now guarded exactly like yours.
  - **A mounted macOS system volume was deletable.** `/Volumes` was protected one level deep, so
    a bootable clone at `/Volumes/Backup HD` had its entire `System`, `usr`, `private` and
    `Library` classified as removable. `/Volumes/*/…` system directories are now protected.
  - **Windows 8.3 short names and trailing-dot/space segments** matched no protected entry and
    were caught only as a side effect of `EvalSymlinks` succeeding. Trailing dots and spaces are
    now stripped per segment, and a path still holding an unresolved 8.3 alias fails closed.
  - **A batch verdict could be stale by the time it was used.** Validating the whole batch up
    front meant path *N* was destroyed on a verdict `N-1` full deletes old, and a protected tree
    renamed into place in the interim was destroyed. Each path is now re-checked immediately
    before it is destroyed. A residual race remains and is documented in
    [SECURITY.md](SECURITY.md) under "Known limitations".

  Separately, the scanner's shield badge in the UI was computed by an *exact* match against the
  protected-path list, discarding each entry's depth, so the badge and the delete policy
  disagreed in both directions. Both now derive from the same `platform.ProtectedEntries()`
  policy, and a test pins them together.

  All of this is covered by tests, and a test is now mandatory for any future change to deletion
  or protected-path logic.

- **Dependency advisories.** Advisories affecting `golang.org/x/crypto` and `golang.org/x/net`
  were investigated and found unreachable from ComputerPruner's call graph by `govulncheck`. Both
  modules were upgraded anyway, and `govulncheck` now runs on every push and pull request. The
  remaining `npm audit` findings are devDependency advisories in the Svelte 4 server-side
  rendering chain; ComputerPruner never server-side-renders, so they are surfaced as annotations
  rather than treated as blocking.

- **No credentials were ever committed.** The git history was audited for secrets and is clean.
  `.gitignore` now excludes `.env` files and the per-developer `.claude/settings.local.json`,
  which had previously leaked a personal path.

- **Least-privilege automation.** Both workflows declare `permissions: contents: read` at the top
  level; only the final release job is granted `contents: write`, and only so GoReleaser can
  create the release and upload assets.

### Known issues

Documented rather than fixed in this pass:

- There is no user interface for editing the scan depth and exclusion path settings. The values
  are read, written and honoured end to end — the shipped application does hand the preferences
  service to the scanner — but the only way to change them today is to edit `preferences.json`.
- `/Applications` is protected all the way down, so ComputerPruner cannot remove a `.app` bundle.
  See "Known limitations" in [SECURITY.md](SECURITY.md).
- `DeletePathsPermanently` is exported and present in the generated bindings but no UI calls it.
- The volume-versus-folder decision in the frontend and the folder browser's path splitting are
  both POSIX-shaped, which is cosmetically wrong on Windows even though both routes reach the
  same backend scan.
- Releases are unsigned and un-notarized. See [SECURITY.md](SECURITY.md).

[Unreleased]: https://github.com/Benbentwo/computer-pruner/commits/main
