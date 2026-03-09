package converters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/BenedictKing/ccx/internal/session"
	"github.com/BenedictKing/ccx/internal/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ============== Responses → Gemini 请求转换 ==============

// ResponsesToGeminiRequest 将 Responses 请求转换为 Gemini 格式
func ResponsesToGeminiRequest(sess *session.Session, req *types.ResponsesRequest, modelName string) (*types.GeminiRequest, error) {
	geminiReq := &types.GeminiRequest{
		Contents: []types.GeminiContent{},
	}

	// 1. 转换 instructions → systemInstruction
	if req.Instructions != "" {
		geminiReq.SystemInstruction = &types.GeminiContent{
			Parts: []types.GeminiPart{
				{Text: req.Instructions},
			},
		}
	}

	// 2. 转换历史消息
	for _, item := range sess.Messages {
		contents := responsesItemToGeminiContents(item)
		geminiReq.Contents = append(geminiReq.Contents, contents...)
	}

	// 3. 转换新输入
	newItems, err := parseResponsesInput(req.Input)
	if err != nil {
		return nil, err
	}

	for _, item := range newItems {
		contents := responsesItemToGeminiContents(item)
		geminiReq.Contents = append(geminiReq.Contents, contents...)
	}

	// 4. 转换生成配置
	geminiReq.GenerationConfig = &types.GeminiGenerationConfig{}
	if req.MaxTokens > 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		geminiReq.GenerationConfig.Temperature = &temp
	}
	if req.TopP > 0 {
		topP := req.TopP
		geminiReq.GenerationConfig.TopP = &topP
	}

	// 5. 转换 tools
	if len(req.Tools) > 0 {
		geminiReq.Tools = responsesToolsToGemini(req.Tools)
	}

	return geminiReq, nil
}

// responsesItemToGeminiContents 将 Responses Item 转换为 Gemini Contents
func responsesItemToGeminiContents(item types.ResponsesItem) []types.GeminiContent {
	item = types.NormalizeResponsesItem(item)
	contents := []types.GeminiContent{}

	switch item.Type {
	case "message":
		// 消息类型
		role, contentText := resolveResponsesTextItem(item)
		role = normalizeGeminiRole(role)
		if contentText == "" {
			return nil
		}

		contents = append(contents, types.GeminiContent{
			Role: role,
			Parts: []types.GeminiPart{
				{Text: contentText},
			},
		})

	case "text":
		// 旧格式文本
		role, contentStr := resolveResponsesTextItem(item)
		role = normalizeGeminiRole(role)
		if contentStr == "" {
			return nil
		}

		contents = append(contents, types.GeminiContent{
			Role: role,
			Parts: []types.GeminiPart{
				{Text: contentStr},
			},
		})

	case "function_call":
		_, name, arguments, err := resolveFunctionCallItem(item)
		if err != nil {
			return nil
		}

		contents = append(contents, types.GeminiContent{
			Role: "model",
			Parts: []types.GeminiPart{
				{
					FunctionCall: &types.GeminiFunctionCall{
						Name:             name,
						Args:             parseGeminiFunctionCallArgs(arguments),
						ThoughtSignature: types.DummyThoughtSignature,
					},
				},
			},
		})

	case "function_call_output":
		callID, output, err := resolveFunctionCallOutputItem(item)
		if err != nil {
			return nil
		}

		contents = append(contents, types.GeminiContent{
			Role: "user",
			Parts: []types.GeminiPart{
				{
					FunctionResponse: &types.GeminiFunctionResponse{
						Name:     callID,
						Response: buildGeminiFunctionResponsePayload(output),
					},
				},
			},
		})
	}

	return contents
}

// ============== Gemini → Responses 响应转换 ==============

