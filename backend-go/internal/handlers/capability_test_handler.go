package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/httpclient"
	"github.com/BenedictKing/ccx/internal/metrics"
	"github.com/BenedictKing/ccx/internal/utils"
	"github.com/gin-gonic/gin"
)

// ============== 缓存定义 ==============

const (
	capabilityCacheTTL    = 30 * time.Minute // 缓存 TTL（每次命中续期）
	capabilityCacheMaxTTL = 4 * time.Hour    // 缓存最大生存时间（从首次创建算起）
)

// capabilityCacheEntry 缓存条目
type capabilityCacheEntry struct {
	response  CapabilityTestResponse
	createdAt time.Time // 首次创建时间（用于计算最大生存期）
	expiresAt time.Time // 过期时间（每次命中续期）
}

// capabilityCache 全局能力测试缓存
var capabilityCache = struct {
	sync.RWMutex
	entries map[string]*capabilityCacheEntry
}{
	entries: make(map[string]*capabilityCacheEntry),
}

// buildCapabilityCacheKey 构建缓存 key（基于 baseURL + apiKey 与协议列表）
func buildCapabilityCacheKey(baseURL string, apiKey string, protocols []string) string {
	sorted := make([]string, len(protocols))
	copy(sorted, protocols)
	sort.Strings(sorted)
	metricsKey := metrics.GenerateMetricsKey(baseURL, apiKey)
	return fmt.Sprintf("%s:%s", metricsKey, strings.Join(sorted, ","))
}

// getCapabilityCache 读取缓存，命中时自动续期（不超过最大生存期）
// 全程持有写锁以避免并发读写 expiresAt 的竞态
func getCapabilityCache(key string) (*CapabilityTestResponse, bool) {
	capabilityCache.Lock()
	defer capabilityCache.Unlock()

	entry, ok := capabilityCache.entries[key]
	if !ok {
		return nil, false
	}

	now := time.Now()
	if now.After(entry.expiresAt) {
		// 已过期，删除
		delete(capabilityCache.entries, key)
		return nil, false
	}

	// 命中，续期（不超过最大生存期）
	newExpiry := now.Add(capabilityCacheTTL)
	maxExpiry := entry.createdAt.Add(capabilityCacheMaxTTL)
	if newExpiry.After(maxExpiry) {
		newExpiry = maxExpiry
	}
	entry.expiresAt = newExpiry

	return &entry.response, true
}

// setCapabilityCache 写入缓存
func setCapabilityCache(key string, resp CapabilityTestResponse) {
	now := time.Now()
	capabilityCache.Lock()
	capabilityCache.entries[key] = &capabilityCacheEntry{
		response:  resp,
		createdAt: now,
		expiresAt: now.Add(capabilityCacheTTL),
	}
	capabilityCache.Unlock()
}

// ============== 类型定义 ==============

// CapabilityTestRequest 能力测试请求体
type CapabilityTestRequest struct {
	TargetProtocols []string `json:"targetProtocols"`
	Timeout         int      `json:"timeout"` // 毫秒
}

type ModelTestResult struct {
	Model              string  `json:"model"`
	Success            bool    `json:"success"`
	Latency            int64   `json:"latency"` // 毫秒
	StreamingSupported bool    `json:"streamingSupported"`
	Error              *string `json:"error,omitempty"`
	StartedAt          string  `json:"startedAt,omitempty"`
	TestedAt           string  `json:"testedAt"`
}

// ProtocolTestResult 单个协议测试结果
type ProtocolTestResult struct {
	Protocol           string            `json:"protocol"`
	Success            bool              `json:"success"`
	Latency            int64             `json:"latency"` // 毫秒
	StreamingSupported bool              `json:"streamingSupported"`
	TestedModel        string            `json:"testedModel"` // 优先返回首个成功模型名称，兼容旧字段
	ModelResults       []ModelTestResult `json:"modelResults,omitempty"`
	SuccessCount       int               `json:"successCount,omitempty"`
	AttemptedModels    int               `json:"attemptedModels,omitempty"`
	Error              *string           `json:"error"`
	TestedAt           string            `json:"testedAt"`
}

