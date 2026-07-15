package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	pulumiprofiles "github.com/devr-tools/szr/internal/profiles/pulumi"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPulumiDiffProfileMatch(t *testing.T) {
	profile := testutil.FindProfile(t, pulumiprofiles.Profiles(6), "pulumi-diff")

	for _, display := range [][]string{
		{"pulumi", "preview"},
		{"pulumi", "up", "--yes"},
		{"pulumi", "destroy"},
		{"pulumi", "refresh"},
		{"pulumi", "-s", "dev", "preview"},
		{"pulumi", "--stack", "prod", "up"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match pulumi-diff", display)
		}
	}
	for _, display := range [][]string{
		{"pulumi", "stack", "ls"},
		{"pulumi", "config", "get", "up"},
		{"pulumi"},
		{"terraform", "plan"},
	} {
		if profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("did not expect %#v to match pulumi-diff", display)
		}
	}
}

func TestPulumiDiffProfileRenderAndStream(t *testing.T) {
	profile := testutil.FindProfile(t, pulumiprofiles.Profiles(6), "pulumi-diff")

	preview := strings.Join([]string{
		"Previewing update (demo/dev):",
		"     Type                 Name       Plan",
		"     pulumi:pulumi:Stack  demo-dev",
		" +   ├─ aws:s3:Bucket     artifacts  create",
		"     ├─ aws:sqs:Queue     jobs",
		"Resources:",
		"    + 1 to create",
		"    2 unchanged",
		"Duration: 3s",
	}, "\n")
	rendered := profile.Render(engine.Invocation{}, engine.Execution{Stdout: preview})
	for _, want := range []string{"+ aws:s3:Bucket artifacts create", "+ 1 to create", "2 unchanged", "Duration: 3s"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in pulumi render:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "aws:sqs:Queue") {
		t.Fatalf("expected unchanged resource rows to be dropped, got:\n%s", rendered)
	}

	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected pulumi-diff stream metadata: %#v", profile)
	}
	stream := profile.StreamRender(engine.Invocation{}, profile.Budget)
	stream.ConsumeStdout([]byte("Diagnostics:\n  aws:lambda:Function (webhook):\n    error: rate exceeded\nDuration: 2s\n"))
	got := stream.Result()
	for _, want := range []string{"Diagnostics:", "error: rate exceeded", "Duration: 2s"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in pulumi streamed output:\n%s", want, got)
		}
	}
}
