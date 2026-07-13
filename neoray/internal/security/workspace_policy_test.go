package security

import (
	"runtime"
	"testing"
)

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{"literal dotdot slash", "../etc/passwd", true},
		{"literal dotdot backslash", "..\\windows\\system32", true},
		{"url encoded dotdot", "%2e%2e%2fetc%2fpasswd", true},
		{"url encoded mixed", "%2e%2e/etc/passwd", true},
		{"double encoded", "%252e%252e%252f", true},
		{"safe command", "ls -la", false},
		{"safe relative", "src/main.go", false},
		{"empty string", "", false},
	}

	// Absolute path check is platform-dependent
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name     string
			command  string
			expected bool
		}{"absolute windows", "C:\\Windows\\System32", true})
	} else {
		tests = append(tests, struct {
			name     string
			command  string
			expected bool
		}{"absolute unix", "/etc/passwd", true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsPathTraversal(tt.command)
			if result != tt.expected {
				t.Errorf("ContainsPathTraversal(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestIsWindowsReservedName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"CON", "CON", true},
		{"con lowercase", "con", true},
		{"NUL", "NUL", true},
		{"COM1", "COM1", true},
		{"LPT9", "LPT9", true},
		{"CON with ext", "CON.txt", true},
		{"NUL with ext", "NUL.log", true},
		{"safe name", "config.txt", false},
		{"safe name 2", "data.json", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWindowsReservedName(tt.input)
			if result != tt.expected {
				t.Errorf("IsWindowsReservedName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsDevicePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"windows UNC", "\\\\.\\COM1", true},
		{"windows reserved CON", "CON", true},
		{"windows reserved in path", "C:\\temp\\NUL", true},
		{"safe relative", "src/main.go", false},
	}

	// Unix device paths only exist on Unix
	if runtime.GOOS != "windows" {
		tests = append([]struct {
			name     string
			path     string
			expected bool
		}{
			{"unix dev", "/dev/null", true},
			{"unix proc", "/proc/self/status", true},
			{"safe path", "/home/user/file.txt", false},
		}, tests...)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDevicePath(tt.path)
			if result != tt.expected {
				t.Errorf("IsDevicePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsBenignDevicePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix device path tests on Windows")
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"null", "/dev/null", true},
		{"urandom", "/dev/urandom", true},
		{"not benign", "/dev/sda", false},
		{"not dev", "/home/user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBenignDevicePath(tt.path)
			if result != tt.expected {
				t.Errorf("IsBenignDevicePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilterCommandForPathSafety(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"shell metachar semicolon", "ls; cat /etc/passwd", true},
		{"shell metachar pipe", "ls | grep secret", true},
		{"shell metachar backtick", "cat `whoami`", true},
		{"shell metachar dollar", "echo $(whoami)", true},
		{"path traversal", "cat ../../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FilterCommandForPathSafety(tt.command, workspace)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterCommandForPathSafety(%q) error = %v, wantErr %v", tt.command, err, tt.wantErr)
			}
		})
	}
}
