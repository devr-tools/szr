package engine

import "testing"

func TestClassifyCanonicalizesWrappedCommands(t *testing.T) {
	inv := Classify(Invocation{
		Command: []string{"git", "-C", "/tmp/worktree", "status", "--short"},
		Display: []string{"env", "COMPLEXITY_TARGET=/tmp/worktree", "/opt/venv/bin/python", "-m", "ruff", "check", "src"},
	})

	if inv.Classification.Command.Head != "git" || inv.Classification.Command.Subcommand != "status" {
		t.Fatalf("expected git -C command to classify as git status, got %#v", inv.Classification.Command)
	}
	if !inv.Classification.Command.Git.StatusFormatRequested {
		t.Fatalf("expected git status format detection to survive -C wrapper: %#v", inv.Classification.Command.Git)
	}

	if inv.Classification.Display.Head != "python" || inv.Classification.Display.Subcommand != "-m" {
		t.Fatalf("expected env-wrapped python tooling to classify as python -m, got %#v", inv.Classification.Display)
	}
}

func TestClassifyCanonicalizesAbsoluteGrepAndGitOptions(t *testing.T) {
	grep := Classify(Invocation{Display: []string{"/usr/bin/grep", "-rn", "FastAPI", "service/src"}})
	if grep.Classification.Display.Head != "grep" {
		t.Fatalf("expected absolute grep path to normalize to grep, got %#v", grep.Classification.Display)
	}

	diff := Classify(Invocation{Command: []string{"git", "--no-pager", "-C", "/tmp/worktree", "diff", "--stat"}})
	if diff.Classification.Command.Head != "git" || diff.Classification.Command.Subcommand != "diff" {
		t.Fatalf("expected git global options to preserve diff classification, got %#v", diff.Classification.Command)
	}
	if !diff.Classification.Command.Git.DiffFormatRequested {
		t.Fatalf("expected git diff format detection to survive global options: %#v", diff.Classification.Command.Git)
	}
}

func TestClassifyCanonicalizesNpxWrappedCommands(t *testing.T) {
	inv := Classify(Invocation{
		Display: []string{"npx", "--yes", "tsc", "--noEmit"},
		Command: []string{"npx", "-p", "typescript", "tsc", "--noEmit"},
	})

	if inv.Classification.Display.Head != "tsc" {
		t.Fatalf("expected npx-wrapped display command to normalize to tsc, got %#v", inv.Classification.Display)
	}
	if !inv.Classification.Display.JavaScript.IsWorkspaceCommand {
		t.Fatalf("expected npx-wrapped tsc to classify as js workspace, got %#v", inv.Classification.Display.JavaScript)
	}
	if inv.Classification.Command.Head != "tsc" {
		t.Fatalf("expected npx package wrapper to normalize command head to tsc, got %#v", inv.Classification.Command)
	}
}
