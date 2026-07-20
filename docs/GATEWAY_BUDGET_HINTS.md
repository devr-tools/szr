# Gateway budget hints

`internal/budgethints` is the local, fail-safe boundary for future gateway
recommendations. A hint contains only a reducer profile, an optional local
fingerprint, aggregate sample count, expiry, direction, and requested output
caps. It never stores command text, paths, command output, provider payloads,
or credentials.

Gateway hints are disabled by default. To apply a stored, signed hint during a
run, explicitly enable `gateway_hints` and provide an HTTPS endpoint, the name
of an environment variable holding the bearer token, and a base64 Ed25519
public key:

```json
{
  "gateway_hints": {
    "enabled": true,
    "endpoint": "https://gateway.example/v1/budget-hints",
    "auth_token_env": "SZR_GATEWAY_TOKEN",
    "signing_public_key": "base64-ed25519-public-key"
  }
}
```

The token is read only by the explicit `szr gateway hints-refresh` command and
is never stored in szr data. `budgethints.Client.Refresh` makes the authenticated GET request,
verifies the response signature, validates expiry (at most seven days), and
atomically replaces the local store. The engine never calls the network: it
only reads the owner-only local store on a run.

The adapter rejects or ignores hints when any of these applies:

- the JSON store is unreadable or malformed;
- the schema version or fields are invalid;
- the hint is expired;
- it has fewer than 20 samples by default;
- the profile or optional exact fingerprint does not match the local run;
- its requested change is too small to matter or points in the wrong direction.

Even accepted hints cannot override local caps: defaults allow at most a 10%
tightening or 15% loosening per run. Gateway hints never receive the larger
fallback-related loosening allowance available to local history evidence. The
store is atomically replaced and owner-readable only (`0600`). Any failure is a
no-op, so a gateway issue cannot delay a command or alter its normal output.

When a gateway hint is applied, szr records a local aggregate outcome
(fallback/verifier-repair only) in `gateway-budget-hint-outcomes.jsonl`. Five
or more observations with a 20%+ harmful rate cause that exact hint to be
locally rolled back until it expires or is replaced. Outcomes contain no
command text, paths, output, or credentials.
