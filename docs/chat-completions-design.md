# OpenAI Chat Completions 端点设计文档

## 一、概述

### 现状

当前系统对外暴露三类用户侧 API：

| apiType | 端点 | 说明 |
|---------|------|------|
| messages | `POST /v1/messages` | Claude Messages 格式 |
| responses | `POST /v1/responses` | Responses 格式 |
| gemini | `POST /v1beta/models/*` | Gemini 原生格式 |

`/v1/messages` 已支持 `serviceType=openai/gemini` 作为上游格式（协议转换），但用户侧只能以 Claude Messages 格式请求。

### 目标

新增第四类用户侧 API：

| apiType | 端点 | 说明 |
|---------|------|------|
| chat | `POST /v1/chat/completions` | OpenAI Chat Completions 格式 |

用户可直接以 OpenAI Chat 格式请求，系统根据渠道 `serviceType` 选择透传或转换：

- `serviceType=openai` → 直接透传
- `serviceType=claude` → OpenAI Chat → Claude Messages 转换
- `serviceType=gemini` → OpenAI Chat → Gemini 转换
- `serviceType=responses` → OpenAI Chat → Responses 转换

### 关键术语

- `apiType`：用户侧接口族（决定端点、调度器、指标域、配置分组）
- `serviceType`：上游适配器/协议（决定请求转换/透传、URL 拼接与鉴权方式）

## 二、后端设计

### 2.1 Config 变更

```go
type Config struct {
    // 新增
    ChatUpstream    []UpstreamConfig `json:"chatUpstream,omitempty"`
    ChatLoadBalance string           `json:"chatLoadBalance,omitempty"`

    // 现有不变
    Upstream             []UpstreamConfig `json:"upstream"`
    ResponsesUpstream    []UpstreamConfig `json:"responsesUpstream"`
    GeminiUpstream       []UpstreamConfig `json:"geminiUpstream"`
    // ...
}
```

`GetConfig()` 深拷贝需扩展 `ChatUpstream`：

```go
if cm.config.ChatUpstream != nil {
    cloned.ChatUpstream = make([]UpstreamConfig, len(cm.config.ChatUpstream))
    for i := range cm.config.ChatUpstream {
        cloned.ChatUpstream[i] = *cm.config.ChatUpstream[i].Clone()
    }
}
```

### 2.2 Chat 渠道 CRUD

新增 `internal/config/config_chat.go`，复用 Messages 模式：

```go
// Chat 渠道方法（与 config_messages.go 对等）
func (cm *ConfigManager) GetCurrentChatUpstream() (*UpstreamConfig, error)
func (cm *ConfigManager) GetCurrentChatUpstreamWithIndex() (*UpstreamConfig, int, error)
func (cm *ConfigManager) AddChatUpstream(upstream UpstreamConfig) error
func (cm *ConfigManager) UpdateChatUpstream(index int, updates UpstreamUpdate) (bool, error)
func (cm *ConfigManager) RemoveChatUpstream(index int) (*UpstreamConfig, error)
func (cm *ConfigManager) AddChatAPIKey(index int, apiKey string) error
func (cm *ConfigManager) RemoveChatAPIKey(index int, apiKey string) error
func (cm *ConfigManager) MoveChatAPIKeyToTop(upstreamIndex int, apiKey string) error
func (cm *ConfigManager) MoveChatAPIKeyToBottom(upstreamIndex int, apiKey string) error
func (cm *ConfigManager) ReorderChatUpstreams(order []int) error
func (cm *ConfigManager) SetChatChannelStatus(index int, status string) error
func (cm *ConfigManager) SetChatChannelPromotion(index int, duration time.Duration) error
func (cm *ConfigManager) SetChatLoadBalance(strategy string) error
```

> `clearFailedKeysForUpstream` 调用时 apiType 传 `"Chat"`。
> `DeprioritizeAPIKey` 需扩展遍历 `ChatUpstream`。

### 2.3 Chat Handler

新增 `internal/handlers/chat/` 目录：

