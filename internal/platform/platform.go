package platform

// GetProtectedPaths returns a list of system-protected paths that should not be deleted
func GetProtectedPaths() []string {
	// Combined list of protected paths across macOS, Windows, and Linux
	return []string{
		// macOS paths
		"/System",
		"/Library",
		"/Applications",
		"/Users",
		"/Volumes",
		"/var",
		"/tmp",
		"/etc",
		"/bin",
		"/sbin",
		"/usr",
		"/opt",
		"/dev",
		"/private",
		"~/Library/Application Support",
		"~/Library/Preferences",
		"~/Library/Caches",
		"~/Music",
		"~/Pictures",
		"~/Documents",
		"~/Desktop",
		"~/Downloads",
		// Windows paths
		"C:\\Windows",
		"C:\\Program Files",
		"C:\\Program Files (x86)",
		"C:\\ProgramData",
		"C:\\Users",
		"C:\\Recovery",
		"C:\\$Recycle.Bin",
		// Linux paths
		"/boot",
		"/root",
		"/home",
		"/proc",
		"/sys",
		"/run",
		"/srv",
		"/media",
		"/mnt",
		"/lost+found",
	}
}

// RevealInFileManager opens the file manager and reveals the specified path
// This is a platform-specific operation that will be implemented later
func RevealInFileManager(path string) error {
	// TODO: Implement platform-specific file manager reveal
	// macOS: open -R path
	// Windows: explorer /select,path
	// Linux: xdg-open path or file manager specific commands
	return nil
}

// GetSystemTheme returns the current system theme preference
func GetSystemTheme() string {
	// TODO: Implement platform-specific theme detection
	// For now, return placeholder value
	return "dark"
}
