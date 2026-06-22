package workflows

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func RunRecommend(rt Runtime, args []string) int {
	asJSON := false
	limit := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: recommend requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(rt.Stderr, "szr: invalid recommend limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown recommend flag %s\n", args[i])
			return 2
		}
	}

	records, err := rt.History.LoadAll()
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	recommendations := BuildRecommendations(records, limit)
	if asJSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(recommendations)
		return 0
	}
	if len(recommendations) == 0 {
		fmt.Fprintln(rt.Stdout, "no recommendations yet")
		return 0
	}

	fmt.Fprintln(rt.Stdout, "recommendations:")
	for _, item := range recommendations {
		fmt.Fprintf(rt.Stdout, "  - [%s] %s\n", item.Kind, item.Command)
		fmt.Fprintf(rt.Stdout, "    reason: %s\n", item.Reason)
		fmt.Fprintf(rt.Stdout, "    action: %s\n", item.Action)
		if item.Profile != "" || item.Samples > 0 || item.Confidence != "" {
			fmt.Fprintf(rt.Stdout, "    profile=%s samples=%d confidence=%s\n", item.Profile, item.Samples, emptyDash(item.Confidence))
		}
		if item.Direction != "" {
			fmt.Fprintf(
				rt.Stdout,
				"    target: %s lines=%d bytes=%d tokens=%d\n",
				item.Direction,
				item.Suggested.MaxLines,
				item.Suggested.MaxBytes,
				item.Suggested.MaxTokens,
			)
		}
	}
	return 0
}

func RunHotspots(rt Runtime, args []string) int {
	asJSON := false
	limit := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: hotspots requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(rt.Stderr, "szr: invalid hotspots limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown hotspots flag %s\n", args[i])
			return 2
		}
	}

	records, err := rt.History.LoadAll()
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	hotspots := BuildHotspots(records, limit)
	if asJSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(hotspots)
		return 0
	}
	if len(hotspots) == 0 {
		fmt.Fprintln(rt.Stdout, "no hotspots yet")
		return 0
	}

	fmt.Fprintln(rt.Stdout, "hotspots:")
	for _, item := range hotspots {
		fmt.Fprintf(
			rt.Stdout,
			"  - %s  profile=%s samples=%d avg=%.1f%% fallback=%.1f%% fail=%.1f%% tee=%.1f%% p50/p95=%d/%dms signals=%s score=%d\n",
			item.Command,
			item.Profile,
			item.Samples,
			item.AveragePct,
			item.FallbackRate,
			item.FailureRate,
			item.TeeRate,
			item.DurationP50MS,
			item.DurationP95MS,
			hotspotSignalList(item),
			item.CoverageScore,
		)
	}
	return 0
}