```
handlers/chat/
├── handler.go      # POST /v1/chat/completions 主处理器
├── channels.go     # Chat 渠道管理 API（CRUD/排序/状态/促销/Ping）
└── converter.go    # OpenAI Chat ↔ Claude/Gemini 双向转换
```

**请求流程**：

```
用户发送 OpenAI Chat 格式请求
  ↓
/v1/chat/completions handler
  ↓
Scheduler 选择 Chat 渠道 (ChannelKindChat)
  ↓
根据渠道 serviceType 选择转换方向:
  - serviceType=openai     → 直接透传（最小转换）
  - serviceType=claude     → OpenAI Chat → Claude Messages
  - serviceType=gemini     → OpenAI Chat → Gemini
  - serviceType=responses  → OpenAI Chat → Responses
  ↓
TryUpstreamWithAllKeys (复用现有 failover 逻辑)
  ↓
响应转换回 OpenAI Chat 格式
  ↓
返回给用户
```

**转换器设计**：

Chat handler 需要一套独立的 Provider 接口（或复用现有 Provider 并增加反向转换）：

```go
// ChatProvider 将 OpenAI Chat 请求转换为上游格式，并将上游响应转换回 OpenAI Chat 格式
type ChatProvider interface {
    // 将 OpenAI Chat 请求转换为上游请求
    ConvertFromChatRequest(c *gin.Context, upstream *config.UpstreamConfig, apiKey string) (*http.Request, []byte, error)
    // 将上游响应转换为 OpenAI Chat 响应
    ConvertToChatResponse(providerResp *types.ProviderResponse) (*types.OpenAIChatResponse, error)
    // 处理流式响应（转换为 OpenAI Chat SSE 格式）
    HandleChatStreamResponse(body io.ReadCloser) (<-chan string, <-chan error, error)
}
```

> 注意：现有 `providers/openai.go` 已实现 OpenAI → Claude 方向的转换（用于 Messages handler 的 `serviceType=openai`）。
> Chat handler 需要的是**反向**：Claude → OpenAI Chat。可以复用现有转换逻辑的数据结构，但转换方向相反。

**OpenAI Chat ↔ Claude Messages 核心映射**：

| OpenAI Chat | Claude Messages | 说明 |
|-------------|-----------------|------|
| `messages[].role=system` | `system` 参数 | Claude 的 system 是顶层参数 |
| `messages[].role=user` | `messages[].role=user` | 直接映射 |
| `messages[].role=assistant` | `messages[].role=assistant` | 直接映射 |
| `messages[].tool_calls` | `messages[].content[].type=tool_use` | 结构差异较大 |
| `messages[].role=tool` | `messages[].content[].type=tool_result` | tool_call_id 映射 |
| `model` | `model` | 直接映射（可经过 ModelMapping） |
| `max_tokens` | `max_tokens` | 直接映射 |
| `temperature` | `temperature` | 直接映射 |
| `top_p` | `top_p` | 直接映射 |
| `stream` | `stream` | 直接映射 |
| `tools` | `tools` | 结构差异：OpenAI 用 `function`，Claude 用 `input_schema` |
| `stop` | `stop_sequences` | 字段名不同 |

**流式响应格式差异**：

- OpenAI Chat SSE：`data: {"id":"...","choices":[{"delta":{"content":"..."}}]}`
- Claude Messages SSE：`event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}`

转换器需要在流式模式下逐 chunk 转换事件格式。

### 2.4 ChannelScheduler 扩展

```go
// ChannelKind 新增
const (
    ChannelKindMessages  ChannelKind = "messages"
    ChannelKindChat      ChannelKind = "chat"      // 新增
    ChannelKindResponses ChannelKind = "responses"
    ChannelKindGemini    ChannelKind = "gemini"
)

// ChannelScheduler 新增字段
type ChannelScheduler struct {
    // ... 现有字段 ...
    chatMetricsManager     *metrics.MetricsManager  // 新增
    chatChannelLogStore    *metrics.ChannelLogStore  // 新增
}
```

`getMetricsManager` / `getChannelLogStore` / `getChannels` 等 switch 分支需扩展 `ChannelKindChat`。

`NewChannelScheduler` 签名扩展，新增 `chatMetrics *metrics.MetricsManager` 参数。

