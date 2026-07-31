// Package volume enumerates the disk volumes mounted on the host machine and
// reports their capacity.
//
// The package is split across build-tagged files:
//
//	volume.go         — portable types, the service facade and pure helpers
//	volume_darwin.go  — statfs(2) based enumeration of "/" and /Volumes/*
//	volume_windows.go — logical-drive enumeration via golang.org/x/sys/windows
//	volume_other.go   — a stub so the package still builds on other platforms
//
// Each platform file supplies two unexported entry points, listVolumes and
// statVolume, which the exported methods below dispatch to. Only the disk
// analyzer pillar is implemented today; the planned startup-app, hardware
// inventory and driver-validation pillars are expected to grow their own
// packages rather than extend this one.
package volume

import (
	"context"
	"strings"
	"unicode/utf16"
)

// VolumeInfo represents information about a disk volume.
//
// The JSON tags are part of the contract with the Svelte frontend and the
// generated Wails bindings (frontend/src/lib/types/index.ts). Do not rename
// them.
type VolumeInfo struct {
	Name       string `json:"name"`
	MountPoint string `json:"mountPoint"`
	FSType     string `json:"fsType"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
}

// VolumeService provides disk volume information and operations
type VolumeService struct {
	ctx context.Context
}

// NewVolumeService creates a new instance of VolumeService
func NewVolumeService() *VolumeService {
	return &VolumeService{}
}

// SetContext sets the wails runtime context for the service
func (v *VolumeService) SetContext(ctx context.Context) {
	v.ctx = ctx
}

// ListVolumes returns information about all mounted volumes on the system.
//
// Volumes that cannot be inspected — an unreadable mount, or a removable drive
// with no media in it — are skipped silently rather than failing the whole
// listing. The result is never nil so the frontend can iterate it directly.
func (v *VolumeService) ListVolumes() []VolumeInfo {
	volumes := listVolumes()
	if volumes == nil {
		return []VolumeInfo{}
	}
	return volumes
}

// GetVolumeInfo returns detailed information about a specific volume.
//
// mountPoint is a filesystem path on darwin ("/", "/Volumes/Backup") and a
// drive root on windows ("C:\"). A drive specifier without a trailing
// separator ("C:") is accepted and normalised.
func (v *VolumeService) GetVolumeInfo(mountPoint string) (*VolumeInfo, error) {
	return statVolume(mountPoint)
}

// defaultRootVolumeName is the fallback display name for the darwin boot
// volume when its real name cannot be determined.
const defaultRootVolumeName = "Macintosh HD"

// rootVolumeName returns the display name to use for the darwin root volume,
// falling back to defaultRootVolumeName when no real name was detected.
func rootVolumeName(detected string) string {
	if name := strings.TrimSpace(detected); name != "" {
		return name
	}
	return defaultRootVolumeName
}

// driveDisplayName builds the display name for a windows drive. label is the
// volume label reported by GetVolumeInformation and may be empty (unlabelled
// volumes are common); root is a drive root such as `C:\`. With no label we
// mirror Explorer and show "Local Disk (C:)".
func driveDisplayName(label, root string) string {
	if name := strings.TrimSpace(label); name != "" {
		return name
	}
	if letter := driveLetter(root); letter != "" {
		return "Local Disk (" + letter + ")"
	}
	return "Local Disk"
}

// driveLetter extracts the "C:" prefix from a windows path. It returns an
// empty string for paths that are not drive-qualified (a UNC path, say).
func driveLetter(path string) string {
	if len(path) < 2 || path[1] != ':' {
		return ""
	}
	c := path[0]
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		return strings.ToUpper(string(c)) + ":"
	}
	return ""
}

// normalizeDriveRoot turns a windows drive specifier into a canonical drive
// root: "c:" and "C:\" both become `C:\`. Anything that is not a bare drive
// specifier (a deeper path, a UNC share) is returned unchanged, since the
// Win32 volume APIs accept directory paths too.
func normalizeDriveRoot(path string) string {
	letter := driveLetter(path)
	if letter == "" {
		return path
	}
	rest := path[2:]
	if rest == "" || rest == `\` || rest == "/" {
		return letter + `\`
	}
	return path
}

// calcUsedBytes returns total-free, clamped at zero. Free space can exceed the
// reported total on quota-backed or sparse filesystems, and unsigned
// subtraction would wrap to an enormous number if we did not guard it.
func calcUsedBytes(total, free uint64) uint64 {
	if free >= total {
		return 0
	}
	return total - free
}

// int8SliceToString converts a NUL-terminated []int8 — the shape of the fixed
// size char arrays in darwin's statfs struct — to a Go string.
func int8SliceToString(s []int8) string {
	b := make([]byte, 0, len(s))
	for _, v := range s {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}

// splitUTF16Strings splits a Win32 "multi-string" — a run of NUL-terminated
// UTF-16 strings ending in an extra NUL, as returned by
// GetLogicalDriveStrings — into Go strings. Empty entries are dropped.
func splitUTF16Strings(buf []uint16) []string {
	var out []string
	start := 0
	for i, c := range buf {
		if c != 0 {
			continue
		}
		if i > start {
			out = append(out, string(utf16.Decode(buf[start:i])))
		}
		start = i + 1
	}
	if start < len(buf) {
		out = append(out, string(utf16.Decode(buf[start:])))
	}
	return out
}
