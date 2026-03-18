package common

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMetadataUserID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // expected user_id value after normalization, empty means unchanged
	}{
		{
			name:     "v2.1.78 JSON - only device_id",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"device_id\":\"b854c106939c\",\"account_uuid\":\"\",\"session_id\":\"\"}"},"stream":true}`,
			expected: "user_b854c106939c",
		},
		{
			name:     "v2.1.78 JSON - device_id + session_id",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"device_id\":\"b854c106939c\",\"account_uuid\":\"\",\"session_id\":\"e692f803-4767\"}"}}`,
			expected: "user_b854c106939c_session_e692f803-4767",
		},
		{
			name:     "v2.1.78 JSON - device_id + account_uuid",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"device_id\":\"dev1\",\"account_uuid\":\"acc1\",\"session_id\":\"\"}"}}`,
			expected: "user_dev1_account_acc1",
		},
		{
			name:     "v2.1.78 JSON - all fields populated",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"device_id\":\"abc123\",\"account_uuid\":\"uuid-456\",\"session_id\":\"sess-789\"}"}}`,
			expected: "user_abc123_account_uuid-456_session_sess-789",
		},
		{
			name:     "v2.1.78 JSON - empty device_id, fallback to generic format",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"device_id\":\"\",\"account_uuid\":\"acc1\",\"session_id\":\"sess1\"}"}}`,
			expected: "account_uuid_acc1_session_id_sess1",
		},
		{
			name:     "v2.1.77 flat string user_id - no change",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"user_67bad5_account__session_7581b58b"},"stream":true}`,
			expected: "user_67bad5_account__session_7581b58b",
		},
		{
			name:     "no metadata - no change",
			input:    `{"model":"claude-opus-4-6","stream":true}`,
			expected: "",
		},
		{
			name:     "empty user_id - no change",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":""},"stream":true}`,
			expected: "",
		},
		{
			name:     "invalid JSON in user_id - no change",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{invalid json"}}`,
			expected: "{invalid json",
		},
		{
			name:     "non-claude JSON object - generic key_value format",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"uid\":\"abc123\"}"}}`,
			expected: "uid_abc123",
		},
		{
			name:     "non-claude JSON object - multiple fields sorted",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"session\":\"xyz\",\"uid\":\"abc123\"}"}}`,
			expected: "session_xyz_uid_abc123",
		},
		{
			name:     "preserves other fields",
			input:    `{"model":"claude-opus-4-6","metadata":{"user_id":"{\"device_id\":\"dev1\",\"account_uuid\":\"acc1\",\"session_id\":\"sess1\"}"},"stream":true,"max_tokens":1024}`,
			expected: "user_dev1_account_acc1_session_sess1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMetadataUserID([]byte(tt.input))

			if tt.expected == "" {
				// Should be unchanged or no user_id
				var data map[string]interface{}
				if err := json.Unmarshal(result, &data); err != nil {
					t.Fatalf("failed to parse result: %v", err)
				}
				metadata, ok := data["metadata"].(map[string]interface{})
				if !ok {
					return // no metadata, as expected
				}
				userID, _ := metadata["user_id"].(string)
				if userID != "" {
					var origData map[string]interface{}
					json.Unmarshal([]byte(tt.input), &origData)
					origMeta, _ := origData["metadata"].(map[string]interface{})
					origUID, _ := origMeta["user_id"].(string)
					if userID != origUID {
						t.Errorf("user_id changed unexpectedly: got %q, want %q", userID, origUID)
					}
				}
				return
			}

			var data map[string]interface{}
			if err := json.Unmarshal(result, &data); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}
			metadata, ok := data["metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("metadata not found in result")
			}
			userID, ok := metadata["user_id"].(string)
			if !ok {
				t.Fatal("user_id not found in metadata")
			}
			if userID != tt.expected {
				t.Errorf("user_id = %q, want %q", userID, tt.expected)
			}

			// Verify other fields are preserved
			var origData map[string]interface{}
			json.Unmarshal([]byte(tt.input), &origData)
			if origModel, ok := origData["model"].(string); ok {
				if resultModel, ok := data["model"].(string); ok {
					if origModel != resultModel {
						t.Errorf("model changed: got %q, want %q", resultModel, origModel)
					}
				}
			}
		})
	}
}