// CapabilityTestResponse 能力测试响应体
type CapabilityTestResponse struct {
	ChannelID           int                  `json:"channelId"`
	ChannelName         string               `json:"channelName"`
	SourceType          string               `json:"sourceType"`
	Tests               []ProtocolTestResult `json:"tests"`
	CompatibleProtocols []string             `json:"compatibleProtocols"`
	TotalDuration       int64                `json:"totalDuration"` // 毫秒
}

// ============== 主处理器 ==============

// TestChannelCapability 渠道能力测试处理器
// channelKind 决定从哪个配置获取渠道：messages/responses/gemini/chat
func TestChannelCapability(cfgManager *config.ConfigManager, channelKind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
			return
		}

		channel, err := getCapabilityTestChannel(cfgManager, channelKind, id)
		if err != nil {
			statusCode := http.StatusBadRequest
			if err.Error() == "channel not found" {
				statusCode = http.StatusNotFound
			}
			c.JSON(statusCode, gin.H{"error": err.Error()})
			return
		}

		var req CapabilityTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		timeout := 10 * time.Second
		if req.Timeout > 0 {
			timeout = time.Duration(req.Timeout) * time.Millisecond
		}

		protocols := req.TargetProtocols
		if len(protocols) == 0 {
			protocols = []string{"messages", "chat", "gemini", "responses"}
		}

		effectiveRPM := channel.RPM
		if effectiveRPM <= 0 {
			effectiveRPM = 10
		}
		channel.RPM = effectiveRPM

		if len(channel.APIKeys) == 0 {
			errMsg := "no_api_key"
			resp := CapabilityTestResponse{
				ChannelID:           id,
				ChannelName:         channel.Name,
				SourceType:          channel.ServiceType,
				Tests:               []ProtocolTestResult{{Protocol: "all", Error: &errMsg, TestedAt: time.Now().Format(time.RFC3339)}},
				CompatibleProtocols: []string{},
				TotalDuration:       0,
			}
			job := createCapabilityJobFromResponse(id, channel.Name, channelKind, channel.ServiceType, protocols, timeout, resp, false, "")
			capabilityJobs.create(job)
			c.JSON(http.StatusOK, gin.H{"jobId": job.JobID, "resumed": false})
			return
		}

		baseURL := ""
		if len(channel.GetAllBaseURLs()) > 0 {
			baseURL = channel.GetAllBaseURLs()[0]
		}
		apiKey := ""
		if len(channel.APIKeys) > 0 {
			apiKey = channel.APIKeys[0]
		}

		cacheKey := buildCapabilityCacheKey(baseURL, apiKey, protocols)
		lookupKey := buildCapabilityJobLookupKey(cacheKey, channelKind, id)

		if cached, ok := getCapabilityCache(cacheKey); ok {
			log.Printf("[CapabilityTest-Cache] 渠道 %s (ID:%d) 命中缓存，创建已完成任务", channel.Name, id)
			cached.ChannelID = id
			cached.ChannelName = channel.Name
			cached.SourceType = channel.ServiceType
			job, reused := capabilityJobs.getOrCreateByLookupKey(lookupKey, func() *CapabilityTestJob {
				return createCapabilityJobFromResponse(id, channel.Name, channelKind, channel.ServiceType, protocols, timeout, *cached, true, "")
			})
			c.JSON(http.StatusOK, gin.H{"jobId": job.JobID, "resumed": reused})
			return
		}

		job, reused := capabilityJobs.getOrCreateByLookupKey(lookupKey, func() *CapabilityTestJob {
			return newCapabilityTestJob(id, channel.Name, channelKind, channel.ServiceType, protocols, timeout)
		})
		if reused {
			log.Printf("[CapabilityTest-Job] 复用能力测试任务 %s，渠道 %s (ID:%d, 类型:%s)", job.JobID, channel.Name, id, channel.ServiceType)
			c.JSON(http.StatusOK, gin.H{"jobId": job.JobID, "resumed": true})
			return
		}
		log.Printf("[CapabilityTest-Job] 创建能力测试任务 %s，渠道 %s (ID:%d, 类型:%s)，协议: %v", job.JobID, channel.Name, id, channel.ServiceType, protocols)

		go runCapabilityTestJob(job.JobID, channelKind, id, *channel, protocols, timeout, cacheKey, lookupKey)

		c.JSON(http.StatusOK, gin.H{"jobId": job.JobID, "resumed": false})
		return
	}
}

