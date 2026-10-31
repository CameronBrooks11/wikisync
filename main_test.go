package main

import (
	"os"
	"testing"
)

// silence redirects stdout/stderr to /dev/null for the duration of a test, so
// the CLI's normal output doesn't clutter `go test`.
func silence(t *testing.T) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	t.Cleanup(func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = devnull.Close()
	})
}

func TestRun(t *testing.T) {
	// Keep the result independent of credentials in the developer's shell.
	for _, v := range []string{"WIKISYNC_USERNAME", "WIKISYNC_PASSWORD",
		"WIKISYNC_USERNAME_APPROPEDIA", "WIKISYNC_PASSWORD_APPROPEDIA"} {
		t.Setenv(v, "")
	}
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no args prints usage", []string{}, 0},
		{"version", []string{"--version"}, 0},
		{"unknown command", []string{"bogus"}, 2},
		{"pull with nothing named or tracked", []string{"pull", "--dir", t.TempDir()}, 2},
		{"status on empty dir", []string{"status", "--dir", t.TempDir()}, 0},
		{"diff on empty dir", []string{"diff", "--dir", t.TempDir()}, 0},
		{"push preview, nothing to push", []string{"push", "--dir", t.TempDir()}, 0},
		{"push apply needs summary", []string{"push", "--apply", "--dir", t.TempDir()}, 2},
		{"push apply needs credentials", []string{"push", "--apply", "--summary", "s", "--dir", t.TempDir()}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silence(t)
			if got := run(tc.args); got != tc.want {
				t.Errorf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}
