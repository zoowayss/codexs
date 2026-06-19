# codexs

<p align="center">
  <strong>为多身份、多端点、多会话的 Codex CLI 工作流打造的本地 Profile 启动台。</strong>
</p>

<p align="center">
  <a href="README.md">English</a>
</p>

<p align="center">
  <img alt="Go 1.24" src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white">
  <img alt="Platform" src="https://img.shields.io/badge/platform-macOS%20Apple%20Silicon-000000?logo=apple&logoColor=white">
  <img alt="Codex CLI" src="https://img.shields.io/badge/Codex-CLI-111111">
  <img alt="Status" src="https://img.shields.io/badge/status-preview-blue">
</p>

---

`codexs` 是一个面向 macOS Apple Silicon 的 Codex CLI profile 管理与启动工具。它让每个 profile 拥有独立的 `CODEX_HOME`、`auth.json`、`config.toml`、日志、会话和本地状态，适合在不同 API Key、不同 base URL、不同项目身份之间快速切换。

它的目标很简单：把多套 Codex 工作环境收束成一个干净、可审计、可复用的本地工作台。

## 为什么需要 codexs

- **Profile 级隔离**：每个 profile 都有自己的 Codex home，凭据、配置、会话互不串扰。
- **多端点切换**：同一台机器上可以维护官方 API、代理 API、团队网关等多套 base URL。
- **安全写入凭据**：支持从 stdin 或文件读取 API key，避免把密钥留在 shell history 里。
- **一键多开窗口**：在 Terminal.app 或 iTerm2 中为多个 profile 分别打开新的 Codex 会话窗口。
- **智能恢复会话**：只给 session ID，`codexs` 会扫描 profile 会话目录并用正确的 profile 恢复。
- **可复制工作环境**：从已有 profile 克隆 `config.toml`、`AGENTS.md`、`skills/` 和凭据，也可以替换新的 base URL 或 API key。
- **平台边界清晰**：平台相关行为集中在 `Platform` 接口后面，便于后续扩展 Linux、Windows 或其他 macOS 架构。

## 当前支持

| 项目 | 状态 |
| --- | --- |
| macOS Apple Silicon (`darwin/arm64`) | 支持 |
| Terminal.app / iTerm2 多窗口启动 | 支持 |
| Go 版本 | `1.24` |
| 其他平台 | 可编译，平台相关命令会返回明确的 unsupported 提示 |

## 快速开始

构建二进制：

```sh
go build -o codexs ./cmd/codexs
```

初始化本地存储：

```sh
./codexs init
```

添加第一个 profile。推荐从 stdin 读取 API key：

```sh
printf '%s' "$OPENAI_API_KEY" | ./codexs add work \
  --base-url https://api.openai.com/v1 \
  --api-key-stdin
```

在当前终端里启动 Codex：

```sh
./codexs run work -- -C /path/to/repo
```

同时打开多个 profile：

```sh
./codexs open work proxy-a -- -C /path/to/repo
```

按 session ID 恢复会话：

```sh
./codexs resume 019ea0e2-d130-7022-a3bd-e92e28e22397
```

## 命令速览

| 命令 | 说明 |
| --- | --- |
| `codexs init` | 初始化 profile 存储目录 |
| `codexs add <profile>` | 新增 profile，并写入专属 Codex 配置和凭据 |
| `codexs add <profile> --profile <source>` | 从已有 profile 克隆工作环境 |
| `codexs update <profile>` | 更新 base URL 或 API key |
| `codexs list` / `codexs ls` | 列出所有 profile |
| `codexs show <profile>` | 查看 profile 的 base URL、Codex home、配置路径和时间戳 |
| `codexs run <profile> -- <codex args...>` | 使用指定 profile 在当前终端启动 Codex |
| `codexs open <profile>... -- <codex args...>` | 在新终端窗口中启动一个或多个 profile |
| `codexs resume <sessionId>` | 自动定位 session 所属 profile 并恢复会话 |
| `codexs delete <profile> [--purge]` | 删除 profile；加 `--purge` 时同时删除文件 |
| `codexs home <profile>` | 输出该 profile 的 Codex home |
| `codexs doctor` | 检查平台、存储目录和 Codex CLI 可用性 |
| `codexs names` | 输出候选项目名 |

