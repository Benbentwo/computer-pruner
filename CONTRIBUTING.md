# Contributing to ComputerPruner

Thanks for wanting to help. This document covers what you need to get a working development
environment, the conventions the codebase uses, and the exact checks CI will run against your
pull request.

ComputerPruner is a small project with a single maintainer. Issues and pull requests get
attention, but not instantly — see [SECURITY.md](SECURITY.md) for response-time expectations on
the security side.

## Before you start on something large

Open an issue first. Two things in particular are worth agreeing on before code is written:

- **New pillars.** Only the disk analyzer is implemented. Startup-app management, hardware
  inventory and driver validation are described in [docs/roadmap.md](docs/roadmap.md) but are
  deliberately unbuilt. A pull request that starts one of them without discussion is likely to
  need substantial rework.
- **New dependencies.** The Go dependency tree is small on purpose — the Windows Recycle Bin
  support, for instance, is a two-screen `syscall` binding rather than a third-party trash
  library. Adding a module means adding supply-chain surface, and that needs a reason.

## Development environment

**Prerequisites**

- Go 1.25.0 or newer. `go.mod` pins the toolchain to 1.25.12 and CI uses exactly that.
- Node.js 22.
- The Wails v2 CLI, pinned to the runtime version in `go.mod`:

  ```
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
  ```

- Xcode Command Line Tools on macOS, or the WebView2 runtime on Windows.

**Setup**

```
./setup.sh
```

This verifies the four tools above, runs `npm install` in `frontend/`, runs `go mod tidy`, and
finishes with `wails doctor`. It suggests `@latest` for the Wails CLI; prefer the pinned version
so your builds match the release pipeline.

**Day-to-day**

```
wails dev      # hot-reload; Svelte updates instantly, Go changes trigger a rebuild
wails build    # production binary -> build/bin/
```

Two gotchas on a fresh clone:

- `frontend/dist/` is the compiled bundle and is gitignored, but `main.go` embeds it with
  `//go:embed all:frontend/dist`. An embed pattern that matches nothing is a compile error, so
  on a clean clone *any* command that loads the root package — `go build ./...`,
  `go vet ./...`, `govulncheck ./...` — fails before it reads a line of Go. `./setup.sh` creates
  a placeholder `frontend/dist/index.html` for you; if you skip setup, create it by hand:

  ```bash
  mkdir -p frontend/dist && printf '<!doctype html><html><body></body></html>\n' > frontend/dist/index.html
  ```

  Do not delete it. `wails build` and `vite build` overwrite it with the real bundle.
- `frontend/wailsjs/` is generated and gitignored. If your editor reports missing modules under
  `../../../wailsjs/`, run `wails generate module`. Never hand-edit anything in that directory;
  it is regenerated from the Go service structs on every build.

Wails cannot cross-compile. A macOS `.app` must be built on macOS and a Windows `.exe` on
Windows. You can still typecheck and test the whole tree from any host — see below.

## Platform build tags

`internal/platform` and `internal/volume` are split into per-OS files behind build tags. The
convention is:

| File suffix | Build tag | Purpose |
| --- | --- | --- |
| `_darwin.go` | `//go:build darwin` | The real macOS implementation |
| `_windows.go` | `//go:build windows` | The real Windows implementation |
| `_other.go` | `//go:build !darwin && !windows` | Fallback for every other OS |

`internal/scanner` uses the same idea in a smaller form:
`paths_caseinsensitive.go` (`//go:build darwin || windows`) and `paths_casesensitive.go`
(`//go:build !darwin && !windows`) each define the `pathsAreCaseInsensitive` constant.

**Add a `_darwin`/`_windows` file when the behaviour genuinely differs at the OS boundary** — a
different syscall, a different shell-out, a different set of protected directories, different
filesystem case semantics. Do not branch on `runtime.GOOS` inline in shared code. The build-tag
split is what makes each implementation compile-checked in isolation and keeps the shared code
free of dead branches.