// ============== 核心测试逻辑 ==============

func runCapabilityTestJob(jobID, channelKind string, channelID int, channel config.UpstreamConfig, protocols []string, timeout time.Duration, cacheKey, lookupKey string) {
	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		job.Status = CapabilityJobStatusRunning
		job.StartedAt = time.Now().Format(time.RFC3339Nano)
	})

	log.Printf("[CapabilityTest-Job] 开始执行能力测试任务 %s，渠道 %s (ID:%d, 类型:%s)，协议: %v", jobID, channel.Name, channelID, channel.ServiceType, protocols)

	totalStart := time.Now()
	results := testProtocolCompatibility(context.Background(), &channel, protocols, timeout, jobID)
	totalDuration := time.Since(totalStart).Milliseconds()

	compatible := make([]string, 0)
	for _, r := range results {
		if r.Success {
			compatible = append(compatible, r.Protocol)
		}
	}

	resp := CapabilityTestResponse{
		ChannelID:           channelID,
		ChannelName:         channel.Name,
		SourceType:          channel.ServiceType,
		Tests:               results,
		CompatibleProtocols: compatible,
		TotalDuration:       totalDuration,
	}

	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		job.ChannelName = channel.Name
		job.SourceType = channel.ServiceType
		job.Tests = capabilityProtocolResultsFromResponse(resp)
		job.CompatibleProtocols = append([]string(nil), compatible...)
		job.TotalDuration = totalDuration
		job.FinishedAt = time.Now().Format(time.RFC3339Nano)
		if len(compatible) > 0 {
			job.Status = CapabilityJobStatusCompleted
		} else {
			job.Status = CapabilityJobStatusFailed
		}
	})

	if len(compatible) > 0 {
		setCapabilityCache(cacheKey, resp)
		log.Printf("[CapabilityTest-Cache] 渠道 %s (ID:%d) 写入缓存，兼容协议: %v", channel.Name, channelID, compatible)
	}

	if lookupKey != "" {
		capabilityJobs.clearLookupKey(lookupKey)
	}

	log.Printf("[CapabilityTest-Job] 能力测试任务 %s 完成，渠道 %s，兼容协议: %v，总耗时: %dms", jobID, channel.Name, compatible, totalDuration)
}

// testProtocolCompatibility 并发测试多个协议的兼容性
func testProtocolCompatibility(ctx context.Context, channel *config.UpstreamConfig, protocols []string, timeout time.Duration, jobID string) []ProtocolTestResult {
	results := make([]ProtocolTestResult, len(protocols))
	var wg sync.WaitGroup

	for i, protocol := range protocols {
		wg.Add(1)
		go func(idx int, proto string) {
			defer wg.Done()
			results[idx] = testSingleProtocol(ctx, channel, proto, timeout, jobID)
		}(i, protocol)
	}

	wg.Wait()
	return results
}

