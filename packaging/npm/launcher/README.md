# szr

Token-aware CLI proxy that trims noisy command output for LLM workflows.

```bash
npm install -g @devr-tools/szr
szr --version
```

This package is a thin launcher. The actual binary is delivered through a
platform-specific optional dependency (for example `@devr-tools/szr-darwin-arm64`)
that npm selects automatically for your OS and CPU — there is no download step at
install time.

Full documentation: https://github.com/devr-tools/szr
