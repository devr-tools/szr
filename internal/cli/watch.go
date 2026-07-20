package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// runWatch streams sanitized diagnostics as JSON Lines. --once is useful for
// scripts that want a snapshot without keeping a process alive.
//
//nolint:gocognit,maintidx // The polling lifecycle is intentionally kept together.
func (a *App) runWatch(ctx context.Context, args []string) int {
	once, err := parseWatchOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 2
	}
	if a.events == nil {
		return 0
	}

	encoder := json.NewEncoder(os.Stdout)
	seen := 0
	if err := a.emitWatchEvents(encoder, &seen); err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read diagnostics events: %v\n", err)
		return 1
	}
	if once {
		return 0
	}

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
			if err := a.emitWatchEvents(encoder, &seen); err != nil {
				fmt.Fprintf(os.Stderr, "szr: failed to read diagnostics events: %v\n", err)
				return 1
			}
		}
	}
}

func parseWatchOptions(args []string) (once bool, err error) {
	for _, arg := range args {
		switch arg {
		case "--jsonl":
			// JSON Lines is the only stable watch format in the first release.
		case "--once":
			once = true
		default:
			return false, fmt.Errorf("unknown watch flag %s", arg)
		}
	}
	return once, nil
}

func (a *App) emitWatchEvents(encoder *json.Encoder, seen *int) error {
	events, err := a.events.ReadAll()
	if err != nil {
		return err
	}
	if *seen > len(events) {
		*seen = 0
	}
	for _, event := range events[*seen:] {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	*seen = len(events)
	return nil
}
