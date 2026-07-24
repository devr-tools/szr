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

## Install

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

## Get started

Put `szr` in front of a command you already run:

```bash
szr git status
szr git diff
szr go test ./...
szr find . --name "*.py"
```

`szr` returns the same exit code as the command it runs. If a shorter result would lose important information, it shows the original output instead.

## Main commands

| Command | What it does |
| --- | --- |
| `szr <command>` | Run a command with cleaner, more compact output. |
| `szr spread` | See the token savings from recent runs. |
| `szr explain <command>` | See how szr would handle a command. |
| `szr expand <ref>` | Recover the complete original output from a saved reference. |
| `szr self doctor` | Check your installation, configuration, and updates. |
| `szr settings` | Open interactive local settings. |

## Works well with AI tools

Set up your preferred assistant with one command:

```bash
szr install codex
szr install claude-code
szr install cursor
szr install gemini
```

Use `szr uninstall <tool>` to remove an integration. See the [user guide](docs/USER_GUIDE.md#ai-tool-integrations) for what each integration changes.

## Learn more

- [User guide](docs/USER_GUIDE.md) — advanced commands, integrations, history, diagnostics, and updates
- [Profiles](docs/PROFILES.MD) — supported command families
- [User-defined filters](docs/FILTERS.md) — add your own command reducers
- [Contributing](CONTRIBUTING.md)
- [Architecture](docs/ARCHITECTURE.md)
