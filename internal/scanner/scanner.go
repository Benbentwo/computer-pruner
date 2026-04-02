package scanner

import (
	"context"
)

// TreeNode represents a file or directory in the scan tree
type TreeNode struct {
	Name      string      `json:"name"`
	Path      string      `json:"path"`
	Size      int64       `json:"size"`
	IsDir     bool        `json:"isDir"`
	IsProtected bool      `json:"isProtected"`
	ModTime   string      `json:"modTime"`
	Children  []*TreeNode `json:"children"`
	FileCount int         `json:"fileCount"`
	DirCount  int         `json:"dirCount"`
}

// ScanProgress represents the current progress of a disk scan
type ScanProgress struct {
	ScannedItems int    `json:"scannedItems"`
	TotalSize    int64  `json:"totalSize"`
	CurrentPath  string `json:"currentPath"`
	ElapsedMs    int    `json:"elapsedMs"`
	IsComplete   bool   `json:"isComplete"`
}

// ScannerService provides disk scanning functionality
type ScannerService struct {
	ctx context.Context
}

// NewScannerService creates a new instance of ScannerService
func NewScannerService() *ScannerService {
	return &ScannerService{}
}

// SetContext sets the wails runtime context for the service
func (s *ScannerService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// StartScan begins a disk scan on the specified mount point
func (s *ScannerService) StartScan(mountPoint string) error {
	// TODO: Implement actual disk scanning logic
	return nil
}

// StartFolderScan begins a scan on a specific folder path
func (s *ScannerService) StartFolderScan(path string) error {
	// TODO: Implement folder scanning logic
	return nil
}

// CancelScan cancels the current running scan
func (s *ScannerService) CancelScan() error {
	// TODO: Implement scan cancellation
	return nil
}

// GetScanTree returns the complete tree structure from the last scan
func (s *ScannerService) GetScanTree() *TreeNode {
	// TODO: Return actual scan tree from last completed scan
	return &TreeNode{
		Name:     "root",
		Path:     "/",
		Size:     0,
		IsDir:    true,
		Children: []*TreeNode{},
	}
}

// GetScanProgress returns the current progress of an ongoing scan
func (s *ScannerService) GetScanProgress() *ScanProgress {
	// TODO: Return actual scan progress
	return &ScanProgress{
		ScannedItems: 0,
		TotalSize:    0,
		CurrentPath:  "",
		ElapsedMs:    0,
		IsComplete:   false,
	}
}