## 常见工作流

### 添加不同端点

```sh
printf '%s' "$OTHER_CODEX_API_KEY" | ./codexs add proxy-a \
  --base-url https://example.com/v1 \
  --api-key-stdin
```

### 从已有 profile 克隆

复用 `work` 的配置和凭据，只修改 base URL：

```sh
./codexs add prework --profile work --base-url https://new.api.com/v1
```

完整克隆 `work`，包括 API key：

```sh
./codexs add prework2 --profile work
```

克隆 profile 时会复制：

- `config.toml`
- `AGENTS.md`
- `skills/`
- `auth.json`，除非显式传入新的 API key

### 更新配置

只更新 base URL：

```sh
./codexs update work --base-url https://api.openai.com/v1
```

只更新 API key：

```sh
printf '%s' "$NEW_OPENAI_API_KEY" | ./codexs update work --api-key-stdin
```

### 运行健康检查

```sh
./codexs doctor
```

`doctor` 会检查当前平台、profile 存储位置、profile 数量、`codex` 可执行文件以及 `codex --version`。

## 存储位置

默认情况下，`codexs` 的元数据保存在：

```text
~/Library/Application Support/codexs/profiles.json
```

每个 profile 都有独立的 Codex home：

```text
~/Library/Application Support/codexs/profiles/<profile>/codex-home
```

其中包含：

- `config.toml`：当前 profile 的 Codex 配置。
- `auth.json`：当前 profile 的 API key。
- `sessions/`：Codex CLI 写入的会话记录。

如果需要自定义存储根目录，可以设置：

```sh
set -x CODEXS_HOME /path/to/codexs-home
```

如果 `codex` 不在 `PATH` 中，可以显式指定：

```sh
set -x CODEXS_CODEX_BIN /path/to/codex
```

## 隔离机制

每次执行 `codexs run <profile>` 时，`codexs` 会为子进程设置：

```text
CODEX_HOME=<profile codex-home>
```

同时，它会把同一套 provider 配置通过 `-c key=value` 参数传给 Codex CLI，确保当前 profile 的 base URL 和文件凭据模式优先于项目内的 `.codex/config.toml`。

生成的 `config.toml` 形如：

```toml
cli_auth_credentials_store = "file"
model_provider = "profile"

[model_providers.profile]
name = "profile"
wire_api = "responses"
base_url = "https://api.openai.com/v1"
requires_openai_auth = true
```

生成的 `auth.json` 形如：

```json
{
  "OPENAI_API_KEY": "..."
}
```

`codexs open` 会在新终端窗口里执行 `codexs run <profile>`，不会把 API key 拼进终端命令。

## Session 恢复

Codex session 文件通常位于 profile Codex home 下的 `sessions/` 目录。执行：

```sh
./codexs resume <sessionId>
```

`codexs` 会：

1. 校验 session ID 格式。
2. 读取本地 session cache。
3. 如果 cache 未命中，就扫描所有 profile 的 `sessions/` 目录。
4. 找到匹配项后记录 profile 映射，并用正确的 profile 调用 `codex resume <sessionId>`。

这意味着你不需要记住某个 session 最初是用哪个 profile 创建的。

## 开发

运行测试：

```sh
go test ./...
```

如果沙箱环境不能写入 Go 默认 build cache，可以指定临时缓存目录：

```sh
env GOCACHE=/private/tmp/codexs-gocache go test ./...
```

项目结构：

```text
cmd/codexs/              CLI 入口
internal/codexs/app.go   命令分发与运行流程
internal/codexs/store.go profile 状态、配置和凭据写入
internal/codexs/config.go Codex config 渲染
internal/codexs/platform_*.go 平台适配层
```

## 安全提示

- 优先使用 `--api-key-stdin` 或 `--api-key-file`。
- 将每个 profile 的 `auth.json` 当作密码文件处理。
- `codexs` 会以文件凭据模式生成 `auth.json`，并避免在 `open` 命令中暴露 API key。
- 删除 profile 时，默认只删除 profile 记录；需要清理文件时使用 `--purge`。

## 致谢

感谢 [LINUX DO](https://linux.do) 的灵感和社区支持。
