//go:build windows

package platform

import (
	"fmt"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// Recycle Bin support.
//
// Dependency note: no third-party module is used here. The only well-known
// cross-platform Go trash libraries either shell out to platform binaries or
// wrap exactly this call; SHFileOperationW is a two-screen binding against the
// standard library's syscall package, so ComputerPruner keeps its dependency
// surface (and therefore its supply-chain surface) unchanged.
//
// Reference: SHFileOperationW / SHFILEOPSTRUCTW, shell32.dll.

const (
	foDelete = 0x0003 // FO_DELETE

	fofSilent          = 0x0004 // no progress dialog
	fofNoConfirmation  = 0x0010 // assume "yes to all"
	fofAllowUndo       = 0x0040 // send to the Recycle Bin instead of erasing
	fofNoErrorUI       = 0x0400 // report errors through the return code
	fofWantNukeWarning = 0x4000 // still warn when the item cannot be recycled
)

// shFileOpStructW mirrors SHFILEOPSTRUCTW. The field order and types must match
// the C declaration exactly; Go's natural alignment reproduces the C layout on
// both amd64 and arm64.
type shFileOpStructW struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

// recycle moves path to the Recycle Bin using the shell's file operation API,
// which is what Explorer itself uses, so the item is restorable from the bin.
func recycle(path string) error {
	if path == "" {
		return fmt.Errorf("cannot recycle an empty path")
	}

	// SHFileOperationW requires a fully qualified path.
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve %q to an absolute path: %w", path, err)
	}
	absolute = filepath.Clean(absolute)

	// pFrom is a double-NUL terminated list of NUL terminated strings.
	from, err := syscall.UTF16FromString(absolute)
	if err != nil {
		return fmt.Errorf("cannot encode %q for the shell API: %w", path, err)
	}
	from = append(from, 0)

	op := shFileOpStructW{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI | fofWantNukeWarning,
	}

	ret, _, callErr := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	// Keep the UTF-16 buffer alive until the shell has finished reading it.
	runtime.KeepAlive(from)

	if ret != 0 {
		return fmt.Errorf("move to Recycle Bin failed for %q: SHFileOperation returned 0x%x", path, ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("move to Recycle Bin was aborted for %q", path)
	}
	// Call reports the thread's last error unconditionally; it is only
	// meaningful when the operation itself failed, which the checks above
	// already covered.
	_ = callErr

	return nil
}
