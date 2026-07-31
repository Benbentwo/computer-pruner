# Roadmap

ComputerPruner is planned as four pillars of computer maintenance. **One is built. Three are
not.**

This document exists so that the shape of the project is legible from the outside, and so that
anyone who wants to work on a future pillar has somewhere to start. It is a plan, not a promise,
and the further down the page you read the more speculative it gets. Sections are marked
accordingly.

| Pillar | Status |
| --- | --- |
| 1. Disk analysis | **Shipping now** |
| 2. Startup-app management | Planned — mechanisms understood, nothing built |
| 3. Hardware and specification inventory | Planned — mechanisms understood, nothing built |
| 4. Driver validation | Speculative — scope not settled |

Nothing in pillars 2 to 4 exists in the tree. There are no stub packages, no feature flags and no
half-wired UI. If you are reading the source looking for them, they are not there.

---

## Pillar 1 — Disk analysis (shipping)

Point ComputerPruner at a volume or a folder; it walks the tree, measures everything, draws a
sunburst, and lets you stage items for deletion to the Trash or Recycle Bin behind a
protected-path guard.

What that involves today is described in [architecture.md](architecture.md) and summarised in the
README. This pillar is complete enough to be useful and is the only thing the first release
covers.

### Remaining work inside this pillar

Ordered roughly by how much a user would notice.

- **Wire the preferences service into the scanner.** `ScannerService` reads scan depth and
  exclusion paths from a `PreferencesProvider`, and `NewScannerServiceWithPreferences` exists for
  exactly this, but `main.go` currently constructs the scanner with `NewScannerService()` — no
  provider. The settings are therefore read, written, normalised and honoured by the package, and
  ignored by the running application. This is a two-line change plus a test.
- **A settings UI.** There is no way to edit preferences from within the app; the JSON file is
  the only interface. Theme, default delete behaviour, scan depth and exclusion paths all need
  controls.
- **Lazy loading of collapsed branches.** The full tree stays in memory and `GetScanTree()`
  exposes it, but the frontend only ever sees the pruned copy. Drilling into a node that pruning
  collapsed should fetch its real children on demand instead of showing an `(other small items)`
  dead end.
- **A permanent-delete path in the UI.** `DeletePathsPermanently` exists on the backend and in
  the bindings with no caller. If it is going to be exposed it needs a deliberately awkward
  confirmation — typed confirmation, not a second OK button — and if it is not, it should be
  removed rather than left as an undocumented capability.
- **Windows-shaped frontend paths.** The volume-versus-folder decision in `App.svelte` tests for
  `/` and `/Volumes/`, and the folder browser splits paths on `/`. Both routes reach the same Go
  code so nothing is broken, but the browser's breadcrumb behaviour on `C:\Users\...` is wrong.
- **Code signing and notarization.** Releases are unsigned on both platforms. This is the single
  biggest barrier to a non-technical user actually running the tool, and — see below — it is also
  a hard prerequisite for the privileged parts of pillars 2 to 4.
- **Symlink and hardlink accounting.** Links are currently skipped entirely. Device-and-inode
  identity tracking would let them be shown without double-counting, but the identity concept
  differs enough on Windows that it is its own project.

---

## Pillar 2 — Startup-app management (planned)

**The goal.** Show everything that runs when you log in, where it comes from, and let you turn
off the things you did not ask for. This is the second most common reason a machine feels slow,
after a full disk, and both platforms scatter the answer across four or five unrelated
mechanisms that no built-in tool presents together.

**The governing design rule, before any mechanism: disable, do not delete.** Every mechanism
below has a reversible "off" state that the OS itself uses. ComputerPruner should use that state
and record what it changed, so that anything it turns off can be turned back on. Deleting a
launch agent plist or a registry value to disable an app is destructive and unnecessary.

### macOS mechanisms

- **`launchd` agents and daemons.** Property lists in four locations, and the distinction between
  them is the whole safety story:
  - `~/Library/LaunchAgents` — the user's own agents. Writable without elevation. This is where
    most third-party auto-start lives and is the primary target.
  - `/Library/LaunchAgents` and `/Library/LaunchDaemons` — installed for all users, typically by
    an application's installer package. **Requires administrator privileges to modify.**
  - `/System/Library/LaunchAgents` and `/System/Library/LaunchDaemons` — Apple's own. Protected
    by System Integrity Protection and must be presented as read-only, never as something the
    user can change.

  Inspection is `launchctl print gui/$(id -u)` and `launchctl print system`, or reading the
  plists directly. The reversible off switch is `launchctl disable gui/<uid>/<label>`, which
  writes to the per-user override database; the system domain equivalent needs root.

