package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	tabularprofiles "github.com/devr-tools/szr/internal/profiles/tabular"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCSVTabularProfilePrepare(t *testing.T) {
	t.Parallel()

	profile := testutil.FindProfile(t, tabularprofiles.Profiles(6), "csv-tabular")

	for _, tc := range []struct {
		display []string
		match   bool
	}{
		{display: []string{"ps", "aux"}, match: true},
		{display: []string{"df", "-h"}, match: true},
		{display: []string{"du", "-sh", "."}, match: true},
		{display: []string{"systemctl", "list-units"}, match: true},
		{display: []string{"helm", "list"}, match: true},
		{display: []string{"kubectl", "get", "pods", "-o", "wide"}, match: true},
		{display: []string{"kubectl", "get", "pods"}, match: false},
		{display: []string{"systemctl", "status", "nginx"}, match: false},
	} {
		if got := profile.Match(engine.Invocation{Display: tc.display}); got != tc.match {
			t.Fatalf("match(%#v)=%v want %v", tc.display, got, tc.match)
		}
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"ps", "aux"}}); !reflect.DeepEqual(got, []string{"ps", "aux", "-eo", "pid,ppid,user,%cpu,%mem,etime,command"}) {
		t.Fatalf("unexpected ps prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"df"}}); !reflect.DeepEqual(got, []string{"df", "-P", "-k"}) {
		t.Fatalf("unexpected df prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"du", "var/log"}}); !reflect.DeepEqual(got, []string{"du", "-k", "var/log"}) {
		t.Fatalf("unexpected du prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"systemctl", "list-units"}}); !reflect.DeepEqual(got, []string{"systemctl", "list-units", "--plain", "--no-pager"}) {
		t.Fatalf("unexpected systemctl prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"kubectl", "get", "pods", "-o", "wide"}}); !reflect.DeepEqual(got, []string{"kubectl", "get", "pods", "-o", "wide"}) {
		t.Fatalf("expected explicit wide kubectl command to be preserved: %#v", got)
	}
}
