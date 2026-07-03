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

func TestSummarizeJSToolingKeepsRegistry404PackageSpec(t *testing.T) {
	input := strings.Join([]string{
		"Lockfile is up to date, resolution step is skipped",
		"Progress: resolved 1, reused 0, downloaded 0, added 0",
		" ERR_PNPM_FETCH_404  GET https://registry.npmjs.org/@acme%2Fdesign-tokens: Not Found - 404",
		"This error happened while installing a direct dependency of /workspace/storefront",
		"@acme/design-tokens is not in the npm registry, or you have no permission to fetch it.",
		"No authorization header was set for the request.",
		"Progress: resolved 19, reused 16, downloaded 0, added 0",
	}, "\n")

	got := jsfilter.SummarizeJSTooling(input, 6)
	for _, want := range []string{
		"ERR_PNPM_FETCH_404",
		"registry.npmjs.org/@acme/design-tokens",
		"@acme/design-tokens is not in the npm registry",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in registry 404 summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Progress: resolved") {
		t.Fatalf("expected install progress noise to be dropped:\n%s", got)
	}
}

// TestSummarizeJSToolingKeepsLinterFileHeaders pins the linter-output
// needles: hyperlink-wrapped (OSC-8) and SGR-styled file headers must
// survive stripping, stay grouped above their findings, and — with the
// compression contract armed — every file header and error rule must fit the
// self-capped render.
func TestSummarizeJSToolingKeepsLinterFileHeaders(t *testing.T) {
	link := func(url, text string) string {
		return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
	}
	input := strings.Join([]string{
		"\x1b[4m" + link("file:///workspace/web/src/checkout.ts", "/workspace/web/src/checkout.ts") + "\x1b[24m",
		"  \x1b[2m12:9\x1b[22m  \x1b[31merror\x1b[39m  'coupon' is assigned a value but never used  " + link("https://example.invalid/rules/no-unused-vars", "no-unused-vars"),
		"  \x1b[2m88:14\x1b[22m  \x1b[31merror\x1b[39m  Unsafe member access .discuont on an `any` value  " + link("https://example.invalid/rules/no-unsafe-member-access", "@typescript-eslint/no-unsafe-member-access"),
		"  \x1b[2m102:1\x1b[22m  \x1b[33mwarning\x1b[39m  Unexpected console statement  no-console",
		"",
		"\x1b[4m" + link("file:///workspace/web/src/cart/total.ts", "/workspace/web/src/cart/total.ts") + "\x1b[24m",
		"  \x1b[2m7:3\x1b[22m  \x1b[31merror\x1b[39m  Expected '===' and instead saw '=='  eqeqeq",
		"",
		"\x1b[31m\x1b[1m✖ 4 problems (3 errors, 1 warning)\x1b[22m\x1b[39m",
	}, "\n")

	for _, tc := range []struct {
		name    string
		needles []string
	}{
		{name: "file headers", needles: []string{"/workspace/web/src/checkout.ts", "/workspace/web/src/cart/total.ts"}},
		{name: "error rules", needles: []string{"no-unused-vars", "eqeqeq"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := jsfilter.SummarizeJSToolingUnderContract(input, 10, true)
			for _, needle := range tc.needles {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected needle %q in linter summary:\n%s", needle, got)
				}
			}
			if strings.Contains(got, "\x1b") {
				t.Fatalf("expected escape sequences to be stripped:\n%q", got)
			}
		})
	}

	got := jsfilter.SummarizeJSToolingUnderContract(input, 10, true)
	header := strings.Index(got, "/workspace/web/src/checkout.ts")
	finding := strings.Index(got, "no-unused-vars")
	if header < 0 || finding < 0 || header > finding {
		t.Fatalf("expected the file header above its findings:\n%s", got)
	}
}

func TestSummarizeJSToolingKeepsNPMResolutionErrors(t *testing.T) {
	input := strings.Join([]string{
		"npm ERR! code E404",
		"npm ERR! 404 Not Found - GET https://registry.npmjs.org/@acme%2fdesign-tokens - Not found",
		"npm ERR! 404  '@acme/design-tokens@^2.0.0' is not in this registry.",
		"npm ERR! A complete log of this run can be found in: /home/dev/.npm/_logs/2026-06-30.log",
	}, "\n")

	got := jsfilter.SummarizeJSTooling(input, 5)
	for _, want := range []string{
		"npm ERR! code E404",
		"'@acme/design-tokens@^2.0.0' is not in this registry.",
		"registry.npmjs.org/@acme/design-tokens",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in npm 404 summary:\n%s", want, got)
		}
	}
}
