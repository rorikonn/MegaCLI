# MegaCLI

> 本项目基于 [Crush](https://github.com/charmbracelet/crush)（由 [Charm](https://charm.land) 开发的终端 AI 编程助手）二次开发而来，仅供内部使用，不对外提供商业服务。
> 原项目遵循 [FSL-1.1-MIT](./LICENSE.md) 协议，本项目保留原始版权声明与许可条款。

终端里的 AI 编程助手，无缝接入你的工具、代码与工作流，全面兼容主流 LLM 模型。

## Features

- **Multi-Model:** 支持 30+ 种 LLM Provider（Anthropic、OpenAI、Gemini、xAI、Bedrock、Copilot 等），也可通过 OpenAI / Anthropic 兼容 API 接入自定义模型
- **Flexible:** 会话中途切换模型，上下文不丢失
- **Multi-Agent:** 内置 Coder、Planner 等 Agent，支持自定义 Agent 和子 Agent
- **Session-Based:** 每个项目支持多个独立工作会话与上下文
- **LSP-Enhanced:** 利用 LSP 提供代码智能上下文，支持自动发现 LSP 服务
- **Extensible:** 通过 MCP（`http`、`stdio`、`sse`）和 Agent Skills 扩展能力
- **Hooks:** 通过用户自定义 shell 脚本在工具调用前拦截、改写或审批，兼容 Claude Code hooks
- **Works Everywhere:** 全面支持 macOS、Linux、Windows（PowerShell 和 WSL）、Android、FreeBSD、OpenBSD 和 NetBSD

## Installation

**Windows:**

```powershell
irm https://raw.githubusercontent.com/rorikonn/MegaCLI/master/scripts/install.ps1 | iex
```

**macOS / Linux:**

```bash
curl -sSf https://raw.githubusercontent.com/rorikonn/MegaCLI/master/scripts/install.sh | sh
```

或从源码构建：

```bash
go install github.com/megacli/megacli@latest
```

也可通过 `megacli --update` 自动更新到最新版本。

## Getting Started

获取你偏好的模型提供商的 API Key，然后运行 `megacli`。首次运行时会引导你完成 API Key 设置和模型选择。

也可以通过环境变量配置 Provider：

| Environment Variable        | Provider                                           |
| --------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`         | Anthropic                                          |
| `OPENAI_API_KEY`            | OpenAI                                             |
| `GEMINI_API_KEY`            | Google Gemini                                      |
| `XAI_API_KEY`               | xAI (Grok)                                         |
| `GROQ_API_KEY`              | Groq                                               |
| `OPENROUTER_API_KEY`        | OpenRouter                                         |
| `VERCEL_API_KEY`            | Vercel AI Gateway                                  |
| `MINIMAX_API_KEY`           | MiniMax                                            |
| `CEREBRAS_API_KEY`          | Cerebras                                           |
| `HF_TOKEN`                  | Hugging Face Inference                             |
| `HYPER_API_KEY`             | Hyper                                              |
| `VERTEXAI_PROJECT`          | Google Cloud VertexAI（需同时设置）                |
| `VERTEXAI_LOCATION`         | Google Cloud VertexAI（需同时设置）                |
| `AZURE_OPENAI_API_KEY`      | Azure OpenAI                                       |
| `AZURE_OPENAI_API_VERSION`  | Azure OpenAI API 版本                              |
| `AWS_ACCESS_KEY_ID`         | Amazon Bedrock（也支持 `AWS_PROFILE`、`AWS_BEARER_TOKEN_BEDROCK`、`~/.aws/credentials` 等方式认证） |
| `AWS_SECRET_ACCESS_KEY`     | Amazon Bedrock                                     |
| `AWS_REGION`                | Amazon Bedrock（或 `AWS_DEFAULT_REGION`）           |

## Configuration

MegaCLI 无需任何配置即可运行。如需自定义，可在本地或全局添加配置文件，优先级从低到高：

1. `~/.megacli/megacli.json`（全局配置）
2. `megacli.json` / `.megacli.json`（从 CWD 向上查找的项目配置，越近优先级越高）
3. `.megacli/megacli.json`（项目工作区配置，最高优先级）

> 旧版路径 `$HOME/.config/megacli/megacli.json` 仍可使用，但新安装默认使用 `~/.megacli/`。

可使用 `megacli schema` 命令生成配置文件的 JSON Schema，方便编辑器提供自动补全和校验。

### LSPs

MegaCLI 可以利用 LSP 获取额外的代码上下文。默认会根据项目根标记文件（如 `go.mod`、`package.json`）自动发现并启动 LSP 服务，也可手动配置：

```json
{
  "lsp": {
    "go": {
      "command": "gopls"
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  }
}
```

可通过 `"auto_lsp": false` 关闭自动 LSP 发现。

### MCPs

MegaCLI 支持三种传输类型的 MCP 服务：`stdio`、`http` 和 `sse`。

```json
{
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"]
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer $GH_PAT"
      }
    }
  }
}
```

每个 MCP 服务还支持 `disabled`、`disabled_tools`、`timeout`、`env` 等字段。

### Hooks

MegaCLI 支持 Hooks 机制，允许在工具调用前运行自定义 shell 脚本来拦截、改写输入或注入上下文。Hooks 兼容 Claude Code hooks 格式。目前支持 `PreToolUse` 事件。详情请参阅 [Hook 指南](./docs/hooks/)。

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/my-hook.sh",
        "timeout": 10
      }
    ]
  }
}
```