// GeminiResponseToResponses 将 Gemini 响应转换为 Responses 格式
func GeminiResponseToResponses(geminiResp map[string]interface{}, sessionID string) (*types.ResponsesResponse, error) {
	// 提取 candidates
	candidates, _ := geminiResp["candidates"].([]interface{})
	if len(candidates) == 0 {
		return &types.ResponsesResponse{
			ID:     generateResponseID(),
			Status: "failed",
		}, nil
	}

	candidate, _ := candidates[0].(map[string]interface{})
	content, _ := candidate["content"].(map[string]interface{})
	parts, _ := content["parts"].([]interface{})

	// 转换 output
	output := []types.ResponsesItem{}

	// 收集文本
	var textParts []string
	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		if text, ok := part["text"].(string); ok && text != "" {
			textParts = append(textParts, text)
		}
	}

	// 添加文本消息
	if len(textParts) > 0 {
		output = append(output, types.ResponsesItem{
			Type: "message",
			Role: "assistant",
			Content: []types.ContentBlock{
				{
					Type: "output_text",
					Text: strings.Join(textParts, "\n"),
				},
			},
		})
	}

	// 处理工具调用
	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		if functionCall, ok := part["functionCall"].(map[string]interface{}); ok {
			name, _ := functionCall["name"].(string)
			args, _ := functionCall["args"].(map[string]interface{})
			argsJSON, _ := JSONMarshal(args)

			// 使用函数名作为 call_id,与 function_call_output 的 name 字段保持一致
			output = append(output, types.ResponsesItem{
				Type:      "function_call",
				Role:      "assistant",
				CallID:    name, // 使用函数名而非随机 ID
				Name:      name,
				Arguments: string(argsJSON),
			})
		}
	}

	// 转换 finishReason → status
	finishReason, _ := candidate["finishReason"].(string)
	status := geminiFinishReasonToResponsesStatus(finishReason)

	// 提取 usage
	usage := ExtractUsageMetrics(geminiResp["usageMetadata"])

	// 生成 response ID
	responseID := generateResponseID()

	return &types.ResponsesResponse{
		ID:     responseID,
		Model:  "",
		Output: output,
		Status: status,
		Usage:  usage,
	}, nil
}

// geminiFinishReasonToResponsesStatus 将 Gemini finishReason 转换为 Responses status
func geminiFinishReasonToResponsesStatus(finishReason string) string {
	switch finishReason {
	case "STOP":
		return "completed"
	case "MAX_TOKENS":
		return "incomplete"
	case "SAFETY", "RECITATION":
		return "failed"
	default:
		return "completed"
	}
}

// ============== Gemini → Responses 流式转换 ==============

// geminiToResponsesStreamState 流式转换状态
type geminiToResponsesStreamState struct {
	Seq          int
	ResponseID   string
	CreatedAt    int64
	CurrentMsgID string
	TextBuf      strings.Builder
	FirstChunk   bool
	InTextBlock  bool
	// usage
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	UsageSeen    bool
	// function calls
	FunctionCalls []map[string]interface{}
}

