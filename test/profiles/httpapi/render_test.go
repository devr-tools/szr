package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	httpapiprofiles "github.com/devr-tools/szr/internal/profiles/httpapi"
	"github.com/devr-tools/szr/test/testutil"
)

func TestHTTPAPIProfileRender(t *testing.T) {
	list := httpapiprofiles.Profiles(10)
	profile := testutil.FindProfile(t, list, "http-api")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"HTTP/2 200 OK",
			"content-type: application/json; charset=utf-8",
			"",
			`{"id":"u_123","ok":true,"items":[{"name":"alpha","count":1}]}`,
		}, "\n"),
	})
	for _, want := range []string{
		"status=200 OK content-type=application/json",
		`id="u_123"`,
		"items: array len=1 sample=object{name,count}",
		"ok=true",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered HTTP API output:\n%s", want, rendered)
		}
	}
	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected HTTP API stream metadata: %#v", profile)
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStderr([]byte("HTTP/1.1 201 Created\ncontent-type: application/json\n\n"))
	streamed.ConsumeStdout([]byte(`{"id":"evt_123","status":"queued"}`))
	got := streamed.Result()
	for _, want := range []string{
		"status=201 Created content-type=application/json",
		`id="evt_123"`,
		`status="queued"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in streamed HTTP API output:\n%s", want, got)
		}
	}
	if streamed.BytesParsed() == 0 {
		t.Fatal("expected HTTP API stream reducer to track parsed bytes")
	}
}

func TestHTTPAPIProfileStreamRecovery(t *testing.T) {
	list := httpapiprofiles.Profiles(10)
	profile := testutil.FindProfile(t, list, "http-api")

	stream := profile.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	stream.ConsumeStderr([]byte("HTTP/1.1 429 Too Many Requests\ncontent-type: text/plain\n\n"))
	stream.ConsumeStdout([]byte("rate limit exceeded\nretry later\ncontact support\nrequest id: req_123\n"))

	recoveryStream, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable HTTP API reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recoveryStream.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected HTTP API recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
