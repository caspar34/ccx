# Credential Pool（密钥池）设计文档

## 一、核心概念

### 现状问题

当前架构中，API Key 绑定在具体渠道（Channel）上，每个渠道有独立的 `apiKeys[]`。
实际场景中，很多上游服务商的同一个 Key 可以同时支持多种协议格式（如 OpenAI Chat、Claude Messages、Responses 等）。
用户不得不在多个渠道中重复配置相同的 Key，维护成本高且容易出错。

### 目标

引入 **Credential Pool（密钥池）** 概念，将 Key 管理从渠道配置中解耦：

- 一个密钥池包含一组 API Key + 一个 Base URL
- 密钥池通过**上游能力探测**确定该 Key+URL 组合支持哪些 `serviceType`
- 用户基于探测结果，为每个用户侧接口族（`apiType`）创建独立渠道，并选择对应的 `serviceType`
- 不同 `apiType` 的渠道共享同一密钥池的 Key，但指标域按 `apiType` 隔离

### 非目标（Non-goals）

为保持方案聚焦并避免实现外溢，本设计**不**覆盖以下内容：

- **密钥加密/密文存储**：仍沿用现有 `config.json` 的明文存储方式（访问控制与文件权限治理不在本方案内）
- **按渠道定制 Key 顺序/权重**：Key 的优先级仅由密钥池内的顺序决定（不同 `apiType` 不做差异化调度）
- **自动周期探测**：探测仅在创建密钥池或用户手动触发时进行
- **覆盖 BaseURL 时的探测一致性**：当渠道显式覆盖 BaseURL/BaseURLs 时，探测结果默认不参与运行期过滤（仅用于展示/提示）

### 关键术语：APIType vs ServiceType

为避免“协议/渠道/上游格式”混用，这里把维度拆开：

- `apiType`：用户侧接口族（决定暴露的端点、调度器、指标域、配置分组）
  - `messages`：Claude Messages（`POST /v1/messages`）
  - `chat`：OpenAI Chat Completions（`POST /v1/chat/completions`）
  - `responses`：Responses（`POST /v1/responses`）
  - `gemini`：Gemini（`POST /v1beta/models/*`）
- `serviceType`：上游适配器/协议（决定请求转换/透传、URL 拼接与鉴权方式）
  - `claude`：Anthropic Messages 上游
  - `openai`：OpenAI Chat Completions 上游
  - `responses`：Responses 上游
  - `gemini`：Gemini 上游

> 约定：密钥池的“探测”只负责判断某个 Key+BaseURL 对某个 `serviceType` 是否可用；
> 渠道的 `apiType` 由其所在的配置分组决定（`upstream/chatUpstream/responsesUpstream/geminiUpstream`），
> 渠道的 `serviceType` 决定最终走哪个上游协议/转换路径。

### 四类用户侧 API（apiType）

| 用户侧 API | apiType | 端点 | 说明 |
|------|-------------|------|------|
| Claude Messages | messages | `POST /v1/messages` | 用户侧以 Claude Messages 格式请求 |
| OpenAI Chat Completions | chat | `POST /v1/chat/completions` | 用户侧以 OpenAI Chat Completions 格式请求 |
| Responses | responses | `POST /v1/responses` | 用户侧以 Responses 格式请求 |
| Gemini | gemini | `POST /v1beta/models/*` | 用户侧以 Gemini 原生协议请求 |

> 注意：`apiType=chat` 是新增的用户侧接口族。当前 `/v1/messages` 已支持 `serviceType=openai/gemini` 作为上游格式，
> 但用户侧只能以 Claude Messages 格式请求；新增 `/v1/chat/completions` 后，用户可直接以 OpenAI Chat 格式请求，
> 同时仍可通过 `serviceType=claude/gemini` 等进行转换路由。

## 二、用户场景

### 场景 1：多协议网关（同一 Key 支持多个 serviceType）

用户有一个聚合网关的 Key，该网关在同一 Base URL 下同时提供多种上游协议兼容端点（组合因服务商而异）。

**操作流程**：
1. 创建密钥池，填入 Key 和 Base URL
2. 系统自动探测（按 `serviceType`）：claude ✅ openai ✅ responses ✅ gemini ❌
3. 用户勾选需要暴露的 `apiType`，并为每个渠道选择 `serviceType`（可提供默认映射）
4. 多个渠道共享同一密钥池，各自独立统计（按 `apiType` 隔离）

### 场景 2：原生 Anthropic Key

用户有 Anthropic 官方 Key，只支持 Claude Messages 格式。

