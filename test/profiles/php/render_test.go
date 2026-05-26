package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPHPProfileCoverage(t *testing.T) {
	list := profiles.Builtins(4)
	php := testutil.FindProfile(t, list, "php-tooling")

	for _, display := range [][]string{
		{"composer", "install"},
		{"phpunit"},
		{"pest"},
		{"phpstan", "analyse"},
		{"psalm"},
		{"phpcs", "app"},
		{"php-cs-fixer", "fix", "--dry-run"},
		{"php", "artisan", "test"},
		{"php", "vendor/bin/phpunit"},
	} {
		if !php.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match php-tooling", display)
		}
	}

	for _, display := range [][]string{
		{"php", "-v"},
		{"bash", "scripts/test.sh"},
	} {
		if php.Match(engine.Invocation{Display: display}) {
			t.Fatalf("did not expect %#v to match php-tooling", display)
		}
	}

	rendered := php.Render(engine.Invocation{}, engine.Execution{
		Stdout: "Problem 1\nScript @phpunit returned with error code 2\n",
		Stderr: "Failed asserting that 200 is identical to 500.\n/app/tests/Feature/HealthCheckTest.php:27\n",
	})
	for _, want := range []string{"Failed asserting that 200 is identical to 500.", "/app/tests/Feature/HealthCheckTest.php:27", "Problem 1"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in php profile render output:\n%s", want, rendered)
		}
	}

	if php.StreamPreference != engine.StreamStdoutFirst || php.StreamRender == nil {
		t.Fatalf("unexpected php stream metadata: %#v", php)
	}
}
