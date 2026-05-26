package profiles_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestJSProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)
	advanced := config.Default().Advanced

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
			if got := pm.Match(engine.Classify(tc.inv)); got != tc.want {
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
					Command:  []string{"npm", "test"},
					Display:  []string{"npm", "test"},
					Cwd:      writePackageJSON(t, "vitest run"),
					Advanced: advanced,
				},
				want: []string{"npm", "test", "--no-progress", "--no-fund", "--no-audit", "--color=false", "--", "--reporter=json"},
			},
			{
				name: "pnpm jest",
				inv: engine.Invocation{
					Command:  []string{"pnpm", "test"},
					Display:  []string{"pnpm", "test"},
					Cwd:      writePackageJSON(t, "jest --runInBand"),
					Advanced: advanced,
				},
				want: []string{"pnpm", "test", "--reporter=append-only", "--color=false", "--", "--json", "--silent"},
			},
			{
				name: "yarn vitest",
				inv: engine.Invocation{
					Command:  []string{"yarn", "test", "--watch"},
					Display:  []string{"yarn", "test", "--watch"},
					Cwd:      writePackageJSON(t, "vitest"),
					Advanced: advanced,
				},
				want: []string{"yarn", "test", "--watch", "--silent", "--color=false", "--reporter=json"},
			},
			{
				name: "unknown script passthrough",
				inv: engine.Invocation{
					Command:  []string{"npm", "test"},
					Display:  []string{"npm", "test"},
					Cwd:      writePackageJSON(t, "tsx scripts/smoke.ts"),
					Advanced: advanced,
				},
				want: []string{"npm", "test", "--no-progress", "--no-fund", "--no-audit", "--color=false"},
			},
			{
				name: "preserve npm structured args",
				inv: engine.Invocation{
					Command:  []string{"npm", "test", "--", "--json"},
					Display:  []string{"npm", "test", "--", "--json"},
					Cwd:      writePackageJSON(t, "jest"),
					Advanced: advanced,
				},
				want: []string{"npm", "test", "--no-progress", "--no-fund", "--no-audit", "--color=false", "--", "--json", "--silent"},
			},
			{
				name: "preserve pnpm separator",
				inv: engine.Invocation{
					Command:  []string{"pnpm", "test", "--", "--runInBand"},
					Display:  []string{"pnpm", "test", "--", "--runInBand"},
					Cwd:      writePackageJSON(t, "jest"),
					Advanced: advanced,
				},
				want: []string{"pnpm", "test", "--reporter=append-only", "--color=false", "--", "--runInBand", "--json", "--silent"},
			},
			{
				name: "infer jest from args",
				inv: engine.Invocation{
					Command:  []string{"npm", "test", "--", "--runInBand"},
					Display:  []string{"test", "npm", "test", "--", "--runInBand"},
					Cwd:      t.TempDir(),
					Advanced: advanced,
				},
				want: []string{"npm", "test", "--no-progress", "--no-fund", "--no-audit", "--color=false", "--", "--runInBand", "--json", "--silent"},
			},
			{
				name: "missing package json",
				inv: engine.Invocation{
					Command:  []string{"npm", "test"},
					Display:  []string{"npm", "test"},
					Cwd:      t.TempDir(),
					Advanced: advanced,
				},
				want: []string{"npm", "test", "--no-progress", "--no-fund", "--no-audit", "--color=false"},
			},
			{
				name: "invalid package json",
				inv: engine.Invocation{
					Command:  []string{"npm", "test"},
					Display:  []string{"npm", "test"},
					Cwd:      invalidRoot,
					Advanced: advanced,
				},
				want: []string{"npm", "test", "--no-progress", "--no-fund", "--no-audit", "--color=false"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := pm.Prepare(engine.Classify(tc.inv)); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("unexpected prepare result: got %#v want %#v", got, tc.want)
				}
			})
		}
	})
}