// ConvertGeminiStreamToResponses 将 Gemini SSE 转换为 Responses SSE
func ConvertGeminiStreamToResponses(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	if *param == nil {
		*param = &geminiToResponsesStreamState{
			FirstChunk: true,
		}
	}
	st := (*param).(*geminiToResponsesStreamState)

	// 期望 `data: {..}` 格式
	line := string(rawJSON)
	if !strings.HasPrefix(line, "data: ") {
		return []string{}
	}
	jsonStr := strings.TrimPrefix(line, "data: ")
	jsonStr = strings.TrimSpace(jsonStr)

	root := gjson.Parse(jsonStr)
	var out []string

	nextSeq := func() int { st.Seq++; return st.Seq }

	// 处理首次 chunk
	if st.FirstChunk {
		st.FirstChunk = false
		st.ResponseID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
		st.CreatedAt = time.Now().Unix()
		st.TextBuf.Reset()
		st.InTextBlock = false
		st.CurrentMsgID = ""
		st.InputTokens = 0
		st.OutputTokens = 0
		st.CachedTokens = 0
		st.UsageSeen = false

		// 发送 response.created
		created := `{"type":"response.created","sequence_number":0,"response":{"id":"","object":"response","created_at":0,"status":"in_progress","background":false,"error":null,"instructions":""}}`
		created, _ = sjson.Set(created, "sequence_number", nextSeq())
		created, _ = sjson.Set(created, "response.id", st.ResponseID)
		created, _ = sjson.Set(created, "response.created_at", st.CreatedAt)
		out = append(out, emitResponsesEvent("response.created", created))

		// 发送 response.in_progress
		inprog := `{"type":"response.in_progress","sequence_number":0,"response":{"id":"","object":"response","created_at":0,"status":"in_progress"}}`
		inprog, _ = sjson.Set(inprog, "sequence_number", nextSeq())
		inprog, _ = sjson.Set(inprog, "response.id", st.ResponseID)
		inprog, _ = sjson.Set(inprog, "response.created_at", st.CreatedAt)
		out = append(out, emitResponsesEvent("response.in_progress", inprog))
	}

	// 解析 candidates
	candidates := root.Get("candidates")
	if !candidates.Exists() || !candidates.IsArray() {
		return out
	}

	for _, candidate := range candidates.Array() {
		content := candidate.Get("content")
		if !content.Exists() {
			continue
		}

		parts := content.Get("parts")
		if !parts.Exists() || !parts.IsArray() {
			continue
		}

		for _, part := range parts.Array() {
			// 处理文本内容
			if text := part.Get("text"); text.Exists() && text.String() != "" {
				textContent := text.String()

				// 开始 text block
				if !st.InTextBlock {
					st.InTextBlock = true
					st.CurrentMsgID = fmt.Sprintf("msg_%s_0", st.ResponseID)

					// response.output_item.added
					item := `{"type":"response.output_item.added","sequence_number":0,"output_index":0,"item":{"id":"","type":"message","status":"in_progress","content":[],"role":"assistant"}}`
					item, _ = sjson.Set(item, "sequence_number", nextSeq())
					item, _ = sjson.Set(item, "item.id", st.CurrentMsgID)
					out = append(out, emitResponsesEvent("response.output_item.added", item))

					// response.content_part.added
					partEvent := `{"type":"response.content_part.added","sequence_number":0,"item_id":"","output_index":0,"content_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}}`
					partEvent, _ = sjson.Set(partEvent, "sequence_number", nextSeq())
					partEvent, _ = sjson.Set(partEvent, "item_id", st.CurrentMsgID)
					out = append(out, emitResponsesEvent("response.content_part.added", partEvent))
				}

				// 发送 text delta
				st.TextBuf.WriteString(textContent)
				msg := `{"type":"response.output_text.delta","sequence_number":0,"item_id":"","output_index":0,"content_index":0,"delta":"","logprobs":[]}`
				msg, _ = sjson.Set(msg, "sequence_number", nextSeq())
				msg, _ = sjson.Set(msg, "item_id", st.CurrentMsgID)
				msg, _ = sjson.Set(msg, "delta", textContent)
				out = append(out, emitResponsesEvent("response.output_text.delta", msg))
			}

			// 处理工具调用
			if functionCall := part.Get("functionCall"); functionCall.Exists() {
				name := functionCall.Get("name").String()
				args := functionCall.Get("args").Value()

				// 使用函数名作为 call_id,与 function_call_output 的 name 字段保持一致
				argsJSON, _ := JSONMarshal(args)

				// 收集到状态中，稍后在 completed 事件中输出
				st.FunctionCalls = append(st.FunctionCalls, map[string]interface{}{
					"call_id":   name, // 使用函数名而非随机 ID
					"name":      name,
					"arguments": string(argsJSON),
				})
			}
		}

		// 处理 finishReason
		if finishReason := candidate.Get("finishReason"); finishReason.Exists() && finishReason.String() != "" {
			// 先处理 usage（如果在同一 chunk 中）
			if usage := root.Get("usageMetadata"); usage.Exists() {
				st.UsageSeen = true
				if v := usage.Get("promptTokenCount"); v.Exists() {
					st.InputTokens = v.Int()
				}
				if v := usage.Get("candidatesTokenCount"); v.Exists() {
					st.OutputTokens = v.Int()
				}
				if v := usage.Get("cachedContentTokenCount"); v.Exists() {
					st.CachedTokens = v.Int()
				}
			}

			// 关闭 text block
			if st.InTextBlock {
				// response.output_text.done
				done := `{"type":"response.output_text.done","sequence_number":0,"item_id":"","output_index":0,"content_index":0,"text":"","logprobs":[]}`
				done, _ = sjson.Set(done, "sequence_number", nextSeq())
				done, _ = sjson.Set(done, "item_id", st.CurrentMsgID)
				done, _ = sjson.Set(done, "text", st.TextBuf.String())
				out = append(out, emitResponsesEvent("response.output_text.done", done))

				// response.content_part.done
				partDone := `{"type":"response.content_part.done","sequence_number":0,"item_id":"","output_index":0,"content_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}}`
				partDone, _ = sjson.Set(partDone, "sequence_number", nextSeq())
				partDone, _ = sjson.Set(partDone, "item_id", st.CurrentMsgID)
				partDone, _ = sjson.Set(partDone, "part.text", st.TextBuf.String())
				out = append(out, emitResponsesEvent("response.content_part.done", partDone))

				// response.output_item.done
				final := `{"type":"response.output_item.done","sequence_number":0,"output_index":0,"item":{"id":"","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":""}],"role":"assistant"}}`
				final, _ = sjson.Set(final, "sequence_number", nextSeq())
				final, _ = sjson.Set(final, "item.id", st.CurrentMsgID)
				final, _ = sjson.Set(final, "item.content.0.text", st.TextBuf.String())
				out = append(out, emitResponsesEvent("response.output_item.done", final))

				st.InTextBlock = false
			}

			// 发送 response.completed
			out = append(out, st.generateCompletedEvent(originalRequestRawJSON, finishReason.String())...)
		}
	}

	// 处理 usage（如果不在 finishReason chunk 中，单独处理）
	if usage := root.Get("usageMetadata"); usage.Exists() && !st.UsageSeen {
		st.UsageSeen = true
		if v := usage.Get("promptTokenCount"); v.Exists() {
			st.InputTokens = v.Int()
		}
		if v := usage.Get("candidatesTokenCount"); v.Exists() {
			st.OutputTokens = v.Int()
		}
		if v := usage.Get("cachedContentTokenCount"); v.Exists() {
			st.CachedTokens = v.Int()
		}
	}

	return out
}

