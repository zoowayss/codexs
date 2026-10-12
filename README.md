# codexs

<p align="center">
  <strong>A profile launchpad for isolated Codex CLI workflows.</strong>
</p>

<p align="center">
  <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <img alt="Go 1.24" src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white">
  <img alt="Platform" src="https://img.shields.io/badge/platform-macOS%20Apple%20Silicon-000000?logo=apple&logoColor=white">
  <img alt="Codex CLI" src="https://img.shields.io/badge/Codex-CLI-111111">
  <img alt="Status" src="https://img.shields.io/badge/status-preview-blue">
</p>

---

`codexs` is a Codex CLI profile manager and launcher for macOS Apple Silicon. It gives every profile its own `CODEX_HOME`, `auth.json`, `config.toml`, logs, sessions, and local state, making it practical to switch between API keys, base URLs, and project identities without cross-contamination.

The goal is straightforward: turn multiple Codex environments into one clean, auditable, reusable local control plane.

## Why codexs

- **Profile-level isolation**: every profile owns a separate Codex home, keeping credentials, config, and sessions apart.
- **Multi-endpoint switching**: maintain official APIs, proxy APIs, and team gateways on one machine.
- **Safer credential input**: read API keys from stdin or a file instead of leaving them in shell history.
- **Multi-window launch**: open multiple profiles in separate Terminal.app or iTerm2 windows.
- **Session-aware resume**: provide only a session ID and let `codexs` find the profile that owns it.
- **Reusable workspaces**: clone `config.toml`, `AGENTS.md`, `skills/`, and credentials from an existing profile, while optionally replacing the base URL or API key.
- **Clear platform boundary**: platform behavior lives behind the `Platform` interface, leaving room for future Linux, Windows, or other macOS implementations.

## Support matrix

| Item | Status |
| --- | --- |
| macOS Apple Silicon (`darwin/arm64`) | Supported |
| Terminal.app / iTerm2 multi-window launch | Supported |
| Go version | `1.24` |
| Other platforms | Buildable, with platform-dependent commands returning an explicit unsupported error |

## Quick start

Build the binary:

```sh
go build -o codexs ./cmd/codexs
```

Initialize local storage:

```sh
./codexs init
```

Add the first profile. Reading the API key from stdin is recommended:

```sh
printf '%s' "$OPENAI_API_KEY" | ./codexs add work \
  --base-url https://api.openai.com/v1 \
  --api-key-stdin
```

Start Codex in the current terminal:

```sh
./codexs run work -- -C /path/to/repo
```

Open multiple profiles:

```sh
./codexs open work proxy-a -- -C /path/to/repo
```

Resume by session ID:

```sh
./codexs resume 019ea0e2-d130-7022-a3bd-e92e28e22397
```

## Command reference

| Command | Description |
| --- | --- |
| `codexs init` | Initialize the profile store |
| `codexs add <profile>` | Create a profile with dedicated Codex config and credentials |
| `codexs add <profile> --profile <source>` | Clone from an existing profile |
| `codexs update <profile>` | Update the base URL or API key |
| `codexs list` / `codexs ls` | List profiles |
| `codexs show <profile>` | Show base URL, Codex home, config paths, and timestamps |
| `codexs run <profile> -- <codex args...>` | Launch Codex with one profile in the current terminal |
| `codexs open <profile>... -- <codex args...>` | Launch one or more profiles in new terminal windows |
| `codexs resume <sessionId>` | Find the profile for a session and resume it |
| `codexs stop` | Stop background servers for all profiles in the current store; keep the default `~/.codex` server |
| `codexs delete <profile> [--purge]` | Delete a profile; `--purge` also removes files |
| `codexs home <profile>` | Print a profile's Codex home |
| `codexs doctor` | Check platform, store, and Codex CLI availability |
| `codexs names` | Print candidate project names |

## Common workflows

### Add another endpoint

```sh
printf '%s' "$OTHER_CODEX_API_KEY" | ./codexs add proxy-a \
  --base-url https://example.com/v1 \
  --api-key-stdin
```

### Clone an existing profile

Reuse `work` credentials and config while changing the base URL:

```sh
./codexs add prework --profile work --base-url https://new.api.com/v1
```

Clone `work` exactly, including the API key:

