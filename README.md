# codexs

`codexs` is a macOS Apple Silicon helper for running multiple isolated
Codex CLI profiles. Each profile owns its own `CODEX_HOME`, `auth.json`, and
`config.toml`, so API keys, base URLs, logs, sessions, and Codex-local state do
not bleed between profiles.

This project currently ships a working implementation only for macOS on Apple
Silicon (`darwin/arm64`). Platform behavior is behind a Go interface, so Linux,
Windows, or other macOS variants can be added as separate implementations later.
Other platforms currently compile through the unsupported implementation and
return a clear "not supported yet" error for platform-dependent commands.

## Project Name Options

The current directory and binary name are provisional. Possible names:

- Codex Lens
- Codex Profiles
- Codex Dock
- Codex Switchboard
- Codex Persona
- Codex Matrix
- Codex Harbor
- Codex Passport
- CodeNest for Codex

## Build

```bash
go build -o codexs ./cmd/codexs
```

Run tests:

```bash
go test ./...
```

If your Codex sandbox cannot write to Go's default build cache, point `GOCACHE`
at a writable temp directory:

```bash
GOCACHE=/private/tmp/codexs-gocache go test ./...
```

## Profile Storage

By default, profile metadata lives here:

```text
~/Library/Application Support/codexs/profiles.json
```

Each profile gets a separate Codex home:

```text
~/Library/Application Support/codexs/profiles/<profile>/codex-home
```

That profile home contains:

- `config.toml` with the profile-specific base URL and file credential mode.
- `auth.json` with the profile-specific API key.

Treat every profile `auth.json` like a password.

Set `CODEXS_HOME` if you want the helper data somewhere else.

## Platform Architecture

Platform-specific behavior is isolated behind `internal/codexs.Platform`.
The common CLI code does not call `runtime.GOOS`, `osascript`, or process exec
directly.

Current implementations:

- `platform_darwin_arm64.go`: real macOS Apple Silicon implementation. It
  provides the default Application Support path, Terminal/iTerm2 launch, and
  process replacement for `codexs run`.
- `platform_unsupported.go`: shared temporary implementation for every other
  target. It keeps the package buildable while returning "not supported yet"
  from platform-dependent commands.

To add another platform, create a new `platform_<os>_<arch>.go` file with a
matching build tag and implement the same interface.

## Usage

Initialize storage:

```bash
./codexs init
```

Add a profile. Reading the API key from stdin keeps it out of shell history:

```bash
printf '%s' "$OPENAI_API_KEY" | ./codexs add work \
  --base-url https://api.openai.com/v1 \
  --api-key-stdin
```

Add a second profile using another proxy or API endpoint:

```bash
printf '%s' "$OTHER_CODEX_API_KEY" | ./codexs add proxy-a \
  --base-url https://example.com/v1 \
  --api-key-stdin
```

Run Codex in the current terminal with one profile:

```bash
./codexs run work -- -C /path/to/repo
```

Open multiple profiles in separate Terminal windows:

```bash
./codexs open work proxy-a -- -C /path/to/repo
```

Use iTerm2 instead of Terminal:

```bash
./codexs open --terminal iterm2 work proxy-a -- -C /path/to/repo
```

List and inspect profiles:

```bash
./codexs list
./codexs show work
```

Update only the base URL:

```bash
./codexs update work --base-url https://api.openai.com/v1
```

Update only the API key:

```bash
printf '%s' "$NEW_OPENAI_API_KEY" | ./codexs update work --api-key-stdin
```

Run a local health check:

```bash
./codexs doctor
```

## How Isolation Works

For every profile, `codexs run` sets:

```text
CODEX_HOME=<profile codex-home>
```

It also passes equivalent `-c` overrides to the Codex CLI at launch time. This
keeps the selected profile's base URL and file credential mode above any
project-local `.codex/config.toml` layer.

The generated `config.toml` sets a profile-specific model provider:

```toml
cli_auth_credentials_store = "file"
model_provider = "profile"

[model_providers.profile]
name = "profile"
wire_api = "responses"
base_url = "https://api.openai.com/v1"
requires_openai_auth = true
```

The generated `auth.json` stores the API key using Codex's file credential
shape:

```json
{
  "OPENAI_API_KEY": "..."
}
```

`codexs open` launches a new terminal window that calls
`codexs run <profile>`. The API key is not embedded in the terminal
command.
