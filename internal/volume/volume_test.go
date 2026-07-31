package volume

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestVolumeInfoJSONKeys locks the wire contract with the Svelte frontend and
// the generated Wails bindings. Renaming any of these keys breaks the UI.
func TestVolumeInfoJSONKeys(t *testing.T) {
	raw, err := json.Marshal(VolumeInfo{})
	if err != nil {
		t.Fatalf("marshal VolumeInfo: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal VolumeInfo: %v", err)
	}

	want := []string{"name", "mountPoint", "fsType", "totalBytes", "freeBytes", "usedBytes"}
	if len(decoded) != len(want) {
		t.Fatalf("VolumeInfo has %d JSON keys (%v), want %d", len(decoded), decoded, len(want))
	}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Errorf("VolumeInfo is missing JSON key %q; got %v", key, decoded)
		}
	}
}

func TestInt8SliceToString(t *testing.T) {
	tests := []struct {
		name  string
		input []int8
		want  string
	}{
		{"empty slice", nil, ""},
		{"leading NUL", []int8{0, 'a', 'b'}, ""},
		{"NUL terminated", []int8{'a', 'p', 'f', 's', 0, 0, 0, 0}, "apfs"},
		{"unterminated fills buffer", []int8{'h', 'f', 's'}, "hfs"},
		{"stops at first NUL", []int8{'m', 's', 'd', 'o', 's', 0, 'x'}, "msdos"},
		// Bytes above 0x7f arrive as negative int8 and must round-trip.
		{"high bytes", []int8{'e', 'x', 'f', 'a', 't', 0}, "exfat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := int8SliceToString(tt.input); got != tt.want {
				t.Errorf("int8SliceToString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCalcUsedBytes(t *testing.T) {
	tests := []struct {
		name        string
		total, free uint64
		want        uint64
	}{
		{"half used", 1000, 400, 600},
		{"empty volume", 1000, 1000, 0},
		{"full volume", 1000, 0, 1000},
		{"zero-sized volume", 0, 0, 0},
		// Bavail can exceed the reported total on quota-backed filesystems;
		// unsigned subtraction must not wrap around.
		{"free exceeds total", 1000, 1500, 0},
		{"large values", 1 << 62, 1 << 61, 1 << 61},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcUsedBytes(tt.total, tt.free); got != tt.want {
				t.Errorf("calcUsedBytes(%d, %d) = %d, want %d", tt.total, tt.free, got, tt.want)
			}
		})
	}
}

func TestRootVolumeName(t *testing.T) {
	tests := []struct {
		name     string
		detected string
		want     string
	}{
		{"real name detected", "Ben's SSD", "Ben's SSD"},
		{"classic name detected", "Macintosh HD", "Macintosh HD"},
		{"nothing detected", "", "Macintosh HD"},
		{"whitespace only", "   ", "Macintosh HD"},
		{"unicode name", "Дисk", "Дисk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rootVolumeName(tt.detected); got != tt.want {
				t.Errorf("rootVolumeName(%q) = %q, want %q", tt.detected, got, tt.want)
			}
		})
	}
}

func TestDriveDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		label string
		root  string
		want  string
	}{
		{"label wins", "Windows", `C:\`, "Windows"},
		{"empty label falls back to letter", "", `C:\`, "Local Disk (C:)"},
		{"whitespace label falls back", "  ", `D:\`, "Local Disk (D:)"},
		{"lowercase root is upcased", "", `e:\`, "Local Disk (E:)"},
		{"bare drive spec", "", "F:", "Local Disk (F:)"},
		{"non drive path", "", `\\server\share`, "Local Disk"},
		{"empty root", "", "", "Local Disk"},
		{"labelled removable", "BACKUP", `E:\`, "BACKUP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := driveDisplayName(tt.label, tt.root); got != tt.want {
				t.Errorf("driveDisplayName(%q, %q) = %q, want %q", tt.label, tt.root, got, tt.want)
			}
		})
	}
}

func TestDriveLetter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"root", `C:\`, "C:"},
		{"bare spec", "C:", "C:"},
		{"lowercase", `c:\`, "C:"},
		{"nested path", `C:\Users\example`, "C:"},
		{"unc path", `\\server\share`, ""},
		{"posix path", "/Volumes/Backup", ""},
		{"too short", "C", ""},
		{"digit drive", "1:", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := driveLetter(tt.input); got != tt.want {
				t.Errorf("driveLetter(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeDriveRoot(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already canonical", `C:\`, `C:\`},
		{"bare spec gains separator", "C:", `C:\`},
		{"lowercase is upcased", "d:", `D:\`},
		{"forward slash root", "C:/", `C:\`},
		{"nested path untouched", `C:\Users\example`, `C:\Users\example`},
		{"unc untouched", `\\server\share`, `\\server\share`},
		{"posix untouched", "/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDriveRoot(tt.input); got != tt.want {
				t.Errorf("normalizeDriveRoot(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitUTF16Strings(t *testing.T) {
	// utf16 of `C:\` + NUL + `D:\` + NUL + NUL, the shape returned by
	// GetLogicalDriveStrings.
	multi := []uint16{'C', ':', '\\', 0, 'D', ':', '\\', 0, 0}

	tests := []struct {
		name  string
		input []uint16
		want  []string
	}{
		{"nil buffer", nil, nil},
		{"only terminator", []uint16{0}, nil},
		{"single drive", []uint16{'C', ':', '\\', 0, 0}, []string{`C:\`}},
		{"two drives", multi, []string{`C:\`, `D:\`}},
		{"unterminated tail", []uint16{'C', ':', '\\'}, []string{`C:\`}},
		{"skips empty entries", []uint16{0, 'Z', ':', '\\', 0, 0, 0}, []string{`Z:\`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitUTF16Strings(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitUTF16Strings(%v) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

// TestListVolumesNeverNil guards the frontend contract: ListVolumes must always
// hand back an iterable slice, never a nil that marshals to JSON null. It runs
// on every platform, including the stub one.
func TestListVolumesNeverNil(t *testing.T) {
	svc := NewVolumeService()
	if svc == nil {
		t.Fatal("NewVolumeService() returned nil")
	}

	volumes := svc.ListVolumes()
	if volumes == nil {
		t.Fatal("ListVolumes() returned nil, want a non-nil slice")
	}

	raw, err := json.Marshal(volumes)
	if err != nil {
		t.Fatalf("marshal volumes: %v", err)
	}
	if string(raw) == "null" {
		t.Error("ListVolumes() marshalled to JSON null, want an array")
	}

	// Whatever the platform reports must be internally consistent.
	for _, v := range volumes {
		if v.MountPoint == "" {
			t.Errorf("volume %+v has an empty mount point", v)
		}
		if v.Name == "" {
			t.Errorf("volume %+v has an empty name", v)
		}
		if v.UsedBytes != calcUsedBytes(v.TotalBytes, v.FreeBytes) {
			t.Errorf("volume %+v: usedBytes is inconsistent with totalBytes/freeBytes", v)
		}
	}
}
