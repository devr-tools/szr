package szr_test

import (
	"context"
	"strings"
	"testing"

	szrapp "github.com/devr-tools/szr/pkg/szr"
	"github.com/devr-tools/szr/test/testutil"
)

func TestRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var code int
	stdout, stderr := testutil.CaptureOutput(t, func() {
		code = szrapp.Run(context.Background(), "1.2.3", []string{"--version"})
	})
	if code != 0 {
		t.Fatalf("unexpected code: %d", code)
	}
	if !strings.Contains(stdout, "szr 1.2.3") || stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestNewWithCLIRun(t *testing.T) {
	app := szrapp.NewWithCLI(testutil.NewTestApp(t))

	var code int
	stdout, stderr := testutil.CaptureOutput(t, func() {
		code = app.Run(context.Background(), []string{"--version"})
	})
	if code != 0 {
		t.Fatalf("unexpected code: %d", code)
	}
	if !strings.Contains(stdout, "szr test") || stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
}
