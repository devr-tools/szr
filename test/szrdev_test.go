package test

import (
	"context"
	"strings"
	"testing"

	"szr/internal/szrdev"
)

func TestSZRDevRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = szrdev.Run(context.Background(), []string{"--version"})
	})
	if code != 0 {
		t.Fatalf("unexpected code: %d", code)
	}
	if !strings.Contains(stdout, "szr dev") || stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestSZRDevAppRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := szrdev.New()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = app.Run(context.Background(), []string{"--version"})
	})
	if code != 0 {
		t.Fatalf("unexpected code: %d", code)
	}
	if !strings.Contains(stdout, "szr dev") || stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
}