// testSingleProtocol 测试单个协议的兼容性（协议内模型全部进入全局 dispatcher 排队）
func testSingleProtocol(ctx context.Context, channel *config.UpstreamConfig, protocol string, timeout time.Duration, jobID string) ProtocolTestResult {
	result := ProtocolTestResult{
		Protocol: protocol,
		TestedAt: time.Now().Format(time.RFC3339),
	}

	log.Printf("[CapabilityTest-Protocol] 开始测试渠道 %s 的 %s 协议兼容性", channel.Name, protocol)
	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		for i := range job.Tests {
			if job.Tests[i].Protocol == protocol {
				job.Tests[i].Status = CapabilityProtocolStatusRunning
				job.Tests[i].TestedAt = time.Now().Format(time.RFC3339Nano)
				break
			}
		}
	})

	models, err := getCapabilityProbeModels(protocol)
	if err != nil {
		errMsg := "no_models_configured"
		result.Error = &errMsg
		capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
			for i := range job.Tests {
				if job.Tests[i].Protocol == protocol {
					job.Tests[i].Status = CapabilityProtocolStatusFailed
					job.Tests[i].Error = result.Error
					job.Tests[i].TestedAt = time.Now().Format(time.RFC3339Nano)
					break
				}
			}
		})
		log.Printf("[CapabilityTest-Protocol] 渠道 %s 获取 %s 协议测试模型失败: %v", channel.Name, protocol, err)
		return result
	}

	totalStart := time.Now()
	result.AttemptedModels = len(models)
	result.ModelResults = make([]ModelTestResult, len(models))
	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		for i := range job.Tests {
			if job.Tests[i].Protocol != protocol {
				continue
			}
			job.Tests[i].AttemptedModels = len(models)
			job.Tests[i].ModelResults = make([]CapabilityModelJobResult, len(models))
			for idx, modelName := range models {
				job.Tests[i].ModelResults[idx] = CapabilityModelJobResult{
					Model:  modelName,
					Status: CapabilityModelStatusQueued,
				}
			}
			break
		}
	})

	type modelOutcome struct {
		index  int
		result ModelTestResult
	}

	outcomeCh := make(chan modelOutcome, len(models))
	var wg sync.WaitGroup

	for i, model := range models {
		log.Printf("[CapabilityTest-Protocol] 渠道 %s 启动模型 %s (%d/%d)", channel.Name, model, i+1, len(models))
		wg.Add(1)
		go func(idx int, modelName string) {
			defer wg.Done()
			outcomeCh <- modelOutcome{
				index:  idx,
				result: testSingleModel(ctx, channel, protocol, modelName, timeout, jobID),
			}
		}(i, model)
	}

	wg.Wait()
	close(outcomeCh)

	var failureSummaries []string
	for outcome := range outcomeCh {
		result.ModelResults[outcome.index] = outcome.result
		if outcome.result.Success {
			result.SuccessCount++
		}
	}

	for _, modelResult := range result.ModelResults {
		if !modelResult.Success && modelResult.Error != nil && *modelResult.Error != "" {
			failureSummaries = append(failureSummaries, fmt.Sprintf("%s=%s", modelResult.Model, *modelResult.Error))
		}
	}

	result.Success = result.SuccessCount > 0
	if result.Success {
		for _, modelResult := range result.ModelResults {
			if modelResult.Success {
				result.TestedModel = modelResult.Model
				result.StreamingSupported = modelResult.StreamingSupported
				break
			}
		}
	}

	if !result.Success {
		result.Latency = time.Since(totalStart).Milliseconds()
		errMsg := "all_models_failed"
		if len(failureSummaries) > 0 {
			errMsg = failureSummaries[0]
		}
		result.Error = &errMsg
		capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
			for i := range job.Tests {
				if job.Tests[i].Protocol == protocol {
					job.Tests[i].Status = CapabilityProtocolStatusFailed
					job.Tests[i].Success = false
					job.Tests[i].Latency = result.Latency
					job.Tests[i].StreamingSupported = false
					job.Tests[i].TestedModel = ""
					job.Tests[i].SuccessCount = result.SuccessCount
					job.Tests[i].AttemptedModels = result.AttemptedModels
					job.Tests[i].Error = result.Error
					job.Tests[i].TestedAt = time.Now().Format(time.RFC3339Nano)
					for j := range job.Tests[i].ModelResults {
						if job.Tests[i].ModelResults[j].Status == CapabilityModelStatusQueued || job.Tests[i].ModelResults[j].Status == CapabilityModelStatusRunning {
							job.Tests[i].ModelResults[j].Status = CapabilityModelStatusFailed
						}
					}
					break
				}
			}
		})
		log.Printf("[CapabilityTest-Protocol] 渠道 %s 的 %s 协议全部模型测试失败 (尝试: %d, 总耗时: %dms): %s",
			channel.Name, protocol, result.AttemptedModels, result.Latency, errMsg)
		return result
	}

	result.Latency = time.Since(totalStart).Milliseconds()
	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		for i := range job.Tests {
			if job.Tests[i].Protocol == protocol {
				job.Tests[i].Status = CapabilityProtocolStatusCompleted
				job.Tests[i].Success = true
				job.Tests[i].Latency = result.Latency
				job.Tests[i].StreamingSupported = result.StreamingSupported
				job.Tests[i].TestedModel = result.TestedModel
				job.Tests[i].SuccessCount = result.SuccessCount
				job.Tests[i].AttemptedModels = result.AttemptedModels
				job.Tests[i].Error = nil
				job.Tests[i].TestedAt = time.Now().Format(time.RFC3339Nano)
				break
			}
		}
	})
	log.Printf("[CapabilityTest-Protocol] 渠道 %s 的 %s 协议测试完成 (成功: %d/%d, 首个成功模型: %s, 总耗时: %dms)",
		channel.Name, protocol, result.SuccessCount, result.AttemptedModels, result.TestedModel, result.Latency)
	return result
}