### 2.5 main.go 变更

```go
// 新增 chat MetricsManager 初始化
var chatMetricsManager *metrics.MetricsManager
if metricsStore != nil {
    chatMetricsManager = metrics.NewMetricsManagerWithPersistence(
        envCfg.MetricsWindowSize, envCfg.MetricsFailureThreshold, metricsStore, "chat")
} else {
    chatMetricsManager = metrics.NewMetricsManagerWithConfig(
        envCfg.MetricsWindowSize, envCfg.MetricsFailureThreshold)
}

// NewChannelScheduler 扩展
channelScheduler := scheduler.NewChannelScheduler(
    cfgManager,
    messagesMetricsManager,
    chatMetricsManager,         // 新增
    responsesMetricsManager,
    geminiMetricsManager,
    traceAffinityManager,
    urlManager,
)
```

### 2.6 路由注册

```go
// Chat 渠道管理 API（与 Messages/Responses/Gemini 对等）
apiGroup.GET("/chat/channels", chat.GetUpstreams(cfgManager))
apiGroup.POST("/chat/channels", chat.AddUpstream(cfgManager))
apiGroup.PUT("/chat/channels/:id", chat.UpdateUpstream(cfgManager, channelScheduler))
apiGroup.DELETE("/chat/channels/:id", chat.DeleteUpstream(cfgManager, channelScheduler))
apiGroup.POST("/chat/channels/:id/keys", chat.AddApiKey(cfgManager))
apiGroup.DELETE("/chat/channels/:id/keys/:apiKey", chat.DeleteApiKey(cfgManager))
apiGroup.POST("/chat/channels/:id/keys/:apiKey/top", chat.MoveApiKeyToTop(cfgManager))
apiGroup.POST("/chat/channels/:id/keys/:apiKey/bottom", chat.MoveApiKeyToBottom(cfgManager))
apiGroup.POST("/chat/channels/reorder", chat.ReorderChannels(cfgManager))
apiGroup.PATCH("/chat/channels/:id/status", chat.SetChannelStatus(cfgManager))
apiGroup.POST("/chat/channels/:id/resume", handlers.ResumeChannel(channelScheduler, /* chat kind */))
apiGroup.POST("/chat/channels/:id/promotion", chat.SetChannelPromotion(cfgManager))
apiGroup.GET("/chat/channels/metrics", handlers.GetChannelMetricsWithConfig(chatMetricsManager, cfgManager, /* chat */))
apiGroup.GET("/chat/channels/metrics/history", handlers.GetChannelMetricsHistory(chatMetricsManager, cfgManager, /* chat */))
apiGroup.GET("/chat/channels/:id/keys/metrics/history", handlers.GetChannelKeyMetricsHistory(chatMetricsManager, cfgManager, /* chat */))
apiGroup.GET("/chat/channels/dashboard", chat.GetDashboard(cfgManager, channelScheduler))
apiGroup.GET("/chat/channels/scheduler/stats", handlers.GetSchedulerStats(channelScheduler))
apiGroup.GET("/chat/global/stats/history", handlers.GetGlobalStatsHistory(chatMetricsManager))
apiGroup.GET("/chat/models/stats/history", handlers.GetModelStatsHistory(chatMetricsManager))
apiGroup.GET("/chat/channels/:id/logs", handlers.GetChannelLogs(channelScheduler.GetChannelLogStore(scheduler.ChannelKindChat)))
apiGroup.GET("/chat/ping/:id", chat.PingChannel(cfgManager))
apiGroup.GET("/chat/ping", chat.PingAllChannels(cfgManager))

// 代理端点
r.POST("/v1/chat/completions", chat.Handler(envCfg, cfgManager, channelScheduler))
```

### 2.7 指标统计

现有指标已按 `apiType` 隔离（每个 apiType 独立 MetricsManager + SQLite `api_type` 列）。

新增 `apiType=chat` 只需：
- 初始化独立的 `MetricsManager(apiType=chat)`
- `metricsKey = hash(baseURL | apiKey)` 算法不变
- 无需历史数据迁移

