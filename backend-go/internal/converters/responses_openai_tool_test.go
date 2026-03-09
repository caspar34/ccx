package converters

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/session"
	"github.com/BenedictKing/ccx/internal/types"
	"github.com/stretchr/testify/assert"
)

// TestResponsesToOpenAIChatMessages_ToolCall 测试 Responses tool_call 转 OpenAI tool_calls
func TestResponsesToOpenAIChatMessages_ToolCall(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 模拟 Responses tool_call
	newInput := []interface{}{
		map[string]interface{}{
			"type":    "text",
			"content": "Search for Go tutorials",
		},
		map[string]interface{}{
			"type": "tool_call",
			"tool_use": map[string]interface{}{
				"id":   "toolu_abc123",
				"name": "web_search",
				"input": map[string]interface{}{
					"query": "golang tutorials",
				},
			},
		},
	}

	messages, err := ResponsesToOpenAIChatMessages(sess, newInput, "")
	assert.NoError(t, err)
	assert.Len(t, messages, 2)

	// 验证第一条消息 (user text)
	assert.Equal(t, "user", messages[0]["role"])
	assert.Equal(t, "Search for Go tutorials", messages[0]["content"])

	// 验证第二条消息 (assistant with tool_calls)
	assert.Equal(t, "assistant", messages[1]["role"])

	// Responses tool_call 应该转换为 OpenAI tool_calls
	toolCalls, ok := messages[1]["tool_calls"].([]map[string]interface{})
	assert.True(t, ok, "应该包含 tool_calls 字段")
	assert.Len(t, toolCalls, 1)

	assert.Equal(t, "toolu_abc123", toolCalls[0]["id"])
	assert.Equal(t, "function", toolCalls[0]["type"])

	function, ok := toolCalls[0]["function"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "web_search", function["name"])

	// arguments 应该是 JSON 字符串
	argsStr, ok := function["arguments"].(string)
	assert.True(t, ok)
	assert.Contains(t, argsStr, "golang tutorials")
}

// TestResponsesToOpenAIChatMessages_ToolResult 测试 Responses tool_result 转 OpenAI tool message
func TestResponsesToOpenAIChatMessages_ToolResult(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 模拟 Responses tool_result
	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"tool_use_id": "toolu_abc123",
				"content":     "Found 10 tutorials",
			},
		},
	}

	messages, err := ResponsesToOpenAIChatMessages(sess, newInput, "")
	assert.NoError(t, err)
	assert.Len(t, messages, 1)

	// 验证 tool message
	assert.Equal(t, "tool", messages[0]["role"])
	assert.Equal(t, "toolu_abc123", messages[0]["tool_call_id"])
	assert.Equal(t, "Found 10 tutorials", messages[0]["content"])
}

// TestResponsesToOpenAIChatMessages_ToolResult_ObjectContent 测试对象类型的 tool_result
func TestResponsesToOpenAIChatMessages_ToolResult_ObjectContent(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 模拟对象类型的 tool_result
	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"tool_use_id": "toolu_xyz789",
				"content": map[string]interface{}{
					"results": []string{"Tutorial 1", "Tutorial 2"},
					"count":   2,
				},
			},
		},
	}

	messages, err := ResponsesToOpenAIChatMessages(sess, newInput, "")
	assert.NoError(t, err)
	assert.Len(t, messages, 1)

	// 验证 tool message
	assert.Equal(t, "tool", messages[0]["role"])
	assert.Equal(t, "toolu_xyz789", messages[0]["tool_call_id"])

	// content 应该是 JSON 字符串
	content, ok := messages[0]["content"].(string)
	assert.True(t, ok)
	assert.Contains(t, content, "Tutorial 1")
	assert.Contains(t, content, "Tutorial 2")
}

// TestResponsesToOpenAIChatMessages_MultipleToolCalls 测试多个工具调用
func TestResponsesToOpenAIChatMessages_MultipleToolCalls(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	newInput := []interface{}{
		map[string]interface{}{
			"type": "tool_call",
			"tool_use": map[string]interface{}{
				"id":   "toolu_001",
				"name": "get_weather",
				"input": map[string]interface{}{
					"location": "Tokyo",
				},
			},
		},
		map[string]interface{}{
			"type": "tool_call",
			"tool_use": map[string]interface{}{
				"id":   "toolu_002",
				"name": "get_time",
				"input": map[string]interface{}{
					"timezone": "Asia/Tokyo",
				},
			},
		},
	}

	messages, err := ResponsesToOpenAIChatMessages(sess, newInput, "")
	assert.NoError(t, err)

	// 应该合并为一条 assistant 消息
	assert.Len(t, messages, 2)
	assert.Equal(t, "assistant", messages[0]["role"])
	assert.Equal(t, "assistant", messages[1]["role"])

	// 每条消息应该有一个 tool_call
	toolCalls0, ok := messages[0]["tool_calls"].([]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, toolCalls0, 1)
	assert.Equal(t, "get_weather", toolCalls0[0]["function"].(map[string]interface{})["name"])

	toolCalls1, ok := messages[1]["tool_calls"].([]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, toolCalls1, 1)
	assert.Equal(t, "get_time", toolCalls1[0]["function"].(map[string]interface{})["name"])
}

// TestResponsesToOpenAIChatMessages_ToolCallRoundtrip 测试完整的工具调用流程
func TestResponsesToOpenAIChatMessages_ToolCallRoundtrip(t *testing.T) {
	sess := &session.Session{
		Messages: []types.ResponsesItem{},
	}

	// 模拟完整的工具调用流程
	newInput := []interface{}{
		map[string]interface{}{
			"type":    "text",
			"content": "What's the weather?",
		},
		map[string]interface{}{
			"type": "tool_call",
			"tool_use": map[string]interface{}{
				"id":   "toolu_123",
				"name": "get_weather",
				"input": map[string]interface{}{
					"location": "Tokyo",
				},
			},
		},
		map[string]interface{}{
			"type": "tool_result",
			"content": map[string]interface{}{
				"tool_use_id": "toolu_123",
				"content":     "22°C, Sunny",
			},
		},
	}

	messages, err := ResponsesToOpenAIChatMessages(sess, newInput, "")
	assert.NoError(t, err)
	assert.Len(t, messages, 3)

	// 验证消息序列
	assert.Equal(t, "user", messages[0]["role"])
	assert.Equal(t, "assistant", messages[1]["role"])
	assert.Equal(t, "tool", messages[2]["role"])

	// 验证 tool_call_id 匹配
	toolCalls, _ := messages[1]["tool_calls"].([]map[string]interface{})
	assert.Equal(t, "toolu_123", toolCalls[0]["id"])
	assert.Equal(t, "toolu_123", messages[2]["tool_call_id"])
}
