## [Unreleased]

### 新增

- **渠道级代理（Proxy）支持** - 为每个渠道配置独立的 HTTP/SOCKS5 代理，用于通过代理访问特定上游服务（网络隔离、地域限制等场景）
  - 后端：`UpstreamConfig` 新增 `ProxyURL` 字段，`GetStandardClient`/`GetStreamClient` 支持代理配置，`SendRequest` 传递代理参数并记录脱敏日志
  - 前端：渠道编辑表单新增代理 URL 输入框，`Channel` 接口支持 `proxyUrl` 字段
  - Ping 适配：Messages 和 Gemini 渠道连通性测试均通过渠道代理发送

### 变更

- **Go 依赖升级** - `golang.org/x/net` 升级到 v0.50.0（直接依赖），支持 SOCKS5 代理功能

---

# 版本历史

> **注意**: v2.0.0 开始为 Go 语言重写版本，v1.x 为 TypeScript 版本

---

## [v2.6.16] - 2026-02-18

### 新增

- **渠道日志接口类型标识** - `ChannelLog` 新增 `interfaceType` 字段，记录请求来源接口类型（Messages/Responses/Gemini），前端日志列表以彩色标签展示

---

## [v2.6.15] - 2026-02-14

### 修复

- **Messages API 空响应误判 tool_use/thinking 响应** - `PreflightStreamEvents` 预检测仅通过 `delta.text` 判断内容是否为空，导致纯 tool_use（工具调用）、thinking（思考）、server_tool_use 等非文本 content block 响应被误判为空响应并触发不必要的重试；新增 `hasNonTextContentBlock` 检测，遇到非文本 content block 时立即放行

---

## [v2.6.14] - 2026-02-14

### 新增

- **自动移除 cch= 计费头参数** - Messages API 预处理阶段自动剥离 system 数组中的 `cch=xxx;` 参数，保留 `cc_version`/`cc_entrypoint` 等其他计费信息，避免上游计费混乱
- **前端 CCH 全局开关** - 操作栏新增 CCH 切换按钮，支持热配置切换（默认启用）

---

## [v2.6.13] - 2026-02-13

### 新增

- **全局流量图按模型堆叠面积曲线** - 流量模式改为按实际模型（如 claude-opus-4-6、claude-haiku-4-5）显示堆叠面积曲线，tooltip 显示各模型请求数/失败数，移除冗余的 ModelStatsChart 组件
- **渠道流量图改为堆叠面积图** - Key+Model 分组方式不变，流量模式改为堆叠面积并显示 legend

### 修复

- **渠道 Token/Cache 图多模型时无数据** - 按模型拆分时 Token/Cache 字段未传递，补齐 KeyModelHistoryDataPoint 的 Token 数据
- **渠道 Token/Cache 图多模型 Y 轴刻度不一致** - 所有 Input series 共享左侧 Y 轴，所有 Output series 共享右侧 Y 轴

---

## [v2.6.12] - 2026-02-12

### 新增

