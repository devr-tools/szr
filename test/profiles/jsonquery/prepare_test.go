package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	jsonqueryprofiles "github.com/devr-tools/szr/internal/profiles/jsonquery"
	"github.com/devr-tools/szr/test/testutil"
)

func TestJSONQueryProfilePrepare(t *testing.T) {
	list := jsonqueryprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "json-query")
	advanced := config.Default().Advanced
	advanced.AggressivePrepareRewrites = true

	for _, display := range [][]string{
		{"jq", ".items[]", "data.json"},
		{"yq", ".items[]", "data.yml"},
		{"yq", "eval", ".items[]", "data.yml"},
		{"cat", "data.json"},
		{"python", "-m", "json.tool", "data.json"},
		{"python3", "-m", "json.tool", "data.ndjson"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match json-query", display)
		}
	}

	for _, display := range [][]string{
		{"cat", "README.md"},
		{"python", "-m", "json.tool"},
		{"yq", "write", "-i", "data.yml", ".foo", "bar"},
	} {
		if profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("did not expect %#v to match json-query", display)
		}
	}

	if got := profile.Prepare(engine.Invocation{
		Command:  []string{"jq", ".items[]", "data.json"},
		Advanced: advanced,
	}); !reflect.DeepEqual(got, []string{"jq", ".items[]", "data.json", "-M"}) {
		t.Fatalf("unexpected jq prepare: %#v", got)
	}

	if got := profile.Prepare(engine.Invocation{
		Command:  []string{"jq", "-C", ".items[]", "data.json"},
		Advanced: advanced,
	}); !reflect.DeepEqual(got, []string{"jq", "-C", ".items[]", "data.json"}) {
		t.Fatalf("expected explicit jq color flag to be preserved: %#v", got)
	}

	if got := profile.Prepare(engine.Invocation{
		Command:  []string{"yq", ".items[]", "data.yml"},
		Advanced: advanced,
	}); !reflect.DeepEqual(got, []string{"yq", ".items[]", "data.yml", "-o=json"}) {
		t.Fatalf("unexpected yq prepare: %#v", got)
	}

	if got := profile.Prepare(engine.Invocation{
		Command:  []string{"yq", "-o=yaml", ".items[]", "data.yml"},
		Advanced: advanced,
	}); !reflect.DeepEqual(got, []string{"yq", "-o=yaml", ".items[]", "data.yml"}) {
		t.Fatalf("expected explicit yq output format to be preserved: %#v", got)
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"jq", ".items[]", "data.json"}}); !reflect.DeepEqual(got, []string{"jq", ".items[]", "data.json"}) {
		t.Fatalf("expected non-aggressive prepare passthrough, got %#v", got)
	}
}
