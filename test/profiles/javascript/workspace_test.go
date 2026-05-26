package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestJSWorkspaceProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	workspace := testutil.FindProfile(t, list, "js-workspace")
	advanced := config.Default().Advanced

	for _, display := range [][]string{
		{"npm", "install"},
		{"pnpm", "lint"},
		{"yarn", "build"},
		{"biome", "check", "."},
		{"esbuild", "src/index.ts"},
		{"next", "build"},
		{"turbo", "run", "build"},
		{"nx", "test", "web"},
		{"rollup", "-c"},
		{"swc", "src"},
		{"ts-node", "scripts/dev.ts"},
		{"vite", "build"},
		{"eslint", "."},
		{"tsc", "--noEmit"},
		{"tsx", "scripts/dev.ts"},
		{"tsup", "src/index.ts"},
		{"webpack", "--config", "webpack.config.js"},
		{"bun", "run", "build"},
		{"node", "scripts/build.mjs"},
	} {
		if !workspace.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match js-workspace", display)
		}
	}

	if workspace.Match(engine.Invocation{Display: []string{"npm", "test"}}) {
		t.Fatal("did not expect npm test to match js-workspace")
	}
	if workspace.Match(engine.Invocation{Display: []string{"node", "server.js"}}) {
		t.Fatal("did not expect plain node scripts to match js-workspace")
	}
	if got := workspace.Prepare(engine.Invocation{Command: []string{"npm", "install"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"npm", "install", "--no-progress", "--no-fund", "--no-audit", "--color=false"}) {
		t.Fatalf("unexpected npm workspace prepare: %#v", got)
	}
	if got := workspace.Prepare(engine.Invocation{Command: []string{"pnpm", "lint"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"pnpm", "lint", "--reporter=append-only", "--color=false"}) {
		t.Fatalf("unexpected pnpm workspace prepare: %#v", got)
	}
	if got := workspace.Prepare(engine.Invocation{Command: []string{"turbo", "run", "build"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"turbo", "run", "build"}) {
		t.Fatalf("expected non-package js-workspace prepare passthrough, got %#v", got)
	}
}
