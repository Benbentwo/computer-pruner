//go:build windows

package volume

import (
	"sync"

	"golang.org/x/sys/windows"
)

// Drive types returned by GetDriveType. golang.org/x/sys/windows does not
// export these constants, so they are declared here.
//
// Reference: https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-getdrivetypew
const (
	driveUnknown   uint32 = 0 // The drive type cannot be determined.
	driveNoRootDir uint32 = 1 // The root path is invalid, e.g. no volume mounted.
	driveRemovable uint32 = 2 // USB stick, card reader, floppy.
	driveFixed     uint32 = 3 // Internal or external hard disk / SSD.
	driveRemote    uint32 = 4 // Network share.
	driveCDROM     uint32 = 5 // Optical drive.
	driveRAMDisk   uint32 = 6 // RAM disk.
)

// errorModeMu serialises the process-wide SetErrorMode juggling in
// withHardErrorsSuppressed.
var errorModeMu sync.Mutex

// withHardErrorsSuppressed runs fn with SEM_FAILCRITICALERRORS set, so that
// probing a removable drive with no media in it fails with an error code
// instead of popping the "There is no disk in the drive" system dialog. The
// previous error mode is restored afterwards.
func withHardErrorsSuppressed(fn func()) {
	errorModeMu.Lock()
	defer errorModeMu.Unlock()

	previous := windows.SetErrorMode(windows.SEM_FAILCRITICALERRORS)
	defer windows.SetErrorMode(previous)

	fn()
}

// listVolumes enumerates the logical drives and reports on the ones worth
// showing in a disk analyzer: fixed and removable local drives. Optical
// drives, network shares and drive letters with nothing mounted are excluded,
// as are drives that cannot be queried (an empty card reader, a BitLocker
// volume that is still locked).
func listVolumes() []VolumeInfo {
	roots, err := logicalDriveRoots()
	if err != nil {
		return nil
	}

	var volumes []VolumeInfo
	withHardErrorsSuppressed(func() {
		for _, root := range roots {
			if !isAnalyzableDriveType(driveTypeOf(root)) {
				continue
			}
			info, err := statVolumeUnguarded(root)
			if err != nil {
				continue // Empty or unavailable drive — skip silently.
			}
			volumes = append(volumes, *info)
		}
	})

	return volumes
}

// statVolume reports on a single drive. mountPoint is normally a drive root
// such as `C:\`; a bare "C:" is normalised to one.
func statVolume(mountPoint string) (*VolumeInfo, error) {
	var (
		info *VolumeInfo
		err  error
	)
	withHardErrorsSuppressed(func() {
		info, err = statVolumeUnguarded(mountPoint)
	})
	return info, err
}

// statVolumeUnguarded is statVolume without the error-mode guard; callers that
// query several drives in a row set the guard once around the whole loop.
func statVolumeUnguarded(mountPoint string) (*VolumeInfo, error) {
	root := normalizeDriveRoot(mountPoint)

	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return nil, err
	}

	// "Available to caller" mirrors the Bavail semantics used on darwin: it
	// respects any disk quota applied to the current user.
	var freeToCaller, totalBytes, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &freeToCaller, &totalBytes, &totalFree); err != nil {
		return nil, err
	}

	// Label and filesystem name are best-effort: a volume can be perfectly
	// usable without them (RAW volumes report no filesystem name).
	label, fsType := volumeInformation(rootPtr)

	return &VolumeInfo{
		Name:       driveDisplayName(label, root),
		MountPoint: root,
		FSType:     fsType,
		TotalBytes: totalBytes,
		FreeBytes:  freeToCaller,
		UsedBytes:  calcUsedBytes(totalBytes, freeToCaller),
	}, nil
}

// isAnalyzableDriveType reports whether a drive is worth scanning. Optical
// media is read-only, network shares are not this machine's storage, and
// DRIVE_NO_ROOT_DIR means the letter has nothing mounted behind it. Anything
// we do not recognise is excluded rather than guessed at.
func isAnalyzableDriveType(t uint32) bool {
	switch t {
	case driveFixed, driveRemovable:
		return true
	default: // driveCDROM, driveRemote, driveNoRootDir, driveUnknown, driveRAMDisk
		return false
	}
}

// driveTypeOf returns the GetDriveType result for a drive root, or
// driveNoRootDir if the path cannot be converted for the Win32 call.
func driveTypeOf(root string) uint32 {
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return driveNoRootDir
	}
	return windows.GetDriveType(rootPtr)
}

// logicalDriveRoots returns every drive root on the machine, e.g.
// []string{`C:\`, `D:\`}.
func logicalDriveRoots() ([]string, error) {
	// A first call with a zero-length buffer asks for the required size, in
	// UTF-16 code units, excluding the final terminator.
	n, err := windows.GetLogicalDriveStrings(0, nil)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}

	buf := make([]uint16, n+1)
	n, err = windows.GetLogicalDriveStrings(uint32(len(buf)), &buf[0])
	if err != nil {
		return nil, err
	}
	if int(n) > len(buf) {
		n = uint32(len(buf))
	}

	return splitUTF16Strings(buf[:n]), nil
}

// volumeInformation returns the volume label and filesystem name (e.g. "NTFS")
// for a drive root. Both are empty when the drive cannot be queried.
func volumeInformation(rootPtr *uint16) (label, fsType string) {
	labelBuf := make([]uint16, windows.MAX_PATH+1)
	fsBuf := make([]uint16, windows.MAX_PATH+1)

	var serialNumber, maxComponentLen, flags uint32
	err := windows.GetVolumeInformation(
		rootPtr,
		&labelBuf[0], uint32(len(labelBuf)),
		&serialNumber,
		&maxComponentLen,
		&flags,
		&fsBuf[0], uint32(len(fsBuf)),
	)
	if err != nil {
		return "", ""
	}

	return windows.UTF16ToString(labelBuf), windows.UTF16ToString(fsBuf)
}
