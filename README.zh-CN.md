# Claude / OpenAI Chat / Codex Responses / Gemini API Proxy - CCX

[![GitHub release](https://img.shields.io/github/v/release/BenedictKing/ccx)](https://github.com/BenedictKing/ccx/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

CCX 是一个高性能的 AI API 代理与协议转换网关，支持 Claude、OpenAI Chat / Codex Responses、Gemini 等上游服务。它提供统一入口、内置 Web 管理界面、故障转移、多密钥管理、渠道编排和模型路由能力。

## 功能特性

- 后端与前端一体化架构，单端口部署
- 双密钥认证：`PROXY_ACCESS_KEY` 与可选 `ADMIN_ACCESS_KEY`
- 内置 Web 管理面板，可视化管理渠道、测试和监控
- 同时支持 Claude Messages、OpenAI Chat Completions、Codex Responses、Gemini
- 支持 Claude Messages、OpenAI Chat、Gemini、Responses 四协议互转
- 智能调度：优先级、促销期、健康检查、自动熔断
- 每个渠道支持多 API Key 轮换、代理、自定义请求头、模型白名单
- 支持模型重定向、Fast 模式和输出冗长度控制
- 支持流式与非流式响应
- Responses API 支持多轮会话跟踪

## 架构概览

CCX 对外提供一个统一后端入口：

```text
客户端 -> backend :3000 ->
  |- /                    -> Web 管理界面
  |- /api/*               -> 管理 API
  |- /v1/messages         -> Claude Messages 代理
  |- /v1/chat/completions -> OpenAI Chat 代理
  |- /v1/responses        -> Codex Responses 代理
  |- /v1/models           -> Models API
  `- /v1beta/models/*     -> Gemini 代理
```

核心特点：

- 单端口
- 前端构建产物嵌入后端
- 不依赖额外 Nginx
- 管理流量和代理流量可使用不同密钥

详细设计请参考 [ARCHITECTURE.md](ARCHITECTURE.md)。

## 快速开始

### 方式一：直接运行二进制

1. 从 [Releases](https://github.com/BenedictKing/ccx/releases/latest) 下载最新可执行文件
2. 在可执行文件同目录创建 `.env`：

```bash
PROXY_ACCESS_KEY=your-super-strong-secret-key
PORT=3000
ENABLE_WEB_UI=true
APP_UI_LANGUAGE=zh-CN
```

3. 启动后访问 `http://localhost:3000`

### 方式二：Docker

```bash
docker run -d \
  --name ccx \
  -p 3000:3000 \
  -e PROXY_ACCESS_KEY=your-super-strong-secret-key \
  -e APP_UI_LANGUAGE=zh-CN \
  -v $(pwd)/.config:/app/.config \
  crpi-i19l8zl0ugidq97v.cn-hangzhou.personal.cr.aliyuncs.com/bene/ccx:latest
```

### 方式三：源码构建

```bash
git clone https://github.com/BenedictKing/ccx
cd ccx
cp backend-go/.env.example backend-go/.env
make run
```

常用命令：

```bash
make run
make dev
make build
```

## UI 语言配置

通过环境变量设置管理界面的默认语言：

```bash
APP_UI_LANGUAGE=zh-CN
```

支持的值：

- `en`
- `id`
- `zh-CN`

如果值无效，会自动回退到 `en`。

## 主要环境变量

```bash
PORT=3000
ENV=production
ENABLE_WEB_UI=true
PROXY_ACCESS_KEY=your-super-strong-secret-key
ADMIN_ACCESS_KEY=your-admin-secret-key
APP_UI_LANGUAGE=zh-CN
LOG_LEVEL=info
REQUEST_TIMEOUT=300000
```

## 主要接口

- Web UI：`GET /`
- 健康检查：`GET /health`
- 管理 API：`/api/*`
- Claude Messages：`POST /v1/messages`
- OpenAI Chat：`POST /v1/chat/completions`
- Codex Responses：`POST /v1/responses`
- Gemini：`POST /v1beta/models/{model}:generateContent`

## 开发

推荐本地开发方式：

```bash
make dev
```

仅前端：

```bash
cd frontend
npm install
npm run dev
```

仅后端：

```bash
cd backend-go
make dev
```

## 相关文档

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [DEVELOPMENT.md](DEVELOPMENT.md)
- [ENVIRONMENT.md](ENVIRONMENT.md)
- [RELEASE.md](RELEASE.md)

## 许可证

MIT