- **Login Items.** Two generations, and they need different handling. The legacy list — the one
  in System Settings → General → Login Items — is readable and writable through
  `osascript` against `System Events` (`login items`), with no elevation. Modern apps register
  through `SMAppService` and land in the Background Task Management database under
  `/var/db/com.apple.backgroundtaskmanagement/`. That database is **not** user-readable;
  `sfltool dumpbtm` requires root, its format is undocumented and Apple has changed it between
  releases. *Speculative:* a reliable read of the BTM database may simply not be achievable
  without root, in which case the honest answer is to show the legacy list plus the launchd
  agents and tell the user that App Store style background items are managed in System Settings.

- **Configuration profiles** (`profiles list`) can install managed login items on a
  corporate-managed Mac. Read-only, and worth surfacing so a user is not confused about why they
  cannot disable something.

### Windows mechanisms

- **Run and RunOnce registry keys.**
  - `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` and `RunOnce` — the current user's.
    Writable **without elevation**.
  - `HKLM\Software\Microsoft\Windows\CurrentVersion\Run`, `RunOnce`, and their
    `Wow6432Node` 32-bit views — machine-wide. **Requires administrator privileges.**
  - `HKCU\...\Explorer\StartupApproved\Run` is how Task Manager records the enabled/disabled state
    of an entry without removing it. This is the reversible off switch and is what ComputerPruner
    should write, so that Task Manager and ComputerPruner agree with each other.

- **Startup folders.**
  - `%AppData%\Microsoft\Windows\Start Menu\Programs\Startup` — per-user, no elevation.
  - `%ProgramData%\Microsoft\Windows\Start Menu\Programs\StartUp` — all users, **requires
    administrator privileges.** Note that entries here are usually `.lnk` shortcuts, so reporting
    a useful target means resolving the shortcut (`IShellLink`).

