package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/devr-tools/szr/internal/dedup"
)

// runExpand resolves a session-dedup reference and prints the stored raw
// output byte-exact to stdout.
func (a *App) runExpand(args []string) int {
	last, ref, code := parseExpandArgs(args)
	if code != 0 {
		return code
	}
	store := dedup.New(a.paths.DataDir)
	entry, ok, err := resolveExpandEntry(store, last, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read dedup index: %v\n", err)
		return 1
	}
	if !ok {
		printExpandMissing(last, ref)
		return 1
	}
	return printExpandArtifact(store, entry)
}

func parseExpandArgs(args []string) (bool, string, int) {
	last, ref, code := scanExpandArgs(args)
	if code != 0 {
		return false, "", code
	}
	return validateExpandArgs(last, ref)
}

func scanExpandArgs(args []string) (bool, string, int) {
	last := false
	ref := ""
	for _, arg := range args {
		switch {
		case arg == "--last":
			last = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "szr: unknown expand flag %s\n", arg)
			return false, "", 2
		case ref != "":
			fmt.Fprintln(os.Stderr, "szr: expand accepts at most one reference")
			return false, "", 2
		default:
			ref = arg
		}
	}
	return last, ref, 0
}

func validateExpandArgs(last bool, ref string) (bool, string, int) {
	if last {
		return true, ref, 0
	}
	if ref == "" {
		fmt.Fprintln(os.Stderr, "szr: expand requires a reference (or --last)")
		return false, "", 2
	}
	if len(strings.TrimSpace(ref)) < dedup.MinRefLength {
		fmt.Fprintf(os.Stderr, "szr: expand reference %q is too short (need at least %d characters)\n", ref, dedup.MinRefLength)
		return false, "", 2
	}
	return false, ref, 0
}

func resolveExpandEntry(store *dedup.Store, last bool, ref string) (dedup.Entry, bool, error) {
	if last {
		return store.Latest()
	}
	return store.FindRef(ref)
}

func printExpandMissing(last bool, ref string) {
	if last {
		fmt.Fprintln(os.Stderr, "szr: no dedup references recorded yet")
		return
	}
	fmt.Fprintf(os.Stderr, "szr: unknown or expired ref %q (references age out with the session dedup window)\n", ref)
}

func printExpandArtifact(store *dedup.Store, entry dedup.Entry) int {
	data, err := store.ReadArtifact(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: ref %s expired: stored output is no longer available (%v)\n", entry.Ref(), err)
		return 1
	}
	if dedup.HashBytes(data) != entry.ArtifactHash {
		fmt.Fprintf(os.Stderr, "szr: ref %s expired: stored output failed integrity verification\n", entry.Ref())
		return 1
	}
	_, _ = os.Stdout.Write(data)
	if entry.Truncated {
		fmt.Fprintf(os.Stderr, "szr: stored output was truncated at %d bytes (raw run produced %d bytes)\n", len(data), entry.RawBytes)
	}
	return 0
}
