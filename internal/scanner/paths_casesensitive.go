//go:build !darwin && !windows

package scanner

// pathsAreCaseInsensitive is false on the non-target platforms (Linux, BSD),
// whose filesystems are case-sensitive. ComputerPruner does not ship there, but
// the package still has to build and behave correctly so the test suite can run
// on a Linux CI host.
const pathsAreCaseInsensitive = false
