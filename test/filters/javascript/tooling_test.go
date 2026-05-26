package javascript_test

import (
	"strings"
	"testing"

	jsfilter "github.com/devr-tools/szr/internal/filters/javascript"
)

func TestSummarizeJSTooling(t *testing.T) {
	input := strings.Join([]string{
		"turbo 2.0.0",
		"vite v5.4.0 building for production...",
		"packages/web:build: src/app.ts:14:7: error: Cannot find module 'x'",
		"packages/api:build: src/server.mts:9:2: Type error: Cannot assign type 'string' to type 'number'",
		"npm ERR! code ELIFECYCLE",
		"npm ERR! missing script: build",
		" ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL  web@1.0.0 build: `vite build`",
	}, "\n")

	got := jsfilter.SummarizeJSTooling(input, 7)
	for _, want := range []string{
		"src/app.ts:14:7: error: Cannot find module 'x'",
		"src/server.mts:9:2: Type error: Cannot assign type 'string' to type 'number'",
		"npm ERR! code ELIFECYCLE",
		"npm ERR! missing script: build",
		"ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in js tooling summary:\n%s", want, got)
		}
	}
}
