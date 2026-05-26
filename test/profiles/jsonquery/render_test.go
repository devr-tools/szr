package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	jsonqueryprofiles "github.com/devr-tools/szr/internal/profiles/jsonquery"
	"github.com/devr-tools/szr/test/testutil"
)

func TestJSONQueryProfileRender(t *testing.T) {
	list := jsonqueryprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "json-query")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: `{"user":{"id":7,"name":"alex"},"items":[1,2]}`,
	})
	for _, want := range []string{`root: object keys=2`, `user: object keys=2`, `user.id=7`} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in json-query render output:\n%s", want, rendered)
		}
	}

	errored := profile.Render(engine.Invocation{}, engine.Execution{
		Stderr: "jq: parse error: Invalid numeric literal at line 1, column 4",
	})
	if !strings.Contains(errored, "jq: parse error") {
		t.Fatalf("expected parse error in render output:\n%s", errored)
	}

	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected json-query stream metadata: %#v", profile)
	}

	reducer := profile.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 6})
	reducer.ConsumeStdout([]byte(`{"items":[1,2],"ok":true}`))
	streamed := reducer.Result()
	for _, want := range []string{`items: array len=2 sample=1, 2`, `ok=true`} {
		if !strings.Contains(streamed, want) {
			t.Fatalf("expected %q in stream render output:\n%s", want, streamed)
		}
	}
}
