package converters

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/session"
	"github.com/BenedictKing/ccx/internal/types"
)

// TestClaudeResponseToResponses_ToolUse 测试 Claude 响应中的 tool_use 转换
func TestClaudeResponseToResponses_ToolUse(t *testing.T) {
	claudeResp := map[string]interface{}{
		"id":    "msg_123",
		"model": "claude-3-5-sonnet-20241022",
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Let me search for that information.",
			},
			map[string]interface{}{
				"type": "tool_use",
				"id":   "toolu_abc123",
				"name": "web_search",
				"input": map[string]interface{}{
					"query": "golang best practices",
				},
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":  100,
			"output_tokens": 50,
		},
	}

	result, err := ClaudeResponseToResponses(claudeResp, "session_123")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 验证输出包含 text 和 tool_call
	if len(result.Output) != 2 {
		t.Fatalf("期望 2 个输出项，实际 %d", len(result.Output))
	}

	// 验证 text 项
	if result.Output[0].Type != "text" {
		t.Errorf("第一项类型期望 'text'，实际 '%s'", result.Output[0].Type)
	}
	if result.Output[0].Content != "Let me search for that information." {
		t.Errorf("text 内容不匹配")
	}

	// 验证 tool_call 项
	if result.Output[1].Type != "tool_call" {
		t.Errorf("第二项类型期望 'tool_call'，实际 '%s'", result.Output[1].Type)
	}
	if result.Output[1].ToolUse == nil {
		t.Fatal("tool_call 缺少 ToolUse 字段")
	}
	if result.Output[1].ToolUse.ID != "toolu_abc123" {
		t.Errorf("tool_use.id 期望 'toolu_abc123'，实际 '%s'", result.Output[1].ToolUse.ID)
	}
	if result.Output[1].ToolUse.Name != "web_search" {
		t.Errorf("tool_use.name 期望 'web_search'，实际 '%s'", result.Output[1].ToolUse.Name)
	}

	// 验证 input 字段
	inputMap, ok := result.Output[1].ToolUse.Input.(map[string]interface{})
	if !ok {
		t.Fatal("tool_use.input 类型错误")
	}
	if inputMap["query"] != "golang best practices" {
		t.Errorf("tool_use.input.query 不匹配")
	}
}

// TestResponsesToClaudeMessages_ToolCall 测试 Responses tool_call 转 Claude
func TestResponsesToClaudeMessages_ToolCall(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_call",
			"tool_use": map[string]interface{}{
				"id":   "toolu_xyz789",
				"name": "get_weather",
				"input": map[string]interface{}{
					"location": "San Francisco",
				},
			},
		},
	}

	messages, _, err := ResponsesToClaudeMessages(sess, newInput, "")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("期望 1 条消息，实际 %d", len(messages))
	}

	// 验证消息角色
	if messages[0].Role != "assistant" {
		t.Errorf("tool_call 角色期望 'assistant'，实际 '%s'", messages[0].Role)
	}

	// 验证内容块
	content, ok := messages[0].Content.([]types.ClaudeContent)
	if !ok {
		t.Fatal("Content 类型错误")
	}
	if len(content) != 1 {
		t.Fatalf("期望 1 个内容块，实际 %d", len(content))
	}

	// 验证 tool_use 内容块
	if content[0].Type != "tool_use" {
		t.Errorf("内容块类型期望 'tool_use'，实际 '%s'", content[0].Type)
	}
	if content[0].ID != "toolu_xyz789" {
		t.Errorf("tool_use.id 期望 'toolu_xyz789'，实际 '%s'", content[0].ID)
	}
	if content[0].Name != "get_weather" {
		t.Errorf("tool_use.name 期望 'get_weather'，实际 '%s'", content[0].Name)
	}

	inputMap, ok := content[0].Input.(map[string]interface{})
	if !ok {
		t.Fatal("tool_use.input 类型错误")
	}
	if inputMap["location"] != "San Francisco" {
		t.Errorf("tool_use.input.location 不匹配")
	}
}

// TestResponsesToClaudeMessages_ToolResult 测试 Responses tool_result 转 Claude
func TestResponsesToClaudeMessages_ToolResult(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 测试字符串结果
	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"tool_use_id": "toolu_xyz789",
				"content":     "Temperature: 72°F, Sunny",
			},
		},
	}

	messages, _, err := ResponsesToClaudeMessages(sess, newInput, "")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("期望 1 条消息，实际 %d", len(messages))
	}

	// 验证消息角色
	if messages[0].Role != "user" {
		t.Errorf("tool_result 角色期望 'user'，实际 '%s'", messages[0].Role)
	}

	// 验证内容块
	content, ok := messages[0].Content.([]types.ClaudeContent)
	if !ok {
		t.Fatal("Content 类型错误")
	}
	if len(content) != 1 {
		t.Fatalf("期望 1 个内容块，实际 %d", len(content))
	}

	// 验证 tool_result 内容块
	if content[0].Type != "tool_result" {
		t.Errorf("内容块类型期望 'tool_result'，实际 '%s'", content[0].Type)
	}
	if content[0].ToolUseID != "toolu_xyz789" {
		t.Errorf("tool_result.tool_use_id 期望 'toolu_xyz789'，实际 '%s'", content[0].ToolUseID)
	}
	if content[0].Content != "Temperature: 72°F, Sunny" {
		t.Errorf("tool_result.content 不匹配")
	}
}

