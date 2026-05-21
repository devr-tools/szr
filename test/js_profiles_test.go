package test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
)

func TestJSProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)
	find := func(name string) engine.Profile {
		t.Helper()
		for _, profile := range list {
			if profile.Name == name {
				return profile
			}
		}
		t.Fatalf("missing profile %s", name)
		return engine.Profile{}
	}

	writePackageJSON := func(t *testing.T, script string) string {
		t.Helper()
		root := t.TempDir()
		data := `{"scripts":{"test":"` + script + `"}}`
		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(data), 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
		return root
	}

	pm := find("js-package-test")
	if !pm.Match(engine.Invocation{Display: []string{"npm", "test"}}) {
		t.Fatal("expected npm test to match package profile")
	}
	if !pm.Match(engine.Invocation{Display: []string{"pnpm", "run", "test"}}) {
		t.Fatal("expected pnpm run test to match package profile")
	}
	if !pm.Match(engine.Invocation{Display: []string{"yarn", "run", "test"}}) {
		t.Fatal("expected yarn run test to match package profile")
	}
	if pm.Match(engine.Invocation{Display: []string{"npm", "install"}}) {
		t.Fatal("did not expect npm install to match package profile")
	}

	npmVitest := pm.Prepare(engine.Invocation{
		Command: []string{"npm", "test"},
		Display: []string{"npm", "test"},
		Cwd:     writePackageJSON(t, "vitest run"),
	})
	if want := []string{"npm", "test", "--", "--reporter=json"}; !reflect.DeepEqual(npmVitest, want) {
		t.Fatalf("unexpected npm vitest prepare: %#v", npmVitest)
	}

	pnpmJest := pm.Prepare(engine.Invocation{
		Command: []string{"pnpm", "test"},
		Display: []string{"pnpm", "test"},
		Cwd:     writePackageJSON(t, "jest --runInBand"),
	})
	if want := []string{"pnpm", "test", "--", "--json"}; !reflect.DeepEqual(pnpmJest, want) {
		t.Fatalf("unexpected pnpm jest prepare: %#v", pnpmJest)
	}

	yarnVitest := pm.Prepare(engine.Invocation{
		Command: []string{"yarn", "test", "--watch"},
		Display: []string{"yarn", "test", "--watch"},
		Cwd:     writePackageJSON(t, "vitest"),
	})
	if want := []string{"yarn", "test", "--watch", "--reporter=json"}; !reflect.DeepEqual(yarnVitest, want) {
		t.Fatalf("unexpected yarn vitest prepare: %#v", yarnVitest)
	}

	unknown := pm.Prepare(engine.Invocation{
		Command: []string{"npm", "test"},
		Display: []string{"npm", "test"},
		Cwd:     writePackageJSON(t, "tsx scripts/smoke.ts"),
	})
	if want := []string{"npm", "test"}; !reflect.DeepEqual(unknown, want) {
		t.Fatalf("expected unknown script passthrough: %#v", unknown)
	}

	preserved := pm.Prepare(engine.Invocation{
		Command: []string{"npm", "test", "--", "--json"},
		Display: []string{"npm", "test", "--", "--json"},
		Cwd:     writePackageJSON(t, "jest"),
	})
	if want := []string{"npm", "test", "--", "--json"}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected structured npm args to be preserved: %#v", preserved)
	}

	pnpmPreserved := pm.Prepare(engine.Invocation{
		Command: []string{"pnpm", "test", "--", "--runInBand"},
		Display: []string{"pnpm", "test", "--", "--runInBand"},
		Cwd:     writePackageJSON(t, "jest"),
	})
	if want := []string{"pnpm", "test", "--", "--runInBand", "--json"}; !reflect.DeepEqual(pnpmPreserved, want) {
		t.Fatalf("expected pnpm forwarded args to reuse existing separator: %#v", pnpmPreserved)
	}

	missingPkg := pm.Prepare(engine.Invocation{
		Command: []string{"npm", "test"},
		Display: []string{"npm", "test"},
		Cwd:     t.TempDir(),
	})
	if want := []string{"npm", "test"}; !reflect.DeepEqual(missingPkg, want) {
		t.Fatalf("expected missing package.json passthrough: %#v", missingPkg)
	}

	invalidRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidRoot, "package.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatalf("write invalid package.json: %v", err)
	}
	invalidPkg := pm.Prepare(engine.Invocation{
		Command: []string{"npm", "test"},
		Display: []string{"npm", "test"},
		Cwd:     invalidRoot,
	})
	if want := []string{"npm", "test"}; !reflect.DeepEqual(invalidPkg, want) {
		t.Fatalf("expected invalid package.json passthrough: %#v", invalidPkg)
	}

	vitest := find("vitest-json")
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

	jest := find("jest-json")
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
}

func TestJSProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)
	var profile engine.Profile
	for _, candidate := range list {
		if candidate.Name == "js-package-test" {
			profile = candidate
			break
		}
	}
	if profile.Name == "" {
		t.Fatal("missing js-package-test profile")
	}

	report := strings.Join([]string{
		"> app@test",
		"> vitest run --reporter=json",
		`{"numPassedTestSuites":1,"numFailedTestSuites":1,"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: expect(received).toBe(expected)\nExpected: 2\nReceived: 3\nat src/math.test.ts:12:3"]}]}]}`,
	}, "\n")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{Stdout: report})
	for _, want := range []string{"suites: pass=1 fail=1", "src/math.test.ts", "math subtracts", "Expected: 2", "Received: 3"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered output:\n%s", want, rendered)
		}
	}
}
