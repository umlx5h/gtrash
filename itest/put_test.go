package itest

import (
	"fmt"
	"os/exec"
	"testing"
)

// Paths ending in a dot must be skipped.
// $ gtrash put .
// gtrash: refusing to remove '.' or '..' directory: skipping "."
func TestSkipDotEndingPath(t *testing.T) {
	paths := []string{".", "./", "..", "../", "../."}

	for _, path := range paths {
		t.Run(fmt.Sprintf("skipped path %q", path), func(t *testing.T) {

			cmd := exec.Command(execBinary, "put", path)
			out, err := cmd.CombinedOutput()
			mustError(t, err)
			assertContains(t, string(out), "refusing to remove '.' or '..' directory")
		})
	}
}