### 2.8 DeprioritizeAPIKey 扩展

现有 `DeprioritizeAPIKey` 遍历 `Upstream` 和 `ResponsesUpstream`，需扩展遍历 `ChatUpstream`：

```go
// 新增 Chat 渠道遍历
for upstreamIdx := range cm.config.ChatUpstream {
    upstream := &cm.config.ChatUpstream[upstreamIdx]
    // ... 同现有逻辑 ...
}
```

### 2.9 FailedKeysCache 按 apiType 隔离

当前 `FailedKeysCache` 的 cache key 是纯 `apiKey` 字符串，导致同一个 Key 在不同渠道类型间互相影响。
引入 Chat 后必须修复——不同 apiType 的失败应互相隔离。

**改造方案**：cache key 从 `apiKey` 改为 `apiType:apiKey`。

```go
// 构造 cache key
func failedKeyCacheKey(apiType, apiKey string) string {
    return apiType + ":" + apiKey
}
```

**受影响方法**：

- `MarkKeyAsFailed(apiKey, apiType)` — 写入时用 `failedKeyCacheKey(apiType, apiKey)`
- `isKeyFailed(apiKey, apiType)` — 新增 `apiType` 参数，查询时用复合 key
- `GetNextAPIKey(upstream, failedKeys, apiType)` — 内部 `isKeyFailed` 调用传入 `apiType`（参数已有）
- `IsKeyFailed(apiKey)` — 公开方法签名改为 `IsKeyFailed(apiKey, apiType string)`
- `clearFailedKeysForUpstream(upstream, apiType)` — 清理时用复合 key
- `cleanupExpiredFailures()` — 无需改动（遍历所有 cache key，前缀不影响过期逻辑）

> 低成本改动（仅改 cache key 拼接），彻底消除跨渠道类型的冷却误伤。
> 对现有 messages/responses/gemini 同样生效。

### 2.10 TraceAffinityManager 按 apiType 隔离

当前 `TraceAffinityManager` 是全局单例，`userID → channelIndex` 映射没有区分 apiType。
不同 apiType 的 channelIndex 会冲突——messages 的 channel[2] 和 chat 的 channel[2] 是完全不同的上游。

虽然不会导致错误路由（调度器会通过 `getUpstreamByIndex(idx, kind)` 获取正确 apiType 的渠道），
但会产生**虚假亲和**：用户在 messages 中绑定的渠道索引会影响 chat 的渠道选择。

**改造方案**：affinity map 的 key 从 `userID` 改为 `apiType:userID`。

```go
// ChannelScheduler 中的调用改造
func (s *ChannelScheduler) SetTraceAffinity(userID string, channelIndex int, kind ChannelKind) {
    affinityKey := string(kind) + ":" + userID
    s.traceAffinity.SetPreferredChannel(affinityKey, channelIndex)
}

func (s *ChannelScheduler) getTraceAffinity(userID string, kind ChannelKind) (int, bool) {
    affinityKey := string(kind) + ":" + userID
    return s.traceAffinity.GetPreferredChannel(affinityKey)
}
```

**受影响调用点**：

- `SelectChannel()` 中查询亲和性：传入 `kind`
- `multi_channel_failover.go` 中设置亲和性：传入 `kind`
- `responses/compact.go` 中设置亲和性：传入 `kind`

> 同样是低成本改动，在 ChannelScheduler 层拼接前缀即可，TraceAffinityManager 本身无需修改。

## 三、前端设计

### 3.1 路由

```typescript
// 现有动态路由不变，type 参数扩展
// /channels/:type  →  type: 'messages' | 'chat' | 'responses' | 'gemini'
```

### 3.2 导航栏

Tab 顺序：**Claude | OpenAI Chat | Codex | Gemini**

### 3.3 Channel Store 扩展