```sh
./codexs add prework2 --profile work
```

Profile cloning copies:

- `config.toml`
- `AGENTS.md`
- `skills/`
- `auth.json`, unless a new API key is provided

### Update configuration

Update only the base URL:

```sh
./codexs update work --base-url https://api.openai.com/v1
```

Update only the API key:

```sh
printf '%s' "$NEW_OPENAI_API_KEY" | ./codexs update work --api-key-stdin
```

### Run diagnostics

```sh
./codexs doctor
```

`doctor` checks the current platform, profile store, profile count, `codex` binary, and `codex --version`.

### Reload credentials after editing auth.json

After changing a profile's `auth.json`, stop the profile background servers before launching again:

```sh
./codexs stop
./codexs run work
```

`stop` calls `codex app-server daemon stop` with each registered profile's `CODEX_HOME`. It targets all profiles in the current store (including a custom `CODEXS_HOME`), leaving the default `~/.codex` server and other stores alone. Connected profile tasks are interrupted; configuration, credentials, and saved history are retained. Profiles with no running server are also handled by Codex's stop command.

The next normal `run` can start a fresh background server and load the profile's current file-based credentials. `run` does not add `--no-daemon`. Stop after editing the file, and avoid starting another client for that profile between cleanup and your next launch. This clears background authentication state; if you explicitly configured a different credential source, that source still applies.

This command requires a Codex CLI version that supports `app-server daemon stop`. It reports failures per profile, attempts the remaining profiles, and exits nonzero if any stop command fails. It does not delete credentials or kill processes by name.

## Storage layout

By default, profile metadata is stored at:

```text
~/.codexs/profiles.json
```

Each profile gets an isolated Codex home:

```text
~/.codexs/profiles/<profile>
```

That home contains:

- `config.toml`: Codex configuration for the profile.
- `auth.json`: API key for the profile.
- `sessions/`: session records written by Codex CLI.

To customize the store root:

```sh
set -x CODEXS_HOME /path/to/codexs-home
```

To point at a specific Codex binary:

```sh
set -x CODEXS_CODEX_BIN /path/to/codex
```

## How isolation works

For every `codexs run <profile>` invocation, `codexs` sets:

```text
CODEX_HOME=~/.codexs/profiles/<profile>
```

Codex then reads the selected profile's `config.toml` and `auth.json` from that directory. Edit the profile's `config.toml` directly when you need to customize its provider settings; `codexs` does not add command-line `-c` overrides at launch time.

The generated `config.toml` looks like this:

```toml
cli_auth_credentials_store = "file"
model_provider = "profile"

[model_providers.profile]
name = "profile"
wire_api = "responses"
base_url = "https://api.openai.com/v1"
requires_openai_auth = true
```

The generated `auth.json` looks like this:

```json
{
  "OPENAI_API_KEY": "..."
}
```

`codexs open` launches a new terminal window that calls `codexs run <profile>`. The API key is not embedded in the terminal command.

## Session resume

Codex session files usually live under the profile Codex home's `sessions/` directory. When you run:

```sh
./codexs resume <sessionId>
```

`codexs` will:

1. Validate the session ID format.
2. Read the local session cache.
3. Scan every profile's `sessions/` directory on cache miss.
4. Record the discovered mapping and invoke `codex resume <sessionId>` with the right profile.

You do not need to remember which profile originally created a session.

## Development

Run tests:

```sh
go test ./...
```

If a sandbox cannot write to Go's default build cache, point `GOCACHE` at a writable temporary directory:

```sh
env GOCACHE=/private/tmp/codexs-gocache go test ./...
```

Project layout:

```text
cmd/codexs/              CLI entrypoint
internal/codexs/app.go   command dispatch and runtime flow
internal/codexs/store.go profile state, config, and credential writes
internal/codexs/config.go Codex config rendering
internal/codexs/platform_*.go platform adapters
```

## Security notes

- Prefer `--api-key-stdin` or `--api-key-file`.
- Treat every profile's `auth.json` as a secret file.
- `codexs` generates file-based credentials and avoids exposing API keys in `open` commands.
- Deleting a profile removes the profile record by default; use `--purge` when you also want to remove its files.

## Acknowledgments

Thanks to [LINUX DO](https://linux.do) for inspiration and community support.
