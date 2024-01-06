package itest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

var (
	HOME_TRASH = "/root/.local/share/Trash"

	EXTERNAL_ROOT     = "/external"
	EXTERNAL_ALT_ROOT = "/external_alt"

	EXTERNAL_TRASH     = filepath.Join(EXTERNAL_ROOT, ".Trash", strconv.Itoa(os.Getuid()))
	EXTERNAL_ALT_TRASH = filepath.Join(EXTERNAL_ALT_ROOT, fmt.Sprintf(".Trash-%d", os.Getuid()))
)

// remove all trash
func cleanTrash(t *testing.T) {
	t.Helper()

	// clean home trash
	err := os.RemoveAll(HOME_TRASH)
	mustNoError(t, err)

	// clean external trash
	err = os.RemoveAll(EXTERNAL_TRASH)
	mustNoError(t, err)

	// clean external_alt trash
	err = os.RemoveAll(EXTERNAL_ALT_TRASH)
	mustNoError(t, err)
}

func TestTrashAllType(t *testing.T) {
	tests := []struct {
		name     string
		fileDir  string
		trashDir string
	}{
		{name: "HOME_TRASH", fileDir: "", trashDir: HOME_TRASH}, // use /tmp
		{name: "EXTERNAL_TRASH", fileDir: EXTERNAL_ROOT, trashDir: EXTERNAL_TRASH},
		{name: "EXTERNAL_ALT_TRASH", fileDir: EXTERNAL_ALT_ROOT, trashDir: EXTERNAL_ALT_TRASH},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanTrash(t)

			f, err := os.CreateTemp(tt.fileDir, "foo")
			mustNoError(t, err)
			trashFilePath := filepath.Join(tt.trashDir, "files", filepath.Base(f.Name()))

			// 1. should be trashed to specific type trashDir
			cmd := exec.Command(execBinary, "put", f.Name())
			out, err := cmd.CombinedOutput()
			mustNoError(t, err)
			assertEmpty(t, string(out))

			checkFileMoved(t, f.Name(), trashFilePath)

			// 2. should list trashed file
			cmd = exec.Command(execBinary, "find")
			out, err = cmd.CombinedOutput()
			mustNoError(t, err, string(out))
			assertContains(t, string(out), f.Name(), "it should list deleted file")

			// 3. should show summary
			cmd = exec.Command(execBinary, "summary")
			out, err = cmd.CombinedOutput()
			mustNoError(t, err, string(out))
			assertEqual(t, string(out), fmt.Sprintf("[%s]\nitem: 1\nsize: 0 B\n", tt.trashDir))

			// 4. should be restored to original path
			cmd = exec.Command(execBinary, "restore", f.Name())
			out, err = cmd.CombinedOutput()
			mustNoError(t, err, string(out))
			assertContains(t, string(out), "Restored 1/1 trashed files")
			checkFileMoved(t, trashFilePath, f.Name())

			// 5. should not list restored file
			cmd = exec.Command(execBinary, "find")
			out, err = cmd.CombinedOutput()
			mustError(t, err, string(out))
			assertContains(t, string(out), "not found trashed files", "should not list deleted file")
		})
	}
}
