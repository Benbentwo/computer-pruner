# Security Policy

ComputerPruner is a desktop application that reads your entire filesystem and can delete files
from it. Security reports are taken seriously.

## Reporting a vulnerability

**Please do not open a public issue for a security problem.**

Use one of these two channels:

1. **GitHub private security advisories** (preferred). Go to
   <https://github.com/Benbentwo/computer-pruner/security/advisories/new> and file a draft
   advisory. This keeps the discussion private, gives us a place to work on a fix together, and
   produces a CVE and a published advisory when it is resolved.
2. **Email** <ben.smith.developer@gmail.com>. Put "ComputerPruner security" in the subject so it
   does not get lost.

A useful report includes the version or commit you tested, the operating system, what an attacker
would need in order to exploit it, and the smallest reproduction you can manage. If the
reproduction involves file paths, synthesise them — `/Users/example/...` is fine and keeps your
own directory names out of the report.

## What to expect

ComputerPruner is maintained by one person in their spare time. These are honest targets, not a
service-level agreement:

| | Target |
| --- | --- |
| Acknowledgement that the report was received | within 5 business days |
| Initial assessment — is it a real issue, and how bad | within 14 days |
| Fix released for a high-severity issue | within 30 days of confirmation |
| Fix released for a lower-severity issue | in the next convenient release |

If you have not heard anything after 14 days, send a follow-up; it means the message was missed,
not ignored.

Coordinated disclosure is appreciated. A 90-day window before public disclosure is the default
assumption, and can be shortened by agreement once a fix is out. Reporters are credited in the
release notes and the advisory unless they ask not to be.

There is no bug bounty. There is no money in this project.

## Supported versions

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Anything older | No |

ComputerPruner is pre-1.0. Only the most recent release is supported: fixes go out as a new
release, and there are no backports to earlier tags. If you are running a build from source,
"supported" means the current `main`.

## Security model

Understanding what ComputerPruner is and is not protecting you from matters more than any single
bug report.

### The application runs with your full privileges

ComputerPruner is a normal desktop application launched by you. It inherits every permission your
user account has. On macOS, if you have granted it Full Disk Access — which the README recommends
so that `~/Library` and similar locations can be measured — it can read essentially every file on
the machine. It does not request administrator or root privileges, it does not install a helper
tool, it does not run a background service, and it does not elevate.

There is no privilege boundary inside the application. The Go backend and the WebView frontend
are the same process and the same trust domain; the Wails bindings are a function-call bridge,
not a sandbox. Anything the frontend can ask for, the backend will do with your full permissions.

### It can permanently delete files

Two destructive operations are exposed on the backend and present in the generated bindings:

- `DeletePaths` moves items to the Trash (macOS, via Finder) or the Recycle Bin (Windows, via
  `SHFileOperationW` with `FOF_ALLOWUNDO`). Recoverable until you empty the bin. This is what the
  user interface calls.
- `DeletePathsPermanently` is an `os.RemoveAll`. Unrecoverable. Nothing in the current UI invokes
  it, but it is part of the application's surface and it exists.

Both go through the same guard. There is no undo inside the application.

### Filenames on disk are untrusted input

This is the least obvious part of the model and the one most likely to produce a real
vulnerability.

ComputerPruner's job is to walk directories you did not create and display what it finds. A
filename can be almost anything: on macOS, any byte sequence except `/` and NUL, including
newlines, quotes, backslashes, control characters and right-to-left overrides. Those names flow
into three places where naive handling is dangerous:

- **The macOS trash path shells out to `osascript`.** A filename is embedded in an AppleScript
  string literal. `internal/platform/applescript.go` doubles backslashes before escaping quotes —
  the order matters — and rejects names containing raw control characters outright rather than
  letting them reach the interpreter. Anything that gets an unescaped quote or a newline into
  that literal is an injection bug; report it.
- **The frontend renders names in a WebView.** Svelte escapes interpolated text by default. Any
  use of `{@html}` on a filesystem-derived string is a cross-site-scripting bug in a context
  where "the site" has full backend access.