### Ignoring Files

MegaCLI 默认遵循 `.gitignore` 规则。你也可以创建 `.megacliignore` 文件来指定额外需要忽略的文件和目录，语法与 `.gitignore` 相同。`.megacliignore` 支持层级放置，与 `.gitignore` 的目录级生效逻辑一致。

### Allowing Tools

默认情况下，MegaCLI 在执行工具调用前会请求用户确认。你可以预先允许特定工具免确认执行：

```json
{
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep",
      "glob",
      "edit"
    ]
  }
}
```

支持 `tool:action` 格式进行更细粒度的控制（如 `"bash:execute"`）。

也可以使用 `--yolo`（`-y`）参数跳过所有权限确认，或在配置中设置 `"permissions": {"yolo": true}`。

### Custom Providers

MegaCLI 支持自定义 Provider 配置。支持的 `type` 包括：`openai`、`openai-compat`、`anthropic`、`gemini`（即 `google`）、`azure`、`google-vertex`。

#### OpenAI-Compatible APIs

```json
{
  "providers": {
    "deepseek": {
      "type": "openai-compat",
      "base_url": "https://api.deepseek.com/v1",
      "api_key": "$DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-chat",
          "name": "Deepseek V3",
          "context_window": 64000,
          "default_max_tokens": 5000
        }
      ]
    }
  }
}
```

#### Anthropic-Compatible APIs

```json
{
  "providers": {
    "my-anthropic": {
      "type": "anthropic",
      "base_url": "https://my-proxy.example.com/v1",
      "api_key": "$MY_API_KEY",
      "models": [
        {
          "id": "claude-sonnet-4-20250514",
          "name": "Claude Sonnet 4",
          "context_window": 200000,
          "default_max_tokens": 16000
        }
      ]
    }
  }
}
```

#### 本地模型 (Ollama / LM Studio)

```json
{
  "providers": {
    "ollama": {
      "name": "Ollama",
      "base_url": "http://localhost:11434/v1/",
      "type": "openai-compat",
      "models": [
        {
          "name": "Qwen 3 30B",
          "id": "qwen3:30b",
          "context_window": 256000,
          "default_max_tokens": 20000
        }
      ]
    }
  }
}
```

Provider 还支持 `extra_headers`、`extra_body`、`system_prompt_prefix`、`disable` 等高级选项。

## CLI Commands

| 命令                      | 说明                                       |
| ------------------------- | ------------------------------------------ |
| `megacli`                 | 启动交互式 TUI                             |
| `megacli run "<prompt>"`  | 非交互式运行（支持管道输入输出）           |
| `megacli --continue`      | 继续最近一次会话                           |
| `megacli --session <id>`  | 继续指定会话                               |
| `megacli --agent <name>`  | 以指定 Agent 启动（如 `coder`、`planner`） |
| `megacli logs`            | 查看日志（默认最后 1000 行）               |
| `megacli logs -f`         | 实时跟踪日志                               |
| `megacli logs -t <N>`     | 查看最后 N 行日志                          |
| `megacli session list`    | 列出会话                                   |
| `megacli stats`           | 查看使用统计                               |
| `megacli projects`        | 查看已注册的项目                           |
| `megacli schema`          | 生成配置文件 JSON Schema                   |
| `megacli update-providers`| 手动更新 Provider 列表                     |
| `megacli login`           | 登录认证                                   |
| `megacli dirs`            | 显示数据目录路径                           |
| `megacli --update`        | 更新到最新版本                             |

## Logging

日志存储在项目目录下的 `.megacli/logs/crush.log`。

```bash
# 打印最后 1000 行日志
megacli logs

# 只看最后 100 行
megacli logs --tail 100

# 实时跟踪日志
megacli logs --follow
```

使用 `--debug` 参数可输出更详细的日志。

## Context Files

MegaCLI 会自动读取项目中的以下文件作为 Agent 上下文（均可选）：

- `AGENTS.md` / `agents.md`
- `MEGACLI.md` / `megacli.md` / `MegaCli.md`（及 `.local.md` 变体）
- `CLAUDE.md` / `CLAUDE.local.md`
- `GEMINI.md` / `gemini.md`
- `.megacli/AGENTS.md` / `.megacli/MEGACLI.md`
- `.cursorrules` / `.cursor/rules/`
- `.github/copilot-instructions.md`

这些文件用于为 Agent 提供项目级别的指令和上下文。