// generateCompletedEvent 生成完成事件
func (st *geminiToResponsesStreamState) generateCompletedEvent(originalRequestRawJSON []byte, finishReason string) []string {
	var out []string
	nextSeq := func() int { st.Seq++; return st.Seq }

	// 将 Gemini finishReason 转换为 Responses status
	status := geminiFinishReasonToResponsesStatus(finishReason)

	// 构建 response.completed
	completed := `{"type":"response.completed","sequence_number":0,"response":{"id":"","object":"response","created_at":0,"status":"completed","background":false,"error":null}}`
	completed, _ = sjson.Set(completed, "sequence_number", nextSeq())
	completed, _ = sjson.Set(completed, "response.id", st.ResponseID)
	completed, _ = sjson.Set(completed, "response.created_at", st.CreatedAt)
	completed, _ = sjson.Set(completed, "response.status", status) // 使用转换后的 status

	// 注入原始请求字段
	if originalRequestRawJSON != nil {
		req := gjson.ParseBytes(originalRequestRawJSON)
		if v := req.Get("instructions"); v.Exists() {
			completed, _ = sjson.Set(completed, "response.instructions", v.String())
		}
		if v := req.Get("max_output_tokens"); v.Exists() {
			completed, _ = sjson.Set(completed, "response.max_output_tokens", v.Int())
		}
		if v := req.Get("model"); v.Exists() {
			completed, _ = sjson.Set(completed, "response.model", v.String())
		}
	}

	// 构建 output
	var outputs []interface{}

	// 添加文本消息（如果有）
	if st.TextBuf.Len() > 0 || st.CurrentMsgID != "" {
		m := map[string]interface{}{
			"id":     st.CurrentMsgID,
			"type":   "message",
			"status": "completed",
			"content": []interface{}{map[string]interface{}{
				"type":        "output_text",
				"annotations": []interface{}{},
				"logprobs":    []interface{}{},
				"text":        st.TextBuf.String(),
			}},
			"role": "assistant",
		}
		outputs = append(outputs, m)
	}

	// 添加工具调用（如果有）
	for _, fc := range st.FunctionCalls {
		outputs = append(outputs, map[string]interface{}{
			"type":    "function_call",
			"role":    "assistant",
			"content": fc,
		})
	}

	if len(outputs) > 0 {
		completed, _ = sjson.Set(completed, "response.output", outputs)
	}

	// 添加 usage
	// Gemini 的 promptTokenCount 已包含 cachedContentTokenCount，需要扣除
	actualInput := st.InputTokens - st.CachedTokens
	if actualInput < 0 {
		actualInput = 0
	}

	completed, _ = sjson.Set(completed, "response.usage.input_tokens", actualInput)
	completed, _ = sjson.Set(completed, "response.usage.output_tokens", st.OutputTokens)
	completed, _ = sjson.Set(completed, "response.usage.total_tokens", actualInput+st.OutputTokens)

	if st.CachedTokens > 0 {
		completed, _ = sjson.Set(completed, "response.usage.input_tokens_details.cached_tokens", st.CachedTokens)
		completed, _ = sjson.Set(completed, "response.usage.cache_read_input_tokens", st.CachedTokens)
	}

	out = append(out, emitResponsesEvent("response.completed", completed))
	return out
}
