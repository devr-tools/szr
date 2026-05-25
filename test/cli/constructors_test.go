package cli_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/test/testutil"
)

func TestConstructors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if app := cli.New("1.2.3"); app == nil {
		t.Fatal("expected cli.New app")
	}

	app := cli.NewWithLoader(
		"1.2.3",
		func() (config.Config, config.Paths, error) {
			paths := testutil.Paths(t.TempDir())
			return config.Default(), paths, nil
		},
		func(int) {},
	)
	if app == nil {
		t.Fatal("expected loaded app")
	}

	stdout, stderr := testutil.CaptureOutput(t, func() {
		got := cli.NewWithLoader(
			"1.2.3",
			func() (config.Config, config.Paths, error) {
				return config.Config{}, config.Paths{}, errors.New("boom")
			},
			func(int) {},
		)
		if got != nil {
			t.Fatal("expected nil app on load error")
		}
	})
	if stdout != "" || !strings.Contains(stderr, "failed to load config: boom") {
		t.Fatalf("unexpected constructor output stdout=%q stderr=%q", stdout, stderr)
	}
}
