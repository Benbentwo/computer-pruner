//go:build darwin

package volume

import (
	"os"
	"path/filepath"
	"syscall"
)

const (
	// rootMountPoint is the boot volume's mount point.
	rootMountPoint = "/"
	// volumesDir is where macOS mounts every other volume.
	volumesDir = "/Volumes"
)

// listVolumes returns the boot volume followed by everything mounted under
// /Volumes. The /Volumes entry that aliases "/" is skipped, since the boot
// volume is already first in the list.
func listVolumes() []VolumeInfo {
	var volumes []VolumeInfo

	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		entries = nil
	}

	// Always include the root volume, named after its /Volumes alias when one
	// exists so we show the user's real volume name rather than a guess.
	if info, err := statVolumeNamed(rootMountPoint, rootVolumeName(detectRootVolumeName(entries))); err == nil {
		volumes = append(volumes, *info)
	}

	for _, entry := range entries {
		mountPoint := filepath.Join(volumesDir, entry.Name())

		// Resolve symlinks — /Volumes/Macintosh HD is often a symlink to /
		resolved, err := filepath.EvalSymlinks(mountPoint)
		if err == nil && resolved == rootMountPoint {
			continue // Skip — already added root
		}

		// Skip if not a directory
		fi, err := os.Stat(mountPoint)
		if err != nil || !fi.IsDir() {
			continue
		}

		if info, err := statVolumeNamed(mountPoint, entry.Name()); err == nil {
			volumes = append(volumes, *info)
		}
	}

	return volumes
}

// statVolume reports on a single mount point, deriving a display name from the
// path.
func statVolume(mountPoint string) (*VolumeInfo, error) {
	name := filepath.Base(mountPoint)
	if mountPoint == rootMountPoint {
		entries, err := os.ReadDir(volumesDir)
		if err != nil {
			entries = nil
		}
		name = rootVolumeName(detectRootVolumeName(entries))
	}
	return statVolumeNamed(mountPoint, name)
}

// detectRootVolumeName finds the /Volumes entry that aliases "/" and returns
// its name, which is the real name of the boot volume (e.g. "Macintosh HD").
// It returns an empty string when no such alias exists.
func detectRootVolumeName(entries []os.DirEntry) string {
	for _, entry := range entries {
		resolved, err := filepath.EvalSymlinks(filepath.Join(volumesDir, entry.Name()))
		if err == nil && resolved == rootMountPoint {
			return entry.Name()
		}
	}
	return ""
}

// statVolumeNamed uses syscall.Statfs to get real volume statistics.
func statVolumeNamed(mountPoint string, name string) (*VolumeInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &stat); err != nil {
		return nil, err
	}

	blockSize := uint64(stat.Bsize)
	totalBytes := stat.Blocks * blockSize
	freeBytes := stat.Bavail * blockSize // Bavail = available to non-root users

	// Detect filesystem type from fstypename
	fsType := int8SliceToString(stat.Fstypename[:])

	return &VolumeInfo{
		Name:       name,
		MountPoint: mountPoint,
		FSType:     fsType,
		TotalBytes: totalBytes,
		FreeBytes:  freeBytes,
		UsedBytes:  calcUsedBytes(totalBytes, freeBytes),
	}, nil
}