**操作流程**：
1. 创建密钥池，填入 Key 和 `https://api.anthropic.com`
2. 系统探测（按 `serviceType`）：claude ✅ openai ❌ responses ❌ gemini ❌
3. 可创建 `serviceType=claude` 的渠道（例如 `apiType=messages` 透传 Claude；或 `apiType=chat` 将 OpenAI Chat 转换到 Claude）

### 场景 3：OpenAI 官方 Key

用户有 OpenAI 官方 Key，支持 Chat Completions 和 Responses。

**操作流程**：
1. 创建密钥池
2. 系统探测（按 `serviceType`）：claude ❌ openai ✅ responses ✅ gemini ❌
3. 用户可创建 `apiType=chat`（`serviceType=openai`）和 `apiType=responses`（`serviceType=responses`）渠道；如仍希望使用 `/v1/messages` 用户协议，也可创建 `apiType=messages`（`serviceType=openai`）渠道

### 场景 4：Google Gemini Key

用户有 Google AI Studio 的 Key。

**操作流程**：
1. 创建密钥池
2. 系统探测（按 `serviceType`）：claude ❌ openai ❌ responses ❌ gemini ✅
3. 用户可创建 `apiType=gemini`（`serviceType=gemini`）渠道；如需要让用户以 Claude/OpenAI 格式请求 Gemini，可分别创建 `apiType=messages/chat` 且 `serviceType=gemini` 的渠道

### 场景 5：多 Key 批量管理

用户有 10 个同一服务商的 Key，需要在多个 `apiType` 渠道中复用，且不同 Key 的上游能力可能不同。

**操作流程**：
1. 创建密钥池，批量添加 10 个 Key
2. 系统逐个探测每个 Key 的 `serviceType` 支持情况（可能不同 Key 权限不同）
3. 基于探测结果创建渠道；每个渠道自动获得密钥池中所有兼容其 `serviceType` 的 Key

### 场景 6：不使用密钥池（向后兼容）

用户不想用密钥池，继续用现有方式直接在渠道中配置 Key。

**操作流程**：完全不变，现有功能 100% 保留。

### 场景 7：密钥池 Key 更新自动传播

用户更新密钥池中的 Key（添加/删除/替换）。

**效果**：所有引用该密钥池的渠道自动获得最新的 Key 列表，无需逐个修改。

## 三、后端设计

### 3.1 数据结构

```go
// Credential 密钥池定义
type Credential struct {
    ID          string   `json:"id"`          // 唯一标识（UUID）
    Name        string   `json:"name"`        // 显示名称
    BaseURL     string   `json:"baseUrl"`     // 上游地址（密钥池级别；建议不带 /v1 或 /v1beta，版本前缀由 serviceType 归一化规则补全）
    BaseURLs    []string `json:"baseUrls,omitempty"` // 多 URL 支持（同 BaseURL 规则）
    APIKeys     []string `json:"apiKeys"`     // Key 列表
    Description string   `json:"description,omitempty"`
    Website     string   `json:"website,omitempty"`

    // 上游能力探测结果（每个 Key 独立，按 serviceType 维度）
    // key: keyHash（避免把明文 key 重复写入 config.json）
    ProbeResults map[string]*KeyProbeResult `json:"probeResults,omitempty"`

    // 通用配置（可被渠道覆盖；继承规则见 3.2）
    InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty"`
    CustomHeaders      map[string]string `json:"customHeaders,omitempty"`

    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}

// KeyProbeResult 单个 Key 的上游能力探测结果（按 serviceType）
type KeyProbeResult struct {
    KeyMask string `json:"keyMask"` // 仅用于展示（不存明文）

    // 各 serviceType 的探测状态（claude/openai/responses/gemini）
    Services map[string]*ServiceProbeStatus `json:"services"`

    ProbedAt time.Time `json:"probedAt"` // 最后探测时间
}

