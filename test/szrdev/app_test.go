package szrdev_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/szrdev"
	"github.com/devr-tools/szr/test/testutil"
)

func TestRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var code int
	stdout, stderr := testutil.CaptureOutput(t, func() {
		code = szrdev.Run(context.Background(), []string{"--version"})
	})
	if code != 0 {
		t.Fatalf("unexpected code: %d", code)
	}
	if !strings.Contains(stdout, "szr dev") || stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestNewRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := szrdev.New()
	var code int
	stdout, stderr := testutil.CaptureOutput(t, func() {
		code = app.Run(context.Background(), []string{"--version"})
	})
	if code != 0 {
		t.Fatalf("unexpected code: %d", code)
	}
	if !strings.Contains(stdout, "szr dev") || stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
}
