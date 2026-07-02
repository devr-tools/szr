package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestBuildSystemProfileTerraformRender(t *testing.T) {
	list := profiles.Builtins(12)
	profile := testutil.FindProfile(t, list, "build-system")

	if !profile.Match(engine.Invocation{Display: []string{"terraform", "plan"}}) {
		t.Fatal("expected terraform plan to match build-system")
	}
	if !profile.Match(engine.Invocation{Display: []string{"tofu", "apply", "-auto-approve"}}) {
		t.Fatal("expected tofu apply to match build-system")
	}

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"Terraform will perform the following actions:",
			"",
			"  # aws_instance.web will be created",
			`  + resource "aws_instance" "web" {`,
			`      + ami           = "ami-123"`,
			`      + instance_type = "t3.micro"`,
			`      + tags          = { "env" = "prod" }`,
			`      + monitoring    = true`,
			`      + ebs_optimized = true`,
			"    }",
			"",
			"Plan: 1 to add, 0 to change, 0 to destroy.",
		}, "\n"),
	})
	for _, want := range []string{
		"Plan: 1 to add, 0 to change, 0 to destroy.",
		"# aws_instance.web will be created",
		"+2 more attribute lines",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in terraform render output:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "+ ebs_optimized") {
		t.Fatalf("expected attribute diff lines beyond the cap to be dropped:\n%s", rendered)
	}
}
