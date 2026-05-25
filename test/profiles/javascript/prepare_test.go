package profiles_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestJSProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	writePackageJSON := func(t *testing.T, script string) string {
		t.Helper()
		root := t.TempDir()
		data := `{"scripts":{"test":"` + script + `"}}`
		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(data), 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
		return root
	}

	pm := testutil.FindProfile(t, list, "js-package-test")
	t.Run("match", func(t *testing.T) {
		for _, tc := range []struct {
			inv  engine.Invocation
			want bool
		}{
			{engine.Invocation{Display: []string{"npm", "test"}}, true},
			{engine.Invocation{Display: []string{"test", "npm", "test"}, Command: []string{"npm", "test"}}, true},
			{engine.Invocation{Display: []string{"pnpm", "run", "test"}}, true},
			{engine.Invocation{Display: []string{"yarn", "run", "test"}}, true},
			{engine.Invocation{Display: []string{"npm", "install"}}, false},
		} {
			if got := pm.Match(tc.inv); got != tc.want {
				t.Fatalf("unexpected match result for %#v: got %t want %t", tc.inv, got, tc.want)
			}
		}
	})

	t.Run("prepare", func(t *testing.T) {
		invalidRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(invalidRoot, "package.json"), []byte("{bad"), 0o644); err != nil {
			t.Fatalf("write invalid package.json: %v", err)
		}

		for _, tc := range []struct {
			name string
			inv  engine.Invocation
			want []string
		}{
			{
				name: "npm vitest",
				inv: engine.Invocation{
					Command: []string{"npm", "test"},
					Display: []string{"npm", "test"},
					Cwd:     writePackageJSON(t, "vitest run"),
				},
				want: []string{"npm", "test", "--", "--reporter=json"},
			},
			{
				name: "pnpm jest",
				inv: engine.Invocation{
					Command: []string{"pnpm", "test"},
					Display: []string{"pnpm", "test"},
					Cwd:     writePackageJSON(t, "jest --runInBand"),
				},
				want: []string{"pnpm", "test", "--", "--json"},
			},
			{
				name: "yarn vitest",
				inv: engine.Invocation{
					Command: []string{"yarn", "test", "--watch"},
					Display: []string{"yarn", "test", "--watch"},
					Cwd:     writePackageJSON(t, "vitest"),
				},
				want: []string{"yarn", "test", "--watch", "--reporter=json"},
			},
			{
				name: "unknown script passthrough",
				inv: engine.Invocation{
					Command: []string{"npm", "test"},
					Display: []string{"npm", "test"},
					Cwd:     writePackageJSON(t, "tsx scripts/smoke.ts"),
				},
				want: []string{"npm", "test"},
			},
			{
				name: "preserve npm structured args",
				inv: engine.Invocation{
					Command: []string{"npm", "test", "--", "--json"},
					Display: []string{"npm", "test", "--", "--json"},
					Cwd:     writePackageJSON(t, "jest"),
				},
				want: []string{"npm", "test", "--", "--json"},
			},
			{
				name: "preserve pnpm separator",
				inv: engine.Invocation{
					Command: []string{"pnpm", "test", "--", "--runInBand"},
					Display: []string{"pnpm", "test", "--", "--runInBand"},
					Cwd:     writePackageJSON(t, "jest"),
				},
				want: []string{"pnpm", "test", "--", "--runInBand", "--json"},
			},
			{
				name: "infer jest from args",
				inv: engine.Invocation{
					Command: []string{"npm", "test", "--", "--runInBand"},
					Display: []string{"test", "npm", "test", "--", "--runInBand"},
					Cwd:     t.TempDir(),
				},
				want: []string{"npm", "test", "--", "--runInBand", "--json"},
			},
			{
				name: "missing package json",
				inv: engine.Invocation{
					Command: []string{"npm", "test"},
					Display: []string{"npm", "test"},
					Cwd:     t.TempDir(),
				},
				want: []string{"npm", "test"},
			},
			{
				name: "invalid package json",
				inv: engine.Invocation{
					Command: []string{"npm", "test"},
					Display: []string{"npm", "test"},
					Cwd:     invalidRoot,
				},
				want: []string{"npm", "test"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := pm.Prepare(tc.inv); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("unexpected prepare result: got %#v want %#v", got, tc.want)
				}
			})
		}
	})
}

func TestStructuredJSProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	bunTest := testutil.FindProfile(t, list, "bun-test")
	if !bunTest.Match(engine.Invocation{Display: []string{"bun", "test"}}) {
		t.Fatal("expected bun test profile to match")
	}
	if got := bunTest.Prepare(engine.Invocation{Command: []string{"bun", "test"}}); !reflect.DeepEqual(got, []string{"bun", "test"}) {
		t.Fatalf("expected bun test prepare passthrough, got %#v", got)
	}

	vitest := testutil.FindProfile(t, list, "vitest-json")
	if !vitest.Match(engine.Invocation{Display: []string{"vitest"}}) {
		t.Fatal("expected vitest profile to match")
	}
	if got := vitest.Prepare(engine.Invocation{Command: []string{"vitest", "run"}}); !reflect.DeepEqual(got, []string{"vitest", "run", "--reporter=json"}) {
		t.Fatalf("unexpected vitest prepare: %#v", got)
	}
	if got := vitest.Prepare(engine.Invocation{Command: []string{"vitest", "--reporter=dot"}}); !reflect.DeepEqual(got, []string{"vitest", "--reporter=dot"}) {
		t.Fatalf("expected explicit vitest reporter to be preserved: %#v", got)
	}
	if got := vitest.Prepare(engine.Invocation{Command: []string{"vitest", "--outputFile=report.json"}}); !reflect.DeepEqual(got, []string{"vitest", "--outputFile=report.json"}) {
		t.Fatalf("expected explicit vitest output file to be preserved: %#v", got)
	}
	if got := vitest.Prepare(engine.Invocation{}); !reflect.DeepEqual(got, []string{"--reporter=json"}) {
		t.Fatalf("expected structured vitest args for empty command, got %#v", got)
	}

	jest := testutil.FindProfile(t, list, "jest-json")
	if !jest.Match(engine.Invocation{Display: []string{"jest"}}) {
		t.Fatal("expected jest profile to match")
	}
	if got := jest.Prepare(engine.Invocation{Command: []string{"jest", "--runInBand"}}); !reflect.DeepEqual(got, []string{"jest", "--runInBand", "--json"}) {
		t.Fatalf("unexpected jest prepare: %#v", got)
	}
	if got := jest.Prepare(engine.Invocation{Command: []string{"jest", "--json"}}); !reflect.DeepEqual(got, []string{"jest", "--json"}) {
		t.Fatalf("expected explicit jest json to be preserved: %#v", got)
	}
	if got := jest.Prepare(engine.Invocation{Command: []string{"jest", "--reporters=default"}}); !reflect.DeepEqual(got, []string{"jest", "--reporters=default"}) {
		t.Fatalf("expected explicit jest reporters to be preserved: %#v", got)
	}
	if got := jest.Prepare(engine.Invocation{Command: []string{"jest", "--outputFile=report.json"}}); !reflect.DeepEqual(got, []string{"jest", "--outputFile=report.json"}) {
		t.Fatalf("expected jest output file to be preserved, got %#v", got)
	}
}

func TestJSWorkspaceProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	workspace := testutil.FindProfile(t, list, "js-workspace")

	for _, display := range [][]string{
		{"npm", "install"},
		{"pnpm", "lint"},
		{"yarn", "build"},
		{"turbo", "run", "build"},
		{"nx", "test", "web"},
		{"vite", "build"},
		{"eslint", "."},
		{"tsc", "--noEmit"},
		{"bun", "run", "build"},
	} {
		if !workspace.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match js-workspace", display)
		}
	}

	if workspace.Match(engine.Invocation{Display: []string{"npm", "test"}}) {
		t.Fatal("did not expect npm test to match js-workspace")
	}
	if got := workspace.Prepare(engine.Invocation{Command: []string{"turbo", "run", "build"}}); !reflect.DeepEqual(got, []string{"turbo", "run", "build"}) {
		t.Fatalf("expected js-workspace prepare passthrough, got %#v", got)
	}
}
