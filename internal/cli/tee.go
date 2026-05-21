package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"szr/internal/teeindex"
)

func (a *App) runTee(args []string) int {
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

func resolveTeeArtifact(store *teeindex.Store, latest bool, id string) (teeindex.Entry, bool, error) {
	if latest {
		return store.Latest()
	}
	return store.Find(id)
}
