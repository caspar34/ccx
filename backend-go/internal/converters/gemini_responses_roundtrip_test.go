package converters

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/BenedictKing/ccx/internal/session"
	"github.com/BenedictKing/ccx/internal/types"
)

func TestResponsesToGeminiRequest_PreservesFunctionItems(t *testing.T) {
	sess := &session.Session{ID: "sess_test"}
	req := &types.ResponsesRequest{
		Model: "gpt-4.1",
		Input: []interface{}{
			map[string]interface{}{
				"type":      "function_call",
				"call_id":   "weather_call",
				"name":      "get_weather",
				"arguments": `{"location":"NYC"}`,
			},
			map[string]interface{}{
				"type":    "function_call_output",
				"call_id": "weather_call",
				"output":  "Sunny, 72°F",
			},
		},
	}

	geminiReq, err := ResponsesToGeminiRequest(sess, req, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("ResponsesToGeminiRequest failed: %v", err)
	}

	if len(geminiReq.Contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(geminiReq.Contents))
	}

	callPart := geminiReq.Contents[0].Parts[0].FunctionCall
	if callPart == nil {
		t.Fatal("expected first content to be function call")
	}
	if callPart.Name != "get_weather" {
		t.Fatalf("expected function name get_weather, got %q", callPart.Name)
	}
	if callPart.Args["location"] != "NYC" {
		t.Fatalf("expected args.location NYC, got %#v", callPart.Args["location"])
	}

	respPart := geminiReq.Contents[1].Parts[0].FunctionResponse
	if respPart == nil {
		t.Fatal("expected second content to be function response")
	}
	if respPart.Name != "weather_call" {
		t.Fatalf("expected function response name weather_call, got %q", respPart.Name)
	}
	if respPart.Response["result"] != "Sunny, 72°F" {
		t.Fatalf("expected tool result preserved, got %#v", respPart.Response["result"])
	}
}

func TestGeminiResponseToResponses_UsesStableFunctionNameAsCallID(t *testing.T) {
	geminiResp := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"parts": []interface{}{
						map[string]interface{}{
							"functionCall": map[string]interface{}{
								"name": "get_weather",
								"args": map[string]interface{}{"location": "NYC"},
							},
						},
					},
				},
				"finishReason": "STOP",
			},
		},
	}

	resp, err := GeminiResponseToResponses(geminiResp, "sess_test")
	if err != nil {
		t.Fatalf("GeminiResponseToResponses failed: %v", err)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}

	content, ok := resp.Output[0].Content.(map[string]interface{})
	if !ok {
		t.Fatal("expected function_call content to be map")
	}
	if content["name"] != "get_weather" {
		t.Fatalf("expected name get_weather, got %#v", content["name"])
	}
	if content["call_id"] != "get_weather" {
		t.Fatalf("expected call_id get_weather, got %#v", content["call_id"])
	}

	followupReq := &types.ResponsesRequest{
		Model: "gpt-4.1",
		Input: []interface{}{
			map[string]interface{}{
				"type":    "function_call_output",
				"call_id": content["call_id"],
				"output":  "Sunny, 72°F",
			},
		},
	}
	geminiReq, err := ResponsesToGeminiRequest(&session.Session{ID: "sess_test"}, followupReq, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("ResponsesToGeminiRequest followup failed: %v", err)
	}
	if len(geminiReq.Contents) != 1 || len(geminiReq.Contents[0].Parts) != 1 {
		t.Fatalf("expected single function response content, got %#v", geminiReq.Contents)
	}
	functionResponse := geminiReq.Contents[0].Parts[0].FunctionResponse
	if functionResponse == nil {
		t.Fatal("expected function response in followup request")
	}
	if functionResponse.Name != "get_weather" {
		t.Fatalf("expected function response name get_weather, got %q", functionResponse.Name)
	}
}

func TestConvertGeminiStreamToResponses_MapsFinishReasonToStatus(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name         string
		finishReason string
		expectStatus string
	}{
		{name: "stop maps to completed", finishReason: "STOP", expectStatus: "completed"},
		{name: "max tokens maps to incomplete", finishReason: "MAX_TOKENS", expectStatus: "incomplete"},
		{name: "safety maps to failed", finishReason: "SAFETY", expectStatus: "failed"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var state any
			line := `data: {"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"` + tt.finishReason + `"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`
			events := ConvertGeminiStreamToResponses(ctx, "gemini-2.5-pro", []byte(`{"model":"gpt-4.1"}`), nil, []byte(line), &state)

			var completed string
			for _, ev := range events {
				if strings.Contains(ev, "response.completed") {
					completed = ev
					break
				}
			}
			if completed == "" {
				t.Fatalf("expected response.completed event, got %#v", events)
			}

			var payload struct {
				Response struct {
					Status string `json:"status"`
				} `json:"response"`
			}
			jsonLine := completed
			if strings.HasPrefix(jsonLine, "event: ") {
				parts := strings.Split(completed, "\n")
				for _, part := range parts {
					if strings.HasPrefix(part, "data: ") {
						jsonLine = strings.TrimPrefix(part, "data: ")
						break
					}
				}
			}
			if err := json.Unmarshal([]byte(jsonLine), &payload); err != nil {
				t.Fatalf("unmarshal completed event failed: %v; event=%s", err, completed)
			}
			if payload.Response.Status != tt.expectStatus {
				t.Fatalf("expected status %q, got %q", tt.expectStatus, payload.Response.Status)
			}
		})
	}
}

func TestConvertGeminiStreamToResponses_PreservesFunctionCallCallID(t *testing.T) {
	ctx := context.Background()
	var state any
	line := `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"location":"NYC"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`
	events := ConvertGeminiStreamToResponses(ctx, "gemini-2.5-pro", []byte(`{"model":"gpt-4.1"}`), nil, []byte(line), &state)

	var completed string
	for _, ev := range events {
		if strings.Contains(ev, "response.completed") {
			completed = ev
			break
		}
	}
	if completed == "" {
		t.Fatalf("expected response.completed event, got %#v", events)
	}

	jsonLine := completed
	if strings.HasPrefix(jsonLine, "event: ") {
		parts := strings.Split(completed, "\n")
		for _, part := range parts {
			if strings.HasPrefix(part, "data: ") {
				jsonLine = strings.TrimPrefix(part, "data: ")
				break
			}
		}
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(jsonLine), &payload); err != nil {
		t.Fatalf("unmarshal completed event failed: %v", err)
	}
	response := payload["response"].(map[string]interface{})
	output := response["output"].([]interface{})
	if len(output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(output))
	}
	item := output[0].(map[string]interface{})
	content := item["content"].(map[string]interface{})
	if content["name"] != "get_weather" {
		t.Fatalf("expected function name get_weather, got %#v", content["name"])
	}
	if content["call_id"] != "get_weather" {
		t.Fatalf("expected call_id get_weather, got %#v", content["call_id"])
	}
}