// TestResponsesToClaudeMessages_ToolResult_ObjectContent 测试对象类型的 tool_result
func TestResponsesToClaudeMessages_ToolResult_ObjectContent(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 测试对象结果
	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"tool_use_id": "toolu_abc123",
				"content": map[string]interface{}{
					"temperature": 72,
					"condition":   "sunny",
				},
			},
		},
	}

	messages, _, err := ResponsesToClaudeMessages(sess, newInput, "")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	content, ok := messages[0].Content.([]types.ClaudeContent)
	if !ok {
		t.Fatal("Content 类型错误")
	}

	// 验证对象内容
	contentMap, ok := content[0].Content.(map[string]interface{})
	if !ok {
		t.Fatal("tool_result.content 应该是 map[string]interface{}")
	}
	if contentMap["temperature"] != 72 {
		t.Errorf("temperature 不匹配")
	}
	if contentMap["condition"] != "sunny" {
		t.Errorf("condition 不匹配")
	}
}

// TestResponsesToClaudeMessages_RoundtripShape 测试往返转换的结构正确性
func TestResponsesToClaudeMessages_RoundtripShape(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 模拟完整的工具调用流程
	newInput := []interface{}{
		map[string]interface{}{
			"type":    "text",
			"content": "Please search for Go tutorials",
		},
		map[string]interface{}{
			"type": "tool_call",
			"tool_use": map[string]interface{}{
				"id":   "toolu_001",
				"name": "web_search",
				"input": map[string]interface{}{
					"query": "golang tutorials",
				},
			},
		},
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"tool_use_id": "toolu_001",
				"content":     "Found 10 tutorials",
			},
		},
	}

	messages, _, err := ResponsesToClaudeMessages(sess, newInput, "")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 验证消息数量和角色序列
	if len(messages) != 3 {
		t.Fatalf("期望 3 条消息，实际 %d", len(messages))
	}

	expectedRoles := []string{"user", "assistant", "user"}
	for i, msg := range messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("消息 %d 角色期望 '%s'，实际 '%s'", i, expectedRoles[i], msg.Role)
		}
	}

	// 验证第一条消息（text）
	content0, ok := messages[0].Content.([]types.ClaudeContent)
	if !ok || len(content0) != 1 || content0[0].Type != "text" {
		t.Error("第一条消息应该是 text 类型")
	}

	// 验证第二条消息（tool_use）
	content1, ok := messages[1].Content.([]types.ClaudeContent)
	if !ok || len(content1) != 1 || content1[0].Type != "tool_use" {
		t.Error("第二条消息应该是 tool_use 类型")
	}

	// 验证第三条消息（tool_result）
	content2, ok := messages[2].Content.([]types.ClaudeContent)
	if !ok || len(content2) != 1 || content2[0].Type != "tool_result" {
		t.Error("第三条消息应该是 tool_result 类型")
	}
}

// TestClaudeResponseToResponses_TextOnlyRegression 回归测试：纯文本场景
func TestClaudeResponseToResponses_TextOnlyRegression(t *testing.T) {
	claudeResp := map[string]interface{}{
		"id":    "msg_456",
		"model": "claude-3-5-sonnet-20241022",
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Hello, how can I help you?",
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":  50,
			"output_tokens": 20,
		},
	}

	result, err := ClaudeResponseToResponses(claudeResp, "session_456")
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 验证纯文本场景不受影响
	if len(result.Output) != 1 {
		t.Fatalf("期望 1 个输出项，实际 %d", len(result.Output))
	}

	if result.Output[0].Type != "text" {
		t.Errorf("类型期望 'text'，实际 '%s'", result.Output[0].Type)
	}

	if result.Output[0].Content != "Hello, how can I help you?" {
		t.Errorf("内容不匹配")
	}

	// 验证 usage
	if result.Usage.InputTokens != 50 {
		t.Errorf("input_tokens 期望 50，实际 %d", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 20 {
		t.Errorf("output_tokens 期望 20，实际 %d", result.Usage.OutputTokens)
	}
}

// TestResponsesToClaudeMessages_ToolCallMissingToolUse 测试缺少 tool_use 字段的兼容性处理
func TestResponsesToClaudeMessages_ToolCallMissingToolUse(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// tool_call 但缺少 tool_use 字段（历史消息场景）
	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_call",
		},
	}

	messages, _, err := ResponsesToClaudeMessages(sess, newInput, "")
	// 应该跳过而不是报错，保持向后兼容
	if err != nil {
		t.Errorf("不应该返回错误，应该跳过: %v", err)
	}
	// 应该返回空消息列表（跳过了无效的 tool_call）
	if len(messages) != 0 {
		t.Errorf("期望返回 0 条消息（跳过无效 tool_call），实际 %d", len(messages))
	}
}

// TestResponsesToClaudeMessages_ToolResultMissingToolUseID 测试缺少 tool_use_id 的错误处理
func TestResponsesToClaudeMessages_ToolResultMissingToolUseID(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// tool_result 但缺少 tool_use_id
	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"content": "some result",
			},
		},
	}

	_, _, err := ResponsesToClaudeMessages(sess, newInput, "")
	if err == nil {
		t.Error("期望返回错误，但成功了")
	}
	if err.Error() != "转换新消息失败: tool_result 缺少 tool_use_id" {
		t.Errorf("错误信息不匹配: %v", err)
	}
}
