//go:build windows

package process

// windowsInspector implements Inspector for Windows.
// This is a stub implementation - Windows support would need proper WMI or syscall integration.
type windowsInspector struct{}

// DefaultInspector returns the default inspector for Windows.
func DefaultInspector() Inspector {
	return &windowsInspector{}
}

// GetChildProcesses returns PIDs of child processes.
// Windows implementation pending - returns empty slice.
func (i *windowsInspector) GetChildProcesses(pid int) []int {
	// TODO: Implement using Windows API or WMI
	return nil
}

// GetProcessName returns the name of a process.
// Windows implementation pending - returns empty string.
func (i *windowsInspector) GetProcessName(pid int) string {
	// TODO: Implement using Windows API
	return ""
}

// IsRunning checks if a process is running.
// Windows implementation pending - returns false.
func (i *windowsInspector) IsRunning(pid int) bool {
	// TODO: Implement using Windows API
	return false
}

// GetState returns the state of a process.
// Windows implementation pending - returns empty string.
func (i *windowsInspector) GetState(pid int) string {
	// TODO: Implement using Windows API
	return ""
}

// HasOpenFiles checks if a process has open file descriptors.
// Windows implementation pending - returns false.
func (i *windowsInspector) HasOpenFiles(pid int) bool {
	// TODO: Implement using Windows API
	return false
}
