package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	sqlqueryprofiles "github.com/devr-tools/szr/internal/profiles/sqlquery"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSQLQueryProfileRender(t *testing.T) {
	list := sqlqueryprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "sql-query")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"id | name",
			"----+------",
			"1  | alpha",
			"(1 row)",
		}, "\n"),
	})
	for _, want := range []string{
		"id | name",
		"1  | alpha",
		"(1 row)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered SQL output:\n%s", want, rendered)
		}
	}

	errorRendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stderr: "ERROR 1054 (42S22): Unknown column 'missing' in 'field list'\n",
	})
	if !strings.Contains(errorRendered, "Unknown column 'missing'") {
		t.Fatalf("expected mysql error in rendered SQL output:\n%s", errorRendered)
	}

	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil || profile.Budget.MaxLines < 6 {
		t.Fatalf("unexpected sql-query stream metadata: %#v", profile)
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte(`[{"id":1,"name":"alpha"},`))
	streamed.ConsumeStdout([]byte(`{"id":2,"name":"beta"}]`))
	streamRendered := streamed.Result()
	for _, want := range []string{
		"2 row(s)",
		`{"id":1,"name":"alpha"}`,
		`{"id":2,"name":"beta"}`,
	} {
		if !strings.Contains(streamRendered, want) {
			t.Fatalf("expected %q in streamed SQL output:\n%s", want, streamRendered)
		}
	}
}
