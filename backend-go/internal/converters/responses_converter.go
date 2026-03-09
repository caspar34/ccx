package converters

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BenedictKing/ccx/internal/session"
	"github.com/BenedictKing/ccx/internal/types"
)

// ============== Responses → Claude Messages ==============

// ResponsesToClaudeMessages 将 Responses 格式转换为 Claude Messages 格式
// instructions 参数会被转换为 Claude API 的 system 参数（不在 messages 中）
func ResponsesToClaudeMessages(sess *session.Session, newInput interface{}, instructions string) ([]types.ClaudeMessage, string, error) {
	messages := []types.ClaudeMessage{}

	// 1. 处理历史消息
	for _, item := range sess.Messages {
		msg, err := responsesItemToClaudeMessage(item)
		if err != nil {
			return nil, "", fmt.Errorf("转换历史消息失败: %w", err)
		}
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	// 2. 处理新输入（统一在解析阶段完成 legacy tool_* → function_* 归一化）
	newItems, err := parseResponsesInput(newInput)
	if err != nil {
		return nil, "", err
	}

	// 3. 收集被跳过的 legacy tool_call ID，避免输出孤立的 tool_result/function_call_output。
	skippedCallIDs := make(map[string]bool)
	for _, item := range newItems {
		if item.Type == "tool_call" && item.ToolUse == nil {
			callID := item.CallID
			if callID == "" {
				callID = item.ID
			}
			if callID != "" {
				skippedCallIDs[callID] = true
			}
		}
	}

	// 4. 转换新输入，跳过与被跳过 tool_call 对应的结果项。
	for _, item := range newItems {
		if item.Type == "function_call_output" && item.CallID != "" && skippedCallIDs[item.CallID] {
			continue
		}

		msg, err := responsesItemToClaudeMessage(item)
		if err != nil {
			return nil, "", fmt.Errorf("转换新消息失败: %w", err)
		}
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	return messages, instructions, nil
}

// responsesItemToClaudeMessage 单个 ResponsesItem 转换为 Claude Message
func responsesItemToClaudeMessage(item types.ResponsesItem) (*types.ClaudeMessage, error) {
	item = types.NormalizeResponsesItem(item)

	if item.Type == "tool_call" {
		return nil, nil
	}
	if item.Type == "tool_result" {
		return nil, fmt.Errorf("tool_result 缺少 tool_use_id")
	}

	switch item.Type {
	case "message":
		// 新格式：嵌套结构（type=message, role=user/assistant, content=[]ContentBlock）
		role, contentText := resolveResponsesTextItem(item)
		if contentText == "" {
			return nil, nil // 空内容，跳过
		}

		return &types.ClaudeMessage{
			Role: role,
			Content: []types.ClaudeContent{
				{
					Type: "text",
					Text: contentText,
				},
			},
		}, nil

	case "text":
		// 旧格式：简单 string（向后兼容）
		role, contentStr := resolveResponsesTextItem(item)
		if contentStr == "" {
			return nil, fmt.Errorf("text 类型的 content 不能为空")
		}

		return &types.ClaudeMessage{
			Role: role,
			Content: []types.ClaudeContent{
				{
					Type: "text",
					Text: contentStr,
				},
			},
		}, nil

	case "function_call":
		callID, name, arguments, err := resolveFunctionCallItem(item)
		if err != nil {
			return nil, err
		}

		return &types.ClaudeMessage{
			Role: "assistant",
			Content: []types.ClaudeContent{{
				Type:  "tool_use",
				ID:    callID,
				Name:  name,
				Input: parseFunctionCallArguments(arguments),
			}},
		}, nil

	case "function_call_output":
		callID, output, err := resolveFunctionCallOutputItem(item)
		if err != nil {
			return nil, err
		}

		return &types.ClaudeMessage{
			Role: "user",
			Content: []types.ClaudeContent{{
				Type:      "tool_result",
				ToolUseID: callID,
				Content:   output,
			}},
		}, nil

	default:
		return nil, fmt.Errorf("未知的 item type: %s", item.Type)
	}
}

// ============== Claude Response → Responses ==============

// ClaudeResponseToResponses 将 Claude 响应转换为 Responses 格式
func ClaudeResponseToResponses(claudeResp map[string]interface{}, sessionID string) (*types.ResponsesResponse, error) {
	// 提取字段
	model, _ := claudeResp["model"].(string)
	content, _ := claudeResp["content"].([]interface{})

	// 转换 output
	output := []types.ResponsesItem{}
	for _, c := range content {
		contentBlock, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		blockType, _ := contentBlock["type"].(string)
		switch blockType {
		case "text":
			text, _ := contentBlock["text"].(string)
			output = append(output, types.ResponsesItem{
				Type:    "text",
				Content: text,
			})
		case "tool_use":
			id, _ := contentBlock["id"].(string)
			name, _ := contentBlock["name"].(string)
			arguments := ""
			if input, ok := contentBlock["input"]; ok {
				if argsJSON, err := JSONMarshal(input); err == nil {
					arguments = string(argsJSON)
				}
			}
			output = append(output, types.ResponsesItem{
				Type:      "function_call",
				CallID:    id,
				Name:      name,
				Arguments: arguments,
			})
		case "tool_result":
			toolUseID, _ := contentBlock["tool_use_id"].(string)
			output = append(output, types.ResponsesItem{
				Type:   "function_call_output",
				CallID: toolUseID,
				Output: contentBlock["content"],
			})
		}
	}

	// 提取 usage（使用统一入口自动检测格式）
	usage := ExtractUsageMetrics(claudeResp["usage"])

	// 生成 response ID
	responseID := generateResponseID()

	return &types.ResponsesResponse{
		ID:         responseID,
		Model:      model,
		Output:     output,
		Status:     "completed",
		PreviousID: "", // 将在外部设置
		Usage:      usage,
	}, nil
}

// ============== Responses → OpenAI Chat ==============

// ResponsesToOpenAIChatMessages 将 Responses 格式转换为 OpenAI Chat 格式
func ResponsesToOpenAIChatMessages(sess *session.Session, newInput interface{}, instructions string) ([]map[string]interface{}, error) {
	messages := []map[string]interface{}{}

	// 1. 处理 instructions（如果存在）
	if instructions != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": instructions,
		})
	}

	// 2. 处理历史消息
	for _, item := range sess.Messages {
		msg := responsesItemToOpenAIMessage(item)
		if msg != nil {
			messages = append(messages, msg)
		}
	}

	// 3. 处理新输入
	newItems, err := parseResponsesInput(newInput)
	if err != nil {
		return nil, err
	}

	for _, item := range newItems {
		msg := responsesItemToOpenAIMessage(item)
		if msg != nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// responsesItemToOpenAIMessage 单个 ResponsesItem 转换为 OpenAI Message
func responsesItemToOpenAIMessage(item types.ResponsesItem) map[string]interface{} {
	item = types.NormalizeResponsesItem(item)

	if item.Type == "tool_call" || item.Type == "tool_result" {
		return nil
	}

	switch item.Type {
	case "message":
		// 新格式：嵌套结构
		role, contentText := resolveResponsesTextItem(item)
		if contentText == "" {
			return nil
		}

		return map[string]interface{}{
			"role":    role,
			"content": contentText,
		}

	case "text":
		// 旧格式：简单 string
		role, contentStr := resolveResponsesTextItem(item)
		if contentStr == "" {
			return nil
		}

		return map[string]interface{}{
			"role":    role,
			"content": contentStr,
		}

	case "function_call":
		callID, name, arguments, err := resolveFunctionCallItem(item)
		if err != nil {
			return nil
		}
		return map[string]interface{}{
			"role": "assistant",
			"tool_calls": []map[string]interface{}{{
				"id":   callID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      name,
					"arguments": arguments,
				},
			}},
		}

	case "function_call_output":
		callID, output, err := resolveFunctionCallOutputItem(item)
		if err != nil {
			return nil
		}

		contentStr := ""
		switch output := output.(type) {
		case string:
			contentStr = output
		case nil:
			contentStr = ""
		default:
			contentJSON, err := JSONMarshal(output)
			if err != nil {
				return nil
			}
			contentStr = string(contentJSON)
		}

		return map[string]interface{}{
			"role":         "tool",
			"tool_call_id": callID,
			"content":      contentStr,
		}
	}

	return nil
}

