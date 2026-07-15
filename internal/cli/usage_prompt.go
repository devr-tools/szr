package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// runUsageInteractive lets a terminal user pick sessions from the numbered
// table and inspect their per-agent breakdowns. It reads line-based input
// like the settings menu; piped invocations never reach this loop.
func runUsageInteractive(report usageReport, stdin io.Reader, stdout, stderr io.Writer) int {
	reader := bufio.NewReader(stdin)
	selected := -1
	for {
		printUsagePrompt(stdout, report, selected)
		line, ok, err := readSettingsLine(reader)
		if err != nil {
			fmt.Fprintf(stderr, "szr: failed to read usage input: %v\n", err)
			return 1
		}
		if !ok {
			fmt.Fprintln(stdout, "usage: exiting")
			return 0
		}
		next, done := handleUsageChoice(report, line, selected, stdout)
		if done {
			return 0
		}
		selected = next
	}
}

func printUsagePrompt(stdout io.Writer, report usageReport, selected int) {
	if selected >= 0 {
		row := report.Sessions[selected]
		fmt.Fprintf(stdout, "session %d (%s, %d agents): y to view agents, another number to switch, q to quit\n",
			selected+1, usageSessionLabel(row), row.AgentCount)
	} else {
		fmt.Fprintf(stdout, "enter a session number (1-%d) or id prefix, q to quit\n", len(report.Sessions))
	}
	fmt.Fprint(stdout, "> ")
}

func handleUsageChoice(report usageReport, line string, selected int, stdout io.Writer) (int, bool) {
	switch strings.ToLower(line) {
	case "q", "quit", "exit":
		fmt.Fprintln(stdout, "usage: exiting")
		return selected, true
	case "":
		return selected, false
	case "y", "yes":
		showUsageSelection(report, selected, stdout)
		return selected, false
	default:
		return selectUsageSession(report, line, selected, stdout), false
	}
}

func showUsageSelection(report usageReport, selected int, stdout io.Writer) {
	if selected < 0 {
		fmt.Fprintln(stdout, "usage: pick a session number first")
		return
	}
	row := report.Sessions[selected]
	if row.AgentCount == 0 {
		fmt.Fprintf(stdout, "usage: session %s has no subagent transcripts\n", usageSessionLabel(row))
		return
	}
	renderUsageAgentTable(spreadUI{color: shouldColorizeStdout()}, row)
	fmt.Fprintln(stdout)
}

func selectUsageSession(report usageReport, line string, selected int, stdout io.Writer) int {
	if index, err := strconv.Atoi(line); err == nil {
		if index < 1 || index > len(report.Sessions) {
			fmt.Fprintf(stdout, "usage: session numbers run 1-%d\n", len(report.Sessions))
			return selected
		}
		return index - 1
	}
	return selectUsageSessionByPrefix(report, line, selected, stdout)
}

func selectUsageSessionByPrefix(report usageReport, prefix string, selected int, stdout io.Writer) int {
	var matches []int
	for i, row := range report.Sessions {
		if strings.HasPrefix(row.SessionID, prefix) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0]
	case 0:
		fmt.Fprintf(stdout, "usage: no session matches %q\n", prefix)
	default:
		fmt.Fprintf(stdout, "usage: %d sessions match %q; be more specific\n", len(matches), prefix)
	}
	return selected
}