func testSingleModel(ctx context.Context, channel *config.UpstreamConfig, protocol, model string, timeout time.Duration, jobID string) ModelTestResult {
	startedAt := time.Now()
	modelResult := ModelTestResult{
		Model:     model,
		StartedAt: startedAt.Format(time.RFC3339Nano),
	}

	req, err := buildTestRequestWithModel(protocol, channel, model)
	if err != nil {
		errMsg := fmt.Sprintf("build_request_failed: %v", err)
		modelResult.Error = &errMsg
		modelResult.TestedAt = time.Now().Format(time.RFC3339Nano)
		capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
			updateCapabilityJobModelResult(job, protocol, model, CapabilityModelStatusFailed, modelResult)
		})
		log.Printf("[CapabilityTest-Model] 渠道 %s 构建 %s 测试请求失败 (模型: %s): %v", channel.Name, protocol, model, err)
		return modelResult
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req = req.WithContext(reqCtx)

	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		updateCapabilityJobModelResult(job, protocol, model, CapabilityModelStatusQueued, modelResult)
	})
	interval := time.Minute / time.Duration(channel.RPM)
	if interval <= 0 {
		interval = time.Minute / 10
	}
	if err := GetCapabilityTestDispatcher().AcquireSendSlot(reqCtx, interval); err != nil {
		errMsg := classifyError(err, 0, reqCtx)
		modelResult.Error = &errMsg
		modelResult.TestedAt = time.Now().Format(time.RFC3339Nano)
		capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
			updateCapabilityJobModelResult(job, protocol, model, CapabilityModelStatusFailed, modelResult)
		})
		log.Printf("[CapabilityTest-Model] 渠道 %s 获取发送槽位失败 (协议: %s, 模型: %s): %s", channel.Name, protocol, model, errMsg)
		return modelResult
	}

	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		updateCapabilityJobModelResult(job, protocol, model, CapabilityModelStatusRunning, modelResult)
	})

	client := httpclient.GetManager().GetStandardClient(timeout, channel.InsecureSkipVerify, channel.ProxyURL)

	startTime := time.Now()
	log.Printf("[CapabilityTest-Model] 渠道 %s 启动 %s 协议模型测试 (模型: %s, startedAt: %s)",
		channel.Name, protocol, model, modelResult.StartedAt)
	success, streamingSupported, statusCode, sendErr := sendAndCheckStream(reqCtx, client, req)
	modelResult.Latency = time.Since(startTime).Milliseconds()
	modelResult.TestedAt = time.Now().Format(time.RFC3339Nano)

	if success {
		modelResult.Success = true
		modelResult.StreamingSupported = streamingSupported
		capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
			updateCapabilityJobModelResult(job, protocol, model, CapabilityModelStatusSuccess, modelResult)
		})
		log.Printf("[CapabilityTest-Model] 渠道 %s 的 %s 协议测试成功 (模型: %s, 流式: %v, 耗时: %dms)",
			channel.Name, protocol, model, streamingSupported, modelResult.Latency)
		return modelResult
	}

	errMsg := classifyError(sendErr, statusCode, reqCtx)
	modelResult.Error = &errMsg
	capabilityJobs.update(jobID, func(job *CapabilityTestJob) {
		updateCapabilityJobModelResult(job, protocol, model, CapabilityModelStatusFailed, modelResult)
	})
	log.Printf("[CapabilityTest-Model] 渠道 %s 的 %s 协议测试失败 (模型: %s, 耗时: %dms): %s",
		channel.Name, protocol, model, modelResult.Latency, errMsg)
	return modelResult
}