// ============== OpenAI Chat Response → Responses ==============

// OpenAIChatResponseToResponses 将 OpenAI Chat 响应转换为 Responses 格式
func OpenAIChatResponseToResponses(openaiResp map[string]interface{}, sessionID string) (*types.ResponsesResponse, error) {
	// 提取字段
	model, _ := openaiResp["model"].(string)
	choices, _ := openaiResp["choices"].([]interface{})

	// 提取第一个 choice 的 message
	output := []types.ResponsesItem{}
	if len(choices) > 0 {
		choice, ok := choices[0].(map[string]interface{})
		if ok {
			message, _ := choice["message"].(map[string]interface{})
			content, _ := message["content"].(string)
			if content != "" {
				output = append(output, types.ResponsesItem{
					Type: "message",
					Role: "assistant",
					Content: []types.ContentBlock{{
						Type: "output_text",
						Text: content,
					}},
				})
			}
			if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
				for _, rawToolCall := range toolCalls {
					toolCall, ok := rawToolCall.(map[string]interface{})
					if !ok {
						continue
					}
					callID, _ := toolCall["id"].(string)
					function, _ := toolCall["function"].(map[string]interface{})
					name, _ := function["name"].(string)
					arguments, _ := function["arguments"].(string)
					output = append(output, types.ResponsesItem{
						Type:      "function_call",
						Status:    "completed",
						CallID:    callID,
						Name:      name,
						Arguments: arguments,
					})
				}
			}
		}
	}

	// 提取 usage（使用统一入口自动检测格式）
	usage := ExtractUsageMetrics(openaiResp["usage"])

	// 生成 response ID
	responseID := generateResponseID()

	return &types.ResponsesResponse{
		ID:         responseID,
		Model:      model,
		Output:     output,
		Status:     "completed",
		PreviousID: "",
		Usage:      usage,
	}, nil
}

