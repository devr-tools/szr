<p align="center">
  <img src="img/szr-gh.png" alt="szr logo" width="280" />
</p>

<p align="center">
  <a href="https://github.com/devr-tools/szr/releases">
    <img src="https://img.shields.io/github/v/release/devr-tools/szr?display_name=tag&include_prereleases" alt="release version" />
  </a>
  <a href="https://www.npmjs.com/package/@devr-tools/szr">
    <img src="https://img.shields.io/npm/v/@devr-tools/szr?logo=npm" alt="npm version" />
  </a>
  <a href="https://pypi.org/project/szr/">
    <img src="https://img.shields.io/pypi/v/szr?logo=pypi&logoColor=white" alt="PyPI version" />
  </a>
  <a href="https://github.com/devr-tools/szr/actions/workflows/ci.yml">
    <img src="https://github.com/devr-tools/szr/actions/workflows/ci.yml/badge.svg" alt="CI" />
  </a>
  <a href="https://opensource.org/licenses/Apache-2.0">
    <img src="https://img.shields.io/badge/License-Apache-green.svg" alt="License: Apache 2.0" />
  </a>
</p>

# szr

`szr` makes noisy terminal output easier to read and cheaper to send to an AI assistant. Run the commands you already use through `szr`; it keeps errors, file locations, and exit codes while trimming the noise.

It is useful whether you are working in a terminal yourself or asking an AI coding tool to help.

## 1. Install szr

Choose the option that fits your setup, then run `szr self doctor` to check that everything is ready.

<details open>
<summary><strong>Homebrew</strong> — macOS or Linux</summary>

```bash
brew install devr-tools/tap/szr
szr self doctor
```

</details>

<details>
<summary><strong>npm</strong> — prebuilt binary; no Go required</summary>

```bash
npm install -g @devr-tools/szr
szr self doctor
```

</details>

<details>
<summary><strong>pip</strong> — prebuilt binary; no Go required</summary>

```bash
pip install szr
szr self doctor
```

</details>

<details>
<summary><strong>Go</strong> — build from source</summary>

```bash
go install github.com/devr-tools/szr/cmd/szr@latest
szr self doctor
```

</details>

<details>
<summary><strong>Local checkout</strong> — for contributors</summary>

```bash
make build
./bin/szr self install
szr self doctor
```

</details>

## 2. Connect szr to your agent

Choose the coding agent you use. This lets it use szr while it works:

```bash
# Choose the assistant you use:
szr install codex
```

Other options: `szr install claude-code`, `szr install cursor`, or `szr install gemini`. Use `szr uninstall <tool>` to remove an integration. The [user guide](docs/USER_GUIDE.md#ai-tool-integrations) explains what each setup changes.

## 3. Use szr

Your agent can now use szr when it runs terminal commands. If you work directly in a terminal, add `szr` before a command yourself:

```bash
szr git status
szr git diff
szr go test ./...
szr find . --name "*.py"
```

Whether you or an assistant runs it, szr keeps the command's normal success or failure result. It only makes the output easier to scan; if it cannot safely shorten something, it shows the original output.

## 4. See what you saved

After using szr, check how much output and how many tokens it has saved:

```bash
szr spread
szr usage
```

`szr spread` shows savings from terminal output. `szr usage` compares that with the token usage recorded by supported local AI-agent sessions.

## Main commands

| Command | What it does |
| --- | --- |
| `szr <command>` | Run a command with cleaner, more compact output. |
| `szr spread` | See the token savings from recent runs. |
| `szr explain <command>` | See how szr would handle a command. |
| `szr expand <ref>` | Recover the complete original output from a saved reference. |
| `szr self doctor` | Check your installation, configuration, and updates. |
| `szr settings` | Open interactive local settings. |

## Learn more

- [User guide](docs/USER_GUIDE.md) — advanced commands, integrations, history, diagnostics, and updates
- [Profiles](docs/PROFILES.MD) — supported command families
- [User-defined filters](docs/FILTERS.md) — add your own command reducers
- [Contributing](CONTRIBUTING.md)
- [Architecture](docs/ARCHITECTURE.md)
