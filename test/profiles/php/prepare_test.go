package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPHPProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "php-tooling")
	advanced := config.Default().Advanced

	for _, tc := range []struct {
		command []string
		want    []string
	}{
		{[]string{"composer", "install"}, []string{"composer", "install", "--no-progress", "--no-ansi"}},
		{[]string{"phpunit"}, []string{"phpunit", "--colors=never", "--no-progress"}},
		{[]string{"pest"}, []string{"pest", "--colors=never", "--no-progress"}},
		{[]string{"php", "artisan", "test"}, []string{"php", "artisan", "test", "--without-tty", "--compact"}},
		{[]string{"phpstan", "analyse"}, []string{"phpstan", "analyse", "--no-progress"}},
		{[]string{"psalm"}, []string{"psalm", "--output-format=console"}},
		{[]string{"phpcs", "app"}, []string{"phpcs", "app", "-q"}},
	} {
		if got := profile.Prepare(engine.Invocation{Command: tc.command, Advanced: advanced}); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("unexpected php prepare for %#v: got %#v want %#v", tc.command, got, tc.want)
		}
	}
}
