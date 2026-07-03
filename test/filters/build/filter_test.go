package build_test

import (
	"fmt"
	"strings"
	"testing"

	buildfilter "github.com/devr-tools/szr/internal/filters/build"
	"github.com/devr-tools/szr/internal/history"
)

func TestSummarizeBuildSystem(t *testing.T) {
	input := strings.Join([]string{
		"[3/10] Building CXX object src/app.cpp.o",
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"ninja: build stopped: subcommand failed.",
	}, "\n")

	got := buildfilter.SummarizeBuildSystem(input, 4)
	for _, want := range []string{
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"ninja: build stopped: subcommand failed.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in build summary:\n%s", want, got)
		}
	}
}

// TestSummarizeBuildSystemFailureDetails pins the failure-detail retention
// rules: the payload under a failure header (a container build step's
// compiler error, a failed test's assertion line, a linker's missing symbol)
// must survive, not just the header naming the failure.
func TestSummarizeBuildSystemFailureDetails(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		maxLines int
		want     []string
		exclude  []string
	}{
		{
			name: "container build step error block",
			input: strings.Join([]string{
				"#1 [internal] load build definition from Dockerfile",
				"#1 transferring dockerfile: 980B done",
				"#1 DONE 0.0s",
				"#2 [1/5] FROM docker.io/library/node:24-alpine",
				"#2 CACHED",
				"#3 [2/5] WORKDIR /srv",
				"#3 CACHED",
				"#4 [3/5] COPY . .",
				"#4 DONE 0.4s",
				"#5 [4/5] RUN npm run compile",
				"#5 2.104 > orders-api@1.9.0 compile /srv",
				"#5 2.104 > tsc -p tsconfig.json",
				"#5 5.612 src/billing.ts(42,7): error TS2551: Property 'totol' does not exist on type 'Invoice'.",
				"#5 5.700 npm ERR! Exit status 2",
				`#5 ERROR: process "/bin/sh -c npm run compile" did not complete successfully: exit code: 2`,
				"------",
				" > [4/5] RUN npm run compile:",
				"5.612 src/billing.ts(42,7): error TS2551: Property 'totol' does not exist on type 'Invoice'.",
				"5.700 npm ERR! Exit status 2",
				"------",
				"Dockerfile:9",
				"--------------------",
				"   7 |     COPY . .",
				"   8 |",
				"   9 | >>> RUN npm run compile",
				"  10 |",
				"--------------------",
				`ERROR: failed to solve: process "/bin/sh -c npm run compile" did not complete successfully: exit code: 2`,
			}, "\n"),
			maxLines: 12,
			want: []string{
				"#5 [4/5] RUN npm run compile",
				"error TS2551: Property 'totol' does not exist on type 'Invoice'.",
				"npm ERR! Exit status 2",
				"Dockerfile:9",
				"ERROR: failed to solve:",
			},
			exclude: []string{"#2 CACHED", "#1 transferring dockerfile"},
		},
		{
			name: "surefire assertion detail",
			input: strings.Join([]string{
				"[INFO] Running com.acme.ledger.RoundingTest",
				"[ERROR] Tests run: 4, Failures: 1, Errors: 0, Skipped: 0, Time elapsed: 0.2 s <<< FAILURE! -- in com.acme.ledger.RoundingTest",
				"[ERROR] com.acme.ledger.RoundingTest.halfUpAtBoundary -- Time elapsed: 0.041 s <<< FAILURE!",
				"org.opentest4j.AssertionFailedError: rounding half up at the boundary ==> expected: <9> but was: <8>",
				"\tat org.junit.jupiter.api.AssertEquals.failNotEqual(AssertEquals.java:197)",
				"\tat com.acme.ledger.RoundingTest.halfUpAtBoundary(RoundingTest.java:31)",
				"[INFO] Results:",
				"[ERROR] Failures:",
				"[ERROR]   RoundingTest.halfUpAtBoundary:31 rounding half up at the boundary ==> expected: <9> but was: <8>",
				"[ERROR] Tests run: 4, Failures: 1, Errors: 0, Skipped: 0",
				"[INFO] BUILD FAILURE",
				"[ERROR] Failed to execute goal org.apache.maven.plugins:maven-surefire-plugin:3.5.0:test (default-test) on project ledger: There are test failures.",
				"[ERROR] -> [Help 1]",
				"[ERROR] Re-run Maven using the -X switch to enable full debug logging.",
				"[ERROR] For more information about the errors and possible solutions, please read the following articles:",
				"[ERROR] [Help 1] http://cwiki.apache.org/confluence/display/MAVEN/MojoFailureException",
			}, "\n"),
			maxLines: 12,
			want: []string{
				"RoundingTest.halfUpAtBoundary",
				"expected: <9> but was: <8>",
			},
			exclude: []string{"-> [Help 1]", "Re-run Maven"},
		},
		{
			name: "linker undefined symbol block",
			input: strings.Join([]string{
				"cc -Wall -O2 -c src/main.c -o build/main.o",
				"src/main.c:51:9: warning: unused variable 'attempts' [-Wunused-variable]",
				"cc -Wall -O2 -c src/token.c -o build/token.o",
				"cc build/main.o build/token.o -o bin/tokend -lcrypto",
				"Undefined symbols for architecture arm64:",
				`  "_verify_token", referenced from:`,
				"      _session_open in token.o",
				"ld: symbol(s) not found for architecture arm64",
				"clang: error: linker command failed with exit code 1 (use -v to see invocation)",
				"make: *** [Makefile:31: bin/tokend] Error 2",
			}, "\n"),
			maxLines: 12,
			want: []string{
				"Undefined symbols for architecture arm64:",
				"_verify_token",
				"make: *** [Makefile:31: bin/tokend] Error 2",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := buildfilter.SummarizeBuildSystem(tc.input, tc.maxLines)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("expected %q in build summary:\n%s", want, got)
				}
			}
			for _, exclude := range tc.exclude {
				if strings.Contains(got, exclude) {
					t.Fatalf("expected %q to be dropped from build summary:\n%s", exclude, got)
				}
			}
		})
	}
}

