package container

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var imageColumnPattern = regexp.MustCompile(`\t+| {2,}`)

type dockerImage struct {
	repository string
	tag        string
	size       string
	created    string
}

func SummarizeDockerImages(input string, maxLines int) string {
	return summarizeDockerImagesResult(input, maxLines).Text
}

func DockerImagesRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDockerImagesResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional images", result.OmittedCount))
}

func summarizeDockerImagesResult(input string, maxLines int) dockerPSSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	clean := shared.StripANSI(input)
	images := parseDockerImages(clean)
	if len(images) == 0 {
		return dockerPSSummaryResult{Text: shared.CompactLines(strings.TrimSpace(clean), maxLines)}
	}
	named, dangling := splitDanglingImages(images)
	out := []string{fmt.Sprintf("images: %d (total %s)", len(images), formatImageSize(sumImageSizes(images)))}
	if len(dangling) > 0 {
		out = append(out, fmt.Sprintf("dangling <none>: %d (%s)", len(dangling), formatImageSize(sumImageSizes(dangling))))
	}
	for _, image := range named {
		out = append(out, formatDockerImage(image))
	}
	result := dockerPSSummaryResult{Text: shared.JoinLimitedLines(out, maxLines)}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
}

func parseDockerImages(input string) []dockerImage {
	out := []dockerImage{}
	for _, line := range shared.NonEmptyLines(input) {
		fields := splitImageColumns(line)
		if len(fields) < 4 || fields[0] == "REPOSITORY" {
			continue
		}
		out = append(out, imageFromFields(fields))
	}
	return out
}

func splitImageColumns(line string) []string {
	fields := imageColumnPattern.Split(strings.TrimSpace(line), -1)
	out := fields[:0]
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

// imageFromFields accepts both the prepared tab format (repo, tag, size,
// created) and the default table (repo, tag, id, created, size).
func imageFromFields(fields []string) dockerImage {
	if len(fields) >= 5 {
		return dockerImage{repository: fields[0], tag: fields[1], created: fields[3], size: fields[4]}
	}
	return dockerImage{repository: fields[0], tag: fields[1], size: fields[2], created: fields[3]}
}

func splitDanglingImages(images []dockerImage) ([]dockerImage, []dockerImage) {
	named := []dockerImage{}
	dangling := []dockerImage{}
	for _, image := range images {
		if image.repository == "<none>" {
			dangling = append(dangling, image)
			continue
		}
		named = append(named, image)
	}
	return named, dangling
}

func formatDockerImage(image dockerImage) string {
	name := image.repository
	if image.tag != "" && image.tag != "<none>" {
		name += ":" + image.tag
	}
	line := name + " " + image.size
	if age := shared.CompactRelativeAge(image.created); age != "" {
		line += " (" + age + ")"
	}
	return shared.Clip(line, 160)
}

func sumImageSizes(images []dockerImage) float64 {
	total := 0.0
	for _, image := range images {
		total += parseImageSize(image.size)
	}
	return total
}

func parseImageSize(size string) float64 {
	trimmed := strings.TrimSpace(size)
	idx := strings.IndexFunc(trimmed, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	if idx <= 0 {
		return 0
	}
	value, err := strconv.ParseFloat(trimmed[:idx], 64)
	if err != nil {
		return 0
	}
	return value * imageSizeUnitFactor(strings.ToUpper(strings.TrimSpace(trimmed[idx:])))
}

func imageSizeUnitFactor(unit string) float64 {
	switch unit {
	case "KB":
		return 1e3
	case "MB":
		return 1e6
	case "GB":
		return 1e9
	case "TB":
		return 1e12
	default:
		return 1
	}
}

func formatImageSize(bytes float64) string {
	switch {
	case bytes >= 1e9:
		return fmt.Sprintf("%.1fGB", bytes/1e9)
	case bytes >= 1e6:
		return fmt.Sprintf("%.0fMB", bytes/1e6)
	case bytes >= 1e3:
		return fmt.Sprintf("%.1fkB", bytes/1e3)
	default:
		return fmt.Sprintf("%.0fB", bytes)
	}
}
