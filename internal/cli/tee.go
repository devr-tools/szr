package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"szr/internal/teeindex"
)

func (a *App) runTee(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "find":
			return a.runTeeFind(args[1:])
		case "prune":
			return a.runTeePrune(args[1:])
		}
	}

	var (
		asJSON     bool
		showLatest bool
		artifactID string
	)

	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		case "--latest":
			showLatest = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown tee flag %s\n", arg)
				return 2
			}
			if artifactID != "" {
				fmt.Fprintln(os.Stderr, "szr: tee accepts at most one artifact id")
				return 2
			}
			artifactID = arg
		}
	}

	store := teeindex.New(a.paths.TeeDir)
	if showLatest || artifactID != "" {
		entry, ok, err := resolveTeeArtifact(store, showLatest, artifactID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to read tee index: %v\n", err)
			return 1
		}
		if !ok {
			if showLatest {
				fmt.Fprintln(os.Stderr, "szr: no tee artifacts found")
			} else {
				fmt.Fprintf(os.Stderr, "szr: unknown tee artifact %q\n", artifactID)
			}
			return 1
		}
		if asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(entry)
			return 0
		}
		data, err := store.Read(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: tee artifact unavailable: %v\n", err)
			return 1
		}
		fmt.Print(string(data))
		return 0
	}

	entries, err := store.List(10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read tee index: %v\n", err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Println("no tee artifacts yet")
		return 0
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(entries)
		return 0
	}
	fmt.Println("tee artifacts:")
	for _, entry := range entries {
		fmt.Printf("  %s  %s  exit=%d  profile=%s  %s\n", entry.ID, entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"), entry.ExitCode, entry.Profile, entry.Command)
	}
	return 0
}

func (a *App) runTeeFind(args []string) int {
	asJSON := false
	limit := 10
	query := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: tee find requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(os.Stderr, "szr: invalid tee find limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown tee find flag %s\n", args[i])
				return 2
			}
			if query != "" {
				fmt.Fprintln(os.Stderr, "szr: tee find accepts a single query")
				return 2
			}
			query = args[i]
		}
	}
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "szr: tee find requires a query")
		return 2
	}

	entries, err := teeindex.New(a.paths.TeeDir).Search(query, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read tee index: %v\n", err)
		return 1
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(entries)
		return 0
	}
	if len(entries) == 0 {
		fmt.Println("no matching tee artifacts")
		return 0
	}
	fmt.Println("matching tee artifacts:")
	for _, entry := range entries {
		fmt.Printf("  %s  %s  exit=%d  profile=%s  %s\n", entry.ID, entry.Timestamp.Format(time.RFC3339), entry.ExitCode, entry.Profile, entry.Command)
	}
	return 0
}

func (a *App) runTeePrune(args []string) int {
	asJSON := false
	keep := 0
	days := 0
	pruneMissing := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--missing":
			pruneMissing = true
		case "--keep":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: tee prune requires a value after --keep")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 0 {
				fmt.Fprintf(os.Stderr, "szr: invalid tee prune keep value %q\n", args[i])
				return 2
			}
			keep = value
		case "--days":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: tee prune requires a value after --days")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 0 {
				fmt.Fprintf(os.Stderr, "szr: invalid tee prune days value %q\n", args[i])
				return 2
			}
			days = value
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown tee prune flag %s\n", args[i])
			return 2
		}
	}
	if !pruneMissing && keep == 0 && days == 0 {
		pruneMissing = true
	}

	store := teeindex.New(a.paths.TeeDir)
	entries, err := store.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read tee index: %v\n", err)
		return 1
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	cutoff := time.Time{}
	if days > 0 {
		cutoff = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	}

	removed := make([]teeindex.Entry, 0)
	survivors := make([]teeindex.Entry, 0, len(entries))
	for idx, entry := range entries {
		remove := false
		if pruneMissing {
			if _, statErr := os.Stat(entry.Path); errors.Is(statErr, os.ErrNotExist) {
				remove = true
			}
		}
		if !remove && !cutoff.IsZero() && entry.Timestamp.Before(cutoff) {
			remove = true
		}
		if !remove && keep > 0 && idx >= keep {
			remove = true
		}
		if remove {
			if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "szr: failed to remove tee artifact %s: %v\n", entry.Path, err)
				return 1
			}
			removed = append(removed, entry)
			continue
		}
		survivors = append(survivors, entry)
	}
	if err := store.Replace(survivors); err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to rewrite tee index: %v\n", err)
		return 1
	}

	if asJSON {
		payload := map[string]any{
			"removed":  removed,
			"kept":     len(survivors),
			"criteria": map[string]any{"missing": pruneMissing, "keep": keep, "days": days},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}
	fmt.Printf("pruned tee artifacts: removed=%d kept=%d\n", len(removed), len(survivors))
	for _, entry := range removed {
		fmt.Printf("  %s  %s\n", entry.ID, entry.Command)
	}
	return 0
}

func resolveTeeArtifact(store *teeindex.Store, latest bool, id string) (teeindex.Entry, bool, error) {
	if latest {
		return store.Latest()
	}
	return store.Find(id)
}
