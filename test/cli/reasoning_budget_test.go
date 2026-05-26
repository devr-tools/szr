package cli_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/szr/test/testutil"
)

func TestExplainReasoningBudgetMode(t *testing.T) {
	app := testutil.NewTestApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "explain", "go", "build")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected standard explain stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	standardLines := extractBudgetLines(t, stdout)
	if !strings.Contains(stdout, "reasoning budget mode: standard") {
		t.Fatalf("expected standard explain mode in %q", stdout)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "--reasoning-budget", "agent", "explain", "go", "build")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected agent explain stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	agentLines := extractBudgetLines(t, stdout)
	if !strings.Contains(stdout, "reasoning budget mode: agent") {
		t.Fatalf("expected agent explain mode in %q", stdout)
	}
	if agentLines >= standardLines {
		t.Fatalf("expected agent reasoning budget to tighten output, standard=%d agent=%d stdout=%q", standardLines, agentLines, stdout)
	}
	if !strings.Contains(stdout, "contract: failures=1 anchors=1 hints=1") {
		t.Fatalf("expected agent explain contract in %q", stdout)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "--reasoning-budget", "aggressive", "explain", "go", "build")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected aggressive explain stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	aggressiveLines := extractBudgetLines(t, stdout)
	if !strings.Contains(stdout, "reasoning budget mode: aggressive") {
		t.Fatalf("expected aggressive explain mode in %q", stdout)
	}
	if aggressiveLines >= agentLines {
		t.Fatalf("expected aggressive reasoning budget to tighten beyond agent, agent=%d aggressive=%d stdout=%q", agentLines, aggressiveLines, stdout)
	}
}

func TestReasoningBudgetFlagErrors(t *testing.T) {
	app := testutil.NewTestApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "--reasoning-budget", "invalid", "help")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "invalid reasoning budget mode") {
		t.Fatalf("unexpected invalid reasoning-budget output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func extractBudgetLines(t *testing.T, output string) int {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "budget: lines=") {
			continue
		}
		rest := strings.TrimPrefix(line, "budget: lines=")
		value, _, _ := strings.Cut(rest, " ")
		lines, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("parse budget line %q: %v", line, err)
		}
		return lines
	}
	t.Fatalf("missing budget line in %q", output)
	return 0
}
