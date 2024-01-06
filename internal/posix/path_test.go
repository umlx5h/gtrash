package posix

import (
	"os"
	"testing"
)

func TestAbsPathToTilde(t *testing.T) {
	home := os.Getenv("HOME")

	tests := []struct {
		absPath      string
		expectedPath string
	}{
		{home + "/example/file.txt", "~/example/file.txt"},
		{"/home/user/another/file.txt", "/home/user/another/file.txt"},
		{"", ""},
	}

	for _, tt := range tests {
		result := AbsPathToTilde(tt.absPath)
		if result != tt.expectedPath {
			t.Errorf("Expected %s, but got %s for path %s", tt.expectedPath, result, tt.absPath)
		}
	}
}

func TestCheckSubPath(t *testing.T) {
	tests := []struct {
		parentPath string
		subPath    string
		expected   bool
	}{
		{"/home/user", "/home/user/Documents", true},
		{"/home/user", "/home/user/Documents/foo", true},
		{"/home/user", "/home/user", true},
		{"/home/user", "/var/www", false},
		{"/home/user", "/home", false},
		{"/", "/", true},
	}

	for _, tt := range tests {
		result, err := CheckSubPath(tt.parentPath, tt.subPath)
		if err != nil {
			t.Errorf("Error occurred: %s", err)
		}

		if result != tt.expected {
			t.Errorf("Expected %v, but got %v for parent: %q, sub: %q", tt.expected, result, tt.parentPath, tt.subPath)
		}
	}
}
