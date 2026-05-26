package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestStructuredJSProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)
	advanced := config.Default().Advanced

	bunTest := testutil.FindProfile(t, list, "bun-test")
	if !bunTest.Match(engine.Classify(engine.Invocation{Display: []string{"bun", "test"}})) {
		t.Fatal("expected bun test profile to match")
	}
	if got := bunTest.Prepare(engine.Classify(engine.Invocation{Command: []string{"bun", "test"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"bun", "test", "--no-color"}) {
		t.Fatalf("expected bun test prepare passthrough, got %#v", got)
	}

	vitest := testutil.FindProfile(t, list, "vitest-json")
	if !vitest.Match(engine.Classify(engine.Invocation{Display: []string{"vitest"}})) {
		t.Fatal("expected vitest profile to match")
	}
	if got := vitest.Prepare(engine.Classify(engine.Invocation{Command: []string{"vitest", "run"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"vitest", "run", "--reporter=json", "--color=false"}) {
		t.Fatalf("unexpected vitest prepare: %#v", got)
	}
	if got := vitest.Prepare(engine.Classify(engine.Invocation{Command: []string{"vitest", "--reporter=dot"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"vitest", "--reporter=dot", "--color=false"}) {
		t.Fatalf("expected explicit vitest reporter to be preserved: %#v", got)
	}
	if got := vitest.Prepare(engine.Classify(engine.Invocation{Command: []string{"vitest", "--outputFile=report.json"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"vitest", "--outputFile=report.json", "--color=false"}) {
		t.Fatalf("expected explicit vitest output file to be preserved: %#v", got)
	}
	if got := vitest.Prepare(engine.Classify(engine.Invocation{Advanced: advanced})); !reflect.DeepEqual(got, []string{"--reporter=json", "--color=false"}) {
		t.Fatalf("expected structured vitest args for empty command, got %#v", got)
	}

	jest := testutil.FindProfile(t, list, "jest-json")
	if !jest.Match(engine.Classify(engine.Invocation{Display: []string{"jest"}})) {
		t.Fatal("expected jest profile to match")
	}
	if got := jest.Prepare(engine.Classify(engine.Invocation{Command: []string{"jest", "--runInBand"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"jest", "--runInBand", "--json", "--color=false", "--silent"}) {
		t.Fatalf("unexpected jest prepare: %#v", got)
	}
	if got := jest.Prepare(engine.Classify(engine.Invocation{Command: []string{"jest", "--json"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"jest", "--json", "--color=false", "--silent"}) {
		t.Fatalf("expected explicit jest json to be preserved: %#v", got)
	}
	if got := jest.Prepare(engine.Classify(engine.Invocation{Command: []string{"jest", "--reporters=default"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"jest", "--reporters=default", "--color=false", "--silent"}) {
		t.Fatalf("expected explicit jest reporters to be preserved: %#v", got)
	}
	if got := jest.Prepare(engine.Classify(engine.Invocation{Command: []string{"jest", "--outputFile=report.json"}, Advanced: advanced})); !reflect.DeepEqual(got, []string{"jest", "--outputFile=report.json", "--color=false", "--silent"}) {
		t.Fatalf("expected jest output file to be preserved, got %#v", got)
	}
}