- **Task Scheduler.** Tasks with a logon or startup trigger are a very common and very
  under-inspected auto-start route. Enumeration is the Task Scheduler COM API (`ITaskService`)
  or `schtasks /query /fo csv /v`. Tasks under `\Microsoft\Windows\` are OS-owned and should be
  presented read-only. Disabling a user task needs no elevation; disabling a machine task does.

- **Services set to Automatic start.** Reachable through the Service Control Manager. Almost
  everything here **requires administrator privileges**, and much of it is load-bearing. This is
  the mechanism most likely to break a machine and is a candidate for read-only reporting only.

- **Packaged (MSIX/UWP) startup tasks** are a separate registration mechanism again, controlled
  through the app's manifest and the Settings app. *Speculative:* probably out of scope; report
  them, do not try to manage them.

### What the feature would need

A normalised model — source mechanism, display name, publisher, on-disk target, scope
(user/machine/system), current state, whether ComputerPruner can change it without elevation —
and a UI that sorts by "you almost certainly did not ask for this". Then a change log, so every
disable is recorded and reversible.

**Elevation.** Everything marked "requires administrator privileges" above needs an elevation
strategy that does not exist yet: a UAC-manifested helper or a `runas` relaunch on Windows, and a
privileged helper tool on macOS. The macOS route (`SMJobBless` or its modern replacement)
**requires a signed, notarized application**, which pillar 1's remaining work has to deliver
first. Until then, the machine-scope half of this pillar is blocked, and the user-scope half —
which is most of what actually matters — is not.

---

## Pillar 3 — Hardware and specification inventory (planned)

**The goal.** A single readable page answering "what is this machine, and is any part of it
unhealthy?" — model, CPU, memory and how much is installed versus how many slots are free, GPUs,
displays, storage devices with their health and wear, battery condition and cycle count, firmware
versions. Both operating systems can tell you all of this; neither presents it in one place, and
the macOS answer is buried in a modal dialog behind a button.

This is the lowest-risk pillar: it is read-only. Nothing here modifies the machine.

### macOS mechanisms

- **`sysctl`** for the cheap facts, readable without elevation: `hw.model`, `hw.memsize`,
  `hw.ncpu`, `hw.perflevel*` for performance/efficiency core counts on Apple Silicon,
  `machdep.cpu.brand_string` on Intel. Available directly through `golang.org/x/sys/unix` with no
  shelling out.
- **`system_profiler -json`** for the structured detail: `SPHardwareDataType`, `SPMemoryDataType`
  (slot population, which `sysctl` cannot give you), `SPDisplaysDataType`, `SPStorageDataType`,
  `SPNVMeDataType`, `SPPowerDataType` for battery cycle count and condition. JSON output makes
  this a clean parse rather than screen-scraping. No elevation required, but it is slow —
  seconds, not milliseconds — so it wants to be cached and refreshed on demand.
- **IOKit / the I/O Registry** for the same data without the subprocess, at the cost of cgo or a
  hand-rolled binding. Worth it only if the `system_profiler` latency proves unacceptable.
- **SMART data** is the awkward part. macOS exposes very little natively; the practical answer is
  `smartctl` from the smartmontools package, which most users do not have installed and which
  needs elevated access to the device. *Speculative:* report NVMe wear from `SPNVMeDataType`
  where available and treat full SMART as an optional enhancement if smartmontools is present.
- **`powermetrics`** gives thermal and power detail but **requires root** and is genuinely
  intrusive. Probably out of scope.

### Windows mechanisms

- **WMI / CIM** is the main road: `Win32_ComputerSystem`, `Win32_ComputerSystemProduct`,
  `Win32_Processor`, `Win32_PhysicalMemory` and `Win32_PhysicalMemoryArray` (installed modules and
  total slots), `Win32_VideoController`, `Win32_DiskDrive`, `Win32_LogicalDisk`,
  `Win32_BIOS`, `Win32_Battery`. All readable by a standard user. From Go this means either COM
  interop or shelling out to PowerShell's `Get-CimInstance -ConvertTo-Json`; the subprocess route
  is far simpler and the latency is acceptable for a page the user opens deliberately.
- **Direct Win32 calls** through `golang.org/x/sys/windows` for the facts that do not need WMI:
  `GetSystemInfo`, `GlobalMemoryStatusEx`, `GetSystemPowerStatus`, `GetDiskFreeSpaceEx` (already
  used by `internal/volume`).
- **SMART and drive health.** `MSStorageDriver_FailurePredictStatus` and
  `MSStorageDriver_FailurePredictData` in the `root\WMI` namespace, or `IOCTL_STORAGE_QUERY_PROPERTY`
  / `SMART_RCV_DRIVE_DATA` directly. **These require administrator privileges**, and vendor NVMe
  drivers vary in what they expose. Expect this to be the part that works on most machines and
  mysteriously does not on some.
- **Firmware and driver dates** from `Win32_BIOS` and the SetupAPI device properties, which
  overlaps with pillar 4.

### What the feature would need

A platform-neutral `SystemSpec` model with everything optional, because no two machines report
the same fields, and a UI that degrades gracefully — showing what it knows rather than blanks or
zeros. Collection should be lazy and cached; nothing here needs to run at startup.

*Speculative:* how much of the health story (drive wear, battery degradation, thermal throttling
history) can be told without elevation varies a lot by machine and by vendor. The design should
assume the unelevated subset is all it gets, and treat anything more as a bonus.

---

## Pillar 4 — Driver validation (speculative)

**The goal.** Find devices that are not working properly, drivers that are unsigned, ancient, or
supplied by Windows Update as a generic fallback when a vendor driver exists, and devices that
have no driver at all. On a Windows machine this is a real and frequent problem. On macOS it
barely exists as a category.

This is the least defined pillar and the one most likely to change shape or be dropped. It is
listed here because it was part of the original four-pillar idea, not because the scope is
settled.

**The strong recommendation, stated up front: ComputerPruner should report, not install.**
Installing, updating or rolling back a driver is the single most dangerous thing a maintenance
tool can do — it can leave a machine without a working display, network or boot disk, it requires
administrator privileges by definition, and it puts the maintainer in the position of
distributing binaries they did not build. Every "driver updater" product with a bad reputation
earned it here. Diagnosing and linking to the vendor's own download is most of the value at a
fraction of the risk.

### Windows mechanisms

- **Device enumeration** through SetupAPI (`SetupDiGetClassDevs`, `SetupDiEnumDeviceInfo`) or
  CfgMgr32 (`CM_Get_Device_ID_List`). Readable without elevation.
- **Problem codes.** `CM_Get_DevNode_Status` returns the `CM_PROB_*` code behind the yellow
  triangle in Device Manager — code 28 for "no driver installed", 43 for "device reported a
  problem", and so on. This is the most directly useful signal in the whole pillar.
- **Driver metadata** via `SetupDiGetDeviceRegistryProperty` and the driver key: provider,
  version, date, signer, INF name. Comparing the installed provider against the device's hardware
  ID is what identifies a generic Microsoft driver sitting where a vendor driver should be.
- **The driver store.** `pnputil /enum-drivers` lists staged driver packages including orphaned
  ones from uninstalled hardware. **Requires administrator privileges.**
- **Signature status.** An unsigned or test-signed driver is worth flagging loudly.

### macOS mechanisms

macOS has no user-facing driver model in the Windows sense, and Apple has spent several releases
removing what there was. What remains worth reporting:

- **Kernel extensions.** `kmutil showloaded` lists loaded kexts; third-party ones are the
  interesting subset. Legacy kexts are deprecated, are blocked by default, and require the user
  to approve them in Recovery with a reboot. Flagging a machine that still depends on a
  deprecated kext is genuinely useful before an OS upgrade. Some `kmutil` subcommands **require
  root.**
- **System extensions and DriverKit.** `systemextensionsctl list` shows the modern replacement —
  network extensions, endpoint security, USB and serial drivers. Read-only reporting is
  straightforward.
- **Nothing to update.** There is no macOS equivalent of "your driver is out of date": drivers
  ship with the OS or with the vendor's own application. The macOS half of this pillar is
  therefore inventory and deprecation warnings, and that is all it should be.

### Why this is marked speculative

Three open questions, none of which have been answered:

1. Is read-only reporting enough to be worth building, given that Device Manager already shows
   problem codes to anyone who knows to look?
2. Where does the "a newer driver exists" comparison come from? Any answer involves either a
   network call to a catalogue — which the project currently does not make, anywhere — or
   scraping vendor sites, which is fragile and ages badly.
3. Does the macOS half justify calling this a cross-platform pillar at all, or is it a
   Windows feature with a small macOS inventory attached?

Until those have answers, this stays a paragraph in a roadmap.

---

## Cross-cutting prerequisites

Three things gate more than one pillar.

**Code signing and notarization.** Blocks the privileged half of pillars 2 and 4, because the
macOS privileged-helper mechanism requires a signed application. Also the biggest usability
barrier for pillar 1 today.

**An elevation strategy.** Nothing in ComputerPruner currently elevates, and the security model
in [SECURITY.md](SECURITY.md) says so plainly. Introducing elevation is a significant change to
that model and deserves its own design discussion, not an incidental commit inside a feature.

**An undo and audit log.** Pillar 1 leans on the Trash for reversibility. Pillars 2 and 4 have no
equivalent, so anything that changes machine state needs to record what it changed, when, and how
to put it back — and that record should exist before the first state-changing feature ships, not
after.

## Non-goals

Worth stating, because tools in this category drift into them:

- **No registry cleaning, no "PC optimisation", no junk-file heuristics.** ComputerPruner shows
  you what is on your disk and lets you decide. It does not have opinions about which of your
  files are junk.
- **No telemetry, no analytics, no crash reporting to a server.** The application makes no
  outbound network connections and should keep it that way. Any future feature needing the
  network must be explicitly opt-in and say exactly what it sends.
- **No background service or scheduled scanning.** It runs when you open it.
- **No Linux release.** The code compiles and is tested on Linux so contributors and CI can work
  there, but macOS and Windows are the shipping targets and the `_other` build-tag fallbacks are
  deliberately minimal.

## Contributing to a pillar

If you want to build one of these, open an issue first. A pillar is a large surface and the
mechanisms above are notes rather than a specification — the design conversation is worth having
before the code is written. [CONTRIBUTING.md](../CONTRIBUTING.md) covers the conventions.
