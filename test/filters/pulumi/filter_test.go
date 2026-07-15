package pulumi_test

import (
	"fmt"
	"strings"
	"testing"

	pulumifilter "github.com/devr-tools/szr/internal/filters/pulumi"
)

func pulumiPreviewOutput(unchanged int) string {
	lines := []string{
		"Previewing update (demo/dev):",
		"     Type                       Name          Plan       Info",
		"     pulumi:pulumi:Stack        demo-dev",
		" +   ├─ aws:s3:Bucket           artifacts     create",
		" ~   ├─ aws:lambda:Function     webhook       update     [diff: ~memorySize]",
		" -   └─ aws:iam:Policy          legacy        delete",
	}
	for i := 0; i < unchanged; i++ {
		lines = append(lines, fmt.Sprintf("     ├─ aws:ssm:Parameter        param-%02d", i))
	}
	lines = append(lines,
		"Resources:",
		"    + 1 to create",
		"    ~ 1 to update",
		"    - 1 to delete",
		fmt.Sprintf("    %d unchanged", unchanged+1),
		"",
		"Duration: 4s",
	)
	return strings.Join(lines, "\n")
}

func TestSummarizePulumiPreviewKeepsChangesAndCounts(t *testing.T) {
	got := pulumifilter.SummarizePulumi(pulumiPreviewOutput(20), 14)
	for _, want := range []string{
		"Previewing update (demo/dev):",
		"+ aws:s3:Bucket artifacts create",
		"~ aws:lambda:Function webhook update [diff: ~memorySize]",
		"- aws:iam:Policy legacy delete",
		"+ 1 to create",
		"21 unchanged",
		"Duration: 4s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in pulumi summary:\n%s", want, got)
		}
	}
	for _, drop := range []string{"param-00", "pulumi:pulumi:Stack", "Type "} {
		if strings.Contains(got, drop) {
			t.Fatalf("expected %q to be dropped from pulumi summary:\n%s", drop, got)
		}
	}
}

func TestSummarizePulumiKeepsDiagnostics(t *testing.T) {
	input := strings.Join([]string{
		"Updating (demo/prod):",
		" ~   ├─ aws:lambda:Function     webhook       update",
		"Diagnostics:",
		"  aws:lambda:Function (webhook):",
		"    error: 1 error occurred: updating function: ThrottlingException: Rate exceeded",
		"Resources:",
		"    ~ 1 to update",
		"    8 unchanged",
		"Duration: 11s",
	}, "\n")
	got := pulumifilter.SummarizePulumi(input, 4)
	for _, want := range []string{
		"Diagnostics:",
		"error: 1 error occurred: updating function: ThrottlingException: Rate exceeded",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to survive a tight budget:\n%s", want, got)
		}
	}
}

func TestPulumiRecoveryInfoReportsOmissions(t *testing.T) {
	kind, summary, requireRaw := pulumifilter.PulumiRecoveryInfo(pulumiPreviewOutput(20), 14)
	if kind != "full-output" || !strings.Contains(summary, "additional lines") || !requireRaw {
		t.Fatalf("unexpected pulumi recovery info: kind=%q summary=%q requireRaw=%v", kind, summary, requireRaw)
	}
	if kind, _, _ := pulumifilter.PulumiRecoveryInfo("Duration: 1s", 14); kind != "" {
		t.Fatalf("expected no recovery for tiny output, got kind=%q", kind)
	}
}

func TestSummarizePulumiFallsBackOnUnknownOutput(t *testing.T) {
	got := pulumifilter.SummarizePulumi("stack ls output\nsomething else\n", 6)
	if !strings.Contains(got, "stack ls output") {
		t.Fatalf("expected compact fallback, got:\n%s", got)
	}
}