// ServiceProbeStatus 单个 serviceType 的探测状态
type ServiceProbeStatus struct {
    Supported bool     `json:"supported"`           // 是否“可用”（语义见下方约定）
    Models    []string `json:"models,omitempty"`    // 支持的模型列表（建议截断，避免配置膨胀）
    Error     string   `json:"error,omitempty"`     // 探测失败原因（建议使用可枚举的 error code）
    Latency   int64    `json:"latency,omitempty"`   // 探测延迟(ms)
}
```

> keyHash 建议使用 `sha256(apiKey)` 的十六进制前 32 位（或更长）作为 map key：稳定、不可逆且不会在配置文件中重复写入明文 key。
>
> `ServiceProbeStatus.Supported` 建议定义为“该 Key+BaseURL 在该 `serviceType` 下端点存在且鉴权方式匹配（可发起请求）”；`429` 可视为支持但限流。对 `timeout/5xx/network` 等**不确定性错误**，建议仅写入 `Error` 并在过滤时视为 unknown（见 3.2 过滤策略），避免误伤可用 Key。
>
> 探测结果默认只对 **Credential 自身的 BaseURL/BaseURLs** 生效；当渠道显式覆盖 BaseURL/BaseURLs 时，不应基于该探测结果进行运行期过滤（否则会把“Key+BaseURL”耦合关系打破）。

**Config 结构变更**：

```go
type Config struct {
    // 新增
    Credentials []Credential `json:"credentials,omitempty"`

    // 现有字段不变
    Upstream             []UpstreamConfig `json:"upstream"`
    ChatUpstream         []UpstreamConfig `json:"chatUpstream,omitempty"`
    ResponsesUpstream    []UpstreamConfig `json:"responsesUpstream"`
    GeminiUpstream       []UpstreamConfig `json:"geminiUpstream"`
    // ...
}
```

**UpstreamConfig 变更**：

```go
type UpstreamConfig struct {
    // 新增：引用密钥池
    CredentialID string `json:"credentialId,omitempty"`

    // 现有字段全部保留（向后兼容）
    BaseURL     string   `json:"baseUrl"`
    APIKeys     []string `json:"apiKeys"`
    ServiceType string   `json:"serviceType"`
    // ...
}
```

> 约定：
> - 当 `CredentialID` 非空时，`UpstreamConfig.APIKeys` 可保存为空数组（运行期以 `Credential.APIKeys` 为准）；为避免认知混乱，渠道级 “add/remove/move keys” 应在该模式下禁用（见 3.2.2）
> - 当 `CredentialID` 非空且 `BaseURL/BaseURLs` 为空时，表示继承密钥池的 BaseURL/BaseURLs；若渠道显式填写 BaseURL/BaseURLs，则视为覆盖并禁用基于 ProbeResults 的运行期过滤

### 3.2 Key 解析逻辑

在 failover 层（`TryUpstreamWithAllKeys` 调用前）解析 effective keys 与 BaseURL（含多 BaseURL）：

```
请求进入
  ↓
Scheduler 选择渠道 → 获取 UpstreamConfig
  ↓
ResolveEffectiveKeys(upstream, configManager):
  keys = upstream.APIKeys
  if upstream.CredentialID != "":
    credential = configManager.GetCredential(upstream.CredentialID)
    if credential != nil:
      keys = credential.APIKeys
      // 仅当渠道未覆盖 BaseURL/BaseURLs 时，才可认为 ProbeResults 与本渠道“Key+BaseURL”匹配
      // 覆盖 BaseURL 时默认不做过滤（避免误判导致运行期无 key）
      if upstream.BaseURL == "" && len(upstream.BaseURLs) == 0:
        // 按当前渠道的 serviceType 过滤（unknown 默认放行；仅过滤确定性不可用的 key）
        keys = FilterKeysByServiceType(keys, credential.ProbeResults, upstream.ServiceType)
  return keys

ResolveEffectiveBaseURLs(upstream, configManager):
  // 渠道优先（兼容旧配置与覆盖场景）
  if len(upstream.BaseURLs) > 0:
    return upstream.BaseURLs
  if upstream.BaseURL != "":
    return [upstream.BaseURL]

  // 继承密钥池
  if upstream.CredentialID != "":
    credential = configManager.GetCredential(upstream.CredentialID)
    if credential != nil:
      if len(credential.BaseURLs) > 0:
        return credential.BaseURLs
      if credential.BaseURL != "":
        return [credential.BaseURL]
  return nil
  ↓
TryUpstreamWithAllKeys(upstream, effectiveKeys, ...)
```

> `FilterKeysByServiceType` 推荐策略：
> - 对 `ProbeResults` 缺失/未探测的 key **默认放行**
> - 对 `supported=true` 的 key 放行
> - 对 `supported=false` 的 key **仅在 Error 为确定性不可用时过滤**（如 `not_found`、`unauthorized`、`billing`、`invalid_key`）
> - 对 `timeout`、`network_error`、`server_error` 等不确定性错误：视为 unknown 放行（避免探测误伤导致容量骤降）
>
> 运行期若过滤后 `keys` 为空，应视为**渠道配置不可用**：前端应提示“0 可用 Key”并阻止保存/Quick Create；后端可在 handler 层返回明确错误，避免进入 failover 空转。

如果密钥池承载了通用配置（如 `insecureSkipVerify/customHeaders`），也建议在进入 failover 前统一解析 effective 值：

```
ResolveEffectiveInsecureSkipVerify(upstream, credential):
  // bool 无法区分“未设置”和“显式 false”，这里采用 KISS 规则：
  // - 渠道显式开启(true)优先
  // - 否则继承密钥池
  insecure = upstream.InsecureSkipVerify
  if !insecure && credential != nil:
    insecure = credential.InsecureSkipVerify
  return insecure