- **渠道自定义请求头支持** - 允许为每个渠道配置自定义 HTTP 请求头，在发送请求到上游时附加或覆盖（关闭 [#4](https://github.com/BenedictKing/ccx/issues/4)）

---

## [v2.6.11] - 2026-02-12

### 改进

- **渠道流量图按 Key+Model 组合显示多条曲线** - 当同一 Key 请求多个模型时，每个 Key+Model 组合显示独立曲线，便于分析不同模型的流量分布
- **全局流量图简化为单曲线** - 流量模式只显示请求总量曲线，失败率通过红色背景色带表示（与渠道流量图风格统一）

---

## [v2.6.10] - 2026-02-12

### 改进

- **日志和流量图显示重定向后的实际模型** - 当渠道配置了模型映射时，日志显示 `原始模型 → 实际模型`，流量图按实际使用的模型分组统计

---

## [v2.6.7] - 2026-02-11

### 修复

- **渠道流量条历史最大值增加指数衰减** - 避免历史峰值过后所有柱子都变得很矮，半衰期 5 分钟

### 改进

- **version-bump 技能增加 CHANGELOG 前置检查** - 检查 `[Unreleased]` 区块是否存在及是否包含实际变更内容，无区块时警告并询问是否跳过，有区块但无内容时中止流程，避免发布空版本记录

---

## [v2.6.6] - 2026-02-10

### 新增

- **模型维度使用统计** - 新增按模型分组的请求量和 Token 消耗时间序列统计
  - `RequestRecord` 和 `PersistentRecord` 新增 `Model` 字段，记录每次请求的模型名
  - SQLite schema migration（user_version 0→1）自动添加 `model` 列和索引
  - 新增 `GET /api/{messages|responses|gemini}/models/stats/history` API，返回按模型分组的历史数据点
  - 前端新增 `ModelStatsChart.vue` 多曲线面积图组件，支持请求量/Token 双视图切换
  - 集成到 `App.vue` 全局统计区域，与 GlobalStatsChart 并列展示

- **渠道快速日志** - 新增渠道级别的请求日志查看功能（内存环形缓冲区，每渠道保留最近 50 条）
  - 新增 `ChannelLogStore` 内存环形缓冲区，按 channelIndex 存储，纯内存，重启丢失
  - `TryUpstreamWithAllKeys` 在每次上游尝试后自动采集日志（含状态码、耗时、错误摘要、是否重试）
  - 新增 `GET /api/{messages|responses|gemini}/channels/:id/logs` API
  - 前端新增 `ChannelLogsDialog.vue` 弹窗组件，支持状态码颜色标识、展开错误详情、3 秒自动刷新
  - 渠道操作菜单新增"日志"入口（mdi-history 图标）
  - 渠道卡片新增独立日志按钮，与三点菜单并列，方便快速访问

### 修复

- **ModelStatsChart 并发请求竞态** - 使用请求版本号机制，确保只有最新请求的结果会更新数据，旧请求返回时自动丢弃，避免数据闪回
- **渠道删除后日志串台** - `ChannelLogStore` 新增 `ClearAll` 方法，删除渠道时清空整个日志存储避免索引错位；`DeleteUpstream` handler 传入 `channelScheduler` 参数

---

## [v2.6.5] - 2026-02-08

### 修复

- **OpenAI/Gemini 上游流式响应缺少 `message_start`/`message_stop` 事件** - 修复通过 OpenAI/Gemini 上游代理时，Anthropic SDK 报 `Unexpected event order, got content_block_start before "message_start"` 的问题
  - OpenAI/Gemini provider 的 `HandleStreamResponse` 在协议转换时未生成 Claude Messages API 规范要求的 `message_start` 和 `message_stop` 事件
  - 新增 `buildMessageStartEvent()` 公共函数，在第一个 `content_block_start` 之前自动发送 `message_start`
  - 流结束时统一发送 `message_delta`（含 `stop_reason`）+ `message_stop`
  - 修复 OpenAI provider `finish_reason == "stop"` 时未发送 `message_delta` 的遗漏
  - 新增 `finish_reason == "length"` → `max_tokens` 的映射
  - 涉及文件：`backend-go/internal/providers/openai.go`, `backend-go/internal/providers/gemini.go`

---

## [v2.6.4] - 2026-02-07

### 新增

- **上游空响应自动重试** - 上游返回 HTTP 200 但流式响应内容为空或几乎为空时，自动触发 failover 重试
  - 空响应定义：OutputTokens == 0（完全无输出）或 OutputTokens == 1 且内容仅为 `{`（截断的 JSON 开头）
  - 智能预检测：在发送 HTTP 200 Header 之前缓冲上游事件并检查实际输出内容
  - Messages API：新增 `PreflightStreamEvents()` 预检测函数，延迟 `SetupStreamHeaders()` 调用
  - Responses API：在 `handleStreamSuccess()` 中新增 scanner 预读取逻辑
  - Failover 集成：空响应触发 `ErrEmptyStreamResponse`，标记 Key 失败并计入熔断指标，继续尝试下一个 Key/BaseURL/渠道
  - 预检测超时 30s 保守放行，正常响应延迟约 100-200ms（等到第一个有效 content_block_delta）
  - 涉及文件：`backend-go/internal/handlers/common/stream.go`, `backend-go/internal/handlers/common/upstream_failover.go`, `backend-go/internal/handlers/responses/handler.go`

---

## [v2.6.3] - 2025-02-05

### 变更

- **渠道删除时保留历史指标数据** - 删除渠道时不再主动清理指标数据，让数据自然过期
  - 移除三个渠道删除处理器中的 `DeleteChannelMetrics()` 调用
  - SQLite 数据将在配置的保留期后自动删除（`METRICS_RETENTION_DAYS`，默认 7 天）
  - 内存指标将在 48 小时无活动后自动清理
  - 保持全局历史统计数据完整性，不再因删除渠道而丢失
  - **注意**：若用相同 BaseURL + APIKey 重建渠道，可能继承近期健康状态/统计（受内存清理窗口与服务重启影响，熔断状态不持久化）
  - 涉及文件：`backend-go/internal/handlers/messages/channels.go`, `backend-go/internal/handlers/responses/channels.go`, `backend-go/internal/handlers/gemini/channels.go`

---

## [v2.6.2] - 2026-02-04

### 新增

- **渠道配置复制功能** - 在渠道右侧弹出菜单中新增"复制配置"选项
  - 点击后将渠道的所有 BaseURL 和 API Key 按行分隔复制到系统剪贴板
  - 方便用户分享配置或快速创建新渠道
  - 支持活跃渠道和备用池渠道
  - 涉及文件：`frontend/src/components/ChannelOrchestration.vue`

- **DeleteChannelMetrics 测试覆盖** - 新增共享 MetricsKey 删除场景的单元测试
  - `TestDeleteChannelMetrics_SharedMetricsKeyPreserved` - 验证共享 metricsKey 被保留
  - `TestDeleteChannelMetrics_AllExclusiveKeysDeleted` - 验证独占 metricsKey 全部删除
  - `TestDeleteChannelMetrics_PreconditionWarning` - 验证前置条件违反时的行为
  - 涉及文件：`backend-go/internal/scheduler/channel_scheduler_test.go`

### 修复

- **删除渠道时共享 MetricsKey 数据丢失** - 修复删除渠道时误删其他渠道共享指标数据的问题
  - 问题：当两个渠道使用相同的 (BaseURL, APIKey) 组合时，删除其中一个渠道会导致另一个渠道的统计数据也被清除
  - 原因：Metrics 按 `hash(baseURL + apiKey)` 存储，删除时直接删除 MetricsKey，未检查是否有其他渠道共享
  - 修复：在 `DeleteChannelMetrics()` 中增加共享检测逻辑，只删除不被其他渠道使用的独占 MetricsKey
  - 新增 `collectUsedCombinations()` 辅助方法收集其他渠道的组合
  - 新增 `isUpstreamInConfig()` 前置条件守卫，检测渠道是否仍在配置中
  - 涉及文件：`backend-go/internal/scheduler/channel_scheduler.go`

- **DeleteByMetricsKeys 返回值语义不清晰** - 补充方法注释说明返回值语义
  - 返回持久化存储删除的记录数，未配置存储或删除失败时返回 0
  - 涉及文件：`backend-go/internal/metrics/channel_metrics.go`

- **前端复制配置 timeout 未清理** - 修复组件卸载时 `copyTimeoutId` 未清理的问题
  - 在 `onUnmounted` 钩子中添加 `clearTimeout(copyTimeoutId)` 清理逻辑
  - 涉及文件：`frontend/src/components/ChannelOrchestration.vue`

---

## [v2.6.1] - 2026-02-01

### 修复

- **低质量渠道 message_start 事件 usage 修补** - 修复低质量渠道模式下 `message_start` 事件中虚假 `input_tokens` 未被修补的问题
  - 问题：当 `lowQuality=true` 且 `input_tokens >= 10` 时，`PatchMessageStartInputTokensIfNeeded` 函数会跳过修补，导致虚假值（如 25599）被直接返回
  - 修复：在条件判断中增加 `!lowQuality` 检查，确保低质量渠道始终调用 `PatchTokensInEvent` 进行 5% 偏差检测
  - 涉及文件：`backend-go/internal/handlers/common/stream.go`

- **message_start 事件 output_tokens 误修补** - 修复 `output_tokens=1` 被错误修补为 `0` 的问题
  - 问题：`patchUsageFieldsWithLog` 的常规修补逻辑在 `estimatedOutput=0` 时仍会将 `output_tokens=1` 修补为 `0`
  - 修复：在常规 output_tokens 修补条件中增加 `estimatedOutput > 0` 检查，避免用无效估算值覆盖正常的初始值
  - 涉及文件：`backend-go/internal/handlers/common/stream.go`

---

## [v2.5.13] - 2026-01-31

### 修复

- **Gemini functionDeclaration parameters 类型修复** - 修复 Gemini API 返回 400 错误的问题
  - 问题：当 Claude 工具的 `InputSchema` 为 nil、缺少 `type` 字段或缺少 `properties` 字段时，Gemini API 拒绝请求
  - 新增 `normalizeGeminiParameters()` 辅助函数，确保 parameters schema 符合 Gemini 要求：
    - `parameters` 必须有 `type: "object"` 字段
    - `parameters` 必须有 `properties` 字段（即使为空对象）
  - 涉及文件：`backend-go/internal/providers/gemini.go`

---

## [v2.5.12] - 2026-01-30

### 新增

- **渠道置顶/置底功能** - 在渠道编排菜单中新增一键调整渠道位置的操作
  - 在渠道右侧弹出菜单中添加"置顶"和"置底"选项
  - 第一个渠道不显示"置顶"，最后一个渠道不显示"置底"
  - 操作后立即保存到后端，复用现有 `saveOrder()` 函数
  - 解决渠道数量较多时拖拽排序不便的问题
  - 涉及文件：
    - `frontend/src/components/ChannelOrchestration.vue` - 添加菜单项和处理函数
    - `frontend/src/plugins/vuetify.ts` - 添加 `arrow-collapse-up/down` 图标

- **隐式缓存读取推断** - 当上游未明确返回 `cache_read_input_tokens` 但存在显著 token 差异时，自动推断缓存命中
  - 检测 `message_start` 与 `message_delta` 事件中 `input_tokens` 的差异
  - 触发条件：差额 > 10% 或差额 > 10000 tokens
  - 将差额自动填充到 `CacheReadInputTokens` 字段，使 token 统计更准确
  - **下游转发支持**：推断的 `cache_read_input_tokens` 会写入 `message_delta` 事件并转发给下游客户端
  - 新增 `StreamContext.MessageStartInputTokens` 字段记录初始 token 数
  - 新增 `inferImplicitCacheRead()` 函数在流结束时执行推断
  - 新增 `PatchTokensInEventWithCache()` 函数在修补 token 的同时写入推断的缓存值
  - **关键修复**：
    - `message_start` 的 `input_tokens` 不再累积到 `CollectedUsage.InputTokens`，确保差额计算正确
    - 使用 `originalUsageData` 传递给 `PatchMessageStartInputTokensIfNeeded`，避免误判
    - Token 修补逻辑增加隐式缓存信号检测，避免覆盖缓存命中场景下的正确低值
    - 隐式缓存推断在转发前执行，确保下游客户端能收到推断值
    - 仅当上游事件中不存在 `cache_read_input_tokens` 字段时才写入推断值，避免覆盖上游显式返回的 0 值
  - 涉及文件：
    - `backend-go/internal/handlers/common/stream.go` - 核心逻辑实现
    - `backend-go/internal/handlers/common/stream_test.go` - 单元测试（15 个边界场景）

---

## [v2.5.10] - 2026-01-26

### 新增

- **删除渠道时自动清理指标数据** - 修复删除渠道后内存和 SQLite 指标数据残留问题
  - 扩展 `PersistenceStore` 接口，新增按 `metrics_key` 和 `api_type` 批量删除记录的方法
  - 新增 `MetricsManager.DeleteChannelMetrics()` 方法，支持同时清理内存和持久化数据
  - 新增 `ChannelScheduler.DeleteChannelMetrics()` 统一删除入口
  - 修改 `DeleteUpstream` Handler（Messages/Responses/Gemini），删除后自动调用指标清理
  - SQLite 清理不依赖内存状态，确保即使内存中无数据也能正确清理持久化记录
  - 删除渠道时同时清理历史 Key 的指标数据
  - **按 `api_type` 过滤删除**：避免误删其他接口类型（messages/responses/gemini）的指标数据
  - **分批删除**：每批 500 条，避免触发 SQLite 变量上限（999）导致删除失败
  - **并发安全**：`flushMu` 互斥锁串行化 flush 与 delete；`asyncFlushWg` 确保 Close 前所有异步 flush 完成
  - 涉及文件：
    - `backend-go/internal/metrics/persistence.go` - 接口扩展（新增 apiType 参数）
    - `backend-go/internal/metrics/sqlite_store.go` - 实现 SQLite 删除逻辑（分批 + api_type 过滤）
    - `backend-go/internal/metrics/channel_metrics.go` - 新增删除方法，导出 `GenerateMetricsKey()`
    - `backend-go/internal/scheduler/channel_scheduler.go` - 新增统一删除入口
    - `backend-go/internal/handlers/*/channels.go` - 删除 Handler 改造
    - `backend-go/main.go` - 路由注册更新

- **换 Key 后历史数据累计统计** - 修复更换 API Key 后旧 Key 的历史统计数据丢失问题
  - 新增 `UpstreamConfig.HistoricalAPIKeys` 字段，存储历史 API Key 列表
  - 更新渠道时自动维护历史 Key 列表：被移除的 Key 进入历史列表，恢复的 Key 从历史列表移除
  - `Add*APIKey` / `Remove*APIKey` 接口同样维护历史 Key 列表
  - `ToResponseMultiURL()` 支持聚合历史 Key 指标（只计入总数，不影响实时失败率和熔断判断）
  - 前端查看渠道统计时，总数包含历史 Key 数据，Key 详情列表只显示当前活跃 Key
  - 涉及文件：
    - `backend-go/internal/config/config.go` - 新增 `HistoricalAPIKeys` 字段
    - `backend-go/internal/config/config_utils.go` - `Clone()` 方法深拷贝历史 Key
    - `backend-go/internal/config/config_*.go` - 更新渠道时维护历史 Key 列表
    - `backend-go/internal/metrics/channel_metrics.go` - 聚合逻辑支持历史 Key
    - `backend-go/internal/handlers/channel_metrics_handler.go` - 传入历史 Key 参数
    - `backend-go/internal/handlers/gemini/dashboard.go` - 传入历史 Key 参数

---

## [v2.5.9] - 2026-01-24

### 新增

- **前端模型映射智能选择功能** - 优化模型重定向配置体验，支持自动获取上游模型列表
  - 前端直连上游 `/v1/models` 接口，无需后端代理
  - 目标模型输入框改为 `v-combobox`，点击时自动获取模型列表
  - 为每个 API Key 并行检测 models 接口状态，提高效率
  - 在 API 密钥列表中实时显示状态标签：
    - 成功：绿色标签显示 `models 200 (N 个)`
    - 失败：红色标签显示 `models 错误码`，鼠标悬停显示详细错误消息
    - 加载中：蓝色标签显示 `检测中...`
  - 智能错误解析，支持上游标准错误格式 `{ "error": { "message": "...", "code": "..." } }`
  - 合并所有成功的模型列表并去重，提供完整的模型选项
  - 涉及文件：
    - `frontend/src/services/api.ts` - 新增 `fetchUpstreamModels` 函数和 `buildModelsURL` 工具函数
    - `frontend/src/components/AddChannelModal.vue` - 优化交互体验和状态管理

---

## [v2.5.8] - 2026-01-21

### 修复

- **客户端取消请求误计入失败** - 修复用户主动取消请求被错误计入渠道失败指标的问题
  - 新增 `isClientSideError` 函数，使用 `errors.Is` 正确识别被包装的 `context.Canceled` 错误
  - 仅识别明确的客户端取消（`context.Canceled`），连接故障（`broken pipe`、`connection reset`）继续 failover
  - 统一口径：`SendRequest` 和 `handleSuccess` 路径均应用客户端取消判断
  - 新增 `RecordRequestFinalizeClientCancel` 方法，客户端取消时仅计入总请求数，不计入失败数和失败率
  - 客户端取消不重置 `ConsecutiveFailures`，保留真实的连续失败计数
  - 涉及文件：
    - `backend-go/internal/handlers/common/upstream_failover.go` - 错误类型判断与分流
    - `backend-go/internal/metrics/channel_metrics.go` - 新增客户端取消记录方法
    - `backend-go/internal/handlers/common/client_error_test.go` - 单元测试

- **指标二次计数 Bug** - 修复 `RecordRequestFinalize*` fallback 路径导致的请求计数重复问题
  - 将 `RequestCount++` 从 `RecordRequestConnected` 移至 `RecordRequestFinalize*` 阶段
  - 采用延迟计数策略：连接时预写历史记录，完成时统一计数
  - 确保 fallback 路径（requestID 丢失/索引越界）不会触发二次计数
  - 涉及文件：`backend-go/internal/metrics/channel_metrics.go`

### 重构

- **指标记录架构优化** - 将指标记录职责从 handler 层下沉到 failover 层，实现"连接即计数"的实时统计
  - 新增 `RecordRequestConnected` / `RecordRequestFinalizeSuccess` / `RecordRequestFinalizeFailure` 三阶段记录机制
  - TCP 建连时即计入活跃请求数，响应完成后回写成功/失败与 token 数据
  - 移除 handler 层的 `RecordSuccessWithUsage` / `RecordFailure` 调用，统一由 `upstream_failover.go` 管理
  - 修改 `HandleSuccessFunc` 签名：返回 `(*types.Usage, error)` 而非 `*types.Usage`，支持流式响应错误处理
  - 修改 `ProcessStreamEvents` / `HandleStreamResponse` 返回 usage，避免在 stream 层直接记录指标
  - 新增 `pendingHistoryIdx` 映射表，支持请求 ID 到历史记录索引的快速查找
  - 新增 `cleanupHistoryLocked` 函数，清理过期历史记录时同步修正索引
  - 涉及文件：
    - `backend-go/internal/handlers/common/stream.go` - 移除指标记录，返回 usage
    - `backend-go/internal/handlers/common/upstream_failover.go` - 三阶段指标记录
    - `backend-go/internal/handlers/messages/handler.go` - 移除指标记录调用
    - `backend-go/internal/handlers/responses/handler.go` - 移除指标记录调用
    - `backend-go/internal/handlers/gemini/handler.go` - 移除指标记录调用
    - `backend-go/internal/metrics/channel_metrics.go` - 新增三阶段记录 API

## [v2.5.6] - 2026-01-20

### 修复

- **Gemini CLI 工具调用签名兼容** - 修复多轮工具调用中签名字段位置/命名不一致导致上游返回 400 的问题（启用 `injectDummyThoughtSignature` 时会为缺失签名的 `functionCall` 注入 dummy）。
- **Gemini CLI tools schema 兼容** - 支持 `parametersJsonSchema` 并在转发前清洗不兼容字段（`$schema` / `additionalProperties` / `const`），避免上游 400。
- **Gemini Dashboard stripThoughtSignature 字段缺失** - Dashboard API 补齐 `stripThoughtSignature` 字段，避免配置在刷新后丢失。

- **Gemini 渠道 stripThoughtSignature 字段无法保存** - 修复前端无法正确显示和保存"移除 Thought Signature"配置的问题
  - 修复 `GetUpstreams` 函数返回数据中缺失 `stripThoughtSignature` 字段
  - 修复前端图标显示问题（将 `mdi-signature-freehand` 改为 `mdi-close-circle`）
  - 统一图标和开关颜色为 `error` 红色，与"移除"操作语义一致
  - 涉及文件：
    - `backend-go/internal/handlers/gemini/channels.go` - 添加缺失字段
    - `frontend/src/components/AddChannelModal.vue` - 修复图标和颜色

### 新增

- **Gemini API thought_signature 兼容性方案** - 新增 `stripThoughtSignature` 配置项，支持兼容旧版 Gemini API
  - 新增 `StripThoughtSignature` 配置字段（布尔值），用于移除 `thought_signature` 字段
  - 实现 `stripThoughtSignatures()` 函数，移除所有 functionCall 的 thought_signature 字段
  - 配置优先级：`StripThoughtSignature` > `InjectDummyThoughtSignature`
  - 保持深拷贝机制，避免多渠道 failover 时污染后续请求
  - 前端添加"移除 Thought Signature"开关（仅 Gemini 渠道显示）
  - 涉及文件：
    - `backend-go/internal/config/config.go` - 配置结构定义
    - `backend-go/internal/config/config_gemini.go` - 配置更新逻辑
    - `backend-go/internal/handlers/gemini/handler.go` - 请求处理逻辑
    - `backend-go/internal/handlers/gemini/handler_test.go` - 单元测试
    - `frontend/src/components/AddChannelModal.vue` - 前端开关
    - `frontend/src/services/api.ts` - 类型定义

## [v2.5.5] - 2026-01-19

## [v2.5.4] - 2026-01-19

### 重构

- **Failover 逻辑模块化** - 将多渠道和单上游 failover 逻辑提取到公共模块，大幅减少代码重复
  - 新增 `backend-go/internal/handlers/common/multi_channel_failover.go` - 多渠道 failover 外壳逻辑
  - 新增 `backend-go/internal/handlers/common/upstream_failover.go` - 单上游 Key/BaseURL 轮转逻辑
  - 重构 Messages、Responses、Gemini 三个 handler，使用统一的 failover 函数
  - 代码行数减少：-1253 行，+475 行（净减少 778 行）
  - 涉及文件：
    - `backend-go/internal/handlers/messages/handler.go`
    - `backend-go/internal/handlers/responses/handler.go`
    - `backend-go/internal/handlers/gemini/handler.go`
    - `backend-go/internal/scheduler/channel_scheduler.go`

## [v2.5.3] - 2026-01-19

### 修复

- **Models API 日志标签修正** - 修正 Models API 相关日志标签，确保正确区分 Messages 和 Responses 渠道
  - 修正 `models.go` 中 `tryModelsRequest` 和 `fetchModelsFromChannel` 函数的日志标签
  - 使用动态 `channelType` 变量替代硬编码的 `"Messages"` 字符串
  - 日志标签格式统一为 `[Messages-Models]` 或 `[Responses-Models]`
  - 涉及文件：`backend-go/internal/handlers/messages/models.go`
- **多渠道 failover 客户端取消检测** - 在 failover 循环中添加客户端断开检测，避免客户端已取消请求后继续尝试其他渠道
  - 在每次渠道选择前检查 `c.Request.Context().Done()`
  - 客户端断开时立即返回，不再进行无效的渠道 failover
  - 涉及文件：
    - `backend-go/internal/handlers/gemini/handler.go` - Gemini API 处理器
    - `backend-go/internal/handlers/messages/handler.go` - Messages API 处理器
    - `backend-go/internal/handlers/responses/handler.go` - Responses API 处理器

### 新增

- **响应 model 字段改写可配置化** - 新增环境变量 `REWRITE_RESPONSE_MODEL` 控制是否改写响应中的 model 字段
  - 默认值：`false`（保持上游返回的原始 model）
  - 启用后：当上游返回的 model 与请求的 model 不一致时，自动改写为请求的 model
  - 适用范围：仅影响 Messages API 的流式响应，不影响 Responses API 和 Gemini API
  - 涉及文件：
    - `backend-go/.env.example` - 添加配置说明和默认值
    - `backend-go/internal/config/env.go` - 添加 `RewriteResponseModel` 配置字段
    - `backend-go/internal/handlers/common/stream.go` - 修改 `PatchMessageStartEvent` 函数，仅在配置启用时改写 model 字段

## [v2.5.2] - 2026-01-19

### 新增

- **Gemini thought_signature 可配置化** - 新增渠道级配置开关 `injectDummyThoughtSignature`
  - 新增 `ensureThoughtSignatures` 函数：为所有缺失 `thought_signature` 的 `functionCall` 注入 dummy 值
  - 使用官方推荐的 `skip_thought_signature_validator` 跳过验证
  - **默认关闭**：保持原样，符合官方 Gemini API 标准
  - **用户可开启**：为需要该字段的第三方 API 注入 dummy signature
  - 前端 UI：在 Gemini 渠道编辑界面添加"注入 Dummy Thought Signature"开关
  - 涉及文件：
    - `backend-go/internal/config/config.go` - 添加 `InjectDummyThoughtSignature` 配置字段
    - `backend-go/internal/config/config_gemini.go` - 更新方法支持新字段
    - `backend-go/internal/config/config_messages.go` - 更新方法支持新字段
    - `backend-go/internal/handlers/gemini/handler.go` - 根据配置决定是否调用 `ensureThoughtSignatures`
    - `backend-go/internal/types/gemini.go` - 新增共享常量 `DummyThoughtSignature`
    - `backend-go/internal/converters/gemini_converter.go` - 使用共享常量
    - `frontend/src/services/api.ts` - 添加类型定义
    - `frontend/src/components/AddChannelModal.vue` - 添加配置开关 UI
    - `frontend/src/plugins/vuetify.ts` - 添加 `mdi-signature` 图标映射
  - 配置优化：将 `.ccb_config/` 目录加入 `.gitignore`，避免泄露本机路径等敏感信息

- **codex-review 技能 v2.1.0** - 新增自动暂存新增文件功能，避免 codex 审核时报 P1 错误
  - 新增步骤 2：在审核前自动暂存所有新增文件
  - 使用安全的 `git ls-files -z | while read` 命令，正确处理特殊文件名（空格、换行、以 `-` 开头）
  - 修复空列表问题：当没有新增文件时安全跳过，不会报错
  - 优化元数据：添加 `user-invocable: true` 和 `context: fork` 字段
  - 优化描述：添加触发关键词，移除 `(user)` 后缀
  - 更新完整审核协议：增加 `[PREPARE] Stage Untracked Files` 步骤
  - 创建 Plugin Marketplace 配置：`.claude-plugin/marketplace.json`
  - 创建详细文档：`.claude/skills/codex-review/README.md`
  - 涉及文件：`.claude/skills/codex-review/SKILL.md`, `.claude-plugin/marketplace.json`, `.claude/skills/codex-review/README.md`

### 优化

- **渠道活跃度图表颜色优化** - 状态条柱状图颜色改为显示每个 6 秒段的独立成功率
  - 修改 SVG 渐变定义：为每个柱子单独定义渐变色（`gradient-${channelIndex}-${i}`）
  - 重构 `getActivityBars` 函数：为每个 6 秒时间段计算独立的成功率并分配颜色
  - 颜色规则（7 档分级）：
    - 深红色（0-5%）：极端故障
    - 红色（5-20%）：严重失败
    - 深橙色（20-40%）：高失败率
    - 橙色（40-60%）：中等失败率
    - 黄色（60-80%）：轻微失败
    - 黄绿色（80-95%）：良好
    - 绿色（95-100%）：优秀
  - 效果：用户可以更清晰地看到每个时间段的健康状况，颜色变化更细腻
  - 性能优化：新增 `activityBarsCache` 计算属性缓存柱状图数据，避免重复计算
  - 代码清理：删除未使用的 `activityColorCache` 和 `getActivityColor` 函数
  - 涉及文件：`frontend/src/components/ChannelOrchestration.vue`

- **修复 Dashboard 切换 Tab 时数据闪烁问题** - 将 Dashboard 数据改为按 API 类型独立缓存
  - 重构 `channelStore`：将单一全局 `dashboardMetrics`/`dashboardStats`/`dashboardRecentActivity` 改为按 Tab（messages/responses/gemini）独立缓存的 `dashboardCache` 结构
  - 新增 `currentDashboardMetrics`、`currentDashboardStats`、`currentDashboardRecentActivity` 计算属性，根据当前 Tab 返回对应缓存数据
  - 切换 Tab 时直接显示该 Tab 的缓存数据，避免显示其他 Tab 的旧数据导致闪烁
  - 涉及文件：`frontend/src/stores/channel.ts`、`frontend/src/views/ChannelsView.vue`

### 重构

- **前端系统状态管理重构** - 将 App.vue 中的系统级状态迁移到 SystemStore
  - 新增 `src/stores/system.ts` 系统状态 Store，统一管理系统运行状态、版本信息、Fuzzy 模式加载状态
  - 重构 `src/App.vue`，移除本地系统状态变量（systemStatus、versionInfo、isCheckingVersion、fuzzyModeLoading、fuzzyModeLoadError），改用 SystemStore 统一管理
  - 更新 `src/stores/index.ts`，导出 SystemStore
  - 新增 2 个计算属性：systemStatusText、systemStatusDesc
  - 新增 8 个状态管理方法：setSystemStatus、setVersionInfo、setCurrentVersion、setCheckingVersion、setFuzzyModeLoading、setFuzzyModeLoadError、resetSystemState
  - 优势：
    - 状态集中：所有系统级状态统一管理，避免分散在组件中
    - 代码简化：App.vue 系统状态逻辑更清晰，减少本地状态管理
    - 可复用性：其他组件可直接使用 SystemStore 的系统状态
    - 易维护：系统状态变更集中在 Store 中，便于调试和扩展
  - 涉及文件：`frontend/src/stores/system.ts`、`frontend/src/stores/index.ts`、`frontend/src/App.vue`

- **前端对话框状态管理重构** - 将 App.vue 中的对话框状态迁移到 DialogStore
  - 新增 `src/stores/dialog.ts` 对话框状态 Store，统一管理添加/编辑渠道对话框和添加 API 密钥对话框
  - 重构 `src/App.vue`，移除本地对话框状态变量（showAddChannelModal、showAddKeyModalRef、editingChannel、selectedChannelForKey、newApiKey），改用 DialogStore 统一管理
  - 更新 `src/stores/index.ts`，导出 DialogStore
  - 新增 6 个状态管理方法：openAddChannelModal、openEditChannelModal、closeAddChannelModal、openAddKeyModal、closeAddKeyModal、resetDialogState
  - 优势：
    - 状态集中：所有对话框相关状态统一管理，避免分散在组件中
    - 代码简化：App.vue 对话框逻辑更清晰，减少本地状态管理
    - 可复用性：其他组件可直接使用 DialogStore 的对话框状态
    - 易维护：对话框状态变更集中在 Store 中，便于调试和扩展
  - 涉及文件：`frontend/src/stores/dialog.ts`、`frontend/src/stores/index.ts`、`frontend/src/App.vue`

- **前端偏好设置管理重构** - 将 App.vue 中的用户偏好设置迁移到 PreferencesStore
  - 新增 `src/stores/preferences.ts` 偏好设置 Store，统一管理暗色模式、Fuzzy 模式、全局统计面板状态
  - 重构 `src/App.vue`，移除本地偏好设置变量（darkModePreference、fuzzyModeEnabled、showGlobalStats），改用 PreferencesStore 统一管理
  - 更新 `src/stores/index.ts`，导出 PreferencesStore
  - 支持自动持久化到 localStorage（使用 pinia-plugin-persistedstate）
  - 优势：
    - 状态集中：所有用户偏好设置统一管理，避免分散在组件中
    - 自动持久化：用户设置自动保存到本地存储，刷新页面后保持
    - 代码简化：App.vue 偏好设置逻辑更清晰，减少本地状态管理
    - 可复用性：其他组件可直接使用 PreferencesStore 的偏好设置
  - 涉及文件：`frontend/src/stores/preferences.ts`、`frontend/src/stores/index.ts`、`frontend/src/App.vue`

- **前端认证状态管理重构** - 将 App.vue 中的认证相关状态迁移到 AuthStore
  - 扩展 `src/stores/auth.ts`，新增认证 UI 状态管理（authError、authAttempts、authLockoutTime、isAutoAuthenticating、isInitialized、authLoading、authKeyInput）
  - 重构 `src/App.vue`，移除本地认证状态变量，改用 AuthStore 统一管理
  - 新增 `isAuthLocked` 计算属性，自动判断认证锁定状态
  - 新增 8 个状态管理方法：setAuthError、incrementAuthAttempts、resetAuthAttempts、setAuthLockout、setAutoAuthenticating、setInitialized、setAuthLoading、setAuthKeyInput
  - 优势：
    - 状态集中：所有认证相关状态统一管理，避免分散在组件中
    - 代码简化：App.vue 认证逻辑更清晰，减少本地状态管理
    - 可复用性：其他组件可直接使用 AuthStore 的认证状态
    - 安全性增强：认证失败次数和锁定时间集中管理，便于扩展
  - 涉及文件：`frontend/src/stores/auth.ts`、`frontend/src/App.vue`

- **前端渠道管理逻辑重构** - 将 App.vue 中的渠道管理逻辑提取到 Pinia Store
  - 新增 `src/stores/channel.ts` 渠道状态 Store，统一管理三种 API 类型（Messages/Responses/Gemini）的渠道数据
  - 重构 `src/App.vue`，移除 300+ 行本地状态和业务逻辑，改用 ChannelStore 统一管理
  - 更新 `src/stores/index.ts`，导出 ChannelStore
  - 优势：
    - 代码解耦：App.vue 从 1000+ 行减少到 700+ 行，职责更清晰
    - 状态集中：渠道数据、指标、自动刷新定时器统一管理
    - 可复用性：其他组件可直接使用 ChannelStore，无需通过 props 传递
    - 可测试性：业务逻辑独立于组件，便于单元测试
  - 涉及文件：`frontend/src/stores/channel.ts`、`frontend/src/stores/index.ts`、`frontend/src/App.vue`

- **前端状态管理架构升级** - 引入 Pinia 状态管理库，替代原有的本地状态管理
  - 新增 `pinia` 和 `pinia-plugin-persistedstate` 依赖，实现响应式状态管理和自动持久化
  - 新增 `src/stores/auth.ts` 认证状态 Store，统一管理 API Key 和认证状态
  - 重构 `src/services/api.ts`，从 AuthStore 获取 API Key，移除本地状态管理逻辑
  - 重构 `src/App.vue`，使用 AuthStore 替代 `isAuthenticated` 本地状态，简化认证流程
  - 更新 `src/main.ts`，初始化 Pinia 和持久化插件
  - 配置 `tsconfig.json` 路径别名 `@/*`，支持模块化导入
  - 优势：响应式状态管理、自动持久化、更好的类型推断、代码解耦
  - 涉及文件：`frontend/package.json`、`frontend/src/stores/auth.ts`、`frontend/src/services/api.ts`、`frontend/src/App.vue`、`frontend/src/main.ts`、`frontend/tsconfig.json`

---

## [v2.4.34] - 2026-01-17

### 新增

- **会话管理增强** - 支持 Gemini API 的 `X-Gemini-Api-Privileged-User-Id` 请求头
  - 在 `ExtractConversationID()` 函数中新增对该请求头的支持，用于会话亲和性管理
  - 优先级顺序：Conversation_id > Session_id > X-Gemini-Api-Privileged-User-Id > prompt_cache_key > metadata.user_id
  - 涉及文件：`backend-go/internal/handlers/common/request.go`

### 优化

- **Gemini Dashboard API 性能优化** - 将前端 3 个独立请求合并为 1 个后端统一接口
  - 新增 `/api/gemini/channels/dashboard` 端点，一次性返回 channels、metrics、stats、recentActivity 数据
  - 后端新增 `internal/handlers/gemini/dashboard.go` 处理器，减少网络往返次数
  - 涉及文件：`backend-go/main.go`、`backend-go/internal/handlers/gemini/dashboard.go`

### 重构

- **前端 UI 框架统一** - 移除 Tailwind CSS 和 DaisyUI，完全使用 Vuetify
  - 从 package.json 移除 tailwindcss、daisyui、autoprefixer、postcss 依赖
  - 删除 tailwind.config.js 和 postcss.config.js 配置文件
  - 更新 src/assets/style.css，移除 @tailwind 指令，保留自定义样式
  - 优势：消除多框架样式冲突、减少打包体积、统一设计语言（Material Design）
  - 涉及文件：`frontend/package.json`、`frontend/src/assets/style.css`、`frontend/src/main.ts`

---

## [v2.4.33] - 2026-01-17

### 新增

- **渠道实时活跃度可视化** - 在渠道列表中显示最近 15 分钟的活跃度数据
  - 后端新增 `GetRecentActivityMultiURL()` 方法，按 **6 秒粒度**分段统计请求量、成功/失败数、Token 消耗（共 150 段）
  - **支持多 URL 和多 Key 聚合**：自动聚合渠道所有故障转移 URL 和所有活跃 API Key 的数据，提供完整的渠道活跃度视图
  - Dashboard API 返回 `recentActivity` 字段，包含每个渠道的 150 段活跃度数据
  - 前端渠道行显示 RPM/TPM 指标，**背景波形柱状图**实时反映活跃度变化（整体颜色根据全局失败率着色：绿色=成功率≥80%，橙色=成功率≥50%，红色=成功率<50%）
  - 柱状图每 2 秒自动更新，用户调用 API 后立即看到柱子"跳动"，提供直观的脉冲式活跃度展示
  - 涉及文件：`backend-go/internal/metrics/channel_metrics.go`、`backend-go/internal/handlers/channel_metrics_handler.go`、`frontend/src/components/ChannelOrchestration.vue`、`frontend/src/services/api.ts`、`frontend/src/App.vue`

---

## [v2.4.32] - 2026-01-14

### ✨ 新增

- **Gemini 渠道支持 thinking 模式函数调用签名传递** - `GeminiFunctionCall` 结构体新增 `ThoughtSignature` 字段
  - 用于 thinking 模式下的签名，需原样传回上游
  - 涉及文件：`backend-go/internal/types/gemini.go`

### 🔧 优化

- **Gemini 渠道添加模态框增强** - 扩展服务类型和模型选项
  - 服务类型新增 OpenAI 和 Claude 选项，支持更多上游协议
  - 更新 Gemini 模型列表：新增 gemini-2、gemini-2.5-flash-lite、gemini-2.5-flash-image、TTS 预览模型、gemini-3 系列预览模型
  - 涉及文件：`frontend/src/components/AddChannelModal.vue`

### 🐛 修复

- **修复快速输入解析器冒号分隔导致 URL 被截断的问题** - 增强 `extractTokens()` 函数支持冒号作为分隔符，同时保护 URL 完整性
  - 新增 URL 占位符机制：先提取完整 URL 并替换为占位符，分割后再恢复
  - 支持中文标点分隔符：逗号（，）、分号（；）、冒号（：）
  - 涉及文件：`frontend/src/utils/quickInputParser.ts`

---

## [v2.4.31] - 2026-01-12

### 🐛 修复

- **修复流式工具调用输出稳定性和合并逻辑** - 增强 `stream_synthesizer.go` 的工具调用处理
  - 工具调用输出按 index 排序，避免 map 遍历顺序不稳定导致日志顺序随机
  - 修复 ID 生成错误：`string(rune(index))` 改为 `strconv.Itoa(index)`，避免非 ASCII 字符
  - 合并逻辑增强：仅合并连续 index 的工具调用，防止误合并不相关调用
  - 新增 ID 匹配检查：合并时验证两个 block 的 ID 一致（或其中一个为空）
  - 支持 ID 补全：合并时若 curr 无 ID 但 next 有，自动补全
  - 涉及文件：`backend-go/internal/utils/stream_synthesizer.go`

---

## [v2.4.30] - 2026-01-10

### 🐛 修复

- **修复流式响应工具调用分裂问题** - 当上游返回的工具调用被意外分成两个 content_block 时自动合并
  - 问题场景：第一个 block 有 name 和 id 但参数为空 "{}"，第二个 block 没有 name 但有完整参数
  - 新增 `mergeSplitToolCalls()` 方法检测并合并分裂的工具调用
  - 在 `GetSynthesizedContent()` 中调用，确保日志输出正确的工具调用信息
  - 涉及文件：`backend-go/internal/utils/stream_synthesizer.go`

---

## [v2.4.29] - 2026-01-10

### 🐛 修复

- **修复空 signature 字段导致 Claude API 400 错误** - 客户端可能发送带空 `signature` 字段（空字符串或 null）的请求，Claude API 会拒绝并返回 400 错误
  - 新增 `RemoveEmptySignatures()` 函数，定向移除 `messages[*].content[*].signature` 路径下的空值
  - 使用 `json.Decoder` 保留数字精度，`SetEscapeHTML(false)` 保持原始格式
  - **注意**：当请求体被修改时，JSON 字段顺序可能发生变化（不影响 API 语义）
  - 在 Messages Handler 入口处调用预处理，确保请求发送前清理无效字段
  - 涉及文件：`backend-go/internal/handlers/common/request.go`、`backend-go/internal/handlers/messages/handler.go`

### ✨ 改进

- **增强 Trace 亲和性日志记录** - 在关键操作点添加详细日志，方便排查亲和性相关问题
  - `[Affinity-Set]` 记录新建/变更用户亲和
  - `[Affinity-Remove]` 记录手动移除用户亲和
  - `[Affinity-RemoveByChannel]` 记录渠道移除时批量清理
  - `[Affinity-Cleanup]` 记录定时清理过期记录
  - 日志在锁外执行，避免高负载下的尾延迟
  - 用户 ID 分级脱敏：短 ID 也保留部分字符便于关联
  - 涉及文件：`backend-go/internal/session/trace_affinity.go`

## [v2.4.28] - 2026-01-07

### 🐛 修复

- **修复内容审核错误导致无限重试问题** - 当上游返回 `sensitive_words_detected` 等内容审核错误时，单渠道场景下会无限重试
  - 根因：`classifyByStatusCode(500)` 触发 failover，但未检查 `error.code` 字段中的不可重试错误码
  - 新增 `isNonRetryableErrorCode()` 函数，检测内容审核和无效请求错误码
  - 新增 `isNonRetryableError()` 函数，从响应体提取并检测不可重试错误
  - 在 `shouldRetryWithNextKeyNormal()` 和 `shouldRetryWithNextKeyFuzzy()` 入口处优先检测
  - 不可重试错误码：`sensitive_words_detected`、`content_policy_violation`、`content_filter`、`content_blocked`、`moderation_blocked`、`invalid_request`、`invalid_request_error`、`bad_request`
  - 涉及文件：`backend-go/internal/handlers/common/failover.go`

### 🧪 测试

- **新增不可重试错误码测试** - 覆盖 `sensitive_words_detected` 等错误码在 Normal/Fuzzy 模式下的行为
  - 涉及文件：`backend-go/internal/handlers/common/failover_test.go`

## [v2.4.27] - 2026-01-05

### 🐛 修复

- **修复多端点 failover 渠道统计丢失问题** - 当渠道配置多个 `baseUrls` 时，请求路由到非主 URL 后指标无法正确聚合到渠道统计
  - 根因：指标存储使用 `hash(baseURL + apiKey)` 作为键，但查询方法只使用主 BaseURL
  - 新增 4 个多 URL 聚合方法：`GetHistoricalStatsMultiURL`、`GetChannelKeyUsageInfoMultiURL`、`GetKeyHistoricalStatsMultiURL`、`calculateAggregatedTimeWindowsMultiURL`
  - `ToResponseMultiURL` 按 API Key 去重聚合，避免同一 Key 在多 URL 场景下产生重复条目
  - Handler 层全部改用 `upstream.GetAllBaseURLs()` 获取所有 URL 进行聚合
  - 涉及文件：`backend-go/internal/metrics/channel_metrics.go`、`backend-go/internal/handlers/channel_metrics_handler.go`

## [v2.4.26] - 2026-01-05

### 🐛 修复

- **修复 Key 趋势图切换时间范围后不刷新问题** - 持久化 view/duration 选择到 localStorage，使用 requestId 防止自动刷新旧响应覆盖新选择
  - 涉及文件：`frontend/src/components/KeyTrendChart.vue`

- **修复 KeyTrendChart SSR 兼容性和健壮性问题**
  - 添加 `isLocalStorageAvailable()` 检查，防止 SSR 环境下访问 localStorage 崩溃
  - 为 localStorage 读写操作添加 try/catch 异常捕获（配额超限、隐私模式等场景）
  - 添加 `channelType` prop 变化监听，切换渠道类型时自动重载偏好设置并刷新数据
  - 优化 channelType watcher 逻辑，避免与 duration watcher 重复触发刷新
  - 涉及文件：`frontend/src/components/KeyTrendChart.vue`

- **修复缓存创建统计缺失问题** - 当上游仅返回 TTL 细分字段（5m/1h）时，兜底汇总为 cacheCreationTokens
  - 涉及文件：`backend-go/internal/metrics/channel_metrics.go`

- **透传缓存 TTL 细分字段到指标层** - Responses 非流式/流式 usage 现在包含 CacheCreation5m/1h + CacheTTL
  - 涉及文件：`backend-go/internal/handlers/responses/handler.go`

### 🧪 测试

- **新增 TTL 细分字段兜底测试** - 覆盖 cache_creation_input_tokens 为 0 时的汇总场景
  - 涉及文件：`backend-go/internal/metrics/channel_metrics_cache_stats_test.go`

## [v2.4.25] - 2026-01-04

### 🧪 测试

- **新增 baseUrl/baseUrls 一致性测试套件** - 覆盖 URL 配置的完整场景，防止编辑渠道时数据不一致问题回归
  - `TestUpdateUpstream_BaseURLConsistency`: 验证 Messages 渠道更新时 baseUrl/baseUrls 的一致性（4 场景）
  - `TestUpdateResponsesUpstream_BaseURLConsistency`: 验证 Responses 渠道更新一致性
  - `TestUpdateGeminiUpstream_BaseURLConsistency`: 验证 Gemini 渠道更新一致性
  - `TestGetAllBaseURLs_Priority`: 验证 URL 获取优先级逻辑（4 场景）
  - `TestGetEffectiveBaseURL_Priority`: 验证有效 URL 选择逻辑（3 场景）
  - `TestDeduplicateBaseURLs`: 验证 URL 去重逻辑（7 场景，含末尾斜杠/井号差异）
  - `TestAddUpstream_BaseURLDeduplication`: 验证添加渠道时的 URL 去重
  - 涉及文件：`internal/config/config_baseurl_test.go`（新增 414 行）

### 🐛 修复

- **修复历史分桶边界导致边界点漏算** - 历史统计 API 的时间过滤条件从开区间 `(startTime, endTime)` 改为半开区间 `[startTime, endTime)`，避免恰好落在 startTime 的记录被遗漏
  - 涉及文件：`internal/metrics/channel_metrics.go`

- **修复历史图表时间戳错位** - 将返回的 Timestamp 从"桶结束时间"改为"桶起始时间"，前端图表不再出现一格偏差
  - 涉及文件：`internal/metrics/channel_metrics.go`

- **修复成功计数可能重复记录** - 移除多渠道/单渠道成功路径上多余的 `RecordSuccess()` 调用，统一使用 `RecordSuccessWithUsage()` 作为唯一成功计数入口
  - Messages 路径：移除重复调用，保留流式/非流式末尾的 `RecordSuccessWithUsage`
  - Responses compact 路径：改用 `RecordSuccessWithUsage(nil)` 替代原 `RecordSuccess`，保持指标一致性
  - 涉及文件：`internal/handlers/messages/handler.go`、`internal/handlers/responses/compact.go`

- **修复多 BaseURL 故障转移时成功指标归属错误** - 当请求通过 fallback BaseURL 成功时，成功指标错误地记录到主 BaseURL 而非实际成功的 URL
  - 根本原因：`handleNormalResponse` 和 `HandleStreamResponse` 接收的是原始 `upstream` 而非设置了 `currentBaseURL` 的 `upstreamCopy`
  - 修复方式：将两处调用点的参数从 `upstream` 改为 `upstreamCopy`
  - 影响范围：多渠道/单渠道的流式与非流式响应处理
  - 涉及文件：`internal/handlers/messages/handler.go`

---

## [v2.4.24] - 2026-01-04

### ✨ 新功能

- **缓存命中率统计** - 按 Token 口径展示各渠道缓存读/写与命中率：
  - 后端：在 `timeWindows` 聚合统计中新增 `inputTokens`/`outputTokens`/`cacheCreationTokens`/`cacheReadTokens`/`cacheHitRate` 字段
  - 命中率定义：`cacheReadTokens / (cacheReadTokens + inputTokens) * 100`
  - 前端：渠道编排列表在 15 分钟有请求时额外显示缓存命中率，tooltip 中按 15m/1h/6h/24h 展示缓存统计
  - 新字段均为 `omitempty`，向后兼容

### 🎨 优化

- **调整渠道指标显示间距** - 优化缓存命中率 chip 与请求数之间的间距，避免布局拥挤

---

## [v2.4.23] - 2026-01-03

### ✨ 新功能

- **lowQuality 模式输出完整的 token 验证过程日志** - 启用低质量渠道时，日志会显示完整的验证过程：
  - 偏差 > 5% 时显示修补详情
  - 偏差 ≤ 5% 时显示保留上游值
  - 上游返回无效值时显示本地估算值

### 🐛 修复

- **修复渠道列表 API 未返回 `lowQuality` 字段** - 在 `GetUpstreams` 和 `GetChannelDashboard` 函数返回的 JSON 中补充 `lowQuality` 字段：
  - 之前前端编辑渠道时无法正确显示已保存的"低质量渠道"开关状态
  - 涉及文件：`handlers/messages/channels.go`、`handlers/responses/channels.go`、`handlers/gemini/channels.go`、`handlers/channel_metrics_handler.go`

---

## [v2.4.22] - 2026-01-02

### ✨ 新功能

- **低质量渠道处理机制** - 新增 `lowQuality` 渠道配置选项，用于处理返回不完整数据的上游渠道：
  - Token 偏差检测：启用后对比上游返回值与本地估算值，偏差 > 5% 时使用本地估算值
  - Model 一致性检查：验证响应中的 model 是否与请求一致，不一致则改写为请求的 model
  - 空 ID 补全：自动补全上游返回的空 `message.id`（生成 `msg_<uuid>` 格式）
  - 前端支持：渠道编辑 modal 新增"低质量渠道"开关

### 🐛 修复

- **暂停渠道时自动清除促销期** - 当用户暂停一个正在抢优先级的渠道时，自动清除其 `promotionUntil` 字段：
  - 避免暂停后仍显示促销期标识
  - 涉及三个渠道类型：Messages、Responses、Gemini
  - 涉及文件：`config_messages.go`、`config_responses.go`、`config_gemini.go`

- **修复 `lowQuality` 字段更新不持久化的问题** - 在 `UpdateUpstream` 系列函数中补充 `LowQuality` 字段处理：
  - 之前前端切换"低质量渠道"开关后变更不会被保存
  - 涉及文件：`config_messages.go`、`config_responses.go`、`config_gemini.go`

- **修复渠道列表 API 未返回 `lowQuality` 字段** - 在 `GetUpstreams` 和 `GetChannelDashboard` 函数返回的 JSON 中补充 `lowQuality` 字段：
  - 之前前端编辑渠道时无法正确显示已保存的"低质量渠道"开关状态
  - 涉及文件：`handlers/messages/channels.go`、`handlers/responses/channels.go`、`handlers/gemini/channels.go`、`handlers/channel_metrics_handler.go`

---

## [v2.4.21] - 2026-01-02

### 🐛 修复

- **修复流式响应 input_tokens 为 nil 时丢失的问题** - 当上游返回的顶层 usage 中 `input_tokens` 为 `nil` 时，之前从 `message.usage` 收集到的有效值无法被修补：
  - 原因：`patchUsageFieldsWithLog` 和 `checkUsageFieldsWithPatch` 函数中类型断言 `.(float64)` 失败时跳过了修补逻辑
  - 表现：日志显示 `InputTokens=<nil>` 而非之前收集到的有效值（如 10920）
  - 修复：在两处函数中新增 `input_tokens == nil` 检测，无论是否有缓存 token 都用收集到的值修补
  - 涉及文件：`backend-go/internal/handlers/common/stream.go`

---

## [v2.4.18] - 2025-12-31

### 🐛 修复

- **Gemini 日志和 Header 透传改进** - 修复 Gemini 接口的日志显示和请求头处理：
  - 修复 `contents`/`parts` 字段在日志中不显示的问题
  - 修复原生 Gemini handler 未透传客户端 Header 的问题
  - 新增 `compactGeminiContentsArray` 和 `compactGeminiPart` 函数
  - 涉及文件：`backend-go/internal/utils/json.go`、`backend-go/internal/handlers/gemini/handler.go`

### 🔧 重构

- **Gemini tools 日志简化支持** - 新增 `extractToolNames` 函数支持 Gemini 格式的工具提取：
  - 支持 Gemini `functionDeclarations` 数组格式
  - 兼容 Claude 和 OpenAI 格式
  - 日志中 tools 字段现在统一显示为 `["tool1", "tool2", ...]` 格式
  - 涉及文件：`backend-go/internal/utils/json.go`

- **移除非标准 Gemini API 路由** - 简化 API 端点，仅保留官方格式：
  - 移除：`POST /v1/models/{model}:generateContent`（非标准简化格式）
  - 保留：`POST /v1beta/models/{model}:generateContent`（Gemini 官方格式）
  - 更新前端预览 URL 显示完整路径格式 `/models/{model}:generateContent`
  - 涉及文件：`backend-go/main.go`、`frontend/src/components/AddChannelModal.vue`

---

## [v2.4.17] - 2025-12-30

### 🐛 修复

- **修复 ModelMapping 导致请求字段丢失** - 解决使用模型重定向时 Claude API 返回 403 的问题：
  - 原因：`ClaudeRequest` 结构体缺少 `metadata` 字段，JSON 反序列化时该字段被丢弃
  - 表现：配置 `modelMapping` 后请求被上游拒绝（如 `opus` → `claude-opus-4-5-20251101`）
  - 修复：在 `ClaudeRequest` 中添加 `Metadata map[string]interface{}` 字段
  - 涉及文件：`backend-go/internal/types/types.go`

---

## [v2.4.16] - 2025-12-30

### 🐛 修复

- **修复 Gemini 渠道预期请求 URL 预览** - 创建渠道时预览显示正确的 `/v1beta` 路径：
  - 原问题：Gemini 渠道预览错误显示 `/v1` 而后端实际使用 `/v1beta`
  - 修复：当 serviceType 为 gemini 时使用 `/v1beta` 作为版本前缀
  - 涉及文件：`frontend/src/components/AddChannelModal.vue`

---

## [v2.4.15] - 2025-12-30

### 🐛 修复

- **修复 Gemini API 路由注册失败** - 解决 Gin 框架路由 panic 问题：
  - 原因：Gin 不支持 `:param\:literal` 格式，即使转义冒号也会被解析为两个通配符
  - 方案：使用 `*modelAction` 通配符捕获 `model:action` 整体，在 handler 内解析
  - 涉及文件：`main.go`、`internal/handlers/gemini/handler.go`

### ✨ 新功能

- **Gemini 历史指标 API 完整实现** - 补全 Gemini 模块的历史数据端点：
  - `GET /api/gemini/channels/metrics/history` - 渠道级别指标历史
  - `GET /api/gemini/channels/:id/keys/metrics/history` - Key 级别指标历史
  - `GET /api/gemini/global/stats/history` - 全局统计历史
  - 涉及文件：`internal/handlers/channel_metrics_handler.go`、`main.go`

- **Gemini 前端管理界面完整实现** - 与 Messages/Responses 功能完全对齐：
  - 新增 Gemini Tab 切换，支持完整渠道 CRUD、Key 管理、状态/促销设置
  - KeyTrendChart 和 GlobalStatsChart 组件支持 Gemini 数据展示（移除降级显示）
  - 涉及文件：`frontend/src/App.vue`、`frontend/src/components/`、`frontend/src/services/api.ts`

---

## [v2.4.14] - 2025-12-29

### ✨ 新功能

- **新增 Gemini API 模块** - 与 `/v1/messages`、`/v1/responses` 同级的完整 Gemini 代理支持：
  - **代理端点**：`POST /v1/models/{model}:generateContent`（非流式）、`:streamGenerateContent`（流式）
  - **协议转换**：支持 Gemini 请求转发到 Claude/OpenAI/Gemini 上游，双向转换器自动处理格式差异
  - **渠道管理 API**：完整 CRUD、API Key 管理、状态/促销设置、指标监控（`/api/gemini/channels/*`）
  - **多渠道调度**：集成 ChannelScheduler，支持优先级、熔断、Trace 亲和性
  - **认证方式**：兼容 Gemini 原生格式（`x-goog-api-key` 头、`?key=` 参数）
  - 涉及文件：`internal/handlers/gemini/`、`internal/converters/gemini_converter.go`、`internal/types/gemini.go`

### 🔧 重构

- **config 包模块化拆分** - 将 1973 行的单文件拆分为 6 个职责清晰的模块：
  - `config.go`（297 行）：核心类型定义 + 共享方法
  - `config_loader.go`（384 行）：配置加载、迁移、验证、文件监听
  - `config_messages.go`（429 行）：Messages 渠道 CRUD
  - `config_responses.go`（380 行）：Responses 渠道 CRUD
  - `config_gemini.go`（361 行）：Gemini 渠道 CRUD
  - `config_utils.go`（183 行）：工具函数（去重、模型重定向、状态辅助）
  - 遵循单一职责原则，提升代码可维护性

---

## [v2.4.12] - 2025-12-29

### 🐛 修复

- **修复 Responses API 错误消息提取失败的问题** - 解决 upstream_error 字段无法被正确解析：
  - 扩展 `classifyByErrorMessage` 函数：支持多个消息字段（`message`, `upstream_error`, `detail`）
  - 支持嵌套对象格式：当 `upstream_error` 为对象时，提取其中的 `message` 字段
  - 之前仅检查 `error.message` 字段，导致 `{type, upstream_error}` 格式的错误无法被识别
  - 新增 4 个测试用例覆盖 upstream_error 字符串、嵌套对象、detail 字段等场景
  - 涉及文件：`internal/handlers/common/failover.go`, `internal/handlers/common/failover_test.go`

---

## [v2.4.11] - 2025-12-29

### 🐛 修复

- **修复 Fuzzy 模式下 403 + 预扣费消息未触发 Key 降级的问题** - 补充 v2.4.10 修复的遗漏场景：
  - 修改 `shouldRetryWithNextKeyFuzzy` 函数：新增 `bodyBytes` 参数，对非 402/429 状态码检查消息体中的配额关键词
  - 之前 Fuzzy 模式仅检查状态码（402/429 = quota），不解析消息体，导致 403 + "预扣费额度失败" 返回 `isQuotaRelated=false`
  - 新增 `TestShouldRetryWithNextKey_FuzzyMode_403WithQuotaMessage` 测试用例
  - 涉及文件：`internal/handlers/common/failover.go`, `internal/handlers/common/failover_test.go`

### 🔧 调试

- **添加 Key 降级调试日志** - 用于追踪 `isQuotaRelated` 值和密钥降级流程：
  - 在 `ShouldRetryWithNextKey` 调用后记录返回值（statusCode, shouldFailover, isQuotaRelated）
  - 在密钥标记为配额相关失败时记录日志
  - 涉及文件：`internal/handlers/messages/handler.go`
- **改进 .env.example 文档** - 添加日志配置默认值说明（默认启用，需显式设置 false 禁用）

---

## [v2.4.10] - 2025-12-29

### 🐛 修复

- **修复 403 预扣费额度不足的 Key 未被自动降级的问题** - 解决配额不足的密钥始终被优先尝试：
  - 修改 `shouldRetryWithNextKeyNormal` 逻辑：即使 HTTP 状态码已触发 failover，仍检查消息体确定是否为配额相关错误
  - 之前 403 状态码直接返回 `isQuotaRelated=false`，跳过消息体解析，导致 `DeprioritizeAPIKey` 未被调用
  - 新增 "预扣费" 关键词到 `quotaKeywords` 列表，确保匹配中文预扣费错误消息
  - 涉及文件：`internal/handlers/common/failover.go`

---

## [v2.4.9] - 2025-12-27

### 🔧 改进

- **重构 URL 预热机制为非阻塞动态排序** - 解决首次请求延迟 500ms+ 的问题：
  - 移除阻塞式 ping 预热（`URLWarmupManager`），改用非阻塞的 `URLManager`
  - 新排序策略：基于实际请求结果动态调整 URL 顺序
    - 请求成功：重置失败计数，URL 保持/提升位置
    - 请求失败：增加失败计数，URL 移到末尾
    - 冷却期机制：失败的 URL 在 30 秒后自动恢复可用
  - 排序规则：无失败记录优先 > 冷却期已过 > 仍在冷却期
  - 涉及文件：`warmup/url_manager.go`（新建）、`warmup/url_warmup.go`（删除）、`scheduler/channel_scheduler.go`、`messages/handler.go`、`responses/handler.go`、`main.go`

---

## [v2.4.8] - 2025-12-27

### 🐛 修复

- **修复多端点渠道密钥轮换时的并发竞争问题** - 解决高并发下 BaseURL 被错误修改导致密钥跨渠道混用：
  - 新增 `UpstreamConfig.Clone()` 深拷贝方法，避免并发修改共享对象
  - Messages/Responses Handler 改用深拷贝替代临时修改模式
  - 新增 `MarkWarmupURLFailed()` 方法，请求失败时触发预热缓存失效
  - HTTP 5xx 和网络超时均会触发预热缓存失效，确保失败端点被重新排序
  - 涉及文件：`config/config.go`、`messages/handler.go`、`responses/handler.go`、`scheduler/channel_scheduler.go`、`warmup/url_warmup.go`

---

## [v2.4.6] - 2025-12-27

### ✨ 新功能

- **多端点预热排序** - 渠道首次访问前自动 ping 所有端点，按延迟排序：
  - 新增 `internal/warmup/url_warmup.go` 预热管理器模块
  - 渠道首次访问时自动并发 ping 所有 BaseURL
  - 排序策略：成功的端点优先，同类型按延迟从低到高排序
  - ping 结果缓存 5 分钟，避免频繁测试
  - 支持并发安全的预热请求去重（多个请求同时触发时只执行一次预热）
  - Messages 和 Responses API 均支持预热排序

---

## [v2.4.5] - 2025-12-27

### 🔧 改进

- **统一日志前缀规范** - Messages 和 Responses 接口日志标签标准化：
  - Messages 流式处理日志统一使用 `[Messages-Stream]`、`[Messages-Stream-Token]` 前缀
  - Responses 流式处理日志保持 `[Responses-Stream]`、`[Responses-Stream-Token]` 前缀
  - 修复 3 处遗漏前缀的错误日志（`messages/handler.go`、`responses/handler.go`）
  - 更新 `backend-go/CLAUDE.md` 日志规范文档

---

## [v2.4.4] - 2025-12-27

### ✨ 新功能

- **全局流量和 Token 统计图表** - 新增全局统计可视化功能：
  - 后端新增 `/api/messages/global/stats/history` 和 `/api/responses/global/stats/history` API
  - 支持请求数量（成功/失败/总量）和 Token 总量（输入/输出）统计
  - 前端新增 `GlobalStatsChart.vue` 组件，支持流量/Token 双视图切换
  - 时间范围支持 1h / 6h / 24h / 今日 多档位切换
  - 用户偏好（时间范围、视图模式）按 Messages/Responses 分别保存到 localStorage
  - 以顶部可折叠卡片形式展示，随当前 Tab 自动切换对应 API 类型的统计

- **渠道 Key 趋势图表支持"今日"** - KeyTrendChart 新增今日时间范围选项：
  - 后端 `GetChannelKeyMetricsHistory` 支持 `duration=today` 参数
  - 前端添加"今日"按钮，动态计算从今日 0 点到当前的时长

---

## [v2.4.3] - 2025-12-27

### 🐛 修复

- **Responses API Token 统计修复** - 解决上游无 usage 时本地统计无数据的问题：
  - 修复 SSE 事件解析格式兼容性：支持 `data:` 和 `data: ` 两种格式（某些上游不带空格）
  - 修复 `handleSuccess` / `handleStreamSuccess` 不返回 usage 数据的问题
  - 修复调用点使用 `RecordSuccess` 而非 `RecordSuccessWithUsage` 导致 token 统计未入库
  - 涉及函数：`checkResponsesEventUsage`、`injectResponsesUsageToCompletedEvent`、`patchResponsesCompletedEventUsage`、`tryChannelWithAllKeys`

---

## [v2.4.2] - 2025-12-26

### 🐛 修复

- **原始请求日志修复** - 修复多渠道模式下原始请求头/请求体日志不显示的问题：
  - 将 `LogOriginalRequest` 调用移至 Handler 入口处，确保无论单/多渠道模式都只记录一次
  - 移除单渠道处理函数中重复的日志调用和未使用变量
  - 同时修复 Messages 和 Responses 两个处理器

### 🧹 清理

- **移除废弃环境变量 `LOAD_BALANCE_STRATEGY`** - 负载均衡策略已迁移至 config.json 热重载配置：
  - 删除 `env.go` 中 `LoadBalanceStrategy` 字段
  - 更新 `.env.example`、`docker-compose.yml`、`README.md` 移除相关配置
  - 更新 `CLAUDE.md` 添加配置方式说明

---

## [v2.4.0] - 2025-12-26

### ✨ 改进

- **渠道编辑表单优化** - 改进 AddChannelModal 用户体验：
  - 预期请求支持显示所有 BaseURL 端点，而非仅显示首个
  - 修复 Gemini 类型渠道预期请求显示错误端点的问题（应为 `/generateContent`）
  - 修复从快速模式切换到详细模式时 BaseURL 输入框为空的问题
  - 表单字段重排：TLS 验证开关和描述字段移至表单末尾
  - BaseURL 输入框不再自动修改用户输入，仅在提交时进行去重处理
  - 调整预期请求区域下方间距，改善视觉效果

- **API Key/BaseURL 策略简化** - 移除过度设计，采用纯 failover 模式：
  - 删除 `ResourceAffinityManager` 及相关代码（资源亲和性）
  - 移除 API Key 策略选择（round-robin/random/failover），始终使用优先级顺序
  - 移除 BaseURL 策略选择，始终使用优先级顺序并在失败时切换
  - 前端删除策略选择器，简化渠道配置界面
  - 保留渠道级 Trace 亲和性（TraceAffinityManager）用于会话一致性
  - 清理遗留无用代码：`requestCount`/`responsesRequestCount` 字段、`EnableStreamEventDedup` 环境变量

### 🐛 修复

- **多 BaseURL failover 失效** - 修复当所有 API Key 在首个 BaseURL 失败后不会切换到下一个 BaseURL 的问题：
  - 重构 `tryChannelWithAllKeys` 函数，采用嵌套循环遍历所有 BaseURL
  - 重构 `handleSingleChannel` 函数，单渠道模式也支持多 BaseURL failover
  - 每个 BaseURL 尝试所有 Key 后，若全部失败则自动切换下一个
  - 每次切换 BaseURL 时重置失败 Key 列表
  - 同时修复 Messages 和 Responses 两个处理器
  - 修复 `GetEffectiveBaseURL()` 优先级：临时设置的 `BaseURL` 字段优先于 `BaseURLs` 数组
  - 移除废弃代码：`MarkBaseURLFailed()`、`baseURLIndex` 字段

- **SSE 流式事件完整性** - 修复 Claude Provider 流式响应可能在事件边界处截断的问题：
  - 改用事件缓冲机制，按空行分隔完整 SSE 事件后再转发
  - 确保 `event:`/`data:`/`id:`/`retry:` 等字段作为整体发送
  - 处理上游未以空行结尾的边界情况

- **前端延迟测试结果被覆盖** - 修复 ping 延迟值显示几秒后消失的问题：
  - 新增 `mergeChannelsWithLocalData()` 函数保留本地延迟测试结果
  - 应用于自动刷新、Tab 切换、手动刷新三处数据更新点
  - 添加 5 分钟有效期检查，确保过期数据自动清除

---

## [v2.3.11] - 2025-12-26

### 🐛 修复

- **Responses API usage 字段缺失** - 修复当上游服务（OpenAI/Gemini）不返回 usage 信息时，`response.completed` 事件完全不包含 `usage` 字段的问题：
  - 转换器现在始终生成基础 `usage` 字段（`input_tokens`、`output_tokens`、`total_tokens`），即使值为 0
  - Handler 检测到 usage 存在后，会用本地 token 估算值替换 0 值
  - 确保下游客户端始终能获得合理的 token 使用估算

### ✨ 新功能

- **API Key/Base URL 去重** - 前后端全链路自动去重：
  - 前端详细表单模式输入时自动过滤重复 URL（忽略末尾 `/` 和 `#` 差异）
  - 后端 AddUpstream/UpdateUpstream 接口添加去重逻辑
  - 同时覆盖 Messages 和 Responses 渠道

### 🔧 改进

- **API Key 策略推荐调整** - 将默认推荐策略从"轮询"改为"故障转移"，更符合实际使用场景
- **延迟测试结果持久显示** - 优化渠道延迟测试体验：
  - 测试结果直接显示在故障转移序列列表中，不再使用短暂 Toast 通知
  - 延迟结果保持显示 5 分钟后自动清除
  - 支持单个渠道测试和批量测试统一行为

---

## [v2.3.10] - 2025-12-25

### ✨ 新功能

- **快速添加支持等号分割** - 输入 `KEY=value` 格式时自动按等号分割，识别 `value` 为 API Key
- **快速添加支持多 Base URL** - 自动识别输入中所有 HTTP 链接作为 Base URL（最多 10 个）
- **多 URL 预期请求展示** - 快速添加模式下逐一展示每个 URL 的预期请求地址

---

## [v2.3.9] - 2025-12-25

### ✨ 新功能

- **渠道级 API Key 策略** - 每个渠道可独立配置 API Key 分配策略：
  - `round-robin`（默认）：轮询分发请求到不同 Key
  - `random`：随机选择 Key
  - `failover`：故障转移，优先使用第一个 Key
  - 单 Key 时自动强制使用 `failover`，UI 显示禁用状态
- **多 BaseURL 支持** - 单个渠道可配置多个 BaseURL，支持三种策略：
  - `round-robin`（默认）：轮询分发请求，自动分散负载
  - `random`：随机选择 URL
  - `failover`：手动故障转移（需配合外部监控切换）
- **促销期状态展示** - 渠道列表显示正在"抢优先级"的渠道，带火箭图标和剩余时间
- **延迟测试优化** - 批量测试时直接在列表显示每个渠道的延迟值，颜色根据延迟等级变化（绿/黄/红）
- **多 URL 延迟测试** - 当渠道配置多个 BaseURL 时，并发测试所有 URL 并显示最快的延迟
- **资源亲和性** - 记录用户成功使用的 BaseURL 和 API Key 索引，后续请求优先使用相同资源组合，减少不必要的资源切换

---

## [v2.3.8] - 2025-12-24

### 🔨 重构

- **日志输出规范化** - 移除所有 emoji 符号，统一使用 `[Component-Action]` 标签格式，确保跨平台兼容性

---

## [v2.3.7] - 2025-12-24

### 🐛 修复

- **滑动窗口重建逻辑优化** - 服务重启时只从最近 15 分钟的历史记录重建滑动窗口，避免历史失败记录导致渠道长期处于不健康状态

---

## [v2.3.6] - 2025-12-24

### ✨ 新功能

- **快速添加渠道 - API Key 识别增强** - 大幅改进 `quickInputParser` 的密钥识别能力
  - 新增各平台特定格式支持：OpenAI (sk-/sk-proj-)、Anthropic (sk-ant-api03-)、Google Gemini (AIza)、OpenRouter (sk-or-v1-)、Hugging Face (hf_)、Groq (gsk_)、Perplexity (pplx-)、Replicate (r8_)、智谱 AI (id.secret)、火山引擎 (UUID/AK)
  - 新增宽松兜底规则：常见前缀 (sk/api/key/ut/hf/gsk/cr/ms/r8/pplx) + 任意后缀，支持识别短密钥如 `sk-111`
  - 新增配置键名排除：全大写下划线分隔格式 (如 `API_TIMEOUT_MS`) 不再被误识别为密钥

### 🐛 修复

- **Claude Code settings.json 解析修复** - 粘贴 Claude Code 配置时，不再将键名 (`ANTHROPIC_AUTH_TOKEN` 等) 误识别为 API 密钥

---

## [v2.3.5] - 2025-12-24

### ✨ 新功能

- **Responses API Token 统计补全** - 为 Responses 接口添加完整的输入输出 Token 统计功能
  - 非流式响应：自动检测上游是否返回 usage，无 usage 时本地估算，修补虚假值（`input_tokens/output_tokens <= 1`）
  - 流式响应：累积收集流事件中的文本内容，在 `response.completed` 事件中检测并修补 Token 统计
  - 新增 `EstimateResponsesRequestTokens`、`EstimateResponsesOutputTokens` 专用估算函数
  - 支持缓存 Token 细分统计（5m/1h TTL）
  - 与 Messages API 保持一致的处理逻辑

### 🐛 修复

- **缓存 Token 5m/1h 字段检测完善** - 修复缓存 Token 检测逻辑，同时检测 `cache_creation_5m_input_tokens` 和 `cache_creation_1h_input_tokens` 字段
- **类型化 ResponsesItem 处理** - `EstimateResponsesOutputTokens` 现支持直接处理 `[]types.ResponsesItem` 类型
- **total_tokens 零值补全** - 修复当上游返回有效 `input_tokens/output_tokens` 但 `total_tokens` 为 0 时未自动补全的问题（非流式和流式均已修复）
- **特殊类型 Token 估算回退** - 当 `ResponsesItem` 的 `Type` 为 `function_call`、`reasoning` 等特殊类型时，自动序列化整个结构进行估算
- **流式 delta 类型扩展** - `extractResponsesTextFromEvent` 现支持更多 delta 事件类型：`output_json.delta`、`content_part.delta`、`audio.delta`、`audio_transcript.delta`
- **流式缓冲区内存保护** - `outputTextBuffer` 添加 1MB 大小上限，防止长流式响应导致内存溢出
- **Claude/OpenAI 缓存格式区分** - 新增 `HasClaudeCache` 标志，正确区分 Claude 原生缓存字段（`cache_creation/read_input_tokens`）和 OpenAI 格式（`input_tokens_details.cached_tokens`），避免 OpenAI 格式错误阻止 `input_tokens` 补全
- **流式缓存标志传播** - 修复 `updateResponsesStreamUsage` 未传播 `HasClaudeCache` 标志的问题，确保流式响应正确识别 Claude 缓存

---

## [v2.3.4] - 2025-12-23

### ✨ 新功能

- **Models API 增强** - `/v1/models` 端点重大改进
  - 使用调度器按故障转移顺序选择渠道（与 Messages/Responses API 一致）
  - 同时从 Messages 和 Responses 两种渠道获取模型列表并合并去重
  - 添加详细日志：渠道名称、脱敏 Key、选择原因
  - 移除对 Claude 原生渠道的跳过限制（第三方 Claude 代理通常支持 /models）
  - 移除不常用的 `DELETE /v1/models/:model` 端点

---

## [v2.3.3] - 2025-12-23

### ✨ 新功能

- **Models API 端点支持** - 新增 `/v1/models` 系列端点，转发到上游 OpenAI 兼容服务
  - `GET /v1/models` - 获取模型列表
  - `GET /v1/models/:model` - 获取单个模型详情
  - `DELETE /v1/models/:model` - 删除微调模型
  - 自动跳过不支持的 Claude 原生渠道，遍历所有上游直到成功或返回 404

---

## [v2.3.2] - 2025-12-23

### ✨ 新功能

- **快速添加渠道自动检测协议类型** - 根据 URL 路径自动选择正确的服务类型
  - `/messages` → Claude 协议
  - `/chat/completions` → OpenAI 协议
  - `/responses` → Responses 协议
  - `/generateContent` → Gemini 协议
- **快速添加支持 `%20` 分隔符** - 解析输入时自动将 URL 编码的空格转换为实际空格

---

## [v2.3.1] - 2025-12-22

### ✨ 新功能

- **HTTP 响应头超时可配置** - 新增 `RESPONSE_HEADER_TIMEOUT` 环境变量（默认 60 秒，范围 30-120 秒），解决上游响应慢导致的 `http2: timeout awaiting response headers` 错误

---

## [v2.3.0] - 2025-12-22

### ✨ 新功能

- **快速添加渠道支持引号内容提取** - 支持从双引号/单引号中提取 URL 和 API Key，可直接粘贴 Claude Code 环境变量 JSON 配置格式
- **SQLite 指标持久化存储** - 服务重启后不再丢失历史指标数据，启动时自动加载最近 24 小时数据
  - 新增 `METRICS_PERSISTENCE_ENABLED`（默认 true）和 `METRICS_RETENTION_DAYS`（默认 7）配置
  - 异步批量写入（100 条/批或每 30 秒），WAL 模式高并发，自动清理过期数据
- **完整的 Responses API Token Usage 统计** - 支持多格式自动检测（Claude/Gemini/OpenAI）、缓存 TTL 细分统计（5m/1h）
- **Messages API 缓存 TTL 细分统计** - 区分 5 分钟和 1 小时 TTL 的缓存创建统计

### 🔨 重构

- **SQLite 驱动切换为纯 Go 实现** - 从 `go-sqlite3`（CGO）切换为 `modernc.org/sqlite`，简化交叉编译

### 🐛 修复

- **Usage 解析数值类型健壮性** - 支持 `float64`/`int`/`int64`/`int32` 四种数值类型
- **CachedTokens 重复计算** - `CachedTokens` 仅包含 `cache_read`，不再包含 `cache_creation`
- **流式响应纯缓存场景 Usage 丢失** - 有任何 usage 字段时都记录

---

## [v2.2.0] - 2025-12-21

### 🔨 重构

- **Handlers 模块重构为同级子包结构** - 将 Messages/Responses API 处理器重构为同级模块，新增 `handlers/common/` 公共包，代码量减少约 180 行

### 🐛 修复

- **Stream 错误处理完善** - 流式传输错误时发送 SSE 错误事件并记录失败指标
- **CountTokens 端点安全加固** - 应用请求体大小限制
- **非 failover 错误指标记录** - 400/401/403 等错误正确记录失败指标

---

## [v2.1.35] - 2025-12-21

- **流量图表失败率可视化** - 失败率超过 10% 显示红色背景，Tooltip 显示详情

---

## [v2.1.34] - 2025-12-20

- **Key 级别使用趋势图表** - 支持流量/Token I/O/缓存三种视图，智能 Key 筛选
- **合并 Dashboard API** - 3 个并行请求优化为 1 个

---

## [v2.1.33] - 2025-12-20

- **Fuzzy Mode 错误处理开关** - 所有非 2xx 错误自动触发 failover
- **渠道指标历史数据 API** - 支持时间序列图表

---

## [v2.1.25] - 2025-12-18

### ✨ 新功能

- **TransformerMetadata 和 CacheControl 支持** - 转换器元数据保留原始格式信息，实现特性透传
- **FinishReason 统一映射函数** - OpenAI/Anthropic/Responses 三种协议间双向映射
- **原始日志输出开关** - `RAW_LOG_OUTPUT` 环境变量，开启后不进行格式化或截断

---

## [v2.1.23] - 2025-12-13

- 修复编辑渠道弹窗中基础 URL 布局和验证问题

---

## [v2.1.31] - 2025-12-19

- **前端显示版本号和更新检查** - 自动检查 GitHub 最新版本

---

## [v2.1.30] - 2025-12-19

- **强制探测模式** - 所有 Key 熔断时自动启用强制探测

---

## [v2.1.28] - 2025-12-19

- **BaseURL 支持 `#` 结尾跳过自动添加 `/v1`**

---

## [v2.1.27] - 2025-12-19

- 移除 Claude Provider 畸形 tool_call 修复逻辑

---

## [v2.1.26] - 2025-12-19

- Responses 渠道新增 `gpt-5.2-codex` 模型选项

---

## [v2.1.24] - 2025-12-17

- Responses 渠道新增 `gpt-5.2`、`gpt-5` 模型选项
- 移除 openaiold 服务类型支持

---

## [v2.1.23] - 2025-12-13

- 修复 402 状态码未触发 failover 的问题
- 重构 HTTP 状态码 failover 判断逻辑（两层分类策略）

---

## [v2.1.22] - 2025-12-13

### 🐛 修复

- **流式日志合成器类型修复** - 所有 Provider 的 HandleStreamResponse 都将响应转换为 Claude SSE 格式，日志合成器使用 "claude" 类型解析
- **insecureSkipVerify 字段提交修复** - 修复前端 insecureSkipVerify 为 false 时不提交的问题

---

## [v2.1.21] - 2025-12-13

### 🐛 修复

- **促销渠道绕过健康检查** - 促销渠道现在绕过健康检查直接尝试使用，只有本次请求实际失败后才跳过

---

## [v2.1.20] - 2025-12-12

- 渠道名称支持点击打开编辑弹窗

---

## [v2.1.19] - 2025-12-12

- 修复添加渠道弹窗密钥重复错误状态残留
- 新增 `/v1/responses/compact` 端点

---

## [v2.1.15] - 2025-12-12

### 🔒 安全加固

- **请求体大小限制** - 新增 `MAX_REQUEST_BODY_SIZE_MB` 环境变量（默认 50MB），超限返回 413
- **Goroutine 泄漏修复** - ConfigManager 添加 `stopChan` 和 `Close()` 方法释放资源
- **数据竞争修复** - 负载均衡计数器改用 `sync/atomic` 原子操作
- **优雅关闭** - 监听 SIGINT/SIGTERM，10 秒超时优雅关闭

---

## [v2.1.14] - 2025-12-12

- 修复流式响应 Token 计数中间更新被覆盖

---

## [v2.1.12] - 2025-12-11

- 支持 Claude 缓存 Token 计数

---

## [v2.1.10] - 2025-12-11

- 修复流式响应 Token 计数补全逻辑

---

## [v2.1.8] - 2025-12-11

- 重构过长方法，提升代码可读性

---

## [v2.1.7] - 2025-12-11

### 🐛 修复

- 修复前端 MDI 图标无法显示
- **Token 计数补全虚假值处理** - 当 `input_tokens <= 1` 或 `output_tokens == 0` 时用本地估算值覆盖

---

## [v2.1.6] - 2025-12-11

### ✨ 新功能

- **Messages API Token 计数补全** - 当上游不返回 usage 时，本地估算 token 数量并附加到响应中

---

## [v2.1.4] - 2025-12-11

- 修复前端渠道健康度统计不显示数据

---

## [v2.1.1] - 2025-12-11

- 新增 `QUIET_POLLING_LOGS` 环境变量（默认 true），过滤前端轮询日志噪音

---

## [v2.1.0] - 2025-12-11

### 🔨 重构

- **指标系统重构：Key 级别绑定** - 指标键改为 `hash(baseURL + apiKey)`，每个 Key 独立追踪
- **熔断器生效修复** - 在 `tryChannelWithAllKeys` 中调用 `ShouldSuspendKey()` 跳过熔断的 Key
- **单渠道路径指标记录** - 转换失败、发送失败、failover、成功时正确记录指标

---

## [v2.0.20-go] - 2025-12-08

- 修复单渠道模式渠道选择逻辑

---

## [v2.0.11-go] - 2025-12-06

### 🚀 多渠道智能调度器

- **ChannelScheduler** - 基于优先级的渠道选择、Trace 亲和性、失败率检测和自动熔断
- **MetricsManager** - 滑动窗口算法计算实时成功率
- **TraceAffinityManager** - 用户会话与渠道绑定

### 🎨 渠道编排面板

- 拖拽排序、实时指标、状态切换、备用池管理

---

## [v2.0.10-go] - 2025-12-06

### 🎨 复古像素主题

- Neo-Brutalism 设计语言：无圆角、等宽字体、粗实体边框、硬阴影

---

## [v2.0.5-go] - 2025-11-15

### 🚀 Responses API 转换器架构重构

- 策略模式 + 工厂模式实现多上游转换器
- 完整支持 Responses API 标准格式

---

## [v2.0.4-go] - 2025-11-14

### ✨ Responses API 透明转发

- Codex Responses API 端点 (`/v1/responses`)
- 会话管理系统（多轮对话跟踪）
- Messages API 多上游协议支持（Claude/OpenAI/Gemini）

---

## [v2.0.0-go] - 2025-10-15

### 🎉 Go 语言重写版本

- **性能提升**: 启动速度 20x，内存占用 -70%
- **单文件部署**: 前端资源嵌入二进制
- **完整功能移植**: 所有上游适配器、协议转换、流式响应、配置热重载

---

## 历史版本

<details>
<summary>v1.x TypeScript 版本</summary>

### v1.2.0 - 2025-09-19
- Web 管理界面、模型映射、渠道置顶、API 密钥故障转移

### v1.1.0 - 2025-09-17
- SSE 数据解析优化、Bearer Token 处理简化、代码重构

### v1.0.0 - 2025-09-13
- 初始版本：多上游支持、负载均衡、配置管理

</details>
