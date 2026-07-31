package fileops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGuard builds a protected-path policy over an explicit set of paths, so
// the delete tests never depend on (or touch) the host's real protected
// locations.
func fakeGuard(protected ...string) func(string) (bool, string) {
	set := make(map[string]bool, len(protected))
	for _, p := range protected {
		set[filepath.Clean(p)] = true
	}
	return func(path string) (bool, string) {
		if strings.TrimSpace(path) == "" {
			return true, "an empty path is never safe to modify"
		}
		if set[filepath.Clean(path)] {
			return true, fmt.Sprintf("%q is a protected system location", filepath.Clean(path))
		}
		return false, ""
	}
}

// installGuard swaps the package's protected-path source for one test.
func installGuard(t *testing.T, guard func(string) (bool, string)) {
	t.Helper()
	original := checkProtected
	checkProtected = guard
	t.Cleanup(func() { checkProtected = original })
}

// recordTrash replaces the trash primitive with a recorder, so no test ever
// moves anything to a real Trash or Recycle Bin.
func recordTrash(t *testing.T, err error) *[]string {
	t.Helper()
	var calls []string
	original := moveToTrash
	moveToTrash = func(path string) error {
		calls = append(calls, path)
		if err != nil {
			return err
		}
		return os.RemoveAll(path) // stand in for "the item left its old location"
	}
	t.Cleanup(func() { moveToTrash = original })
	return &calls
}

func writeFile(t *testing.T, dir, name string, size int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeletePathsSkipsProtectedAndProcessesTheRest(t *testing.T) {
	dir := t.TempDir()
	protected := writeFile(t, dir, "protected.bin", 4096)
	deletable := writeFile(t, dir, "deletable.bin", 1024)

	installGuard(t, fakeGuard(protected))
	trashed := recordTrash(t, nil)

	svc := NewFileOpsService() // no Wails context: emit must be a no-op, not a panic
	result, err := svc.DeletePaths([]string{protected, deletable})
	if err != nil {
		t.Fatalf("DeletePaths returned an error: %v", err)
	}

	if got := len(*trashed); got != 1 {
		t.Fatalf("expected exactly one trash call, got %d: %v", got, *trashed)
	}
	if (*trashed)[0] != deletable {
		t.Fatalf("trashed %q, want %q", (*trashed)[0], deletable)
	}

	if result.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1", result.DeletedCount)
	}
	if result.FreedBytes != 1024 {
		t.Errorf("FreedBytes = %d, want 1024", result.FreedBytes)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors = %v, want exactly one entry", result.Errors)
	}
	if !strings.Contains(result.Errors[0], protected) {
		t.Errorf("error %q does not name the protected path %q", result.Errors[0], protected)
	}
	if !strings.Contains(result.Errors[0], "protected system location") {
		t.Errorf("error %q does not carry the reason from the guard", result.Errors[0])
	}

	if _, err := os.Stat(protected); err != nil {
		t.Errorf("the protected file was touched: %v", err)
	}
}

func TestDeletePathsPermanentlyRefusesProtectedPaths(t *testing.T) {
	dir := t.TempDir()

	protectedDir := filepath.Join(dir, "keep")
	if err := os.MkdirAll(protectedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, protectedDir, "inner.bin", 32)

	deletable := writeFile(t, dir, "gone.bin", 2048)

	installGuard(t, fakeGuard(protectedDir))

	svc := NewFileOpsService()
	result, err := svc.DeletePathsPermanently([]string{protectedDir, deletable})
	if err != nil {
		t.Fatalf("DeletePathsPermanently returned an error: %v", err)
	}

	if _, err := os.Stat(protectedDir); err != nil {
		t.Fatalf("the protected directory was removed: %v", err)
	}
	if _, err := os.Stat(deletable); !os.IsNotExist(err) {
		t.Fatalf("the unprotected file should have been removed, stat err = %v", err)
	}

	if result.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1", result.DeletedCount)
	}
	if result.FreedBytes != 2048 {
		t.Errorf("FreedBytes = %d, want 2048", result.FreedBytes)
	}
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "Protected path, skipped") {
		t.Errorf("Errors = %v, want one 'Protected path, skipped' entry", result.Errors)
	}
}

func TestDeleteBatchValidatesEveryPathBeforeAnyDestructiveCall(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.bin", 1)
	blocked := writeFile(t, dir, "blocked.bin", 1)
	c := writeFile(t, dir, "c.bin", 1)

	var log []string

	guard := fakeGuard(blocked)
	installGuard(t, func(path string) (bool, string) {
		log = append(log, "check:"+filepath.Base(path))
		return guard(path)
	})

	original := moveToTrash
	moveToTrash = func(path string) error {
		log = append(log, "destroy:"+filepath.Base(path))
		return nil
	}
	t.Cleanup(func() { moveToTrash = original })

	svc := NewFileOpsService()
	if _, err := svc.DeletePaths([]string{a, blocked, c}); err != nil {
		t.Fatal(err)
	}

	// The whole batch is classified before anything is destroyed, and each
	// survivor is re-checked immediately before its own destroy so it is never
	// acted on with a verdict that is several deletes old.
	want := []string{
		"check:a.bin", "check:blocked.bin", "check:c.bin",
		"check:a.bin", "destroy:a.bin",
		"check:c.bin", "destroy:c.bin",
	}
	if strings.Join(log, ",") != strings.Join(want, ",") {
		t.Fatalf("call order = %v, want %v", log, want)
	}
}