- **Path comparison decides whether a delete is allowed.** Path handling is done on whole
  segments, never on raw string prefixes, and `..` is resolved before comparison. A crafted path
  that slips past the guard is the most serious class of bug in this project.

### The protected-path guard is the primary safety control

Everything destructive funnels through `platform.IsProtected`. It is designed to fail closed: an
empty path, a relative path, an unexpanded `~` path, a filesystem or volume root, a Windows path
on a volume that cannot be reduced to a drive letter or a UNC share, a Windows path left holding
an unresolved 8.3 short name, and a machine whose home directory cannot be resolved are all
reported as protected.

What the comparison does:

- **Whole segments, never string prefixes.** `/Users/bobby` is not inside `/Users/bob`.
- **Symlinks are resolved first**, including up through however many missing components a path
  has, so a link — or a link several levels above a path that does not exist yet — is not a
  bypass.
- **Case-insensitive on macOS and Windows**, matching how APFS and NTFS actually behave.
- **Unicode-normalised.** `José` in NFC and `José` in NFD are byte-different and case folding
  does not reconcile them. macOS hands you one form from `$HOME` and the other from `readdir`, so
  both are folded to NFC before comparison.
- **Windows volumes are canonicalised.** `\\?\C:\`, `\\.\C:\`, `\??\C:\`,
  `\\host\C$\` and `\\?\UNC\...` all reduce to the volume they actually name, and trailing
  dots and spaces are stripped from each segment the way Windows strips them on open.
- **Every ancestor of a protected entry is protected too.** Protecting
  `~/Library/Application Support` while leaving `~/Library` open would have made the whole scheme
  pointless.
- **Per-user locations are templated across every account**, not just the one the process is
  running as, so another user's `~/Documents` is guarded exactly like your own — and tampering
  with `$HOME` cannot relocate the protections away from the real accounts.
- **Environment variables add to the Windows list, they never replace it.** `%SystemRoot%` is
  attacker-controllable by whatever launched the process; treating it as an override let a
  shortcut shrink the protected list to nothing.

Batches are validated in full before any deletion runs, so a rejected path later in the list
cannot be preceded by files that were already destroyed — and each surviving path is checked
*again* immediately before it is destroyed, so it is never acted on with a verdict that is
several deletes old.

The guard protects OS-owned trees all the way down, and containers of user data (your home
directory, `~/Documents`, `~/Downloads`, `%LOCALAPPDATA%`, and so on) as directories only — their
contents remain deletable, because that is the function of the application. Credential stores
(`~/.ssh`, `~/.gnupg`, `~/.aws`, `~/.kube`, `~/Library/Keychains`) are the exception: they are
protected all the way down, because none of them is ever a legitimate disk-cleanup target.

It is a backstop against catastrophic mistakes. It is not a permissions system, it does not know
which of your files matter, and it is not a substitute for a backup.

A previous version of this guard was ineffective in exactly the way that matters most; see the
Security section of [CHANGELOG.md](CHANGELOG.md) for what was wrong and who was affected.

### Known limitations

These are accepted, understood gaps in the guard. Reporting one of them is welcome, but it will
be closed as known unless you can show it is worse than described.

- **A residual time-of-check/time-of-use window.** Each path is re-validated immediately before
  it is destroyed, but "immediately before" is not "atomically with". Someone who can already
  write inside a directory you are cleaning can, in principle, rename a protected tree into place
  between the check and the syscall. Closing this completely needs fd-relative deletion
  (`openat`/`unlinkat` with `O_NOFOLLOW`), which the macOS trash path — an AppleScript round-trip
  through Finder — cannot express at all. An attacker who can win this race can already write to
  your files directly.
- **The macOS trash primitive follows symlinks.** `POSIX file … as alias` resolves the link, so
  trashing a symlink trashes its target. The guard resolves symlinks too and therefore classifies
  the *target*, so this is not a bypass — but it is surprising behaviour, and it interacts with
  the race above.
- **`$HOME` is still trusted for a home outside the users root.** A network or relocated home is
  protected only because `os.UserHomeDir()` reported it. If that value is wrong, that specific
  home is unguarded. Every account under `/Users` (resp. `%SystemDrive%\Users`) is guarded
  regardless, by template, so tampering with `$HOME` on a stock machine achieves nothing.
- **Windows drives other than `C:` and the ones the environment names are not enumerated.** A
  second Windows installation on `E:` has its `E:\Windows` unguarded unless an environment
  variable points at it. Enumerating fixed drives needs a Windows API call the pure-Go path
  layer deliberately avoids.
- **`\\host\C$\…` is treated as the local `C:`.** Which machine an administrative share names
  cannot be decided from the string, so it is assumed to be this one. That over-protects a
  genuinely remote admin share, which is the safe direction, but it does mean you cannot use
  ComputerPruner to clean `\\otherpc\C$\Windows\Temp`.
- **Volume-GUID and device paths are refused outright.** `\\?\Volume{…}\…` and
  `\\?\GLOBALROOT\Device\…` name real directories that the protected list cannot reason
  about, so they fail closed. A drive mounted only at a volume GUID path cannot be cleaned.
- **`/Applications` is protected all the way down**, so ComputerPruner cannot remove a `.app`
  bundle — arguably the most common macOS cleanup action. Loosening it is a product decision, not
  a security one, and it has not been taken. Drag the bundle to the Trash in Finder instead.
- **Some paths are over-protected.** `/System/Volumes/Data/Users/…` is refused via `/System`, and
  a directory literally named `*` collides with the per-user template's wildcard. Both err in the
  safe direction and neither has been worked around.
- **The Windows and macOS code paths are unit-tested but not integration-tested on their own
  operating systems.** The rules are written as separator-parameterised pure functions so a Linux
  CI runner exercises both, and `go vet` runs for `darwin/arm64`, `windows/amd64` and
  `windows/arm64`. What CI cannot do is run the resulting binary on a real Mac or a real Windows
  box. Behaviour that depends on the actual filesystem — `EvalSymlinks` on an 8.3 alias, Finder's
  trash semantics, `SHFileOperationW` — is reasoned about from the standard library source, not
  observed.

### Local state

Preferences and the scan cache live in one per-user directory (`%AppData%\computer-pruner` on
Windows, `~/Library/Application Support/computer-pruner` on macOS), created mode `0700`, with the
preferences file written mode `0600`. The scan cache contains a full directory listing of
whatever you scanned, which is sensitive — it is a map of your machine. The cache is gzip plus
`gob` and is decoded under a size cap with a `recover`, because `gob` is not hardened against
hostile input; entries written by an older schema version are dropped rather than decoded.

### Out of scope

- **Unsigned binaries.** ComputerPruner releases are **not code-signed on either platform and the
  macOS build is not notarized**. This is a deliberate, documented trade-off, not an oversight.
  It means macOS Gatekeeper and Windows SmartScreen will warn on first launch, and it means the
  integrity of a download rests on the published `checksums.txt` rather than on a platform
  vendor's signature. Verify your download. A report that "the binary is unsigned" is
  acknowledged here and will be closed as known.
- **No network, no telemetry, no auto-update.** The application makes no outbound connections.
  There is no update channel to hijack — and equally, no mechanism to push a fix to you
  automatically. You have to download new releases yourself.
- **An attacker who already has code execution as your user.** They can delete your files
  directly; they do not need this application.
- **Advisories in dependencies that are not reachable.** CI runs `govulncheck`, which does
  call-graph reachability analysis. Unreachable advisories are still worth upgrading away from
  and generally are, but they are not treated as vulnerabilities in ComputerPruner.
- **The Svelte 4 devDependency SSR advisories.** ComputerPruner never server-side-renders; the
  bundle is compiled at build time and served from Go's embedded filesystem. `npm audit` is
  advisory-only in CI for this reason.

## Credentials

No credentials, tokens or keys have ever been committed to this repository, and the git history
has been checked. If you believe you have found one, report it through the channels above and
treat it as live until told otherwise.