func updateCapabilityJobModelResult(job *CapabilityTestJob, protocol, model string, status CapabilityModelStatus, result ModelTestResult) {
	for i := range job.Tests {
		if job.Tests[i].Protocol != protocol {
			continue
		}
		for j := range job.Tests[i].ModelResults {
			if job.Tests[i].ModelResults[j].Model != model {
				continue
			}
			job.Tests[i].ModelResults[j].Status = status
			job.Tests[i].ModelResults[j].Success = result.Success
			job.Tests[i].ModelResults[j].Latency = result.Latency
			job.Tests[i].ModelResults[j].StreamingSupported = result.StreamingSupported
			job.Tests[i].ModelResults[j].Error = result.Error
			job.Tests[i].ModelResults[j].StartedAt = result.StartedAt
			job.Tests[i].ModelResults[j].TestedAt = result.TestedAt
			return
		}
	}
}

// ============== 请求构建 ==============

// buildTestRequestWithModel 构建最小化测试请求（指定模型）
func buildTestRequestWithModel(protocol string, channel *config.UpstreamConfig, model string) (*http.Request, error) {
	// 获取 BaseURL
	urls := channel.GetAllBaseURLs()
	if len(urls) == 0 {
		return nil, fmt.Errorf("no base URL configured")
	}
	baseURL := urls[0]

	// 如果末尾有 #，去掉 # 后不添加版本前缀
	noVersionPrefix := false
	if strings.HasSuffix(baseURL, "#") {
		baseURL = strings.TrimSuffix(baseURL, "#")
		noVersionPrefix = true
	}

	// 处理 BaseURL：去除末尾 /
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := channel.APIKeys[0]

	var (
		requestURL string
		body       []byte
		err        error
		isGemini   bool
	)

	switch protocol {
	case "messages":
		if noVersionPrefix {
			requestURL = baseURL + "/messages"
		} else {
			requestURL = baseURL + "/v1/messages"
		}
		body, err = json.Marshal(map[string]interface{}{
			"model": model,
			"system": []map[string]interface{}{
				{
					"type": "text",
					"text": "x-anthropic-billing-header: cc_version=2.1.71.2f9; cc_entrypoint=cli;",
				},
				{
					"type": "text",
					"text": "You are a Claude agent, built on Anthropic's Claude Agent SDK.",
					"cache_control": map[string]string{
						"type": "ephemeral",
					},
				},
			},
			"messages":   []map[string]string{{"role": "user", "content": "What are you best at: code generation, creative writing, or math problem solving?"}},
			"max_tokens": 100,
			"stream":     true,
			"thinking": map[string]interface{}{
				"type": "disabled",
			},
		})

	case "chat":
		if noVersionPrefix {
			requestURL = baseURL + "/chat/completions"
		} else {
			requestURL = baseURL + "/v1/chat/completions"
		}
		body, err = json.Marshal(map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "What are you best at: code generation, creative writing, or math problem solving?"},
			},
			"max_tokens":       100,
			"stream":           true,
			"reasoning_effort": "none",
		})

	case "gemini":
		if noVersionPrefix {
			requestURL = baseURL + "/models/" + model + ":streamGenerateContent?alt=sse"
		} else {
			requestURL = baseURL + "/v1beta/models/" + model + ":streamGenerateContent?alt=sse"
		}
		body, err = json.Marshal(map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"role":  "user",
					"parts": []map[string]string{{"text": "What are you best at: code generation, creative writing, or math problem solving?"}},
				},
			},
			"systemInstruction": map[string]interface{}{
				"parts": []map[string]string{{"text": "You are Gemini CLI, an interactive CLI agent specializing in software engineering tasks."}},
			},
			"generationConfig": map[string]interface{}{
				"maxOutputTokens": 100,
				"thinkingConfig": map[string]interface{}{
					"thinkingLevel": "low",
				},
			},
		})
		isGemini = true

	case "responses":
		if noVersionPrefix {
			requestURL = baseURL + "/responses"
		} else {
			requestURL = baseURL + "/v1/responses"
		}
		body, err = json.Marshal(map[string]interface{}{
			"model":             model,
			"input":             "What are you best at: code generation, creative writing, or math problem solving?",
			"instructions":      "You are Codex, a coding agent based on GPT-5.",
			"max_output_tokens": 100,
			"stream":            true,
			"reasoning": map[string]interface{}{
				"effort": "low",
			},
		})

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("marshal request body failed: %w", err)
	}

	req, err := http.NewRequest("POST", requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置通用头部
	req.Header.Set("Content-Type", "application/json")

	// 设置认证头部
	if isGemini {
		utils.SetGeminiAuthenticationHeader(req.Header, apiKey)
	} else {
		utils.SetAuthenticationHeader(req.Header, apiKey)
		// Messages 协议需要 anthropic-version、anthropic-beta、User-Agent 和 X-App 头部
		if protocol == "messages" {
			req.Header.Set("anthropic-version", "2023-06-01")
			req.Header.Set("anthropic-beta", "claude-code-20250219,adaptive-thinking-2026-01-28,prompt-caching-scope-2026-01-05,effort-2025-11-24")
			req.Header.Set("User-Agent", "claude-cli/2.1.71 (external, cli)")
			req.Header.Set("X-App", "cli")
		}
		// Responses 协议需要 Originator 和 User-Agent 头部
		if protocol == "responses" {
			req.Header.Set("Originator", "codex_cli_rs")
			req.Header.Set("User-Agent", "codex_cli_rs/0.111.0 (Mac OS 26.3.0; arm64) iTerm.app/3.6.6")
		}
	}

	// 应用自定义请求头
	if channel.CustomHeaders != nil {
		for key, value := range channel.CustomHeaders {
			req.Header.Set(key, value)
		}
	}

	return req, nil
}

