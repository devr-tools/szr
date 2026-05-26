package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	httpapiprofiles "github.com/devr-tools/szr/internal/profiles/httpapi"
	"github.com/devr-tools/szr/test/testutil"
)

func TestHTTPAPIProfilePrepare(t *testing.T) {
	list := httpapiprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "http-api")

	if !profile.Match(engine.Invocation{Display: []string{"curl", "https://api.example.test/v1/users"}}) {
		t.Fatal("expected curl API request to match")
	}
	if profile.Match(engine.Invocation{Display: []string{"curl", "https://example.test/index.html"}}) {
		t.Fatal("did not expect plain webpage curl request to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"curl", "https://api.example.test/v1/users"}}); !reflect.DeepEqual(got, []string{"curl", "-sS", "-i", "https://api.example.test/v1/users"}) {
		t.Fatalf("unexpected curl prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"curl", "-i", "https://api.example.test/v1/users"}}); !reflect.DeepEqual(got, []string{"curl", "-i", "-sS", "https://api.example.test/v1/users"}) {
		t.Fatalf("expected explicit curl include flag to be preserved: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"http", "GET", "https://api.example.test/v1/users"}}) {
		t.Fatal("expected httpie request to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"http", "GET", "https://api.example.test/v1/users"}}); !reflect.DeepEqual(got, []string{"http", "GET", "--print=hb", "https://api.example.test/v1/users"}) {
		t.Fatalf("unexpected http prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"http", "GET", "--print=HhBb", "https://api.example.test/v1/users"}}); !reflect.DeepEqual(got, []string{"http", "GET", "--print=HhBb", "https://api.example.test/v1/users"}) {
		t.Fatalf("expected explicit http print mode to be preserved: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"wget", "-qO-", "https://api.example.test/v1/users"}}) {
		t.Fatal("expected wget stdout API request to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"wget", "-qO-", "https://api.example.test/v1/users"}}); !reflect.DeepEqual(got, []string{"wget", "-qO-", "-S", "https://api.example.test/v1/users"}) {
		t.Fatalf("unexpected wget prepare: %#v", got)
	}
}
