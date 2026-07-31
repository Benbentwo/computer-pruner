//go:build !darwin && !windows

package volume

import "testing"

// TestStubListVolumesIsEmpty pins the behaviour of the unsupported-platform
// stub: an empty list, no panic.
func TestStubListVolumesIsEmpty(t *testing.T) {
	if got := listVolumes(); got != nil {
		t.Errorf("listVolumes() = %#v, want nil on an unsupported platform", got)
	}

	volumes := NewVolumeService().ListVolumes()
	if volumes == nil {
		t.Fatal("ListVolumes() returned nil, want an empty slice")
	}
	if len(volumes) != 0 {
		t.Errorf("ListVolumes() returned %d volumes, want 0 on an unsupported platform", len(volumes))
	}
}

// TestStubGetVolumeInfoErrors checks the stub reports a clear error rather than
// silently returning a zero-valued volume.
func TestStubGetVolumeInfoErrors(t *testing.T) {
	for _, mountPoint := range []string{"/", `C:\`, t.TempDir()} {
		info, err := NewVolumeService().GetVolumeInfo(mountPoint)
		if err == nil {
			t.Errorf("GetVolumeInfo(%q) returned no error, want one on an unsupported platform", mountPoint)
		}
		if info != nil {
			t.Errorf("GetVolumeInfo(%q) = %+v, want nil", mountPoint, info)
		}
	}
}