// TestSummarizeBuildSystemUnderContractKeepsAssertionDetails pins the JVM
// test-runner needles under the armed compression contract: every failing
// test's header and its assertion payload (the "expected/but was" line) must
// reach the display verbatim, ahead of suite counters and advisory noise,
// while the render fits the contract allowance.
func TestSummarizeBuildSystemUnderContractKeepsAssertionDetails(t *testing.T) {
	lines := []string{
		"[INFO] Scanning for projects...",
		"[INFO] Building billing-suite 1.0.0-SNAPSHOT",
		"[INFO] --- surefire:3.5.5:test (default-test) @ billing-suite ---",
	}
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("[INFO] Running com.acme.billing.Suite%02dTest", i))
	}
	lines = append(lines,
		"[ERROR] Tests run: 3, Failures: 1, Errors: 1, Skipped: 0, Time elapsed: 0.108 s <<< FAILURE! -- in com.acme.billing.CalcTest",
		"[ERROR] com.acme.billing.CalcTest.failOne -- Time elapsed: 0.050 s <<< FAILURE!",
		"org.opentest4j.AssertionFailedError: failOne: addition should equal five ==> expected: <5> but was: <4>",
		"\tat org.junit.jupiter.api.AssertEquals.failNotEqual(AssertEquals.java:197)",
		"[ERROR] Tests run: 1, Failures: 0, Errors: 1, Skipped: 0, Time elapsed: 0.006 s <<< FAILURE! -- in com.acme.billing.BoomTest",
		"[ERROR] com.acme.billing.BoomTest.boom -- Time elapsed: 0.004 s <<< ERROR!",
		"java.lang.IllegalStateException: boom: induced error",
		"[ERROR] Tests run: 4, Failures: 1, Errors: 2, Skipped: 0",
		"[INFO] BUILD FAILURE",
		"[ERROR] -> [Help 1]",
		"[ERROR] Re-run Maven using the -X switch to enable full debug logging.",
	)
	input := strings.Join(lines, "\n")

	for _, tc := range []struct {
		name    string
		needles []string
	}{
		{name: "failing test names", needles: []string{"CalcTest.failOne", "BoomTest.boom"}},
		{name: "assertion payload", needles: []string{"expected: <5> but was: <4>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := buildfilter.SummarizeBuildSystemUnderContract(input, 12, true)
			for _, needle := range tc.needles {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected needle %q in self-capped build summary:\n%s", needle, got)
				}
			}
			if strings.Contains(got, "-> [Help 1]") {
				t.Fatalf("expected advisory noise to be dropped:\n%s", got)
			}
		})
	}

	allowed := (history.EstimateTokens(input) + 4) / 5
	if allowed < 48 {
		allowed = 48
	}
	if got := history.EstimateTokens(buildfilter.SummarizeBuildSystemUnderContract(input, 12, true)); got > allowed {
		t.Fatalf("expected self-capped build summary within the contract allowance (%d), got %d tokens", allowed, got)
	}
}

func TestBuildSystemRecoveryInfo(t *testing.T) {
	input := strings.Join([]string{
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"src/app.cpp:14:2: note: candidate function not viable",
		"src/lib.cpp:20:7: error: no member named 'x' in 'Thing'",
		"ninja: build stopped: subcommand failed.",
	}, "\n")

	kind, summary, requireRawCapture := buildfilter.BuildSystemRecoveryInfo(input, 3)
	if kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected build recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