```typescript
// ApiTab 类型扩展
type ApiTab = 'messages' | 'chat' | 'responses' | 'gemini'

// 新增 chat 数据
const chatChannelsData = ref<ChannelsResponse>({
  channels: [],
  current: -1,
  loadBalance: 'round-robin'
})

// dashboardCache 扩展
const dashboardCache = ref<Record<ApiTab, DashboardCache>>({
  messages: { metrics: [], stats: undefined, recentActivity: undefined },
  chat: { metrics: [], stats: undefined, recentActivity: undefined },
  responses: { metrics: [], stats: undefined, recentActivity: undefined },
  gemini: { metrics: [], stats: undefined, recentActivity: undefined }
})

// currentChannelType 路由匹配扩展
const currentChannelType = computed(() => {
  const type = route.params.type as ApiTab
  return ['messages', 'chat', 'responses', 'gemini'].includes(type) ? type : 'messages'
})
```

### 3.4 API 服务扩展

`services/api.ts` 新增 Chat 渠道的完整 API 封装（与现有 messages/responses/gemini 保持一致）：

- `fetchChatChannels()` / `addChatChannel()` / `updateChatChannel()` / `deleteChatChannel()`
- `pingChatChannel()` / `pingAllChatChannels()`
- `reorderChatChannels()` / `setChatChannelStatus()` / `setChatChannelPromotion()`
- `fetchChatMetrics()` / `fetchChatMetricsHistory()` / `fetchChatKeyMetricsHistory()`
- `fetchChatDashboard()` / `fetchChatGlobalStatsHistory()` / `fetchChatModelStatsHistory()`
- `fetchChatChannelLogs()`

### 3.5 复用现有组件

`ChannelsView.vue` 和 `ChannelOrchestration.vue` 已按 `channelType` 参数化，新增 `chat` 类型后应能直接复用。需要检查的点：

- `AddChannelModal.vue`：`serviceType` 下拉选项需包含 `openai`（Chat 的默认透传选项）
- `ChannelCard.vue`：展示逻辑无需修改（已按通用模式实现）
- 指标图表组件：确认 API 路径按 `channelType` 动态拼接

## 四、实施步骤

### Step 1: 后端 Config + CRUD + 全局状态隔离

- `Config` 新增 `ChatUpstream` / `ChatLoadBalance`
- `config_chat.go`：Chat 渠道 CRUD 方法
- `GetConfig()` 深拷贝扩展
- `DeprioritizeAPIKey` 扩展
- `FailedKeysCache` cache key 改为 `apiType:apiKey`（见 2.9）
- `TraceAffinityManager` affinity key 改为 `apiType:userID`（见 2.10）

### Step 2: 后端 Scheduler + Metrics

- `ChannelKind` 新增 `ChannelKindChat`
- `ChannelScheduler` 扩展（chatMetricsManager + chatChannelLogStore）
- `main.go` 初始化

### Step 3: 后端 Chat Handler + 转换器

- `handlers/chat/handler.go`：主处理器
- `handlers/chat/converter.go`：OpenAI Chat ↔ Claude/Gemini 转换
- `handlers/chat/channels.go`：渠道管理 API
- 路由注册
- 流式响应支持

### Step 4: 前端 Chat Tab

- 路由 + 导航栏
- `stores/channel.ts` 扩展
- `services/api.ts` 扩展
- 复用 `ChannelsView.vue` / `ChannelOrchestration.vue`

### Step 5: 测试

- 单元测试：OpenAI Chat ↔ Claude 转换器（含 tool_use、streaming）
- 单元测试：Chat 渠道 CRUD
- 集成测试：`/v1/chat/completions` 端到端（各 serviceType）
- 前端测试：Chat Tab 渲染、渠道 CRUD、指标展示

## 五、向后兼容性

- `chatUpstream` 字段为 `omitempty`，现有 config.json 无需迁移
- 现有 messages/responses/gemini 端点和 API 完全不变
- 前端保留现有三个 Tab，新增 Chat Tab

## 六、风险点

1. **OpenAI Chat ↔ Claude 转换精度**：system message 处理（Claude 是顶层参数）、tool_use 结构差异、streaming 事件格式差异——需要充分的边界测试
2. **OpenAI Chat 格式的多样性**：不同客户端（Cursor、Continue、Cline 等）发送的 OpenAI Chat 请求可能有细微差异（如 `function_call` vs `tool_calls`），需要兼容处理