**`_other` exists so the tree still compiles, vets and tests on Linux.** ComputerPruner does not
ship for Linux and never will, but CI's `lint-go`, `test` and `govulncheck` jobs all run on
`ubuntu-latest`, and contributors run tests on whatever machine they have. Without an `_other`
file the package would simply not build there and the test suite would be unrunnable. The
fallback is best-effort and must **fail closed**: `platform_other.go` refuses to move a file to
the trash when the `gio` helper is absent rather than silently deleting it permanently, and
`volume_other.go` returns no volumes rather than guessing. Never make `_other` the place where a
missing platform behaviour is faked.

**Keep the exported API identical across all three files.** Every platform file supplies the same
unexported entry points (`platformProtectedPaths`, `listVolumes`, `statVolume`) or the same
exported functions (`MoveToTrash`, `RevealInFileManager`, `PreviewFile`, `GetSystemTheme`). If
you add a function to one, add it to all three, or `GOOS=windows go vet ./...` will tell you
about it.

**Write portable tests.** Most of the platform logic is deliberately factored into pure helpers
that take the home directory, the environment lookup, the path separator and the case-sensitivity
flag as parameters — `buildDarwinProtectedPaths`, `buildWindowsProtectedPaths`,
`withinDepthSep` — precisely so the Windows rules can be exercised from a Linux test host. Follow
that pattern: table-driven tests over synthesised paths and `t.TempDir()`, never a test that
needs a real macOS or Windows machine.

## Running the checks CI runs

CI is defined in [`.github/workflows/ci.yml`](.github/workflows/ci.yml) and has five jobs. You
can reproduce four of them locally on any OS.

**Formatting.** `gofmt -l` exits 0 even when it finds problems, so CI checks for non-empty
output. The equivalent for you is simply:

```
gofmt -l .        # must print nothing
gofmt -w .        # to fix
```

**Vet, every shipping target.** A single-GOOS vet leaves half of `internal/platform` and
`internal/volume` unchecked, and `windows/arm64` is a release target in its own right. Run all
three:

```
GOOS=darwin  GOARCH=arm64 go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=arm64 go vet ./...
```

**Tests, with the race detector.** Only `./internal/...` is tested; the root package is Wails
wiring that needs a windowing system. `-race` is what catches concurrent writes in the scanner.

```
go test ./internal/... -race
```

CI additionally writes `-coverprofile=coverage.out` and uploads it as an artifact.

**Vulnerability scan.** `govulncheck` does call-graph reachability analysis, so it only reports
advisories your code can actually reach. `GOOS=darwin` is set so the darwin-tagged code is
analysed too.

```
go install golang.org/x/vuln/cmd/govulncheck@latest
GOOS=darwin GOARCH=arm64 govulncheck ./...
```

**Frontend.**

```
cd frontend
npm ci
npm audit --audit-level=high
```

`npm audit` is **non-blocking** in CI and should be treated as informational. Everything
currently flagged is a devDependency advisory in the Svelte 4 server-side-rendering chain;
ComputerPruner never server-side-renders — the bundle is compiled at build time and served from
Go's embedded filesystem — and the only fix npm offers is a breaking bump to Svelte 5. Surface
new advisories, do not block unrelated work on them.

**Typecheck.** `svelte-check` is *not* run in the Linux frontend job, because
`frontend/src/lib/utils/wailsBindings.ts` imports the generated bindings that a clean checkout
does not have. It runs in the `build` job on real macOS and Windows runners, right after
`wails generate module`, where it is a hard gate. Locally:

```
wails generate module
cd frontend && npm run check
```

**Build.** The `build` job is the only one that produces a binary, on `macos-latest` and
`windows-latest`. You cannot reproduce it on Linux; don't try.

## Branches

- `main` is stable and always builds. Everything lands there through a pull request.
- `feature/*` for new functionality, branched off `main`.
- `fix/*` for bug fixes, branched off `main`.

Git worktrees are encouraged for parallel work. Rebase onto `main` rather than merging `main` in.

## Commit and pull request titles

Release notes are generated by [Release Drafter](.github/release-drafter.yml), whose autolabeler
reads your **branch name** and **pull request title** and applies the labels that decide both the
changelog section and the next version number. Getting the prefix right is the difference between
a change appearing under "🚀 Features" and appearing under "🔀 Other Changes".

Use conventional-commit prefixes, imperative mood, concise:

| Prefix | Also matches branch | Lands under | Version bump |
| --- | --- | --- | --- |
| `feat:` / `feat(scope):` | `feat/`, `feature/` | 🚀 Features | minor |
| `fix:` | `fix/`, `bugfix/`, `hotfix/` | 🐛 Bug Fixes | patch |
| `security:` / `sec:` | `sec/`, `security/` | 🔒 Security | patch |
| `docs:` | `docs/`, `doc/`, or touching `docs/**` or `*.md` | 📚 Documentation | patch |
| `chore(deps):` / `build(deps):` | `deps/`, `dependabot/`, or touching `go.mod`, `go.sum`, `frontend/package*.json` | 📦 Dependencies | patch |
| `chore:` `refactor:` `test:` `ci:` `build:` `style:` `perf:` | matching branch prefixes | 🧰 Maintenance | patch |

A breaking change is marked with `!` before the colon — `feat!:` or `fix(scanner)!:` — or by
putting `BREAKING CHANGE` in the pull request body. Either one labels the PR `breaking-change`
and resolves the next version to a major bump.

Examples, taken from the shape of real changes in this repo:

```
feat(volume): enumerate Windows logical drives
fix(scanner): stop a cancelled scan from overwriting its replacement
security(platform): expand tilde paths before the protected-path check
chore(deps): bump golang.org/x/crypto to v0.54.0
docs: describe the build-tag layout
```

The autolabeler is not first-match — every rule that matches applies — so a `feat/...` branch
with a `!` in the title correctly picks up both `feature` and `breaking-change`. Labels can also
be applied by hand if the automation gets it wrong, and `skip-changelog` keeps a PR out of the
notes entirely.

## The one hard rule

**Any change that touches deletion or the protected-path logic must come with tests.**

Concretely, that means any change to:

- `internal/fileops/` — `DeletePaths`, `DeletePathsPermanently`, `deleteBatch`, or anything they
  call;
- `internal/platform/protected_paths.go` — the protected-path lists or the depth semantics;
- `IsProtected`, `isProtectedAgainst`, `resolutionCandidates`, `withinDepth*`, `splitPathSep` or
  `isFilesystemRoot` in `internal/platform/platform.go`.

This is not a style preference. The bug that motivated most of the current design shipped in a
build where the guard looked correct and did nothing: tilde paths were never expanded, so every
home-directory entry in the protected list was compared as the literal string `~/Documents` and
never matched anything, and the comparison used exact string equality so nothing *inside* a
protected root was covered either. A single table-driven test asserting that
`/Users/someone/Documents` is protected would have caught it on the day it was written.

The seams are already in place to make this easy. `internal/fileops` indirects `checkProtected`,
`moveToTrash` and `removeAll` through package-level variables so a test can prove a protected
path is skipped without touching a real file. `internal/platform` indirects `protectedSource` and
`userHomeDir` for the same reason. Use them; production code must never reassign them.

When you add a case, add it in both directions: assert that the dangerous path is refused **and**
that a legitimate path nearby is still allowed. A guard that refuses everything is as broken as
one that refuses nothing — it just fails more visibly.

`internal/platform/protected_bypass_test.go` is the regression suite for every bypass the guard
has actually had. Each test in it is verified to fail when you revert the fix it covers, and it
is written so the Windows rules run on a Linux host — the comparison primitives (`matchDepthSep`,
`isAbsSep`, `splitPathSep`, `refuseOnSyntax`) all take the separator and the case rule as
parameters. Add to that file rather than starting a new one, and keep the "fails without the fix"
property: a regression test that passes against the bug is worse than no test, because it makes
the bug look covered.

## Pull request checklist

The [pull request template](.github/pull_request_template.md) asks you to confirm:

- `gofmt -l .` is clean, and `go vet ./...` passes for `darwin/arm64`, `windows/amd64` and
  `windows/arm64` (all three are release targets; see the verification block above).
- `go test ./internal/...` passes.
- Any change to a destructive operation comes with a test.
- No private file paths, usernames or personal directory names appear in the diff, the
  description, or any attached screenshot. This tool renders your filesystem; screenshots leak
  more than people expect.

Tick the platform you actually verified on. "Not platform-specific" is a legitimate answer when
the change is covered by the typecheck and the test suite.

## License

By contributing you agree that your contributions are licensed under the MIT License, the same
terms as the rest of the project. See [LICENSE](LICENSE).