// buildTestRequest 构建最小化测试请求（使用首选模型，兼容旧接口）
func buildTestRequest(protocol string, channel *config.UpstreamConfig) (*http.Request, error) {
	model, err := getCapabilityProbeModel(protocol)
	if err != nil {
		return nil, err
	}
	return buildTestRequestWithModel(protocol, channel, model)
}

// ============== 流式响应检测 ==============

// sendAndCheckStream 发送请求并检查流式响应能力
// 返回: success（HTTP 2xx）, streamingSupported（能解析 SSE chunk）, statusCode, error
func sendAndCheckStream(ctx context.Context, client *http.Client, req *http.Request) (bool, bool, int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return false, false, 0, err
	}
	defer resp.Body.Close()

	// 非 2xx 视为不兼容
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, false, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// HTTP 2xx，尝试读取第一个 SSE chunk 检测流式支持
	streamingSupported := false

	// 使用 5 秒读取超时
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()

	// 在 goroutine 中扫描以支持超时取消
	doneCh := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				// 跳过 [DONE] 标记
				if data == "[DONE]" {
					continue
				}
				// 尝试 JSON 解析
				var jsonObj map[string]interface{}
				if json.Unmarshal([]byte(data), &jsonObj) == nil {
					doneCh <- true
					return
				}
			}
		}
		doneCh <- false
	}()

	select {
	case result := <-doneCh:
		streamingSupported = result
	case <-readCtx.Done():
		// 读取超时，但 HTTP 2xx 所以 success 仍为 true
		streamingSupported = false
	}

	return true, streamingSupported, resp.StatusCode, nil
}

// ============== 错误分类 ==============

// classifyError 对错误进行分类
func classifyError(err error, statusCode int, ctx context.Context) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "timeout"
	}

	if statusCode == 429 {
		return "rate_limited"
	}

	if statusCode > 0 {
		return fmt.Sprintf("http_error_%d", statusCode)
	}

	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return "timeout"
	}

	return "request_failed: " + errStr
}
