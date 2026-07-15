# User-Defined Filters

`szr` can load your own declarative filters as additional profiles. A filter is a single JSON spec file that pairs a `match` section (which commands it routes) with the same line-oriented reducer keys the declarative builtins use. No Go code, no rebuild.

## Where specs load from

| Scope | Directory | Enabled |
| --- | --- | --- |
| Global | `filters/` inside szr's config directory (next to `config.json`) | always |
| Project | `.szr/filters/` in the current working directory | opt-in |

Project filters are trust-gated and **off by default**: enable them via `szr settings` (option `22`, `project filters`) or `advanced.project_filters` in `config.json`. While disabled, szr warns when `.szr/filters/` contains spec files instead of silently ignoring them.

Only `*.json` files are considered, loaded in filename order.

## Precedence

Profile sources register in this order:

1. project rules (`.szr` project profiles)
2. builtin profiles
3. global user filters
4. project filters

A spec whose `name` collides with a profile from an earlier source is skipped with a warning — user filters never silently shadow builtins or each other. Invalid files (bad JSON, bad regex, failed validation) are also skipped with warnings; the remaining specs still load.

`szr profiles` lists loaded filters with `source: user` or `source: project`, and `szr explain <cmd...>` shows the spec file a matched filter was loaded from.

## Spec format

```text
{
  "name":              string   optional; defaults to the filename without extension
  "description":       string   optional; shown by szr profiles
  "match": {                    required; at least one positive rule
    "command_prefix":  [string] token-by-token prefix of the executed command
    "display_prefix":  [string] token-by-token prefix of the displayed command
    "any_args":        [string] at least one of these tokens appears in the args
    "exclude_args":    [string] none of these tokens may appear
  },
  "keep_patterns":     [string] keep only lines matching any pattern (Go regexp)
  "strip_patterns":    [string] drop lines matching any pattern (Go regexp)
  "head":              int      keep the first N lines after filtering
  "tail":              int      keep the last N lines after filtering
  "max_line_width":    int      clip lines to N runes
  "drop_empty":        bool     drop blank lines
  "dedup_consecutive": bool     fold runs of identical lines into "line (xN)"
  "fold_similar":      bool     also fold lines identical after normalizing timestamps/counters
  "empty_message":     string   rendered when filtering leaves nothing
}
```

Stages run in a fixed order: `drop_empty` → `keep_patterns`/`strip_patterns` → dedup/fold → `max_line_width` clipping → `head`/`tail` truncation with omission markers. Setting both `head` and `tail` keeps the first `head` and last `tail` lines with a `... +N more lines` marker in between. `match` requires at least one of `command_prefix`, `display_prefix`, or `any_args`.

Whatever the filter omits stays recoverable through the usual artifact and omission-marker machinery — user filters do not weaken the recovery contract.

## Worked example

Suppose a deploy tool `flowctl` prints hundreds of progress lines and you only care about warnings, errors, and the final status. Save this as `flowctl-deploy.json` in your global `filters/` directory (or `.szr/filters/` for one project):

```json
{
  "description": "flowctl deploy: keep warnings, errors, and the final status line.",
  "match": {
    "command_prefix": ["flowctl", "deploy"],
    "exclude_args": ["--dry-run"]
  },
  "keep_patterns": [
    "(?i)\\b(warn|error|fail)",
    "^deploy (succeeded|failed)"
  ],
  "strip_patterns": ["^\\s*progress:"],
  "drop_empty": true,
  "dedup_consecutive": true,
  "max_line_width": 200,
  "head": 5,
  "tail": 15,
  "empty_message": "flowctl deploy: no warnings or errors"
}
```

Then:

```bash
szr flowctl deploy --env staging
```

renders only WARN/ERROR lines (repeated ones folded into counts) plus the final status line, and a clean run collapses to the `empty_message`. `szr explain flowctl deploy` confirms the routing and the spec file in use.

Prefer a Go builtin profile when the tool exposes a structured output mode — declarative filters are intentionally limited to line-oriented keep/strip/truncate logic. See [profile-families/fallback-reducers.md](profile-families/fallback-reducers.md) for where that boundary sits.
