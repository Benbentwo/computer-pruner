//go:build darwin || windows

package scanner

// pathsAreCaseInsensitive reflects the default behaviour of the filesystems on
// the platforms ComputerPruner ships to: HFS+/APFS are case-insensitive
// (case-preserving) by default, and NTFS is case-insensitive in practice.
//
// It matches the constant of the same name in internal/platform; the two are
// separate only because scanner must not depend on platform internals.
const pathsAreCaseInsensitive = true
