package glog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	errorCalled int
	progName    = filepath.Base(os.Args[0])
)

var stderr io.Writer = os.Stderr

func Error(msg string) {
	errorCalled++
	fmt.Fprintln(stderr, progName+":", msg)
}

func Errorf(format string, args ...any) {
	errorCalled++
	fmt.Fprintf(stderr, progName+": "+format, args...)
}

func ExitCode() int {
	if errorCalled == 0 {
		return 0
	} else {
		return 1
	}
}
