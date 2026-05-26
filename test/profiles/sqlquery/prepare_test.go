package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	sqlqueryprofiles "github.com/devr-tools/szr/internal/profiles/sqlquery"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSQLQueryProfilePrepare(t *testing.T) {
	list := sqlqueryprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "sql-query")
	advanced := config.Default().Advanced

	for _, display := range [][]string{
		{"psql", "app", "-c", "select 1"},
		{"sqlite3", "app.db", "select * from users"},
		{"mysql", "app", "-e", "show tables"},
		{"duckdb", "app.duckdb", "-c", "select * from widgets"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match sql-query", display)
		}
	}
	for _, display := range [][]string{
		{"psql", "app"},
		{"sqlite3", "app.db"},
		{"mysql", "app"},
		{"duckdb", "app.duckdb"},
	} {
		if profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("did not expect %#v to match sql-query", display)
		}
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"psql", "app", "-c", "select 1"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"psql", "app", "-c", "select 1", "-q", "--csv"}) {
		t.Fatalf("unexpected psql prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"sqlite3", "app.db", "select * from users"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"sqlite3", "app.db", "select * from users", "-json"}) {
		t.Fatalf("unexpected sqlite3 prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"mysql", "app", "-e", "show tables"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"mysql", "app", "-e", "show tables", "--batch", "--raw"}) {
		t.Fatalf("unexpected mysql prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"duckdb", "app.duckdb", "-c", "select * from widgets"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"duckdb", "app.duckdb", "-c", "select * from widgets", "-json"}) {
		t.Fatalf("unexpected duckdb prepare: %#v", got)
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"psql", "-A", "-c", "select 1"}, Advanced: advanced})
	if want := []string{"psql", "-A", "-c", "select 1", "-q"}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected explicit psql format flag to be preserved: %#v", preserved)
	}

	passthrough := profile.Prepare(engine.Invocation{Command: []string{"mysql", "app", "-e", "select 1"}})
	if want := []string{"mysql", "app", "-e", "select 1"}; !reflect.DeepEqual(passthrough, want) {
		t.Fatalf("expected non-aggressive prepare passthrough: %#v", passthrough)
	}
}
