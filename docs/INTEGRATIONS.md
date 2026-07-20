# Integrations

szr exposes a small local integration surface for agents, editors, and terminal
dashboards. The default path is local-only: integrations can consume metrics
without receiving commands, paths, command output, tee artifacts, or agent
transcript content.

## Live diagnostics

Start the JSON Lines stream in the same user account that runs szr:

```sh
szr watch --jsonl
```

Use `--once` to read the events currently persisted and exit:

```sh
szr watch --jsonl --once
```

Each line is a standalone JSON object. Consumers should decode lines
independently, ignore fields they do not recognize, and only rely on fields
documented for the event `version`. Version `1` has two event types:

| Type | When emitted | Fields to use |
| --- | --- | --- |
| `run_progress` | A streaming reducer has parsed additional input. | `run_id`, `timestamp`, `profile`, `profile_confidence`, `bytes_parsed` |
| `run_final` | A wrapped command completes. | `run_id`, `timestamp`, `profile`, token estimates, `duration_ms`, `exit_class`, and reduction flags |

`run_id` is an opaque per-run value. It is useful only for joining progress
with the matching final event; do not treat it as a user, workspace, session,
or command identifier.

Example final event:

```json
{"version":1,"type":"run_final","run_id":"…","timestamp":"2026-07-20T16:00:00Z","profile":"go-test-json","raw_tokens_est":4200,"emitted_tokens_est":180,"saved_tokens_est":4020,"duration_ms":320,"exit_class":"success"}
```

For a compact live display, render final events as:

```text
go-test-json  success  180 emitted / 4,020 saved tokens  320ms
```

Treat missing optional fields as unavailable rather than zero. In particular,
progress events intentionally do not contain final token estimates or an exit
class.

### Minimal consumer

This standard-library Python example only renders completed runs and remains
forward-compatible by reading the fields it needs:

```python
import json
import subprocess

watch = subprocess.Popen(
    ["szr", "watch", "--jsonl"], stdout=subprocess.PIPE, text=True
)
for line in watch.stdout:
    event = json.loads(line)
    if event.get("version") != 1 or event.get("type") != "run_final":
        continue
    print(
        f"{event.get('profile', 'unknown')}  {event.get('exit_class', 'unknown')}  "
        f"{event.get('emitted_tokens_est', '?')} emitted / "
        f"{event.get('saved_tokens_est', '?')} saved tokens  "
        f"{event.get('duration_ms', '?')}ms"
    )
```

The stream replays events already stored when it starts. Consumers that require
exactly-once display should retain the final `run_id` values they have shown.
If the event file is replaced while watching, szr may replay the current file;
deduplicating by `run_id` handles this safely.

## Routing decisions

For integrations that need to decide whether a shell command should be routed
through szr, use the existing read-only routing interface:

```sh
szr rewrite --json --command 'go test ./...'
```

Keep the result as guidance rather than executing a rewritten command without
user approval. This avoids duplicating szr's shell, pipeline, and command
classification policy in each integration.

## Diagnostics storage and export

`szr diagnostics status --json` reports the local event and export-outbox
state without making a network request. Diagnostics export is disabled by
default and must be explicitly configured. When enabled, exported payloads use
the same allowlisted event schema as the local stream.

See [GATEWAY_BUDGET_HINTS.md](GATEWAY_BUDGET_HINTS.md) for the separate,
fail-safe local boundary for gateway budget recommendations.