ResolveEffectiveCustomHeaders(upstream, credential):
  headers = clone(credential.CustomHeaders)
  merge(headers, upstream.CustomHeaders) // 渠道同名 key 覆盖密钥池
  return headers
```

> 该 KISS 规则的副作用：当密钥池设置 `insecureSkipVerify=true` 时，渠道无法显式覆盖为 `false`。如未来确有需求，可将渠道侧字段升级为 `*bool`（三态：inherit/true/false）。

### 3.2.1 HistoricalAPIKeys 与密钥池

现有渠道包含 `historicalApiKeys`（用于统计聚合与展示历史 Key 指标）。引入 Credential 后，Key 的增删改发生在密钥池层，
需要在密钥池更新时同步到所有引用该 Credential 的渠道：

- 从 Credential 中**移除**的 key：追加到引用渠道的 `HistoricalAPIKeys`（去重）
- Credential 新增的 key：不写入历史列表

这样可以保持“换 Key 不丢历史指标”的既有体验。

### 3.2.2 受影响的调用点（必须复用 ResolveEffective*）

除“真实转发请求”的 failover 流程外，以下逻辑也依赖 `BaseURL/BaseURLs` 与 `APIKeys`，在引入 `credentialId` 后必须改为使用解析后的 effective 值：

- **failover 入口**：`TryUpstreamWithAllKeys` 当前会在 `len(upstream.APIKeys)==0` 时直接返回，因此调用前需要把 `effectiveKeys` 写入 `upstreamCopy.APIKeys`（或调整函数签名显式传入）
- **指标聚合/展示**：渠道指标接口通常以“渠道的 BaseURLs + Keys + HistoricalAPIKeys”聚合，需改为“effectiveBaseURLs + effectiveKeys + HistoricalAPIKeys”
- **渠道 Ping**：若渠道继承 BaseURL/SSL 配置，需要用 effectiveBaseURLs 与 effectiveInsecureSkipVerify
- **删除/清理**：删除渠道时清理失败 key 缓存、删除独占 metricsKey 等逻辑，需要正确枚举“该渠道实际可能使用的 keys”（否则会残留内存缓存或误删/漏删指标）
- **渠道 Key 管理端点**：当渠道启用 `credentialId` 时，应禁用/拒绝渠道级 `add/remove/move apiKeys` 操作，并引导用户到密钥池管理（避免出现“改了渠道 key 但运行期仍用密钥池 key”的困惑）

### 3.3 上游能力探测（serviceType Probe）

新增 `internal/handlers/credentials/probe.go`：

**BaseURL 归一化（版本前缀）**：

为保证同一 `Credential.BaseURL` 能被多个 `serviceType` 复用，探测与真实请求都应使用统一的 `NormalizeBaseURL(baseURL, serviceType)` 规则（与现有 `providers/*` 的 `#` 约定保持一致）：

- BaseURL 以 `#` 结尾：去掉 `#` 后直接使用（跳过自动补版本前缀）
- BaseURL 已以 `/v\\d+[a-z]*` 结尾（如 `/v1`、`/v2`、`/v1beta`）：直接使用（不再补版本前缀）
- 其他情况：
  - `claude/openai/responses`：默认补 `/v1`
  - `gemini`：默认补 `/v1beta`

> 因此：当密钥池希望跨协议复用时，BaseURL 推荐配置为不带版本前缀的 host 根路径（如 `https://gateway.example.com`）。

**探测策略**（按 `serviceType`）：

| serviceType | 探测方式（优先轻量） | 端点（相对 `NormalizeBaseURL` 后） | 认证方式 | 判定要点 |
|------|---------|------|------|------|
| claude | 最小 `POST` | `POST /messages` | 复用 `SetAuthenticationHeader`（`sk-ant-*` → `x-api-key`，否则 `Authorization: Bearer`） | `404`=端点不存在；`401/403`=无权限；其他（含 `400/429/5xx`）视为端点存在 |
| openai | `GET models`（失败则回退最小 `POST`） | `GET /models`（fallback: `POST /chat/completions`） | 复用 `SetAuthenticationHeader`（通常为 `Authorization: Bearer`） | `200`=支持；`401/403`=无权限；`429`=支持但限流；`404` 且 fallback 仍 `404`=不支持 |
| responses | 最小 `POST`（允许返回 4xx） | `POST /responses` | 复用 `SetAuthenticationHeader` | `404`=端点不存在；`401/403`=无权限；其他视为端点存在（不追求成功生成） |
| gemini | `GET models` | `GET /models` | 复用 `SetGeminiAuthenticationHeader`（`x-goog-api-key`） | `200`=支持；`404`=端点不存在；`401/403`=无权限；`429`=支持但限流 |

**Error code 约定（建议）**：

- `not_found`：`404`（端点不存在，确定性不可用，可过滤）
- `unauthorized`：`401/403`（鉴权失败，确定性不可用，可过滤）
- `billing`：`402`（余额/套餐问题，确定性不可用，可过滤）
- `rate_limited`：`429`（支持但限流；可保留 `Supported=true` 并写入 `Error=rate_limited`）
- `bad_request`：`400`（常见于“最小探测请求”缺字段/模型不可用；通常可视为端点存在，建议 `Supported=true`，并写入 `Error=bad_request` 仅用于展示）
- `timeout` / `network_error` / `server_error`：超时/网络错误/`5xx`（不确定性错误；建议 **不要** 作为过滤依据）

> `openai` 的 `GET /models` 只能证明“OpenAI 风格 models 端点可用”，不必然 100% 覆盖 `/chat/completions` 的可用性；如需要更严格探测，可在 `GET /models=200` 后追加一次“最小 `POST /chat/completions`”（预期 `400/401/403/429` 等非 404），但这会增加探测开销。

**探测流程**：
1. 用户触发探测（创建密钥池时 / 手动重新探测）
2. 后端对每个 Key 并发探测 4 种 `serviceType`
3. 结果写入 `Credential.ProbeResults[keyHash]`
4. 持久化到 config.json

> 并发与超时建议：对外发探测请求使用 worker pool（例如 `min(8, keyCount*4)`），并对单次探测设置硬超时（如 5-10s）；避免批量 key 探测时瞬时打爆上游或本机连接。
>
> 多 BaseURL：若 `Credential.BaseURLs` 非空，默认仅对“首个 URL”做探测（假设同一密钥池的多 URL 仅是 failover 镜像且能力一致）；如需更严格校验，可在 UI 提供“逐 URL 探测”开关（非本阶段必需）。

**URL 拼接与鉴权**（需与现有 providers 逻辑保持一致）：
- BaseURL：统一按 `NormalizeBaseURL` 处理版本前缀（见上文）
- 鉴权：
  - `claude/openai/responses`：统一复用 `SetAuthenticationHeader`
  - `gemini`：统一复用 `SetGeminiAuthenticationHeader`（`x-goog-api-key`）

> 对 `claude` 的探测请求建议额外设置 `anthropic-version: 2023-06-01`（减少 `400` 噪音并提升可读性）；即便未设置导致 `400`，仍可按“端点存在”处理。

> 注意：探测应该尽量轻量，优先用 models 列表接口而非实际生成请求，避免消耗 token。
> 对于无法通过 models 接口判断的 `serviceType`（如 `claude/responses`），可以发送 `max_tokens=1` 的最小请求，或允许其返回校验错误（只要不是 `404`）。

### 3.3.1（可选）渠道能力测试（模型 + 转换路径）

> 说明：当前系统已有 `/api/{messages|responses|gemini}/ping`，主要用于 BaseURL 可达性（HEAD/延迟）检测；
> 本节的“能力测试”属于**真实请求级**验证，可能产生计费与限流影响，建议作为后续增强（非 MVP 阶段强制要求）。

`serviceType` 探测只判断某个 Key+BaseURL 是否“可按该 `serviceType` 调用”（不保证某个具体模型可用），
**渠道能力测试**用于验证“指定模型 + 指定转换路径”在真实请求下是否可用。

**测试策略**：
- 不使用简单问题（如 "hi"、"test"），这类问题无法有效验证模型能力
- 使用启发性问题，能触发模型真实推理能力
- 严格限制输出长度，避免消耗过多 token

**测试 Prompt**：

| serviceType | 请求格式 | Prompt | 限制 |
|------|---------|--------|------|
| claude | Messages API | `"请用一句话介绍你的能力"` | `max_tokens: 100` |
| openai | Chat Completions | `"请用一句话介绍你的能力"` | `max_tokens: 100` |
| responses | Responses API | `"请用一句话介绍你的能力"` | `max_tokens: 100` |
| gemini | Gemini API | `"请用一句话介绍你的能力"` | `maxOutputTokens: 100` |

**判定规则**：
- 收到有效文本响应 → 可用（记录延迟）
- 收到 `401/403` → Key 无权限
- 收到 `429` → 限流（标记为可用但限流中）
- 收到 `404` → 端点不存在
- 超时（10s）→ 不可用

### 3.4 API 端点设计

```
# 密钥池 CRUD
GET    /api/credentials                    # 列出所有密钥池
POST   /api/credentials                    # 创建密钥池
PUT    /api/credentials/:id                # 更新密钥池
DELETE /api/credentials/:id                # 删除密钥池（需检查引用）

# 密钥池内的 Key 管理
POST   /api/credentials/:id/keys           # 添加 Key
DELETE /api/credentials/:id/keys/:keyHash  # 删除 Key（避免在 URL 中传明文 key；也避免 index 因并发变更导致误删）

# 上游能力探测（serviceType Probe）
POST   /api/credentials/:id/probe          # 触发探测（全部 Key）
POST   /api/credentials/:id/probe/:keyHash # 探测单个 Key

# 引用查询
GET    /api/credentials/:id/references      # 查询哪些渠道引用了此密钥池

# 从密钥池快速创建渠道
POST   /api/credentials/:id/create-channels # 基于探测结果批量创建渠道
```

> `keyHash` 取值规则与 3.1 的 keyHash 约定一致；Credential 详情接口建议同时返回每个 key 的 `keyMask` 与 `keyHash`，用于前端执行“删除 key / 单 key 探测”，避免在 URL 与日志中携带明文 key。

### 3.5 指标统计隔离（按 apiType）

现状实现里，指标本身已经按 `apiType` 分域：

- 内存侧：每个 `apiType` 各自维护独立的 `MetricsManager`
- 持久化侧：SQLite `request_records` 已包含 `api_type` 列，读取/写入时按 `api_type` 隔离

因此同一 `metricsKey = hash(baseURL | apiKey)` 在不同 `apiType` 下不会混淆，无需修改 `metricsKey` 算法，也不需要历史迁移。

新增 `apiType=chat` 时，只需按既有模式增加一个独立的 `MetricsManager(apiType=chat)` 和对应的持久化读写。

### 3.6 Key 冷却机制兼容性

当前 `FailedKeysCache` 是基于 key 字符串做缓存的。
同一个 Key 被多个渠道共享时，一个渠道标记它失败，其他渠道也会受影响。

需要区分两类失败并采用不同的冷却作用域：

1. **Key 级别失败（全局）**：401/403（鉴权）、402（余额/套餐）、429（限流）等  
   → 应当让所有 `apiType` 共享冷却，避免跨端点重复撞限额。
2. **协议/端点级失败（局部）**：404（端点不存在）、415（不支持的内容类型）、特定转换路径导致的 4xx  
   → 不应影响其他 `apiType`/`serviceType` 的可用性。

**建议方案：FailedKeysCache 分两级 key**：

```go
// 全局冷却（跨 apiType 共享）
globalKey = "global:" + apiKey

// 局部冷却（按 apiType 隔离；必要时可进一步细化到 serviceType）
scopedKey = apiType + ":" + apiKey
```

并在错误分类处明确写入哪一级缓存：全局类错误写 `global:*`，协议/端点类错误写 `apiType:*`（或 `apiType:serviceType:*`）。

## 四、前端设计

目标：在尽量不破坏现有“渠道管理 Tab + Dialog”的交互习惯下，引入密钥池管理与渠道引用能力。

### 4.1 新增密钥池管理入口（不新增页面）

- 保留现有渠道管理路由 `/channels/:type`（`messages/responses/gemini`），并新增 `/channels/chat`
- 顶部导航栏在现有 Tab（Claude / Codex / Gemini / OpenAI Chat）旁新增「密钥池」按钮，打开 `CredentialPoolDialog.vue`

> `apiType` 由当前 Tab/路由决定；`serviceType` 仍在“添加/编辑渠道”中选择。

### 4.2 CredentialPoolDialog.vue（密钥池管理弹窗）

列表卡片展示：
- 名称、BaseURL/BaseURLs、Key 数量
- 探测概览（按 `serviceType`：claude/openai/responses/gemini）
- 引用数（哪些渠道在使用此 Credential）

支持操作：
- 新建/编辑/删除（删除前必须检查引用）
- 触发探测（全部 Key / 单 Key）
- 查看引用
- 从密钥池批量创建渠道（见 4.4）

### 4.3 密钥池编辑弹窗

字段：
- 名称、BaseURL/BaseURLs、Key 列表
- `insecureSkipVerify`、`customHeaders`
- 每个 Key 行展示 `keyMask` + 各 `serviceType` 的探测状态、延迟与错误原因

### 4.4 从密钥池批量创建渠道（Quick Create）

用户选择要创建的渠道 `apiType`（messages/chat/responses/gemini），系统为每个 `apiType` 选择一个默认 `serviceType`：
- `messages` → 优先 `claude`（不支持则 `openai`/`gemini`）
- `chat` → 优先 `openai`（不支持则 `claude`/`gemini`）
- `responses` → 优先 `responses`（不支持则 `openai`/`claude`）
- `gemini` → 优先 `gemini`（不支持则 `openai`/`claude`）

创建渠道时：
- 渠道写入 `credentialId`
- 渠道 `apiKeys` 为空（或忽略），由后端按 `credentialId` 解析有效 Key
- BaseURL 默认继承密钥池（必要时允许在渠道中覆盖）
- 名称默认生成：`{credentialName}-{apiType}-{serviceType}`

UI 需要展示“可用 key 数 / 总 key 数”，避免创建后立即不可用的渠道。

### 4.5 AddChannelModal 改造（支持使用密钥池）

在现有“API Keys”区域增加模式切换：
- 自定义密钥（现有逻辑不变）
- 使用密钥池：选择 `credentialId`，并展示兼容性（按当前渠道选择的 `serviceType` 与探测结果）

BaseURL 规则：
- 默认继承密钥池 BaseURL/BaseURLs
- 渠道层仍允许覆盖 BaseURL（向后兼容与特殊网关场景）

提供「管理密钥池」入口（打开 `CredentialPoolDialog.vue`）。

### 4.6 Pinia Store + API

- 新增 `stores/credential.ts`（CRUD / probe / references / create-channels）
- `services/api.ts` 扩展 `/api/credentials*` 调用封装
- `stores/channel.ts` 保持现有按 `apiType` 分组（messages/responses/gemini），新增 `chat` 分组，并支持 `credentialId` 字段

## 五、新增 OpenAI Chat 用户协议

### 5.1 概述

当前系统中，`/v1/messages` 端点接收 Claude Messages 格式的请求，
内部可以转换为 OpenAI/Gemini 格式发送给上游。

新增 OpenAI Chat 协议后，需要一个新的端点接收 OpenAI Chat Completions 格式的请求，
内部可以转换为 Claude/Gemini 格式发送给上游（或直接透传给 OpenAI 上游）。

### 5.2 新增端点

```
POST /v1/chat/completions    # OpenAI Chat Completions API（用户侧）
GET  /v1/models              # 已有，复用
```

### 5.3 实现方式

新增 `internal/handlers/chat/` 目录：

```
handlers/chat/
├── handler.go      # POST /v1/chat/completions 主处理器
├── channels.go     # Chat 渠道 CRUD（复用现有模式）
└── converter.go    # OpenAI Chat → Claude/Gemini 转换
```

**请求流程**：

```
用户发送 OpenAI Chat 格式请求
  ↓
/v1/chat/completions handler
  ↓
根据渠道 serviceType 选择转换方向:
  - serviceType=openai  → 直接透传（最小转换）
  - serviceType=claude  → OpenAI Chat → Claude Messages 转换
  - serviceType=gemini  → OpenAI Chat → Gemini 转换
  ↓
发送到上游
  ↓
响应转换回 OpenAI Chat 格式
  ↓
返回给用户
```

### 5.4 配置变更

```go
type Config struct {
    // 新增
    ChatUpstream    []UpstreamConfig `json:"chatUpstream,omitempty"`
    ChatLoadBalance string           `json:"chatLoadBalance,omitempty"`

    // 现有不变
    Upstream             []UpstreamConfig `json:"upstream"`
    ResponsesUpstream    []UpstreamConfig `json:"responsesUpstream"`
    GeminiUpstream       []UpstreamConfig `json:"geminiUpstream"`
}
```

### 5.5 前端变更

- 导航栏新增第四个 tab：Claude / Codex / Gemini / **OpenAI Chat**
- 路由保持 `/channels/:type` 动态路由，新增 `type=chat`
- `ChannelsView.vue` 复用现有模式，按 `channelType=chat` 加载/保存 `chatUpstream`
- `services/api.ts` 扩展 Chat 渠道的 CRUD/排序/状态/metrics/dashboard 等接口（与现有 messages/responses/gemini 保持一致的调用形态）

## 六、实施步骤

### Phase 1: 基础设施（后端）

1. **Config 结构扩展**
   - `Config` 新增 `Credentials []Credential` 字段
   - `UpstreamConfig` 新增 `CredentialID` 字段
   - 新增 `Credential` 及相关类型定义
   - 配置热重载兼容

2. **Credential CRUD**
   - `config_credentials.go`: 密钥池的增删改查方法
   - `handlers/credentials/`: HTTP handler
   - 路由注册

3. **Key 解析逻辑**
   - `ResolveEffectiveKeys()` 函数
   - `ResolveEffectiveBaseURLs()` 函数
   - 集成到 failover 流程
   - 同步改造 metrics/ping/删除清理/渠道 Key 管理端点等依赖 `BaseURL/APIKeys` 的逻辑，统一复用 `ResolveEffective*`（见 3.2.2）

### Phase 2: 上游能力探测（后端）

4. **探测引擎**
   - `internal/probe/`: 上游能力探测模块（serviceType Probe）
   - 四种 `serviceType` 的探测实现
   - 并发探测 + 超时控制

5. **探测 API**
   - `handlers/credentials/probe.go`
   - 探测结果持久化到 config.json

### Phase 3: 指标与冷却（后端）

6. **新增 chat 指标域**
   - 为 `apiType=chat` 增加独立的 MetricsManager 与持久化读写（复用 SQLite `api_type` 机制）
   - `metricsKey` 算法保持不变，无需迁移历史数据

7. **FailedKeysCache 分级**
   - 增加 `global:*` 与 `apiType:*` 两级缓存 key
   - 明确错误分类规则（哪些写入全局、哪些写入局部）

### Phase 4: OpenAI Chat 协议（后端）

8. **Chat handler**
   - `handlers/chat/handler.go`
   - OpenAI Chat → Claude/Gemini 转换器
   - 响应转换器

9. **Chat 渠道管理**
   - `config_chat.go`
   - `handlers/chat/channels.go`
   - Chat 专用 MetricsManager 和 Scheduler

### Phase 5: 前端改造

10. **新增 Chat tab**
    - 路由：`/channels/chat`
    - 导航栏：新增 OpenAI Chat 入口
    - `stores/channel.ts`：新增 chat 分组（对应 `chatUpstream`）与对应的 fetch/save/reorder/status/metrics 调用
    - 复用 `ChannelsView.vue` 和 `ChannelOrchestration.vue`

11. **Credential Store + API**
    - `stores/credential.ts`
    - `services/api.ts` 扩展

12. **密钥池管理弹窗**
    - 密钥池列表 + 编辑弹窗
    - 上游能力探测 UI

13. **AddChannelModal 改造**
    - 保持 `apiType` 由当前 Tab 决定
    - 支持密钥来源：自定义密钥 / 使用密钥池（向后兼容）
    - 使用密钥池时写入 `credentialId`，并展示兼容性（按 `serviceType` + 探测结果）

## 七、向后兼容性

- 现有 `config.json` 无需迁移，`credentials` 字段为 `omitempty`
- 现有渠道的 `apiKeys` 字段保持不变
- `CredentialID` 为空时，行为与当前完全一致
- 现有 API 端点保持不变（新增密钥池与 chat 相关端点）
- 前端保留现有 messages/responses/gemini tab，并新增 chat tab；现有渠道展示与操作习惯不变

## 八、风险点

1. **探测准确性**：某些上游有特殊认证/版本路径规则，探测结果可能误判（需与 URL 拼接/鉴权保持一致）
2. **FailedKeysCache 分类**：全局/局部冷却的错误分类不准会导致过度 failover 或恢复过慢
3. **Key 共享的冷却传播**：全局冷却会跨 `apiType` 传播（这是期望行为，但需要在 UI/文档中解释清楚）
4. **配置文件膨胀**：ProbeResults 记录较多时 config.json 体积增大（已用 keyHash 避免明文重复，但仍需评估规模）
5. **渠道覆盖 BaseURL 的一致性**：覆盖 BaseURL/BaseURLs 会使密钥池探测结果失去“Key+BaseURL”约束；需要在 UI 做显式提示，并在运行期禁用基于 ProbeResults 的过滤
6. **探测并发与上游限流**：批量 key × 多 serviceType 的探测可能触发上游限流或本机连接耗尽；需要并发上限、超时与重试策略
