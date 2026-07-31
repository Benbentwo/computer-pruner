# ComputerPruner

A cross-platform desktop maintenance tool for macOS and Windows. Today it is a disk space
analyzer with an interactive sunburst visualisation.

[![CI](https://github.com/Benbentwo/computer-pruner/actions/workflows/ci.yml/badge.svg)](https://github.com/Benbentwo/computer-pruner/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/Benbentwo/computer-pruner?sort=semver)](https://github.com/Benbentwo/computer-pruner/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/Benbentwo/computer-pruner)](go.mod)

---

## What it is

ComputerPruner answers the question every full disk raises: *what is actually using all that
space?* Point it at a volume or a folder, and it walks the tree, measures every file, and draws
the result as a sunburst you can click through. Directories you want to reclaim get staged in a
collector panel and moved to the Trash or Recycle Bin in one go.

It is built as a single native desktop application — a Go backend for the filesystem work, a
Svelte and D3 frontend for the visualisation, wrapped by [Wails v2](https://wails.io). There is
no server, no telemetry and no network access; everything happens on your machine.

Disk analysis is the first of four planned pillars. Startup-app management, a hardware and
specification inventory, and driver validation are on the roadmap but are **not implemented**.
See [docs/roadmap.md](docs/roadmap.md) for what each one would involve.

<!--
SCREENSHOT PLACEHOLDER
----------------------
Drop a PNG of the analyzer view (sunburst + sidebar + collector panel) at:

    docs/images/screenshot-analyzer.png

Create the docs/images/ directory if it does not exist yet, then replace this entire
comment block with:

    ![ComputerPruner analyzing a volume](docs/images/screenshot-analyzer.png)

Before you commit it: the sunburst renders real directory names. Crop or blur anything
you would not want on a public README — client names, employer names, and the username in
/Users/<you>.
-->

## Features

These work today.

**Volume and folder selection.** Mounted volumes are enumerated natively — the boot volume plus
everything under `/Volumes` on macOS, logical drives on Windows — each with capacity, free space
and a usage bar. You can also browse into any directory and scan just that subtree.

**Full-tree scanning with live progress.** The walk recurses the whole tree regardless of the
render depth, streaming a progress event every 200 entries so the overlay shows the current path,
item count, bytes accounted for and elapsed time. Symlinks are counted but never followed, which
keeps the totals honest — a link is not credited with the size of what it points at, and a link
to an ancestor cannot send the walk into a cycle.

**Interactive sunburst.** The tree is pruned before it crosses into the frontend (depth-capped,
at most 80 children per ring segment, anything under half a percent of its parent grouped into a
single "other small items" slice) so the chart stays responsive on a multi-million-file volume.
Clicking a segment drills in; a breadcrumb bar walks you back out.

**Sidebar listing.** The current directory's children, sortable by size, name or modification
date, each with its share of the parent as a percentage.

**Staged deletion.** Items are added to a collector panel and reviewed as a batch. The whole
batch is checked against the protected-path guard before anything is touched, then moved to the
Trash or Recycle Bin. See [Safety](#safety) below.

**Reveal and preview.** Open a path in Finder or Explorer with the item selected, or preview it
via QuickLook on macOS and the default file handler on Windows.

**Scan caching.** A finished scan is compressed and stored in a local [bbolt](https://github.com/etcd-io/bbolt)
database for 24 hours, so reopening a volume you already scanned is instant. Rescanning is always
available and always re-walks the disk.

Two things exist in the backend but have no user interface yet, and it is worth knowing which:
the permanent-delete operation (see [Safety](#safety)) and the preferences for scan depth and
exclusion paths. The preferences file is read and written correctly, but nothing in the UI edits
it, and the running application does not currently pass the preferences service to the scanner —
so scan depth and exclusions sit at their defaults regardless of what the file says.

## Install

Download the archive for your platform from the
[latest release](https://github.com/Benbentwo/computer-pruner/releases/latest):

| Platform | File |
| --- | --- |
| macOS (Apple Silicon and Intel) | `computer-pruner_<version>_darwin_universal.zip` |
| Windows (x64) | `computer-pruner_<version>_windows_amd64.zip` |
| Windows (ARM64) | `computer-pruner_<version>_windows_arm64.zip` |

Every release ships a `computer-pruner_<version>_checksums.txt`. To verify a download:

```
shasum -a 256 -c computer-pruner_<version>_checksums.txt --ignore-missing
```

### These builds are unsigned — here is what you will see, and what to do

ComputerPruner is not code-signed on either platform, and the macOS build is not notarized.
Signing certificates cost money and are tied to a personal identity; for a free tool with no
funding behind it, that trade has not been made yet. The consequence is that **both operating
systems will warn you the first time you open the app.** This is expected. It is not a sign that
anything is wrong with the download — but it does mean you are relying on the checksum above and
on your own read of the source, rather than on Apple or Microsoft having vouched for the binary.

**macOS.** Gatekeeper will say *"computer-pruner" cannot be opened because the developer cannot
be verified*, or on recent versions *"Apple could not verify 'computer-pruner' is free of
malware."* To open it anyway:

1. Unzip the archive and move `computer-pruner.app` to `/Applications`.
2. Right-click (or Control-click) the app and choose **Open**, then **Open** again in the dialog.
   Double-clicking will not offer this choice; the right-click menu is what unlocks it.
3. If macOS refuses outright with no *Open* button, go to
   **System Settings → Privacy & Security**, scroll to the Security section, and click
   **Open Anyway** next to the blocked-app message. Then repeat step 2.

If you would rather clear the quarantine flag directly:

```
xattr -dr com.apple.quarantine /Applications/computer-pruner.app
```

You only have to do this once per installed copy.

**Windows.** SmartScreen will show a blue *"Windows protected your PC"* dialog naming an
unrecognised publisher. Click **More info**, then **Run anyway**. If you unzipped with File
Explorer you may also need to right-click the `.exe`, choose **Properties**, tick **Unblock** at
the bottom of the General tab, and click OK.

### macOS Full Disk Access

Scanning system-managed locations — most of `~/Library`, mail stores, and anything under
`/Library` — requires Full Disk Access. Without it those directories read as empty and the totals
will be lower than reality. Grant it under **System Settings → Privacy & Security → Full Disk
Access**, add ComputerPruner, and restart the app.

## Build from source

### Prerequisites

- **Go 1.25.0 or newer.** The toolchain is pinned to 1.25.12 in `go.mod` and CI builds with
  exactly that.
- **Node.js 22.** The version CI uses; the frontend is built with Vite 6.
- **The Wails v2 CLI**, pinned to the runtime version in `go.mod`:

  ```
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
  ```

  Make sure `$(go env GOPATH)/bin` is on your `PATH`.
- **Platform toolchain.** Xcode Command Line Tools on macOS; the WebView2 runtime on Windows
  (present by default on Windows 11 and current Windows 10).

Wails cannot cross-compile: a macOS `.app` must be built on macOS and a Windows `.exe` on
Windows.

### Setup

```
git clone https://github.com/Benbentwo/computer-pruner.git
cd computer-pruner
./setup.sh
```

`setup.sh` checks that Go, Node, npm and the Wails CLI are present, runs `npm install` in
`frontend/`, resolves Go modules with `go mod tidy`, and finishes with `wails doctor`. Note that
it suggests installing the Wails CLI with `@latest`; prefer the pinned version above so your
local builds match the release pipeline.

The manual equivalent, if you would rather not run the script:

```
cd frontend && npm install && cd ..
go mod tidy
wails doctor
```

### Run and build

```
wails dev      # hot-reload dev mode; Svelte changes apply instantly, Go changes rebuild
wails build    # production binary -> build/bin/
```

`wails build` produces `build/bin/computer-pruner.app` on macOS and
`build/bin/computer-pruner.exe` on Windows.

Two notes for anyone poking at a fresh clone. `frontend/dist/` is gitignored but `main.go`
embeds it with `//go:embed all:frontend/dist`, and an embed pattern matching nothing is a
compile error — so the Go tooling will not even load the root package until the directory
exists. `./setup.sh` drops a placeholder `frontend/dist/index.html` there; the real bundle is
produced by `wails build`. And
`frontend/wailsjs/` — the generated TypeScript bindings — is gitignored; run
`wails generate module` to produce it if your editor complains about missing imports.

## Safety

ComputerPruner deletes files. Read this section before you run it.

**Deletes go to the Trash or Recycle Bin by default.** On macOS the item is handed to Finder via
AppleScript, which preserves the "Put Back" location; on Windows it goes through
`SHFileOperationW` with `FOF_ALLOWUNDO`, which is the same path Explorer uses. In both cases the
file is recoverable until you empty the bin. The collector panel in the UI only ever performs
this kind of delete, and it asks for confirmation first.

**A permanent-delete path exists.** `DeletePathsPermanently` is an exported backend method — an
unrecoverable `os.RemoveAll` — and it is present in the generated Wails bindings. No button in
the current UI calls it, but it is part of the application's surface and you should know it is
there. It is subject to exactly the same protected-path guard as the Trash path.

**A protected-path guard refuses system and user-data directories.** Every path in a delete batch
is validated *before* any destruction happens, so no file in a batch is removed if the guard is
going to reject a later one — and each path is checked again immediately before it is deleted.
The guard fails closed: an empty path, a relative path, an unexpanded `~` path, a filesystem or
volume root, a Windows path on a volume that cannot be reduced to a drive letter or a UNC share,
and a machine whose home directory cannot be resolved are all treated as protected. Matching is
done on whole path segments after resolving symlinks, so `/Users/bobby` is not mistaken for a
child of `/Users/bob` and a symlink into a protected tree cannot be used as a way around the
check. Comparison folds case on macOS and Windows and normalises Unicode, so an accented account
name does not quietly lose its protections.

Protection is applied at three different reaches, deliberately:

- **OS-owned trees are protected all the way down.** `/System`, `/Library`, `/Applications`,
  `/usr`, `/etc`, `/private`, `/cores`, `/Network` and friends on macOS; `%SystemRoot%`,
  `%ProgramFiles%`, `%ProgramFiles(x86)%` and `%ProgramData%` on Windows. Nothing inside them can
  be deleted. The same applies to a macOS system volume mounted under `/Volumes`, so a bootable
  clone is not treated as ordinary external storage.
- **Credential stores are protected all the way down.** `~/.ssh`, `~/.gnupg`, `~/.aws`, `~/.kube`
  and `~/Library/Keychains`. None of them is ever a disk-cleanup target and losing one is
  unrecoverable.
- **Containers of user data are protected as directories only.** Your home directory,
  `~/Library`, `~/Documents`, `~/Desktop`, `~/Downloads`, `~/Music`, `~/Movies`, `~/Pictures`,
  `~/Library/Caches`, `~/Library/Preferences`, `~/Library/Application Support`, `%APPDATA%`,
  `%LOCALAPPDATA%` and the Windows equivalents cannot be removed *themselves*, but their contents
  can. That is the whole point of the tool — refusing everything under `~/Downloads` would refuse
  the most useful thing it does. `/Users` and `/Volumes` (and `%SystemDrive%\Users`) additionally
  protect their immediate children, so you cannot delete another user's entire home directory.

Every one of these applies to **every account on the machine**, not just the one running the
app: another user's `~/Documents` is guarded exactly like yours. Ancestors count too — protecting
`~/Library/Application Support` is worthless if `~/Library` can be removed in one call, so every
ancestor of a protected location is protected as a directory in its own right.

**What the guard does not do.** It does not protect the *contents* of your Documents or Desktop
folder, it does not know which files matter to you, and it does not undo anything. It is a
backstop against catastrophic mistakes, not a substitute for reading the list in the collector
panel before you click delete. Have a backup. Its accepted limitations — including a residual
time-of-check/time-of-use window and the fact that `/Applications` is protected so thoroughly
that you cannot remove a `.app` bundle with this tool — are listed under "Known limitations" in
[SECURITY.md](SECURITY.md).

If you were running a build of this project from before the current release, see the Security
section of [CHANGELOG.md](CHANGELOG.md): the guard used to be substantially weaker than the above
describes.

## Project layout

| Path | What lives there |
| --- | --- |
| `main.go`, `app.go` | Wails entry point, window options, service wiring, directory dialogs and the folder browser |
| `internal/scanner/` | Filesystem walk, tree building, progress streaming, tree pruning, and the bbolt scan cache |
| `internal/volume/` | Mounted-volume enumeration and capacity reporting |
| `internal/fileops/` | Delete batching, trash and permanent delete, reveal, preview, file info |
| `internal/platform/` | Everything OS-specific: the protected-path policy, trash implementations, file manager, theme |
| `internal/preferences/` | Settings persistence and the single per-user config directory |
| `frontend/src/lib/components/` | Svelte components — sunburst, sidebar, breadcrumbs, collector, overlays |
| `frontend/src/lib/stores/` | Svelte writable stores holding the frontend's reactive state |
| `frontend/src/lib/utils/` | Wails binding wrappers, the event bridge, formatting and the colour scale |
| `frontend/src/lib/types/` | TypeScript interfaces mirroring the Go structs |
| `build/darwin/` | `Info.plist` templates for the macOS bundle |
| `docs/` | Architecture notes and the roadmap |
| `.github/workflows/` | CI, the tagged release pipeline, and Release Drafter |

`internal/platform` and `internal/volume` are split into build-tagged per-OS files
(`*_darwin.go`, `*_windows.go`, `*_other.go`). [docs/architecture.md](docs/architecture.md)
explains the layout and why the `_other` fallback exists.

## Contributing

Bug reports, ideas and pull requests are welcome. Start with
[CONTRIBUTING.md](CONTRIBUTING.md) — it covers the dev environment, the build-tag conventions, the
exact checks CI runs, and the commit-message convention the release tooling depends on.

One rule is worth repeating here: **any change that touches deletion or the protected-path logic
must come with tests.**

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md). Please do not open a public issue for
one.

## Links

- [Roadmap](docs/roadmap.md) — the four pillars, and what is speculative
- [Architecture](docs/architecture.md) — process model, data flow, packages, caching
- [Changelog](CHANGELOG.md)

## License

MIT. Copyright (c) 2026 Ben Smith. See [LICENSE](LICENSE).
