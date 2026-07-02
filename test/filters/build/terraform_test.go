package build_test

import (
	"strings"
	"testing"

	buildfilter "github.com/devr-tools/szr/internal/filters/build"
)

func TestSummarizeTerraformPlan(t *testing.T) {
	input := strings.Join([]string{
		"Terraform used the selected providers to generate the following execution plan.",
		"Resource actions are indicated with the following symbols:",
		"  + create",
		"",
		"Terraform will perform the following actions:",
		"",
		"  # aws_instance.web[0] will be created",
		`  + resource "aws_instance" "web" {`,
		`      + ami           = "ami-123"`,
		`      + instance_type = "t3.micro"`,
		"      + tags = {",
		`      ~ vpc_id = "a" -> "b"`,
		"    }",
		"",
		"  # aws_instance.web[1] will be created",
		"  # aws_s3_bucket.logs will be created",
		"  # aws_iam_role.app will be created",
		"  # aws_security_group.web will be created",
		"  # aws_db_instance.main will be destroyed",
		"",
		"Plan: 5 to add, 0 to change, 1 to destroy.",
	}, "\n")

	got := buildfilter.SummarizeBuildSystem(input, 8)
	for _, want := range []string{
		"Plan: 5 to add, 0 to change, 1 to destroy.",
		"# aws_instance.web[0] will be created",
		"+2 more resource changes",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in terraform plan summary:\n%s", want, got)
		}
	}
	for _, reject := range []string{"+ instance_type", "aws_db_instance.main"} {
		if strings.Contains(got, reject) {
			t.Fatalf("expected %q to be dropped from terraform plan summary:\n%s", reject, got)
		}
	}
}

func TestSummarizeTerraformApplyComplete(t *testing.T) {
	input := strings.Join([]string{
		"aws_instance.web: Creating...",
		"aws_instance.web: Still creating... [10s elapsed]",
		"aws_instance.web: Creation complete after 12s [id=i-abc]",
		"",
		"Apply complete! Resources: 1 added, 0 changed, 0 destroyed.",
	}, "\n")

	got := buildfilter.SummarizeBuildSystem(input, 6)
	if !strings.Contains(got, "Apply complete! Resources: 1 added, 0 changed, 0 destroyed.") {
		t.Fatalf("expected apply summary to be kept:\n%s", got)
	}
	if strings.Contains(got, "Still creating") {
		t.Fatalf("expected apply progress noise to be dropped:\n%s", got)
	}
}

func TestSummarizeTerraformErrorBlock(t *testing.T) {
	input := strings.Join([]string{
		"╷",
		"│ Error: Reference to undeclared resource",
		"│",
		`│   on main.tf line 12, in resource "aws_instance" "web":`,
		"│   12:   ami = aws_ami.foo.id",
		"│",
		`│ A managed resource "aws_ami" "foo" has not been declared in the root module.`,
		"╵",
		"Planning failed.",
	}, "\n")

	got := buildfilter.SummarizeBuildSystem(input, 8)
	for _, want := range []string{
		"Error: Reference to undeclared resource",
		"on main.tf line 12",
		`A managed resource "aws_ami" "foo" has not been declared in the root module.`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in terraform error summary:\n%s", want, got)
		}
	}
}

func TestSummarizeBuildSystemMavenBracketedKeywords(t *testing.T) {
	input := strings.Join([]string{
		"[INFO] Scanning for projects...",
		"[INFO] Building app 1.0.0",
		"[INFO] --- maven-compiler-plugin:3.11.0:compile ---",
		"[INFO] Compiling 12 source files",
		"[INFO] ------------------------------------------------------------",
		"[INFO] BUILD FAILURE",
		"[INFO] ------------------------------------------------------------",
		"[ERROR] Failed to execute goal org.apache.maven.plugins:maven-compiler-plugin:3.11.0:compile (default-compile) on project app: Compilation failure",
		"[ERROR] /src/main/java/App.java:[10,5] cannot find symbol",
	}, "\n")

	got := buildfilter.SummarizeBuildSystem(input, 4)
	for _, want := range []string{
		"[ERROR] Failed to execute goal",
		"[ERROR] /src/main/java/App.java:[10,5] cannot find symbol",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in maven summary:\n%s", want, got)
		}
	}
}