// ============== 工具函数 ==============

// extractTextFromContent 从 content 中提取文本内容
// 支持三种格式：
// 1. string - 直接返回
// 2. []ContentBlock - 提取 input_text/output_text 类型的 text 字段
// 3. []interface{} - 动态解析为 ContentBlock
func extractTextFromContent(content interface{}) string {
	// 1. 如果是 string，直接返回
	if str, ok := content.(string); ok {
		return str
	}

	// 2. 如果是 []ContentBlock（已解析类型）
	if blocks, ok := content.([]types.ContentBlock); ok {
		texts := []string{}
		for _, block := range blocks {
			if block.Type == "input_text" || block.Type == "output_text" {
				texts = append(texts, block.Text)
			}
		}
		return strings.Join(texts, "\n")
	}

	// 3. 如果是 []interface{}（未解析类型）
	if arr, ok := content.([]interface{}); ok {
		texts := []string{}
		for _, c := range arr {
			if block, ok := c.(map[string]interface{}); ok {
				blockType, _ := block["type"].(string)
				if blockType == "input_text" || blockType == "output_text" {
					if text, ok := block["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, "\n")
	}

	return ""
}

// parseResponsesInput 解析 input 字段（可能是 string 或 []ResponsesItem）
func parseResponsesInput(input interface{}) ([]types.ResponsesItem, error) {
	return types.ParseResponsesInput(input)
}

// generateResponseID 生成响应ID
func generateResponseID() string {
	return fmt.Sprintf("resp_%d", getCurrentTimestamp())
}

// getCurrentTimestamp 获取当前时间戳（毫秒）
func getCurrentTimestamp() int64 {
	return 0 // 占位符，实际应使用 time.Now().UnixNano() / 1e6
}

// ExtractTextFromResponses 从 Responses 消息中提取纯文本（用于 OpenAI Completions）
func ExtractTextFromResponses(sess *session.Session, newInput interface{}) (string, error) {
	texts := []string{}

	// 历史消息
	for _, item := range sess.Messages {
		if item.Type == "text" {
			if text, ok := item.Content.(string); ok {
				texts = append(texts, text)
			}
		}
	}

	// 新输入
	newItems, err := parseResponsesInput(newInput)
	if err != nil {
		return "", err
	}

	for _, item := range newItems {
		if item.Type == "text" {
			if text, ok := item.Content.(string); ok {
				texts = append(texts, text)
			}
		}
	}

	return strings.Join(texts, "\n"), nil
}

// OpenAICompletionsResponseToResponses OpenAI Completions 响应转 Responses
func OpenAICompletionsResponseToResponses(completionsResp map[string]interface{}, sessionID string) (*types.ResponsesResponse, error) {
	model, _ := completionsResp["model"].(string)
	choices, _ := completionsResp["choices"].([]interface{})

	output := []types.ResponsesItem{}
	if len(choices) > 0 {
		choice, ok := choices[0].(map[string]interface{})
		if ok {
			text, _ := choice["text"].(string)
			output = append(output, types.ResponsesItem{
				Type:    "text",
				Content: text,
			})
		}
	}

	// 提取 usage（使用统一入口自动检测格式）
	usage := ExtractUsageMetrics(completionsResp["usage"])

	responseID := generateResponseID()

	return &types.ResponsesResponse{
		ID:         responseID,
		Model:      model,
		Output:     output,
		Status:     "completed",
		PreviousID: "",
		Usage:      usage,
	}, nil
}

// JSONToMap 将 JSON 字节转为 map
func JSONToMap(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal(data, &result)
	return result, err
}

// getIntFromMap 从 map 中安全提取整数值
// 支持 float64（JSON 反序列化）和 int/int64（内部构造）两种类型
func getIntFromMap(m map[string]interface{}, key string) (int, bool) {
	v, exists := m[key]
	if !exists {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int64:
		return int(val), true
	case int32:
		return int(val), true
	default:
		return 0, false
	}
}

// parseResponsesUsage 解析 Responses API 的 usage 字段
// 完整支持 OpenAI Responses API 的详细 usage 结构
func parseResponsesUsage(usageRaw interface{}) types.ResponsesUsage {
	usage := types.ResponsesUsage{}

	usageMap, ok := usageRaw.(map[string]interface{})
	if !ok {
		return usage
	}

	// 解析基础字段（兼容两种命名风格）
	// OpenAI Responses API: input_tokens / output_tokens
	// OpenAI Chat API: prompt_tokens / completion_tokens
	if v, ok := getIntFromMap(usageMap, "input_tokens"); ok {
		usage.InputTokens = v
	} else if v, ok := getIntFromMap(usageMap, "prompt_tokens"); ok {
		usage.InputTokens = v
	}

	if v, ok := getIntFromMap(usageMap, "output_tokens"); ok {
		usage.OutputTokens = v
	} else if v, ok := getIntFromMap(usageMap, "completion_tokens"); ok {
		usage.OutputTokens = v
	}

	if v, ok := getIntFromMap(usageMap, "total_tokens"); ok {
		usage.TotalTokens = v
	} else {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	// 解析 input_tokens_details（兼容 prompt_tokens_details）
	inputDetailsRaw := usageMap["input_tokens_details"]
	if inputDetailsRaw == nil {
		inputDetailsRaw = usageMap["prompt_tokens_details"]
	}
	if detailsMap, ok := inputDetailsRaw.(map[string]interface{}); ok {
		usage.InputTokensDetails = &types.InputTokensDetails{}
		if v, ok := getIntFromMap(detailsMap, "cached_tokens"); ok {
			usage.InputTokensDetails.CachedTokens = v
		}
	}

	// 解析 output_tokens_details（兼容 completion_tokens_details）
	outputDetailsRaw := usageMap["output_tokens_details"]
	if outputDetailsRaw == nil {
		outputDetailsRaw = usageMap["completion_tokens_details"]
	}
	if detailsMap, ok := outputDetailsRaw.(map[string]interface{}); ok {
		usage.OutputTokensDetails = &types.OutputTokensDetails{}
		if v, ok := getIntFromMap(detailsMap, "reasoning_tokens"); ok {
			usage.OutputTokensDetails.ReasoningTokens = v
		}
	}

	return usage
}

// parseClaudeUsage 解析 Claude API 的 usage 字段
// 完整支持 Claude 的缓存统计，包括 TTL 细分 (5m/1h)
// 参考 claude-code-hub 的 extractUsageMetrics 实现
func parseClaudeUsage(usageRaw interface{}) types.ResponsesUsage {
	usage := types.ResponsesUsage{}

	usageMap, ok := usageRaw.(map[string]interface{})
	if !ok {
		return usage
	}

	// 基础字段
	if v, ok := getIntFromMap(usageMap, "input_tokens"); ok {
		usage.InputTokens = v
	}
	if v, ok := getIntFromMap(usageMap, "output_tokens"); ok {
		usage.OutputTokens = v
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens

	// Claude 缓存创建统计（区分 TTL）
	var cacheCreation, cacheCreation5m, cacheCreation1h int
	var has5m, has1h bool

	// 总缓存创建量
	if v, ok := getIntFromMap(usageMap, "cache_creation_input_tokens"); ok {
		cacheCreation = v
		usage.CacheCreationInputTokens = cacheCreation
	}

	// 5分钟 TTL 缓存创建
	if v, ok := getIntFromMap(usageMap, "cache_creation_5m_input_tokens"); ok {
		cacheCreation5m = v
		usage.CacheCreation5mInputTokens = cacheCreation5m
		has5m = cacheCreation5m > 0
	}

	// 1小时 TTL 缓存创建
	if v, ok := getIntFromMap(usageMap, "cache_creation_1h_input_tokens"); ok {
		cacheCreation1h = v
		usage.CacheCreation1hInputTokens = cacheCreation1h
		has1h = cacheCreation1h > 0
	}

	// 缓存读取
	var cacheRead int
	if v, ok := getIntFromMap(usageMap, "cache_read_input_tokens"); ok {
		cacheRead = v
		usage.CacheReadInputTokens = cacheRead
	}

	// 设置缓存 TTL 标识
	if has5m && has1h {
		usage.CacheTTL = "mixed"
	} else if has1h {
		usage.CacheTTL = "1h"
	} else if has5m {
		usage.CacheTTL = "5m"
	}

	// 同时设置 InputTokensDetails（兼容 OpenAI 格式）
	// CachedTokens = cache_read（仅缓存读取，不包含缓存创建）
	// 注意：cache_creation 是新创建的缓存，不是"已缓存的 token"
	if cacheRead > 0 {
		usage.InputTokensDetails = &types.InputTokensDetails{
			CachedTokens: cacheRead,
		}
	}

	return usage
}

// parseGeminiUsage 解析 Gemini API 的 usage 字段
// Gemini 使用 promptTokenCount/candidatesTokenCount，需要特殊处理缓存去重
// 参考 claude-code-hub: Gemini 的 promptTokenCount 已包含 cachedContentTokenCount，需要扣除避免重复计费
func parseGeminiUsage(usageRaw interface{}) types.ResponsesUsage {
	usage := types.ResponsesUsage{}

	usageMap, ok := usageRaw.(map[string]interface{})
	if !ok {
		return usage
	}

	var promptTokens, cachedTokens, outputTokens int

	// Gemini 字段名
	if v, ok := getIntFromMap(usageMap, "promptTokenCount"); ok {
		promptTokens = v
	}
	if v, ok := getIntFromMap(usageMap, "cachedContentTokenCount"); ok {
		cachedTokens = v
	}
	if v, ok := getIntFromMap(usageMap, "candidatesTokenCount"); ok {
		outputTokens = v
	}

	// 关键处理：Gemini 的 promptTokenCount 已包含 cachedContentTokenCount
	// 为避免重复计费，实际输入 token = promptTokenCount - cachedContentTokenCount
	actualInputTokens := promptTokens - cachedTokens
	if actualInputTokens < 0 {
		actualInputTokens = 0
	}

	usage.InputTokens = actualInputTokens
	usage.OutputTokens = outputTokens
	usage.TotalTokens = actualInputTokens + outputTokens

	// 缓存读取统计
	if cachedTokens > 0 {
		usage.CacheReadInputTokens = cachedTokens
		usage.InputTokensDetails = &types.InputTokensDetails{
			CachedTokens: cachedTokens,
		}
	}

	return usage
}

// ExtractUsageMetrics 多格式 Token 提取统一入口
// 自动检测并解析 Claude/Gemini/OpenAI 三种格式的 usage
// 参考 claude-code-hub 的 extractUsageMetrics 实现
func ExtractUsageMetrics(usageRaw interface{}) types.ResponsesUsage {
	usageMap, ok := usageRaw.(map[string]interface{})
	if !ok {
		return types.ResponsesUsage{}
	}

	// 1. 检测 Claude 格式：有 cache_creation_input_tokens 或 cache_read_input_tokens
	if _, hasCacheCreation := usageMap["cache_creation_input_tokens"]; hasCacheCreation {
		return parseClaudeUsage(usageRaw)
	}
	if _, hasCacheRead := usageMap["cache_read_input_tokens"]; hasCacheRead {
		return parseClaudeUsage(usageRaw)
	}

	// 2. 检测 Gemini 格式：有 promptTokenCount
	if _, hasPromptTokenCount := usageMap["promptTokenCount"]; hasPromptTokenCount {
		return parseGeminiUsage(usageRaw)
	}

	// 3. 默认 OpenAI 格式
	return parseResponsesUsage(usageRaw)
}
