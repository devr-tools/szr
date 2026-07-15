package cloudlist

import (
	"fmt"
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeAWSS3Ls(input string, maxLines int) string {
	return summarizeAWSS3LsResult(input, maxLines).Text
}

func AWSS3LsRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return cloudListRecovery(summarizeAWSS3LsResult(input, maxLines), "entries")
}

// s3Listing is the parsed shape of `aws s3 ls` text output: object rows,
// PRE prefix rows, bucket rows, and the optional --summarize footer.
type s3Listing struct {
	objects   []awsSizedLine
	buckets   []string
	prefixes  int
	totalSize float64
	footer    []string
}

func summarizeAWSS3LsResult(input string, maxLines int) cloudListSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	clean := shared.StripANSI(input)
	listing := parseS3Listing(clean)
	switch {
	case len(listing.objects) > 0:
		return renderS3Objects(listing, maxLines)
	case len(listing.buckets) > 0 || listing.prefixes > 0:
		return renderS3BucketsAndPrefixes(listing, maxLines)
	default:
		return cloudListSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
}

func parseS3Listing(clean string) s3Listing {
	listing := s3Listing{}
	for _, line := range shared.NonEmptyLines(clean) {
		listing.ingestLine(strings.TrimSpace(line))
	}
	return listing
}

func (l *s3Listing) ingestLine(trimmed string) {
	fields := strings.Fields(trimmed)
	switch {
	case len(fields) == 0:
	case fields[0] == "PRE":
		l.prefixes++
	case strings.HasPrefix(trimmed, "Total Objects:"), strings.HasPrefix(trimmed, "Total Size:"):
		l.footer = append(l.footer, trimmed)
	case len(fields) >= 4 && looksLikeS3Date(fields[0], fields[1]):
		l.ingestObjectRow(fields)
	case len(fields) == 3 && looksLikeS3Date(fields[0], fields[1]):
		l.buckets = append(l.buckets, fields[2])
	}
}

func (l *s3Listing) ingestObjectRow(fields []string) {
	size, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return
	}
	key := strings.Join(fields[3:], " ")
	l.totalSize += size
	l.objects = append(l.objects, awsSizedLine{
		size: size,
		line: fmt.Sprintf("%s %s (%s)", humanBytes(size), key, fields[0]),
	})
}

func looksLikeS3Date(date, clock string) bool {
	return strings.Count(date, "-") == 2 && strings.Count(clock, ":") == 2
}

// renderS3Objects reports the object count and total size, then the largest
// entries: in listing output size is the anomaly axis, so the biggest keys
// carry the signal positional truncation would lose.
func renderS3Objects(listing s3Listing, maxLines int) cloudListSummaryResult {
	out := append([]string{s3ObjectsHeader(listing)}, listing.footer...)
	lines, omitted := largestS3Lines(listing.objects, maxLines-len(out))
	out = append(out, lines...)
	if omitted > 0 {
		out = append(out, fmt.Sprintf("... +%d smaller objects", omitted))
	}
	return cloudListSummaryResult{Text: strings.Join(out, "\n"), OmittedCount: omitted}
}

func s3ObjectsHeader(listing s3Listing) string {
	header := fmt.Sprintf("objects: %d (total %s)", len(listing.objects), humanBytes(listing.totalSize))
	if listing.prefixes > 0 {
		header += fmt.Sprintf(" prefixes: %d", listing.prefixes)
	}
	return header
}

func largestS3Lines(objects []awsSizedLine, limit int) ([]string, int) {
	if limit < 1 {
		limit = 1
	}
	entries := append([]awsSizedLine{}, objects...)
	sortBySizeDesc(entries)
	omitted := 0
	if len(entries) > limit {
		omitted = len(entries) - limit
		entries = entries[:limit]
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, shared.Clip(entry.line, 160))
	}
	return lines, omitted
}

func renderS3BucketsAndPrefixes(listing s3Listing, maxLines int) cloudListSummaryResult {
	if len(listing.buckets) == 0 {
		return cloudListSummaryResult{Text: fmt.Sprintf("prefixes: %d", listing.prefixes)}
	}
	return awsListSummary("buckets", listing.buckets, make([]string, len(listing.buckets)), maxLines)
}
