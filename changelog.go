// Package szr exposes repository-level assets that ship inside the szr
// binaries.
package szr

import _ "embed"

// Changelog is the release-please managed changelog. The CLI menu parses its
// newest section into the "What's New" banner, so the banner stays current
// with each release without manual edits.
//
//go:embed CHANGELOG.md
var Changelog string
