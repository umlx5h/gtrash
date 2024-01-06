package posix

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// same as du -B1 or du -sh
// The size is calculated as the disk space used by the directory and its contents, that is, the size of the blocks, in bytes (in the same way as the `du -B1` command calculates).
func DirSize(path string) (int64, error) {
	var block int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		sys, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("cannot get stat_t")
		}

		block += sys.Blocks
		return err
	})
	return block * 512, err
}

// Look at both block-size and apparant-size and choose the larger one.
// Because there are file systems for which block size cannot be obtained.
// max(du -sB1, du -sb)
func DirSizeFallback(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		sys, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("cannot get stat_t")
		}

		// stat(2)
		// blkcnt_t  st_blocks;      /* Number of 512B blocks allocated */
		size += max(sys.Size, sys.Blocks*512)
		return err
	})

	return size, err
}

// check name path is empty directory
func DirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
