# MegaCLI

> 本项目基于 [Crush](https://github.com/charmbracelet/crush)（由 [Charm](https://charm.land) 开发的终端 AI 编程助手）二次开发而来，仅供内部使用，不对外提供商业服务。
> 原项目遵循 [FSL-1.1-MIT](./LICENSE.md) 协议，本项目保留原始版权声明与许可条款。

终端里的 AI 编程助手，无缝接入你的工具、代码与工作流，全面兼容主流 LLM 模型。

## Features

- **Multi-Model:** 支持丰富的 LLM 模型，也可通过 OpenAI 或 Anthropic 兼容 API 接入自定义模型
- **Flexible:** 会话中途切换模型，上下文不丢失
- **Session-Based:** 每个项目支持多个独立工作会话与上下文
- **LSP-Enhanced:** 利用 LSP 提供代码智能上下文
- **Extensible:** 通过 MCP（`http`、`stdio`、`sse`）扩展能力
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

## Getting Started

获取你偏好的模型提供商的 API Key（Anthropic、OpenAI、Groq、OpenRouter 等），然后运行 `megacli`。首次运行时会提示你输入 API Key。

也可以通过环境变量配置：

| Environment Variable        | Provider                                           |
| --------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`         | Anthropic                                          |
| `OPENAI_API_KEY`            | OpenAI                                             |
| `VERCEL_API_KEY`            | Vercel AI Gateway                                  |
| `GEMINI_API_KEY`            | Google Gemini                                      |
| `GROQ_API_KEY`              | Groq                                               |
| `OPENROUTER_API_KEY`        | OpenRouter                                         |
| `MINIMAX_API_KEY`           | MiniMax                                            |
| `CEREBRAS_API_KEY`          | Cerebras                                           |
| `HF_TOKEN`                  | Hugging Face Inference                             |
| `VERTEXAI_PROJECT`          | Google Cloud VertexAI (Gemini)                     |
| `VERTEXAI_LOCATION`         | Google Cloud VertexAI (Gemini)                     |
| `AWS_ACCESS_KEY_ID`         | Amazon Bedrock (Claude)                            |
| `AWS_SECRET_ACCESS_KEY`     | Amazon Bedrock (Claude)                            |
| `AWS_REGION`                | Amazon Bedrock (Claude)                            |

## Configuration

MegaCLI 无需任何配置即可运行。如需自定义，可在本地或全局添加配置文件，优先级如下：

1. `.megacli.json`（项目目录）
2. `megacli.json`（项目目录）
3. `$HOME/.config/megacli/megacli.json`（全局配置）

### LSPs

MegaCLI 可以利用 LSP 获取额外的代码上下文：

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

### Hooks

MegaCLI 初步支持 Hooks 机制。详情请参阅 [Hook 指南](./docs/hooks/)。

### Ignoring Files

MegaCLI 默认遵循 `.gitignore` 规则。你也可以创建 `.megacliignore` 文件来指定额外需要忽略的文件和目录。

### Allowing Tools

默认情况下，MegaCLI 在执行工具调用前会请求用户确认。你可以预先允许特定工具免确认执行：

```json
{
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep",
      "edit"
    ]
  }
}
```

也可以使用 `--yolo` 参数跳过所有权限确认。

### Custom Providers

MegaCLI 支持 OpenAI 兼容和 Anthropic 兼容的自定义 Provider 配置。

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

## Logging

日志存储在项目目录下的 `./.megacli/logs/crush.log`。

```bash
# 打印最后 1000 行日志
megacli logs

# 实时跟踪日志
megacli logs --follow
```

使用 `--debug` 参数可输出更详细的日志。
