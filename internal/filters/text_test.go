package filters

import (
	"strings"
	"testing"
)

func TestSummarizeGitStatus(t *testing.T) {
	input := `## main...origin/main
M  README.md
MM internal/cli/app.go
?? docs/ARCHITECTURE.md
`

	got := SummarizeGitStatus(input)
	for _, want := range []string{
		"main...origin/main",
		"staged=2 unstaged=1 untracked=1",
		"README.md",
		"docs/ARCHITECTURE.md",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestSummarizeGoTestJSON(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"run","Package":"example.com/app","Test":"TestHappyPath"}`,
		`{"Action":"pass","Package":"example.com/app","Test":"TestHappyPath"}`,
		`{"Action":"run","Package":"example.com/app","Test":"TestSadPath"}`,
		`{"Action":"output","Package":"example.com/app","Test":"TestSadPath","Output":"panic: boom"}`,
		`{"Action":"fail","Package":"example.com/app","Test":"TestSadPath"}`,
		`{"Action":"fail","Package":"example.com/app"}`,
	}, "\n")

	got := SummarizeGoTestJSON(input)
	for _, want := range []string{
		"packages: pass=0 fail=1",
		"example.com/app",
		"TestSadPath",
		"panic: boom",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestGroupRipgrep(t *testing.T) {
	input := strings.Join([]string{
		"internal/cli/app.go:12:func New(version string) *App {",
		"internal/cli/app.go:45:fmt.Println(\"szr\")",
		"README.md:8:szr gain --history",
	}, "\n")

	got := GroupRipgrep(input, 8)
	for _, want := range []string{
		"internal/cli/app.go (2 matches)",
		"12: func New(version string) *App {",
		"README.md (1 matches)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}
