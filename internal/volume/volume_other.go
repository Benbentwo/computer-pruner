//go:build !darwin && !windows

package volume

import (
	"fmt"
	"runtime"
)

// ComputerPruner ships for macOS and Windows only. This stub keeps the package
// building — and `go test ./internal/...` running — on other platforms such as
// the linux CI containers, without pretending to enumerate anything.

// listVolumes returns no volumes: enumeration is not implemented for this
// platform.
func listVolumes() []VolumeInfo {
	return nil
}

// statVolume always fails on unsupported platforms.
func statVolume(mountPoint string) (*VolumeInfo, error) {
	return nil, fmt.Errorf("volume: enumeration is not supported on %s (macOS and Windows only): %q", runtime.GOOS, mountPoint)
}
