package itest

import (
	"log"
	"os"
	"strings"
	"testing"
)

var execBinary = "/app/gtrash"

func TestMain(m *testing.M) {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		log.Println("please execute on docker enviornment.")
		os.Exit(1)
	}
	ret := m.Run()
	os.Exit(ret)
}

func checkFileMoved(t *testing.T, from string, to string) {
	t.Helper()

	if _, err := os.Stat(from); err == nil {
		t.Errorf("from still exists. from=%q", from)
	}

	if _, err := os.Stat(to); err != nil {
		t.Errorf("to not found. to=%q", to)
	}
}

func mustError(t testing.TB, err error, msg ...string) {
	t.Helper()

	if err == nil {
		if len(msg) == 0 {
			t.Fatalf("Received unexpected error: %v", err)
		} else {
			t.Fatalf("Received unexpected error: %s: %v", msg[0], err)
		}
	}
}

func mustNoError(t testing.TB, err error, msg ...string) {
	t.Helper()

	if err != nil {
		if len(msg) == 0 {
			t.Fatalf("Received unexpected error: %v", err)
		} else {
			t.Fatalf("Received unexpected error: %s: %v", msg[0], err)
		}
	}
}

func assertEmpty(t *testing.T, s string) {
	t.Helper()

	if s != "" {
		t.Errorf("Received non empty value: %v", s)
	}
}

func assertContains(t *testing.T, s string, substr string, msg ...string) {
	t.Helper()

	if !strings.Contains(s, substr) {
		if len(msg) > 0 {
			t.Logf(msg[0])
		}
		t.Errorf("%q does not contain %q", s, substr)
	}
}

func assertEqual(t *testing.T, got string, want string, msg ...string) {
	t.Helper()

	if want != got {
		if len(msg) > 0 {
			t.Logf(msg[0])
		}
		t.Errorf("does not match\nExpected:\n\t%q\nActual:\n\t%q\n", want, got)
	}
}