// TestDeleteBatchRevalidatesImmediatelyBeforeEachDestroy is the regression test
// for the batch TOCTOU window: a path cleared during the up-front pass but
// replaced by a protected tree before its turn came round was destroyed on the
// strength of the stale verdict.
func TestDeleteBatchRevalidatesImmediatelyBeforeEachDestroy(t *testing.T) {
	dir := t.TempDir()
	first := writeFile(t, dir, "first.bin", 8)
	victim := writeFile(t, dir, "victim.bin", 8)

	// The guard clears everything on the up-front pass, then starts refusing
	// "victim" — standing in for a protected directory renamed into place
	// while the earlier delete was running.
	upFrontPassDone := false
	installGuard(t, func(path string) (bool, string) {
		if filepath.Base(path) == "victim.bin" && upFrontPassDone {
			return true, "a protected directory was moved here"
		}
		return false, ""
	})

	original := removeAll
	removeAll = func(path string) error {
		upFrontPassDone = true
		return os.RemoveAll(path)
	}
	t.Cleanup(func() { removeAll = original })

	svc := NewFileOpsService()
	result, err := svc.DeletePathsPermanently([]string{first, victim})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("the path that became protected mid-batch was destroyed anyway: %v", err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("the first path should still have been deleted, stat err = %v", err)
	}
	if result.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1", result.DeletedCount)
	}
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "Protected path, skipped") {
		t.Errorf("Errors = %v, want one 'Protected path, skipped' entry", result.Errors)
	}
}

func TestDeletePathsReportsPrimitiveFailures(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "stubborn.bin", 16)

	installGuard(t, fakeGuard())
	recordTrash(t, errors.New("Finder said no"))

	svc := NewFileOpsService()
	result, err := svc.DeletePaths([]string{target})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedCount != 0 {
		t.Errorf("DeletedCount = %d, want 0", result.DeletedCount)
	}
	if result.FreedBytes != 0 {
		t.Errorf("FreedBytes = %d, want 0", result.FreedBytes)
	}
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "Finder said no") {
		t.Fatalf("Errors = %v, want the primitive's message", result.Errors)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("the file should still be present: %v", err)
	}
}

func TestDeletePathsRejectsEmptyAndUnresolvableInput(t *testing.T) {
	installGuard(t, fakeGuard())
	trashed := recordTrash(t, nil)

	svc := NewFileOpsService()
	result, err := svc.DeletePaths([]string{"", "   "})
	if err != nil {
		t.Fatal(err)
	}
	if len(*trashed) != 0 {
		t.Fatalf("nothing should have been trashed, got %v", *trashed)
	}
	if result.DeletedCount != 0 {
		t.Errorf("DeletedCount = %d, want 0", result.DeletedCount)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("Errors = %v, want two entries", result.Errors)
	}
}

func TestDeletePathsWithEmptyBatchIsANoOp(t *testing.T) {
	installGuard(t, fakeGuard())
	trashed := recordTrash(t, nil)

	svc := NewFileOpsService()
	result, err := svc.DeletePaths(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(*trashed) != 0 || result.DeletedCount != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected result %+v (trashed %v)", result, *trashed)
	}
	if result.Errors == nil {
		t.Error("Errors must be a non-nil slice so it marshals to [] rather than null")
	}
}

// TestDeletePathsUsesTheRealGuardByDefault makes sure nobody accidentally
// leaves the package wired to a test double.
func TestDeletePathsUsesTheRealGuardByDefault(t *testing.T) {
	if checkProtected == nil {
		t.Fatal("checkProtected must be wired to platform.IsProtected")
	}
	// A tilde path must be refused by the production guard.
	protected, reason := checkProtected("~/Documents")
	if !protected {
		t.Fatalf("the production guard accepted %q", "~/Documents")
	}
	if reason == "" {
		t.Fatal("the production guard gave no reason")
	}
}

func TestDirSizeCountsRegularFilesOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.bin", 100)
	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, nested, "b.bin", 250)

	if got := dirSize(dir); got != 350 {
		t.Errorf("dirSize = %d, want 350", got)
	}
	if got := pathSize(filepath.Join(dir, "a.bin")); got != 100 {
		t.Errorf("pathSize = %d, want 100", got)
	}
	if got := pathSize(filepath.Join(dir, "missing")); got != 0 {
		t.Errorf("pathSize of a missing path = %d, want 0", got)
	}
}

func TestGetFileInfo(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "info.bin", 7)

	svc := NewFileOpsService()
	info, err := svc.GetFileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "info.bin" || info.Size != 7 || info.IsDir {
		t.Fatalf("unexpected FileInfo %+v", info)
	}
	if info.ModTime == "" || info.Permissions == "" {
		t.Fatalf("FileInfo is missing metadata: %+v", info)
	}

	if _, err := svc.GetFileInfo(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
